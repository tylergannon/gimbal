package taskdaemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/backend"
	"github.com/tylergannon/gimbal/internal/effects"
)

// Harness preserves the native harness contract across the concrete task
// transport. The returned identity carries the run needed by Close after the
// execution context has already been cancelled.
type Harness struct {
	Endpoint, Name string
	Connection     backend.Connection
}

func (h *Harness) call(ctx context.Context, op string, in any) (effects.Reply, error) {
	if h.Connection.Execution != nil || h.Connection.Messages != nil {
		ctx = effects.WithConnection(ctx, h.Connection)
	}
	var out effects.Reply
	if effects.Messages(ctx) != nil {
		identity := effects.ExecutionIdentityFromContext(ctx)
		requestID := identity.RequestID
		if op == "steer" {
			requestID = effects.InputID(ctx)
			if requestID == "" {
				requestID = identity.RequestID
			}
			if requestID == "" {
				requestID = backend.NewExecutionIdentity("turn").RequestID
			}
		}
		r := effects.Request(requestID, op, effects.Identity(ctx))
		r.ExecutionIdentity = identity
		switch v := in.(type) {
		case effects.Session:
			r.Session = &v
			r.SessionID = v.Task
		case effects.Target:
			r.Target = &v
			r.Run = v.Run
			r.SessionID = v.ID
		default:
			return out, fmt.Errorf("unsupported messaging operation %s", op)
		}
		if r.Session != nil && r.Session.Parent != "" {
			return out, fmt.Errorf("%s is unsupported on messaging routes", op)
		}
		e, err := effects.MessageCall(ctx, r)
		if e.Result != nil {
			out.ID = e.Result.ID
			out.Value = e.Result.Value
		}
		return out, err
	}
	err := effects.Call(ctx, h.Endpoint, op, in, &out)
	if err == nil && out.Error != "" {
		err = fmt.Errorf("task daemon: %s", out.Error)
	}
	return out, err
}
func (h *Harness) CreateSession(ctx context.Context, model, effort, workdir string) (string, error) {
	o := effects.Identity(ctx)
	r, err := h.call(ctx, "identified-session", effects.Session{ExecutionIdentity: effects.ExecutionIdentityFromContext(ctx), Owner: o, Harness: h.Name, Model: model, Effort: effort, Workdir: workdir})
	if r.ID == "" {
		return "", err
	}
	return key(o.Run, r.ID), err
}
func target(id string) effects.Target {
	run, session, _ := strings.Cut(id, "\x00")
	return effects.Target{Run: run, ID: session}
}
func (h *Harness) RunTurn(ctx context.Context, id, prompt string, schema json.RawMessage, onEvent func(gimbal.AgentEvent) error) (gimbal.TurnResult, error) {
	if h.Connection.Execution != nil || h.Connection.Messages != nil {
		ctx = effects.WithConnection(ctx, h.Connection)
	}
	t := target(id)
	o := effects.Identity(ctx)
	if effects.Messages(ctx) != nil {
		return h.messageTurn(ctx, t, o, prompt, schema, onEvent)
	}
	files := effects.TaskFiles(ctx)
	reply, err := h.call(ctx, "identified-turn", effects.Turn{Deadline: turnDeadline(ctx), ExecutionIdentity: effects.ExecutionIdentityFromContext(ctx), Owner: o, Session: t.ID, Prompt: prompt, Schema: schema, Files: files, Requires: effects.Requirements(ctx)})
	if err != nil {
		if reply.Error != "" {
			return gimbal.TurnResult{}, err
		}
		if reconcile := effects.ConfirmStarted(ctx, h.Endpoint, effects.Target{Run: o.Run, ID: o.Task}, err); reconcile != nil {
			return gimbal.TurnResult{}, fmt.Errorf("%v; %w", err, reconcile)
		}
	}
	raw, err := effects.Wait(ctx, h.Endpoint, effects.Target{Run: o.Run, ID: o.Task}, func(raw json.RawMessage) {
		var e struct {
			gimbal.AgentEvent
			Native json.RawMessage
		}
		if json.Unmarshal(raw, &e) == nil {
			e.NativeRef = e.Native
			if e.Type == "observation-gap" || e.Type == "observation-restored" {
				unavailable := e.Type == "observation-gap"
				eventType := "session.observation.available"
				message := "task event delivery resumed"
				if unavailable {
					message = "task events unavailable; execution continues"
					eventType = "session.observation.unavailable"
				}
				e.Type = eventType
				e.Data, _ = json.Marshal(map[string]any{"sessionID": id, "message": message})
				e.NativeRef, _ = json.Marshal(map[string]any{"provider": "gimbal", "sessionID": id, "turnID": o.Task})
			}
			if onEvent != nil {
				_ = onEvent(e.AgentEvent)
			}
		}
	})
	var result gimbal.TurnResult
	if invalid, ok := errors.AsType[*effects.InvalidOutput](err); ok {
		err = fmt.Errorf("%w: %s", gimbal.ErrInvalidOutput, invalid.Message)
	}
	if len(raw) > 0 {
		var wire backend.TurnResult
		err = errors.Join(err, json.Unmarshal(raw, &wire))
		result.Output = wire.Output
		if len(wire.Usage) > 0 {
			err = errors.Join(err, json.Unmarshal(wire.Usage, &result.Usage))
		}
	}
	return result, err
}
func (h *Harness) Steer(ctx context.Context, id, message string) (bool, error) {
	t := target(id)
	t.Message = message
	t.InputID = effects.InputID(ctx)
	t.ExecutionIdentity = effects.ExecutionIdentityFromContext(ctx)
	r, err := h.call(ctx, "steer", t)
	return r.Value, err
}
func (h *Harness) Fork(ctx context.Context, id string) (string, error) {
	o := effects.Identity(ctx)
	t := target(id)
	r, err := h.call(ctx, "identified-session", effects.Session{ExecutionIdentity: effects.ExecutionIdentityFromContext(ctx), Owner: o, Parent: t.ID})
	if r.ID == "" {
		return "", err
	}
	return key(o.Run, r.ID), err
}
func (h *Harness) Close(ctx context.Context, id string) error {
	t := target(id)
	t.ExecutionIdentity = effects.ExecutionIdentityFromContext(ctx)
	_, err := h.call(ctx, "identified-close", t)
	return err
}

