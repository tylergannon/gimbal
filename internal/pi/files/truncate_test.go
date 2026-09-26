package files

import (
	"encoding/json"
	"strings"
	"testing"
)

//go:fix inline

func linesOf(n int) string {
	var b strings.Builder
	for i := range n {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString("line")
	}
	return b.String()
}

func TestFormatSize(t *testing.T) {
	tests := []struct {
		bytes int
		want  string
	}{
		{0, "0B"},
		{1, "1B"},
		{1023, "1023B"},
		{1024, "1.0KB"},
		{1280, "1.3KB"}, // exactly 1.25KB: toFixed rounds half up
		{1536, "1.5KB"},
		{10240, "10.0KB"},
		{51200, "50.0KB"},
		{1048575, "1024.0KB"},
		{1048576, "1.0MB"},
		{1572864, "1.5MB"},
		{1500000, "1.4MB"},
	}
	for _, tc := range tests {
		if got := FormatSize(tc.bytes); got != tc.want {
			t.Errorf("FormatSize(%d) = %q, want %q", tc.bytes, got, tc.want)
		}
	}
}

func TestTruncateHeadNoTruncation(t *testing.T) {
	content := "alpha\nbeta\ngamma"
	got := TruncateHead(content, Options{})

	if got.Content != content {
		t.Errorf("Content = %q, want %q", got.Content, content)
	}
	if got.Truncated {
		t.Error("Truncated = true, want false")
	}
	if got.TruncatedBy != nil {
		t.Errorf("TruncatedBy = %v, want nil", *got.TruncatedBy)
	}
	if got.TotalLines != 3 || got.OutputLines != 3 {
		t.Errorf("lines = %d/%d, want 3/3", got.OutputLines, got.TotalLines)
	}
	if got.TotalBytes != len(content) || got.OutputBytes != len(content) {
		t.Errorf("bytes = %d/%d, want %d", got.OutputBytes, got.TotalBytes, len(content))
	}
	if got.MaxLines != DefaultMaxLines || got.MaxBytes != DefaultMaxBytes {
		t.Errorf("limits = %d/%d, want defaults %d/%d", got.MaxLines, got.MaxBytes, DefaultMaxLines, DefaultMaxBytes)
	}
	if got.LastLinePartial || got.FirstLineExceedsLimit {
		t.Error("unexpected partial/limit flags")
	}
}

func TestTruncateHeadByLines(t *testing.T) {
	content := "line1\nline2\nline3"
	got := TruncateHead(content, Options{MaxLines: new(2)})

	if got.Content != "line1\nline2" {
		t.Errorf("Content = %q, want %q", got.Content, "line1\nline2")
	}
	if !got.Truncated || got.TruncatedBy == nil || *got.TruncatedBy != "lines" {
		t.Errorf("TruncatedBy = %v, want lines", got.TruncatedBy)
	}
	if got.TotalLines != 3 || got.OutputLines != 2 {
		t.Errorf("lines = %d/%d, want 2/3", got.OutputLines, got.TotalLines)
	}
	if got.TotalBytes != 17 || got.OutputBytes != 11 {
		t.Errorf("bytes = %d/%d, want 11/17", got.OutputBytes, got.TotalBytes)
	}
}

func TestTruncateHeadByBytes(t *testing.T) {
	content := "aaaa\nbbbb"
	got := TruncateHead(content, Options{MaxBytes: new(6)})

	if got.Content != "aaaa" {
		t.Errorf("Content = %q, want %q", got.Content, "aaaa")
	}
	if got.TruncatedBy == nil || *got.TruncatedBy != "bytes" {
		t.Errorf("TruncatedBy = %v, want bytes", got.TruncatedBy)
	}
	if got.OutputLines != 1 || got.OutputBytes != 4 {
		t.Errorf("output = %d lines/%d bytes, want 1/4", got.OutputLines, got.OutputBytes)
	}
	if got.TotalLines != 2 || got.TotalBytes != 9 {
		t.Errorf("total = %d lines/%d bytes, want 2/9", got.TotalLines, got.TotalBytes)
	}
	if got.FirstLineExceedsLimit {
		t.Error("FirstLineExceedsLimit = true, want false")
	}
}

