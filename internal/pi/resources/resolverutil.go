package resources

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// skillDiscoveryMode selects which markdown files count as skills alongside
// SKILL.md roots. pi loads root-level .md files; the AGENTS convention loads
// nested ones.
type skillDiscoveryMode int

const (
	skillModePi skillDiscoveryMode = iota
	skillModeAgents
)

// collectFiles recursively collects files whose name matches keep, honoring the
// tree's ignore files and skipping dot entries and node_modules.
func collectFiles(dir string, keep func(string) bool, skipNodeModules bool, ig *skillIgnore, rootDir string) []string {
	files := []string{}
	if !dirExists(dir) {
		return files
	}
	root := rootDir
	if root == "" {
		root = dir
	}
	if ig == nil {
		ig = newSkillIgnore()
	}
	ig.addRules(dir, root)

	entries, err := os.ReadDir(dir)
	if err != nil {
		return files
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".") || (skipNodeModules && entry.Name() == "node_modules") {
			continue
		}
		fullPath := filepath.Join(dir, entry.Name())
		isDir, isFile := statIsDirFile(fullPath, entry)

		relPath := toPosixPath(relativePath(root, fullPath))
		ignorePath := relPath
		if isDir {
			ignorePath = relPath + "/"
		}
		if ig.ignores(ignorePath, isDir) {
			continue
		}
		if isDir {
			files = append(files, collectFiles(fullPath, keep, skipNodeModules, ig, root)...)
		} else if isFile && keep(entry.Name()) {
			files = append(files, fullPath)
		}
	}
	return files
}

// collectSkillEntries collects skill files from a directory, following pi's
// discovery: a SKILL.md root wins, root-level .md files load in pi mode and
// nested .md files load in agents mode.
func collectSkillEntries(dir string, mode skillDiscoveryMode) []string {
	return collectSkillEntriesInternal(dir, dir, mode, nil)
}

func collectSkillEntriesInternal(dir, root string, mode skillDiscoveryMode, ig *skillIgnore) []string {
	entries := []string{}
	if !dirExists(dir) {
		return entries
	}
	if ig == nil {
		ig = newSkillIgnore()
	}
	ig.addRules(dir, root)

	dirEntries, err := os.ReadDir(dir)
	if err != nil {
		return entries
	}

	for _, entry := range dirEntries {
		if entry.Name() != "SKILL.md" {
			continue
		}
		fullPath := filepath.Join(dir, entry.Name())
		isFile, ok := statIsFile(fullPath, entry)
		if !ok {
			continue
		}
		relPath := toPosixPath(relativePath(root, fullPath))
		if isFile && !ig.ignores(relPath, false) {
			entries = append(entries, fullPath)
			return entries
		}
	}

	for _, entry := range dirEntries {
		if strings.HasPrefix(entry.Name(), ".") || entry.Name() == "node_modules" {
			continue
		}
		fullPath := filepath.Join(dir, entry.Name())
		isDir, isFile := statIsDirFile(fullPath, entry)

		relPath := toPosixPath(relativePath(root, fullPath))
		shouldIncludeMarkdown := isFile &&
			strings.HasSuffix(entry.Name(), ".md") &&
			!ig.ignores(relPath, false) &&
			((mode == skillModePi && dir == root) || (mode == skillModeAgents && dir != root))
		if shouldIncludeMarkdown {
			entries = append(entries, fullPath)
			continue
		}
		if !isDir {
			continue
		}
		if ig.ignores(relPath+"/", true) {
			continue
		}
		entries = append(entries, collectSkillEntriesInternal(fullPath, root, mode, ig)...)
	}
	return entries
}

// collectAutoPromptEntries collects root-level .md files in dir.
func collectAutoPromptEntries(dir string) []string {
	return collectFiles(dir, func(name string) bool { return strings.HasSuffix(name, ".md") }, true, nil, dir)
}

