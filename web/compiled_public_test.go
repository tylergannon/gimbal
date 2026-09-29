package web_test

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/contextdata"
	"github.com/tylergannon/gimbal/web"
	"github.com/tylergannon/gimbal/workflow"
)

// An external package can enter, observe and close a hosted compiled run using
// only the supported packages. Console delivery and retained artifact serving
// are exercised against this same public entry by compiled_control_test.go.
func TestPublicCompiledRunParticipatesInInstanceShutdown(t *testing.T) {
	const name = "public-compiled-entry"
	gimbal.RegisterGraph(workflow.Graph{Name: name})
	project := t.TempDir()
	store := &contextdata.Store{Root: t.TempDir()}
	entry, err := contextdata.Encode("input", "submitted before hosted entry")
	if err != nil {
		t.Fatal(err)
	}
	initial, err := store.Extend(t.Context(), "", entry)
	if err != nil {
		t.Fatal(err)
	}
	ctx, stop := context.WithCancel(t.Context())
	instance, err := web.NewInstance(ctx, t.TempDir(), []string{project},
		web.WithNoWeb(), web.WithContextStore(project, store, t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { stop(); instance.Wait() })
	root, finish, err := instance.OpenCompiledRun(t.Context(), project, name,
		nil, initial, t.TempDir(), web.CompiledControls{
			CancelRun: func(context.Context, gimbal.Killed) error {
				t.Error("instance shutdown delivered an operator cancellation")
				return nil
			},
		})
	if err != nil {
		t.Fatal(err)
	}
	stop()
	select {
	case <-root.Done():
	case <-time.After(time.Second):
		t.Fatal("instance shutdown did not cancel compiled work")
	}
	if err := finish(context.Cause(root)); !errors.Is(err, context.Canceled) {
		t.Fatalf("finish = %v; want cancelled", err)
	}
	instance.Wait()
	runs, err := os.ReadDir(filepath.Join(project, ".gimbal", "runs"))
	if err != nil || len(runs) != 1 {
		t.Fatalf("hosted records = %v, %v", runs, err)
	}
	file, err := os.Open(filepath.Join(project, ".gimbal", "runs", runs[0].Name(), "run.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = file.Close() }()
	var wroteInput bool
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var record gimbal.LifecycleRecord
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			t.Fatal(err)
		}
		if event, ok := record.Event.(gimbal.ValueSet); ok && event.Key == "input" {
			wroteInput = true
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if !wroteInput {
		t.Fatal("hosted entry did not record the submitted input")
	}
}
