// Package directory stores run records in framed append-only segments and
// immutable blobs on the workflow host. It has no dependency on the engine.
package directory

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/tylergannon/gimbal/runstore"
	"golang.org/x/sys/unix"
)

const maxBatch = 1 << 20
const segmentSize = 16 << 20
const pendingLimit = 4 << 20

// Store locates runs below Root (normally the project's runs directory).
type Store struct{ Root string }

type frame struct {
	Header  *runstore.Header  `json:"header,omitempty"`
	Records []runstore.Record `json:"records"`
}
type location struct {
	segment int
	offset  int64
	index   int
	digest  [32]byte
}
type segment struct {
	file *os.File
	size int64
}
type run struct {
	syncedSegments int
	syncFile       func(*os.File) error // package-level fault injection; nil uses File.Sync
	blobMu         sync.RWMutex
	mu             sync.Mutex
	flushMu        sync.Mutex
	header         runstore.Header
	dir            string
	lock           *os.File
	segments       []segment
	nodes          map[runstore.NodeID]runstore.Node
	slots          map[runstore.Slot]runstore.NodeID
	ids            map[runstore.RecordID]location
	unique         map[string]runstore.RecordID
	records        []runstore.RecordID
	committed      uint64
	pendingBytes   int
	poison         error
	closed         bool
	closing        bool
	stop           chan struct{}
	done           chan struct{}
	progress       chan struct{}
}

