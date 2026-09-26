//go:build darwin

package files

import (
	"os"
	"strconv"
	"strings"
	"syscall"
)

// GetFileRevision returns a revision string built from the file's device,
// inode, size, mtime and ctime, or false when the file cannot be statted.
func GetFileRevision(path string) (string, bool) {
	info, err := os.Stat(path)
	if err != nil {
		return "", false
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return "", false
	}
	parts := []string{
		strconv.FormatUint(uint64(st.Dev), 10),
		strconv.FormatUint(st.Ino, 10),
		strconv.FormatInt(st.Size, 10),
		strconv.FormatInt(st.Mtimespec.Nano(), 10),
		strconv.FormatInt(st.Ctimespec.Nano(), 10),
	}
	return strings.Join(parts, ":"), true
}