func TestTruncateHeadFirstLineExceedsByteLimit(t *testing.T) {
	content := "aaaaaaaaaa"
	got := TruncateHead(content, Options{MaxBytes: new(5)})

	if got.Content != "" {
		t.Errorf("Content = %q, want empty", got.Content)
	}
	if !got.Truncated || got.TruncatedBy == nil || *got.TruncatedBy != "bytes" {
		t.Errorf("TruncatedBy = %v, want bytes", got.TruncatedBy)
	}
	if !got.FirstLineExceedsLimit {
		t.Error("FirstLineExceedsLimit = false, want true")
	}
	if got.OutputLines != 0 || got.OutputBytes != 0 {
		t.Errorf("output = %d lines/%d bytes, want 0/0", got.OutputLines, got.OutputBytes)
	}
}

func TestTruncateHeadTrailingNewlineAndEmpty(t *testing.T) {
	got := TruncateHead("a\nb\n", Options{})
	if got.Content != "a\nb\n" {
		t.Errorf("Content = %q, want %q", got.Content, "a\nb\n")
	}
	if got.TotalLines != 2 {
		t.Errorf("TotalLines = %d, want 2", got.TotalLines)
	}

	empty := TruncateHead("", Options{})
	if empty.Content != "" || empty.Truncated || empty.TotalLines != 0 || empty.TotalBytes != 0 {
		t.Errorf("empty result = %+v", empty)
	}
}

func TestTruncateHeadExplicitZero(t *testing.T) {
	got := TruncateHead("a", Options{MaxLines: new(0)})
	if got.Content != "" || !got.Truncated || got.MaxLines != 0 {
		t.Errorf("result = %+v, want truncated with zero max lines", got)
	}
	if got.TruncatedBy == nil || *got.TruncatedBy != "lines" {
		t.Errorf("TruncatedBy = %v, want lines", got.TruncatedBy)
	}

	byteLimited := TruncateHead("abc", Options{MaxBytes: new(0)})
	if byteLimited.Content != "" || !byteLimited.FirstLineExceedsLimit {
		t.Errorf("result = %+v, want first line exceeds limit", byteLimited)
	}
	if byteLimited.TruncatedBy == nil || *byteLimited.TruncatedBy != "bytes" {
		t.Errorf("TruncatedBy = %v, want bytes", byteLimited.TruncatedBy)
	}
}

func TestTruncateHeadUTF8Bytes(t *testing.T) {
	// "é" is two bytes; the head keeps whole lines only.
	content := "é\nplain"
	got := TruncateHead(content, Options{MaxBytes: new(3)})
	if got.Content != "é" {
		t.Errorf("Content = %q, want %q", got.Content, "é")
	}
	if got.OutputBytes != 2 {
		t.Errorf("OutputBytes = %d, want 2", got.OutputBytes)
	}
}

func TestTruncateTailNoTruncation(t *testing.T) {
	content := "one\ntwo"
	got := TruncateTail(content, Options{})
	if got.Content != content || got.Truncated || got.TruncatedBy != nil {
		t.Errorf("result = %+v, want untouched", got)
	}
}

func TestTruncateTailByLines(t *testing.T) {
	content := "line1\nline2\nline3"
	got := TruncateTail(content, Options{MaxLines: new(2)})

	if got.Content != "line2\nline3" {
		t.Errorf("Content = %q, want %q", got.Content, "line2\nline3")
	}
	if got.TruncatedBy == nil || *got.TruncatedBy != "lines" {
		t.Errorf("TruncatedBy = %v, want lines", got.TruncatedBy)
	}
	if got.TotalLines != 3 || got.OutputLines != 2 || got.OutputBytes != 11 {
		t.Errorf("result = %+v", got)
	}
}

func TestTruncateTailByBytes(t *testing.T) {
	content := "aaaa\nbbbb"
	got := TruncateTail(content, Options{MaxBytes: new(6)})

	if got.Content != "bbbb" {
		t.Errorf("Content = %q, want %q", got.Content, "bbbb")
	}
	if got.TruncatedBy == nil || *got.TruncatedBy != "bytes" {
		t.Errorf("TruncatedBy = %v, want bytes", got.TruncatedBy)
	}
	if got.OutputLines != 1 || got.OutputBytes != 4 || got.TotalLines != 2 {
		t.Errorf("result = %+v", got)
	}
}

