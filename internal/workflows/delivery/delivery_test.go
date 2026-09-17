package delivery

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/tylergannon/gimble"
)

type deliveryHarness struct {
	mu            sync.Mutex
	plans         int
	prompts       []string
	created       int
	closedAtPlan  []int
	verdicts      []bool
	blocks        bool
	started       chan struct{}
	closed        []string
	cancelOnClose context.CancelFunc
}

func (h *deliveryHarness) CreateSession(context.Context, string, string, string) (string, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.created++
	return "session-" + strconv.Itoa(h.created), nil
}
func (h *deliveryHarness) Fork(context.Context, string) (string, error)        { return "fork", nil }
func (h *deliveryHarness) Steer(context.Context, string, string) (bool, error) { return false, nil }
func (h *deliveryHarness) Close(_ context.Context, session string) error {
	h.mu.Lock()
	h.closed = append(h.closed, session)
	cancel := h.cancelOnClose
	h.cancelOnClose = nil
	h.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	return nil
}
func (h *deliveryHarness) RunTurn(ctx context.Context, _ string, prompt string, schema json.RawMessage, _ func(gimble.AgentEvent) error) (gimble.TurnResult, error) {
	if strings.Contains(prompt, "You plan the loop") {
		h.mu.Lock()
		h.prompts = append(h.prompts, prompt)
		h.plans++
		plan := h.plans
		h.closedAtPlan = append(h.closedAtPlan, len(h.closed))
		h.mu.Unlock()
		if plan > len(h.verdicts) && len(h.verdicts) == 0 {
			return gimble.TurnResult{Output: json.RawMessage(`{"tasks":[],"next":null}`)}, nil
		}
		return gimble.TurnResult{Output: json.RawMessage(`{"tasks":[{"name":"task","description":"do it","definition_of_done":"it works","validation":{"command":"","query":""}}],"next":0}`)}, nil
	}
	if strings.Contains(prompt, "Independently inspect") {
		h.mu.Lock()
		satisfied := true
		if len(h.verdicts) > 0 {
			satisfied = h.verdicts[0]
			h.verdicts = h.verdicts[1:]
		}
		h.mu.Unlock()
		if satisfied {
			return gimble.TurnResult{Output: json.RawMessage(`{"satisfied":true,"findings":[]}`)}, nil
		}
		return gimble.TurnResult{Output: json.RawMessage(`{"satisfied":false,"findings":["still incomplete"]}`)}, nil
	}
	if h.blocks {
		if h.started != nil {
			close(h.started)
			h.started = nil
		}
		<-ctx.Done()
		return gimble.TurnResult{}, ctx.Err()
	}
	return gimble.TurnResult{Output: json.RawMessage(`"implemented"`)}, nil
}

func deliveryInput(t *testing.T, validation string, max int) Input {
	t.Helper()
	brief := t.TempDir()
	path := brief + "/brief.md"
	if err := os.WriteFile(path, []byte("Implement the feature."), 0o644); err != nil {
		t.Fatal(err)
	}
	return Input{WorkDir: brief, Brief: "brief.md", Validation: validation, MaxTasks: max}
}

func runDelivery(t *testing.T, h *deliveryHarness, in Input) error {
	t.Helper()
	return gimble.Run(gimble.Project(t.Context(), t.TempDir()), "delivery-test", map[string]gimble.ModelBinding{
		"planner": {Adapter: h, Model: "model"}, "implementer": {Adapter: h, Model: "model"}, "validator": {Adapter: h, Model: "model"},
	}, func(ctx context.Context) error { return Delivery(ctx, in) })
}

func TestDeliverySuccess(t *testing.T) {
	h := &deliveryHarness{verdicts: []bool{true}}
	if err := runDelivery(t, h, deliveryInput(t, "true", 2)); err != nil {
		t.Fatal(err)
	}
	if len(h.closed) != 3 {
		t.Fatalf("closed sessions = %d, want planner, implementer, validator", len(h.closed))
	}
}

func TestDeliveryFailedValidationCannotBeOverridden(t *testing.T) {
	h := &deliveryHarness{verdicts: []bool{true}}
	if err := runDelivery(t, h, deliveryInput(t, "false", 1)); err == nil {
		t.Fatal("failed validation was accepted")
	}
	if h.plans != 1 {
		t.Fatalf("planner turns = %d, want one with exhausted budget", h.plans)
	}
}

func TestDeliveryValidatorFailureReplans(t *testing.T) {
	h := &deliveryHarness{verdicts: []bool{false, true}}
	in := deliveryInput(t, "printf validation-output", 2)
	if err := runDelivery(t, h, in); err != nil {
		t.Fatal(err)
	}
	if h.plans != 2 {
		t.Fatalf("planner turns = %d, want two", h.plans)
	}
	if h.closedAtPlan[1] != 2 {
		t.Fatalf("sessions closed before second plan = %d, want implementer and validator", h.closedAtPlan[1])
	}
	if !strings.Contains(h.prompts[1], "still incomplete") || !strings.Contains(h.prompts[1], "validation-output") {
		t.Fatalf("second planner prompt lacks validator or command evidence:\n%s", h.prompts[1])
	}
	if !strings.Contains(h.prompts[0], "Implement the complete brief in") || !strings.Contains(h.prompts[1], "Implement the complete brief in") {
		t.Fatal("planner goal was not retained")
	}
}

func TestDeliveryEmptyPlannerIsFailure(t *testing.T) {
	h := &deliveryHarness{}
	err := runDelivery(t, h, deliveryInput(t, "true", 2))
	if err == nil || !strings.Contains(err.Error(), "without satisfying") {
		t.Fatalf("error = %v, want unsatisfied empty plan", err)
	}
	if h.plans != 1 {
		t.Fatalf("planner turns = %d, want one", h.plans)
	}
}

func TestDeliveryCancellationClosesSessions(t *testing.T) {
	started := make(chan struct{})
	h := &deliveryHarness{verdicts: []bool{true}, blocks: true, started: started}
	brief := deliveryInput(t, "true", 1)
	err := gimble.Run(gimble.Project(t.Context(), t.TempDir()), "delivery-test", map[string]gimble.ModelBinding{
		"planner": {Adapter: h, Model: "model"}, "implementer": {Adapter: h, Model: "model"}, "validator": {Adapter: h, Model: "model"},
	}, func(ctx context.Context) error {
		cancelled, cancel := context.WithCancel(ctx)
		go func() { <-started; cancel() }()
		return Delivery(cancelled, brief)
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want cancellation", err)
	}
	if len(h.closed) < 2 {
		t.Fatalf("closed sessions = %d, want planner and implementer", len(h.closed))
	}
}

func TestDeliveryCancellationDuringCleanupCannotSucceed(t *testing.T) {
	h := &deliveryHarness{verdicts: []bool{true}}
	brief := deliveryInput(t, "true", 1)
	err := gimble.Run(gimble.Project(t.Context(), t.TempDir()), "delivery-test", map[string]gimble.ModelBinding{
		"planner": {Adapter: h, Model: "model"}, "implementer": {Adapter: h, Model: "model"}, "validator": {Adapter: h, Model: "model"},
	}, func(ctx context.Context) error {
		cancelled, cancel := context.WithCancel(ctx)
		h.cancelOnClose = cancel
		return Delivery(cancelled, brief)
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want cancellation during cleanup", err)
	}
}
