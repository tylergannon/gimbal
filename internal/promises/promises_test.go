package promises

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/internal/transcript"
)

// fake is a HarnessAdapter whose turns answer with what reply returns for
// the request the worker sent.
type fake struct {
	mu       sync.Mutex
	requests []request
	reply    func(ctx context.Context, req request) ([]operation, error)
}

func (f *fake) CreateSession(ctx context.Context, model, effort, workdir string) (string, error) {
	return "session", nil
}

func (f *fake) RunTurn(ctx context.Context, sessionID, prompt string, schema json.RawMessage, onEvent func(gimbal.AgentEvent) error) (gimbal.TurnResult, error) {
	_, payload, _ := strings.Cut(prompt, "\n\nRequest:\n")
	var req request
	if err := json.Unmarshal([]byte(payload), &req); err != nil {
		return gimbal.TurnResult{}, err
	}
	f.mu.Lock()
	f.requests = append(f.requests, req)
	f.mu.Unlock()
	ops, err := f.reply(ctx, req)
	if err != nil {
		return gimbal.TurnResult{}, err
	}
	out, _ := json.Marshal(map[string]any{"operations": ops})
	return gimbal.TurnResult{Output: out}, nil
}

func (f *fake) Steer(ctx context.Context, sessionID, message string) (bool, error) { return false, nil }
func (f *fake) Fork(ctx context.Context, sessionID string) (string, error)         { return "", nil }
func (f *fake) Close(ctx context.Context, sessionID string) error                  { return nil }

func (f *fake) calls() []request {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]request{}, f.requests...)
}

// mentions is how many requests carried text in their new chunks.
func (f *fake) mentions(text string) int {
	n := 0
	for _, req := range f.calls() {
		for _, c := range req.NewChunks {
			if strings.Contains(c.Text, text) {
				n++
				break
			}
		}
	}
	return n
}

func add(title string, chunk int, quote string, tags ...string) operation {
	return operation{Op: "add", Title: title, Details: "Details of " + title, Tags: tags, Chunk: chunk, Quote: quote, Reason: "new"}
}

func setup(t *testing.T, reply func(ctx context.Context, req request) ([]operation, error)) (*Store, *transcript.DurableStore, *fake, gimbal.ModelBinding) {
	t.Helper()
	dir := t.TempDir()
	recordings, err := transcript.NewDurable(dir)
	if err != nil {
		t.Fatal(err)
	}
	store, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	f := &fake{reply: reply}
	return store, recordings, f, gimbal.ModelBinding{Adapter: f, Model: "fake"}
}

// record opens recording id with one finalized chunk per text, live or stopped.
func record(t *testing.T, recordings *transcript.DurableStore, id string, requirements bool, status string, texts ...string) {
	t.Helper()
	if _, err := recordings.Open(transcript.OpenRequest{SessionID: id, Source: "synthetic", Requirements: requirements}); err != nil {
		t.Fatal(err)
	}
	if _, err := recordings.State(id, "recording", nil); err != nil {
		t.Fatal(err)
	}
	speak(t, recordings, id, 1, 0, texts...)
	if status != "recording" {
		if _, err := recordings.State(id, status, nil); err != nil {
			t.Fatal(err)
		}
	}
}

// speak finalizes texts as update sequence, after the chunks already spoken.
func speak(t *testing.T, recordings *transcript.DurableStore, id string, sequence int64, spoken int, texts ...string) {
	t.Helper()
	var chunks []transcript.Chunk
	for i, text := range texts {
		start := int64(spoken+i) * 1000
		chunks = append(chunks, transcript.Chunk{StartMs: start, EndMs: start + 500, Text: text})
	}
	if _, err := recordings.Accept(transcript.Update{SessionID: id, Sequence: sequence, Finalized: chunks}); err != nil {
		t.Fatal(err)
	}
}

func cursor(s *Store, id string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ledger.Cursors[id]
}

func errorFor(view View, id string) string {
	for _, f := range view.Errors {
		if f.Recording == id {
			return f.Message
		}
	}
	return ""
}

