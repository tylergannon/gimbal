// Package messagefiles stores retained immutable messaging facts. It is shared
// by filesystem delivery and SSE producers, not part of the messaging seam.
package messagefiles

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/tylergannon/gimbal/backend"
)

type Store struct{ root string }

func Open(root string) (*Store, error) {
	if root == "" {
		return nil, fmt.Errorf("event directory is required")
	}
	if err := os.MkdirAll(root, 0700); err != nil {
		return nil, err
	}
	return &Store{root: root}, nil
}
func (s *Store) PublishEvent(ctx context.Context, e backend.MessageEvent) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if _, err := ulid.ParseStrict(e.ID); err != nil {
		return fmt.Errorf("event ID: %w", err)
	}
	raw, err := json.Marshal(e)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(s.root, ".event-")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(f.Name()) }()
	if _, err = f.Write(raw); err == nil {
		err = f.Sync()
	}
	err2 := f.Close()
	if err != nil {
		return err
	}
	if err2 != nil {
		return err2
	}
	// Hard-link publication is atomic and cannot overwrite the original fact.
	err = os.Link(f.Name(), filepath.Join(s.root, e.ID+".json"))
	if os.IsExist(err) {
		return nil
	}
	return err
}
func (s *Store) ListenEvents(ctx context.Context, selection backend.MessageSelection, since time.Time, emit func(backend.MessageEvent) error) error {
	seen := map[string]bool{}
	ticker := time.NewTicker(40 * time.Millisecond)
	defer ticker.Stop()
	for {
		entries, err := os.ReadDir(s.root)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" || seen[entry.Name()] {
				continue
			}
			raw, err := os.ReadFile(filepath.Join(s.root, entry.Name()))
			if err != nil {
				return err
			}
			var e backend.MessageEvent
			if err = json.Unmarshal(raw, &e); err != nil {
				return err
			}
			seen[entry.Name()] = true
			if !e.Created.Before(since) && selection.Matches(e) {
				if err = emit(e); err != nil {
					return err
				}
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}
