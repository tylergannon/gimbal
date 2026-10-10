package generate_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/tylergannon/gimbal/internal/generate"
)

func TestStockRecoversMissingAndStaleOutput(t *testing.T) {
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
	firstCommand, err := os.ReadFile(command)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.WriteFile(form, firstForm, 0o644)
		_ = os.WriteFile(command, firstCommand, 0o644)
	})
	if err := os.Remove(form); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(command, []byte("stale command\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := generate.Stock(root); err != nil {
		t.Fatal(err)
	}
	formAgain, err := os.ReadFile(form)
	if err != nil {
		t.Fatal(err)
	}
	commandAgain, err := os.ReadFile(command)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(firstForm, formAgain) || !slices.Equal(firstCommand, commandAgain) {
		t.Fatal("stock generation did not restore original form and command bytes")
	}
	if err := generate.Stock(root); err != nil {
		t.Fatal(err)
	}
	formSecond, _ := os.ReadFile(form)
	commandSecond, _ := os.ReadFile(command)
	if !slices.Equal(formAgain, formSecond) || !slices.Equal(commandAgain, commandSecond) {
		t.Fatal("second stock generation changed output")
	}
}
