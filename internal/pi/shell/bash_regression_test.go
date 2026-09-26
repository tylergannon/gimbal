package shell

import (
	"context"
	"strings"
	"testing"
	"time"
)

// TestBashIgnoresLateOutputCallbacks is pi#5208: a custom operation that calls
// onData after it has resolved must not change the finished result.
func TestBashIgnoresLateOutputCallbacks(t *testing.T) {
	ops := BashOperations{Exec: func(_ context.Context, _ string, _ string, options BashExecOptions) (*int, error) {
		options.OnData([]byte("before\n"))
		go func() {
			time.Sleep(10 * time.Millisecond)
			options.OnData([]byte("late\n"))
		}()
		zero := 0
		return &zero, nil
	}}
	tool := CreateBashTool(t.TempDir(), &BashToolOptions{Operations: &ops})

	result, err := runTool(t, tool, map[string]any{"command": "late-output"})
	if err != nil {
		t.Fatal(err)
	}
	// Give the late callback time to fire, then confirm it was rejected.
	time.Sleep(30 * time.Millisecond)

	if got := strings.TrimSpace(resultText(result)); got != "before" {
		t.Fatalf("output = %q, want %q", got, "before")
	}
}

// TestBashReleasesPromptlyOnQuietHeldPipe is the second half of pi#5303: a
// detached sleeper inherits the stdout pipe and holds it open without writing,
// so EOF never arrives and release must come from the idle grace. The
// actively-writing half is covered deterministically by
// TestDrainPipeCapturesLateChunks, which re-arms the grace without depending on
// real subprocess scheduling.
func TestBashReleasesPromptlyOnQuietHeldPipe(t *testing.T) {
	command := `printf "DONE\n"; ( sleep 30 ) &`
	start := time.Now()
	result, err := runTool(t, CreateBashTool(t.TempDir(), nil), map[string]any{"command": command})
	elapsed := time.Since(start)
	if err != nil {
		t.Fatal(err)
	}
	if got := resultText(result); !strings.Contains(got, "DONE") {
		t.Fatalf("output = %q, want DONE", got)
	}
	if elapsed > 2*time.Second {
		t.Fatalf("release took %s, want under 2s via the idle grace", elapsed)
	}
}
