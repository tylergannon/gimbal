package files

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExpandPath(t *testing.T) {
	if got := ExpandPath("~"); containsTildePrefix(got, "~") {
		t.Errorf("ExpandPath(~) = %q, still contains ~", got)
	}
	if got := ExpandPath("~/Documents/file.txt"); containsTildePrefix(got, "~/") {
		t.Errorf("ExpandPath(~/Documents/file.txt) = %q, still contains ~/", got)
	}
	if got := ExpandPath("~draft.md"); got != "~draft.md" {
		t.Errorf("ExpandPath(~draft.md) = %q, want literal", got)
	}
	if got := ExpandPath("@~draft.md"); got != "~draft.md" {
		t.Errorf("ExpandPath(@~draft.md) = %q, want ~draft.md", got)
	}
	if got := ExpandPath("file\u00A0name.txt"); got != "file name.txt" {
		t.Errorf("ExpandPath unicode space = %q, want %q", got, "file name.txt")
	}
}

func containsTildePrefix(s, prefix string) bool {
	for i := 0; i+len(prefix) <= len(s); i++ {
		if s[i:i+len(prefix)] == prefix {
			return true
		}
	}
	return false
}

func TestResolveToCwdAbsolute(t *testing.T) {
	absolutePath := filepath.Join(t.TempDir(), "absolute", "path", "file.txt")
	got := ResolveToCwd(absolutePath, filepath.Join(t.TempDir(), "some", "cwd"))
	if got != filepath.Clean(absolutePath) {
		t.Errorf("ResolveToCwd absolute = %q, want %q", got, absolutePath)
	}
}

func TestResolveToCwdRelative(t *testing.T) {
	got := ResolveToCwd("relative/file.txt", "/some/cwd")
	want := filepath.Clean(filepath.Join("/some/cwd", "relative/file.txt"))
	if got != want {
		t.Errorf("ResolveToCwd relative = %q, want %q", got, want)
	}
}

func TestResolveToCwdTildePrefixedFilename(t *testing.T) {
	cwd := filepath.Join(t.TempDir(), "pi-path-utils-cwd")
	want := filepath.Join(cwd, "~draft.md")
	if got := ResolveToCwd("~draft.md", cwd); got != want {
		t.Errorf("ResolveToCwd(~draft.md) = %q, want %q", got, want)
	}
	if got := ResolveToCwd("@~draft.md", cwd); got != want {
		t.Errorf("ResolveToCwd(@~draft.md) = %q, want %q", got, want)
	}
}

func TestResolveReadPathExisting(t *testing.T) {
	tempDir := t.TempDir()
	const fileName = "test-file.txt"
	if err := os.WriteFile(filepath.Join(tempDir, fileName), []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := ResolveReadPath(fileName, tempDir)
	if got != filepath.Join(tempDir, fileName) {
		t.Errorf("ResolveReadPath = %q, want %q", got, filepath.Join(tempDir, fileName))
	}
}

func TestResolveReadPathNFD(t *testing.T) {
	tempDir := t.TempDir()
	nfdFileName := "file\u0065\u0301.txt" // e + combining acute accent
	if err := os.WriteFile(filepath.Join(tempDir, nfdFileName), []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}
	result := ResolveReadPath("file\u00e9.txt", tempDir)
	if !PathExists(result) {
		t.Fatalf("ResolveReadPath did not resolve to an existing file: %q", result)
	}
	if filepath.Dir(result) != tempDir {
		t.Errorf("ResolveReadPath result dir = %q, want %q", filepath.Dir(result), tempDir)
	}
}

func TestResolveReadPathCurlyQuote(t *testing.T) {
	tempDir := t.TempDir()
	curly := "Capture d\u2019cran.txt"
	if err := os.WriteFile(filepath.Join(tempDir, curly), []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := ResolveReadPath("Capture d'cran.txt", tempDir)
	if got != filepath.Join(tempDir, curly) {
		t.Errorf("ResolveReadPath curly = %q, want %q", got, filepath.Join(tempDir, curly))
	}
}

func TestResolveReadPathCombinedNFCAndCurlyQuote(t *testing.T) {
	tempDir := t.TempDir()
	curly := "Capture d\u2019\u00e9cran.txt"
	if err := os.WriteFile(filepath.Join(tempDir, curly), []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := ResolveReadPath("Capture d'\u00e9cran.txt", tempDir)
	if got != filepath.Join(tempDir, curly) {
		t.Errorf("ResolveReadPath combined = %q, want %q", got, filepath.Join(tempDir, curly))
	}
}

func TestResolveReadPathMacOSScreenshotAMPM(t *testing.T) {
	tempDir := t.TempDir()
	macosName := "Screenshot 2024-01-01 at 10.00.00\u202FAM.png"
	if err := os.WriteFile(filepath.Join(tempDir, macosName), []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := ResolveReadPath("Screenshot 2024-01-01 at 10.00.00 AM.png", tempDir)
	if got != filepath.Join(tempDir, macosName) {
		t.Errorf("ResolveReadPath AM/PM = %q, want %q", got, filepath.Join(tempDir, macosName))
	}
}

func TestResolveReadPathMacOSScreenshotLowercaseAMPM(t *testing.T) {
	tempDir := t.TempDir()
	macosName := "Screenshot 2024-01-01 at 10.00.00\u202Fam.png"
	if err := os.WriteFile(filepath.Join(tempDir, macosName), []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := ResolveReadPath("Screenshot 2024-01-01 at 10.00.00 am.png", tempDir)
	if got != filepath.Join(tempDir, macosName) {
		t.Errorf("ResolveReadPath lowercase am/pm = %q, want %q", got, filepath.Join(tempDir, macosName))
	}
}

func TestCanonicalizeAndRevision(t *testing.T) {
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "file.txt")
	if err := os.WriteFile(path, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := CanonicalizePath(tempDir); got == "" {
		t.Error("CanonicalizePath returned empty")
	}
	rev, ok := GetFileRevision(path)
	if !ok || rev == "" {
		t.Fatalf("GetFileRevision = %q, %v", rev, ok)
	}
	if _, ok := GetFileRevision(filepath.Join(tempDir, "missing.txt")); ok {
		t.Error("GetFileRevision should fail for a missing file")
	}
}

func TestFormatPathRelativeToCwdOrAbsolute(t *testing.T) {
	cwd := t.TempDir()
	inside := filepath.Join(cwd, "sub", "file.txt")
	if got := FormatPathRelativeToCwdOrAbsolute(inside, cwd); got != "sub/file.txt" {
		t.Errorf("inside = %q, want sub/file.txt", got)
	}
	outside := filepath.Join(t.TempDir(), "other.txt")
	got := FormatPathRelativeToCwdOrAbsolute(outside, cwd)
	if got != filepath.ToSlash(filepath.Clean(outside)) {
		t.Errorf("outside = %q, want %q", got, filepath.ToSlash(filepath.Clean(outside)))
	}
}
