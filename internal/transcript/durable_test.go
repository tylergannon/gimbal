package transcript

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/tylergannon/polytype"
)

func TestDurableCorrectionsRestartAndHistory(t *testing.T) {
	dir := t.TempDir()
	s, e := NewDurable(dir)
	if e != nil {
		t.Fatal(e)
	}
	open := OpenRequest{SessionID: "one", Source: "synthetic"}
	if _, e = s.Open(open); e != nil {
		t.Fatal(e)
	}
	if _, e = s.State("one", "recording", nil, polytype.Nullable[int64]{}); e != nil {
		t.Fatal(e)
	}
	first := Update{SessionID: "one", Sequence: 1, Provisional: []Chunk{{StartMs: 0, EndMs: 1000, Text: "wrong"}}}
	if _, e = s.Accept(first); e != nil {
		t.Fatal(e)
	}
	corrected := Update{SessionID: "one", Sequence: 2, Finalized: []Chunk{{StartMs: 0, EndMs: 1000, Text: "correct"}}}
	if _, e = s.Accept(corrected); e != nil {
		t.Fatal(e)
	}
	if _, e = s.Accept(corrected); e != nil {
		t.Fatal(e)
	}
	bad := corrected
	bad.Finalized = []Chunk{{StartMs: 0, EndMs: 1000, Text: "conflict"}}
	if _, e = s.Accept(bad); e == nil {
		t.Fatal("accepted conflicting replay")
	}
	fresh, e := NewDurable(dir)
	if e != nil {
		t.Fatal(e)
	}
	r, e := fresh.Get("one")
	if e != nil {
		t.Fatal(e)
	}
	if r.Status != "interrupted" || r.Sequence != 2 || !reflect.DeepEqual(r.Finalized, corrected.Finalized) || len(r.Provisional) != 0 {
		t.Fatalf("lost saved recording: %+v", r)
	}
	for i := range 30 {
		id := fmt.Sprint(i)
		if _, e = fresh.Open(OpenRequest{SessionID: "record-" + id, Source: "synthetic"}); e != nil {
			t.Fatal(e)
		}
	}
	page := fresh.List("")
	if len(page.Items) != 25 || page.Next == "" {
		t.Fatalf("first page: %+v", page)
	}
	older := fresh.List(page.Next)
	if len(older.Items) != 6 {
		t.Fatalf("older page: %+v", older)
	}
}
func TestCloseRequiresAcknowledgedSequence(t *testing.T) {
	s, _ := NewDurable(t.TempDir())
	if _, err := s.Open(OpenRequest{SessionID: "id", Source: "synthetic"}); err != nil {
		t.Fatal(err)
	}
	seq := int64(9)
	if _, e := s.State("id", "stopped", &seq, polytype.Nullable[int64]{}); e == nil {
		t.Fatal("closed unacknowledged text")
	}
}

