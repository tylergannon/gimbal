// Package effects carries concrete task requests. Environment provisioning and
// workflow observations do not participate in this protocol.
package effects

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type Owner struct{ Run, Scope, Task string }
type Process struct {
	Owner
	Workdir, Command string
	Args             []string
	Stdout, Stderr   string
	Service          bool
}
type ProcessResult struct {
	ExitCode    int
	Interrupted bool
	Error       string
}
type Scope struct{ Run, Scope string }
type Session struct {
	Owner
	Harness, Model, Effort, Workdir, Parent string
}
type Turn struct {
	Owner
	Session, Prompt string
	Schema          json.RawMessage `json:",omitempty"`
}
type Target struct{ Run, ID, Message string }
type Reply struct {
	ID    string
	Value bool
	Error string
}
type Poll struct {
	Run, ID string
	After   int
}
type Progress struct {
	Events []json.RawMessage
	Done   bool
	Result json.RawMessage
	Error  string
}
type State struct {
	Pending int
	Error   string
}

type endpointKey struct{}
type ownerKey struct{}

func WithEndpoint(ctx context.Context, url string) context.Context {
	return context.WithValue(ctx, endpointKey{}, url)
}
func Endpoint(ctx context.Context) string { s, _ := ctx.Value(endpointKey{}).(string); return s }
func WithOwner(ctx context.Context, owner Owner) context.Context {
	return context.WithValue(ctx, ownerKey{}, owner)
}
func Identity(ctx context.Context) Owner { o, _ := ctx.Value(ownerKey{}).(Owner); return o }

func Call(ctx context.Context, endpoint, operation string, input, output any) error {
	raw, err := json.Marshal(input)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint+"/tasks/"+operation, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("task daemon %s: HTTP %d", operation, response.StatusCode)
	}
	return json.NewDecoder(response.Body).Decode(output)
}

func Cleanup(ctx context.Context, endpoint, run, scope string) error {
	var reply Reply
	if err := Call(ctx, endpoint, "cleanup", Scope{Run: run, Scope: scope}, &reply); err != nil {
		return err
	}
	if reply.Error != "" {
		return fmt.Errorf("scope cleanup: %s", reply.Error)
	}
	return nil
}

// Wait reconnects reads only; it never repeats execution. Interruption is an
// explicit effect, so losing an observation connection cannot stop native work.
func Wait(ctx context.Context, endpoint string, target Target, onEvent func(json.RawMessage)) (json.RawMessage, error) {
	after := 0
	var disconnected time.Time
	for {
		if ctx.Err() != nil {
			stopCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			var reply Reply
			err := Call(stopCtx, endpoint, "interrupt", target, &reply)
			cancel()
			if err != nil {
				return nil, fmt.Errorf("%w; task remains owned: %v", ctx.Err(), err)
			}
			return nil, ctx.Err()
		}
		var progress Progress
		readCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		err := Call(readCtx, endpoint, "poll", Poll{Run: target.Run, ID: target.ID, After: after}, &progress)
		cancel()
		if err != nil {
			if disconnected.IsZero() {
				disconnected = time.Now()
			}
			if time.Since(disconnected) >= 15*time.Second {
				return nil, fmt.Errorf("task %s remains owned; result unavailable: %w", target.ID, err)
			}
		} else {
			disconnected = time.Time{}
			for _, event := range progress.Events {
				if onEvent != nil {
					onEvent(event)
				}
				after++
			}
			if progress.Done {
				if progress.Error != "" {
					return nil, fmt.Errorf("task %s: %s", target.ID, progress.Error)
				}
				return progress.Result, nil
			}
		}
		select {
		case <-ctx.Done():
		case <-time.After(100 * time.Millisecond):
		}
	}
}

// ConfirmStarted reconciles a lost admission reply using the already assigned
// identity. Reading is safe to retry; the mutation itself is never replayed.
func ConfirmStarted(ctx context.Context, endpoint string, target Target) error {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	var last error
	for {
		var progress Progress
		err := Call(ctx, endpoint, "poll", Poll{Run: target.Run, ID: target.ID}, &progress)
		if err == nil && progress.Error != "task not found" {
			return nil
		}
		last = err
		select {
		case <-ctx.Done():
			return fmt.Errorf("admission of %s is unconfirmed; cleanup retains its scope: %v: %w", target.ID, last, ctx.Err())
		case <-time.After(100 * time.Millisecond):
		}
	}
}
