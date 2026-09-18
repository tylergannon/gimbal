package observation

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// record lines. A fixture is the exact JSON a run log holds, because that is
// what the store folds: live from gimble.Run, and again on replay.
func sessionCreated(scope, session, model string) json.RawMessage {
	return json.RawMessage(`{"seq":1,"time":"2026-09-13T00:00:00Z","scope":"` + scope + `","session":"` + session +
		`","event":{"kind":"session_created","name":"` + session + `","adapter":"fixture","model":"` + model + `","parent":"","workdir":"/w"}}`)
}

func turnStarted(scope, session, turn string) json.RawMessage {
	return json.RawMessage(`{"seq":2,"time":"2026-09-13T00:00:01Z","scope":"` + scope + `","session":"` + session +
		`","turn":"` + turn + `","event":{"kind":"turn_started","prompt":"hello","output_type":"gimble.Text"}}`)
}

// turnEnded is the harness's own report for one turn, as []ModelUsage.
func turnEnded(scope, session, turn, usage string) json.RawMessage {
	return json.RawMessage(`{"seq":3,"time":"2026-09-13T00:00:02Z","scope":"` + scope + `","session":"` + session +
		`","turn":"` + turn + `","event":{"kind":"turn_ended","result":"\"ok\"","error":"","interrupted":false,` +
		`"duration":2000000000,"usage":` + usage + `}}`)
}

func scopeBegan(scope, name string) json.RawMessage {
	return json.RawMessage(`{"seq":4,"time":"2026-09-13T00:00:00Z","scope":"` + scope +
		`","event":{"kind":"scope_began","name":"` + name + `"}}`)
}

func created(id, native string) json.RawMessage {
	return json.RawMessage(`{"id":"` + id + `","type":"session.created","created":10,"data":{"sessionID":"` + native + `"}}`)
}

func stepStarted(id, message, model string) json.RawMessage {
	return json.RawMessage(`{"id":"` + id + `","type":"session.step.started","created":10,"data":{"sessionID":"ses_a","assistantMessageID":"` +
		message + `","agent":"fixture","model":{"id":"` + model + `","providerID":"fixture"}}}`)
}

// stepEnded is one step that reached the model, with the five counts nested
// as the native event carries them.
func stepEnded(id, message string, created int, input, read, write, output, reasoning int, cost string) json.RawMessage {
	return json.RawMessage(`{"id":"` + id + `","type":"session.step.ended","created":` + strconv.Itoa(created) +
		`,"data":{"sessionID":"ses_a","assistantMessageID":"` + message + `","finish":"stop","cost":` + cost +
		`,"tokens":{"input":` + strconv.Itoa(input) + `,"output":` + strconv.Itoa(output) + `,"reasoning":` + strconv.Itoa(reasoning) +
		`,"cache":{"read":` + strconv.Itoa(read) + `,"write":` + strconv.Itoa(write) + `}}}}`)
}

func openStore(t *testing.T) *Store {
	t.Helper()
	store, err := Open(nil, "run-1", "fixture", t.TempDir())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

// fold hands one record to the store and fails the test if it is refused.
func fold(t *testing.T, store *Store, records ...json.RawMessage) {
	t.Helper()
	for _, rec := range records {
		if err := store.Lifecycle(rec); err != nil {
			t.Fatalf("lifecycle %s: %v", rec, err)
		}
	}
}

// drain reads every frame a finished subscription hands over.
func drain(sub *Subscription) []Frame {
	var out []Frame
	for frame := range sub.Frames() {
		sub.Took(frame)
		out = append(out, frame)
	}
	return out
}

func names(frames []Frame) []string {
	out := make([]string, 0, len(frames))
	for _, frame := range frames {
		out = append(out, frame.Name)
	}
	return out
}

// TestSubscribeStartsWithSnapshotThenSuffix is C06 at the public boundary:
// accepted events are in the initial snapshot, and later events are queued as
// its ordered suffix.
func TestSubscribeStartsWithSnapshotThenSuffix(t *testing.T) {
	store := openStore(t)
	fold(t, store, sessionCreated("", "s1", "m"))
	if err := store.Event(Placement{Session: "s1", Turn: "before"}, stepStarted("evt-before", "msg-before", "m"), nil); err != nil {
		t.Fatalf("event before subscribe: %v", err)
	}

	snapshot, sub, err := store.Subscribe()
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer sub.Close()
	if _, ok := snapshot.Transcripts["before"]; !ok {
		t.Fatalf("accepted event missing from initial snapshot: %+v", snapshot.Transcripts)
	}
	raw, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "msg-before") {
		t.Fatalf("initial snapshot lacks state from accepted event: %s", raw)
	}
	if err := store.Event(Placement{Session: "s1", Turn: "after"}, created("evt-after", "ses_a"), nil); err != nil {
		t.Fatalf("event after subscribe: %v", err)
	}
	frame := <-sub.Frames()
	sub.Took(frame)
	if frame.Name != FrameEvent || !strings.Contains(string(frame.Data), `"evt-after"`) {
		t.Fatalf("suffix frame = %s %s, want evt-after event", frame.Name, frame.Data)
	}
}

