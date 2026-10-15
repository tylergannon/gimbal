package conversation

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// Offered levels follow the adapter's vocabulary and the chosen model family.
// Empty always means the harness default, including native effort-suffixed IDs.
func modelEfforts(provider, model string) []string {
	switch provider {
	case "codex":
		if strings.HasPrefix(model, "gpt-6") || strings.HasPrefix(model, "gpt-5.2") || strings.HasPrefix(model, "gpt-5.3") || strings.HasPrefix(model, "gpt-5.4") || strings.HasPrefix(model, "gpt-5.5") || strings.HasPrefix(model, "gpt-5.6") || strings.Contains(model, "codex-max") {
			return []string{"low", "medium", "high", "xhigh"}
		}
		if strings.HasPrefix(model, "gpt-5") || strings.HasPrefix(model, "o3") || strings.HasPrefix(model, "o4") {
			return []string{"low", "medium", "high"}
		}
	case "claude":
		parts := strings.Split(model, "-")
		if len(parts) < 3 || (parts[1] != "opus" && parts[1] != "sonnet") {
			return []string{}
		}
		major, _ := strconv.Atoi(parts[2])
		minor := 0
		if len(parts) > 3 {
			minor, _ = strconv.Atoi(parts[3])
		}
		if major < 4 || (major == 4 && (minor < 5 || (parts[1] == "sonnet" && minor < 6))) {
			return []string{}
		}
		levels := []string{"low", "medium", "high"}
		// Claude SDK: max supports Opus 4.6+/Sonnet 4.6+, xhigh Opus 4.7+.
		if parts[1] == "opus" && (major > 4 || minor >= 7) {
			levels = append(levels, "xhigh")
		}
		if major > 4 || minor >= 6 {
			levels = append(levels, "max")
		}
		return levels
	case "agy":
		return []string{"low", "medium", "high"}
	}
	// Pi rejects explicit effort; OpenCode needs actual model variant metadata.
	return []string{}
}
func selectModel(provider, model, effort string) (string, error) {
	model, _, err := providerModel(provider, model)
	if err != nil {
		return "", err
	}
	if effort == "" {
		return model, nil
	}
	if !slices.Contains(modelEfforts(provider, model), effort) {
		return "", fmt.Errorf("conversation: effort %q is unavailable for %s on %s", effort, model, provider)
	}
	if provider == "agy" {
		original := model
		for _, level := range []string{"low", "medium", "high"} {
			model = strings.TrimSuffix(model, "-"+level)
		}
		// Other native IDs pass effort separately to the adapter.
		if model != original {
			model += "-" + effort
		}
	}
	return model, nil
}
func harnessOptions(id, fallback string, models []string) Harness {
	efforts := make([]ModelEfforts, 0, len(models))
	for _, model := range models {
		efforts = append(efforts, ModelEfforts{Model: model, Levels: modelEfforts(id, model)})
		if id == "agy" {
			for _, level := range modelEfforts(id, model) {
				normalized, _ := selectModel(id, model, level)
				if normalized != model && !slices.ContainsFunc(efforts, func(item ModelEfforts) bool { return item.Model == normalized }) {
					efforts = append(efforts, ModelEfforts{Model: normalized, Levels: modelEfforts(id, normalized)})
				}
			}
		}
	}
	return Harness{ID: id, Default: fallback, Models: models, Efforts: efforts}
}
