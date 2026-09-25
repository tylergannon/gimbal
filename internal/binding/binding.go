// Package binding turns a model name written on a command line into the
// gimbal.ModelBinding a role runs on: the harness that serves that model,
// the provider-native model id, and the reasoning effort.
package binding

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/agy"
	"github.com/tylergannon/gimbal/claude"
	"github.com/tylergannon/gimbal/codex"
	"github.com/tylergannon/gimbal/internal/modelalias"
	"github.com/tylergannon/gimbal/opencode"
)

// Parse reads "model" or "model:effort", resolves the model through
// modelalias, and binds it to the harness that serves it. Harnesses are made
// once per call, so a caller binding several roles to one harness should
// reuse the adapter rather than call Parse twice for it.
func Parse(spec string) (gimbal.ModelBinding, error) {
	resolved, err := resolve(spec)
	if err != nil {
		return gimbal.ModelBinding{}, err
	}
	adapter, err := Adapter(resolved.Harness)
	if err != nil {
		return gimbal.ModelBinding{}, fmt.Errorf("model %q: %w", spec, err)
	}
	return gimbal.ModelBinding{Adapter: adapter, Model: resolved.Model, Effort: resolved.Effort}, nil
}

func resolve(spec string) (modelalias.ResolvedSelection, error) {
	name, effort, hasEffort := strings.Cut(spec, ":")
	selection := modelalias.Selection{Name: strings.TrimSpace(name)}
	if hasEffort {
		selection.Effort, selection.EffortPresent = strings.TrimSpace(effort), true
	}
	resolved, err := modelalias.Resolve(selection)
	if err != nil {
		return modelalias.ResolvedSelection{}, fmt.Errorf("model %q: %w", spec, err)
	}
	return resolved, nil
}

// Adapter is the harness of that name.
func Adapter(harness string) (gimbal.HarnessAdapter, error) {
	switch harness {
	case "agy":
		return agy.New(), nil
	case "claude":
		return claude.New(), nil
	case "codex":
		return codex.New(), nil
	case "opencode":
		return opencode.New(), nil
	default:
		return nil, fmt.Errorf("unknown harness %q", harness)
	}
}

// Roles binds each role to the model its flag gave, sharing one binding
// among the roles given the same model so one harness serves them. A role
// given no model is an error naming its flag.
func Roles(specs map[gimbal.WorkflowRole]string) (map[gimbal.WorkflowRole]gimbal.ModelBinding, error) {
	bound := map[string]gimbal.ModelBinding{}
	var sharedOpenCode gimbal.HarnessAdapter
	models := make(map[gimbal.WorkflowRole]gimbal.ModelBinding, len(specs))
	for _, role := range slices.Sorted(maps.Keys(specs)) {
		spec := specs[role]
		if spec == "" {
			return nil, fmt.Errorf("give --%s", role)
		}
		if _, ok := bound[spec]; !ok {
			resolved, err := resolve(spec)
			if err != nil {
				return nil, fmt.Errorf("--%s: %w", role, err)
			}
			var adapter gimbal.HarnessAdapter
			if resolved.Harness == "opencode" && sharedOpenCode != nil {
				adapter = sharedOpenCode
			} else {
				adapter, err = Adapter(resolved.Harness)
				if err != nil {
					return nil, fmt.Errorf("--%s: model %q: %w", role, spec, err)
				}
				if resolved.Harness == "opencode" {
					sharedOpenCode = adapter
				}
			}
			bound[spec] = gimbal.ModelBinding{Adapter: adapter, Model: resolved.Model, Effort: resolved.Effort}
		}
		models[role] = bound[spec]
	}
	return models, nil
}
