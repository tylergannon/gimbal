package gimbal

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/tylergannon/gimbal/runstore"
)

// memoryStore is a test double for replacing the authoritative backend. The
// engine must still materialize its ordinary consumer/UI files independently.
type memoryStore struct {
	mu   sync.Mutex
	runs map[runstore.RunID]*memoryState
}
type memoryState struct {
	header runstore.Header
	writer bool
	nodes  map[runstore.Slot]runstore.Node
	facts  []runstore.Record
	blobs  map[string][]byte
}
type memoryRun struct {
	store  *memoryStore
	state  *memoryState
	closed bool
}

func (s *memoryStore) Create(ctx context.Context, h runstore.Header) (runstore.Run, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	if s.runs == nil {
		s.runs = map[runstore.RunID]*memoryState{}
	}
	if s.runs[h.RunID] != nil {
		return nil, runstore.ErrExists
	}
	h.Version = 1
	h.Root = runstore.NewNodeID()
	h.CreatedAt = time.Now().UTC()
	state := &memoryState{header: h, writer: true, nodes: map[runstore.Slot]runstore.Node{}, blobs: map[string][]byte{}}
	node := runstore.Node{ID: h.Root, NodeSpec: runstore.NodeSpec{Slot: runstore.Slot{Kind: runstore.Scope, Name: h.Workflow}, DisplayKey: h.Workflow}}
	state.nodes[node.Slot] = node
	raw, _ := json.Marshal(node)
	state.facts = []runstore.Record{{Sequence: 1, Fact: runstore.Fact{Version: 1, ID: runstore.NewRecordID(), Node: h.Root, Kind: "node-created", Payload: raw}}}
	s.runs[h.RunID] = state
	return &memoryRun{s, state, false}, nil
}
func (s *memoryStore) Open(ctx context.Context, id runstore.RunID) (runstore.Run, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	state := s.runs[id]
	if state == nil {
		return nil, runstore.ErrNotFound
	}
	if state.writer {
		return nil, runstore.ErrBusy
	}
	state.writer = true
	return &memoryRun{s, state, false}, nil
}
func (r *memoryRun) check(ctx context.Context) error {
	if e := ctx.Err(); e != nil {
		return e
	}
	if r.closed {
		return runstore.ErrClosed
	}
	return nil
}
func (r *memoryRun) Header(ctx context.Context) (runstore.Header, error) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	return r.state.header, r.check(ctx)
}
func (r *memoryRun) EnsureNode(ctx context.Context, spec runstore.NodeSpec) (runstore.Node, error) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	if e := r.check(ctx); e != nil {
		return runstore.Node{}, e
	}
	if n, ok := r.state.nodes[spec.Slot]; ok {
		a, _ := json.Marshal(n.NodeSpec)
		b, _ := json.Marshal(spec)
		if !bytes.Equal(a, b) {
			return runstore.Node{}, runstore.ErrConflict
		}
		return n, nil
	}
	n := runstore.Node{ID: runstore.NewNodeID(), NodeSpec: spec}
	raw, e := json.Marshal(n)
	if e != nil {
		return runstore.Node{}, e
	}
	r.state.nodes[spec.Slot] = n
	r.state.facts = append(r.state.facts, runstore.Record{Sequence: uint64(len(r.state.facts) + 1), Fact: runstore.Fact{Version: 1, ID: runstore.NewRecordID(), Node: n.ID, Kind: "node-created", Payload: raw}})
	return n, nil
}
func (r *memoryRun) Node(ctx context.Context, slot runstore.Slot) (runstore.Node, error) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	if e := r.check(ctx); e != nil {
		return runstore.Node{}, e
	}
	n, ok := r.state.nodes[slot]
	if !ok {
		return n, runstore.ErrNotFound
	}
	return n, nil
}
func (r *memoryRun) Append(ctx context.Context, facts []runstore.Fact, _ runstore.Durability) ([]runstore.Record, error) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	if e := r.check(ctx); e != nil {
		return nil, e
	}
	pending := append([]runstore.Record(nil), r.state.facts...)
	result := make([]runstore.Record, 0, len(facts))
	for _, f := range facts {
		var prior *runstore.Record
		for i := range pending {
			if pending[i].ID == f.ID {
				prior = &pending[i]
				break
			}
		}
		if prior != nil {
			a, _ := json.Marshal(prior.Fact)
			b, _ := json.Marshal(f)
			if !bytes.Equal(a, b) {
				return nil, runstore.ErrConflict
			}
			result = append(result, *prior)
			continue
		}
		f.Payload = bytes.Clone(f.Payload)
		rec := runstore.Record{Sequence: uint64(len(pending) + 1), Fact: f}
		pending = append(pending, rec)
		result = append(result, rec)
	}
	r.state.facts = pending
	return result, nil
}
func (r *memoryRun) Flush(ctx context.Context) error {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	return r.check(ctx)
}
func (r *memoryRun) Read(ctx context.Context, after uint64, limit int) ([]runstore.Record, error) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	if e := r.check(ctx); e != nil {
		return nil, e
	}
	if after >= uint64(len(r.state.facts)) {
		return nil, nil
	}
	result := append([]runstore.Record(nil), r.state.facts[after:min(after+uint64(limit), uint64(len(r.state.facts)))]...)
	for i := range result {
		result[i].Payload = bytes.Clone(result[i].Payload)
	}
	return result, nil
}
func (r *memoryRun) PutBlob(ctx context.Context, src io.Reader) (runstore.BlobRef, error) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	if e := r.check(ctx); e != nil {
		return runstore.BlobRef{}, e
	}
	raw, e := io.ReadAll(src)
	if e != nil {
		return runstore.BlobRef{}, e
	}
	digest := sha256.Sum256(raw)
	hash := hex.EncodeToString(digest[:])
	ref := runstore.BlobRef{Key: hash, SHA256: hash, Size: int64(len(raw))}
	r.state.blobs[ref.Key] = raw
	return ref, nil
}
func (r *memoryRun) OpenBlob(ctx context.Context, ref runstore.BlobRef) (io.ReadCloser, error) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	if e := r.check(ctx); e != nil {
		return nil, e
	}
	raw, ok := r.state.blobs[ref.Key]
	if !ok {
		return nil, runstore.ErrNotFound
	}
	return io.NopCloser(bytes.NewReader(bytes.Clone(raw))), nil
}
func (r *memoryRun) Close() error {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	if r.closed {
		return nil
	}
	r.closed = true
	r.state.writer = false
	return nil
}

