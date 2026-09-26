package resources

import (
	"path/filepath"
	"strings"

	"github.com/tylergannon/gimbal/internal/pi/config"
	"github.com/tylergannon/gimbal/internal/pi/files"
)

// PathMetadata describes where a resolved resource came from.
type PathMetadata struct {
	Source      string
	Scope       SourceScope
	Origin      SourceOrigin
	BaseDir     string
	PackageRoot string
}

// ResolvedResource is one resolved resource path with its metadata and enabled
// state.
type ResolvedResource struct {
	Path     string
	Enabled  bool
	Metadata PathMetadata
}

// ResolvedPaths is the set of resolved resources by type.
type ResolvedPaths struct {
	Extensions []ResolvedResource
	Skills     []ResolvedResource
	Prompts    []ResolvedResource
	Themes     []ResolvedResource
}

type resourceAccumulator struct {
	extensions map[string]resourceEntry
	skills     map[string]resourceEntry
	prompts    map[string]resourceEntry
	themes     map[string]resourceEntry
}

type resourceEntry struct {
	metadata PathMetadata
	enabled  bool
}

// DefaultPackageManager resolves configured and auto-discovered resources,
// including already-installed package paths. Installing, updating and removing
// packages are out of scope.
type DefaultPackageManager struct {
	cwd      string
	agentDir string
	settings *config.SettingsManager
}

// NewDefaultPackageManager returns a resolver for a project and agent dir.
func NewDefaultPackageManager(cwd, agentDir string, settings *config.SettingsManager) *DefaultPackageManager {
	return &DefaultPackageManager{
		cwd:      resolvePath(cwd, "."),
		agentDir: resolvePath(agentDir, "."),
		settings: settings,
	}
}

// Resolve loads configured packages and resource paths plus automatic
// discovery.
func (m *DefaultPackageManager) Resolve() (ResolvedPaths, error) {
	accumulator := newResourceAccumulator()
	globalSettings := m.settings.GetGlobalSettings()
	projectSettings := m.settings.GetProjectSettings()

	packages := []packageSpec{}
	for _, pkg := range packageSources(projectSettings, "packages") {
		packages = append(packages, packageSpec{source: pkg, scope: ScopeProject})
	}
	for _, pkg := range packageSources(globalSettings, "packages") {
		packages = append(packages, packageSpec{source: pkg, scope: ScopeUser})
	}
	m.resolvePackageSources(m.dedupePackages(packages), accumulator)

	globalBaseDir := m.agentDir
	projectBaseDir := filepath.Join(m.cwd, config.ConfigDirName)

	for _, resourceType := range []string{"extensions", "skills", "prompts", "themes"} {
		metadata := PathMetadata{Source: "local", Scope: ScopeProject, Origin: OriginTopLevel}
		m.resolveLocalEntries(stringSliceValue(projectSettings, resourceType), resourceType, accumulator, metadata, projectBaseDir)
		metadata.Scope = ScopeUser
		m.resolveLocalEntries(stringSliceValue(globalSettings, resourceType), resourceType, accumulator, metadata, globalBaseDir)
	}

	m.addAutoDiscoveredResources(accumulator, globalSettings, projectSettings, globalBaseDir, projectBaseDir)

	return toResolvedPaths(accumulator), nil
}

// ResolveExtensionSources resolves explicit sources without defaults.
func (m *DefaultPackageManager) ResolveExtensionSources(sources []string, temporary bool) (ResolvedPaths, error) {
	accumulator := newResourceAccumulator()
	scope := ScopeUser
	if temporary {
		scope = ScopeTemporary
	}
	packages := make([]packageSpec, 0, len(sources))
	for _, source := range sources {
		packages = append(packages, packageSpec{source: source, scope: scope})
	}
	m.resolvePackageSources(packages, accumulator)
	return toResolvedPaths(accumulator), nil
}

// packageSpec is one configured package source and its scope.
type packageSpec struct {
	source string
	scope  SourceScope
}

// packageSources reads the string entries of a settings list field.
func packageSources(settings config.Settings, key string) []string {
	raw, ok := settings[key].([]any)
	if !ok {
		return []string{}
	}
	sources := []string{}
	for _, item := range raw {
		switch value := item.(type) {
		case string:
			sources = append(sources, value)
		case map[string]any:
			if source, ok := value["source"].(string); ok {
				sources = append(sources, source)
			}
		}
	}
	return sources
}

