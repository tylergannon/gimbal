package directory

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tylergannon/gimbal/runstore"
)

var ctx = context.Background()

func create(t *testing.T) (Store, runstore.Run, runstore.Header) {
	t.Helper()
	s := Store{Root: t.TempDir()}
	r, e := s.Create(ctx, runstore.Header{RunID: runstore.NewRunID(), Workflow: "test"})
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { _ = r.Close() })
	h, e := r.Header(ctx)
	if e != nil {
		t.Fatal(e)
	}
	return s, r, h
}
func fact(n runstore.NodeID, kind string) runstore.Fact {
	return runstore.Fact{Version: 1, ID: runstore.NewRecordID(), Node: n, Kind: kind, Payload: json.RawMessage(`{"ok":true}`)}
}
func TestOwnershipAndFreshProcessReadback(t *testing.T) {
	if os.Getenv("GIMBAL_STORE_CHILD") == "1" {
		s := Store{Root: os.Getenv("GIMBAL_STORE_ROOT")}
		r, e := s.Open(ctx, runstore.RunID(os.Getenv("GIMBAL_STORE_ID")))
		if os.Getenv("GIMBAL_STORE_BUSY") == "1" {
			if !errors.Is(e, runstore.ErrBusy) {
				t.Fatalf("want busy, got %v", e)
			}
			return
		}
		if e != nil {
			t.Fatal(e)
		}
		defer r.Close()
		records, e := r.Read(ctx, 0, 20)
		if e != nil || len(records) != 3 {
			t.Fatalf("records=%v error=%v", records, e)
		}
		n, e := r.EnsureNode(ctx, runstore.NodeSpec{Slot: runstore.Slot{Parent: records[0].Node, Kind: runstore.Action, Name: "check", Ordinal: 1}, DisplayKey: "test/check.1"})
		if e != nil || string(n.ID) != os.Getenv("GIMBAL_STORE_NODE") {
			t.Fatalf("node=%v error=%v", n, e)
		}
		ref := runstore.BlobRef{Key: os.Getenv("GIMBAL_STORE_BLOB"), Size: 6, SHA256: strings.TrimPrefix(os.Getenv("GIMBAL_STORE_BLOB"), "blobs/")}
		f, e := r.OpenBlob(ctx, ref)
		if e != nil {
			t.Fatal(e)
		}
		defer f.Close()
		raw, _ := io.ReadAll(f)
		if string(raw) != "output" {
			t.Fatal(string(raw))
		}
		return
	}
	s, r, h := create(t)
	n, e := r.EnsureNode(ctx, runstore.NodeSpec{Slot: runstore.Slot{Parent: h.Root, Kind: runstore.Action, Name: "check", Ordinal: 1}, DisplayKey: "test/check.1"})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.Create(ctx, h); !errors.Is(e, runstore.ErrExists) {
		t.Fatal(e)
	}
	if _, e = s.Open(ctx, h.RunID); !errors.Is(e, runstore.ErrBusy) {
		t.Fatal(e)
	}
	ref, e := r.PutBlob(ctx, strings.NewReader("output"))
	if e != nil {
		t.Fatal(e)
	}
	f := fact(n.ID, "outcome")
	f.Payload, _ = json.Marshal(ref)
	if _, e = r.Append(ctx, []runstore.Fact{f}, runstore.Committed); e != nil {
		t.Fatal(e)
	}
	child := func(busy bool) {
		t.Helper()
		cmd := exec.Command(os.Args[0], "-test.run=^TestOwnershipAndFreshProcessReadback$")
		cmd.Env = append(os.Environ(), "GIMBAL_STORE_CHILD=1", "GIMBAL_STORE_ROOT="+s.Root, "GIMBAL_STORE_ID="+string(h.RunID), "GIMBAL_STORE_NODE="+string(n.ID), "GIMBAL_STORE_BLOB="+ref.Key)
		if busy {
			cmd.Env = append(cmd.Env, "GIMBAL_STORE_BUSY=1")
		}
		if out, e := cmd.CombinedOutput(); e != nil {
			t.Fatalf("child: %v: %s", e, out)
		}
	}
	child(true)
	if e = r.Close(); e != nil {
		t.Fatal(e)
	}
	child(false)
}
func TestSlotsAndAtomicConflicts(t *testing.T) {
	_, r, h := create(t)
	spec := runstore.NodeSpec{Slot: runstore.Slot{Parent: h.Root, Kind: runstore.Scope, Name: "branch", Ordinal: 1}, DisplayKey: "branch.1"}
	ids := make(chan runstore.NodeID, 20)
	var wg sync.WaitGroup
	for range 20 {
		wg.Go(func() {
			n, e := r.EnsureNode(ctx, spec)
			if e != nil {
				t.Error(e)
			}
			ids <- n.ID
		})
	}
	wg.Wait()
	close(ids)
	var id runstore.NodeID
	for got := range ids {
		if id != "" && id != got {
			t.Fatal("unstable ID")
		}
		id = got
	}
	spec.DisplayKey = "changed"
	if _, e := r.EnsureNode(ctx, spec); !errors.Is(e, runstore.ErrConflict) {
		t.Fatal(e)
	}
	a := fact(id, "intent")
	b := fact(id, "intent")
	if _, e := r.Append(ctx, []runstore.Fact{a, b}, runstore.Committed); !errors.Is(e, runstore.ErrConflict) {
		t.Fatal(e)
	}
	rec, e := r.Append(ctx, []runstore.Fact{a, a}, runstore.Committed)
	if e != nil || rec[0].Sequence != rec[1].Sequence {
		t.Fatalf("%v %v", rec, e)
	}
	again, e := r.Append(ctx, []runstore.Fact{a}, runstore.Committed)
	if e != nil || again[0].Sequence != rec[0].Sequence {
		t.Fatal(e)
	}
	a.Payload = json.RawMessage(`null`)
	if _, e = r.Append(ctx, []runstore.Fact{a}, runstore.Committed); !errors.Is(e, runstore.ErrConflict) {
		t.Fatal(e)
	}
	got, e := r.Read(ctx, 0, 20)
	if e != nil || len(got) != 3 {
		t.Fatalf("partial conflicting batch: %d %v", len(got), e)
	}
}
func TestInterruptedTailAndCorruption(t *testing.T) {
	for _, corrupt := range []bool{false, true} {
		t.Run(map[bool]string{false: "incomplete", true: "checksum"}[corrupt], func(t *testing.T) {
			s, r, h := create(t)
			if e := r.Close(); e != nil {
				t.Fatal(e)
			}
			path := filepath.Join(s.Root, string(h.RunID), "state", "00000000.segment")
			f, e := os.OpenFile(path, os.O_RDWR, 0)
			if e != nil {
				t.Fatal(e)
			}
			st, _ := f.Stat()
			size := st.Size()
			if corrupt {
				_, e = f.WriteAt([]byte{0}, 8)
			} else {
				_, e = f.WriteAt([]byte{0, 0, 0}, size)
			}
			if e != nil {
				t.Fatal(e)
			}
			_ = f.Close()
			opened, e := s.Open(ctx, h.RunID)
			if corrupt {
				if !errors.Is(e, runstore.ErrCorrupt) {
					t.Fatal(e)
				}
				return
			}
			if e != nil {
				t.Fatal(e)
			}
			defer opened.Close()
			st, e = os.Stat(path)
			if e != nil || st.Size() != size {
				t.Fatalf("tail not truncated: %v %v", st, e)
			}
		})
	}
}
func TestIncompleteInitializationAndBlobIntegrity(t *testing.T) {
	s := Store{Root: t.TempDir()}
	id := runstore.NewRunID()
	dir := filepath.Join(s.Root, string(id), "state")
	if e := os.MkdirAll(dir, 0700); e != nil {
		t.Fatal(e)
	}
	if _, e := s.Open(ctx, id); !errors.Is(e, runstore.ErrIncomplete) {
		t.Fatal(e)
	}
	if _, e := os.Stat(filepath.Join(dir, "writer.lock")); !errors.Is(e, os.ErrNotExist) {
		t.Fatal("Open created lock")
	}
	s, r, h := create(t)
	payload := bytes.Repeat([]byte("a"), 2<<20)
	ref, e := r.PutBlob(ctx, bytes.NewReader(payload))
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(filepath.Join(s.Root, string(h.RunID), "state", ref.Key), []byte("bad"), 0600); e != nil {
		t.Fatal(e)
	}
	if _, e = r.OpenBlob(ctx, ref); !errors.Is(e, runstore.ErrCorrupt) {
		t.Fatal(e)
	}
}