func TestApply(t *testing.T) {
	chunks := []chunk{{Chunk: 4, Text: "Exports finish   in under TWO seconds."}, {Chunk: 5, Text: "Users can tag notes."}}
	l := Ledger{Entries: []Entry{}, Cursors: map[string]int{}}
	apply(&l, []operation{
		add("Exports finish in under two seconds.", 4, "exports finish in under two seconds", " Performance ", "performance", "EXPORT", ""),
		add("Users can tag notes.", 4, "users can TAG   notes"),
		add("It reads minds.", 5, "it reads minds"),
	}, chunks, "rec", 100)
	if len(l.Entries) != 3 || l.Entries[0].ID != 1 || l.Entries[1].ID != 2 || l.Entries[2].ID != 3 {
		t.Fatalf("ids: %+v", l.Entries)
	}
	if got := strings.Join(l.Entries[0].Tags, ","); got != "performance,export" {
		t.Fatalf("tags = %q, want lowercase, trimmed, deduped", got)
	}
	if ev := l.Entries[0].Evidence; len(ev) != 1 || ev[0] != (Evidence{Recording: "rec", Chunk: 4, Quote: "exports finish in under two seconds"}) {
		t.Fatalf("evidence = %+v", ev)
	}
	if ev := l.Entries[1].Evidence; len(ev) != 1 || ev[0].Chunk != 5 {
		t.Fatalf("a quote found in another new chunk is attributed to it: %+v", ev)
	}
	if len(l.Entries[2].Evidence) != 0 || l.Entries[2].Title != "It reads minds." {
		t.Fatalf("a fabricated quote applies the op without evidence: %+v", l.Entries[2])
	}

	first := l.Entries[0]
	apply(&l, []operation{{Op: "update", ID: 1, Title: first.Title, Details: first.Details, Tags: []string{"PERFORMANCE", "export"}, Chunk: 5, Quote: "Users can tag notes."}}, chunks, "rec", 200)
	if e := l.Entries[0]; e.Updated != 100 || e.Title != first.Title || len(e.Evidence) != 2 {
		t.Fatalf("an equivalent update records evidence only: %+v", e)
	}

	apply(&l, []operation{{Op: "update", ID: 2, Title: "Users can tag and filter notes.", Tags: []string{"tags"}, Chunk: 5, Quote: "users can tag notes"}}, chunks, "rec", 300)
	if e := l.Entries[1]; e.Title != "Users can tag and filter notes." || e.Details != "Details of Users can tag notes." || e.Updated != 300 || len(e.Evidence) != 2 || e.Tags[0] != "tags" {
		t.Fatalf("a changed update replaces, keeping an empty field's current value: %+v", e)
	}
	apply(&l, []operation{{Op: "update", ID: 2, Details: "Filter by tag too."}}, chunks, "rec", 350)
	if e := l.Entries[1]; e.Title != "Users can tag and filter notes." || e.Details != "Filter by tag too." || !slices.Equal(e.Tags, []string{"tags"}) {
		t.Fatalf("an empty title or tags keeps the current one: %+v", e)
	}

	apply(&l, []operation{{Op: "withdraw", ID: 3, Chunk: 5, Quote: "users can tag notes"}}, chunks, "rec", 400)
	apply(&l, []operation{{Op: "withdraw", ID: 3, Chunk: 5, Quote: "users can tag notes"}}, chunks, "rec", 500)
	if e := l.Entries[2]; !e.Withdrawn || e.Updated != 400 || len(e.Evidence) != 1 {
		t.Fatalf("withdraw, then a repeated withdraw is a no-op: %+v", e)
	}
	apply(&l, []operation{{Op: "update", ID: 3, Title: "It reads some minds.", Details: "d"}}, chunks, "rec", 600)
	apply(&l, []operation{add("Fourth.", 4, "")}, chunks, "rec", 700)
	if !l.Entries[2].Withdrawn || l.Entries[3].ID != 4 {
		t.Fatalf("a withdrawn entry stays withdrawn and its id is never reused: %+v", l.Entries)
	}
}

func TestQuoteIsTruncated(t *testing.T) {
	long := strings.Repeat("é", 400)
	ev := cite(operation{Chunk: 0, Quote: long}, []chunk{{Chunk: 0, Text: long}}, "rec")
	if len(ev) != 1 || len([]rune(ev[0].Quote)) != quoteLimit {
		t.Fatalf("evidence = %+v", ev)
	}
}

