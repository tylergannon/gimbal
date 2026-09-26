package files

import (
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"golang.org/x/text/unicode/norm"
)

// unicodeSpaces matches the Unicode space variants normalized to a regular
// space (paths.ts UNICODE_SPACES: U+00A0, U+2000-U+200A, U+202F, U+205F,
// U+3000).
var unicodeSpaces = regexp.MustCompile("[\u00A0\u2000-\u200A\u202F\u205F\u3000]")

// PathInputOptions controls path normalization. A nil ExpandTilde defaults to
// true, matching the upstream `expandTilde ?? true`.
type PathInputOptions struct {
	// Trim trims leading/trailing whitespace before normalization.
	Trim bool
	// ExpandTilde expands a leading `~` to a home directory. Nil defaults to true.
	ExpandTilde *bool
	// HomeDir is the home directory used for `~` expansion. Defaults to os.UserHomeDir().
	HomeDir string
	// StripAtPrefix strips a leading `@`, used for CLI @file paths.
	StripAtPrefix bool
	// NormalizeUnicodeSpaces normalizes Unicode space variants to regular spaces.
	NormalizeUnicodeSpaces bool
}

// CanonicalizePath resolves symlinks, falling back to the input when the path
// cannot be resolved (e.g. it does not exist yet).
func CanonicalizePath(path string) string {
	if real, err := filepath.EvalSymlinks(path); err == nil {
		return real
	}
	return path
}

// IsLocalPath reports whether value is not a package source or remote URL
// protocol. Bare names, relative paths, and file: URLs are considered local.
func IsLocalPath(value string) bool {
	trimmed := jsTrim(value)
	switch {
	case strings.HasPrefix(trimmed, "npm:"),
		strings.HasPrefix(trimmed, "git:"),
		strings.HasPrefix(trimmed, "github:"),
		strings.HasPrefix(trimmed, "http:"),
		strings.HasPrefix(trimmed, "https:"),
		strings.HasPrefix(trimmed, "ssh:"):
		return false
	}
	return true
}

// normalizeWindowsShellPath converts Git Bash, MSYS, Cygwin, and WSL drive
// paths to a form native Windows APIs accept.
func normalizeWindowsShellPath(filePath string) string {
	if !strings.HasPrefix(filePath, "/") || strings.HasPrefix(filePath, "//") || strings.Contains(filePath, "\\") {
		return filePath
	}
	re := regexp.MustCompile(`^/(?:mnt/|cygdrive/)?([a-zA-Z])(?:/(.*))?$`)
	match := re.FindStringSubmatch(filePath)
	if match == nil {
		return filePath
	}
	suffix := ""
	if match[2] != "" {
		suffix = strings.ReplaceAll(match[2], "/", "\\")
	}
	return strings.ToUpper(match[1]) + ":\\" + suffix
}

// NormalizePath ports pi's normalizePath. Options select trimming, unicode
// space folding, `@` stripping, tilde expansion, and file: URL decoding.
func NormalizePath(input string, options PathInputOptions) string {
	normalized := input
	if options.Trim {
		normalized = jsTrim(normalized)
	}
	if options.NormalizeUnicodeSpaces {
		normalized = unicodeSpaces.ReplaceAllString(normalized, " ")
	}
	if options.StripAtPrefix && strings.HasPrefix(normalized, "@") {
		normalized = normalized[1:]
	}
	if runtime.GOOS == "windows" {
		normalized = normalizeWindowsShellPath(normalized)
	}

	expandTilde := true
	if options.ExpandTilde != nil {
		expandTilde = *options.ExpandTilde
	}
	if expandTilde {
		home := options.HomeDir
		if home == "" {
			home, _ = os.UserHomeDir()
		}
		if normalized == "~" {
			return home
		}
		if strings.HasPrefix(normalized, "~/") || (runtime.GOOS == "windows" && strings.HasPrefix(normalized, "~\\")) {
			return filepath.Join(home, normalized[2:])
		}
	}

	if strings.HasPrefix(normalized, "file://") {
		if u, err := url.Parse(normalized); err == nil {
			return u.Path
		}
	}

	return normalized
}

// ResolvePath normalizes input and resolves it against baseDir when it is not
// absolute.
func ResolvePath(input, baseDir string, options PathInputOptions) string {
	if baseDir == "" {
		baseDir, _ = os.Getwd()
	}
	normalized := NormalizePath(input, options)
	normalizedBaseDir := NormalizePath(baseDir, PathInputOptions{})
	if filepath.IsAbs(normalized) {
		return filepath.Clean(normalized)
	}
	return filepath.Clean(filepath.Join(normalizedBaseDir, normalized))
}

