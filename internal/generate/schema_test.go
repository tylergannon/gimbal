package generate_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tylergannon/gimble/internal/generate"
)

// TestSourceOwnsStructuredOutputGeneration covers the lifecycle that matters
// to a handwritten workflow: no generated files, a renamed result, and then
// removal of the result altogether.
func TestSourceOwnsStructuredOutputGeneration(t *testing.T) {
	dir, err := os.MkdirTemp(".", ".schema-generation-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(dir); err != nil {
			t.Errorf("remove temporary package: %v", err)
		}
	})
	write := func(source string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, "review.go"), []byte(source), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	base := `package review

import (
 "context"
 "github.com/tylergannon/gimble"
)

type Input struct { WorkDir string; Goal string }
type Result struct { Findings []string ` + "`json:\"findings\"`" + ` }
func Review(ctx context.Context, in Input) error {
 s := gimble.NewSession(ctx, "reviewer", in.WorkDir)
 result, err := s.Generate[Result](ctx, "review")
 if err != nil { return err }
 gimble.SetJSON(ctx, "result", result)
 return nil
}
`
	write(base)
	if err := generate.Source(dir, "Review", "review", "workflow_gen.go"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "jsonschema", "Result.json")); err != nil {
		t.Fatalf("fresh generation did not write Result schema: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "result_test.go"), []byte(`package review

import "testing"

func TestGeneratedResultValidation(t *testing.T) {
 if err := (Result{}).ValidateJSON([]byte("{\"findings\":[\"bug\"]}")); err != nil { t.Fatal(err) }
 if err := (Result{}).ValidateJSON([]byte("{\"findings\":1}")); err == nil { t.Fatal("invalid findings accepted") }
}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "test", ".")
	cmd.Dir = dir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generated package test failed: %v\n%s", err, output)
	}

	renamed := strings.Replace(base, "Result", "FindingReport", 2)
	write(renamed)
	if err := generate.Source(dir, "Review", "review", "workflow_gen.go"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "jsonschema", "FindingReport.json")); err != nil {
		t.Fatalf("renamed generation did not write schema: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "jsonschema", "Result.json")); !os.IsNotExist(err) {
		t.Fatalf("stale Result schema remains: %v", err)
	}

	if err := os.WriteFile(filepath.Join(dir, "jsonschema", "keep.json"), []byte("manual"), 0o644); err != nil {
		t.Fatal(err)
	}
	text := strings.Replace(renamed, "type FindingReport struct { Findings []string `json:\"findings\"` }\n", "", 1)
	text = strings.Replace(text, "[FindingReport]", "[gimble.Text]", 1)
	text = strings.Replace(text, "result, err := s.Generate[gimble.Text]", "_, err := s.Generate[gimble.Text]", 1)
	text = strings.Replace(text, " gimble.SetJSON(ctx, \"result\", result)", "", 1)
	write(text)
	if err := generate.Source(dir, "Review", "review", "workflow_gen.go"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "jsonschema", "keep.json")); err != nil {
		t.Fatalf("unrelated schema was removed: %v", err)
	}
}
