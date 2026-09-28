package execution

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/tylergannon/gimbal"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
)

type harnessProxy struct {
	environment *Environment
	role        gimbal.WorkflowRole
	binding     RoleBinding
}

// completedHarness is a harness activity's outcome as its Get reports it.
type completedHarness struct {
	result harnessResult
	err    error
}

// start schedules one harness activity and collects its outcome. The caller
// must settle it: every return path either has the outcome or cancels.
func (p *harnessProxy) start(ctx context.Context, input *harnessInput) (client.ActivityHandle, <-chan completedHarness, context.CancelFunc, error) {
	if err := p.environment.removal(); err != nil {
		return nil, nil, nil, err
	}
	input.Environment = p.environment.bootstrap.Name
	input.Role = string(p.role)
	input.Operation = uuid.NewString()
	handle, err := p.environment.backend.schedule(ctx, input.Operation, p.environment.bootstrap.Queue, harnessActivity, *input)
	if err != nil {
		return nil, nil, nil, p.environment.finish(fmt.Errorf("execution: schedule harness %s: %w", input.Action, err))
	}
	getCtx, cancelGet := context.WithCancel(context.Background())
	completed := make(chan completedHarness, 1)
	go func() {
		var result harnessResult
		err := handle.Get(getCtx, &result)
		completed <- completedHarness{result, err}
	}()
	return handle, completed, cancelGet, nil
}

// settle classifies a completed harness activity. An unconfirmed completion
// removes the environment, since nothing else can stop what it left running.
func (p *harnessProxy) settle(action string, err error) error {
	if err == nil {
		return nil
	}
	err = fmt.Errorf("execution: harness %s: %w", action, err)
	if activityUnconfirmed(err) {
		return errors.Join(err, p.environment.remove())
	}
	return p.environment.finish(err)
}

// cancel asks a running harness activity to stop because ctx ended, and waits
// a bounded time for the worker to confirm it did. Confirmation is the same
// classification as settle; no confirmation removes the environment.
func (p *harnessProxy) cancel(ctx context.Context, action string, handle client.ActivityHandle, completed <-chan completedHarness) error {
	cancelCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	cancelErr := handle.Cancel(cancelCtx, client.CancelActivityOptions{Reason: ctx.Err().Error()})
	cancel()
	select {
	case done := <-completed:
		if temporal.IsCanceledError(done.err) {
			done.err = nil
		}
		return p.environment.finish(errors.Join(ctx.Err(), cancelErr, p.settle(action, done.err)))
	case <-time.After(activityWaitTimeout):
		return errors.Join(ctx.Err(), cancelErr, fmt.Errorf("execution: harness %s cancellation was not confirmed", action), p.environment.remove())
	}
}

func (p *harnessProxy) request(ctx context.Context, input harnessInput) (harnessResult, error) {
	handle, completed, stopGet, err := p.start(ctx, &input)
	if err != nil {
		return harnessResult{}, err
	}
	defer stopGet()
	select {
	case done := <-completed:
		if err := p.settle(input.Action, done.err); err != nil {
			return done.result, err
		}
		if done.result.RecordingError != "" {
			log.Printf("gimbal: harness event recording degraded for %s: %s", input.Operation, done.result.RecordingError)
		}
		return done.result, nil
	case <-ctx.Done():
		return harnessResult{}, p.cancel(ctx, input.Action, handle, completed)
	}
}

func (p *harnessProxy) CreateSession(ctx context.Context, model, effort, workdir string) (string, error) {
	if model != p.binding.Model || effort != p.binding.Effort {
		return "", fmt.Errorf("execution: role %q is bound to Codex model %q effort %q, got model %q effort %q", p.role, p.binding.Model, p.binding.Effort, model, effort)
	}
	if !mounted(p.environment.bootstrap.Mounts, workdir) {
		return "", fmt.Errorf("execution: session workdir %q is outside configured mounts", workdir)
	}
	result, err := p.request(ctx, harnessInput{Action: "create", Model: p.binding.Model, Effort: p.binding.Effort, Workdir: workdir})
	return result.Session, err
}

func (p *harnessProxy) Fork(ctx context.Context, sessionID string) (string, error) {
	result, err := p.request(ctx, harnessInput{Action: "fork", Session: sessionID})
	return result.Session, err
}

func (p *harnessProxy) RunTurn(ctx context.Context, sessionID, prompt string, schema json.RawMessage, onEvent func(gimbal.AgentEvent) error) (gimbal.TurnResult, error) {
	input := harnessInput{Action: "turn", Session: sessionID, Model: p.binding.Model, Effort: p.binding.Effort, Prompt: prompt, Schema: schema}
	handle, completed, stopGet, err := p.start(ctx, &input)
	if err != nil {
		return gimbal.TurnResult{}, err
	}
	defer stopGet()

	// Events are independently persisted by the worker and tailed while the
	// activity runs, so supervision and local run roll-ups see live provider
	// events. Observation failures never cancel an authoritative provider turn.
	var lastID int64
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		var pollErr error
		lastID, pollErr = p.forwardEvents(ctx, input, lastID, onEvent)
		if pollErr != nil {
			log.Printf("gimbal: harness event relay degraded for %s: %v", input.Operation, pollErr)
		}
		select {
		case done := <-completed:
			if err := p.settle(input.Action, done.err); err != nil {
				return gimbal.TurnResult{}, err
			}
			drainCtx, cancelDrain := context.WithTimeout(context.Background(), time.Second)
			_, tailErr := p.forwardEvents(drainCtx, input, lastID, onEvent)
			cancelDrain()
			if tailErr != nil {
				log.Printf("gimbal: final harness event relay degraded for %s: %v", input.Operation, tailErr)
			}
			if done.result.RecordingError != "" {
				log.Printf("gimbal: harness event recording degraded for %s: %s", input.Operation, done.result.RecordingError)
			}
			return done.result.Turn, nil
		case <-ticker.C:
		case <-ctx.Done():
			return gimbal.TurnResult{}, p.cancel(ctx, input.Action, handle, completed)
		}
	}
}

func (p *harnessProxy) forwardEvents(ctx context.Context, input harnessInput, after int64, onEvent func(gimbal.AgentEvent) error) (int64, error) {
	queryCtx, cancel := context.WithTimeout(ctx, 300*time.Millisecond)
	defer cancel()
	rows, err := p.environment.backend.db.Query(queryCtx, `SELECT id,event FROM gimbal_harness_events WHERE environment_name=$1 AND operation_id=$2 AND id>$3 ORDER BY id`, input.Environment, input.Operation, after)
	if err != nil {
		return after, err
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var raw []byte
		if err := rows.Scan(&id, &raw); err != nil {
			return after, err
		}
		var event gimbal.AgentEvent
		if err := json.Unmarshal(raw, &event); err != nil {
			return after, err
		}
		if onEvent != nil {
			if err := onEvent(event); err != nil {
				log.Printf("gimbal: local event callback failed for %s: %v", input.Operation, err)
			}
		}
		after = id
	}
	return after, rows.Err()
}

func (p *harnessProxy) Steer(ctx context.Context, sessionID, message string) (bool, error) {
	result, err := p.request(ctx, harnessInput{Action: "steer", Session: sessionID, Message: message})
	return result.Landed, err
}

func (p *harnessProxy) Close(ctx context.Context, sessionID string) error {
	_, err := p.request(ctx, harnessInput{Action: "close", Session: sessionID})
	return err
}
