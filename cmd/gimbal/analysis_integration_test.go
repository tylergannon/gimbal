package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestBinaryAnalysisAndOrdinaryCLIRoutes(t *testing.T) {
	repo, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	binDir := t.TempDir()
	binary := filepath.Join(binDir, "gimbal")
	build := exec.Command("go", "build", "-o", binDir+string(os.PathSeparator), "./cmd/gimbal")
	build.Dir = repo
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build gimbal: %v\n%s", err, output)
	}

	stdout, stderr, code := runCommand(t, repo, binary, "lint", "./internal/workflows/review")
	if code != 0 || stdout != "" || stderr != "" {
		t.Fatalf("standalone lint = exit %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}

	stdout, stderr, code = runCommand(t, repo, "go", "vet", "-vettool="+binary, "./internal/workflows/review")
	if code != 0 || stdout != "" || stderr != "" {
		t.Fatalf("vettool lint = exit %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}

	// Even a real-looking unitchecker config remains an ordinary argument
	// after an explicit Gimbal command or server option.
	config := filepath.Join(t.TempDir(), "settings.cfg")
	if err := os.WriteFile(config, []byte(`{"ImportPath":"example.com/p","GoFiles":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	args := []string{"run-prompt", "-h", config}
	stdout, stderr, code = runCommand(t, repo, binary, args...)
	if code != 0 || stdout != "" || !strings.Contains(stderr, "Usage of gimbal") || strings.Contains(stderr, "gimballint") {
		t.Fatalf("ordinary CLI %q = exit %d\nstdout:\n%s\nstderr:\n%s", args, code, stdout, stderr)
	}
	args = []string{"--help", "--uds", config}
	stdout, stderr, code = runCommand(t, repo, binary, args...)
	if code != 0 || stderr != "" || !strings.Contains(stdout, "Usage:") || strings.Contains(stdout, "gimballint") {
		t.Fatalf("ordinary CLI %q = exit %d\nstdout:\n%s\nstderr:\n%s", args, code, stdout, stderr)
	}
}

func runCommand(t *testing.T, dir, name string, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	var out, errOut bytes.Buffer
	command := exec.Command(name, args...)
	command.Dir = dir
	command.Stdout = &out
	command.Stderr = &errOut
	err := command.Run()
	if err == nil {
		return out.String(), errOut.String(), 0
	}
	var exit *exec.ExitError
	if !errors.As(err, &exit) {
		t.Fatalf("run %s: %v", name, err)
	}
	return out.String(), errOut.String(), exit.ExitCode()
}
