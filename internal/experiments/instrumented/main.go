package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/internal/compiledscope"
	"github.com/tylergannon/gimbal/internal/experiments/instrumented/fanout"
	"github.com/tylergannon/gimbal/internal/host"
	"github.com/tylergannon/gimbal/internal/observation"
	"github.com/tylergannon/gimbal/web"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	example := flag.String("workflow", "continuity", "continuity, fanout, or planning")
	itemsJSON := flag.String("items-json", `[{"File":"one.txt","Receipt":"duplicate","Delay":8},{"File":"two.txt","Receipt":"duplicate","Delay":4}]`, "assignments for the two authored fanout branches as JSON")
	mode := flag.String("mode", "start", "start, control, activities, or view")
	address := flag.String("temporal", "127.0.0.1:7233", "Temporal server address")
	queue := flag.String("queue", controlQueue, "activity worker queue")
	task := flag.String("task", "Continue a conversation across scoped work.", "small, non-sensitive task context")
	projectPath := flag.String("project", "", "completed specimen workspace to view")
	flag.Parse()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if *mode == "view" {
		if *projectPath == "" {
			return errors.New("view requires -project pointing to a completed specimen workspace")
		}
		instanceDir, err := os.MkdirTemp("", "instrumented-view-")
		if err != nil {
			return err
		}
		defer func() { _ = os.RemoveAll(instanceDir) }()
		projects := append([]string{*projectPath}, flag.Args()...)
		viewCtx := ctx
		for _, project := range projects {
			store := &compiledscope.Store{Root: filepath.Join(filepath.Dir(project), "context")}
			viewCtx, err = host.WithContextStore(viewCtx, project, store, filepath.Join(project, ".gimbal", "context-cache"))
			if err != nil {
				return err
			}
		}
		instance, err := web.NewInstance(viewCtx, instanceDir, projects, web.WithPort(8082))
		if err != nil {
			return err
		}
		defer instance.Wait()
		log.Print("Recorded run UI: http://127.0.0.1:8082")
		<-ctx.Done()
		return nil
	}
	c, err := client.Dial(client.Options{HostPort: *address, DataConverter: dataConverter(stateRoot())})
	if err != nil {
		return err
	}
	defer c.Close()
	switch *mode {
	case "start":
		id := fmt.Sprintf("%s-%d", *example, time.Now().UnixMilli())
		input, err := prepareInput(stateRoot(), id, *task)
		if err != nil {
			return err
		}
		var entry any
		var argument any = input
		switch *example {
		case "continuity":
			entry = ContinuityWorkflow
		case "planning":
			entry = PlanningWorkflow
		case "fanout":
			var items []fanout.Work
			if err = json.Unmarshal([]byte(*itemsJSON), &items); err != nil {
				return err
			}
			entry = FanoutWorkflow
			if len(items) != 2 {
				return errors.New("fanout requires exactly two assignments for its authored branches")
			}
			argument = fanoutInput{Input: input, Items: [2]fanout.Work(items)}
		default:
			return fmt.Errorf("unknown workflow %q", *example)
		}
		run, err := c.ExecuteWorkflow(ctx, client.StartWorkflowOptions{ID: id, TaskQueue: controlQueue}, entry, argument)
		if err != nil {
			return err
		}
		fmt.Printf("Temporal workflow: %s\n", run.GetID())
		var report json.RawMessage
		err = run.Get(ctx, &report)
		if ctx.Err() != nil {
			cancelCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			_ = c.CancelWorkflow(cancelCtx, run.GetID(), run.GetRunID())
		}
		if err != nil {
			return err
		}
		return json.NewEncoder(os.Stdout).Encode(report)
	case "control":
		w := worker.New(c, controlQueue, worker.Options{})
		registerContinuity(w)
		w.RegisterWorkflow(FanoutWorkflow)
		registerPlanning(w)
		w.RegisterActivity(ProvisionEnvironment)
		w.RegisterActivity(ReleaseEnvironment)
		if err := w.Start(); err != nil {
			return err
		}
		defer w.Stop()
		<-ctx.Done()
		return nil
	case "activities":
		workspace, contextDir := os.Getenv("SPECIMEN_WORKSPACE"), os.Getenv("SPECIMEN_CONTEXT")
		if !filepath.IsAbs(workspace) || !filepath.IsAbs(contextDir) {
			return errors.New("activity worker requires absolute workspace and context paths")
		}
		store := compiledscope.Store{Root: contextDir, LocalDir: filepath.Join(contextDir, "materialized")}
		hostCtx, err := host.WithContextStore(ctx, workspace, &store, filepath.Join(workspace, ".gimbal", "context-cache"))
		if err != nil {
			return err
		}
		instance, err := web.NewInstance(hostCtx, "/tmp/instrumented-instance", []string{workspace}, web.WithPort(8081))
		if err != nil {
			return err
		}
		project, err := instance.Owner.Project(workspace)
		if err != nil {
			return err
		}
		a := &Activities{project: project, workdir: workspace, store: store, controls: host.CompiledControls{
			CancelRun: func(ctx context.Context, _ gimbal.Killed) error {
				return c.CancelWorkflow(ctx, os.Getenv("SPECIMEN_WORKFLOW_ID"), "")
			},
		}}
		target, _ := url.Parse("http://127.0.0.1:8081")
		proxy := &httputil.ReverseProxy{Rewrite: func(r *httputil.ProxyRequest) {
			r.SetURL(target)
			if r.In.Header.Get("Origin") != "" {
				r.Out.Header.Set("Origin", target.String())
			}
		}}
		mux := http.NewServeMux()
		mux.HandleFunc("POST /control/steer", func(w http.ResponseWriter, r *http.Request) {
			var input struct{ Run, Session, Message string }
			if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&input); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			landed, err := project.Steer(r.Context(), input.Run, input.Session, input.Message)
			if err != nil {
				http.Error(w, err.Error(), http.StatusConflict)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]bool{"landed": landed})
		})
		mux.HandleFunc("POST /control/cancel", func(w http.ResponseWriter, r *http.Request) {
			if err := c.CancelWorkflow(r.Context(), os.Getenv("SPECIMEN_WORKFLOW_ID"), ""); err != nil {
				http.Error(w, err.Error(), http.StatusBadGateway)
				return
			}
			w.WriteHeader(http.StatusAccepted)
		})
		mux.Handle("/", proxy)
		server := &http.Server{Addr: ":8080", Handler: sameOrigin(mux), ReadHeaderTimeout: 5 * time.Second}
		go func() {
			if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				log.Print(err)
				stop()
			}
		}()
		defer func() { _ = server.Close() }()
		w := worker.New(c, *queue, worker.Options{MaxConcurrentActivityExecutionSize: 4, MaxHeartbeatThrottleInterval: time.Second, DefaultHeartbeatThrottleInterval: time.Second})
		w.RegisterActivity(a)
		if err := w.Start(); err != nil {
			return err
		}
		defer w.Stop()
		<-ctx.Done()
		instance.Wait()
		return nil
	default:
		return fmt.Errorf("unknown mode %q", *mode)
	}
}

