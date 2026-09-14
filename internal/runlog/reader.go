// Package runlog reads Gimble's persisted run records for the runtime and its
// web application.
package runlog

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"time"
)

// Read replays the run log at dir and follows it until a record whose kind is
// "complete" is observed. T must decode the records written to the log.
// Read calls yield synchronously and returns its error, a read error, or the
// context error. A nil return means the log is complete, not that the run
// succeeded. The caller owns observation separately from execution: cancelling
// this context stops reading, not the run. Do not join Read inside the run's
// body: the final record is written only after that body returns. When dir
// exists before run.jsonl does, Read waits for the file while ctx lives.
func Read[T any](ctx context.Context, dir string, yield func(T) error) error {
	f, err := openRunLog(ctx, dir)
	if err != nil {
		return err
	}
	defer f.Close()

	r := bufio.NewReader(f)
	var pending []byte
	for {
		line, readErr := r.ReadBytes('\n')
		pending = append(pending, line...)
		if len(pending) != 0 && pending[len(pending)-1] == '\n' {
			var envelope struct {
				Event struct {
					Kind string `json:"kind"`
				} `json:"event"`
			}
			if err := json.Unmarshal(pending, &envelope); err != nil {
				return err
			}
			var event T
			if err := json.Unmarshal(pending, &event); err != nil {
				return err
			}
			pending = pending[:0]
			if err := yield(event); err != nil {
				return err
			}
			if envelope.Event.Kind == "complete" {
				return nil
			}
		}
		if readErr != nil && !errors.Is(readErr, io.EOF) {
			return readErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if readErr == nil {
			continue
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(10 * time.Millisecond):
		}
	}
}

func openRunLog(ctx context.Context, dir string) (*os.File, error) {
	file := filepath.Join(dir, "run.jsonl")
	for {
		f, err := os.Open(file)
		if err == nil {
			return f, nil
		}
		if !os.IsNotExist(err) {
			return nil, err
		}
		info, statErr := os.Stat(dir)
		if statErr != nil || !info.IsDir() {
			return nil, err
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(10 * time.Millisecond):
		}
	}
}
