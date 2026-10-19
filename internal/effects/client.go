// Package effects carries concrete task requests. Environment provisioning and
// workflow observations do not participate in this protocol.
package effects

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/tylergannon/gimbal/backend"
)

type Owner = backend.Owner
type Process = backend.Process
type ProcessResult = backend.ProcessResult
type Scope = backend.Scope
type Session = backend.Session
type Turn = backend.Turn
type Target = backend.Target
type Reply = backend.Reply
type Poll = backend.Poll
type Progress = backend.Progress
type State = backend.State

type endpointKey struct{}
type ownerKey struct{}

func WithEndpoint(ctx context.Context, url string) context.Context {
	return context.WithValue(ctx, endpointKey{}, url)
}
func Endpoint(ctx context.Context) string {
	s, _ := ctx.Value(endpointKey{}).(string)
	if s == "" && (Connection(ctx).Execution != nil || Messages(ctx) != nil) {
		return "backend"
	}
	return s
}
func WithOwner(ctx context.Context, owner Owner) context.Context {
	return context.WithValue(ctx, ownerKey{}, owner)
}
func Identity(ctx context.Context) Owner { o, _ := ctx.Value(ownerKey{}).(Owner); return o }

func Call(ctx context.Context, endpoint, operation string, input, output any) error {
	if Environment(ctx) != "" {
		return fmt.Errorf("named environments require messaging delivery")
	}
	if c := Connection(ctx); c.Execution != nil {
		return dispatch(ctx, c, operation, input, output)
	}
	raw, err := json.Marshal(input)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint+"/tasks/"+operation, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		if backend.Identified(operation) && (response.StatusCode == 404 || response.StatusCode == 405) {
			return &backend.ContractRejected{Message: fmt.Sprintf("unsupported %s", operation)}
		}
		return fmt.Errorf("task daemon %s: HTTP %d", operation, response.StatusCode)
	}
	return json.NewDecoder(response.Body).Decode(output)
}

func Cleanup(ctx context.Context, endpoint, run, scope string) error {
	if Messages(ctx) != nil {
		r := Request(backend.NewExecutionIdentity("control").RequestID, "cleanup", Owner{Run: run, Scope: scope})
		r.ScopeTarget = &Scope{Run: run, Scope: scope}
		if _, err := MessageCall(ctx, r); err != nil {
			return err
		}
		// All assigned daemons have stopped native dependents before a shared
		// store or supporting environment can be released.
		connection := Connection(ctx)
		if connection.Secrets != nil {
			if err := connection.Secrets.Release(ctx, *r.ScopeTarget); err != nil {
				return err
			}
		}
		if connection.Orchestrator != nil {
			return connection.Orchestrator.Release(ctx, *r.ScopeTarget)
		}
		return nil
	}
	var reply Reply
	if err := Call(ctx, endpoint, "cleanup", Scope{Run: run, Scope: scope}, &reply); err != nil {
		return err
	}
	if reply.Error != "" {
		return fmt.Errorf("scope cleanup: %s", reply.Error)
	}
	return nil
}

// InvalidOutput preserves native answer classification across task transport.
type InvalidOutput struct{ Message string }

func (e *InvalidOutput) Error() string { return e.Message }

