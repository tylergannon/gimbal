package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/tylergannon/gimble"
	"github.com/tylergannon/gimble/web"
)

func TestSteerCommandDeliversToRunningSession(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	project := t.TempDir()
	_, runtime, err := testProject(ctx, project, web.WithNoWeb())
	if err != nil {
		t.Fatal(err)
	}
	adapter := &steerAdapter{started: make(chan struct{}), steered: make(chan string, 1)}
	var runWG sync.WaitGroup
	defer func() {
		cancel()
		runWG.Wait()
	}()
	runWG.Go(func() {
		_ = runtime.Run(ctx, "steering", map[gimble.WorkflowRole]gimble.ModelBinding{
			"coder": {Adapter: adapter, Model: "m"},
		}, func(ctx context.Context) error {
			return gimble.Scope(ctx, "lap", func(ctx context.Context) error {
				session := gimble.NewSession(ctx, "coder", "/w")
				_, err := session.Generate[gimble.Text](ctx, "active")
				return err
			})
		})
	})
	<-adapter.started
	runID := oneCLIRunID(t, filepath.Join(project, ".gimble"))

	var output bytes.Buffer
	if err := run([]string{"steer", "--work-dir", project, "--session", "lap.1/coder.1", runID, "look at this"}, &output, &bytes.Buffer{}, os.Getenv); err != nil {
		t.Fatal(err)
	}
	var response struct {
		Landed bool `json:"landed"`
	}
	if err := json.Unmarshal(output.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if !response.Landed {
		t.Fatalf("steer response = %s; want landed", output.Bytes())
	}
	if message := <-adapter.steered; message != "look at this" {
		t.Fatalf("steered message = %q; want look at this", message)
	}
}

func TestSteerCommandReportsIdleSessionAsDropped(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	project := t.TempDir()
	_, runtime, err := testProject(ctx, project, web.WithNoWeb())
	if err != nil {
		t.Fatal(err)
	}
	adapter := &steerAdapter{idleMode: true, started: make(chan struct{}), idle: make(chan struct{}), release: make(chan struct{})}
	var runWG sync.WaitGroup
	defer func() {
		cancel()
		runWG.Wait()
	}()
	runWG.Go(func() {
		_ = runtime.Run(ctx, "steering", map[gimble.WorkflowRole]gimble.ModelBinding{
			"coder": {Adapter: adapter, Model: "m"},
		}, func(ctx context.Context) error {
			return gimble.Scope(ctx, "lap", func(ctx context.Context) error {
				session := gimble.NewSession(ctx, "coder", "/w")
				_, err := session.Generate[gimble.Text](ctx, "idle")
				close(adapter.idle)
				<-adapter.release
				return err
			})
		})
	})
	<-adapter.started
	<-adapter.idle
	runID := oneCLIRunID(t, filepath.Join(project, ".gimble"))

	var output bytes.Buffer
	if err := run([]string{"steer", "--work-dir", project, "--session", "lap.1/coder.1", runID, "hello"}, &output, &bytes.Buffer{}, os.Getenv); err != nil {
		t.Fatal(err)
	}
	var response struct {
		Landed bool `json:"landed"`
	}
	if err := json.Unmarshal(output.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Landed {
		t.Fatalf("steer response = %s; want dropped", output.Bytes())
	}
	close(adapter.release)
}

func TestSteerCommandRejectsInvalidAndUnknownTargets(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	project := t.TempDir()
	_, runtime, err := testProject(ctx, project, web.WithNoWeb())
	if err != nil {
		t.Fatal(err)
	}
	adapter := &steerAdapter{started: make(chan struct{})}
	var runWG sync.WaitGroup
	defer func() {
		cancel()
		runWG.Wait()
	}()
	runWG.Go(func() {
		_ = runtime.Run(ctx, "steering", map[gimble.WorkflowRole]gimble.ModelBinding{
			"coder": {Adapter: adapter, Model: "m"},
		}, func(ctx context.Context) error {
			return gimble.Scope(ctx, "lap", func(ctx context.Context) error {
				session := gimble.NewSession(ctx, "coder", "/w")
				_, err := session.Generate[gimble.Text](ctx, "active")
				return err
			})
		})
	})
	<-adapter.started
	runID := oneCLIRunID(t, filepath.Join(project, ".gimble"))

	for _, test := range []struct {
		name string
		args []string
		want string
	}{
		{name: "unknown run", args: []string{"steer", "--work-dir", project, "--session", "lap.1/coder.1", "missing", "hello"}, want: "owns run"},
		{name: "unknown session", args: []string{"steer", "--work-dir", project, "--session", "lap.1/missing.1", runID, "hello"}, want: "session"},
		{name: "blank message", args: []string{"steer", "--work-dir", project, "--session", "lap.1/coder.1", runID, "   "}, want: "blank"},
	} {
		t.Run(test.name, func(t *testing.T) {
			var output, errorsOut bytes.Buffer
			err := run(test.args, &output, &errorsOut, os.Getenv)
			if err == nil || !strings.Contains(strings.ToLower(err.Error()), test.want) {
				t.Fatalf("error = %v; want containing %q", err, test.want)
			}
		})
	}
}

type steerAdapter struct {
	started  chan struct{}
	steered  chan string
	idleMode bool
	idle     chan struct{}
	release  chan struct{}
	mu       sync.Mutex
	active   bool
}

func (a *steerAdapter) CreateSession(context.Context, string, string, string) (string, error) {
	return "native", nil
}

func (a *steerAdapter) RunTurn(ctx context.Context, _, prompt string, _ json.RawMessage, _ func(gimble.AgentEvent) error) (gimble.TurnResult, error) {
	a.mu.Lock()
	a.active = true
	a.mu.Unlock()
	select {
	case a.started <- struct{}{}:
	default:
	}
	defer func() {
		a.mu.Lock()
		a.active = false
		a.mu.Unlock()
	}()
	if prompt == "idle" {
		return gimble.TurnResult{Output: json.RawMessage(`"ok"`)}, nil
	}
	<-ctx.Done()
	return gimble.TurnResult{}, ctx.Err()
}

func (a *steerAdapter) Steer(_ context.Context, _, message string) (bool, error) {
	a.mu.Lock()
	active := a.active && !a.idleMode
	a.mu.Unlock()
	if active && a.steered != nil {
		a.steered <- message
	}
	return active, nil
}

func (*steerAdapter) Fork(context.Context, string) (string, error) { return "fork", nil }
func (*steerAdapter) Close(context.Context, string) error          { return nil }
