package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestLsListsTheWorkflowsBuiltIn(t *testing.T) {
	var out, errOut bytes.Buffer
	if err := run([]string{"ls"}, &out, &errOut, os.Getenv); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"easyloop", "execute", "plan", "sprint"} {
		if !strings.Contains(out.String(), name+" ") {
			t.Errorf("ls does not list %s:\n%s", name, out.String())
		}
	}
}

// TestRunHelpShowsTheInputsAndTheRoles: a workflow's flags are its input's
// properties, required where the input requires them, and one model flag
// per role its graph names.
func TestRunHelpShowsTheInputsAndTheRoles(t *testing.T) {
	help := helpOf(t, "sprint")
	for _, flag := range []string{"--sprint int", "--issue string", "--tasks int", "--repo string", "--researcher string", "--validator string", "--supervisor string", "--model string", "--port int", "--no-web"} {
		if !strings.Contains(help, flag) {
			t.Errorf("run sprint --help lacks %s:\n%s", flag, help)
		}
	}
	if strings.Contains(help, "(required)") {
		t.Errorf("run sprint --help marks a flag required, but a sprint takes a sprint or an issue:\n%s", help)
	}
	help = helpOf(t, "execute")
	if !strings.Contains(lineWith(help, "--sprint"), "(required)") {
		t.Errorf("run execute --help does not mark --sprint required:\n%s", help)
	}
	if strings.Contains(lineWith(help, "--test"), "(required)") {
		t.Errorf("run execute --help marks the optional --test required:\n%s", help)
	}
	if !strings.Contains(lineWith(help, "--worker"), "the model for role worker") {
		t.Errorf("run execute --help lacks the worker's model flag:\n%s", help)
	}
}

func TestRunRefusesAMissingInputOrRole(t *testing.T) {
	var out, errOut bytes.Buffer
	err := run([]string{"run", "execute", "--model", "gpt-5.6-luna"}, &out, &errOut, os.Getenv)
	if err == nil || !strings.Contains(err.Error(), `"sprint"`) {
		t.Errorf("run execute without --sprint = %v, want the required flag named", err)
	}
	err = run([]string{"run", "execute", "--sprint", "1"}, &out, &errOut, os.Getenv)
	if err == nil || !strings.Contains(err.Error(), "give --worker or --model") {
		t.Errorf("run execute without a model = %v, want the role named", err)
	}
}

func helpOf(t *testing.T, workflow string) string {
	t.Helper()
	var out, errOut bytes.Buffer
	if err := run([]string{"run", workflow, "--help"}, &out, &errOut, os.Getenv); err != nil {
		t.Fatal(err)
	}
	return out.String()
}

func lineWith(text, flag string) string {
	for _, line := range strings.Split(text, "\n") {
		if strings.Contains(line, flag) {
			return line
		}
	}
	return ""
}
