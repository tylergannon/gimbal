package opencode

import (
	"context"
	"encoding/json"
	"errors"
	"io"
)

func (a *adapter) ensureEvents(ctx context.Context, client *Client) error {
	a.mu.Lock()
	stream := a.events
	if stream == nil {
		streamCtx, cancel := context.WithCancel(context.Background())
		stream = &eventStream{
			cancel: cancel,
			ready:  make(chan struct{}),
			done:   make(chan struct{}),
		}
		a.events = stream
		go a.readEvents(streamCtx, client, stream)
	}
	a.mu.Unlock()

	select {
	case <-stream.ready:
		select {
		case <-stream.done:
			return stream.result()
		default:
			return nil
		}
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (a *adapter) readEvents(ctx context.Context, client *Client, stream *eventStream) {
	err := client.Events(ctx, func(event RawEvent) error {
		stream.once.Do(func() { close(stream.ready) })
		a.handleRawEvent(event)
		return nil
	})
	stream.mu.Lock()
	stream.err = err
	stream.mu.Unlock()
	stream.once.Do(func() { close(stream.ready) })
	close(stream.done)

	a.mu.Lock()
	if a.events == stream {
		a.events = nil
	}
	a.mu.Unlock()
	if err != nil && !errors.Is(err, context.Canceled) {
		a.captureRecord(captureEntry{Kind: "stream_error", Error: err.Error()})
	}
}

func (stream *eventStream) result() error {
	stream.mu.Lock()
	defer stream.mu.Unlock()
	if errors.Is(stream.err, context.Canceled) {
		return nil
	}
	if stream.err == nil {
		return io.ErrUnexpectedEOF
	}
	return stream.err
}

func (a *adapter) handleRawEvent(event RawEvent) {
	eventType, nativeID, sessionID := rawEventIdentity(event.Payload)
	var active *activeTurn
	var requestID string
	if sessionID != "" {
		a.mu.Lock()
		s := a.sessions[sessionID]
		a.mu.Unlock()
		if s != nil {
			s.mu.Lock()
			active = s.active
			if active != nil {
				requestID = active.requestID
			}
			s.mu.Unlock()
		}
	}
	entry := captureEntry{
		Kind: "event", SessionID: sessionID, Workdir: event.Directory,
		Raw: string(event.Raw), Error: event.Malformed,
	}
	if active != nil {
		entry.RequestID = requestID
	}
	a.captureRecord(entry)
	if active == nil || eventType == "" {
		return
	}
	if err := active.observe(eventType, nativeID, event.Payload); err != nil {
		a.captureRecord(captureEntry{
			Kind: "observation_error", RequestID: requestID,
			SessionID: sessionID, Workdir: event.Directory, Error: err.Error(),
		})
	}
}

func rawEventIdentity(payload json.RawMessage) (eventType, eventID, sessionID string) {
	var event struct {
		ID         string `json:"id"`
		Type       string `json:"type"`
		Properties struct {
			SessionID string `json:"sessionID"`
			Info      struct {
				SessionID string `json:"sessionID"`
			} `json:"info"`
			Part struct {
				SessionID string `json:"sessionID"`
			} `json:"part"`
		} `json:"properties"`
	}
	if json.Unmarshal(payload, &event) != nil {
		return "", "", ""
	}
	sessionID = event.Properties.SessionID
	if sessionID == "" {
		sessionID = event.Properties.Info.SessionID
	}
	if sessionID == "" {
		sessionID = event.Properties.Part.SessionID
	}
	return event.Type, event.ID, sessionID
}
