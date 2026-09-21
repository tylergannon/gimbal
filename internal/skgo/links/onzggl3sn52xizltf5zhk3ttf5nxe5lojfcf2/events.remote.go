package runid

// Spike for the question "can the run page stream through a skgo
// query.live instead of the hand-rolled SSE endpoint in
// internal/observation/http.go, without re-sending the whole observation on
// every change?" See ephemeral/research/live-query-poc/ for the write-up.
// This file is not wired into the page in main; it exists on
// claude/live-query-poc only.

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/tylergannon/skgo"

	"github.com/tylergannon/gimble/internal/observation"
	hooks "github.com/tylergannon/gimble/web/src"
)

// coalesceWindow batches deltas that land together into one frame, so a
// burst of native events crosses the wire once instead of once per delta.
const coalesceWindow = 50 * time.Millisecond

// EventArg is what the page's live query subscribes with: which run, which
// stream generation, and the position it already holds. Kit holds this fixed
// for the life of one stream instance; the page opens a new instance (a new
// argument) whenever it needs to resume from a different position.
type EventArg struct {
	Run      string `json:"run"`
	Stream   string `json:"stream"`
	Position uint64 `json:"position"`
}

// EventBatch is hooks.EventBatch: one frame of the live query, carrying its
// payload as its own JSON, the same transport trick hooks.RunSnapshot uses,
// because the delta frames and the reset snapshot both carry the run's open
// native schema that polytype's static grammar cannot project directly.
//
// The JSON decodes to one of:
//
//	{"from": 3, "to": 5, "deltas": [Delta, Delta]}   // advance from 3 to 5
//	{"reset": RunSnapshot}                           // the join could not resume
type EventBatch = hooks.EventBatch

// wireBatch is EventBatch.JSON's shape, mirrored in
// web/src/lib/observation/index.ts as EventBatchWire.
type wireBatch struct {
	From   uint64                   `json:"from,omitempty"`
	To     uint64                   `json:"to,omitempty"`
	Deltas []observation.Delta      `json:"deltas,omitempty"`
	Reset  *observation.RunSnapshot `json:"reset,omitempty"`
}

// watchRun streams one run's observation from arg.Position onward: a reset
// snapshot when the join cannot resume there, otherwise the deltas the store
// already has queued and then live ones as they are produced, coalesced by
// coalesceWindow. It joins the store exactly as serveEvents
// (internal/observation/http.go) does; only the framing differs.
func watchRun(ctx context.Context, arg EventArg, yield func(EventBatch) error) error {
	registry := observation.FromContext(ctx)
	if registry == nil {
		return skgo.Errorf(http.StatusInternalServerError,
			"This server has no observation registry in its context, so no run can be watched.")
	}

	store, live := registry.Live(arg.Run)
	if !live {
		snapshot, err := registry.Snapshot(arg.Run)
		if err != nil {
			return skgo.Errorf(http.StatusNotFound, "There is no run %s in this project.", arg.Run)
		}
		return yieldReset(yield, snapshot)
	}

	snapshot, suffix, sub, err := store.Join(arg.Stream, arg.Position)
	if err != nil {
		return err
	}
	if sub != nil {
		defer sub.Close()
	}
	if snapshot != nil {
		if err := yieldReset(yield, *snapshot); err != nil {
			return err
		}
	}
	if len(suffix) > 0 {
		if err := yieldDeltas(yield, suffix); err != nil {
			return err
		}
	}
	if sub == nil {
		// A finished run has a complete snapshot and no suffix: nothing more
		// will ever be produced, so the stream ends here.
		return nil
	}

	timer := time.NewTimer(coalesceWindow)
	if !timer.Stop() {
		<-timer.C
	}
	armed := false
	var pending []observation.Delta
	flush := func() error {
		if len(pending) == 0 {
			return nil
		}
		batch := pending
		pending = nil
		return yieldDeltas(yield, batch)
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-sub.Done():
			return sub.Err()
		case delta, open := <-sub.Deltas():
			if !open {
				// The run finished normally and its queue has been handed
				// over in full, the run's terminal row included.
				return flush()
			}
			sub.Took(delta)
			pending = append(pending, delta)
			if !armed {
				armed = true
				timer.Reset(coalesceWindow)
			}
		case <-timer.C:
			armed = false
			if err := flush(); err != nil {
				return err
			}
		}
	}
}

func yieldReset(yield func(EventBatch) error, snapshot observation.RunSnapshot) error {
	encoded, err := json.Marshal(wireBatch{Reset: &snapshot})
	if err != nil {
		return err
	}
	return yield(EventBatch{JSON: string(encoded)})
}

func yieldDeltas(yield func(EventBatch) error, deltas []observation.Delta) error {
	batch := wireBatch{
		From:   deltas[0].Position - 1,
		To:     deltas[len(deltas)-1].Position,
		Deltas: deltas,
	}
	encoded, err := json.Marshal(batch)
	if err != nil {
		return err
	}
	return yield(EventBatch{JSON: string(encoded)})
}

var _ = skgo.LiveQuery(watchRun)
