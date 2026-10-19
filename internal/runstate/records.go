package runstate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/tylergannon/gimbal/runstore"
)

type CleanupState struct {
	Status string `json:"status"`
	Error  string `json:"error"`
}
type Summary struct {
	Intents  map[runstore.NodeID]bool
	Header   runstore.Header
	Ended    bool
	Nodes    map[string]runstore.Node
	Cleanup  map[runstore.NodeID]CleanupState
	Outcomes map[runstore.NodeID]bool
}

func Inspect(ctx context.Context, r runstore.Run) (Summary, error) {
	h, e := r.Header(ctx)
	s := Summary{Header: h, Nodes: map[string]runstore.Node{}, Cleanup: map[runstore.NodeID]CleanupState{}, Outcomes: map[runstore.NodeID]bool{}, Intents: map[runstore.NodeID]bool{}}
	if e != nil {
		return s, e
	}
	var cursor uint64
	for {
		records, e := r.Read(ctx, cursor, 512)
		if e != nil {
			return s, e
		}
		if len(records) == 0 {
			return s, nil
		}
		for _, rec := range records {
			cursor = rec.Sequence
			switch rec.Kind {
			case "node-created":
				var n runstore.Node
				if e = json.Unmarshal(rec.Payload, &n); e != nil {
					return s, e
				}
				s.Nodes[n.DisplayKey] = n
			case "cleanup-state":
				var state CleanupState
				if e = json.Unmarshal(rec.Payload, &state); e != nil {
					return s, e
				}
				s.Cleanup[rec.Node] = state
			case "intent":
				s.Intents[rec.Node] = true
			case "outcome":
				s.Outcomes[rec.Node] = true
			case "lifecycle":
				var v struct {
					Event struct {
						Kind string `json:"kind"`
					} `json:"event"`
				}
				if e = json.Unmarshal(rec.Payload, &v); e != nil {
					return s, e
				}
				if v.Event.Kind == "run_ended" {
					s.Ended = true
				}
			}
		}
	}
}
func Append(ctx context.Context, r runstore.Run, node, scope runstore.NodeID, kind string, value any) error {
	raw, e := json.Marshal(value)
	if e != nil {
		return e
	}
	_, e = r.Append(ctx, []runstore.Fact{{Version: 1, ID: runstore.NewRecordID(), Node: node, Scope: scope, Kind: kind, Payload: raw}}, runstore.Committed)
	return e
}
func UpdateCleanup(ctx context.Context, r runstore.Run, node runstore.NodeID, state CleanupState) (bool, error) {
	summary, e := Inspect(ctx, r)
	if e != nil {
		return false, e
	}
	if old, ok := summary.Cleanup[node]; ok && old == state {
		return false, nil
	}
	scope := summary.Header.Root
	for _, n := range summary.Nodes {
		if n.ID == node {
			if n.Kind == runstore.Scope {
				scope = n.ID
			} else {
				scope = n.Parent
			}
			break
		}
	}
	return true, Append(ctx, r, node, scope, "cleanup-state", state)
}

// NextAction derives post-terminal occurrence from stored slots, so a new owner
// cannot reset its ordinal counter. It does not activate workflow execution.
func NextAction(ctx context.Context, r runstore.Run, parent runstore.NodeID, name, key string) (runstore.Node, error) {
	for ordinal := 1; ; ordinal++ {
		slot := runstore.Slot{Parent: parent, Kind: runstore.Action, Name: name, Ordinal: ordinal}
		_, e := r.Node(ctx, slot)
		if e == nil {
			continue
		}
		if !errors.Is(e, runstore.ErrNotFound) {
			return runstore.Node{}, e
		}
		return r.EnsureNode(ctx, runstore.NodeSpec{Slot: slot, DisplayKey: fmt.Sprintf("%s/%s.%d", key, name, ordinal)})
	}
}

// Payload reads a versioned record's inline JSON or its run-owned blob.
func Payload(ctx context.Context, r runstore.Run, payload json.RawMessage) (json.RawMessage, error) {
	var wrapped struct {
		Blob *runstore.BlobRef `json:"blob"`
	}
	if json.Unmarshal(payload, &wrapped) != nil || wrapped.Blob == nil {
		return payload, nil
	}
	f, e := r.OpenBlob(ctx, *wrapped.Blob)
	if e != nil {
		return nil, e
	}
	raw, e := io.ReadAll(f)
	return raw, errors.Join(e, f.Close())
}
