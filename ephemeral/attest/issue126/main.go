// Run with go run ./ephemeral/attest/issue126. It keeps Run's result and the
// concurrent log reader separate, then leaves the durable record in logs/.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/tylergannon/gimble"
	"github.com/tylergannon/gimble/codex"
	"github.com/tylergannon/gimble/internal/runlog"
)

const model = "gpt-5.6-luna"

func main() {
	base, err := filepath.Abs("ephemeral/attest/issue126")
	if err != nil {
		log.Fatal(err)
	}
	logs := filepath.Join(base, "logs")
	if err := os.RemoveAll(logs); err != nil {
		log.Fatal(err)
	}
	if err := os.MkdirAll(logs, 0o755); err != nil {
		log.Fatal(err)
	}
	workspace, err := os.MkdirTemp("", "gimble-issue126-")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(workspace)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	fmt.Printf("Model: codex=%s\n", model)

	observed := make(chan struct{}) // readiness, not a result channel
	var observerOnce sync.Once
	releaseRun := func() { observerOnce.Do(func() { close(observed) }) }
	var result gimble.Text
	var runErr error
	var runWG sync.WaitGroup
	runWG.Go(func() {
		runErr = gimble.Run(gimble.Project(ctx, logs), "completion", func(ctx context.Context) error {
			worker := gimble.NewSession(ctx, "worker", codex.New(), model, workspace)
			var err error
			result, err = worker.Generate[gimble.Text](ctx, "Reply with exactly ISSUE_126_LIVE and do nothing else.")
			if err != nil {
				return err
			}
			<-observed // the external reader has started before the body returns
			return nil
		})
	})

	runDir := waitRunDir(ctx, logs)
	var complete bool
	readErr := runlog.Read[gimble.LifecycleRecord](ctx, runDir, func(record gimble.LifecycleRecord) error {
		switch record.Event.(type) {
		case gimble.RunStarted:
			releaseRun()
		case gimble.Complete:
			complete = true
		}
		return nil
	})
	releaseRun() // also releases the body if observation itself failed
	runWG.Wait()

	if readErr != nil {
		log.Fatalf("read run log: %v", readErr)
	}
	if runErr != nil {
		log.Fatalf("Run: %v", runErr)
	}
	if !complete || strings.TrimSpace(string(result)) != "ISSUE_126_LIVE" {
		log.Fatalf("complete=%v result=%q", complete, result)
	}
	fmt.Printf("Run: %s\n", runDir)
	fmt.Printf("Result: %s\n", result)
	fmt.Println("Proof: reader reached complete; Run returned nil after its explicit join.")
}

func waitRunDir(ctx context.Context, logs string) string {
	for {
		entries, _ := os.ReadDir(filepath.Join(logs, "runs"))
		if len(entries) == 1 {
			dir := filepath.Join(logs, "runs", entries[0].Name())
			if _, err := os.Stat(filepath.Join(dir, "run.jsonl")); err == nil {
				return dir
			}
		}
		if err := ctx.Err(); err != nil {
			log.Fatal(err)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
