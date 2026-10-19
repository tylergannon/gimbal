package taskdaemon

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/internal/effects"
	"github.com/tylergannon/gimbal/internal/live"

	"github.com/tylergannon/gimbal/runstore"
)

func TestLostProcessReplyPreservesCommandResultWithoutReplay(t *testing.T) {
	d := New(t.TempDir())
	native := d.Handler()
	var starts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/tasks/process" {
			native.ServeHTTP(w, r)
			return
		}
		starts.Add(1)
		var req effects.Process
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
			return
		}
		if err := d.process(req); err != nil {
			t.Error(err)
			return
		}
		conn, _, err := w.(http.Hijacker).Hijack()
		if err != nil {
			t.Error(err)
			return
		}
		_ = conn.Close()
	}))
	defer server.Close()
	ctx := effects.WithEndpoint(gimbal.Project(context.Background(), t.TempDir()), server.URL)
	err := gimbal.Run(ctx, string(runstore.NewRunID()), "lost-process-reply", nil, func(ctx context.Context) error {
		code, stdout, stderr, err := gimbal.RunCommand(ctx, "command", t.TempDir(), "zsh", "-c", "printf actual-output; printf actual-error >&2; exit 7")
		if err != nil {
			return err
		}
		if code != 7 || stdout != "actual-output" || stderr != "actual-error" {
			t.Errorf("command result: %d %q %q", code, stdout, stderr)
		}
		return nil
	})
	if err != nil || starts.Load() != 1 {
		t.Fatalf("starts=%d err=%v", starts.Load(), err)
	}
}

func TestWorkflowScopeUnwindsRemoteResources(t *testing.T) {
	for _, mode := range []string{"return", "error", "panic", "cancel"} {
		t.Run(mode, func(t *testing.T) {
			d := New(t.TempDir())
			server := httptest.NewServer(d.Handler())
			defer server.Close()
			ctx := effects.WithEndpoint(gimbal.Project(context.Background(), t.TempDir()), server.URL)
			var runID string
			body := func(ctx context.Context) error {
				if err := gimbal.Service(ctx, "parent", t.TempDir(), `touch "$ENV_FILE.tmp"; mv "$ENV_FILE.tmp" "$ENV_FILE"; exec sleep 60`); err != nil {
					return err
				}
				err := gimbal.Scope(ctx, "child", func(ctx context.Context) error {
					if err := gimbal.Service(ctx, "dependency", t.TempDir(), `touch "$ENV_FILE.tmp"; mv "$ENV_FILE.tmp" "$ENV_FILE"; exec sleep 60`); err != nil {
						return err
					}
					d.mu.Lock()
					for _, v := range d.tasks {
						runID = v.owner.Run
					}
					d.mu.Unlock()
					switch mode {
					case "error":
						return errors.New("body failed")
					case "panic":
						panic("body panic")
					case "cancel":
						ctx, cancel := context.WithCancel(ctx)
						cancel()
						_, _, _, err := gimbal.RunCommand(ctx, "cancelled", t.TempDir(), "sleep", "60")
						return err
					}
					return nil
				})
				if d.state(effects.Scope{Run: runID, Scope: "child.1"}).Pending != 0 {
					t.Error("child resources retained")
				}
				if d.state(effects.Scope{Run: runID}).Pending != 1 {
					t.Error("child closed parent")
				}
				return err
			}
			var err error
			func() {
				defer func() {
					if p := recover(); p != nil && mode != "panic" {
						t.Fatalf("unexpected panic: %v", p)
					}
				}()
				err = gimbal.Run(ctx, string(runstore.NewRunID()), "remote-scope", nil, body)
			}()
			if mode == "return" && err != nil {
				t.Fatal(err)
			}
			if (mode == "error" || mode == "cancel") && err == nil {
				t.Fatal("body error lost")
			}
			if d.state(effects.Scope{Run: runID}).Pending != 0 {
				t.Fatal("root resources retained after unwind")
			}
		})
	}
}

func TestUnexpectedRemoteServiceExitFailsScope(t *testing.T) {
	d := New(t.TempDir())
	server := httptest.NewServer(d.Handler())
	defer server.Close()
	ctx := effects.WithEndpoint(gimbal.Project(context.Background(), t.TempDir()), server.URL)
	err := gimbal.Run(ctx, string(runstore.NewRunID()), "remote-failure", nil, func(ctx context.Context) error {
		if err := gimbal.Service(ctx, "dependency", t.TempDir(), `touch "$ENV_FILE.tmp"; mv "$ENV_FILE.tmp" "$ENV_FILE"; sleep 0.1; exit 0`); err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(3 * time.Second):
			return nil
		}
	})
	if err == nil {
		t.Fatal("required service exit did not fail scope")
	}
}

func TestSuccessfulAncestorCleanupClearsFailedChildOwnership(t *testing.T) {
	d := New(t.TempDir())
	native := d.Handler()
	var failed atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/tasks/cleanup" && !failed.Load() {
			var req effects.Scope
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Error(err)
				return
			}
			if req.Scope != "" {
				failed.Store(true)
				_ = json.NewEncoder(w).Encode(effects.Reply{Error: "transient child cleanup failure"})
				return
			}
			r.Body = io.NopCloser(strings.NewReader(`{"Run":"` + req.Run + `","Scope":""}`))
		}
		native.ServeHTTP(w, r)
	}))
	defer server.Close()
	ctx := effects.WithEndpoint(gimbal.Project(context.Background(), t.TempDir()), server.URL)
	var controller live.Controller
	ctx = live.WithHook(ctx, func(_ string, c live.Controller) func() { controller = c; return func() {} })
	ctx = gimbal.WithCleanupTimeout(ctx, 250*time.Millisecond)
	err := gimbal.Run(ctx, string(runstore.NewRunID()), "cleanup-retry", nil, func(ctx context.Context) error {
		return gimbal.Scope(ctx, "child", func(ctx context.Context) error {
			return gimbal.Service(ctx, "dependency", t.TempDir(), `touch "$ENV_FILE.tmp"; mv "$ENV_FILE.tmp" "$ENV_FILE"; exec sleep 60`)
		})
	})
	if err == nil {
		t.Fatal("original cleanup failure lost")
	}
	if controller.CleanupPending() {
		t.Fatal("successful ancestor cleanup left child ownership pending")
	}
}
