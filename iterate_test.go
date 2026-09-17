package gimble

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"testing"
)

func TestIterateScopesItemsAndClosesTheirSessions(t *testing.T) {
	adapter := &fake{answer: func(context.Context, string, string, json.RawMessage, func(AgentEvent) error) (string, error) {
		return "done", nil
	}}
	err := runTest(t, bind(adapter, "model", "outer", "inner"), func(ctx context.Context) error {
		outer := NewSession(ctx, "outer", ".")
		if _, err := outer.Generate[Text](ctx, "prime outer"); err != nil {
			return err
		}

		var seen []string
		var child *Session
		for itemCtx, item := range Iterate(ctx, "item", []string{"one", "two", "three"}) {
			seen = append(seen, item)
			child = NewSession(itemCtx, "inner", ".")
			if _, err := child.Generate[Text](itemCtx, "prime child"); err != nil {
				return err
			}
			if item == "one" {
				continue
			}
			if len(adapter.closed) != 1 {
				t.Fatalf("closed sessions before second item = %v, want first child", adapter.closed)
			}
			break
		}
		if want := []string{"one", "two"}; !slices.Equal(seen, want) {
			t.Fatalf("items = %v, want %v", seen, want)
		}
		if _, err := child.Generate[Text](ctx, "closed child"); err == nil {
			t.Fatal("saved child session remained usable after break")
		}
		if len(adapter.closed) != 2 {
			t.Fatalf("closed sessions before outer resumes = %v, want two children", adapter.closed)
		}
		_, err := outer.Generate[Text](ctx, "still usable")
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if adapter.made != 3 || len(adapter.closed) != 3 {
		t.Fatalf("sessions made=%d closed=%v, want three of each", adapter.made, adapter.closed)
	}
}

func TestIterateClosesItemScopeWhenFunctionReturns(t *testing.T) {
	adapter := &fake{answer: func(context.Context, string, string, json.RawMessage, func(AgentEvent) error) (string, error) {
		return "done", nil
	}}
	err := runTest(t, bind(adapter, "model", "worker"), func(ctx context.Context) error {
		stop := errors.New("stop")
		step := func() error {
			for itemCtx := range Iterate(ctx, "item", []int{1}) {
				worker := NewSession(itemCtx, "worker", ".")
				if _, err := worker.Generate[Text](itemCtx, "prime"); err != nil {
					return err
				}
				return stop
			}
			return nil
		}
		if err := step(); !errors.Is(err, stop) {
			t.Fatalf("step returned %v, want %v", err, stop)
		}
		if len(adapter.closed) != 1 {
			t.Fatalf("closed sessions = %v, want one", adapter.closed)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestIterateStopsBeforeYieldWhenCancelled(t *testing.T) {
	err := runTest(t, nil, func(ctx context.Context) error {
		cancelled, cancel := context.WithCancel(ctx)
		cancel()
		for range Iterate(cancelled, "item", []int{1}) {
			t.Fatal("cancelled iterator yielded an item")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
