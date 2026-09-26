package resources

import (
	"context"
	"os"
	"path/filepath"

	"github.com/tylergannon/gimbal/internal/pi/config"
)

// ExtensionPath is one resource path supplied by an extension.
type ExtensionPath struct {
	Path     string
	Metadata PathMetadata
}

// ResourceExtensionPaths are resource paths added after a load.
type ResourceExtensionPaths struct {
	SkillPaths  []ExtensionPath
	PromptPaths []ExtensionPath
}

// ResourceLoaderReloadOptions configures Reload.
type ResourceLoaderReloadOptions struct {
	// ResolveProjectTrust decides project trust before the trusted reload.
	ResolveProjectTrust func(ctx context.Context) (bool, error)
}

// DefaultResourceLoaderOptions configures DefaultResourceLoader.
type DefaultResourceLoaderOptions struct {
	Cwd             string
	AgentDir        string
	SettingsManager *config.SettingsManager

	AdditionalSkillPaths          []string
	AdditionalPromptTemplatePaths []string

	NoSkills          bool
	NoPromptTemplates bool
	NoContextFiles    bool

	SystemPrompt       string
	AppendSystemPrompt []string

	SkillsOverride             func(LoadSkillsResult) LoadSkillsResult
	PromptsOverride            func(LoadPromptTemplatesResult) LoadPromptTemplatesResult
	AgentsFilesOverride        func([]ContextFile) []ContextFile
	SystemPromptOverride       func(*string) *string
	AppendSystemPromptOverride func([]string) []string
}

// DefaultResourceLoader discovers instructions, skills and prompt templates
// for one project and agent directory.
type DefaultResourceLoader struct {
	cwd             string
	agentDir        string
	settingsManager *config.SettingsManager
	packageManager  *DefaultPackageManager

	additionalSkillPaths          []string
	additionalPromptTemplatePaths []string
	noSkills                      bool
	noPromptTemplates             bool
	noContextFiles                bool

	systemPromptSource       string
	appendSystemPromptSource []string

	skillsOverride             func(LoadSkillsResult) LoadSkillsResult
	promptsOverride            func(LoadPromptTemplatesResult) LoadPromptTemplatesResult
	agentsFilesOverride        func([]ContextFile) []ContextFile
	systemPromptOverride       func(*string) *string
	appendSystemPromptOverride func([]string) []string

	skills                        []Skill
	skillDiagnostics              []ResourceDiagnostic
	prompts                       []PromptTemplate
	promptDiagnostics             []ResourceDiagnostic
	agentsFiles                   []ContextFile
	systemPrompt                  *string
	systemPromptSourcePath        *string
	appendSystemPrompt            []string
	appendSystemPromptSourcePaths []string
	lastSkillPaths                []string
	lastPromptPaths               []string
	loaded                        bool
}

// NewDefaultResourceLoader returns a loader for the options.
func NewDefaultResourceLoader(options DefaultResourceLoaderOptions) *DefaultResourceLoader {
	cwd := resolvePath(options.Cwd, ".")
	agentDir := resolvePath(options.AgentDir, ".")
	settingsManager := options.SettingsManager
	if settingsManager == nil {
		settingsManager = config.NewSettingsManager(cwd, agentDir, config.CreateOptions{})
	}
	return &DefaultResourceLoader{
		cwd:                           cwd,
		agentDir:                      agentDir,
		settingsManager:               settingsManager,
		packageManager:                NewDefaultPackageManager(cwd, agentDir, settingsManager),
		additionalSkillPaths:          options.AdditionalSkillPaths,
		additionalPromptTemplatePaths: options.AdditionalPromptTemplatePaths,
		noSkills:                      options.NoSkills,
		noPromptTemplates:             options.NoPromptTemplates,
		noContextFiles:                options.NoContextFiles,
		systemPromptSource:            options.SystemPrompt,
		appendSystemPromptSource:      options.AppendSystemPrompt,
		skillsOverride:                options.SkillsOverride,
		promptsOverride:               options.PromptsOverride,
		agentsFilesOverride:           options.AgentsFilesOverride,
		systemPromptOverride:          options.SystemPromptOverride,
		appendSystemPromptOverride:    options.AppendSystemPromptOverride,
	}
}

// GetSkills returns the loaded skills and their diagnostics.
func (l *DefaultResourceLoader) GetSkills() LoadSkillsResult {
	return LoadSkillsResult{Skills: l.skills, Diagnostics: l.skillDiagnostics}
}

// GetPrompts returns the loaded prompt templates and their diagnostics.
func (l *DefaultResourceLoader) GetPrompts() LoadPromptTemplatesResult {
	return LoadPromptTemplatesResult{Templates: l.prompts, Diagnostics: l.promptDiagnostics}
}

