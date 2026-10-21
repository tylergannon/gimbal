package gimbal

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tylergannon/gimbal/backend"
	"github.com/tylergannon/gimbal/internal/effects"
	"github.com/tylergannon/gimbal/internal/live"
	"github.com/tylergannon/gimbal/runstore"
	"github.com/tylergannon/gimbal/runstore/directory"
)

type interactionAdapter struct {
	mu                 sync.Mutex
	replies            []string
	calls              int
	prompts            []string
	native             string
	started            chan struct{}
	release            chan struct{}
	landed             bool
	steerErr           error
	successAfterCancel bool
	onCancel           func() (TurnResult, error)
	silent             bool
	closes             int
}

func (*interactionAdapter) CreateSession(context.Context, string, string, string) (string, error) {
	return "native-continuity", nil
}
func (a *interactionAdapter) RunTurn(ctx context.Context, native, prompt string, _ json.RawMessage, emit func(AgentEvent) error) (TurnResult, error) {
	a.mu.Lock()
	a.native = native
	a.prompts = append(a.prompts, prompt)
	ordinal := a.calls
	a.calls++
	a.mu.Unlock()
	if !a.silent || ordinal != 0 {
		if err := emit(nativeEvent("session.step.started", map[string]any{"sessionID": native, "assistantMessageID": fmt.Sprintf("reply-%d", ordinal)}, map[string]any{"provider": "test", "sessionID": native, "messageID": fmt.Sprintf("reply-%d", ordinal)})); err != nil {
			return TurnResult{}, err
		}
	}
	if ordinal == 0 && a.started != nil {
		close(a.started)
		select {
		case <-ctx.Done():
			if a.onCancel != nil {
				return a.onCancel()
			}
			if !a.successAfterCancel {
				return TurnResult{}, ctx.Err()
			}
		case <-a.release:
		}
	}
	reply := `{"decision":{"type":"chat","message":"Acknowledged"}}`
	if len(a.replies) > ordinal {
		reply = a.replies[ordinal]
	}
	return TurnResult{Output: json.RawMessage(reply)}, nil
}
func (a *interactionAdapter) Steer(context.Context, string, string) (bool, error) {
	return a.landed, a.steerErr
}
func (*interactionAdapter) Fork(context.Context, string) (string, error) { return "fork", nil }
func (a *interactionAdapter) Close(context.Context, string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.closes++
	return nil
}
func (a *interactionAdapter) count() int { a.mu.Lock(); defer a.mu.Unlock(); return a.calls }

func eventuallyInteractive(t *testing.T, ready func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !ready() {
		if time.Now().After(deadline) {
			t.Fatal("interaction did not reach expected state")
		}
		time.Sleep(time.Millisecond)
	}
}
func waitingInteractive(r *run, messages int) bool {
	r.interactionMu.Lock()
	defer r.interactionMu.Unlock()
	for _, state := range r.interactionStates {
		if state.Waiting && len(state.Transcript.Messages) == messages {
			return true
		}
	}
	return false
}
func startInteractiveTest(t *testing.T, adapter *interactionAdapter, body func(context.Context) error) (*run, <-chan error) {
	t.Helper()
	ctx, cancel := context.WithCancel(Project(t.Context(), t.TempDir()))
	t.Cleanup(cancel)
	registered := make(chan *run, 1)
	ctx = live.WithHook(ctx, func(_ string, c live.Controller) func() { registered <- c.(*run); return func() {} })
	done := make(chan error, 1)
	go func() {
		done <- Run(ctx, string(runstore.NewRunID()), "test", map[WorkflowRole]ModelBinding{RoleAgenticDialogues: {Adapter: adapter}}, body)
	}()
	r := <-registered
	eventuallyInteractive(t, func() bool { return waitingInteractive(r, 0) })
	return r, done
}
func assertPending(t *testing.T, done <-chan error) {
	t.Helper()
	select {
	case err := <-done:
		t.Fatalf("Run returned while waiting: %v", err)
	default:
	}
}

