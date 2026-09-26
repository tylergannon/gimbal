package readtools

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/tylergannon/gimbal/internal/pi/files"
	"github.com/tylergannon/gimbal/internal/pi/model"
)

// grepPromptSnippet mirrors pi's grepToolSystemPromptContribution.
const grepPromptSnippet = "Search file contents for patterns (respects .gitignore)"

// GrepOperations are the file operations the grep tool performs.
type GrepOperations struct {
	// IsDirectory reports whether a path is a directory, erroring if it does
	// not exist.
	IsDirectory func(ctx context.Context, absolutePath string) (bool, error)
	// ReadFile reads a file's contents. NOTE: pi matches with ripgrep and calls
	// readFile only to fetch context lines around a hit; this port matches in
	// Go, so this member is the primary scan read and whatever it returns is
	// what grep can match.
	ReadFile func(ctx context.Context, absolutePath string) ([]byte, error)
}

// GrepToolOptions configure the grep tool.
type GrepToolOptions struct {
	// Operations overrides the local filesystem.
	Operations *GrepOperations
}

// GrepToolDetails is the grep tool's optional details payload.
type GrepToolDetails struct {
	Truncation        *files.Result `json:"truncation,omitempty"`
	MatchLimitReached *int          `json:"matchLimitReached,omitempty"`
	LinesTruncated    bool          `json:"linesTruncated,omitempty"`
}

// DefaultGrepOperations reads the local filesystem.
func DefaultGrepOperations() GrepOperations {
	return GrepOperations{
		IsDirectory: func(_ context.Context, p string) (bool, error) {
			st, err := os.Stat(p)
			if err != nil {
				return false, err
			}
			return st.IsDir(), nil
		},
		ReadFile: func(_ context.Context, p string) ([]byte, error) {
			return os.ReadFile(p)
		},
	}
}

func resolveGrepOperations(ops *GrepOperations) GrepOperations {
	if ops == nil {
		return DefaultGrepOperations()
	}
	return *ops
}

