// Package runner hosts independent ordinary workflows in a shared Go host.
package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/backend"
	"github.com/tylergannon/gimbal/internal/effects"
	"github.com/tylergannon/gimbal/internal/invocation"
	"github.com/tylergannon/gimbal/internal/live"
	"github.com/tylergannon/gimbal/internal/observation"
	"github.com/tylergannon/gimbal/runstore"
	"golang.org/x/sys/unix"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

type Response struct {
	Retry bool   `json:"retry,omitempty"`
	RunID string `json:"run_id,omitempty"`
	Error string `json:"error,omitempty"`
}

func Run(request invocation.Request, startup io.Writer) error {
	if err := invocation.LoadPlugins(request.PluginDirs); err != nil {
		_ = json.NewEncoder(startup).Encode(Response{Error: err.Error()})
		return err
	}
	// Validate declarations and configuration before opening task connections.
	if _, _, err := invocation.Prepare(request); err != nil {
		_ = json.NewEncoder(startup).Encode(Response{Error: err.Error()})
		return err
	}
	ctx := effects.WithEndpoint(context.Background(), request.TaskEndpoint)
	if request.TaskRoute.Environment != "" {
		ctx = effects.WithEnvironment(ctx, request.TaskRoute.Environment)
	}
	var connection backend.Connection
	if request.TaskRoute.Name != "" && request.TaskRoute.Name != "local" {
		var err error
		connection, err = gimbal.DefaultRegistry.Connect(ctx, request.TaskRoute)
		if err != nil {
			_ = json.NewEncoder(startup).Encode(Response{Error: err.Error()})
			return err
		}
		ctx = effects.WithConnection(ctx, connection)
		defer func() { _ = connection.Close() }()
	}
	if request.TaskRoute.Environment != "" && connection.Messages == nil {
		err := fmt.Errorf("named task environments require orchestration and messaging")
		_ = json.NewEncoder(startup).Encode(Response{Error: err.Error()})
		return err
	}
	models, body, err := invocation.PrepareContext(ctx, request)
	if err != nil {
		_ = json.NewEncoder(startup).Encode(Response{Error: err.Error()})
		return err
	}
	return run(request, startup, models, body, connection)
}

func run(request invocation.Request, startup io.Writer, models map[gimbal.WorkflowRole]gimbal.ModelBinding, body func(context.Context) error, connections ...backend.Connection) (err error) {
	encoder := json.NewEncoder(startup)
	started := false
	defer func() {
		if value := recover(); value != nil {
			err = fmt.Errorf("workflow panic: %v", value)
		}
		if !started {
			_ = encoder.Encode(Response{Error: fmt.Sprint(err), Retry: errors.Is(err, runstore.ErrBusy)})
		}
	}()
	state := filepath.Join(request.Project, ".gimbal")
	registry := observation.NewRegistry(state)
	runs := live.NewRuns()
	runs.Resolve = func(id string) (live.Controller, error) { return Connect(state, id) }
	ctx := observation.WithRegistry(effects.WithRoute(context.Background(), request.TaskRoute), registry)
	ctx = live.WithRuns(ctx, runs)
	ctx = effects.WithEndpoint(ctx, request.TaskEndpoint)
	if request.TaskRoute.Environment != "" {
		ctx = effects.WithEnvironment(ctx, request.TaskRoute.Environment)
	}
	if len(connections) > 0 && (connections[0].Execution != nil || connections[0].Messages != nil) {
		ctx = effects.WithConnection(ctx, connections[0])
	} else if request.TaskRoute.Name != "" && request.TaskRoute.Name != "local" {
		connection, err := gimbal.DefaultRegistry.Connect(ctx, request.TaskRoute)
		if err != nil {
			return err
		}
		ctx = effects.WithConnection(ctx, connection)
		defer func() { _ = connection.Close() }()
	}

	var server *http.Server
	var lock *os.File
	var socket string
	defer func() {
		if server != nil {
			_ = server.Shutdown(context.Background())
		}
		if socket != "" {
			_ = os.Remove(socket)
		}
		if lock != nil {
			_ = lock.Close()
		}
	}()
	ctx = live.WithHook(ctx, func(id string, c live.Controller) func() {
		dir := filepath.Join(state, "runs", id)
		lock, err = os.OpenFile(filepath.Join(dir, "runner.lock"), os.O_CREATE|os.O_RDWR, 0600)
		if err != nil {
			panic(err)
		}
		if err = unix.Flock(int(lock.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
			panic(err)
		}
		socket = filepath.Join(os.TempDir(), "gimbal-run-"+id[:26]+".sock")
		_ = os.Remove(socket)
		listener, e := net.Listen("unix", socket)
		if e != nil {
			panic(e)
		}
		if e = os.WriteFile(filepath.Join(dir, "controller"), []byte(socket), 0600); e != nil {
			_ = listener.Close()
			panic(e)
		}
		if request.TaskEndpoint != "" {
			if e = os.WriteFile(filepath.Join(dir, "task-owner"), []byte(request.TaskEndpoint), 0600); e != nil {
				panic(e)
			}
		}
		if request.TaskRoute.Name != "" {
			raw, e := json.Marshal(request.TaskRoute)
			if e != nil {
				panic(e)
			}
			if e = os.WriteFile(filepath.Join(dir, "task-route"), raw, 0600); e != nil {
				panic(e)
			}
		}
		server = &http.Server{Handler: handler(c)}
		go func() { _ = server.Serve(listener) }()
		release := runs.Hook(id, c)
		if e = encoder.Encode(Response{RunID: id}); e != nil {
			panic(e)
		}
		started = true
		if closer, ok := startup.(io.Closer); ok {
			_ = closer.Close()
		}
		return release
	})
	dir := filepath.Join(state, "invocations")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	raw, _ := json.Marshal(request)
	file, err := os.OpenFile(filepath.Join(dir, request.RunID+".json"), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	_, writeErr := file.Write(raw)
	syncErr := file.Sync()
	closeErr := file.Close()
	if err = errors.Join(writeErr, syncErr, closeErr); err != nil {
		return err
	}
	err = func() (err error) {
		defer func() {
			if value := recover(); value != nil {
				err = fmt.Errorf("workflow panic: %v", value)
			}
		}()
		return gimbal.Run(gimbal.Project(ctx, state), request.RunID, request.Workflow, models, body)
	}()
	_ = os.Remove(filepath.Join(dir, request.RunID+".json"))
	return err
}

type operation struct {
	Action  string `json:"action"`
	Target  string `json:"target,omitempty"`
	Message string `json:"message,omitempty"`
	By      string `json:"by,omitempty"`
}
type result struct {
	Value bool   `json:"value"`
	Error string `json:"error,omitempty"`
}

func handler(c live.Controller) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var op operation
		if err := json.NewDecoder(r.Body).Decode(&op); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		var value bool
		var err error
		cause := gimbal.Killed{Target: op.Target, By: op.By, Reason: op.Message}
		switch op.Action {
		case "interactive":
			err = c.SubmitInteractive(op.Target, op.Message)
		case "steer":
			value, err = c.Steer(r.Context(), op.Target, op.Message)
		case "loop":
			err = c.SteerLoop(op.Target, op.Message)
		case "scope":
			err = c.CancelScope(op.Target, cause)
		case "turn":
			err = c.CancelTurn(op.Target, cause)
		case "live":
			value = true
		default:
			err = fmt.Errorf("unknown run operation %q", op.Action)
		}
		reply := result{Value: value}
		if err != nil {
			reply.Error = err.Error()
		}
		_ = json.NewEncoder(w).Encode(reply)
	})
}

