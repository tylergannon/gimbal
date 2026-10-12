// HTTP requests and retained producer SSE streams. Build with the host toolchain:
// go build -buildmode=plugin -o bin/http-sse.so ./plugins/messaging-http
package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/backend"
	"github.com/tylergannon/gimbal/backend/messagefiles"
)

// Each explicitly started producer has its own Listen and EventsDir. Caller
// configuration contains only the two fixed URLs. Native sessions belong to
// AgentURL; processes/services belong to ProcessURL. No discovery is performed.
type config struct{ AgentURL, ProcessURL, Listen, EventsDir string }

func parse(raw json.RawMessage) (config, error) {
	var c config
	if err := json.Unmarshal(raw, &c); err != nil {
		return c, err
	}
	for _, endpoint := range []string{c.AgentURL, c.ProcessURL} {
		u, err := url.Parse(endpoint)
		if err != nil || u.Scheme != "http" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
			return c, fmt.Errorf("http-sse requires plain localhost AgentURL and ProcessURL")
		}
		if u.Hostname() != "localhost" && !net.ParseIP(u.Hostname()).IsLoopback() {
			return c, fmt.Errorf("http-sse supports localhost producers only: %s", endpoint)
		}
	}
	c.AgentURL = strings.TrimRight(c.AgentURL, "/")
	c.ProcessURL = strings.TrimRight(c.ProcessURL, "/")
	if c.AgentURL == c.ProcessURL {
		return c, fmt.Errorf("http-sse requires two distinct producer URLs")
	}
	if (c.Listen == "") != (c.EventsDir == "") {
		return c, fmt.Errorf("producer configuration needs both Listen and EventsDir")
	}
	if c.Listen != "" {
		host, _, err := net.SplitHostPort(c.Listen)
		if err != nil || (host != "localhost" && !net.ParseIP(host).IsLoopback()) {
			return c, fmt.Errorf("producer Listen must be a localhost host:port")
		}
	}
	return c, nil
}

type controlResultKey struct{}

type connection struct {
	config
	client *http.Client
	store  *messagefiles.Store
}

func connect(_ context.Context, raw json.RawMessage) (backend.Messaging, error) {
	c, err := parse(raw)
	if err != nil {
		return nil, err
	}
	b := &connection{config: c, client: &http.Client{Transport: &http.Transport{ResponseHeaderTimeout: 3 * time.Second}}}
	if c.EventsDir != "" {
		b.store, err = messagefiles.Open(c.EventsDir)
	}
	return b, err
}

func (b *connection) post(ctx context.Context, endpoint, path string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint+path, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	response, err := b.client.Do(req)
	if err != nil {
		return fmt.Errorf("http-sse publication acceptance unconfirmed at %s: %w; inspect worker and do not retry as new work", endpoint, err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		reason, _ := io.ReadAll(io.LimitReader(response.Body, 8192))
		return fmt.Errorf("http-sse %s: HTTP %d: %s", endpoint, response.StatusCode, strings.TrimSpace(string(reason)))
	}
	return nil
}

func (b *connection) PublishRequest(ctx context.Context, r backend.MessageRequest) error {
	endpoint, path := b.AgentURL, "/requests"
	switch r.Kind {
	case "process":
		endpoint = b.ProcessURL
	case "cleanup", "interrupt":
		path = "/broadcast"
	case "session", "turn", "close":
	default:
		return fmt.Errorf("http-sse unsupported request kind %q", r.Kind)
	}
	return b.post(ctx, endpoint, path, r)
}
func (b *connection) PublishEvent(ctx context.Context, e backend.MessageEvent) error {
	if result, ok := ctx.Value(controlResultKey{}).(chan backend.MessageEvent); ok {
		if e.Kind == "completed" {
			result <- e
		}
		return nil
	}
	if b.store != nil {
		return b.store.PublishEvent(ctx, e)
	}
	return b.post(ctx, b.AgentURL, "/events", e)
}

// Every stream reconnects using the original inclusive lower bound. Its own
// seen set suppresses duplicates without advancing a timestamp/ULID cursor.
// A temporary disconnect of one producer does not block the other producer.
func (b *connection) ListenEvents(ctx context.Context, s backend.MessageSelection, since time.Time, fn func(backend.MessageEvent) error) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	events := make(chan backend.MessageEvent)
	failures := make(chan error, 2)
	for _, endpoint := range []string{b.AgentURL, b.ProcessURL} {
		go func() { failures <- b.stream(ctx, endpoint, s, since, events) }()
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-failures:
			return err
		case e := <-events:
			if err := fn(e); err != nil {
				return err
			}
		}
	}
}

