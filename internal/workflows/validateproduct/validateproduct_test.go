package validateproduct

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tylergannon/gimble"
	"github.com/tylergannon/gimble/workflow"
)

type testSession struct{ role, dir string }
type testingHarness struct {
	mu                                sync.Mutex
	sessions                          map[string]testSession
	arrivals                          chan struct{}
	want, finished                    int
	failTester, failVisual, failClose bool
	calls                             []string
	prompts                           map[string][]string
}

func (h *testingHarness) CreateSession(_ context.Context, role, _ string, dir string) (string, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	id := role + "-" + dir
	h.sessions[id] = testSession{role, dir}
	return id, nil
}
func (*testingHarness) Fork(context.Context, string) (string, error) {
	return "", errors.New("unexpected fork")
}
func (*testingHarness) Steer(context.Context, string, string) (bool, error) { return false, nil }
func (h *testingHarness) Close(context.Context, string) error {
	if h.failClose {
		return errors.New("injected close failure")
	}
	return nil
}
func (h *testingHarness) RunTurn(ctx context.Context, id, prompt string, _ json.RawMessage, _ func(gimble.AgentEvent) error) (gimble.TurnResult, error) {
	h.mu.Lock()
	s := h.sessions[id]
	h.calls = append(h.calls, s.role)
	h.prompts[s.role] = append(h.prompts[s.role], prompt)
	h.mu.Unlock()
	if s.role == "tester" {
		h.arrivals <- struct{}{}
		// Neither tester can finish until both entered their turns: real fan-out.
		for len(h.arrivals) < h.want {
			select {
			case <-ctx.Done():
				return gimble.TurnResult{}, ctx.Err()
			case <-time.After(time.Millisecond):
			}
		}
		h.mu.Lock()
		h.finished++
		h.mu.Unlock()
		if h.failTester && filepath.Base(s.dir) == "a" {
			return gimble.TurnResult{}, errors.New("tester unavailable")
		}
	} else {
		h.mu.Lock()
		finished := h.finished
		h.mu.Unlock()
		if finished != h.want {
			return gimble.TurnResult{}, errors.New("review ran before all testers finished")
		}
		if s.role == "visual" && h.failVisual {
			return gimble.TurnResult{}, errors.New("image tool unavailable")
		}
	}
	raw, _ := json.Marshal("# " + s.role + " report\nObserved task outcome and limitations.")
	return gimble.TurnResult{Output: raw}, nil
}

func TestUserTestingStages(t *testing.T) {
	for _, tc := range []struct {
		name                  string
		n                     int
		tester, visual, close bool
	}{
		{name: "two parallel workloads", n: 2},
		{name: "unused slots skip", n: 1},
		{name: "tester failure still reaches triage", n: 2, tester: true},
		{name: "visual failure still reaches triage", n: 2, visual: true},
		{name: "cleanup failure reaches run outcome", n: 1, close: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, input := suiteFixture(t, tc.n)
			s.Timeout = "10s"
			s.PlaywrightCLI = filepath.Join(filepath.Dir(input), "browser")
			// Browser lifecycle only; agent calls and the workflow runtime are real.
			if err := os.WriteFile(s.PlaywrightCLI, []byte("#!/bin/sh\nif [ \"$2\" = video-start ]; then printf video > \"$3\"; fi\n"), 0700); err != nil {
				t.Fatal(err)
			}
			saveSuite(t, s, input)
			h := &testingHarness{sessions: map[string]testSession{}, arrivals: make(chan struct{}, 3), want: tc.n, failTester: tc.tester, failVisual: tc.visual, failClose: tc.close, prompts: map[string][]string{}}
			models := map[gimble.WorkflowRole]gimble.ModelBinding{"product-operation": {Adapter: h, Model: "tester"}, "product-visual-review": {Adapter: h, Model: "visual"}, "product-triage": {Adapter: h, Model: "triage"}}
			err := gimble.Run(gimble.Project(t.Context(), t.TempDir()), "user-testing", models, func(ctx context.Context) error {
				return ValidateProduct(ctx, gimble.Env{WorkDir: filepath.Dir(input)}, Params{SuiteFile: input})
			})
			if (err != nil) != (tc.tester || tc.visual || tc.close) {
				t.Fatalf("run error: %v", err)
			}
			if tc.close {
				if _, ok := errors.AsType[*gimble.CloseError](err); !ok {
					t.Fatalf("want CloseError: %v", err)
				}
			}
			if len(h.calls) != tc.n+2 || h.calls[tc.n] != "visual" || h.calls[tc.n+1] != "triage" {
				t.Fatalf("stage order: %v", h.calls)
			}
			paths, _ := filepath.Glob(filepath.Join(s.OutputDir, "user-testing-*", "reports.json"))
			if len(paths) != 1 {
				t.Fatalf("reports: %v", paths)
			}
			data, err := os.ReadFile(paths[0])
			if err != nil {
				t.Fatal(err)
			}
			var reports []workloadReport
			if err := json.Unmarshal(data, &reports); err != nil {
				t.Fatal(err)
			}
			if len(reports) != tc.n {
				t.Fatalf("reports = %+v", reports)
			}
			for i, r := range reports {
				if r.ElapsedSeconds <= 0 {
					t.Fatal("elapsed time not measured")
				}
				if (r.Error != "") != (tc.tester && i == 0) {
					t.Fatalf("wrong execution error: %+v", r)
				}
			}
			for _, file := range []string{"visual-review.md", "findings.md"} {
				if _, err := os.Stat(filepath.Join(filepath.Dir(paths[0]), file)); err != nil {
					t.Fatal(err)
				}
			}
			for _, p := range h.prompts["tester"] {
				if !strings.Contains(p, "Never inspect the source code of the product under test (A)") || !strings.Contains(p, "screenshots directory") {
					t.Fatal("tester missing user boundary or capture context")
				}
			}
			if tc.visual && !strings.Contains(h.prompts["triage"][0], "image tool unavailable") {
				t.Fatal("triage lost visual-review failure")
			}
		})
	}
}

func TestStaticWorkflowGraph(t *testing.T) {
	if len(Graph.Diagnostics) != 0 {
		t.Fatalf("graph diagnostics: %+v", Graph.Diagnostics)
	}
	var found bool
	for _, op := range Graph.Body {
		if group, ok := op.(workflow.Group); ok && group.Name == "user-testing" {
			found = true
			if len(group.Children) != 3 {
				t.Fatalf("children: %+v", group.Children)
			}
			for i, name := range []string{"tester1", "tester2", "tester3"} {
				if group.Children[i].Name != name {
					t.Fatalf("slot %d: %s", i, group.Children[i].Name)
				}
			}
		}
	}
	if !found {
		t.Fatal("missing explicit tester group")
	}
}
