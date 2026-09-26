package edittools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"strings"
	"syscall"

	"github.com/tylergannon/gimbal/internal/pi/files"
	"github.com/tylergannon/gimbal/internal/pi/model"
)

// utf8BOM is the decoded UTF-8 byte order mark, split off before matching and
// re-prepended when writing.
const utf8BOM = "\ufeff"

func splitBOM(content string) (bom, text string) {
	if rest, ok := strings.CutPrefix(content, utf8BOM); ok {
		return utf8BOM, rest
	}
	return "", content
}

// EditOperations are the file operations the edit tool performs. A nil member of
// a nil *EditOperations falls back to the local filesystem (pi's
// `options?.operations ?? defaultEditOperations`).
type EditOperations struct {
	ReadFile  func(ctx context.Context, absolutePath string) ([]byte, error)
	WriteFile func(ctx context.Context, absolutePath, content string) error
	// Access reports whether the file is readable and writable. pi passes
	// R_OK|W_OK here, where the read tool passes R_OK alone.
	Access func(ctx context.Context, absolutePath string) error
}

// EditOptions configures an edit tool.
type EditOptions struct {
	Operations *EditOperations
}

// EditToolDetails is the edit tool result payload.
type EditToolDetails struct {
	// Diff is the display-oriented diff of the changes made.
	Diff string `json:"diff"`
	// Patch is the standard unified patch of the changes made.
	Patch string `json:"patch"`
	// FirstChangedLine is the line number of the first change in the new file.
	FirstChangedLine *int `json:"firstChangedLine,omitempty"`
}

func defaultEditOperations() EditOperations {
	return EditOperations{
		ReadFile: func(_ context.Context, absolutePath string) ([]byte, error) {
			return os.ReadFile(absolutePath)
		},
		WriteFile: func(_ context.Context, absolutePath, content string) error {
			return os.WriteFile(absolutePath, []byte(content), 0o644)
		},
		Access: func(_ context.Context, absolutePath string) error {
			return accessReadWrite(absolutePath)
		},
	}
}

func resolveEditOperations(ops *EditOperations) EditOperations {
	if ops == nil {
		return defaultEditOperations()
	}
	return *ops
}

func accessReadWrite(absolutePath string) error {
	f, err := os.OpenFile(absolutePath, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	return f.Close()
}

func openReadable(absolutePath string) (*os.File, error) {
	return os.Open(absolutePath)
}

// editAccessErrorMessage renders a filesystem error like pi's
// `Error code: ${error.code}` and falls back to Node's `String(error)`
// ("Error: ...") for errors without a code.
func editAccessErrorMessage(err error) string {
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return "Error code: ENOENT"
	case errors.Is(err, fs.ErrPermission):
		return "Error code: EACCES"
	case errors.Is(err, syscall.EISDIR):
		return "Error code: EISDIR"
	}
	return "Error: " + err.Error()
}

// PrepareEditArguments ports pi's prepareEditArguments: it parses an `edits`
// JSON string sent by some models, wraps a single edit object sent instead of a
// one-element array, and folds legacy top-level oldText/newText into edits.
func PrepareEditArguments(input map[string]any) map[string]any {
	if input == nil {
		return nil
	}

	editsValue, hasEdits := input["edits"]
	oldText, oldOK := input["oldText"].(string)
	newText, newOK := input["newText"].(string)
	legacy := oldOK && newOK

	var replacement any
	needsCopy := false
	if hasEdits {
		if s, ok := editsValue.(string); ok {
			var parsed any
			if err := json.Unmarshal([]byte(s), &parsed); err == nil {
				if arr, ok := parsed.([]any); ok {
					replacement = arr
					needsCopy = true
				} else if isSingleEditInput(parsed) {
					replacement = []any{parsed}
					needsCopy = true
				}
			}
		} else if isSingleEditInput(editsValue) {
			replacement = []any{editsValue}
			needsCopy = true
		}
	}

	if !needsCopy && !legacy {
		return input
	}

	args := make(map[string]any, len(input))
	maps.Copy(args, input)
	if needsCopy {
		args["edits"] = replacement
	}
	if legacy {
		var edits []any
		if existing, ok := args["edits"].([]any); ok {
			edits = append(edits, existing...)
		}
		edits = append(edits, map[string]any{"oldText": oldText, "newText": newText})
		args["edits"] = edits
		delete(args, "oldText")
		delete(args, "newText")
	}
	return args
}

func isSingleEditInput(value any) bool {
	m, ok := value.(map[string]any)
	if !ok {
		return false
	}
	_, oldOK := m["oldText"].(string)
	_, newOK := m["newText"].(string)
	return oldOK && newOK
}

func validateEditInput(input map[string]any) (string, []Edit, error) {
	path, _ := input["path"].(string)
	edits := editsFromParams(input["edits"])
	if len(edits) == 0 {
		return "", nil, errors.New("Edit tool input is invalid. edits must contain at least one replacement.") //nolint:staticcheck // upstream message punctuation
	}
	return path, edits, nil
}

func editsFromParams(value any) []Edit {
	switch t := value.(type) {
	case []Edit:
		return t
	case []map[string]any:
		out := make([]Edit, 0, len(t))
		for _, m := range t {
			out = append(out, Edit{OldText: stringValue(m["oldText"]), NewText: stringValue(m["newText"])})
		}
		return out
	case []any:
		out := make([]Edit, 0, len(t))
		for _, item := range t {
			if m, ok := item.(map[string]any); ok {
				out = append(out, Edit{OldText: stringValue(m["oldText"]), NewText: stringValue(m["newText"])})
			}
		}
		return out
	default:
		return nil
	}
}

