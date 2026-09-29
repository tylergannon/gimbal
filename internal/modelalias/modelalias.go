// Package modelalias resolves CLI model names to provider-native model IDs
// and the Gimbal harness that serves them.
package modelalias

import (
	"fmt"
	"strconv"
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

// familyVersion is one generated model of an alias family. The generator
// lists them; this package decides which is the unversioned alias.
type familyVersion struct {
	family, provider, version, native string
}

// flashFamily is the one hand-kept family: Gemini encodes effort in the model
// ID, which the catalog generator does not model.
var flashFamily = map[string]model{
	"3.8": {alias: "flash", provider: "gemini", native: "gemini-3.8-flash-medium", version: "3.8", effort: "medium"},
	"3.7": {alias: "flash", provider: "gemini", native: "gemini-3.7-flash-medium", version: "3.7", effort: "medium"},
	"3.6": {alias: "flash", provider: "gemini", native: "gemini-3.6-flash-medium", version: "3.6", effort: "medium"},
}

// families maps an alias family to its versions; aliases maps every name a
// caller may pass, the family name meaning its newest version and
// "<family>-<version>" a specific one. Both derive from the generated catalog,
// so a new model version becomes the default by regenerating.
var families, aliases = deriveAliases()

func deriveAliases() (map[string]map[string]model, map[string]model) {
	families := map[string]map[string]model{"flash": flashFamily}
	for _, generated := range generatedFamilies {
		effort := "high"
		if generated.provider == "diffusion" {
			effort = ""
		}
		if families[generated.family] == nil {
			families[generated.family] = map[string]model{}
		}
		families[generated.family][generated.version] = model{
			alias: generated.family, provider: generated.provider, native: generated.native,
			version: generated.version, effort: effort,
		}
	}
	aliases := map[string]model{}
	for name, versions := range families {
		newest := ""
		for version := range versions {
			if newest == "" || newerVersion(version, newest) {
				newest = version
			}
		}
		aliases[name] = versions[newest]
		if name == "flash" {
			continue
		}
		for version, entry := range versions {
			aliases[name+"-"+version] = entry
		}
	}
	return families, aliases
}

// newerVersion compares dotted numeric versions such as "5.5" and "5".
func newerVersion(a, b string) bool {
	left, right := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(left) || i < len(right); i++ {
		var l, r int
		if i < len(left) {
			l, _ = strconv.Atoi(left[i])
		}
		if i < len(right) {
			r, _ = strconv.Atoi(right[i])
		}
		if l != r {
			return l > r
		}
	}
	return false
}

var harnesses = map[string]string{"openai": "codex", "anthropic": "claude", "gemini": "agy", "opencode": "opencode", "diffusion": "pi"}

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
	if after, ok := strings.CutPrefix(selection.Name, "pi/diffusion/"); ok {
		if after == "" || strings.HasPrefix(after, "/") || strings.HasSuffix(after, "/") || strings.Contains(after, "//") {
			return ResolvedSelection{}, fmt.Errorf("invalid Pi model name %q; expected pi/diffusion/<model-id>", selection.Name)
		}
		if selection.VersionPresent {
			return ResolvedSelection{}, fmt.Errorf("pi model name %q already selects a model and cannot also declare version", selection.Name)
		}
		entry = model{alias: selection.Name, provider: "diffusion", native: "diffusion/" + after}
		known = true
	}
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
		if _, isFamily := families[selection.Name]; known && !isFamily {
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
	if entry.provider != "opencode" && entry.provider != "diffusion" {
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
