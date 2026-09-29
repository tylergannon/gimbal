// This executable is a consumer-owned, single-run Temporal worker and console.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/claude"
	"github.com/tylergannon/gimbal/contextdata"
	"github.com/tylergannon/gimbal/web"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

const coder gimbal.WorkflowRole = "coder"

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}
func run() error {
	address := flag.String("temporal", "127.0.0.1:7233", "Temporal development server address")
	root := flag.String("state", "", "absolute retained state directory (default: new temporary directory)")
	port := flag.Int("port", 8084, "Gimbal console port")
	delay := flag.Duration("delay", 2*time.Second, "deterministic provider turn duration; increase to exercise cancellation")
	provider := flag.String("provider", "demo", "demo (deterministic substitute) or claude (configured Claude Code credentials)")
	keep := flag.Bool("keep-open", false, "keep console serving after workflow finishes until interrupted")
	view := flag.Bool("view", false, "serve retained runs from -state without starting Temporal work")
	flag.Parse()
	if *root == "" {
		d, err := os.MkdirTemp("", "gimbal-temporal-")
		if err != nil {
			return err
		}
		*root = d
	}
	absolute, err := filepath.Abs(*root)
	if err != nil {
		return err
	}
	*root = absolute
	project := filepath.Join(*root, "project")
	if err = os.MkdirAll(project, 0755); err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	store := contextdata.Store{Root: filepath.Join(*root, "context"), LocalDir: filepath.Join(*root, "materialized")}
	instance, err := web.NewInstance(ctx, filepath.Join(*root, "instance"), []string{project}, web.WithPort(*port), web.WithContextStore(project, &store, filepath.Join(*root, "web-cache")))
	if err != nil {
		return err
	}
	defer func() { stop(); instance.Wait() }()
	fmt.Printf("Gimbal console: http://127.0.0.1:%d\nRetained state: %s\n", *port, *root)
	if *view {
		<-ctx.Done()
		return nil
	}
	c, err := client.Dial(client.Options{HostPort: *address})
	if err != nil {
		return err
	}
	defer c.Close()
	id := fmt.Sprintf("delivery-%d", time.Now().UnixNano())
	var adapter gimbal.HarnessAdapter
	model := "deterministic-demo"
	switch *provider {
	case "demo":
		adapter = &demoAdapter{delay: *delay}
	case "claude":
		adapter = claude.New()
		model = "claude-haiku-4-5"
	default:
		return fmt.Errorf("unknown provider %q", *provider)
	}
	fmt.Printf("Provider: %s; model: %s\n", *provider, model)
	a := &Activities{workdir: project, store: store, rootContext: ctx, models: map[gimbal.WorkflowRole]gimbal.ModelBinding{coder: {Adapter: adapter, Model: model}}}
	a.open = func(runCtx context.Context, name string, models map[gimbal.WorkflowRole]gimbal.ModelBinding, initial contextdata.Snapshot, local string) (context.Context, func(error) error, error) {
		return instance.OpenCompiledRun(runCtx, project, name, models, initial, local, web.CompiledControls{CancelRun: func(delivery context.Context, _ gimbal.Killed) error { return c.CancelWorkflow(delivery, id, "") }})
	}
	// A single local owner uses a dedicated queue. Other processes cannot resolve
	// its session/scope handles; the example deliberately has no failover promise.
	queue := controlQueue + "-" + id
	controlQueue = queue
	w := worker.New(c, queue, worker.Options{MaxHeartbeatThrottleInterval: time.Second, DefaultHeartbeatThrottleInterval: time.Second})
	registerDelivery(w)
	w.RegisterActivity(a)
	w.RegisterActivity(ProvisionEnvironment)
	w.RegisterActivity(ReleaseEnvironment)
	if err = w.Start(); err != nil {
		return err
	}
	defer w.Stop()
	entry, err := contextdata.Encode("request", "Demonstrate an externally generated workflow.")
	if err != nil {
		return err
	}
	initial, err := store.Extend(ctx, "", entry)
	if err != nil {
		return err
	}
	run, err := c.ExecuteWorkflow(ctx, client.StartWorkflowOptions{ID: id, TaskQueue: queue}, DeliveryWorkflow, Input{Context: initial})
	if err != nil {
		return err
	}
	fmt.Printf("Temporal workflow: %s\n", id)
	result := make(chan error, 1)
	go func() { result <- run.Get(context.Background(), nil) }()
	select {
	case err = <-result:
	case <-ctx.Done():
		cancelCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		deliveryErr := c.CancelWorkflow(cancelCtx, id, "")
		cancel()
		select {
		case err = <-result:
		case <-time.After(20 * time.Second):
			err = errors.New("workflow cleanup not confirmed before shutdown")
		}
		err = errors.Join(err, deliveryErr)
	}
	fmt.Printf("Workflow outcome: %v\n", err)
	if *keep && ctx.Err() == nil {
		<-ctx.Done()
	}
	return err
}

// The consumer chooses its execution environment. This bounded example owns one
// local workspace; resource cleanup happens in Activities.Finish, not here.
func ProvisionEnvironment(context.Context) (Environment, error) {
	return Environment{Queue: controlQueue}, nil
}
func ReleaseEnvironment(context.Context) error { return nil }