func TestEngineUsesInjectedStoreThroughRetainedCleanup(t *testing.T) {
	project := t.TempDir()
	factory := &memoryStore{}
	id := runstore.NewRunID()
	var owner *run
	var closes int
	adapter := &fake{answer: func(context.Context, string, string, json.RawMessage, func(AgentEvent) error) (string, error) {
		return "ok", nil
	}, closeErr: func(string) error {
		closes++
		if closes == 1 {
			return errors.New("retry close")
		}
		return nil
	}}
	ctx := WithRunStore(Project(t.Context(), project), factory)
	err := Run(ctx, string(id), "substitution", bind(adapter, "test", "agent"), func(ctx context.Context) error {
		s, _ := current(ctx)
		owner = s.run
		_, _, _, e := RunCommand(ctx, "command", project, "printf", "owned output")
		if e != nil {
			return e
		}
		_, e = NewSession(ctx, "agent", project).Generate[Text](ctx, "reply")
		return e
	})
	if err == nil || owner == nil {
		t.Fatalf("%v", err)
	}
	if _, e := factory.Open(t.Context(), id); !errors.Is(e, runstore.ErrBusy) {
		t.Fatalf("lost retained writer: %v", e)
	}
	if e := owner.retryCleanup(); e != nil {
		t.Fatal(e)
	}
	if _, e := os.Stat(filepath.Join(project, "runs", string(id), "state")); !errors.Is(e, os.ErrNotExist) {
		t.Fatal("engine bypassed injected store", e)
	}
	r, e := factory.Open(t.Context(), id)
	if e != nil {
		t.Fatal(e)
	}
	defer func() { _ = r.Close() }()
	records, e := r.Read(t.Context(), 0, 512)
	if e != nil {
		t.Fatal(e)
	}
	counts := map[string]int{}
	for _, rec := range records {
		counts[rec.Kind]++
	}
	for _, kind := range []string{"node-created", "intent", "binding", "outcome", "lifecycle", "agent", "cleanup-state"} {
		if counts[kind] == 0 {
			t.Fatal("seam bypass or missing records", kind, counts)
		}
	}
	if len(factory.runs[id].blobs) == 0 {
		t.Fatal("outputs bypassed authoritative blob store")
	}
}
