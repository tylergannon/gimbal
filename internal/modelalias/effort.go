package modelalias

import "slices"

// Efforts offers supported explicit choices for catalogued native models.
// Unknown model capabilities, Pi and OpenCode variants stay at harness Default.
// This belongs beside alias/native selection, not in a second UI model catalog.
func Efforts(harness, native string) []string {
	if harness == "agy" {
		if fixed := nativeEffort(native); fixed != "" {
			return []string{fixed}
		}
		return nil
	}
	for family, versions := range families {
		for version, model := range versions {
			if model.native != native || harnesses[model.provider] != harness {
				continue
			}
			switch harness {
			case "codex":
				// The current catalog's reasoning families share these levels. Keep
				// optional provider-specific levels out until their support is known.
				if family == "luna" || family == "sol" || family == "terra" || family == "astra" || family == "gpt" {
					return []string{"low", "medium", "high", "xhigh"}
				}
			case "claude":
				// https://platform.claude.com/docs/en/build-with-claude/effort
				// Older Haiku and Sonnet versions do not accept the effort parameter.
				minimum := "4.6"
				if family == "haiku" {
					minimum = "5.5"
				}
				if family == "fable" {
					minimum = "5"
				}
				if !slices.Contains([]string{"opus", "sonnet", "haiku", "fable"}, family) || newerVersion(minimum, version) {
					return nil
				}
				levels := []string{"low", "medium", "high"}
				if (family == "opus" && !newerVersion("4.7", version)) || !newerVersion("5", version) {
					levels = append(levels, "xhigh")
				}
				return append(levels, "max")
			}
		}
	}
	return nil
}
