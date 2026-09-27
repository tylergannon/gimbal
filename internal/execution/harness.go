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

func (p *harnessProxy) request(ctx context.Context, input harnessInput) (harnessResult, error) {
	input.Environment = p.environment.bootstrap.Name
	input.Role = string(p.role)
	input.Operation = uuid.NewString()
	handle, err := p.environment.backend.temporal.ExecuteActivity(ctx, client.StartActivityOptions{
		ID: input.Operation, TaskQueue: p.environment.bootstrap.Queue,
		ScheduleToStartTimeout: 30 * time.Second, StartToCloseTimeout: 24 * time.Hour,
		HeartbeatTimeout: 15 * time.Second, RetryPolicy: &temporal.RetryPolicy{MaximumAttempts: 1},
	}, harnessActivity, input)
	if err != nil {
		return harnessResult{}, fmt.Errorf("execution: schedule harness %s: %w", input.Action, err)
	}
	getCtx, cancelGet := context.WithCancel(context.Background())
	defer cancelGet()
	got := make(chan struct {
		result harnessResult
		err    error
	}, 1)
	go func() {
		var result harnessResult
		err := handle.Get(getCtx, &result)
		got <- struct {
			result harnessResult
			err    error
		}{result, err}
	}()
	var result harnessResult
	select {
	case completed := <-got:
		result = completed.result
		if completed.err != nil {
			return result, fmt.Errorf("execution: harness %s: %w", input.Action, completed.err)
		}
	case <-ctx.Done():
		cancelCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		cancelErr := handle.Cancel(cancelCtx, client.CancelActivityOptions{Reason: ctx.Err().Error()})
		cancel()
		select {
		case <-got:
		case <-time.After(10 * time.Second):
			return result, errors.Join(ctx.Err(), cancelErr, fmt.Errorf("execution: harness %s cancellation was not confirmed", input.Action))
		}
		return result, errors.Join(ctx.Err(), cancelErr)
	}
	if result.RecordingError != "" {
		log.Printf("gimbal: harness event recording degraded for %s: %s", input.Operation, result.RecordingError)
	}
	return result, nil
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
	input.Environment = p.environment.bootstrap.Name
	input.Role = string(p.role)
	input.Operation = uuid.NewString()
	handle, err := p.environment.backend.temporal.ExecuteActivity(ctx, client.StartActivityOptions{
		ID: input.Operation, TaskQueue: p.environment.bootstrap.Queue,
		ScheduleToStartTimeout: 30 * time.Second, StartToCloseTimeout: 24 * time.Hour,
		HeartbeatTimeout: 15 * time.Second, RetryPolicy: &temporal.RetryPolicy{MaximumAttempts: 1},
	}, harnessActivity, input)
	if err != nil {
		return gimbal.TurnResult{}, fmt.Errorf("execution: schedule harness turn: %w", err)
	}

	// Events are independently persisted by the worker and tailed while the
	// activity runs, so supervision and local run roll-ups see live provider
	// events. Observation failures never cancel an authoritative provider turn.
	type completedTurn struct {
		result harnessResult
		err    error
	}
	getCtx, cancelGet := context.WithCancel(context.Background())
	defer cancelGet()
	completed := make(chan completedTurn, 1)
	go func() {
		var result harnessResult
		getErr := handle.Get(getCtx, &result)
		completed <- completedTurn{result, getErr}
	}()
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
			if done.err != nil {
				return gimbal.TurnResult{}, fmt.Errorf("execution: harness turn: %w", done.err)
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
			cancelCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			cancelErr := handle.Cancel(cancelCtx, client.CancelActivityOptions{Reason: ctx.Err().Error()})
			cancel()
			select {
			case <-completed:
				return gimbal.TurnResult{}, errors.Join(ctx.Err(), cancelErr)
			case <-time.After(10 * time.Second):
				return gimbal.TurnResult{}, errors.Join(ctx.Err(), cancelErr, errors.New("execution: harness turn cancellation was not confirmed"))
			}
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
