// Package agentexecutor owns native harness turns in independent processes.
// Its control protocol is separate from the command supervisor's contract.
package agentexecutor

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/agy"
	"github.com/tylergannon/gimbal/claude"
	"github.com/tylergannon/gimbal/codex"

	"github.com/tylergannon/gimbal/internal/executor"
	"github.com/tylergannon/gimbal/pi"
)

func init() {
	if len(os.Args) == 3 && os.Args[1] == "--gimbal-agent-worker" {
		var request Request
		if err := json.NewDecoder(os.Stdin).Decode(&request); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		if err := Run(os.Args[2], request); err != nil {
			executor.RejectStartup(err)
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	}
}

type Session struct {
	ID      executor.SessionID `json:"id"`
	Harness string             `json:"harness"`
	Model   string             `json:"model"`
	Effort  string             `json:"effort"`
	Workdir string             `json:"workdir"`
	Parent  executor.SessionID `json:"parent,omitempty"`
}
type Request struct {
	ID      executor.TurnID    `json:"id"`
	Session executor.SessionID `json:"session"`
	Prompt  string             `json:"prompt"`
	Schema  json.RawMessage    `json:"schema,omitempty"`
	Timeout time.Duration      `json:"timeout"`
	Close   bool               `json:"close,omitempty"`
	Fork    bool               `json:"fork,omitempty"`
}
type Result struct {
	Output           []byte                  `json:"output,omitempty"`
	Usage            map[string]gimbal.Usage `json:"usage,omitempty"`
	Error            string                  `json:"error,omitempty"`
	ObservationError string                  `json:"observation_error,omitempty"`
	InvalidOutput    bool                    `json:"invalid_output"`
	Cancelled        bool                    `json:"cancelled"`
	TimedOut         bool                    `json:"timed_out"`
	Completed        time.Time               `json:"completed"`
}
type Event struct {
	Sequence int64 `json:"sequence"`
	gimbal.AgentEvent
	Native json.RawMessage `json:"native,omitempty"`
}
type nativeAdapter interface {
	gimbal.HarnessAdapter
	SaveSession(context.Context, string, string) error
	LoadSession(context.Context, string, bool) (string, error)
	DetachSession(context.Context, string) error
}

func native(harness string) (nativeAdapter, error) {
	var a gimbal.HarnessAdapter
	switch harness {
	case "agy":
		a = agy.New()
	case "pi":
		a = pi.New()
	case "claude":
		a = claude.New()
	case "codex":
		a = codex.New()
	default:
		return nil, fmt.Errorf("unsupported executor harness %q", harness)
	}
	return a.(nativeAdapter), nil
}
func sessionDir(root string, id executor.SessionID) string {
	return filepath.Join(root, "sessions", string(id))
}
func Create(root string, s Session) error {
	if err := executor.Validate(string(s.ID), "session_"); err != nil {
		return err
	}
	if s.Parent != "" {
		if err := executor.Validate(string(s.Parent), "session_"); err != nil {
			return err
		}
	}
	if _, err := native(s.Harness); err != nil {
		return err
	}
	var err error
	s.Workdir, err = filepath.Abs(s.Workdir)
	if err != nil {
		return err
	}
	dir := sessionDir(root, s.ID)
	path := filepath.Join(dir, "session.json")
	for {
		if executor.Exists(path) {
			var accepted Session
			if err = executor.Read(path, &accepted); err != nil {
				return err
			}
			a, _ := json.Marshal(accepted)
			b, _ := json.Marshal(s)
			if string(a) != string(b) {
				return errors.New("session identity conflicts with accepted inputs")
			}
			return nil
		}
		lease, err := executor.Lock(dir)
		if err != nil {
			return err
		}
		if lease != nil {
			defer lease.Close()
			break
		}
		time.Sleep(time.Millisecond)
	}
	if executor.Exists(path) {
		return Create(root, s)
	}

	return executor.Write(path, s)
}
func Submit(root string, req Request) (executor.Descriptor, error) {
	if err := executor.Validate(string(req.ID), "turn_"); err != nil {
		return executor.Descriptor{}, err
	}
	if err := executor.Validate(string(req.Session), "session_"); err != nil {
		return executor.Descriptor{}, err
	}
	if req.Timeout <= 0 {
		req.Timeout = 20 * time.Minute
	}
	binary := os.Getenv("GIMBAL_EXECUTABLE")
	if binary == "" {
		var err error
		binary, err = os.Executable()
		if err != nil {
			return executor.Descriptor{}, err
		}
	}
	d, err := executor.Launch(binary, []string{"--gimbal-agent-worker", root}, req, root)
	if err != nil {
		dir := filepath.Join(root, string(req.ID))
		if executor.Match(dir, req) == nil {
			executor.AssertOwner(dir)
			return executor.Describe(dir, true), nil
		}
	}
	return d, err
}
func Run(root string, req Request) error {
	if err := executor.Validate(string(req.ID), "turn_"); err != nil {
		return err
	}
	if err := executor.Validate(string(req.Session), "session_"); err != nil {
		return err
	}
	dir := filepath.Join(root, string(req.ID))
	lease, err := executor.Claim(dir, req)
	if err != nil {
		return err
	}
	if lease == nil {
		return executor.Startup(executor.Describe(dir, true))
	}
	defer lease.Close()
	sdir := sessionDir(root, req.Session)
	sessionLease, err := executor.Lock(sdir)
	if err != nil {
		return err
	}
	if sessionLease == nil {
		return errors.New("another turn owns this session")
	}
	defer sessionLease.Close()
	var previous executor.TurnID
	if err = executor.Read(filepath.Join(sdir, "active.json"), &previous); err == nil && previous != req.ID {
		executor.AssertOwner(filepath.Join(root, string(previous)))
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}
	var config Session
	if err = executor.Read(filepath.Join(sdir, "session.json"), &config); err != nil {
		return err
	}
	stopVacuum := executor.PeriodicVacuum(root)
	defer stopVacuum()
	ctx, cancel := context.WithTimeout(context.Background(), req.Timeout)
	defer cancel()
	a, err := native(config.Harness)
	if err != nil {
		return err
	}
	var nativeID string
	var activeMu sync.Mutex
	server, err := executor.Listen(dir, func(c executor.Control) executor.Reply {
		if c.Op == "cancel" {
			cancel()
			return executor.Reply{Landed: true}
		}
		if c.Op != "steer" {
			return executor.Reply{Error: "unsupported agent control"}
		}
		activeMu.Lock()
		id := nativeID
		activeMu.Unlock()
		if id == "" {
			return executor.Reply{}
		}
		deadline := c.Deadline
		if deadline.IsZero() {
			deadline = time.Now().Add(30 * time.Second)
		}
		controlCtx, stop := context.WithDeadline(context.Background(), deadline)
		defer stop()
		landed, err := a.Steer(controlCtx, id, c.Message)
		reply := executor.Reply{Landed: landed}
		if err != nil {
			reply.Error = err.Error()
		}
		return reply
	})
	if err != nil {
		return err
	}
	defer server.Close()
	events, err := os.OpenFile(filepath.Join(dir, "events.jsonl"), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer events.Close()
	if executor.Exists(filepath.Join(sdir, "closed.json")) && !req.Close {
		return errors.New("session is closed")
	}
	if err = executor.Write(filepath.Join(dir, "deadline.json"), time.Now().Add(req.Timeout)); err != nil {
		return err
	}
	if err = executor.Write(filepath.Join(sdir, "active.json"), req.ID); err != nil {
		return err
	}
	if err = executor.Accept(dir, req); err != nil {
		return err
	}

	_ = executor.Startup(executor.Describe(dir, true))
	checkpoint := filepath.Join(sdir, "native.json")
	id, setupErr := func() (string, error) {
		if executor.Exists(checkpoint) {
			return a.LoadSession(ctx, checkpoint, req.Close)
		}
		if req.Close {
			return "", nil
		}
		if config.Parent != "" {
			parentDir := sessionDir(root, config.Parent)
			parentLease, err := executor.Lock(parentDir)
			if err != nil {
				return "", err
			}
			if parentLease == nil {
				return "", errors.New("parent session is busy")
			}
			defer parentLease.Close()
			parent, err := a.LoadSession(ctx, filepath.Join(parentDir, "native.json"), false)
			if err != nil {
				return "", err
			}
			defer a.DetachSession(context.Background(), parent)
			return a.Fork(ctx, parent)
		}
		return a.CreateSession(ctx, config.Model, config.Effort, config.Workdir)
	}()
	activeMu.Lock()
	nativeID = id
	activeMu.Unlock()
	if setupErr == nil && id != "" && !req.Close {
		setupErr = a.SaveSession(ctx, id, checkpoint)
	}
	var result gimbal.TurnResult
	turnErr := setupErr
	var observationMu sync.Mutex
	var observationErr error
	if setupErr == nil && !req.Fork && !req.Close {
		var sequence int64
		var eventsMu sync.Mutex
		result, turnErr = a.RunTurn(ctx, id, req.Prompt, req.Schema, func(event gimbal.AgentEvent) error {
			eventsMu.Lock()
			defer eventsMu.Unlock()
			observationMu.Lock()
			failed := observationErr != nil
			observationMu.Unlock()
			if failed {
				return nil
			}
			sequence++
			err := json.NewEncoder(events).Encode(Event{Sequence: sequence, AgentEvent: event, Native: event.NativeRef})
			if err != nil {
				observationMu.Lock()
				observationErr = errors.Join(observationErr, err)
				observationMu.Unlock()
			}
			server.Notify()
			return nil
		})
	}
	if id != "" {
		if req.Close || !executor.Exists(checkpoint) {
			cleanupCtx, stop := context.WithTimeout(context.WithoutCancel(ctx), 15*time.Second)
			turnErr = errors.Join(turnErr, a.Close(cleanupCtx, id))
			stop()
		} else {
			turnErr = errors.Join(turnErr, a.SaveSession(context.Background(), id, checkpoint))
			detachCtx, stop := context.WithTimeout(context.Background(), 15*time.Second)
			turnErr = errors.Join(turnErr, a.DetachSession(detachCtx, id))
			stop()
		}
	}
	if req.Close && turnErr == nil {
		turnErr = executor.Write(filepath.Join(sdir, "closed.json"), time.Now())
	}

	saved := Result{Output: result.Output, Usage: result.Usage, Completed: time.Now(), InvalidOutput: errors.Is(turnErr, gimbal.ErrInvalidOutput), Cancelled: errors.Is(ctx.Err(), context.Canceled), TimedOut: errors.Is(ctx.Err(), context.DeadlineExceeded)}
	if turnErr != nil {
		saved.Error = turnErr.Error()
	}
	if ctx.Err() != nil {
		saved.Error = ctx.Err().Error()
	}
	observationMu.Lock()
	observationErr = errors.Join(observationErr, events.Sync())
	if observationErr != nil {
		saved.ObservationError = observationErr.Error()
	}
	observationMu.Unlock()
	if err = executor.Write(filepath.Join(dir, "result.json"), saved); err != nil {
		panic(err)
	}
	server.Notify()
	return nil
}

// ReadEvents consumes only complete lines. A partial tail remains unread and
// is retried after a notification; applied sequence numbers tolerate replay.
func ReadEvents(path string, offset *int64, sequence *int64, onEvent func(Event) error) error {
	file, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	defer file.Close()
	if _, err = file.Seek(*offset, io.SeekStart); err != nil {
		return err
	}
	reader := bufio.NewReader(file)
	for {
		line, err := reader.ReadBytes('\n')
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		var event Event
		if err = json.Unmarshal(line, &event); err != nil {
			return err
		}
		if event.Sequence > *sequence {
			if err = onEvent(event); err != nil {
				return err
			}
			*sequence = event.Sequence
		}
		*offset += int64(len(line))
	}
}
func Wait(ctx context.Context, d executor.Descriptor, onEvent func(gimbal.AgentEvent) error) (gimbal.TurnResult, error) {
	var offset, sequence int64
	for {
		apply := func(event Event) error {
			event.NativeRef = event.Native
			if onEvent != nil {
				return onEvent(event.AgentEvent)
			}
			return nil
		}
		if err := ReadEvents(d.Events, &offset, &sequence, apply); err != nil {
			return gimbal.TurnResult{}, err
		}
		var result Result
		err := executor.Read(d.Result, &result)
		if err == nil {
			// Publication follows all event writes. Drain once more after seeing result.
			if err = ReadEvents(d.Events, &offset, &sequence, apply); err != nil {
				return gimbal.TurnResult{}, err
			}
			out := gimbal.TurnResult{Output: result.Output, Usage: result.Usage}
			if result.InvalidOutput {
				return out, fmt.Errorf("%w: %s", gimbal.ErrInvalidOutput, result.Error)
			}
			if result.TimedOut {
				return out, context.DeadlineExceeded
			}
			if result.Cancelled {
				return out, context.Canceled
			}
			if result.Error != "" {
				return out, errors.New(result.Error)
			}
			return out, nil
		}
		if !os.IsNotExist(err) {
			return gimbal.TurnResult{}, err
		}
		if err = executor.Wait(ctx, filepath.Dir(d.Result), offset); err != nil {
			return gimbal.TurnResult{}, err
		}
	}
}

type Adapter struct{ harness, root string }

func New(harness string) *Adapter         { return &Adapter{harness: harness} }
func (a *Adapter) state() (string, error) { return executor.Root(a.root) }
func (a *Adapter) CreateSession(_ context.Context, model, effort, workdir string) (string, error) {
	root, err := a.state()
	if err != nil {
		return "", err
	}
	id := executor.NewSessionID()
	err = Create(root, Session{ID: id, Harness: a.harness, Model: model, Effort: effort, Workdir: workdir})
	return string(id), err
}
func (a *Adapter) RunTurn(ctx context.Context, id, prompt string, schema json.RawMessage, onEvent func(gimbal.AgentEvent) error) (gimbal.TurnResult, error) {
	if err := executor.Validate(id, "session_"); err != nil {
		return gimbal.TurnResult{}, err
	}
	if err := ctx.Err(); err != nil {
		return gimbal.TurnResult{}, err
	}
	root, err := a.state()
	if err != nil {
		return gimbal.TurnResult{}, err
	}
	req := Request{ID: executor.NewTurnID(), Session: executor.SessionID(id), Prompt: prompt, Schema: schema, Timeout: 20 * time.Minute}
	if deadline, ok := ctx.Deadline(); ok {
		req.Timeout = time.Until(deadline)
	}
	// Persist the turn identity before handing it to a subprocess.
	submissions := filepath.Join(sessionDir(root, req.Session), "submissions")
	if err = os.MkdirAll(submissions, 0700); err != nil {
		return gimbal.TurnResult{}, err
	}
	if err = executor.Write(filepath.Join(submissions, string(req.ID)+".json"), req); err != nil {
		return gimbal.TurnResult{}, err
	}
	d, err := Submit(root, req)
	if err != nil {
		return gimbal.TurnResult{}, err
	}
	result, err := Wait(ctx, d, onEvent)
	if ctx.Err() != nil {
		stopCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_, controlErr := executor.Call(stopCtx, filepath.Dir(d.Result), executor.Control{Op: "cancel"})
		if controlErr != nil {
			executor.AssertOwner(filepath.Dir(d.Result))
		}
		_, waitErr := Wait(stopCtx, d, nil)
		if waitErr != nil && !errors.Is(waitErr, context.Canceled) && !errors.Is(waitErr, context.DeadlineExceeded) {
			return result, errors.Join(ctx.Err(), waitErr)
		}
		return result, ctx.Err()
	}
	return result, err
}
func (a *Adapter) Steer(ctx context.Context, id, message string) (bool, error) {
	if err := executor.Validate(id, "session_"); err != nil {
		return false, err
	}
	root, err := a.state()
	if err != nil {
		return false, err
	}
	var turn executor.TurnID
	if err = executor.Read(filepath.Join(sessionDir(root, executor.SessionID(id)), "active.json"), &turn); os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	dir := filepath.Join(root, string(turn))
	if executor.Exists(filepath.Join(dir, "result.json")) {
		return false, nil
	}
	reply, err := executor.Call(ctx, dir, executor.Control{Op: "steer", Message: message})
	return reply.Landed, err
}
func (a *Adapter) Fork(ctx context.Context, id string) (string, error) {
	if err := executor.Validate(id, "session_"); err != nil {
		return "", err
	}
	root, err := a.state()
	if err != nil {
		return "", err
	}
	var config Session
	if err = executor.Read(filepath.Join(sessionDir(root, executor.SessionID(id)), "session.json"), &config); err != nil {
		return "", err
	}
	config.Parent = executor.SessionID(id)
	config.ID = executor.NewSessionID()
	if err = Create(root, config); err != nil {
		return "", err
	}
	req := Request{ID: executor.NewTurnID(), Session: config.ID, Fork: true, Timeout: 20 * time.Minute}
	d, err := Submit(root, req)
	if err != nil {
		return "", err
	}
	_, err = Wait(ctx, d, nil)
	return string(config.ID), err
}
func (a *Adapter) Close(ctx context.Context, id string) error {
	root, err := a.state()
	if err != nil {
		return err
	}
	sid := executor.SessionID(id)
	if err = executor.Validate(id, "session_"); err != nil {
		return err
	}
	sdir := sessionDir(root, sid)
	if executor.Exists(filepath.Join(sdir, "closed.json")) {
		return nil
	}
	var turn executor.TurnID
	if err = executor.Read(filepath.Join(sdir, "active.json"), &turn); err == nil {
		dir := filepath.Join(root, string(turn))
		if !executor.Exists(filepath.Join(dir, "result.json")) {
			_, err = executor.Call(ctx, dir, executor.Control{Op: "cancel"})
			if err != nil && !executor.Exists(filepath.Join(dir, "result.json")) {
				executor.AssertOwner(dir)
				return err
			}
			_, waitErr := Wait(ctx, executor.Describe(dir, true), nil)
			if waitErr != nil && !errors.Is(waitErr, context.Canceled) && !errors.Is(waitErr, context.DeadlineExceeded) {
				return waitErr
			}
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	request := Request{ID: executor.NewTurnID(), Session: sid, Close: true, Timeout: 30 * time.Second}
	if err = executor.Write(filepath.Join(sdir, "close.json"), request.ID); err != nil {
		return err
	}
	d, err := Submit(root, request)
	if err != nil {
		return err
	}
	_, err = Wait(ctx, d, nil)
	return err
}
