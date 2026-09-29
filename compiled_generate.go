package gimbal

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/tylergannon/gimbal/contextdata"
)

// GenerateResponse runs the shared agent operation against an explicit immutable
// input snapshot, returning accepted bytes for consumption at the authored call.
// A consumer compiler must first admit T with compiler.CheckSplitResponse.
// Empty input explicitly selects no scoped values; it never uses live ancestors.
func (s *Session) GenerateResponse[T Output](ctx context.Context, input contextdata.Snapshot, prompt string, opts ...AgentOption) ([]byte, error) {
	if s == nil {
		return nil, fmt.Errorf("gimbal: session is nil")
	}
	bound, err := bindCompiledContext(ctx, input)
	if err != nil {
		return nil, err
	}
	var output T
	return s.generateResponse(bound, prompt, output, opts)
}

// ConsumeResponse reconstructs an admitted response without agent calls.
// Operation failure returns T's zero value. Decode failure means the accepted
// result or its transport did not satisfy the compiler's split-response contract.
func ConsumeResponse[T Output](raw []byte, operationErr error) (T, error) {
	var out T
	if operationErr != nil {
		return out, operationErr
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return out, fmt.Errorf("gimbal: accepted response cannot be decoded: %w", err)
	}
	return out, nil
}
