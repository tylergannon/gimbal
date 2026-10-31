package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tylergannon/gimbal/backend"
)

var artifactBytes = []byte{0, 1, 2, 128, 255, '\r', '\n'}

// This native child avoids depending on a Unix shell in Windows endpoint tests.
func TestWorkerChild(t *testing.T) {
	index := -1
	for i, arg := range os.Args {
		if arg == "gimbal-worker-test-child" {
			index = i
			break
		}
	}
	if index < 0 {
		return
	}
	args := os.Args[index+1:]
	switch args[0] {
	case "environment":
		_, _ = fmt.Fprint(os.Stdout, os.Getenv("GIMBAL_WORKER_CHILD_VALUE"))
	case "write":
		if err := os.WriteFile("report.bin", artifactBytes, 0600); err != nil {
			panic(err)
		}
		dir, err := os.Getwd()
		if err != nil {
			panic(err)
		}
		_, _ = fmt.Fprint(os.Stdout, dir)
		fmt.Fprint(os.Stderr, "diagnostic")
	case "read":
		raw, err := os.ReadFile("report.bin")
		if err != nil {
			panic(err)
		}
		_, _ = os.Stdout.Write(raw)
	case "fail":
		_, _ = fmt.Fprint(os.Stdout, "partial")
		os.Exit(7)
	case "wait":
		if err := os.WriteFile("ready", nil, 0600); err != nil {
			panic(err)
		}
		until := time.Now().Add(20 * time.Second)
		for {
			if _, err := os.Stat("release"); err == nil {
				break
			}
			if time.Now().After(until) {
				os.Exit(9)
			}
			time.Sleep(10 * time.Millisecond)
		}
		_, _ = fmt.Fprint(os.Stdout, "survived")
	}
	os.Exit(0)
}

type startupWriter struct{ ready chan string }

func (w startupWriter) Write(p []byte) (int, error) {
	w.ready <- string(p)
	return len(p), nil
}

func startWorker(t *testing.T, workspace, state string) (backend.HTTP, func()) {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	ready := make(chan string, 1)
	command := newWorkerCommand()
	command.SetContext(ctx)
	command.SetArgs([]string{"--listen", "127.0.0.1:0", "--workdir", workspace, "--executor-state-dir", state})
	command.SetOut(startupWriter{ready})
	command.SetErr(io.Discard)
	done := make(chan error, 1)
	go func() { done <- command.Execute() }()
	var message string
	select {
	case message = <-ready:
	case err := <-done:
		cancel()
		t.Fatalf("worker startup: %v", err)
	case <-time.After(10 * time.Second):
		cancel()
		t.Fatal("worker startup timed out")
	}
	var once sync.Once
	stop := func() {
		once.Do(func() {
			cancel()
			select {
			case err := <-done:
				if err != nil {
					t.Errorf("worker stopped: %v", err)
				}
			case <-time.After(5 * time.Second):
				t.Error("worker failed to stop")
			}
		})
	}
	t.Cleanup(stop)
	address := strings.Split(strings.TrimPrefix(message, "Task worker listening on "), ";")[0]
	client := backend.HTTP{Endpoint: "http://" + address}
	response, err := http.Get(client.Endpoint + "/health")
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatal(response.Status)
	}
	return client, stop
}

func childRequest(t *testing.T, name, action string) backend.Process {
	t.Helper()
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return backend.Process{
		ExecutionIdentity: backend.NewExecutionIdentity("process"),
		Owner:             backend.Owner{Run: "worker", Task: name},
		Command:           binary, Args: []string{"-test.run=^TestWorkerChild$", "--", "gimbal-worker-test-child", action},
		Workdir: filepath.Join(t.TempDir(), "coordinator-path-does-not-exist"),
	}
}

func waitResult(t *testing.T, client backend.HTTP, request backend.Process) backend.ProcessResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	for {
		result, err := client.Result(ctx, backend.Target{Run: request.Run, ID: request.Task})
		if err != nil {
			t.Fatal(err)
		}
		if result.Done {
			if result.Error != "" {
				t.Fatal(result.Error)
			}
			var status backend.ProcessResult
			if err := json.Unmarshal(result.Result, &status); err != nil {
				t.Fatal(err)
			}
			return status
		}
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-time.After(10 * time.Millisecond):
		}
	}
}

