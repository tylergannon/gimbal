package config

import (
	"os"
	"path/filepath"
	"strings"
)

// ConfigDirName is the per-project configuration directory (pi CONFIG_DIR_NAME).
const ConfigDirName = ".pi"

// StripBOM removes a leading UTF-8 byte-order mark from text.
func StripBOM(content string) string {
	return strings.TrimPrefix(content, "\uFEFF")
}

// NormalizePath expands a leading ~ to the user's home directory. It is the
// input normalization pi applies before storing a settings path.
func NormalizePath(input string) string {
	if input == "~" {
		return HomeDir()
	}
	if strings.HasPrefix(input, "~/") || strings.HasPrefix(input, `~\`) {
		return filepath.Join(HomeDir(), input[2:])
	}
	return input
}

// HomeDir returns the current user's home directory, or "" when unavailable.
func HomeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return home
}

// GetAgentDir returns pi's per-user agent directory (~/.pi/agent). An explicit
// PI_CODING_AGENT_DIR (or the configured override) wins.
func GetAgentDir() string {
	if dir := os.Getenv("PI_CODING_AGENT_DIR"); dir != "" {
		return NormalizePath(dir)
	}
	home := HomeDir()
	if home == "" {
		return filepath.Join(ConfigDirName, "agent")
	}
	return filepath.Join(home, ConfigDirName, "agent")
}

// ResolvePath resolves input against baseDir, expanding a leading ~ first.
func ResolvePath(input, baseDir string) string {
	normalized := NormalizePath(input)
	if filepath.IsAbs(normalized) {
		return filepath.Clean(normalized)
	}
	return filepath.Clean(filepath.Join(baseDir, normalized))
}

// StripJSONComments removes // line comments and /* block comments */ from
// JSONC content, leaving string contents untouched.
func StripJSONComments(content string) string {
	var out strings.Builder
	out.Grow(len(content))
	inString := false
	escaped := false
	for i := 0; i < len(content); i++ {
		c := content[i]
		if inString {
			out.WriteByte(c)
			if escaped {
				escaped = false
				continue
			}
			switch c {
			case '\\':
				escaped = true
			case '"':
				inString = false
			}
			continue
		}
		switch c {
		case '"':
			inString = true
			out.WriteByte(c)
		case '/':
			if i+1 < len(content) && content[i+1] == '/' {
				for i < len(content) && content[i] != '\n' {
					i++
				}
				if i < len(content) {
					out.WriteByte('\n')
				}
			} else if i+1 < len(content) && content[i+1] == '*' {
				i += 2
				for i+1 < len(content) && (content[i] != '*' || content[i+1] != '/') {
					i++
				}
				i++
			} else {
				out.WriteByte(c)
			}
		default:
			out.WriteByte(c)
		}
	}
	return out.String()
}
