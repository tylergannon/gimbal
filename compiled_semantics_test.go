package gimbal

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tylergannon/gimbal/internal/compiledscope"
)

func TestCompiledCheckCanonicalRecordAndPreconditions(t *testing.T) {
	store := compiledscope.Store{Root: t.TempDir()}
	err := runTest(t, nil, func(ctx context.Context) error {
		if err := compiledscope.InitializeContext(ctx, store, store.LocalDir, ""); err != nil {
			return err
		}
		ref, result, err := compiledscope.CheckContext(ctx, "", "check", ".", "sh", "-c", "printf evidence; exit 7")
		if err != nil {
			return err
		}
		absolute, _ := filepath.Abs(".")
		if result.Workdir != absolute || result.ExitCode != 7 || result.Stdout != "evidence" {
			t.Fatalf("result=%+v", result)
		}
		entries, err := store.Load(t.Context(), ref)
		if err != nil {
			return err
		}
		if len(entries) != 1 || !strings.Contains(string(entries[0].Value), absolute) {
			t.Fatal("canonical record missing")
		}
		marker := filepath.Join(t.TempDir(), "must-not-execute")
		unchanged, _, err := compiledscope.CheckContext(ctx, ref, "check", ".", "touch", marker)
		if err == nil || unchanged != ref {
			t.Fatal("duplicate write accepted")
		}
		if _, err = os.Stat(marker); !os.IsNotExist(err) {
			t.Fatal("duplicate key command executed")
		}
		failed, record, err := compiledscope.CheckContext(ctx, ref, "start-failure", ".", "/nonexistent/gimbal-command")
		if err == nil || failed == ref || record.Error == "" {
			t.Fatal("execution failure did not publish evidence")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestCompiledTaskFeedbackUsesRecordedLocalWrites(t *testing.T) {
	store := compiledscope.Store{Root: t.TempDir()}
	err := runTest(t, nil, func(ctx context.Context) error {
		if err := compiledscope.InitializeContext(ctx, store, store.LocalDir, ""); err != nil {
			return err
		}
		root, err := compiledscope.WriteContext(ctx, "", compiledscope.Entry{Key: "shadow", Value: json.RawMessage(`"same"`)}, compiledscope.Entry{Key: "parent", Value: json.RawMessage(`"not feedback"`)})
		if err != nil {
			return err
		}
		loop, finishLoop, err := compiledscope.OpenLoop(ctx, "loop")
		if err != nil {
			return err
		}
		raw := []byte(`{"name":"work","description":"do work","definition_of_done":"done","validation":{"command":"","query":""}}`)
		task, ref, finishTask, err := compiledscope.OpenTask(loop, "task", raw, root)
		if err != nil {
			return err
		}
		_, err = compiledscope.WriteContext(task, ref, compiledscope.Entry{Key: "shadow", Value: json.RawMessage(`"same"`)})
		if err != nil {
			return err
		}
		// An inherited binding must not make feedback include parent-only data.
		bound, err := compiledscope.BindContext(task, root)
		if err != nil {
			return err
		}
		feedback, err := compiledscope.TaskFeedback(bound)
		if err != nil {
			return err
		}
		if !strings.Contains(feedback, "## task") || !strings.Contains(feedback, "## shadow") || strings.Contains(feedback, "## parent") {
			t.Fatalf("feedback=%s", feedback)
		}
		if err = finishTask(nil); err != nil {
			return err
		}
		return finishLoop(nil)
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestCompiledLifecycleAndSessionReachability(t *testing.T) {
	adapter := &fake{answer: func(context.Context, string, string, json.RawMessage, func(AgentEvent) error) (string, error) {
		return "ok", nil
	}}
	root, finish, err := compiledscope.OpenRun(Project(t.Context(), t.TempDir()), "legality", bind(adapter, "m", "worker"))
	if err != nil {
		t.Fatal(err)
	}
	left, finishLeft, err := compiledscope.OpenScope(root, "left")
	if err != nil {
		t.Fatal(err)
	}
	right, finishRight, err := compiledscope.OpenScope(root, "right")
	if err != nil {
		t.Fatal(err)
	}
	session := NewSession(left, "worker", ".")
	if _, err = session.Generate[Text](right, "No sibling access."); err == nil {
		t.Fatal("sibling session use accepted")
	}
	if _, err = session.Fork(root, "escaped"); err == nil {
		t.Fatal("child session escaped through Fork")
	}
	if err = finish(nil); err == nil {
		t.Fatal("root ended before children")
	}
	if _, err = session.Generate[Text](left, "Still active."); err != nil {
		t.Fatal(err)
	}
	if err = finishLeft(nil); err != nil {
		t.Fatal(err)
	}
	if _, _, err = compiledscope.OpenScope(left, "late"); err == nil {
		t.Fatal("entered ended scope")
	}
	if err = finishLeft(nil); err == nil {
		t.Fatal("duplicate finish accepted")
	}
	if err = finishRight(nil); err != nil {
		t.Fatal(err)
	}
	if err = finish(nil); err != nil {
		t.Fatal(err)
	}
}

func TestCompiledGenerateExplicitEmptySnapshot(t *testing.T) {
	store := compiledscope.Store{Root: t.TempDir()}
	adapter := &fake{answer: func(_ context.Context, _, prompt string, _ json.RawMessage, _ func(AgentEvent) error) (string, error) {
		if strings.Contains(prompt, "ambient secret") {
			t.Fatal("empty snapshot fell back to live context")
		}
		return "ok", nil
	}}
	err := runTest(t, bind(adapter, "m", "worker"), func(ctx context.Context) error {
		if err := compiledscope.InitializeContext(ctx, store, store.LocalDir, ""); err != nil {
			return err
		}
		Set(ctx, "secret", "ambient secret")
		session := NewSession(ctx, "worker", ".")
		_, err := compiledscope.Generate[Text](ctx, session, "", "Read explicit input.")
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
}

type unavailableObjects struct {
	compiledscope.Objects
	fail bool
}

func (s *unavailableObjects) Put(ctx context.Context, id string, data []byte) error {
	if s.fail {
		return errors.New("storage unavailable")
	}
	return s.Objects.Put(ctx, id, data)
}
func TestCompiledCheckPublicationFailurePreservesInput(t *testing.T) {
	objects := &unavailableObjects{Objects: compiledscope.FileObjects{Root: t.TempDir()}}
	store := compiledscope.Store{Objects: objects, LocalDir: t.TempDir()}
	initial, err := store.Extend(t.Context(), "", compiledscope.Entry{Key: "original", Value: json.RawMessage(`"retained"`)})
	if err != nil {
		t.Fatal(err)
	}
	objects.fail = true
	err = runTest(t, nil, func(ctx context.Context) error {
		if err := compiledscope.InitializeContext(ctx, store, store.LocalDir, ""); err != nil {
			return err
		}
		ref, record, err := compiledscope.CheckContext(ctx, initial, "check", ".", "sh", "-c", "printf completed")
		if err == nil || ref != initial || record.Stdout != "completed" {
			t.Fatalf("ref=%q record=%+v err=%v", ref, record, err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
