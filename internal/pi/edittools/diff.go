package edittools

import (
	"fmt"
	"strconv"
	"strings"
)

// maxDiffCells bounds the LCS table for the changed middle of two files. Edits
// are small, so the middle is normally tiny; a wholesale rewrite falls back to
// a delete-all/insert-all script rather than allocating an unbounded table.
const maxDiffCells = 4_000_000

type lineOpKind int

const (
	opEqual lineOpKind = iota
	opDelete
	opInsert
)

type lineOp struct {
	kind lineOpKind
	text string
}

type diffPart struct {
	added   bool
	removed bool
	value   string
}

// diffLineOps produces a line-level edit script. It strips the common prefix
// and suffix first so the LCS table only covers the changed middle.
func diffLineOps(oldLines, newLines []string) []lineOp {
	n, m := len(oldLines), len(newLines)
	prefix := 0
	for prefix < n && prefix < m && oldLines[prefix] == newLines[prefix] {
		prefix++
	}
	suffix := 0
	for suffix < n-prefix && suffix < m-prefix && oldLines[n-1-suffix] == newLines[m-1-suffix] {
		suffix++
	}

	ops := make([]lineOp, 0, n+m)
	for i := 0; i < prefix; i++ {
		ops = append(ops, lineOp{opEqual, oldLines[i]})
	}
	ops = append(ops, diffMiddle(oldLines[prefix:n-suffix], newLines[prefix:m-suffix])...)
	for i := n - suffix; i < n; i++ {
		ops = append(ops, lineOp{opEqual, oldLines[i]})
	}
	return ops
}

func diffMiddle(a, b []string) []lineOp {
	n, m := len(a), len(b)
	switch {
	case n == 0:
		return insertAll(b)
	case m == 0:
		return deleteAll(a)
	}
	if n*m > maxDiffCells {
		ops := deleteAll(a)
		return append(ops, insertAll(b)...)
	}

	dp := make([][]int, n+1)
	for i := range dp {
		dp[i] = make([]int, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if a[i] == b[j] {
				dp[i][j] = dp[i+1][j+1] + 1
			} else if dp[i+1][j] >= dp[i][j+1] {
				dp[i][j] = dp[i+1][j]
			} else {
				dp[i][j] = dp[i][j+1]
			}
		}
	}

	ops := make([]lineOp, 0, n+m)
	i, j := 0, 0
	for i < n && j < m {
		switch {
		case a[i] == b[j]:
			ops = append(ops, lineOp{opEqual, a[i]})
			i++
			j++
		case dp[i+1][j] >= dp[i][j+1]:
			ops = append(ops, lineOp{opDelete, a[i]})
			i++
		default:
			ops = append(ops, lineOp{opInsert, b[j]})
			j++
		}
	}
	for ; i < n; i++ {
		ops = append(ops, lineOp{opDelete, a[i]})
	}
	for ; j < m; j++ {
		ops = append(ops, lineOp{opInsert, b[j]})
	}
	return ops
}

func deleteAll(lines []string) []lineOp {
	ops := make([]lineOp, 0, len(lines))
	for _, line := range lines {
		ops = append(ops, lineOp{opDelete, line})
	}
	return ops
}

func insertAll(lines []string) []lineOp {
	ops := make([]lineOp, 0, len(lines))
	for _, line := range lines {
		ops = append(ops, lineOp{opInsert, line})
	}
	return ops
}

func diffPartsFromOps(ops []lineOp) []diffPart {
	var parts []diffPart
	for _, op := range ops {
		added := op.kind == opInsert
		removed := op.kind == opDelete
		if n := len(parts); n > 0 && parts[n-1].added == added && parts[n-1].removed == removed {
			parts[n-1].value += op.text
			continue
		}
		parts = append(parts, diffPart{added: added, removed: removed, value: op.text})
	}
	return parts
}

func padStart(value string, width int) string {
	if len(value) >= width {
		return value
	}
	return strings.Repeat(" ", width-len(value)) + value
}

// GenerateDiffString renders a display-oriented diff with line numbers and
// collapsed context, returning the diff and the first changed line in the new
// file (port of edit-diff.ts generateDiffString with its default four context
// lines).
func GenerateDiffString(oldContent, newContent string) (string, int, bool) {
	return generateDiffString(oldContent, newContent, 4)
}

