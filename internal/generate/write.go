package generate

import (
	"bytes"
	"io/fs"
	"os"
)

// Publishing an identical source file needlessly wakes Vite and the Go watcher.
// Generation still restores missing or changed output through the same path.
func writeGeneratedFile(path string, data []byte, mode fs.FileMode) error {
	if existing, err := os.ReadFile(path); err == nil && bytes.Equal(existing, data) {
		return nil
	}
	return os.WriteFile(path, data, mode)
}
