package config

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/tylergannon/gimbal/internal/pi/model"
)

// ValidThinkingLevels is the set of thinking-level suffixes accepted in a
// model pattern.
var ValidThinkingLevels = map[model.ThinkingLevel]bool{
	model.ThinkingOff:     true,
	model.ThinkingMinimal: true,
	model.ThinkingLow:     true,
	model.ThinkingMedium:  true,
	model.ThinkingHigh:    true,
	model.ThinkingXHigh:   true,
	model.ThinkingMax:     true,
}

// ScopedModel is a resolved model with an optional explicit thinking level.
type ScopedModel struct {
	Model         *model.Model
	ThinkingLevel model.ThinkingLevel
}

// ParsedModelResult is the result of parsing one model pattern.
type ParsedModelResult struct {
	Model         *model.Model
	ThinkingLevel model.ThinkingLevel
	Warning       string
}

// FindExactModelReferenceMatch finds a unique exact match for a bare model id
// or a canonical provider/modelId reference. Ambiguous matches return nil.
func FindExactModelReferenceMatch(modelReference string, availableModels []*model.Model) *model.Model {
	trimmed := strings.TrimSpace(modelReference)
	if trimmed == "" {
		return nil
	}
	lower := strings.ToLower(trimmed)

	canonical := filterModels(availableModels, func(m *model.Model) bool {
		return strings.ToLower(m.Provider+"/"+m.ID) == lower
	})
	if len(canonical) == 1 {
		return canonical[0]
	}
	if len(canonical) > 1 {
		return nil
	}

	if before, after, ok := strings.Cut(trimmed, "/"); ok {
		provider := strings.TrimSpace(before)
		modelID := strings.TrimSpace(after)
		if provider != "" && modelID != "" {
			byProvider := filterModels(availableModels, func(m *model.Model) bool {
				return strings.EqualFold(m.Provider, provider) && strings.EqualFold(m.ID, modelID)
			})
			if len(byProvider) == 1 {
				return byProvider[0]
			}
			if len(byProvider) > 1 {
				return nil
			}
		}
	}

	byID := filterModels(availableModels, func(m *model.Model) bool {
		return strings.ToLower(m.ID) == lower
	})
	if len(byID) == 1 {
		return byID[0]
	}
	return nil
}

func filterModels(models []*model.Model, keep func(*model.Model) bool) []*model.Model {
	var out []*model.Model
	for _, entry := range models {
		if entry != nil && keep(entry) {
			out = append(out, entry)
		}
	}
	return out
}

var modelDatePattern = regexp.MustCompile(`-\d{8}$`)

func isModelAlias(id string) bool {
	if strings.HasSuffix(id, "-latest") {
		return true
	}
	return !modelDatePattern.MatchString(id)
}

func tryMatchModel(pattern string, availableModels []*model.Model) *model.Model {
	if exact := FindExactModelReferenceMatch(pattern, availableModels); exact != nil {
		return exact
	}
	lower := strings.ToLower(pattern)
	matches := filterModels(availableModels, func(m *model.Model) bool {
		return strings.Contains(strings.ToLower(m.ID), lower) || strings.Contains(strings.ToLower(m.Name), lower)
	})
	if len(matches) == 0 {
		return nil
	}
	var aliases, dated []*model.Model
	for _, entry := range matches {
		if isModelAlias(entry.ID) {
			aliases = append(aliases, entry)
		} else {
			dated = append(dated, entry)
		}
	}
	if len(aliases) > 0 {
		sort.Slice(aliases, func(i, j int) bool { return aliases[i].ID > aliases[j].ID })
		return aliases[0]
	}
	sort.Slice(dated, func(i, j int) bool { return dated[i].ID > dated[j].ID })
	return dated[0]
}

// ParseModelPatternOptions controls invalid-thinking-level handling.
type ParseModelPatternOptions struct {
	// AllowInvalidThinkingLevelFallback defaults to true. When false (CLI
	// parsing), an invalid suffix makes the pattern fail rather than resolve
	// to a different model.
	AllowInvalidThinkingLevelFallback *bool
}

func allowInvalidFallback(options *ParseModelPatternOptions) bool {
	if options == nil || options.AllowInvalidThinkingLevelFallback == nil {
		return true
	}
	return *options.AllowInvalidThinkingLevelFallback
}