func stateDir(root string, id runstore.RunID) (string, error) {
	if !runstore.ValidID(string(id)) {
		return "", fmt.Errorf("runstore: invalid run ID %q", id)
	}
	return filepath.Join(root, string(id), "state"), nil
}
func syncDir(path string) error {
	f, e := os.Open(path)
	if e != nil {
		return e
	}
	return errors.Join(f.Sync(), f.Close())
}
func syncAncestors(path string) error {
	for {
		if err := syncDir(path); err != nil {
			return err
		}
		p := filepath.Dir(path)
		if p == path {
			return nil
		}
		path = p
	}
}
func (s Store) Create(ctx context.Context, h runstore.Header) (result runstore.Run, err error) {
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	dir, err := stateDir(s.Root, h.RunID)
	if err != nil {
		return nil, err
	}
	if h.Version == 0 {
		h.Version = 1
	}
	if h.Version != 1 || h.Workflow == "" {
		return nil, fmt.Errorf("runstore: invalid header")
	}
	if h.Root == "" {
		h.Root = runstore.NewNodeID()
	}
	if !runstore.ValidID(string(h.Root)) {
		return nil, fmt.Errorf("runstore: invalid root")
	}
	if h.CreatedAt.IsZero() {
		h.CreatedAt = time.Now().UTC()
	}
	if err = os.MkdirAll(s.Root, 0700); err != nil {
		return nil, err
	}
	if err = os.Mkdir(filepath.Dir(dir), 0700); errors.Is(err, os.ErrExist) {
		return nil, runstore.ErrExists
	} else if err != nil {
		return nil, err
	}
	if err = os.Mkdir(dir, 0700); err != nil {
		return nil, err
	}
	if err = os.Mkdir(filepath.Join(dir, "blobs"), 0700); err != nil {
		return nil, err
	}
	r, err := newRun(dir, true)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			_ = r.Close()
		}
	}()
	r.header = h
	root := runstore.Node{ID: h.Root, NodeSpec: runstore.NodeSpec{Slot: runstore.Slot{Kind: runstore.Scope, Name: h.Workflow}, DisplayKey: h.Workflow}}
	payload, _ := json.Marshal(root)
	fact := runstore.Fact{Version: 1, ID: runstore.NewRecordID(), Node: h.Root, Kind: "node-created", Payload: payload}
	if _, err = r.appendLocked([]runstore.Fact{fact}, &h, true); err != nil {
		return nil, err
	}
	if err = r.Flush(ctx); err != nil {
		return nil, err
	}
	if err = syncAncestors(dir); err != nil {
		return nil, err
	}
	marker := filepath.Join(dir, "ready.tmp")
	if err = os.WriteFile(marker, []byte("1\n"), 0600); err != nil {
		return nil, err
	}
	f, e := os.OpenFile(marker, os.O_RDWR, 0)
	if e != nil {
		return nil, e
	}
	err = errors.Join(f.Sync(), f.Close())
	if err != nil {
		return nil, err
	}
	if err = os.Rename(marker, filepath.Join(dir, "ready")); err != nil {
		return nil, err
	}
	if err = syncDir(dir); err != nil {
		return nil, err
	}
	r.startTimer()
	return r, nil
}
func newRun(dir string, create bool) (*run, error) {
	flags := os.O_RDWR
	if create {
		flags |= os.O_CREATE
	}
	lock, err := os.OpenFile(filepath.Join(dir, "writer.lock"), flags, 0600)
	if errors.Is(err, os.ErrNotExist) {
		return nil, runstore.ErrIncomplete
	}
	if err != nil {
		return nil, err
	}
	if err = unix.Flock(int(lock.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		_ = lock.Close()
		if errors.Is(err, unix.EWOULDBLOCK) {
			return nil, runstore.ErrBusy
		}
		return nil, err
	}
	return &run{dir: dir, lock: lock, nodes: map[runstore.NodeID]runstore.Node{}, slots: map[runstore.Slot]runstore.NodeID{}, ids: map[runstore.RecordID]location{}, unique: map[string]runstore.RecordID{}, stop: make(chan struct{}), progress: make(chan struct{})}, nil
}
func (s Store) Open(ctx context.Context, id runstore.RunID) (result runstore.Run, err error) {
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	dir, err := stateDir(s.Root, id)
	if err != nil {
		return nil, err
	}
	ready, err := os.ReadFile(filepath.Join(dir, "ready"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, runstore.ErrIncomplete
	}
	if err != nil {
		return nil, err
	}
	if string(ready) != "1\n" {
		return nil, runstore.ErrCorrupt
	}
	r, err := newRun(dir, false)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			_ = r.Close()
		}
	}()
	names, err := filepath.Glob(filepath.Join(dir, "*.segment"))
	if err != nil {
		return nil, err
	}
	sort.Strings(names)
	if len(names) == 0 {
		return nil, runstore.ErrCorrupt
	}
	for si, name := range names {
		if filepath.Base(name) != fmt.Sprintf("%08d.segment", si) {
			return nil, runstore.ErrCorrupt
		}
		f, e := os.OpenFile(name, os.O_RDWR, 0)
		if e != nil {
			return nil, e
		}
		r.segments = append(r.segments, segment{file: f})
		offset := int64(0)
		for {
			if err = ctx.Err(); err != nil {
				return nil, err
			}
			fr, size, e := readFrame(f, offset)
			if e == io.EOF {
				break
			}
			if errors.Is(e, io.ErrUnexpectedEOF) && si == len(names)-1 {
				if err = f.Truncate(offset); err != nil {
					return nil, err
				}
				break
			}
			if e != nil {
				return nil, fmt.Errorf("%w: segment %d offset %d: %v", runstore.ErrCorrupt, si, offset, e)
			}
			if offset == 0 && si == 0 {
				if fr.Header == nil || fr.Header.Version != 1 || fr.Header.RunID != id || !runstore.ValidID(string(fr.Header.Root)) || fr.Header.Workflow == "" {
					return nil, runstore.ErrCorrupt
				}
				r.header = *fr.Header
			} else if fr.Header != nil {
				return nil, runstore.ErrCorrupt
			}
			if len(fr.Records) == 0 {
				return nil, runstore.ErrCorrupt
			}
			for i, rec := range fr.Records {
				if rec.Sequence != uint64(len(r.records)+1) {
					return nil, runstore.ErrCorrupt
				}
				if _, exists := r.ids[rec.ID]; exists {
					return nil, runstore.ErrCorrupt
				}
				if e = r.validate(rec.Fact, true); e != nil {
					return nil, fmt.Errorf("%w: %v", runstore.ErrCorrupt, e)
				}
				raw, _ := json.Marshal(rec.Fact)
				r.install(rec, location{segment: si, offset: offset, index: i, digest: sha256.Sum256(raw)})
			}
			offset += size
		}
		r.segments[si].size = offset
	}
	if root, ok := r.nodes[r.header.Root]; !ok || root.Parent != "" || root.Kind != runstore.Scope {
		return nil, runstore.ErrCorrupt
	}
	// The crash model is process death with an intact page cache: valid complete
	// frames found on reopen are synced before they become committed readback.
	if err = r.Flush(ctx); err != nil {
		return nil, err
	}
	r.startTimer()
	return r, nil
}
func readFrame(f *os.File, off int64) (frame, int64, error) {
	var fr frame
	var head [8]byte
	n, err := f.ReadAt(head[:], off)
	if err != nil {
		if n == 0 && err == io.EOF {
			return fr, 0, io.EOF
		}
		return fr, 0, io.ErrUnexpectedEOF
	}
	size := binary.BigEndian.Uint32(head[:4])
	if size == 0 || size > maxBatch {
		return fr, 0, fmt.Errorf("invalid frame length")
	}
	data := make([]byte, int(size))
	if _, err = f.ReadAt(data, off+8); err != nil {
		return fr, 0, io.ErrUnexpectedEOF
	}
	if crc32.ChecksumIEEE(data) != binary.BigEndian.Uint32(head[4:]) {
		return fr, 0, fmt.Errorf("checksum mismatch")
	}
	if err = json.Unmarshal(data, &fr); err != nil {
		return fr, 0, err
	}
	return fr, int64(size) + 8, nil
}
func (r *run) startTimer() {
	r.done = make(chan struct{})
	done := r.done
	go func() {
		defer close(done)
		t := time.NewTicker(250 * time.Millisecond)
		defer t.Stop()
		for {
			select {
			case <-r.stop:
				return
			case <-t.C:
				_ = r.Flush(context.Background())
			}
		}
	}()
}
func (r *run) signal() { close(r.progress); r.progress = make(chan struct{}) }
func (r *run) check(ctx context.Context) error {
	if e := ctx.Err(); e != nil {
		return e
	}
	if r.closed || r.closing {
		return runstore.ErrClosed
	}
	return r.poison
}
func (r *run) Header(ctx context.Context) (runstore.Header, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.header, r.check(ctx)
}
func cloneNode(n runstore.Node) runstore.Node { n.Metadata = bytes.Clone(n.Metadata); return n }
func (r *run) Node(ctx context.Context, s runstore.Slot) (runstore.Node, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if e := r.check(ctx); e != nil {
		return runstore.Node{}, e
	}
	id, ok := r.slots[s]
	if !ok {
		return runstore.Node{}, runstore.ErrNotFound
	}
	return cloneNode(r.nodes[id]), nil
}
func (r *run) EnsureNode(ctx context.Context, s runstore.NodeSpec) (runstore.Node, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if e := r.check(ctx); e != nil {
		return runstore.Node{}, e
	}
	if id, ok := r.slots[s.Slot]; ok {
		n := r.nodes[id]
		oldSpec, _ := json.Marshal(n.NodeSpec)
		newSpec, err := json.Marshal(s)
		if err != nil {
			return runstore.Node{}, err
		}
		if !bytes.Equal(oldSpec, newSpec) {
			return runstore.Node{}, runstore.ErrConflict
		}
		return cloneNode(n), nil
	}
	n := runstore.Node{ID: runstore.NewNodeID(), NodeSpec: s}
	raw, e := json.Marshal(n)
	if e != nil {
		return runstore.Node{}, e
	}
	if _, e = r.appendLocked([]runstore.Fact{{Version: 1, ID: runstore.NewRecordID(), Node: n.ID, Kind: "node-created", Payload: raw}}, nil, true); e != nil {
		return runstore.Node{}, e
	}
	return cloneNode(n), nil
}
func uniqueKey(f runstore.Fact) string {
	switch f.Kind {
	case "intent", "binding", "native-binding", "outcome":
		return string(f.Node) + "/" + f.Kind
	}
	return ""
}
func (r *run) validate(f runstore.Fact, internal bool) error {
	if f.Version != 1 || !runstore.ValidID(string(f.ID)) || !runstore.ValidID(string(f.Node)) || f.Kind == "" || !json.Valid(f.Payload) {
		return fmt.Errorf("runstore: invalid fact")
	}
	if key := uniqueKey(f); key != "" {
		if _, ok := r.unique[key]; ok {
			return runstore.ErrConflict
		}
	}
	if f.Kind == "node-created" {
		if !internal {
			return fmt.Errorf("runstore: use EnsureNode to create a node")
		}
		var n runstore.Node
		if json.Unmarshal(f.Payload, &n) != nil || n.ID != f.Node || n.Ordinal < 0 || n.Name == "" || (len(n.Metadata) > 0 && !json.Valid(n.Metadata)) {
			return fmt.Errorf("runstore: invalid node")
		}
		switch n.Kind {
		case runstore.Scope, runstore.Session, runstore.Action, runstore.Service, runstore.Question:
		default:
			return fmt.Errorf("runstore: invalid node kind")
		}
		if _, ok := r.nodes[n.ID]; ok {
			return runstore.ErrConflict
		}
		if _, ok := r.slots[n.Slot]; ok {
			return runstore.ErrConflict
		}
		if n.Parent == "" {
			if n.ID != r.header.Root || len(r.nodes) != 0 || n.Kind != runstore.Scope {
				return fmt.Errorf("runstore: invalid root")
			}
		} else if parent, ok := r.nodes[n.Parent]; !ok {
			return fmt.Errorf("runstore: missing parent")
		} else if parent.Kind != runstore.Scope && (n.Kind != runstore.Action || (parent.Kind != runstore.Session && parent.Kind != runstore.Action)) {
			return fmt.Errorf("runstore: invalid parent kind")
		}
	} else if _, ok := r.nodes[f.Node]; !ok {
		return fmt.Errorf("runstore: missing node")
	}
	for _, ref := range []struct {
		id   runstore.NodeID
		kind runstore.Kind
	}{{f.Scope, runstore.Scope}, {f.Session, runstore.Session}} {
		if ref.id != "" {
			n, ok := r.nodes[ref.id]
			if !ok || n.Kind != ref.kind {
				return fmt.Errorf("runstore: invalid %s reference", ref.kind)
			}
		}
	}
	return nil
}
func (r *run) install(rec runstore.Record, loc location) {
	r.ids[rec.ID] = loc
	r.records = append(r.records, rec.ID)
	if k := uniqueKey(rec.Fact); k != "" {
		r.unique[k] = rec.ID
	}
	if rec.Kind == "node-created" {
		var n runstore.Node
		_ = json.Unmarshal(rec.Payload, &n)
		r.nodes[n.ID] = n
		r.slots[n.Slot] = n.ID
	}
}
func (r *run) appendLocked(facts []runstore.Fact, h *runstore.Header, internal bool) ([]runstore.Record, error) {
	out := make([]runstore.Record, 0, len(facts))
	fresh := make([]runstore.Record, 0, len(facts))
	seen := map[runstore.RecordID][32]byte{}
	keys := map[string]bool{}
	// Validate the entire batch before any bytes become visible.
	for _, f := range facts {
		raw, e := json.Marshal(f)
		if e != nil {
			return nil, e
		}
		digest := sha256.Sum256(raw)
		if loc, ok := r.ids[f.ID]; ok {
			if loc.digest != digest {
				return nil, runstore.ErrConflict
			}
			rec, e := r.recordAt(loc)
			if e != nil {
				return nil, e
			}
			out = append(out, rec)
			continue
		}
		if d, ok := seen[f.ID]; ok {
			if d != digest {
				return nil, runstore.ErrConflict
			}
			for _, rec := range fresh {
				if rec.ID == f.ID {
					out = append(out, rec)
					break
				}
			}
			continue
		}
		if e = r.validate(f, internal); e != nil {
			return nil, e
		}
		if k := uniqueKey(f); k != "" {
			if keys[k] {
				return nil, runstore.ErrConflict
			}
			keys[k] = true
		}
		seen[f.ID] = digest
		rec := runstore.Record{Sequence: uint64(len(r.records) + len(fresh) + 1), Fact: f}
		fresh = append(fresh, rec)
		out = append(out, rec)
	}
	if len(fresh) == 0 {
		return out, nil
	}
	data, e := json.Marshal(frame{Header: h, Records: fresh})
	if e != nil {
		return nil, e
	}
	if len(data) > maxBatch {
		return nil, fmt.Errorf("runstore: batch exceeds %d bytes; use blobs", maxBatch)
	}
	if len(r.segments) == 0 || r.segments[len(r.segments)-1].size+int64(len(data)+8) > segmentSize {
		name := filepath.Join(r.dir, fmt.Sprintf("%08d.segment", len(r.segments)))
		f, err := os.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			r.poison = err
			r.signal()
			return nil, err
		}
		r.segments = append(r.segments, segment{file: f})

	}
	si := len(r.segments) - 1
	seg := &r.segments[si]
	off := seg.size
	buf := make([]byte, 8+len(data))
	binary.BigEndian.PutUint32(buf[:4], uint32(len(data)))
	binary.BigEndian.PutUint32(buf[4:8], crc32.ChecksumIEEE(data))
	copy(buf[8:], data)
	if n, err := seg.file.WriteAt(buf, off); err != nil || n != len(buf) {
		if err == nil {
			err = io.ErrShortWrite
		}
		r.poison = err
		r.signal()
		return nil, err
	}
	seg.size += int64(len(buf))
	r.pendingBytes += len(buf)
	for i, rec := range fresh {
		raw, _ := json.Marshal(rec.Fact)
		r.install(rec, location{segment: si, offset: off, index: i, digest: sha256.Sum256(raw)})
	}
	return out, nil
}
func (r *run) Append(ctx context.Context, f []runstore.Fact, d runstore.Durability) ([]runstore.Record, error) {
	if d != runstore.Buffered && d != runstore.Committed {
		return nil, fmt.Errorf("runstore: invalid durability")
	}
	r.mu.Lock()
	for r.pendingBytes >= pendingLimit && r.poison == nil && !r.closed && !r.closing {
		ch := r.progress
		r.mu.Unlock()
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ch:
		}
		r.mu.Lock()
	}
	if e := r.check(ctx); e != nil {
		r.mu.Unlock()
		return nil, e
	}
	rec, e := r.appendLocked(f, nil, false)
	r.mu.Unlock()
	if e != nil {
		return nil, e
	}
	if d == runstore.Committed {
		if e = r.Flush(ctx); e != nil {
			return nil, e
		}
	}
	return rec, nil
}
func (r *run) Flush(ctx context.Context) error {
	return r.flush(ctx, false)
}
func (r *run) flush(ctx context.Context, closing bool) error {
	r.flushMu.Lock()
	defer r.flushMu.Unlock()
	r.mu.Lock()
	if e := r.check(ctx); e != nil && !(closing && r.closing && !r.closed && ctx.Err() == nil && r.poison == nil) {
		r.mu.Unlock()
		return e
	}
	seq := uint64(len(r.records))
	if seq == r.committed {
		r.mu.Unlock()
		return nil
	}
	files := make([]*os.File, len(r.segments))
	for i, s := range r.segments {
		files[i] = s.file
	}
	needsDirSync := len(files) > r.syncedSegments
	bytesAtStart := r.pendingBytes
	syncFile := r.syncFile
	if syncFile == nil {
		syncFile = (*os.File).Sync
	}
	r.mu.Unlock()
	var err error
	for _, f := range files {
		if e := syncFile(f); e != nil {
			err = errors.Join(err, e)
		}
	}
	if err == nil && needsDirSync {
		err = syncDir(r.dir)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if err != nil {
		r.poison = err
		r.signal()
	} else {
		r.committed = seq
		r.syncedSegments = len(files)
		r.pendingBytes -= bytesAtStart
	}
	r.signal()
	if err != nil {
		return err
	}
	return ctx.Err()
}
func (r *run) recordAt(loc location) (runstore.Record, error) {
	fr, _, e := readFrame(r.segments[loc.segment].file, loc.offset)
	if e != nil {
		return runstore.Record{}, e
	}
	if loc.index >= len(fr.Records) {
		return runstore.Record{}, runstore.ErrCorrupt
	}
	return fr.Records[loc.index], nil
}
func (r *run) Read(ctx context.Context, after uint64, limit int) ([]runstore.Record, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if e := r.check(ctx); e != nil {
		return nil, e
	}
	if limit <= 0 || limit > 4096 {
		return nil, fmt.Errorf("runstore: read limit must be 1..4096")
	}
	out := make([]runstore.Record, 0, min(uint64(limit), r.committed))
	for seq := after; seq < r.committed && len(out) < limit; seq++ {
		rec, e := r.recordAt(r.ids[r.records[seq]])
		if e != nil {
			return nil, e
		}
		out = append(out, rec)
	}
	return out, nil
}
func (r *run) Close() error {
	r.blobMu.Lock()
	defer r.blobMu.Unlock()
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return nil
	}
	r.closing = true
	r.signal()
	done := r.done
	if done != nil {
		close(r.stop)
		r.done = nil
	}
	r.mu.Unlock()
	if done != nil {
		<-done
	}
	err := r.flush(context.Background(), true)
	r.flushMu.Lock()
	defer r.flushMu.Unlock()
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return err
	}
	r.closed = true
	r.signal()
	for _, s := range r.segments {
		err = errors.Join(err, s.file.Close())
	}
	err = errors.Join(err, r.lock.Close())
	return err
}

