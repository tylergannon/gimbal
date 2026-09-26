package resources

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"github.com/tylergannon/gimbal/internal/pi/config"
)

// PromptTemplate is a slash-command prompt loaded from a markdown file.
type PromptTemplate struct {
	Name         string
	Description  string
	ArgumentHint string
	Content      string
	SourceInfo   SourceInfo
	FilePath     string
}

// LoadPromptTemplatesOptions configures LoadPromptTemplates.
type LoadPromptTemplatesOptions struct {
	// Cwd is the working directory for project-local templates.
	Cwd string
	// AgentDir is the global config directory.
	AgentDir string
	// PromptPaths are explicit files or directories.
	PromptPaths []string
	// IncludeDefaults loads agentDir/prompts and cwd/.pi/prompts.
	IncludeDefaults bool
}

// LoadPromptTemplatesResult is the outcome of a prompt-template load.
type LoadPromptTemplatesResult struct {
	Templates   []PromptTemplate
	Diagnostics []ResourceDiagnostic
}

// ParseCommandArgs parses command arguments, respecting single and double
// quotes. Empty quoted strings are dropped, matching pi.
func ParseCommandArgs(argsString string) []string {
	args := []string{}
	current := strings.Builder{}
	var inQuote rune
	hasQuote := false

	for _, char := range argsString {
		if hasQuote {
			if char == inQuote {
				hasQuote = false
			} else {
				current.WriteRune(char)
			}
			continue
		}
		switch {
		case char == '"' || char == '\'':
			inQuote = char
			hasQuote = true
		case unicode.IsSpace(char):
			if current.Len() > 0 {
				args = append(args, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(char)
		}
	}
	if current.Len() > 0 {
		args = append(args, current.String())
	}
	return args
}

// substituteArgsPattern is pi's substituteArgs regex, unchanged.
var substituteArgsPattern = regexp.MustCompile(
	`\$\{(\d+|ARGUMENTS|@):-([^}]*)\}|\$\{@:(\d+)(?::(\d+))?\}|\$(ARGUMENTS|@|\d+)`,
)

// SubstituteArgs substitutes argument placeholders in template content.
// Replacement is single-pass: patterns inside argument or default values are
// not re-substituted.
func SubstituteArgs(content string, args []string) string {
	allArgs := strings.Join(args, " ")
	matches := substituteArgsPattern.FindAllStringSubmatchIndex(content, -1)
	if len(matches) == 0 {
		return content
	}

	var out strings.Builder
	last := 0
	for _, match := range matches {
		out.WriteString(content[last:match[0]])
		last = match[1]

		group := func(index int) (string, bool) {
			start, end := match[index*2], match[index*2+1]
			if start < 0 {
				return "", false
			}
			return content[start:end], true
		}

		if defaultTarget, ok := group(1); ok {
			defaultValue, _ := group(2)
			value := allArgs
			if defaultTarget != "@" && defaultTarget != "ARGUMENTS" {
				index := parseInt(defaultTarget) - 1
				if index >= 0 && index < len(args) {
					value = args[index]
				} else {
					value = ""
				}
			}
			if value != "" {
				out.WriteString(value)
			} else {
				out.WriteString(defaultValue)
			}
			continue
		}

		if sliceStart, ok := group(3); ok {
			start := min(max(parseInt(sliceStart)-1, 0), len(args))
			end := len(args)
			if sliceLength, hasLength := group(4); hasLength && sliceLength != "" {
				end = min(start+parseInt(sliceLength), len(args))
			}
			out.WriteString(strings.Join(args[start:end], " "))
			continue
		}

		simple, _ := group(5)
		if simple == "ARGUMENTS" || simple == "@" {
			out.WriteString(allArgs)
			continue
		}
		index := parseInt(simple) - 1
		if index >= 0 && index < len(args) {
			out.WriteString(args[index])
		}
	}
	out.WriteString(content[last:])
	return out.String()
}

// parseInt parses a non-negative decimal prefix, matching JS parseInt for the
// digit runs the pattern guarantees.
func parseInt(value string) int {
	result := 0
	for _, char := range value {
		if char < '0' || char > '9' {
			break
		}
		result = result*10 + int(char-'0')
	}
	return result
}

// templateLoadResult is one template file's load result.
type templateLoadResult struct {
	Template    *PromptTemplate
	Diagnostics []ResourceDiagnostic
}

// loadTemplateFromFile loads one prompt template.
func loadTemplateFromFile(filePath string, sourceInfo SourceInfo) templateLoadResult {
	diagnostics := []ResourceDiagnostic{}

	data, err := os.ReadFile(filePath)
	if err != nil {
		diagnostics = append(diagnostics, ResourceDiagnostic{
			Type: DiagnosticWarning, Message: err.Error(), Path: filePath,
		})
		return templateLoadResult{Diagnostics: diagnostics}
	}

	frontmatter, body, err := ParseFrontmatter(string(data))
	if err != nil {
		diagnostics = append(diagnostics, ResourceDiagnostic{
			Type: DiagnosticWarning, Message: err.Error(), Path: filePath,
		})
		return templateLoadResult{Diagnostics: diagnostics}
	}

	name := strings.TrimSuffix(filepath.Base(filePath), ".md")

	description, _ := stringValue(frontmatter["description"])
	if description == "" {
		for line := range strings.SplitSeq(body, "\n") {
			if strings.TrimSpace(line) == "" {
				continue
			}
			description = line
			if len([]rune(line)) > 60 {
				description = string([]rune(line)[:60]) + "..."
			}
			break
		}
	}

	argumentHint, _ := stringValue(frontmatter["argument-hint"])

	template := &PromptTemplate{
		Name:         name,
		Description:  description,
		ArgumentHint: argumentHint,
		Content:      body,
		SourceInfo:   sourceInfo,
		FilePath:     filePath,
	}
	return templateLoadResult{Template: template, Diagnostics: diagnostics}
}

// loadTemplatesFromDir scans a directory (non-recursively) for .md files.
func loadTemplatesFromDir(dir string, getSourceInfo func(string) SourceInfo) LoadPromptTemplatesResult {
	templates := []PromptTemplate{}
	diagnostics := []ResourceDiagnostic{}

	if !dirExists(dir) {
		return LoadPromptTemplatesResult{Templates: templates, Diagnostics: diagnostics}
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return LoadPromptTemplatesResult{Templates: templates, Diagnostics: diagnostics}
	}

	for _, entry := range entries {
		fullPath := filepath.Join(dir, entry.Name())
		isFile := entry.Type().IsRegular()
		if entry.Type()&os.ModeSymlink != 0 {
			info, err := os.Stat(fullPath)
			if err != nil {
				continue
			}
			isFile = info.Mode().IsRegular()
		}
		if !isFile || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		result := loadTemplateFromFile(fullPath, getSourceInfo(fullPath))
		if result.Template != nil {
			templates = append(templates, *result.Template)
		}
		diagnostics = append(diagnostics, result.Diagnostics...)
	}

	return LoadPromptTemplatesResult{Templates: templates, Diagnostics: diagnostics}
}

// LoadPromptTemplates loads prompt templates from the global and project
// default directories and any explicit paths.
func LoadPromptTemplates(options LoadPromptTemplatesOptions) LoadPromptTemplatesResult {
	resolvedCwd := resolvePath(options.Cwd, ".")
	resolvedAgentDir := resolvePath(options.AgentDir, ".")

	templates := []PromptTemplate{}
	diagnostics := []ResourceDiagnostic{}
	addResult := func(result LoadPromptTemplatesResult) {
		templates = append(templates, result.Templates...)
		diagnostics = append(diagnostics, result.Diagnostics...)
	}

	globalPromptsDir := filepath.Join(resolvedAgentDir, "prompts")
	projectPromptsDir := filepath.Join(resolvedCwd, config.ConfigDirName, "prompts")

	getSourceInfo := func(resolvedPath string) SourceInfo {
		if isUnderPath(resolvedPath, globalPromptsDir) {
			return CreateSyntheticSourceInfo(resolvedPath, SyntheticSourceInfoOptions{
				Source: "local", Scope: ScopeUser, BaseDir: globalPromptsDir,
			})
		}
		if isUnderPath(resolvedPath, projectPromptsDir) {
			return CreateSyntheticSourceInfo(resolvedPath, SyntheticSourceInfoOptions{
				Source: "local", Scope: ScopeProject, BaseDir: projectPromptsDir,
			})
		}
		baseDir := filepath.Dir(resolvedPath)
		if dirExists(resolvedPath) {
			baseDir = resolvedPath
		}
		return CreateSyntheticSourceInfo(resolvedPath, SyntheticSourceInfoOptions{Source: "local", BaseDir: baseDir})
	}

	if options.IncludeDefaults {
		addResult(loadTemplatesFromDir(globalPromptsDir, getSourceInfo))
		addResult(loadTemplatesFromDir(projectPromptsDir, getSourceInfo))
	}

	for _, rawPath := range options.PromptPaths {
		resolvedPath := resolvePath(rawPath, resolvedCwd)
		if !pathExists(resolvedPath) {
			continue
		}

		info, err := os.Stat(resolvedPath)
		if err != nil {
			diagnostics = append(diagnostics, ResourceDiagnostic{
				Type: DiagnosticWarning, Message: err.Error(), Path: resolvedPath,
			})
			continue
		}
		if info.IsDir() {
			addResult(loadTemplatesFromDir(resolvedPath, getSourceInfo))
		} else if info.Mode().IsRegular() && strings.HasSuffix(resolvedPath, ".md") {
			result := loadTemplateFromFile(resolvedPath, getSourceInfo(resolvedPath))
			if result.Template != nil {
				templates = append(templates, *result.Template)
			}
			diagnostics = append(diagnostics, result.Diagnostics...)
		}
	}

	return LoadPromptTemplatesResult{Templates: templates, Diagnostics: diagnostics}
}

// expandPattern matches a leading slash command and captures the argument
// string, including newlines.
var expandPattern = regexp.MustCompile(`^/([^\s]+)(?:\s+([\s\S]*))?$`)

// ExpandPromptTemplate expands text when it names a known template, otherwise
// returns it unchanged.
func ExpandPromptTemplate(text string, templates []PromptTemplate) string {
	if !strings.HasPrefix(text, "/") {
		return text
	}

	match := expandPattern.FindStringSubmatch(text)
	if match == nil {
		return text
	}
	templateName := match[1]
	argsString := match[2]

	for _, template := range templates {
		if template.Name == templateName {
			args := ParseCommandArgs(argsString)
			return SubstituteArgs(template.Content, args)
		}
	}
	return text
}
