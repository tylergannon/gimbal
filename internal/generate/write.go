package generate

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
)

// Publishing an identical source file needlessly wakes Vite and the Go watcher.
// Generation still restores missing or changed output through the same path.
func writeGeneratedFile(path string, data []byte, mode fs.FileMode) error {
	_, err := writeGeneratedFileResult(path, data, mode)
	return err
}

func writeGeneratedFileResult(path string, data []byte, mode fs.FileMode) (bool, error) {
	existing, err := os.ReadFile(path)
	if err == nil && bytes.Equal(existing, data) {
		return false, nil
	}
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return false, err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return false, err
	}
	// Opening successfully has created or truncated the owned output. Preserve
	// that fact even if the following write or close fails.
	_, writeErr := file.Write(data)
	closeErr := file.Close()
	return true, errors.Join(writeErr, closeErr)
}

// publication accumulates changes before errors, including an earlier output
// published successfully when a later output cannot be written.
type publication struct{ changed bool }

func (p *publication) write(path string, data []byte, mode fs.FileMode) error {
	changed, err := writeGeneratedFileResult(path, data, mode)
	p.changed = p.changed || changed
	return err
}
func (p *publication) remove(path string) error {
	err := os.Remove(path)
	if err == nil {
		p.changed = true
		return nil
	}
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return err
}
