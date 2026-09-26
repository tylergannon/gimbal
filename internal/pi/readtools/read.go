package readtools

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/tylergannon/gimbal/internal/pi/files"
	"github.com/tylergannon/gimbal/internal/pi/model"
)

// readPromptSnippet and readPromptGuidelines mirror pi's
// readToolSystemPromptContribution.
const readPromptSnippet = "Read file contents"

var readPromptGuidelines = []string{"Use read to examine files instead of cat or sed."}

// ReadOperations are the file operations the read tool performs. A nil member
// falls back to the local default, matching pi's spread-over-defaults.
type ReadOperations struct {
	// ReadFile reads a file's contents. Required.
	ReadFile func(ctx context.Context, absolutePath string) ([]byte, error)
	// Access reports whether the file is readable, returning an error if not.
	// Required.
	Access func(ctx context.Context, absolutePath string) error
	// DetectImageMimeType returns the image MIME type for a path, or "" for a
	// non-image. Optional; nil uses the built-in detector.
	DetectImageMimeType func(ctx context.Context, absolutePath string) string
}

// ReadToolOptions configure the read tool.
type ReadToolOptions struct {
	// AutoResizeImages resizes oversized images to inline provider limits.
	// Nil defaults to true.
	AutoResizeImages *bool
	// ResizeOptions is the fallback resize profile when the caller has no
	// model metadata.
	ResizeOptions *model.ModelImageResizeOptions
	// Operations overrides the local filesystem.
	Operations *ReadOperations
}

// ReadToolDetails is the read tool's optional details payload.
type ReadToolDetails struct {
	Truncation *files.Result `json:"truncation,omitempty"`
}

// DefaultReadOperations reads from the local filesystem.
func DefaultReadOperations() ReadOperations {
	return ReadOperations{
		ReadFile: func(_ context.Context, p string) ([]byte, error) {
			return os.ReadFile(p)
		},
		Access: func(_ context.Context, p string) error {
			// os.Open reports a directory as an error, standing in for Node's
			// EISDIR from fs.readFile; access only needs readability.
			f, err := os.Open(p)
			if err != nil {
				return err
			}
			return f.Close()
		},
		DetectImageMimeType: func(_ context.Context, p string) string {
			return detectSupportedImageMimeTypeFromFile(p)
		},
	}
}

func resolveReadOperations(ops *ReadOperations) ReadOperations {
	if ops == nil {
		return DefaultReadOperations()
	}
	return *ops
}

// ReadTool builds the read tool. cwd anchors relative paths; the returned
// definition closes over it, since model.ToolDefinition.Execute has no
// execution context of its own.
func ReadTool(cwd string, options *ReadToolOptions) model.ToolDefinition {
	autoResizeImages := true
	var resizeOptions *model.ModelImageResizeOptions
	var custom *ReadOperations
	if options != nil {
		if options.AutoResizeImages != nil {
			autoResizeImages = *options.AutoResizeImages
		}
		resizeOptions = options.ResizeOptions
		custom = options.Operations
	}
	ops := resolveReadOperations(custom)

	return model.ToolDefinition{
		Name:          "read",
		Label:         "read",
		Description:   readDescription(),
		PromptSnippet: readPromptSnippet,
		PromptGuidelines: append([]string(nil),
			readPromptGuidelines...),
		Parameters: model.Object(
			model.Prop("path", desc(model.String(), "Path to the file to read (relative or absolute)")),
			model.Opt("offset", desc(model.Number(), "Line number to start reading from (1-indexed)")),
			model.Opt("limit", desc(model.Number(), "Maximum number of lines to read")),
		),
		ConstrainedSampling: &model.ConstrainedSamplingConfig{
			Type:   model.ConstrainedSamplingJSONSchema,
			Strict: model.ConstrainedSamplingPrefer,
		},
		Execute: func(ctx context.Context, _ string, params map[string]any, _ model.ToolUpdateFunc) (model.AgentToolResult, error) {
			if err := ctx.Err(); err != nil {
				return model.AgentToolResult{}, err
			}
			path := argStr(params, "path")
			abs := files.ResolveReadPath(path, cwd)
			if err := ops.Access(ctx, abs); err != nil {
				return model.AgentToolResult{}, err
			}
			if err := ctx.Err(); err != nil {
				return model.AgentToolResult{}, err
			}
			mime := ""
			if ops.DetectImageMimeType != nil {
				mime = ops.DetectImageMimeType(ctx, abs)
			}
			if mime != "" {
				return readImage(ctx, ops, abs, mime, autoResizeImages, resizeOptions)
			}
			return readText(ctx, ops, abs, params, path)
		},
	}
}

func readDescription() string {
	return fmt.Sprintf("Read the contents of a file. Supports text files and images (jpg, png, gif, webp, bmp). "+
		"Images are sent as attachments. For text files, output is truncated to %d lines or %dKB (whichever is hit first). "+
		"Use offset/limit for large files. When you need the full file, continue with offset until complete.",
		files.DefaultMaxLines, files.DefaultMaxBytes/1024)
}

