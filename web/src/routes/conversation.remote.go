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
		return conversation.Conversation{}, skgo.Invalidf("", "%s", err)
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
		return item, skgo.Invalidf("", "%s", err)
	}
	return item, nil
}

func watchConversation(ctx context.Context, id string, yield func(conversation.Conversation) error) error {
	event := skgo.EventFrom(ctx)
	if request := event.Request(); request != nil {
		ctx = request.Context()
	}
	manager := conversation.FromContext(ctx)
	if manager == nil {
		return skgo.Errorf(http.StatusInternalServerError,
			"This server has no conversation manager in its runtime context.")
	}

	for {
		changes := manager.Changes()
		item, ok := manager.Get(id)
		if !ok {
			return skgo.Errorf(http.StatusNotFound, "There is no conversation %s in this project.", id)
		}
		if err := yield(item); err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return nil
		case <-changes:
		}
	}
}

var _ = skgo.Form(createConversation)

var _ = skgo.Form(sendConversationMessage)

var _ = skgo.LiveQuery(watchConversation)
