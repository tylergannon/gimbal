// Package conversationid serves one conversation at its canonical path.
package conversationid

import (
	"context"
	"net/http"

	"github.com/tylergannon/skgo"

	"github.com/tylergannon/gimble/internal/conversation"
)

// Data is the initial snapshot for the selected conversation.
type Data struct {
	Selected conversation.Conversation `json:"selected"`
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
	conversationID := event.Param("conversationID")
	selected, ok := manager.Get(conversationID)
	if !ok {
		return Data{}, skgo.Errorf(http.StatusNotFound, "There is no conversation %s in this project.", conversationID)
	}
	return Data{Selected: selected}, nil
}

var _ = skgo.Load(load)