// ParseModelPattern parses "model:thinking" patterns, tolerating colons inside
// model ids by trying the full pattern first and then stripping trailing
// suffixes.
func ParseModelPattern(pattern string, availableModels []*model.Model, options *ParseModelPatternOptions) ParsedModelResult {
	if exact := tryMatchModel(pattern, availableModels); exact != nil {
		return ParsedModelResult{Model: exact}
	}
	before, after, ok := strings.CutLast(pattern, ":")
	if !ok {
		return ParsedModelResult{}
	}
	prefix := before
	suffix := after
	if ValidThinkingLevels[model.ThinkingLevel(suffix)] {
		result := ParseModelPattern(prefix, availableModels, options)
		if result.Model == nil {
			return result
		}
		level := model.ThinkingLevel(suffix)
		if result.Warning != "" {
			level = ""
		}
		return ParsedModelResult{Model: result.Model, ThinkingLevel: level, Warning: result.Warning}
	}
	if !allowInvalidFallback(options) {
		return ParsedModelResult{}
	}
	result := ParseModelPattern(prefix, availableModels, options)
	if result.Model != nil {
		return ParsedModelResult{
			Model:   result.Model,
			Warning: fmt.Sprintf("Invalid thinking level %q in pattern %q. Using default instead.", suffix, pattern),
		}
	}
	return result
}

// ModelScopeDiagnostic is a non-fatal model-scope resolution warning.
type ModelScopeDiagnostic struct {
	Type    string
	Code    string
	Message string
	Pattern string
}

// ResolveModelScopeResult is the outcome of resolving a list of patterns.
type ResolveModelScopeResult struct {
	ScopedModels []ScopedModel
	Diagnostics  []ModelScopeDiagnostic
}

// ResolveModelScopeFromModels resolves patterns to models with optional
// thinking levels, preferring aliases and skipping duplicates.
func ResolveModelScopeFromModels(patterns []string, models []*model.Model) ResolveModelScopeResult {
	availableModels := append([]*model.Model(nil), models...)
	result := ResolveModelScopeResult{ScopedModels: []ScopedModel{}}

	has := func(candidate *model.Model) bool {
		for _, scoped := range result.ScopedModels {
			if modelsAreEqual(scoped.Model, candidate) {
				return true
			}
		}
		return false
	}

	for _, pattern := range patterns {
		if strings.ContainsAny(pattern, "*?[") {
			globPattern := pattern
			var thinkingLevel model.ThinkingLevel
			if before, after, ok := strings.CutLast(pattern, ":"); ok {
				suffix := after
				if ValidThinkingLevels[model.ThinkingLevel(suffix)] {
					thinkingLevel = model.ThinkingLevel(suffix)
					globPattern = before
				}
			}
			if exact := FindExactModelReferenceMatch(globPattern, availableModels); exact != nil {
				if !has(exact) {
					result.ScopedModels = append(result.ScopedModels, ScopedModel{Model: exact, ThinkingLevel: thinkingLevel})
				}
				continue
			}
			matching := filterModels(availableModels, func(m *model.Model) bool {
				return globMatch(globPattern, m.Provider+"/"+m.ID) || globMatch(globPattern, m.ID)
			})
			if len(matching) == 0 {
				result.Diagnostics = append(result.Diagnostics, ModelScopeDiagnostic{
					Type: "warning", Code: "no-match", Message: fmt.Sprintf("No models match pattern %q", pattern), Pattern: pattern,
				})
				continue
			}
			for _, entry := range matching {
				if !has(entry) {
					result.ScopedModels = append(result.ScopedModels, ScopedModel{Model: entry, ThinkingLevel: thinkingLevel})
				}
			}
			continue
		}

		parsed := ParseModelPattern(pattern, availableModels, nil)
		if parsed.Warning != "" {
			result.Diagnostics = append(result.Diagnostics, ModelScopeDiagnostic{
				Type: "warning", Code: "invalid-thinking-level", Message: parsed.Warning, Pattern: pattern,
			})
		}
		if parsed.Model == nil {
			result.Diagnostics = append(result.Diagnostics, ModelScopeDiagnostic{
				Type: "warning", Code: "no-match", Message: fmt.Sprintf("No models match pattern %q", pattern), Pattern: pattern,
			})
			continue
		}
		if !has(parsed.Model) {
			result.ScopedModels = append(result.ScopedModels, ScopedModel{Model: parsed.Model, ThinkingLevel: parsed.ThinkingLevel})
		}
	}
	return result
}

