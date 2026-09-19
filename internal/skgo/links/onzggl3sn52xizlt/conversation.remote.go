package routes

import (
	"context"
	"net/http"
	"strings"

	"github.com/tylergannon/skgo"

	"github.com/tylergannon/gimble/internal/conversation"
)

// CreateConversation is the explicit provider and model selection for a new
// conversation. Model may be blank to use the provider's cheap default.
type CreateConversation struct {
	Title    string `json:"title"`
	Provider string `json:"provider"`
	Model    string `json:"model"`
}

// SendConversationMessage is one visible user turn.
type SendConversationMessage struct {
	Conversation string `json:"conversation"`
	Message      string `json:"message"`
}

func createConversation(ctx context.Context, arg CreateConversation) (conversation.Conversation, error) {
	manager := conversation.FromContext(ctx)
	if manager == nil {
		return conversation.Conversation{}, skgo.Errorf(http.StatusInternalServerError,
			"This server has no conversation manager in its runtime context.")
	}
	if strings.TrimSpace(arg.Provider) == "" {
		return conversation.Conversation{}, skgo.Invalidf("provider", "Choose Codex, Claude, or agy / Gemini.")
	}
	item, err := manager.Create(ctx, conversation.NewConversation{
		Title: arg.Title, Provider: arg.Provider, Model: arg.Model,
	})
	if err != nil {
		return conversation.Conversation{}, skgo.Errorf(http.StatusBadRequest, "%s", err)
	}
	return item, nil
}

func sendConversationMessage(ctx context.Context, arg SendConversationMessage) (conversation.Conversation, error) {
	manager := conversation.FromContext(ctx)
	if manager == nil {
		return conversation.Conversation{}, skgo.Errorf(http.StatusInternalServerError,
			"This server has no conversation manager in its runtime context.")
	}
	if strings.TrimSpace(arg.Message) == "" {
		return conversation.Conversation{}, skgo.Invalidf("message", "Type a message before sending it.")
	}
	item, err := manager.Send(arg.Conversation, arg.Message)
	if err != nil {
		return item, skgo.Errorf(http.StatusBadGateway, "%s", err)
	}
	return item, nil
}

var _ = skgo.Command(createConversation)

var _ = skgo.Command(sendConversationMessage)
