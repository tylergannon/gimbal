//go:build !darwin && !linux

package files

import (
	"os"
	"strconv"
	"strings"
)

// GetFileRevision returns a revision string. On platforms without an inode
// revision, it falls back to the size and modification time.
func GetFileRevision(path string) (string, bool) {
	info, err := os.Stat(path)
	if err != nil {
		return "", false
	}
	parts := []string{
		"0",
		"0",
		strconv.FormatInt(info.Size(), 10),
		strconv.FormatInt(info.ModTime().UnixNano(), 10),
		"0",
	}
	return strings.Join(parts, ":"), true
}
