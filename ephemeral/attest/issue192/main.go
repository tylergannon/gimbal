// Run with go run ./ephemeral/attest/issue192. It proves that a reader may
// attach as soon as the run directory exists, before run.jsonl is created.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/tylergannon/gimble"
	"github.com/tylergannon/gimble/internal/runlog"
)

type envelope struct {
	Event struct {
		Kind string `json:"kind"`
	} `json:"event"`
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	proveStartupGap(ctx)
	proveRuntimeWatch(ctx)
}

func proveStartupGap(ctx context.Context) {
	dir, err := os.MkdirTemp("", "gimble-issue192-gap-")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(dir)

	attached := make(chan struct{})
	done := make(chan error, 1)
	var kinds []string
	go func() {
		close(attached)
		done <- runlog.Read[envelope](ctx, dir, func(record envelope) error {
			kinds = append(kinds, record.Event.Kind)
			return nil
		})
	}()
	<-attached
	time.Sleep(50 * time.Millisecond)
	select {
	case err := <-done:
		log.Fatalf("reader returned before run.jsonl existed: %v", err)
	default:
	}

	const records = "{\"event\":{\"kind\":\"run_started\"}}\n{\"event\":{\"kind\":\"complete\"}}\n"
	if err := os.WriteFile(filepath.Join(dir, "run.jsonl"), []byte(records), 0o600); err != nil {
		log.Fatal(err)
	}
	if err := <-done; err != nil {
		log.Fatal(err)
	}
	if fmt.Sprint(kinds) != "[run_started complete]" {
		log.Fatalf("startup-gap records = %v", kinds)
	}
	fmt.Printf("Startup gap: reader attached before run.jsonl and saw %v\n", kinds)
}

func proveRuntimeWatch(ctx context.Context) {
	project, err := os.MkdirTemp("", "gimble-issue192-runtime-")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(project)

	release := make(chan struct{})
	var releaseOnce sync.Once
	releaseRun := func() { releaseOnce.Do(func() { close(release) }) }
	runDone := make(chan error, 1)
	go func() {
		runDone <- gimble.Run(gimble.Project(ctx, project), "startup-race", func(context.Context) error {
			<-release
			return nil
		})
	}()

	dir := waitRunDir(ctx, project)
	var kinds []string
	readErr := runlog.Read[gimble.LifecycleRecord](ctx, dir, func(record gimble.LifecycleRecord) error {
		switch record.Event.(type) {
		case gimble.RunStarted:
			kinds = append(kinds, "run_started")
			releaseRun()
		case gimble.Complete:
			kinds = append(kinds, "complete")
		}
		return nil
	})
	releaseRun()
	runErr := <-runDone
	if readErr != nil || runErr != nil || fmt.Sprint(kinds) != "[run_started complete]" {
		log.Fatalf("runtime watch: Read=%v Run=%v records=%v", readErr, runErr, kinds)
	}
	fmt.Printf("Runtime watch: directory-only discovery of %s saw %v\n", filepath.Base(dir), kinds)
}

func waitRunDir(ctx context.Context, project string) string {
	for ctx.Err() == nil {
		entries, err := os.ReadDir(filepath.Join(project, "runs"))
		if err == nil && len(entries) == 1 {
			return filepath.Join(project, "runs", entries[0].Name())
		}
		time.Sleep(time.Millisecond)
	}
	log.Fatal(ctx.Err())
	return ""
}
