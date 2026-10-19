package gimbal

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/tylergannon/gimbal/backend"
	"github.com/tylergannon/gimbal/internal/effects"

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
	raw, err := r.payload(payload)
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

func (r *run) nodeID(key string) runstore.NodeID {
	r.nodeMu.Lock()
	defer r.nodeMu.Unlock()
	return r.nodeByKey[key]
}
func actionIdentity(scope, session, action runstore.NodeID, kind string) backend.ExecutionIdentity {
	i := backend.ExecutionIdentity{ScopeID: string(scope), SessionID: string(session), ActionID: string(action), RequestID: string(action), ExecutorOperation: "turn_" + string(action)}
	if kind == "cleanup" {
		i.ExecutorOperation = ""
	}
	if kind == "process" {
		i.ExecutorOperation = "command_" + string(action)
	}
	if session != "" {
		i.ExecutorSession = "session_" + string(session)
	}
	return i
}
func (r *run) prepareAction(ctx context.Context, scope, session, node runstore.NodeID, kind string, input any, identity backend.ExecutionIdentity) error {
	if err := r.recordingError(); err != nil {
		return err
	}
	intent := map[string]any{"operation": kind, "input": input, "route": effects.Route(ctx), "identity": identity}
	raw, err := r.payload(intent)
	if err != nil {
		return err
	}
	binding, err := json.Marshal(identity)
	if err != nil {
		return err
	}
	_, err = r.records.Append(context.WithoutCancel(ctx), []runstore.Fact{
		{Version: 1, ID: runstore.NewRecordID(), Node: node, Scope: scope, Session: session, Kind: "intent", Payload: raw},
		{Version: 1, ID: runstore.NewRecordID(), Node: node, Scope: scope, Session: session, Kind: "binding", Payload: binding},
	}, runstore.Committed)
	r.recordFailure("commit action intent", err)
	return err
}
func (r *run) payload(value any) (json.RawMessage, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		r.recordFailure("encode action", err)
		return nil, err
	}
	if len(raw) < 512<<10 {
		return raw, nil
	}
	ref, err := r.records.PutBlob(context.Background(), bytes.NewReader(raw))
	r.recordFailure("store payload", err)
	if err != nil {
		return nil, err
	}
	return json.Marshal(struct {
		Blob runstore.BlobRef `json:"blob"`
	}{ref})
}
func (r *run) finishAction(scope, session, node runstore.NodeID, result any, nativeErr error) error {
	raw, err := r.payload(result)
	if err != nil {
		return err
	}
	category := "success"
	if nativeErr != nil {
		category = "error"
	}
	if errors.Is(nativeErr, ErrInvalidOutput) {
		category = "invalid-output"
	}
	if errors.Is(nativeErr, context.DeadlineExceeded) {
		category = "deadline-exceeded"
	}
	if errors.Is(nativeErr, context.Canceled) {
		category = "cancelled"
	}
	var uncertain *backend.DeliveryUncertain
	if errors.As(nativeErr, &uncertain) {
		category = "unresolved"
	}
	return r.recordFact(node, scope, session, "outcome", struct {
		Category string          `json:"category"`
		Error    string          `json:"error,omitempty"`
		Result   json.RawMessage `json:"result"`
	}{category, errString(nativeErr), raw}, runstore.Committed)
}
func (r *run) nativeBinding(scope, session, node runstore.NodeID, native string) error {
	return r.recordFact(node, scope, session, "native-binding", map[string]string{"native": native}, runstore.Committed)
}
func (r *run) commandOutcome(s *scope, ended CommandEnded) error {
	r.lifecycleMu.Lock()
	defer r.lifecycleMu.Unlock()
	if r.closed {
		return nil
	}
	refs := map[string]runstore.BlobRef{}
	for name, path := range map[string]string{"stdout": ended.StdoutFile, "stderr": ended.StderrFile} {
		if path == "" {
			continue
		}
		f, err := os.Open(filepath.Join(r.dir, filepath.FromSlash(path)))
		if err != nil {
			r.recordFailure("open completed output", err)
			return err
		}
		ref, err := r.records.PutBlob(context.Background(), f)
		err = errors.Join(err, f.Close())
		if err != nil {
			r.recordFailure("store completed output", err)
			return err
		}
		refs[name] = ref
	}
	return r.finishAction(s.node, "", r.nodeID(ended.ID), struct {
		ExitCode    int                         `json:"exit_code"`
		Interrupted bool                        `json:"interrupted"`
		Output      map[string]runstore.BlobRef `json:"output"`
	}{ended.ExitCode, ended.Interrupted, refs}, stringError(ended.Error))
}
func stringError(message string) error {
	if message == "" {
		return nil
	}
	return errors.New(message)
}

func (r *run) optionalNodeID(key string) runstore.NodeID {
	if key == "" {
		return ""
	}
	return r.nodeID(key)
}
