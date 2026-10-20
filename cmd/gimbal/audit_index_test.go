package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tylergannon/gimbal/internal/claimaudit"
)

func TestAuditIndexReturnsRepairableClaimErrors(t *testing.T) {
	t.Setenv("TYPESAFE_API_KEY", "unused-before-validation")
	for _, malformed := range []bool{true, false} {
		t.Run(map[bool]string{true: "json", false: "occurrence"}[malformed], func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "INDEX.md"), []byte("The limit is **12 KiB**.\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			inv, err := claimaudit.Prepare(dir)
			if err != nil {
				t.Fatal(err)
			}
			raw := []byte("{broken JSON\n")
			if !malformed {
				raw, err = json.Marshal(claimaudit.Claim{ID: "limit", Text: "The limit is 12 KiB.", Kind: "fact", Occurrences: []claimaudit.Occurrence{{BlockID: inv.Blocks[0].ID, Text: "The limit is 12 KiB."}}})
				if err != nil {
					t.Fatal(err)
				}
			}
			if err := os.WriteFile(filepath.Join(dir, claimaudit.StateDir, "claims.jsonl"), raw, 0o644); err != nil {
				t.Fatal(err)
			}
			var out, stderr bytes.Buffer
			code := executeCLI([]string{"audit-index", "--research-dir", dir}, &out, &stderr, os.Getenv, defaultArtifactUploaders())
			if code != 65 {
				t.Fatalf("exit=%d stderr=%s", code, stderr.String())
			}
			if !strings.Contains(stderr.String(), map[bool]string{true: "claims.jsonl", false: "invalid occurrence"}[malformed]) {
				t.Fatal(stderr.String())
			}
		})
	}
}

func TestAuditIndexReadOnlyModesKeepCompletionValidation(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "INDEX.md"), []byte("A documented limit.\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := claimaudit.Prepare(dir); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, claimaudit.StateDir, "claims.jsonl"), nil, 0644); err != nil {
		t.Fatal(err)
	}
	begun, err := claimaudit.Begin(dir, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"--completion", "--verify-current"} {
		var output bytes.Buffer
		command := newAuditIndexCommand(&output)
		command.SetArgs([]string{mode, "--research-dir", dir})
		err := command.Execute()
		if mode == "--completion" {
			if err != nil {
				t.Fatal(err)
			}
			var got claimaudit.Completion
			if err := json.Unmarshal(output.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			if got.Revision != begun.Revision || got.Complete || got.AuthoringAllowed {
				t.Fatalf("completion changed: %+v", got)
			}
		} else if err == nil || !strings.Contains(err.Error(), "does not allow authoring") {
			t.Fatalf("incomplete audit passed freshness gate: %v", err)
		}
	}
	path := filepath.Join(dir, claimaudit.StateDir, "completion.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	corrupted := bytes.Replace(raw, []byte(begun.Revision), []byte("different-revision"), 1)
	if err := os.WriteFile(path, corrupted, 0644); err != nil {
		t.Fatal(err)
	}
	command := newAuditIndexCommand(&bytes.Buffer{})
	command.SetArgs([]string{"--completion", "--research-dir", dir})
	if err := command.Execute(); err == nil || !strings.Contains(err.Error(), "revision does not match") {
		t.Fatalf("invalid revision accepted: %v", err)
	}
}

func TestAuditIndexReadOnlyModesAreExclusive(t *testing.T) {
	command := newAuditIndexCommand(&bytes.Buffer{})
	command.SetArgs([]string{"--completion", "--verify-current", "--research-dir", t.TempDir()})
	if err := command.Execute(); err == nil || !strings.Contains(err.Error(), "mutually exclusive") {
		t.Fatalf("combined modes: %v", err)
	}
}