// Blob writes use constant-sized buffers and become durable before a result can
// reference them. Their identities contain no host filesystem paths.
func (r *run) PutBlob(ctx context.Context, src io.Reader) (runstore.BlobRef, error) {
	r.blobMu.RLock()
	defer r.blobMu.RUnlock()
	r.mu.Lock()
	err := r.check(ctx)
	r.mu.Unlock()
	if err != nil {
		return runstore.BlobRef{}, err
	}
	f, e := os.CreateTemp(filepath.Join(r.dir, "blobs"), ".pending-")
	if e != nil {
		return runstore.BlobRef{}, e
	}
	defer func() { _ = os.Remove(f.Name()) }()
	hash := sha256.New()
	size, e := io.Copy(io.MultiWriter(f, hash), contextReader{ctx, src})
	if e != nil {
		_ = f.Close()
		return runstore.BlobRef{}, e
	}
	if e = errors.Join(f.Sync(), f.Close()); e != nil {
		return runstore.BlobRef{}, e
	}
	digest := hex.EncodeToString(hash.Sum(nil))
	key := "blobs/" + digest
	if e = os.Rename(f.Name(), filepath.Join(r.dir, key)); e != nil {
		return runstore.BlobRef{}, e
	}
	if e = syncDir(filepath.Join(r.dir, "blobs")); e != nil {
		return runstore.BlobRef{}, e
	}
	return runstore.BlobRef{Key: key, Size: size, SHA256: digest}, nil
}

