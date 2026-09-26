package files

import (
	"math"
	"strconv"
	"strings"
)

// Truncation is based on two independent limits - whichever is hit first wins:
// a line limit and a byte limit. Output never contains partial lines, except
// for the tail truncation edge case where a single line exceeds the byte limit.

// DefaultMaxLines is the default line limit applied when Options.MaxLines is nil.
const DefaultMaxLines = 2000

// DefaultMaxBytes is the default byte limit applied when Options.MaxBytes is nil.
const DefaultMaxBytes = 50 * 1024 // 50KB

// GrepMaxLineLength is the maximum number of characters kept per grep match line.
const GrepMaxLineLength = 500

// Options controls truncation. A nil field means "use the upstream default";
// an explicit zero is honored as zero.
type Options struct {
	MaxLines *int
	MaxBytes *int
}

// Result describes the outcome of a truncation. Its JSON field names match the
// upstream TruncationResult shape. TruncatedBy is nil (JSON null) when no
// truncation occurred, or points to "lines" or "bytes" otherwise.
type Result struct {
	Content               string  `json:"content"`
	Truncated             bool    `json:"truncated"`
	TruncatedBy           *string `json:"truncatedBy"`
	TotalLines            int     `json:"totalLines"`
	TotalBytes            int     `json:"totalBytes"`
	OutputLines           int     `json:"outputLines"`
	OutputBytes           int     `json:"outputBytes"`
	LastLinePartial       bool    `json:"lastLinePartial"`
	FirstLineExceedsLimit bool    `json:"firstLineExceedsLimit"`
	MaxLines              int     `json:"maxLines"`
	MaxBytes              int     `json:"maxBytes"`
}

func resolveLimits(opts Options) (maxLines, maxBytes int) {
	maxLines = DefaultMaxLines
	if opts.MaxLines != nil {
		maxLines = *opts.MaxLines
	}
	maxBytes = DefaultMaxBytes
	if opts.MaxBytes != nil {
		maxBytes = *opts.MaxBytes
	}
	return maxLines, maxBytes
}

