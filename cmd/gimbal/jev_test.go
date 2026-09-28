package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMain(m *testing.M) {
	// CLI tests that start instances use fake harnesses.
	if os.Getenv("GIMBAL_LIVE") != "1" {
		_ = os.Setenv("TYPESAFE_API_KEY", "test-key")
	}
	os.Exit(m.Run())
}

func TestCLIRequiresTypeSafeKeyForStartup(t *testing.T) {
	t.Setenv("TYPESAFE_API_KEY", "")
	for _, mode := range []string{"server", "run-prompt"} {
		t.Run(mode, func(t *testing.T) {
			state := filepath.Join(t.TempDir(), "state")
			args := []string{"--no-web", "--instance-dir", state}
			if mode == "run-prompt" {
				args = []string{"run-prompt", "--model", "gpt-5.6-luna", "--logs", state, "hello"}
			}
			var out, stderr bytes.Buffer
			code := executeCLI(args, &out, &stderr, os.Getenv, defaultArtifactUploaders())
			if code != 1 || !strings.Contains(stderr.String(), "TYPESAFE_API_KEY is required") {
				t.Fatalf("exit=%d stderr=%s", code, &stderr)
			}
			if _, err := os.Stat(state); !os.IsNotExist(err) {
				t.Fatalf("startup created state without a key: %v", err)
			}
		})
	}
}

func TestCLIHelpDoesNotRequireTypeSafeKey(t *testing.T) {
	t.Setenv("TYPESAFE_API_KEY", "")
	for _, args := range [][]string{{"--help"}, {"run-prompt", "--help"}, {"run", "--help"}} {
		var out, stderr bytes.Buffer
		if code := executeCLI(args, &out, &stderr, os.Getenv, defaultArtifactUploaders()); code != 0 {
			t.Fatalf("help %v exit=%d stderr=%s", args, code, &stderr)
		}
	}
}
