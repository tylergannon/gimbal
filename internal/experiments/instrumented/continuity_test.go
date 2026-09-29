package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tylergannon/gimbal/internal/compiledscope"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/converter"
	"go.temporal.io/sdk/testsuite"
)

func TestPayloadStoreRoundTripAndCorruption(t *testing.T) {
	root := t.TempDir()
	base := dataConverter(root)
	dc := converter.WithDataConverterSerializationContext(base, converter.ActivitySerializationContext{WorkflowID: "large-result"})
	want := Report{Summary: strings.Repeat("complete-result-", 300000), Receipt: "tail"}
	payload, err := dc.ToPayload(want)
	if err != nil {
		t.Fatal(err)
	}
	if len(payload.Data) > 250 {
		t.Fatal("large payload escaped")
	}
	var got Report
	if err = base.FromPayload(payload, &got); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatal("typed result changed")
	}
	files, err := filepath.Glob(filepath.Join(root, environmentID("large-result"), "context", "blobs", "*"))
	if err != nil || len(files) != 1 {
		t.Fatal(files, err)
	}
	if err = os.Chmod(files[0], 0600); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(files[0], []byte("corrupt"), 0600); err != nil {
		t.Fatal(err)
	}
	if err = base.FromPayload(payload, &got); err == nil {
		t.Fatal("corrupt result accepted")
	}
}
func TestNonzeroCommandIsData(t *testing.T) {
	a := testActivities(t, "continuity")
	var suite testsuite.WorkflowTestSuite
	e := suite.NewTestActivityEnvironment()
	e.RegisterActivity(a)
	result, err := e.ExecuteActivity(a.ContinuityRunCommand1, operationInput{}, "diagnostic", a.workdir, "sh", []string{"-c", "printf observed; printf diagnostic >&2; exit 7"})
	if err != nil {
		t.Fatal(err)
	}
	var out commandResult
	if err = result.Get(&out); err != nil {
		t.Fatal(err)
	}
	if out.ExitCode != 7 || out.Stdout != "observed" || out.Stderr != "diagnostic" || out.Err() != nil {
		t.Fatalf("%+v", out)
	}
	result, err = e.ExecuteActivity(a.MissingCommand, operationInput{})
	if err != nil {
		t.Fatal(err)
	}
	if err = result.Get(&out); err != nil {
		t.Fatal(err)
	}
	if out.Err() == nil || out.Failure.Kind != "execution" {
		t.Fatalf("missing command: %+v", out)
	}
}
func TestLiveContinuity(t *testing.T) {
	if os.Getenv("SPECIMEN_LIVE_WORKFLOW") != "1" {
		t.Skip("requires Temporal, prepared workers, and paid Haiku access")
	}
	c, err := client.Dial(client.Options{DataConverter: dataConverter(stateRoot())})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	id := fmt.Sprintf("continuity-%d", time.Now().UnixMilli())
	input, err := prepareInput(stateRoot(), id, "Continue a conversation across activities.")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Minute)
	defer cancel()
	run, err := c.ExecuteWorkflow(ctx, client.StartWorkflowOptions{ID: id, TaskQueue: controlQueue}, ContinuityWorkflow, input)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("workflow %s", id)
	defer func() {
		cleanup, stop := context.WithTimeout(context.Background(), 10*time.Second)
		defer stop()
		_ = c.CancelWorkflow(cleanup, id, run.GetRunID())
	}()
	if err = run.Get(ctx, nil); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(stateRoot(), environmentID(id), "workspace")
	assertContinuityRecords(t, filepath.Join(dir, ".gimbal"))
	b, err := os.ReadFile(filepath.Join(dir, "continuity.txt"))
	if err != nil || strings.TrimSpace(string(b)) != "amber-17" {
		t.Fatalf("edit=%q %v", b, err)
	}
	b, err = os.ReadFile(filepath.Join(dir, "recovery.txt"))
	if err != nil || string(b) != "recovered" {
		t.Fatalf("recovery=%q %v", b, err)
	}
	t.Logf("authored continuation validation and workspace effects completed; workspace %s", dir)
}

func TestSetCapturesExactLargeInteger(t *testing.T) {
	a := testActivities(t, "exact-value")
	var suite testsuite.WorkflowTestSuite
	e := suite.NewTestActivityEnvironment()
	e.RegisterActivity(a)
	value := struct{ Number int64 }{9007199254740993}
	encoded, err := e.ExecuteActivity(a.SetValue, "", compiledscope.Snapshot(""), contextEntry("precise", value))
	if err != nil {
		t.Fatal(err)
	}
	var snapshot compiledscope.Snapshot
	if err = encoded.Get(&snapshot); err != nil {
		t.Fatal(err)
	}
	entries, err := a.store.Load(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := a.store.Value(entries[0])
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "{\"Number\":9007199254740993}" {
		t.Fatalf("numeric value corrupted: %s", raw)
	}
}
