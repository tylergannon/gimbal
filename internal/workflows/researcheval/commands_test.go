package researcheval

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestRecordedFilesystemCommands(t *testing.T) {
	ctx := workflowContext(t)
	dir := t.TempDir()
	for _, name := range []string{"b-pass-2.json", "a-pass-1.json", ".hidden-pass-1.json", "unrelated.txt"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("body"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	paths, err := globPaths(ctx, filepath.Join(dir, "*-pass-*.json"))
	want := []string{filepath.Join(dir, "a-pass-1.json"), filepath.Join(dir, "b-pass-2.json")}
	if err != nil || !slices.Equal(paths, want) {
		t.Fatalf("paths %v %v", paths, err)
	}
	paths, err = globPaths(ctx, filepath.Join(dir, "missing", "*.json"))
	if err != nil || len(paths) != 0 {
		t.Fatalf("missing paths %v %v", paths, err)
	}
	link := filepath.Join(dir, "quoted ' link")
	if err = os.Symlink(filepath.Join(dir, "unrelated.txt"), link); err != nil {
		t.Fatal(err)
	}
	resolved, err := realPath(ctx, link)
	wantPath, resolveErr := filepath.EvalSymlinks(filepath.Join(dir, "unrelated.txt"))
	if err != nil || resolveErr != nil || resolved != wantPath {
		t.Fatalf("realpath %q %v", resolved, err)
	}
	if _, err = sourceTexts(ctx, dir, researchCase{Sources: "."}); err == nil {
		t.Fatal("corpus accepted symlink")
	}
}
func TestRecordedClockUsesMonotonicSeconds(t *testing.T) {
	ctx := workflowContext(t)
	first, err := clock(ctx)
	if err != nil {
		t.Fatal(err)
	}
	second, err := clock(ctx)
	if err != nil || first <= 0 || second < first {
		t.Fatalf("clock %v then %v: %v", first, second, err)
	}
}
