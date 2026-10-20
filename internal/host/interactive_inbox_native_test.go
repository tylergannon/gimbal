package host

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/internal/binding"
	"github.com/tylergannon/gimbal/internal/live"
	"github.com/tylergannon/gimbal/internal/observation"
	"github.com/tylergannon/gimbal/internal/runner"
	"github.com/tylergannon/gimbal/internal/runstate"
	"github.com/tylergannon/gimbal/runstore"
)

func TestNativeInboxActivation(t *testing.T) {
	state := os.Getenv("GIMBAL_NATIVE_INBOX_CHILD")
	if state == "" {
		t.Skip("subprocess restart fixture")
	}
	model, err := binding.Parse("luna")
	if err != nil {
		t.Fatal(err)
	}
	id := os.Getenv("GIMBAL_NATIVE_INBOX_RUN")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	ctx = live.WithHook(ctx, func(_ string, c live.Controller) func() {
		socket := filepath.Join(os.TempDir(), "gimbal-native-"+id+".sock")
		_ = os.Remove(socket)
		listener, err := net.Listen("unix", socket)
		if err != nil {
			panic(err)
		}
		if err := os.WriteFile(filepath.Join(state, "runs", id, "controller"), []byte(socket), 0600); err != nil {
			panic(err)
		}
		go func() {
			_ = http.Serve(listener, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var request struct{ Action, Target, Message string }
				_ = json.NewDecoder(r.Body).Decode(&request)
				result := struct {
					Value bool   `json:"value"`
					Error string `json:"error,omitempty"`
				}{Value: true}
				if request.Action == "interactive" {
					if err := c.SubmitInteractive(request.Target, request.Message); err != nil {
						result.Error = err.Error()
					}
				}
				_ = json.NewEncoder(w).Encode(result)
			}))
		}()
		return func() { _ = listener.Close(); _ = os.Remove(socket) }
	})
	err = gimbal.Run(gimbal.Project(ctx, state), id, "native-inbox", map[gimbal.WorkflowRole]gimbal.ModelBinding{gimbal.RoleAgenticDialogues: model}, func(ctx context.Context) error {
		code, out, _, err := gimbal.RunCommand(ctx, "recorded-command", filepath.Dir(state), "sh", "-c", "printf x >> executions; printf context-cobalt-319")
		if err != nil {
			return err
		}
		if code != 0 || out != "context-cobalt-319" {
			return fmt.Errorf("saved command: exit=%d output=%q", code, out)
		}
		gimbal.Set(ctx, "saved-command", out)
		session := gimbal.NewSession(ctx, gimbal.RoleAgenticDialogues, filepath.Dir(state))
		for {
			transcript, err := gimbal.Interactive(ctx, "dialogue", session, "Remember the human's token. Reply briefly with chat, echoing the token when asked. Read saved-command in context when asked. Do not end unless asked.")
			if err != nil {
				return err
			}
			if transcript.EndedBy == "human" {
				return nil
			}
		}
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestNativeInboxSurvivesHostRestart(t *testing.T) {
	if os.Getenv("GIMBAL_TEST_NATIVE_INTERACTIVE") != "1" {
		t.Skip("set GIMBAL_TEST_NATIVE_INTERACTIVE=1 to run Codex luna")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Minute)
	defer cancel()
	state := filepath.Join(t.TempDir(), ".gimbal")
	id := string(runstore.NewRunID())
	var child *exec.Cmd
	start := func() {
		t.Helper()
		child = exec.CommandContext(ctx, os.Args[0], "-test.run=^TestNativeInboxActivation$", "-test.v")
		child.Env = append(os.Environ(), "GIMBAL_NATIVE_INBOX_CHILD="+state, "GIMBAL_NATIVE_INBOX_RUN="+id)
		child.Stdout = os.Stdout
		child.Stderr = os.Stderr
		if err := child.Start(); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		if child != nil && child.ProcessState == nil {
			_ = child.Process.Kill()
			_ = child.Wait()
		}
	})
	wait := func(description string, ready func() bool) {
		t.Helper()
		for !ready() {
			select {
			case <-ctx.Done():
				t.Fatalf("%s: %v", description, ctx.Err())
			case <-time.After(20 * time.Millisecond):
			}
		}
	}
	snapshot := func() (observation.RunSnapshot, error) { return observation.NewRegistry(state).Snapshot(id) }
	accept := func(inputID, message string, finish bool) {
		t.Helper()
		raw, _ := json.Marshal(map[string]any{"id": inputID, "message": message, "finish": finish})
		wait("live owner accepts input", func() bool {
			c, err := runner.Connect(state, id)
			return err == nil && c.SubmitInteractive("dialogue", string(raw)) == nil
		})
	}
	messages := func(count int) bool {
		s, err := snapshot()
		if err != nil {
			return false
		}
		for _, row := range s.Interactions {
			if row.Status == observation.InteractiveStatusPending && len(row.Messages) == count {
				return true
			}
		}
		return false
	}
	nativeBinding := func() string {
		t.Helper()
		records, err := runstate.ForProject(state).Open(ctx, runstore.RunID(id))
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = records.Close() }()
		rows, err := records.Read(ctx, 0, 4096)
		if err != nil {
			t.Fatal(err)
		}
		for _, record := range rows {
			if record.Kind == "native-binding" {
				var value struct{ Native string }
				if err := json.Unmarshal(record.Payload, &value); err != nil {
					t.Fatal(err)
				}
				return value.Native
			}
		}
		t.Fatal("native binding missing")
		return ""
	}
	start()
	wait("human-first wait", func() bool { return messages(0) })
	accept("remember", "Remember token cobalt-sparrow-731. Confirm briefly.", false)
	wait("first chat and wait", func() bool { return messages(2) })
	first, _ := snapshot()
	var original string
	for _, row := range first.Interactions {
		original = row.Messages[1].Message
	}
	if err := child.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = child.Wait()
	native := nativeBinding()
	start()
	accept("recall", "What token did I give you? Echo it and the saved-command context value exactly.", false)
	wait("resumed chat and wait", func() bool { return messages(4) })
	second, _ := snapshot()
	for _, row := range second.Interactions {
		if row.Messages[1].Message != original || !strings.Contains(row.Messages[3].Message, "cobalt-sparrow-731") || !strings.Contains(row.Messages[3].Message, "context-cobalt-319") {
			t.Fatalf("resumed=%+v", row)
		}
	}
	accept("finish", "", true)
	if err := child.Wait(); err != nil {
		t.Fatal(err)
	}
	if next := nativeBinding(); next != native {
		t.Fatalf("native binding changed: %s -> %s", native, next)
	}
	raw, err := os.ReadFile(filepath.Join(filepath.Dir(state), "executions"))
	if err != nil || string(raw) != "x" {
		t.Fatalf("command repeated %q %v", raw, err)
	}
	final, err := snapshot()
	if err != nil || final.Run.Status != observation.StatusCompleted {
		t.Fatalf("finish=%+v %v", final.Run, err)
	}
	t.Logf("alias=luna run=%s executor session=%s: abrupt host restart preserved command result, scoped context, original reply and native session", id, native)
}
