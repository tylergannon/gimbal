package gimbal

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/tylergannon/gimbal/internal/compiledscope"
)

// Exercise the same scope seam used by worker activities, including a parent
// session used inside children and again after their explicit cleanup.
func TestCompiledScopeLayeringAndResourceOwnership(t *testing.T) {
	for _, mode := range []string{"success", "error", "cancel"} {
		t.Run(mode, func(t *testing.T) {
			var prompts, nativeIDs []string
			adapter := &fake{answer: func(_ context.Context, id, prompt string, _ json.RawMessage, _ func(AgentEvent) error) (string, error) {
				prompts = append(prompts, prompt)
				nativeIDs = append(nativeIDs, id)
				return "done", nil
			}}
			project := t.TempDir()
			root, finish, err := compiledscope.OpenRun(Project(t.Context(), project), "nested", bind(adapter, "m", "worker"))
			if err != nil {
				t.Fatal(err)
			}
			Set(root, "layer", "root-layer")
			session := NewSession(root, "worker", project)
			generate := func(ctx context.Context) {
				t.Helper()
				if _, err := session.Generate[Text](ctx, "Inspect context."); err != nil {
					t.Fatal(err)
				}
			}
			generate(root)
			outer, closeOuter, err := compiledscope.OpenScope(root, "context")
			if err != nil {
				t.Fatal(err)
			}
			Set(outer, "layer", "outer-layer")
			Set(outer, "inherited", "outer-inherited")
			generate(outer)
			innerParent, cancel := context.WithCancel(outer)
			inner, closeInner, err := compiledscope.OpenScope(innerParent, "details")
			if err != nil {
				t.Fatal(err)
			}
			Set(inner, "layer", "inner-layer")
			Set(inner, "child-only", "inner-private")
			generate(inner)
			childSession := NewSession(inner, "worker", project)
			if _, err := childSession.Generate[Text](inner, "Inspect child."); err != nil {
				t.Fatal(err)
			}
			var reason error
			switch mode {
			case "error":
				reason = errors.New("inner failed")
			case "cancel":
				cancel()
				reason = context.Canceled
			}
			if got := closeInner(reason); !errors.Is(got, reason) {
				t.Fatalf("inner close=%v", got)
			}
			cancel()
			if inner.Err() == nil || outer.Err() != nil || root.Err() != nil {
				t.Fatal("child cleanup cancelled an ancestor or left child alive")
			}
			if !slices.Equal(adapter.closed, []string{"native-2"}) {
				t.Fatalf("closed=%v", adapter.closed)
			}
			generate(outer)
			if err := closeOuter(nil); err != nil {
				t.Fatal(err)
			}
			generate(root)
			if len(adapter.closed) != 1 {
				t.Fatal("parent-owned session closed with child")
			}
			if err := finish(nil); err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(adapter.closed, []string{"native-2", "native-1"}) {
				t.Fatalf("close order=%v", adapter.closed)
			}
			if !slices.Equal(nativeIDs, []string{"native-1", "native-1", "native-1", "native-2", "native-1", "native-1"}) {
				t.Fatalf("session continuity=%v", nativeIDs)
			}
			if prompts[0] != prompts[5] || prompts[1] != prompts[4] {
				t.Fatal("child values leaked into resumed parent prompts")
			}
			if !strings.Contains(prompts[2], "inner-layer") || !strings.Contains(prompts[2], "outer-inherited") || !strings.Contains(prompts[2], "inner-private") || strings.Contains(prompts[2], "outer-layer") || strings.Contains(prompts[2], "root-layer") {
				t.Fatalf("bad effective inner context: %s", prompts[2])
			}
			paths, _ := filepath.Glob(filepath.Join(project, "runs", "*", "run.jsonl"))
			bytes, err := os.ReadFile(paths[0])
			if err != nil {
				t.Fatal(err)
			}
			var ends []string
			for line := range strings.SplitSeq(string(bytes), "\n") {
				if line == "" {
					continue
				}
				var record struct {
					Scope string
					Event struct{ Kind string }
				}
				if err := json.Unmarshal([]byte(line), &record); err != nil {
					t.Fatal(err)
				}
				if record.Event.Kind == "scope_ended" {
					ends = append(ends, record.Scope)
				}
			}
			if !slices.Equal(ends, []string{"context.1/details.1", "context.1", ""}) {
				t.Fatalf("scope end order=%v", ends)
			}
		})
	}
}

func TestCompiledRunPreservesCleanupFailure(t *testing.T) {
	closeErr := errors.New("adapter close failed")
	adapter := &fake{answer: func(context.Context, string, string, json.RawMessage, func(AgentEvent) error) (string, error) {
		return "done", nil
	}, closeErr: func(string) error { return closeErr }}
	root, finish, err := compiledscope.OpenRun(Project(t.Context(), t.TempDir()), "cleanup", bind(adapter, "m", "worker"))
	if err != nil {
		t.Fatal(err)
	}
	session := NewSession(root, "worker", t.TempDir())
	if _, err := session.Generate[Text](root, "Do work."); err != nil {
		t.Fatal(err)
	}
	if err := finish(nil); !errors.Is(err, closeErr) {
		t.Fatalf("lost cleanup failure: %v", err)
	}
}
