package resources

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/tylergannon/gimbal/internal/pi/config"
)

// Skill name and description limits per the Agent Skills spec.
const (
	maxSkillNameLength        = 64
	maxSkillDescriptionLength = 1024
)

// SkillFrontmatter is the recognized frontmatter of a skill file. Unknown
// keys are ignored.
type SkillFrontmatter struct {
	Name                   string
	Description            string
	DisableModelInvocation bool
}

// Skill is a discovered Agent Skill.
type Skill struct {
	Name                   string
	Description            string
	FilePath               string
	BaseDir                string
	SourceInfo             SourceInfo
	DisableModelInvocation bool
}

// LoadSkillsResult is the outcome of a skill load.
type LoadSkillsResult struct {
	Skills      []Skill
	Diagnostics []ResourceDiagnostic
}

// LoadSkillsFromDirOptions configures LoadSkillsFromDir.
type LoadSkillsFromDirOptions struct {
	// Dir is the directory to scan for skills.
	Dir string
	// Source identifies where the skills came from ("user", "project", "path", ...).
	Source string
}

// LoadSkillsFromDir loads skills from a directory. A directory containing
// SKILL.md is a skill root and is not recursed further; otherwise direct .md
// children of the root load and subdirectories are searched for SKILL.md.
func LoadSkillsFromDir(options LoadSkillsFromDirOptions) LoadSkillsResult {
	return loadSkillsFromDirInternal(options.Dir, options.Source, true, nil, "")
}

// LoadSkillsOptions configures LoadSkills.
type LoadSkillsOptions struct {
	// Cwd is the working directory for project-local skills.
	Cwd string
	// AgentDir is the global config directory. Empty means config.GetAgentDir().
	AgentDir string
	// SkillPaths are explicit files or directories.
	SkillPaths []string
	// IncludeDefaults loads the agent and project default skill directories.
	IncludeDefaults bool
}

