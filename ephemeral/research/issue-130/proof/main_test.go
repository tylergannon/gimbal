package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/tylergannon/gimble/web"
)

func TestWaitRunStopsWhenRunFailsBeforeCreatingALog(t *testing.T) {
	project := t.TempDir()
	if err := os.WriteFile(filepath.Join(project, "runs"), []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	runtime, err := web.NewRuntime(ctx, project, web.WithNoWeb())
	if err != nil {
		t.Fatal(err)
	}
	waitCtx, stopWaiting := context.WithCancel(ctx)
	defer stopWaiting()
	var runErr error
	var runWG sync.WaitGroup
	runWG.Go(func() {
		defer stopWaiting()
		runErr = runtime.Run(ctx, "startup-failure", func(context.Context) error { return nil })
	})

	_, waitErr := waitRun(waitCtx, project)
	runWG.Wait()
	if !errors.Is(waitErr, context.Canceled) {
		t.Fatalf("waitRun = %v, want cancellation when Run returned", waitErr)
	}
	if runErr == nil {
		t.Fatal("Run returned nil after runs was a regular file")
	}
}
