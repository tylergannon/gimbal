package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tylergannon/gimble"
	"github.com/tylergannon/gimble/web"
)

func TestRunsCommandListsRunsFromRunningInstance(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	project := t.TempDir()
	runtime, err := web.NewRuntime(ctx, project, web.WithNoWeb())
	if err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{}, 2)
	var workers sync.WaitGroup
	for range 2 {
		workers.Go(func() {
			_ = runtime.Run(ctx, "listed", nil, func(ctx context.Context) error {
				started <- struct{}{}
				<-ctx.Done()
				return ctx.Err()
			})
		})
	}
	for range 2 {
		<-started
	}

	var output bytes.Buffer
	var errOutput bytes.Buffer
	if err := run([]string{"runs", "--work-dir", project}, &output, &errOutput, os.Getenv); err != nil {
		t.Fatal(err)
	}
	var got runsOutput
	if err := json.Unmarshal(output.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Runs) != 2 {
		t.Fatalf("runs = %+v; want 2", got.Runs)
	}
	for _, run := range got.Runs {
		if run.Status != "running" || run.PID == 0 || run.Name != "listed" {
			t.Errorf("run = %+v; want running listed run with pid", run)
		}
	}

	cancel()
	workers.Wait()
}

func TestRunsCommandSelectsAdmittedProjectThroughAlias(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	base := t.TempDir()
	var work sync.WaitGroup
	t.Cleanup(func() { cancel(); work.Wait() })
	a := filepath.Join(base, "a")
	b := filepath.Join(base, "b")
	for _, path := range []string{a, b} {
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	instance, err := web.NewInstance(ctx, filepath.Join(base, "instance"), nil, web.WithNoWeb())
	if err != nil {
		t.Fatal(err)
	}
	projectA, err := instance.AdmitProject(a)
	if err != nil {
		t.Fatal(err)
	}
	projectB, err := instance.AdmitProject(b)
	if err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{}, 2)
	for _, project := range []*web.Runtime{projectA, projectB} {
		work.Go(func() {
			_ = project.Run(ctx, "selected", nil, func(ctx context.Context) error {
				started <- struct{}{}
				<-ctx.Done()
				return ctx.Err()
			})
		})
	}
	<-started
	<-started
	alias := filepath.Join(base, "alias-b")
	if err := os.Symlink(b, alias); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := run([]string{"runs", "--work-dir", alias}, &output, io.Discard, os.Getenv); err != nil {
		t.Fatal(err)
	}
	var selected runsOutput
	if err := json.Unmarshal(output.Bytes(), &selected); err != nil {
		t.Fatal(err)
	}
	if len(selected.Runs) != 1 {
		t.Fatalf("B alias lists %+v", selected.Runs)
	}
	bID := selected.Runs[0].ID
	output.Reset()
	if err := run([]string{"runs", "--work-dir", filepath.Join(base, "b", "..", "a")}, &output, io.Discard, os.Getenv); err != nil {
		t.Fatal(err)
	}
	var other runsOutput
	if err := json.Unmarshal(output.Bytes(), &other); err != nil {
		t.Fatal(err)
	}
	if len(other.Runs) != 1 || other.Runs[0].ID == bID {
		t.Fatalf("A normalized path lists %+v", other.Runs)
	}
}

func TestRunsCommandReturnsEmptyJSONWithoutRuntime(t *testing.T) {
	var output bytes.Buffer
	if err := listRuns(t.Context(), &output, t.TempDir()); err != nil {
		t.Fatal(err)
	}
	var got runsOutput
	if err := json.Unmarshal(output.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Runs) != 0 {
		t.Fatalf("runs = %+v; want empty", got.Runs)
	}
}

