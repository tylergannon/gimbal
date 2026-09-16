package gimble

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"text/template"
)

// The template of a shaped call is parsed once, at package level, from text
// that is readable in the source: a constant here, and in a workflow whose
// template is long, a file embedded from disk.
const shapedScopeText = `The goal is {{.By.goal.Value}}, and the steps are {{(index .By "steps").Text}}.
{{range .Values}}
[{{.Key}}]{{end}}`

var shapedScope = template.Must(template.New("shaped scope").Parse(shapedScopeText))

const brokenScopeText = `{{.Values.NotAField}}`

var brokenScope = template.Must(template.New("broken scope").Parse(brokenScopeText))

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
		if _, err := worker.Generate[Text](ctx, askPrompt, WithScopeTemplate(shapedScope)); err != nil {
			return err
		}
		if _, err := worker.Generate[Text](ctx, askPrompt); err != nil {
			return err
		}
		watcher := NewSession(ctx, "watcher", ".")
		_, err := worker.Generate[Text](ctx, askPrompt, WithScopeTemplate(shapedScope), WithSupervisor(watcher, watchInstruction))
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

// TestAScopeTemplateThatCannotRenderIsTheError: the template is the
// workflow's, so a template that cannot render the scope is a workflow
// error, reported before any model is called.
func TestAScopeTemplateThatCannotRenderIsTheError(t *testing.T) {
	turns := 0
	f := &fake{answer: func(_ context.Context, _, _ string, _ json.RawMessage, _ func(AgentEvent) error) (string, error) {
		turns++
		return "done", nil
	}}
	err := runTest(t, bind(f, "model", "worker"), func(ctx context.Context) error {
		Set(ctx, "goal", "a shaped prompt")
		_, err := NewSession(ctx, "worker", ".").Generate[Text](ctx, askPrompt, WithScopeTemplate(brokenScope))
		return err
	})
	if err == nil || !strings.Contains(err.Error(), "broken scope") {
		t.Fatalf("Generate = %v, want the template named in the error", err)
	}
	if turns != 0 {
		t.Fatalf("turns = %d, want none: the prompt was never built", turns)
	}
}
