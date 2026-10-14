package promises

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/internal/transcript"

	"github.com/tylergannon/polytype"
)

type fake struct {
	calls    int
	requests []request
	reply    func(context.Context, request) ([]operation, error)
	closed   bool
}

func (f *fake) CreateSession(context.Context, string, string, string) (string, error) {
	return "session", nil
}
func (f *fake) RunTurn(ctx context.Context, _ string, text string, _ json.RawMessage, _ func(gimbal.AgentEvent) error) (gimbal.TurnResult, error) {
	_, payload, _ := strings.Cut(text, "\n\nRequest:\n")
	var req request
	if err := json.Unmarshal([]byte(payload), &req); err != nil {
		return gimbal.TurnResult{}, err
	}
	f.calls++
	f.requests = append(f.requests, req)
	ops, err := f.reply(ctx, req)
	if err != nil {
		return gimbal.TurnResult{}, err
	}
	out, _ := json.Marshal(map[string]any{"operations": ops})
	return gimbal.TurnResult{Output: out}, nil
}
func (*fake) Steer(context.Context, string, string) (bool, error) { return false, nil }
func (*fake) Fork(context.Context, string) (string, error)        { return "", nil }
func (f *fake) Close(context.Context, string) error               { f.closed = true; return nil }

func record(t *testing.T, recordings *transcript.DurableStore, id string, marked bool, text string) {
	t.Helper()
	if _, err := recordings.Open(transcript.OpenRequest{SessionID: id, Source: "synthetic", Requirements: marked}); err != nil {
		t.Fatal(err)
	}
	if _, err := recordings.State(id, "recording", nil, polytype.Nullable[int64]{}); err != nil {
		t.Fatal(err)
	}
	if _, err := recordings.Accept(transcript.Update{SessionID: id, Sequence: 1, Finalized: []transcript.Chunk{{Text: text, StartMs: 0, EndMs: 1000}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := recordings.State(id, "stopped", nil, polytype.Nullable[int64]{}); err != nil {
		t.Fatal(err)
	}
}

func TestMarkedSpeechPersistenceAndSources(t *testing.T) {
	dir := t.TempDir()
	r, err := transcript.NewDurable(dir)
	if err != nil {
		t.Fatal(err)
	}
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	record(t, r, "ordinary", false, "The app exports records.")
	f := &fake{reply: func(_ context.Context, req request) ([]operation, error) {
		return []operation{{Op: "add", Title: "Export records", Details: "The app exports records.", Tags: []string{" Functionality ", "export", "export"}, Chunk: 0, Quote: req.NewChunks[0].Text}}, nil
	}}
	model := gimbal.ModelBinding{Adapter: f, Model: "fake"}
	now := time.Now()
	s.step(context.Background(), r, model, now)
	if f.calls != 0 || len(s.Snapshot().Entries) != 0 {
		t.Fatal("ordinary speech entered ledger")
	}
	if _, err := r.MarkRequirements("ordinary"); err != nil {
		t.Fatal(err)
	}
	s.step(context.Background(), r, model, now)
	v := s.Snapshot()
	if len(v.Entries) != 1 || len(v.Entries[0].Tags) != 2 || len(v.Entries[0].Evidence) != 1 || v.Entries[0].Evidence[0].Recording != "ordinary" {
		t.Fatalf("ledger = %+v", v)
	}
	if !f.closed {
		t.Fatal("batch session not released")
	}
	if _, err := s.SetWithdrawn(1, true); err != nil {
		t.Fatal(err)
	}
	fresh, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	fresh.step(context.Background(), r, model, now)
	if f.calls != 1 || !fresh.Snapshot().Entries[0].Withdrawn {
		t.Fatal("reload lost withdrawal or reprocessed speech")
	}
	if _, err := fresh.SetWithdrawn(1, false); err != nil {
		t.Fatal(err)
	}
}

func TestProvisionalSpeechWaitsForFinalization(t *testing.T) {
	dir := t.TempDir()
	r, _ := transcript.NewDurable(dir)
	s, _ := Open(dir)
	_, _ = r.Open(transcript.OpenRequest{SessionID: "live", Source: "synthetic", Requirements: true})
	_, _ = r.State("live", "recording", nil, polytype.Nullable[int64]{})
	_, _ = r.Accept(transcript.Update{SessionID: "live", Sequence: 1, Provisional: []transcript.Chunk{{Text: "unfinished", EndMs: 1000}}})
	f := &fake{reply: func(context.Context, request) ([]operation, error) { return nil, nil }}
	model := gimbal.ModelBinding{Adapter: f, Model: "fake"}
	now := time.Now()
	s.step(context.Background(), r, model, now)
	if f.calls != 0 {
		t.Fatal("processed provisional speech")
	}
	_, _ = r.Accept(transcript.Update{SessionID: "live", Sequence: 2, Finalized: []transcript.Chunk{{Text: "finished", EndMs: 1000}}})
	s.step(context.Background(), r, model, now)
	if f.calls != 0 {
		t.Fatal("did not wait for speech to settle")
	}
	s.step(context.Background(), r, model, now.Add(settleTime))
	if f.calls != 1 || f.requests[0].NewChunks[0].Text != "finished" {
		t.Fatal("did not consume finalized speech")
	}
}

func TestFailedBatchesAreBoundedAndDoNotAdvanceCursor(t *testing.T) {
	dir := t.TempDir()
	r, _ := transcript.NewDurable(dir)
	s, _ := Open(dir)
	record(t, r, "marked", true, "Save this promise.")
	f := &fake{reply: func(context.Context, request) ([]operation, error) { return nil, errors.New("provider failed") }}
	model := gimbal.ModelBinding{Adapter: f, Model: "fake"}
	now := time.Now()
	for i := range 5 {
		s.step(context.Background(), r, model, now.Add(time.Duration(i)*time.Minute))
	}
	if f.calls != attempts || s.ledger.Cursors["marked"] != 0 || s.Snapshot().Working {
		t.Fatalf("calls=%d ledger=%+v", f.calls, s.ledger)
	}
}

func TestApplyKeepsTagsAndGroundsQuotes(t *testing.T) {
	l := Ledger{Entries: []Entry{{ID: 2, Title: "Save", Details: "Save files", Tags: []string{"reliability"}, Evidence: []Evidence{}}}}
	chunks := []chunk{{Chunk: 4, Text: "We save files atomically."}}
	apply(&l, []operation{{Op: "update", ID: 2, Details: "Save atomically", Chunk: 0, Quote: "save files atomically"}}, chunks, "recording", 100)
	if l.Entries[0].Tags[0] != "reliability" || len(l.Entries[0].Evidence) != 1 || l.Entries[0].Evidence[0].Chunk != 4 {
		t.Fatalf("entry=%+v", l.Entries[0])
	}
	apply(&l, []operation{{Op: "withdraw", ID: 2, Quote: "invented quote"}}, chunks, "recording", 200)
	if !l.Entries[0].Withdrawn || len(l.Entries[0].Evidence) != 1 {
		t.Fatalf("withdrawal=%+v", l.Entries[0])
	}
}

func TestInvalidTargetsAndRequiredFieldsDoNotConsumeSpeech(t *testing.T) {
	for _, op := range []operation{{Op: "add", Title: "", Details: "details"}, {Op: "withdraw", ID: 12}, {Op: "update", ID: 12}, {Op: "unknown"}} {
		t.Run(op.Op, func(t *testing.T) {
			dir := t.TempDir()
			r, _ := transcript.NewDurable(dir)
			s, _ := Open(dir)
			record(t, r, "marked", true, "Something")
			f := &fake{reply: func(context.Context, request) ([]operation, error) { return []operation{op}, nil }}
			s.step(context.Background(), r, gimbal.ModelBinding{Adapter: f, Model: "fake"}, time.Now())
			if s.ledger.Cursors["marked"] != 0 || len(s.Snapshot().Entries) != 0 {
				t.Fatal("invalid reply committed")
			}
		})
	}
}

func TestFailedSaveDoesNotPublishMutation(t *testing.T) {
	s, _ := Open(t.TempDir())
	s.ledger.Entries = []Entry{{ID: 1, Title: "Keep me", Tags: []string{}, Evidence: []Evidence{}}}
	if err := os.Mkdir(filepath.Join(s.dir, "ledger.json"), 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SetWithdrawn(1, true); err == nil {
		t.Fatal("save should fail")
	}
	if s.Snapshot().Entries[0].Withdrawn {
		t.Fatal("published unsaved change")
	}
}

func TestCanceledBatchReleasesSession(t *testing.T) {
	dir := t.TempDir()
	r, _ := transcript.NewDurable(dir)
	s, _ := Open(dir)
	record(t, r, "marked", true, "Save this.")
	ctx, cancel := context.WithCancel(context.Background())
	f := &fake{reply: func(ctx context.Context, _ request) ([]operation, error) {
		cancel()
		<-ctx.Done()
		return nil, ctx.Err()
	}}
	s.step(ctx, r, gimbal.ModelBinding{Adapter: f, Model: "fake"}, time.Now())
	if !f.closed || s.Snapshot().Working || s.ledger.Cursors["marked"] != 0 {
		t.Fatal("cancel did not unwind batch")
	}
}

func TestModelSelection(t *testing.T) {
	model, err := Model("haiku")
	if err != nil {
		t.Fatal(err)
	}
	if model.Model == "" || model.Adapter == nil {
		t.Fatal("empty model binding")
	}
	if _, err := Model("gpt-5.6-luna"); err == nil {
		t.Fatal("accepted non-Claude model")
	}
}

func TestWorkerTraversesHistoryAndBatchesWithOverlap(t *testing.T) {
	dir := t.TempDir()
	r, _ := transcript.NewDurable(dir)
	s, _ := Open(dir)
	record(t, r, "old-marked", true, "First promise")
	for i := range 26 {
		id := strings.Repeat("x", i+1)
		record(t, r, id, false, "Ordinary speech")
	}
	f := &fake{reply: func(context.Context, request) ([]operation, error) { return nil, nil }}
	model := gimbal.ModelBinding{Adapter: f, Model: "fake"}
	s.step(context.Background(), r, model, time.Now())
	if f.calls != 1 || s.ledger.Cursors["old-marked"] != 1 {
		t.Fatal("worker ignored older marked recording")
	}
	_, _ = r.Open(transcript.OpenRequest{SessionID: "batch", Source: "synthetic", Requirements: true})
	_, _ = r.State("batch", "recording", nil, polytype.Nullable[int64]{})
	chunks := make([]transcript.Chunk, 14)
	for i := range chunks {
		chunks[i] = transcript.Chunk{StartMs: int64(i * 1000), EndMs: int64(i*1000 + 500), Text: "Commitment"}
	}
	if _, err := r.Accept(transcript.Update{SessionID: "batch", Sequence: 1, Finalized: chunks}); err != nil {
		t.Fatal(err)
	}
	_, _ = r.State("batch", "stopped", nil, polytype.Nullable[int64]{})
	s.step(context.Background(), r, model, time.Now())
	s.step(context.Background(), r, model, time.Now())
	if s.ledger.Cursors["batch"] != 14 || len(f.requests[1].NewChunks) != 12 || len(f.requests[2].NewChunks) != 2 || len(f.requests[2].OverlapChunks) != 2 {
		t.Fatalf("bad batching: %+v", f.requests)
	}
}