// splitLinesForCounting returns the lines used for counting. A trailing
// newline does not produce an extra empty line, and empty content has no lines.
func splitLinesForCounting(content string) []string {
	if len(content) == 0 {
		return []string{}
	}
	lines := strings.Split(content, "\n")
	if strings.HasSuffix(content, "\n") {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// FormatSize renders a byte count as a human-readable size.
func FormatSize(bytes int) string {
	switch {
	case bytes < 1024:
		return strconv.Itoa(bytes) + "B"
	case bytes < 1024*1024:
		return toFixed1(float64(bytes)/1024) + "KB"
	default:
		return toFixed1(float64(bytes)/(1024*1024)) + "MB"
	}
}

// toFixed1 mimics JavaScript's Number.prototype.toFixed(1), which rounds ties
// toward positive infinity.
func toFixed1(v float64) string {
	rounded := math.Floor(v*10 + 0.5)
	return strconv.FormatFloat(rounded/10, 'f', 1, 64)
}

// TruncateHead keeps the first N lines/bytes. It never returns partial lines;
// if the first line alone exceeds the byte limit it returns empty content with
// FirstLineExceedsLimit set.
func TruncateHead(content string, opts Options) Result {
	maxLines, maxBytes := resolveLimits(opts)

	totalBytes := len(content)
	lines := splitLinesForCounting(content)
	totalLines := len(lines)

	if totalLines <= maxLines && totalBytes <= maxBytes {
		return Result{
			Content:               content,
			Truncated:             false,
			TruncatedBy:           nil,
			TotalLines:            totalLines,
			TotalBytes:            totalBytes,
			OutputLines:           totalLines,
			OutputBytes:           totalBytes,
			LastLinePartial:       false,
			FirstLineExceedsLimit: false,
			MaxLines:              maxLines,
			MaxBytes:              maxBytes,
		}
	}

	firstLineBytes := 0
	if len(lines) > 0 {
		firstLineBytes = len(lines[0])
	}
	if firstLineBytes > maxBytes {
		return Result{
			Content:               "",
			Truncated:             true,
			TruncatedBy:           new("bytes"),
			TotalLines:            totalLines,
			TotalBytes:            totalBytes,
			OutputLines:           0,
			OutputBytes:           0,
			LastLinePartial:       false,
			FirstLineExceedsLimit: true,
			MaxLines:              maxLines,
			MaxBytes:              maxBytes,
		}
	}

	outputLines := make([]string, 0, len(lines))
	outputBytesCount := 0
	truncatedBy := "lines"

	for i := 0; i < len(lines) && i < maxLines; i++ {
		line := lines[i]
		lineBytes := len(line)
		if i > 0 {
			lineBytes++ // +1 for newline
		}

		if outputBytesCount+lineBytes > maxBytes {
			truncatedBy = "bytes"
			break
		}

		outputLines = append(outputLines, line)
		outputBytesCount += lineBytes
	}

	if len(outputLines) >= maxLines && outputBytesCount <= maxBytes {
		truncatedBy = "lines"
	}

	outputContent := strings.Join(outputLines, "\n")

	return Result{
		Content:               outputContent,
		Truncated:             true,
		TruncatedBy:           new(truncatedBy),
		TotalLines:            totalLines,
		TotalBytes:            totalBytes,
		OutputLines:           len(outputLines),
		OutputBytes:           len(outputContent),
		LastLinePartial:       false,
		FirstLineExceedsLimit: false,
		MaxLines:              maxLines,
		MaxBytes:              maxBytes,
	}
}

// TruncateTail keeps the last N lines/bytes. It may return a partial first line
// when the final line of the original content exceeds the byte limit.
func TruncateTail(content string, opts Options) Result {
	maxLines, maxBytes := resolveLimits(opts)

	totalBytes := len(content)
	lines := splitLinesForCounting(content)
	totalLines := len(lines)

	if totalLines <= maxLines && totalBytes <= maxBytes {
		return Result{
			Content:               content,
			Truncated:             false,
			TruncatedBy:           nil,
			TotalLines:            totalLines,
			TotalBytes:            totalBytes,
			OutputLines:           totalLines,
			OutputBytes:           totalBytes,
			LastLinePartial:       false,
			FirstLineExceedsLimit: false,
			MaxLines:              maxLines,
			MaxBytes:              maxBytes,
		}
	}

	outputLines := make([]string, 0, len(lines))
	outputBytesCount := 0
	truncatedBy := "lines"
	lastLinePartial := false

	for i := len(lines) - 1; i >= 0 && len(outputLines) < maxLines; i-- {
		line := lines[i]
		lineBytes := len(line)
		if len(outputLines) > 0 {
			lineBytes++ // +1 for newline
		}

		if outputBytesCount+lineBytes > maxBytes {
			truncatedBy = "bytes"
			// Edge case: if we haven't added any lines yet and this line
			// exceeds maxBytes, take the end of the line (partial).
			if len(outputLines) == 0 {
				truncatedLine := truncateStringToBytesFromEnd(line, maxBytes)
				outputLines = append([]string{truncatedLine}, outputLines...)
				outputBytesCount = len(truncatedLine)
				lastLinePartial = true
			}
			break
		}

		outputLines = append([]string{line}, outputLines...)
		outputBytesCount += lineBytes
	}

	if len(outputLines) >= maxLines && outputBytesCount <= maxBytes {
		truncatedBy = "lines"
	}

	outputContent := strings.Join(outputLines, "\n")

	return Result{
		Content:               outputContent,
		Truncated:             true,
		TruncatedBy:           new(truncatedBy),
		TotalLines:            totalLines,
		TotalBytes:            totalBytes,
		OutputLines:           len(outputLines),
		OutputBytes:           len(outputContent),
		LastLinePartial:       lastLinePartial,
		FirstLineExceedsLimit: false,
		MaxLines:              maxLines,
		MaxBytes:              maxBytes,
	}
}

// truncateStringToBytesFromEnd truncates a string to fit within maxBytes,
// measured from the end, preserving UTF-8 boundaries.
func truncateStringToBytesFromEnd(s string, maxBytes int) string {
	if len(s) <= maxBytes {
		return s
	}
	start := max(len(s)-maxBytes, 0)
	if start > len(s) {
		return ""
	}
	// Skip continuation bytes so we begin at a valid UTF-8 boundary.
	for start < len(s) && s[start]&0xc0 == 0x80 {
		start++
	}
	return s[start:]
}

// TruncateLine truncates a single line to maxChars, appending a marker. When
// maxChars is 0 the GrepMaxLineLength default applies. The character count uses
// UTF-16 code units like the upstream `line.length`/`line.slice` (astral
// characters count as 2).
func TruncateLine(line string, maxChars int) (text string, wasTruncated bool) {
	if maxChars == 0 {
		maxChars = GrepMaxLineLength
	}
	if utf16Len(line) <= maxChars {
		return line, false
	}
	var b strings.Builder
	n := 0
	for _, r := range line {
		units := 1
		if r > 0xFFFF {
			units = 2
		}
		if n+units > maxChars {
			if units == 2 && n+1 == maxChars {
				// A JS slice cuts mid-pair, leaving a lone high surrogate.
				b.WriteRune('\uFFFD')
			}
			break
		}
		b.WriteRune(r)
		n += units
		if n == maxChars {
			break
		}
	}
	return b.String() + "... [truncated]", true
}

// utf16Len returns the number of UTF-16 code units in s, matching JS String
// `.length` (astral characters count as 2).
func utf16Len(s string) int {
	n := 0
	for _, r := range s {
		if r > 0xFFFF {
			n += 2
		} else {
			n++
		}
	}
	return n
}
