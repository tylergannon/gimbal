package gimble

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/oklog/ulid/v2"
	"github.com/tylergannon/gimble/internal/live"
	"github.com/tylergannon/gimble/internal/observation"
	"github.com/tylergannon/gimble/workflow"
)

type projectKey struct{}

func runDir(ctx context.Context) string {
	s, _ := ctx.Value(scopeKey{}).(*scope)
	if s == nil || s.run == nil {
		return ""
	}
	return s.run.dir
}

// Project puts the project's directory in the root ctx. Runs are made
// under its runs directory.
func Project(ctx context.Context, dir string) context.Context {
	return context.WithValue(ctx, projectKey{}, dir)
}

// graphs holds the shape of every workflow compiled into this binary, by
// name. Only generated code writes here, and only from an init, so the map
// is complete and never written again by the time a run reads it.
var graphs = map[string]workflow.Graph{}

// RegisterGraph records the shape of one workflow, which the workflow generator
// read from its source, under the workflow's name, which is also its runs'
// name. It is how the binary that runs a workflow knows the workflow's
// shape: the run page draws a run against it. It is not written into the
// run's record; a run read without its binary has the logs.
//
// Only generated code calls this, from the init of the file the generator
// writes; a workflow is wired up by being built. Registering one name twice
// is a programming error and panics.
func RegisterGraph(graph workflow.Graph) {
	if _, ok := graphs[graph.Name]; ok {
		panic(fmt.Sprintf("gimble: a graph named %q is already registered", graph.Name))
	}
	graphs[graph.Name] = graph
}

type run struct {
	dir       string                  // <project>/runs/<id>
	models    map[string]ModelBinding // what each role the workflow names runs on
	writer    *eventWriter
	project   *eventWriter
	store     *observation.Store
	mu        sync.Mutex
	sessions  map[string]*eventWriter
	scopes    map[string]*scope                  // live scopes by key, for cancelScope
	turns     map[string]context.CancelCauseFunc // running turns by id, for cancelTurn
	errMu     sync.Mutex
	recordErr error
	closeMu   sync.Mutex
	closeErrs []error
}

// CloseError aggregates every HarnessAdapter.Close failure a run's sessions
// produced as their scopes ended, one entry per failing session. Run joins
// it into its returned error; it never enters a scope's own error, since
// cleanup happens after the scope's body has already established its
// result. Unwrap lets errors.Is and errors.As see through to the individual
// failures.
type CloseError struct {
	errs []error
}

func (e *CloseError) Error() string {
	parts := make([]string, len(e.errs))
	for i, err := range e.errs {
		parts[i] = err.Error()
	}
	return "gimble: close: " + strings.Join(parts, "; ")
}

func (e *CloseError) Unwrap() []error { return e.errs }

// recordCloseFailure folds one session's Close failure into the run's
// aggregate close error, under a mutex since sibling scopes end
// concurrently (Group, Loop tasks).
func (r *run) recordCloseFailure(sessionID string, err error) {
	if r == nil || err == nil {
		return
	}
	r.closeMu.Lock()
	defer r.closeMu.Unlock()
	r.closeErrs = append(r.closeErrs, fmt.Errorf("%s: %w", sessionID, err))
}

// closeError returns the run's aggregate close error, or nil if every
// session closed cleanly.
func (r *run) closeError() error {
	if r == nil {
		return nil
	}
	r.closeMu.Lock()
	defer r.closeMu.Unlock()
	if len(r.closeErrs) == 0 {
		return nil
	}
	return &CloseError{errs: append([]error(nil), r.closeErrs...)}
}

