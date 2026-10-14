package transcript

import (
	"fmt"
	"reflect"
	"testing"
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
	if _, e = s.State("one", "recording", nil); e != nil {
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
	if _, e := s.State("id", "stopped", &seq); e == nil {
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
