package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
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
	bus, err := connect(ctx, raw, backend.Dependencies{})
	if err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{})
	release := make(chan struct{})
	controlled := make(chan struct{})
	done := make(chan error, 1)
	var starts atomic.Int32
	go func() {
		done <- worker(ctx, raw, backend.Dependencies{}, func(_ context.Context, r backend.MessageRequest) error {
			if r.Kind == "identified-turn" {
				starts.Add(1)
				close(started)
				<-release
			} else {
				close(controlled)
			}
			return nil
		})
	}()
	action := backend.MessageRequest{ID: ulid.Make().String(), Created: time.Now(), Kind: "identified-turn"}
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

func TestBadRequestDoesNotStopOtherRequests(t *testing.T) {
	for _, poison := range []string{"malformed", "unreadable", "handler"} {
		t.Run(poison, func(t *testing.T) {
			directory := t.TempDir()
			raw, _ := json.Marshal(config{Directory: directory})
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			bus, err := connect(ctx, raw, backend.Dependencies{})
			if err != nil {
				t.Fatal(err)
			}
			started := make(chan struct{})
			released := make(chan struct{})
			handled := make(chan struct{})
			done := make(chan error, 1)
			go func() {
				done <- worker(ctx, raw, backend.Dependencies{}, func(ctx context.Context, r backend.MessageRequest) error {
					switch r.Kind {
					case "held":
						close(started)
						select {
						case <-released:
						case <-ctx.Done():
						}
					case "poison":
						return errors.New("event publication failed")
					case "healthy":
						close(handled)
					}
					return nil
				})
			}()
			defer func() { cancel(); <-done }()
			publish := func(kind string) {
				t.Helper()
				if err := bus.PublishRequest(ctx, backend.MessageRequest{ID: ulid.Make().String(), Created: time.Now(), Kind: kind}); err != nil {
					t.Fatal(err)
				}
			}
			publish("held")
			select {
			case <-started:
			case <-ctx.Done():
				t.Fatal("initial request not admitted")
			}
			path := filepath.Join(directory, "requests", "000.json")
			switch poison {
			case "malformed":
				err = os.WriteFile(path, []byte("{"), 0600)
			case "unreadable":
				err = os.Symlink(filepath.Join(directory, "missing"), path)
			case "handler":
				publish("poison")
			}
			if err != nil {
				t.Fatal(err)
			}
			// Allow two polls to observe the bad request before offering new work.
			time.Sleep(100 * time.Millisecond)
			publish("healthy")
			select {
			case <-handled:
			case <-ctx.Done():
				t.Fatal("bad request stopped subsequent admission")
			}
			close(released)
		})
	}
}
