package conversation

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/tylergannon/gimbal/internal/projectfiles"
)

// CacheFile resolves a project library file before conversation admission. Its
// private copy outlives turns and native sessions, like the saved conversation.
func (m *Manager) CacheFile(ctx context.Context, id string) error {
	doc, err := projectfiles.Get(m.projectDir, id)
	if err != nil {
		return fmt.Errorf("read project file: %w", err)
	}
	body, _, _, err := projectfiles.Body(ctx, doc, "")
	if err != nil {
		return fmt.Errorf("read %s: %w", doc.Name, err)
	}
	defer func() { _ = body.Close() }()
	dir := filepath.Join(m.projectDir, "conversation-files", id)
	if err = os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	file, err := os.CreateTemp(dir, ".incoming-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(file.Name()) }()
	written, err := io.Copy(file, body)
	closeErr := file.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if written != doc.Size {
		return fmt.Errorf("read %s: received %d of %d bytes", doc.Name, written, doc.Size)
	}
	return os.Rename(file.Name(), filepath.Join(dir, filepath.Base(doc.Name)))
}

func (m *Manager) filePath(id string) (string, string, error) {
	doc, err := projectfiles.Get(m.projectDir, id)
	if err != nil {
		return "", "", fmt.Errorf("read project file context: %w", err)
	}
	path := filepath.Join(m.projectDir, "conversation-files", id, filepath.Base(doc.Name))
	info, err := os.Stat(path)
	if err != nil {
		return "", "", fmt.Errorf("read cached %s: %w", doc.Name, err)
	}
	if info.Size() != doc.Size {
		return "", "", fmt.Errorf("cached %s is incomplete; retry the file discussion", doc.Name)
	}
	if !info.Mode().IsRegular() {
		return "", "", fmt.Errorf("cached %s is not a file", doc.Name)
	}
	return path, doc.Name, nil
}
