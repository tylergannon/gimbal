package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenCodeStopDoesNotStartServer(t *testing.T) {
	t.Parallel()
	stateDir := filepath.Join(t.TempDir(), "state")
	var output bytes.Buffer
	if err := run([]string{"opencode", "stop", "--state-dir", stateDir}, &output, &bytes.Buffer{}, os.Getenv); err != nil {
		t.Fatal(err)
	}
	if got := output.String(); got != "OpenCode is not running\n" {
		t.Fatalf("output = %q", got)
	}
	if _, err := os.Stat(filepath.Join(stateDir, "server.json")); !os.IsNotExist(err) {
		t.Fatalf("stop published server state: %v", err)
	}
}

func TestOpenCodeHelpDocumentsSharedStateAndStopConsequence(t *testing.T) {
	t.Parallel()
	var output bytes.Buffer
	if err := run([]string{"opencode", "stop", "--help"}, &output, &bytes.Buffer{}, os.Getenv); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"interrupt its active work", "--state-dir", "shared runtime state directory"} {
		if !strings.Contains(output.String(), want) {
			t.Fatalf("help missing %q:\n%s", want, output.String())
		}
	}
}

func TestOpenCodeHelpDocumentsConfigurationAndCaptures(t *testing.T) {
	t.Parallel()
	var output bytes.Buffer
	if err := run([]string{"opencode", "--help"}, &output, &bytes.Buffer{}, os.Getenv); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"opencode/MODEL", "GIMBAL_OPENCODE_DIR", "~/.gimbal/opencode", "captures/"} {
		if !strings.Contains(output.String(), want) {
			t.Fatalf("help missing %q:\n%s", want, output.String())
		}
	}
}