func TestInteractiveHumanFirstDurableFinish(t *testing.T) {
	adapter := &interactionAdapter{}
	var result InteractiveTranscript
	r, done := startInteractiveTest(t, adapter, func(ctx context.Context) error {
		var err error
		result, err = Interactive(ctx, "dialogue", NewSession(ctx, RoleAgenticDialogues, ""), "Discuss.")
		return err
	})
	assertPending(t, done)
	if err := r.acceptInteractive("dialogue", "finish-1", "", true); err != nil {
		t.Fatal(err)
	}
	if err := r.acceptInteractive("dialogue", "finish-1", "", true); err != nil {
		t.Fatal(err)
	}
	if err := r.acceptInteractive("dialogue", "finish-1", "conflict", true); !errors.Is(err, runstore.ErrConflict) {
		t.Fatalf("conflicting retry: %v", err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if result.EndedBy != "human" || adapter.count() != 0 {
		t.Fatalf("result=%+v calls=%d", result, adapter.count())
	}
	if err := r.acceptInteractive("dialogue", "late", "too late", false); err == nil {
		t.Fatal("closed ingress accepted message")
	}
}
func TestInteractiveWaitingSiblingAndOrdinaryJoin(t *testing.T) {
	directory := t.TempDir()
	joined := make(chan struct{})
	sibling := make(chan struct{})
	r, done := startInteractiveTest(t, &interactionAdapter{}, func(ctx context.Context) error {
		group := Group(ctx, "parallel")
		group.Go("human", func(ctx context.Context) error {
			_, err := Interactive(ctx, "dialogue", NewSession(ctx, RoleAgenticDialogues, ""), "Wait.")
			return err
		})
		group.Go("useful", func(ctx context.Context) error {
			code, out, _, err := RunCommand(ctx, "output", directory, "sh", "-c", "printf x >> executions; head -c 80000 /dev/zero | tr '\\0' z")
			if err == nil && (code != 0 || len(out) != 80000) {
				err = fmt.Errorf("exit %d output bytes %d", code, len(out))
			}
			close(sibling)
			return err
		})
		if err := group.Wait(); err != nil {
			return err
		}
		close(joined)
		return nil
	})
	<-sibling
	assertPending(t, done)
	select {
	case <-joined:
		t.Fatal("Group joined waiting child")
	default:
	}
	if err := r.acceptInteractive("parallel.1/human.1/dialogue", "finish", "", true); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	<-joined
	raw, err := os.ReadFile(filepath.Join(directory, "executions"))
	if err != nil || string(raw) != "x" {
		t.Fatalf("sibling effect %q %v", raw, err)
	}
}
func TestInteractiveChatUsesNativeHistoryAndOneRegistration(t *testing.T) {
	adapter := &interactionAdapter{replies: []string{`{"decision":{"type":"chat","message":"First reply"}}`, `{"decision":{"type":"end"}}`}}
	var result InteractiveTranscript
	r, done := startInteractiveTest(t, adapter, func(ctx context.Context) error {
		var err error
		result, err = Interactive(ctx, "dialogue", NewSession(ctx, RoleAgenticDialogues, ""), "Discuss.")
		return err
	})
	_ = r.acceptInteractive("dialogue", "one", "Remember the token.", false)
	eventuallyInteractive(t, func() bool { return waitingInteractive(r, 2) })
	assertPending(t, done)
	adapter.mu.Lock()
	closes := adapter.closes
	adapter.mu.Unlock()
	if closes != 0 {
		t.Fatal("waiting closed native session")
	}
	_ = r.acceptInteractive("dialogue", "two", "Recall it.", false)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	adapter.mu.Lock()
	defer adapter.mu.Unlock()
	if result.EndedBy != "agent" || len(result.Messages) != 3 || adapter.calls != 2 || adapter.native != "native-continuity" {
		t.Fatalf("result=%+v adapter=%+v", result, adapter)
	}
	if strings.Contains(adapter.prompts[1], "First reply") || strings.Contains(adapter.prompts[1], "Remember the token") {
		t.Fatal("prompt repeated native history")
	}
	if adapter.closes != 1 || r.State() != "terminal" {
		t.Fatalf("closes %d state %s", adapter.closes, r.State())
	}
}
func TestInteractiveActiveFinishAndStopRace(t *testing.T) {
	for _, control := range []string{"finish", "stop"} {
		for _, success := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/success=%v", control, success), func(t *testing.T) {
				adapter := &interactionAdapter{started: make(chan struct{}), release: make(chan struct{}), successAfterCancel: success, replies: []string{`{"decision":{"type":"end"}}`}}
				var result InteractiveTranscript
				r, done := startInteractiveTest(t, adapter, func(ctx context.Context) error {
					var err error
					result, err = Interactive(ctx, "dialogue", NewSession(ctx, RoleAgenticDialogues, ""), "Discuss.")
					return err
				})
				_ = r.acceptInteractive("dialogue", "opening", "Hello", false)
				<-adapter.started
				err := r.acceptInput(interactionInput{Address: "dialogue", ID: "control", Finish: control == "finish", Stop: control == "stop"})
				if err != nil {
					t.Fatal(err)
				}
				if control == "stop" && !success {
					eventuallyInteractive(t, func() bool { return waitingInteractive(r, 1) })
					assertPending(t, done)
					_ = r.acceptInteractive("dialogue", "finish", "", true)
				}
				if err := <-done; err != nil {
					t.Fatal(err)
				}
				expected := "human"
				if control == "stop" && success {
					expected = "agent"
				}
				if result.EndedBy != expected {
					t.Fatalf("result %+v", result)
				}
			})
		}
	}
}
func TestInteractiveSteeringAssignedOnce(t *testing.T) {
	for _, landed := range []bool{false, true} {
		t.Run(fmt.Sprintf("landed=%v", landed), func(t *testing.T) {
			adapter := &interactionAdapter{started: make(chan struct{}), release: make(chan struct{}), landed: landed}
			r, done := startInteractiveTest(t, adapter, func(ctx context.Context) error {
				_, err := Interactive(ctx, "dialogue", NewSession(ctx, RoleAgenticDialogues, ""), "Discuss.")
				return err
			})
			_ = r.acceptInteractive("dialogue", "opening", "Hello", false)
			<-adapter.started
			_ = r.acceptInteractive("dialogue", "second", "More", false)
			eventuallyInteractive(t, func() bool {
				r.interactionMu.Lock()
				defer r.interactionMu.Unlock()
				input := r.interactionInputs["second"]
				return input.Disposition == "delivered" || (!landed && input.Disposition == "pending")
			})
			close(adapter.release)
			messages := 3
			if !landed {
				messages = 4
			}
			eventuallyInteractive(t, func() bool { return waitingInteractive(r, messages) })
			want := 1
			if !landed {
				want = 2
			}
			if adapter.count() != want {
				t.Fatalf("calls %d want %d", adapter.count(), want)
			}
			_ = r.acceptInteractive("dialogue", "finish", "", true)
			if err := <-done; err != nil {
				t.Fatal(err)
			}
		})
	}
}