func TestRequirementsFlagAndMarkPersist(t *testing.T) {
	dir := t.TempDir()
	s, err := NewDurable(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, req := range []OpenRequest{{SessionID: "ordinary", Source: "synthetic"}, {SessionID: "requirements", Source: "synthetic", Requirements: true}} {
		if _, err := s.Open(req); err != nil {
			t.Fatal(err)
		}
	}
	ordinary, _ := s.Get("ordinary")
	if ordinary.Requirements {
		t.Fatal("ordinary recording is marked")
	}
	if _, err := s.MarkRequirements("ordinary"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.MarkRequirements("missing"); err == nil {
		t.Fatal("marked missing recording")
	}
	fresh, err := NewDurable(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, h := range fresh.List("").Items {
		r, err := fresh.Get(h.ID)
		if err != nil || !h.Requirements || !r.Requirements {
			t.Fatalf("flag lost: header=%+v recording=%+v err=%v", h, r, err)
		}
	}
}

func TestCapturedDurationSurvivesPauseCloseRestartAndOlderUnknown(t *testing.T) {
	dir := t.TempDir()
	s, err := NewDurable(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"captured", "old"} {
		if _, err := s.Open(OpenRequest{SessionID: id, Source: "synthetic"}); err != nil {
			t.Fatal(err)
		}
	}
	paused, final, stale, invalid := int64(7200), int64(11000), int64(3000), int64(-1)
	if _, err := s.State("captured", "paused", nil, polytype.Nullable[int64]{Present: true, Value: paused}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.State("captured", "recording", nil, polytype.Nullable[int64]{Present: true, Value: stale}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.State("captured", "recording", nil, polytype.Nullable[int64]{Present: true, Value: invalid}); err == nil {
		t.Fatal("accepted negative duration")
	}
	r, _ := s.Get("captured")
	if !r.DurationMs.Present || r.DurationMs.Value != paused {
		t.Fatalf("stale heartbeat changed duration: %+v", r.Header)
	}
	seq := int64(0)
	if _, err := s.State("captured", "stopped", &seq, polytype.Nullable[int64]{Present: true, Value: final}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.State("captured", "stopped", &seq, polytype.Nullable[int64]{Present: true, Value: final}); err != nil {
		t.Fatal(err)
	}
	fresh, err := NewDurable(dir)
	if err != nil {
		t.Fatal(err)
	}
	r, err = fresh.Get("captured")
	if err != nil || !r.DurationMs.Present || r.DurationMs.Value != final || r.Status != "stopped" || len(r.Finalized) != 0 {
		t.Fatalf("silence duration lost: %+v %v", r, err)
	}
	old, err := fresh.Get("old")
	if err != nil || old.DurationMs.Present {
		t.Fatalf("invented old duration: %+v %v", old, err)
	}
	for _, h := range fresh.List("").Items {
		if h.ID == "captured" && (!h.DurationMs.Present || h.DurationMs.Value != final) {
			t.Fatalf("header duration lost: %+v", h)
		}
	}
}

func TestInterruptedRecordingIsNotSuccessfulClose(t *testing.T) {
	dir := t.TempDir()
	s, err := NewDurable(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Open(OpenRequest{SessionID: "session", Source: "synthetic"}); err != nil {
		t.Fatal(err)
	}
	fresh, err := NewDurable(dir) // restart interrupts the active session
	if err != nil {
		t.Fatal(err)
	}
	seq := int64(0)
	duration := polytype.Nullable[int64]{Present: true, Value: 3000}
	if _, err = fresh.State("session", "stopped", &seq, duration); err == nil {
		t.Fatal("interrupted session acknowledged as successful close")
	}
	r, _ := fresh.Get("session")
	if r.Status != "interrupted" {
		t.Fatalf("rewrote interrupted status: %+v", r.Header)
	}
}

func TestCloseRetryRepairsFailedMetadataWrite(t *testing.T) {
	dir := t.TempDir()
	s, err := NewDurable(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Open(OpenRequest{SessionID: "session", Source: "synthetic"}); err != nil {
		t.Fatal(err)
	}
	meta := filepath.Join(dir, "recordings", "session.meta.json")
	if err = os.Remove(meta); err != nil {
		t.Fatal(err)
	}
	if err = os.Mkdir(meta, 0755); err != nil {
		t.Fatal(err)
	}
	seq := int64(0)
	duration := polytype.Nullable[int64]{Present: true, Value: 4800}
	if _, err = s.State("session", "stopped", &seq, duration); err == nil {
		t.Fatal("close acknowledged despite failed metadata write")
	}
	if _, err = s.State("session", "stopped", &seq, duration); err == nil {
		t.Fatal("retry acknowledged while metadata remains unwritable")
	}
	if err = os.Remove(meta); err != nil {
		t.Fatal(err)
	}
	if _, err = s.State("session", "stopped", &seq, duration); err != nil {
		t.Fatal(err)
	}
	fresh, err := NewDurable(dir)
	if err != nil {
		t.Fatal(err)
	}
	r, err := fresh.Get("session")
	if err != nil || r.Status != "stopped" || !r.DurationMs.Present || r.DurationMs.Value != 4800 {
		t.Fatalf("successful retry lost saved duration: %+v %v", r, err)
	}
}
