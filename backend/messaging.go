package backend

import (
	"context"
	"encoding/json"
	"time"
)

// MessageRequest identifies one admission, independently of the conversation.
// Exactly one payload corresponding to Kind is present. Destinations belong to
// plugin configuration, never the action vocabulary.
type MessageRequest struct {
	ID      string
	Created time.Time
	Owner
	SessionID   string `json:",omitempty"`
	Kind        string
	Session     *Session `json:",omitempty"`
	Turn        *Turn    `json:",omitempty"`
	Process     *Process `json:",omitempty"`
	Target      *Target  `json:",omitempty"`
	ScopeTarget *Scope   `json:",omitempty"`
}

// MessageResult carries authoritative native output; bytes survive even when
// they are not valid JSON. ID is the session created by a session request.
type MessageResult struct {
	ID             string          `json:",omitempty"`
	Output         []byte          `json:",omitempty"`
	Usage          json.RawMessage `json:",omitempty"`
	Process        *ProcessResult  `json:",omitempty"`
	Stdout, Stderr []byte
	Error          string `json:",omitempty"`
	InvalidOutput  bool   `json:",omitempty"`
}

// MessageEvent kinds are accepted, activity, established and completed.
// Activity contains the native AgentEvent envelope, including NativeRef.
type MessageEvent struct {
	ID, RequestID string
	Created       time.Time
	Owner
	SessionID string `json:",omitempty"`
	Kind      string
	Activity  json.RawMessage `json:",omitempty"`
	Result    *MessageResult  `json:",omitempty"`
}

type MessageSelection struct{ RequestID, Run, SessionID string }

func (s MessageSelection) Matches(e MessageEvent) bool {
	return (s.RequestID == "" || e.RequestID == s.RequestID) && (s.Run == "" || e.Run == s.Run) && (s.SessionID == "" || e.SessionID == s.SessionID)
}

// Messaging retains published events for the configured store lifetime.
// ListenEvents includes matching events since the inclusive lower bound,
// including events published before listening, then continues live. Consumers
// are independent. It returns callback errors unchanged and stops on ctx.
// Closing a listener never cancels native work. Publication errors describe
// delivery uncertainty and must not be interpreted as native execution failure.
type Messaging interface {
	PublishRequest(context.Context, MessageRequest) error
	PublishEvent(context.Context, MessageEvent) error
	ListenEvents(context.Context, MessageSelection, time.Time, func(MessageEvent) error) error
}

// MessageHandler admits native work and arranges completion publication without
// waiting for a turn/process to finish. A transport must keep controls available
// during concurrent admissions. Native ownership remains in the task daemon.
type MessageHandler func(context.Context, MessageRequest) error
