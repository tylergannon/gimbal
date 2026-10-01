package temporalgen

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestBareGenerateInvalidatesOutput(t *testing.T) {
	dir, err := os.MkdirTemp(".", "temporalgen-test-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	dir, err = filepath.Abs(dir)
	if err != nil {
		t.Fatal(err)
	}
	outputDir, err := os.MkdirTemp(".", "temporalgen-output-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(outputDir) })
	source := `package authored
import (
	"context"
	"github.com/tylergannon/gimbal"
)
func Delivery(ctx context.Context, env gimbal.Env) error {
	worker := gimbal.NewSession(ctx, "coder", env.WorkDir)
	worker.Generate[gimbal.Text](ctx, "Reply with one sentence.")
	return nil
}
`
	if err := os.WriteFile(filepath.Join(dir, "workflow.go"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(outputDir, "delivery_temporal_gen.go")
	if err := os.WriteFile(output, []byte("package main\n// stale answer\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	err = Source(dir, "Delivery", "delivery", output)
	if err == nil || !strings.Contains(err.Error(), "Generate results cannot be discarded by a bare call") || !regexp.MustCompile(`workflow\.go:\d+:\d+:`).MatchString(err.Error()) {
		t.Fatalf("diagnostic: %v", err)
	}
	generated, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(generated), "temporalGenerationFailed") || strings.Contains(string(generated), "stale answer") {
		t.Fatalf("rejected source did not invalidate stale output: %s", generated)
	}
}
