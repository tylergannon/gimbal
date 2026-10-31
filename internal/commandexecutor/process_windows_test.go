package commandexecutor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tylergannon/gimbal/internal/executor"
)

func TestWindowsProcessChild(t *testing.T) {
	index := -1
	for i, arg := range os.Args {
		if arg == "windows-process-child" {
			index = i
			break
		}
	}
	if index < 0 {
		return
	}
	args := os.Args[index+1:]
	switch args[0] {
	case "descendant":
		time.Sleep(700 * time.Millisecond)
		if err := os.WriteFile(args[1], []byte("escaped"), 0600); err != nil {
			panic(err)
		}
	case "spawn":
		binary, err := os.Executable()
		if err != nil {
			panic(err)
		}
		child := exec.Command(binary, "-test.run=^TestWindowsProcessChild$", "--", "windows-process-child", "descendant", args[1])
		if err := child.Start(); err != nil {
			panic(err)
		}
	case "wait":
		fmt.Fprint(os.Stdout, "ready")
		time.Sleep(30 * time.Second)
	}
	os.Exit(0)
}

func windowsChildRequest(t *testing.T, root, action string, args ...string) Request {
	t.Helper()
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return Request{ID: executor.NewCommandID(), Command: binary, Args: append([]string{"-test.run=^TestWindowsProcessChild$", "--", "windows-process-child", action}, args...), Workdir: root, Environment: os.Environ(), Timeout: 10 * time.Second}
}

func TestWindowsJobClosesOrdinaryDescendants(t *testing.T) {
	root := t.TempDir()
	marker := filepath.Join(root, "escaped")
	d, err := Submit(root, windowsChildRequest(t, root, "spawn", marker))
	if err != nil {
		t.Fatal(err)
	}
	result, err := Wait(t.Context(), d)
	if err != nil || result.ExitCode != 0 || result.Error != "" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	time.Sleep(time.Second)
	if executor.Exists(marker) {
		t.Fatal("ordinary descendant survived job close")
	}
}

func TestWindowsCancellationAndDeadlineUseForcedTermination(t *testing.T) {
	for _, timeout := range []bool{false, true} {
		t.Run(fmt.Sprint("deadline=", timeout), func(t *testing.T) {
			root := t.TempDir()
			request := windowsChildRequest(t, root, "wait")
			if timeout {
				request.Timeout = 2 * time.Second
			}
			d, err := Submit(root, request)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()
			until := time.Now().Add(5 * time.Second)
			for {
				raw, _ := os.ReadFile(d.Stdout)
				if string(raw) == "ready" {
					break
				}
				if time.Now().After(until) {
					t.Fatal("native child did not start")
				}
				time.Sleep(10 * time.Millisecond)
			}
			if !timeout {
				if err := Cancel(ctx, d); err != nil {
					t.Fatal(err)
				}
			}
			result, err := Wait(ctx, d)
			if err != nil || result.TimedOut != timeout || result.Cancelled == timeout || !result.Forced || !strings.Contains(result.Error, "forcibly") {
				t.Fatalf("result=%+v err=%v", result, err)
			}
		})
	}
}
