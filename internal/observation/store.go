package observation

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"sync"
	"time"

	"github.com/tylergannon/gimble/internal/sessionstate"
)

// Frame is one SSE frame: the event name and its data. The store produces
// them in the order it reduced them, and every subscriber sees that order.
type Frame struct {
	Name string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

// Delta is one indivisible public-observation change. Position advances once
// for the whole set of frames, so reconnecting can never skip part of a
// lifecycle record or agent event.
type Delta struct {
	Stream   string  `json:"stream"`
	Position uint64  `json:"position"`
	Frames   []Frame `json:"frames"`
}

// Frame names. A connection receives one snapshot, then an ordered suffix of
// row, totals and event frames.
const (
	FrameSnapshot = "snapshot"
	FrameRow      = "row"
	FrameTotals   = "totals"
	FrameEvent    = "event"
)

// transcript is one turn's reduction. The projection is mutated in place;
// only a snapshot detaches it.
type transcript struct {
	projection *sessionstate.Projection
	provenance map[string]json.RawMessage
}

// openCall is a step that started and has not ended: what model it named and
// when it began. It becomes a model_call row when the step ends.
type openCall struct {
	model   string
	started int64
}

// Store is one run's observation: the six tables as Go maps, one transcript
// per turn, and the subscribers the page streams from. Every mutation and
// every detachment goes through mu, so a snapshot and the suffix behind it
// come from one cut.
type Store struct {
	id       string
	dir      string
	registry *Registry

	mu          sync.Mutex
	run         RunRow
	scopes      map[string]*ScopeRow
	sessions    map[string]*SessionRow
	turns       map[string]*TurnRow
	turnUsage   map[string]map[string]Usage
	modelCalls  map[string][]ModelCallRow
	transcripts map[string]*transcript
	calls       map[string]openCall
	subs        map[*Subscription]struct{}
	joins       map[*JoinSubscription]struct{}
	closed      bool
	stream      string
	position    uint64
	recent      []Delta
	journalEnd  int64
	sinceSave   int
	// noWrite suppresses the per-fold file writes. Opening a run sets it: the
	// files are its input, and a rebuild writes them once at the end.
	noWrite bool
	// fromTables says the facts came from the six files. A finished run's
	// tables are its accounting, so the session logs read afterwards are only
	// transcripts: a step in them accounts for nothing.
	fromTables bool

	maxFrames int
	maxBytes  int
}

// newStore is one run's empty tables.
func newStore(registry *Registry, id, name, dir string) *Store {
	return &Store{
		id:          id,
		dir:         dir,
		registry:    registry,
		run:         RunRow{ID: id, Name: name, Status: StatusRunning},
		scopes:      map[string]*ScopeRow{},
		sessions:    map[string]*SessionRow{},
		turns:       map[string]*TurnRow{},
		turnUsage:   map[string]map[string]Usage{},
		modelCalls:  map[string][]ModelCallRow{},
		transcripts: map[string]*transcript{},
		calls:       map[string]openCall{},
		subs:        map[*Subscription]struct{}{},
		joins:       map[*JoinSubscription]struct{}{},
		stream:      newStreamID(),
		maxFrames:   defaultMaxFrames,
		maxBytes:    defaultMaxBytes,
	}
}

func newStreamID() string {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		panic(err)
	}
	return fmt.Sprintf("%x", raw[:])
}

// Open returns the store for one run and registers it, if there is a
// registry. A run started without the web runtime still gets a store: it owns
// it privately and writes the same six files.
//
// The six files are written empty straight away, so a run with no steps still
// has a model_calls.json. The error is that write's, and the run records it as
// a recording failure: the store itself is usable either way.
func Open(registry *Registry, id, name, dir string) (*Store, error) {
	s := newStore(registry, id, name, dir)
	if registry != nil {
		registry.add(s)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s, s.writeAllTablesLocked()
}

// ID is the run's id.
func (s *Store) ID() string { return s.id }

// isOpen reports whether the run is still going. A closed store stays in the
// registry and is still readable; it is no longer live.
func (s *Store) isOpen() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return !s.closed
}

