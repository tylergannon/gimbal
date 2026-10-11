package taskdaemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

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
	if h.Connection.Execution != nil {
		ctx = effects.WithConnection(ctx, h.Connection)
	}
	var out effects.Reply
	err := effects.Call(ctx, h.Endpoint, op, in, &out)
	if err == nil && out.Error != "" {
		err = fmt.Errorf("task daemon: %s", out.Error)
	}
	return out, err
}
func (h *Harness) CreateSession(ctx context.Context, model, effort, workdir string) (string, error) {
	o := effects.Identity(ctx)
	r, err := h.call(ctx, "session", effects.Session{Owner: o, Harness: h.Name, Model: model, Effort: effort, Workdir: workdir})
	return key(o.Run, r.ID), err
}
func target(id string) effects.Target {
	run, session, _ := strings.Cut(id, "\x00")
	return effects.Target{Run: run, ID: session}
}
func (h *Harness) RunTurn(ctx context.Context, id, prompt string, schema json.RawMessage, onEvent func(gimbal.AgentEvent) error) (gimbal.TurnResult, error) {
	if h.Connection.Execution != nil {
		ctx = effects.WithConnection(ctx, h.Connection)
	}
	t := target(id)
	o := effects.Identity(ctx)
	files := effects.TaskFiles(ctx)
	reply, err := h.call(ctx, "turn", effects.Turn{Owner: o, Session: t.ID, Prompt: prompt, Schema: schema, Files: files})
	if err != nil {
		if reply.Error != "" {
			return gimbal.TurnResult{}, err
		}
		if reconcile := effects.ConfirmStarted(ctx, h.Endpoint, effects.Target{Run: o.Run, ID: o.Task}); reconcile != nil {
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
	r, err := h.call(ctx, "steer", t)
	return r.Value, err
}
func (h *Harness) Fork(ctx context.Context, id string) (string, error) {
	o := effects.Identity(ctx)
	t := target(id)
	r, err := h.call(ctx, "session", effects.Session{Owner: o, Parent: t.ID})
	return key(o.Run, r.ID), err
}
func (h *Harness) Close(ctx context.Context, id string) error {
	_, err := h.call(ctx, "close", target(id))
	return err
}