func stringSliceValue(settings config.Settings, key string) []string {
	raw, ok := settings[key].([]any)
	if !ok {
		return []string{}
	}
	values := []string{}
	for _, item := range raw {
		if text, ok := item.(string); ok {
			values = append(values, text)
		}
	}
	return values
}

func newResourceAccumulator() *resourceAccumulator {
	return &resourceAccumulator{
		extensions: map[string]resourceEntry{},
		skills:     map[string]resourceEntry{},
		prompts:    map[string]resourceEntry{},
		themes:     map[string]resourceEntry{},
	}
}

func (a *resourceAccumulator) target(resourceType string) map[string]resourceEntry {
	switch resourceType {
	case "extensions":
		return a.extensions
	case "skills":
		return a.skills
	case "prompts":
		return a.prompts
	default:
		return a.themes
	}
}

func (a *resourceAccumulator) add(resourceType, path string, metadata PathMetadata, enabled bool) {
	if path == "" {
		return
	}
	target := a.target(resourceType)
	if _, exists := target[path]; !exists {
		target[path] = resourceEntry{metadata: metadata, enabled: enabled}
	}
}

// dedupePackages keeps the project entry when the same package identity appears
// in both scopes.
func (m *DefaultPackageManager) dedupePackages(packages []packageSpec) []packageSpec {
	result := []packageSpec{}
	seen := map[string]int{}
	for _, entry := range packages {
		identity := m.packageIdentity(entry.source, entry.scope)
		index, ok := seen[identity]
		if !ok {
			seen[identity] = len(result)
			result = append(result, entry)
			continue
		}
		if result[index].scope == ScopeProject && entry.scope == ScopeUser {
			continue
		}
		if entry.scope == ScopeProject {
			result[index] = entry
		}
	}
	return result
}

func (m *DefaultPackageManager) packageIdentity(source string, scope SourceScope) string {
	if name, ok := npmName(source); ok {
		return "npm:" + name
	}
	if host, path, ok := gitHostPath(source); ok {
		return "git:" + host + "/" + path
	}
	return "local:" + m.resolveFromBase(source, m.baseDirForScope(scope))
}

func (m *DefaultPackageManager) baseDirForScope(scope SourceScope) string {
	switch scope {
	case ScopeProject:
		return filepath.Join(m.cwd, config.ConfigDirName)
	case ScopeUser:
		return m.agentDir
	default:
		return m.cwd
	}
}

func (m *DefaultPackageManager) resolveFromBase(input, baseDir string) string {
	return files.ResolvePath(input, baseDir, files.PathInputOptions{Trim: true})
}

func (m *DefaultPackageManager) resolvePackageSources(packages []packageSpec, accumulator *resourceAccumulator) {
	for _, entry := range packages {
		baseDir := m.baseDirForScope(entry.scope)
		metadata := PathMetadata{Source: entry.source, Scope: entry.scope, Origin: OriginPackage}

		if name, ok := npmName(entry.source); ok {
			installPath := m.npmInstallPath(name, entry.scope)
			if !dirExists(installPath) {
				continue
			}
			metadata.BaseDir = installPath
			metadata.PackageRoot = installPath
			m.collectPackageResources(installPath, accumulator, metadata)
			continue
		}
		if host, path, ok := gitHostPath(entry.source); ok {
			installPath := m.gitInstallPath(host, path, entry.scope)
			if !dirExists(installPath) {
				continue
			}
			metadata.BaseDir = installPath
			metadata.PackageRoot = installPath
			m.collectPackageResources(installPath, accumulator, metadata)
			continue
		}

		resolved := m.resolveFromBase(entry.source, baseDir)
		if !pathExists(resolved) {
			continue
		}
		if fileExists(resolved) {
			metadata.BaseDir = filepath.Dir(resolved)
			accumulator.add("extensions", resolved, metadata, true)
			continue
		}
		metadata.BaseDir = resolved
		metadata.PackageRoot = resolved
		m.collectPackageResources(resolved, accumulator, metadata)
	}
}

