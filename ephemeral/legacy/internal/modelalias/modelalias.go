// Package modelalias resolves authored model selections to provider-native
// model IDs, effective effort, provider, and harness. Provider-native IDs
// remain valid without registration when their provider is recognizable.
package modelalias

import (
	"fmt"
	"strings"
)

// Selection is the provider-free model object accepted by authored workflows.
type Selection struct {
	Name           string
	Version        string
	VersionPresent bool
	Effort         string
	EffortPresent  bool
}

// ResolvedSelection is the complete routing and invocation result.
type ResolvedSelection struct {
	Name     string
	Version  string
	Model    string
	Effort   string
	Provider string
	Harness  string
}

// Model is one maintained Gimbal alias or version-bearing compatibility alias.
type Model struct {
	Alias         string
	Provider      string
	Model         string
	Version       string
	DefaultEffort string
}

var available = []Model{
	{Alias: "gpt", Provider: "openai", Model: "gpt-5.6-sol", Version: "5.6", DefaultEffort: "high"},
	{Alias: "flash", Provider: "gemini", Model: "gemini-3.8-flash-medium", Version: "3.8", DefaultEffort: "medium"},
	{Alias: "fable", Provider: "anthropic", Model: "claude-fable-5-1", Version: "5.1", DefaultEffort: "high"},
	{Alias: "fable-5.1", Provider: "anthropic", Model: "claude-fable-5-1", Version: "5.1", DefaultEffort: "high"},
	{Alias: "fable-5", Provider: "anthropic", Model: "claude-fable-5", Version: "5", DefaultEffort: "high"},
}

var familyVersions = map[string]map[string]Model{
	"gpt": {
		"5.6": {Alias: "gpt", Provider: "openai", Model: "gpt-5.6-sol", Version: "5.6", DefaultEffort: "high"},
	},
	"flash": {
		"3.8": {Alias: "flash", Provider: "gemini", Model: "gemini-3.8-flash-medium", Version: "3.8", DefaultEffort: "medium"},
		"3.7": {Alias: "flash", Provider: "gemini", Model: "gemini-3.7-flash-medium", Version: "3.7", DefaultEffort: "medium"},
		"3.6": {Alias: "flash", Provider: "gemini", Model: "gemini-3.6-flash-medium", Version: "3.6", DefaultEffort: "medium"},
	},
	"fable": {
		"5.1": {Alias: "fable", Provider: "anthropic", Model: "claude-fable-5-1", Version: "5.1", DefaultEffort: "high"},
		"5":   {Alias: "fable", Provider: "anthropic", Model: "claude-fable-5", Version: "5", DefaultEffort: "high"},
	},
}

var providerHarness = map[string]string{
	"anthropic": "claude",
	"gemini":    "agy",
	"openai":    "codex",
}

// Available returns the maintained aliases in display order.
func Available() []Model {
	return append([]Model(nil), available...)
}

// Resolve returns a maintained alias entry.
func Resolve(alias string) (Model, bool) {
	for _, candidate := range available {
		if candidate.Alias == alias {
			return candidate, true
		}
	}
	return Model{}, false
}

