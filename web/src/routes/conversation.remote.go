package routes

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/tylergannon/gimbal/internal/app"
	"github.com/tylergannon/gimbal/internal/skgo/params"
	"github.com/tylergannon/skgo"

	"github.com/tylergannon/gimbal/internal/conversation"
	"github.com/tylergannon/gimbal/internal/host"
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
	ID           string `json:"id"`
	Created      int64  `json:"created"`
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
	item, err := manager.Create(ctx, conversation.NewConversation{
		Title: arg.Title, Provider: arg.Provider, Model: arg.Model,
	})
	if err != nil {
		return conversation.Conversation{}, skgo.Errorf(http.StatusBadRequest, "%s", err)
	}
	return item, nil
}

func sendConversationMessage(ctx context.Context, event app.RequestEvent[params.Params], arg SendConversationMessage) (conversation.RequestState, error) {
	manager := conversation.FromContext(ctx)
	if manager == nil {
		return conversation.RequestState{}, skgo.Errorf(500, "Conversation manager unavailable")
	}
	if strings.TrimSpace(arg.Message) == "" {
		return conversation.RequestState{}, skgo.Invalidf("message", "Type a message before sending it.")
	}
	result, err := manager.Submit(arg.Conversation, conversation.Request{ID: arg.ID, Created: arg.Created, Kind: "prompt", Message: arg.Message})
	if err != nil {
		return result, skgo.Errorf(400, "%s", err)
	}
	return result, nil
}

type StopConversationRequest struct {
	Conversation string `json:"conversation"`
	ID           string `json:"id"`
	Created      int64  `json:"created"`
	Target       string `json:"target"`
}

func stopConversationRequest(ctx context.Context, event app.RequestEvent[params.Params], arg StopConversationRequest) (conversation.RequestState, error) {
	manager := conversation.FromContext(ctx)
	if manager == nil {
		return conversation.RequestState{}, skgo.Errorf(500, "Conversation manager unavailable")
	}
	result, err := manager.Submit(arg.Conversation, conversation.Request{ID: arg.ID, Created: arg.Created, Kind: "interrupt", Target: arg.Target})
	if err != nil {
		return result, skgo.Errorf(400, "%s", err)
	}
	return result, nil
}

type WatchConversation struct {
	Project      string `json:"project"`
	Conversation string `json:"conversation"`
}

func watchConversation(ctx context.Context, arg WatchConversation, yield func(conversation.Conversation) error) error {
	manager := conversation.FromContext(ctx)
	if owner := host.OwnerFrom(ctx); owner != nil {
		for _, p := range owner.Projects() {
			if p.ID() == arg.Project {
				manager = p.Conversations()
				break
			}
		}
	}
	if manager == nil {
		return skgo.Errorf(404, "Project not found")
	}
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	var previous uint64
	first := true
	for {
		item, ok := manager.Get(arg.Conversation)
		if !ok {
			return skgo.Errorf(404, "Conversation not found")
		}
		if first || item.Revision != previous {
			if err := yield(item); err != nil {
				return err
			}
			previous = item.Revision
			first = false
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

var _ = skgo.Command(stopConversationRequest)
var _ = skgo.LiveQuery(watchConversation)

var _ = skgo.Form(createConversation)

var _ = skgo.Form(sendConversationMessage)

// projectConversations reads the existing project-owned chat inventory.
func projectConversations(ctx context.Context, project string) ([]conversation.Conversation, error) {
	if owner := host.OwnerFrom(ctx); owner != nil {
		for _, p := range owner.Projects() {
			if p.ID() == project {
				return p.Conversations().List(), nil
			}
		}
	}
	return nil, skgo.Errorf(http.StatusNotFound, "Project not found")
}

var _ = skgo.Query(projectConversations)
