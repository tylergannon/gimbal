package runstate

import (
	"context"
	"encoding/json"
	"errors"
	"io"

	"github.com/tylergannon/gimbal/runstore"
)

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