// Close freezes the tables and ends every subscription. It writes nothing:
// every row reached its file as it changed.
func (s *Store) Close() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	s.finishSubscribersLocked()
	s.finishJoinSubscribersLocked()
	return s.saveSnapshotLocked()
}

// record is one lifecycle record, decoded flat: every field any kind carries,
// under the names the log spells. The store switches on Event.Kind rather
// than decoding a sealed union it cannot import.
type record struct {
	Seq     int64     `json:"seq"`
	Time    time.Time `json:"time"`
	Scope   string    `json:"scope"`
	Session string    `json:"session"`
	Turn    string    `json:"turn"`
	Event   struct {
		Kind        string          `json:"kind"`
		Name        string          `json:"name"`
		Adapter     string          `json:"adapter"`
		Model       string          `json:"model"`
		Parent      string          `json:"parent"`
		Error       string          `json:"error"`
		Task        json.RawMessage `json:"task"`
		Key         string          `json:"key"`
		Value       string          `json:"value"`
		Prompt      string          `json:"prompt"`
		OutputType  string          `json:"output_type"`
		Result      string          `json:"result"`
		Interrupted bool            `json:"interrupted"`
		Duration    int64           `json:"duration"`
		Usage       []struct {
			Model  string  `json:"model"`
			Cost   float64 `json:"cost"`
			Tokens struct {
				Input     float64 `json:"input"`
				Output    float64 `json:"output"`
				Reasoning float64 `json:"reasoning"`
				Cache     struct {
					Read  float64 `json:"read"`
					Write float64 `json:"write"`
				} `json:"cache"`
			} `json:"tokens"`
		} `json:"usage"`
	} `json:"event"`
}

