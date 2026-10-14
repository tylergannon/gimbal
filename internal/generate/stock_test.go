package generate_test

import (
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/tylergannon/gimbal/internal/builtin"
	"github.com/tylergannon/gimbal/internal/generate"
)

func TestStockRecoversMissingAndStaleOutput(t *testing.T) {
	t.Setenv("GIMBAL_GENERATE_CACHE_DIR", t.TempDir())
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	source := root
	root = t.TempDir()
	if err := filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if strings.HasPrefix(entry.Name(), ".") || entry.Name() == "node_modules" || entry.Name() == "ephemeral" || entry.Name() == "bin" {
				return filepath.SkipDir
			}
			return os.MkdirAll(filepath.Join(root, rel), 0755)
		}
		if !entry.Type().IsRegular() {
			return nil
		}
		if !strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, ".json") && !strings.HasSuffix(path, ".txt") && !strings.HasSuffix(path, ".md") && entry.Name() != "go.mod" && entry.Name() != "go.sum" {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(root, rel), raw, 0644)
	}); err != nil {
		t.Fatal(err)
	}
	form := filepath.Join(root, "internal", "invocation", "review_gen.go")
	command := filepath.Join(root, "cmd", "gimbal", "review_gen.go")
	firstForm, err := os.ReadFile(form)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_ = os.WriteFile(form, firstForm, 0o644)
	})
	if !strings.Contains(string(firstForm), `Graph: review.Graph`) || !strings.Contains(string(firstForm), `ReviewDefinition`) {
		t.Fatalf("fixture lacks review adapter: %s", firstForm)
	}
	if err := os.Remove(form); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(command, []byte("stale command\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Batched extraction must not depend on any old generated graph existing.
	for _, selected := range builtin.Workflows {
		if err := os.Remove(filepath.Join(root, "internal", "workflows", selected.Package, "workflow_gen.go")); err != nil {
			t.Fatal(err)
		}
	}
	if err := generate.Stock(root); err != nil {
		t.Fatal(err)
	}
	formAgain, err := os.ReadFile(form)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(command); !os.IsNotExist(err) {
		t.Fatal("obsolete static command survived generation")
	}
	if !slices.Equal(firstForm, formAgain) {
		t.Fatal("stock generation did not restore original form and command bytes")
	}
	// A new process must recover its IR without another root typecheck.
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	commandProcess := exec.Command(executable, "-test.run=^TestGenerationCacheProcess$")
	commandProcess.Env = append(os.Environ(), "GIMBAL_CACHE_TEST_DIR="+root, "GIMBAL_CACHE_TEST_MODE=warm", "GIMBAL_CACHE_TEST_STOCK=1")
	if out, err := commandProcess.CombinedOutput(); err != nil {
		t.Fatalf("warm Stock: %v\n%s", err, out)
	}
	formSecond, _ := os.ReadFile(form)
	if !slices.Equal(formAgain, formSecond) {
		t.Fatal("second stock generation changed output")
	}
}