// LoadSkills loads skills from every configured location, deduplicating by
// canonical path and reporting name collisions.
func LoadSkills(options LoadSkillsOptions) LoadSkillsResult {
	resolvedCwd := resolvePath(options.Cwd, ".")
	agentDir := options.AgentDir
	if agentDir == "" {
		agentDir = config.GetAgentDir()
	}
	resolvedAgentDir := resolvePath(agentDir, ".")

	skillMap := map[string]Skill{}
	order := []string{}
	realPathSet := map[string]bool{}
	allDiagnostics := []ResourceDiagnostic{}
	collisionDiagnostics := []ResourceDiagnostic{}

	addSkills := func(result LoadSkillsResult) {
		allDiagnostics = append(allDiagnostics, result.Diagnostics...)
		for _, skill := range result.Skills {
			realPath := canonicalizePath(skill.FilePath)
			if realPathSet[realPath] {
				continue
			}
			if existing, ok := skillMap[skill.Name]; ok {
				collisionDiagnostics = append(collisionDiagnostics, ResourceDiagnostic{
					Type:    DiagnosticCollision,
					Message: fmt.Sprintf("name %q collision", skill.Name),
					Path:    skill.FilePath,
					Collision: &ResourceCollision{
						ResourceType: "skill",
						Name:         skill.Name,
						WinnerPath:   existing.FilePath,
						LoserPath:    skill.FilePath,
					},
				})
			} else {
				skillMap[skill.Name] = skill
				order = append(order, skill.Name)
				realPathSet[realPath] = true
			}
		}
	}

	if options.IncludeDefaults {
		addSkills(loadSkillsFromDirInternal(filepath.Join(resolvedAgentDir, "skills"), "user", true, nil, ""))
		addSkills(loadSkillsFromDirInternal(filepath.Join(resolvedCwd, config.ConfigDirName, "skills"), "project", true, nil, ""))
	}

	userSkillsDir := filepath.Join(resolvedAgentDir, "skills")
	projectSkillsDir := filepath.Join(resolvedCwd, config.ConfigDirName, "skills")

	getSource := func(resolvedPath string) string {
		if !options.IncludeDefaults {
			if isUnderPath(resolvedPath, userSkillsDir) {
				return "user"
			}
			if isUnderPath(resolvedPath, projectSkillsDir) {
				return "project"
			}
		}
		return "path"
	}

	for _, rawPath := range options.SkillPaths {
		resolvedPath := resolvePath(rawPath, resolvedCwd)
		if !pathExists(resolvedPath) {
			allDiagnostics = append(allDiagnostics, ResourceDiagnostic{
				Type: DiagnosticWarning, Message: "skill path does not exist", Path: resolvedPath,
			})
			continue
		}

		info, err := os.Stat(resolvedPath)
		if err != nil {
			allDiagnostics = append(allDiagnostics, ResourceDiagnostic{
				Type: DiagnosticWarning, Message: err.Error(), Path: resolvedPath,
			})
			continue
		}
		source := getSource(resolvedPath)
		if info.IsDir() {
			addSkills(loadSkillsFromDirInternal(resolvedPath, source, true, nil, ""))
		} else if info.Mode().IsRegular() && strings.HasSuffix(resolvedPath, ".md") {
			result := loadSkillFromFile(resolvedPath, source)
			if result.Skill != nil {
				addSkills(LoadSkillsResult{Skills: []Skill{*result.Skill}, Diagnostics: result.Diagnostics})
			} else {
				allDiagnostics = append(allDiagnostics, result.Diagnostics...)
			}
		} else {
			allDiagnostics = append(allDiagnostics, ResourceDiagnostic{
				Type: DiagnosticWarning, Message: "skill path is not a markdown file", Path: resolvedPath,
			})
		}
	}

	skills := make([]Skill, 0, len(order))
	for _, name := range order {
		skills = append(skills, skillMap[name])
	}
	return LoadSkillsResult{
		Skills:      skills,
		Diagnostics: append(allDiagnostics, collisionDiagnostics...),
	}
}

// skillLoadResult is one file's load result.
type skillLoadResult struct {
	Skill       *Skill
	Diagnostics []ResourceDiagnostic
}

// loadSkillsFromDirInternal is the recursive directory walk.
func loadSkillsFromDirInternal(
	dir, source string,
	includeRootFiles bool,
	ig *skillIgnore,
	rootDir string,
) LoadSkillsResult {
	skills := []Skill{}
	diagnostics := []ResourceDiagnostic{}

	if !dirExists(dir) {
		return LoadSkillsResult{Skills: skills, Diagnostics: diagnostics}
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
		return LoadSkillsResult{Skills: skills, Diagnostics: diagnostics}
	}

	// A SKILL.md in this directory makes it a skill root: load it and stop.
	for _, entry := range entries {
		if entry.Name() != "SKILL.md" {
			continue
		}
		fullPath := filepath.Join(dir, entry.Name())
		isFile, ok := statIsFile(fullPath, entry)
		if !ok {
			continue
		}
		relPath := toPosixPath(relativePath(root, fullPath))
		if !isFile || ig.ignores(relPath, false) {
			continue
		}
		result := loadSkillFromFile(fullPath, source)
		if result.Skill != nil {
			skills = append(skills, *result.Skill)
		}
		diagnostics = append(diagnostics, result.Diagnostics...)
		return LoadSkillsResult{Skills: skills, Diagnostics: diagnostics}
	}

	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") || name == "node_modules" {
			continue
		}
		fullPath := filepath.Join(dir, name)
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
			sub := loadSkillsFromDirInternal(fullPath, source, false, ig, root)
			skills = append(skills, sub.Skills...)
			diagnostics = append(diagnostics, sub.Diagnostics...)
			continue
		}

		if !isFile || !includeRootFiles || !strings.HasSuffix(name, ".md") {
			continue
		}
		result := loadSkillFromFile(fullPath, source)
		if result.Skill != nil {
			skills = append(skills, *result.Skill)
		}
		diagnostics = append(diagnostics, result.Diagnostics...)
	}

	return LoadSkillsResult{Skills: skills, Diagnostics: diagnostics}
}