// globMatch reports whether name matches a shell-glob pattern. Matching is
// case-insensitive and * does not cross a path separator.
func globMatch(pattern, name string) bool {
	re, err := regexp.Compile("^" + globToRegex(pattern) + "$")
	if err != nil {
		return false
	}
	return re.MatchString(strings.ToLower(name))
}

func globToRegex(pattern string) string {
	var out strings.Builder
	for i := 0; i < len(pattern); i++ {
		c := pattern[i]
		switch c {
		case '*':
			out.WriteString("[^/]*")
		case '?':
			out.WriteString("[^/]")
		case '[':
			end := strings.IndexByte(pattern[i:], ']')
			if end == -1 {
				out.WriteString(`\[`)
				continue
			}
			class := pattern[i : i+end+1]
			out.WriteString(class)
			i += end
		default:
			out.WriteString(regexp.QuoteMeta(strings.ToLower(string(c))))
		}
	}
	return strings.ToLower(out.String())
}

// ResolveCLIModelOptions selects a single model from CLI-style flags.
type ResolveCLIModelOptions struct {
	CLIProvider       string
	CLIModel          string
	CLIThinking       model.ThinkingLevel
	Models            []*model.Model
	HasConfiguredAuth func(provider string) bool
}

// ResolveCLIModelResult is the outcome of resolving one CLI model.
type ResolveCLIModelResult struct {
	Model         *model.Model
	ThinkingLevel model.ThinkingLevel
	Warning       string
	Error         string
}

func (o ResolveCLIModelOptions) hasAuth(provider string) bool {
	return o.HasConfiguredAuth != nil && o.HasConfiguredAuth(provider)
}

