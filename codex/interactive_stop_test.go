package codex

import (
	"context"
	"encoding/json"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/internal/live"
	"github.com/tylergannon/gimbal/internal/observation"
	"github.com/tylergannon/gimbal/runstore"
)

// A real gimbal.Run and Interactive drive the real Codex adapter against the
// controlled JSON-RPC daemon, with every input submitted through the run's
// live.Controller and every state read from the run's own observation journal.
// The daemon ignores interrupts, so native cessation after Stop stays unknown.
func TestInteractiveStopThenContinuationOnOneNativeThread(t *testing.T) {
	shortenControlTimeout(t)
	ad, d := newRecoveryDaemon(t, "default")
	d.ignoreInterrupt = true
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	defer cancel()
	state := filepath.Join(t.TempDir(), ".gimbal")
	id := string(runstore.NewRunID())

	controllers := make(chan live.Controller, 1)
	runCtx := live.WithHook(gimbal.Project(ctx, state), func(_ string, c live.Controller) func() {
		controllers <- c
		return func() {}
	})
	var transcript gimbal.InteractiveTranscript
	done := make(chan error, 1)
	go func() {
		done <- gimbal.Run(runCtx, id, "interactive-stop", map[gimbal.WorkflowRole]gimbal.ModelBinding{gimbal.RoleAgenticDialogues: {Adapter: ad, Model: "chat"}}, func(ctx context.Context) error {
			session := gimbal.NewSession(ctx, gimbal.RoleAgenticDialogues, t.TempDir())
			var err error
			transcript, err = gimbal.Interactive(ctx, "dialogue", session, "Discuss.")
			return err
		})
	}()
	var controller live.Controller
	select {
	case controller = <-controllers:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	registry := observation.NewRegistry(state)
	row := func() observation.InteractiveRow {
		snapshot, err := registry.Snapshot(id)
		if err != nil {
			return observation.InteractiveRow{}
		}
		for _, r := range snapshot.Interactions {
			if r.Address == "dialogue" {
				return r
			}
		}
		return observation.InteractiveRow{}
	}
	// An interaction row is waiting for the human when it is pending, and
	// working while a turn (or a prompt waiting to start) is outstanding.
	waiting := func() bool {
		r := row()
		return r.Status == observation.InteractiveStatusPending && r.Address == "dialogue"
	}
	working := func() bool {
		r := row()
		return r.Address == "dialogue" && r.Status != observation.InteractiveStatusPending && r.Unresolved == "" && r.EndedBy == ""
	}
	messages := func() []string {
		var out []string
		for _, m := range row().Messages {
			out = append(out, m.Role+": "+m.Message)
		}
		return out
	}
	submit := func(request map[string]any) {
		t.Helper()
		raw, _ := json.Marshal(request)
		eventually(t, func() bool { return controller.SubmitInteractive("dialogue", string(raw)) == nil })
	}
	stillRunning := func() {
		t.Helper()
		select {
		case err := <-done:
			t.Fatalf("the run ended (%v); the conversation must stay usable", err)
		default:
		}
	}
	starts := func() int { return d.count(func() int { return d.starts }) }
	interrupts := func() int { return d.count(func() int { return d.interrupts }) }
	const thread = "thread-1"

	// An acknowledged turn with no output events: the daemon answers turn/start
	// and then says nothing.
	eventually(t, waiting)
	submit(map[string]any{"id": "m1", "message": "first"})
	eventually(t, func() bool { return starts() == 1 && acknowledged(ad, thread) })
	eventually(t, working)
	if got := messages(); len(got) != 0 {
		t.Fatalf("transcript before the turn produced anything: %q", got)
	}

	// Stop: the daemon ignores the interrupt, so cessation is unknown, but the
	// prompt was acknowledged, so it is delivered and the conversation is open.
	base := interrupts()
	submit(map[string]any{"id": "s1", "stop": true})
	eventually(t, func() bool { return interrupts() > base })
	eventually(t, waiting)
	stillRunning()
	if r := row(); r.Unresolved != "" || r.EndedBy != "" || !slices.Equal(messages(), []string{"user: first"}) {
		t.Fatalf("after Stop: unresolved=%q ended=%q messages=%q", r.Unresolved, r.EndedBy, messages())
	}

	// The next prompt stays pending while the old native turn runs: nothing is
	// started, the run is alive, and it is not yet in the transcript.
	base = interrupts()
	submit(map[string]any{"id": "m2", "message": "second"})
	eventually(t, func() bool { return interrupts() > base })
	eventually(t, working)
	time.Sleep(50 * time.Millisecond)
	stillRunning()
	if starts() != 1 || !slices.Equal(messages(), []string{"user: first"}) {
		t.Fatalf("pending prompt: starts=%d messages=%q", starts(), messages())
	}

	// Stop while that prompt waits abandons it: Codex never saw it, so it is not
	// published as delivered, and still nothing started.
	submit(map[string]any{"id": "s2", "stop": true})
	eventually(t, waiting)
	stillRunning()
	if starts() != 1 || !slices.Equal(messages(), []string{"user: first"}) {
		t.Fatalf("after Stop of a pending prompt: starts=%d messages=%q", starts(), messages())
	}

	// A fresh prompt waits again. The old turn's late terminal output arrives on
	// the connection; then the old turn settles natively and exactly one next
	// turn starts on the same thread.
	submit(map[string]any{"id": "m3", "message": "third"})
	eventually(t, working)
	eventually(t, func() bool { return interrupts() > base })
	d.mu.Lock()
	ws := d.sockets[len(d.sockets)-1]
	d.mu.Unlock()
	late := json.RawMessage(mustEncode(map[string]any{"id": "late", "type": "agentMessage", "phase": "final_answer", "text": mustEncode(map[string]any{"decision": map[string]string{"type": "chat", "message": "late-old"}})}))
	_ = fakeSend(ws, map[string]any{"method": "item/completed", "params": map[string]any{"threadId": thread, "turnId": "turn-1", "item": late}})
	_ = fakeSend(ws, map[string]any{"method": "turn/completed", "params": map[string]any{"threadId": thread, "turn": map[string]string{"id": "turn-1", "status": "completed"}}})
	time.Sleep(50 * time.Millisecond)
	stillRunning()
	if starts() != 1 {
		t.Fatalf("started while the old turn still runs: %d", starts())
	}
	d.mu.Lock()
	d.threads[thread].turns[0].Status = "interrupted"
	d.threads[thread].status = "idle"
	d.completeStalled = true
	d.mu.Unlock()
	eventually(t, waiting)
	want := []string{"user: first", "user: third", "assistant: reply from turn-2"}
	if got := messages(); !slices.Equal(got, want) {
		t.Fatalf("transcript %q, want %q", got, want)
	}
	d.mu.Lock()
	threads, turns := len(d.threads), len(d.threads[thread].turns)
	d.mu.Unlock()
	if starts() != 2 || threads != 1 || turns != 2 {
		t.Fatalf("starts=%d threads=%d turns on the thread=%d", starts(), threads, turns)
	}

	// Finish while a prompt waits behind an unconfirmed Stop ends the
	// conversation on purpose; the waiting prompt is never delivered.
	d.mu.Lock()
	d.completeStalled = false
	d.mu.Unlock()
	submit(map[string]any{"id": "m4", "message": "fourth"})
	eventually(t, func() bool { return starts() == 3 && acknowledged(ad, thread) })
	submit(map[string]any{"id": "s3", "stop": true})
	eventually(t, waiting)
	base = interrupts()
	submit(map[string]any{"id": "m5", "message": "fifth"})
	eventually(t, func() bool { return interrupts() > base })
	eventually(t, working)
	stillRunning()
	submit(map[string]any{"id": "f1", "finish": true})
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("run: %v", err)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	want = append(want, "user: fourth")
	if r := row(); r.EndedBy != "human" || r.Unresolved != "" || !slices.Equal(messages(), want) || transcript.EndedBy != "human" {
		t.Fatalf("after Finish: ended=%q unresolved=%q messages=%q transcript=%q", r.EndedBy, r.Unresolved, messages(), transcript.EndedBy)
	}
	if starts() != 3 {
		t.Fatalf("the waiting prompt was submitted by Finish: starts=%d", starts())
	}
}