// loadSkillFromFile parses one skill file.
func loadSkillFromFile(filePath, source string) skillLoadResult {
	diagnostics := []ResourceDiagnostic{}
	isDeclaredSkill := filepath.Base(filePath) == "SKILL.md"

	data, err := os.ReadFile(filePath)
	if err != nil {
		return skillLoadResult{Diagnostics: []ResourceDiagnostic{
			{Type: DiagnosticWarning, Message: err.Error(), Path: filePath},
		}}
	}

	frontmatter, _, err := ParseFrontmatter(string(data))
	if err != nil {
		if isDeclaredSkill {
			diagnostics = append(diagnostics, ResourceDiagnostic{
				Type: DiagnosticWarning, Message: err.Error(), Path: filePath,
			})
		}
		return skillLoadResult{Diagnostics: diagnostics}
	}

	description, hasDescription := stringValue(frontmatter["description"])
	description = strings.TrimSpace(description)
	hasDescription = hasDescription && description != ""
	if !isDeclaredSkill && !hasDescription {
		return skillLoadResult{Diagnostics: diagnostics}
	}

	skillDir := filepath.Dir(filePath)
	parentDirName := filepath.Base(skillDir)

	for _, message := range validateDescription(description, hasDescription) {
		diagnostics = append(diagnostics, ResourceDiagnostic{Type: DiagnosticWarning, Message: message, Path: filePath})
	}

	name, hasName := stringValue(frontmatter["name"])
	if !hasName || name == "" {
		name = parentDirName
	}
	for _, message := range validateSkillName(name) {
		diagnostics = append(diagnostics, ResourceDiagnostic{Type: DiagnosticWarning, Message: message, Path: filePath})
	}

	if !hasDescription {
		return skillLoadResult{Diagnostics: diagnostics}
	}

	return skillLoadResult{
		Skill: &Skill{
			Name:                   name,
			Description:            description,
			FilePath:               filePath,
			BaseDir:                skillDir,
			SourceInfo:             createSkillSourceInfo(filePath, skillDir, source),
			DisableModelInvocation: boolValueTrue(frontmatter["disable-model-invocation"]),
		},
		Diagnostics: diagnostics,
	}
}

func createSkillSourceInfo(filePath, baseDir, source string) SourceInfo {
	switch source {
	case "user":
		return CreateSyntheticSourceInfo(filePath, SyntheticSourceInfoOptions{
			Source: "local", Scope: ScopeUser, BaseDir: baseDir,
		})
	case "project":
		return CreateSyntheticSourceInfo(filePath, SyntheticSourceInfoOptions{
			Source: "local", Scope: ScopeProject, BaseDir: baseDir,
		})
	case "path":
		return CreateSyntheticSourceInfo(filePath, SyntheticSourceInfoOptions{
			Source: "local", BaseDir: baseDir,
		})
	default:
		return CreateSyntheticSourceInfo(filePath, SyntheticSourceInfoOptions{Source: source, BaseDir: baseDir})
	}
}

// stringValue returns value as a string and whether it was string-typed.
func stringValue(value any) (string, bool) {
	text, ok := value.(string)
	return text, ok
}

// boolValueTrue reports whether value is the YAML boolean true (not a string).
func boolValueTrue(value any) bool {
	boolean, ok := value.(bool)
	return ok && boolean
}

var skillNamePattern = regexp.MustCompile(`^[a-z0-9-]+$`)

