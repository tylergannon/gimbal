package shell

import (
	"context"
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tylergannon/gimbal/internal/pi/files"
)

// BashExecutorOptions are the per-execution controls for
// ExecuteBashWithOperations.
type BashExecutorOptions struct {
	// OnChunk receives each sanitized output chunk as it arrives.
	OnChunk func(chunk string)
}

// BashResult is the outcome of ExecuteBashWithOperations.
type BashResult struct {
	// Output is the combined, sanitized stdout+stderr, possibly truncated.
	Output string
	// ExitCode is the process exit code, nil when the command was cancelled.
	ExitCode *int
	// Cancelled reports whether the context ended the command.
	Cancelled bool
	// Truncated reports whether Output was shortened.
	Truncated bool
	// FullOutputPath is the temp file holding the full output when truncated.
	FullOutputPath string
}

// ExecuteBashWithOperations runs a bash command through custom BashOperations,
// sanitizing output and truncating it to pi's tail limits. It is the session's
// own execution path (bash-executor.ts), separate from the bash tool.
func ExecuteBashWithOperations(ctx context.Context, command, cwd string, operations BashOperations, options *BashExecutorOptions) (BashResult, error) {
	var outputChunks []string
	outputBytes := 0
	maxOutputBytes := files.DefaultMaxBytes * 2

	var tempFilePath string
	var tempFile *os.File
	totalBytes := 0

	ensureTempFile := func() {
		if tempFilePath != "" {
			return
		}
		var suffix [8]byte
		_, _ = rand.Read(suffix[:])
		tempFilePath = filepath.Join(os.TempDir(), fmt.Sprintf("pi-bash-%x.log", suffix))
		file, err := os.Create(tempFilePath)
		if err != nil {
			tempFilePath = ""
			return
		}
		tempFile = file
		for _, chunk := range outputChunks {
			_, _ = tempFile.WriteString(chunk)
		}
	}

	decoder := &utf8StreamDecoder{}
	onData := func(data []byte) {
		totalBytes += len(data)
		// Sanitize: strip ANSI, replace binary garbage, drop carriage returns.
		text := strings.ReplaceAll(SanitizeBinaryOutput(StripAnsi(decoder.Decode(data))), "\r", "")
		if totalBytes > files.DefaultMaxBytes {
			ensureTempFile()
		}
		if tempFile != nil {
			_, _ = tempFile.WriteString(text)
		}
		outputChunks = append(outputChunks, text)
		outputBytes += len(text)
		for outputBytes > maxOutputBytes && len(outputChunks) > 1 {
			outputBytes -= len(outputChunks[0])
			outputChunks = outputChunks[1:]
		}
		if options != nil && options.OnChunk != nil {
			options.OnChunk(text)
		}
	}

	finish := func(cancelled bool) BashResult {
		fullOutput := strings.Join(outputChunks, "")
		truncation := files.TruncateTail(fullOutput, files.Options{})
		if truncation.Truncated {
			ensureTempFile()
		}
		if tempFile != nil {
			_ = tempFile.Close()
			tempFile = nil
		}
		output := fullOutput
		if truncation.Truncated {
			output = truncation.Content
		}
		return BashResult{
			Output:         output,
			Cancelled:      cancelled,
			Truncated:      truncation.Truncated,
			FullOutputPath: tempFilePath,
		}
	}

	result, err := operations.Exec(ctx, command, cwd, BashExecOptions{
		OnData: onData,
		Env:    GetShellEnv(),
	})
	if err != nil {
		if ctx.Err() != nil {
			return finish(true), nil
		}
		if tempFile != nil {
			_ = tempFile.Close()
			tempFile = nil
		}
		return BashResult{}, err
	}

	cancelled := ctx.Err() != nil
	out := finish(cancelled)
	if !cancelled {
		out.ExitCode = result
	}
	return out, nil
}