// ResolveCLIModel resolves a model from --provider/--model flags with fuzzy
// matching and custom-id fallback.
func ResolveCLIModel(options ResolveCLIModelOptions) ResolveCLIModelResult {
	availableModels := append([]*model.Model(nil), options.Models...)
	if len(availableModels) == 0 {
		return ResolveCLIModelResult{Error: "No models available. Check your installation or add models to models.json."}
	}
	if options.CLIModel == "" {
		return ResolveCLIModelResult{}
	}

	providerByLower := map[string]string{}
	for _, m := range availableModels {
		providerByLower[strings.ToLower(m.Provider)] = m.Provider
	}

	provider := ""
	if options.CLIProvider != "" {
		canonical, ok := providerByLower[strings.ToLower(options.CLIProvider)]
		if !ok {
			return ResolveCLIModelResult{Error: fmt.Sprintf("Unknown provider %q. Use --list-models to see available providers/models.", options.CLIProvider)}
		}
		provider = canonical
	}

	pattern := options.CLIModel
	inferredProvider := false
	if provider == "" {
		if slash := strings.Index(options.CLIModel, "/"); slash != -1 {
			if canonical, ok := providerByLower[strings.ToLower(options.CLIModel[:slash])]; ok {
				provider = canonical
				pattern = options.CLIModel[slash+1:]
				inferredProvider = true
			}
		}
	}

	if provider == "" {
		lower := strings.ToLower(options.CLIModel)
		exactMatches := filterModels(availableModels, func(m *model.Model) bool {
			return strings.ToLower(m.ID) == lower || strings.ToLower(m.Provider+"/"+m.ID) == lower
		})
		if len(exactMatches) == 1 {
			return ResolveCLIModelResult{Model: exactMatches[0]}
		}
		if len(exactMatches) > 1 {
			var authenticated []*model.Model
			for _, m := range exactMatches {
				if options.hasAuth(m.Provider) {
					authenticated = append(authenticated, m)
				}
			}
			if len(authenticated) == 1 {
				return ResolveCLIModelResult{Model: authenticated[0]}
			}
			matches := make([]string, 0, len(exactMatches))
			for _, m := range exactMatches {
				matches = append(matches, m.Provider+"/"+m.ID)
			}
			sort.Strings(matches)
			hint := "No matching provider is authenticated."
			if len(authenticated) > 1 {
				hint = "More than one matching provider is authenticated."
			}
			return ResolveCLIModelResult{Error: fmt.Sprintf("Model %q is ambiguous across providers: %s. %s Use --provider or provider/model.", options.CLIModel, strings.Join(matches, ", "), hint)}
		}
	}

	if options.CLIProvider != "" && provider != "" {
		prefix := provider + "/"
		if strings.HasPrefix(strings.ToLower(options.CLIModel), strings.ToLower(prefix)) {
			pattern = options.CLIModel[len(prefix):]
		}
	}

	candidates := availableModels
	if provider != "" {
		candidates = filterModels(availableModels, func(m *model.Model) bool { return m.Provider == provider })
	}
	allowInvalid := false
	parsed := ParseModelPattern(pattern, candidates, &ParseModelPatternOptions{AllowInvalidThinkingLevelFallback: &allowInvalid})
	if parsed.Model != nil {
		if inferredProvider {
			rawExactMatches := filterModels(availableModels, func(m *model.Model) bool {
				return strings.EqualFold(m.ID, options.CLIModel) && !modelsAreEqual(m, parsed.Model)
			})
			if len(rawExactMatches) > 0 && !options.hasAuth(parsed.Model.Provider) {
				var authenticatedRaw []*model.Model
				for _, m := range rawExactMatches {
					if options.hasAuth(m.Provider) {
						authenticatedRaw = append(authenticatedRaw, m)
					}
				}
				if len(authenticatedRaw) == 1 {
					return ResolveCLIModelResult{Model: authenticatedRaw[0]}
				}
			}
		}
		return ResolveCLIModelResult{Model: parsed.Model, ThinkingLevel: parsed.ThinkingLevel, Warning: parsed.Warning}
	}

	if inferredProvider {
		lower := strings.ToLower(options.CLIModel)
		for _, m := range availableModels {
			if strings.ToLower(m.ID) == lower || strings.ToLower(m.Provider+"/"+m.ID) == lower {
				return ResolveCLIModelResult{Model: m}
			}
		}
		fallback := ParseModelPattern(options.CLIModel, availableModels, &ParseModelPatternOptions{AllowInvalidThinkingLevelFallback: &allowInvalid})
		if fallback.Model != nil {
			return ResolveCLIModelResult{Model: fallback.Model, ThinkingLevel: fallback.ThinkingLevel, Warning: fallback.Warning}
		}
	}

	if provider != "" {
		fallbackPattern := pattern
		var fallbackThinking model.ThinkingLevel
		if options.CLIThinking == "" {
			if colon := strings.LastIndex(pattern, ":"); colon != -1 {
				suffix := model.ThinkingLevel(pattern[colon+1:])
				if ValidThinkingLevels[suffix] {
					fallbackPattern = pattern[:colon]
					fallbackThinking = suffix
				}
			}
		}
		if fallback := buildFallbackModel(provider, fallbackPattern, availableModels); fallback != nil {
			requested := options.CLIThinking
			if requested == "" {
				requested = fallbackThinking
			}
			if requested != "" && requested != model.ThinkingOff {
				fallback.Reasoning = true
			}
			warning := fmt.Sprintf("Model %q not found for provider %q. Using custom model id.", fallbackPattern, provider)
			if parsed.Warning != "" {
				warning = parsed.Warning + " " + warning
			}
			return ResolveCLIModelResult{Model: fallback, ThinkingLevel: fallbackThinking, Warning: warning}
		}
	}

	display := options.CLIModel
	if provider != "" {
		display = provider + "/" + pattern
	}
	return ResolveCLIModelResult{Warning: parsed.Warning, Error: fmt.Sprintf("Model %q not found. Use --list-models to see available models.", display)}
}

func buildFallbackModel(provider, modelID string, availableModels []*model.Model) *model.Model {
	providerModels := filterModels(availableModels, func(m *model.Model) bool { return m.Provider == provider })
	if len(providerModels) == 0 {
		return nil
	}
	base := providerModels[0]
	clone := base.Clone()
	clone.ID = modelID
	clone.Name = modelID
	return &clone
}

func modelsAreEqual(a, b *model.Model) bool {
	return a != nil && b != nil && a.Provider == b.Provider && a.ID == b.ID
}