func (b *connection) stream(ctx context.Context, endpoint string, s backend.MessageSelection, since time.Time, out chan<- backend.MessageEvent) error {
	seen := map[string]bool{}
	failures := 0
	for {
		delivered, err := b.readStream(ctx, endpoint, s, since, seen, out)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if delivered {
			failures = 0
		}
		failures++
		if failures >= 20 {
			return fmt.Errorf("http-sse producer %s delivery unavailable after reconnects: %w", endpoint, err)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
}
func (b *connection) readStream(ctx context.Context, endpoint string, s backend.MessageSelection, since time.Time, seen map[string]bool, out chan<- backend.MessageEvent) (bool, error) {
	q := url.Values{"since": {since.Format(time.RFC3339Nano)}, "request": {s.RequestID}, "run": {s.Run}, "session": {s.SessionID}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"/events?"+q.Encode(), nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("Accept", "text/event-stream")
	response, err := b.client.Do(req)
	if err != nil {
		return false, err
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return false, fmt.Errorf("HTTP %d", response.StatusCode)
	}
	if !strings.HasPrefix(response.Header.Get("Content-Type"), "text/event-stream") {
		return false, fmt.Errorf("producer did not serve SSE")
	}
	reader := bufio.NewReader(response.Body)
	delivered := false
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return delivered, err
		}
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		var e backend.MessageEvent
		if err := json.Unmarshal([]byte(strings.TrimSpace(strings.TrimPrefix(line, "data: "))), &e); err != nil {
			return delivered, fmt.Errorf("invalid producer event: %w", err)
		}
		if e.ID == "" {
			return delivered, fmt.Errorf("producer event has no identity")
		}
		if seen[e.ID] || e.Created.Before(since) || !s.Matches(e) {
			continue
		}
		select {
		case <-ctx.Done():
			return delivered, ctx.Err()
		case out <- e:
			seen[e.ID] = true
			delivered = true
		}
	}
}

type admission struct {
	done chan struct{}
	err  error
}
type producer struct {
	bus      *connection
	ctx      context.Context
	handle   backend.MessageHandler
	mu       sync.Mutex
	admitted map[string]*admission
	errors   chan error
}

func (p *producer) admit(id string, fn func() error) error {
	p.mu.Lock()
	if previous := p.admitted[id]; previous != nil {
		p.mu.Unlock()
		select {
		case <-p.ctx.Done():
			return p.ctx.Err()
		case <-previous.done:
			return previous.err
		}
	}
	a := &admission{done: make(chan struct{})}
	p.admitted[id] = a
	p.mu.Unlock()
	a.err = fn()
	close(a.done)
	return a.err
}

func (p *producer) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /internal/control", func(w http.ResponseWriter, r *http.Request) {
		var request backend.MessageRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil || (request.Kind != "cleanup" && request.Kind != "interrupt") {
			http.Error(w, "invalid control", 400)
			return
		}
		if err := request.Validate(); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		result := make(chan backend.MessageEvent, 1)
		ctx := context.WithValue(p.ctx, controlResultKey{}, result)
		if err := p.handle(ctx, request); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		select {
		case event := <-result:
			_ = json.NewEncoder(w).Encode(event.Result)
		case <-p.ctx.Done():
			http.Error(w, "control cessation unconfirmed", http.StatusServiceUnavailable)
		}
	})
	mux.HandleFunc("POST /requests", func(w http.ResponseWriter, r *http.Request) {
		var request backend.MessageRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil || request.ID == "" {
			http.Error(w, "invalid request", 400)
			return
		}
		if err := request.Validate(); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		// Admission and native ownership use the worker context, never an HTTP
		// connection's lifetime. HTTP accepts delivery immediately; the native
		// handler publishes executor acceptance before starting work.
		err := p.admit(request.ID, func() error {
			go func() {
				if err := p.handle(p.ctx, request); err != nil {
					select {
					case p.errors <- fmt.Errorf("request %s admission/result delivery: %w", request.ID, err):
					case <-p.ctx.Done():
					}
				}
			}()
			return nil
		})
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	})
	mux.HandleFunc("POST /broadcast", func(w http.ResponseWriter, r *http.Request) {
		var request backend.MessageRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil || request.ID == "" || (request.Kind != "cleanup" && request.Kind != "interrupt") {
			http.Error(w, "invalid broadcast control", 400)
			return
		}
		if err := request.Validate(); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		err := p.admit(request.ID, func() error {
			if err := p.fact(request, "accepted", nil); err != nil {
				return err
			}
			go func() {
				if err := p.broadcast(request); err != nil {
					select {
					case p.errors <- err:
					case <-p.ctx.Done():
					}
				}
			}()
			return nil
		})
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	})
	mux.HandleFunc("POST /events", func(w http.ResponseWriter, r *http.Request) {
		var e backend.MessageEvent
		if err := json.NewDecoder(r.Body).Decode(&e); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		if err := p.bus.store.PublishEvent(r.Context(), e); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		w.WriteHeader(http.StatusCreated)
	})
	mux.HandleFunc("GET /events", func(w http.ResponseWriter, r *http.Request) {
		since, err := time.Parse(time.RFC3339Nano, r.URL.Query().Get("since"))
		if err != nil {
			http.Error(w, "invalid since", 400)
			return
		}
		s := backend.MessageSelection{RequestID: r.URL.Query().Get("request"), Run: r.URL.Query().Get("run"), SessionID: r.URL.Query().Get("session")}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, ": connected\n\n")
		if err := http.NewResponseController(w).Flush(); err != nil {
			return
		}
		err = p.bus.store.ListenEvents(r.Context(), s, since, func(e backend.MessageEvent) error {
			data, err := json.Marshal(e)
			if err != nil {
				return err
			}
			if _, err = fmt.Fprintf(w, "id: %s\ndata: %s\n\n", e.ID, data); err != nil {
				return err
			}
			return http.NewResponseController(w).Flush()
		})
		// Closing an SSE stream lets the caller reconnect; errors are not results.
		if err != nil && r.Context().Err() == nil {
			return
		}
	})
	return mux
}
func (p *producer) fact(r backend.MessageRequest, kind string, result *backend.MessageResult) error {
	return p.bus.PublishEvent(p.ctx, backend.MessageEvent{ID: ulid.Make().String(), RequestID: r.ID, Created: time.Now().UTC(), Owner: r.Owner, SessionID: r.SessionID, Kind: kind, Result: result})
}

