package opencode

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tylergannon/gimbal"
)

// TestLiveAdapterContract exercises the installed OpenCode legacy API through
// the real adapter. It owns an isolated server state directory and workdirs.
//
//	GIMBAL_OPENCODE_LIVE=1 go test ./opencode -run TestLiveAdapterContract -v
func TestLiveAdapterContract(t *testing.T) {
	if os.Getenv("GIMBAL_OPENCODE_LIVE") != "1" {
		t.Skip("set GIMBAL_OPENCODE_LIVE=1 to exercise the installed OpenCode adapter")
	}
	model := strings.TrimSpace(os.Getenv("GIMBAL_OPENCODE_LIVE_MODEL"))
	if model == "" {
		model = "ling-3.0-flash-fin-free"
	}
	t.Logf("OpenCode provider/model: opencode/%s", model)

	stateDir := strings.TrimSpace(os.Getenv("GIMBAL_OPENCODE_LIVE_STATE_DIR"))
	if stateDir == "" {
		stateDir = filepath.Join(t.TempDir(), "state")
	} else {
		var err error
		stateDir, err = filepath.Abs(stateDir)
		if err != nil {
			t.Fatal(err)
		}
	}
	ad := newAdapter(adapterConfig{stateDir: stateDir})
	cleanup := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		_, _ = StopServer(ctx, stateDir)
	}
	t.Cleanup(cleanup)

	workOne := t.TempDir()
	workTwo := t.TempDir()
	one := liveCreateSession(t, ad, model, workOne)
	two := liveCreateSession(t, ad, model, workTwo)
	t.Cleanup(func() {
		_ = ad.Close(context.Background(), one)
		_ = ad.Close(context.Background(), two)
	})

	t.Run("text schema continuation and tools", func(t *testing.T) {
		result := liveTurn(t, ad, one, "Reply with exactly OPENCODE_TEXT_OK", nil, nil)
		if strings.TrimSpace(decodeLiveText(t, result.Output)) != "OPENCODE_TEXT_OK" {
			t.Fatalf("text output = %s", result.Output)
		}

		schema := json.RawMessage(`{"type":"object","properties":{"answer":{"type":"string"}},"required":["answer"],"additionalProperties":false}`)
		structured := liveTurn(t, ad, one, "Return structured output with answer exactly SCHEMA_OK.", schema, nil)
		var value struct {
			Answer string `json:"answer"`
		}
		if err := json.Unmarshal(structured.Output, &value); err != nil || value.Answer != "SCHEMA_OK" {
			t.Fatalf("structured output = %s (%v)", structured.Output, err)
		}

		token := fmt.Sprintf("BASE_%d", time.Now().UnixNano())
		_ = liveTurn(t, ad, one, "Remember this token for later: "+token+". Reply exactly READY.", nil, nil)
		continued := liveTurn(t, ad, one, "Reply with only the token I asked you to remember.", nil, nil)
		if strings.TrimSpace(decodeLiveText(t, continued.Output)) != token {
			t.Fatalf("continued output = %s, want %s", continued.Output, token)
		}

		file := filepath.Join(workOne, "adapter-live.txt")
		_ = liveTurn(t, ad, one, "Use a file tool to create adapter-live.txt in the current project with exact contents TOOL_FILE_OK. Then reply exactly FILE_OK.", nil, nil)
		contents, err := os.ReadFile(file)
		if err != nil || string(contents) != "TOOL_FILE_OK" {
			t.Fatalf("tool file = %q (%v)", contents, err)
		}
	})

	t.Run("native fork is independent", func(t *testing.T) {
		parent := liveCreateSession(t, ad, model, workOne)
		defer func() { _ = ad.Close(context.Background(), parent) }()
		baseToken := fmt.Sprintf("BASE_%d", time.Now().UnixNano())
		_ = liveTurn(t, ad, parent, "Remember this token: "+baseToken+". Reply exactly READY.", nil, nil)
		fork, err := ad.Fork(liveContext(t), parent)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = ad.Close(context.Background(), fork) }()
		forkToken := fmt.Sprintf("FORK_%d", time.Now().UnixNano())
		_ = liveTurn(t, ad, fork, "Replace the remembered token with "+forkToken+". Reply exactly READY.", nil, nil)
		forked := liveTurn(t, ad, fork, "Reply with only the current remembered token.", nil, nil)
		base := liveTurn(t, ad, parent, "Reply with only the token I originally asked you to remember.", nil, nil)
		if strings.TrimSpace(decodeLiveText(t, forked.Output)) != forkToken || strings.TrimSpace(decodeLiveText(t, base.Output)) != baseToken {
			t.Fatalf("fork/base outputs = %s / %s", forked.Output, base.Output)
		}
	})

	t.Run("concurrent project routing and close isolation", func(t *testing.T) {
		sessions := []string{one, two}
		contents := []string{"PROJECT_ONE", "PROJECT_TWO"}
		var wait sync.WaitGroup
		errs := make(chan error, 2)
		for index := range sessions {
			wait.Go(func() {
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
				defer cancel()
				result, err := ad.RunTurn(ctx, sessions[index], "Reply exactly "+contents[index], nil, func(event gimbal.AgentEvent) error {
					var data struct {
						SessionID string `json:"sessionID"`
					}
					if err := json.Unmarshal(event.Data, &data); err != nil {
						return err
					}
					if data.SessionID != sessions[index] {
						return fmt.Errorf("session %s received event for %s", sessions[index], data.SessionID)
					}
					return nil
				})
				if err == nil {
					var text string
					err = json.Unmarshal(result.Output, &text)
					if err == nil && strings.TrimSpace(text) != contents[index] {
						err = fmt.Errorf("session %s output %q, want %q", sessions[index], text, contents[index])
					}
				}
				errs <- err
			})
		}
		wait.Wait()
		close(errs)
		for err := range errs {
			if err != nil {
				t.Fatal(err)
			}
		}
		if err := ad.Close(liveContext(t), one); err != nil {
			t.Fatal(err)
		}
		remaining := liveTurn(t, ad, two, "Reply exactly OTHER_STILL_WORKS", nil, nil)
		if strings.TrimSpace(decodeLiveText(t, remaining.Output)) != "OTHER_STILL_WORKS" {
			t.Fatalf("remaining session output = %s", remaining.Output)
		}
	})

	t.Run("active steering cancellation and explicit stop", func(t *testing.T) {
		steerSession := liveCreateSession(t, ad, model, workTwo)
		defer func() { _ = ad.Close(context.Background(), steerSession) }()
		started := make(chan struct{}, 1)
		steered := make(chan struct {
			result gimbal.TurnResult
			err    error
		}, 1)
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			result, err := ad.RunTurn(ctx, steerSession, "Use the shell tool to run sleep 20, then reply TOO_LATE.", nil, toolStarted(started))
			steered <- struct {
				result gimbal.TurnResult
				err    error
			}{result, err}
		}()
		waitLiveSignal(t, started, "steering tool start")
		landed, err := ad.Steer(liveContext(t), steerSession, "Stop waiting and reply exactly STEER_OK.")
		if err != nil || !landed {
			t.Fatalf("Steer() = (%v, %v)", landed, err)
		}
		outcome := <-steered
		if outcome.err != nil || strings.TrimSpace(decodeLiveText(t, outcome.result.Output)) != "STEER_OK" {
			t.Fatalf("steered output = %s (%v)", outcome.result.Output, outcome.err)
		}

		cancelSession := liveCreateSession(t, ad, model, workTwo)
		defer func() { _ = ad.Close(context.Background(), cancelSession) }()
		cancelStarted := make(chan struct{}, 1)
		ctx, cancel := context.WithCancel(context.Background())
		cancelled := make(chan error, 1)
		go func() {
			_, err := ad.RunTurn(ctx, cancelSession, "Use the shell tool to run sleep 20, then reply TOO_LATE.", nil, toolStarted(cancelStarted))
			cancelled <- err
		}()
		waitLiveSignal(t, cancelStarted, "cancellation tool start")
		cancel()
		select {
		case err := <-cancelled:
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("cancelled error = %v", err)
			}
		case <-time.After(15 * time.Second):
			t.Fatal("cancelled active model POST hung")
		}

		stopSession := liveCreateSession(t, ad, model, workTwo)
		stopStarted := make(chan struct{}, 1)
		stopped := make(chan error, 1)
		go func() {
			_, err := ad.RunTurn(context.Background(), stopSession, "Use the shell tool to run sleep 20, then reply TOO_LATE.", nil, toolStarted(stopStarted))
			stopped <- err
		}()
		waitLiveSignal(t, stopStarted, "stop tool start")
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 15*time.Second)
		wasStopped, err := StopServer(stopCtx, stateDir)
		stopCancel()
		if err != nil || !wasStopped {
			t.Fatalf("StopServer() = (%v, %v)", wasStopped, err)
		}
		select {
		case err := <-stopped:
			if err == nil {
				t.Fatal("active RunTurn succeeded after explicit server stop")
			}
		case <-time.After(15 * time.Second):
			t.Fatal("explicit stop left active model POST hanging")
		}
	})
}

func liveCreateSession(t *testing.T, ad *adapter, model, workdir string) string {
	t.Helper()
	ctx := liveContext(t)
	id, err := ad.CreateSession(ctx, model, "", workdir)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func liveTurn(t *testing.T, ad *adapter, sessionID, prompt string, schema json.RawMessage, onEvent func(gimbal.AgentEvent) error) gimbal.TurnResult {
	t.Helper()
	ctx := liveContext(t)
	result, err := ad.RunTurn(ctx, sessionID, prompt, schema, onEvent)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func liveContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)
	return ctx
}

func decodeLiveText(t *testing.T, raw json.RawMessage) string {
	t.Helper()
	var text string
	if err := json.Unmarshal(raw, &text); err != nil {
		t.Fatal(err)
	}
	return text
}

func toolStarted(signal chan<- struct{}) func(gimbal.AgentEvent) error {
	var once sync.Once
	return func(event gimbal.AgentEvent) error {
		if event.Type == "session.tool.called" {
			once.Do(func() { signal <- struct{}{} })
		}
		return nil
	}
}

func waitLiveSignal(t *testing.T, signal <-chan struct{}, name string) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(45 * time.Second):
		t.Fatalf("timed out waiting for %s", name)
	}
}
