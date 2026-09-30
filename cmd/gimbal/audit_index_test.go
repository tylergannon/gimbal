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
