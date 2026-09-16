package promptchecks

import (
	"context"
	"fmt"

	"github.com/tylergannon/gimble"
)

const constantPrompt = "constant prompt"
const constantInstruction = "constant instruction"

func promptChecks(ctx context.Context, input string) {
	session := &gimble.Session{}

	_, _ = session.Generate[string](ctx, constantPrompt)
	_, _ = session.Generate[string](ctx, "literal prompt", gimble.WithSupervisor(session, constantInstruction))

	_, _ = session.Generate[string](ctx, "prompt "+input)                 // want `GIMBLE108-SIMPLE-WORKFLOWS/CONSTANT-PROMPT`
	_, _ = session.Generate[string](ctx, fmt.Sprintf("prompt %s", input)) // want `GIMBLE108-SIMPLE-WORKFLOWS/CONSTANT-PROMPT`

	variable := "instruction"
	_ = gimble.WithSupervisor(session, variable) // want `GIMBLE108-SIMPLE-WORKFLOWS/CONSTANT-PROMPT`
}
