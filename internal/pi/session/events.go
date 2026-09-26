package session

import (
	"github.com/tylergannon/gimbal/internal/pi/compact"
	"github.com/tylergannon/gimbal/internal/pi/model"
)

// EventType is the discriminator for a session event. Agent loop events keep
// their model.AgentEventType names; session-specific events use the names from
// agent-session.ts's AgentSessionEvent union.
type EventType string

// Session event types beyond the forwarded agent loop events.
const (
	EventAgentSettled         EventType = "agent_settled"
	EventQueueUpdate          EventType = "queue_update"
	EventCompactionStart      EventType = "compaction_start"
	EventCompactionEnd        EventType = "compaction_end"
	EventEntryAppended        EventType = "entry_appended"
	EventSessionInfoChanged   EventType = "session_info_changed"
	EventThinkingLevelChanged EventType = "thinking_level_changed"
	EventAutoRetryStart       EventType = "auto_retry_start"
	EventAutoRetryEnd         EventType = "auto_retry_end"
)

// CompactionReason is why compaction ran.
type CompactionReason string

// Compaction reasons.
const (
	CompactionManual    CompactionReason = "manual"
	CompactionThreshold CompactionReason = "threshold"
	CompactionOverflow  CompactionReason = "overflow"
)

// Event is one session event. Type selects which payload fields are set. Agent
// loop events are carried whole in Agent.
type Event struct {
	Type EventType

	// Agent carries a forwarded model.AgentEvent for agent loop event types.
	Agent model.AgentEvent

	// AgentEnd fields.
	Messages  []model.AgentMessage
	WillRetry bool

	// QueueUpdate fields.
	Steering []string
	FollowUp []string

	// Compaction fields.
	Reason       CompactionReason
	Result       *compact.CompactionResult
	Aborted      bool
	ErrorMessage string

	// Auto retry fields.
	Attempt     int
	MaxAttempts int
	DelayMs     int
	Success     bool
	FinalError  string

	// EntryAppended field.
	Entry model.SessionEntry

	// ThinkingLevelChanged field.
	Level model.ThinkingLevel

	// SessionInfoChanged field.
	Name *string
}

// Listener receives session events. A listener error stops event delivery for
// that event and is returned to the agent loop.
type Listener func(event Event) error
