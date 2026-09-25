package promptchecks

import (
	"context"
	_ "embed"
	"fmt"

	"github.com/tylergannon/gimbal"
)

const constantPrompt = "constant prompt"
const constantInstruction = "constant instruction"
const constantShape = "{{range .Values}}{{.Key}}{{end}}"

// embeddedShape is a long template, which reads better as a file. The
// directive names the file, so GIMBAL109 allows it: the text is as readable
// from the source as a constant is.
//
//go:embed shape.tmpl
var embeddedShape string

// builtShape is neither, so GIMBAL109 reports it.
var builtShape = "{{range .Values}}" + constantShape + "{{end}}"

func promptChecks(ctx context.Context, input string) {
	session := &gimbal.Session{}

	_, _ = session.Generate[string](ctx, constantPrompt)
	_, _ = session.Generate[string](ctx, "literal prompt", gimbal.WithSupervisor(session, constantInstruction))

	_, _ = session.Generate[string](ctx, "prompt "+input)                 // want `GIMBAL108-SIMPLE-WORKFLOWS/CONSTANT-PROMPT`
	_, _ = session.Generate[string](ctx, fmt.Sprintf("prompt %s", input)) // want `GIMBAL108-SIMPLE-WORKFLOWS/CONSTANT-PROMPT`

	variable := "instruction"
	_ = gimbal.WithSupervisor(session, variable) // want `GIMBAL108-SIMPLE-WORKFLOWS/CONSTANT-PROMPT`

	_, _ = session.Generate[string](ctx, constantPrompt, gimbal.WithScopeTemplate(constantShape))
	_, _ = session.Generate[string](ctx, constantPrompt, gimbal.WithScopeTemplate(embeddedShape))

	_, _ = session.Generate[string](ctx, constantPrompt, gimbal.WithScopeTemplate(builtShape))                       // want `GIMBAL109-SIMPLE-WORKFLOWS/CONSTANT-SCOPE-TEMPLATE`
	_, _ = session.Generate[string](ctx, constantPrompt, gimbal.WithScopeTemplate("{{.Values}}"+input))              // want `GIMBAL109-SIMPLE-WORKFLOWS/CONSTANT-SCOPE-TEMPLATE`
	_, _ = session.Generate[string](ctx, constantPrompt, gimbal.WithScopeTemplate(fmt.Sprintf("%s", constantShape))) // want `GIMBAL109-SIMPLE-WORKFLOWS/CONSTANT-SCOPE-TEMPLATE`
}