func environmentName(ctx context.Context) string {
	return environmentID(activity.GetInfo(ctx).WorkflowExecution.ID)
}

func environmentID(workflowID string) string {
	sum := sha256.Sum256([]byte(workflowID))
	return fmt.Sprintf("gimbal-specimen-%x", sum[:8])
}
func stateRoot() string {
	if root := os.Getenv("SPECIMEN_STATE_ROOT"); root != "" {
		return root
	}
	return "/tmp/gimbal-instrumented-state"
}

// Submission is another producing boundary. Even the initial task reaches
// Temporal by reference, without changing the ordinary workflow's parameters.
func prepareInput(root, id, task string) (Input, error) {
	if !filepath.IsAbs(root) {
		return Input{}, errors.New("SPECIMEN_STATE_ROOT must be absolute")
	}
	store := compiledscope.Store{Root: filepath.Join(root, environmentID(id), "context")}
	// Host submission and the container's unprivileged worker both publish into
	// this retained run store. Object files are separately published read-only.
	for _, sub := range []string{"", "objects", "materialized"} {
		path := filepath.Join(store.Root, sub)
		if err := os.MkdirAll(path, 0777); err != nil {
			return Input{}, err
		}
		if err := os.Chmod(path, 0777); err != nil {
			return Input{}, err
		}
	}
	ref, err := store.Extend(context.Background(), "", contextEntry("task", task))
	return Input{Context: ref}, err
}

