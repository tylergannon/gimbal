package temporalgen_test

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/tylergannon/gimbal/internal/generate"
	"github.com/tylergannon/gimbal/internal/temporalgen"
)

// Copy a disposable module so source mutations never race with other checks or
// modify the working checkout. The generator and backend stay fixed throughout.
func moduleCopy(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	dest := t.TempDir()
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", ".agents", ".claude", "node_modules", "bin", "ephemeral":
				return filepath.SkipDir
			}
			return os.MkdirAll(filepath.Join(dest, rel), 0755)
		}
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dest, rel), data, 0644)
	})
	if err != nil {
		t.Fatal(err)
	}
	return dest
}
func read(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
func write(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
}
func command(t *testing.T, dir, variation string, args ...string) {
	t.Helper()
	c := exec.CommandContext(t.Context(), "go", args...)
	c.Dir = dir
	c.Env = append(os.Environ(), "GIMBAL_GENERATION_CASE="+variation)
	if b, err := c.CombinedOutput(); err != nil {
		t.Fatalf("go %v: %v\n%s", args, err, b)
	}
}

func TestSourceVariations(t *testing.T) {
	root := moduleCopy(t)
	base := filepath.Join(root, "internal/experiments/instrumented")
	path := filepath.Join(base, "planning/plain.go")
	original := read(t, path)
	// The actual documented entry point must recreate both missing targets.
	for _, name := range []string{"continuity", "planning"} {
		if err := os.Remove(filepath.Join(base, name+"_temporal_gen.go")); err != nil {
			t.Fatal(err)
		}
	}
	command(t, root, "", "generate", "./internal/experiments/instrumented/...")
	command(t, root, "", "test", "./internal/experiments/instrumented", "-run", "TestPaired|TestInfrastructureFailure", "-count=1")
	baseline := read(t, filepath.Join(base, "planning_temporal_gen.go"))
	cases := []struct {
		name   string
		change func(string) string
		want   string
	}{
		{"rename", func(s string) string { return strings.ReplaceAll(s, `"implementation"`, `"delivery"`) }, `"delivery"`},
		{"additional", func(s string) string {
			return strings.Replace(s, `gimbal.SetJSON(ctx, "implementation", result)`, `gimbal.SetJSON(ctx, "implementation", result)`+"\n gimbal.Set(ctx,\"proof_note\",\"additional evidence\")", 1)
		}, `"proof_note"`},
		{"conditional", func(s string) string {
			return strings.Replace(s, `result.Summary == "reviewed"`, `result.Summary == "done"`, 1)
		}, `"done"`},
		{"prompt", func(s string) string {
			return strings.Replace(s, "Return a concise summary, file planned.txt, and receipt done.", "Return a concise summary, file planned.txt, and receipt done. Changed by source.", 1)
		}, "Changed by source."},
		{"command", func(s string) string { return strings.Replace(s, `= done");`, `= done && printf source-check");`, 1) }, "source-check"},
		{"generate", func(s string) string {
			return strings.Replace(s, "\n\t}\n\treturn loop.Err()", "\n _, err = worker.Generate[continuity.Report](ctx,\"EXTRA SOURCE TURN\")\n if err != nil {return err}\n\t}\n\treturn loop.Err()", 1)
		}, "func (a *Activities) PlanningGenerate2"},
		{"loop_name", func(s string) string {
			return strings.Replace(s, `PromiseLoop(ctx, "plan"`, `PromiseLoop(ctx, "review-loop"`, 1)
		}, "review-loop"},
		{"empty_scope", func(s string) string {
			return strings.Replace(s, `gimbal.Set(ctx, "review", "parent")`, `gimbal.Scope(ctx,"empty",func(ctx context.Context)error{return nil})`+"\n"+`gimbal.Set(ctx, "review", "parent")`, 1)
		}, "empty"},
		{"bindings", func(s string) string {
			s = strings.Replace(s, `"github.com/tylergannon/gimbal"`, `flow "github.com/tylergannon/gimbal"`, 1)
			s = strings.ReplaceAll(s, "gimbal.", "flow.")
			s = regexp.MustCompile(`\bworker\b`).ReplaceAllString(s, "executor")
			return regexp.MustCompile(`\bresult\b`).ReplaceAllString(s, "report")
		}, "executor"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			source := tc.change(string(original))
			if source == string(original) {
				t.Fatal("mutation did not change source")
			}
			write(t, path, []byte(source))
			command(t, root, tc.name, "generate", "./internal/experiments/instrumented/...")
			output := read(t, filepath.Join(base, "planning_temporal_gen.go"))
			if !bytes.Contains(output, []byte(tc.want)) {
				t.Fatalf("generated output missing %q", tc.want)
			}
			// Graph extraction is a separate source consumer, with no target input.
			graph, err := generate.Extract(filepath.Join(base, "planning"), "Planning", "planning")
			if err != nil {
				t.Fatal(err)
			}
			if len(graph.Diagnostics) != 0 {
				t.Fatalf("source graph diagnostics: %+v", graph.Diagnostics)
			}
			graphText := string(read(t, filepath.Join(base, "planning/workflow_gen.go")))
			if tc.name == "rename" && !strings.Contains(graphText, `Key: "delivery"`) {
				t.Fatal("renamed graph key missing")
			}
			if tc.name == "additional" && !strings.Contains(graphText, `Key: "proof_note"`) {
				t.Fatal("added graph key missing")
			}
			if tc.name == "generate" && !strings.Contains(graphText, "EXTRA SOURCE TURN") {
				t.Fatal("new source graph turn missing")
			}
			command(t, root, tc.name, "test", "./internal/experiments/instrumented", "-run", "^TestPairedWorkflows$", "-count=1")
			// Identical input regenerates byte-identically.
			command(t, root, tc.name, "generate", "./internal/experiments/instrumented")
			if !bytes.Equal(output, read(t, filepath.Join(base, "planning_temporal_gen.go"))) {
				t.Fatal("unstable generation")
			}
		})
	}
	write(t, path, original)
	command(t, root, "", "generate", "./internal/experiments/instrumented/...")
	if !bytes.Equal(baseline, read(t, filepath.Join(base, "planning_temporal_gen.go"))) {
		t.Fatal("revert left stale generated operations")
	}
	command(t, root, "", "test", "./internal/experiments/instrumented", "-run", "^TestPairedWorkflows$", "-count=1")
}