func (m *DefaultPackageManager) collectPackageResources(packageRoot string, accumulator *resourceAccumulator, metadata PathMetadata) {
	manifest, hasManifest := readPiManifest(filepath.Join(packageRoot, "package.json"))
	files := collectPackageFiles(packageRoot, manifest, hasManifest)
	for resourceType, paths := range files {
		for _, path := range paths {
			accumulator.add(resourceType, path, metadata, true)
		}
	}
}

// npmInstallPath is the managed install path for an npm package.
func (m *DefaultPackageManager) npmInstallPath(name string, scope SourceScope) string {
	if scope == ScopeProject {
		return filepath.Join(m.cwd, config.ConfigDirName, "npm", "node_modules", name)
	}
	return filepath.Join(m.agentDir, "npm", "node_modules", name)
}

// gitInstallPath is the managed install path for a git package.
func (m *DefaultPackageManager) gitInstallPath(host, path string, scope SourceScope) string {
	root := filepath.Join(m.agentDir, "git")
	if scope == ScopeProject {
		root = filepath.Join(m.cwd, config.ConfigDirName, "git")
	}
	return filepath.Join(root, host, path)
}

// npmName extracts the package name from an `npm:` source.
func npmName(source string) (string, bool) {
	trimmed := strings.TrimSpace(source)
	if !strings.HasPrefix(trimmed, "npm:") {
		return "", false
	}
	spec := strings.TrimSpace(strings.TrimPrefix(trimmed, "npm:"))
	if spec == "" {
		return "", false
	}
	// Drop a trailing @version for scoped and unscoped names.
	if strings.HasPrefix(spec, "@") {
		if index := strings.Index(spec[1:], "@"); index >= 0 {
			return spec[:index+1], true
		}
		return spec, true
	}
	if index := strings.Index(spec, "@"); index > 0 {
		return spec[:index], true
	}
	return spec, true
}

// gitHostPath extracts host and path from a git URL.
func gitHostPath(source string) (host, path string, ok bool) {
	trimmed := strings.TrimSpace(source)
	if !strings.HasPrefix(trimmed, "git:") && !strings.Contains(trimmed, "://") && !strings.HasPrefix(trimmed, "git@") {
		return "", "", false
	}
	if after, ok0 := strings.CutPrefix(trimmed, "git:"); ok0 {
		trimmed = after
	}
	if after, ok0 := strings.CutPrefix(trimmed, "git@"); ok0 {
		rest := after
		host, path, found := strings.Cut(rest, ":")
		if !found {
			return "", "", false
		}
		return host, strings.TrimSuffix(path, ".git"), true
	}
	if index := strings.Index(trimmed, "://"); index >= 0 {
		rest := trimmed[index+3:]
		host, path, found := strings.Cut(rest, "/")
		if !found {
			return "", "", false
		}
		return host, strings.TrimSuffix(path, ".git"), true
	}
	return "", "", false
}

func (m *DefaultPackageManager) resolveLocalEntries(entries []string, resourceType string, accumulator *resourceAccumulator, metadata PathMetadata, baseDir string) {
	for _, entry := range entries {
		resolved := m.resolveFromBase(entry, baseDir)
		for _, path := range collectResourcePathsFromPath(resolved, resourceType) {
			accumulator.add(resourceType, path, metadata, true)
		}
	}
}

// collectResourcePathsFromPath expands a file or directory into the resource
// paths it contains.
func collectResourcePathsFromPath(path, resourceType string) []string {
	if !pathExists(path) {
		return nil
	}
	if fileExists(path) {
		return []string{path}
	}
	switch resourceType {
	case "skills":
		return collectSkillEntries(path, skillModePi)
	case "extensions":
		return collectFiles(path, func(name string) bool {
			return strings.HasSuffix(name, ".ts") || strings.HasSuffix(name, ".js")
		}, true, nil, path)
	case "themes":
		return collectAutoThemeEntries(path)
	default:
		return collectAutoPromptEntries(path)
	}
}

