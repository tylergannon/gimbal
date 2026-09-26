package resources

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/tylergannon/gimbal/internal/pi/model"
)

// ContextFile is a project instruction file injected into the system prompt.
type ContextFile struct {
	Path    string
	Content string
}

// BuildSystemPromptOptions configures system-prompt assembly.
type BuildSystemPromptOptions struct {
	// CustomPrompt replaces the default prompt prefix with its text as the
	// preamble. Empty keeps the default prefix.
	CustomPrompt string
	// ForceSystemPrompt, when non-nil, is an exact full prompt with no
	// sections, even when empty.
	ForceSystemPrompt *string
	// SelectedTools are the active tool names. Nil means the pi default
	// [read, bash, edit, write]; an empty, non-nil slice means no tools.
	SelectedTools []string
	// ToolSnippets are one-line tool descriptions keyed by tool name.
	ToolSnippets map[string]string
	// ToolGuidelines are guideline bullets contributed by each tool.
	ToolGuidelines map[string][]string
	// PromptGuidelines are additional guideline bullets.
	PromptGuidelines []string
	// AppendSystemPrompt is user-configured text placed in the addendum.
	AppendSystemPrompt string
	// Sections are additional prompt sections, each wrapped in a tag.
	Sections model.SystemSections
	// Cwd is the working directory; backslashes render as forward slashes.
	Cwd string
	// ContextFiles are pre-loaded project context files.
	ContextFiles []ContextFile
	// Skills are pre-loaded skills.
	Skills []Skill
	// ReadmePath, DocsPath and ExamplesPath are the pi documentation paths.
	// Empty values fall back to the package paths.
	ReadmePath   string
	DocsPath     string
	ExamplesPath string
}

// SystemPromptState is the complete prompt state a set of options describes.
type SystemPromptState struct {
	Content  string
	Sections model.SystemSections
}

// SystemPromptSectionName validates a custom section name.
var SystemPromptSectionName = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)

const defaultPromptPreamble = "You are an expert coding assistant operating inside pi, a coding agent harness. You help users by reading files, executing commands, editing code, and writing new files."

// NormalizeBuildSystemPromptOptions returns options in pi's normalized,
// collection-complete shape.
func NormalizeBuildSystemPromptOptions(input BuildSystemPromptOptions) BuildSystemPromptOptions {
	out := input
	if input.SelectedTools == nil {
		out.SelectedTools = []string{"read", "bash", "edit", "write"}
	} else {
		out.SelectedTools = append([]string{}, input.SelectedTools...)
	}
	out.ToolSnippets = map[string]string{}
	maps.Copy(out.ToolSnippets, input.ToolSnippets)
	out.ToolGuidelines = map[string][]string{}
	for name, guidelines := range input.ToolGuidelines {
		out.ToolGuidelines[name] = append([]string{}, guidelines...)
	}
	out.PromptGuidelines = append([]string{}, input.PromptGuidelines...)
	out.ContextFiles = append([]ContextFile{}, input.ContextFiles...)
	out.Skills = append([]Skill{}, input.Skills...)
	out.Sections = model.SystemSections{}
	for _, section := range input.Sections.Entries() {
		out.Sections.Set(section.Name, section.Value)
	}
	return out
}

func renderProjectContext(contextFiles []ContextFile) string {
	parts := []string{"Project-specific instructions and guidelines:"}
	for _, file := range contextFiles {
		parts = append(parts, `<project_instructions path="`+file.Path+`">`+"\n"+file.Content+"\n</project_instructions>")
	}
	return strings.Join(parts, "\n\n")
}

