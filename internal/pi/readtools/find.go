package readtools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tylergannon/gimbal/internal/pi/files"
	"github.com/tylergannon/gimbal/internal/pi/model"
)

// findPromptSnippet mirrors pi's findToolSystemPromptContribution.
const findPromptSnippet = "Find files by glob pattern (respects .gitignore)"

// FindOperations are the file operations the find tool performs.
type FindOperations struct {
	// Exists reports whether a path exists.
	Exists func(ctx context.Context, absolutePath string) (bool, error)
	// Glob returns paths matching a pattern. Nil keeps pi's behavior, where the
	// default is a placeholder and real matching happens in the tool.
	Glob func(ctx context.Context, pattern, cwd string, ignore []string, limit int) ([]string, error)
}

// FindToolOptions configure the find tool.
type FindToolOptions struct {
	// Operations overrides the local filesystem.
	Operations *FindOperations
}

// FindToolDetails is the find tool's optional details payload.
type FindToolDetails struct {
	Truncation         *files.Result `json:"truncation,omitempty"`
	ResultLimitReached *int          `json:"resultLimitReached,omitempty"`
}

// DefaultFindOperations checks the local filesystem. Glob is nil, matching pi,
// whose default is a placeholder because real matching happens in the tool.
func DefaultFindOperations() FindOperations {
	return FindOperations{
		Exists: func(_ context.Context, p string) (bool, error) {
			_, err := os.Stat(p)
			return err == nil, nil
		},
		Glob: nil,
	}
}

func resolveFindOperations(ops *FindOperations) FindOperations {
	if ops == nil {
		return DefaultFindOperations()
	}
	return *ops
}

// FindTool builds the find tool.
func FindTool(cwd string, options *FindToolOptions) model.ToolDefinition {
	var custom *FindOperations
	if options != nil {
		custom = options.Operations
	}
	ops := resolveFindOperations(custom)

	return model.ToolDefinition{
		Name:          "find",
		Label:         "find",
		Description:   findDescription(),
		PromptSnippet: findPromptSnippet,
		Parameters: model.Object(
			model.Prop("pattern", desc(model.String(), "Glob pattern to match files, e.g. '*.ts', '**/*.json', or 'src/**/*.spec.ts'")),
			model.Opt("path", desc(model.String(), "Directory to search in (default: current directory)")),
			model.Opt("limit", desc(model.Number(), "Maximum number of results (default: 1000)")),
		),
		Execute: func(ctx context.Context, _ string, params map[string]any, _ model.ToolUpdateFunc) (model.AgentToolResult, error) {
			if err := ctx.Err(); err != nil {
				return model.AgentToolResult{}, err
			}
			pattern := argStr(params, "pattern")
			root := cwd
			if p := argStr(params, "path"); p != "" {
				root = files.ResolveToCwd(p, cwd)
			}
			if custom != nil && custom.Glob != nil {
				return findWithGlob(ctx, ops, custom, pattern, root, params)
			}
			return findLocal(ctx, ops, pattern, root, params)
		},
	}
}

// findWithGlob uses a caller-supplied glob implementation, porting the
// `if (customOps?.glob)` branch of find.ts.
func findWithGlob(ctx context.Context, ops FindOperations, custom *FindOperations, pattern, root string, params map[string]any) (model.AgentToolResult, error) {
	limit := findDefaultLimit
	if l, ok := argInt(params, "limit"); ok {
		limit = l
	}
	if ok, err := ops.Exists(ctx, root); err != nil || !ok {
		return model.AgentToolResult{}, fmt.Errorf("path not found: %s", root)
	}
	results, err := custom.Glob(ctx, pattern, root, []string{"**/node_modules/**", "**/.git/**"}, limit)
	if err != nil {
		return model.AgentToolResult{}, err
	}
	if len(results) == 0 {
		return textResult("No files found matching pattern"), nil
	}
	relativized := make([]string, len(results))
	for i, p := range results {
		relativized[i] = relativizeFindResultPath(p, root)
	}
	return findOutput(relativized, limit, limit >= 0 && len(relativized) >= limit), nil
}

