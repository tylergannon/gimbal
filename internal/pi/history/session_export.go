package history

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/tylergannon/gimbal/internal/pi/model"
)

// TrailingEntries builds export-only entries appended after the current
// branch.
type TrailingEntries func(parentID *string, timestamp string) []model.SessionEntry

// SerializeSessionBranch serializes the current branch and optional
// export-only entries as JSONL.
func SerializeSessionBranch(sessionManager *SessionManager, createTrailingEntries TrailingEntries) string {
	timestamp := nowISO()
	header := model.NewSessionHeader(sessionManager.GetSessionID(), timestamp, sessionManager.GetCwd())
	var lines []string
	lines = append(lines, marshalLine(header))

	var parentID *string
	for _, entry := range sessionManager.GetBranch() {
		clone := cloneEntryWithParent(entry, parentID)
		lines = append(lines, marshalLine(clone))
		id := entry.Base().ID
		parentID = &id
	}
	if createTrailingEntries != nil {
		for _, entry := range createTrailingEntries(parentID, timestamp) {
			lines = append(lines, marshalLine(entry))
		}
	}
	return strings.Join(lines, "\n") + "\n"
}

// ExportSessionToJSONL writes the current branch as JSONL and returns the
// written path.
func ExportSessionToJSONL(sessionManager *SessionManager, outputPath string, createTrailingEntries TrailingEntries) (string, error) {
	if outputPath == "" {
		stamp := strings.NewReplacer(":", "-", ".", "-").Replace(nowISO())
		outputPath = "session-" + stamp + ".jsonl"
	}
	filePath := resolvePath(outputPath)
	if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(filePath, []byte(SerializeSessionBranch(sessionManager, createTrailingEntries)), 0o644); err != nil {
		return "", err
	}
	return filePath, nil
}

func marshalLine(value any) string {
	raw, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return string(raw)
}