func readOutput(t *testing.T, client backend.HTTP, request backend.Process, name string) []byte {
	t.Helper()
	reader, err := client.ReadBytes(t.Context(), backend.Artifact{Owner: request.Owner, Name: name})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = reader.Close() }()
	raw, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestWorkerEndpointCommandsAndExplicitArtifactBytes(t *testing.T) {
	workspace := filepath.Join(t.TempDir(), "workspace with spaces")
	if err := os.Mkdir(workspace, 0700); err != nil {
		t.Fatal(err)
	}
	client, _ := startWorker(t, workspace, t.TempDir())
	request := childRequest(t, "write", "write")
	if err := client.Process(t.Context(), request); err != nil {
		t.Fatal(err)
	}
	if result := waitResult(t, client, request); result.ExitCode != 0 || result.Error != "" {
		t.Fatalf("success: %+v", result)
	}
	got, err := filepath.EvalSymlinks(string(readOutput(t, client, request, "stdout")))
	want, wantErr := filepath.EvalSymlinks(workspace)
	if err != nil || wantErr != nil || got != want {
		t.Fatalf("workdir = %q, want %q (%v, %v)", got, want, err, wantErr)
	}
	if got := string(readOutput(t, client, request, "stderr")); got != "diagnostic" {
		t.Fatal(got)
	}
	// Explicitly request a native file's bytes with another ordinary command.
	// The existing stdout artifact route carries binary data without a new
	// protocol field, shared filesystem, or implicit arbitrary-file access.
	artifact := childRequest(t, "read", "read")
	if err := client.Process(t.Context(), artifact); err != nil {
		t.Fatal(err)
	}
	if result := waitResult(t, client, artifact); result.ExitCode != 0 || result.Error != "" {
		t.Fatalf("artifact read: %+v", result)
	}
	if got := readOutput(t, client, artifact, "stdout"); !bytes.Equal(got, artifactBytes) {
		t.Fatalf("artifact bytes = %v", got)
	}
	failure := childRequest(t, "fail", "fail")
	if err := client.Process(t.Context(), failure); err != nil {
		t.Fatal(err)
	}
	if result := waitResult(t, client, failure); result.ExitCode != 7 || result.Error != "" {
		t.Fatalf("nonzero exit: %+v", result)
	}
	if got := string(readOutput(t, client, failure, "stdout")); got != "partial" {
		t.Fatal(got)
	}
	missing := childRequest(t, "missing", "write")
	missing.Command = filepath.Join(workspace, "does-not-exist")
	if err := client.Process(t.Context(), missing); err == nil {
		t.Fatal("missing executable falsely accepted")
	}
}

func TestWorkerRestartPreservesIndependentCommand(t *testing.T) {
	workspace, state := t.TempDir(), t.TempDir()
	client, stop := startWorker(t, workspace, state)
	request := childRequest(t, "restart", "wait")
	if err := client.Process(t.Context(), request); err != nil {
		t.Fatal(err)
	}
	until := time.Now().Add(10 * time.Second)
	for {
		if _, err := os.Stat(filepath.Join(workspace, "ready")); err == nil {
			break
		}
		if time.Now().After(until) {
			t.Fatal("command did not start")
		}
		time.Sleep(10 * time.Millisecond)
	}
	stop()
	client, _ = startWorker(t, workspace, state)
	if err := os.WriteFile(filepath.Join(workspace, "release"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	if result := waitResult(t, client, request); result.ExitCode != 0 || result.Error != "" {
		t.Fatalf("restart: %+v", result)
	}
	if got := string(readOutput(t, client, request, "stdout")); got != "survived" {
		t.Fatal(got)
	}
}

func TestWorkerRelativeExecutable(t *testing.T) {
	workspace := t.TempDir()
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(binary)
	if err != nil {
		t.Fatal(err)
	}
	name := "helper"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	if err := os.WriteFile(filepath.Join(workspace, name), raw, 0700); err != nil {
		t.Fatal(err)
	}
	client, _ := startWorker(t, workspace, t.TempDir())
	request := childRequest(t, "relative", "fail")
	// Both slash forms are legal Windows paths. Forward slash also works on Unix.
	request.Command = "./" + name
	if err := client.Process(t.Context(), request); err != nil {
		t.Fatal(err)
	}
	if result := waitResult(t, client, request); result.ExitCode != 7 || result.Error != "" {
		t.Fatalf("relative executable: %+v", result)
	}
}

func TestWorkerHelpAndConfigurationErrors(t *testing.T) {
	var help bytes.Buffer
	command := newWorkerCommand()
	command.SetOut(&help)
	command.SetArgs([]string{"--help"})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{"--workdir", "--executor-state-dir", "--listen", "foreground", "SCM", "authentication", "--service", "--service-name", "--env-file", "DIFFUSION_API_KEY", "HKCU\\Environment", "Log on as a service"} {
		if !strings.Contains(help.String(), text) {
			t.Errorf("help missing %q", text)
		}
	}
	command = newWorkerCommand()
	command.SetErr(io.Discard)
	command.SetOut(io.Discard)
	command.SetArgs([]string{"--workdir", filepath.Join(t.TempDir(), "missing")})
	if err := command.Execute(); err == nil {
		t.Fatal("missing workspace accepted")
	}
}
