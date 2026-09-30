package claimaudit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrepareKeepsInvalidAnchorForRepeatedRepair(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "INDEX.md"), []byte("The limit is 12 KiB.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Prepare(dir); err != nil {
		t.Fatal(err)
	}
	claim := Claim{ID: "limit", Text: "The limit is 12 KiB.", Occurrences: []Occurrence{{BlockID: "wrong-block-id", Text: "The limit is 12 KiB."}}}
	if err := writeClaims(dir, []Claim{claim}); err != nil {
		t.Fatal(err)
	}

	inv, err := Prepare(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(inv.Extract) != 0 {
		t.Fatalf("unchanged block unexpectedly assigned extraction: %v", inv.Extract)
	}
	claims, err := ReadClaims(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(claims) != 1 || len(claims[0].Occurrences) != 1 || claims[0].Occurrences[0].BlockID != "wrong-block-id" {
		t.Fatalf("invalid claim disappeared before repair: %+v", claims)
	}
	if err := validateClaims(inv, claims); err == nil || !strings.Contains(err.Error(), "invalid occurrence") {
		t.Fatalf("invalid anchor no longer produces repairable error: %v", err)
	}
}

func TestPreparePreservesUnchangedOccurrences(t *testing.T) {
	for _, tc := range []struct {
		name string
		text string
	}{
		{name: "changed", text: "Shared fact.\n\nA revised fact.\n"},
		{name: "removed", text: "Shared fact.\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			indexPath := filepath.Join(dir, "INDEX.md")
			if err := os.WriteFile(indexPath, []byte("Shared fact.\n\nShared fact.\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			first, err := Prepare(dir)
			if err != nil {
				t.Fatal(err)
			}
			if len(first.Blocks) != 2 {
				t.Fatalf("initial blocks = %d, want 2", len(first.Blocks))
			}
			claim := Claim{ID: "shared", Text: "Shared fact.", Occurrences: []Occurrence{
				{BlockID: first.Blocks[0].ID, Text: "Shared fact."},
				{BlockID: first.Blocks[1].ID, Text: "Shared fact."},
			}}
			if err := writeClaims(dir, []Claim{claim}); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(indexPath, []byte(tc.text), 0o644); err != nil {
				t.Fatal(err)
			}

			inv, err := Prepare(dir)
			if err != nil {
				t.Fatal(err)
			}
			claims, err := ReadClaims(dir)
			if err != nil {
				t.Fatal(err)
			}
			if len(claims) != 1 || len(claims[0].Occurrences) != 1 || claims[0].Occurrences[0].BlockID != first.Blocks[0].ID {
				t.Fatalf("unchanged occurrence was lost: %+v", claims)
			}
			if tc.name == "changed" && (len(inv.Extract) != 1 || inv.Extract[0] != inv.Blocks[1].ID) {
				t.Fatalf("changed block not assigned extraction: %v", inv.Extract)
			}
		})
	}
}
