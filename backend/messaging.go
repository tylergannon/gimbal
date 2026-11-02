package backend

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// MessageRequest identifies one admission, independently of the conversation.
// Exactly one payload corresponding to Kind is present. Environment is a
// logical destination; physical connection details belong to plugin configuration.
type MessageRequest struct {
	ID          string
	Created     time.Time
	Environment string `json:",omitempty"`
	WorkerID    string `json:",omitempty"`
	Owner
	SessionID         string `json:",omitempty"`
	ExecutionIdentity ExecutionIdentity
	Kind              string
	Session           *Session `json:",omitempty"`
	Turn              *Turn    `json:",omitempty"`
	Process           *Process `json:",omitempty"`
	Target            *Target  `json:",omitempty"`
	ScopeTarget       *Scope   `json:",omitempty"`
}

// MessageResult carries the request's returned outcome and native data, not
// just successful output. Interrupted reports authoritative native turn
// cancellation; it is never inferred from Error text. Process.Interrupted
// retains the native process outcome. Unresolved marks controls whose cessation
// could not be confirmed. Output bytes survive even when invalid JSON; ID is
// the session created by a session request. Value reports whether steering landed.
type MessageResult struct {
	Value          bool            `json:",omitempty"`
	ReadyResources []ResourceFact  `json:",omitempty"`
	Interrupted    bool            `json:",omitempty"`
	Unresolved     bool            `json:",omitempty"`
	ID             string          `json:",omitempty"`
	Output         []byte          `json:",omitempty"`
	Usage          json.RawMessage `json:",omitempty"`
	Process        *ProcessResult  `json:",omitempty"`
	Stdout, Stderr []byte
	Error          string `json:",omitempty"`
	InvalidOutput  bool   `json:",omitempty"`
}

// MessageEvent kinds are accepted, activity, established and completed.
// Completed settles the request with its returned outcome; it does not assert
// native success or cessation. Inspect Result for failure, authoritative
// interruption, or unresolved control delivery. Activity contains the native
// AgentEvent envelope, including NativeRef. An activity with Resource
// instead records the supplied dependency's declared readiness/access, without
// an address, implementation handle or fabricated provider-native event.
type MessageEvent struct {
	ID, RequestID string
	Created       time.Time
	Environment   string `json:",omitempty"`
	WorkerID      string `json:",omitempty"`
	Owner
	SessionID string `json:",omitempty"`
	Kind      string
	Resource  *ResourceFact   `json:",omitempty"`
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
// are independent. The since bound is diagnostic; execution waits must replay
// the complete matching request because publisher clocks may differ. It returns
// callback errors unchanged and stops on ctx.
// Closing a listener never cancels native work. Publication errors describe
// delivery uncertainty and must not be interpreted as native execution failure.
type Messaging interface {
	PublishRequest(context.Context, MessageRequest) error
	PublishEvent(context.Context, MessageEvent) error
	ListenEvents(context.Context, MessageSelection, time.Time, func(MessageEvent) error) error
}

// NamedMessaging reports whether this connection owns logical placement.
type NamedMessaging interface {
	NamedEnvironments() bool
}

// MessageHandler admits native work and arranges completion publication without
// waiting for a turn/process to finish. A transport must keep controls available
// during concurrent admissions. Native ownership remains in the task daemon.
type MessageHandler func(context.Context, MessageRequest) error

// Validate rejects ambiguous requests before admission or native effects. The
// envelope owns the action; repeated payload identities must agree exactly.
func (r MessageRequest) Validate() error {
	if r.ID == "" || r.Created.IsZero() {
		return fmt.Errorf("request ID and creation time required")
	}
	count := 0
	for _, present := range []bool{r.Session != nil, r.Turn != nil, r.Process != nil, r.Target != nil, r.ScopeTarget != nil} {
		if present {
			count++
		}
	}
	if count != 1 {
		return fmt.Errorf("exactly one operation payload required")
	}
	if Identified(r.Kind) {
		kind := strings.TrimPrefix(r.Kind, "identified-")
		if err := r.ExecutionIdentity.Validate(kind); err != nil {
			return err
		}
		var supplied ExecutionIdentity
		switch kind {
		case "session":
			if r.Session != nil {
				supplied = r.Session.ExecutionIdentity
			}
		case "turn":
			if r.Turn != nil {
				supplied = r.Turn.ExecutionIdentity
			}
		case "process":
			if r.Process != nil {
				supplied = r.Process.ExecutionIdentity
			}
		case "close":
			if r.Target != nil {
				supplied = r.Target.ExecutionIdentity
			}
		}
		if supplied != r.ExecutionIdentity || r.ID != supplied.RequestID {
			return &ContractRejected{Message: "conflicting envelope execution identity"}
		}
	}
	var owner Owner
	var session string
	switch r.Kind {
	case "identified-session":
		if r.Session == nil {
			return fmt.Errorf("session payload required")
		}
		owner = r.Session.Owner
		session = owner.Task
		if r.Session.Parent != "" {
			return fmt.Errorf("fork is unsupported on messaging routes")
		}
	case "identified-turn":
		if r.Turn == nil {
			return fmt.Errorf("turn payload required")
		}
		owner = r.Turn.Owner
		session = r.Turn.Session
		if session == "" {
			return fmt.Errorf("turn session required")
		}
	case "identified-process":
		if r.Process == nil {
			return fmt.Errorf("process payload required")
		}
		owner = r.Process.Owner
	case "identified-close", "interrupt", "steer":
		if r.Target == nil {
			return fmt.Errorf("target required")
		}
		if r.Target.Run != r.Run || r.Target.ID == "" {
			return fmt.Errorf("conflicting or absent target identity")
		}
		if r.Kind == "identified-close" || r.Kind == "steer" {
			session = r.Target.ID
		}
		owner = r.Owner
	case "cleanup":
		if r.ScopeTarget == nil {
			return fmt.Errorf("scope payload required")
		}
		if r.ScopeTarget.Run != r.Run || r.ScopeTarget.Scope != r.Scope {
			return fmt.Errorf("conflicting scope identity")
		}
		owner = r.Owner
	default:
		return fmt.Errorf("unsupported messaging action %q", r.Kind)
	}
	if owner != r.Owner || session != r.SessionID {
		return fmt.Errorf("conflicting request owner or session identity")
	}
	if (r.Kind == "identified-session" || r.Kind == "identified-turn" || r.Kind == "identified-process") && r.Task == "" {
		return fmt.Errorf("task identity required")
	}
	if r.Run == "" {
		return fmt.Errorf("run identity required")
	}
	return nil
}

// DeliveryUncertain means a caller could not obtain an authoritative outcome.
// It never establishes native failure or cessation; the request remains owned.
type DeliveryUncertain struct {
	RequestID string
	// Accepted is set only by an executor that saw the native operation
	// acknowledge the request: delivery is certain and only its outcome or
	// cessation is not. A RequestID alone never implies it.
	Accepted bool
	Err      error
}

func (e *DeliveryUncertain) Error() string {
	return fmt.Sprintf("request %s outcome unresolved: %v", e.RequestID, e.Err)
}
func (e *DeliveryUncertain) Unwrap() error { return e.Err }
