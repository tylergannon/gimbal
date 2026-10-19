package gimbal

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/tylergannon/gimbal/internal/runstate"
	"github.com/tylergannon/gimbal/runstore"
)

// WithRunStore supplies the recording backend for a direct in-process Run.
// Application runners resolve their store from project policy on their host.
func WithRunStore(ctx context.Context, store runstore.Store) context.Context {
	return runstate.WithStore(ctx, store)
}

func (r *run) ensureNode(parent runstore.NodeID, kind runstore.Kind, name string, ordinal int, key string, metadata any) runstore.NodeID {
	var raw json.RawMessage
	if metadata != nil {
		var err error
		raw, err = json.Marshal(metadata)
		if err != nil {
			r.recordFailure("encode node", err)
			return ""
		}
	}
	node, err := r.records.EnsureNode(context.Background(), runstore.NodeSpec{Slot: runstore.Slot{Parent: parent, Kind: kind, Name: name, Ordinal: ordinal}, DisplayKey: key, Metadata: raw})
	r.recordFailure("create node "+key, err)
	if err != nil {
		return ""
	}
	r.nodeMu.Lock()
	r.nodeByKey[key] = node.ID
	r.nodeMu.Unlock()
	return node.ID
}
func (s *scope) nextNode(kind runstore.Kind, name string) (string, runstore.NodeID) {
	s.mu.Lock()
	key := s.next(name)
	ordinal := s.ordinals[name]
	ended := s.ended
	s.mu.Unlock()
	if ended {
		s.run.recordFailure("create node", fmt.Errorf("scope %q ended", s.key))
		return key, ""
	}
	id := s.run.ensureNode(s.node, kind, name, ordinal, key, nil)
	s.mu.Lock()
	ended = s.ended
	s.mu.Unlock()
	if ended {
		s.run.recordFailure("create node", fmt.Errorf("scope %q ended", s.key))
	}
	return key, id
}
func (r *run) recordFact(node, scope, session runstore.NodeID, kind string, payload any, d runstore.Durability) error {
	raw, err := json.Marshal(payload)
	if err == nil {
		_, err = r.records.Append(context.Background(), []runstore.Fact{{Version: 1, ID: runstore.NewRecordID(), Node: node, Scope: scope, Session: session, Kind: kind, Payload: raw}}, d)
	}
	r.recordFailure(kind, err)
	return err
}
func (r *run) activate(node runstore.NodeID) error {
	if err := r.recordingError(); err != nil {
		return err
	}
	return r.recordFact(node, "", "", "activated", struct{}{}, runstore.Committed)
}
