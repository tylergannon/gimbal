package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBuiltinCancellationClosesPendingInput(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	input, writer := io.Pipe()
	defer func() { _ = writer.Close() }()
	join := closeBuiltinInputOnCancel(ctx, input)
	defer join()
	done := make(chan error, 1)
	go func() { _, err := readBuiltinLine(ctx, bufio.NewReader(input)); done <- err }()
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("got %v, want cancellation", err)
		}
	case <-time.After(time.Second):
		t.Fatal("cancellation left the question blocked")
	}
}

func TestBuiltinPathsResolveAgainstTargetRepository(t *testing.T) {
	repo := t.TempDir()
	for _, name := range []string{"design.md", "context.txt"} {
		if err := os.WriteFile(filepath.Join(repo, name), []byte("inputs"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	in := builtinRequest{Repo: repo, Plan: "design.md", ContextFiles: []string{"context.txt"}}
	if err := normalizeBuiltin(&in); err != nil {
		t.Fatal(err)
	}
	if in.Plan != filepath.Join(repo, "design.md") || in.ContextFiles[0] != filepath.Join(repo, "context.txt") {
		t.Fatalf("wrong paths: %+v", in)
	}
	if !strings.Contains(in.Goal, in.Plan) {
		t.Fatal("plan-only input did not provide goal")
	}
}

func TestLFGPreviewDoesNotExecuteChecks(t *testing.T) {
	repo := t.TempDir()
	var stdout, stderr bytes.Buffer
	err := runBuiltin("lfg", []string{"-repo", repo, "-dry-run", "-goal", "Write the greeting", "-acceptance", "hello.txt contains hello", "-check", "touch should-not-exist"}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("%v\n%s", err, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(repo, "should-not-exist")); !os.IsNotExist(err) {
		t.Fatal("dry run executed a check")
	}
	for _, want := range []string{"Write the greeting", "hello.txt contains hello", "supervisor", "dry run"} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("preview missing %q", want)
		}
	}
}

func TestBuiltinInvalidInputStopsBeforeWork(t *testing.T) {
	for _, args := range [][]string{{"-yes"}, {"-goal", "x", "-tasks", "0"}, {"-goal", "x", "-supervisor-interval", "500ms"}, {"-goal", "x", "-finish", "maybe"}, {"-plan", "nonexistent-plan"}} {
		repo := t.TempDir()
		var out bytes.Buffer
		err := runBuiltin("work", append([]string{"-repo", repo}, args...), strings.NewReader(""), &out, &out)
		if err == nil {
			t.Errorf("accepted %v", args)
		}
		if _, err := os.Stat(filepath.Join(repo, ".gimble")); !os.IsNotExist(err) {
			t.Errorf("created run for invalid input %v", args)
		}
	}
}