// findLocal walks the filesystem in Go, standing in for fd.
func findLocal(ctx context.Context, ops FindOperations, pattern, root string, params map[string]any) (model.AgentToolResult, error) {
	limit := findDefaultLimit
	if l, ok := argInt(params, "limit"); ok {
		limit = l
	}
	// fd --max-results treats 0 as unlimited; never slice with a non-positive
	// limit.
	unlimited := limit <= 0
	if err := validateGlob(pattern); err != nil {
		return model.AgentToolResult{}, fmt.Errorf("error parsing glob: %w", err)
	}
	if ok, err := ops.Exists(ctx, root); err != nil || !ok {
		return model.AgentToolResult{}, fmt.Errorf("path not found: %s", root)
	}

	// fd: gitignore applies whether or not we are in a repo (the old
	// --no-require-git effect outside a repo). Inside a repo, fd's default
	// git-aware traversal stops parent .gitignore rules at nested repository
	// boundaries, so request that here.
	ig := newIgnoreStack(root, false, true)
	var results []string
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, walkErr error) error {
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
		if !unlimited && len(results) >= limit {
			return filepath.SkipAll
		}
		// fd matches directories as well as files, and marks them with a
		// trailing separator in its output.
		if matchFdGlob(pattern, rel, p) {
			result := p
			if d.IsDir() {
				result += string(filepath.Separator)
			}
			results = append(results, relativizeFindResultPath(result, root))
		}
		return nil
	})
	if err != nil {
		return model.AgentToolResult{}, err
	}
	if ctx.Err() != nil {
		return model.AgentToolResult{}, ctx.Err()
	}
	// Deterministic, documented ordering: fd's native traversal order is
	// unspecified; a stable sort keeps output reproducible.
	sort.Strings(results)
	// pi: resultLimitReached = relativized.length >= effectiveLimit.
	resultLimitReached := limit >= 0 && len(results) >= limit
	return findOutput(results, limit, resultLimitReached), nil
}

// findOutput renders results, truncating and adding the actionable notices.
func findOutput(results []string, limit int, resultLimitReached bool) model.AgentToolResult {
	if len(results) == 0 {
		return textResult("No files found matching pattern")
	}
	rawOutput := strings.Join(results, "\n")
	tr := files.TruncateHead(rawOutput, files.Options{MaxLines: new(maxInt)})
	output := tr.Content
	var details FindToolDetails
	hasDetails := false
	var notices []string
	if resultLimitReached {
		notices = append(notices, fmt.Sprintf("%d results limit reached. Use limit=%d for more, or refine pattern", limit, limit*2))
		details.ResultLimitReached = &limit
		hasDetails = true
	}
	if tr.Truncated {
		notices = append(notices, files.FormatSize(files.DefaultMaxBytes)+" limit reached")
		details.Truncation = &tr
		hasDetails = true
	}
	if len(notices) > 0 {
		output += "\n\n[" + strings.Join(notices, ". ") + "]"
	}
	res := textResult(output)
	if hasDetails {
		res.Details = details
	}
	return res
}

// relativizeFindResultPath relativizes a find result against the search root and
// normalizes it to posix separators (pi's relativizeFindResultPath). A trailing
// separator marks directory results and is carried across relativization.
func relativizeFindResultPath(resultPath, searchPath string) string {
	hadTrailingSeparator := strings.HasSuffix(resultPath, "/") || strings.HasSuffix(resultPath, `\`)
	rel, err := filepath.Rel(searchPath, resultPath)
	if err != nil {
		rel = resultPath
	}
	rel = filepath.ToSlash(rel)
	if hadTrailingSeparator && !strings.HasSuffix(rel, "/") {
		rel += "/"
	}
	return rel
}

func findDescription() string {
	return fmt.Sprintf("Search for files by glob pattern. Returns matching file paths relative to the search directory. "+
		"Respects .gitignore. Output is truncated to %d results or %dKB (whichever is hit first).",
		findDefaultLimit, files.DefaultMaxBytes/1024)
}