func TestBufferedEventsDoNotWaitForSyncAndTerminalDoes(t *testing.T) {
	_, handle, h := create(t)
	r := handle.(*run)
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	r.mu.Lock()
	r.syncFile = func(f *os.File) error { once.Do(func() { close(entered) }); <-release; return f.Sync() }
	r.mu.Unlock()
	first := fact(h.Root, "event")
	if _, e := r.Append(ctx, []runstore.Fact{first}, runstore.Buffered); e != nil {
		t.Fatal(e)
	}
	flushed := make(chan error, 1)
	go func() { flushed <- r.Flush(ctx) }()
	<-entered
	appended := make(chan error, 1)
	go func() {
		_, e := r.Append(ctx, []runstore.Fact{fact(h.Root, "event")}, runstore.Buffered)
		appended <- e
	}()
	select {
	case e := <-appended:
		if e != nil {
			t.Fatal(e)
		}
	case <-time.After(time.Second):
		close(release)
		t.Fatal("buffered append blocked on sync")
	}
	terminal := make(chan error, 1)
	go func() {
		_, e := r.Append(ctx, []runstore.Fact{fact(h.Root, "outcome")}, runstore.Committed)
		terminal <- e
	}()
	select {
	case e := <-terminal:
		close(release)
		t.Fatalf("terminal acknowledged before sync: %v", e)
	case <-time.After(20 * time.Millisecond):
	}
	close(release)
	if e := <-flushed; e != nil {
		t.Fatal(e)
	}
	if e := <-terminal; e != nil {
		t.Fatal(e)
	}
}
func TestLostCommitAcknowledgmentReopensExactContents(t *testing.T) {
	s, handle, h := create(t)
	r := handle.(*run)
	injected := errors.New("sync acknowledgment lost")
	r.mu.Lock()
	r.syncFile = func(f *os.File) error {
		if e := f.Sync(); e != nil {
			return e
		}
		return injected
	}
	r.mu.Unlock()
	f := fact(h.Root, "outcome")
	if _, e := r.Append(ctx, []runstore.Fact{f}, runstore.Committed); !errors.Is(e, injected) {
		t.Fatal(e)
	}
	if _, e := r.Append(ctx, []runstore.Fact{fact(h.Root, "event")}, runstore.Buffered); !errors.Is(e, injected) {
		t.Fatal("poisoned handle accepted mutation", e)
	}
	_ = r.Close()
	opened, e := s.Open(ctx, h.RunID)
	if e != nil {
		t.Fatal(e)
	}
	defer opened.Close()
	rec, e := opened.Append(ctx, []runstore.Fact{f}, runstore.Committed)
	if e != nil || rec[0].Sequence != 2 {
		t.Fatalf("%v %v", rec, e)
	}
	all, e := opened.Read(ctx, 0, 20)
	if e != nil || len(all) != 2 {
		t.Fatalf("lost or duplicated contents: %v %v", all, e)
	}
}