func TestTruncateTailPartialLine(t *testing.T) {
	got := TruncateTail("abcdefghij", Options{MaxBytes: new(4)})

	if got.Content != "ghij" {
		t.Errorf("Content = %q, want %q", got.Content, "ghij")
	}
	if !got.LastLinePartial {
		t.Error("LastLinePartial = false, want true")
	}
	if got.OutputLines != 1 || got.OutputBytes != 4 {
		t.Errorf("output = %d lines/%d bytes, want 1/4", got.OutputLines, got.OutputBytes)
	}
	if got.TruncatedBy == nil || *got.TruncatedBy != "bytes" {
		t.Errorf("TruncatedBy = %v, want bytes", got.TruncatedBy)
	}
}

func TestTruncateTailPartialUTF8Boundary(t *testing.T) {
	// "aé" is bytes 0x61 0xc3 0xa9. Taking the last 1 byte must yield "".
	if got := TruncateTail("aé", Options{MaxBytes: new(1)}); got.Content != "" {
		t.Errorf("Content = %q, want empty (no split rune)", got.Content)
	}
	// Taking the last 2 bytes lands on the 0xc3 leading byte and keeps "é".
	if got := TruncateTail("aé", Options{MaxBytes: new(2)}); got.Content != "é" {
		t.Errorf("Content = %q, want %q", got.Content, "é")
	}
}

func TestTruncateTailTrailingNewline(t *testing.T) {
	got := TruncateTail("a\nb\n", Options{MaxLines: new(1)})
	if got.Content != "b" {
		t.Errorf("Content = %q, want %q", got.Content, "b")
	}
	if got.TotalLines != 2 || got.OutputLines != 1 {
		t.Errorf("lines = %d/%d, want 1/2", got.OutputLines, got.TotalLines)
	}
}

func TestTruncateHeadExactlyAtLineLimit(t *testing.T) {
	content := linesOf(DefaultMaxLines)
	got := TruncateHead(content, Options{})
	if got.Truncated {
		t.Errorf("Truncated = true, want false at exactly %d lines", DefaultMaxLines)
	}
	if got.OutputLines != DefaultMaxLines {
		t.Errorf("OutputLines = %d, want %d", got.OutputLines, DefaultMaxLines)
	}

	over := linesOf(DefaultMaxLines + 1)
	got = TruncateHead(over, Options{})
	if !got.Truncated || got.TruncatedBy == nil || *got.TruncatedBy != "lines" {
		t.Errorf("TruncatedBy = %v, want lines", got.TruncatedBy)
	}
	if got.OutputLines != DefaultMaxLines {
		t.Errorf("OutputLines = %d, want %d", got.OutputLines, DefaultMaxLines)
	}
}

func TestTruncatedByJSONNull(t *testing.T) {
	got := TruncateHead("hello", Options{})
	b, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if !strings.Contains(string(b), `"truncatedBy":null`) {
		t.Errorf("JSON %s does not contain null truncatedBy", b)
	}

	truncated := TruncateHead("hello", Options{MaxLines: new(0)})
	b, err = json.Marshal(truncated)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if !strings.Contains(string(b), `"truncatedBy":"lines"`) {
		t.Errorf("JSON %s does not contain lines truncatedBy", b)
	}
}

func TestTruncateLine(t *testing.T) {
	if text, truncated := TruncateLine("short", 0); text != "short" || truncated {
		t.Errorf("short line = %q, %v", text, truncated)
	}
	text, truncated := TruncateLine("abcdefghij", 5)
	if text != "abcde... [truncated]" || !truncated {
		t.Errorf("truncated line = %q, %v", text, truncated)
	}
	// A two-unit character straddling the limit is cut, leaving a replacement rune.
	text, truncated = TruncateLine("a😀b", 2)
	if !truncated || text != "a\uFFFD... [truncated]" {
		t.Errorf("surrogate line = %q, %v", text, truncated)
	}
}
