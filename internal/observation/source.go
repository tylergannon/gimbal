package observation

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/tylergannon/gimbal/backend"
)

// OpenSource observes an environment without interpreting its filesystem or lifetime.
func OpenSource(registry *Registry, id, name string, source backend.RunObservations) *Store {
	s := newStore(registry, id, name, "")
	s.external = true
	s.noWrite = true
	s.source = source
	return s
}
func (s *Store) refreshSource() error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	response, err := s.source.ReadObservation(ctx, backend.Target{Run: s.id, ID: s.id}, s.sourceCursor)
	if err != nil {
		return err
	}
	if response.Unavailable {
		return fmt.Errorf("observation unavailable")
	}
	hasSnapshot := len(response.Snapshot) > 0 && !bytes.Equal(bytes.TrimSpace(response.Snapshot), []byte("null"))
	if response.Gap && !hasSnapshot {
		s.sourceCursor = ""
		return fmt.Errorf("observation gap")
	}
	if hasSnapshot {
		var snapshot RunSnapshot
		if err := json.Unmarshal(response.Snapshot, &snapshot); err != nil {
			return err
		}
		if s.sourceCursor != "" {
			for sub := range s.joins {
				sub.mu.Lock()
				if !sub.closed {
					sub.err = ErrOverflow
					sub.closed = true
					close(sub.done)
				}
				sub.mu.Unlock()
				delete(s.joins, sub)
			}
			s.finishSubscribersLocked()
		}
		fresh := newStore(nil, s.id, s.run.Name, "")
		s.scopes = fresh.scopes
		s.sessions = fresh.sessions
		s.interactions = fresh.interactions
		s.turns = fresh.turns
		s.turnUsage = fresh.turnUsage
		s.modelCalls = fresh.modelCalls
		s.commands = fresh.commands
		s.transcripts = fresh.transcripts
		s.calls = fresh.calls
		s.recent = nil
		s.restoreLocked(snapshot)
		s.closed = s.run.Status != StatusRunning
	}
	for _, raw := range response.Deltas {
		var delta Delta
		if err := json.Unmarshal(raw, &delta); err != nil {
			return err
		}
		if delta.Stream != s.stream || delta.Position != s.position+1 {
			s.sourceCursor = ""
			return fmt.Errorf("observation gap")
		}
		s.applyDeltaLocked(delta)
		s.retainLocked(delta)
		s.publishDeltaLocked(delta)
	}
	s.sourceCursor = response.Cursor
	if s.run.Status != StatusRunning {
		s.closed = true
		s.finishJoinSubscribersLocked()
	}
	return nil
}
