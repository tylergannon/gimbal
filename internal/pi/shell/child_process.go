package shell

import (
	"io"
	"os"
	"os/exec"
	"time"
	"unicode/utf8"
)

// bashExitStdioGrace is the idle window we keep reading merged stdout/stderr
// after the process exits. A detached descendant can hold the pipe open and
// keep writing past exit; the stream must not be destroyed on a fixed deadline
// measured from exit or its tail is silently lost (pi#5303). The timer is
// re-armed on every chunk, so an actively writing pipe keeps being read while a
// quiet held-open handle still releases after the grace elapses.
const bashExitStdioGrace = 100 * time.Millisecond

// utf8StreamDecoder is the Go stand-in for a non-fatal TextDecoder fed
// `{stream: true}` chunks. It holds an incomplete trailing UTF-8 sequence until
// the next chunk completes it; a genuinely invalid byte decodes to U+FFFD.
type utf8StreamDecoder struct {
	pending []byte
}

// Decode appends chunk and returns the decoded text up to the last complete
// rune. Bytes of an incomplete trailing sequence are held for the next call.
func (d *utf8StreamDecoder) Decode(chunk []byte) string {
	data := chunk
	if len(d.pending) > 0 {
		data = append(d.pending, chunk...)
		d.pending = nil
	}
	return d.consume(data, false)
}

// Flush returns any held bytes as replacement characters and clears the buffer,
// pi's `decoder.decode()` with no argument.
func (d *utf8StreamDecoder) Flush() string {
	if len(d.pending) == 0 {
		return ""
	}
	held := d.pending
	d.pending = nil
	return d.consume(held, true)
}

func (d *utf8StreamDecoder) consume(data []byte, flush bool) string {
	var out []byte
	for len(data) > 0 {
		r, size := utf8.DecodeRune(data)
		if r == utf8.RuneError && size == 1 {
			if !flush && !utf8.FullRune(data) {
				break
			}
			// Non-fatal decoding: an invalid byte becomes U+FFFD.
			out = utf8.AppendRune(out, utf8.RuneError)
			data = data[1:]
			continue
		}
		out = append(out, data[:size]...)
		data = data[size:]
	}
	if len(data) > 0 {
		d.pending = append([]byte(nil), data...)
	}
	return string(out)
}

// runBashCommand starts cmd with stdout and stderr merged onto a single pipe
// (so they interleave in write order, like pi's shared onData handler), streams
// the merged output into w, and waits for the process.
//
// After exit it drains the pipe on a re-arming idle grace rather than a fixed
// deadline, so output a detached descendant writes past exit is captured and a
// quiet held-open handle still releases after the grace (pi#5303). It returns
// the same error cmd.Wait returns: nil, or *exec.ExitError on a non-zero or
// signalled exit.
func runBashCommand(cmd *exec.Cmd, w io.Writer) error {
	pr, pw, err := os.Pipe()
	if err != nil {
		return err
	}
	// The same *os.File on both stdout and stderr keeps the child on one pipe.
	cmd.Stdout = pw
	cmd.Stderr = pw
	if err := cmd.Start(); err != nil {
		_ = pw.Close()
		_ = pr.Close()
		return err
	}
	// The parent must drop its write end or pr never sees EOF.
	_ = pw.Close()

	if cmd.Process != nil {
		TrackDetachedChildPID(cmd.Process.Pid)
		defer UntrackDetachedChildPID(cmd.Process.Pid)
	}

	// The reader feeds w and reports each chunk (and final EOF) on chunks.
	// Closing pr unblocks a read parked on a pipe a descendant still holds open.
	chunks, readDone := pipeReader(pr, w)

	waitErr := cmd.Wait()

	drainPipeAfterExit(pr, chunks, readDone, bashExitStdioGrace)
	return waitErr
}

// pipeReader copies pr into w on its own goroutine and signals each chunk (and
// final EOF) on the returned channel. The caller closes pr to unblock a read
// parked on a pipe a descendant still holds open.
func pipeReader(pr *os.File, w io.Writer) (chunks <-chan struct{}, done <-chan struct{}) {
	chunkCh := make(chan struct{}, 1)
	doneCh := make(chan struct{})
	go func() {
		defer close(doneCh)
		buf := make([]byte, 32*1024)
		for {
			n, readErr := pr.Read(buf)
			if n > 0 {
				_, _ = w.Write(buf[:n])
			}
			select {
			case chunkCh <- struct{}{}:
			default:
			}
			if readErr != nil {
				return
			}
		}
	}()
	return chunkCh, doneCh
}

// drainPipeAfterExit keeps reading after the process has exited. The idle-grace
// timer is re-armed on every chunk, so an actively writing descendant keeps
// being read while a quiet held-open handle still releases after the grace
// (pi#5303). The pipe is closed to stop tracking an inherited handle.
func drainPipeAfterExit(pr io.Closer, chunks <-chan struct{}, done <-chan struct{}, grace time.Duration) {
	timer := time.NewTimer(grace)
	defer timer.Stop()
drain:
	for {
		select {
		case <-done:
			break drain
		case <-chunks:
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timer.Reset(grace)
		case <-timer.C:
			break drain
		}
	}
	_ = pr.Close()
	<-done
}
