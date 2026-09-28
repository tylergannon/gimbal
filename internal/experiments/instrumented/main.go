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
	mode := flag.String("mode", "start", "start, control, activities, or view")
	address := flag.String("temporal", "127.0.0.1:7233", "Temporal server address")
	queue := flag.String("queue", controlQueue, "activity worker queue")
	task := flag.String("task", "Verify command evidence and workspace continuity.", "small, non-sensitive task context")
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
		instance, err := web.NewInstance(ctx, instanceDir, []string{*projectPath}, web.WithPort(8082))
		if err != nil {
			return err
		}
		defer instance.Wait()
		log.Print("Recorded run UI: http://127.0.0.1:8082")
		<-ctx.Done()
		return nil
	}
	c, err := client.Dial(client.Options{HostPort: *address})
	if err != nil {
		return err
	}
	defer c.Close()
	switch *mode {
	case "start":
		id := fmt.Sprintf("instrumented-%d", time.Now().UnixMilli())
		run, err := c.ExecuteWorkflow(ctx, client.StartWorkflowOptions{ID: id, TaskQueue: controlQueue}, ReviewWorkflow, Input{Task: *task})
		if err != nil {
			return err
		}
		fmt.Printf("Temporal workflow: %s\n", run.GetID())
		var report Report
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
		w.RegisterWorkflow(ReviewWorkflow)
		w.RegisterActivity(ProvisionEnvironment)
		w.RegisterActivity(ReleaseEnvironment)
		if err := w.Start(); err != nil {
			return err
		}
		defer w.Stop()
		<-ctx.Done()
		return nil
	case "activities":
		instance, err := web.NewInstance(ctx, "/tmp/instrumented-instance", []string{"/workspace"}, web.WithPort(8081))
		if err != nil {
			return err
		}
		project, err := instance.Owner.Project("/workspace")
		if err != nil {
			return err
		}
		a := &Activities{project: project, workdir: "/workspace"}
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
		w := worker.New(c, *queue, worker.Options{MaxConcurrentActivityExecutionSize: 1, MaxHeartbeatThrottleInterval: time.Second, DefaultHeartbeatThrottleInterval: time.Second})
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
	sum := sha256.Sum256([]byte(activity.GetInfo(ctx).WorkflowExecution.ID))
	return fmt.Sprintf("gimbal-specimen-%x", sum[:8])
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
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0777); err != nil {
		return Environment{}, err
	}
	if err := os.Chmod(dir, 0777); err != nil {
		return Environment{}, err
	}
	// Configuration names only: credential values never enter activity arguments.
	args := []string{"run", "-d", "--name", name, "--init", "--user", "1000:1000", "--publish", "127.0.0.1::8080", "--mount", "type=bind,src=" + dir + ",dst=/workspace", "--env", "HOME=/tmp/agent-home", "--env", "CLAUDE_CODE_OAUTH_TOKEN", "--env", "ANTHROPIC_API_KEY", "--env", "DIFFUSION_API_KEY", "--env", "SPECIMEN_WORKFLOW_ID=" + activity.GetInfo(ctx).WorkflowExecution.ID, "--entrypoint", "/usr/local/bin/instrumented", image, "-mode", "activities", "-queue", name, "-temporal", "host.docker.internal:7233"}
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
