package transcript

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/tylergannon/polytype"
)

// Recording retains audio ranges and both accepted final and provisional text.
// Only the server creates and writes this store.
type Recording struct {
	Header
	Finalized   []Chunk `json:"finalized"`
	Provisional []Chunk `json:"provisional"`
	Last        string  `json:"last"`
}
type Header struct {
	DurationMs   polytype.Nullable[int64] `json:"durationMs"`
	ID           string                   `json:"id"`
	Source       string                   `json:"source"`
	Created      int64                    `json:"created"`
	Updated      int64                    `json:"updated"`
	Sequence     int64                    `json:"sequence"`
	Status       string                   `json:"status"`
	Requirements bool                     `json:"requirements"`
}
type History struct {
	Items []Header `json:"items"`
	Next  string   `json:"next"`
}
type DurableStore struct {
	mu      sync.Mutex
	dir     string
	headers map[string]Header
}

var safeID = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)

func NewDurable(dir string) (*DurableStore, error) {
	s := &DurableStore{dir: filepath.Join(dir, "recordings"), headers: map[string]Header{}}
	if err := os.MkdirAll(s.dir, 0755); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".meta.json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.dir, entry.Name()))
		if err != nil {
			return nil, err
		}
		var h Header
		if err = json.Unmarshal(data, &h); err != nil {
			return nil, err
		}
		if !safeID.MatchString(h.ID) {
			return nil, errors.New("invalid saved recording ID")
		}
		s.headers[h.ID] = h
		if h.Status != "stopped" && h.Status != "interrupted" {
			r, err := s.read(h.ID)
			if err != nil {
				return nil, err
			}
			r.Status = "interrupted"
			if err = s.save(r); err != nil {
				return nil, err
			}
		}
	}
	return s, nil
}
func (s *DurableStore) read(id string) (Recording, error) {
	if !safeID.MatchString(id) {
		return Recording{}, errors.New("invalid recording ID")
	}
	data, err := os.ReadFile(filepath.Join(s.dir, id+".json"))
	if err != nil {
		return Recording{}, err
	}
	var r Recording
	err = json.Unmarshal(data, &r)
	return r, err
}

