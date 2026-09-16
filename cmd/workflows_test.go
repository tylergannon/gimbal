package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestRunListsTheWorkflowsBuiltIn(t *testing.T) {
	help := helpOf(t)
	for _, name := range []string{"easyloop", "execute", "plan", "sprint"} {
		if !strings.Contains(help, "\n  "+name+" ") {
			t.Errorf("run --help does not list %s:\n%s", name, help)
		}
	}
}

// TestRunHelpShowsTheInputsAndTheRoles: a workflow's flags are its input's
// fields, required where the input requires them, and one model flag per
// role its graph names.
func TestRunHelpShowsTheInputsAndTheRoles(t *testing.T) {
	help := helpOf(t, "sprint")
	for _, flag := range []string{"--sprint int", "--issue string", "--tasks int", "--repo string", "--researcher string", "--validator string", "--supervisor string", "--port int", "--no-web"} {
		if !strings.Contains(help, flag) {
			t.Errorf("run sprint --help lacks %s:\n%s", flag, help)
		}
	}
	for _, flag := range []string{"--sprint", "--issue", "--tasks", "--repo"} {
		if strings.Contains(lineWith(help, flag), "(required)") {
			t.Errorf("run sprint --help marks %s required:\n%s", flag, help)
		}
	}
	help = helpOf(t, "execute")
	if !strings.Contains(lineWith(help, "--sprint"), "(required)") {
		t.Errorf("run execute --help does not mark --sprint required:\n%s", help)
	}
	if strings.Contains(lineWith(help, "--test"), "(required)") {
		t.Errorf("run execute --help marks the optional --test required:\n%s", help)
	}
	worker := lineWith(help, "--worker")
	if !strings.Contains(worker, "the model for role worker") || !strings.Contains(worker, `(default "gpt-5.6-luna")`) || strings.Contains(worker, "(required)") {
		t.Errorf("run execute --help does not give the worker its default model:\n%s", help)
	}
}

func TestRunRefusesAMissingInput(t *testing.T) {
	var out, errOut bytes.Buffer
	err := run([]string{"run", "execute", "--worker", "gpt-5.6-luna"}, &out, &errOut, os.Getenv)
	if err == nil || !strings.Contains(err.Error(), `"sprint"`) {
		t.Errorf("run execute without --sprint = %v, want the required flag named", err)
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
	for _, line := range strings.Split(text, "\n") {
		if strings.Contains(line, flag) {
			return line
		}
	}
	return ""
}
