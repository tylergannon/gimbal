// Package conversations serves the saved conversation list.
package conversations

import (
	"context"
	"net/http"

	"github.com/tylergannon/skgo"

	"github.com/tylergannon/gimble/internal/conversation"
)

// Data is the saved list rendered in the conversation layout.
type Data struct {
	Items []conversation.Conversation `json:"items"`
}

func load(ctx context.Context) (Data, error) {
	event := skgo.EventFrom(ctx)
	if request := event.Request(); request != nil {
		ctx = request.Context()
	}
	manager := conversation.FromContext(ctx)
	if manager == nil {
		return Data{}, skgo.Errorf(http.StatusInternalServerError,
			"This server has no conversation manager in its runtime context.")
	}
	return Data{Items: manager.List()}, nil
}

var _ = skgo.Load(load)
