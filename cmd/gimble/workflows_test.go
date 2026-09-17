package main

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/tylergannon/gimble/internal/workflows/review"
)

func TestRunListsTheWorkflowsBuiltIn(t *testing.T) {
	help := helpOf(t)
	for _, name := range []string{"review"} {
		if !strings.Contains(help, "\n  "+name+" ") {
			t.Errorf("run --help does not list %s:\n%s", name, help)
		}
	}
}

// TestRunHelpShowsTheInputsAndTheRoles checks the review workflow's generated
// input flags and its centrally supplied reviewer model.
func TestRunHelpShowsTheInputsAndTheRoles(t *testing.T) {
	help := helpOf(t, "review")
	for _, flag := range []string{"--work-dir string", "--goal string", "--reviewer string", "--port int", "--no-web"} {
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
	reviewer := lineWith(help, "--reviewer")
	if !strings.Contains(reviewer, "the model for role reviewer") || !strings.Contains(reviewer, `(default "gpt-5.6-luna")`) || strings.Contains(reviewer, "(required)") {
		t.Errorf("run review --help does not give the reviewer its default model:\n%s", help)
	}
}

func TestRunRefusesAMissingInput(t *testing.T) {
	var out, errOut bytes.Buffer
	err := run([]string{"run", "review", "--reviewer", "gpt-5.6-luna"}, &out, &errOut, os.Getenv)
	if err == nil || !strings.Contains(err.Error(), `"goal"`) {
		t.Errorf("run review without --goal = %v, want the required flag named", err)
	}
}

func TestReviewCommandRoleDefaultAndOverride(t *testing.T) {
	withDefault := review.Command(map[string]string{"reviewer": "gpt-5.6-luna"})
	if got := withDefault.Flags().Lookup("reviewer").DefValue; got != "gpt-5.6-luna" {
		t.Fatalf("reviewer default = %q", got)
	}
	withoutDefault := review.Command(map[string]string{})
	flag := withoutDefault.Flags().Lookup("reviewer")
	if len(flag.Annotations[cobra.BashCompOneRequiredFlag]) == 0 {
		t.Fatal("reviewer without a default is not marked required")
	}
	if err := withoutDefault.Flags().Set("reviewer", "gpt-5.6-luna:high"); err != nil {
		t.Fatal(err)
	}
	if got := withoutDefault.Flags().Lookup("reviewer").Value.String(); got != "gpt-5.6-luna:high" {
		t.Fatalf("reviewer override = %q", got)
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
