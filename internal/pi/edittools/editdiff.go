// Package edittools ports pi's exact-text edit and write tools.
//
// It is a hand port of the following upstream modules (pin d6af72e1):
//
//   - core/tools/edit.ts
//   - core/tools/edit-diff.ts
//   - core/tools/write.ts
//
// The renderers, extension context and tool-definition wrapper those modules
// also touch are TUI/extension concerns and are not part of this port. Edit and
// write serialize through the shared file mutation queue in internal/pi/files
// and describe themselves with internal/pi/model tool contracts.
package edittools

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"golang.org/x/text/unicode/norm"
)

// Edit is one oldText -> newText replacement.
type Edit struct {
	OldText string `json:"oldText"`
	NewText string `json:"newText"`
}

// AppliedEditsResult is the base content an edit matched against together with
// the rewritten content.
type AppliedEditsResult struct {
	BaseContent string
	NewContent  string
}

// DetectLineEnding reports whether content predominantly uses CRLF. A CRLF is
// chosen only when its first occurrence precedes the first bare LF (port of
// edit-diff.ts detectLineEnding).
func DetectLineEnding(content string) string {
	crlfIdx := strings.Index(content, "\r\n")
	lfIdx := strings.Index(content, "\n")
	if lfIdx == -1 {
		return "\n"
	}
	if crlfIdx == -1 {
		return "\n"
	}
	if crlfIdx < lfIdx {
		return "\r\n"
	}
	return "\n"
}

// NormalizeToLF folds CRLF and lone CR line endings to LF.
func NormalizeToLF(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	return strings.ReplaceAll(text, "\r", "\n")
}

// RestoreLineEndings rewrites LF endings back to the original convention.
func RestoreLineEndings(text, ending string) string {
	if ending == "\r\n" {
		return strings.ReplaceAll(text, "\n", "\r\n")
	}
	return text
}

// isJSWhitespace reports whether r is removed by String.prototype.trim. The set
// is ECMAScript WhiteSpace ∪ LineTerminator, which differs from Go's
// unicode.IsSpace (it strips U+FEFF and keeps U+0085). This matches the
// unexported predicate internal/pi/files uses for the same job.
func isJSWhitespace(r rune) bool {
	switch r {
	case '\t', '\n', '\v', '\f', '\r', ' ', '\u00a0', '\u1680',
		'\u2000', '\u2001', '\u2002', '\u2003', '\u2004', '\u2005', '\u2006',
		'\u2007', '\u2008', '\u2009', '\u200a', '\u2028', '\u2029', '\u202f',
		'\u205f', '\u3000', '\ufeff':
		return true
	}
	return false
}

func jsTrimEnd(s string) string {
	return strings.TrimRightFunc(s, isJSWhitespace)
}

// NormalizeForFuzzyMatch applies pi's progressive fuzzy normalization: Unicode
// NFKC, trailing per-line whitespace stripped, smart quotes and dashes folded to
// ASCII, and exotic spaces folded to a regular space (port of edit-diff.ts
// normalizeForFuzzyMatch).
func NormalizeForFuzzyMatch(text string) string {
	text = norm.NFKC.String(text)
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = jsTrimEnd(line)
	}
	joined := strings.Join(lines, "\n")
	return strings.Map(func(r rune) rune {
		switch r {
		case '\u2018', '\u2019', '\u201a', '\u201b':
			return '\''
		case '\u201c', '\u201d', '\u201e', '\u201f':
			return '"'
		case '\u2010', '\u2011', '\u2012', '\u2013', '\u2014', '\u2015', '\u2212':
			return '-'
		case '\u00a0', '\u2002', '\u2003', '\u2004', '\u2005', '\u2006',
			'\u2007', '\u2008', '\u2009', '\u200a', '\u202f', '\u205f', '\u3000':
			return ' '
		}
		return r
	}, joined)
}

