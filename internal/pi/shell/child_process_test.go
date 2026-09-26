package shell

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"
)

// TestDrainPipeCapturesLateChunks exercises the pi#5303 fix directly: after the
// process has exited, chunks that keep arriving within the idle grace re-arm it,
// so a descendant that writes past exit is still captured.
func TestDrainPipeCapturesLateChunks(t *testing.T) {
	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	chunks, done := pipeReader(pr, &output)
	go func() {
		_, _ = pw.Write([]byte("HEAD\n"))
		for i := 1; i <= 6; i++ {
			time.Sleep(30 * time.Millisecond)
			_, _ = pw.Write([]byte("TICK" + itoaByte(i) + "\n"))
		}
	}()
	drainPipeAfterExit(pr, chunks, done, 100*time.Millisecond)
	_ = pw.Close()
	if !strings.Contains(output.String(), "TICK6") {
		t.Fatalf("output = %q, want the late TICK6", output.String())
	}
}

// TestDrainPipeReleasesQuietPipe exercises the other half of pi#5303: a pipe
// that is held open but produces no further output releases after the grace.
func TestDrainPipeReleasesQuietPipe(t *testing.T) {
	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	chunks, done := pipeReader(pr, &output)
	go func() {
		_, _ = pw.Write([]byte("DONE\n"))
		time.Sleep(time.Second)
	}()
	start := time.Now()
	drainPipeAfterExit(pr, chunks, done, 50*time.Millisecond)
	elapsed := time.Since(start)
	_ = pw.Close()
	if !strings.Contains(output.String(), "DONE") {
		t.Fatalf("output = %q, want DONE", output.String())
	}
	if elapsed > 500*time.Millisecond {
		t.Fatalf("drain took %s, want a prompt release", elapsed)
	}
}

func itoaByte(n int) string {
	return string(rune('0' + n))
}