// TestSubscriberJoiningAfterTurnStartedSeesThePrompt is the other half of the
// cut: what a connection missed is in the snapshot it opens with, so the page
// never asks for a replay.
func TestSubscriberJoiningAfterTurnStartedSeesThePrompt(t *testing.T) {
	store := openStore(t)
	fold(t, store, sessionCreated("", "s1", "m"), turnStarted("", "s1", "t1"))

	snapshot, sub, err := store.Subscribe()
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer sub.Close()
	if got := snapshot.Turns["t1"]; got.Prompt != "hello" || got.Session != "s1" || got.OutputType != "gimble.Text" {
		t.Fatalf("first snapshot's turn = %+v", got)
	}
}

// TestOverflowIsolatesOneSubscriber is C08. A reader that stops reading is
// closed at its bound; the producer does not wait for it, and the subscriber
// beside it keeps receiving the whole ordered suffix.
func TestOverflowIsolatesOneSubscriber(t *testing.T) {
	store := openStore(t)
	fold(t, store, sessionCreated("", "s1", "m"))

	_, healthy, err := store.Subscribe()
	if err != nil {
		t.Fatalf("subscribe healthy: %v", err)
	}
	defer healthy.Close()

	// The next subscriber gets a small bound, so it overflows quickly while
	// the one already registered keeps its own.
	store.mu.Lock()
	store.maxFrames = 4
	store.mu.Unlock()

	_, stalled, err := store.Subscribe()
	if err != nil {
		t.Fatalf("subscribe stalled: %v", err)
	}
	defer stalled.Close()

	// The healthy reader drains continuously; the stalled one never reads.
	seen := make(chan int, 1)
	go func() {
		count := 0
		for frame := range healthy.Frames() {
			healthy.Took(frame)
			if frame.Name == FrameEvent {
				count++
			}
		}
		seen <- count
	}()

	const events = 64
	for i := range events {
		if err := store.Event(Placement{Session: "s1", Turn: "t1"}, created("evt", "ses_a"), nil); err != nil {
			t.Fatalf("event %d: %v", i, err)
		}
	}

	select {
	case <-stalled.Done():
	default:
		t.Fatal("the stalled subscriber was not closed at its bound")
	}
	if err := stalled.Err(); err != ErrOverflow {
		t.Fatalf("stalled subscriber ended with %v, want %v", err, ErrOverflow)
	}
	if err := healthy.Err(); err != nil {
		t.Fatalf("the healthy subscriber was affected: %v", err)
	}

	if err := store.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if count := <-seen; count != events {
		t.Fatalf("the healthy subscriber saw %d event frames, want all %d", count, events)
	}
}

// TestNormalCloseDrainsQueuedFrames is the completion boundary: a run that
// finishes normally hands over what it already published, the run's terminal
// row included, so the browser learns the run ended without reconnecting.
func TestNormalCloseDrainsQueuedFrames(t *testing.T) {
	store := openStore(t)
	_, sub, err := store.Subscribe()
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer sub.Close()

	fold(t, store, json.RawMessage(`{"seq":9,"time":"2026-09-13T00:00:09Z","scope":"","event":{"kind":"run_ended","name":"fixture","error":""}}`))
	if err := store.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	frames := drain(sub) // returns only once the channel is closed
	if len(frames) != 1 || frames[0].Name != FrameRow {
		t.Fatalf("the terminal suffix was %v, want one row frame", names(frames))
	}
	if !strings.Contains(string(frames[0].Data), `"completed"`) {
		t.Fatalf("the terminal frame was %s", frames[0].Data)
	}
	select {
	case <-sub.Done():
		t.Fatal("a normal completion cut the reader off instead of handing over its queue")
	default:
	}
}