func generateDiffString(oldContent, newContent string, contextLines int) (string, int, bool) {
	parts := diffPartsFromOps(diffLineOps(splitLinesWithEndings(oldContent), splitLinesWithEndings(newContent)))
	var output []string

	oldLines := strings.Split(oldContent, "\n")
	newLines := strings.Split(newContent, "\n")
	maxLineNum := max(len(oldLines), len(newLines))
	lineNumWidth := len(strconv.Itoa(maxLineNum))

	oldLineNum, newLineNum := 1, 1
	lastWasChange := false
	firstChangedLine := 0
	hasFirstChangedLine := false

	for i, part := range parts {
		raw := strings.Split(part.value, "\n")
		if len(raw) > 0 && raw[len(raw)-1] == "" {
			raw = raw[:len(raw)-1]
		}

		if part.added || part.removed {
			if !hasFirstChangedLine {
				firstChangedLine = newLineNum
				hasFirstChangedLine = true
			}
			for _, line := range raw {
				if part.added {
					output = append(output, "+"+padStart(strconv.Itoa(newLineNum), lineNumWidth)+" "+line)
					newLineNum++
				} else {
					output = append(output, "-"+padStart(strconv.Itoa(oldLineNum), lineNumWidth)+" "+line)
					oldLineNum++
				}
			}
			lastWasChange = true
			continue
		}

		nextPartIsChange := i < len(parts)-1 && (parts[i+1].added || parts[i+1].removed)
		hasLeadingChange := lastWasChange
		hasTrailingChange := nextPartIsChange

		switch {
		case hasLeadingChange && hasTrailingChange:
			if len(raw) <= contextLines*2 {
				for _, line := range raw {
					output = append(output, " "+padStart(strconv.Itoa(oldLineNum), lineNumWidth)+" "+line)
					oldLineNum++
					newLineNum++
				}
			} else {
				leading := raw[:contextLines]
				trailing := raw[len(raw)-contextLines:]
				skipped := len(raw) - len(leading) - len(trailing)
				for _, line := range leading {
					output = append(output, " "+padStart(strconv.Itoa(oldLineNum), lineNumWidth)+" "+line)
					oldLineNum++
					newLineNum++
				}
				output = append(output, " "+strings.Repeat(" ", lineNumWidth)+" ...")
				oldLineNum += skipped
				newLineNum += skipped
				for _, line := range trailing {
					output = append(output, " "+padStart(strconv.Itoa(oldLineNum), lineNumWidth)+" "+line)
					oldLineNum++
					newLineNum++
				}
			}
		case hasLeadingChange:
			shown := raw
			if len(raw) > contextLines {
				shown = raw[:contextLines]
			}
			skipped := len(raw) - len(shown)
			for _, line := range shown {
				output = append(output, " "+padStart(strconv.Itoa(oldLineNum), lineNumWidth)+" "+line)
				oldLineNum++
				newLineNum++
			}
			if skipped > 0 {
				output = append(output, " "+strings.Repeat(" ", lineNumWidth)+" ...")
				oldLineNum += skipped
				newLineNum += skipped
			}
		case hasTrailingChange:
			skipped := max(0, len(raw)-contextLines)
			if skipped > 0 {
				output = append(output, " "+strings.Repeat(" ", lineNumWidth)+" ...")
				oldLineNum += skipped
				newLineNum += skipped
			}
			for _, line := range raw[skipped:] {
				output = append(output, " "+padStart(strconv.Itoa(oldLineNum), lineNumWidth)+" "+line)
				oldLineNum++
				newLineNum++
			}
		default:
			oldLineNum += len(raw)
			newLineNum += len(raw)
		}
		lastWasChange = false
	}

	return strings.Join(output, "\n"), firstChangedLine, hasFirstChangedLine
}

// GenerateUnifiedPatch produces a standard unified patch with four context
// lines (port of edit-diff.ts generateUnifiedPatch). It has no trailing
// newline, matching the upstream string.
func GenerateUnifiedPatch(path, oldContent, newContent string) string {
	return generateUnifiedPatch(path, oldContent, newContent, 4)
}

type unifiedLine struct {
	kind  byte
	oldNo int
	newNo int
	text  string
}

func generateUnifiedPatch(path, oldContent, newContent string, contextLines int) string {
	ops := diffLineOps(splitLinesWithEndings(oldContent), splitLinesWithEndings(newContent))
	lines := make([]unifiedLine, 0, len(ops))
	oldNo, newNo := 1, 1
	for _, op := range ops {
		switch op.kind {
		case opEqual:
			lines = append(lines, unifiedLine{' ', oldNo, newNo, op.text})
			oldNo++
			newNo++
		case opDelete:
			lines = append(lines, unifiedLine{'-', oldNo, newNo, op.text})
			oldNo++
		case opInsert:
			lines = append(lines, unifiedLine{'+', oldNo, newNo, op.text})
			newNo++
		}
	}

	var changed []int
	for i, line := range lines {
		if line.kind != ' ' {
			changed = append(changed, i)
		}
	}
	type hunk struct{ start, end int }
	var hunks []hunk
	curStart, curEnd := -1, -1
	for _, c := range changed {
		s := max(0, c-contextLines)
		e := min(len(lines), c+contextLines+1)
		if curStart == -1 || s > curEnd {
			if curStart != -1 {
				hunks = append(hunks, hunk{curStart, curEnd})
			}
			curStart, curEnd = s, e
			continue
		}
		if e > curEnd {
			curEnd = e
		}
	}
	if curStart != -1 {
		hunks = append(hunks, hunk{curStart, curEnd})
	}

	var b strings.Builder
	b.WriteString("--- " + path + "\n")
	b.WriteString("+++ " + path + "\n")
	for _, h := range hunks {
		oldStart := lines[h.start].oldNo
		newStart := lines[h.start].newNo
		oldCount, newCount := 0, 0
		for _, line := range lines[h.start:h.end] {
			if line.kind != '+' {
				oldCount++
			}
			if line.kind != '-' {
				newCount++
			}
		}
		fmt.Fprintf(&b, "@@ -%d,%d +%d,%d @@\n", oldStart, oldCount, newStart, newCount)
		for _, line := range lines[h.start:h.end] {
			b.WriteByte(line.kind)
			b.WriteString(line.text)
			if !strings.HasSuffix(line.text, "\n") {
				b.WriteString("\n\\ No newline at end of file\n")
			}
		}
	}
	return b.String()
}