func buildRules(selectedTools []string, toolGuidelines map[string][]string, promptGuidelines []string) string {
	rules := []string{}
	seen := map[string]bool{}
	addRule := func(rule string) {
		normalized := strings.TrimSpace(rule)
		if normalized == "" || seen[normalized] {
			return
		}
		seen[normalized] = true
		rules = append(rules, normalized)
	}

	has := func(name string) bool {
		return slices.Contains(selectedTools, name)
	}
	hasBash, hasPowerShell := has("bash"), has("powershell")
	if (hasBash || hasPowerShell) && !has("grep") && !has("find") && !has("ls") {
		switch {
		case hasBash && hasPowerShell:
			addRule("Use bash or PowerShell for file operations like listing, searching, and finding files")
		case hasPowerShell:
			addRule("Use PowerShell for file operations like listing, searching, and finding files")
		default:
			addRule("Use bash for file operations like ls, rg, find")
		}
	}

	for _, name := range selectedTools {
		for _, rule := range toolGuidelines[name] {
			addRule(rule)
		}
	}
	for _, rule := range promptGuidelines {
		addRule(rule)
	}
	addRule("Be concise in your responses")
	addRule("Show file paths clearly when working with files")

	lines := make([]string, len(rules))
	for i, rule := range rules {
		lines[i] = "- " + rule
	}
	return strings.Join(lines, "\n")
}