func TestSettle(t *testing.T) {
	store, recordings, f, model := setup(t, func(ctx context.Context, req request) ([]operation, error) { return nil, nil })
	now := time.Now()
	record(t, recordings, "live", true, "recording", "a", "b")
	record(t, recordings, "ordinary", false, "stopped", "not requirements")
	store.step(context.Background(), recordings, model, now)
	if len(f.calls()) != 0 {
		t.Fatal("a live recording was processed before its chunks settled")
	}
	store.step(context.Background(), recordings, model, now.Add(7*time.Second))
	if len(f.calls()) != 0 {
		t.Fatal("processed at 7 s")
	}
	store.step(context.Background(), recordings, model, now.Add(settleTime))
	if len(f.calls()) != 1 || cursor(store, "live") != 2 {
		t.Fatalf("not processed after 8 s quiet: %d calls, cursor %d", len(f.calls()), cursor(store, "live"))
	}

	speak(t, recordings, "live", 2, 2, "c")
	store.step(context.Background(), recordings, model, now.Add(9*time.Second))
	speak(t, recordings, "live", 3, 3, "d")
	store.step(context.Background(), recordings, model, now.Add(16*time.Second))
	if len(f.calls()) != 1 {
		t.Fatal("new speech restarts the quiet period")
	}
	if _, err := recordings.State("live", "stopped", nil); err != nil {
		t.Fatal(err)
	}
	store.step(context.Background(), recordings, model, now.Add(16*time.Second))
	if len(f.calls()) != 2 || cursor(store, "live") != 4 {
		t.Fatal("a stopped recording is processed immediately")
	}
	for range 3 {
		store.step(context.Background(), recordings, model, now.Add(time.Minute))
	}
	if len(f.calls()) != 2 || f.mentions("not requirements") != 0 {
		t.Fatal("processed again, or processed an ordinary recording")
	}
}

func TestBatchesOfTwelveWithTwoOverlap(t *testing.T) {
	store, recordings, f, model := setup(t, func(ctx context.Context, req request) ([]operation, error) { return nil, nil })
	var texts []string
	for i := range 15 {
		texts = append(texts, fmt.Sprintf("chunk %d", i))
	}
	record(t, recordings, "long", true, "recording", texts...)
	now := time.Now()
	store.step(context.Background(), recordings, model, now)
	store.step(context.Background(), recordings, model, now)
	calls := f.calls()
	if len(calls) != 1 || len(calls[0].NewChunks) != 12 || len(calls[0].OverlapChunks) != 0 || calls[0].NewChunks[11].Chunk != 11 {
		t.Fatalf("first batch: %+v", calls)
	}
	if _, err := recordings.State("long", "stopped", nil); err != nil {
		t.Fatal(err)
	}
	store.step(context.Background(), recordings, model, now)
	calls = f.calls()
	if len(calls) != 2 || len(calls[1].NewChunks) != 3 || calls[1].NewChunks[0].Chunk != 12 ||
		len(calls[1].OverlapChunks) != 2 || calls[1].OverlapChunks[0] != (chunk{Chunk: 10, Text: "chunk 10"}) {
		t.Fatalf("second batch: %+v", calls[1])
	}
	if cursor(store, "long") != 15 {
		t.Fatalf("cursor = %d", cursor(store, "long"))
	}
}

func TestInvalidReplyFailsTheWholeBatch(t *testing.T) {
	for name, op := range map[string]operation{
		"empty details":   {Op: "add", Title: "A promise.", Quote: "a"},
		"empty title":     {Op: "add", Details: "Some details.", Quote: "a"},
		"unknown id":      {Op: "update", ID: 9, Title: "t", Details: "d"},
		"withdraw absent": {Op: "withdraw", ID: 9},
		"unknown op":      {Op: "confirm", ID: 1},
	} {
		t.Run(name, func(t *testing.T) {
			store, recordings, _, model := setup(t, func(ctx context.Context, req request) ([]operation, error) {
				return []operation{add("Valid.", 0, "a"), op}, nil
			})
			record(t, recordings, "rec", true, "stopped", "a")
			store.step(context.Background(), recordings, model, time.Now())
			view := store.Snapshot()
			if cursor(store, "rec") != 0 || len(view.Entries) != 0 || !strings.Contains(errorFor(view, "rec"), "operation 1") {
				t.Fatalf("cursor %d, view %+v", cursor(store, "rec"), view)
			}
		})
	}
}

func TestFailingRecordingBacksOffAndParks(t *testing.T) {
	store, recordings, f, model := setup(t, func(ctx context.Context, req request) ([]operation, error) {
		if req.NewChunks[0].Text == "a speaks" {
			return nil, fmt.Errorf("model unavailable")
		}
		return []operation{add("B works.", req.NewChunks[0].Chunk, "b speaks")}, nil
	})
	record(t, recordings, "a", true, "stopped", "a speaks")
	record(t, recordings, "b", true, "stopped", "b speaks")
	now := time.Now()
	for _, at := range []time.Duration{0, 1, 2, 10, 20, 21, 40, 60, 61, 120, 600} {
		store.step(context.Background(), recordings, model, now.Add(at*time.Second))
	}
	if n := f.mentions("a speaks"); n != 3 {
		t.Fatalf("a was tried %d times, want 3", n)
	}
	view := store.Snapshot()
	if cursor(store, "b") != 1 || len(view.Entries) != 1 || errorFor(view, "b") != "" {
		t.Fatalf("b did not progress past a: %+v", view)
	}
	if msg := errorFor(view, "a"); !strings.Contains(msg, "model unavailable") || !strings.HasSuffix(msg, "stopped after 3 attempts; restart Gimbal to retry") {
		t.Fatalf("a's error = %q", msg)
	}
	if cursor(store, "a") != 0 {
		t.Fatal("a's cursor advanced")
	}
}

