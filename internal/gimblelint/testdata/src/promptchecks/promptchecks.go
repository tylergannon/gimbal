package promptchecks

import (
	"context"
	_ "embed"
	"fmt"

	"github.com/tylergannon/gimble"
)

const constantPrompt = "constant prompt"
const constantInstruction = "constant instruction"
const constantShape = "{{range .Values}}{{.Key}}{{end}}"

// embeddedShape is a long template, which reads better as a file. The
// directive names the file, so GIMBLE109 allows it: the text is as readable
// from the source as a constant is.
//
//go:embed shape.tmpl
var embeddedShape string

// builtShape is neither, so GIMBLE109 reports it.
var builtShape = "{{range .Values}}" + constantShape + "{{end}}"

func promptChecks(ctx context.Context, input string) {
	session := &gimble.Session{}

	_, _ = session.Generate[string](ctx, constantPrompt)
	_, _ = session.Generate[string](ctx, "literal prompt", gimble.WithSupervisor(session, constantInstruction))

	_, _ = session.Generate[string](ctx, "prompt "+input)                 // want `GIMBLE108-SIMPLE-WORKFLOWS/CONSTANT-PROMPT`
	_, _ = session.Generate[string](ctx, fmt.Sprintf("prompt %s", input)) // want `GIMBLE108-SIMPLE-WORKFLOWS/CONSTANT-PROMPT`

	variable := "instruction"
	_ = gimble.WithSupervisor(session, variable) // want `GIMBLE108-SIMPLE-WORKFLOWS/CONSTANT-PROMPT`

	_, _ = session.Generate[string](ctx, constantPrompt, gimble.WithScopeTemplate(constantShape))
	_, _ = session.Generate[string](ctx, constantPrompt, gimble.WithScopeTemplate(embeddedShape))

	_, _ = session.Generate[string](ctx, constantPrompt, gimble.WithScopeTemplate(builtShape))                       // want `GIMBLE109-SIMPLE-WORKFLOWS/CONSTANT-SCOPE-TEMPLATE`
	_, _ = session.Generate[string](ctx, constantPrompt, gimble.WithScopeTemplate("{{.Values}}"+input))              // want `GIMBLE109-SIMPLE-WORKFLOWS/CONSTANT-SCOPE-TEMPLATE`
	_, _ = session.Generate[string](ctx, constantPrompt, gimble.WithScopeTemplate(fmt.Sprintf("%s", constantShape))) // want `GIMBLE109-SIMPLE-WORKFLOWS/CONSTANT-SCOPE-TEMPLATE`
}