// pathInputOptions is the option set used by expandPath and resolveToCwd.
var pathInputOptions = PathInputOptions{NormalizeUnicodeSpaces: true, StripAtPrefix: true}

// ExpandPath normalizes a user-supplied path: unicode spaces folded, a leading
// `@` stripped, and a leading `~` expanded.
func ExpandPath(filePath string) string {
	return NormalizePath(filePath, pathInputOptions)
}

// ResolveToCwd resolves a path relative to cwd, handling ~ expansion, unicode
// spaces, and a leading `@`, and absolute paths.
func ResolveToCwd(filePath, cwd string) string {
	return ResolvePath(filePath, cwd, pathInputOptions)
}

// PathExists reports whether the path exists.
func PathExists(filePath string) bool {
	_, err := os.Stat(filePath)
	return err == nil
}

const narrowNoBreakSpace = "\u202F"

var macAMPMRe = regexp.MustCompile(`(?i) (AM|PM)\.`)

func tryMacOSScreenshotPath(filePath string) string {
	return macAMPMRe.ReplaceAllString(filePath, narrowNoBreakSpace+"$1.")
}

func tryNFDVariant(filePath string) string {
	return norm.NFD.String(filePath)
}

func tryCurlyQuoteVariant(filePath string) string {
	return strings.ReplaceAll(filePath, "'", "\u2019")
}

// ResolveReadPath resolves a path for reading and, when the resolved path does
// not exist, tries pi's macOS filename fallbacks in order: narrow no-break
// space before AM/PM, NFD, curly quote, and combined NFD + curly quote.
func ResolveReadPath(filePath, cwd string) string {
	resolved := ResolveToCwd(filePath, cwd)

	if PathExists(resolved) {
		return resolved
	}
	if v := tryMacOSScreenshotPath(resolved); v != resolved && PathExists(v) {
		return v
	}
	nfdVariant := tryNFDVariant(resolved)
	if nfdVariant != resolved && PathExists(nfdVariant) {
		return nfdVariant
	}
	if v := tryCurlyQuoteVariant(resolved); v != resolved && PathExists(v) {
		return v
	}
	if v := tryCurlyQuoteVariant(nfdVariant); v != resolved && PathExists(v) {
		return v
	}
	return resolved
}

// GetCwdRelativePath returns the path of filePath relative to cwd when it is
// inside cwd, or ok=false when it is outside.
func GetCwdRelativePath(filePath, cwd string) (string, bool) {
	resolvedCwd := ResolvePath(cwd, "", PathInputOptions{})
	resolvedPath := ResolvePath(filePath, resolvedCwd, PathInputOptions{})
	rel, err := filepath.Rel(resolvedCwd, resolvedPath)
	if err != nil {
		return "", false
	}
	isInside := rel == "." ||
		(rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel))
	if !isInside {
		return "", false
	}
	if rel == "." {
		return ".", true
	}
	return rel, true
}

// FormatPathRelativeToCwdOrAbsolute returns filePath relative to cwd using
// forward slashes, or the absolute path when it lies outside cwd.
func FormatPathRelativeToCwdOrAbsolute(filePath, cwd string) string {
	absolutePath := ResolvePath(filePath, cwd, PathInputOptions{})
	if rel, ok := GetCwdRelativePath(absolutePath, cwd); ok {
		return strings.ReplaceAll(rel, string(filepath.Separator), "/")
	}
	return strings.ReplaceAll(absolutePath, string(filepath.Separator), "/")
}

// MarkPathIgnoredByCloudSync best-effort marks path ignored by Dropbox and
// iCloud Drive. Failures are ignored, matching the upstream stdio:"ignore"
// spawn.
func MarkPathIgnoredByCloudSync(path string) {
	var attrs []string
	var command string
	switch runtime.GOOS {
	case "darwin":
		attrs = []string{"com.dropbox.ignored", "com.apple.fileprovider.ignore#P"}
		command = "xattr"
	case "linux":
		attrs = []string{"user.com.dropbox.ignored"}
		command = "setfattr"
	default:
		return
	}
	for _, attr := range attrs {
		var args []string
		if runtime.GOOS == "darwin" {
			args = []string{"-w", attr, "1", path}
		} else {
			args = []string{"-n", attr, "-v", "1", path}
		}
		_ = exec.Command(command, args...).Run()
	}
}
