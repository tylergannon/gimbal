package runid

import (
	"bytes"
	"context"
	"encoding/json"
	"strconv"
	"time"

	"github.com/tylergannon/skgo"

	"github.com/tylergannon/gimble/internal/observation"
	"github.com/tylergannon/gimble/internal/sessionstate"
	hooks "github.com/tylergannon/gimble/web/src"
)

// This file is a spike (see ephemeral/research for the brief): one
// `query.live` that replaces the run page's hand-rolled SSE delta stream
// (internal/observation/http.go serveEvents) with a windowed live query.
// Every yield carries the complete current value of only the items that
// changed recently -- no delta log, no resume cursor. It is not meant to be
// merged; internal/observation/http.go stays as the CLI's own reader.

// WindowArg is the run this window follows.
type WindowArg struct {
	Run string `json:"run"`
}

const (
	// windowTick is how often this answers again. skgo's live-query writer
	// already drops a yield whose encoding is byte-identical to the last one
	// sent (see remote_live.go), so ticking faster than the run actually
	// changes costs nothing on the wire -- it only bounds how quickly a
	// healed frame can catch a browser up.
	windowTick = 100 * time.Millisecond

	// windowSpan (T) is how long an item keeps appearing in every yield after
	// it last changed. A frame the browser never saw is healed by the next
	// one inside this span, so a consumer never needs a resume cursor: it
	// just needs to keep listening for T seconds.
	windowSpan = 3 * time.Second
)

// PartFrame is the complete current value of one message header or one
// content-array entry (a text, reasoning or tool part), with enough
// placement for the page to put it where it belongs. Kind is "header" for
// the message's own fields (everything but content) or the part's own
// "type" (text/reasoning/tool/...) for a content entry, in which case Index
// is its position in the message's content array.
type PartFrame struct {
	Turn    string          `json:"turn"`
	Session string          `json:"session"`
	Message string          `json:"message"`
	Kind    string          `json:"kind"`
	Index   int             `json:"index,omitempty"`
	Value   json.RawMessage `json:"value"`
}

// RowFrame is the complete current value of one table row, table and key
// exactly as the store's own row frame names them.
type RowFrame struct {
	Table string          `json:"table"`
	Key   string          `json:"key"`
	Value json.RawMessage `json:"value"`
}

// window is one live yield. It travels to the browser as raw JSON (see
// windowTransport below) for the same reason RunSnapshot does: a part's
// shape is whatever the provider sent, which polytype's static grammar
// cannot spell.
type window struct {
	At     int64                `json:"at"`
	Parts  map[string]PartFrame `json:"parts"`
	Rows   map[string]RowFrame  `json:"rows"`
	Totals json.RawMessage      `json:"totals,omitempty"`
	Ended  bool                 `json:"ended"`
}

// trackedPart is one part or header this window has ever seen change: its
// last-sent frame, when it last actually changed, and whether it is still
// open (streaming). Still-open items are repeated every tick regardless of
// age, which is what lets a client that reconnects after a short outage
// catch up without a cursor: the still-open items are still there next tick.
type trackedPart struct {
	frame   PartFrame
	changed time.Time
	open    bool
}

type trackedRow struct {
	frame   RowFrame
	changed time.Time
}

// turnTracker is one turn's own projection, kept only so this window can
// tell which part of a message a native event actually changed. It applies
// the exact same events, through the exact same public package, the store's
// own transcript does; the only addition here is diffing the result against
// what was last emitted, part by part, instead of resending the message.
type turnTracker struct {
	projection *sessionstate.Projection
	sent       map[string]json.RawMessage
}

