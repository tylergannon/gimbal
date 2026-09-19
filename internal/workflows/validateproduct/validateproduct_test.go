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

// The fixture replaces only external agents/media tools; the real workflow and
// runtime own the scopes, reports, citation checks, and adapter cleanup.
type validationHarness struct {
	dirs      map[string]string
	attack    string
	closeRole string
	prompts   []string
}

func (h *validationHarness) CreateSession(_ context.Context, model, _ string, dir string) (string, error) {
	h.dirs[model] = dir
	return model, nil
}
func (*validationHarness) Fork(context.Context, string) (string, error) {
	return "", errors.New("unexpected fork")
}
func (*validationHarness) Steer(context.Context, string, string) (bool, error) { return false, nil }
func (h *validationHarness) Close(_ context.Context, id string) error {
	if id == h.closeRole {
		return errors.New("injected close failure")
	}
	return nil
}
func (h *validationHarness) RunTurn(_ context.Context, id, prompt string, _ json.RawMessage, _ func(gimble.AgentEvent) error) (gimble.TurnResult, error) {
	h.prompts = append(h.prompts, prompt)
	dir := h.dirs[id]
	original := filepath.Join(dir, "original.txt")
	var output any
	if id == "operator" {
		if err := os.WriteFile(original, []byte("observed behavior"), 0600); err != nil {
			return gimble.TurnResult{}, err
		}
		output = Observation{Summary: "observed", EvidenceFiles: []string{original}}
	} else {
		citation := original
		switch h.attack {
		case "foreign":
			citation = filepath.Join(dir, "..", "foreign.txt")
			if err := os.WriteFile(citation, []byte("another feature"), 0600); err != nil {
				return gimble.TurnResult{}, err
			}
		case "report-citation":
			citation = filepath.Join(dir, "..", "report.json")
		case "modify-original":
			if err := os.WriteFile(original, []byte("forged behavior"), 0600); err != nil {
				return gimble.TurnResult{}, err
			}
		case "modify-report":
			if err := os.WriteFile(filepath.Join(dir, "..", "report.json"), []byte(`{"complete":true}`), 0600); err != nil {
				return gimble.TurnResult{}, err
			}
		}
		output = Verdict{Status: "pass", Reason: "observed expected behavior", EvidenceFiles: []string{citation}}
	}
	raw, err := json.Marshal(output)
	return gimble.TurnResult{Output: raw}, err
}

func TestWorkflowEvidenceAndCleanupOutcome(t *testing.T) {
	for _, tc := range []struct{ name, attack, closeRole string }{
		{name: "success"},
		{name: "foreign evidence", attack: "foreign"},
		{name: "circular report citation", attack: "report-citation"},
		{name: "original evidence modified", attack: "modify-original"},
		{name: "report modified", attack: "modify-report"},
		{name: "operator cleanup fails", closeRole: "operator"},
		{name: "validator cleanup fails", closeRole: "validator"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			driver := filepath.Join(dir, "driver")
			decoder := filepath.Join(dir, "decoder")
			for path, body := range map[string]string{
				driver:  "#!/bin/sh\nif [ \"$2\" = video-start ]; then printf 'fixture video' > \"$3\"; fi\n",
				decoder: "#!/bin/sh\nfor last; do :; done\nprintf 'fixture frame' > \"$last\"\n",
			} {
				if err := os.WriteFile(path, []byte(body), 0700); err != nil {
					t.Fatal(err)
				}
			}
			suite := Suite{Product: Product{Name: "fixture", Workdir: dir, BrowserURL: "http://127.0.0.1:1"}, Tools: Tools{PlaywrightCLI: driver, VideoDecoder: decoder}, OutputDir: filepath.Join(dir, "output"), Features: []Feature{{ID: "feature", Surface: "browser", Exercise: "observe", Expected: "expected behavior"}}}
			data, err := json.Marshal(suite)
			if err != nil {
				t.Fatal(err)
			}
			input := filepath.Join(dir, "suite.json")
			if err := os.WriteFile(input, data, 0600); err != nil {
				t.Fatal(err)
			}
			h := &validationHarness{dirs: map[string]string{}, attack: tc.attack, closeRole: tc.closeRole}
			models := map[gimble.WorkflowRole]gimble.ModelBinding{"product-operation": {Adapter: h, Model: "operator"}, "product-validation": {Adapter: h, Model: "validator"}}
			runErr := gimble.Run(gimble.Project(t.Context(), t.TempDir()), "validate-product-test", models, func(ctx context.Context) error {
				return ValidateProduct(ctx, gimble.Env{WorkDir: dir}, Params{SuiteFile: input})
			})
			if tc.attack == "" && tc.closeRole == "" && runErr != nil {
				t.Fatal(runErr)
			}
			if (tc.attack != "" || tc.closeRole != "") && runErr == nil {
				t.Fatal("run accepted invalid evidence or failed cleanup")
			}
			if tc.closeRole != "" {
				if _, ok := errors.AsType[*gimble.CloseError](runErr); !ok {
					t.Fatalf("error = %v, want CloseError", runErr)
				}
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
			want := "pass"
			if tc.attack != "" {
				want = "blocked"
			}
			if len(got.Features) != 1 || got.Features[0].Status != want {
				t.Fatalf("report = %+v, want %s", got, want)
			}
			if strings.Contains(string(data), `"complete"`) {
				t.Fatal("feature report must not certify outer run completion")
			}
			for _, prompt := range h.prompts {
				if strings.Contains(prompt, paths[0]) {
					t.Fatal("workflow report path leaked into agent context")
				}
			}
			if tc.attack != "" && got.Error == "" {
				t.Fatalf("blocked report lacks workflow error: %+v", got)
			}
		})
	}
}