// Lifecycle folds one lifecycle record into the tables. The argument is the
// exact LifecycleRecord JSON the run log holds, so the live run and a replay
// of its log reduce the same bytes the same way.
//
// The returned error is a malformed record or a failed table write, and the
// caller records it: the run's recording verdict says when its own store
// stopped describing it.
func (s *Store) Lifecycle(raw json.RawMessage) error {
	if s == nil {
		return nil
	}
	var rec record
	if err := json.Unmarshal(raw, &rec); err != nil {
		return fmt.Errorf("observation: decode lifecycle record: %w", err)
	}
	at := rec.Time.UnixMilli()

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}

	var changed []change
	switch rec.Event.Kind {
	case "run_started":
		s.run.Name, s.run.Status, s.run.Started = rec.Event.Name, StatusRunning, at
		changed = append(changed, change{tableRun, ""})
	case "run_ended":
		// A cancelled run that then reports its body's error stays cancelled:
		// cancellation is the terminal fact, not a second outcome.
		if s.run.Status == StatusRunning {
			s.run.Status, s.run.Error = StatusCompleted, rec.Event.Error
			if rec.Event.Error != "" {
				s.run.Status = StatusFailed
			}
		}
		s.run.Ended = at
		changed = append(changed, change{tableRun, ""})
	case "run_cancelled":
		s.run.Status, s.run.Error, s.run.Ended = StatusCancelled, rec.Event.Error, at
		changed = append(changed, change{tableRun, ""})
	case "scope_began":
		scope := s.scopeLocked(rec.Scope)
		scope.Name, scope.Status, scope.Began = rec.Event.Name, StatusRunning, at
		if len(rec.Event.Task) > 0 {
			scope.Task = rec.Event.Task
		}
		changed = append(changed, change{tableScopes, rec.Scope})
	case "scope_ended":
		// An ended scope with no error is ended, never succeeded.
		scope := s.scopeLocked(rec.Scope)
		scope.Status, scope.Error, scope.Ended = StatusEnded, rec.Event.Error, at
		changed = append(changed, change{tableScopes, rec.Scope})
	case "value_set":
		// The record carries a stored value as its JSON source text.
		scope := s.scopeLocked(rec.Scope)
		scope.Values[rec.Event.Key] = json.RawMessage(rec.Event.Value)
		changed = append(changed, change{tableScopes, rec.Scope})
	case "planner_decision":
		var body struct {
			Event json.RawMessage `json:"event"`
		}
		if err := json.Unmarshal(raw, &body); err != nil {
			return fmt.Errorf("observation: decode planner decision: %w", err)
		}
		scope := s.scopeLocked(rec.Scope)
		scope.Decisions = append(scope.Decisions, Decision{Seq: rec.Seq, Body: body.Event})
		changed = append(changed, change{tableScopes, rec.Scope})
	case "session_created":
		s.sessions[rec.Session] = &SessionRow{
			Run: s.run.ID, ID: rec.Session, Name: rec.Event.Name,
			Adapter: rec.Event.Adapter, Model: rec.Event.Model,
			Scope: rec.Scope, Parent: rec.Event.Parent, Created: at,
		}
		changed = append(changed, change{tableSessions, rec.Session})
	case "turn_started":
		turn := s.turnLocked(Placement{Scope: rec.Scope, Session: rec.Session, Turn: rec.Turn})
		turn.Prompt, turn.OutputType, turn.Started = rec.Event.Prompt, rec.Event.OutputType, at
		changed = append(changed, change{tableTurns, rec.Turn})
	case "turn_ended":
		turn := s.turnLocked(Placement{Scope: rec.Scope, Session: rec.Session, Turn: rec.Turn})
		turn.Result, turn.Error, turn.Interrupted = rec.Event.Result, rec.Event.Error, rec.Event.Interrupted
		turn.Ended, turn.Duration = at, rec.Event.Duration/int64(time.Millisecond)
		// The harness's own report replaces whatever the steps accumulated,
		// including when the report is empty.
		report := make(map[string]Usage, len(rec.Event.Usage))
		for _, model := range rec.Event.Usage {
			report[model.Model] = report[model.Model].add(Usage{
				StatedCost: model.Cost,
				Tokens: Tokens{
					Input: model.Tokens.Input, CacheRead: model.Tokens.Cache.Read,
					CacheWrite: model.Tokens.Cache.Write, Output: model.Tokens.Output,
					Reasoning: model.Tokens.Reasoning,
				},
			})
		}
		s.turnUsage[rec.Turn] = report
		changed = append(changed, change{tableTurns, rec.Turn}, change{tableTurnUsage, rec.Turn})
	}
	return s.commitLocked(changed)
}

// change is one row one record touched: which table, and the key it sits at.
type change struct {
	table string
	key   string
}

// commitLocked writes every table a fold changed, publishes one row frame per
// changed row, and republishes the roll-ups when a turn's usage moved.
func (s *Store) commitLocked(changed []change, extra ...Frame) error {
	if len(changed) == 0 && len(extra) == 0 {
		return nil
	}
	changedTables := make([]string, 0, len(changed))
	usage := false
	for _, one := range changed {
		if !slices.Contains(changedTables, one.table) {
			changedTables = append(changedTables, one.table)
		}
		usage = usage || one.table == tableTurnUsage
	}
	frames := append([]Frame(nil), extra...)
	for _, one := range changed {
		frames = append(frames, Frame{Name: FrameRow, Data: mustMarshal(rowFrame{
			Table: one.table, Key: one.key, Row: s.rowLocked(one.table, one.key),
		})})
	}
	if usage {
		frames = append(frames, Frame{Name: FrameTotals, Data: mustMarshal(s.totalsLocked())})
	}
	delta := Delta{Stream: s.stream, Position: s.position + 1, Frames: frames}
	if err := s.appendDeltaLocked(delta); err != nil {
		return err
	}
	if err := s.writeTablesLocked(changedTables); err != nil {
		return err
	}
	s.position = delta.Position
	s.retainLocked(delta)
	for _, frame := range frames {
		s.publishLocked(frame)
	}
	s.publishDeltaLocked(delta)
	s.sinceSave++
	if s.sinceSave >= snapshotEvery {
		if err := s.saveSnapshotLocked(); err != nil {
			return err
		}
	}
	return nil
}

