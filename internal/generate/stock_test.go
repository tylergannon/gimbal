package generate_test

import (
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/tylergannon/gimbal/internal/builtin"
	"github.com/tylergannon/gimbal/internal/generate"
)

func TestStockRecoversMissingAndStaleOutput(t *testing.T) {
	t.Setenv("GOWORK", "off")
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
	expectedOutputs := map[string][]byte{}
	for _, selected := range builtin.Workflows {
		for _, path := range []string{
			filepath.Join(root, "internal", "invocation", selected.Package+"_gen.go"),
			filepath.Join(root, "internal", "builtin", "pages", selected.Name+".json"),
		} {
			expected, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			expectedOutputs[path] = expected
		}
	}
	for _, name := range []string{"registry_gen.go", "registry_projection_gen.go"} {
		path := filepath.Join(root, "internal", "invocation", name)
		expected, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		expectedOutputs[path] = expected
	}
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
	if err := generate.Stock(root, nil); err != nil {
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
	// Damage both Go adapters and inspection pages before the cached process.
	for path := range expectedOutputs {
		if err := os.WriteFile(path, []byte("damaged generated output\n"), 0644); err != nil {
			t.Fatal(err)
		}
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
	for path, expected := range expectedOutputs {
		actual, err := os.ReadFile(path)
		if err != nil || !slices.Equal(actual, expected) {
			t.Fatalf("warm generated product differs: %s (%v)", path, err)
		}
	}
	// Replaying identical authored input must not wake file watchers. Use a
	// fixed timestamp supplied by this fixture, not an observed baseline.
	untouched := time.Unix(1_500_000_000, 0)
	for path := range expectedOutputs {
		if err := os.Chtimes(path, untouched, untouched); err != nil {
			t.Fatal(err)
		}
	}
	if err := generate.Stock(root, nil); err != nil {
		t.Fatal(err)
	}
	for path := range expectedOutputs {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if !info.ModTime().Equal(untouched) {
			t.Fatalf("identical output rewrote %s", path)
		}
	}
	formSecond, _ := os.ReadFile(form)
	if !slices.Equal(formAgain, formSecond) {
		t.Fatal("second stock generation changed output")
	}
}
