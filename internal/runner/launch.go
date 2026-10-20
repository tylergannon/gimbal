package runner

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/tylergannon/gimbal/internal/invocation"
	"github.com/tylergannon/gimbal/runstore"
	"io"
)

// Spawn starts an independent workflow; the admission caller never owns its lifetime.
func Spawn(request invocation.Request) (Response, error) {
	if request.RunID == "" {
		request.RunID = string(runstore.NewRunID())
	}
	read, write := io.Pipe()
	go func() { defer write.Close(); _ = Run(request, write) }()
	var response Response
	if err := json.NewDecoder(read).Decode(&response); err != nil {
		return response, err
	}
	_ = read.Close()
	if response.Error != "" {
		if response.Retry {
			return response, fmt.Errorf("%w: %s", runstore.ErrBusy, response.Error)
		}
		return response, errors.New(response.Error)
	}
	return response, nil
}
