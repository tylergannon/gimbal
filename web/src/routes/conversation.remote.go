package routes

import (
	"context"
	"net/http"
	"strings"

	"github.com/tylergannon/gimbal/internal/app"
	"github.com/tylergannon/gimbal/internal/skgo/params"
	"github.com/tylergannon/skgo"

	"github.com/tylergannon/gimbal/internal/conversation"
	"github.com/tylergannon/gimbal/internal/host"
)

// CreateConversation starts a conversation with its first message, which
// also titles it. Model may be blank to use the provider's cheap default.
type CreateConversation struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
	Message  string `json:"message"`
}

// SendConversationMessage is one visible user turn.
type SendConversationMessage struct {
	Conversation string `json:"conversation"`
	Message      string `json:"message"`
}

func createConversation(ctx context.Context, event app.RequestEvent[params.Params], arg CreateConversation) (conversation.Conversation, error) {
	manager := conversation.FromContext(ctx)
	if manager == nil {
		return conversation.Conversation{}, skgo.Errorf(http.StatusInternalServerError,
			"This server has no conversation manager in its runtime context.")
	}
	if strings.TrimSpace(arg.Provider) == "" {
		return conversation.Conversation{}, skgo.Invalidf("provider", "Choose Codex, Claude, or agy / Gemini.")
	}
	if strings.TrimSpace(arg.Message) == "" {
		return conversation.Conversation{}, skgo.Invalidf("message", "Describe the work to start the session.")
	}
	item, err := manager.Start(ctx, conversation.NewConversation{Provider: arg.Provider, Model: arg.Model}, arg.Message)
	if err != nil {
		return conversation.Conversation{}, skgo.Errorf(http.StatusBadRequest, "%s", err)
	}
	return item, skgo.RefreshRequested(ctx, projectConversations, 1)
}

func sendConversationMessage(ctx context.Context, event app.RequestEvent[params.Params], arg SendConversationMessage) (conversation.Conversation, error) {
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
	return item, skgo.RefreshRequested(ctx, projectConversations, 1)
}

var _ = skgo.Form(createConversation)

var _ = skgo.Form(sendConversationMessage)

// projectConversations reads the existing project-owned chat inventory.
func projectConversations(ctx context.Context, project string) ([]conversation.Conversation, error) {
	manager, err := projectManager(ctx, project)
	if err != nil {
		return nil, err
	}
	return manager.List(), nil
}

// conversationSetup is what the new session composer offers in a project.
func conversationSetup(ctx context.Context, project string) (conversation.Setup, error) {
	manager, err := projectManager(ctx, project)
	if err != nil {
		return conversation.Setup{}, err
	}
	return manager.Setup(ctx)
}

func projectManager(ctx context.Context, project string) (*conversation.Manager, error) {
	if owner := host.OwnerFrom(ctx); owner != nil {
		for _, p := range owner.Projects() {
			if p.ID() == project {
				return p.Conversations(), nil
			}
		}
	}
	return nil, skgo.Errorf(http.StatusNotFound, "Project not found")
}

var _ = skgo.Query(projectConversations)

var _ = skgo.Query(conversationSetup)
