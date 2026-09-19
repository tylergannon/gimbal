package validateproduct

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tylergannon/gimble"
)

// The real workflow and runtime own scopes, recording cleanup, and reports.
type validationHarness struct {
	dir                                 string
	status                              string
	emptyOutput, badCitation, closeFail bool
	prompts                             []string
}

func (h *validationHarness) CreateSession(_ context.Context, _, _ string, dir string) (string, error) {
	h.dir = dir
	return "operator", nil
}
func (*validationHarness) Fork(context.Context, string) (string, error) {
	return "", errors.New("unexpected fork")
}
func (*validationHarness) Steer(context.Context, string, string) (bool, error) { return false, nil }
func (h *validationHarness) Close(context.Context, string) error {
	if h.closeFail {
		return errors.New("injected close failure")
	}
	return nil
}
func (h *validationHarness) RunTurn(_ context.Context, _, prompt string, _ json.RawMessage, _ func(gimble.AgentEvent) error) (gimble.TurnResult, error) {
	h.prompts = append(h.prompts, prompt)
	path := filepath.Join(h.dir, "stdout.txt")
	content := []byte("observed behavior")
	if h.emptyOutput {
		content = nil
	}
	if err := os.WriteFile(path, content, 0600); err != nil {
		return gimble.TurnResult{}, err
	}
	verdict := Verdict{Status: h.status, Reason: "what the agent actually observed", EvidenceFiles: []string{path}}
	if h.badCitation {
		verdict.EvidenceFiles = append(verdict.EvidenceFiles, filepath.Join(h.dir, "..", "report.json"))
	}
	raw, err := json.Marshal(verdict)
	return gimble.TurnResult{Output: raw}, err
}

func TestWorkflowEvidenceAndCleanupOutcome(t *testing.T) {
	for _, tc := range []struct {
		name, status                                      string
		emptyOutput, badCitation, closeFail, missingVideo bool
	}{
		{name: "success", status: "pass"},
		{name: "empty command output is valid", status: "pass", emptyOutput: true},
		{name: "failure survives bad attachment", status: "fail", badCitation: true},
		{name: "foreign attachment prevents overall success", status: "pass", badCitation: true},
		{name: "unable to exercise", status: "blocked"},
		{name: "agent cleanup fails", status: "pass", closeFail: true},
		{name: "recording missing", status: "pass", missingVideo: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			driver := filepath.Join(dir, "driver")
			body := "#!/bin/sh\nif [ \"$2\" = video-start ]; then printf 'fixture video' > \"$3\"; fi\n"
			if tc.missingVideo {
				body = "#!/bin/sh\nexit 0\n"
			}
			if err := os.WriteFile(driver, []byte(body), 0700); err != nil {
				t.Fatal(err)
			}
			suite := Suite{Product: Product{Name: "fixture", Workdir: dir, BrowserURL: "http://127.0.0.1:1"}, Tools: Tools{PlaywrightCLI: driver}, OutputDir: filepath.Join(dir, "output"), Features: []Feature{{ID: "feature", Surface: "browser", Exercise: "observe", Expected: "expected behavior"}}}
			data, err := json.Marshal(suite)
			if err != nil {
				t.Fatal(err)
			}
			input := filepath.Join(dir, "suite.json")
			if err := os.WriteFile(input, data, 0600); err != nil {
				t.Fatal(err)
			}
			h := &validationHarness{status: tc.status, emptyOutput: tc.emptyOutput, badCitation: tc.badCitation, closeFail: tc.closeFail}
			models := map[gimble.WorkflowRole]gimble.ModelBinding{"product-operation": {Adapter: h, Model: "operator"}}
			runErr := gimble.Run(gimble.Project(t.Context(), t.TempDir()), "validate-product-test", models, func(ctx context.Context) error {
				return ValidateProduct(ctx, gimble.Env{WorkDir: dir}, Params{SuiteFile: input})
			})
			wantError := tc.status != "pass" || tc.badCitation || tc.closeFail || tc.missingVideo
			if (runErr != nil) != wantError {
				t.Fatalf("run error = %v, want error %v", runErr, wantError)
			}
			if tc.closeFail {
				if _, ok := errors.AsType[*gimble.CloseError](runErr); !ok {
					t.Fatalf("want CloseError, got %v", runErr)
				}
			}
			if len(h.prompts) != 1 {
				t.Fatalf("got %d agent turns, want one", len(h.prompts))
			}
			paths, err := filepath.Glob(filepath.Join(suite.OutputDir, "validation-*", "report.json"))
			if err != nil || len(paths) != 1 {
				t.Fatalf("reports: %v, %v", paths, err)
			}
			data, err = os.ReadFile(paths[0])
			if err != nil {
				t.Fatal(err)
			}
			var got report
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatal(err)
			}
			if len(got.Features) != 1 || got.Features[0].Status != tc.status || got.Features[0].Reason != "what the agent actually observed" {
				t.Fatalf("agent finding was lost: %+v", got)
			}
			if (got.Features[0].Error != "") != (tc.badCitation || tc.missingVideo) {
				t.Fatalf("artifact error = %q", got.Features[0].Error)
			}
			if strings.Contains(string(data), `"complete"`) {
				t.Fatal("feature report must not certify outer run completion")
			}
			for _, prompt := range h.prompts {
				if strings.Contains(prompt, paths[0]) {
					t.Fatal("workflow report path leaked into agent context")
				}
			}
			if (tc.badCitation || tc.missingVideo || tc.status != "pass") && got.Error == "" {
				t.Fatalf("blocked report lacks workflow error: %+v", got)
			}
		})
	}
}
