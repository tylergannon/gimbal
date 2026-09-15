package gimble

import (
	"context"
	"testing"
)

// caught runs f and returns the value it panicked with, or nil.
func caught(f func()) (v any) {
	defer func() { v = recover() }()
	f()
	return nil
}

func TestSetMisusePanics(t *testing.T) {
	if got := caught(func() { Set(t.Context(), "goal", "x") }); got != `gimble: set "goal": no scope in the ctx; it must come from gimble.Run` {
		t.Errorf("Set with no scope panicked with %v", got)
	}
	err := runTest(t, func(ctx context.Context) error {
		var ended context.Context
		err := Scope(ctx, "delivery", func(ctx context.Context) error {
			ended = ctx
			Set(ctx, "task", "first")
			if got := caught(func() { Set(ctx, "task", "second") }); got != `gimble: "task" is already set in scope "delivery.1"` {
				t.Errorf("a second Set of a key panicked with %v", got)
			}
			if got := caught(func() { SetJSON(ctx, "task", review{}) }); got != `gimble: "task" is already set in scope "delivery.1"` {
				t.Errorf("SetJSON of a set key panicked with %v", got)
			}
			return nil
		})
		if err != nil {
			return err
		}
		if got := caught(func() { Set(ended, "late", "x") }); got != `gimble: set "late" in scope "delivery.1" after it ended` {
			t.Errorf("Set on an ended scope panicked with %v", got)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
