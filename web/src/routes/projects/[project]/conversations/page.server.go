// Package conversations serves the conversation list and selected transcript.
package conversations

import (
	"net/http"

	"github.com/tylergannon/skgo"

	"github.com/tylergannon/gimbal/internal/conversation"
)

// ConversationsData is the complete saved list and selected transcript.
type ConversationsData struct {
	Items    []conversation.Conversation `json:"items"`
	Selected conversation.Conversation   `json:"selected"`
}

func Load(event PageRequestEvent) (ConversationsData, error) {
	ctx := event.Context()
	manager := conversation.FromContext(ctx)
	if manager == nil {
		return ConversationsData{}, skgo.Errorf(http.StatusInternalServerError,
			"This server has no conversation manager in its runtime context.")
	}
	items := manager.List()
	data := ConversationsData{Items: items, Selected: conversation.Conversation{Messages: []conversation.Message{}}}
	if len(items) > 0 {
		data.Selected = items[0]
	}
	return data, nil
}

var _ = skgo.Load(Load)