func TestRunsCommandIgnoresStaleDiscovery(t *testing.T) {
	project := t.TempDir()
	controlDir := filepath.Join(project, ".gimble", "control")
	if err := os.MkdirAll(controlDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(controlDir, "stale.json"), []byte(`{"pid":1234,"socket":"/tmp/gimble-missing.sock"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := listRuns(t.Context(), &output, project); err != nil {
		t.Fatal(err)
	}
	var got runsOutput
	if err := json.Unmarshal(output.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Runs) != 0 {
		t.Fatalf("runs = %+v; want stale instance ignored", got.Runs)
	}
}

func TestWatchStreamPrintsSnapshotSessionsAndDeltas(t *testing.T) {
	input := strings.NewReader("event: snapshot\ndata: {\"stream\":\"s\",\"sessions\":{\"lap.1/coder.1\":{\"id\":\"lap.1/coder.1\"}}}\n\n" +
		"id: s:1\nevent: delta\ndata: {\"stream\":\"s\",\"position\":1,\"frames\":[]}\n\n")
	var output bytes.Buffer
	if err := streamObservation(&output, input); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("watch output = %q; want snapshot and delta", output.String())
	}
	var snapshot struct {
		Type     string                       `json:"type"`
		Sessions map[string]map[string]string `json:"sessions"`
	}
	if err := json.Unmarshal([]byte(lines[0]), &snapshot); err != nil {
		t.Fatal(err)
	}
	if snapshot.Type != "snapshot" || snapshot.Sessions["lap.1/coder.1"]["id"] != "lap.1/coder.1" {
		t.Fatalf("snapshot = %+v; want session ID", snapshot)
	}
	var delta struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal([]byte(lines[1]), &delta); err != nil {
		t.Fatal(err)
	}
	if delta.Type != "delta" {
		t.Fatalf("delta = %+v; want delta type", delta)
	}
}

func TestWatchRunReadsSnapshotAndSSEFromRuntime(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	project := t.TempDir()
	runtime, err := web.NewRuntime(ctx, project, web.WithNoWeb())
	if err != nil {
		t.Fatal(err)
	}
	adapter := &watchBlocking{started: make(chan struct{})}
	runDone := make(chan error, 1)
	go func() {
		runDone <- runtime.Run(ctx, "watched", map[gimble.WorkflowRole]gimble.ModelBinding{
			"coder": {Adapter: adapter, Model: "m"},
		}, func(ctx context.Context) error {
			return gimble.Scope(ctx, "lap", func(ctx context.Context) error {
				session := gimble.NewSession(ctx, "coder", "/w")
				_, err := session.Generate[gimble.Text](ctx, "wait")
				return err
			})
		})
	}()
	<-adapter.started
	runID := oneCLIRunID(t, filepath.Join(project, ".gimble"))

	reader, writer := io.Pipe()
	watchDone := make(chan error, 1)
	go func() {
		err := watchRun(ctx, writer, project, runID)
		_ = writer.CloseWithError(err)
		watchDone <- err
	}()
	lines := make(chan string, 1)
	go func() {
		scanner := bufio.NewScanner(reader)
		if scanner.Scan() {
			lines <- scanner.Text()
			return
		}
		lines <- ""
	}()
	var first string
	select {
	case first = <-lines:
	case <-time.After(5 * time.Second):
		t.Fatal("watch did not emit its snapshot")
	}
	var snapshot struct {
		Sessions map[string]json.RawMessage `json:"sessions"`
	}
	if err := json.Unmarshal([]byte(first), &snapshot); err != nil {
		t.Fatal(err)
	}
	if _, ok := snapshot.Sessions["lap.1/coder.1"]; !ok {
		t.Fatalf("snapshot sessions = %+v; want lap.1/coder.1", snapshot.Sessions)
	}

	cancel()
	if err := <-runDone; err != nil && !errors.Is(err, context.Canceled) {
		t.Fatalf("run = %v", err)
	}
	select {
	case err := <-watchDone:
		if err != nil && !errors.Is(err, context.Canceled) {
			t.Fatalf("watch = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("watch did not terminate after runtime ended")
	}
}

type watchBlocking struct {
	started chan struct{}
	once    sync.Once
}

func (b *watchBlocking) CreateSession(context.Context, string, string, string) (string, error) {
	return "native", nil
}

func (b *watchBlocking) RunTurn(ctx context.Context, _, _ string, _ json.RawMessage, _ func(gimble.AgentEvent) error) (gimble.TurnResult, error) {
	b.once.Do(func() { close(b.started) })
	<-ctx.Done()
	return gimble.TurnResult{}, ctx.Err()
}

func (*watchBlocking) Steer(context.Context, string, string) (bool, error) { return false, nil }
func (*watchBlocking) Fork(context.Context, string) (string, error)        { return "fork", nil }
func (*watchBlocking) Close(context.Context, string) error                 { return nil }

func oneCLIRunID(t *testing.T, project string) string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(project, "runs"))
	if err != nil || len(entries) != 1 {
		t.Fatalf("run directories = %v, %v", entries, err)
	}
	return entries[0].Name()
}