// GetAgentsFiles returns the loaded project context files.
func (l *DefaultResourceLoader) GetAgentsFiles() []ContextFile { return l.agentsFiles }

// GetSystemPrompt returns the loaded system prompt, or nil.
func (l *DefaultResourceLoader) GetSystemPrompt() *string { return l.systemPrompt }

// GetSystemPromptSource returns the path the system prompt came from, if any.
func (l *DefaultResourceLoader) GetSystemPromptSource() (string, bool) {
	if l.systemPromptSourcePath == nil {
		return "", false
	}
	return *l.systemPromptSourcePath, true
}

// GetAppendSystemPrompt returns the append-system-prompt text.
func (l *DefaultResourceLoader) GetAppendSystemPrompt() []string { return l.appendSystemPrompt }

// GetAppendSystemPromptSources returns the paths the append text came from.
func (l *DefaultResourceLoader) GetAppendSystemPromptSources() []string {
	return l.appendSystemPromptSourcePaths
}

// ExtendResources adds skill and prompt paths after a load.
func (l *DefaultResourceLoader) ExtendResources(paths ResourceExtensionPaths) {
	for _, entry := range paths.SkillPaths {
		l.lastSkillPaths = mergePaths(l.lastSkillPaths, []string{l.resolveResourcePath(entry.Path)})
	}
	if len(paths.SkillPaths) > 0 {
		l.updateSkillsFromPaths(l.lastSkillPaths)
	}
	for _, entry := range paths.PromptPaths {
		l.lastPromptPaths = mergePaths(l.lastPromptPaths, []string{l.resolveResourcePath(entry.Path)})
	}
	if len(paths.PromptPaths) > 0 {
		l.updatePromptsFromPaths(l.lastPromptPaths)
	}
}

// Reload reloads every resource. When a trust resolver is supplied it runs
// against an untrusted settings snapshot first.
func (l *DefaultResourceLoader) Reload(ctx context.Context, options *ResourceLoaderReloadOptions) error {
	if options != nil && options.ResolveProjectTrust != nil {
		l.settingsManager.SetProjectTrusted(false)
		l.settingsManager.Reload()
		trusted, err := options.ResolveProjectTrust(ctx)
		if err != nil {
			return err
		}
		l.settingsManager.SetProjectTrusted(trusted)
	}
	l.settingsManager.Reload()

	resolved, err := l.packageManager.Resolve()
	if err != nil {
		return err
	}

	skillPaths := enabledPaths(resolved.Skills)
	skillPaths = mergePaths(skillPaths, l.additionalSkillPaths)
	l.lastSkillPaths = skillPaths
	l.updateSkillsFromPaths(skillPaths)

	promptPaths := enabledPaths(resolved.Prompts)
	promptPaths = mergePaths(promptPaths, l.additionalPromptTemplatePaths)
	l.lastPromptPaths = promptPaths
	l.updatePromptsFromPaths(promptPaths)

	agentsFiles := []ContextFile{}
	if !l.noContextFiles {
		agentsFiles = LoadProjectContextFiles(l.cwd, l.agentDir)
	}
	if l.agentsFilesOverride != nil {
		agentsFiles = l.agentsFilesOverride(agentsFiles)
	}
	l.agentsFiles = agentsFiles

	systemPromptSource := l.systemPromptSource
	if systemPromptSource == "" {
		systemPromptSource = l.discoverSystemPromptFile()
	}
	baseSystemPrompt := resolvePromptInput(systemPromptSource)
	l.systemPrompt = &baseSystemPrompt
	if systemPromptSource == "" {
		l.systemPrompt = nil
	}
	if l.systemPromptOverride != nil {
		l.systemPrompt = l.systemPromptOverride(l.systemPrompt)
	}
	if systemPromptSource != "" && pathExists(systemPromptSource) {
		resolvedSource := resolvePath(systemPromptSource, ".")
		l.systemPromptSourcePath = &resolvedSource
	}

	appendSources := l.appendSystemPromptSource
	if appendSources == nil {
		if discovered := l.discoverAppendSystemPromptFile(); discovered != "" {
			appendSources = []string{discovered}
		} else {
			appendSources = []string{}
		}
	}
	baseAppend := []string{}
	for _, source := range appendSources {
		if text := resolvePromptInput(source); text != "" {
			baseAppend = append(baseAppend, text)
		}
	}
	l.appendSystemPrompt = baseAppend
	if l.appendSystemPromptOverride != nil {
		l.appendSystemPrompt = l.appendSystemPromptOverride(baseAppend)
	}
	l.appendSystemPromptSourcePaths = []string{}
	for _, source := range appendSources {
		if pathExists(source) {
			l.appendSystemPromptSourcePaths = append(l.appendSystemPromptSourcePaths, resolvePath(source, "."))
		}
	}
	l.loaded = true
	return nil
}

