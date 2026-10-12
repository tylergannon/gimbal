package main

import (
	"context"
	"encoding/json"
	"sync/atomic"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/tylergannon/gimbal/backend"
)

func TestConcurrentControlAndSameRequestIdentity(t *testing.T) {
	raw, _ := json.Marshal(config{Directory: t.TempDir()})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	bus, err := connect(ctx, raw)
	if err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{})
	release := make(chan struct{})
	controlled := make(chan struct{})
	done := make(chan error, 1)
	var starts atomic.Int32
	go func() {
		done <- worker(ctx, raw, func(_ context.Context, r backend.MessageRequest) error {
			if r.Kind == "turn" {
				starts.Add(1)
				close(started)
				<-release
			} else {
				close(controlled)
			}
			return nil
		})
	}()
	action := backend.MessageRequest{ID: ulid.Make().String(), Created: time.Now(), Kind: "turn"}
	if err = bus.PublishRequest(ctx, action); err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-ctx.Done():
		t.Fatal("not admitted")
	}
	if err = bus.PublishRequest(ctx, action); err != nil {
		t.Fatal(err)
	}
	control := backend.MessageRequest{ID: ulid.Make().String(), Created: time.Now(), Kind: "interrupt"}
	if err = bus.PublishRequest(ctx, control); err != nil {
		t.Fatal(err)
	}
	select {
	case <-controlled:
	case <-ctx.Done():
		t.Fatal("control queued behind action")
	}
	close(release)
	cancel()
	<-done
	if starts.Load() != 1 {
		t.Fatalf("native starts %d", starts.Load())
	}
}
