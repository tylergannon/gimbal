package generate

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/tools/go/packages"
)

func TestSourceImportsPreserveBodyOnlyHelperDiagnosticAndReload(t *testing.T) {
	t.Setenv("GIMBAL_GENERATE_CACHE_DIR", t.TempDir())
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	mod, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	mod = []byte(strings.Replace(string(mod), "module github.com/tylergannon/gimbal", "module example.com/diagnostic", 1) + fmt.Sprintf("\nrequire github.com/tylergannon/gimbal v0.0.0\nreplace github.com/tylergannon/gimbal => %s\n", root))
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), mod, 0644); err != nil {
		t.Fatal(err)
	}
	sums, err := os.ReadFile(filepath.Join(root, "go.sum"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "go.sum"), sums, 0644); err != nil {
		t.Fatal(err)
	}
	write := func(name, source string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(source), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, "helper"), 0755); err != nil {
		t.Fatal(err)
	}
	write("flow.go", `package diagnostic
import("context";"example.com/diagnostic/helper";"github.com/tylergannon/gimbal")
func Flow(ctx context.Context, env gimbal.Env) error { helper.Touch(ctx); return nil }
`)
	// Gimbal occurs only in a non-inlinable body, not the exported signature.
	// Compiler export data cannot substitute for source-import metadata here.
	write("helper/helper.go", `package helper
import("context";"github.com/tylergannon/gimbal")
//go:noinline
func Touch(ctx context.Context) { gimbal.Set(ctx,"helper","value") }
`)
	graph, _, err := extractSource(dir, "Flow", "diagnostic", nil, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(graph.Diagnostics) != 1 || !strings.Contains(graph.Diagnostics[0].Message, "example.com/diagnostic/helper, which imports Gimbal, is not followed") {
		t.Fatalf("body-only helper diagnostic: %+v", graph.Diagnostics)
	}
	load := loadSourceTypes
	t.Cleanup(func() { loadSourceTypes = load })
	loadSourceTypes = func(*packages.Config, ...string) ([]*packages.Package, error) {
		return nil, fmt.Errorf("body-only diagnostic cache did not hit")
	}
	if _, _, err := extractSource(dir, "Flow", "diagnostic", nil, true); err != nil {
		t.Fatal(err)
	}
	loadSourceTypes = load
	write("helper/helper.go", `package helper
import "context"
//go:noinline
func Touch(ctx context.Context) {}
`)
	graph, _, err = extractSource(dir, "Flow", "diagnostic", nil, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(graph.Diagnostics) != 0 {
		t.Fatalf("retained source imports after edit: %+v", graph.Diagnostics)
	}
	// A new dependency source file must also refresh the source-import graph.
	write("helper/extra.go", `package helper
import("context";"github.com/tylergannon/gimbal")
//go:noinline
func Other(ctx context.Context) { gimbal.Set(ctx,"other","value") }
`)
	graph, _, err = extractSource(dir, "Flow", "diagnostic", nil, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(graph.Diagnostics) != 1 {
		t.Fatalf("new dependency import omitted: %+v", graph.Diagnostics)
	}
}
