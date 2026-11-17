package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tylergannon/gimbal/internal/agentexecutor"
	"github.com/tylergannon/gimbal/internal/executor"
)

// Run with GIMBAL_LIVE=1 and GIMBAL_TEST_BINARY pointing at a built CLI.
// Each invocation is a separate process using the real native harness.
func TestRunPromptRetainedSessionLive(t *testing.T) {
	if os.Getenv("GIMBAL_LIVE") != "1" {
		t.Skip("set GIMBAL_LIVE=1 to exercise real Claude and Codex sessions")
	}
	binary := os.Getenv("GIMBAL_TEST_BINARY")
	if binary == "" {
		t.Fatal("GIMBAL_TEST_BINARY must name the built CLI")
	}
	for _, model := range []string{"luna", "haiku"} {
		t.Run(model, func(t *testing.T) {
			workdir := t.TempDir()
			fact := string(executor.NewTurnID())
			call := func(args ...string) (string, string) {
				t.Helper()
				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
				defer cancel()
				cmd := exec.CommandContext(ctx, binary, append([]string{"run-prompt"}, args...)...)
				cmd.Dir = workdir
				var out, stderr bytes.Buffer
				cmd.Stdout, cmd.Stderr = &out, &stderr
				if err := cmd.Run(); err != nil {
					t.Fatalf("CLI: %v\nstdout: %s\nstderr: %s", err, &out, &stderr)
				}
				return out.String(), stderr.String()
			}
			out, stderr := call("--model", model, "--effort", "low", "--workdir", workdir, "Remember this random fact for our conversation: "+fact+". Do not write it to files or use tools. Reply exactly READY.")
			if strings.TrimSpace(out) != "READY" {
				t.Fatalf("initial stdout = %q", out)
			}
			var id, state string
			for line := range strings.SplitSeq(stderr, "\n") {
				if value, ok := strings.CutPrefix(line, "Session: "); ok {
					id = value
				}
				if value, ok := strings.CutPrefix(line, "State: "); ok {
					state = value
				}
			}
			footer := "To continue this session, run: gimbal run-prompt --resume " + id + " \"Your next prompt\"\n"
			if executor.Validate(id, "session_") != nil || !strings.HasSuffix(stderr, footer) {
				t.Fatalf("missing session/footer: %s", stderr)
			}
			t.Cleanup(func() { call("--close", id) })
			var initial, resumed struct{ ID string }
			if err := executor.Read(filepath.Join(state, "native.json"), &initial); err != nil || initial.ID == "" {
				t.Fatalf("initial native identity: %+v (%v)", initial, err)
			}
			schema := `{"type":"object","properties":{"fact":{"type":"string"}},"required":["fact"],"additionalProperties":false}`
			out, stderr = call("--resume", id, "--output-schema", schema, "What was the random fact I asked you to remember? Return it in the fact field. Do not use tools or read any files.")
			var recalled struct{ Fact string }
			if err := json.Unmarshal([]byte(out), &recalled); err != nil || recalled.Fact != fact {
				t.Fatalf("recall stdout = %q, error = %v", out, err)
			}
			if !strings.HasSuffix(stderr, footer) {
				t.Fatalf("resume footer: %s", stderr)
			}
			if err := executor.Read(filepath.Join(state, "native.json"), &resumed); err != nil || resumed != initial {
				t.Fatalf("native identity changed: %+v -> %+v (%v)", initial, resumed, err)
			}
			entries, err := os.ReadDir(workdir)
			if err != nil || len(entries) != 0 {
				t.Fatalf("agent wrote project files: %v (%v)", entries, err)
			}
			root := filepath.Dir(filepath.Dir(state))
			paths, err := filepath.Glob(filepath.Join(root, "turn_*", "request.json"))
			if err != nil {
				t.Fatal(err)
			}
			var turns []executor.TurnID
			for _, path := range paths {
				var request agentexecutor.Request
				if err := executor.Read(path, &request); err != nil {
					t.Fatal(err)
				}
				if string(request.Session) == id {
					turns = append(turns, request.ID)
				}
			}
			if len(turns) < 2 {
				t.Fatalf("missing distinct turns: %v", turns)
			}
			t.Logf("model=%s session=%s native=%s turns=%v recalled=%s; stdout is clean JSON; footer is last on stderr", model, id, initial.ID, turns, recalled.Fact)
		})
	}
}
