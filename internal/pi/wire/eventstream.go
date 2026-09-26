package wire

import (
	"io"
	"iter"
	"sync"

	"github.com/tylergannon/gimbal/internal/pi/model"
)

// EventStream is a generic, thread-safe event queue with a deferred final
// result, ported from pi's EventStream<T, R> (utils/event-stream.ts).
//
// Producers call Push for each event and optionally End to terminate. The
// terminal event, chosen by isComplete, captures the final result via extract
// and unblocks Result. Consumers pull events with Next or range over Events.
//
// Push hands an event directly to the longest-waiting consumer, bypassing the
// queue, so events reach blocked consumers in registration order.
type EventStream[T any, R any] struct {
	mu         sync.Mutex
	cond       *sync.Cond
	queue      []T
	waiting    []chan T
	done       bool
	result     R
	hasResult  bool
	isComplete func(T) bool
	extract    func(T) R
}

// NewEventStream creates an EventStream. isComplete reports whether an event is
// the terminal event; extract derives the final result from that event.
func NewEventStream[T any, R any](isComplete func(T) bool, extract func(T) R) *EventStream[T, R] {
	s := &EventStream[T, R]{isComplete: isComplete, extract: extract}
	s.cond = sync.NewCond(&s.mu)
	return s
}

func (s *EventStream[T, R]) releaseWaitersLocked() {
	for _, ch := range s.waiting {
		close(ch)
	}
	s.waiting = nil
}

// Push enqueues an event, or hands it straight to the longest-waiting consumer.
// A terminal event captures the final result; pushes after the stream is done
// are ignored.
func (s *EventStream[T, R]) Push(event T) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.done {
		return
	}

	terminal := s.isComplete(event)
	if terminal {
		s.done = true
		if !s.hasResult {
			s.result = s.extract(event)
			s.hasResult = true
		}
	}

	if len(s.waiting) > 0 {
		ch := s.waiting[0]
		s.waiting = s.waiting[1:]
		ch <- event
	} else {
		s.queue = append(s.queue, event)
	}

	if terminal {
		// Releasing waiters keeps Next from blocking forever when a producer
		// ends the stream with a terminal event and never calls End.
		s.releaseWaitersLocked()
		s.cond.Broadcast()
	}
}

// End terminates the stream. A supplied result becomes the final result unless
// a terminal event already captured one. Waiting consumers are woken.
func (s *EventStream[T, R]) End(result ...R) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(result) > 0 && !s.hasResult {
		s.result = result[0]
		s.hasResult = true
	}
	s.done = true
	s.releaseWaitersLocked()
	s.cond.Broadcast()
}

// Next pulls the next event, blocking until one is available or the stream is
// drained. ok is false once no more events will arrive. Buffered events drain
// before the done flag is honored.
func (s *EventStream[T, R]) Next() (event T, ok bool) {
	s.mu.Lock()
	if len(s.queue) > 0 {
		event = s.queue[0]
		s.queue = s.queue[1:]
		s.mu.Unlock()
		return event, true
	}
	if s.done {
		s.mu.Unlock()
		var zero T
		return zero, false
	}
	ch := make(chan T, 1)
	s.waiting = append(s.waiting, ch)
	s.mu.Unlock()

	event, ok = <-ch
	return event, ok
}

// Events returns a single-use iterator over the stream's events.
func (s *EventStream[T, R]) Events() iter.Seq[T] {
	return func(yield func(T) bool) {
		for {
			event, ok := s.Next()
			if !ok {
				return
			}
			if !yield(event) {
				return
			}
		}
	}
}

// Result blocks until the final result is available, or the stream ends without
// one (returning the zero value).
func (s *EventStream[T, R]) Result() R {
	s.mu.Lock()
	defer s.mu.Unlock()
	for !s.hasResult && !s.done {
		s.cond.Wait()
	}
	return s.result
}

// AssistantMessageEventStream is the concrete stream pi's API implementations
// return: it completes on a "done" or "error" event and yields the assistant
// message as the final result. It satisfies model.AssistantMessageEventChannel.
type AssistantMessageEventStream struct {
	*EventStream[model.AssistantMessageEvent, *model.AssistantMessage]
}

// NewAssistantMessageEventStream creates an open assistant message stream.
func NewAssistantMessageEventStream() *AssistantMessageEventStream {
	return &AssistantMessageEventStream{EventStream: NewEventStream(
		func(event model.AssistantMessageEvent) bool {
			return event.Type == model.EventDone || event.Type == model.EventError
		},
		func(event model.AssistantMessageEvent) *model.AssistantMessage {
			switch event.Type {
			case model.EventDone:
				return event.Message
			case model.EventError:
				return event.Error
			default:
				panic("wire: unexpected event type for final result")
			}
		},
	)}
}

// Recv yields the next event, or io.EOF once the stream is drained. It
// implements model.AssistantMessageEventChannel.
func (s *AssistantMessageEventStream) Recv() (model.AssistantMessageEvent, error) {
	event, ok := s.Next()
	if !ok {
		return model.AssistantMessageEvent{}, io.EOF
	}
	return event, nil
}

// Close ends the stream. It implements model.AssistantMessageEventChannel.
func (s *AssistantMessageEventStream) Close() error {
	s.End()
	return nil
}
