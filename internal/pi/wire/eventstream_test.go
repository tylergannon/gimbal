package wire

import (
	"testing"
	"time"

	"github.com/tylergannon/gimbal/internal/pi/model"
)

func waitForWaiters(t *testing.T, stream *EventStream[int, int], n int) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		stream.mu.Lock()
		registered := len(stream.waiting)
		stream.mu.Unlock()
		if registered >= n {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("timed out waiting for %d stream waiters", n)
}

func TestEventStreamDrainsBufferedInOrder(t *testing.T) {
	stream := NewEventStream(func(event int) bool { return event == 3 }, func(event int) int { return event })
	stream.Push(1)
	stream.Push(2)
	stream.Push(3)
	stream.Push(4)

	if got := stream.Result(); got != 3 {
		t.Fatalf("Result() = %d, want 3", got)
	}

	var events []int
	for event := range stream.Events() {
		events = append(events, event)
	}
	if len(events) != 3 || events[0] != 1 || events[1] != 2 || events[2] != 3 {
		t.Fatalf("events = %v, want [1 2 3]", events)
	}
}

func TestEventStreamPreservesOrderAfterBufferedDrain(t *testing.T) {
	stream := NewEventStream(func(int) bool { return false }, func(event int) int { return event })
	stream.Push(1)
	stream.Push(2)

	if event, ok := stream.Next(); !ok || event != 1 {
		t.Fatalf("first Next() = %d, %v, want 1, true", event, ok)
	}
	stream.Push(3)
	if event, ok := stream.Next(); !ok || event != 2 {
		t.Fatalf("second Next() = %d, %v, want 2, true", event, ok)
	}
	if event, ok := stream.Next(); !ok || event != 3 {
		t.Fatalf("third Next() = %d, %v, want 3, true", event, ok)
	}
	stream.End(3)
	if _, ok := stream.Next(); ok {
		t.Fatal("Next() after End() reported an event")
	}
}

func TestEventStreamDeliversToWaitersInRegistrationOrder(t *testing.T) {
	stream := NewEventStream(func(int) bool { return false }, func(event int) int { return event })
	// Register the waiters explicitly, in order, so the test does not depend on
	// goroutine scheduling.
	first := make(chan int, 1)
	second := make(chan int, 1)
	stream.mu.Lock()
	stream.waiting = []chan int{first, second}
	stream.mu.Unlock()

	stream.Push(1)
	stream.Push(2)

	if got := <-first; got != 1 {
		t.Fatalf("first waiter got %d, want 1", got)
	}
	if got := <-second; got != 2 {
		t.Fatalf("second waiter got %d, want 2", got)
	}
}

func TestEventStreamDrainsAfterEndAndResolvesExplicitResult(t *testing.T) {
	stream := NewEventStream(func(int) bool { return false }, func(event int) string { return "event" })
	stream.Push(1)
	stream.Push(2)
	stream.End("complete")

	if got := stream.Result(); got != "complete" {
		t.Fatalf("Result() = %q, want complete", got)
	}
	var events []int
	for event := range stream.Events() {
		events = append(events, event)
	}
	if len(events) != 2 || events[0] != 1 || events[1] != 2 {
		t.Fatalf("events = %v, want [1 2]", events)
	}
}

func TestEventStreamWakesAllWaitersWhenEndedWithoutResult(t *testing.T) {
	stream := NewEventStream(func(int) bool { return false }, func(event int) int { return event })
	first := make(chan bool, 1)
	second := make(chan bool, 1)
	go func() {
		_, ok := stream.Next()
		first <- ok
	}()
	go func() {
		_, ok := stream.Next()
		second <- ok
	}()
	waitForWaiters(t, stream, 2)

	stream.End()

	if ok := <-first; ok {
		t.Fatal("first waiter received an event after End")
	}
	if ok := <-second; ok {
		t.Fatal("second waiter received an event after End")
	}
}

func TestAssistantMessageEventStreamResult(t *testing.T) {
	stream := NewAssistantMessageEventStream()
	message := &model.AssistantMessage{Model: "test"}
	stream.Push(model.AssistantMessageEvent{Type: model.EventDone, Message: message})

	if got := stream.Result(); got != message {
		t.Fatalf("Result() = %p, want %p", got, message)
	}
	event, err := stream.Recv()
	if err != nil {
		t.Fatalf("Recv() error = %v, want nil", err)
	}
	if event.Type != model.EventDone || event.Message != message {
		t.Fatalf("Recv() = %+v, want done event with message", event)
	}
	if _, err := stream.Recv(); err == nil {
		t.Fatal("Recv() after drain must report EOF")
	}
}
