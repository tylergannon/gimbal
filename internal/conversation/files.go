package conversation

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/tylergannon/gimbal/backend"
	"github.com/tylergannon/gimbal/internal/projectfiles"
	"github.com/tylergannon/gimbal/internal/sessionstate"
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

// outgoingFiles is called outside the manager lock, only for a fresh delivery.
// Journaled envelopes retain the scalar request ID, never these file bytes.
func (m *Manager) outgoingFiles(id string) ([]backend.TaskFile, error) {
	if id == "" {
		return nil, nil
	}
	path, name, err := m.filePath(id)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return []backend.TaskFile{{Name: name, Bytes: data}}, nil
}

// fileJournal hides raw attachment reads from the observation journal. The
// harness's native session keeps its own context; ordinary tool results and
// the agent's replies still use the normal conversation projection.
func fileJournal(item *Conversation, state sessionstate.ProjectionState) sessionstate.ProjectionState {
	var prefixes []string
	for _, request := range item.Requests {
		if request.FileID != "" {
			prefixes = append(prefixes, "/conversation-files/"+request.FileID+"/")
		}
	}
	if len(prefixes) == 0 {
		return state
	}
	// TaskFile materialization is private to the selected daemon. Its generated
	// path arrives in native tool input, rather than as a desktop path.
	prefixes = append(prefixes, "/gimbal-task-input-")
	attached := func(input any) bool {
		raw, _ := json.Marshal(input)
		for _, prefix := range prefixes {
			if bytes.Contains(raw, []byte(prefix)) {
				return true
			}
		}
		return false
	}
	for _, rows := range state.Message {
		for _, row := range rows {
			if row.Get("type") == "shell" && attached(row.Get("command")) {
				row.Set("output", "Attached file contents are kept in the private file cache.")
			}
			parts, _ := row.Get("content").([]any)
			for _, value := range parts {
				part, ok := value.(*sessionstate.Obj)
				if !ok || part.Get("type") != "tool" {
					continue
				}
				tool, ok := part.Get("state").(*sessionstate.Obj)
				if !ok || !attached(tool.Get("input")) {
					continue
				}
				tool.Set("metadata", sessionstate.NewObj())
				if tool.Get("status") == "completed" || tool.Get("status") == "error" {
					text := sessionstate.NewObj()
					text.Set("type", "text")
					text.Set("text", "Attached file contents are kept in the private file cache.")
					tool.Set("content", []any{text})
				}
				if tool.Get("status") == "error" {
					failure := sessionstate.NewObj()
					failure.Set("type", "attachment.read.failed")
					failure.Set("message", "The attached-file command failed. Its raw output is omitted from the conversation journal.")
					tool.Set("error", failure)
				}
				part.Delete("providerState")
				part.Delete("providerResultState")
			}
		}
	}
	return state
}