type contextReader struct {
	ctx context.Context
	r   io.Reader
}

func (r contextReader) Read(p []byte) (int, error) {
	if e := r.ctx.Err(); e != nil {
		return 0, e
	}
	return r.r.Read(p)
}
func (r *run) OpenBlob(ctx context.Context, ref runstore.BlobRef) (io.ReadCloser, error) {
	r.blobMu.RLock()
	defer r.blobMu.RUnlock()
	r.mu.Lock()
	err := r.check(ctx)
	r.mu.Unlock()
	if err != nil {
		return nil, err
	}
	if len(ref.SHA256) != 64 || ref.Key != "blobs/"+ref.SHA256 || ref.Size < 0 {
		return nil, runstore.ErrCorrupt
	}
	if _, e := hex.DecodeString(ref.SHA256); e != nil {
		return nil, runstore.ErrCorrupt
	}
	f, e := os.Open(filepath.Join(r.dir, ref.Key))
	if e != nil {
		return nil, e
	}
	h := sha256.New()
	size, e := io.Copy(h, contextReader{ctx, f})
	if e != nil || size != ref.Size || hex.EncodeToString(h.Sum(nil)) != ref.SHA256 {
		_ = f.Close()
		return nil, errors.Join(runstore.ErrCorrupt, e)
	}
	if _, e = f.Seek(0, io.SeekStart); e != nil {
		_ = f.Close()
		return nil, e
	}
	return f, nil
}

// Recognized reports a newly formatted run, including incomplete initialization.
// It is format detection only and never opens a writer.
func Recognized(runDir string) bool { _, e := os.Stat(filepath.Join(runDir, "state")); return e == nil }

var _ runstore.Store = Store{}
var _ runstore.Run = (*run)(nil)
