package shell

import (
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/tylergannon/gimbal/internal/pi/files"
)

func TestOutputAccumulatorSmallSnapshot(t *testing.T) {
	acc := NewOutputAccumulator(OutputAccumulatorOptions{})
	acc.Append([]byte("hello\nworld\n"))
	acc.Finish()

	snapshot := acc.Snapshot(false)
	if snapshot.Content != "hello\nworld\n" {
		t.Fatalf("content = %q, want the full text", snapshot.Content)
	}
	if snapshot.Truncation.Truncated {
		t.Fatalf("truncation = %#v, want not truncated", snapshot.Truncation)
	}
	if snapshot.FullOutputPath != "" {
		t.Fatalf("full output path = %q, want empty", snapshot.FullOutputPath)
	}
}

func TestOutputAccumulatorLineTruncation(t *testing.T) {
	acc := NewOutputAccumulator(OutputAccumulatorOptions{})
	var b strings.Builder
	for i := 1; i <= 4000; i++ {
		b.WriteString("line-" + strconv.Itoa(i) + "\n")
	}
	acc.Append([]byte(b.String()))
	acc.Finish()

	snapshot := acc.Snapshot(false)
	if !snapshot.Truncation.Truncated {
		t.Fatalf("truncation = %#v, want truncated", snapshot.Truncation)
	}
	if snapshot.Truncation.TruncatedBy == nil || *snapshot.Truncation.TruncatedBy != "lines" {
		t.Fatalf("truncatedBy = %v, want lines", snapshot.Truncation.TruncatedBy)
	}
	if snapshot.Truncation.TotalLines != 4000 || snapshot.Truncation.OutputLines != 2000 {
		t.Fatalf("truncation = %#v, want 4000 total and 2000 output", snapshot.Truncation)
	}
	if !strings.Contains(snapshot.Content, "line-2001") || !strings.Contains(snapshot.Content, "line-4000") {
		t.Fatalf("content = %q, want the last 2000 lines", snapshot.Content)
	}
}

func TestOutputAccumulatorByteTruncation(t *testing.T) {
	acc := NewOutputAccumulator(OutputAccumulatorOptions{MaxLines: 1000, MaxBytes: 32})
	acc.Append([]byte(strings.Repeat("a", 100)))
	acc.Finish()

	snapshot := acc.Snapshot(false)
	if !snapshot.Truncation.Truncated {
		t.Fatalf("truncation = %#v, want truncated", snapshot.Truncation)
	}
	if snapshot.Truncation.TruncatedBy == nil || *snapshot.Truncation.TruncatedBy != "bytes" {
		t.Fatalf("truncatedBy = %v, want bytes", snapshot.Truncation.TruncatedBy)
	}
	if len(snapshot.Content) > 32 {
		t.Fatalf("content length = %d, want at most 32", len(snapshot.Content))
	}
}

func TestOutputAccumulatorDecodesSplitUTF8(t *testing.T) {
	acc := NewOutputAccumulator(OutputAccumulatorOptions{})
	euro := []byte("€\n")
	acc.Append(euro[:1])
	acc.Append(euro[1:])
	acc.Finish()

	snapshot := acc.Snapshot(false)
	if snapshot.Content != "€\n" {
		t.Fatalf("content = %q, want € newline", snapshot.Content)
	}
}

func TestOutputAccumulatorIgnoresAppendAfterFinish(t *testing.T) {
	acc := NewOutputAccumulator(OutputAccumulatorOptions{})
	acc.Append([]byte("before\n"))
	acc.Finish()
	acc.Append([]byte("after\n"))

	snapshot := acc.Snapshot(false)
	if snapshot.Content != "before\n" {
		t.Fatalf("content = %q, want %q", snapshot.Content, "before\n")
	}
}

func TestOutputAccumulatorPersistsFullOutput(t *testing.T) {
	acc := NewOutputAccumulator(OutputAccumulatorOptions{TempFilePrefix: "pi-test"})
	for i := 1; i <= 3000; i++ {
		acc.Append([]byte(strconv.Itoa(i) + "\n"))
	}
	acc.Finish()

	snapshot := acc.Snapshot(true)
	if snapshot.FullOutputPath == "" {
		t.Fatal("full output path is empty")
	}
	if err := acc.CloseTempFile(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(snapshot.FullOutputPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "1\n2\n3") || !strings.Contains(string(data), "2998\n2999\n3000") {
		t.Fatal("full output does not contain the beginning and end")
	}
}

func TestTruncateTailMatchesFilesResult(t *testing.T) {
	result := files.TruncateTail("a\nb\nc\n", files.Options{})
	if result.Content != "a\nb\nc\n" {
		t.Fatalf("content = %q, want unchanged", result.Content)
	}
}