// watchWindow answers the run page's one `query.live`. It joins the store
// the same way serveEvents does (Subscribe, not Join: this spike tracks
// current values, not a position to resume from), keeps the current value of
// every part and row it has seen change, and answers again every windowTick
// with the complete current value of whatever changed in the last
// windowSpan, plus every part still open.
func watchWindow(ctx context.Context, arg WindowArg, yield func(hooks.Window) error) error {
	event := skgo.EventFrom(ctx)
	if request := event.Request(); request != nil {
		ctx = request.Context()
	}
	registry := observation.FromContext(ctx)
	if registry == nil {
		return yield(encodeWindow(window{At: time.Now().UnixMilli(), Ended: true}))
	}
	store, live := registry.Live(arg.Run)
	if !live {
		// The run had already finished by the time the page asked: the
		// load's own snapshot is already the complete, final picture, so
		// one ended frame is all this window ever has to say.
		return yield(encodeWindow(window{At: time.Now().UnixMilli(), Ended: true}))
	}
	_, sub, err := store.Subscribe()
	if err != nil {
		return yield(encodeWindow(window{At: time.Now().UnixMilli(), Ended: true}))
	}
	defer sub.Close()

	parts := map[string]*trackedPart{}
	rows := map[string]*trackedRow{}
	var totals *trackedRow
	turns := map[string]*turnTracker{}

	if err := yield(encodeWindow(buildWindow(time.Now(), parts, rows, totals, false))); err != nil {
		return err
	}

	ticker := time.NewTicker(windowTick)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case frame, ok := <-sub.Frames():
			if !ok {
				// The run finished normally and handed over its terminal
				// frames already; one last, complete window closes this out.
				return yield(encodeWindow(buildWindow(time.Now(), parts, rows, totals, true)))
			}
			sub.Took(frame)
			applyFrame(frame, parts, rows, &totals, turns)
		case now := <-ticker.C:
			if err := yield(encodeWindow(buildWindow(now, parts, rows, totals, false))); err != nil {
				return err
			}
		}
	}
}

var _ = skgo.LiveQuery(watchWindow)

func encodeWindow(w window) hooks.Window {
	raw, err := json.Marshal(w)
	if err != nil {
		panic(err)
	}
	return hooks.Window{JSON: string(raw)}
}

// buildWindow is the complete current value of every item that changed in
// the last windowSpan, plus every part still open, regardless of age.
func buildWindow(now time.Time, parts map[string]*trackedPart, rows map[string]*trackedRow, totals *trackedRow, ended bool) window {
	cutoff := now.Add(-windowSpan)
	out := window{At: now.UnixMilli(), Parts: map[string]PartFrame{}, Rows: map[string]RowFrame{}, Ended: ended}
	for key, part := range parts {
		if !part.open && part.changed.Before(cutoff) {
			continue
		}
		out.Parts[key] = part.frame
	}
	for key, row := range rows {
		if row.changed.Before(cutoff) {
			continue
		}
		out.Rows[key] = row.frame
	}
	if totals != nil && !totals.changed.Before(cutoff) {
		out.Totals = totals.frame.Value
	}
	return out
}

// applyFrame folds one frame the store published into this window's own
// tracked items. Row and totals frames are already the complete current
// value of one small thing, so they are tracked as-is. An event frame is the
// one case this spike does real work for: the native event is applied to a
// private, per-turn mirror of the store's own transcript, and only the parts
// that came out different from what this window last saw are tracked -- that
// is the whole windowing bet, that a message part is cheap to notice and
// resend and a whole message is not.
func applyFrame(frame observation.Frame, parts map[string]*trackedPart, rows map[string]*trackedRow, totals **trackedRow, turns map[string]*turnTracker) {
	now := time.Now()
	switch frame.Name {
	case observation.FrameRow:
		var in struct {
			Table string          `json:"table"`
			Key   string          `json:"key"`
			Row   json.RawMessage `json:"row"`
		}
		if json.Unmarshal(frame.Data, &in) != nil {
			return
		}
		key := in.Table + "/" + in.Key
		existing := rows[key]
		if existing != nil && bytes.Equal(existing.frame.Value, in.Row) {
			return
		}
		rows[key] = &trackedRow{frame: RowFrame{Table: in.Table, Key: in.Key, Value: in.Row}, changed: now}

	case observation.FrameTotals:
		if *totals != nil && bytes.Equal((*totals).frame.Value, frame.Data) {
			return
		}
		*totals = &trackedRow{frame: RowFrame{Value: frame.Data}, changed: now}

	case observation.FrameEvent:
		var in struct {
			Session string          `json:"session"`
			Turn    string          `json:"turn"`
			Event   json.RawMessage `json:"event"`
		}
		if json.Unmarshal(frame.Data, &in) != nil {
			return
		}
		applyEvent(in.Turn, in.Session, in.Event, parts, turns, now)
	}
}

