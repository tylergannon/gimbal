package history

import (
	"encoding/json"
	"slices"

	"github.com/tylergannon/gimbal/internal/pi/model"
)

// SessionEntryToContextMessages projects one selected session entry into
// model-visible messages.
//
// Plain custom entries are display/state entries and do not participate in
// context.
func SessionEntryToContextMessages(entry model.SessionEntry) []model.AgentMessage {
	switch typed := entry.(type) {
	case *model.SessionMessageEntry:
		return []model.AgentMessage{typed.Message}
	case *model.CustomMessageEntry:
		timestamp := timestampMillis(typed.Timestamp)
		// The entry records content as a string or an array; preserve the form
		// while rebuilding an agent message whose timestamp is in milliseconds.
		raw, err := json.Marshal(typed)
		if err != nil {
			return nil
		}
		var probe struct {
			Content json.RawMessage `json:"content"`
		}
		if err := json.Unmarshal(raw, &probe); err == nil && len(probe.Content) > 0 && probe.Content[0] == '"' {
			var text string
			if err := json.Unmarshal(probe.Content, &text); err != nil {
				return nil
			}
			return []model.AgentMessage{model.NewCustomText(typed.CustomType, text, typed.Display, typed.Details, timestamp)}
		}
		return []model.AgentMessage{model.CustomMessage{
			CustomType: typed.CustomType,
			Content:    typed.Content.Clone(),
			Display:    typed.Display,
			Details:    typed.Details,
			Timestamp:  timestamp,
		}}
	case *model.BranchSummaryEntry:
		if typed.Summary == "" {
			return nil
		}
		return []model.AgentMessage{model.CreateBranchSummaryMessage(typed.Summary, typed.FromID, timestampMillis(typed.Timestamp))}
	case *model.CompactionEntry:
		summary := model.CreateCompactionSummaryMessage(typed.Summary, typed.TokensBefore, timestampMillis(typed.Timestamp))
		if typed.SystemMessage != nil {
			return []model.AgentMessage{*typed.SystemMessage, summary}
		}
		return []model.AgentMessage{summary}
	default:
		return nil
	}
}

func projectContextEntry(entry model.SessionEntry, edit *model.ContextEditEntry) []model.AgentMessage {
	messages := SessionEntryToContextMessages(entry)
	if edit == nil {
		return messages
	}
	if edit.Replacement == nil {
		return nil
	}
	replacement := edit.Replacement.Content.Clone()
	out := make([]model.AgentMessage, 0, len(messages))
	for _, message := range messages {
		switch typed := message.(type) {
		case model.UserMessage:
			typed.Content = replacement
			out = append(out, typed)
		case model.AssistantMessage:
			typed.Content = replacement
			out = append(out, typed)
		case model.ToolResultMessage:
			typed.Content = replacement
			out = append(out, typed)
		case model.CustomMessage:
			typed.Content = replacement
			out = append(out, typed)
		default:
			out = append(out, message)
		}
	}
	return out
}

func buildEntryIndex(entries []model.SessionEntry, byID map[string]model.SessionEntry) map[string]model.SessionEntry {
	if byID != nil {
		return byID
	}
	index := make(map[string]model.SessionEntry, len(entries))
	for _, entry := range entries {
		index[entry.Base().ID] = entry
	}
	return index
}

func buildSessionPath(entries []model.SessionEntry, leafID *string, byID map[string]model.SessionEntry, useLast bool) []model.SessionEntry {
	index := buildEntryIndex(entries, byID)
	var leaf model.SessionEntry
	if leafID != nil {
		leaf = index[*leafID]
	}
	if leaf == nil && useLast && len(entries) > 0 {
		leaf = entries[len(entries)-1]
	}
	if leaf == nil {
		return nil
	}
	var path []model.SessionEntry
	current := leaf
	for current != nil {
		path = append(path, current)
		parentID := current.Base().ParentID
		if parentID == nil {
			current = nil
			continue
		}
		current = index[*parentID]
	}
	for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
		path[i], path[j] = path[j], path[i]
	}
	return path
}

func getSessionContextSettings(path []model.SessionEntry) (string, *model.SessionModelRef) {
	thinkingLevel := "off"
	var sessionModel *model.SessionModelRef
	for _, entry := range path {
		switch typed := entry.(type) {
		case *model.ThinkingLevelChangeEntry:
			thinkingLevel = typed.ThinkingLevel
		case *model.ModelChangeEntry:
			sessionModel = &model.SessionModelRef{Provider: typed.Provider, ModelID: typed.ModelID}
		case *model.SessionMessageEntry:
			if assistant, ok := typed.Message.(model.AssistantMessage); ok {
				sessionModel = &model.SessionModelRef{Provider: assistant.Provider, ModelID: assistant.Model}
			}
		}
	}
	return thinkingLevel, sessionModel
}

