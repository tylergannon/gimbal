package conversation

import (
	"fmt"
	"github.com/tylergannon/gimbal/internal/modelalias"
	"slices"
)

func selectModel(provider, model, effort string) (string, error) {
	model, _, err := providerModel(provider, model)
	if err != nil {
		return "", err
	}
	if effort == "" {
		return model, nil
	}
	if !slices.Contains(modelalias.Efforts(provider, model), effort) {
		return "", fmt.Errorf("conversation: effort %q is unavailable for %s on %s", effort, model, provider)
	}
	// Preserve native identity, including Gemini IDs with fixed effort.
	resolved, err := modelalias.Resolve(modelalias.Selection{Name: model, Effort: effort, EffortPresent: true})
	if err != nil {
		return "", err
	}
	return resolved.Model, nil
}
func harnessOptions(id, fallback string, models []string) Harness {
	efforts := make([]ModelEfforts, 0, len(models))
	for _, model := range models {
		levels := modelalias.Efforts(id, model)
		if levels == nil {
			levels = []string{}
		}
		efforts = append(efforts, ModelEfforts{Model: model, Levels: levels})
	}
	return Harness{ID: id, Default: fallback, Models: models, Efforts: efforts}
}
