package readtools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/text/collate"
	"golang.org/x/text/language"

	"github.com/tylergannon/gimbal/internal/pi/files"
	"github.com/tylergannon/gimbal/internal/pi/model"
)

// lsPromptSnippet mirrors pi's lsToolSystemPromptContribution.
const lsPromptSnippet = "List directory contents"

// LsOperations are the file operations the ls tool performs.
type LsOperations struct {
	// Exists reports whether a path exists.
	Exists func(ctx context.Context, absolutePath string) (bool, error)
	// Stat reports whether a path is a directory, erroring if it is not found.
	Stat func(ctx context.Context, absolutePath string) (isDir bool, err error)
	// Readdir lists a directory's entry names.
	Readdir func(ctx context.Context, absolutePath string) ([]string, error)
}

// LsToolOptions configure the ls tool.
type LsToolOptions struct {
	// Operations overrides the local filesystem.
	Operations *LsOperations
}

// LsToolDetails is the ls tool's optional details payload.
type LsToolDetails struct {
	Truncation        *files.Result `json:"truncation,omitempty"`
	EntryLimitReached *int          `json:"entryLimitReached,omitempty"`
}

// DefaultLsOperations lists the local filesystem.
func DefaultLsOperations() LsOperations {
	return LsOperations{
		Exists: func(_ context.Context, p string) (bool, error) {
			_, err := os.Stat(p)
			return err == nil, nil
		},
		Stat: func(_ context.Context, p string) (bool, error) {
			st, err := os.Stat(p)
			if err != nil {
				return false, err
			}
			return st.IsDir(), nil
		},
		Readdir: func(_ context.Context, p string) ([]string, error) {
			entries, err := os.ReadDir(p)
			if err != nil {
				return nil, err
			}
			names := make([]string, 0, len(entries))
			for _, e := range entries {
				names = append(names, e.Name())
			}
			return names, nil
		},
	}
}

func resolveLsOperations(ops *LsOperations) LsOperations {
	if ops == nil {
		return DefaultLsOperations()
	}
	return *ops
}

// LsTool builds the ls tool.
func LsTool(cwd string, options *LsToolOptions) model.ToolDefinition {
	var custom *LsOperations
	if options != nil {
		custom = options.Operations
	}
	ops := resolveLsOperations(custom)

	return model.ToolDefinition{
		Name:          "ls",
		Label:         "ls",
		Description:   lsDescription(),
		PromptSnippet: lsPromptSnippet,
		Parameters: model.Object(
			model.Opt("path", desc(model.String(), "Directory to list (default: current directory)")),
			model.Opt("limit", desc(model.Number(), "Maximum number of entries to return (default: 500)")),
		),
		Execute: func(ctx context.Context, _ string, params map[string]any, _ model.ToolUpdateFunc) (model.AgentToolResult, error) {
			if err := ctx.Err(); err != nil {
				return model.AgentToolResult{}, err
			}
			dir := cwd
			if p := argStr(params, "path"); p != "" {
				dir = files.ResolveToCwd(p, cwd)
			}
			limit := lsDefaultLimit
			if l, ok := argInt(params, "limit"); ok {
				limit = l
			}

			if ok, err := ops.Exists(ctx, dir); err != nil || !ok {
				return model.AgentToolResult{}, fmt.Errorf("path not found: %s", dir)
			}
			isDir, err := ops.Stat(ctx, dir)
			if err != nil {
				return model.AgentToolResult{}, fmt.Errorf("path not found: %s", dir)
			}
			if !isDir {
				return model.AgentToolResult{}, fmt.Errorf("not a directory: %s", dir)
			}
			names, err := ops.Readdir(ctx, dir)
			if err != nil {
				return model.AgentToolResult{}, fmt.Errorf("cannot read directory: %v", err)
			}

			// pi sorts with a.toLowerCase().localeCompare(b.toLowerCase()).
			coll := collate.New(language.Und)
			sort.SliceStable(names, func(a, b int) bool {
				return coll.CompareString(strings.ToLower(names[a]), strings.ToLower(names[b])) < 0
			})

			results := make([]string, 0, len(names))
			entryLimitReached := false
			for _, name := range names {
				if len(results) >= limit {
					entryLimitReached = true
					break
				}
				// Stat (follows symlinks) to detect dir-ness, like pi.
				suffix := ""
				entryIsDir, err := ops.Stat(ctx, filepath.Join(dir, name))
				if err != nil {
					// Skip entries we cannot stat.
					continue
				}
				if entryIsDir {
					suffix = "/"
				}
				results = append(results, name+suffix)
			}

			if len(results) == 0 {
				return textResult("(empty directory)"), nil
			}

			rawOutput := strings.Join(results, "\n")
			tr := files.TruncateHead(rawOutput, files.Options{MaxLines: new(maxInt)})
			output := tr.Content
			var details LsToolDetails
			hasDetails := false
			var notices []string
			if entryLimitReached {
				notices = append(notices, fmt.Sprintf("%d entries limit reached. Use limit=%d for more", limit, limit*2))
				details.EntryLimitReached = &limit
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
			return res, nil
		},
	}
}

func lsDescription() string {
	return fmt.Sprintf("List directory contents. Returns entries sorted alphabetically, with '/' suffix for directories. "+
		"Includes dotfiles. Output is truncated to %d entries or %dKB (whichever is hit first).",
		lsDefaultLimit, files.DefaultMaxBytes/1024)
}