func Connect(state, id string) (live.Controller, error) {
	if id == "" || filepath.Base(id) != id || id == "." || id == ".." {
		return nil, fmt.Errorf("invalid run ID")
	}
	socket, err := os.ReadFile(filepath.Join(state, "runs", id, "controller"))
	if err != nil {
		return nil, fmt.Errorf("run %q has no controller: %w", id, err)
	}
	c := remote{socket: string(socket)}
	_, err = c.call(context.Background(), operation{Action: "live"})
	return c, err
}

type remote struct {
	socket string
}

func (c remote) call(ctx context.Context, op operation) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	transport := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", c.socket)
	}}
	defer transport.CloseIdleConnections()
	raw, err := json.Marshal(op)
	if err != nil {
		return false, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://run/control", bytes.NewReader(raw))
	if err != nil {
		return false, err
	}
	response, err := (&http.Client{Transport: transport}).Do(req)
	if err != nil {
		return false, err
	}
	defer func() { _ = response.Body.Close() }()
	var reply result
	if err := json.NewDecoder(response.Body).Decode(&reply); err != nil {
		return false, err
	}
	if reply.Error != "" {
		return false, errors.New(reply.Error)
	}
	return reply.Value, nil
}
func (c remote) SubmitInteractive(id, answer string) error {
	_, err := c.call(context.Background(), operation{Action: "interactive", Target: id, Message: answer})
	return err
}
func (c remote) Steer(ctx context.Context, id, message string) (bool, error) {
	return c.call(ctx, operation{Action: "steer", Target: id, Message: message})
}
func (c remote) SteerLoop(id, message string) error {
	_, err := c.call(context.Background(), operation{Action: "loop", Target: id, Message: message})
	return err
}
func (c remote) cancel(action, id string, cause error) error {
	op := operation{Action: action, Target: id, Message: cause.Error()}
	if killed, ok := errors.AsType[gimbal.Killed](cause); ok {
		op.By, op.Message = killed.By, killed.Reason
	}
	_, err := c.call(context.Background(), op)
	return err
}
func (c remote) CancelScope(id string, cause error) error { return c.cancel("scope", id, cause) }
func (c remote) CancelTurn(id string, cause error) error  { return c.cancel("turn", id, cause) }