func TestWorkingAndErrors(t *testing.T) {
	var during []bool
	var store *Store
	fail := true
	store, recordings, _, model := setup(t, func(ctx context.Context, req request) ([]operation, error) {
		during = append(during, store.Snapshot().Working)
		if fail && req.NewChunks[0].Text == "a" {
			return nil, fmt.Errorf("boom")
		}
		return nil, nil
	})
	record(t, recordings, "a", true, "stopped", "a")
	now := time.Now()
	before := store.Snapshot().Version
	store.step(context.Background(), recordings, model, now)
	view := store.Snapshot()
	if view.Working || errorFor(view, "a") != "boom" || view.Version <= before || cursor(store, "a") != 0 {
		t.Fatalf("after failure: %+v", view)
	}
	store.step(context.Background(), recordings, model, now.Add(time.Second))
	if len(during) != 1 {
		t.Fatal("retried during backoff")
	}

	record(t, recordings, "b", true, "stopped", "b")
	store.step(context.Background(), recordings, model, now.Add(2*time.Second))
	view = store.Snapshot()
	if view.Working || cursor(store, "b") != 1 || errorFor(view, "a") != "boom" {
		t.Fatalf("a success for b cleared a's failure: %+v", view)
	}

	fail = false
	store.step(context.Background(), recordings, model, now.Add(20*time.Second))
	view = store.Snapshot()
	if view.Working || len(view.Errors) != 0 || cursor(store, "a") != 1 {
		t.Fatalf("a's success did not clear its failure: %+v", view)
	}
	if len(during) != 3 || !during[0] || !during[1] || !during[2] {
		t.Fatalf("working during the calls: %v", during)
	}
}

func TestReopenRestoresWithoutReprocessing(t *testing.T) {
	store, recordings, f, model := setup(t, func(ctx context.Context, req request) ([]operation, error) {
		return []operation{add("It never loses an unsaved note.", 0, "never loses an unsaved note", "reliability")}, nil
	})
	record(t, recordings, "rec", true, "stopped", "It never loses an unsaved note.")
	store.step(context.Background(), recordings, model, time.Now())
	store.step(context.Background(), recordings, model, time.Now())
	if len(f.calls()) != 1 {
		t.Fatalf("calls = %d after success, want 1", len(f.calls()))
	}
	if _, err := store.SetWithdrawn(1, true); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(filepath.Dir(store.dir))
	if err != nil {
		t.Fatal(err)
	}
	view := reopened.Snapshot()
	if len(view.Entries) != 1 || !view.Entries[0].Withdrawn || view.Entries[0].Evidence[0].Quote != "never loses an unsaved note" || cursor(reopened, "rec") != 1 || view.Working {
		t.Fatalf("reopened: %+v", view)
	}
	reopened.step(context.Background(), recordings, model, time.Now())
	if len(f.calls()) != 1 {
		t.Fatal("a reopened ledger reprocessed its recording")
	}
	if entry, err := reopened.SetWithdrawn(1, false); err != nil || entry.Withdrawn {
		t.Fatalf("restore = %+v, %v", entry, err)
	}
	if _, err := reopened.SetWithdrawn(7, true); err == nil {
		t.Fatal("withdrew an unknown promise")
	}
}

func TestUnwritableLedgerKeepsTheOldLedger(t *testing.T) {
	store, recordings, f, model := setup(t, func(ctx context.Context, req request) ([]operation, error) {
		return []operation{add("Users can tag notes.", req.NewChunks[0].Chunk, "users can tag notes")}, nil
	})
	record(t, recordings, "rec", true, "stopped", "Users can tag notes.")
	if err := os.Chmod(store.dir, 0500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(store.dir, 0755) })
	now := time.Now()
	store.step(context.Background(), recordings, model, now)
	view := store.Snapshot()
	if len(view.Entries) != 0 || cursor(store, "rec") != 0 || view.Working || !strings.Contains(errorFor(view, "rec"), "save ledger") {
		t.Fatalf("after a failed save: %+v", view)
	}
	if err := os.Chmod(store.dir, 0755); err != nil {
		t.Fatal(err)
	}
	store.step(context.Background(), recordings, model, now.Add(time.Second))
	if len(f.calls()) != 1 {
		t.Fatal("retried during backoff")
	}
	store.step(context.Background(), recordings, model, now.Add(15*time.Second))
	view = store.Snapshot()
	if len(f.calls()) != 2 || len(view.Entries) != 1 || view.Entries[0].ID != 1 || cursor(store, "rec") != 1 || len(view.Errors) != 0 {
		t.Fatalf("after the retry: %+v", view)
	}
}

