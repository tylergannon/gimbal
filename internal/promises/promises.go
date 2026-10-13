// Package promises is a STUB of the promises ledger API (plan §3), so the host
// and web layers compile while the real package is built on pl-core. It is
// replaced by the real package at merge.
package promises

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/claude"
	"github.com/tylergannon/gimbal/internal/modelalias"
	"github.com/tylergannon/gimbal/internal/transcript"
)

type Store struct {
	mu      sync.Mutex
	dir     string
	ledger  Ledger
	version int64
}

type Ledger struct {
	Entries []Entry        `json:"entries"`
	Cursors map[string]int `json:"cursors"`
	Updated int64          `json:"updated"`
}

type Failure struct {
	Recording string `json:"recording"`
	Message   string `json:"message"`
}

type View struct {
	Entries []Entry   `json:"entries"`
	Working bool      `json:"working"`
	Errors  []Failure `json:"errors"`
	Version int64     `json:"version"`
}

type Entry struct {
	ID        int        `json:"id"`
	Title     string     `json:"title"`
	Details   string     `json:"details"`
	Tags      []string   `json:"tags"`
	Withdrawn bool       `json:"withdrawn"`
	Created   int64      `json:"created"`
	Updated   int64      `json:"updated"`
	Evidence  []Evidence `json:"evidence"`
}

type Evidence struct {
	Recording string `json:"recording"`
	Chunk     int    `json:"chunk"`
	Quote     string `json:"quote"`
}

func Open(dir string) (*Store, error) {
	s := &Store{dir: filepath.Join(dir, "promises"), version: 1}
	if err := os.MkdirAll(filepath.Join(s.dir, "work"), 0o755); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(s.dir, "ledger.json"))
	if err == nil {
		err = json.Unmarshal(data, &s.ledger)
	} else if errors.Is(err, os.ErrNotExist) {
		err = nil
	}
	return s, err
}

func (s *Store) Snapshot() View {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries := make([]Entry, len(s.ledger.Entries))
	copy(entries, s.ledger.Entries)
	return View{Entries: entries, Errors: []Failure{}, Version: s.version}
}

func (s *Store) SetWithdrawn(id int, withdrawn bool) (Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, e := range s.ledger.Entries {
		if e.ID == id {
			next := s.ledger
			next.Entries = append([]Entry(nil), s.ledger.Entries...)
			e.Withdrawn, e.Updated = withdrawn, time.Now().UnixMilli()
			next.Entries[i] = e
			if err := transcript.WriteJSON(filepath.Join(s.dir, "ledger.json"), next); err != nil {
				return Entry{}, err
			}
			s.ledger = next
			s.version++
			return e, nil
		}
	}
	return Entry{}, fmt.Errorf("promise %d not found", id)
}

func (s *Store) Run(ctx context.Context, recordings *transcript.DurableStore, model gimbal.ModelBinding) {
	<-ctx.Done()
}

func Model(spec string) (gimbal.ModelBinding, error) {
	if spec == "" {
		spec = "sonnet:low"
	}
	name, effort, has := strings.Cut(spec, ":")
	r, err := modelalias.Resolve(modelalias.Selection{Name: name, Effort: effort, EffortPresent: has})
	if err != nil {
		return gimbal.ModelBinding{}, fmt.Errorf("GIMBAL_PROMISES_MODEL %q: %w", spec, err)
	}
	if r.Harness != "claude" {
		return gimbal.ModelBinding{}, fmt.Errorf("GIMBAL_PROMISES_MODEL %q: the promises ledger needs a Claude model", spec)
	}
	return gimbal.ModelBinding{Adapter: claude.New(), Model: r.Model, Effort: r.Effort}, nil
}
