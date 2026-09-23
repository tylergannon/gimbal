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

type testSession struct {
	role, dir string
	turns     int
}
type testingHarness struct {
	mu                                             sync.Mutex
	sessions                                       map[string]testSession
	arrivals                                       chan struct{}
	want, finished                                 int
	failTester, failDebrief, failVisual, failClose bool
	calls                                          []string
	prompts                                        map[string][]string
}

func (h *testingHarness) CreateSession(_ context.Context, role, _ string, dir string) (string, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	id := role + "-" + dir
	h.sessions[id] = testSession{role: role, dir: dir}
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
	s.turns++
	h.sessions[id] = s
	h.calls = append(h.calls, s.role)
	h.prompts[s.role] = append(h.prompts[s.role], prompt)
	h.mu.Unlock()
	if s.role == "tester" {
		if s.turns == 1 {
			h.arrivals <- struct{}{}
			// Neither tester can finish until both entered their turns: real fan-out.
			for len(h.arrivals) < h.want {
				select {
				case <-ctx.Done():
					return gimble.TurnResult{}, ctx.Err()
				case <-time.After(time.Millisecond):
				}
			}
		}
		failedTask := s.turns == 1 && h.failTester && filepath.Base(s.dir) == "a"
		if s.turns == 2 || failedTask {
			h.mu.Lock()
			h.finished++
			h.mu.Unlock()
		}
		if failedTask {
			return gimble.TurnResult{}, errors.New("tester unavailable")
		}
		if s.turns == 2 {
			if h.failDebrief && filepath.Base(s.dir) == "a" {
				return gimble.TurnResult{}, errors.New("debrief unavailable")
			}
			raw, _ := json.Marshal("# UI/UX debrief\nConcrete preferences from the completed task.")
			return gimble.TurnResult{Output: raw}, nil
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
		name                           string
		n                              int
		tester, debrief, visual, close bool
	}{
		{name: "two parallel workloads", n: 2},
		{name: "unused slots skip", n: 1},
		{name: "all three testers get a debrief", n: 3},
		{name: "debrief failure preserves task report", n: 2, debrief: true},
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
			h := &testingHarness{sessions: map[string]testSession{}, arrivals: make(chan struct{}, 3), want: tc.n, failTester: tc.tester, failDebrief: tc.debrief, failVisual: tc.visual, failClose: tc.close, prompts: map[string][]string{}}
			models := map[gimble.WorkflowRole]gimble.ModelBinding{"product-operation": {Adapter: h, Model: "tester"}, "product-visual-review": {Adapter: h, Model: "visual"}, "product-triage": {Adapter: h, Model: "triage"}}
			err := gimble.Run(gimble.Project(t.Context(), t.TempDir()), "user-testing", models, func(ctx context.Context) error {
				return ValidateProduct(ctx, gimble.Env{WorkDir: filepath.Dir(input)}, Params{SuiteFile: input})
			})
			if (err != nil) != (tc.tester || tc.debrief || tc.visual || tc.close) {
				t.Fatalf("run error: %v", err)
			}
			if tc.close {
				if _, ok := errors.AsType[*gimble.CloseError](err); !ok {
					t.Fatalf("want CloseError: %v", err)
				}
			}
			testerCalls := tc.n * 2
			if tc.tester {
				testerCalls--
			}
			if len(h.calls) != testerCalls+2 || h.calls[testerCalls] != "visual" || h.calls[testerCalls+1] != "triage" {
				t.Fatalf("stage order: %v", h.calls)
			}
			for _, session := range h.sessions {
				if session.role != "tester" {
					continue
				}
				want := 2
				if tc.tester && filepath.Base(session.dir) == "a" {
					want = 1
				}
				if session.turns != want {
					t.Fatalf("session lost continuity: %+v", session)
				}
			}
			if len(h.sessions) != tc.n+2 {
				t.Fatalf("unexpected extra sessions: %+v", h.sessions)
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
				if (r.Error != "") != ((tc.tester || tc.debrief) && i == 0) {
					t.Fatalf("wrong execution error: %+v", r)
				}
				body, readErr := os.ReadFile(r.Report)
				if readErr != nil {
					t.Fatal(readErr)
				}
				if (!tc.tester || i != 0) && !strings.Contains(string(body), "# tester report") {
					t.Fatal("lost original task report")
				}
				wantDebrief := !tc.tester && !tc.debrief || i != 0
				if strings.Contains(string(body), "# UI/UX debrief") != wantDebrief {
					t.Fatalf("wrong debrief content: %s", body)
				}
			}
			for _, file := range []string{"visual-review.md", "findings.md"} {
				if _, err := os.Stat(filepath.Join(filepath.Dir(paths[0]), file)); err != nil {
					t.Fatal(err)
				}
			}
			for _, p := range h.prompts["tester"] {
				if !strings.HasPrefix(p, userPrompt) {
					continue
				}
				if !strings.Contains(p, "Never inspect the source code of the product under test (A)") || !strings.Contains(p, "screenshots directory") {
					t.Fatal("tester missing user boundary or capture context")
				}
			}
			if tc.debrief && !strings.Contains(h.prompts["triage"][0], "debrief unavailable") {
				t.Fatal("triage lost debrief failure")
			}
			if tc.visual && !strings.Contains(h.prompts["triage"][0], "image tool unavailable") {
				t.Fatal("triage lost visual-review failure")
			}
			if !strings.Contains(h.prompts["triage"][0], "example/product-a") {
				t.Fatal("triage lost the required issue destination")
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

func TestScreenshotClaimsStayGrounded(t *testing.T) {
	for name, prompt := range map[string]string{
		"tester":     userPrompt,
		"experience": experiencePrompt,
	} {
		if !strings.Contains(prompt, "reopen") && !strings.Contains(prompt, "open that exact image") {
			t.Fatalf("%s prompt does not require inspecting the saved image", name)
		}
		if !strings.Contains(prompt, "exact path") || !strings.Contains(prompt, "visibly support") {
			t.Fatalf("%s prompt does not ground claims and references in the saved image", name)
		}
	}
	if !strings.Contains(triagePrompt, "screenshot review's corrections") || !strings.Contains(triagePrompt, "reference it found incorrect") {
		t.Fatal("triage prompt does not preserve independent screenshot corrections")
	}
	if !strings.Contains(triagePrompt, "gimble upload-artifact") || !strings.Contains(triagePrompt, "hosted images") {
		t.Fatal("triage prompt does not require online screenshot evidence for issues")
	}
}