// splitLinesWithEndings splits content into lines that keep their trailing
// "\n". A trailing "\n" produces no empty final element (port of pi's
// /[^\n]*\n|[^\n]+/g).
func splitLinesWithEndings(content string) []string {
	var out []string
	for i := 0; i < len(content); {
		j := strings.IndexByte(content[i:], '\n')
		if j == -1 {
			out = append(out, content[i:])
			break
		}
		out = append(out, content[i:i+j+1])
		i += j + 1
	}
	return out
}

type lineSpan struct{ start, end int }

func getLineSpans(content string) []lineSpan {
	lines := splitLinesWithEndings(content)
	spans := make([]lineSpan, len(lines))
	offset := 0
	for i, line := range lines {
		spans[i] = lineSpan{start: offset, end: offset + len(line)}
		offset = spans[i].end
	}
	return spans
}

type matchedEdit struct {
	editIndex   int
	matchIndex  int
	matchLength int
	newText     string
}

// getReplacementLineRange widens a replacement to the [startLine,endLine) lines
// it touches (port of pi's getReplacementLineRange). endLine is exclusive.
func getReplacementLineRange(lines []lineSpan, m matchedEdit) (startLine, endLine int, err error) {
	repStart, repEnd := m.matchIndex, m.matchIndex+m.matchLength
	startLine = -1
	for i, line := range lines {
		if repStart >= line.start && repStart < line.end {
			startLine = i
			break
		}
	}
	if startLine == -1 {
		//nolint:staticcheck // upstream message punctuation
		return 0, 0, fmt.Errorf("Replacement range is outside the base content.")
	}
	endLine = startLine
	for endLine < len(lines) && lines[endLine].end < repEnd {
		endLine++
	}
	if endLine >= len(lines) {
		//nolint:staticcheck // upstream message punctuation
		return 0, 0, fmt.Errorf("Replacement range is outside the base content.")
	}
	return startLine, endLine + 1, nil
}

// applyReplacements rewrites content with the given replacements, applied in
// reverse so earlier offsets stay valid (port of pi's applyReplacements). offset
// shifts each replacement's matchIndex into content-local coordinates.
func applyReplacements(content string, replacements []matchedEdit, offset int) string {
	result := content
	for _, r := range slices.Backward(replacements) {
		mi := r.matchIndex - offset
		result = result[:mi] + r.newText + result[mi+r.matchLength:]
	}
	return result
}

// ApplyReplacementsPreservingUnchangedLines overlays line-level replacements
// matched against baseContent (a normalized view) onto originalContent, copying
// untouched lines back verbatim (port of pi's function of the same name). The
// actual replacement ranges drive preservation so duplicate normalized lines
// cannot align to the wrong occurrence.
func ApplyReplacementsPreservingUnchangedLines(originalContent, baseContent string, matches []MatchedEdit) (string, error) {
	originalLines := splitLinesWithEndings(originalContent)
	baseLines := getLineSpans(baseContent)
	if len(originalLines) != len(baseLines) {
		//nolint:staticcheck // upstream message punctuation
		return "", fmt.Errorf("Cannot preserve unchanged lines because the base content has a different line count.")
	}

	sorted := make([]MatchedEdit, len(matches))
	copy(sorted, matches)
	sort.Slice(sorted, func(a, b int) bool { return sorted[a].MatchIndex < sorted[b].MatchIndex })

	type group struct {
		startLine, endLine int
		replacements       []matchedEdit
	}
	var groups []group
	for _, r := range sorted {
		m := matchedEdit{editIndex: r.EditIndex, matchIndex: r.MatchIndex, matchLength: r.MatchLength, newText: r.NewText}
		startLine, endLine, err := getReplacementLineRange(baseLines, m)
		if err != nil {
			return "", err
		}
		if n := len(groups); n > 0 && startLine < groups[n-1].endLine {
			if endLine > groups[n-1].endLine {
				groups[n-1].endLine = endLine
			}
			groups[n-1].replacements = append(groups[n-1].replacements, m)
			continue
		}
		groups = append(groups, group{startLine: startLine, endLine: endLine, replacements: []matchedEdit{m}})
	}

	var b strings.Builder
	originalLineIndex := 0
	for _, g := range groups {
		for _, line := range originalLines[originalLineIndex:g.startLine] {
			b.WriteString(line)
		}
		groupStartOffset := baseLines[g.startLine].start
		groupEndOffset := baseLines[g.endLine-1].end
		b.WriteString(applyReplacements(baseContent[groupStartOffset:groupEndOffset], g.replacements, groupStartOffset))
		originalLineIndex = g.endLine
	}
	for _, line := range originalLines[originalLineIndex:] {
		b.WriteString(line)
	}
	return b.String(), nil
}

