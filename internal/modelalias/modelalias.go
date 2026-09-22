// Package modelalias resolves CLI model names to provider-native model IDs
// and the Gimble harness that serves them.
package modelalias

import (
	"fmt"
	"strings"
)

type Selection struct {
	Name           string
	Version        string
	VersionPresent bool
	Effort         string
	EffortPresent  bool
}

type ResolvedSelection struct {
	Name     string
	Version  string
	Model    string
	Effort   string
	Provider string
	Harness  string
}

type model struct {
	alias, provider, native, version, effort string
}

var aliases = map[string]model{
	"gpt":       {alias: "gpt", provider: "openai", native: "gpt-6-sol", version: "6", effort: "high"},
	"flash":     {alias: "flash", provider: "gemini", native: "gemini-3.8-flash-medium", version: "3.8", effort: "medium"},
	"fable":     {alias: "fable", provider: "anthropic", native: "claude-fable-5-1", version: "5.1", effort: "high"},
	"fable-5.1": {alias: "fable-5.1", provider: "anthropic", native: "claude-fable-5-1", version: "5.1", effort: "high"},
	"fable-5":   {alias: "fable-5", provider: "anthropic", native: "claude-fable-5", version: "5", effort: "high"},
	"opus":      {alias: "opus", provider: "anthropic", native: "claude-opus-5-5", version: "5.5", effort: "high"},
	"opus-5.5":  {alias: "opus-5.5", provider: "anthropic", native: "claude-opus-5-5", version: "5.5", effort: "high"},
	"opus-5":    {alias: "opus-5", provider: "anthropic", native: "claude-opus-5", version: "5", effort: "high"},
}

var families = map[string]map[string]model{
	"gpt": {
		"6":   {alias: "gpt", provider: "openai", native: "gpt-6-sol", version: "6", effort: "high"},
		"5.6": {alias: "gpt", provider: "openai", native: "gpt-5.6-sol", version: "5.6", effort: "high"},
	},
	"flash": {
		"3.8": {alias: "flash", provider: "gemini", native: "gemini-3.8-flash-medium", version: "3.8", effort: "medium"},
		"3.7": {alias: "flash", provider: "gemini", native: "gemini-3.7-flash-medium", version: "3.7", effort: "medium"},
		"3.6": {alias: "flash", provider: "gemini", native: "gemini-3.6-flash-medium", version: "3.6", effort: "medium"},
	},
	"fable": {
		"5.1": {alias: "fable", provider: "anthropic", native: "claude-fable-5-1", version: "5.1", effort: "high"},
		"5":   {alias: "fable", provider: "anthropic", native: "claude-fable-5", version: "5", effort: "high"},
	},
	"opus": {
		"5.5": {alias: "opus", provider: "anthropic", native: "claude-opus-5-5", version: "5.5", effort: "high"},
		"5":   {alias: "opus", provider: "anthropic", native: "claude-opus-5", version: "5", effort: "high"},
	},
}

var harnesses = map[string]string{"openai": "codex", "anthropic": "claude", "gemini": "agy", "opencode": "opencode"}

func Resolve(selection Selection) (ResolvedSelection, error) {
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

	entry, known := aliases[selection.Name]
	if after, ok := strings.CutPrefix(selection.Name, "opencode/"); ok {
		modelID := after
		if modelID == "" || strings.HasPrefix(modelID, "/") || strings.HasSuffix(modelID, "/") || strings.Contains(modelID, "//") {
			return ResolvedSelection{}, fmt.Errorf("invalid OpenCode model name %q; expected opencode/<model-id> or opencode/<provider>/<model-id>", selection.Name)
		}
		if selection.VersionPresent {
			return ResolvedSelection{}, fmt.Errorf("OpenCode model name %q already selects a model and cannot also declare version", selection.Name)
		}
		entry = model{alias: selection.Name, provider: "opencode", native: modelID}
		known = true
	}
	if selection.VersionPresent {
		if known && selection.Name != "gpt" && selection.Name != "flash" && selection.Name != "fable" && selection.Name != "opus" {
			return ResolvedSelection{}, fmt.Errorf("model name %q already selects a version and cannot also declare version", selection.Name)
		}
		versions, ok := families[selection.Name]
		if !ok {
			if providerForNative(selection.Name) != "" {
				return ResolvedSelection{}, fmt.Errorf("provider-native model name %q already selects a version and cannot also declare version", selection.Name)
			}
			return ResolvedSelection{}, fmt.Errorf("model name %q does not support a separate version", selection.Name)
		}
		entry, ok = versions[selection.Version]
		if !ok {
			return ResolvedSelection{}, fmt.Errorf("model %q does not support version %q", selection.Name, selection.Version)
		}
	} else if !known {
		provider := providerForNative(selection.Name)
		if provider == "" {
			return ResolvedSelection{}, fmt.Errorf("cannot determine provider for model name %q", selection.Name)
		}
		entry = model{alias: selection.Name, provider: provider, native: selection.Name, effort: "high"}
	}

	effort := entry.effort
	fixedEffort := ""
	if entry.provider != "opencode" {
		fixedEffort = nativeEffort(entry.native)
	}
	if fixedEffort != "" {
		effort = fixedEffort
	}
	if selection.EffortPresent {
		if fixedEffort != "" && fixedEffort != selection.Effort {
			if selection.Name != "flash" {
				return ResolvedSelection{}, fmt.Errorf("model name %q fixes effort at %q and conflicts with explicit effort %q", selection.Name, fixedEffort, selection.Effort)
			}
			entry.native = strings.TrimSuffix(entry.native, "-"+fixedEffort) + "-" + selection.Effort
		}
		effort = selection.Effort
	}
	return ResolvedSelection{
		Name: selection.Name, Version: entry.version, Model: entry.native, Effort: effort,
		Provider: entry.provider, Harness: harnesses[entry.provider],
	}, nil
}

func validEffort(value string) bool {
	return value == "low" || value == "medium" || value == "high" || value == "xhigh" || value == "max"
}

func nativeEffort(value string) string {
	if strings.HasPrefix(value, "gemini-") {
		for _, effort := range []string{"low", "medium", "high"} {
			if strings.HasSuffix(value, "-"+effort) {
				return effort
			}
		}
	}
	return ""
}

func providerForNative(value string) string {
	switch {
	case strings.HasPrefix(value, "claude-"):
		return "anthropic"
	case strings.HasPrefix(value, "gemini-"):
		return "gemini"
	case strings.HasPrefix(value, "gpt-"), strings.HasPrefix(value, "o1"), strings.HasPrefix(value, "o3"), strings.HasPrefix(value, "o4"):
		return "openai"
	default:
		return ""
	}
}