func stringValue(value any) string {
	s, _ := value.(string)
	return s
}

// Edit schema and prompt contribution (port of edit.ts).
const (
	EditName        = "edit"
	EditLabel       = "edit"
	EditDescription = "Edit a single file using exact text replacement. Every edits[].oldText must match a unique, non-overlapping region of the original file. If two changes affect the same block or nearby lines, merge them into one edit instead of emitting overlapping edits. Do not include large unchanged regions just to connect distant changes."

	EditPromptSnippet = "Make precise file edits with exact text replacement, including multiple disjoint edits in one call"
)

// EditPromptGuidelines are the edit tool's system-prompt guideline bullets.
var EditPromptGuidelines = []string{
	"Use edit for precise changes (edits[].oldText must match exactly)",
	"When changing multiple separate locations in one file, use one edit call with multiple entries in edits[] instead of multiple edit calls",
	"Each edits[].oldText is matched against the original file, not after earlier edits are applied. Do not emit overlapping or nested edits. Merge nearby changes into one edit.",
	"Keep edits[].oldText as small as possible while still being unique in the file. Do not pad with large unchanged regions.",
}

func editParameters() *model.Schema {
	oldText := model.String()
	oldText.Description = "Exact text for one targeted replacement. It must be unique in the original file and must not overlap with any other edits[].oldText in the same call."
	newText := model.String()
	newText.Description = "Replacement text for this targeted edit."
	edits := model.Array(model.Object(
		model.Prop("oldText", oldText),
		model.Prop("newText", newText),
	))
	edits.Description = "One or more targeted replacements. Each edit is matched against the original file, not incrementally. Do not include overlapping or nested edits. If two changes touch the same block or nearby lines, merge them into one edit instead."
	path := model.String()
	path.Description = "Path to the file to edit (relative or absolute)"
	return model.Object(
		model.Prop("path", path),
		model.Prop("edits", edits),
	)
}

func editExecute(cwd string, ops EditOperations) func(context.Context, string, map[string]any, model.ToolUpdateFunc) (model.AgentToolResult, error) {
	return func(ctx context.Context, _ string, params map[string]any, _ model.ToolUpdateFunc) (model.AgentToolResult, error) {
		path, edits, err := validateEditInput(params)
		if err != nil {
			return model.AgentToolResult{}, err
		}
		absolutePath := files.ResolveToCwd(path, cwd)

		return files.WithFileMutationQueue(absolutePath, func() (model.AgentToolResult, error) {
			if err := ctx.Err(); err != nil {
				return model.AgentToolResult{}, err
			}
			if err := ops.Access(ctx, absolutePath); err != nil {
				if abortErr := ctx.Err(); abortErr != nil {
					return model.AgentToolResult{}, abortErr
				}
				//nolint:staticcheck // upstream message punctuation
				return model.AgentToolResult{}, fmt.Errorf("Could not edit file: %s. %s.", path, editAccessErrorMessage(err))
			}
			if err := ctx.Err(); err != nil {
				return model.AgentToolResult{}, err
			}

			data, err := ops.ReadFile(ctx, absolutePath)
			if err != nil {
				return model.AgentToolResult{}, err
			}
			if err := ctx.Err(); err != nil {
				return model.AgentToolResult{}, err
			}

			bom, raw := splitBOM(string(data))
			originalEnding := DetectLineEnding(raw)
			normalized := NormalizeToLF(raw)
			result, err := ApplyEditsToNormalizedContent(normalized, edits, path)
			if err != nil {
				return model.AgentToolResult{}, err
			}
			if err := ctx.Err(); err != nil {
				return model.AgentToolResult{}, err
			}

			finalContent := bom + RestoreLineEndings(result.NewContent, originalEnding)
			if err := ops.WriteFile(ctx, absolutePath, finalContent); err != nil {
				return model.AgentToolResult{}, err
			}
			if err := ctx.Err(); err != nil {
				return model.AgentToolResult{}, err
			}

			diff, firstChangedLine, hasFirstChangedLine := GenerateDiffString(result.BaseContent, result.NewContent)
			details := EditToolDetails{Diff: diff, Patch: GenerateUnifiedPatch(path, result.BaseContent, result.NewContent)}
			if hasFirstChangedLine {
				line := firstChangedLine
				details.FirstChangedLine = &line
			}
			return model.AgentToolResult{
				Content: model.ContentList{model.TextContent{
					Text: fmt.Sprintf("Successfully replaced %d block(s) in %s.", len(edits), path),
				}},
				Details: details,
			}, nil
		})
	}
}

// EditDefinition returns the definition-first form of the edit tool.
func EditDefinition(cwd string, options *EditOptions) model.ToolDefinition {
	ops := resolveEditOperations(options.operations())
	return model.ToolDefinition{
		Name:                EditName,
		Label:               EditLabel,
		Description:         EditDescription,
		PromptSnippet:       EditPromptSnippet,
		PromptGuidelines:    append([]string(nil), EditPromptGuidelines...),
		Parameters:          editParameters(),
		ConstrainedSampling: preferStrictToolSampling(),
		PrepareArguments:    PrepareEditArguments,
		Execute:             editExecute(cwd, ops),
	}
}

// EditTool returns the edit tool as an agent tool.
func EditTool(cwd string, options *EditOptions) model.AgentTool {
	return agentToolFromDefinition(EditDefinition(cwd, options))
}

func (o *EditOptions) operations() *EditOperations {
	if o == nil {
		return nil
	}
	return o.Operations
}
