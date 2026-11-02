package agentexecutor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/backend"
	"github.com/tylergannon/gimbal/internal/effects"
	"github.com/tylergannon/gimbal/internal/executor"
)

type lifetimeNative struct {
	nativeAdapter
	mode                    string
	Saves, Detaches, Closes int
}

func (a *lifetimeNative) CreateSession(context.Context, string, string, string) (string, error) {
	return "native-session", nil
}
func (a *lifetimeNative) LoadSession(_ context.Context, file string, _ bool) (string, error) {
	var id string
	err := executor.Read(file, &id)
	return id, err
}
func (a *lifetimeNative) SaveSession(_ context.Context, id, file string) error {
	a.Saves++
	return executor.Write(file, id)
}
func (a *lifetimeNative) DetachSession(ctx context.Context, _ string) error {
	a.Detaches++
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("local release inherited cancellation: %w", err)
	}
	if a.mode == "detach-fail" || a.mode == "fork-detach-fail" || a.mode == "cancel-detach-fail" {
		return errors.New("unsubscribe rejected")
	}
	return nil
}
func (a *lifetimeNative) Fork(context.Context, string) (string, error) {
	return "native-fork", nil
}
func (a *lifetimeNative) Close(ctx context.Context, _ string) error {
	a.Closes++
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("local release inherited cancellation: %w", err)
	}
	return errors.New("archive rejected") // never becomes successful on retry
}
func (a *lifetimeNative) RunTurn(ctx context.Context, _, _ string, _ json.RawMessage, _ func(gimbal.AgentEvent) error) (gimbal.TurnResult, error) {
	if a.mode == "cancel" || a.mode == "cancel-race" || a.mode == "cancel-detach-fail" {
		<-ctx.Done()
		if a.mode == "cancel-detach-fail" {
			return gimbal.TurnResult{}, ctx.Err()
		}
		if a.mode == "cancel" {
			return gimbal.TurnResult{}, &backend.DeliveryUncertain{RequestID: "native-turn", Accepted: true, Err: errors.Join(ctx.Err(), errors.New("native cessation remains unknown"))}
		}
	}
	out := gimbal.TurnResult{Output: json.RawMessage(`"native final result"`)}
	if a.mode == "error" {
		return out, errors.New("native turn failed")
	}
	return out, nil
}

// A Go-test child owns the ordinary worker loop and its startup pipe. All
// provider methods are replaced by the local double above; no SDK runs.
func TestAgentLifetimeWorkerFixture(t *testing.T) {
	root := os.Getenv("GIMBAL_TEST_AGENT_LIFETIME_ROOT")
	if root == "" {
		t.Skip("worker fixture only")
	}
	var req Request
	if err := json.NewDecoder(os.Stdin).Decode(&req); err != nil {
		panic(err)
	}
	adapter := &lifetimeNative{mode: req.Prompt}
	err := run(root, req, func(string) (nativeAdapter, error) { return adapter, nil })
	if err != nil {
		executor.RejectStartup(err)
	}
	if writeErr := executor.Write(filepath.Join(root, "fixture-returned.json"), adapter); writeErr != nil {
		panic(writeErr)
	}
	if err != nil {
		os.Exit(1)
	}
	os.Exit(0)
}

