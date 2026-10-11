package runner

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/tylergannon/gimbal/internal/invocation"
)

// Spawn starts an independent workflow; the admission caller never owns its lifetime.
func Spawn(request invocation.Request) (Response, error) {
	executable, err := os.Executable()
	if err != nil {
		return Response{Error: err.Error()}, err
	}
	input, err := json.Marshal(request)
	if err != nil {
		return Response{Error: err.Error()}, err
	}
	read, write, err := os.Pipe()
	if err != nil {
		return Response{Error: err.Error()}, err
	}
	defer func() { _ = read.Close() }()
	defer func() { _ = write.Close() }()
	child := exec.Command(executable, "run-process")
	child.Stdin = bytes.NewReader(input)
	child.ExtraFiles = []*os.File{write}
	child.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := child.Start(); err != nil {
		return Response{Error: err.Error()}, err
	}
	_ = write.Close()
	go func() { _ = child.Wait() }()
	var response Response
	if err := json.NewDecoder(read).Decode(&response); err != nil {
		return response, fmt.Errorf("start runner: %w", err)
	}
	if response.Error != "" {
		return response, errors.New(response.Error)
	}
	if response.RunID == "" {
		return response, errors.New("runner returned no run ID")
	}
	return response, nil
}
