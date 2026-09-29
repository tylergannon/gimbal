package compiledscope

import (
	"context"
	"encoding/json"
	"fmt"
)

// Output is the existing Gimbal result contract, repeated here to avoid a cycle.
// Compiler admission must establish that schema acceptance implies decodability.
type Output interface {
	Schema() json.RawMessage
	ValidateJSON([]byte) error
}

// Generate consumes an explicit immutable snapshot at the operation boundary.
func Generate[T Output](ctx context.Context, session any, input Snapshot, prompt string) ([]byte, error) {
	var output T
	r, err := runtime(ctx)
	if err != nil {
		return nil, err
	}
	return r.GenerateResponse(ctx, session, input, prompt, output)
}

// DecodeInto invokes Polytype-generated JSON methods where the type has them,
// and standard JSON decoding for ordinary fields. It neither validates nor asks
// an agent; acceptance belongs to the operation that produced these bytes.
func DecodeInto[T any](raw []byte, out *T) error { return json.Unmarshal(raw, out) }

// Consume reconstructs a supported result once, at its authored point of use.
// No bytes on operation failure means the source's zero result. Decoding an
// accepted response must succeed; an error here is a contract/transport defect.
func Consume[T any](raw []byte, operationErr error) (T, error) {
	var out T
	if operationErr != nil {
		return out, operationErr
	}
	if err := DecodeInto(raw, &out); err != nil {
		return out, fmt.Errorf("gimbal: accepted response cannot be decoded: %w", err)
	}
	return out, nil
}
