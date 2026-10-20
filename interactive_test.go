package gimbal

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/tylergannon/gimbal/internal/live"
	"github.com/tylergannon/gimbal/runstore"
	"os"
	"path/filepath"
	"testing"
)

func TestInteractiveHumanFirstDurableFinish(t *testing.T) {
	ctx := Project(context.Background(), t.TempDir())
	id := string(runstore.NewRunID())
	var controller *run
	ctx = live.WithHook(ctx, func(_ string, c live.Controller) func() { controller = c.(*run); return func() {} })
	returned := false
	body := func(ctx context.Context) error {
		session := NewSession(ctx, RoleAgenticDialogues, "")
		transcript, err := Interactive(ctx, "dialogue", session, "Discuss the task.")
		if err != nil {
			return err
		}
		returned = transcript.EndedBy == "human"
		return nil
	}
	models := map[WorkflowRole]ModelBinding{RoleAgenticDialogues: {}}
	if err := Run(ctx, id, "test", models, body); err != nil {
		t.Fatal(err)
	}
	if returned {
		t.Fatal("waiting returned through authored continuation")
	}
	if controller == nil {
		t.Fatal("controller missing")
	}
	if err := controller.acceptInteractive("dialogue", "finish-1", "", true); err != nil {
		t.Fatal(err)
	}
	if err := controller.acceptInteractive("dialogue", "finish-1", "", true); err != nil {
		t.Fatal(err)
	}
	if err := controller.acceptInteractive("dialogue", "finish-1", "conflict", true); err == nil {
		t.Fatal("conflicting retry accepted")
	}
	if err := Run(ctx, id, "test", models, body); err != nil {
		t.Fatal(err)
	}
	if !returned {
		t.Fatal("finish did not run authored continuation")
	}
}

type interactionAdapter struct {
	replies []string
	calls   int
	native  string
}

func (a *interactionAdapter) CreateSession(context.Context, string, string, string) (string, error) {
	return "native-continuity", nil
}
func (a *interactionAdapter) RunTurn(_ context.Context, native, _ string, _ json.RawMessage, _ func(AgentEvent) error) (TurnResult, error) {
	a.native = native
	reply := a.replies[a.calls]
	a.calls++
	return TurnResult{Output: json.RawMessage(reply)}, nil
}
func (*interactionAdapter) Steer(context.Context, string, string) (bool, error) { return false, nil }
func (*interactionAdapter) Fork(context.Context, string) (string, error)        { return "fork", nil }
func (*interactionAdapter) Close(context.Context, string) error                 { return nil }

func TestInteractiveChatReconstructsWithoutRedispatch(t *testing.T) {
	directory := t.TempDir()
	ctx := Project(context.Background(), directory)
	id := string(runstore.NewRunID())
	adapter := &interactionAdapter{replies: []string{`{"type":"chat","message":"Hello"}`, `{"type":"end"}`}}
	models := map[WorkflowRole]ModelBinding{RoleAgenticDialogues: {Adapter: adapter}}
	var controller *run
	ctx = live.WithHook(ctx, func(_ string, c live.Controller) func() { controller = c.(*run); return func() {} })
	var result InteractiveTranscript
	body := func(ctx context.Context) error {
		session := NewSession(ctx, RoleAgenticDialogues, "")
		var err error
		result, err = Interactive(ctx, "dialogue", session, "Converse.")
		return err
	}
	if err := Run(ctx, id, "test", models, body); err != nil {
		t.Fatal(err)
	}
	if err := controller.acceptInteractive("dialogue", "one", "Hi", false); err != nil {
		t.Fatal(err)
	}
	if err := Run(ctx, id, "test", models, body); err != nil {
		t.Fatal(err)
	}
	if adapter.calls != 1 {
		t.Fatalf("turn calls %d", adapter.calls)
	}
	controller.interactionMu.Lock()
	controller.refreshInteractions()
	for _, state := range controller.interactionStates {
		if !state.Waiting || len(state.Transcript.Messages) != 2 {
			t.Fatalf("chat/wait transition: %+v", state)
		}
	}
	controller.interactionMu.Unlock()
	// Simulate process loss by discarding activation-local handles and opening
	// the same committed directory, without native adapter lifecycle assertions.
	dormantRuns.Delete(controller.dir)
	controller.closeObservers()
	if err := controller.records.Close(); err != nil {
		t.Fatal(err)
	}
	if err := Run(ctx, id, "test", models, body); err != nil {
		t.Fatal(err)
	}
	if adapter.calls != 1 {
		t.Fatal("reconstruction dispatched a completed turn")
	}
	if err := controller.acceptInteractive("dialogue", "two", "Finish", false); err != nil {
		t.Fatal(err)
	}
	if err := Run(ctx, id, "test", models, body); err != nil {
		t.Fatal(err)
	}
	if adapter.calls != 2 || adapter.native != "native-continuity" {
		t.Fatalf("native continuity: %+v", adapter)
	}
	if result.EndedBy != "agent" || len(result.Messages) != 3 {
		t.Fatalf("transcript %+v", result)
	}
}