func (h *Harness) messageTurn(ctx context.Context, t effects.Target, o effects.Owner, prompt string, schema json.RawMessage, onEvent func(gimbal.AgentEvent) error) (gimbal.TurnResult, error) {
	identity := effects.ExecutionIdentityFromContext(ctx)
	r := effects.Request(identity.RequestID, "identified-turn", o)
	r.ExecutionIdentity = identity
	r.SessionID = t.ID
	r.Turn = &effects.Turn{Deadline: turnDeadline(ctx), ExecutionIdentity: effects.ExecutionIdentityFromContext(ctx), Owner: o, Session: t.ID, Prompt: prompt, Schema: schema, Files: effects.TaskFiles(ctx), Requires: effects.Requirements(ctx)}
	if err := effects.Submit(ctx, r); err != nil {
		return gimbal.TurnResult{}, err
	}
	event, err := effects.Await(ctx, r, "completed", func(raw json.RawMessage) {
		var e struct {
			gimbal.AgentEvent
			Native json.RawMessage
		}
		if json.Unmarshal(raw, &e) == nil && onEvent != nil {
			e.NativeRef = e.Native
			_ = onEvent(e.AgentEvent)
		}
	})
	if ctx.Err() != nil {
		if stopErr := effects.InterruptMessage(ctx, r); stopErr != nil {
			err = errors.Join(err, stopErr)
		} else {
			err = ctx.Err()
		}
	}
	if invalid, ok := errors.AsType[*effects.InvalidOutput](err); ok {
		err = fmt.Errorf("%w: %s", gimbal.ErrInvalidOutput, invalid.Message)
	}
	var result gimbal.TurnResult
	if event.Result != nil {
		result.Output = event.Result.Output
		if len(event.Result.Usage) > 0 {
			err = errors.Join(err, json.Unmarshal(event.Result.Usage, &result.Usage))
		}
	}
	return result, err
}

func turnDeadline(ctx context.Context) time.Time { deadline, _ := ctx.Deadline(); return deadline }
