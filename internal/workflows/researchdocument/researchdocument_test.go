package researchdocument

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"text/template"

	"github.com/tylergannon/gimbal/workflow"
)

func TestResearchStageTemplatesRenderWithoutAuditContext(t *testing.T) {
	type value struct{ Text string }
	data := struct{ By map[string]value }{By: map[string]value{
		"document goal": {"goal"}, "source mode": {"fixed"}, "minimum sources per topic": {"1"},
		"assigned topic group": {"group"}, "topic directories": {"/tmp/research/topic-001"}, "minimum sources per assigned topic": {"1"},
		"missing topics": {"gap"}, "gap research directory": {"/tmp/research/gap"}, "gap research index path": {"/tmp/research/gap/INDEX.md"}, "minimum gap source files": {"1"},
		"semantic index path": {"/tmp/research/INDEX.md"}, "topic index paths": {"/tmp/research/topic-001/INDEX.md"}, "research directory": {"/tmp/research"},
	}}
	for name, source := range map[string]string{"plan": planContext, "research": researchContext, "gap": gapResearchContext, "index": indexContext} {
		tmpl, err := template.New(name).Parse(source)
		if err != nil {
			t.Fatalf("%s template: %v", name, err)
		}
		var rendered strings.Builder
		if err := tmpl.Execute(&rendered, data); err != nil {
			t.Fatalf("%s render: %v", name, err)
		}
		if strings.Contains(rendered.String(), "token counter") || strings.Contains(rendered.String(), "audit-index") {
			t.Fatalf("%s leaks later-stage tools: %s", name, rendered.String())
		}
	}
}

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

	if err := verifyResearchFloor(dir, "", 2); err == nil || !strings.Contains(err.Error(), "found 1 source files, need at least 2") {
		t.Fatalf("verifyResearchFloor below minimum = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sources", "two.md"), []byte("source two"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := verifyResearchFloor(dir, "", 2); err != nil {
		t.Fatalf("verifyResearchFloor at minimum: %v", err)
	}
	shared := filepath.Join(dir, "sources")
	topic := t.TempDir()
	if err := os.WriteFile(filepath.Join(topic, "INDEX.md"), []byte("index of shared originals"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := verifyResearchFloor(topic, shared, 2); err != nil {
		t.Fatalf("shared originals require no per-topic copies: %v", err)
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
