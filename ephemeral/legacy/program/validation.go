package program

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/tylergannon/gimbal/checklist"
)

// Validation is the typed verdict shared by item validation and goal evaluation.
type Validation struct {
	Passed bool   `json:"passed"`
	Notes  string `json:"notes"`
}

var validationSchema = map[string]any{"type": "object", "properties": map[string]any{
	"passed": map[string]any{"type": "boolean"}, "notes": map[string]any{"type": "string"}},
	"required": []string{"passed", "notes"}, "additionalProperties": false}

// Validate runs both configured gates. A failed command remains a semantic
// result and never prevents a configured inference judge from inspecting evidence.
func (r *Runtime) Validate(ctx context.Context, item checklist.Item) (Validation, error) {
	if strings.TrimSpace(item.Command) == "" && item.Infer == nil {
		return Validation{}, fmt.Errorf("item %q has neither command nor inference validation", item.Name)
	}
	command := CommandResult{}
	var err error
	if strings.TrimSpace(item.Command) != "" {
		command, err = r.Command(ctx, item.Command)
		if err != nil {
			return Validation{}, err
		}
	}
	if item.Infer == nil {
		return Validation{Passed: command.ExitCode == 0, Notes: strings.TrimSpace(command.Output)}, nil
	}
	files, missing, err := r.matchEvidence(item.Infer.Files)
	if err != nil {
		return Validation{}, err
	}
	prompt := fmt.Sprintf("Judge this checklist item from the actual workspace evidence. Inspect every listed file. Return passed only when the check and inference prompt are satisfied.\n\nitem: %s\ncheck: %s\ncommand exit: %d\ncommand output:\n%s\n\ninference: %s\n\nevidence files:\n%s", item.Name, item.Check, command.ExitCode, command.Output, item.Infer.Prompt, strings.Join(files, "\n"))
	if len(missing) > 0 {
		prompt += "\n\nUnmatched evidence patterns:\n" + strings.Join(missing, "\n")
	}
	judge, err := Codergen[Validation](ctx, r, CodergenRequest{Name: "validate-" + item.Name, Prompt: prompt, JSONSchema: validationSchema, Model: r.DefaultJudgeModel})
	if err != nil {
		return Validation{}, err
	}
	judge.Passed = judge.Passed && command.ExitCode == 0 && len(missing) == 0
	return judge, nil
}

func (r *Runtime) matchEvidence(patterns []string) ([]string, []string, error) {
	var files, missing []string
	for _, pattern := range patterns {
		matches, err := doublestar.FilepathGlob(filepath.Join(r.Workdir, pattern))
		if err != nil {
			return nil, nil, fmt.Errorf("evidence pattern %q: %w", pattern, err)
		}
		if len(matches) == 0 {
			missing = append(missing, pattern)
			continue
		}
		files = append(files, matches...)
	}
	return files, missing, nil
}