func TestMarkedRecordingOnALaterPage(t *testing.T) {
	store, recordings, f, model := setup(t, func(ctx context.Context, req request) ([]operation, error) { return nil, nil })
	record(t, recordings, "r00", true, "stopped", "the oldest")
	for i := 1; i < 30; i++ {
		time.Sleep(time.Millisecond)
		record(t, recordings, fmt.Sprintf("r%02d", i), false, "stopped", "ordinary")
	}
	if first := recordings.List(""); first.Next == "" || first.Items[len(first.Items)-1].ID == "r00" {
		t.Fatal("the marked recording is on the first page")
	}
	store.step(context.Background(), recordings, model, time.Now())
	if f.mentions("the oldest") != 1 || cursor(store, "r00") != 1 {
		t.Fatal("a marked recording on page 2 was not processed")
	}
}

func TestWithdrawDuringTheCallSurvives(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	store, recordings, _, model := setup(t, func(ctx context.Context, req request) ([]operation, error) {
		if len(req.Promises) == 0 {
			return []operation{add("Keep the cedar bench.", 0, "cedar bench")}, nil
		}
		close(entered)
		<-release
		return []operation{{Op: "withdraw", ID: 1, Chunk: 1, Quote: "drop it"}, add("Install a plaque.", 1, "plaque")}, nil
	})
	record(t, recordings, "rec", true, "recording", "Keep the cedar bench.")
	now := time.Now()
	store.step(context.Background(), recordings, model, now)
	store.step(context.Background(), recordings, model, now.Add(settleTime))
	speak(t, recordings, "rec", 2, 1, "Install a plaque. Actually drop it.")
	if _, err := recordings.State("rec", "stopped", nil); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		store.step(context.Background(), recordings, model, now.Add(time.Minute))
	}()
	<-entered
	if _, err := store.SetWithdrawn(1, true); err != nil {
		t.Fatal(err)
	}
	close(release)
	<-done
	view := store.Snapshot()
	if len(view.Entries) != 2 || !view.Entries[0].Withdrawn || len(view.Entries[0].Evidence) != 1 || view.Entries[1].Title != "Install a plaque." || len(view.Errors) != 0 {
		t.Fatalf("after apply: %+v", view)
	}
}

func TestShutdownUnwindsTheTurn(t *testing.T) {
	entered := make(chan struct{})
	var unwound atomic.Bool
	store, recordings, _, model := setup(t, func(ctx context.Context, req request) ([]operation, error) {
		close(entered)
		<-ctx.Done()
		time.Sleep(50 * time.Millisecond)
		unwound.Store(true)
		return nil, ctx.Err()
	})
	record(t, recordings, "rec", true, "stopped", "Exports finish in under two seconds.")
	ctx, cancel := context.WithCancel(context.Background())
	returned := make(chan struct{})
	go func() {
		defer close(returned)
		store.Run(ctx, recordings, model)
	}()
	<-entered
	cancel()
	<-returned
	if !unwound.Load() {
		t.Fatal("Run returned before the turn unwound")
	}
	time.Sleep(50 * time.Millisecond)
	view := store.Snapshot()
	if view.Working || len(view.Errors) != 0 || cursor(store, "rec") != 0 {
		t.Fatalf("after shutdown: %+v", view)
	}
	if _, err := os.Stat(filepath.Join(store.dir, "ledger.json")); !os.IsNotExist(err) {
		t.Fatalf("the ledger was saved: %v", err)
	}
}

func TestModel(t *testing.T) {
	model, err := Model("")
	if err != nil || !strings.HasPrefix(model.Model, "claude-sonnet") || model.Effort != "low" {
		t.Fatalf("default = %+v, %v", model, err)
	}
	if model, err := Model("haiku"); err != nil || !strings.HasPrefix(model.Model, "claude-haiku") {
		t.Fatalf("haiku = %+v, %v", model, err)
	}
	for _, spec := range []string{"sol", "nonsense", "sonnet:extreme"} {
		if _, err := Model(spec); err == nil {
			t.Fatalf("Model(%q) = nil error", spec)
		}
	}
}