// validateSkillName ports pi's validateName.
func validateSkillName(name string) []string {
	errors := []string{}
	if len([]rune(name)) > maxSkillNameLength {
		errors = append(errors, fmt.Sprintf("name exceeds %d characters (%d)", maxSkillNameLength, len([]rune(name))))
	}
	if !skillNamePattern.MatchString(name) {
		errors = append(errors, "name contains invalid characters (must be lowercase a-z, 0-9, hyphens only)")
	}
	if strings.HasPrefix(name, "-") || strings.HasSuffix(name, "-") {
		errors = append(errors, "name must not start or end with a hyphen")
	}
	if strings.Contains(name, "--") {
		errors = append(errors, "name must not contain consecutive hyphens")
	}
	return errors
}

// validateDescription ports pi's validateDescription.
func validateDescription(description string, hasDescription bool) []string {
	errors := []string{}
	if !hasDescription || strings.TrimSpace(description) == "" {
		errors = append(errors, "description is required")
	} else if len([]rune(description)) > maxSkillDescriptionLength {
		errors = append(errors, fmt.Sprintf("description exceeds %d characters (%d)", maxSkillDescriptionLength, len([]rune(description))))
	}
	return errors
}

// FormatSkillsForPrompt renders skills as the Agent Skills XML block, telling
// the model to load skill files with the read tool.
func FormatSkillsForPrompt(skills []Skill) string {
	return FormatSkillsForPromptWithTool(skills, "read")
}

// FormatSkillsForPromptWithTool renders skills as the Agent Skills XML block,
// naming fileReadTool as the way to load a skill's file. Only "bash" selects
// the bash wording; every other value reads as "read".
func FormatSkillsForPromptWithTool(skills []Skill, fileReadTool string) string {
	visible := []Skill{}
	for _, skill := range skills {
		if !skill.DisableModelInvocation {
			visible = append(visible, skill)
		}
	}
	if len(visible) == 0 {
		return ""
	}

	loadInstruction := "Use the read tool to load a skill's file when the task matches its description."
	if fileReadTool == "bash" {
		loadInstruction = "Use bash to load a skill's file when the task matches its description."
	}

	lines := []string{
		"\n\nThe following skills provide specialized instructions for specific tasks.",
		loadInstruction,
		"When a skill file references a relative path, resolve it against the skill directory (parent of SKILL.md / dirname of the path) and use that absolute path in tool commands.",
		"",
		"<available_skills>",
	}
	for _, skill := range visible {
		lines = append(lines,
			"  <skill>",
			"    <name>"+escapeXML(skill.Name)+"</name>",
			"    <description>"+escapeXML(skill.Description)+"</description>",
			"    <location>"+escapeXML(skill.FilePath)+"</location>",
			"  </skill>",
		)
	}
	lines = append(lines, "</available_skills>")
	return strings.Join(lines, "\n")
}

func escapeXML(value string) string {
	value = strings.ReplaceAll(value, "&", "&amp;")
	value = strings.ReplaceAll(value, "<", "&lt;")
	value = strings.ReplaceAll(value, ">", "&gt;")
	value = strings.ReplaceAll(value, `"`, "&quot;")
	value = strings.ReplaceAll(value, "'", "&apos;")
	return value
}

// statIsFile resolves whether full is a regular file, following symlinks.
func statIsFile(full string, entry os.DirEntry) (isFile, ok bool) {
	if entry.Type()&os.ModeSymlink != 0 {
		info, err := os.Stat(full)
		if err != nil {
			return false, false
		}
		return info.Mode().IsRegular(), true
	}
	return entry.Type().IsRegular(), true
}

// statIsDirFile resolves dir/file-ness following symlinks.
func statIsDirFile(full string, entry os.DirEntry) (isDir, isFile bool) {
	if entry.Type()&os.ModeSymlink != 0 {
		info, err := os.Stat(full)
		if err != nil {
			return false, false
		}
		return info.IsDir(), info.Mode().IsRegular()
	}
	return entry.IsDir(), entry.Type().IsRegular()
}