// Run starts one run of a workflow and blocks until the body returns. models
// binds every role the workflow names to the harness, model, and reasoning
// effort it runs on; a role the workflow names and models leaves out is a
// programming error and panics where the session is created.
// The run's ctx derives from the caller's, so main can put a deadline on it.
// The run is the root scope: when the body returns, each session it created
// is actually released through its adapter's Close, then its ctx is
// cancelled.
// The body must join its concurrent work before returning. Run then finishes
// every log it owns and returns the body's error joined with a *CloseError
// aggregating every session's Close failure (nil when there were none) and
// with the first recording failure, if any; cancellation alone is not
// completion.
func Run(ctx context.Context, name string, models map[string]ModelBinding, body func(ctx context.Context) error) error {
	project, _ := ctx.Value(projectKey{}).(string)
	if project == "" {
		return errors.New("gimble: Run needs gimble.Project in its ctx")
	}
	project, err := filepath.Abs(project)
	if err != nil {
		return fmt.Errorf("gimble: %w", err)
	}
	id := ulid.Make().String() + "." + name
	dir := filepath.Join(project, "runs", id)
	if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
		return fmt.Errorf("gimble: %w", err)
	}
	if err := os.Mkdir(dir, 0o755); err != nil {
		return fmt.Errorf("gimble: %w", err)
	}
	w, err := newEventWriter(filepath.Join(dir, "run.jsonl"))
	if err != nil {
		return fmt.Errorf("gimble: %w", err)
	}
	r := &run{dir: dir, models: models, writer: w, sessions: make(map[string]*eventWriter), scopes: make(map[string]*scope), turns: make(map[string]context.CancelCauseFunc)}
	// The run owns its observation store. With the web runtime in ctx it is
	// registered there and the page can read it; without one the run still
	// owns a private store and writes the same table files, so observation
	// does not depend on who started the run.
	store, storeErr := observation.Open(observation.FromContext(ctx), id, name, dir)
	r.store = store
	r.recordFailure("open observation", storeErr)
	pw, pwErr := newEventWriter(filepath.Join(project, "project.jsonl"))
	if pwErr != nil {
		r.recordFailure("open project log", pwErr)
	} else {
		r.project = pw
	}
	r.projectEvent(RunStarted{Name: name})
	r.event("", "", "", RunStarted{Name: name})
	logf("run %s started in %s", id, dir)
	// With the web runtime in ctx the run is reachable by id for as long as
	// its body runs: an operator with only ids can steer a session or kill
	// a scope or a turn through it. It leaves the runtime's table as soon
	// as the body returns or panics, before the logs close.
	release := func() {}
	if hook := live.FromContext(ctx); hook != nil {
		release = hook(id, r)
	}
	err = func() error {
		defer release()
		return r.root(ctx, name, body)
	}()
	// A panic that a Group child recovered comes back as the body's error
	// and is raised again here, after the same terminal records a panic in
	// the body itself gets: misuse anywhere means one thing, a complete
	// run.jsonl and a dead process.
	if escaped, ok := errors.AsType[*panicError](err); ok {
		_ = r.finish(name, err)
		panic(escaped)
	}
	if ctx.Err() != nil {
		cancelled := RunCancelled{Name: name, Source: steerSource(ctx), Error: ctx.Err().Error()}
		r.event("", "", "", cancelled)
		r.projectEvent(cancelled)
	}
	return r.finish(name, err)
}

// root runs body as the run's root scope. A panic in the body, which is
// what Set and its kin do on misuse, is recorded before it escapes: the
// run's terminal records are written, then the panic continues with its
// original value.
func (r *run) root(ctx context.Context, name string, body func(ctx context.Context) error) error {
	defer func() {
		if v := recover(); v != nil {
			_ = r.finish(name, fmt.Errorf("gimble: panic: %v", v))
			panic(v)
		}
	}()
	return (&scope{run: r}).do(ctx, body)
}

// finish writes the run's terminal records and closes what it holds:
// RunEnded with the verdict, then Complete with any recording failure.
// It returns the verdict joined with the recording failure.
func (r *run) finish(name string, err error) error {
	// The root scope's deferred end already ran inside do, closing every
	// session before body returned control here, so the aggregate close
	// error is complete by now: RunEnded, the observation status, and
	// Run's returned error all carry the same verdict.
	err = errors.Join(err, r.closeError())
	r.event("", "", "", RunEnded{Name: name, Error: errString(err)})
	r.projectEvent(RunEnded{Name: name, Error: errString(err)})
	// The store's tables are already on disk: closing it ends the page's
	// subscriptions and nothing else. After this the run is no longer live,
	// and it is served from the store the registry kept.
	r.recordFailure("close observation", r.store.Close())
	r.closeSessions()
	if r.project != nil {
		r.recordFailure("close project log", r.project.close())
	}
	r.event("", "", "", Complete{RecordingError: errString(r.recordingError())})
	r.recordFailure("close run log", r.writer.close())
	err = errors.Join(err, r.recordingError())
	logf("run %s ended: %v", filepath.Base(r.dir), orNone(err))
	return err
}

// addScope and removeScope keep the run's table of live scopes: a scope is
// reachable by key from the moment its ctx exists until end starts closing
// its sessions.
func (r *run) addScope(s *scope) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.scopes[s.key] = s
}

func (r *run) removeScope(s *scope) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.scopes, s.key)
}

// addTurn and removeTurn keep the run's table of running turns, keyed by
// turn id, around RunTurn.
func (r *run) addTurn(id string, cancel context.CancelCauseFunc) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.turns[id] = cancel
}

func (r *run) removeTurn(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.turns, id)
}

// The run is the live.Controller the web runtime holds while the body
// runs. Its methods are exported so the interface can name them; run itself
// stays unexported, so nothing here is part of the package's API.