func TestUnsupportedSourceInvalidatesOutput(t *testing.T) {
	root := moduleCopy(t)
	base := filepath.Join(root, "internal/experiments/instrumented")
	path := filepath.Join(base, "planning/plain.go")
	original := string(read(t, path))
	out := filepath.Join(base, "planning_temporal_gen.go")
	cases := []struct{ name, body, want string }{
		{"package_value", `gimbal.Set(ctx,"argv",os.Args)`, "unsupported imported package value"},
		{"scope_parameter", `gimbal.Scope(ctx,"empty",func(context.Context)error{return nil})`, "Scope callback must name"},
		{"context_value", `_ = fmt.Errorf("%T",ctx)`, "context values are supported only"},
		{"fmt_variadic", `_ = fmt.Errorf("%s", []any{"a"}...)`, "variadic helper expansion"},
		{"opaque_format", `_ = fmt.Errorf("%v", []any{Unsafe{}})`, "opaque interface values"},
		{"pointer_json", `gimbal.SetJSON(ctx,"unsafe",UnsafeList{1})`, "unsupported implicit json method"},
		{"implicit_format", `_ = fmt.Errorf("%v",Unsafe{})`, "unsupported implicit format method"},
		{"dynamic_prompt", `prompt := WorkPrompt`, "Generate prompt must be a compile-time string constant"},
		{"defer", "defer func(){}()", "unsupported statement *ast.DeferStmt"},
		{"helper", "helper()", "unsupported helper/effect"},
		{"dynamic_loop", "for i:=0;i<1;i++ {}", "unsupported statement *ast.ForStmt"},
		{"wrong_context", "gimbal.Set(context.Background(),\"bad\",\"value\")", "current lexical scope context"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			source := strings.Replace(original, `gimbal.Set(ctx, "review", "parent")`, tc.body+"\n gimbal.Set(ctx,\"review\",\"parent\")", 1)
			if tc.name == "dynamic_prompt" {
				source = strings.Replace(source, `(ctx, WorkPrompt)`, `(ctx, prompt)`, 1)
			}
			if tc.name == "pointer_json" {
				source = strings.Replace(source, `"context"`, `"context";"encoding/json"`, 1)
				source += `
type Unsafe int
func(*Unsafe)MarshalJSON()([]byte,error){return []byte("1"),nil}
type UnsafeList []Unsafe
func(UnsafeList)Schema()json.RawMessage{return nil}
func(UnsafeList)ValidateJSON([]byte)error{return nil}
`
			}
			if tc.name == "implicit_format" || tc.name == "opaque_format" {
				source += `
type Unsafe struct{}
func(Unsafe)String()string{return "user callback"}
`
				source = strings.Replace(source, `"context"`, `"context";"fmt"`, 1)
			}
			if tc.name == "context_value" || tc.name == "fmt_variadic" {
				source = strings.Replace(source, `"context"`, `"context";"fmt"`, 1)
			}
			if tc.name == "package_value" {
				source = strings.Replace(source, `"context"`, `"context"; "os"`, 1)
			}
			if tc.name == "helper" {
				source += "\nfunc helper(){}\n"
			}
			write(t, path, []byte(source))
			write(t, out, []byte("package main\n// stale answer\n"))
			err := temporalgen.Source(filepath.Join(base, "planning"), "Planning", "planning", out)
			if err == nil || !strings.Contains(err.Error(), tc.want) || !regexp.MustCompile(`plain.go:\d+:\d+:`).MatchString(err.Error()) {
				t.Fatalf("diagnostic: %v, want %s", err, tc.want)
			}
			b := read(t, out)
			if !bytes.Contains(b, []byte("temporalGenerationFailed")) || bytes.Contains(b, []byte("stale answer")) {
				t.Fatal("stale output survived")
			}
		})
	}
	write(t, path, []byte(original))
	if err := temporalgen.Source(filepath.Join(base, "planning"), "Planning", "planning", out); err != nil {
		t.Fatal(fmt.Errorf("repair regeneration: %w", err))
	}
}
