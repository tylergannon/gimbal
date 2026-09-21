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
	mu                                                                             sync.Mutex
	sessions                                                                       map[string]testSession
	arrivals                                                                       chan struct{}
	want, finished                                                                 int
	failTester, failDebrief, failVisual, incompleteVisual, rejectVisual, failClose bool
	calls                                                                          []string
	prompts                                                                        map[string][]string
	mismatchFile                                                                   string
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
	switch s.role {
	case "tester":
		if strings.HasPrefix(prompt, userPrompt) {
			h.arrivals <- struct{}{}
			// Neither tester can finish until both entered their turns: real fan-out.
			for len(h.arrivals) < h.want {
				select {
				case <-ctx.Done():
					return gimble.TurnResult{}, ctx.Err()
				case <-time.After(time.Millisecond):
				}
			}
			if h.failTester && filepath.Base(s.dir) == "a" {
				h.mu.Lock()
				h.finished++
				h.mu.Unlock()
				return gimble.TurnResult{}, errors.New("tester unavailable")
			}
			raw, _ := json.Marshal("# tester report\nObserved task outcome and limitations.")
			return gimble.TurnResult{Output: raw}, nil
		}
		if strings.HasPrefix(prompt, correctionPrompt) {
			h.mu.Lock()
			mismatchFile := h.mismatchFile
			h.mu.Unlock()
			if data, err := os.ReadFile(mismatchFile); err != nil || !strings.Contains(string(data), "caption overclaims") {
				return gimble.TurnResult{}, errors.New("visual feedback was not written before correction")
			}
			raw, _ := json.Marshal("# corrected tester report\nCaption now matches the visible screenshot.\n\n# UI/UX debrief\nConcrete preferences from the completed task.")
			return gimble.TurnResult{Output: raw}, nil
		}
		if strings.HasPrefix(prompt, experiencePrompt) {
			h.mu.Lock()
			h.finished++
			h.mu.Unlock()
			if h.failDebrief && filepath.Base(s.dir) == "a" {
				return gimble.TurnResult{}, errors.New("debrief unavailable")
			}
			raw, _ := json.Marshal("# UI/UX debrief\nConcrete preferences from the completed task.")
			return gimble.TurnResult{Output: raw}, nil
		}
		return gimble.TurnResult{}, errors.New("unexpected tester prompt")
	case "visual":
		if h.failVisual && filepath.Base(s.dir) == "tester1" {
			return gimble.TurnResult{}, errors.New("image tool unavailable")
		}
		verdict := VisualVerdict{ReviewCompleted: true, Supported: true, OpenedImages: []string{filepath.Join(s.dir, "01.png")}, Feedback: "Every visible claim is supported."}
		if h.incompleteVisual && filepath.Base(s.dir) == "tester1" {
			verdict.ReviewCompleted = false
			verdict.Supported = false
			verdict.OpenedImages = []string{}
			verdict.Feedback = "Could not open 01.png."
		} else if h.rejectVisual && filepath.Base(s.dir) == "tester1" && s.turns == 1 {
			verdict.Supported = false
			verdict.Feedback = "The caption overclaims what 01.png visibly shows."
			h.mu.Lock()
			h.mismatchFile = filepath.Join(s.dir, "visual-review.md")
			h.mu.Unlock()
		}
		raw, _ := json.Marshal(verdict)
		return gimble.TurnResult{Output: raw}, nil
	case "triage":
		h.mu.Lock()
		finished := h.finished
		h.mu.Unlock()
		if finished != h.want {
			return gimble.TurnResult{}, errors.New("triage ran before all testers finished")
		}
		raw, _ := json.Marshal("# triage report\nObserved task outcome and limitations.")
		return gimble.TurnResult{Output: raw}, nil
	}
	return gimble.TurnResult{}, errors.New("unexpected role")
}