// The two native owners receive the same logical control privately. Only the
// aggregate is published; task completions remain on their normal public paths.
func (p *producer) broadcast(r backend.MessageRequest) error {
	results := make(chan error, 2)
	for _, endpoint := range []string{p.bus.AgentURL, p.bus.ProcessURL} {
		go func() {
			data, err := json.Marshal(r)
			if err != nil {
				results <- err
				return
			}
			req, err := http.NewRequestWithContext(p.ctx, http.MethodPost, endpoint+"/internal/control", bytes.NewReader(data))
			if err != nil {
				results <- err
				return
			}
			response, err := p.bus.client.Do(req)
			if err != nil {
				results <- err
				return
			}
			defer func() { _ = response.Body.Close() }()
			if response.StatusCode != http.StatusOK {
				results <- fmt.Errorf("control response HTTP %d", response.StatusCode)
				return
			}
			var result backend.MessageResult
			if err = json.NewDecoder(response.Body).Decode(&result); err == nil && (result.Error != "" || result.Unresolved) {
				err = fmt.Errorf("control cessation unconfirmed: %s", result.Error)
			}
			results <- err
		}()
	}
	result := &backend.MessageResult{}
	for range 2 {
		if err := <-results; err != nil {
			result.Unresolved = true
			result.Error += err.Error() + "; "
		}
	}
	return p.fact(r, "completed", result)
}

func worker(ctx context.Context, raw json.RawMessage, handle backend.MessageHandler) error {
	messaging, err := connect(ctx, raw)
	if err != nil {
		return err
	}
	b := messaging.(*connection)
	if b.Listen == "" {
		return fmt.Errorf("http-sse worker needs Listen and EventsDir")
	}
	listener, err := net.Listen("tcp", b.Listen)
	if err != nil {
		return err
	}
	p := &producer{bus: b, ctx: ctx, handle: handle, admitted: map[string]*admission{}, errors: make(chan error, 1)}
	server := &http.Server{Handler: p.handler(), ReadHeaderTimeout: 5 * time.Second}
	done := make(chan error, 1)
	go func() { done <- server.Serve(listener) }()
	defer func() { _ = server.Close() }()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-p.errors:
		return err
	case err := <-done:
		return err
	}
}

func Register(r *gimbal.Registry) error {
	return r.RegisterMessaging(gimbal.MessagingDefinition{Name: "http-sse", ValidateConfiguration: func(raw json.RawMessage) error { _, err := parse(raw); return err }, Connect: connect, Worker: worker})
}
