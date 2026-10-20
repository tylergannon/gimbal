// Package zoomconnect records one Zoom Connect call into a Gimbal conversation.
// It reads the call's existing SSE endpoint, GET /api/v1/calls/{id}/stream, and
// appends the normalized events as conversation messages. What Zoom Connect
// exposes is text: transcript lines, public chat, and participant and stream
// lifecycle events. No audio or video is captured.
package zoomconnect

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/tylergannon/gimbal/internal/conversation"
)

// Person is a Zoom Connect participant.
type Person struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// Event is Zoom Connect's normalized meeting event. TS and Recv are when Zoom
// Connect ingested it, in Unix milliseconds; the content's own Zoom time, when
// supplied, is Data["zoom_timestamp"]. Seq orders events within one call.
type Event struct {
	Seq         int64          `json:"seq"`
	TS          int64          `json:"ts"`
	Recv        int64          `json:"recv"`
	Kind        string         `json:"kind"`
	Participant *Person        `json:"participant,omitempty"`
	Data        map[string]any `json:"data"`
}

// Config says which call to record and how to reach and authenticate to its server.
type Config struct {
	// BaseURL is the Zoom Connect server, such as https://zoom.example.com.
	BaseURL string
	// Token is a Zoom Connect API key, sent as a bearer token.
	Token string
	// CallID is Zoom Connect's call ID (a ULID), not Zoom's meeting UUID.
	CallID string
	// Retry is the wait before reconnecting a dropped stream. Zero means two seconds.
	Retry time.Duration
	// Client is the HTTP client. Nil means http.DefaultClient.
	Client *http.Client
}

// Origin names the call and the server that numbered its events: host/call.
// Sequence numbers are only meaningful within one server, so the origin is
// part of the durable source identity.
func (c Config) Origin() string {
	origin := strings.TrimRight(c.BaseURL, "/")
	if u, err := url.Parse(origin); err == nil && u.Host != "" {
		origin = u.Host
	}
	return origin + "/" + c.CallID
}

// Source is the cursor key under which a call's events are recorded.
func (c Config) Source() string { return "zoom-connect:" + c.Origin() }

// FatalError is a response retrying cannot fix: bad key, no access, or no such call.
type FatalError struct {
	Status int
	Body   string
}

func (e *FatalError) Error() string {
	return fmt.Sprintf("zoomconnect: server answered %d: %s", e.Status, strings.TrimSpace(e.Body))
}

var errEnded = errors.New("zoomconnect: call ended")

// ErrUnavailable marks a store failure worth retrying, such as a host that
// restarted or a reply that was lost. The recorder reconnects and asks the store
// for its cursor again, so a write whose reply was lost is never sent twice.
var ErrUnavailable = errors.New("zoomconnect: store unavailable")

// Store holds the conversation a call is recorded into. *conversation.Manager is
// one, for a project no host owns; the gimbal command's control client is the
// other, so that a running host's own manager makes every write.
type Store interface {
	// Recording returns the call's conversation, creating it only the first time.
	Recording(ctx context.Context, source, title string) (string, error)
	Cursor(id, source string) (int64, error)
	Record(id, source string, entries []conversation.Entry) (int, error)
}

// Record follows the call into its conversation until the call ends or ctx is
// done. conversationID may be empty, which uses the call's own conversation. It
// resumes after the store's saved cursor, so a restarted recorder skips what is
// already there, and it reconnects when the stream or the store drops. It
// returns nil when Zoom Connect reports the call ended.
func Record(ctx context.Context, cfg Config, store Store, conversationID string, progress func(Event)) error {
	if cfg.BaseURL == "" || cfg.CallID == "" {
		return errors.New("zoomconnect: base URL and call ID are required")
	}
	retry := cfg.Retry
	if retry == 0 {
		retry = 2 * time.Second
	}
	source := cfg.Source()
	wait := func() error {
		select {
		case <-ctx.Done():
			return context.Cause(ctx)
		case <-time.After(retry):
			return nil
		}
	}
	for conversationID == "" {
		id, err := store.Recording(ctx, source, "Zoom call "+cfg.Origin())
		if err == nil {
			conversationID = id
		} else if !errors.Is(err, ErrUnavailable) {
			return err
		} else if err := wait(); err != nil {
			return err
		}
	}
	for {
		from, err := store.Cursor(conversationID, source)
		if errors.Is(err, ErrUnavailable) {
			if err := wait(); err != nil {
				return err
			}
			continue
		}
		if err != nil {
			return err
		}
		var recordErr error
		err = stream(ctx, cfg, from, func(event Event) error {
			entry, keep := Entry(cfg, event)
			if keep {
				if _, recordErr = store.Record(conversationID, source, []conversation.Entry{entry}); recordErr != nil {
					return recordErr
				}
			}
			if progress != nil {
				progress(event)
			}
			return nil
		})
		var fatal *FatalError
		switch {
		case errors.Is(err, errEnded):
			return nil
		case recordErr != nil && !errors.Is(recordErr, ErrUnavailable):
			return recordErr
		case ctx.Err() != nil:
			return context.Cause(ctx)
		case errors.As(err, &fatal):
			return err
		}
		if err := wait(); err != nil {
			return err
		}
	}
}

