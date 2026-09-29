package generate_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tylergannon/gimbal/internal/generate"
)

func TestGenerateGraphOwnership(t *testing.T) {
	dir, err := os.MkdirTemp("testdata", "graph-ownership-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	source := `package authored
import("context";"github.com/tylergannon/gimbal")
type Params struct{ WorkDir string; Files []string }
func Flow(ctx context.Context,env gimbal.Env, params Params)error{gimbal.Set(ctx,"value","first");return nil}
`
	write := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}
	write("flow.go", source)
	run := func(name string) error { return generate.GenerateGraph(dir, "Flow", name, "workflow_gen.go") }
	if err := run("flow"); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "workflow_gen.go")
	first, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(first), "gimbal.RegisterGraph(Graph)") {
		t.Fatal("no registration")
	}
	if err := run("other"); err == nil || !strings.Contains(err.Error(), "refusing to overwrite") {
		t.Fatalf("other workflow ownership: %v", err)
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(after, first) {
		t.Fatal("conflict changed output")
	}
	write("workflow_gen.go", "package authored\nvar handwritten = true\n")
	if err := run("flow"); err == nil || !strings.Contains(err.Error(), "refusing to overwrite") {
		t.Fatalf("handwritten ownership: %v", err)
	}
	write("workflow_gen.go", string(first)+"\nvar obsolete = noLongerDefined\n")
	write("flow.go", strings.Replace(source, "first", "changed", 1))
	if err := run("flow"); err != nil {
		t.Fatal(err)
	}
	after, _ = os.ReadFile(path)
	if !strings.Contains(string(after), `Key: "value"`) || bytes.Contains(after, []byte("obsolete")) {
		t.Fatal("stale owned output not regenerated")
	}
	write("other.go", `package authored
import flow "github.com/tylergannon/gimbal"
func init(){flow.RegisterGraph(Graph)}
`)
	if err := run("flow"); err == nil || !strings.Contains(err.Error(), "conflicting graph or registration") {
		t.Fatalf("registration conflict: %v", err)
	}
	if err := generate.GenerateGraph(dir, "Flow", "flow", "../outside.go"); err == nil {
		t.Fatal("accepted output outside package")
	}
}
