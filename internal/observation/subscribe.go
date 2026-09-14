package observation

import (
	"errors"
	"sync"
)

// Default subscriber bounds. A subscriber's backlog is bounded by both the
// number of frames and their total bytes, because either one alone can be
// the runaway: a stalled browser can accumulate a great many small deltas or
// a few very large ones.
const (
	defaultMaxFrames = 512
	defaultMaxBytes  = 4 << 20
)

// ErrOverflow is why a subscription that could not keep up was closed.
var ErrOverflow = errors.New("observation: subscriber backlog overflowed its bound")

// ErrClosed is why a subscription ended because its run did.
var ErrClosed = errors.New("observation: run ended")

// JoinSubscription is the transaction-granular stream used by the page.
type JoinSubscription struct {
	store  *Store
	deltas chan Delta
	done   chan struct{}
	mu     sync.Mutex
	closed bool
	bytes  int
	err    error
}

func (sub *JoinSubscription) Deltas() <-chan Delta  { return sub.deltas }
func (sub *JoinSubscription) Done() <-chan struct{} { return sub.done }
func (sub *JoinSubscription) Err() error            { sub.mu.Lock(); defer sub.mu.Unlock(); return sub.err }
func (sub *JoinSubscription) Took(delta Delta) {
	sub.mu.Lock()
	sub.bytes -= deltaBytes(delta)
	sub.mu.Unlock()
}
func (sub *JoinSubscription) Close() {
	sub.store.mu.Lock()
	defer sub.store.mu.Unlock()
	sub.mu.Lock()
	defer sub.mu.Unlock()
	if sub.closed {
		return
	}
	sub.closed = true
	delete(sub.store.joins, sub)
	close(sub.done)
}

// Join resumes strictly after position when it belongs to this run stream and
// the bounded suffix is still retained. Otherwise it returns a replacement
// snapshot. Capture and registration share the reduction lock with producers.
func (s *Store) Join(stream string, position uint64) (*RunSnapshot, []Delta, *JoinSubscription, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		snap := s.snapshotLocked()
		return &snap, nil, nil, ErrClosed
	}
	var snapshot *RunSnapshot
	var suffix []Delta
	available := stream == s.stream && position <= s.position
	if available && position < s.position {
		first := s.position + 1
		if len(s.recent) > 0 {
			first = s.recent[0].Position
		}
		available = position+1 >= first
	}
	if available {
		for _, delta := range s.recent {
			if delta.Position > position {
				suffix = append(suffix, delta)
			}
		}
	} else {
		value := s.snapshotLocked()
		snapshot = &value
	}
	sub := &JoinSubscription{store: s, deltas: make(chan Delta, s.maxFrames), done: make(chan struct{})}
	s.joins[sub] = struct{}{}
	return snapshot, suffix, sub, nil
}

func (s *Store) retainLocked(delta Delta) {
	s.recent = append(s.recent, delta)
	bytes := 0
	for _, one := range s.recent {
		for _, frame := range one.Frames {
			bytes += len(frame.Data)
		}
	}
	for len(s.recent) > s.maxFrames || bytes > s.maxBytes {
		for _, frame := range s.recent[0].Frames {
			bytes -= len(frame.Data)
		}
		s.recent = s.recent[1:]
	}
}

func (s *Store) publishDeltaLocked(delta Delta) {
	for sub := range s.joins {
		sub.mu.Lock()
		if sub.closed {
			sub.mu.Unlock()
			delete(s.joins, sub)
			continue
		}
		size := deltaBytes(delta)
		if sub.bytes+size > s.maxBytes {
			sub.err, sub.closed = ErrOverflow, true
			close(sub.done)
			sub.mu.Unlock()
			delete(s.joins, sub)
			continue
		}
		select {
		case sub.deltas <- delta:
			sub.bytes += size
			sub.mu.Unlock()
		default:
			sub.err = ErrOverflow
			sub.closed = true
			close(sub.done)
			sub.mu.Unlock()
			delete(s.joins, sub)
		}
	}
}

func (s *Store) finishJoinSubscribersLocked() {
	for sub := range s.joins {
		delete(s.joins, sub)
		sub.mu.Lock()
		if !sub.closed {
			sub.closed = true
			close(sub.deltas)
		}
		sub.mu.Unlock()
	}
}

func deltaBytes(delta Delta) int {
	total := 0
	for _, frame := range delta.Frames {
		total += len(frame.Data)
	}
	return total
}

