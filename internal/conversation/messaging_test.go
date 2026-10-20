package conversation

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/tylergannon/gimbal/internal/runstate"
)

func TestInterruptedWorkflowAcknowledgementRetriesSameInputIdentity(t *testing.T) {
	m, item := managerWithControl(t, &controlledChat{})
	var accepted []runstate.InteractionInput
	m.WorkflowInput = func(_ string, _ string, raw string) error {
		var input runstate.InteractionInput
		_ = json.Unmarshal([]byte(raw), &input)
		accepted = append(accepted, input)
		if len(accepted) == 1 {
			return errors.New("acknowledgement interrupted")
		}
		return nil
	}
	input := request("stable-input", "Hello")
	if _, err := m.Submit(item.ID, input); err == nil {
		t.Fatal("lost acknowledgement claimed accepted")
	}
	if _, err := m.Submit(item.ID, input); err != nil {
		t.Fatal(err)
	}
	if len(accepted) != 2 || accepted[0] != accepted[1] {
		t.Fatalf("retry changed durable input %+v", accepted)
	}
	saved, _ := m.Get(item.ID)
	if saved.WorkflowRun == "" || len(saved.Requests) != 1 {
		t.Fatalf("invocation or request lost %+v", saved)
	}
}