func (m *DefaultPackageManager) addAutoDiscoveredResources(
	accumulator *resourceAccumulator,
	globalSettings, projectSettings config.Settings,
	globalBaseDir, projectBaseDir string,
) {
	userMetadata := PathMetadata{Source: "auto", Scope: ScopeUser, Origin: OriginTopLevel, BaseDir: globalBaseDir}
	projectMetadata := PathMetadata{Source: "auto", Scope: ScopeProject, Origin: OriginTopLevel, BaseDir: projectBaseDir}

	projectTrusted := m.settings.IsProjectTrusted()
	userAgentsSkillsDir := filepath.Join(config.HomeDir(), ".agents", "skills")
	projectAgentsSkillDirs := []string{}
	if projectTrusted {
		for _, dir := range collectAncestorAgentsSkillDirs(m.cwd) {
			if resolvePath(dir, ".") != resolvePath(userAgentsSkillsDir, ".") {
				projectAgentsSkillDirs = append(projectAgentsSkillDirs, dir)
			}
		}
	}

	if projectTrusted {
		for _, path := range collectSkillEntries(filepath.Join(projectBaseDir, "skills"), skillModePi) {
			accumulator.add("skills", path, projectMetadata, true)
		}
		for _, path := range collectAutoPromptEntries(filepath.Join(projectBaseDir, "prompts")) {
			accumulator.add("prompts", path, projectMetadata, true)
		}
	}
	for _, agentsSkillsDir := range projectAgentsSkillDirs {
		agentsBaseDir := filepath.Dir(agentsSkillsDir)
		agentsMetadata := projectMetadata
		agentsMetadata.BaseDir = agentsBaseDir
		for _, path := range collectSkillEntries(agentsSkillsDir, skillModeAgents) {
			accumulator.add("skills", path, agentsMetadata, true)
		}
	}

	for _, path := range collectSkillEntries(filepath.Join(globalBaseDir, "skills"), skillModePi) {
		accumulator.add("skills", path, userMetadata, true)
	}
	if dirExists(userAgentsSkillsDir) {
		userAgentsMetadata := userMetadata
		userAgentsMetadata.BaseDir = filepath.Dir(userAgentsSkillsDir)
		for _, path := range collectSkillEntries(userAgentsSkillsDir, skillModeAgents) {
			accumulator.add("skills", path, userAgentsMetadata, true)
		}
	}
	for _, path := range collectAutoPromptEntries(filepath.Join(globalBaseDir, "prompts")) {
		accumulator.add("prompts", path, userMetadata, true)
	}
}

// toResolvedPaths sorts by precedence and deduplicates canonical paths.
func toResolvedPaths(accumulator *resourceAccumulator) ResolvedPaths {
	mapToResolved := func(entries map[string]resourceEntry) []ResolvedResource {
		resolved := make([]ResolvedResource, 0, len(entries))
		for path, entry := range entries {
			resolved = append(resolved, ResolvedResource{Path: path, Enabled: entry.enabled, Metadata: entry.metadata})
		}
		sortResolvedResources(resolved)
		seen := map[string]bool{}
		filtered := resolved[:0]
		for _, entry := range resolved {
			canonical := canonicalizePath(entry.Path)
			if seen[canonical] {
				continue
			}
			seen[canonical] = true
			filtered = append(filtered, entry)
		}
		return filtered
	}
	return ResolvedPaths{
		Extensions: mapToResolved(accumulator.extensions),
		Skills:     mapToResolved(accumulator.skills),
		Prompts:    mapToResolved(accumulator.prompts),
		Themes:     mapToResolved(accumulator.themes),
	}
}

// resourcePrecedenceRank orders resources so that name-collision resolution
// ("first wins") yields the correct outcome. Lower rank wins.
func resourcePrecedenceRank(metadata PathMetadata) int {
	if metadata.Origin == OriginPackage {
		return 4
	}
	scopeBase := 2
	if metadata.Scope == ScopeProject {
		scopeBase = 0
	}
	if metadata.Source == "local" {
		return scopeBase
	}
	return scopeBase + 1
}

func sortResolvedResources(resources []ResolvedResource) {
	for i := 1; i < len(resources); i++ {
		for j := i; j > 0; j-- {
			if resourcePrecedenceRank(resources[j-1].Metadata) <= resourcePrecedenceRank(resources[j].Metadata) {
				break
			}
			resources[j-1], resources[j] = resources[j], resources[j-1]
		}
	}
}