// TestReaderGoingAwayDoesNotStopTheRun is the cancellation half: releasing a
// connection frees it and leaves the workflow producing exactly as before.
func TestReaderGoingAwayDoesNotStopTheRun(t *testing.T) {
	store := openStore(t)
	fold(t, store, sessionCreated("", "s1", "m"))

	_, leaving, err := store.Subscribe()
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	_, staying, err := store.Subscribe()
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer staying.Close()

	leaving.Close()
	select {
	case <-leaving.Done():
	default:
		t.Fatal("closing a subscription did not release it")
	}

	if err := store.Event(Placement{Session: "s1", Turn: "t1"}, created("evt1", "ses_a"), nil); err != nil {
		t.Fatalf("the run stopped producing after a reader left: %v", err)
	}
	store.mu.Lock()
	subscribers := len(store.subs)
	store.mu.Unlock()
	if subscribers != 1 {
		t.Fatalf("%d subscribers remain, want only the one still reading", subscribers)
	}

	select {
	case frame := <-staying.Frames():
		if frame.Name != FrameEvent {
			t.Fatalf("the remaining subscriber got %q first", frame.Name)
		}
	default:
		t.Fatal("the remaining subscriber received nothing")
	}
}

// TestFinishedRunStaysReadable is the other side of closing subscriptions: a
// finished run is still inspectable, from the store the registry kept rather
// than from a second replay of its log.
func TestFinishedRunStaysReadable(t *testing.T) {
	project := t.TempDir()
	dir := filepath.Join(project, "runs", "run-1")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	registry := NewRegistry(project)
	store, err := Open(registry, "run-1", "fixture", dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	fold(t, store, sessionCreated("", "s1", "m"))
	if err := store.Event(Placement{Session: "s1", Turn: "t1"}, created("evt1", "ses_a"), nil); err != nil {
		t.Fatalf("event: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if _, live := registry.Live("run-1"); live {
		t.Fatal("a finished run is still registered as live")
	}
	snapshot, err := registry.Snapshot("run-1")
	if err != nil {
		t.Fatalf("a finished run became unreadable: %v", err)
	}
	if _, ok := snapshot.Transcripts["t1"]; !ok {
		t.Fatalf("the finished run's observation lost its transcript: %+v", snapshot)
	}
	if _, err := registry.Snapshot("run-2"); err != ErrNoRun {
		t.Fatalf("an unknown run reported %v, want %v", err, ErrNoRun)
	}
}

// TestOpenWritesEmptyTables is the files half of the store: a run has its
// eight arrays from the start, so a run with no steps still has a
// model_calls.json and nothing has to guess whether a file will appear.
func TestOpenWritesEmptyTables(t *testing.T) {
	dir := t.TempDir()
	if _, err := Open(nil, "run-1", "fixture", dir); err != nil {
		t.Fatalf("open: %v", err)
	}
	for _, table := range tables {
		raw, err := os.ReadFile(filepath.Join(dir, table+".json"))
		if err != nil {
			t.Fatalf("read %s.json: %v", table, err)
		}
		want := "[]"
		if table == tableRun {
			want = `[{"id":"run-1","name":"fixture","status":"running","error":"","started":0,"ended":0}]`
		}
		if string(raw) != want {
			t.Fatalf("%s.json = %s, want %s", table, raw, want)
		}
	}
}

// TestClosedRunHoldsTablesAndFinalSnapshot verifies clean completion saves
// the final reduced state at its exact stream position.
func TestClosedRunHoldsTablesAndFinalSnapshot(t *testing.T) {
	dir := t.TempDir()
	store, err := Open(nil, "run-1", "fixture", dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	fold(t, store,
		scopeBegan("", "."),
		sessionCreated("", "s1", "m"),
		turnStarted("", "s1", "t1"),
		turnEnded("", "s1", "t1", `[{"model":"m","cost":0.5,"tokens":{"input":10,"output":2,"reasoning":1,"cache":{"read":3,"write":4}}}]`),
	)
	// The file is there before Close: the fold wrote it, not the shutdown.
	before, err := os.ReadFile(filepath.Join(dir, "turn_usage.json"))
	if err != nil {
		t.Fatalf("turn_usage.json while live: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	var saved durableSnapshot
	raw, err := os.ReadFile(filepath.Join(dir, snapshotFile))
	if err != nil {
		t.Fatalf("read durable snapshot: %v", err)
	}
	if err := json.Unmarshal(raw, &saved); err != nil {
		t.Fatalf("decode durable snapshot: %v", err)
	}
	if saved.Snapshot.Position != store.Snapshot().Position || saved.Offset == 0 {
		t.Fatalf("saved cursor = %d/%d, live = %d", saved.Snapshot.Position, saved.Offset, store.Snapshot().Position)
	}
	var rows []TurnUsageRow
	if err := json.Unmarshal(before, &rows); err != nil {
		t.Fatalf("decode turn_usage.json: %v", err)
	}
	want := []TurnUsageRow{{Run: "run-1", Turn: "t1", Model: "m",
		Input: 10, CacheRead: 3, CacheWrite: 4, Output: 2, Reasoning: 1, StatedCost: 0.5}}
	if len(rows) != 1 || rows[0] != want[0] {
		t.Fatalf("turn_usage.json = %+v, want %+v", rows, want)
	}
	// And the file decodes to exactly what the snapshot holds.
	if got := store.Snapshot().TurnUsage["t1"]["m"]; got != want[0].Usage {
		t.Fatalf("snapshot's turn usage = %+v, want %+v", got, want[0].Usage)
	}
}

// TestWriteFailureIsReturned is the error path the plan asks for: a table
// that cannot be written is the fold's error, which the run records as a
// recording failure.
func TestWriteFailureIsReturned(t *testing.T) {
	dir := t.TempDir()
	store, err := Open(nil, "run-1", "fixture", dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}
	if err := store.Lifecycle(sessionCreated("", "s1", "m")); err == nil {
		t.Fatal("a fold that could not write its table reported success")
	}
}

// TestTurnIsChargedToTheScopeItRanIn is definition-of-done item 3 and the
// placement correction #157 asked for: a session created at the root and used
// in a child scope puts its turn under the child, and a scope whose name
// merely shares a prefix is not an ancestor.
func TestTurnIsChargedToTheScopeItRanIn(t *testing.T) {
	store := openStore(t)
	fold(t, store,
		scopeBegan("", "."), scopeBegan("lap.1", "lap.1"), scopeBegan("lap.1/task.2", "task.2"), scopeBegan("lap.10", "lap.10"),
		sessionCreated("", "shared.1", "m"),
		turnStarted("lap.1/task.2", "shared.1", "shared.1/turn.1"),
	)
	if err := store.Event(Placement{Scope: "lap.1/task.2", Session: "shared.1", Turn: "shared.1/turn.1"},
		stepStarted("e1", "msg-1", "m"), nil); err != nil {
		t.Fatal(err)
	}
	for i, step := range []json.RawMessage{
		stepEnded("e2", "msg-1", 20, 10, 1, 2, 3, 4, "0.25"),
		stepEnded("e3", "msg-2", 30, 100, 0, 0, 5, 0, "0.75"),
	} {
		if err := store.Event(Placement{Scope: "lap.1/task.2", Session: "shared.1", Turn: "shared.1/turn.1"}, step, nil); err != nil {
			t.Fatalf("step %d: %v", i, err)
		}
	}

	snapshot := store.Snapshot()
	if got := snapshot.Sessions["shared.1"].Scope; got != "" {
		t.Fatalf("the session was created at %q, want the root", got)
	}
	if got := snapshot.Turns["shared.1/turn.1"].Scope; got != "lap.1/task.2" {
		t.Fatalf("the turn ran in %q, want lap.1/task.2", got)
	}
	both := Usage{Input: 110, CacheRead: 1, CacheWrite: 2, Output: 8, Reasoning: 4, StatedCost: 1}
	for _, scope := range []string{"", "lap.1", "lap.1/task.2"} {
		if got := snapshot.Totals.Scopes[scope].All; got != both {
			t.Fatalf("scope %q total = %+v, want %+v", scope, got, both)
		}
		if got := snapshot.Totals.Scopes[scope].ByModel["m"]; got != both {
			t.Fatalf("scope %q per-model total = %+v, want %+v", scope, got, both)
		}
	}
	if got := snapshot.Totals.Scopes["lap.10"].All; got != (Usage{}) {
		t.Fatalf("lap.10 shares a prefix and was charged %+v", got)
	}
	// The session's own total is the same turn, keyed by the session it ran in.
	if got := snapshot.Totals.Sessions["shared.1"].All; got != both {
		t.Fatalf("session total = %+v, want %+v", got, both)
	}
}

// TestTurnEndedReplacesTheLiveUsage is the sum rule's other half: the
// harness's report is the turn's usage, whatever the steps accumulated, and
// an empty report means the turn spent nothing.
func TestTurnEndedReplacesTheLiveUsage(t *testing.T) {
	for _, test := range []struct {
		name   string
		report string
		want   map[string]Usage
	}{
		{"a report under the same model replaces the steps", `[{"model":"m","cost":9,"tokens":{"input":1,"output":0,"reasoning":0,"cache":{"read":0,"write":0}}}]`,
			map[string]Usage{"m": {Tokens: Tokens{Input: 1}, StatedCost: 9}}},
		{"a report under another model moves the turn's usage there", `[{"model":"other","cost":0,"tokens":{"input":7,"output":0,"reasoning":0,"cache":{"read":0,"write":0}}}]`,
			map[string]Usage{"other": {Tokens: Tokens{Input: 7}}}},
		{"an empty report is an empty map", `[]`, map[string]Usage{}},
	} {
		t.Run(test.name, func(t *testing.T) {
			store := openStore(t)
			fold(t, store, scopeBegan("", "."), sessionCreated("", "s1", "m"), turnStarted("", "s1", "t1"))
			at := Placement{Session: "s1", Turn: "t1"}
			if err := store.Event(at, stepEnded("e1", "msg-1", 20, 1000, 0, 0, 0, 0, "5"), nil); err != nil {
				t.Fatal(err)
			}
			fold(t, store, turnEnded("", "s1", "t1", test.report))

			snapshot := store.Snapshot()
			if len(snapshot.TurnUsage["t1"]) != len(test.want) {
				t.Fatalf("turn usage = %+v, want %+v", snapshot.TurnUsage["t1"], test.want)
			}
			for model, want := range test.want {
				if got := snapshot.TurnUsage["t1"][model]; got != want {
					t.Fatalf("turn usage for %s = %+v, want %+v", model, got, want)
				}
			}
			// The step that already ended is still a model call: calls are
			// facts and the report never rewrites them.
			if calls := snapshot.ModelCalls["t1"]; len(calls) != 1 || calls[0].Input != 1000 {
				t.Fatalf("model calls = %+v, want the one step that ended", calls)
			}
			// A step that lands after the turn ended does not touch the usage.
			if err := store.Event(at, stepEnded("e2", "msg-2", 40, 1, 0, 0, 0, 0, "1"), nil); err != nil {
				t.Fatal(err)
			}
			after := store.Snapshot()
			if len(after.TurnUsage["t1"]) != len(test.want) {
				t.Fatalf("a step after turn_ended changed the usage: %+v", after.TurnUsage["t1"])
			}
			if len(after.ModelCalls["t1"]) != 2 {
				t.Fatalf("a step after turn_ended is still a model call: %+v", after.ModelCalls["t1"])
			}
		})
	}
}

// TestStepFailedWithoutCostAddsNothing is session.go's rule: a step that
// failed accounts for itself only when it reached the model, which is when
// its data carries both cost and tokens.
func TestStepFailedWithoutCostAddsNothing(t *testing.T) {
	store := openStore(t)
	fold(t, store, sessionCreated("", "s1", "m"), turnStarted("", "s1", "t1"))
	at := Placement{Session: "s1", Turn: "t1"}
	failed := json.RawMessage(`{"id":"e1","type":"session.step.failed","created":20,"data":{"sessionID":"ses_a",` +
		`"assistantMessageID":"msg-1","tokens":{"input":50,"output":0,"reasoning":0,"cache":{"read":0,"write":0}}}}`)
	if err := store.Event(at, failed, nil); err != nil {
		t.Fatal(err)
	}
	snapshot := store.Snapshot()
	if len(snapshot.TurnUsage["t1"]) != 0 || len(snapshot.ModelCalls["t1"]) != 0 {
		t.Fatalf("a step that never reached the model was accounted: usage %+v calls %+v",
			snapshot.TurnUsage["t1"], snapshot.ModelCalls["t1"])
	}
}

// TestStepPairIsOneModelCall is the drill below a turn: a step that started
// and ended is one call, with the model the step named and both of its times.
func TestStepPairIsOneModelCall(t *testing.T) {
	store := openStore(t)
	fold(t, store, sessionCreated("", "s1", "session-model"), turnStarted("", "s1", "t1"))
	at := Placement{Session: "s1", Turn: "t1"}
	if err := store.Event(at, stepStarted("e1", "msg-1", "step-model"), nil); err != nil {
		t.Fatal(err)
	}
	if err := store.Event(at, stepEnded("e2", "msg-1", 99, 5, 0, 0, 1, 0, "0"), nil); err != nil {
		t.Fatal(err)
	}
	calls := store.Snapshot().ModelCalls["t1"]
	want := ModelCallRow{Run: "run-1", Turn: "t1", Message: "msg-1", Model: "step-model",
		Input: 5, Output: 1, Started: 10, Ended: 99}
	if len(calls) != 1 || calls[0] != want {
		t.Fatalf("model calls = %+v, want %+v", calls, want)
	}
	// The turn's usage is keyed by the step's model, not the session's.
	if got := store.Snapshot().TurnUsage["t1"]; len(got) != 1 || got["step-model"].Input != 5 {
		t.Fatalf("turn usage = %+v, want it under step-model", got)
	}
}

// TestStepWithoutAModelUsesTheSessions is the fallback the plan names: a step
// event that names no model is charged to the model the session runs.
func TestStepWithoutAModelUsesTheSessions(t *testing.T) {
	store := openStore(t)
	fold(t, store, sessionCreated("", "s1", "session-model"), turnStarted("", "s1", "t1"))
	at := Placement{Session: "s1", Turn: "t1"}
	if err := store.Event(at, stepEnded("e1", "msg-1", 20, 3, 0, 0, 0, 0, "0"), nil); err != nil {
		t.Fatal(err)
	}
	if got := store.Snapshot().TurnUsage["t1"]; len(got) != 1 || got["session-model"].Input != 3 {
		t.Fatalf("turn usage = %+v, want it under session-model", got)
	}
}

// TestTotalsArriveAsAFrame is definition-of-done item 5: the roll-ups are
// pushed when a turn's usage changes, so the page never sums anything and
// never reloads to see a new number.
func TestTotalsArriveAsAFrame(t *testing.T) {
	store := openStore(t)
	fold(t, store, scopeBegan("", "."), scopeBegan("lap.1", "lap.1"),
		sessionCreated("", "s1", "m"), turnStarted("lap.1", "s1", "t1"))

	_, sub, err := store.Subscribe()
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer sub.Close()

	at := Placement{Scope: "lap.1", Session: "s1", Turn: "t1"}
	if err := store.Event(at, stepEnded("e1", "msg-1", 20, 42, 0, 0, 0, 0, "0"), nil); err != nil {
		t.Fatal(err)
	}

	var totals Totals
	seen := false
	for range 4 {
		select {
		case frame := <-sub.Frames():
			sub.Took(frame)
			if frame.Name != FrameTotals {
				continue
			}
			if err := json.Unmarshal(frame.Data, &totals); err != nil {
				t.Fatal(err)
			}
			seen = true
		default:
		}
		if seen {
			break
		}
	}
	if !seen {
		t.Fatal("no totals frame followed the step end")
	}
	if got := totals.Scopes["lap.1"].All.Input; got != 42 {
		t.Fatalf("the totals frame says lap.1 spent %v input, want 42", got)
	}
	if got := totals.Scopes[""].ByModel["m"].Input; got != 42 {
		t.Fatalf("the totals frame's root per-model input = %v, want 42", got)
	}
}

// TestConcurrentProducersAndSubscribers is the race check: the reduction, the
// snapshot cut and the queue handoff are all exercised at once under -race.
func TestConcurrentProducersAndSubscribers(t *testing.T) {
	store := openStore(t)
	fold(t, store, sessionCreated("", "s1", "m"))

	var wg sync.WaitGroup
	for turn := range 4 {
		wg.Go(func() {
			at := Placement{Session: "s1", Turn: string(rune('a' + turn))}
			for range 50 {
				_ = store.Event(at, created("evt", "ses_a"), nil)
			}
		})
	}
	for range 4 {
		wg.Go(func() {
			_, sub, err := store.Subscribe()
			if err != nil {
				return
			}
			defer sub.Close()
			for range 20 {
				select {
				case frame, open := <-sub.Frames():
					if !open {
						return
					}
					sub.Took(frame)
				case <-sub.Done():
					return
				}
			}
			_ = store.Snapshot()
		})
	}
	wg.Wait()
	if err := store.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
}

// TestSnapshotCarriesScopeTree is issue 144 item 3: the snapshot holds the
// workflow state a reload must show, not only the sessions and turns.
func TestSnapshotCarriesScopeTree(t *testing.T) {
	store := openStore(t)
	fold(t, store,
		scopeBegan("loop.1", "loop.1"),
		json.RawMessage(`{"seq":2,"time":"2026-09-13T00:00:01Z","scope":"loop.1","event":{"kind":"planner_decision","task":{"name":"write a.txt"}}}`),
		json.RawMessage(`{"seq":3,"time":"2026-09-13T00:00:02Z","scope":"loop.1/task.2","event":{"kind":"scope_began","name":"task.2","task":{"name":"write a.txt"}}}`),
		json.RawMessage(`{"seq":4,"time":"2026-09-13T00:00:03Z","scope":"loop.1/task.2","event":{"kind":"value_set","key":"worker result","value":"\"done\""}}`),
		json.RawMessage(`{"seq":5,"time":"2026-09-13T00:00:04Z","scope":"loop.1/task.2","event":{"kind":"scope_ended","error":""}}`),
	)

	scopes := store.Snapshot().Scopes
	got := scopes["loop.1/task.2"]
	// An ended scope with no error is ended, never succeeded.
	if got.Name != "task.2" || got.Status != StatusEnded || got.Error != "" {
		t.Fatalf("task scope = %+v", got)
	}
	if string(got.Task) != `{"name":"write a.txt"}` || string(got.Values["worker result"]) != `"done"` {
		t.Fatalf("task scope lost its dispatch or its values: %+v", got)
	}
	decisions := scopes["loop.1"].Decisions
	if len(decisions) != 1 || decisions[0].Seq != 2 || !strings.Contains(string(decisions[0].Body), "write a.txt") {
		t.Fatalf("loop scope decisions = %+v", decisions)
	}
	if scopes["loop.1"].Status != StatusRunning {
		t.Fatalf("the running loop scope = %+v", scopes["loop.1"])
	}
}

// TestFoldingAStepTwiceAccountsItOnce is what makes a log safe to read over
// facts already held: a model call is its turn and its assistant message.
func TestFoldingAStepTwiceAccountsItOnce(t *testing.T) {
	store := openStore(t)
	fold(t, store, sessionCreated("", "s1", "m"), turnStarted("", "s1", "t1"))
	at := Placement{Session: "s1", Turn: "t1"}
	for range 2 {
		if err := store.Event(at, stepEnded("e1", "msg-1", 20, 7, 0, 0, 0, 0, "0.5"), nil); err != nil {
			t.Fatal(err)
		}
	}
	snapshot := store.Snapshot()
	if calls := snapshot.ModelCalls["t1"]; len(calls) != 1 {
		t.Fatalf("model calls = %+v, want one", calls)
	}
	want := Usage{Input: 7, StatedCost: 0.5}
	if got := snapshot.TurnUsage["t1"]["m"]; got != want {
		t.Fatalf("turn usage = %+v, want %+v", got, want)
	}
}