// Wait reconnects reads only; it never repeats execution. Interruption is an
// explicit effect, so losing an observation connection cannot stop native work.
func Wait(ctx context.Context, endpoint string, target Target, onEvent func(json.RawMessage)) (json.RawMessage, error) {
	eventCtx, stop := context.WithCancel(context.WithoutCancel(ctx))
	var eventsDone chan struct{}
	defer func() {
		stop()
		if eventsDone != nil {
			select {
			case <-eventsDone:
			case <-time.After(1100 * time.Millisecond):
			}
		}
	}()
	observation, _ := ctx.Value(observationKey{}).(observationOptions)
	if onEvent != nil || observation.task != nil {
		eventsDone = make(chan struct{})
		go func() {
			defer close(eventsDone)
			after := observation.cursor
			failures := 0
			for eventCtx.Err() == nil {
				readCtx, cancel := context.WithTimeout(eventCtx, time.Second)
				var events backend.Events
				err := Call(readCtx, endpoint, "events", Poll{Run: target.Run, ID: target.ID, After: after}, &events)
				cancel()
				if err == nil && !events.Unavailable && !events.Gap {
					if failures > 0 && len(events.Events) > 0 {
						if onEvent != nil {
							onEvent(json.RawMessage(`{"type":"observation-restored","data":{"message":"task event delivery resumed"}}`))
						}
					}
					if len(events.Events) > 0 {
						failures = 0
					}
					for i, event := range events.Events {
						if observation.task != nil {
							observation.task(after+i+1, event)
						} else {
							onEvent(event)
						}
					}
					after = events.Cursor
				} else {
					if err == nil && events.Gap && events.Cursor >= 0 {
						after = events.Cursor
					}
					failures++
					if failures == 1 {
						if onEvent != nil {
							onEvent(json.RawMessage(`{"Type":"observation-gap","Data":{"message":"task events unavailable; execution continues"}}`))
						}
					}
					if failures >= 3 {
						select {
						case <-eventCtx.Done():
							return
						case <-time.After(time.Second):
						}
						failures = 1
					}
				}

				select {
				case <-eventCtx.Done():
					return
				case <-time.After(100 * time.Millisecond):
				}
			}
		}()
	}
	var disconnected time.Time
	for {
		if ctx.Err() != nil {
			if detached(ctx) {
				return nil, &backend.DeliveryUncertain{RequestID: target.ID, Err: ctx.Err()}
			}
			stopCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
			var reply Reply
			err := Call(stopCtx, endpoint, "interrupt", target, &reply)
			if err == nil && reply.Error != "" {
				err = errors.New(reply.Error)
			}
			if err == nil {
				// An interrupt acknowledgement is not native cessation. Read the
				// original task until its terminal result confirms the outcome.
				for stopCtx.Err() == nil {
					var result backend.Result
					err = Call(stopCtx, endpoint, "result", target, &result)
					if err == nil && result.Done && result.Error != "task not found" && !result.Expired {
						cancel()
						return terminalResult(result, target.ID)
					}
					select {
					case <-stopCtx.Done():
					case <-time.After(50 * time.Millisecond):
					}
				}
			}
			cancel()
			return nil, &backend.DeliveryUncertain{RequestID: target.ID, Err: fmt.Errorf("stop unconfirmed; task remains owned: %v", err)}
		}
		var result backend.Result
		readCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		err := Call(readCtx, endpoint, "result", target, &result)
		cancel()
		if err != nil {
			if disconnected.IsZero() {
				disconnected = time.Now()
			}
			if time.Since(disconnected) >= 15*time.Second {
				return nil, &backend.DeliveryUncertain{RequestID: target.ID, Err: fmt.Errorf("result unavailable; task remains owned: %w", err)}
			}
		} else {
			disconnected = time.Time{}
			if result.Done {
				if result.Error == "task not found" || result.Expired {
					return nil, &backend.DeliveryUncertain{RequestID: target.ID, Err: fmt.Errorf("original task outcome unavailable: %s", result.Error)}
				}
				if eventsDone != nil {
					time.Sleep(110 * time.Millisecond)
				}
				return terminalResult(result, target.ID)
			}
		}
		select {
		case <-ctx.Done():
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func terminalResult(result backend.Result, id string) (json.RawMessage, error) {
	if result.Interrupted {
		return result.Result, context.Canceled
	}
	if result.InvalidOutput {
		return result.Result, &InvalidOutput{Message: result.Error}
	}
	if result.Error != "" {
		return result.Result, fmt.Errorf("task %s: %s", id, result.Error)
	}
	return result.Result, nil
}

// ConfirmStarted reconciles a lost admission reply using the already assigned
// identity. Reading is safe to retry; the mutation itself is never replayed.
func ConfirmStarted(ctx context.Context, endpoint string, target Target, admission error) error {
	var rejected *backend.ContractRejected
	if errors.As(admission, &rejected) {
		return admission
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	var last error
	for {
		var progress backend.Result
		err := Call(ctx, endpoint, "result", target, &progress)
		if err == nil && progress.Error != "task not found" {
			return nil
		}
		last = err
		select {
		case <-ctx.Done():
			return &backend.DeliveryUncertain{RequestID: target.ID, Err: fmt.Errorf("admission unconfirmed; cleanup retains its scope: %v: %w", last, ctx.Err())}
		case <-time.After(100 * time.Millisecond):
		}
	}
}
