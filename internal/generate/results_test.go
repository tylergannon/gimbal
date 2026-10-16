package generate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tylergannon/gimbal/internal/builtin"
	"github.com/tylergannon/gimbal/workflow"
	"golang.org/x/tools/go/packages"
)

func TestGraphResultNoOpRepairAndPartialFailure(t *testing.T) {
	dir := cacheFixture(t)
	output := filepath.Join(dir, "workflow_gen.go")
	changed, err := GenerateGraphResult(dir, "Flow", "fixture", "workflow_gen.go")
	if err != nil || !changed {
		t.Fatalf("first graph: changed=%v err=%v", changed, err)
	}
	raw, err := os.ReadFile(output)
	if err != nil || !strings.Contains(string(raw), `Key: "first"`) {
		t.Fatalf("literal graph: %s, %v", raw, err)
	}
	stamp := time.Unix(1_500_000_000, 0)
	if err := os.Chtimes(output, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	changed, err = GenerateGraphResult(dir, "Flow", "fixture", "workflow_gen.go")
	if err != nil || changed {
		t.Fatalf("equal graph: changed=%v err=%v", changed, err)
	}
	info, err := os.Stat(output)
	if err != nil || !info.ModTime().Equal(stamp) {
		t.Fatalf("equal graph timestamp: %v %v", info, err)
	}
	if err := os.Remove(output); err != nil {
		t.Fatal(err)
	}
	changed, err = GenerateGraphResult(dir, "Flow", "fixture", "workflow_gen.go")
	if err != nil || !changed {
		t.Fatalf("repair graph: changed=%v err=%v", changed, err)
	}
	// Graph publication succeeds, then the requested Mermaid format fails.
	if err := os.Remove(output); err != nil {
		t.Fatal(err)
	}
	changed, err = SourceResult(dir, "Flow", "fixture", "workflow_gen.go", "invalid.txt")
	if err == nil || !changed {
		t.Fatalf("partial graph publication: changed=%v err=%v", changed, err)
	}
	changed, err = SourceResult(dir, "Flow", "fixture", "workflow_gen.go", "invalid.txt")
	if err == nil || changed {
		t.Fatalf("equal graph before failure: changed=%v err=%v", changed, err)
	}
}

func TestMermaidAndInspectionEqualPublication(t *testing.T) {
	dir := cacheFixture(t)
	for _, output := range []string{"viewer.json", "viewer.html"} {
		changed, err := SourceResult(dir, "Flow", "fixture", output, "")
		if err != nil || !changed {
			t.Fatalf("first %s: changed=%v err=%v", output, changed, err)
		}
		changed, err = SourceResult(dir, "Flow", "fixture", output, "")
		if err != nil || changed {
			t.Fatalf("equal %s: changed=%v err=%v", output, changed, err)
		}
	}
	changed, err := SourceResult(dir, "Flow", "fixture", "workflow_gen.go", "diagram.mmd")
	if err != nil || !changed {
		t.Fatalf("first Mermaid: changed=%v err=%v", changed, err)
	}
	stamp := time.Unix(1_500_000_000, 0)
	for _, name := range []string{"workflow_gen.go", "diagram.mmd", "diagram.svelte"} {
		if err := os.Chtimes(filepath.Join(dir, name), stamp, stamp); err != nil {
			t.Fatal(err)
		}
	}
	changed, err = SourceResult(dir, "Flow", "fixture", "workflow_gen.go", "diagram.mmd")
	if err != nil || changed {
		t.Fatalf("equal Mermaid: changed=%v err=%v", changed, err)
	}
	for _, name := range []string{"workflow_gen.go", "diagram.mmd", "diagram.svelte"} {
		info, err := os.Stat(filepath.Join(dir, name))
		if err != nil || !info.ModTime().Equal(stamp) {
			t.Fatalf("rewrote %s: %v", name, err)
		}
	}
	if err := os.Remove(filepath.Join(dir, "diagram.svelte")); err != nil {
		t.Fatal(err)
	}
	changed, err = SourceResult(dir, "Flow", "fixture", "workflow_gen.go", "diagram.mmd")
	if err != nil || !changed {
		t.Fatalf("repair component: changed=%v err=%v", changed, err)
	}
}

func TestStockSelectedProductsAndResult(t *testing.T) {
	t.Setenv("GIMBAL_GENERATE_CACHE", "off")
	dir := cacheFixture(t)
	selected := builtin.Workflows[0]
	flowdir := filepath.Join(dir, "internal", "workflows", selected.Package)
	for _, path := range []string{flowdir, filepath.Join(dir, "internal", "invocation")} {
		if err := os.MkdirAll(path, 0755); err != nil {
			t.Fatal(err)
		}
	}
	code := strings.Replace(cacheFixtureSource, "func Flow(", "func "+selected.Entry+"(", 1)
	if err := os.WriteFile(filepath.Join(flowdir, "flow.go"), []byte(code), 0644); err != nil {
		t.Fatal(err)
	}
	stock := StockGenerator{root: dir, products: map[string]analysisResult{}}
	for _, item := range builtin.Workflows {
		stock.products[item.Package] = analysisResult{Graph: workflow.Graph{Name: item.Name}, Page: sourcePage{Name: item.Name, Entry: item.Entry}, Entry: cachedEntry{Summary: "Unaffected literal"}}
	}
	original := loadSourceTypes
	defer func() { loadSourceTypes = original }()
	calls := 0
	loadSourceTypes = func(config *packages.Config, patterns ...string) ([]*packages.Package, error) {
		calls++
		if len(patterns) != 1 || patterns[0] != "./internal/workflows/"+selected.Package {
			t.Fatalf("unrelated extraction: %v", patterns)
		}
		return original(config, patterns...)
	}
	changed, err := stock.Generate(dir, nil, []string{selected.Package})
	if err != nil || !changed || calls != 1 {
		t.Fatalf("selected: changed=%v err=%v calls=%d", changed, err, calls)
	}
	adapter := filepath.Join(dir, "internal", "invocation", selected.Package+"_gen.go")
	raw, err := os.ReadFile(adapter)
	if err != nil || !strings.Contains(string(raw), "first") {
		t.Fatalf("selected literal missing: %s %v", raw, err)
	}
	stamp := time.Unix(1_500_000_000, 0)
	if err := os.Chtimes(adapter, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	changed, err = stock.Generate(dir, nil, []string{})
	if err != nil || changed || calls != 1 {
		t.Fatalf("cached equal: changed=%v err=%v calls=%d", changed, err, calls)
	}
	info, _ := os.Stat(adapter)
	if !info.ModTime().Equal(stamp) {
		t.Fatal("equal Stock adapter rewritten")
	}
	if err := os.Remove(adapter); err != nil {
		t.Fatal(err)
	}
	changed, err = stock.Generate(dir, nil, []string{})
	if err != nil || !changed || calls != 1 {
		t.Fatalf("cached repair: changed=%v err=%v calls=%d", changed, err, calls)
	}
	// A watcher can report only the renamed directory. Recreate every owned
	// product from this initialized cache without extracting any workflow.
	invocation := filepath.Join(dir, "internal", "invocation")
	if err := os.Rename(invocation, filepath.Join(t.TempDir(), "moved-invocation")); err != nil {
		t.Fatal(err)
	}
	changed, err = stock.Generate(dir, nil, []string{})
	if err != nil || !changed || calls != 1 {
		t.Fatalf("cached directory repair: changed=%v err=%v calls=%d", changed, err, calls)
	}
	for _, filename := range []string{selected.Package + "_gen.go", "registry_gen.go", "registry_projection_gen.go"} {
		if _, err := os.Stat(filepath.Join(invocation, filename)); err != nil {
			t.Fatalf("missing repaired Stock output %s: %v", filename, err)
		}
	}
	ownedDesign := filepath.Join(dir, "web", "src", "lib", "workflow-design")
	if err := os.Rename(ownedDesign, filepath.Join(t.TempDir(), "moved-workflow-design")); err != nil {
		t.Fatal(err)
	}
	changed, err = stock.Generate(dir, nil, []string{})
	if err != nil || !changed || calls != 1 {
		t.Fatalf("cached design ancestor repair: changed=%v err=%v calls=%d", changed, err, calls)
	}
	for _, filename := range []string{"index.ts", "catalog.json"} {
		if _, err := os.Stat(filepath.Join(ownedDesign, "generated", filename)); err != nil {
			t.Fatalf("missing repaired design output %s: %v", filename, err)
		}
	}
	legacy := filepath.Join(dir, "cmd", "gimbal", selected.Command)
	if err := os.MkdirAll(filepath.Dir(legacy), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacy, []byte("obsolete owned adapter"), 0644); err != nil {
		t.Fatal(err)
	}
	changed, err = stock.Generate(dir, nil, []string{})
	if err != nil || !changed || calls != 1 {
		t.Fatalf("owned removal: changed=%v err=%v calls=%d", changed, err, calls)
	}
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Fatalf("obsolete adapter survived: %v", err)
	}
	// A later output failure retains an earlier successful adapter repair.
	registry := filepath.Join(dir, "internal", "invocation", "registry_gen.go")
	if err := os.Remove(registry); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(registry, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(adapter); err != nil {
		t.Fatal(err)
	}
	changed, err = stock.Generate(dir, nil, []string{})
	if err == nil || !changed {
		t.Fatalf("partial stock: changed=%v err=%v", changed, err)
	}
}