// collectAutoThemeEntries collects root-level .json files in dir.
func collectAutoThemeEntries(dir string) []string {
	return collectFiles(dir, func(name string) bool { return strings.HasSuffix(name, ".json") }, true, nil, dir)
}

// findGitRepoRoot walks up from start to the first directory holding .git.
func findGitRepoRoot(startDir string) (string, bool) {
	dir := resolvePath(startDir, ".")
	for {
		if pathExists(filepath.Join(dir, ".git")) {
			return dir, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

// collectAncestorAgentsSkillDirs returns each ancestor's .agents/skills
// directory, stopping at the git repo root when one is found.
func collectAncestorAgentsSkillDirs(startDir string) []string {
	dirs := []string{}
	resolvedStart := resolvePath(startDir, ".")
	gitRepoRoot, hasGitRepoRoot := findGitRepoRoot(resolvedStart)

	dir := resolvedStart
	for {
		dirs = append(dirs, filepath.Join(dir, ".agents", "skills"))
		if hasGitRepoRoot && dir == gitRepoRoot {
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return dirs
}

// piManifest is the resource-declaration subset of a package.json's "pi" key.
type piManifest struct {
	Extensions []string
	Skills     []string
	Prompts    []string
	Themes     []string
}

// readPiManifest reads the pi manifest from a package.json path, or reports
// false when absent or malformed.
func readPiManifest(packageJSONPath string) (piManifest, bool) {
	data, err := os.ReadFile(packageJSONPath)
	if err != nil {
		return piManifest{}, false
	}
	var pkg map[string]any
	if err := json.Unmarshal([]byte(stripBOM(string(data))), &pkg); err != nil {
		return piManifest{}, false
	}
	piValue, ok := pkg["pi"].(map[string]any)
	if !ok {
		return piManifest{}, false
	}
	manifest := piManifest{}
	for key, target := range map[string]*[]string{
		"extensions": &manifest.Extensions,
		"skills":     &manifest.Skills,
		"prompts":    &manifest.Prompts,
		"themes":     &manifest.Themes,
	} {
		raw, ok := piValue[key].([]any)
		if !ok {
			continue
		}
		values := []string{}
		allStrings := true
		for _, item := range raw {
			text, ok := item.(string)
			if !ok {
				allStrings = false
				break
			}
			values = append(values, text)
		}
		if allStrings {
			*target = values
		}
	}
	return manifest, true
}

// collectPackageFiles resolves manifest entries and convention directories
// into skill and prompt file paths, keyed by resource type.
func collectPackageFiles(packageRoot string, manifest piManifest, hasManifest bool) map[string][]string {
	files := map[string][]string{"skills": {}, "prompts": {}}

	if hasManifest {
		files["skills"] = resolveManifestEntries(manifest.Skills, packageRoot, func(path string) []string {
			if dirExists(path) {
				return collectSkillEntries(path, skillModePi)
			}
			if fileExists(path) {
				return []string{path}
			}
			return nil
		})
		files["prompts"] = resolveManifestEntries(manifest.Prompts, packageRoot, func(path string) []string {
			if dirExists(path) {
				return collectAutoPromptEntries(path)
			}
			if fileExists(path) {
				return []string{path}
			}
			return nil
		})
		return files
	}

	if dir := filepath.Join(packageRoot, "skills"); dirExists(dir) {
		files["skills"] = collectSkillEntries(dir, skillModePi)
	}
	if dir := filepath.Join(packageRoot, "prompts"); dirExists(dir) {
		files["prompts"] = collectAutoPromptEntries(dir)
	}
	return files
}

// resolveManifestEntries resolves each manifest entry against packageRoot.
func resolveManifestEntries(entries []string, packageRoot string, collect func(string) []string) []string {
	result := []string{}
	for _, entry := range entries {
		path := resolvePath(entry, packageRoot)
		result = append(result, collect(path)...)
	}
	return result
}