// MatchedEdit is one replacement located in the content used for matching. It
// is exported so callers of ApplyReplacementsPreservingUnchangedLines can pass
// match positions.
type MatchedEdit struct {
	EditIndex   int
	MatchIndex  int
	MatchLength int
	NewText     string
}

// FuzzyMatchResult is the result of FuzzyFindText.
type FuzzyMatchResult struct {
	// Found reports whether a match was found.
	Found bool
	// Index is the byte offset of the match in ContentForReplacement.
	Index int
	// MatchLength is the byte length of the matched text.
	MatchLength int
	// UsedFuzzyMatch reports whether fuzzy normalization was required.
	UsedFuzzyMatch bool
	// ContentForReplacement is the content to use for replacement: the original
	// for an exact match, the fuzzy-normalized view otherwise.
	ContentForReplacement string
}

// FuzzyFindText finds oldText in content, trying an exact match first and then a
// fuzzy match. When fuzzy matching is used, indices and lengths are in the
// normalized content space (port of edit-diff.ts fuzzyFindText).
func FuzzyFindText(content, oldText string) FuzzyMatchResult {
	if i := strings.Index(content, oldText); i != -1 {
		return FuzzyMatchResult{Found: true, Index: i, MatchLength: len(oldText), ContentForReplacement: content}
	}
	fuzzyContent := NormalizeForFuzzyMatch(content)
	fuzzyOldText := NormalizeForFuzzyMatch(oldText)
	if i := strings.Index(fuzzyContent, fuzzyOldText); i != -1 {
		return FuzzyMatchResult{
			Found:                 true,
			Index:                 i,
			MatchLength:           len(fuzzyOldText),
			UsedFuzzyMatch:        true,
			ContentForReplacement: fuzzyContent,
		}
	}
	return FuzzyMatchResult{Found: false, Index: -1, ContentForReplacement: content}
}

func countOccurrences(content, oldText string) int {
	return strings.Count(NormalizeForFuzzyMatch(content), NormalizeForFuzzyMatch(oldText))
}

func emptyOldTextError(path string, editIndex, totalEdits int) error {
	if totalEdits == 1 {
		//nolint:staticcheck // upstream message punctuation
		return fmt.Errorf("oldText must not be empty in %s.", path)
	}
	//nolint:staticcheck // upstream message punctuation
	return fmt.Errorf("edits[%d].oldText must not be empty in %s.", editIndex, path)
}

func notFoundError(path string, editIndex, totalEdits int) error {
	if totalEdits == 1 {
		//nolint:staticcheck // upstream message punctuation
		return fmt.Errorf("Could not find the exact text in %s. The old text must match exactly including all whitespace and newlines.", path)
	}
	//nolint:staticcheck // upstream message punctuation
	return fmt.Errorf("Could not find edits[%d] in %s. The oldText must match exactly including all whitespace and newlines.", editIndex, path)
}

