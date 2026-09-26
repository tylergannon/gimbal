package resources

import (
	"strings"

	"gopkg.in/yaml.v3"
)

// ParsedFrontmatter is a parsed `--- ... ---` header and the body that
// follows it.
type ParsedFrontmatter struct {
	Frontmatter map[string]any
	Body        string
}

// normalizeNewlines converts CRLF and lone CR line endings to LF.
func normalizeNewlines(value string) string {
	return strings.ReplaceAll(strings.ReplaceAll(value, "\r\n", "\n"), "\r", "\n")
}

// extractFrontmatter splits content into its optional YAML header and the
// trimmed body. It is a direct port of utils/frontmatter.ts extractFrontmatter.
func extractFrontmatter(content string) (yamlString string, hasYAML bool, body string) {
	normalized := normalizeNewlines(stripBOM(content))

	if !strings.HasPrefix(normalized, "---") {
		return "", false, normalized
	}

	endIndex := strings.Index(normalized[3:], "\n---")
	if endIndex == -1 {
		return "", false, normalized
	}
	endIndex += 3

	return normalized[4:endIndex], true, strings.TrimSpace(normalized[endIndex+4:])
}

// ParseFrontmatter parses an optional YAML frontmatter block. Invalid YAML is
// an error, matching the upstream `yaml` parser.
func ParseFrontmatter(content string) (map[string]any, string, error) {
	yamlString, hasYAML, body := extractFrontmatter(content)
	if !hasYAML {
		return map[string]any{}, body, nil
	}

	parsed := map[string]any{}
	if strings.TrimSpace(yamlString) != "" {
		if err := yaml.Unmarshal([]byte(yamlString), &parsed); err != nil {
			return nil, body, err
		}
	}
	if parsed == nil {
		parsed = map[string]any{}
	}
	return parsed, body, nil
}

// StripFrontmatter returns content without its frontmatter header.
func StripFrontmatter(content string) string {
	_, body, err := ParseFrontmatter(content)
	if err != nil {
		return normalizeNewlines(stripBOM(content))
	}
	return body
}