// A real child process is killed while waiting. Its replacement executes the
// unchanged body, reading command results/context and session positions from disk.
func TestInteractiveRestartChild(t *testing.T) {
	state := os.Getenv("GIMBAL_INTERACTIVE_RESTART_CHILD")
	if state == "" {
		t.Skip("restart child")
	}
	adapter := &restartInteractionAdapter{interactionAdapter: &interactionAdapter{}, state: state}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx := WithRunStore(Project(context.Background(), state), pauseInteractionStore{Store: directory.Store{Root: filepath.Join(state, "runs")}, state: state})
	ctx = live.WithHook(ctx, func(_ string, c live.Controller) func() {
		go func() {
			_ = http.Serve(listener, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				raw, _ := ioReadAll(r)
				if err := c.SubmitInteractive("dialogue", string(raw)); err != nil {
					http.Error(w, err.Error(), http.StatusServiceUnavailable)
					return
				}
				w.WriteHeader(204)
			}))
		}()
		if err := os.WriteFile(filepath.Join(state, "endpoint"), []byte("http://"+listener.Addr().String()), 0600); err != nil {
			panic(err)
		}
		return func() {}
	})
	err = Run(ctx, os.Getenv("GIMBAL_INTERACTIVE_RESTART_ID"), "restart", map[WorkflowRole]ModelBinding{RoleAgenticDialogues: {Adapter: adapter}}, func(ctx context.Context) error {
		var prior Text
		if err := Scope(ctx, "completed", func(ctx context.Context) error {
			var err error
			prior, err = NewSession(ctx, RoleAgenticDialogues, "").Generate[Text](ctx, "Saved generation.")
			return err
		}); err != nil {
			return err
		}
		if prior != "saved-generation" {
			return fmt.Errorf("saved generation lost: %q", prior)
		}
		code, out, _, err := RunCommand(ctx, "once", state, "sh", "-c", "printf x >> executions; printf saved-result")
		if err != nil {
			return err
		}
		if code != 0 || out != "saved-result" {
			return fmt.Errorf("saved command %d %q", code, out)
		}
		Set(ctx, "remember", out)
		session := NewSession(ctx, RoleAgenticDialogues, "")
		_, err = Interactive(ctx, "dialogue", session, "Context must contain saved-result.")
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	adapter.mu.Lock()
	defer adapter.mu.Unlock()
	if len(adapter.prompts) > 0 && !strings.Contains(adapter.prompts[len(adapter.prompts)-1], "saved-result") {
		t.Fatal("scope context lost")
	}
}
func ioReadAll(r *http.Request) ([]byte, error) {
	var b bytes.Buffer
	_, err := b.ReadFrom(r.Body)
	return b.Bytes(), err
}
func TestInteractiveResultsContextAndSessionSurviveRestart(t *testing.T) {
	state := t.TempDir()
	id := string(runstore.NewRunID())
	var child *exec.Cmd
	var output bytes.Buffer
	start := func() {
		t.Helper()
		output.Reset()
		child = exec.Command(os.Args[0], "-test.run=^TestInteractiveRestartChild$")
		child.Env = append(os.Environ(), "GIMBAL_INTERACTIVE_RESTART_CHILD="+state, "GIMBAL_INTERACTIVE_RESTART_ID="+id)
		child.Stdout = &output
		child.Stderr = &output
		if err := child.Start(); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		if child != nil && child.ProcessState == nil {
			_ = child.Process.Kill()
			_ = child.Wait()
		}
	})
	snapshot := func() (interactionState, bool) {
		raw, err := os.ReadFile(filepath.Join(state, "runs", id, "run.jsonl"))
		if err != nil {
			return interactionState{}, false
		}
		var latest InteractiveChanged
		for _, line := range bytes.Split(raw, []byte{'\n'}) {
			var event LifecycleRecord
			if json.Unmarshal(line, &event) == nil {
				if changed, ok := event.Event.(InteractiveChanged); ok {
					latest = changed
				}
			}
		}
		return interactionState{Waiting: latest.Waiting, Transcript: latest.Transcript}, latest.ID != ""
	}
	send := func(input, message string, finish bool) {
		t.Helper()
		eventuallyInteractive(t, func() bool {
			endpoint, err := os.ReadFile(filepath.Join(state, "endpoint"))
			if err != nil {
				return false
			}
			raw, _ := json.Marshal(interactionInput{ID: input, Message: message, Finish: finish})
			response, err := http.Post(string(endpoint), "application/json", bytes.NewReader(raw))
			if err != nil {
				return false
			}
			_ = response.Body.Close()
			return response.StatusCode == 204
		})
	}
	start()
	eventuallyInteractive(t, func() bool { s, ok := snapshot(); return ok && s.Waiting })
	send("one", "First", false)
	eventuallyInteractive(t, func() bool { s, ok := snapshot(); return ok && s.Waiting && len(s.Transcript.Messages) == 2 })
	// Hold the transition after owner admission, before the accepted input
	// can be consumed or dispatched. Its acknowledgement has already returned.
	if err := os.WriteFile(filepath.Join(state, "pause-input"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	send("two", "Second", false)
	eventuallyInteractive(t, func() bool { _, err := os.Stat(filepath.Join(state, "input-blocked")); return err == nil })
	if err := child.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = child.Wait()
	first := string(mustReadInteractive(t, filepath.Join(state, "runs", id, "run.jsonl")))
	if err := os.Remove(filepath.Join(state, "pause-input")); err != nil {
		t.Fatal(err)
	}
	start()
	// No resend: startup must find and consume the acknowledged queue item.
	eventuallyInteractive(t, func() bool { s, ok := snapshot(); return ok && s.Waiting && len(s.Transcript.Messages) == 4 })
	send("finish", "", true)
	if err := child.Wait(); err != nil {
		t.Fatalf("child %v: %s", err, output.String())
	}
	raw := mustReadInteractive(t, filepath.Join(state, "executions"))
	if string(raw) != "x" {
		t.Fatalf("command redispatched %q", raw)
	}
	second := string(mustReadInteractive(t, filepath.Join(state, "runs", id, "run.jsonl")))
	if !strings.HasPrefix(second, first) {
		t.Fatal("restart replaced prior history")
	}
	if strings.Count(second, `"kind":"turn_started"`) != 3 {
		t.Fatalf("turn history: %s", second)
	}
	if got := string(mustReadInteractive(t, filepath.Join(state, "generations"))); got != "x" {
		t.Fatalf("completed Generate redispatched: %q", got)
	}
	if got := string(mustReadInteractive(t, filepath.Join(state, "closes"))); strings.Count(got, "\n") != 2 {
		t.Fatalf("completed scope cleanup repeated: %q", got)
	}

}
func mustReadInteractive(t *testing.T, path string) []byte {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestInteractiveIdleConfigurationAndFinalDeregistration(t *testing.T) {
	adapter := &interactionAdapter{}
	r, done := startInteractiveTest(t, adapter, func(ctx context.Context) error {
		_, err := Interactive(ctx, "dialogue", NewSession(ctx, RoleAgenticDialogues, ""), "Discuss.")
		return err
	})
	if err := r.acceptInput(interactionInput{Address: "dialogue", ID: "config", Configure: true, Model: "new-model", Effort: "low"}); err != nil {
		t.Fatal(err)
	}
	eventuallyInteractive(t, func() bool {
		r.interactionMu.Lock()
		defer r.interactionMu.Unlock()
		for _, state := range r.interactionStates {
			for _, id := range state.Consumed {
				if id == "config" {
					return true
				}
			}
		}
		return false
	})
	_ = r.acceptInteractive("dialogue", "finish", "", true)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	r.interactionMu.Lock()
	defer r.interactionMu.Unlock()
	if len(r.activeInteractions) != 0 {
		t.Fatal("terminal run retained interaction owner")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.scopes) != 0 {
		t.Fatal("terminal run retained live scopes")
	}
}

// This store fixture pauses only the uncommitted transition in the child. It
// lets the parent crash the real process after input acknowledgement.
type pauseInteractionStore struct {
	runstore.Store
	state string
}
type pauseInteractionRun struct {
	runstore.Run
	state string
}

func (s pauseInteractionStore) Create(ctx context.Context, header runstore.Header) (runstore.Run, error) {
	r, err := s.Store.Create(ctx, header)
	if err != nil {
		return nil, err
	}
	return pauseInteractionRun{r, s.state}, nil
}
func (s pauseInteractionStore) Open(ctx context.Context, id runstore.RunID) (runstore.Run, error) {
	r, err := s.Store.Open(ctx, id)
	if err != nil {
		return nil, err
	}
	return pauseInteractionRun{r, s.state}, nil
}
func (r pauseInteractionRun) Append(ctx context.Context, facts []runstore.Fact, d runstore.Durability) ([]runstore.Record, error) {
	for _, fact := range facts {
		if fact.Kind != "interaction" {
			continue
		}
		var state interactionState
		if json.Unmarshal(fact.Payload, &state) == nil && state.ActiveInput == "two" {
			if _, err := os.Stat(filepath.Join(r.state, "pause-input")); err == nil {
				if err := os.WriteFile(filepath.Join(r.state, "input-blocked"), nil, 0600); err != nil {
					return nil, err
				}
				<-ctx.Done()
				return nil, ctx.Err()
			}
		}
	}
	return r.Run.Append(ctx, facts, d)
}

type restartInteractionAdapter struct {
	*interactionAdapter
	state string
}

func (a *restartInteractionAdapter) CreateSession(ctx context.Context, _, _, _ string) (string, error) {
	return effects.ExecutionIdentityFromContext(ctx).ExecutorSession, nil
}
func (a *restartInteractionAdapter) RunTurn(ctx context.Context, native, prompt string, schema json.RawMessage, emit func(AgentEvent) error) (TurnResult, error) {
	if len(schema) == 0 {
		if err := a.appendMarker("generations", "x"); err != nil {
			return TurnResult{}, err
		}
		return TurnResult{Output: json.RawMessage(`"saved-generation"`)}, nil
	}
	return a.interactionAdapter.RunTurn(ctx, native, prompt, schema, emit)
}
func (a *restartInteractionAdapter) Close(_ context.Context, native string) error {
	return a.appendMarker("closes", native+"\n")
}
func (a *restartInteractionAdapter) appendMarker(name, text string) error {
	file, err := os.OpenFile(filepath.Join(a.state, name), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	_, writeErr := file.WriteString(text)
	return errors.Join(writeErr, file.Close())
}

type delayedInteractionAdapter struct {
	*interactionAdapter
	cancelled chan struct{}
}

func (a *delayedInteractionAdapter) RunTurn(ctx context.Context, _ string, _ string, _ json.RawMessage, _ func(AgentEvent) error) (TurnResult, error) {
	close(a.started)
	<-ctx.Done()
	close(a.cancelled)
	<-a.release
	return TurnResult{}, ctx.Err()
}
func TestInteractiveInputFailureJoinsNativeTurnBeforeRunReturns(t *testing.T) {
	adapter := &delayedInteractionAdapter{interactionAdapter: &interactionAdapter{started: make(chan struct{}), release: make(chan struct{})}, cancelled: make(chan struct{})}
	directoryPath := t.TempDir()
	injected := errors.New("input assignment recording failed")
	var fail atomic.Bool
	store := faultStore{Store: directory.Store{Root: filepath.Join(directoryPath, "runs")}, beforeAppend: func(facts []runstore.Fact) error {
		if fail.Load() {
			for _, fact := range facts {
				var input interactionInput
				if fact.Kind == "interaction-input" && json.Unmarshal(fact.Payload, &input) == nil && input.Disposition == "inflight" {
					return injected
				}
			}
		}
		return nil
	}}
	ctx := WithRunStore(Project(t.Context(), directoryPath), store)
	owners := make(chan *run, 1)
	ctx = live.WithHook(ctx, func(_ string, c live.Controller) func() { owners <- c.(*run); return func() {} })
	done := make(chan error, 1)
	go func() {
		done <- Run(ctx, string(runstore.NewRunID()), "join", map[WorkflowRole]ModelBinding{RoleAgenticDialogues: {Adapter: adapter}}, func(ctx context.Context) error {
			_, err := Interactive(ctx, "dialogue", NewSession(ctx, RoleAgenticDialogues, ""), "Discuss.")
			return err
		})
	}()
	owner := <-owners
	eventuallyInteractive(t, func() bool { return waitingInteractive(owner, 0) })
	if err := owner.acceptInteractive("dialogue", "opening", "Hello", false); err != nil {
		t.Fatal(err)
	}
	<-adapter.started
	fail.Store(true)
	if err := owner.acceptInteractive("dialogue", "second", "Steer", false); err != nil {
		t.Fatal(err)
	}
	<-adapter.cancelled
	select {
	case err := <-done:
		t.Fatalf("Run returned before native cessation: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
	if owner.State() == "terminal" {
		t.Fatal("run closed its journal before native cessation")
	}
	close(adapter.release)
	if err := <-done; !errors.Is(err, injected) {
		t.Fatalf("input failure result %v", err)
	}
}

// Each adapter shape is a way a provider can report an interrupted native turn.
func TestInteractiveStopSettlesWhateverTheAdapterReports(t *testing.T) {
	uncertain := func(accepted bool) func() (TurnResult, error) {
		return func() (TurnResult, error) {
			return TurnResult{}, &backend.DeliveryUncertain{RequestID: "t", Accepted: accepted, Err: errors.New("unconfirmed")}
		}
	}
	for name, shape := range map[string]struct {
		onCancel func() (TurnResult, error)
		silent   bool
	}{
		"canceled":  {},
		"native":    {onCancel: func() (TurnResult, error) { return TurnResult{}, errors.New("codex: the turn was interrupted") }},
		"empty":     {onCancel: func() (TurnResult, error) { return TurnResult{}, nil }},
		"uncertain": {onCancel: uncertain(false)},
		// The native turn acknowledged the prompt, but no step, text or tool
		// event arrived before Stop.
		"accepted-before-any-event": {onCancel: uncertain(true), silent: true},
	} {
		t.Run(name, func(t *testing.T) {
			adapter := &interactionAdapter{started: make(chan struct{}), release: make(chan struct{}), onCancel: shape.onCancel, silent: shape.silent, replies: []string{"", `{"decision":{"type":"chat","message":"Back again"}}`}}
			r, done := startInteractiveTest(t, adapter, func(ctx context.Context) error {
				_, err := Interactive(ctx, "dialogue", NewSession(ctx, RoleAgenticDialogues, ""), "Discuss.")
				return err
			})
			_ = r.acceptInteractive("dialogue", "opening", "Hello", false)
			<-adapter.started
			if err := r.acceptInput(interactionInput{Address: "dialogue", ID: "stop", Stop: true}); err != nil {
				t.Fatal(err)
			}
			if err := r.acceptInput(interactionInput{Address: "dialogue", ID: "stop", Stop: true}); err != nil {
				t.Fatalf("repeated Stop: %v", err)
			}
			deadline := time.After(3 * time.Second)
			for !waitingInteractive(r, 1) {
				select {
				case err := <-done:
					t.Fatalf("workflow ended on Stop: %v", err)
				case <-deadline:
					t.Fatal("never settled")
				case <-time.After(time.Millisecond):
				}
			}
			if err := r.acceptInteractive("dialogue", "next", "Continue", false); err != nil {
				t.Fatal(err)
			}
			eventuallyInteractive(t, func() bool { return waitingInteractive(r, 3) })
			adapter.mu.Lock()
			native, calls := adapter.native, adapter.calls
			adapter.mu.Unlock()
			if native != "native-continuity" || calls != 2 {
				t.Fatalf("native %q calls %d", native, calls)
			}
			_ = r.acceptInteractive("dialogue", "finish", "", true)
			if err := <-done; err != nil {
				t.Fatal(err)
			}
		})
	}
}

// A prompt whose native acknowledgement is unknown is never settled by Stop,
// and is not sent again.
func TestInteractiveStopBeforeAcknowledgementStaysUnresolved(t *testing.T) {
	adapter := &interactionAdapter{started: make(chan struct{}), release: make(chan struct{}), silent: true, onCancel: func() (TurnResult, error) {
		return TurnResult{}, &backend.DeliveryUncertain{RequestID: "t", Err: errors.New("acknowledgement lost")}
	}}
	r, done := startInteractiveTest(t, adapter, func(ctx context.Context) error {
		_, err := Interactive(ctx, "dialogue", NewSession(ctx, RoleAgenticDialogues, ""), "Discuss.")
		return err
	})
	_ = r.acceptInteractive("dialogue", "opening", "Hello", false)
	<-adapter.started
	_ = r.acceptInput(interactionInput{Address: "dialogue", ID: "stop", Stop: true})
	if err := <-done; err == nil || !isUnresolved(err) && !strings.Contains(err.Error(), "unresolved") {
		t.Fatalf("run ended with %v", err)
	}
	if adapter.count() != 1 {
		t.Fatalf("prompt sent %d times", adapter.count())
	}
}

// A prompt still waiting for the provider was never submitted. Stop abandons it
// without ending the conversation, and the next prompt is sent alone.
func TestInteractiveStopAbandonsPromptNotYetSubmitted(t *testing.T) {
	adapter := &interactionAdapter{started: make(chan struct{}), release: make(chan struct{}), silent: true, replies: []string{"", `{"decision":{"type":"chat","message":"Fresh"}}`}}
	r, done := startInteractiveTest(t, adapter, func(ctx context.Context) error {
		_, err := Interactive(ctx, "dialogue", NewSession(ctx, RoleAgenticDialogues, ""), "Discuss.")
		return err
	})
	_ = r.acceptInteractive("dialogue", "opening", "Abandoned", false)
	<-adapter.started
	_ = r.acceptInput(interactionInput{Address: "dialogue", ID: "stop", Stop: true})
	eventuallyInteractive(t, func() bool { return waitingInteractive(r, 0) })
	assertPending(t, done)
	_ = r.acceptInteractive("dialogue", "next", "Continue", false)
	eventuallyInteractive(t, func() bool { return waitingInteractive(r, 2) })
	adapter.mu.Lock()
	prompts := strings.Join(adapter.prompts, "\n")
	adapter.mu.Unlock()
	if strings.Count(prompts, "Abandoned") != 1 || strings.Count(prompts, "Continue") != 1 {
		t.Fatalf("prompts %q", prompts)
	}
	_ = r.acceptInteractive("dialogue", "finish", "", true)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}