// rowFrame is the `row` frame's body: the snapshot's value at one key of one
// table. Table names are the snapshot's own keys.
type rowFrame struct {
	Table string `json:"table"`
	Key   string `json:"key"`
	Row   any    `json:"row"`
}

// rowLocked is the snapshot's value at one key of one table.
func (s *Store) rowLocked(table, key string) any {
	switch table {
	case tableRun:
		return s.run
	case tableScopes:
		return *s.scopes[key]
	case tableSessions:
		return *s.sessions[key]
	case tableTurns:
		return *s.turns[key]
	case tableTurnUsage:
		return s.turnUsage[key]
	case tableModelCalls:
		return s.modelCalls[key]
	}
	panic(fmt.Errorf("observation: no table named %s", table))
}

// scopeLocked is one scope's row, created on first sight.
func (s *Store) scopeLocked(key string) *ScopeRow {
	if row, ok := s.scopes[key]; ok {
		return row
	}
	row := &ScopeRow{Run: s.run.ID, Key: key, Name: key, Status: StatusRunning, Values: map[string]json.RawMessage{}}
	s.scopes[key] = row
	return row
}

// turnLocked is one turn's row, created from its placement on first sight so
// a test can feed native events alone.
func (s *Store) turnLocked(at Placement) *TurnRow {
	if row, ok := s.turns[at.Turn]; ok {
		return row
	}
	row := &TurnRow{Run: s.run.ID, ID: at.Turn, Session: at.Session, Scope: at.Scope}
	s.turns[at.Turn] = row
	return row
}

// eventFrame is the `event` frame's body.
type eventFrame struct {
	Scope     string          `json:"scope"`
	Session   string          `json:"session"`
	Turn      string          `json:"turn"`
	Event     json.RawMessage `json:"event"`
	NativeRef json.RawMessage `json:"nativeRef,omitempty"`
}

