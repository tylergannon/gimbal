package config

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"
)

// ResolveConfigValue resolves configuration values that may be shell commands,
// environment templates, or literals. It is a port of resolve-config-value.ts.
//
//   - A value starting with "!" runs the rest as a shell command and uses the
//     trimmed stdout. Command results are cached for the process lifetime.
//   - "$VAR" and "${VAR}" interpolate an environment variable. The explicit
//     env map takes precedence over the process environment.
//   - "$$" escapes a literal "$" and "$!" escapes a literal "!".
//   - Anything else is a literal.

var envVarNameRE = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
var envVarNamePrefixRE = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*`)

const commandResolutionTimeout = 10 * time.Second

type templatePart struct {
	literal bool
	value   string
	name    string
}

type configValueReference struct {
	command  bool
	config   string
	template []templatePart
}

var commandCache = struct {
	sync.Mutex
	results map[string]*string
}{results: map[string]*string{}}

func appendLiteral(parts []templatePart, value string) []templatePart {
	if value == "" {
		return parts
	}
	if len(parts) > 0 && parts[len(parts)-1].literal {
		parts[len(parts)-1].value += value
		return parts
	}
	return append(parts, templatePart{literal: true, value: value})
}

func parseConfigValueTemplate(config string) []templatePart {
	var parts []templatePart
	index := 0
	for index < len(config) {
		dollarIndex := strings.Index(config[index:], "$")
		if dollarIndex < 0 {
			parts = appendLiteral(parts, config[index:])
			break
		}
		dollarIndex += index
		parts = appendLiteral(parts, config[index:dollarIndex])
		var nextChar byte
		if dollarIndex+1 < len(config) {
			nextChar = config[dollarIndex+1]
		}

		if nextChar == '$' || nextChar == '!' {
			parts = appendLiteral(parts, string(nextChar))
			index = dollarIndex + 2
			continue
		}

		if nextChar == '{' {
			endRel := strings.Index(config[dollarIndex+2:], "}")
			if endRel < 0 {
				parts = appendLiteral(parts, "$")
				index = dollarIndex + 1
				continue
			}
			endIndex := dollarIndex + 2 + endRel
			name := config[dollarIndex+2 : endIndex]
			if envVarNameRE.MatchString(name) {
				parts = append(parts, templatePart{name: name})
			} else {
				parts = appendLiteral(parts, config[dollarIndex:endIndex+1])
			}
			index = endIndex + 1
			continue
		}

		match := envVarNamePrefixRE.FindString(config[dollarIndex+1:])
		if match != "" {
			parts = append(parts, templatePart{name: match})
			index = dollarIndex + 1 + len(match)
			continue
		}

		parts = appendLiteral(parts, "$")
		index = dollarIndex + 1
	}
	return parts
}

func parseConfigValueReference(config string) configValueReference {
	if strings.HasPrefix(config, "!") {
		return configValueReference{command: true, config: config}
	}
	return configValueReference{template: parseConfigValueTemplate(config)}
}

func resolveEnvConfigValue(name string, env map[string]string) (string, bool) {
	if v := env[name]; v != "" {
		return v, true
	}
	if v := os.Getenv(name); v != "" {
		return v, true
	}
	return "", false
}

func templateEnvVarNames(parts []templatePart) []string {
	var names []string
	for _, part := range parts {
		if part.literal {
			continue
		}
		found := slices.Contains(names, part.name)
		if !found {
			names = append(names, part.name)
		}
	}
	return names
}

func resolveTemplate(parts []templatePart, env map[string]string) (string, bool) {
	var resolved strings.Builder
	for _, part := range parts {
		if part.literal {
			resolved.WriteString(part.value)
			continue
		}
		value, ok := resolveEnvConfigValue(part.name, env)
		if !ok {
			return "", false
		}
		resolved.WriteString(value)
	}
	return resolved.String(), true
}

// GetConfigValueEnvVarName returns the single environment variable a template
// references, or "" when the config is a command or a multi-part template.
func GetConfigValueEnvVarName(config string) string {
	reference := parseConfigValueReference(config)
	if reference.command || len(reference.template) != 1 || reference.template[0].literal {
		return ""
	}
	return reference.template[0].name
}

// GetConfigValueEnvVarNames returns every environment variable a template
// references, in first-seen order.
func GetConfigValueEnvVarNames(config string) []string {
	reference := parseConfigValueReference(config)
	if reference.command {
		return nil
	}
	return templateEnvVarNames(reference.template)
}

// GetMissingConfigValueEnvVarNames returns the referenced environment
// variables that neither env nor the process environment provides.
func GetMissingConfigValueEnvVarNames(config string, env map[string]string) []string {
	var missing []string
	for _, name := range GetConfigValueEnvVarNames(config) {
		if _, ok := resolveEnvConfigValue(name, env); !ok {
			missing = append(missing, name)
		}
	}
	return missing
}

// IsCommandConfigValue reports whether the value is a shell command.
func IsCommandConfigValue(config string) bool {
	return parseConfigValueReference(config).command
}

// IsConfigValueConfigured reports whether every referenced environment
// variable is present.
func IsConfigValueConfigured(config string, env map[string]string) bool {
	return len(GetMissingConfigValueEnvVarNames(config, env)) == 0
}

// ResolveConfigValue resolves a config value, using the process-lifetime
// command cache for command values.
func ResolveConfigValue(config string, env map[string]string) (string, bool) {
	reference := parseConfigValueReference(config)
	if reference.command {
		return executeCommand(reference.config)
	}
	return resolveTemplate(reference.template, env)
}

// ResolveConfigValueUncached resolves a config value without consulting or
// updating the command cache.
func ResolveConfigValueUncached(config string, env map[string]string) (string, bool) {
	reference := parseConfigValueReference(config)
	if reference.command {
		return executeCommandUncached(reference.config)
	}
	return resolveTemplate(reference.template, env)
}

// ResolveConfigValueOrThrow resolves a value or returns an error naming the
// description and the missing environment variable or failed command.
func ResolveConfigValueOrThrow(config, description string, env map[string]string) (string, error) {
	resolved, ok := ResolveConfigValueUncached(config, env)
	if ok {
		return resolved, nil
	}
	reference := parseConfigValueReference(config)
	if reference.command {
		return "", &ResolveError{Description: description, Command: reference.config[1:]}
	}
	if missing := GetMissingConfigValueEnvVarNames(config, env); len(missing) == 1 {
		return "", &ResolveError{Description: description, EnvVar: missing[0]}
	} else if len(missing) > 1 {
		return "", &ResolveError{Description: description, EnvVars: missing}
	}
	return "", &ResolveError{Description: description}
}

// ResolveError describes why a config value could not be resolved.
type ResolveError struct {
	Description string
	Command     string
	EnvVar      string
	EnvVars     []string
}

func (e *ResolveError) Error() string {
	if e.Command != "" {
		return "Failed to resolve " + e.Description + " from shell command: " + e.Command
	}
	if e.EnvVar != "" {
		return "Failed to resolve " + e.Description + " from environment variable: " + e.EnvVar
	}
	if len(e.EnvVars) > 0 {
		return "Failed to resolve " + e.Description + " from environment variables: " + strings.Join(e.EnvVars, ", ")
	}
	return "Failed to resolve " + e.Description
}

// ResolveHeaders resolves every header value, dropping headers that resolve to
// nothing. It returns nil when no header survives.
func ResolveHeaders(headers map[string]string, env map[string]string) map[string]string {
	if headers == nil {
		return nil
	}
	resolved := map[string]string{}
	for key, value := range headers {
		if resolvedValue, ok := ResolveConfigValue(value, env); ok && resolvedValue != "" {
			resolved[key] = resolvedValue
		}
	}
	if len(resolved) == 0 {
		return nil
	}
	return resolved
}

// ResolveHeadersOrThrow resolves every header value, returning the first
// resolution error.
func ResolveHeadersOrThrow(headers map[string]string, description string, env map[string]string) (map[string]string, error) {
	if headers == nil {
		return nil, nil
	}
	resolved := map[string]string{}
	for key, value := range headers {
		resolvedValue, err := ResolveConfigValueOrThrow(value, description+" header \""+key+"\"", env)
		if err != nil {
			return nil, err
		}
		resolved[key] = resolvedValue
	}
	if len(resolved) == 0 {
		return nil, nil
	}
	return resolved, nil
}

// ClearConfigValueCache clears the shell-command result cache.
func ClearConfigValueCache() {
	commandCache.Lock()
	defer commandCache.Unlock()
	commandCache.results = map[string]*string{}
}

func executeCommand(commandConfig string) (string, bool) {
	commandCache.Lock()
	if value, ok := commandCache.results[commandConfig]; ok {
		commandCache.Unlock()
		if value == nil {
			return "", false
		}
		return *value, true
	}
	commandCache.Unlock()

	value, ok := executeCommandUncached(commandConfig)

	commandCache.Lock()
	defer commandCache.Unlock()
	if ok {
		commandCache.results[commandConfig] = &value
	} else {
		commandCache.results[commandConfig] = nil
	}
	return value, ok
}

func executeCommandUncached(commandConfig string) (string, bool) {
	return executeWithDefaultShell(commandConfig[1:])
}

// executeWithConfiguredShell runs a command with bash -c, reporting whether
// the shell itself started (the configured-shell branch of the Windows path).
func executeWithConfiguredShell(command string) (string, bool, bool) {
	shell := "bash"
	if _, err := exec.LookPath(shell); err != nil {
		return "", false, false
	}
	value, ok := runShell(shell, []string{"-c", command})
	return value, ok, true
}

func executeWithDefaultShell(command string) (string, bool) {
	if runtime.GOOS == "windows" {
		if value, ok, executed := executeWithConfiguredShell(command); executed {
			return value, ok
		}
		return runShell("cmd", []string{"/c", command})
	}
	if _, err := os.Stat("/bin/sh"); err == nil {
		return runShell("/bin/sh", []string{"-c", command})
	}
	return runShell("sh", []string{"-c", command})
}

func runShell(shell string, args []string) (string, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), commandResolutionTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, shell, args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		return "", false
	}
	value := strings.TrimSpace(out.String())
	if value == "" {
		return "", false
	}
	return value, true
}