// InitialModelResult is the model selected for a new session.
type InitialModelResult struct {
	Model           *model.Model
	ThinkingLevel   model.ThinkingLevel
	FallbackMessage string
	Error           string
}

// FindInitialModelOptions selects the model a session starts with.
type FindInitialModelOptions struct {
	CLIProvider          string
	CLIModel             string
	ScopedModels         []ScopedModel
	IsContinuing         bool
	DefaultProvider      string
	DefaultModelID       string
	DefaultThinkingLevel model.ThinkingLevel
	ModelThinkingLevels  map[string]model.ThinkingLevel
	AvailableModels      []*model.Model
	GetModel             func(provider, modelID string) *model.Model
	HasConfiguredAuth    func(provider string) bool
}

func (o FindInitialModelOptions) hasAuth(provider string) bool {
	return o.HasConfiguredAuth != nil && o.HasConfiguredAuth(provider)
}

func (o FindInitialModelOptions) perModelThinking(model *model.Model) model.ThinkingLevel {
	if o.ModelThinkingLevels == nil || model == nil {
		return ""
	}
	return o.ModelThinkingLevels[model.Provider+"/"+model.ID]
}

// FindInitialModel applies pi's initial-model priority: CLI flags, then the
// first scoped model, then a saved default with configured auth, then the
// first available model.
func FindInitialModel(options FindInitialModelOptions) InitialModelResult {
	if options.CLIProvider != "" && options.CLIModel != "" {
		resolved := ResolveCLIModel(ResolveCLIModelOptions{
			CLIProvider:       options.CLIProvider,
			CLIModel:          options.CLIModel,
			Models:            options.AvailableModels,
			HasConfiguredAuth: options.HasConfiguredAuth,
		})
		if resolved.Error != "" {
			return InitialModelResult{Error: resolved.Error}
		}
		if resolved.Model != nil {
			return InitialModelResult{Model: resolved.Model, ThinkingLevel: model.DefaultThinkingLevel}
		}
	}

	if len(options.ScopedModels) > 0 && !options.IsContinuing {
		scoped := options.ScopedModels[0]
		level := scoped.ThinkingLevel
		if level == "" {
			level = options.perModelThinking(scoped.Model)
		}
		if level == "" {
			level = options.DefaultThinkingLevel
		}
		if level == "" {
			level = model.DefaultThinkingLevel
		}
		return InitialModelResult{Model: scoped.Model, ThinkingLevel: level}
	}

	if options.DefaultProvider != "" && options.DefaultModelID != "" && options.GetModel != nil {
		if found := options.GetModel(options.DefaultProvider, options.DefaultModelID); found != nil && options.hasAuth(found.Provider) {
			level := options.perModelThinking(found)
			if level == "" {
				level = options.DefaultThinkingLevel
			}
			if level == "" {
				level = model.DefaultThinkingLevel
			}
			return InitialModelResult{Model: found, ThinkingLevel: level}
		}
	}

	if len(options.AvailableModels) > 0 {
		return InitialModelResult{Model: options.AvailableModels[0], ThinkingLevel: model.DefaultThinkingLevel}
	}

	return InitialModelResult{ThinkingLevel: model.DefaultThinkingLevel}
}

// RestoreModelFromSession restores a saved model when it still exists and has
// configured auth, falling back to the current model or the first available
// model.
func RestoreModelFromSession(savedProvider, savedModelID string, current *model.Model, availableModels []*model.Model, getModel func(provider, modelID string) *model.Model, hasAuth func(provider string) bool) (restored *model.Model, fallbackMessage string) {
	var saved *model.Model
	if getModel != nil {
		saved = getModel(savedProvider, savedModelID)
	}
	configured := saved != nil && hasAuth != nil && hasAuth(saved.Provider)
	if configured {
		return saved, ""
	}
	reason := "model no longer exists"
	if saved != nil {
		reason = "no auth configured"
	}
	if current != nil {
		return current, fmt.Sprintf("Could not restore model %s/%s (%s). Using %s/%s.", savedProvider, savedModelID, reason, current.Provider, current.ID)
	}
	if len(availableModels) > 0 {
		return availableModels[0], fmt.Sprintf("Could not restore model %s/%s (%s). Using %s/%s.", savedProvider, savedModelID, reason, availableModels[0].Provider, availableModels[0].ID)
	}
	return nil, ""
}
