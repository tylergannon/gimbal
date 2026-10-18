package commandexecutor

import (
	"context"
	"github.com/tylergannon/gimbal/internal/executor"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestConcurrentSubmitExecutesOnceAndRetainsCompletedOutput(t *testing.T) {
	root := t.TempDir()
	marker := filepath.Join(root, "executions")
	request := Request{ID: executor.NewCommandID(), Command: "sh", Args: []string{"-c", `printf run >> "$1"; sleep .15; printf out; printf err >&2; exit 7`, "sh", marker}, Workdir: root, Environment: os.Environ(), Timeout: 5 * time.Second}
	var wg sync.WaitGroup
	descriptors := make([]executor.Descriptor, 8)
	failures := make([]error, 8)
	for i := range descriptors {
		wg.Go(func() { descriptors[i], failures[i] = Submit(root, request) })
	}
	wg.Wait()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for i, d := range descriptors {
		if failures[i] != nil {
			t.Fatal(failures[i])
		}
		if d.Result != descriptors[0].Result {
			t.Fatal("different operation")
		}
		result, err := Wait(ctx, d)
		if err != nil || result.ExitCode != 7 || result.Error != "" {
			t.Fatalf("%+v %v", result, err)
		}
	}
	data, err := os.ReadFile(marker)
	if err != nil || string(data) != "run" {
		t.Fatalf("executions=%q %v", data, err)
	}
	request.Args = []string{"-c", "echo conflict"}
	if _, err = Submit(root, request); err == nil {
		t.Fatal("conflicting inputs accepted")
	}
	d := descriptors[0]
	out, _ := os.ReadFile(d.Stdout)
	stderr, _ := os.ReadFile(d.Stderr)
	if string(out) != "out" || string(stderr) != "err" {
		t.Fatalf("%q %q", out, stderr)
	}
	// Wait until the owner lease is gone, then read the completed files alone.
	for {
		lease, err := executor.Lock(filepath.Dir(d.Result))
		if err != nil {
			t.Fatal(err)
		}
		if lease != nil {
			lease.Close()
			break
		}
		time.Sleep(time.Millisecond)
	}
	executor.AssertOwner(filepath.Dir(d.Result))
	if d = executor.Describe(filepath.Dir(d.Result), false); d.Socket != nil || d.State != "completed" {
		t.Fatal(d)
	}
}
func TestDeadlineAndCancellationStopProcessGroup(t *testing.T) {
	for _, timeout := range []bool{false, true} {
		t.Run(map[bool]string{true: "deadline", false: "cancel"}[timeout], func(t *testing.T) {
			root := t.TempDir()
			marker := filepath.Join(root, "escaped")
			request := Request{ID: executor.NewCommandID(), Command: "sh", Args: []string{"-c", `(sleep .4; touch "$1") & wait`, "sh", marker}, Workdir: root, Environment: os.Environ(), Timeout: 100 * time.Millisecond}
			if !timeout {
				request.Timeout = 5 * time.Second
			}
			d, err := Submit(root, request)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			if !timeout {
				time.Sleep(50 * time.Millisecond)
				if err = Cancel(ctx, d); err != nil {
					t.Fatal(err)
				}
			}
			result, err := Wait(ctx, d)
			if err != nil || result.TimedOut != timeout || result.Cancelled == timeout {
				t.Fatalf("%+v %v", result, err)
			}
			time.Sleep(450 * time.Millisecond)
			if executor.Exists(marker) {
				t.Fatal("descendant survived cancellation")
			}
		})
	}
}
func TestLargeOutputIsFileBacked(t *testing.T) {
	root := t.TempDir()
	d, err := Submit(root, Request{ID: executor.NewCommandID(), Command: "sh", Args: []string{"-c", `dd if=/dev/zero bs=1048576 count=16 2>/dev/null; printf end`}, Workdir: root, Environment: os.Environ(), Timeout: 5 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	result, err := Wait(context.Background(), d)
	if err != nil || result.Error != "" {
		t.Fatalf("%+v %v", result, err)
	}
	info, err := os.Stat(d.Stdout)
	if err != nil || info.Size() != 16*1048576+3 {
		t.Fatalf("%v %v", info, err)
	}
	raw, _ := os.ReadFile(d.Result)
	if strings.Contains(string(raw), "stdout") {
		t.Fatal("terminal result embeds output")
	}
}
