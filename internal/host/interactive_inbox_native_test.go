package host

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/internal/binding"
	"github.com/tylergannon/gimbal/internal/observation"
	"github.com/tylergannon/gimbal/internal/runstate"
	"github.com/tylergannon/gimbal/runstore"
)

func TestNativeInboxActivation(t *testing.T) {
	state := os.Getenv("GIMBAL_NATIVE_INBOX_CHILD")
	if state == "" {
		t.Skip("subprocess activation fixture")
	}
	model, err := binding.Parse("luna")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	err = gimbal.Run(gimbal.Project(ctx, state), os.Getenv("GIMBAL_NATIVE_INBOX_RUN"), "native-inbox", map[gimbal.WorkflowRole]gimbal.ModelBinding{gimbal.RoleAgenticDialogues: model}, func(ctx context.Context) error {
		session := gimbal.NewSession(ctx, gimbal.RoleAgenticDialogues, filepath.Dir(state))
		for {
			transcript, err := gimbal.Interactive(ctx, "dialogue", session, "Remember the human's token. Reply briefly with chat, echoing the token when asked. Do not end unless asked.")
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

func TestNativeInboxSurvivesAbsentExecution(t *testing.T) {
	if os.Getenv("GIMBAL_TEST_NATIVE_INTERACTIVE") != "1" {
		t.Skip("set GIMBAL_TEST_NATIVE_INTERACTIVE=1 to run Codex luna")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Minute)
	defer cancel()
	state := filepath.Join(t.TempDir(), ".gimbal")
	id := string(runstore.NewRunID())
	activate := func() {
		t.Helper()
		cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestNativeInboxActivation$", "-test.v")
		cmd.Env = append(os.Environ(), "GIMBAL_NATIVE_INBOX_CHILD="+state, "GIMBAL_NATIVE_INBOX_RUN="+id)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("activation: %v\n%s", err, out)
		}
	}
	accept := func(inputID, message string, finish bool) {
		t.Helper()
		raw, _ := json.Marshal(map[string]any{"id": inputID, "message": message, "finish": finish})
		if err := runstate.SubmitInteractive(ctx, state, id, "dialogue", string(raw)); err != nil {
			t.Fatal(err)
		}
	}
	snapshot := func() observation.RunSnapshot {
		t.Helper()
		s, err := observation.NewRegistry(state).Snapshot(id)
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	nativeBinding := func() string {
		t.Helper()
		records, err := runstate.ForProject(state).Open(ctx, runstore.RunID(id))
		if err != nil {
			t.Fatal(err)
		}
		defer records.Close()
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
	activate() // Exits at the human-first wait; no execution/controller remains.
	accept("remember", "Remember token cobalt-sparrow-731.", false)
	activate() // Startup reconstructs the accepted inbox without another input.
	first := snapshot()
	var original string
	for _, row := range first.Interactions {
		if len(row.Messages) != 2 || row.Status != "pending" {
			t.Fatalf("first=%+v", row)
		}
		original = row.Messages[1].Message
	}
	native := nativeBinding()
	accept("recall", "What token did I give you? Echo it exactly.", false)
	activate()
	second := snapshot()
	for _, row := range second.Interactions {
		if len(row.Messages) != 4 || row.Messages[1].Message != original || !strings.Contains(row.Messages[3].Message, "cobalt-sparrow-731") {
			t.Fatalf("resumed=%+v", row)
		}
	}
	if next := nativeBinding(); next != native {
		t.Fatalf("native binding changed: %s -> %s", native, next)
	}
	accept("finish", "", true)
	activate()
	if snapshot().Run.Status != observation.StatusCompleted {
		t.Fatal("finish did not complete")
	}
	t.Logf("alias=luna run=%s executor session=%s: absent-host accepted inputs resumed once; original reply preserved; same session recalled token", id, native)
}
