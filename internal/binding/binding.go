// Package binding turns a model name written on a command line into the
// gimble.ModelBinding a role runs on: the harness that serves that model,
// the provider-native model id, and the reasoning effort.
package binding

import (
	"cmp"
	"fmt"
	"maps"
	"slices"
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

// Roles binds each role to the model its flag gave, or to fallback when the
// flag was left empty, sharing one binding among the roles given the same
// model so one harness serves them. A role given neither is an error naming
// its flag.
func Roles(fallback string, specs map[string]string) (map[string]gimble.ModelBinding, error) {
	bound := map[string]gimble.ModelBinding{}
	models := make(map[string]gimble.ModelBinding, len(specs))
	for _, role := range slices.Sorted(maps.Keys(specs)) {
		spec := cmp.Or(specs[role], fallback)
		if spec == "" {
			return nil, fmt.Errorf("give --%s or --model", role)
		}
		if _, ok := bound[spec]; !ok {
			b, err := Parse(spec)
			if err != nil {
				return nil, fmt.Errorf("--%s: %w", role, err)
			}
			bound[spec] = b
		}
		models[role] = bound[spec]
	}
	return models, nil
}
