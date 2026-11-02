package messagefiles

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/tylergannon/gimbal/backend"
)

func TestRetainedLateOlderEventAndIndependentReaders(t *testing.T) {
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	since := time.Now().UTC()
	old := backend.MessageEvent{ID: ulid.Make().String(), Created: since, RequestID: "request", Kind: "activity"}
	newer := backend.MessageEvent{ID: ulid.Make().String(), Created: since, RequestID: "request", Kind: "completed"}
	if err = store.PublishEvent(context.Background(), newer); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	received := make(chan string, 2)
	done := make(chan error, 1)
	go func() {
		done <- store.ListenEvents(ctx, backend.MessageSelection{RequestID: "request"}, since, func(e backend.MessageEvent) error { received <- e.ID; return nil })
	}()
	if got := <-received; got != newer.ID {
		t.Fatal(got)
	}
	if err = store.PublishEvent(ctx, old); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-received:
		if got != old.ID {
			t.Fatal(got)
		}
	case <-ctx.Done():
		t.Fatal("late older event skipped")
	}
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("listener cancellation: %v", err)
	}
	for range 2 {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		count := 0
		stop := errors.New("done")
		err := store.ListenEvents(ctx, backend.MessageSelection{RequestID: "request"}, since, func(backend.MessageEvent) error {
			count++
			if count == 2 {
				return stop
			}
			return nil
		})
		if !errors.Is(err, stop) || count != 2 {
			t.Fatalf("independent retained read count=%d err=%v", count, err)
		}
	}
}

func TestReplayOrdersOneSourceBySequence(t *testing.T) {
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	later := time.Now().UTC()
	for _, event := range []backend.MessageEvent{
		{ID: "01ARZ3NDEKTSV4RRFFQ69G5FA0", RequestID: "request", Source: "request", Sequence: 2, Created: later.Add(-time.Hour)},
		{ID: "01ARZ3NDEKTSV4RRFFQ69G5FA1", RequestID: "request", Source: "request", Sequence: 1, Created: later},
	} {
		if err := store.PublishEvent(t.Context(), event); err != nil {
			t.Fatal(err)
		}
	}
	stop := errors.New("done")
	var got []uint64
	err = store.ListenEvents(t.Context(), backend.MessageSelection{RequestID: "request"}, time.Time{}, func(e backend.MessageEvent) error {
		got = append(got, e.Sequence)
		if len(got) == 2 {
			return stop
		}
		return nil
	})
	if !errors.Is(err, stop) || !reflect.DeepEqual(got, []uint64{1, 2}) {
		t.Fatalf("sequence replay: %v %v", got, err)
	}
}
