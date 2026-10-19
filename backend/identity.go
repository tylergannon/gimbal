package backend

import (
	"fmt"
	"github.com/oklog/ulid/v2"
	"strings"
)

// ExecutionIdentity binds a logical workflow node to the operation admitted by
// the native executor. Owner retains hierarchical addresses for control routes.
type ExecutionIdentity struct {
	ScopeID, SessionID, ActionID, RequestID string
	ExecutorSession, ExecutorOperation      string
}

// NewExecutionIdentity allocates identities for standalone operation producers.
// Workflow engines supply their already recorded node IDs instead.
func NewExecutionIdentity(kind string) ExecutionIdentity {
	id := ulid.Make().String()
	i := ExecutionIdentity{ActionID: id, RequestID: id, ExecutorOperation: "turn_" + id}
	if kind == "session" {
		i.SessionID = id
		i.ExecutorSession = "session_" + id
	}
	if kind == "process" {
		i.ExecutorOperation = "command_" + id
	}
	return i
}
func (i ExecutionIdentity) Validate(kind string) error {
	for _, id := range []string{i.ScopeID, i.SessionID, i.ActionID, i.RequestID} {
		if id != "" {
			if _, e := ulid.ParseStrict(id); e != nil {
				return &ContractRejected{Message: "invalid execution identity"}
			}
		}
	}
	if i.RequestID == "" || i.ActionID == "" {
		return &ContractRejected{Message: "execution identity is required"}
	}
	prefix := "turn_"
	if kind == "process" {
		prefix = "command_"
	}
	if i.ExecutorOperation != prefix+i.ActionID {
		return &ContractRejected{Message: "executor operation does not match action identity"}
	}
	if kind == "session" && (i.SessionID == "" || i.ExecutorSession != "session_"+i.SessionID) {
		return &ContractRejected{Message: "executor session does not match session identity"}
	}
	return nil
}

// ContractRejected establishes that this request was not admitted. It must not
// enter reconciliation polling or be wrapped as an uncertain native outcome.
type ContractRejected struct{ Message string }

func (e *ContractRejected) Error() string {
	return fmt.Sprintf("execution contract rejected; upgrade Gimbal on the task host: %s", e.Message)
}
func Identified(op string) bool { return strings.HasPrefix(op, "identified-") }
