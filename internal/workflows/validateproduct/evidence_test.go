package validateproduct

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEvidenceRejectsForeignAndNewFiles(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "feature")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	original := filepath.Join(dir, "original.txt")
	foreign := filepath.Join(root, "report.json")
	newFile := filepath.Join(dir, "new.txt")
	for _, path := range []string{original, foreign, newFile} {
		if err := os.WriteFile(path, []byte("evidence"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	link := filepath.Join(dir, "linked.txt")
	if err := os.Symlink(foreign, link); err != nil {
		t.Fatal(err)
	}
	originals, err := snapshotEvidence(dir, []string{original})
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{foreign, "../report.json", link} {
		if _, err := snapshotEvidence(dir, []string{path}); err == nil {
			t.Errorf("accepted foreign operator evidence %s", path)
		}
	}
	for _, path := range []string{foreign, "../report.json", link, newFile} {
		if _, err := verdictEvidence(dir, originals, []string{path}); err == nil {
			t.Errorf("accepted invalid citation %s", path)
		}
	}
	if _, err := verdictEvidence(dir, originals, []string{"original.txt"}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(original, []byte("changed"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := verdictEvidence(dir, originals, nil); err == nil {
		t.Fatal("accepted modified original even when uncited")
	}
}
