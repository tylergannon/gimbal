package environments

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestConnectionWatchDoesNotMissAChangeDuringYield(t *testing.T) {
	s := testService(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	updates := make(chan ConnectionProgress, 4)
	entered := make(chan struct{})
	release := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		first := true
		done <- s.WatchConnection(ctx, "draft", func(progress ConnectionProgress) error {
			updates <- progress
			if first {
				first = false
				close(entered)
				<-release
			}
			return nil
		})
	}()
	<-entered
	initial := <-updates
	if initial.ID != "draft" || len(initial.Checks) != 0 {
		t.Fatalf("an idle draft invented checks: %+v", initial)
	}
	// This update happens after the initial snapshot but before the watcher can
	// wait. A separately acquired subscription would miss it and block forever.
	s.connectionCheck("draft", ConnectionCheck{ID: "health", Label: "Health", State: "pending"})
	close(release)
	select {
	case progress := <-updates:
		if len(progress.Checks) != 1 || progress.Checks[0].State != "pending" {
			t.Fatalf("watcher missed the pending check: %+v", progress)
		}
		progress.Checks[0].State = "tampered"
		if s.Connection("draft").Checks[0].State != "pending" {
			t.Fatal("watch callback can mutate authoritative progress")
		}
	case <-time.After(time.Second):
		t.Fatal("watcher missed a change between its snapshot and wait")
	}
	s.connectionCheck("draft", ConnectionCheck{ID: "health", Label: "Health", State: "completed"})
	select {
	case progress := <-updates:
		if len(progress.Checks) != 1 || progress.Checks[0].State != "completed" {
			t.Fatalf("replacement notification channel lost the next update: %+v", progress)
		}
	case <-time.After(time.Second):
		t.Fatal("watcher did not receive the next progress change")
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("canceled watcher did not exit")
	}
}

func TestIdleConnectionWatchSleepsUntilItsMachineChangesAndStops(t *testing.T) {
	for _, stop := range []string{"context", "service"} {
		t.Run(stop, func(t *testing.T) {
			s := testService(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			updates := make(chan ConnectionProgress, 2)
			done := make(chan error, 1)
			go func() {
				done <- s.WatchConnection(ctx, Local, func(progress ConnectionProgress) error { updates <- progress; return nil })
			}()
			select {
			case initial := <-updates:
				if len(initial.Checks) != 0 {
					t.Fatal("This Mac has no connection attempt")
				}
			case <-time.After(time.Second):
				t.Fatal("watcher did not yield its initial snapshot")
			}
			s.connectionCheck("another-machine", ConnectionCheck{ID: "health", State: "pending"})
			select {
			case progress := <-updates:
				t.Fatalf("idle watcher woke for an unrelated machine or polling: %+v", progress)
			case <-time.After(150 * time.Millisecond):
			}
			if stop == "context" {
				cancel()
			} else if err := s.Close(); err != nil {
				t.Fatal(err)
			}
			select {
			case err := <-done:
				if err != nil {
					t.Fatal(err)
				}
			case <-time.After(time.Second):
				t.Fatal("idle watcher survived cancellation")
			}
		})
	}
}

func TestConnectionWatchReturnsAConsumerError(t *testing.T) {
	s := testService(t)
	failure := errors.New("connection stream closed")
	if err := s.WatchConnection(context.Background(), "draft", func(ConnectionProgress) error { return failure }); !errors.Is(err, failure) {
		t.Fatalf("watcher lost its consumer error: %v", err)
	}
}

func TestConnectionWatchSeesANewAttemptAndItsFinalFailure(t *testing.T) {
	s := testService(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusServiceUnavailable) }))
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	initial := make(chan struct{})
	final := make(chan ConnectionProgress, 1)
	done := make(chan error, 1)
	go func() {
		first := true
		done <- s.WatchConnection(ctx, "draft", func(progress ConnectionProgress) error {
			if first {
				first = false
				close(initial)
			}
			if progress.Complete {
				final <- progress
			}
			return nil
		})
	}()
	<-initial
	doc, _ := s.Config(ctx)
	_, err := s.ConnectMachine(ctx, MachineEdit{Revision: doc.Revision, Machine: Machine{ID: "draft", Name: "Draft", Kind: KindAddress, Address: server.URL}})
	if err == nil {
		t.Fatal("unhealthy daemon reported successful connection")
	}
	select {
	case progress := <-final:
		if progress.CanConnect || progress.Saved || len(progress.Checks) != 1 || progress.Checks[0].ID != "health" || !progress.Checks[0].Fatal {
			t.Fatalf("watcher missed the complete failed attempt: %+v", progress)
		}
	case <-time.After(time.Second):
		t.Fatal("starting a new attempt or completing it lost the idle watcher's subscription")
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("completed attempt watcher survived cancellation")
	}
}
