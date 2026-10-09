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
	if request := event.Request(); request != nil {
		ctx = request.Context()
	}
	manager := conversation.FromContext(ctx)
	if manager == nil {
		return ConversationsData{}, skgo.Errorf(http.StatusInternalServerError,
			"This server has no conversation manager in its runtime context.")
	}
	items := manager.List()
	data := ConversationsData{Items: items, Selected: conversation.Conversation{Messages: []conversation.Message{}}}
	selected := ""
	if selected == "" && len(items) > 0 {
		selected = items[0].ID
	}
	if item, ok := manager.Get(selected); ok {
		data.Selected = item
	}
	return data, nil
}

var _ = skgo.Load(Load)
