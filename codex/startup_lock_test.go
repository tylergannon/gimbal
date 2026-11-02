package codex

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// The child holds a real OS lock until its parent closes stdin or kills it.
func TestStartupLockProcess(t *testing.T) {
	if os.Getenv("GIMBAL_TEST_STARTUP_LOCK_CHILD") != "1" {
		return
	}
	unlock, err := lockStartup(context.Background(), os.Getenv("GIMBAL_TEST_STARTUP_SOCKET"))
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	fmt.Println("locked")
	_, _ = io.Copy(io.Discard, os.Stdin)
}

func TestStartupLockSerializesProcessesAndReleasesAfterExit(t *testing.T) {
	socket := filepath.Join(t.TempDir(), "daemon.sock")
	type child struct {
		cmd   *exec.Cmd
		input io.WriteCloser
		ready <-chan string
	}
	start := func() child {
		t.Helper()
		cmd := exec.Command(os.Args[0], "-test.run=^TestStartupLockProcess$")
		cmd.Env = append(os.Environ(), "GIMBAL_TEST_STARTUP_LOCK_CHILD=1", "GIMBAL_TEST_STARTUP_SOCKET="+socket)
		input, err := cmd.StdinPipe()
		if err != nil {
			t.Fatal(err)
		}
		output, err := cmd.StdoutPipe()
		if err != nil {
			t.Fatal(err)
		}
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
		ready := make(chan string, 1)
		go func() { line, _ := bufio.NewReader(output).ReadString('\n'); ready <- line }()
		return child{cmd, input, ready}
	}
	waitReady := func(c child) {
		t.Helper()
		select {
		case line := <-c.ready:
			if line != "locked\n" {
				t.Fatalf("child output %q", line)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("child did not acquire lock")
		}
	}
	first := start()
	waitReady(first)
	second := start()
	select {
	case line := <-second.ready:
		t.Fatalf("second child entered held critical section: %q", line)
	case <-time.After(200 * time.Millisecond):
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if unlock, err := lockStartup(ctx, socket); !errors.Is(err, context.DeadlineExceeded) {
		if unlock != nil {
			unlock()
		}
		t.Fatalf("waiting lock error = %v", err)
	}
	if err := first.cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	if err := first.cmd.Wait(); err == nil {
		t.Fatal("killed child exited successfully")
	}
	waitReady(second)
	if err := second.input.Close(); err != nil {
		t.Fatal(err)
	}
	if err := second.cmd.Wait(); err != nil {
		t.Fatal(err)
	}
	ctx2, cancel2 := context.WithTimeout(context.Background(), time.Second)
	defer cancel2()
	unlock, err := lockStartup(ctx2, socket)
	if err != nil {
		t.Fatal(err)
	}
	unlock()
}