// Event applies one stamped native event to its turn's transcript, folds what
// the step events say about model calls and usage, and publishes the event.
//
// envelope is the native event exactly as the runtime recorded it.
// nativeRef is the placement sidecar and is never inserted into the event.
//
// The returned error is malformed input or a failed table write, and the
// caller propagates it: an invalidly observed event must not silently
// continue a provider turn.
func (s *Store) Event(at Placement, envelope, nativeRef json.RawMessage) error {
	if s == nil {
		return nil
	}
	decoded, err := sessionstate.DecodeValue(envelope)
	if err != nil {
		return fmt.Errorf("observation: decode event in turn %s: %w", at.Turn, err)
	}
	event, ok := decoded.(*sessionstate.Obj)
	if !ok {
		return fmt.Errorf("observation: event in turn %s is not a JSON object", at.Turn)
	}
	ref, err := decodeRef(nativeRef)
	if err != nil {
		return fmt.Errorf("observation: decode native ref in turn %s: %w", at.Turn, err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return fmt.Errorf("observation: run %s is closed", s.id)
	}
	changed := s.turnSeenLocked(at)
	script := s.transcriptLocked(at.Turn)
	script.projection.Apply(event)
	s.foldProvenanceLocked(script, ref, nativeRef)
	changed = append(changed, s.foldStepLocked(at, event, envelope)...)

	return s.commitLocked(changed, Frame{Name: FrameEvent, Data: mustMarshal(eventFrame{
		Scope: at.Scope, Session: at.Session, Turn: at.Turn,
		Event: envelope, NativeRef: nativeRef,
	})})
}

// turnSeenLocked makes the turn's row if no turn_started record made it, and
// says so, since that is a row the page has not been told about.
//
// A store filled from the tables makes none: turns.json is the authority
// there, and a turn it does not hold gets its transcript and nothing else.
func (s *Store) turnSeenLocked(at Placement) []change {
	if s.fromTables {
		return nil
	}
	if _, ok := s.turns[at.Turn]; ok {
		return nil
	}
	s.turnLocked(at)
	return []change{{tableTurns, at.Turn}}
}

// step is the usage half of a step event's data.
type step struct {
	Message string   `json:"assistantMessageID"`
	Cost    *float64 `json:"cost"`
	Tokens  *struct {
		Input     float64 `json:"input"`
		Output    float64 `json:"output"`
		Reasoning float64 `json:"reasoning"`
		Cache     struct {
			Read  float64 `json:"read"`
			Write float64 `json:"write"`
		} `json:"cache"`
	} `json:"tokens"`
	Model struct {
		ID string `json:"id"`
	} `json:"model"`
}

// foldStepLocked turns a session's step events into model calls and, while
// the turn is still going, into the turn's usage.
//
// A step that ended always accounts for itself; a step that failed does so
// only when it reached the model, which is when its data carries both cost
// and tokens. That is session.go's own rule.
func (s *Store) foldStepLocked(at Placement, event *sessionstate.Obj, envelope json.RawMessage) []change {
	// A store filled from the tables takes no fact from a log. The files are
	// the run's accounting and the log is read for its transcripts.
	if s.fromTables {
		return nil
	}
	kind, _ := event.Get("type").(string)
	ended := kind == "session.step.ended"
	if !ended && kind != "session.step.started" && kind != "session.step.failed" {
		return nil
	}
	var frame struct {
		Data step `json:"data"`
	}
	if err := json.Unmarshal(envelope, &frame); err != nil {
		return nil
	}
	data := frame.Data
	key := at.Turn + "\x00" + data.Message
	created := creationTime(event)
	model := data.Model.ID
	if model == "" {
		model = s.sessions[at.Session].model()
	}

	if kind == "session.step.started" {
		s.calls[key] = openCall{model: model, started: created}
		return nil
	}
	if !ended && (data.Cost == nil || data.Tokens == nil) {
		return nil
	}
	call, seen := s.calls[key]
	if !seen {
		call = openCall{model: model}
	}
	delete(s.calls, key)

	var tokens Tokens
	if data.Tokens != nil {
		tokens = Tokens{
			Input: data.Tokens.Input, CacheRead: data.Tokens.Cache.Read,
			CacheWrite: data.Tokens.Cache.Write, Output: data.Tokens.Output,
			Reasoning: data.Tokens.Reasoning,
		}
	}
	row := ModelCallRow{
		Run: s.run.ID, Turn: at.Turn, Message: data.Message, Model: call.model,
		Tokens: tokens, Started: call.started, Ended: created,
	}
	changed := []change{{tableModelCalls, at.Turn}}

	// A model call is its turn and its assistant message, so folding the same
	// step twice rewrites its row instead of adding a second one, and does
	// not account it twice either. That is what lets a log be read over
	// facts that are already there.
	if i := indexOfCall(s.modelCalls[at.Turn], data.Message); i >= 0 {
		s.modelCalls[at.Turn][i] = row
		return changed
	}
	s.modelCalls[at.Turn] = append(s.modelCalls[at.Turn], row)

	// A step that lands after its turn ended does not touch the turn's usage:
	// the harness's report is already there. That is what makes a replay
	// order-free, and live it never happens.
	if s.turns[at.Turn].Ended != 0 {
		return changed
	}
	var cost float64
	if data.Cost != nil {
		cost = *data.Cost
	}
	byModel := s.turnUsage[at.Turn]
	if byModel == nil {
		byModel = map[string]Usage{}
		s.turnUsage[at.Turn] = byModel
	}
	byModel[call.model] = byModel[call.model].add(Usage{Tokens: tokens, StatedCost: cost})
	return append(changed, change{tableTurnUsage, at.Turn})
}

// indexOfCall finds a turn's model call for one assistant message, or -1. A
// step event that names no message is never matched: there is nothing to
// identify it by.
func indexOfCall(calls []ModelCallRow, message string) int {
	if message == "" {
		return -1
	}
	return slices.IndexFunc(calls, func(have ModelCallRow) bool { return have.Message == message })
}

// model is the session's configured model, which is what a step that names
// none is charged to. A session the store never saw created has none.
func (row *SessionRow) model() string {
	if row == nil {
		return ""
	}
	return row.Model
}

// creationTime is the event's own clock, in Unix ms.
func creationTime(event *sessionstate.Obj) int64 {
	number, ok := event.Get("created").(json.Number)
	if !ok {
		return 0
	}
	created, err := number.Float64()
	if err != nil {
		return 0
	}
	return int64(created)
}

func (s *Store) transcriptLocked(turn string) *transcript {
	if script, ok := s.transcripts[turn]; ok {
		return script
	}
	script := &transcript{
		projection: sessionstate.New(sessionstate.NewProjectionState()),
		provenance: map[string]json.RawMessage{},
	}
	s.transcripts[turn] = script
	return script
}

// Snapshot detaches the complete public observation.
func (s *Store) Snapshot() RunSnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.snapshotLocked()
}