func TestNativeOwnerReturnsAfterNormalCancelAndErrorWithoutCleanupRetry(t *testing.T) {
	for _, mode := range []string{"normal", "cancel", "cancel-race", "cancel-detach-fail", "error", "detach-fail", "archive-fail", "fork-detach-fail"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			t.Setenv("GIMBAL_TEST_AGENT_LIFETIME_ROOT", root)
			sid := executor.NewSessionID()
			config := Session{ID: sid, Harness: "codex", Workdir: root}
			if mode == "fork-detach-fail" {
				config.Parent = executor.NewSessionID()
				if err := os.MkdirAll(sessionDir(root, config.Parent), 0700); err != nil {
					t.Fatal(err)
				}
				if err := executor.Write(filepath.Join(sessionDir(root, config.Parent), "native.json"), "native-parent"); err != nil {
					t.Fatal(err)
				}
			}
			if err := Create(root, config); err != nil {
				t.Fatal(err)
			}
			req := Request{ID: executor.NewTurnID(), Session: sid, Prompt: mode}
			req.Fork = mode == "fork-detach-fail"
			if mode == "cancel" || mode == "cancel-race" || mode == "cancel-detach-fail" {
				req.Timeout = 20 * time.Millisecond
			}
			checkpoint := filepath.Join(sessionDir(root, sid), "native.json")
			if mode == "archive-fail" {
				req.Close = true
				if err := executor.Write(checkpoint, "native-session"); err != nil {
					t.Fatal(err)
				}
			}
			binary, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			dir := filepath.Join(root, string(req.ID))
			descriptor, err := executor.Launch(binary, []string{"-test.run=^TestAgentLifetimeWorkerFixture$"}, req, dir, "", "")
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			out, resultErr := Wait(ctx, descriptor, nil)
			if mode == "normal" || mode == "cancel-race" {
				if resultErr != nil || string(out.Output) != `"native final result"` {
					t.Fatalf("authoritative result lost: %s %v", out.Output, resultErr)
				}
			} else if resultErr == nil {
				t.Fatal("native failure was hidden")
			}
			if mode == "cancel" {
				uncertain, ok := errors.AsType[*backend.DeliveryUncertain](resultErr)
				if !ok || uncertain.RequestID != "native-turn" || !uncertain.Accepted || !errors.Is(resultErr, context.DeadlineExceeded) || !strings.Contains(resultErr.Error(), "cessation remains unknown") {
					t.Fatalf("native uncertainty lost at worker boundary: %v", resultErr)
				}
			}
			if mode == "cancel-detach-fail" && (!errors.Is(resultErr, context.DeadlineExceeded) || !strings.Contains(resultErr.Error(), "unsubscribe rejected")) {
				t.Fatalf("cancelled result hid release failure: %v", resultErr)
			}
			var observed lifetimeNative
			for {
				err = executor.Read(filepath.Join(root, "fixture-returned.json"), &observed)
				if err == nil {
					break
				}
				select {
				case <-ctx.Done():
					t.Fatal("worker retained its native owner after publishing result")
				case <-time.After(time.Millisecond):
				}
			}
			lease, err := executor.Lock(dir)
			if err != nil || lease == nil {
				t.Fatalf("operation owner retained: %v", err)
			}
			lease.Close()
			lease, err = executor.Lock(sessionDir(root, sid))
			if err != nil || lease == nil {
				t.Fatalf("session owner retained: %v", err)
			}
			lease.Close()
			if executor.Exists(filepath.Join(dir, "cleanup.json")) || executor.Exists(filepath.Join(sessionDir(root, sid), "active.json")) {
				t.Fatal("worker left cancellation confirmation or live owner state")
			}
			if mode == "archive-fail" {
				if observed.Closes != 1 || observed.Detaches != 0 {
					t.Fatalf("archive was retried: %+v", observed)
				}
				adapter := &Adapter{Root: root, harness: "codex"}
				if err := adapter.Close(effects.WithExecutionIdentity(t.Context(), backend.NewExecutionIdentity("close")), string(sid)); err == nil || !strings.Contains(err.Error(), "archive rejected") {
					t.Fatalf("closed local scope lost its error: %v", err)
				}
			} else if mode == "fork-detach-fail" {
				if observed.Closes != 1 || observed.Detaches != 1 || !strings.Contains(resultErr.Error(), "unsubscribe rejected") {
					t.Fatalf("fork hid failed parent detach or retained its child: %+v %v", observed, resultErr)
				}
			} else if observed.Closes != 0 || observed.Detaches != 1 || !executor.Exists(checkpoint) || executor.Exists(filepath.Join(sessionDir(root, sid), "closed.json")) {
				t.Fatalf("turn release broke durable human session lifetime: %+v", observed)
			}
			t.Logf("%s: owner returned; closes=%d detaches=%d, unresolved=%v", mode, observed.Closes, observed.Detaches, mode == "cancel")
		})
	}
}