// readImage reads and processes an image attachment, porting the image branch
// of read.ts execute.
func readImage(ctx context.Context, ops ReadOperations, abs, mime string, autoResize bool, resizeOptions *model.ModelImageResizeOptions) (model.AgentToolResult, error) {
	data, err := ops.ReadFile(ctx, abs)
	if err != nil {
		return model.AgentToolResult{}, err
	}
	processed := files.ProcessImage(data, mime, &files.ProcessImageOptions{
		AutoResizeImages: &autoResize,
		ResizeOptions:    resizeOptions,
	})
	if !processed.Ok {
		return textResult("Read image file [" + mime + "]\n" + processed.Message), nil
	}
	note := "Read image file [" + processed.MimeType + "]"
	if len(processed.Hints) > 0 {
		note += "\n" + strings.Join(processed.Hints, "\n")
	}
	return model.AgentToolResult{Content: model.ContentList{
		model.TextContent{Text: note},
		model.ImageContent{Data: processed.Data, MimeType: processed.MimeType},
	}}, nil
}

// readText reads a text file and applies pi's offset/limit/truncation rules,
// porting the text branch of read.ts execute.
func readText(ctx context.Context, ops ReadOperations, abs string, params map[string]any, displayPath string) (model.AgentToolResult, error) {
	data, err := ops.ReadFile(ctx, abs)
	if err != nil {
		return model.AgentToolResult{}, err
	}
	allLines := strings.Split(string(data), "\n")
	totalFileLines := len(allLines)

	offset, hasOffset := argInt(params, "offset")
	startLine := 0
	if hasOffset && offset > 0 {
		startLine = offset - 1
	}
	startLineDisplay := startLine + 1
	if startLine >= len(allLines) {
		return model.AgentToolResult{}, &OffsetBeyondEOFError{Offset: offset, TotalLines: len(allLines)}
	}

	limit, hasLimit := argInt(params, "limit")
	var selected string
	userLimitedLines := 0
	if hasLimit {
		// pi: endLine = Math.min(startLine + limit, allLines.length), then
		// allLines.slice(startLine, endLine) with JavaScript slice semantics: a
		// negative end counts back from the array end, and an end before start
		// yields an empty slice (never a panic).
		endLine := min(startLine+limit, len(allLines))
		effEnd := endLine
		if effEnd < 0 {
			effEnd = max(len(allLines)+effEnd, 0)
		}
		if effEnd < startLine {
			effEnd = startLine
		}
		selected = strings.Join(allLines[startLine:effEnd], "\n")
		// pi keeps the raw arithmetic value (which may be zero or negative) for
		// the continuation-footer math.
		userLimitedLines = endLine - startLine
	} else {
		selected = strings.Join(allLines[startLine:], "\n")
	}

	tr := files.TruncateHead(selected, files.Options{})
	var out string
	details := (*ReadToolDetails)(nil)
	switch {
	case tr.FirstLineExceedsLimit:
		firstLineSize := files.FormatSize(len(allLines[startLine]))
		out = fmt.Sprintf("[Line %d is %s, exceeds %s limit. Use bash: sed -n '%dp' %s | head -c %d]",
			startLineDisplay, firstLineSize, files.FormatSize(files.DefaultMaxBytes), startLineDisplay, displayPath, files.DefaultMaxBytes)
		details = &ReadToolDetails{Truncation: &tr}
	case tr.Truncated:
		endLineDisplay := startLineDisplay + tr.OutputLines - 1
		nextOffset := endLineDisplay + 1
		out = tr.Content
		if tr.TruncatedBy != nil && *tr.TruncatedBy == "lines" {
			out += fmt.Sprintf("\n\n[Showing lines %d-%d of %d. Use offset=%d to continue.]",
				startLineDisplay, endLineDisplay, totalFileLines, nextOffset)
		} else {
			out += fmt.Sprintf("\n\n[Showing lines %d-%d of %d (%s limit). Use offset=%d to continue.]",
				startLineDisplay, endLineDisplay, totalFileLines, files.FormatSize(files.DefaultMaxBytes), nextOffset)
		}
		details = &ReadToolDetails{Truncation: &tr}
	case hasLimit && startLine+userLimitedLines < len(allLines):
		remaining := len(allLines) - (startLine + userLimitedLines)
		nextOffset := startLine + userLimitedLines + 1
		out = fmt.Sprintf("%s\n\n[%d more lines in file. Use offset=%d to continue.]", tr.Content, remaining, nextOffset)
	default:
		out = tr.Content
	}

	res := textResult(out)
	if details != nil {
		res.Details = details
	}
	return res, nil
}

// OffsetBeyondEOFError is returned when a read offset starts past the last
// line. Its message matches pi's.
type OffsetBeyondEOFError struct {
	Offset     int
	TotalLines int
}

func (e *OffsetBeyondEOFError) Error() string {
	return fmt.Sprintf("Offset %d is beyond end of file (%d lines total)", e.Offset, e.TotalLines)
}