func ProvisionEnvironment(ctx context.Context) (Environment, error) {
	name := environmentName(ctx)
	image := os.Getenv("SPECIMEN_IMAGE")
	if image == "" {
		image = "gimbal-instrumented:local"
	}
	root := os.Getenv("SPECIMEN_STATE_ROOT")
	if root == "" || !filepath.IsAbs(root) {
		return Environment{}, errors.New("SPECIMEN_STATE_ROOT must be an absolute host path, mounted at the same path in the control worker")
	}
	runDir := filepath.Join(root, name)
	dir := filepath.Join(runDir, "workspace")
	contextDir := filepath.Join(runDir, "context")
	if err := os.MkdirAll(contextDir, 0777); err != nil {
		return Environment{}, err
	}
	if err := os.Chmod(contextDir, 0777); err != nil {
		return Environment{}, err
	}
	if err := os.MkdirAll(dir, 0777); err != nil {
		return Environment{}, err
	}
	if err := os.Chmod(dir, 0777); err != nil {
		return Environment{}, err
	}
	// Configuration names only: credential values never enter activity arguments.
	args := []string{"run", "-d", "--name", name, "--init", "--user", "1000:1000", "--publish", "127.0.0.1::8080", "--mount", "type=bind,src=" + dir + ",dst=" + dir, "--mount", "type=bind,src=" + contextDir + ",dst=" + contextDir, "--env", "SPECIMEN_STATE_ROOT=" + root, "--env", "SPECIMEN_WORKSPACE=" + dir, "--env", "SPECIMEN_CONTEXT=" + contextDir, "--env", "HOME=/tmp/agent-home", "--env", "CLAUDE_CODE_OAUTH_TOKEN", "--env", "ANTHROPIC_API_KEY", "--env", "DIFFUSION_API_KEY", "--env", "TYPESAFE_API_KEY", "--env", "SPECIMEN_WORKFLOW_ID=" + activity.GetInfo(ctx).WorkflowExecution.ID, "--entrypoint", "/usr/local/bin/instrumented", image, "-mode", "activities", "-queue", name, "-temporal", "host.docker.internal:7233"}
	output, err := exec.CommandContext(ctx, "docker", args...).CombinedOutput()
	if err != nil {
		return Environment{}, fmt.Errorf("start activity container: %w: %s", err, output)
	}
	port, err := exec.CommandContext(ctx, "docker", "port", name, "8080/tcp").Output()
	if err != nil {
		return Environment{}, err
	}
	endpoint := "http://" + strings.TrimSpace(string(port))
	log.Printf("Activity container %s; Gimbal UI and controls: %s; files: %s", name, endpoint, dir)
	return Environment{Name: name, Queue: name, URL: endpoint}, nil
}

func ReleaseEnvironment(ctx context.Context) error {
	name := environmentName(ctx)
	// docker stop sends SIGTERM, allowing the worker to close sessions and logs.
	_ = exec.CommandContext(ctx, "docker", "stop", "--time", "20", name).Run()
	output, err := exec.CommandContext(ctx, "docker", "rm", "-f", name).CombinedOutput()
	if err != nil && !strings.Contains(string(output), "No such container") {
		return fmt.Errorf("remove activity container: %w: %s", err, output)
	}
	// The container is gone: the control worker is now the sole authority
	// able to settle an interrupted record. Never invent resource-close events.
	runs, err := filepath.Glob(filepath.Join(stateRoot(), name, "workspace", ".gimbal", "runs", "*"))
	if err != nil {
		return err
	}
	for _, dir := range runs {
		if err = observation.RecordTerminated(dir, "Orchestrator: activity worker stopped without a terminal run record; worker-local cleanup is unconfirmed."); err != nil {
			return err
		}
	}
	return nil
}

// Browsers must originate control requests from this worker's own UI.
// Requests without Origin are the explicit CLI/RPC path.
func sameOrigin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if origin := r.Header.Get("Origin"); origin != "" && origin != "http://"+r.Host {
			http.Error(w, "origin mismatch", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}
