package main

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/tylergannon/gimble"
	"github.com/tylergannon/gimble/internal/workflows/review"
)

func TestRunListsTheWorkflowsBuiltIn(t *testing.T) {
	help := helpOf(t)
	for _, name := range []string{"implement", "pyramid-summary", "research-document", "review"} {
		if !strings.Contains(help, "\n  "+name+" ") {
			t.Errorf("run --help does not list %s:\n%s", name, help)
		}
	}
}

func TestImplementHelpExplainsItsGenericContract(t *testing.T) {
	help := helpOf(t, "implement")
	for _, flag := range []string{"--promise string", "--definition-of-done-file string", "--max-tasks int"} {
		if !strings.Contains(help, flag) || !strings.Contains(lineWith(help, flag), "(required)") {
			t.Errorf("run implement --help lacks required %s:\n%s", flag, help)
		}
	}
	for _, text := range []string{"planner-directed loop", "does not commit", "independent validator", "90–95%", "must not cause another lap"} {
		if !strings.Contains(help, text) {
			t.Errorf("run implement --help lacks %q:\n%s", text, help)
		}
	}
	if strings.Contains(help, "frontend") || strings.Contains(help, "Storybook") {
		t.Errorf("run implement --help retains frontend-specific language:\n%s", help)
	}
	if strings.Contains(help, "validation command") || strings.Contains(help, "check-command") {
		t.Errorf("run implement --help conflates checks with validation:\n%s", help)
	}
}

func TestPyramidSummaryHelpExplainsItsFixedInputs(t *testing.T) {
	help := helpOf(t, "pyramid-summary")
	for _, flag := range []string{"--goal string", "--semantic-index string", "--largest-document string", "--output-dir string"} {
		if !strings.Contains(help, flag) || !strings.Contains(lineWith(help, flag), "(required)") {
			t.Errorf("run pyramid-summary --help lacks required %s:\n%s", flag, help)
		}
	}
	if !strings.Contains(help, "--largest-token-budget int") || !strings.Contains(help, "default starting budget of 3200") {
		t.Errorf("run pyramid-summary --help lacks the configurable largest budget:\n%s", help)
	}
	if !strings.Contains(help, "until the next level") || !strings.Contains(help, "under 100 tokens") {
		t.Errorf("run pyramid-summary --help lacks the halving rule:\n%s", help)
	}
}

func TestResearchDocumentHelpShowsLimitsAndModelDefaults(t *testing.T) {
	help := helpOf(t, "research-document")
	for _, flag := range []string{"--goal string", "--research-dir string", "--output string", "--token-budget int", "--min-sources-per-topic int", "--max-editorial-rounds int"} {
		if !strings.Contains(help, flag) {
			t.Errorf("run research-document --help lacks %s:\n%s", flag, help)
		}
	}
	if !strings.Contains(lineWith(help, "--document-authoring"), `(default "gpt-6-astra")`) {
		t.Errorf("document authoring does not default to Astra:\n%s", help)
	}
	if !strings.Contains(lineWith(help, "--research-indexing"), `(default "gpt-5.6-luna")`) {
		t.Errorf("research indexing does not default to Luna:\n%s", help)
	}
}

// TestRunHelpShowsTheInputsAndTheRoles checks the review workflow's generated
// input flags and its centrally supplied code-review model.
func TestRunHelpShowsTheInputsAndTheRoles(t *testing.T) {
	help := helpOf(t, "review")
	for _, flag := range []string{"--work-dir string", "--goal string", "--code-review string", "--port int", "--no-web"} {
		if !strings.Contains(help, flag) {
			t.Errorf("run review --help lacks %s:\n%s", flag, help)
		}
	}
	if strings.Contains(lineWith(help, "--work-dir"), "(required)") {
		t.Errorf("run review --help marks --work-dir required:\n%s", help)
	}
	if !strings.Contains(lineWith(help, "--goal"), "(required)") {
		t.Errorf("run review --help does not mark --goal required:\n%s", help)
	}
	codeReview := lineWith(help, "--code-review")
	if !strings.Contains(codeReview, "the model for role code-review") || !strings.Contains(codeReview, `(default "gpt-5.6-luna")`) || strings.Contains(codeReview, "(required)") {
		t.Errorf("run review --help does not give code review its default model:\n%s", help)
	}
}

func TestRunRefusesAMissingInput(t *testing.T) {
	var out, errOut bytes.Buffer
	err := run([]string{"run", "review", "--code-review", "gpt-5.6-luna"}, &out, &errOut, os.Getenv)
	if err == nil || !strings.Contains(err.Error(), `"goal"`) {
		t.Errorf("run review without --goal = %v, want the required flag named", err)
	}
}

func TestReviewCommandRoleDefaultAndOverride(t *testing.T) {
	withDefault := review.Command(map[gimble.WorkflowRole]string{gimble.RoleCodeReview: "gpt-5.6-luna"})
	if got := withDefault.Flags().Lookup("code-review").DefValue; got != "gpt-5.6-luna" {
		t.Fatalf("code-review default = %q", got)
	}
	withoutDefault := review.Command(map[gimble.WorkflowRole]string{})
	flag := withoutDefault.Flags().Lookup("code-review")
	if len(flag.Annotations[cobra.BashCompOneRequiredFlag]) == 0 {
		t.Fatal("code-review without a default is not marked required")
	}
	if err := withoutDefault.Flags().Set("code-review", "gpt-5.6-luna:high"); err != nil {
		t.Fatal(err)
	}
	if got := withoutDefault.Flags().Lookup("code-review").Value.String(); got != "gpt-5.6-luna:high" {
		t.Fatalf("code-review override = %q", got)
	}
	if err := withoutDefault.Flags().Set("goal", "find bugs"); err != nil {
		t.Fatal(err)
	}
}

func helpOf(t *testing.T, args ...string) string {
	t.Helper()
	var out, errOut bytes.Buffer
	if err := run(append(append([]string{"run"}, args...), "--help"), &out, &errOut, os.Getenv); err != nil {
		t.Fatal(err)
	}
	return out.String()
}

func lineWith(text, flag string) string {
	for line := range strings.SplitSeq(text, "\n") {
		if strings.Contains(line, flag) {
			return line
		}
	}
	return ""
}
