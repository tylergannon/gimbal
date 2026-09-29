package gimbal

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tylergannon/gimbal/contextdata"
)

func TestOpenRunRecordsInitialSnapshotAndRejectsFailedEntry(t *testing.T) {
	store := &contextdata.Store{Root: t.TempDir()}
	entry, err := contextdata.Encode("input", "submitted input")
	if err != nil {
		t.Fatal(err)
	}
	initial, err := store.Extend(t.Context(), "", entry)
	if err != nil {
		t.Fatal(err)
	}
	project := t.TempDir()
	root, finish, err := OpenRun(Project(t.Context(), project), "public-entry", nil, initial, ContextAccess{Store: store})
	if err != nil {
		t.Fatal(err)
	}
	if got := scopeText(root); !strings.Contains(got, "submitted input") {
		t.Fatalf("initial observation = %q", got)
	}
	if err = finish(nil); err != nil {
		t.Fatal(err)
	}
	if _, _, err = OpenScope(root, "ended"); err == nil {
		t.Fatal("ended run accepted a scope")
	}

	// An unavailable manifest must not leak a live runtime or its finish handle.
	if err = os.Remove(filepath.Join(store.Root, "objects", string(initial))); err != nil {
		t.Fatal(err)
	}
	failed, closeFailed, err := OpenRun(Project(t.Context(), project), "failed-entry", nil, initial, ContextAccess{Store: store})
	if err == nil || failed != nil || closeFailed != nil {
		t.Fatalf("entry = %v, finish present=%v, error=%v", failed, closeFailed != nil, err)
	}
	files, err := filepath.Glob(filepath.Join(project, "runs", "*", "run.jsonl"))
	if err != nil || len(files) != 2 {
		t.Fatalf("run files=%v err=%v", files, err)
	}
	for _, file := range files {
		records := readRecords[LifecycleRecord](t, file)
		found := false
		for _, record := range records {
			if _, ok := record.Event.(RunEnded); ok {
				found = true
			}
		}
		if !found {
			t.Fatalf("entry left unfinished run %s", file)
		}
	}
}

func TestPublicPlannerTaskCheckAndResponse(t *testing.T) {
	store := &contextdata.Store{Root: t.TempDir()}
	input, _ := contextdata.Encode("input", "explicit planning input")
	initial, err := store.Extend(t.Context(), "", input)
	if err != nil {
		t.Fatal(err)
	}
	adapter := &fake{answer: func(_ context.Context, _, prompt string, _ json.RawMessage, _ func(AgentEvent) error) (string, error) {
		if !strings.Contains(prompt, "explicit planning input") {
			t.Errorf("missing snapshot input: %s", prompt)
		}
		return `{"tasks":[{"name":"work","description":"inspect","definition_of_done":"observed","validation":{"command":"","query":""}}],"next":0}`, nil
	}}
	root, finish, err := OpenRun(Project(t.Context(), t.TempDir()), "typed-planning", bind(adapter, "fake", "planner"), initial, ContextAccess{Store: store})
	if err != nil {
		t.Fatal(err)
	}
	planner := NewSession(root, "planner", t.TempDir())
	if _, err = PlanNext(root, initial, planner, "goal", nil, ""); err == nil {
		t.Fatal("planner accepted a non-loop scope")
	}
	loop, finishLoop, err := OpenLoop(root, "loop")
	if err != nil {
		t.Fatal(err)
	}
	p, err := PlanNext(loop, initial, planner, "goal", nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if !p.Next.Present || p.Next.Value != 0 {
		t.Fatalf("plan=%+v", p)
	}
	if err = RecordPlan(loop, "goal", p); err != nil {
		t.Fatal(err)
	}
	task, ref, finishTask, err := OpenTask(loop, "task", p.Tasks[0], initial)
	if err != nil {
		t.Fatal(err)
	}
	ref, err = CheckContext(task, ref, "check", t.TempDir(), "sh", "-c", "printf public-evidence; exit 7")
	if err != nil {
		t.Fatal(err)
	}
	entries, err := store.Load(t.Context(), ref)
	if err != nil {
		t.Fatal(err)
	}
	var check checkResult
	for _, entry := range entries {
		if entry.Key == "check" {
			if err = json.Unmarshal(entry.Value, &check); err != nil {
				t.Fatal(err)
			}
		}
	}
	if check.Stdout != "public-evidence" || check.ExitCode != 7 {
		t.Fatalf("check=%+v", check)
	}
	feedback, err := TaskFeedback(task)
	if err != nil || !strings.Contains(feedback, "public-evidence") || strings.Contains(feedback, "explicit planning input") {
		t.Fatalf("feedback=%q err=%v", feedback, err)
	}
	if err = finishTask(nil); err != nil {
		t.Fatal(err)
	}
	// Task snapshot initialization failure retires its partially opened scope.
	if ctx, _, closeTask, err := OpenTask(loop, "broken", p.Tasks[0], "missing"); err == nil || ctx != nil || closeTask != nil {
		t.Fatal("invalid snapshot returned a live task")
	}
	if err = finishLoop(nil); err != nil {
		t.Fatal(err)
	}
	if err = finish(nil); err != nil {
		t.Fatal(err)
	}
}

func TestPublicCancelRunRetainsFirstCause(t *testing.T) {
	root, finish, err := OpenRun(Project(t.Context(), t.TempDir()), "public-cancel", nil, "", ContextAccess{Store: &contextdata.Store{Root: t.TempDir()}})
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(runDir(root), "run.jsonl")
	first := errors.New("first cancellation")
	if err = CancelRun(root, first); err != nil {
		t.Fatal(err)
	}
	if err = CancelRun(root, errors.New("later cancellation")); err != nil {
		t.Fatal(err)
	}
	if context.Cause(root) != first {
		t.Fatalf("cause=%v", context.Cause(root))
	}
	if err = finish(first); !errors.Is(err, first) {
		t.Fatalf("finish=%v", err)
	}
	if err = CancelRun(root, first); err == nil {
		t.Fatal("ended run accepted cancellation")
	}
	kills := 0
	for _, record := range readRecords[LifecycleRecord](t, file) {
		if _, ok := record.Event.(Killed); ok {
			kills++
		}
	}
	if kills != 1 {
		t.Fatalf("kill events=%d", kills)
	}
}