// updateSkillsFromPaths loads and stores skills for the given paths.
func (l *DefaultResourceLoader) updateSkillsFromPaths(skillPaths []string) {
	var result LoadSkillsResult
	if l.noSkills && len(skillPaths) == 0 {
		result = LoadSkillsResult{}
	} else {
		result = LoadSkills(LoadSkillsOptions{
			Cwd:             l.cwd,
			AgentDir:        l.agentDir,
			SkillPaths:      skillPaths,
			IncludeDefaults: false,
		})
	}
	if l.skillsOverride != nil {
		result = l.skillsOverride(result)
	}
	l.skills = result.Skills
	l.skillDiagnostics = result.Diagnostics
}

// updatePromptsFromPaths loads and stores prompt templates for the paths.
func (l *DefaultResourceLoader) updatePromptsFromPaths(promptPaths []string) {
	var result LoadPromptTemplatesResult
	if l.noPromptTemplates && len(promptPaths) == 0 {
		result = LoadPromptTemplatesResult{}
	} else {
		loaded := LoadPromptTemplates(LoadPromptTemplatesOptions{
			Cwd:             l.cwd,
			AgentDir:        l.agentDir,
			PromptPaths:     promptPaths,
			IncludeDefaults: false,
		})
		deduped, dedupeDiagnostics := dedupePrompts(loaded.Templates)
		result = LoadPromptTemplatesResult{
			Templates:   deduped,
			Diagnostics: append(loaded.Diagnostics, dedupeDiagnostics...),
		}
	}
	if l.promptsOverride != nil {
		result = l.promptsOverride(result)
	}
	l.prompts = result.Templates
	l.promptDiagnostics = result.Diagnostics
}

// dedupePrompts keeps the first template for each name.
func dedupePrompts(prompts []PromptTemplate) ([]PromptTemplate, []ResourceDiagnostic) {
	seen := map[string]PromptTemplate{}
	order := []string{}
	diagnostics := []ResourceDiagnostic{}
	for _, prompt := range prompts {
		if existing, ok := seen[prompt.Name]; ok {
			diagnostics = append(diagnostics, ResourceDiagnostic{
				Type:    DiagnosticCollision,
				Message: "name \"/" + prompt.Name + "\" collision",
				Path:    prompt.FilePath,
				Collision: &ResourceCollision{
					ResourceType: "prompt",
					Name:         prompt.Name,
					WinnerPath:   existing.FilePath,
					LoserPath:    prompt.FilePath,
				},
			})
			continue
		}
		seen[prompt.Name] = prompt
		order = append(order, prompt.Name)
	}
	result := make([]PromptTemplate, 0, len(order))
	for _, name := range order {
		result = append(result, seen[name])
	}
	return result, diagnostics
}

func enabledPaths(resources []ResolvedResource) []string {
	paths := []string{}
	for _, resource := range resources {
		if resource.Enabled {
			paths = append(paths, resource.Path)
		}
	}
	return paths
}

// mergePaths resolves and deduplicates paths by canonical form.
func mergePaths(primary, additional []string) []string {
	merged := []string{}
	seen := map[string]bool{}
	for _, path := range append(append([]string{}, primary...), additional...) {
		resolved := resolvePath(path, ".")
		canonical := canonicalizePath(resolved)
		if seen[canonical] {
			continue
		}
		seen[canonical] = true
		merged = append(merged, resolved)
	}
	return merged
}

func (l *DefaultResourceLoader) resolveResourcePath(path string) string {
	return resolvePath(path, l.cwd)
}

// resolvePromptInput returns file content when input is an existing path, else
// the input text itself. Empty input returns empty.
func resolvePromptInput(input string) string {
	if input == "" {
		return ""
	}
	if pathExists(input) {
		if data, err := os.ReadFile(input); err == nil {
			return stripBOM(string(data))
		}
	}
	return input
}

func (l *DefaultResourceLoader) discoverSystemPromptFile() string {
	projectPath := filepath.Join(l.cwd, config.ConfigDirName, "SYSTEM.md")
	if l.settingsManager.IsProjectTrusted() && pathExists(projectPath) {
		return projectPath
	}
	globalPath := filepath.Join(l.agentDir, "SYSTEM.md")
	if pathExists(globalPath) {
		return globalPath
	}
	return ""
}

func (l *DefaultResourceLoader) discoverAppendSystemPromptFile() string {
	projectPath := filepath.Join(l.cwd, config.ConfigDirName, "APPEND_SYSTEM.md")
	if l.settingsManager.IsProjectTrusted() && pathExists(projectPath) {
		return projectPath
	}
	globalPath := filepath.Join(l.agentDir, "APPEND_SYSTEM.md")
	if pathExists(globalPath) {
		return globalPath
	}
	return ""
}
