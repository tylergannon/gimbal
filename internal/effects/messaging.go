package effects

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/tylergannon/gimbal/backend"
)

func Messages(ctx context.Context) backend.Messaging { return Connection(ctx).Messages }

func Request(id string, kind string, owner Owner) backend.MessageRequest {
	r := backend.MessageRequest{ID: id, Created: time.Now().UTC(), Kind: kind, Owner: owner}
	if backend.Identified(kind) {
		r.ExecutionIdentity = backend.ExecutionIdentity{ActionID: id, RequestID: id, ExecutorOperation: "turn_" + id}
		switch kind {
		case "identified-process":
			r.ExecutionIdentity.ExecutorOperation = "command_" + id
		case "identified-session":
			r.ExecutionIdentity.SessionID = id
			r.ExecutionIdentity.ExecutorSession = "session_" + id
		}
	}
	return r
}

func Submit(ctx context.Context, r backend.MessageRequest) error {
	if (r.Kind == "identified-session" || r.Kind == "identified-process") && Connection(ctx).Orchestrator != nil {
		if _, err := Connection(ctx).Orchestrator.AcquireEnvironment(ctx, r.Owner, Environment(ctx)); err != nil {
			return err
		}
	}
	if Environment(ctx) != "" && Connection(ctx).Orchestrator == nil {
		return fmt.Errorf("named environment %q requires an orchestrator backend", Environment(ctx))
	}
	if err := r.Validate(); err != nil {
		return err
	}
	if err := Messages(ctx).PublishRequest(ctx, r); err != nil {
		var rejected *backend.ContractRejected
		if errors.As(err, &rejected) {
			return err
		}
		return &backend.DeliveryUncertain{RequestID: r.ID, Err: fmt.Errorf("publication unconfirmed; do not retry as new work: %w", err)}
	}
	return nil
}

var errEventReached = errors.New("requested messaging fact received")

// Await keeps acceptance uncertainty separate from execution outcome. Replay
// the whole request: the worker's clock need not agree with the caller's.
func Await(ctx context.Context, r backend.MessageRequest, until string, onActivity func(json.RawMessage)) (backend.MessageEvent, error) {

	var got backend.MessageEvent
	seenResources := map[backend.ResourceFact]bool{}
	observeResource := func(use backend.ResourceFact) {
		if !seenResources[use] {
			seenResources[use] = true
			if observe := resourceObserver(ctx); observe != nil {
				observe(use)
			}
		}
	}
	err := Messages(ctx).ListenEvents(ctx, backend.MessageSelection{RequestID: r.ID}, time.Time{}, func(e backend.MessageEvent) error {
		if e.Resource != nil {
			observeResource(*e.Resource)
		}
		if e.Result != nil {
			for _, use := range e.Result.ReadyResources {
				observeResource(use)
			}
		}
		if e.Kind == "activity" && len(e.Activity) > 0 {
			observer, _ := ctx.Value(observationKey{}).(observationOptions)
			if observer.message != nil {
				observer.message(e)
			} else if onActivity != nil {
				onActivity(e.Activity)
			}
		}
		if e.Kind == until || e.Kind == "completed" {
			got = e
			if e.Kind == "completed" && e.Result == nil {
				return fmt.Errorf("request %s completion missing result", r.ID)
			}
			return errEventReached
		}
		return nil
	})
	if errors.Is(err, errEventReached) {
		if got.Result != nil && got.Result.Unresolved {
			return got, &backend.DeliveryUncertain{RequestID: r.ID, Err: fmt.Errorf("%s", got.Result.Error)}
		}
		if got.Result != nil && got.Result.Interrupted {
			return got, context.Canceled
		}
		if got.Result != nil && got.Result.Error != "" {
			if got.Result.InvalidOutput {
				return got, &InvalidOutput{Message: got.Result.Error}
			}
			return got, fmt.Errorf("request %s: %s", r.ID, got.Result.Error)
		}
		return got, nil
	}
	if cause := context.Cause(ctx); cause != nil {
		return got, &backend.DeliveryUncertain{RequestID: r.ID, Err: cause}
	}
	return got, &backend.DeliveryUncertain{RequestID: r.ID, Err: fmt.Errorf("event delivery unavailable: %w", err)}
}
func MessageCall(ctx context.Context, r backend.MessageRequest) (backend.MessageEvent, error) {
	if err := Submit(ctx, r); err != nil {
		return backend.MessageEvent{}, err
	}
	return Await(ctx, r, "completed", nil)
}
func InterruptMessage(ctx context.Context, r backend.MessageRequest) error {
	stopCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	stop := Request(ulid.Make().String(), "interrupt", r.Owner)
	stop.Target = &Target{Run: r.Run, ID: r.Task}
	_, err := MessageCall(stopCtx, stop)
	if err != nil {
		return fmt.Errorf("request %s cessation unconfirmed: %w", r.ID, err)
	}
	return nil
}
