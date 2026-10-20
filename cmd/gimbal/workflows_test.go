package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/internal/invocation"
	"github.com/tylergannon/gimbal/web"
	"golang.org/x/sys/unix"
)

func TestGeneratedCLIClientReachesUnknownProjectAndFollowReportsFailure(t *testing.T) {
	base := t.TempDir()
	project := filepath.Join(base, "project")
	if err := os.Mkdir(project, 0o755); err != nil {
		t.Fatal(err)
	}
	instanceDir := filepath.Join(base, "instance")
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	instance, err := web.NewInstance(ctx, instanceDir, nil, web.WithNoWeb())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { cancel(); instance.Wait() }()
	var output bytes.Buffer
	args := []string{"run", "validate-product", "--instance-dir", instanceDir, "--project", project, "--suite-file", filepath.Join(base, "missing-suite.json"), "--follow"}
	err = run(args, &output, &bytes.Buffer{}, os.Getenv)
	if err == nil || !strings.Contains(err.Error(), "failed") {
		t.Fatalf("--follow error = %v; output %q", err, output.String())
	}
	match := regexp.MustCompile(`^[0-9A-HJKMNP-TV-Z]{26}(?:\.[a-z-]+)?$`).FindString(strings.TrimSpace(output.String()))
	if match == "" {
		t.Fatalf("CLI did not print accepted run ID: %q", output.String())
	}
	row, err := web.Follow(context.Background(), instanceDir, project, match)
	if err != nil || row.Status != "failed" {
		t.Fatalf("server-owned terminal row = %+v, %v", row, err)
	}
	// A terminal observation precedes runner teardown. Its lifetime lock is
	// released only after all durable writes and controller shutdown finish.
	lock, err := os.OpenFile(filepath.Join(project, ".gimbal", "runs", match, "runner.lock"), os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err = unix.Flock(int(lock.Fd()), unix.LOCK_EX); err != nil {
		_ = lock.Close()
		t.Fatal(err)
	}
	if err = lock.Close(); err != nil {
		t.Fatal(err)
	}
	if len(instance.Owner.Projects()) != 1 {
		t.Fatalf("first start admitted %d projects", len(instance.Owner.Projects()))
	}
}

func TestRunListsTheWorkflowsBuiltIn(t *testing.T) {
	help := helpOf(t)
	for _, name := range []string{"sprint-plan", "implement", "pyramid-summary", "research-document", "review", "validate-product"} {
		if !strings.Contains(help, "\n  "+name+" ") {
			t.Errorf("run --help does not list %s:\n%s", name, help)
		}
	}
}

func TestImplementHelpExplainsItsOutcomeContract(t *testing.T) {
	help := helpOf(t, "implement")
	for _, flag := range []string{"--outcomes-file string", "--max-tasks-per-outcome int"} {
		if !strings.Contains(help, flag) || !strings.Contains(lineWith(help, flag), "(required)") {
			t.Errorf("run implement --help lacks required %s:\n%s", flag, help)
		}
	}
	for _, text := range []string{"ordered list of outcomes", "PromiseLoop", "not commit", "independent validator", "90–95%", "next outcome"} {
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
	for _, text := range []string{"Model defaults use Gemini 3.1 Pro", "Gemini 3.8 Flash", "up to six slots"} {
		if !strings.Contains(help, text) {
			t.Errorf("run pyramid-summary --help lacks model-selection guidance %q:\n%s", text, help)
		}
	}
}

func TestResearchDocumentHelpShowsLimitsAndModelDefaults(t *testing.T) {
	help := helpOf(t, "research-document")
	for _, flag := range []string{"--goal string", "--research-dir string", "--output string", "--token-budget int", "--min-sources-per-topic int", "--max-editorial-rounds int"} {
		if !strings.Contains(help, flag) {
			t.Errorf("run research-document --help lacks %s:\n%s", flag, help)
		}
	}
	for _, text := range []string{"broad collection on Gemini Flash", "synthesis on Gemini Pro", "Gemini 3.8 Flash", "Gemini 3.1 Pro"} {
		if !strings.Contains(help, text) {
			t.Errorf("run research-document --help lacks model-selection guidance %q:\n%s", text, help)
		}
	}
}

// TestRunHelpShowsTheInputsAndTheRoles checks the review workflow's generated
// input flags and its centrally supplied code-review model.
func TestRunHelpShowsTheInputsAndTheRoles(t *testing.T) {
	help := helpOf(t, "review")
	for _, flag := range []string{"--work-dir string", "--project string", "--instance-dir string", "--follow", "--goal string", "--code-review string"} {
		if !strings.Contains(help, flag) {
			t.Errorf("run review --help lacks %s:\n%s", flag, help)
		}
	}
	for _, phrase := range []string{"share their host's Go execution capacity", "Exiting this client does not cancel the run", "re-enters unfinished runs", "instance startup environment", "optional effort"} {
		if !strings.Contains(help, phrase) {
			t.Errorf("run review --help lacks %q", phrase)
		}
	}
	if strings.Contains(lineWith(help, "--work-dir"), "(required)") {
		t.Errorf("run review --help marks --work-dir required:\n%s", help)
	}
	if !strings.Contains(lineWith(help, "--goal"), "(required)") {
		t.Errorf("run review --help does not mark --goal required:\n%s", help)
	}
	codeReview := lineWith(help, "--code-review string")
	if !strings.Contains(codeReview, "advanced override for role code-review") || !strings.Contains(codeReview, "omit this flag") || strings.Contains(codeReview, "(required)") {
		t.Errorf("run review --help does not give code review its default model:\n%s", help)
	}
}

func TestRunRefusesAMissingInput(t *testing.T) {
	_, _, err := invocation.Prepare(invocation.Request{Workflow: "review", Input: []byte(`{}`)})
	if err == nil || !strings.Contains(err.Error(), "goal") {
		t.Fatalf("missing goal: %v", err)
	}
}

func TestReviewCommandRoleDefaultAndOverride(t *testing.T) {
	d, ok := gimbal.DefaultRegistry.Workflow("review")
	if !ok {
		t.Fatal("no review")
	}
	description := d.WorkflowDescription
	description.Roles = map[gimbal.WorkflowRole]string{gimbal.RoleCodeReview: "model"}
	withDefault := catalogCommand(description)
	defaultFlag := withDefault.Flags().Lookup("code-review")
	if got := defaultFlag.DefValue; got != "model" {
		t.Fatalf("code-review default = %q", got)
	}
	if !strings.Contains(defaultFlag.Usage, "omit this flag") {
		t.Fatalf("code-review usage does not explain how to preserve its default: %q", defaultFlag.Usage)
	}

}

func helpOf(t *testing.T, args ...string) string {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	instanceDir := filepath.Join(t.TempDir(), "instance")
	instance, err := web.NewInstance(ctx, instanceDir, nil, web.WithNoWeb())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cancel(); instance.Wait() })
	var out, errOut bytes.Buffer
	if err := run(append(append([]string{"run"}, args...), "--help", "--instance-dir", instanceDir), &out, &errOut, os.Getenv); err != nil {
		t.Fatal(err)
	}
	return out.String()
}

func lineWith(text, flag string) string {
	for line := range strings.SplitSeq(text, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), flag+" ") {
			return line
		}
	}
	return ""
}

func TestValidateProductHelp(t *testing.T) {
	help := helpOf(t, "validate-product")
	if !strings.Contains(lineWith(help, "--suite-file string"), "(required)") {
		t.Fatal(help)
	}
	for _, text := range []string{"one to three workloads", "implementation source", "issue_repo", "human review"} {
		if !strings.Contains(help, text) {
			t.Fatalf("missing %s", text)
		}
	}
}
