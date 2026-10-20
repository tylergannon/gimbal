// Command interactive hosts a durable, human-first conversation in the web app.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/internal/invocation"
	"github.com/tylergannon/gimbal/web"
)

func main() {
	projectFlag := flag.String("project", ".", "project directory")
	workDirFlag := flag.String("work-dir", "", "execution directory (default: project)")
	purpose := flag.String("purpose", "", "what the conversation should help accomplish (required)")
	model := flag.String("model", "luna", "model for the agentic-dialogues role")
	port := flag.Int("port", 8081, "loopback web port")
	flag.Parse()
	if *purpose == "" || flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "interactive: give --purpose and no positional arguments")
		os.Exit(2)
	}
	err := serve(*projectFlag, *workDirFlag, *purpose, *model, *port)
	if err != nil {
		fmt.Fprintln(os.Stderr, "interactive:", err)
		os.Exit(1)
	}
}

func serve(project, workdir, purpose, model string, port int) error {
	project, err := filepath.Abs(project)
	if err != nil {
		return err
	}
	if workdir == "" {
		workdir = project
	}
	workdir, err = filepath.Abs(workdir)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	instance, err := web.NewInstance(ctx, filepath.Join(project, ".gimbal"), []string{project}, web.WithPort(port))
	if err != nil {
		return err
	}
	runtime, err := instance.Owner.Project(project)
	if err != nil {
		return err
	}
	// NewInstance reconstructs persisted invocations. Reuse the example's saved
	// identity while it is unfinished instead of creating a second conversation.
	marker := filepath.Join(project, ".gimbal", "interactive-example")
	saved, readErr := os.ReadFile(marker)
	if readErr != nil && !os.IsNotExist(readErr) {
		return readErr
	}
	id := strings.TrimSpace(string(saved))
	if id != "" {
		snap, snapshotErr := runtime.Registry().Snapshot(id)
		if snapshotErr != nil || snap.Run.Status == "completed" || snap.Run.Status == "failed" || snap.Run.Status == "cancelled" {
			id = ""
		}
	}
	if id == "" {
		input, err := json.Marshal(struct {
			Purpose string `json:"purpose"`
		}{purpose})
		if err != nil {
			return err
		}
		id, err = runtime.Start(invocation.Request{Workflow: "conversation", Input: input, WorkDir: workdir, Roles: map[gimbal.WorkflowRole]string{gimbal.RoleAgenticDialogues: model}})
		if err != nil {
			return err
		}
		if err := os.WriteFile(marker, []byte(id+"\n"), 0600); err != nil {
			return err
		}
	}
	fmt.Printf("Send the opening message at http://127.0.0.1:%d/ (run %s). Restart this command to resume it.\n", port, id)
	<-ctx.Done()
	return nil
}
