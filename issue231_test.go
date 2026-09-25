package gimbal

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// A shaped call passes the template's text, which like the prompt is
// readable in the source: a constant here, and in a workflow whose template
// is long, a file brought in with go:embed.
const shapedScopeText = `The goal is {{.By.goal.Value}}, and the steps are {{(index .By "steps").Text}}.
{{range .Values}}
[{{.Key}}]{{end}}`

// brokenScopeText parses and then cannot render: Values is a list, and has
// no field of that name.
const brokenScopeText = `{{.Values.NotAField}}`

// unparsedScopeText is not a template at all.
const unparsedScopeText = `{{range .Values}}`

const askPrompt = "Answer the question."

const watchInstruction = "Object to nothing."

// TestWithScopeTemplateShapesTheScopeForOneCall: the option renders the
// scope its own way for the call it is given to, with and without a
// supervisor, and leaves every other call on the runtime's own rendering.
func TestWithScopeTemplateShapesTheScopeForOneCall(t *testing.T) {
	var prompts []string
	f := &fake{answer: func(_ context.Context, _, prompt string, _ json.RawMessage, _ func(AgentEvent) error) (string, error) {
		prompts = append(prompts, prompt)
		return "done", nil
	}}
	err := runTest(t, bind(f, "model", "worker", "watcher"), func(ctx context.Context) error {
		Set(ctx, "goal", "a shaped prompt")
		Set(ctx, "steps", []string{"one", "two"})
		worker := NewSession(ctx, "worker", ".")
		if _, err := worker.Generate[Text](ctx, askPrompt, WithScopeTemplate(shapedScopeText)); err != nil {
			return err
		}
		if _, err := worker.Generate[Text](ctx, askPrompt); err != nil {
			return err
		}
		watcher := NewSession(ctx, "watcher", ".")
		_, err := worker.Generate[Text](ctx, askPrompt, WithScopeTemplate(shapedScopeText), WithSupervisor(watcher, watchInstruction))
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(prompts) != 3 {
		t.Fatalf("turns = %d, want three", len(prompts))
	}
	// A shaped call renders the scope through its template, where Value is
	// the value itself and Text is the runtime's own rendering of it.
	shaped := askPrompt + "\n\nThe goal is a shaped prompt, and the steps are " + "[\n  \"one\",\n  \"two\"\n]" + ".\n\n[goal]\n[steps]"
	if prompts[0] != shaped {
		t.Fatalf("shaped prompt =\n%s\nwant\n%s", prompts[0], shaped)
	}
	if prompts[2] != shaped {
		t.Fatalf("a supervised call was not shaped:\n%s", prompts[2])
	}
	if !strings.HasPrefix(prompts[1], askPrompt+"\n\n## goal\n\na shaped prompt\n\n## steps\n\n[") {
		t.Fatalf("a call without the option lost the default rendering:\n%s", prompts[1])
	}
}

// TestAScopeTemplateGimbalCannotUseIsTheError: the template is the
// workflow's, so one that does not parse, and one that parses and cannot
// render the scope, are both workflow errors, reported before any model is
// called. Each template is named where it is passed: GIMBAL109 reports a
// call that reaches its template through a variable, including in a test.
func TestAScopeTemplateGimbalCannotUseIsTheError(t *testing.T) {
	turns := 0
	f := &fake{answer: func(_ context.Context, _, _ string, _ json.RawMessage, _ func(AgentEvent) error) (string, error) {
		turns++
		return "done", nil
	}}
	says := func(err error, want string) {
		t.Helper()
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Fatalf("Generate = %v, want it to say %q", err, want)
		}
	}
	says(runTest(t, bind(f, "model", "worker"), func(ctx context.Context) error {
		Set(ctx, "goal", "a shaped prompt")
		_, err := NewSession(ctx, "worker", ".").Generate[Text](ctx, askPrompt, WithScopeTemplate(unparsedScopeText))
		return err
	}), "parse the scope template")
	says(runTest(t, bind(f, "model", "worker"), func(ctx context.Context) error {
		Set(ctx, "goal", "a shaped prompt")
		_, err := NewSession(ctx, "worker", ".").Generate[Text](ctx, askPrompt, WithScopeTemplate(brokenScopeText))
		return err
	}), "render the scope template")
	if turns != 0 {
		t.Fatalf("turns = %d, want none: no prompt was ever built", turns)
	}
}
