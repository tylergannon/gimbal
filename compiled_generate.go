package gimbal

import (
	"context"
	"fmt"

	"github.com/tylergannon/gimbal/internal/compiledscope"
)

func (compiledRuntime) GenerateResponse(ctx context.Context, session any, input compiledscope.Snapshot, prompt string, output compiledscope.Output) ([]byte, error) {
	s, ok := session.(*Session)
	if !ok {
		return nil, fmt.Errorf("compiled Generate requires a Gimbal session")
	}
	bound, err := bindCompiledContext(ctx, input)
	if err != nil {
		return nil, err
	}
	return s.generateResponse(bound, prompt, output, nil)
}
