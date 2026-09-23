package generate_test

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/tylergannon/gimble/internal/generate"
)

func TestStockRecoversMissingAndStaleOutput(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	form := filepath.Join(root, "web", "src", "routes", "review_start.remote.go")
	command := filepath.Join(root, "cmd", "gimble", "review_gen.go")
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
