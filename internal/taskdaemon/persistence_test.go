package taskdaemon

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/backend"
	"github.com/tylergannon/gimbal/backend/messagefiles"
	"github.com/tylergannon/gimbal/internal/effects"
	"github.com/tylergannon/gimbal/internal/executor"
)

func TestRestartRetainsCompletedTaskAndArtifacts(t *testing.T) {
	root := t.TempDir()
	d := New(root)
	owner := effects.Owner{Run: "run", Task: "read"}
	if err := d.process(effects.Process{ExecutionIdentity: backend.NewExecutionIdentity("process"), Owner: owner, Command: "sh", Args: []string{"-c", "printf complete"}, Workdir: root}); err != nil {
		t.Fatal(err)
	}
	task := d.tasks[key(owner.Run, owner.Task)]
	<-task.done
	if err := d.writeBytes(backend.Artifact{Owner: owner, Name: "input.txt"}, strings.NewReader(`saved`)); err != nil {
		t.Fatal(err)
	}
	restored := New(root)
	restored.ensureRecovery()
	if restored.restoreErr != nil {
		t.Fatal(restored.restoreErr)
	}
	result := restored.result(backend.Target{Run: owner.Run, ID: owner.Task})
	if !result.Done || result.Error != "" {
		t.Fatalf("result %+v", result)
	}
	capture := restored.tasks[key(owner.Run, owner.Task)].captures["stdout"]
	raw, err := os.ReadFile(capture)
	if err != nil || string(raw) != "complete" {
		t.Fatalf("capture %s %v", raw, err)
	}
	path := restored.files[backend.Artifact{Owner: owner, Name: "input.txt"}]
	raw, err = os.ReadFile(path)
	if err != nil || string(raw) != "saved" {
		t.Fatalf("input %s %v", raw, err)
	}
}

func TestRestartReattachesRunningCommandWithoutExecutingAgain(t *testing.T) {
	root := t.TempDir()
	d := New(root)
	owner := effects.Owner{Run: "run", Task: "once"}
	marker := filepath.Join(root, "count")
	req := effects.Process{ExecutionIdentity: backend.NewExecutionIdentity("process"), Owner: owner, Command: "sh", Args: []string{"-c", "printf x >> '" + marker + "'; sleep 0.2; printf done"}, Workdir: root}
	if err := d.process(req); err != nil {
		t.Fatal(err)
	}
	restored := New(root)
	restored.ensureRecovery()
	if restored.restoreErr != nil {
		t.Fatal(restored.restoreErr)
	}
	select {
	case <-restored.tasks[key(owner.Run, owner.Task)].done:
	case <-time.After(5 * time.Second):
		t.Fatal("reattach timeout")
	}
	if err := restored.process(req); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(marker)
	if err != nil || string(raw) != "x" {
		t.Fatalf("executed more than once %q %v", raw, err)
	}
	req.Args = []string{"-c", "exit 0"}
	if err := restored.process(req); err == nil {
		t.Fatal("conflicting identity accepted")
	}
}

func TestRestartMissingAcceptedTurnEvidenceIsUnresolved(t *testing.T) {
	root := t.TempDir()
	d := New(root)
	owner := effects.Owner{Run: "run", Task: "agent"}
	native := string(executor.NewSessionID())
	session := effects.Session{ExecutionIdentity: backend.NewExecutionIdentity("session"), Owner: owner, Harness: "codex", Model: "luna", Workdir: root}
	if err := d.writeState("sessions", key(owner.Run, owner.Task), savedSession{Harness: "codex", Request: session, Native: native}); err != nil {
		t.Fatal(err)
	}
	turn := effects.Turn{ExecutionIdentity: backend.NewExecutionIdentity("turn"), Owner: effects.Owner{Run: "run", Task: "turn"}, Session: "agent", Prompt: "must not dispatch"}
	input, _ := json.Marshal(turn)
	if err := d.writeState("tasks", key("run", "turn"), savedTask{Owner: turn.Owner, Turn: &turn, Input: input}); err != nil {
		t.Fatal(err)
	}
	restored := New(root)
	restored.ensureRecovery()
	if restored.restoreErr != nil {
		t.Fatal(restored.restoreErr)
	}
	select {
	case <-restored.tasks[key("run", "turn")].done:
	case <-time.After(time.Second):
		t.Fatal("missing evidence did not settle")
	}
	result := restored.result(backend.Target{Run: "run", ID: "turn"})
	if !result.Done || !result.Unresolved {
		t.Fatalf("false terminal outcome %+v", result)
	}
}