func TestUserTestingStages(t *testing.T) {
	for _, tc := range []struct {
		name                                                 string
		n                                                    int
		tester, debrief, visual, incomplete, mismatch, close bool
	}{
		{name: "two parallel workloads", n: 2},
		{name: "unused slots skip", n: 1},
		{name: "all three testers get a debrief", n: 3},
		{name: "visual mismatch gets one correction and recheck", n: 1, mismatch: true},
		{name: "debrief failure preserves task report", n: 2, debrief: true},
		{name: "tester failure still reaches triage", n: 2, tester: true},
		{name: "visual failure still reaches triage", n: 2, visual: true},
		{name: "unopenable screenshot is an evidence error", n: 1, incomplete: true},
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
			h := &testingHarness{sessions: map[string]testSession{}, arrivals: make(chan struct{}, 3), want: tc.n, failTester: tc.tester, failDebrief: tc.debrief, failVisual: tc.visual, incompleteVisual: tc.incomplete, rejectVisual: tc.mismatch, failClose: tc.close, prompts: map[string][]string{}}
			models := map[gimble.WorkflowRole]gimble.ModelBinding{"product-operation": {Adapter: h, Model: "tester"}, "product-visual-review": {Adapter: h, Model: "visual"}, "product-triage": {Adapter: h, Model: "triage"}}
			err := gimble.Run(gimble.Project(t.Context(), t.TempDir()), "user-testing", models, func(ctx context.Context) error {
				return ValidateProduct(ctx, gimble.Env{WorkDir: filepath.Dir(input)}, Params{SuiteFile: input})
			})
			if (err != nil) != (tc.tester || tc.debrief || tc.visual || tc.incomplete || tc.close) {
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
			visualCalls := tc.n
			if tc.tester {
				visualCalls--
			}
			if tc.mismatch {
				testerCalls++
				visualCalls++
			}
			counts := map[string]int{}
			for _, role := range h.calls {
				counts[role]++
			}
			if counts["tester"] != testerCalls || counts["visual"] != visualCalls || counts["triage"] != 1 || h.calls[len(h.calls)-1] != "triage" {
				t.Fatalf("stage calls: %v", h.calls)
			}
			for _, session := range h.sessions {
				if session.role != "tester" {
					continue
				}
				want := 2
				if tc.tester && filepath.Base(session.dir) == "a" {
					want = 1
				} else if tc.mismatch && filepath.Base(session.dir) == "a" {
					want = 3
				}
				if session.turns != want {
					t.Fatalf("session lost continuity: %+v", session)
				}
			}
			wantSessions := tc.n*2 + 1
			if tc.tester {
				wantSessions--
			}
			if len(h.sessions) != wantSessions {
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
				if (r.Error != "") != ((tc.tester || tc.debrief || tc.visual || tc.incomplete) && i == 0) {
					t.Fatalf("wrong execution error: %+v", r)
				}
				body, readErr := os.ReadFile(r.Report)
				if readErr != nil {
					t.Fatal(readErr)
				}
				wantReport := "# tester report"
				if tc.mismatch && i == 0 {
					wantReport = "# corrected tester report"
				}
				if (!tc.tester || i != 0) && !strings.Contains(string(body), wantReport) {
					t.Fatal("lost original task report")
				}
				wantDebrief := !tc.tester && !tc.debrief || i != 0
				if strings.Contains(string(body), "# UI/UX debrief") != wantDebrief {
					t.Fatalf("wrong debrief content: %s", body)
				}
				visualBody, err := os.ReadFile(r.VisualReview)
				if err != nil {
					t.Fatal(err)
				}
				if tc.mismatch && i == 0 && (!strings.Contains(string(visualBody), "## Round 2") || !strings.Contains(string(visualBody), "Claims supported: true")) {
					t.Fatalf("correction was not rechecked: %s", visualBody)
				}
			}
			if _, err := os.Stat(filepath.Join(filepath.Dir(paths[0]), "findings.md")); err != nil {
				t.Fatal(err)
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
			if tc.incomplete && !strings.Contains(h.prompts["triage"][0], "screenshot review incomplete") {
				t.Fatal("triage lost unopenable screenshot evidence error")
			}
			if tc.mismatch {
				var correction string
				for _, prompt := range h.prompts["tester"] {
					if strings.HasPrefix(prompt, correctionPrompt) {
						correction = prompt
					}
				}
				if !strings.Contains(correction, "screenshot review") {
					t.Fatal("tester did not receive the local screenshot-review handoff")
				}
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
	if !strings.Contains(visualPrompt, "Open every screenshot") || !strings.Contains(visualPrompt, "visible contents support") {
		t.Fatal("visual reviewer is not grounded in the cited image files")
	}
	if !strings.Contains(correctionPrompt, "screenshot review named in the context") || !strings.Contains(correctionPrompt, "complete replacement Markdown user report") {
		t.Fatal("correction turn does not receive the visual verdict or replace the report")
	}
	if !strings.Contains(triagePrompt, "final screenshot-review verdict") || !strings.Contains(triagePrompt, "reference it found incorrect") {
		t.Fatal("triage prompt does not preserve independent screenshot corrections")
	}
}
