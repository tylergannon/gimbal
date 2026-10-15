package shell

import (
	"context"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/tylergannon/gimbal/internal/pi/model"
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

// An AfterFunc callback can already be waiting for the mutex when Stop returns
// false. A newer leading edge may then replace its timer before it gets the lock.
func TestBashIgnoresReplacedTimerCallback(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		updates := []string{}
		u := newBashUpdater(func(r model.AgentToolResult) { updates = append(updates, resultText(r)) }, NewOutputAccumulator(OutputAccumulatorOptions{}))
		read := func() ([]string, *time.Timer, uint64) {
			u.mu.Lock()
			defer u.mu.Unlock()
			return append([]string(nil), updates...), u.timer, u.timerGeneration
		}
		u.write([]byte("first\n"))
		u.write([]byte("second\n"))
		_, timer, stale := read()
		if timer == nil {
			t.Fatal("trailing update was not scheduled")
		}
		// Reproduce the ordering after a newer leading edge wins the mutex. Keep
		// the old callback to deliver after the following write owns another timer.
		u.mu.Lock()
		u.clearTimerLocked()
		u.emitLocked()
		u.mu.Unlock()
		u.write([]byte("third\n"))
		_, current, _ := read()
		u.flushTimer(stale)
		got, pending, _ := read()
		if len(got) != 2 || pending != current {
			t.Fatalf("stale callback emitted output or discarded the current timer: updates=%d", len(got))
		}
		time.Sleep(bashUpdateThrottle)
		synctest.Wait()
		got, _, _ = read()
		if len(got) != 3 || got[2] != "first\nsecond\nthird\n" {
			t.Fatalf("current timer did not preserve trailing output: %#v", got)
		}
		u.finish()
	})
}

func TestBashThrottlePreservesLeadingPeriodicAndFinalOutput(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		updates := []string{}
		u := newBashUpdater(func(r model.AgentToolResult) { updates = append(updates, resultText(r)) }, NewOutputAccumulator(OutputAccumulatorOptions{}))
		read := func() []string { u.mu.Lock(); defer u.mu.Unlock(); return append([]string(nil), updates...) }
		u.write([]byte("first\n"))
		if len(read()) != 1 {
			t.Fatal("leading output was delayed")
		}
		u.write([]byte("second\n"))
		time.Sleep(bashUpdateThrottle / 2)
		u.write([]byte("third\n"))
		if len(read()) != 1 {
			t.Fatal("output emitted within throttle interval")
		}
		time.Sleep(bashUpdateThrottle / 2)
		synctest.Wait()
		got := read()
		if len(got) != 2 || !strings.Contains(got[1], "third") {
			t.Fatalf("periodic trailing update missing: %#v", got)
		}
		u.write([]byte("last\n"))
		snapshot := u.finish()
		got = read()
		if len(got) != 3 || !strings.Contains(got[2], "last") || !strings.Contains(snapshot.Content, "last") {
			t.Fatalf("finish lost buffered output: %#v, %+v", got, snapshot)
		}
		time.Sleep(bashUpdateThrottle)
		synctest.Wait()
		u.write([]byte("late\n"))
		if len(read()) != 3 {
			t.Fatal("finished tool emitted a late update")
		}
	})
}