// BuildContextEntries builds the active, compaction-aware entry list.
//
// A nil leafID follows the last entry in entries. Entries before the newest
// compaction's first kept entry are omitted.
func BuildContextEntries(entries []model.SessionEntry, leafID *string, byID map[string]model.SessionEntry) []model.SessionEntry {
	return buildContextEntries(entries, leafID, byID, true)
}

func buildContextEntries(entries []model.SessionEntry, leafID *string, byID map[string]model.SessionEntry, useLast bool) []model.SessionEntry {
	path := buildSessionPath(entries, leafID, byID, useLast)
	var compaction *model.CompactionEntry
	for _, entry := range path {
		if typed, ok := entry.(*model.CompactionEntry); ok {
			compaction = typed
		}
	}
	if compaction == nil {
		return path
	}

	compactionIndex := -1
	for i, entry := range path {
		if entry.Base().ID == compaction.Base().ID {
			compactionIndex = i
			break
		}
	}
	if compactionIndex < 0 {
		return path
	}

	contextEntries := []model.SessionEntry{compaction}
	foundFirstKept := false
	for i := 0; i < compactionIndex; i++ {
		entry := path[i]
		if entry.Base().ID == compaction.FirstKeptEntryID {
			foundFirstKept = true
		}
		if !foundFirstKept {
			continue
		}
		if message, ok := entry.(*model.SessionMessageEntry); ok {
			if message.Message.MessageRole() == model.RoleSystem {
				continue
			}
		}
		contextEntries = append(contextEntries, entry)
	}
	contextEntries = append(contextEntries, path[compactionIndex+1:]...)
	return contextEntries
}

// BuildSessionProjection builds provenance-preserving, compaction-aware model
// context. A nil leafID follows the last entry in entries.
func BuildSessionProjection(entries []model.SessionEntry, leafID *string, byID map[string]model.SessionEntry) model.SessionProjection {
	return buildSessionProjection(entries, leafID, byID, true)
}

func buildSessionProjection(entries []model.SessionEntry, leafID *string, byID map[string]model.SessionEntry, useLast bool) model.SessionProjection {
	path := buildSessionPath(entries, leafID, byID, useLast)
	thinkingLevel, sessionModel := getSessionContextSettings(path)
	contextEntries := buildContextEntries(entries, leafID, byID, useLast)
	edits := map[string]*model.ContextEditEntry{}
	for _, entry := range contextEntries {
		if edit, ok := entry.(*model.ContextEditEntry); ok {
			edits[edit.TargetID] = edit
		}
	}
	projected := make([]model.ProjectedSessionEntry, 0, len(contextEntries))
	for index, sourceEntry := range contextEntries {
		var messages []model.AgentMessage
		if compaction, ok := sourceEntry.(*model.CompactionEntry); ok && index > 0 {
			_ = compaction
			messages = nil
		} else {
			messages = projectContextEntry(sourceEntry, edits[sourceEntry.Base().ID])
		}
		projected = append(projected, model.ProjectedSessionEntry{SourceEntry: sourceEntry, Messages: messages})
	}
	allMessages := make([]model.AgentMessage, 0)
	for _, entry := range projected {
		allMessages = append(allMessages, entry.Messages...)
	}
	return model.SessionProjection{
		Entries:       projected,
		Messages:      allMessages,
		ThinkingLevel: thinkingLevel,
		Model:         sessionModel,
	}
}

// BuildSessionContext builds the finalized model context. A nil leafID follows
// the last entry in entries.
func BuildSessionContext(entries []model.SessionEntry, leafID *string, byID map[string]model.SessionEntry) model.SessionContext {
	projection := BuildSessionProjection(entries, leafID, byID)
	return model.SessionContext{
		Messages:      projection.Messages,
		ThinkingLevel: projection.ThinkingLevel,
		Model:         projection.Model,
	}
}

// GetLatestCompactionEntry returns the last compaction entry, or nil.
func GetLatestCompactionEntry(entries []model.SessionEntry) *model.CompactionEntry {
	for _, entrie := range slices.Backward(entries) {
		if compaction, ok := entrie.(*model.CompactionEntry); ok {
			return compaction
		}
	}
	return nil
}
