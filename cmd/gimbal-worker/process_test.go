package main

import (
	"bufio"
	"bytes"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tylergannon/gimbal/backend"
)

// Re-execute the real foreground command in a separate process. Executor owner
// dispatch happens in package init, exactly as it does in the worker binary.
func TestWorkerHostProcess(t *testing.T) {
	for i, arg := range os.Args {
		if arg == "gimbal-worker-host-process" {
			command := newWorkerCommand()
			command.SetArgs(os.Args[i+1:])
			if err := command.Execute(); err != nil {
				os.Exit(2)
			}
			os.Exit(0)
		}
	}
}

func startWorkerProcess(t *testing.T, workspace, state, envFile string) (backend.HTTP, func()) {
	t.Helper()
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	process := exec.Command(binary, "-test.run=^TestWorkerHostProcess$", "--", "gimbal-worker-host-process", "--listen", "127.0.0.1:0", "--workdir", workspace, "--executor-state-dir", state, "--env-file", envFile)
	var diagnostic bytes.Buffer
	process.Stderr = &diagnostic
	pipe, err := process.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := process.Start(); err != nil {
		t.Fatal(err)
	}
	var once sync.Once
	stop := func() {
		once.Do(func() {
			_ = process.Process.Kill()
			_ = process.Wait()
		})
	}
	t.Cleanup(stop)
	message := make(chan string, 1)
	go func() {
		scanner := bufio.NewScanner(pipe)
		if scanner.Scan() {
			message <- scanner.Text()
		} else {
			message <- ""
		}
	}()
	select {
	case line := <-message:
		if !strings.HasPrefix(line, "Task worker listening on ") {
			stop()
			t.Fatalf("worker did not start: %s", diagnostic.String())
		}
		address := strings.Split(strings.TrimPrefix(line, "Task worker listening on "), ";")[0]
		client := backend.HTTP{Endpoint: "http://" + address}
		response, err := http.Get(client.Endpoint + "/health")
		if err != nil {
			t.Fatal(err)
		}
		_ = response.Body.Close()
		if response.StatusCode != 200 {
			t.Fatal(response.Status)
		}
		return client, stop
	case <-time.After(10 * time.Second):
		stop()
		t.Fatal("foreground child startup timed out")
		return backend.HTTP{}, stop
	}
}

func TestWorkerProcessRestartAndEnvironment(t *testing.T) {
	workspace := filepath.Join(t.TempDir(), "workspace with spaces")
	if err := os.Mkdir(workspace, 0700); err != nil {
		t.Fatal(err)
	}
	state := t.TempDir()
	envFile := environmentFile(t, `{"set":{"GIMBAL_WORKER_CHILD_VALUE":"synthetic-environment-value"}}`)
	client, stop := startWorkerProcess(t, workspace, state, envFile)
	// The independent native command inherits the explicit environment file.
	environment := childRequest(t, "environment", "environment")
	if err := client.Process(t.Context(), environment); err != nil {
		t.Fatal(err)
	}
	if result := waitResult(t, client, environment); result.ExitCode != 0 {
		t.Fatal(result)
	}
	if got := string(readOutput(t, client, environment, "stdout")); got != "synthetic-environment-value" {
		t.Fatal("command did not inherit explicit environment")
	}
	completed := childRequest(t, "completed", "fail")
	if err := client.Process(t.Context(), completed); err != nil {
		t.Fatal(err)
	}
	if result := waitResult(t, client, completed); result.ExitCode != 7 {
		t.Fatal(result)
	}
	running := childRequest(t, "running", "wait")
	if err := client.Process(t.Context(), running); err != nil {
		t.Fatal(err)
	}
	until := time.Now().Add(10 * time.Second)
	for {
		if _, err := os.Stat(filepath.Join(workspace, "ready")); err == nil {
			break
		}
		if time.Now().After(until) {
			t.Fatal("independent command did not start")
		}
		time.Sleep(10 * time.Millisecond)
	}
	stop()
	client, _ = startWorkerProcess(t, workspace, state, envFile)
	result, err := client.Result(t.Context(), backend.Target{Run: running.Run, ID: running.Task})
	if err != nil || result.Done {
		t.Fatalf("running owner was not restored: %+v %v", result, err)
	}
	// An identical submission reuses the existing owner, including after restart.
	if err := client.Process(t.Context(), running); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "release"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	if result := waitResult(t, client, running); result.ExitCode != 0 {
		t.Fatal(result)
	}
	if got := string(readOutput(t, client, running, "stdout")); got != "survived" {
		t.Fatalf("owner output=%q", got)
	}
	if result := waitResult(t, client, completed); result.ExitCode != 7 {
		t.Fatal(result)
	}
	if got := string(readOutput(t, client, completed, "stdout")); got != "partial" {
		t.Fatal(got)
	}
	// Fetch explicit binary output through the same foreground protocol after restart.
	write := childRequest(t, "write-after-restart", "write")
	if err := client.Process(t.Context(), write); err != nil {
		t.Fatal(err)
	}
	if result := waitResult(t, client, write); result.ExitCode != 0 {
		t.Fatal(result)
	}
	read := childRequest(t, "artifact-after-restart", "read")
	if err := client.Process(t.Context(), read); err != nil {
		t.Fatal(err)
	}
	if result := waitResult(t, client, read); result.ExitCode != 0 {
		t.Fatal(result)
	}
	if got := readOutput(t, client, read, "stdout"); !bytes.Equal(got, artifactBytes) {
		t.Fatal("binary artifact changed")
	}
}
