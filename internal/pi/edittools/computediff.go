package edittools

import (
	"fmt"
	"io"

	"github.com/tylergannon/gimbal/internal/pi/files"
)

// EditDiffResult is the rendered diff for edits that were not applied.
type EditDiffResult struct {
	Diff string
	// FirstChangedLine is the line number of the first change in the new file,
	// or nil when there is no change.
	FirstChangedLine *int
}

// ComputeEditsDiff computes the display diff for edits without applying them
// (port of edit-diff.ts computeEditsDiff). It resolves path against cwd.
func ComputeEditsDiff(path string, edits []Edit, cwd string) (EditDiffResult, error) {
	absolutePath := files.ResolveToCwd(path, cwd)

	f, err := openReadable(absolutePath)
	if err != nil {
		return EditDiffResult{}, fmt.Errorf("Could not edit file: %s. %s.", path, editAccessErrorMessage(err)) //nolint:staticcheck // upstream message punctuation
	}
	defer func() { _ = f.Close() }()
	data, err := io.ReadAll(f)
	if err != nil {
		return EditDiffResult{}, err
	}

	_, content := splitBOM(string(data))
	normalizedContent := NormalizeToLF(content)
	result, err := ApplyEditsToNormalizedContent(normalizedContent, edits, path)
	if err != nil {
		return EditDiffResult{}, err
	}
	diff, firstChangedLine, ok := GenerateDiffString(result.BaseContent, result.NewContent)
	out := EditDiffResult{Diff: diff}
	if ok {
		line := firstChangedLine
		out.FirstChangedLine = &line
	}
	return out, nil
}

// ComputeEditDiff is the single-edit convenience wrapper around ComputeEditsDiff
// (port of edit-diff.ts computeEditDiff).
func ComputeEditDiff(path, oldText, newText, cwd string) (EditDiffResult, error) {
	return ComputeEditsDiff(path, []Edit{{OldText: oldText, NewText: newText}}, cwd)
}