func (s *Store) snapshotLocked() RunSnapshot {
	out := RunSnapshot{
		Stream:      s.stream,
		Position:    s.position,
		Run:         s.run,
		Scopes:      make(map[string]ScopeRow, len(s.scopes)),
		Sessions:    make(map[string]SessionRow, len(s.sessions)),
		Turns:       make(map[string]TurnRow, len(s.turns)),
		TurnUsage:   make(map[string]map[string]Usage, len(s.turnUsage)),
		ModelCalls:  make(map[string][]ModelCallRow, len(s.modelCalls)),
		Totals:      s.totalsLocked(),
		Transcripts: make(map[string]Transcript, len(s.transcripts)),
	}
	for key, row := range s.scopes {
		out.Scopes[key] = cloneScope(*row)
	}
	for id, row := range s.sessions {
		out.Sessions[id] = *row
	}
	for id, row := range s.turns {
		out.Turns[id] = *row
	}
	for turn, byModel := range s.turnUsage {
		out.TurnUsage[turn] = maps.Clone(byModel)
	}
	for turn, calls := range s.modelCalls {
		out.ModelCalls[turn] = slices.Clone(calls)
	}
	for turn, script := range s.transcripts {
		out.Transcripts[turn] = Transcript{
			Snapshot:   script.projection.Snapshot(),
			Provenance: cloneProvenance(script.provenance),
		}
	}
	return out
}

// cloneScope detaches one scope's own JSON, so a snapshot never shares a
// buffer with the store's reduction.
func cloneScope(in ScopeRow) ScopeRow {
	out := in
	out.Task = cloneRaw(in.Task)
	out.Values = make(map[string]json.RawMessage, len(in.Values))
	for key, value := range in.Values {
		out.Values[key] = cloneRaw(value)
	}
	out.Decisions = make([]Decision, 0, len(in.Decisions))
	for _, decision := range in.Decisions {
		out.Decisions = append(out.Decisions, Decision{Seq: decision.Seq, Body: cloneRaw(decision.Body)})
	}
	return out
}

func cloneRaw(in json.RawMessage) json.RawMessage {
	if in == nil {
		return nil
	}
	return append(json.RawMessage(nil), in...)
}

func cloneProvenance(in map[string]json.RawMessage) map[string]json.RawMessage {
	out := make(map[string]json.RawMessage, len(in))
	for key, value := range in {
		copied := make(json.RawMessage, len(value))
		copy(copied, value)
		out[key] = copied
	}
	return out
}

func mustMarshal(value any) json.RawMessage {
	raw, err := json.Marshal(value)
	if err != nil {
		panic(fmt.Errorf("observation: encode frame: %w", err))
	}
	return raw
}