// WriteJSON acknowledges only after file and parent directory have been synced.
func WriteJSON(path string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".pending-")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(f.Name()) }()
	if _, err = f.Write(data); err != nil {
		_ = f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		_ = f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	if err = os.Rename(f.Name(), path); err != nil {
		return err
	}
	d, err := os.Open(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer func() { _ = d.Close() }()
	return d.Sync()
}
func (s *DurableStore) save(r Recording) error {
	if err := WriteJSON(filepath.Join(s.dir, r.ID+".json"), r); err != nil {
		return err
	}
	if err := WriteJSON(filepath.Join(s.dir, r.ID+".meta.json"), r.Header); err != nil {
		return err
	}
	s.headers[r.ID] = r.Header
	return nil
}
func ackRecording(r Recording) Ack {
	return Ack{r.ID, r.Sequence, r.Status == "stopped" || r.Status == "interrupted"}
}
func (s *DurableStore) Open(r OpenRequest) (Ack, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !safeID.MatchString(r.SessionID) || (r.Source != "microphone" && r.Source != "synthetic") {
		return Ack{}, errors.New("invalid recording")
	}
	if _, ok := s.headers[r.SessionID]; ok {
		v, err := s.read(r.SessionID)
		if err != nil {
			return Ack{}, err
		}
		if v.Source != r.Source {
			return Ack{}, errors.New("conflicting recording")
		}
		return ackRecording(v), nil
	}
	now := time.Now().UnixMilli()
	v := Recording{Header: Header{ID: r.SessionID, Source: r.Source, Created: now, Updated: now, Status: "preparing", Requirements: r.Requirements}, Finalized: []Chunk{}, Provisional: []Chunk{}}
	return ackRecording(v), s.save(v)
}
func (s *DurableStore) Accept(u Update) (Ack, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, err := s.read(u.SessionID)
	if err != nil {
		return Ack{}, err
	}
	payload, _ := json.Marshal(u)
	if len(payload) > MaxPayloadBytes {
		return Ack{}, errors.New("transcript update too large")
	}
	if u.Sequence == r.Sequence && u.Sequence > 0 {
		if string(payload) != r.Last {
			return Ack{}, errors.New("conflicting duplicate")
		}
		return ackRecording(r), s.save(r)
	}
	if ackRecording(r).Closed {
		return Ack{}, errors.New("recording ended")
	}
	if u.Sequence != r.Sequence+1 {
		return Ack{}, errors.New("out of order sequence")
	}
	var boundary int64
	if len(r.Finalized) > 0 {
		boundary = r.Finalized[len(r.Finalized)-1].EndMs
	}
	boundary, _, err = validate(u.Finalized, boundary)
	if err != nil {
		return Ack{}, err
	}
	if _, _, err = validate(u.Provisional, boundary); err != nil {
		return Ack{}, err
	}
	r.Finalized = append(r.Finalized, u.Finalized...)
	r.Provisional = u.Provisional
	if r.Provisional == nil {
		r.Provisional = []Chunk{}
	}
	r.Sequence = u.Sequence
	r.Last = string(payload)
	r.Updated = time.Now().UnixMilli()
	return ackRecording(r), s.save(r)
}
func (s *DurableStore) State(id, status string, sequence *int64, durationMs polytype.Nullable[int64]) (Ack, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, err := s.read(id)
	if err != nil {
		return Ack{}, err
	}
	if sequence != nil && *sequence != r.Sequence {
		return Ack{}, errors.New("close sequence does not match durable transcript")
	}
	if ackRecording(r).Closed {
		if status == "stopped" && r.Status != "stopped" {
			return Ack{}, errors.New("recording was interrupted; it cannot be confirmed as successfully saved")
		}
		if status == "stopped" || status == "interrupted" {
			// A prior close may have written the recording but failed its metadata.
			// A terminal retry acknowledges both durable files and repairs history.
			return ackRecording(r), s.save(r)
		}
		return ackRecording(r), nil
	}
	switch status {
	case "recording", "paused", "stopped", "interrupted":
	default:
		return Ack{}, fmt.Errorf("invalid recording state %s", status)
	}
	if durationMs.Present {
		if durationMs.Value < 0 || durationMs.Value > MaxInteger {
			return Ack{}, errors.New("invalid recording duration")
		}
		if !r.DurationMs.Present || durationMs.Value > r.DurationMs.Value {
			r.DurationMs = durationMs
		}
	}
	r.Status = status
	r.Updated = time.Now().UnixMilli()
	return ackRecording(r), s.save(r)
}
func visible(h Header) Header {
	if h.Status != "stopped" && h.Status != "interrupted" && time.Now().UnixMilli()-h.Updated > 20000 {
		h.Status = "interrupted"
	}
	return h
}
func (s *DurableStore) Get(id string) (Recording, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, err := s.read(id)
	r.Header = visible(r.Header)
	return r, err
}
func (s *DurableStore) List(after string) History {
	s.mu.Lock()
	defer s.mu.Unlock()
	items := make([]Header, 0, len(s.headers))
	for _, h := range s.headers {
		items = append(items, visible(h))
	}
	slices.SortFunc(items, func(a, b Header) int {
		if a.Created > b.Created {
			return -1
		}
		if a.Created < b.Created {
			return 1
		}
		return strings.Compare(b.ID, a.ID)
	})
	start := 0
	if after != "" {
		start = len(items)
		for i, h := range items {
			if h.ID == after {
				start = i + 1
				break
			}
		}
	}
	end := min(start+25, len(items))
	out := History{Items: items[start:end]}
	if end < len(items) {
		out.Next = items[end-1].ID
	}
	return out
}

// MarkRequirements flags a recording as a requirements discussion. Write the
// metadata first because List and the promises worker read it.
func (s *DurableStore) MarkRequirements(id string) (Header, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, err := s.read(id)
	if err != nil {
		return Header{}, err
	}
	r.Requirements = true
	if err := WriteJSON(filepath.Join(s.dir, r.ID+".meta.json"), r.Header); err != nil {
		return Header{}, err
	}
	s.headers[r.ID] = r.Header
	if err := WriteJSON(filepath.Join(s.dir, r.ID+".json"), r); err != nil {
		return Header{}, err
	}
	return visible(r.Header), nil
}
