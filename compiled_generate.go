package gimbal

import (
	"context"
	"fmt"

	"github.com/tylergannon/gimbal/internal/compiledscope"
)

func init() {
	compiledscope.GenerateResponse = func(ctx context.Context, session any, prompt string, output compiledscope.Output) ([]byte, error) {
		s, ok := session.(*Session)
		if !ok {
			return nil, fmt.Errorf("compiled Generate requires a Gimbal session")
		}
		return s.generateResponse(ctx, prompt, output, nil)
	}
}