// ResolveModel resolves one complete authored or system selection. It performs
// no credential checks, binary launches, or provider catalog queries.
func ResolveModel(selection Selection) (ResolvedSelection, error) {
	if strings.TrimSpace(selection.Name) == "" {
		return ResolvedSelection{}, fmt.Errorf("model name must be a nonblank string")
	}
	if selection.Name != strings.TrimSpace(selection.Name) {
		return ResolvedSelection{}, fmt.Errorf("model name %q must not have surrounding whitespace", selection.Name)
	}
	if selection.VersionPresent && strings.TrimSpace(selection.Version) == "" {
		return ResolvedSelection{}, fmt.Errorf("model version must be a nonblank string")
	}
	if selection.VersionPresent && selection.Version != strings.TrimSpace(selection.Version) {
		return ResolvedSelection{}, fmt.Errorf("model version %q must not have surrounding whitespace", selection.Version)
	}
	if selection.EffortPresent && !validEffort(selection.Effort) {
		return ResolvedSelection{}, fmt.Errorf("unsupported model effort %q; expected low, medium, high, xhigh, or max", selection.Effort)
	}

	entry, knownAlias := Resolve(selection.Name)
	if selection.VersionPresent {
		if knownAlias && selection.Name != "gpt" && selection.Name != "flash" && selection.Name != "fable" {
			return ResolvedSelection{}, fmt.Errorf("model name %q already selects a version and cannot also declare version", selection.Name)
		}
		versions, family := familyVersions[selection.Name]
		if !family {
			if providerForNative(selection.Name) != "" {
				return ResolvedSelection{}, fmt.Errorf("provider-native model name %q already selects a version and cannot also declare version", selection.Name)
			}
			return ResolvedSelection{}, fmt.Errorf("model name %q does not support a separate version", selection.Name)
		}
		var supported bool
		entry, supported = versions[selection.Version]
		if !supported {
			return ResolvedSelection{}, fmt.Errorf("model %q does not support version %q", selection.Name, selection.Version)
		}
	} else if !knownAlias {
		provider := providerForNative(selection.Name)
		if provider == "" {
			return ResolvedSelection{}, fmt.Errorf("cannot determine provider for model name %q", selection.Name)
		}
		entry = Model{Alias: selection.Name, Provider: provider, Model: selection.Name, DefaultEffort: "high"}
	}

	effort := entry.DefaultEffort
	if effort == "" {
		effort = "high"
	}
	fixedEffort := nativeEffort(entry.Model)
	if fixedEffort != "" {
		effort = fixedEffort
	}
	if selection.EffortPresent {
		if fixedEffort != "" && selection.Effort != fixedEffort {
			if selection.Name == "flash" {
				entry.Model = strings.TrimSuffix(entry.Model, "-"+fixedEffort) + "-" + selection.Effort
			} else {
				return ResolvedSelection{}, fmt.Errorf("model name %q fixes effort at %q and conflicts with explicit effort %q", selection.Name, fixedEffort, selection.Effort)
			}
		}
		effort = selection.Effort
	}
	harnessName := providerHarness[entry.Provider]
	if harnessName == "" {
		return ResolvedSelection{}, fmt.Errorf("provider %q for model %q has no harness route", entry.Provider, selection.Name)
	}
	return ResolvedSelection{
		Name: selection.Name, Version: entry.Version, Model: entry.Model, Effort: effort,
		Provider: entry.Provider, Harness: harnessName,
	}, nil
}

// ResolveSelection preserves the standalone CLI compatibility API while using
// the same resolver. Authored workflows do not accept provider selections.
func ResolveSelection(provider, model string) (string, string, error) {
	resolved, err := ResolveModel(Selection{Name: model})
	if err != nil {
		return "", "", err
	}
	if provider != "" && provider != resolved.Provider {
		return "", "", fmt.Errorf("provider %q conflicts with model %q (provider %q)", provider, model, resolved.Provider)
	}
	return resolved.Provider, resolved.Model, nil
}

func validEffort(effort string) bool {
	return effort == "low" || effort == "medium" || effort == "high" || effort == "xhigh" || effort == "max"
}

func nativeEffort(model string) string {
	if !strings.HasPrefix(model, "gemini-") {
		return ""
	}
	for _, effort := range []string{"low", "medium", "high"} {
		if strings.HasSuffix(model, "-"+effort) {
			return effort
		}
	}
	return ""
}

func providerForNative(model string) string {
	switch {
	case strings.HasPrefix(model, "claude-"):
		return "anthropic"
	case strings.HasPrefix(model, "gemini-"):
		return "gemini"
	case strings.HasPrefix(model, "gpt-"), strings.HasPrefix(model, "o1"), strings.HasPrefix(model, "o3"), strings.HasPrefix(model, "o4"):
		return "openai"
	default:
		return ""
	}
}
