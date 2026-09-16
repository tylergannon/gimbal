// Package binding turns a model name written on a command line into the
// gimble.ModelBinding a role runs on: the harness that serves that model,
// the provider-native model id, and the reasoning effort.
package binding

import (
	"fmt"
	"strings"

	"github.com/tylergannon/gimble"
	"github.com/tylergannon/gimble/agy"
	"github.com/tylergannon/gimble/claude"
	"github.com/tylergannon/gimble/codex"
	"github.com/tylergannon/gimble/internal/modelalias"
)

// Parse reads "model" or "model:effort", resolves the model through
// modelalias, and binds it to the harness that serves it. Harnesses are made
// once per call, so a caller binding several roles to one harness should
// reuse the adapter rather than call Parse twice for it.
func Parse(spec string) (gimble.ModelBinding, error) {
	name, effort, hasEffort := strings.Cut(spec, ":")
	selection := modelalias.Selection{Name: strings.TrimSpace(name)}
	if hasEffort {
		selection.Effort, selection.EffortPresent = strings.TrimSpace(effort), true
	}
	resolved, err := modelalias.Resolve(selection)
	if err != nil {
		return gimble.ModelBinding{}, fmt.Errorf("model %q: %w", spec, err)
	}
	adapter, err := Adapter(resolved.Harness)
	if err != nil {
		return gimble.ModelBinding{}, fmt.Errorf("model %q: %w", spec, err)
	}
	return gimble.ModelBinding{Adapter: adapter, Model: resolved.Model, Effort: resolved.Effort}, nil
}

// Adapter is the harness of that name.
func Adapter(harness string) (gimble.HarnessAdapter, error) {
	switch harness {
	case "agy":
		return agy.New(), nil
	case "claude":
		return claude.New(), nil
	case "codex":
		return codex.New(), nil
	default:
		return nil, fmt.Errorf("unknown harness %q", harness)
	}
}
