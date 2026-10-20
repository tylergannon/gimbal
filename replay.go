package gimbal

import (
	"context"
	"encoding/json"
	"io"
	"time"

	"github.com/tylergannon/gimbal/runstore"
)

// suspension crosses only library-owned recovery boundaries. It is neither a
// workflow error nor normal completion, so scope teardown must not run.
type suspension struct{}

func (r *run) loadReplay() error {
	r.replay = make(map[runstore.NodeID][]runstore.Record)
	var cursor uint64
	for {
		records, err := r.records.Read(context.Background(), cursor, 1000)
		if err != nil {
			return err
		}
		if len(records) == 0 {
			return nil
		}
		for _, record := range records {
			r.replay[record.Node] = append(r.replay[record.Node], record)
			cursor = record.Sequence
		}
	}
}

func (r *run) saved(node runstore.NodeID, kind string, into any) (bool, error) {
	for _, record := range r.replay[node] {
		if record.Kind != kind {
			continue
		}
		raw, err := r.expand(record.Payload)
		if err != nil {
			return true, err
		}
		return true, json.Unmarshal(raw, into)
	}
	return false, nil
}
func (r *run) expand(raw json.RawMessage) (json.RawMessage, error) {
	var envelope struct {
		Blob runstore.BlobRef `json:"blob"`
	}
	if json.Unmarshal(raw, &envelope) != nil || envelope.Blob.Key == "" {
		return raw, nil
	}
	reader, err := r.records.OpenBlob(context.Background(), envelope.Blob)
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	return io.ReadAll(reader)
}
func (r *run) outcome(node runstore.NodeID, into any) (bool, error) {
	var envelope struct {
		Category string          `json:"category"`
		Error    string          `json:"error"`
		Result   json.RawMessage `json:"result"`
	}
	found, err := r.saved(node, "outcome", &envelope)
	for _, record := range r.replay[node] {
		if record.Kind == "resolved-outcome" {
			raw, e := r.expand(record.Payload)
			if e != nil {
				return true, e
			}
			if e = json.Unmarshal(raw, &envelope); e != nil {
				return true, e
			}
			found = true
		}
	}
	if !found || err != nil {
		return found, err
	}
	if envelope.Category == "unresolved" {
		return false, nil
	}
	raw, err := r.expand(envelope.Result)
	if err != nil {
		return true, err
	}
	if into != nil {
		if err = json.Unmarshal(raw, into); err != nil {
			return true, err
		}
	}
	return true, stringError(envelope.Error)
}
func (r *run) hasFact(node runstore.NodeID, kind string) bool {
	for _, record := range r.replay[node] {
		if record.Kind == kind {
			return true
		}
	}
	return false
}

func (r *run) acceptedContext(ctx context.Context, node runstore.NodeID) (context.Context, context.CancelFunc) {
	var intent struct {
		Input struct {
			Deadline time.Time `json:"deadline"`
		} `json:"input"`
	}
	if found, err := r.saved(node, "intent", &intent); found && err == nil && !intent.Input.Deadline.IsZero() {
		return context.WithDeadline(ctx, intent.Input.Deadline)
	}
	return ctx, func() {}
}