func duplicateError(path string, editIndex, totalEdits, occurrences int) error {
	if totalEdits == 1 {
		//nolint:staticcheck // upstream message punctuation
		return fmt.Errorf("Found %d occurrences of the text in %s. The text must be unique. Please provide more context to make it unique.", occurrences, path)
	}
	//nolint:staticcheck // upstream message punctuation
	return fmt.Errorf("Found %d occurrences of edits[%d] in %s. Each oldText must be unique. Please provide more context to make it unique.", occurrences, editIndex, path)
}

func noChangeError(path string, totalEdits int) error {
	if totalEdits == 1 {
		//nolint:staticcheck // upstream message punctuation
		return fmt.Errorf("No changes made to %s. The replacement produced identical content. This might indicate an issue with special characters or the text not existing as expected.", path)
	}
	//nolint:staticcheck // upstream message punctuation
	return fmt.Errorf("No changes made to %s. The replacements produced identical content.", path)
}

// ApplyEditsToNormalizedContent applies edits to LF-normalized content using
// exact-then-fuzzy matching, with pi's uniqueness, overlap and no-change checks
// (port of applyEditsToNormalizedContent). It returns the base content the edits
// matched against and the resulting content.
func ApplyEditsToNormalizedContent(normalizedContent string, edits []Edit, path string) (AppliedEditsResult, error) {
	total := len(edits)
	normalized := make([]Edit, total)
	for i, e := range edits {
		normalized[i] = Edit{OldText: NormalizeToLF(e.OldText), NewText: NormalizeToLF(e.NewText)}
		if normalized[i].OldText == "" {
			return AppliedEditsResult{}, emptyOldTextError(path, i, total)
		}
	}

	anyFuzzy := false
	for _, e := range normalized {
		if FuzzyFindText(normalizedContent, e.OldText).UsedFuzzyMatch {
			anyFuzzy = true
			break
		}
	}
	replacementBase := normalizedContent
	if anyFuzzy {
		replacementBase = NormalizeForFuzzyMatch(normalizedContent)
	}

	matched := make([]MatchedEdit, 0, total)
	for i, e := range normalized {
		m := FuzzyFindText(replacementBase, e.OldText)
		if !m.Found {
			return AppliedEditsResult{}, notFoundError(path, i, total)
		}
		if occ := countOccurrences(replacementBase, e.OldText); occ > 1 {
			return AppliedEditsResult{}, duplicateError(path, i, total, occ)
		}
		matched = append(matched, MatchedEdit{EditIndex: i, MatchIndex: m.Index, MatchLength: m.MatchLength, NewText: e.NewText})
	}

	sort.Slice(matched, func(a, b int) bool { return matched[a].MatchIndex < matched[b].MatchIndex })
	for i := 1; i < len(matched); i++ {
		prev, cur := matched[i-1], matched[i]
		if prev.MatchIndex+prev.MatchLength > cur.MatchIndex {
			//nolint:staticcheck // upstream message punctuation
			return AppliedEditsResult{}, fmt.Errorf("edits[%d] and edits[%d] overlap in %s. Merge them into one edit or target disjoint regions.", prev.EditIndex, cur.EditIndex, path)
		}
	}

	baseContent := normalizedContent
	var newContent string
	if anyFuzzy {
		var err error
		newContent, err = ApplyReplacementsPreservingUnchangedLines(normalizedContent, replacementBase, matched)
		if err != nil {
			return AppliedEditsResult{}, err
		}
	} else {
		newContent = applyReplacements(replacementBase, toInternalMatches(matched), 0)
	}
	if baseContent == newContent {
		return AppliedEditsResult{}, noChangeError(path, total)
	}
	return AppliedEditsResult{BaseContent: baseContent, NewContent: newContent}, nil
}

func toInternalMatches(matches []MatchedEdit) []matchedEdit {
	out := make([]matchedEdit, len(matches))
	for i, m := range matches {
		out[i] = matchedEdit{editIndex: m.EditIndex, matchIndex: m.MatchIndex, matchLength: m.MatchLength, newText: m.NewText}
	}
	return out
}