// BuildSystemPromptSections builds the ordered, independently replaceable
// prompt sections.
func BuildSystemPromptSections(input BuildSystemPromptOptions) (model.SystemSections, error) {
	options := NormalizeBuildSystemPromptOptions(input)

	for _, section := range options.Sections.Entries() {
		if !SystemPromptSectionName.MatchString(section.Name) || section.Name == "preamble" {
			return nil, fmt.Errorf("invalid system prompt section name: %s", section.Name)
		}
	}

	promptSections := model.SystemSections{}
	set := func(name, content string) { promptSections.Set(name, &content) }

	if options.CustomPrompt != "" {
		set("preamble", options.CustomPrompt)
	} else {
		set("preamble", defaultPromptPreamble)
		visible := []string{}
		for _, name := range options.SelectedTools {
			if snippet := options.ToolSnippets[name]; snippet != "" {
				visible = append(visible, "- "+name+": "+snippet)
			}
		}
		tools := "(none)"
		if len(visible) > 0 {
			tools = strings.Join(visible, "\n")
		}
		set("tools", tools+"\n\nIn addition to the tools above, you may have access to other custom tools depending on the project.")
		set("rules", buildRules(options.SelectedTools, options.ToolGuidelines, options.PromptGuidelines))
		set("docs", docsSection(options))
	}

	if options.AppendSystemPrompt != "" {
		set("addendum", options.AppendSystemPrompt)
	}
	if len(options.ContextFiles) > 0 {
		set("project_context", renderProjectContext(options.ContextFiles))
	}
	skillFileReadTool := ""
	switch {
	case contains(options.SelectedTools, "read"):
		skillFileReadTool = "read"
	case contains(options.SelectedTools, "bash"):
		skillFileReadTool = "bash"
	}
	if skillFileReadTool != "" && len(options.Skills) > 0 {
		if skills := strings.TrimSpace(FormatSkillsForPromptWithTool(options.Skills, skillFileReadTool)); skills != "" {
			set("skills", skills)
		}
	}
	set("cwd", strings.ReplaceAll(options.Cwd, `\`, "/"))

	for _, section := range options.Sections.Entries() {
		if section.Value != nil && *section.Value != "" {
			set(section.Name, *section.Value)
		}
	}

	entries := promptSections.Entries()
	sections := make(model.SystemSections, len(entries))
	for i, section := range entries {
		content := ""
		if section.Value != nil {
			content = *section.Value
		}
		if section.Name != "preamble" {
			content = "<" + section.Name + ">\n" + content + "\n</" + section.Name + ">"
		}
		sections[i] = model.SystemSection{Name: section.Name, Value: &content}
	}
	return sections, nil
}

func contains(values []string, target string) bool {
	return slices.Contains(values, target)
}

// docsSection is the default prompt's pi documentation section.
func docsSection(options BuildSystemPromptOptions) string {
	readmePath := options.ReadmePath
	if readmePath == "" {
		readmePath = ReadmePath()
	}
	docsPath := options.DocsPath
	if docsPath == "" {
		docsPath = DocsPath()
	}
	examplesPath := options.ExamplesPath
	if examplesPath == "" {
		examplesPath = ExamplesPath()
	}
	return `Pi documentation (read only when the user asks about pi itself, its SDK, extensions, themes, skills, or TUI):
- Main documentation: ` + readmePath + `
- Additional docs: ` + docsPath + `
- Examples: ` + examplesPath + ` (extensions, custom tools, SDK)
- When reading pi docs or examples, resolve docs/... under Additional docs and examples/... under Examples, not the current working directory
- When asked about: extensions (docs/extensions.md, examples/extensions/), themes (docs/themes.md), skills (docs/skills.md), prompt templates (docs/prompt-templates.md), TUI components (docs/tui.md), keybindings (docs/keybindings.md), SDK integrations (docs/sdk.md), custom providers (docs/custom-provider.md), adding models (docs/models.md), pi packages (docs/packages.md), environment variables (docs/environment-variables.md)
- When working on pi topics, read the docs and examples, and follow .md cross-references before implementing
- Always read pi .md files completely and follow links to related docs (e.g., tui.md for TUI API details)`
}

// BuildSystemPromptState returns the complete prompt state for input. A
// non-nil ForceSystemPrompt is the whole content, verbatim.
func BuildSystemPromptState(input BuildSystemPromptOptions) (SystemPromptState, error) {
	if input.ForceSystemPrompt != nil {
		return SystemPromptState{Content: *input.ForceSystemPrompt}, nil
	}
	sections, err := BuildSystemPromptSections(input)
	if err != nil {
		return SystemPromptState{}, err
	}
	return SystemPromptState{Sections: sections}, nil
}

// BuildSystemPrompt renders the full system prompt text.
func BuildSystemPrompt(input BuildSystemPromptOptions) (string, error) {
	state, err := BuildSystemPromptState(input)
	if err != nil {
		return "", err
	}
	message := model.NewSystemText(state.Content, 0)
	message.Sections = state.Sections
	return model.GetSystemMessageText(message), nil
}

// DiffSystemPromptSections returns the patch from previous sections to
// current ones, or false when nothing changed.
func DiffSystemPromptSections(previous, current model.SystemSections) (model.SystemSections, bool) {
	patch := model.SystemSections{}
	for _, section := range current.Entries() {
		if previousValue, ok := previous.Get(section.Name); !ok || !sameSectionValue(previousValue, section.Value) {
			patch.Set(section.Name, section.Value)
		}
	}
	for _, section := range previous.Entries() {
		if _, ok := current.Get(section.Name); !ok {
			patch.Set(section.Name, nil)
		}
	}
	if len(patch) == 0 {
		return nil, false
	}
	return patch, true
}

func sameSectionValue(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

// PackageDir returns the pi package root directory. It honors PI_PACKAGE_DIR,
// else walks up from the executable until a package.json is found, else falls
// back to the executable's directory.
func PackageDir() string {
	if env := os.Getenv("PI_PACKAGE_DIR"); env != "" {
		return env
	}
	exe, err := os.Executable()
	if err != nil {
		return "."
	}
	return findNodePackageDir(filepath.Dir(exe))
}

func findNodePackageDir(start string) string {
	for dir := start; dir != filepath.Dir(dir); dir = filepath.Dir(dir) {
		if !fileExists(filepath.Join(dir, "package.json")) {
			continue
		}
		if filepath.Base(dir) == "dist" {
			if parent := filepath.Dir(dir); fileExists(filepath.Join(parent, "package.json")) {
				return parent
			}
		}
		return dir
	}
	return start
}

// ReadmePath returns the absolute path to the pi package README.md.
func ReadmePath() string {
	path, _ := filepath.Abs(filepath.Join(PackageDir(), "README.md"))
	return path
}

// DocsPath returns the absolute path to the pi package docs directory.
func DocsPath() string {
	path, _ := filepath.Abs(filepath.Join(PackageDir(), "docs"))
	return path
}

// ExamplesPath returns the absolute path to the pi package examples directory.
func ExamplesPath() string {
	path, _ := filepath.Abs(filepath.Join(PackageDir(), "examples"))
	return path
}
