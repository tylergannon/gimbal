package gimbal_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/tylergannon/gimbal"
)

func TestPublishedEventTypesAreUsableOutsideGimbal(t *testing.T) {
	lifecycle := []gimbal.LifecycleEvent{
		gimbal.RunStarted{},
		gimbal.RunEnded{},
		gimbal.RunCancelled{},
		gimbal.ScopeBegan{},
		gimbal.ScopeEnded{},
		gimbal.PlannerDecision{},
		gimbal.ValueSet{},
		gimbal.SessionCreated{},
		gimbal.SessionClosed{},
		gimbal.TurnStarted{},
		gimbal.TurnEnded{Usage: []gimbal.ModelUsage{}},
		gimbal.InterviewQuestionAsked{},
		gimbal.InterviewQuestionAnswered{},
		gimbal.SuperviseAttached{},
		gimbal.Steer{},
		gimbal.Killed{},
		gimbal.Complete{},
	}
	agent := gimbal.AgentEvent{Type: "session.execution.started", ID: "evt_1", Created: 1, Data: json.RawMessage(`{"sessionID":"ses_1"}`)}

	lifecycleRaw, err := json.Marshal(gimbal.LifecycleRecord{Seq: 1, Time: time.Now().UTC(), Event: lifecycle[0]})
	if err != nil {
		t.Fatal(err)
	}
	var lifecycleRecord gimbal.LifecycleRecord
	if err := json.Unmarshal(lifecycleRaw, &lifecycleRecord); err != nil {
		t.Fatal(err)
	}
	if _, ok := lifecycleRecord.Event.(gimbal.RunStarted); !ok {
		t.Fatalf("decoded lifecycle event = %T", lifecycleRecord.Event)
	}

	agentRaw, err := json.Marshal(gimbal.AgentRecord{Seq: 1, Time: time.Now().UTC(), Session: "coder.1", Turn: "coder.1/turn.1", Event: agent})
	if err != nil {
		t.Fatal(err)
	}
	var agentRecord gimbal.AgentRecord
	if err := json.Unmarshal(agentRaw, &agentRecord); err != nil {
		t.Fatal(err)
	}
	if agentRecord.Event.Type != "session.execution.started" {
		t.Fatalf("decoded agent event = %#v", agentRecord.Event)
	}
}