type steeringHarness struct {
	testHarness
	calls  int
	landed bool
}

func (h *steeringHarness) Steer(context.Context, string, string) (bool, error) {
	h.calls++
	return h.landed, nil
}

func TestSteeringDispositionSurvivesRestart(t *testing.T) {
	for _, landed := range []bool{false, true} {
		t.Run(map[bool]string{false: "missed", true: "landed"}[landed], func(t *testing.T) {
			root := t.TempDir()
			d := New(root)
			native := &steeringHarness{landed: landed}
			ready := make(chan struct{})
			close(ready)
			d.sessions[key("run", "agent")] = &session{native: "native", adapter: native, ready: ready}
			req := effects.Target{Run: "run", ID: "agent", Message: "next", InputID: "arbitrary-input-id"}
			got, err := d.steer(t.Context(), req)
			if err != nil || got != landed {
				t.Fatalf("steer %v %v", got, err)
			}
			restored := New(root)
			restored.ensureRecovery()
			got, err = restored.steer(t.Context(), req)
			if err != nil || got != landed || native.calls != 1 {
				t.Fatalf("duplicate steering %v %v calls=%d", got, err, native.calls)
			}
			req.Message = "different"
			if _, err = restored.steer(t.Context(), req); err == nil {
				t.Fatal("conflicting steer accepted")
			}
		})
	}
}
func TestAcceptedOnlySteerNeverResends(t *testing.T) {
	root := t.TempDir()
	d := New(root)
	req := effects.Target{Run: "run", ID: "agent", InputID: "pending", Message: "next"}
	if err := d.writeState("controls", req.InputID, disposition{Request: backend.MessageRequest{ID: req.InputID, Kind: "steer", Target: &req}}); err != nil {
		t.Fatal(err)
	}
	restored := New(root)
	restored.ensureRecovery()
	_, err := restored.steer(t.Context(), req)
	var uncertain *backend.DeliveryUncertain
	if !errors.As(err, &uncertain) {
		t.Fatalf("expected uncertainty: %v", err)
	}
}

func TestMessagingCompletedOutcomeRepublishesAfterRestart(t *testing.T) {
	root := t.TempDir()
	d := New(root)
	eventsDir := t.TempDir()
	store, err := messagefiles.Open(eventsDir)
	if err != nil {
		t.Fatal(err)
	}
	bus := &directMessages{Store: store, ctx: t.Context()}
	req := effects.Request("stable-input", "steer", effects.Owner{Run: "run", Task: "turn"})
	req.SessionID = "agent"
	req.Target = &effects.Target{Run: "run", ID: "agent", InputID: "stable-input", Message: "hello"}
	ready := make(chan struct{})
	close(ready)
	native := &steeringHarness{landed: true}
	d.sessions[key("run", "agent")] = &session{native: "native", adapter: native, ready: ready}
	if err = d.Messages(bus)(t.Context(), req); err != nil {
		t.Fatal(err)
	}
	restored := New(root)
	restored.ensureRecovery()
	req.Created = time.Now()
	if err = restored.Messages(bus)(t.Context(), req); err != nil {
		t.Fatal(err)
	}
	saved := restored.messages[req.ID]
	if saved.Result == nil || !saved.Result.Value || native.calls != 1 {
		t.Fatalf("lost or repeated outcome %+v", saved)
	}
	entries, err := os.ReadDir(eventsDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected one acceptance and one completion, got %d records", len(entries))
	}
}

var _ gimbal.HarnessAdapter = (*steeringHarness)(nil)

func TestMessagingSteerUsesStableInputID(t *testing.T) {
	d, bus, ctx := messagingOwner(t)
	ready := make(chan struct{})
	close(ready)
	native := &steeringHarness{landed: true}
	d.sessions[key("run", "agent")] = &session{native: "native", adapter: native, ready: ready}
	h := &Harness{Connection: backend.Connection{Messages: bus}}
	ctx = effects.WithInputID(effects.WithExecutionIdentity(effects.WithOwner(ctx, effects.Owner{Run: "run", Task: "turn"}), backend.NewExecutionIdentity("turn")), "non-ulid-input")
	for range 2 {
		landed, err := h.Steer(ctx, key("run", "agent"), "next")
		if err != nil || !landed {
			t.Fatalf("steer %v %v", landed, err)
		}
	}
	if native.calls != 1 || d.messages["non-ulid-input"] == nil {
		t.Fatal("stable input identity did not deduplicate steering")
	}
}