// applyEvent applies one native event to its turn's own projection and
// tracks whatever part of whatever message came out different.
func applyEvent(turn, session string, envelope json.RawMessage, parts map[string]*trackedPart, turns map[string]*turnTracker, now time.Time) {
	decoded, err := sessionstate.DecodeValue(envelope)
	if err != nil {
		return
	}
	event, ok := decoded.(*sessionstate.Obj)
	if !ok {
		return
	}
	tracker, ok := turns[turn]
	if !ok {
		tracker = &turnTracker{projection: sessionstate.New(sessionstate.NewProjectionState()), sent: map[string]json.RawMessage{}}
		turns[turn] = tracker
	}
	tracker.projection.Apply(event)

	snap := tracker.projection.Snapshot()
	for _, msg := range snap.State.Message[session] {
		diffMessage(turn, session, msg, tracker.sent, parts, now)
	}
}

// diffMessage compares one message's header and content parts against what
// this window last sent for it, and tracks anything that came out different.
func diffMessage(turn, session string, msg *sessionstate.Obj, sent map[string]json.RawMessage, parts map[string]*trackedPart, now time.Time) {
	id, _ := msg.Get("id").(string)
	if id == "" {
		return
	}

	header := sessionstate.NewObj()
	for _, key := range msg.Keys() {
		if key == "content" {
			continue
		}
		header.Set(key, msg.Get(key))
	}
	headerBytes, err := header.MarshalJSON()
	if err == nil {
		headerKey := "hdr:" + turn + "\x00" + id
		if !bytes.Equal(sent[headerKey], headerBytes) {
			sent[headerKey] = headerBytes
			parts[headerKey] = &trackedPart{
				frame:   PartFrame{Turn: turn, Session: session, Message: id, Kind: "header", Value: headerBytes},
				changed: now,
				open:    rowOpen(msg),
			}
		}
	}

	content, _ := msg.Get("content").([]any)
	for i, entry := range content {
		part, ok := entry.(*sessionstate.Obj)
		if !ok {
			continue
		}
		partBytes, err := part.MarshalJSON()
		if err != nil {
			continue
		}
		partKey := "part:" + turn + "\x00" + id + "\x00" + strconv.Itoa(i)
		if bytes.Equal(sent[partKey], partBytes) {
			continue
		}
		sent[partKey] = partBytes
		kind, _ := part.Get("type").(string)
		parts[partKey] = &trackedPart{
			frame:   PartFrame{Turn: turn, Session: session, Message: id, Kind: kind, Index: i, Value: partBytes},
			changed: now,
			open:    partOpen(kind, part, msg),
		}
	}
}

// partOpen says whether one content part is still being streamed to. A tool
// call closes on its own state; everything else (text, reasoning) closes
// when the message that owns it does, since neither carries its own
// completion marker.
func partOpen(kind string, part, msg *sessionstate.Obj) bool {
	if kind == "tool" {
		state, _ := part.Get("state").(*sessionstate.Obj)
		status, _ := state.Get("status").(string)
		switch status {
		case "completed", "error":
			return false
		default:
			return true
		}
	}
	return rowOpen(msg)
}

// rowOpen says whether one message row (or, inherited, the parts it has no
// completion marker of their own) is still open: no completed time, or a
// status this port uses to mean "still going".
func rowOpen(msg *sessionstate.Obj) bool {
	if t, ok := msg.Get("time").(*sessionstate.Obj); ok && t != nil {
		return !t.Has("completed")
	}
	if status, ok := msg.Get("status").(string); ok {
		switch status {
		case "running", "pending", "streaming":
			return true
		}
	}
	return false
}
