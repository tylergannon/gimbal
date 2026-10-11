package gimbal

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tylergannon/gimbal/backend"
)

type taskFilesKey struct{}

// WithTaskFiles explicitly transfers these named bytes to an agent's execution
// environment. The runtime appends their native paths; it does not rewrite prose.
func WithTaskFiles(ctx context.Context, files ...backend.TaskFile) context.Context {
	detached := make([]backend.TaskFile, len(files))
	for i, file := range files {
		detached[i] = backend.TaskFile{Name: file.Name, Bytes: append([]byte(nil), file.Bytes...)}
	}
	return context.WithValue(ctx, taskFilesKey{}, detached)
}

// TaskFiles returns detached explicit file inputs for a remote harness bridge.
func TaskFiles(ctx context.Context) []backend.TaskFile {
	files, _ := ctx.Value(taskFilesKey{}).([]backend.TaskFile)
	return files
}

// RemoteTaskFiles collects only runtime-owned complete-value markers and explicit bytes.
func RemoteTaskFiles(ctx context.Context, prompt string) ([]backend.TaskFile, error) {
	files := append([]backend.TaskFile(nil), TaskFiles(ctx)...)
	s, err := current(ctx)
	if err != nil {
		return files, nil
	}
	queue := []string{prompt}
	seen := map[string]bool{}
	for len(queue) > 0 {
		text := queue[0]
		queue = queue[1:]
		for line := range strings.SplitSeq(text, "\n") {
			if !strings.HasPrefix(line, "Complete value: ") {
				continue
			}
			path := strings.TrimPrefix(line, "Complete value: ")
			rel, err := filepath.Rel(s.run.dir, path)
			if err != nil || strings.HasPrefix(rel, "..") || !strings.HasPrefix(filepath.ToSlash(rel), "artifacts/") || seen[path] {
				continue
			}
			seen[path] = true
			raw, err := os.ReadFile(path)
			if err != nil {
				return nil, err
			}
			digest := sha256.Sum256([]byte(path))
			files = append(files, backend.TaskFile{Name: fmt.Sprintf("context-%x", digest[:12]), Reference: path, Bytes: raw})
			queue = append(queue, string(raw))
		}
	}

	return files, nil
}
