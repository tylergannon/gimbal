package resources

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/tylergannon/gimbal/internal/pi/config"
	"github.com/tylergannon/gimbal/internal/pi/files"
)

// stripBOM removes a leading UTF-8 byte-order mark.
func stripBOM(content string) string { return config.StripBOM(content) }

// resolvePath resolves input against baseDir, expanding a leading ~, trimming
// and normalizing Unicode spaces the way pi's resolvePath does at resource
// boundaries.
func resolvePath(input, baseDir string) string {
	return files.ResolvePath(input, baseDir, files.PathInputOptions{
		Trim:                   true,
		NormalizeUnicodeSpaces: true,
	})
}

// canonicalizePath resolves symlinks, falling back to the input.
func canonicalizePath(path string) string { return files.CanonicalizePath(path) }

// fileExists reports whether path is an existing non-directory.
func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// dirExists reports whether path is an existing directory.
func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// pathExists reports whether path exists at all.
func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// isUnderPath reports whether target equals root or lives beneath it, comparing
// resolved paths. It ports pi's isUnderPath.
func isUnderPath(target, root string) bool {
	normalizedRoot := resolvePath(root, ".")
	if target == normalizedRoot {
		return true
	}
	prefix := normalizedRoot
	if !strings.HasSuffix(prefix, string(filepath.Separator)) {
		prefix += string(filepath.Separator)
	}
	return strings.HasPrefix(target, prefix)
}

// toPosixPath converts a native path to forward slashes.
func toPosixPath(path string) string { return filepath.ToSlash(path) }

// relativePath returns p relative to root in native form, or p on error.
func relativePath(root, p string) string {
	rel, err := filepath.Rel(root, p)
	if err != nil {
		return p
	}
	return rel
}
