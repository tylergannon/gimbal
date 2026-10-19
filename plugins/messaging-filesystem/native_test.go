package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/backend"
	"github.com/tylergannon/gimbal/internal/effects"
	"github.com/tylergannon/gimbal/internal/taskdaemon"
)

// Opt in because this exercises the authenticated native Codex harness.
func TestNativeMessagingWorkflowInputsContinuityAndCleanup(t *testing.T) {
	if os.Getenv("GIMBAL_MESSAGING_NATIVE_TEST") != "1" {
		t.Skip("set GIMBAL_MESSAGING_NATIVE_TEST=1 for authenticated Codex parity")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	raw, _ := json.Marshal(config{Directory: t.TempDir()})
	bus, err := connect(ctx, raw, backend.Dependencies{})
	if err != nil {
		t.Fatal(err)
	}
	workerCtx, stop := context.WithCancel(ctx)
	defer stop()
	done := make(chan error, 1)
	go func() {
		done <- worker(workerCtx, raw, backend.Dependencies{}, taskdaemon.New(t.TempDir()).Messages(bus))
	}()
	defer func() { stop(); <-done }()
	token := "input-" + ulid.Make().String()
	project := t.TempDir()
	runID := "native-" + ulid.Make().String()
	ctx = effects.WithRunID(ctx, runID)
	ctx = effects.WithConnection(gimbal.Project(ctx, project), backend.Connection{Messages: bus})
	h := &taskdaemon.Harness{Name: "codex", Connection: backend.Connection{Messages: bus}}
	err = gimbal.Run(ctx, "native-messaging", map[gimbal.WorkflowRole]gimbal.ModelBinding{"agent": {Adapter: h, Model: "gpt-5.6-luna"}}, func(ctx context.Context) error {
		session := gimbal.NewSession(ctx, "agent", project)
		first, err := session.Generate[gimbal.Text](gimbal.WithTaskFiles(ctx, backend.TaskFile{Name: "unique", Bytes: []byte(token)}), "Read the supplied task file unique. Remember its entire content for later. Return exactly that content, with no quotes or explanation.")
		if err != nil {
			return err
		}
		if string(first) != token {
			t.Errorf("native input use: %q want %q", first, token)
		}
		second, err := session.Generate[gimbal.Text](ctx, "Return exactly the entire file content you remembered in the previous turn, with no quotes or explanation.")
		if err != nil {
			return err
		}
		if string(second) != token {
			t.Errorf("session continuity: %q want %q", second, token)
		}
		return gimbal.Service(ctx, "scoped-service", project, "printf ready; sleep 120")
	})
	if err != nil {
		t.Fatal(err)
	}
	stopObserved := errors.New("service completion observed")
	err = bus.ListenEvents(ctx, backend.MessageSelection{Run: runID}, time.Time{}, func(e backend.MessageEvent) error {
		if e.Kind == "completed" && e.Result != nil && e.Result.Process != nil {
			if !e.Result.Process.Interrupted {
				t.Errorf("service cessation not confirmed: %+v", e.Result.Process)
			}
			return stopObserved
		}
		return nil
	})
	if !errors.Is(err, stopObserved) {
		t.Fatal(err)
	}
	t.Log("gpt-5.6-luna: exact memory-only task bytes read, typed Text accepted twice on one native session, scoped shell service confirmed interrupted")
}
