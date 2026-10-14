package main

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func assetFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		c := exec.Command("git", args...)
		c.Dir = root
		if raw, err := c.CombinedOutput(); err != nil {
			t.Fatalf("%v: %v %s", args, err, raw)
		}
	}
	writeAsset(t, root, ".gitignore", "web/build/\ndist/\n")
	writeAsset(t, root, "obsolete.go", "old generated code")
	writeAsset(t, root, "binding.go", "old binding")
	run("init", "-q")
	run("config", "user.email", "test@example.invalid")
	run("config", "user.name", "Test")
	run("add", ".")
	run("commit", "-qm", "source")
	writeAsset(t, root, "web/build/skgo.manifest.json", `{"fixture":"built frontend"}`)
	return root
}
func writeAsset(t *testing.T, root, name, text string) {
	t.Helper()
	full := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(full), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
}
func readAsset(t *testing.T, root, name string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, name))
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}
func TestRestoresPreparedFrontendAndChangedNewDeletedSource(t *testing.T) {
	root := assetFixture(t)
	if err := os.Remove(filepath.Join(root, "obsolete.go")); err != nil {
		t.Fatal(err)
	}
	writeAsset(t, root, "binding.go", "fresh binding")
	writeAsset(t, root, "new.go", "new generated code")
	writeAsset(t, root, "run.sh", "#!/bin/sh\n")
	if err := os.Chmod(filepath.Join(root, "run.sh"), 0755); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(root, "dist/inputs.tar.gz")
	if err := pack(root, archive); err != nil {
		t.Fatal(err)
	}
	if _, err := git(root, "restore", "."); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"new.go", "run.sh", "web/build/skgo.manifest.json"} {
		if err := os.Remove(filepath.Join(root, name)); err != nil {
			t.Fatal(err)
		}
	}
	if err := restore(root, archive); err != nil {
		t.Fatal(err)
	}
	if readAsset(t, root, "binding.go") != "fresh binding" || readAsset(t, root, "new.go") != "new generated code" || readAsset(t, root, "web/build/skgo.manifest.json") != `{"fixture":"built frontend"}` {
		t.Fatal("prepared content changed")
	}
	if _, err := os.Stat(filepath.Join(root, "obsolete.go")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("deleted output restored")
	}
	info, err := os.Stat(filepath.Join(root, "run.sh"))
	if err != nil || info.Mode().Perm() != 0755 {
		t.Fatal("executable mode lost")
	}
	if _, err := os.Stat(filepath.Join(root, "release-inputs.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("metadata left in checkout")
	}
}
func TestWrongRevisionRejectedBeforeWriting(t *testing.T) {
	root := assetFixture(t)
	writeAsset(t, root, "binding.go", "candidate binding")
	archive := filepath.Join(root, "dist/inputs.tar.gz")
	if err := pack(root, archive); err != nil {
		t.Fatal(err)
	}
	if _, err := git(root, "restore", "."); err != nil {
		t.Fatal(err)
	}
	writeAsset(t, root, "other.txt", "new revision")
	for _, args := range [][]string{{"add", "other.txt"}, {"commit", "-qm", "different source"}} {
		if _, err := git(root, args...); err != nil {
			t.Fatal(err)
		}
	}
	if err := restore(root, archive); err == nil || !strings.Contains(err.Error(), "different release revision") {
		t.Fatal(err)
	}
	if readAsset(t, root, "binding.go") != "old binding" {
		t.Fatal("wrong revision wrote content")
	}
}
func TestEmptyOrMissingFrontendRejected(t *testing.T) {
	for _, missing := range []bool{false, true} {
		t.Run(map[bool]string{false: "empty", true: "missing"}[missing], func(t *testing.T) {
			root := assetFixture(t)
			name := filepath.Join(root, "web/build/skgo.manifest.json")
			if missing {
				if err := os.Remove(name); err != nil {
					t.Fatal(err)
				}
			} else {
				writeAsset(t, root, "web/build/skgo.manifest.json", "")
			}
			if pack(root, filepath.Join(root, "dist/inputs.tar.gz")) == nil {
				t.Fatal("unprepared frontend accepted")
			}
		})
	}
}
func makeArchive(t *testing.T, archive string, meta metadata, name string, kind byte) {
	t.Helper()
	file, err := os.Create(archive)
	if err != nil {
		t.Fatal(err)
	}
	zip := gzip.NewWriter(file)
	writer := tar.NewWriter(zip)
	raw, err := json.Marshal(meta)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range []struct {
		header tar.Header
		data   []byte
	}{{tar.Header{Name: "release-inputs.json", Mode: 0600, Size: int64(len(raw)), Typeflag: tar.TypeReg}, raw}, {tar.Header{Name: name, Mode: 0600, Size: 0, Typeflag: kind, Linkname: "binding.go"}, nil}} {
		if err := writer.WriteHeader(&entry.header); err != nil {
			t.Fatal(err)
		}
		if _, err := writer.Write(entry.data); err != nil {
			t.Fatal(err)
		}
	}
	if err := errors.Join(writer.Close(), zip.Close(), file.Close()); err != nil {
		t.Fatal(err)
	}
}
func TestUnsafeArchiveAndDeletionRejectedBeforeMutation(t *testing.T) {
	for _, test := range []struct {
		name, path string
		kind       byte
		deleted    []string
	}{{"traversal", "../escape", tar.TypeReg, nil}, {"absolute", "/tmp/escape", tar.TypeReg, nil}, {"symlink", "link", tar.TypeSymlink, nil}, {"hardlink", "link", tar.TypeLink, nil}, {"deleted escape", "binding.go", tar.TypeReg, []string{"../escape"}}, {"existing escaping symlink", "escape/file", tar.TypeReg, nil}} {
		t.Run(test.name, func(t *testing.T) {
			root := assetFixture(t)
			outside := t.TempDir()
			if err := os.Symlink(outside, filepath.Join(root, "escape")); err != nil {
				t.Fatal(err)
			}
			sha, err := git(root, "rev-parse", "HEAD")
			if err != nil {
				t.Fatal(err)
			}
			archive := filepath.Join(t.TempDir(), "bad.tar.gz")
			makeArchive(t, archive, metadata{SHA: sha, Deleted: test.deleted}, test.path, test.kind)
			if restore(root, archive) == nil {
				t.Fatal("unsafe archive accepted")
			}
			if readAsset(t, root, "binding.go") != "old binding" {
				t.Fatal("unsafe archive changed checkout")
			}
			entries, err := os.ReadDir(outside)
			if err != nil || len(entries) != 0 {
				t.Fatal("archive escaped checkout")
			}
		})
	}
}
func TestPackRecordsExactRevision(t *testing.T) {
	root := assetFixture(t)
	archive := filepath.Join(root, "dist/inputs.tar.gz")
	if err := pack(root, archive); err != nil {
		t.Fatal(err)
	}
	sha, err := git(root, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if err := visit(archive, func(header *tar.Header, reader *tar.Reader) error {
		if header.Name == "release-inputs.json" {
			raw, err := io.ReadAll(reader)
			if err != nil {
				return err
			}
			var meta metadata
			if err := json.Unmarshal(raw, &meta); err != nil {
				return err
			}
			if meta.SHA != sha {
				t.Fatal("revision differs")
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
