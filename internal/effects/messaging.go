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

func Request(kind string, owner Owner) backend.MessageRequest {
	return backend.MessageRequest{ID: ulid.Make().String(), Created: time.Now().UTC(), Kind: kind, Owner: owner}
}

func Submit(ctx context.Context, r backend.MessageRequest) error {
	if err := Messages(ctx).PublishRequest(ctx, r); err != nil {
		return fmt.Errorf("request %s publication unconfirmed; do not retry as new work: %w", r.ID, err)
	}
	return nil
}

var errEventReached = errors.New("requested messaging fact received")

// Await keeps acceptance uncertainty separate from execution outcome. The
// original creation bound is retained for every new subscription.
func Await(ctx context.Context, r backend.MessageRequest, until string, onActivity func(json.RawMessage)) (backend.MessageEvent, error) {
	return await(ctx, r, until, onActivity, 15*time.Second)
}
func await(ctx context.Context, r backend.MessageRequest, until string, onActivity func(json.RawMessage), acceptance time.Duration) (backend.MessageEvent, error) {
	listenCtx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	timer := time.AfterFunc(acceptance, func() {
		cancel(fmt.Errorf("request %s acceptance unconfirmed; inspect the configured worker and request store; do not retry as new work", r.ID))
	})
	defer timer.Stop()
	var got backend.MessageEvent
	err := Messages(ctx).ListenEvents(listenCtx, backend.MessageSelection{RequestID: r.ID}, r.Created, func(e backend.MessageEvent) error {
		if e.Kind == "accepted" || e.Kind == "established" || e.Kind == "completed" {
			timer.Stop()
		}
		if e.Kind == "activity" && onActivity != nil {
			onActivity(e.Activity)
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
		if got.Result != nil && got.Result.Error != "" {
			if got.Result.InvalidOutput {
				return got, &InvalidOutput{Message: got.Result.Error}
			}
			return got, fmt.Errorf("request %s: %s", r.ID, got.Result.Error)
		}
		return got, nil
	}
	if cause := context.Cause(listenCtx); cause != nil {
		return got, cause
	}
	return got, fmt.Errorf("request %s event delivery unavailable: %w", r.ID, err)
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
	stop := Request("interrupt", r.Owner)
	stop.Target = &Target{Run: r.Run, ID: r.Task}
	_, err := MessageCall(stopCtx, stop)
	if err != nil {
		return fmt.Errorf("request %s cessation unconfirmed: %w", r.ID, err)
	}
	return nil
}