// Steer sends message into the running turn of the session id, as the
// person watching the run: the Steer record carries Source "person", which
// tells an operator's steer from a supervisor's. The session is found
// through its scope, so a session whose scope has ended is unknown.
func (r *run) Steer(ctx context.Context, sessionID, message string) (bool, error) {
	key := path.Dir(sessionID) // ids are <scope key>/<session name.N>
	if key == "." {
		key = "" // a session of the root scope
	}
	r.mu.Lock()
	s := r.scopes[key]
	r.mu.Unlock()
	if s == nil {
		return false, fmt.Errorf("gimble: no live session %q", sessionID)
	}
	s.mu.Lock()
	i := slices.IndexFunc(s.sessions, func(session *Session) bool { return session.id == sessionID })
	s.mu.Unlock()
	if i < 0 {
		return false, fmt.Errorf("gimble: no live session %q", sessionID)
	}
	return s.sessions[i].Steer(withSteerSource(context.WithValue(ctx, scopeKey{}, s), "person"), message)
}

// SteerLoop holds message for the planner of the live loop key. A planner
// is not always in a turn, so the message waits for the loop's next
// planning decision instead of being dropped: it is recorded on the loop's
// scope as landed when the planner reads it, and as dropped if dispatch
// ends first. A key that is not a loop still dispatching is an error.
func (r *run) SteerLoop(key, message string) error {
	r.mu.Lock()
	s := r.scopes[key]
	r.mu.Unlock()
	if s == nil || !s.loop {
		return fmt.Errorf("gimble: no live loop %q", key)
	}
	if !s.queueMessage(message) {
		return fmt.Errorf("gimble: the loop %q has stopped dispatching", key)
	}
	logf("%s: a message is waiting for the planner's next decision: %s", key, oneLine(message))
	return nil
}

// CancelScope cancels the live scope key with cause, which every scope and
// turn under it then reports through context.Cause. The kill is recorded
// as a Killed lifecycle event on the scope before its ctx ends. An unknown
// or already ended key is an error.
func (r *run) CancelScope(key string, cause error) error {
	r.mu.Lock()
	s := r.scopes[key]
	r.mu.Unlock()
	if s == nil {
		return fmt.Errorf("gimble: no live scope %q", key)
	}
	r.event(key, "", "", killedEvent(key, cause))
	s.cancel(cause)
	return nil
}

// CancelTurn cancels the running turn id with cause. Only that turn ends:
// its Generate returns an error whose cause is Killed, and its scope and
// session keep running. An unknown or already ended id is an error.
func (r *run) CancelTurn(id string, cause error) error {
	r.mu.Lock()
	cancel := r.turns[id]
	r.mu.Unlock()
	if cancel == nil {
		return fmt.Errorf("gimble: no running turn %q", id)
	}
	session := path.Dir(id) // ids are <scope key>/<session name.N>/turn.N
	scope := path.Dir(session)
	if scope == "." {
		scope = "" // a session of the root scope
	}
	r.event(scope, session, id, killedEvent(id, cause))
	cancel(cause)
	return nil
}

// killedEvent is the record of a kill: the cause itself when it is a
// Killed, and otherwise the target with the cause's text as the reason.
func killedEvent(target string, cause error) Killed {
	if killed, ok := errors.AsType[Killed](cause); ok {
		return killed
	}
	return Killed{Target: target, Reason: errString(cause)}
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// logf traces what the runtime does on stderr until the run log exists.
func logf(format string, args ...any) {
	log.Printf("gimble: "+format, args...)
}

func orNone(err error) any {
	if err == nil {
		return "ok"
	}
	return err
}

// observeLifecycle folds one lifecycle record into the run's store and
// publishes what changed to the page. record is the exact LifecycleRecord
// JSON the run log wrote, so the store reduces the same bytes live that a
// replay of the log reduces later.
//
// The store decides what each kind means; this is only the handover. A record
// the store cannot fold, or a table file it cannot write, is a recording
// failure and enters the run's own verdict.
func (r *run) observeLifecycle(_, _, _ string, _ LifecycleEvent, record json.RawMessage) {
	if r == nil || r.store == nil {
		return
	}
	r.recordFailure("observe lifecycle", r.store.Lifecycle(record))
}

// observeAgent applies one stamped native event to its invocation's
// projection and publishes it. The envelope is the event exactly as the
// session log holds it; NativeRef rides beside it as placement metadata and
// is never inserted into the native event.
//
// The error is both recorded as a recording failure and returned, so a turn
// whose events cannot be observed fails where it is produced instead of
// continuing against an observation that no longer describes it. A read the
// runtime cannot answer is not this error: it is reported on its own read
// frame and does not fail the turn.
func (r *run) observeAgent(scope, session, turn string, event AgentEvent) error {
	if r == nil || r.store == nil {
		return nil
	}
	envelope, err := json.Marshal(event)
	if err == nil {
		err = r.store.Event(observation.Placement{Scope: scope, Session: session, Turn: turn}, envelope, event.NativeRef)
	}
	if err != nil {
		r.recordFailure("observe event "+turn, err)
		return err
	}
	return nil
}
