package shell

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"

	"github.com/tylergannon/gimbal/internal/pi/files"
)

// OutputAccumulatorOptions mirrors pi's OutputAccumulatorOptions. A zero
// MaxLines or MaxBytes means pi's default; an empty TempFilePrefix means
// "pi-output".
type OutputAccumulatorOptions struct {
	MaxLines       int
	MaxBytes       int
	TempFilePrefix string
}

// OutputSnapshot is the display form of the accumulated output, pi's
// OutputSnapshot.
type OutputSnapshot struct {
	Content        string
	Truncation     files.Result
	FullOutputPath string
}

// OutputAccumulator incrementally tracks streaming output with bounded memory:
// it keeps only a decoded rolling tail for display snapshots and opens a temp
// file holding the raw bytes once the full output outgrows the limits. It is a
// port of output-accumulator.ts.
type OutputAccumulator struct {
	maxLines        int
	maxBytes        int
	maxRollingBytes int
	tempFilePrefix  string

	decoder               utf8StreamDecoder
	rawChunks             [][]byte
	tail                  []byte
	tailBytes             int
	tailStartsAtLineBound bool
	totalRawBytes         int
	totalDecodedBytes     int
	completedLines        int
	totalLines            int
	currentLineBytes      int
	hasOpenLine           bool
	finished              bool

	tempFilePath string
	tempFile     *os.File
}

// NewOutputAccumulator returns an accumulator with pi's default limits applied.
func NewOutputAccumulator(options OutputAccumulatorOptions) *OutputAccumulator {
	maxLines := options.MaxLines
	if maxLines == 0 {
		maxLines = files.DefaultMaxLines
	}
	maxBytes := options.MaxBytes
	if maxBytes == 0 {
		maxBytes = files.DefaultMaxBytes
	}
	rolling := max(maxBytes*2, 1)
	prefix := options.TempFilePrefix
	if prefix == "" {
		prefix = "pi-output"
	}
	return &OutputAccumulator{
		maxLines:              maxLines,
		maxBytes:              maxBytes,
		maxRollingBytes:       rolling,
		tempFilePrefix:        prefix,
		tailStartsAtLineBound: true,
	}
}

// Append decodes a raw chunk for display and captures it for the temp file. A
// late append after Finish is ignored, pi's "Cannot append to a finished output
// accumulator" guard rendered as a no-op.
func (a *OutputAccumulator) Append(data []byte) {
	if a.finished || len(data) == 0 {
		return
	}
	a.totalRawBytes += len(data)
	a.appendDecodedText(a.decoder.Decode(data))

	if a.tempFile != nil || a.shouldUseTempFile() {
		a.ensureTempFile()
		if a.tempFile != nil {
			_, _ = a.tempFile.Write(data)
		}
	} else {
		// Copy: the caller (os/exec pipe reader) reuses its buffer.
		a.rawChunks = append(a.rawChunks, append([]byte(nil), data...))
	}
}

// Finish flushes the streaming decoder and ensures the temp file exists when
// the output has outgrown the limits. It is idempotent.
func (a *OutputAccumulator) Finish() {
	if a.finished {
		return
	}
	a.finished = true
	a.appendDecodedText(a.decoder.Flush())
	if a.shouldUseTempFile() {
		a.ensureTempFile()
	}
}

// Snapshot renders the current tail with truncation applied. When
// persistIfTruncated is set the temp file is opened as soon as truncation is
// known, even if no raw chunk has crossed the threshold yet.
func (a *OutputAccumulator) Snapshot(persistIfTruncated bool) OutputSnapshot {
	tail := files.TruncateTail(a.snapshotText(), files.Options{
		MaxLines: &a.maxLines,
		MaxBytes: &a.maxBytes,
	})
	truncated := a.totalLines > a.maxLines || a.totalDecodedBytes > a.maxBytes
	if truncated && tail.TruncatedBy == nil {
		if a.totalDecodedBytes > a.maxBytes {
			tail.TruncatedBy = new("bytes")
		} else {
			tail.TruncatedBy = new("lines")
		}
	}
	tail.Truncated = truncated
	tail.TotalLines = a.totalLines
	tail.TotalBytes = a.totalDecodedBytes
	tail.MaxLines = a.maxLines
	tail.MaxBytes = a.maxBytes

	if persistIfTruncated && truncated {
		a.ensureTempFile()
	}
	return OutputSnapshot{
		Content:        tail.Content,
		Truncation:     tail,
		FullOutputPath: a.tempFilePath,
	}
}

// CloseTempFile flushes and closes the temp file, if one was opened.
func (a *OutputAccumulator) CloseTempFile() error {
	if a.tempFile == nil {
		return nil
	}
	file := a.tempFile
	a.tempFile = nil
	return file.Close()
}

// LastLineBytes is the decoded byte length of the currently open last line,
// used to report a partially truncated line.
func (a *OutputAccumulator) LastLineBytes() int { return a.currentLineBytes }

func (a *OutputAccumulator) appendDecodedText(text string) {
	if text == "" {
		return
	}
	a.totalDecodedBytes += len(text)
	a.tail = append(a.tail, text...)
	a.tailBytes += len(text)
	if a.tailBytes > a.maxRollingBytes*2 {
		a.trimTail()
	}

	newlines := bytes.Count([]byte(text), []byte{'\n'})
	if newlines == 0 {
		a.currentLineBytes += len(text)
		a.hasOpenLine = true
	} else {
		a.completedLines += newlines
		lastNewline := bytes.LastIndexByte([]byte(text), '\n')
		tailLen := len(text) - lastNewline - 1
		a.currentLineBytes = tailLen
		a.hasOpenLine = tailLen > 0
	}
	a.totalLines = a.completedLines
	if a.hasOpenLine {
		a.totalLines++
	}
}

func (a *OutputAccumulator) trimTail() {
	if len(a.tail) <= a.maxRollingBytes {
		a.tailBytes = len(a.tail)
		return
	}
	start := len(a.tail) - a.maxRollingBytes
	for start < len(a.tail) && a.tail[start]&0xc0 == 0x80 {
		start++
	}
	if start > 0 {
		a.tailStartsAtLineBound = a.tail[start-1] == '\n'
	}
	a.tail = append([]byte(nil), a.tail[start:]...)
	a.tailBytes = len(a.tail)
}

func (a *OutputAccumulator) snapshotText() string {
	if a.tailStartsAtLineBound {
		return string(a.tail)
	}
	if firstNewline := bytes.IndexByte(a.tail, '\n'); firstNewline != -1 {
		return string(a.tail[firstNewline+1:])
	}
	return string(a.tail)
}

func (a *OutputAccumulator) shouldUseTempFile() bool {
	return a.totalRawBytes > a.maxBytes || a.totalDecodedBytes > a.maxBytes || a.totalLines > a.maxLines
}

func (a *OutputAccumulator) ensureTempFile() {
	if a.tempFilePath != "" {
		return
	}
	var suffix [8]byte
	_, _ = rand.Read(suffix[:])
	a.tempFilePath = filepath.Join(os.TempDir(), fmt.Sprintf("%s-%x.log", a.tempFilePrefix, suffix))
	file, err := os.Create(a.tempFilePath)
	if err != nil {
		a.tempFilePath = ""
		return
	}
	a.tempFile = file
	for _, chunk := range a.rawChunks {
		_, _ = file.Write(chunk)
	}
	a.rawChunks = nil
}
