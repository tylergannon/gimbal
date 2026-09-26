package edittools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/tylergannon/gimbal/internal/pi/files"
	"github.com/tylergannon/gimbal/internal/pi/model"
)

// WriteOperations are the file operations the write tool performs.
type WriteOperations struct {
	WriteFile func(ctx context.Context, absolutePath, content string) error
	// Mkdir creates a directory and its parents.
	Mkdir func(ctx context.Context, dir string) error
}

// WriteOptions configures a write tool.
type WriteOptions struct {
	Operations *WriteOperations
}

func defaultWriteOperations() WriteOperations {
	return WriteOperations{
		WriteFile: func(_ context.Context, absolutePath, content string) error {
			return os.WriteFile(absolutePath, []byte(content), 0o644)
		},
		Mkdir: func(_ context.Context, dir string) error {
			return os.MkdirAll(dir, 0o755)
		},
	}
}

func resolveWriteOperations(ops *WriteOperations) WriteOperations {
	if ops == nil {
		return defaultWriteOperations()
	}
	return *ops
}

// Write schema and prompt contribution (port of write.ts).
const (
	WriteName        = "write"
	WriteLabel       = "write"
	WriteDescription = "Write content to a file. Creates the file if it doesn't exist, overwrites if it does. Automatically creates parent directories."

	WritePromptSnippet = "Create or overwrite files"
)

// WritePromptGuidelines are the write tool's system-prompt guideline bullets.
var WritePromptGuidelines = []string{
	"Use write only for new files or complete rewrites.",
}

func writeParameters() *model.Schema {
	path := model.String()
	path.Description = "Path to the file to write (relative or absolute)"
	content := model.String()
	content.Description = "Content to write to the file"
	return model.Object(
		model.Prop("path", path),
		model.Prop("content", content),
	)
}

func writeExecute(cwd string, ops WriteOperations) func(context.Context, string, map[string]any, model.ToolUpdateFunc) (model.AgentToolResult, error) {
	return func(ctx context.Context, _ string, params map[string]any, _ model.ToolUpdateFunc) (model.AgentToolResult, error) {
		path := stringValue(params["path"])
		content := stringValue(params["content"])
		absolutePath := files.ResolveToCwd(path, cwd)
		dir := filepath.Dir(absolutePath)

		return files.WithFileMutationQueue(absolutePath, func() (model.AgentToolResult, error) {
			if err := ctx.Err(); err != nil {
				return model.AgentToolResult{}, err
			}
			if err := ops.Mkdir(ctx, dir); err != nil {
				return model.AgentToolResult{}, err
			}
			if err := ctx.Err(); err != nil {
				return model.AgentToolResult{}, err
			}
			if err := ops.WriteFile(ctx, absolutePath, content); err != nil {
				return model.AgentToolResult{}, err
			}
			if err := ctx.Err(); err != nil {
				return model.AgentToolResult{}, err
			}
			return model.AgentToolResult{
				Content: model.ContentList{model.TextContent{
					Text: fmt.Sprintf("Successfully wrote to %s", path),
				}},
			}, nil
		})
	}
}

// WriteDefinition returns the definition-first form of the write tool.
func WriteDefinition(cwd string, options *WriteOptions) model.ToolDefinition {
	ops := resolveWriteOperations(options.operations())
	return model.ToolDefinition{
		Name:                WriteName,
		Label:               WriteLabel,
		Description:         WriteDescription,
		PromptSnippet:       WritePromptSnippet,
		PromptGuidelines:    append([]string(nil), WritePromptGuidelines...),
		Parameters:          writeParameters(),
		ConstrainedSampling: preferStrictToolSampling(),
		Execute:             writeExecute(cwd, ops),
	}
}

// WriteTool returns the write tool as an agent tool.
func WriteTool(cwd string, options *WriteOptions) model.AgentTool {
	return agentToolFromDefinition(WriteDefinition(cwd, options))
}

func (o *WriteOptions) operations() *WriteOperations {
	if o == nil {
		return nil
	}
	return o.Operations
}
