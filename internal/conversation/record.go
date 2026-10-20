package conversation

import (
	"context"
	"errors"
	"slices"
	"time"
)

// ErrWorkflowBacked refuses Record on a conversation that has a workflow run:
// Get replaces such a conversation's messages with the run's transcript, which
// would discard what Record added.
var ErrWorkflowBacked = errors.New("conversation: backed by a workflow run; recording needs a record-only conversation")

// ErrRecordOnly refuses chat input, and runs, on a conversation that Record has
// written to: a workflow would replace the recorded messages with its own
// transcript.
var ErrRecordOnly = errors.New("conversation: record-only; recorded conversations take no chat input")

// ErrNotFound is the error for a conversation that does not exist.
var ErrNotFound = errors.New("conversation: not found")

// Entry is one externally sourced message and its position in that source.
type Entry struct {
	Seq int64 `json:"seq"`
	Message
}

// Cursor returns the highest sequence Record has accepted from source. It
// refuses what Record would refuse, so a recorder learns that before it streams.
func (m *Manager) Cursor(id, source string) (int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	item := m.items[id]
	if item == nil {
		return 0, ErrNotFound
	}
	if item.WorkflowRun != "" {
		return 0, ErrWorkflowBacked
	}
	return m.cursors[id][source], nil
}

// Recording returns the ID of the record-only conversation for source, creating
// it the first time. The conversation file is first written with the source's
// cursor in it, and the conversation is found by that cursor, so asking again
// after an uncertain reply never makes a second one. The one gap is a checkout
// failure inside Create itself, which happens before anything is saved.
func (m *Manager) Recording(ctx context.Context, source, title string) (string, error) {
	if source == "" {
		return "", errors.New("conversation: recording needs a source")
	}
	m.recordingMu.Lock()
	defer m.recordingMu.Unlock()
	m.mu.RLock()
	found := ""
	for id, cursors := range m.cursors {
		if _, ok := cursors[source]; ok && (found == "" || m.items[id].Created < m.items[found].Created) {
			found = id
		}
	}
	m.mu.RUnlock()
	if found != "" {
		return found, nil
	}
	item, err := m.Create(ctx, NewConversation{Title: title, Provider: "claude", SharedCheckout: true, recordSource: source})
	if err != nil {
		return "", err
	}
	return item.ID, nil
}

// Record appends messages that arrived from an outside source, such as a
// meeting recorder, without starting a harness. Entries must be in source order.
// An entry at or below the source's saved cursor is already recorded and is
// skipped, so replaying a stream after a reconnect or restart is harmless. The
// messages and the advanced cursor are saved together. It returns how many
// entries were appended.
func (m *Manager) Record(id, source string, entries []Entry) (int, error) {
	if source == "" {
		return 0, errors.New("conversation: record needs a source")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	item := m.items[id]
	if item == nil {
		return 0, ErrNotFound
	}
	if m.closing {
		return 0, errors.New("conversation: unavailable")
	}
	if item.WorkflowRun != "" {
		return 0, ErrWorkflowBacked
	}
	before, beforeCursors := clone(item), m.cursors[id]
	cursor := beforeCursors[source]
	appended := 0
	for _, entry := range entries {
		if entry.Seq <= cursor {
			continue
		}
		cursor = entry.Seq
		item.Messages = append(item.Messages, entry.Message)
		appended++
	}
	if appended == 0 {
		return 0, nil
	}
	cursors := make(map[string]int64, len(beforeCursors)+1)
	for key, value := range beforeCursors {
		cursors[key] = value
	}
	cursors[source] = cursor
	m.cursors[id] = cursors
	item.Revision++
	item.Updated = time.Now().UnixMilli()
	if err := m.save(item); err != nil {
		item.Messages, item.Revision, item.Updated = slices.Clone(before.Messages), before.Revision, before.Updated
		m.cursors[id] = beforeCursors
		return 0, err
	}
	return appended, nil
}
