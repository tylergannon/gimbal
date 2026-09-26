package edittools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestComputeEditsDiffMissingFile(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "missing-preview.txt")

	_, err := ComputeEditsDiff(missing, []Edit{{OldText: "hello", NewText: "world"}}, dir)
	want := "Could not edit file: " + missing + ". Error code: ENOENT."
	if err == nil || err.Error() != want {
		t.Fatalf("error = %v, want %q", err, want)
	}
}

func TestComputeEditsDiffUnreadableFile(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses file permission checks")
	}
	dir := t.TempDir()
	unreadable := filepath.Join(dir, "unreadable-preview.txt")
	writeTestFile(t, unreadable, "hello\n")
	if err := os.Chmod(unreadable, 0o222); err != nil {
		t.Fatal(err)
	}

	_, err := ComputeEditsDiff(unreadable, []Edit{{OldText: "hello", NewText: "world"}}, dir)
	want := "Could not edit file: " + unreadable + ". Error code: EACCES."
	if err == nil || err.Error() != want {
		t.Fatalf("error = %v, want %q", err, want)
	}
}

func TestComputeEditDiffRendersChange(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "preview.txt")
	writeTestFile(t, path, "hello world\n")

	result, err := ComputeEditDiff(path, "world", "there", dir)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Diff, "there") {
		t.Fatalf("diff = %q", result.Diff)
	}
	if result.FirstChangedLine == nil || *result.FirstChangedLine != 1 {
		t.Fatalf("first changed line = %v", result.FirstChangedLine)
	}
}