// GrepTool builds the grep tool.
func GrepTool(cwd string, options *GrepToolOptions) model.ToolDefinition {
	var custom *GrepOperations
	if options != nil {
		custom = options.Operations
	}
	ops := resolveGrepOperations(custom)

	return model.ToolDefinition{
		Name:          "grep",
		Label:         "grep",
		Description:   grepDescription(),
		PromptSnippet: grepPromptSnippet,
		Parameters: model.Object(
			model.Prop("pattern", desc(model.String(), "Search pattern (regex or literal string)")),
			model.Opt("path", desc(model.String(), "Directory or file to search (default: current directory)")),
			model.Opt("glob", desc(model.String(), "Filter files by glob pattern, e.g. '*.ts' or '**/*.spec.ts'")),
			model.Opt("ignoreCase", desc(model.Boolean(), "Case-insensitive search (default: false)")),
			model.Opt("literal", desc(model.Boolean(), "Treat pattern as literal string instead of regex (default: false)")),
			model.Opt("context", desc(model.Number(), "Number of lines to show before and after each match (default: 0)")),
			model.Opt("limit", desc(model.Number(), "Maximum number of matches to return (default: 100)")),
		),
		Execute: func(ctx context.Context, _ string, params map[string]any, _ model.ToolUpdateFunc) (model.AgentToolResult, error) {
			if err := ctx.Err(); err != nil {
				return model.AgentToolResult{}, err
			}
			patternStr := argStr(params, "pattern")
			root := cwd
			if p := argStr(params, "path"); p != "" {
				root = files.ResolveToCwd(p, cwd)
			}
			globPat := argStr(params, "glob")
			// pi: Math.max(1, limit ?? 100) — non-positive limits clamp to 1.
			limit := grepDefaultLimit
			if l, ok := argInt(params, "limit"); ok {
				limit = l
			}
			if limit < 1 {
				limit = 1
			}
			ctxLines := 0
			if c, ok := argInt(params, "context"); ok {
				ctxLines = c
			}

			flags := ""
			if argBool(params, "ignoreCase") {
				flags = "(?i)"
			}
			expr := patternStr
			if argBool(params, "literal") {
				expr = regexp.QuoteMeta(patternStr)
			}
			re, err := regexp.Compile(flags + expr)
			if err != nil {
				return model.AgentToolResult{}, fmt.Errorf("invalid regex: %v", err)
			}

			isDir, err := ops.IsDirectory(ctx, root)
			if err != nil {
				return model.AgentToolResult{}, fmt.Errorf("path not found: %s", root)
			}

			var matchLines []string
			matchCount := 0
			matchLimitReached := false
			linesTruncated := false

			// searchFile scans one file; skipBinary mirrors rg's NUL sniff (a NUL
			// byte in the first 8KB marks the file binary; only applies during
			// directory traversal — explicitly-given files are always searched).
			// It returns false when the match limit was reached.
			searchFile := func(path, rel string, skipBinary bool) bool {
				data, err := ops.ReadFile(ctx, path)
				if err != nil {
					return true
				}
				if skipBinary {
					window := data
					if len(window) > 8*1024 {
						window = window[:8*1024]
					}
					if bytes.IndexByte(window, 0) != -1 {
						return true
					}
				}
				// pi normalizes \r\n and bare \r to \n before splitting.
				content := strings.ReplaceAll(string(data), "\r\n", "\n")
				content = strings.ReplaceAll(content, "\r", "\n")
				lines := strings.Split(content, "\n")
				for i, line := range lines {
					if matchCount >= limit {
						matchLimitReached = true
						return false
					}
					if !re.MatchString(line) {
						continue
					}
					matchCount++
					start := i - ctxLines
					if ctxLines <= 0 {
						start = i
					} else if start < 0 {
						start = 0
					}
					end := i + ctxLines
					if ctxLines <= 0 {
						end = i
					} else if end >= len(lines) {
						end = len(lines) - 1
					}
					for j := start; j <= end; j++ {
						text, was := files.TruncateLine(lines[j], 0)
						if was {
							linesTruncated = true
						}
						// Match line: "path:N: text". Context line: "path-N- text".
						if j == i {
							matchLines = append(matchLines, fmt.Sprintf("%s:%d: %s", rel, j+1, text))
						} else {
							matchLines = append(matchLines, fmt.Sprintf("%s-%d- %s", rel, j+1, text))
						}
					}
					if matchCount >= limit {
						matchLimitReached = true
						return false
					}
				}
				return true
			}

			if !isDir {
				searchFile(root, filepath.Base(root), false)
			} else {
				// rg semantics: gitignore applies only inside a git repository.
				ig := newIgnoreStack(root, true, false)
				err = filepath.WalkDir(root, func(p string, d os.DirEntry, walkErr error) error {
					if walkErr != nil {
						return nil
					}
					rel, _ := filepath.Rel(root, p)
					if rel == "." {
						return nil
					}
					if ig.ignored(p, rel, d.IsDir()) {
						if d.IsDir() {
							return filepath.SkipDir
						}
						return nil
					}
					if d.IsDir() {
						return nil
					}
					if globPat != "" && !matchRgGlob(globPat, rel) {
						return nil
					}
					if matchCount >= limit {
						matchLimitReached = true
						return filepath.SkipAll
					}
					if !searchFile(p, rel, true) {
						return filepath.SkipAll
					}
					return nil
				})
				if err != nil {
					return model.AgentToolResult{}, err
				}
			}
			if ctx.Err() != nil {
				return model.AgentToolResult{}, ctx.Err()
			}

			if matchCount == 0 {
				return textResult("No matches found"), nil
			}

			rawOutput := strings.Join(matchLines, "\n")
			tr := files.TruncateHead(rawOutput, files.Options{MaxLines: new(maxInt)})
			output := tr.Content
			var details GrepToolDetails
			hasDetails := false
			var notices []string
			if matchLimitReached {
				notices = append(notices, fmt.Sprintf("%d matches limit reached. Use limit=%d for more, or refine pattern", limit, limit*2))
				details.MatchLimitReached = &limit
				hasDetails = true
			}
			if tr.Truncated {
				notices = append(notices, files.FormatSize(files.DefaultMaxBytes)+" limit reached")
				details.Truncation = &tr
				hasDetails = true
			}
			if linesTruncated {
				notices = append(notices, fmt.Sprintf("Some lines truncated to %d chars. Use read tool to see full lines", files.GrepMaxLineLength))
				details.LinesTruncated = true
				hasDetails = true
			}
			if len(notices) > 0 {
				output += "\n\n[" + strings.Join(notices, ". ") + "]"
			}
			res := textResult(output)
			if hasDetails {
				res.Details = details
			}
			return res, nil
		},
	}
}

func grepDescription() string {
	return fmt.Sprintf("Search file contents for a pattern. Returns matching lines with file paths and line numbers. "+
		"Respects .gitignore. Output is truncated to %d matches or %dKB (whichever is hit first). "+
		"Long lines are truncated to %d chars.",
		grepDefaultLimit, files.DefaultMaxBytes/1024, files.GrepMaxLineLength)
}
