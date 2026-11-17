package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tylergannon/gimbal/internal/agentexecutor"
	"github.com/tylergannon/gimbal/internal/executor"
)

func TestPromptSessionResumePreservesConfiguration(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	workdir := t.TempDir()
	root, saved, err := preparePromptSession(runPromptOptions{model: "luna", effort: "low", workdir: workdir}, false, promptCallerClaude)
	if err != nil {
		t.Fatal(err)
	}
	if err = agentexecutor.Create(root, saved); err != nil {
		t.Fatal(err)
	}
	dir := promptSessionDir(root, string(saved.ID))
	if err = executor.Write(filepath.Join(dir, "native.json"), struct{}{}); err != nil {
		t.Fatal(err)
	}
	for _, caller := range []promptCaller{promptCallerClaude, promptCallerCodex, promptCallerNone} {
		for _, explicit := range []bool{false, true} {
			options := runPromptOptions{resume: string(saved.ID), workdir: "/ignored"}
			if explicit {
				options.workdir, options.model = workdir, saved.Model
			}
			gotRoot, got, err := preparePromptSession(options, explicit, caller)
			if err != nil || got != saved || gotRoot != root {
				t.Fatalf("resume caller %q explicit %v = %+v, %v", caller, explicit, got, err)
			}
		}
	}
	for _, test := range []struct {
		name            string
		opts            runPromptOptions
		workdirExplicit bool
		want            string
	}{
		{"model", runPromptOptions{model: "haiku"}, false, "--model conflicts"},
		{"version", runPromptOptions{modelVersion: "6"}, false, "--model-version requires"},
		{"effort", runPromptOptions{effort: "high"}, false, "--effort conflicts"},
		{"workspace", runPromptOptions{workdir: t.TempDir()}, true, "--workdir conflicts"},
	} {
		t.Run(test.name, func(t *testing.T) {
			test.opts.resume = string(saved.ID)
			_, _, err := preparePromptSession(test.opts, test.workdirExplicit, promptCallerCodex)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
		})
	}
	lease, err := executor.Lock(dir)
	if err != nil || lease == nil {
		t.Fatalf("lock: %v", err)
	}
	_, _, err = preparePromptSession(runPromptOptions{resume: string(saved.ID)}, false, promptCallerNone)
	lease.Close()
	if err == nil || !strings.Contains(err.Error(), "active turn") {
		t.Fatalf("busy error = %v", err)
	}
	// Ordinary scope cleanup must leave the retained conversation open.
	adapter := &promptSessionAdapter{Adapter: &agentexecutor.Adapter{Root: root}, id: string(saved.ID)}
	if err = adapter.Close(context.Background(), string(saved.ID)); err != nil || executor.Exists(filepath.Join(dir, "closed.json")) {
		t.Fatalf("scope cleanup closed the retained session: %v", err)
	}
	if err = executor.Write(filepath.Join(dir, "closed.json"), agentexecutor.Result{}); err != nil {
		t.Fatal(err)
	}
	_, _, err = preparePromptSession(runPromptOptions{resume: string(saved.ID)}, false, promptCallerNone)
	if err == nil || !strings.Contains(err.Error(), "is closed") {
		t.Fatalf("closed error = %v", err)
	}
}

func TestPromptSessionRejectsInvalidAndMissingReferences(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	for _, id := range []string{"", " ", "../bad", "example", string(executor.NewSessionID())} {
		var stdout, stderr bytes.Buffer
		err := runPrompt([]string{"--resume", id, "follow-up"}, &stdout, &stderr, func(string) string { return "" })
		if err == nil || stdout.Len() != 0 || stderr.Len() != 0 {
			t.Fatalf("resume %q: %v, stdout %q stderr %q", id, err, &stdout, &stderr)
		}
	}
}

func TestPromptSessionCloseWithoutNativeTurn(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	root, saved, err := preparePromptSession(runPromptOptions{model: "haiku", workdir: t.TempDir()}, false, promptCallerNone)
	if err != nil {
		t.Fatal(err)
	}
	if err = agentexecutor.Create(root, saved); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	for range 2 {
		if err = runPrompt([]string{"--close", string(saved.ID)}, &stdout, &stderr, os.Getenv); err != nil {
			t.Fatal(err)
		}
	}
	if stdout.Len() != 0 || strings.Contains(stderr.String(), "To continue") {
		t.Fatalf("close output: stdout=%q stderr=%q", &stdout, &stderr)
	}
	_, _, err = preparePromptSession(runPromptOptions{resume: string(saved.ID)}, false, promptCallerNone)
	if err == nil || !strings.Contains(err.Error(), "closed") {
		t.Fatalf("closed session accepted: %v", err)
	}
}
