package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/web"
)

type pendingCleanupAdapter struct {
	mu            sync.Mutex
	ready         bool
	turns, closes int
}

func (*pendingCleanupAdapter) CreateSession(context.Context, string, string, string) (string, error) {
	return "owned-native", nil
}
func (*pendingCleanupAdapter) Fork(context.Context, string) (string, error) {
	return "", errors.New("unused")
}
func (*pendingCleanupAdapter) Steer(context.Context, string, string) (bool, error) { return false, nil }
func (a *pendingCleanupAdapter) RunTurn(context.Context, string, string, json.RawMessage, func(gimbal.AgentEvent) error) (gimbal.TurnResult, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.turns++
	return gimbal.TurnResult{Output: json.RawMessage(`"existing work"`)}, nil
}
func (a *pendingCleanupAdapter) Close(context.Context, string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.closes++
	if !a.ready {
		return errors.New("remote stop not yet verified")
	}
	return nil
}

func TestCancelCommandRetainsAndRetriesTerminalRunCleanup(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	project := t.TempDir()
	_, owner, err := testProject(ctx, project, web.WithNoWeb())
	if err != nil {
		t.Fatal(err)
	}
	ad := &pendingCleanupAdapter{}
	err = owner.Run(ctx, "retained-cleanup", map[gimbal.WorkflowRole]gimbal.ModelBinding{"coder": {Adapter: ad, Model: "m"}}, func(ctx context.Context) error {
		_, err := gimbal.NewSession(ctx, "coder", project).Generate[gimbal.Text](ctx, "perform this task once")
		if err != nil {
			return err
		}
		return errors.New("original transport failure")
	})
	if err == nil || !strings.Contains(err.Error(), "original transport failure") {
		t.Fatalf("run error=%v", err)
	}
	id := oneCLIRunID(t, filepath.Join(project, ".gimbal"))
	invoke := func() error {
		return run([]string{"cancel", id, "--work-dir", project}, &bytes.Buffer{}, &bytes.Buffer{}, os.Getenv)
	}
	if err := invoke(); err == nil || !strings.Contains(err.Error(), "remote stop not yet verified") {
		t.Fatalf("pending stop=%v", err)
	}
	ad.mu.Lock()
	ad.ready = true
	ad.mu.Unlock()
	if err := invoke(); err != nil {
		t.Fatal(err)
	}
	if err := invoke(); err == nil || !strings.Contains(err.Error(), "retains control") {
		t.Fatalf("completed cleanup still advertised=%v", err)
	}
	ad.mu.Lock()
	defer ad.mu.Unlock()
	if ad.turns != 1 || ad.closes != 3 {
		t.Fatalf("turns=%d cleanup attempts=%d", ad.turns, ad.closes)
	}
	raw, err := os.ReadFile(filepath.Join(project, ".gimbal", "runs", id, "run.json"))
	if err != nil || !strings.Contains(string(raw), "original transport failure") {
		t.Fatalf("original error lost=%s,%v", raw, err)
	}
}