// Entry maps one event to a conversation message. The first line is the
// readable speaker and text. A second line carries the event's provenance
// using only what the event supplied: the origin and seq, kind, ts and recv
// (Zoom Connect's ingestion times, in Unix ms), the participant's id and email,
// and data.zoom_timestamp, the original content time the RTMS bridge stores
// separately. Created is ts, left 0 when the event has none. It returns false
// for events that carry nothing to read: active-speaker changes, interim
// transcript text, and kinds this client does not know.
func Entry(cfg Config, event Event) (conversation.Entry, bool) {
	text, _ := event.Data["text"].(string)
	speaker := ""
	if event.Participant != nil {
		speaker = event.Participant.Name
	}
	var line string
	switch event.Kind {
	case "transcript":
		if final, ok := event.Data["final"].(bool); (ok && !final) || strings.TrimSpace(text) == "" {
			return conversation.Entry{}, false
		}
		line = text
		if speaker != "" {
			line = speaker + ": " + text
		}
	case "chat":
		if strings.TrimSpace(text) == "" {
			return conversation.Entry{}, false
		}
		line = "(chat): " + text
		if speaker != "" {
			line = speaker + " (chat): " + text
		}
	case "participant.joined":
		line = speaker + " joined"
	case "participant.left":
		line = speaker + " left"
	case "stream.started", "stream.paused", "stream.stopped", "stream.interrupted", "call.cancelled":
		line = "[" + event.Kind + "]"
		if reason, _ := event.Data["reason"].(string); reason != "" {
			line += " " + reason
		}
	default:
		return conversation.Entry{}, false
	}
	provenance := []string{fmt.Sprintf("%s#%d", cfg.Origin(), event.Seq), event.Kind}
	if event.TS > 0 {
		provenance = append(provenance, "ts="+strconv.FormatInt(event.TS, 10))
	}
	if event.Recv > 0 {
		provenance = append(provenance, "recv="+strconv.FormatInt(event.Recv, 10))
	}
	if stamp, ok := event.Data["zoom_timestamp"]; ok && stamp != nil {
		provenance = append(provenance, "zoom_timestamp="+scalar(stamp))
	}
	if p := event.Participant; p != nil {
		if p.ID != "" {
			provenance = append(provenance, "participant="+p.ID)
		}
		if p.Email != "" {
			provenance = append(provenance, "email="+p.Email)
		}
	}
	line += "\n(" + strings.Join(provenance, " ") + ")"
	return conversation.Entry{Seq: event.Seq, Message: conversation.Message{Role: "zoom", Text: line, Created: max(event.TS, 0)}}, true
}

// scalar prints a JSON number without an exponent, and anything else as is.
func scalar(value any) string {
	if number, ok := value.(float64); ok {
		return strconv.FormatFloat(number, 'f', -1, 64)
	}
	return fmt.Sprint(value)
}

// stream reads one SSE connection, passing each event after from to yield. It
// returns errEnded on Zoom Connect's end event, and otherwise why the
// connection stopped; a stream that closes cleanly returns io.EOF.
func stream(ctx context.Context, cfg Config, from int64, yield func(Event) error) error {
	address := strings.TrimRight(cfg.BaseURL, "/") + "/api/v1/calls/" + url.PathEscape(cfg.CallID) + "/stream?from=" + strconv.FormatInt(from, 10)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return &FatalError{Body: err.Error()}
	}
	request.Header.Set("Accept", "text/event-stream")
	if from > 0 {
		request.Header.Set("Last-Event-ID", strconv.FormatInt(from, 10))
	}
	if cfg.Token != "" {
		request.Header.Set("Authorization", "Bearer "+cfg.Token)
	}
	client := cfg.Client
	if client == nil {
		client = http.DefaultClient
	}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 512))
		if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden || response.StatusCode == http.StatusNotFound {
			return &FatalError{Status: response.StatusCode, Body: string(body)}
		}
		return fmt.Errorf("zoomconnect: server answered %d", response.StatusCode)
	}
	reader := bufio.NewReader(response.Body)
	var name, data string
	for {
		line, err := reader.ReadString('\n')
		line = strings.TrimRight(line, "\r\n")
		switch {
		case line == "" && err == nil:
			if name == "end" {
				return errEnded
			}
			if data != "" {
				var event Event
				if json.Unmarshal([]byte(data), &event) == nil && event.Seq > from {
					if err := yield(event); err != nil {
						return err
					}
				}
			}
			name, data = "", ""
		case strings.HasPrefix(line, "event:"):
			name = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		case strings.HasPrefix(line, "data:"):
			data += strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " ")
		}
		if err != nil {
			return err
		}
	}
}
