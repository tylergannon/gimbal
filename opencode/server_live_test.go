package opencode

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

type helperResult struct {
	Info      ServerInfo `json:"info"`
	Directory string     `json:"directory"`
}

func TestOpenCodeProcessHelper(t *testing.T) {
	if os.Getenv("GIMBLE_OPENCODE_TEST_HELPER") != "1" {
		return
	}
	args := os.Args
	for len(args) > 0 && args[0] != "--" {
		args = args[1:]
	}
	if len(args) != 3 {
		t.Fatalf("helper args = %q", os.Args)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	info, err := StartServer(ctx, args[1])
	if err != nil {
		t.Fatal(err)
	}
	client, err := Connect(ctx, args[2], args[1])
	if err != nil {
		t.Fatal(err)
	}
	session, err := client.CreateSession(ctx, SessionCreateInput{})
	if err != nil {
		t.Fatal(err)
	}
	if err := json.NewEncoder(os.Stdout).Encode(helperResult{Info: info, Directory: session.Directory}); err != nil {
		t.Fatal(err)
	}
}

func TestLiveSharedServerLifecycle(t *testing.T) {
	if os.Getenv("GIMBLE_OPENCODE_LIVE") != "1" {
		t.Skip("set GIMBLE_OPENCODE_LIVE=1 to exercise the installed OpenCode server")
	}
	if _, err := exec.LookPath("opencode"); err != nil {
		t.Skip("opencode is not installed")
	}
	stateDir := filepath.Join(t.TempDir(), "state")
	workdirs := []string{filepath.Join(t.TempDir(), "one"), filepath.Join(t.TempDir(), "two")}
	for _, dir := range workdirs {
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
		defer cancel()
		_, _ = StopServer(ctx, stateDir)
	}()

	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	commands := make([]*exec.Cmd, len(workdirs))
	outputs := make([]bytes.Buffer, len(workdirs))
	errorsOutput := make([]bytes.Buffer, len(workdirs))
	for index, workdir := range workdirs {
		commands[index] = exec.Command(executable, "-test.run=^TestOpenCodeProcessHelper$", "--", stateDir, workdir)
		commands[index].Dir = workdir
		commands[index].Env = append(os.Environ(), "GIMBLE_OPENCODE_TEST_HELPER=1")
		commands[index].Stdout = &outputs[index]
		commands[index].Stderr = &errorsOutput[index]
		if err := commands[index].Start(); err != nil {
			t.Fatal(err)
		}
	}
	results := make([]helperResult, len(commands))
	for index, command := range commands {
		if err := command.Wait(); err != nil {
			t.Fatalf("caller %d: %v: %s", index, err, errorsOutput[index].String())
		}
		if err := json.NewDecoder(bytes.NewReader(outputs[index].Bytes())).Decode(&results[index]); err != nil {
			t.Fatalf("decode caller %d output %q: %v", index, outputs[index].Bytes(), err)
		}
	}
	if results[0].Info.PID != results[1].Info.PID || results[0].Info.URL != results[1].Info.URL {
		t.Fatalf("separate callers discovered different servers: %+v %+v", results[0].Info, results[1].Info)
	}
	for index, result := range results {
		want, err := filepath.EvalSymlinks(workdirs[index])
		if err != nil {
			t.Fatal(err)
		}
		if result.Directory != want {
			t.Fatalf("caller %d session directory = %q, want %q", index, result.Directory, want)
		}
	}

	client, err := Connect(context.Background(), workdirs[0], stateDir)
	if err != nil {
		t.Fatal(err)
	}
	connected := make(chan struct{})
	eventsDone := make(chan error, 1)
	var once sync.Once
	go func() {
		eventsDone <- client.Events(context.Background(), func(RawEvent) error {
			once.Do(func() { close(connected) })
			return nil
		})
	}()
	select {
	case <-connected:
	case <-time.After(5 * time.Second):
		t.Fatal("global event request did not connect")
	}
	stopCtx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	stopped, err := StopServer(stopCtx, stateDir)
	if err != nil || !stopped {
		t.Fatalf("StopServer() = (%v, %v)", stopped, err)
	}
	select {
	case err := <-eventsDone:
		if err == nil {
			t.Fatal("pending event request ended without reporting the stopped connection")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("pending event request hung after explicit stop")
	}
	if _, err := os.Stat(filepath.Join(stateDir, stateFileName)); !os.IsNotExist(err) {
		t.Fatalf("discovery state remains after stop: %v", err)
	}
}