func TestInteractiveSuspensionLeavesSiblingProgressAndReplaysCompleteOutput(t *testing.T) {
	directory := t.TempDir()
	ctx := Project(context.Background(), directory)
	id := string(runstore.NewRunID())
	var controller *run
	ctx = live.WithHook(ctx, func(_ string, c live.Controller) func() { controller = c.(*run); return func() {} })
	models := map[WorkflowRole]ModelBinding{RoleAgenticDialogues: {}}
	joined := false
	body := func(ctx context.Context) error {
		group := Group(ctx, "parallel")
		group.Go("human", func(ctx context.Context) error {
			_, err := Interactive(ctx, "dialogue", NewSession(ctx, RoleAgenticDialogues, ""), "Wait.")
			return err
		})
		group.Go("useful", func(ctx context.Context) error {
			code, out, _, err := RunCommand(ctx, "output", directory, "sh", "-c", "printf x >> executions; head -c 80000 /dev/zero | tr '\\0' z")
			if err == nil && (code != 0 || len(out) != 80000) {
				return fmt.Errorf("complete output: exit %d bytes %d", code, len(out))
			}
			return err
		})
		if err := group.Wait(); err != nil {
			return err
		}
		joined = true
		return nil
	}
	if err := Run(ctx, id, "test", models, body); err != nil {
		t.Fatal(err)
	}
	if joined {
		t.Fatal("pending group joined")
	}
	if raw, err := os.ReadFile(filepath.Join(directory, "executions")); err != nil || string(raw) != "x" {
		t.Fatalf("runnable sibling: %q %v", raw, err)
	}
	if err := controller.acceptInteractive("parallel.1/human.1/dialogue", "finish", "", true); err != nil {
		t.Fatal(err)
	}
	if err := Run(ctx, id, "test", models, body); err != nil {
		t.Fatal(err)
	}
	if !joined {
		t.Fatal("resumed branches did not join")
	}
	if raw, _ := os.ReadFile(filepath.Join(directory, "executions")); string(raw) != "x" {
		t.Fatal("completed command dispatched twice")
	}
}

type activeInteractionAdapter struct {
	started  chan struct{}
	release  chan struct{}
	landed   bool
	steerErr error
	calls    int
	steers   int
}

func (*activeInteractionAdapter) CreateSession(context.Context, string, string, string) (string, error) {
	return "active-native", nil
}
func (a *activeInteractionAdapter) RunTurn(ctx context.Context, _ string, _ string, _ json.RawMessage, _ func(AgentEvent) error) (TurnResult, error) {
	a.calls++
	if a.calls == 1 {
		close(a.started)
		select {
		case <-ctx.Done():
			return TurnResult{}, ctx.Err()
		case <-a.release:
		}
	}
	return TurnResult{Output: json.RawMessage(`{"type":"chat","message":"Acknowledged"}`)}, nil
}
func (a *activeInteractionAdapter) Steer(context.Context, string, string) (bool, error) {
	a.steers++
	return a.landed, a.steerErr
}
func (*activeInteractionAdapter) Fork(context.Context, string) (string, error) { return "fork", nil }
func (*activeInteractionAdapter) Close(context.Context, string) error          { return nil }

func TestInteractiveActiveFinishReturnsNormally(t *testing.T) {
	ctx := Project(context.Background(), t.TempDir())
	id := string(runstore.NewRunID())
	adapter := &activeInteractionAdapter{started: make(chan struct{}), release: make(chan struct{})}
	models := map[WorkflowRole]ModelBinding{RoleAgenticDialogues: {Adapter: adapter}}
	var controller *run
	ctx = live.WithHook(ctx, func(_ string, c live.Controller) func() { controller = c.(*run); return func() {} })
	continued := false
	body := func(ctx context.Context) error {
		result, err := Interactive(ctx, "dialogue", NewSession(ctx, RoleAgenticDialogues, ""), "Converse.")
		if err == nil {
			continued = result.EndedBy == "human"
		}
		return err
	}
	if err := Run(ctx, id, "test", models, body); err != nil {
		t.Fatal(err)
	}
	if err := controller.acceptInteractive("dialogue", "opening", "Hello", false); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- Run(ctx, id, "test", models, body) }()
	<-adapter.started
	if err := controller.acceptInteractive("dialogue", "archive", "", true); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if !continued {
		t.Fatal("archive did not take normal workflow continuation")
	}
}
func TestInteractiveStopWaitsAndNextInputContinues(t *testing.T) {
	ctx := Project(context.Background(), t.TempDir())
	id := string(runstore.NewRunID())
	adapter := &activeInteractionAdapter{started: make(chan struct{}), release: make(chan struct{})}
	models := map[WorkflowRole]ModelBinding{RoleAgenticDialogues: {Adapter: adapter}}
	var controller *run
	ctx = live.WithHook(ctx, func(_ string, c live.Controller) func() { controller = c.(*run); return func() {} })
	body := func(ctx context.Context) error {
		_, err := Interactive(ctx, "dialogue", NewSession(ctx, RoleAgenticDialogues, ""), "Converse.")
		return err
	}
	if err := Run(ctx, id, "test", models, body); err != nil {
		t.Fatal(err)
	}
	_ = controller.acceptInteractive("dialogue", "opening", "Hello", false)
	done := make(chan error, 1)
	go func() { done <- Run(ctx, id, "test", models, body) }()
	<-adapter.started
	raw := `{"id":"stop","stop":true}`
	if err := controller.SubmitInteractive("dialogue", raw); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if controller.State() != "waiting" {
		t.Fatal("stop ended workflow")
	}
	if err := controller.acceptInteractive("dialogue", "next", "Continue", false); err != nil {
		t.Fatal(err)
	}
	if err := Run(ctx, id, "test", models, body); err != nil {
		t.Fatal(err)
	}
	if adapter.calls != 2 {
		t.Fatalf("stop replayed or lost turn: %d", adapter.calls)
	}
	_ = controller.acceptInteractive("dialogue", "finish", "", true)
	if err := Run(ctx, id, "test", models, body); err != nil {
		t.Fatal(err)
	}
}