// Subscription is one connection's ordered suffix. The producer never waits
// on it: a subscriber that cannot keep up is closed, and the next connection
// starts from a fresh complete snapshot. Nothing is silently dropped while
// the subscription stays open.
type Subscription struct {
	store  *Store
	frames chan Frame

	mu       sync.Mutex
	bytes    int
	closed   bool
	finished bool
	err      error
	done     chan struct{}
}

// Frames is the ordered suffix. When the run finishes normally the channel
// is closed after the last frame it accepted, so a reader that drains it
// sees the run's terminal row without reconnecting. Done ends a subscription
// that must stop now instead.
func (sub *Subscription) Frames() <-chan Frame { return sub.frames }

// Done closes when the subscription must stop at once: it overflowed, or its
// reader closed it. A run finishing normally does not close it -- that
// closes Frames, so the queued suffix is still delivered.
func (sub *Subscription) Done() <-chan struct{} { return sub.done }

// Err says why the subscription ended, or nil if it has not.
func (sub *Subscription) Err() error {
	sub.mu.Lock()
	defer sub.mu.Unlock()
	return sub.err
}

// Close releases the subscription. It is safe to call more than once, and a
// reader that goes away must call it: a cancelled observation releases its
// own resources and never stops the workflow producing them.
func (sub *Subscription) Close() {
	sub.store.mu.Lock()
	defer sub.store.mu.Unlock()
	delete(sub.store.subs, sub)
	sub.end(nil)
}

// finish says no further frame will be offered. The queue is left intact and
// its channel closed, so a reader keeps draining and then sees the end. It
// is how a run that completed normally hands over its terminal frames.
func (sub *Subscription) finish() {
	sub.mu.Lock()
	defer sub.mu.Unlock()
	if sub.closed || sub.finished {
		return
	}
	sub.finished = true
	close(sub.frames)
}

// Took accounts a frame the reader has taken off the queue, so its bytes
// stop counting against the bound.
func (sub *Subscription) Took(frame Frame) {
	sub.mu.Lock()
	sub.bytes -= len(frame.Data)
	sub.mu.Unlock()
}

// end closes the subscription once. The caller holds no lock but sub.mu.
func (sub *Subscription) end(cause error) {
	sub.mu.Lock()
	defer sub.mu.Unlock()
	if sub.closed {
		return
	}
	sub.closed = true
	sub.err = cause
	close(sub.done)
}

// ended reports whether the subscription has stopped for good -- it does not
// count a normal finish, whose frames are still being handed over.
func (sub *Subscription) ended() bool {
	sub.mu.Lock()
	defer sub.mu.Unlock()
	return sub.closed
}

// offer queues one frame. The store holds its reduction lock, so this is the
// only writer: a full queue or an exceeded byte bound closes the
// subscription instead of blocking the run.
func (sub *Subscription) offer(frame Frame) bool {
	sub.mu.Lock()
	if sub.closed || sub.finished {
		sub.mu.Unlock()
		return false
	}
	if len(sub.frames) >= cap(sub.frames) || sub.bytes+len(frame.Data) > sub.store.maxBytes {
		sub.mu.Unlock()
		sub.end(ErrOverflow)
		return false
	}
	sub.bytes += len(frame.Data)
	sub.mu.Unlock()
	sub.frames <- frame
	return true
}

// Subscribe registers a subscriber and captures the snapshot it starts from
// under one reduction lock. An event accepted after that cut queues behind
// the snapshot; an event accepted before it is already in the snapshot. No
// event can be in both, and none can be in neither.
func (s *Store) Subscribe() (RunSnapshot, *Subscription, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return s.snapshotLocked(), nil, ErrClosed
	}
	snapshot := s.snapshotLocked()
	sub := &Subscription{
		store:  s,
		frames: make(chan Frame, s.maxFrames),
		done:   make(chan struct{}),
	}
	s.subs[sub] = struct{}{}
	return snapshot, sub, nil
}

// publishLocked hands one frame to every subscriber, in order. A subscriber
// that overflowed is dropped here and never sees a partial suffix.
func (s *Store) publishLocked(frame Frame) {
	for sub := range s.subs {
		if !sub.offer(frame) {
			delete(s.subs, sub)
		}
	}
}

// finishSubscribersLocked tells every subscription that the run is over.
//
// A run that ends normally has already published the run row that says so,
// and it may still be queued. Ending the subscription here would let a
// reader return before sending it and make the browser reconnect to learn
// the run finished, so the queue is handed over instead: no further
// frame is accepted, the channel closes after the last one, and the reader
// drains it. Overflow and an abandoned reader still stop immediately -- an
// overflowed suffix is deliberately reset, not delivered.
func (s *Store) finishSubscribersLocked() {
	for sub := range s.subs {
		delete(s.subs, sub)
		sub.finish()
	}
}
