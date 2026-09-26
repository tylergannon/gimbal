package resources

import (
	"os"
	"path/filepath"
	"strings"
)

var skillIgnoreFileNames = []string{".gitignore", ".ignore", ".fdignore"}

// skillIgnore accumulates gitignore-style rules from ignore files found while
// descending a resource tree. It ports the `ignore` matcher behavior skills.ts
// relies on: the last matching rule wins, and a negated rule un-ignores.
type skillIgnore struct {
	rules []skillIgnoreRule
	seen  map[string]bool
}

type skillIgnoreRule struct {
	pattern  string
	negated  bool
	dirOnly  bool
	anchored bool
	matchAll bool
}

func newSkillIgnore() *skillIgnore {
	return &skillIgnore{seen: map[string]bool{}}
}

// addRules loads the ignore files in dir, prefixing each pattern with dir's
// path relative to root.
func (ig *skillIgnore) addRules(dir, root string) {
	if ig.seen[dir] {
		return
	}
	ig.seen[dir] = true

	rel := relativePath(root, dir)
	prefix := ""
	if rel != "." && rel != "" {
		prefix = toPosixPath(rel) + "/"
	}

	for _, filename := range skillIgnoreFileNames {
		data, err := os.ReadFile(filepath.Join(dir, filename))
		if err != nil {
			continue
		}
		for line := range strings.SplitSeq(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n") {
			pattern, ok := prefixIgnorePattern(line, prefix)
			if !ok {
				continue
			}
			if rule, ok := newSkillIgnoreRule(pattern); ok {
				ig.rules = append(ig.rules, rule)
			}
		}
	}
}

// prefixIgnorePattern ports skills.ts prefixIgnorePattern.
func prefixIgnorePattern(line, prefix string) (string, bool) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return "", false
	}
	if strings.HasPrefix(trimmed, "#") && !strings.HasPrefix(trimmed, `\#`) {
		return "", false
	}

	pattern := line
	negated := false
	if strings.HasPrefix(pattern, "!") {
		negated = true
		pattern = pattern[1:]
	} else if strings.HasPrefix(pattern, `\!`) {
		pattern = pattern[1:]
	}
	pattern = strings.TrimPrefix(pattern, "/")
	prefixed := prefix + pattern
	if negated {
		return "!" + prefixed, true
	}
	return prefixed, prefixed != ""
}

// newSkillIgnoreRule compiles one pattern. See the donor notes for the exact
// whitespace steps; this keeps the behavior that matters for resource trees.
func newSkillIgnoreRule(pattern string) (skillIgnoreRule, bool) {
	if strings.Trim(pattern, " ") == "" || hasLoneTrailingBackslash(pattern) || strings.HasPrefix(pattern, "#") {
		return skillIgnoreRule{}, false
	}
	var rule skillIgnoreRule
	body := pattern
	if strings.HasPrefix(body, "!") {
		rule.negated = true
		body = body[1:]
	}
	rule.anchored = body != "" && strings.Contains(body[:len(body)-1], "/")

	body = ignoreRuleBody(body)
	if body == "" {
		rule.matchAll = true
		return rule, true
	}
	if strings.HasPrefix(body, "/") {
		rule.anchored = true
		body = body[1:]
	}
	rule.dirOnly = strings.HasSuffix(body, "/")
	rule.pattern = strings.TrimSuffix(body, "/")
	if rule.pattern == "" {
		return skillIgnoreRule{}, false
	}
	return rule, true
}

func hasLoneTrailingBackslash(value string) bool {
	return strings.HasSuffix(value, `\`) && !strings.HasSuffix(value, `\\`)
}

// ignoreRuleBody applies the whitespace steps of the ignore matcher's rule
// compiler.
func ignoreRuleBody(body string) string {
	body = strings.TrimPrefix(body, "\ufeff")

	if kept := strings.TrimRight(body, "\r\n"); kept != body {
		if endsInEscape(kept) {
			return body[:len(kept)+1]
		}
		body = kept
	}

	if kept := strings.TrimRight(body, " "); kept != body {
		if endsInEscape(kept) {
			return kept[:len(kept)-1] + " "
		}
		body = kept
	}
	return body
}

func endsInEscape(value string) bool {
	return (len(value)-len(strings.TrimRight(value, `\`)))%2 == 1
}

// ignores reports whether the root-relative posix path is ignored.
func (ig *skillIgnore) ignores(relPosix string, isDir bool) bool {
	relPosix = strings.TrimSuffix(relPosix, "/")
	ignored := false
	for _, rule := range ig.rules {
		if rule.dirOnly && !isDir {
			continue
		}
		if rule.matchAll || gitignoreMatchPath(rule.pattern, rule.anchored, relPosix) {
			ignored = !rule.negated
		}
	}
	return ignored
}

// gitignoreMatchPath reports whether a root-relative posix path matches a
// gitignore pattern.
func gitignoreMatchPath(pattern string, anchored bool, path string) bool {
	if !anchored {
		base := path
		if _, after, ok := strings.CutLast(path, "/"); ok {
			base = after
		}
		if ok, _ := filepath.Match(pattern, base); ok {
			return true
		}
		for segment := range strings.SplitSeq(path, "/") {
			if ok, _ := filepath.Match(pattern, segment); ok {
				return true
			}
		}
		return false
	}
	if ok, _ := filepath.Match(pattern, path); ok {
		return true
	}
	return strings.HasPrefix(path, pattern+"/")
}
