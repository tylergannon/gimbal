package researchdocument

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tylergannon/gimble/workflow"
)

func TestVerifyResearchFloor(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "sources"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sources", "one.md"), []byte("source one"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "INDEX.md"), []byte("index"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := verifyResearchFloor(dir, 2); err == nil || !strings.Contains(err.Error(), "found 1 source files, need at least 2") {
		t.Fatalf("verifyResearchFloor below minimum = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sources", "two.md"), []byte("source two"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := verifyResearchFloor(dir, 2); err != nil {
		t.Fatalf("verifyResearchFloor at minimum: %v", err)
	}
}

func TestGeneratedGraphHasTheResearchFanout(t *testing.T) {
	if len(Graph.Diagnostics) != 0 {
		t.Fatalf("generated graph diagnostics = %+v", Graph.Diagnostics)
	}
	for _, operation := range Graph.Body {
		group, ok := operation.(workflow.Group)
		if !ok || group.Name != "research" {
			continue
		}
		if len(group.Children) != 5 {
			t.Fatalf("research group has %d children, want 5", len(group.Children))
		}
		for i, child := range group.Children {
			want := "agent" + string(rune('1'+i))
			if child.Name != want {
				t.Errorf("research child %d = %q, want %q", i, child.Name, want)
			}
		}
		return
	}
	t.Fatal("generated graph has no research group")
}
