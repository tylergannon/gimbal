package gimbal

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

	"github.com/tylergannon/gimbal/backend"
	"github.com/tylergannon/gimbal/internal/effects"

	"github.com/tylergannon/gimbal/internal/live"
	"github.com/tylergannon/gimbal/internal/observation"
	"github.com/tylergannon/gimbal/internal/runstate"
	"github.com/tylergannon/gimbal/runstore"
	"github.com/tylergannon/gimbal/workflow"
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
// under its runs directory. This alone does not attach a run to a web instance.
func Project(ctx context.Context, dir string) context.Context {
	return context.WithValue(ctx, projectKey{}, dir)
}

// RegisteredGraph returns the workflow graph compiled into this binary under
// name. A run's workflow name is the lookup key used by the web application.
// The returned graph is immutable registration data and must not be modified.
func RegisteredGraph(name string) (workflow.Graph, bool) {
	d, ok := DefaultRegistry.Workflow(name)
	return d.Graph, ok
}

type run struct {
	interactionMu      sync.Mutex
	interactionEnded   map[string]bool
	interactionCursor  uint64
	interactionStates  map[runstore.NodeID]interactionState
	interactionInputs  map[string]interactionInput
	interactionOrder   []string
	activeInteractions map[string]interactionRuntime
	replay             map[runstore.NodeID][]runstore.Record
	closed             bool
	lifecycleMu        sync.Mutex
	observationErr     error
	records            runstore.Run
	rootNode           runstore.NodeID
	nodeMu             sync.Mutex
	nodeByKey          map[string]runstore.NodeID
	dir                string                        // <project>/runs/<id>
	models             map[WorkflowRole]ModelBinding // what each role the workflow names runs on
	writer             *eventWriter
	project            *eventWriter
	store              *observation.Store
	mu                 sync.Mutex
	sessions           map[string]*eventWriter
	scopes             map[string]*scope                  // live scopes by key, for cancelScope
	turns              map[string]context.CancelCauseFunc // running turns by id, for cancelTurn
	rootCancel         error                              // explicit cancellation of the root scope
	errMu              sync.Mutex
	recordErr          error
	closeMu            sync.Mutex
	closeErrs          []error
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
	return "gimbal: close: " + strings.Join(parts, "; ")
}

func (e *CloseError) Unwrap() []error { return e.errs }

// recordCloseFailure folds one session's Close failure into the run's
// aggregate close error, under a mutex since sibling scopes end
// concurrently (Group, PromiseLoop tasks).
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

// Run executes one recorded workflow until its body completes. Interactive
// waits keep Run pending. After process restart, re-enter the same body with
// the same run ID to reuse saved results and continue unfinished work.
// A TypeSafe key is required only when the workflow uses WithSupervisor.
// models binds every role the workflow names to the harness, model, and reasoning
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
//
// Called with only a Project context, Run executes in the caller's process and
// writes its supplied project's durable files without joining a running web
// instance or its live controls. Use a separate project directory from one
// admitted to a concurrently running instance; shared-state use has no
// supported contract. Hosted runs are submitted to an instance with a
// workflow compiled into its binary.
func Run(ctx context.Context, runID string, name string, models map[WorkflowRole]ModelBinding, body func(ctx context.Context) error) (result error) {
	r, release, err := beginRun(ctx, runID, name, models)
	if err != nil {
		return err
	}
	defer release()
	if completed, err := r.completedResult(); completed {
		r.closeObservers()
		return err
	}
	err = r.root(ctx, name, body)
	// A panic that a Group child recovered comes back as the body's error
	// and is raised again here, after the same terminal records a panic in
	// the body itself gets: misuse anywhere means one thing, a complete
	// run.jsonl and a dead process.
	if escaped, ok := errors.AsType[*panicError](err); ok {
		_ = r.finish(name, err)
		panic(escaped)
	}
	return r.complete(ctx, name, err)
}

// beginRun allocates the existing run recorder/control owner without invoking
// a workflow body. Run owns the matching completion.
func beginRun(ctx context.Context, runID string, name string, models map[WorkflowRole]ModelBinding) (*run, func(), error) {
	project, _ := ctx.Value(projectKey{}).(string)
	if project == "" {
		return nil, nil, errors.New("gimbal: Run needs gimbal.Project in its ctx")
	}
	project, err := filepath.Abs(project)
	if err != nil {
		return nil, nil, fmt.Errorf("gimbal: %w", err)
	}
	id := runID
	if !runstore.ValidID(id) {
		return nil, nil, errors.New("gimbal: Run requires a valid ULID run ID")
	}
	dir := filepath.Join(project, "runs", id)

	if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
		return nil, nil, fmt.Errorf("gimbal: %w", err)
	}
	// The run owns its observation store. With the web runtime in ctx it is
	// registered there and the page can read it; without one the run still
	// owns a private store and writes the same table files, so observation
	// does not depend on who started the run.
	var records runstore.Run
	initialized := false
	defer func() {
		if records != nil && !initialized {
			_ = records.Close()
		}
	}()
	store, storeErr := observation.Open(observation.FromContext(ctx), id, name, dir, func() error {
		var err error
		records, err = runstate.Resolve(ctx, project).Create(ctx, runstore.Header{RunID: runstore.RunID(id), Workflow: name})
		if errors.Is(err, runstore.ErrExists) {
			records, err = runstate.Resolve(ctx, project).Open(ctx, runstore.RunID(id))
		}
		if err != nil {
			return err
		}
		if err = os.MkdirAll(dir, 0700); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dir, "recording-format"), []byte("1\n"), 0600)
	})
	if store == nil {
		return nil, nil, fmt.Errorf("gimbal: %w", storeErr)
	}
	w, err := newEventWriter(filepath.Join(dir, "run.jsonl"))
	header, headerErr := records.Header(ctx)
	if headerErr != nil {
		_ = store.Close()
		if w != nil {
			_ = w.close()
		}
		return nil, nil, headerErr
	}
	r := &run{interactionStates: map[runstore.NodeID]interactionState{}, interactionInputs: map[string]interactionInput{}, activeInteractions: map[string]interactionRuntime{}, records: records, rootNode: header.Root, nodeByKey: map[string]runstore.NodeID{"": header.Root}, dir: dir, models: models, writer: w, sessions: make(map[string]*eventWriter), scopes: make(map[string]*scope), turns: make(map[string]context.CancelCauseFunc)}
	if err := r.loadReplay(); err != nil {
		return nil, nil, err
	}
	r.observationFailure("open run log", err)
	r.store = store
	if d, ok := DefaultRegistry.Workflow(name); ok {
		raw, err := json.Marshal(d.Graph)
		if err == nil {
			err = os.WriteFile(filepath.Join(dir, "graph.json"), raw, 0644)
		}
		r.observationFailure("save run graph", err)
		// The designer's source page, saved beside the graph so the run page
		// draws this run with the shapes the workflow designer draws.
		if len(d.SourcePage) > 0 {
			r.observationFailure("save run source page", os.WriteFile(filepath.Join(dir, "source.json"), d.SourcePage, 0644))
		}
	}
	r.observationFailure("open observation", storeErr)
	pw, pwErr := newEventWriter(filepath.Join(project, "project.jsonl"))
	if pwErr != nil {
		r.observationFailure("open project log", pwErr)
	} else {
		r.project = pw
	}
	r.event("", "", "", RunStarted{Name: name})
	r.projectEvent(RunStarted{Name: name})
	logf("run %s started in %s", id, dir)
	// With the web runtime in ctx the run is reachable by id for as long as
	// its body runs: an operator with only ids can steer a session or kill
	// a scope or a turn through it. It leaves the runtime's table as soon
	// as the body returns or panics, before the logs close.
	release := func() {}
	if hook := live.FromContext(ctx); hook != nil {
		release = hook(id, r)
	}
	initialized = true
	return r, release, nil
}

func (r *run) complete(ctx context.Context, name string, err error) error {
	if cause := r.rootCancellation(); cause != nil {
		source := ""
		if killed, ok := errors.AsType[Killed](cause); ok {
			source = killed.By
		}
		cancelled := RunCancelled{Name: name, Source: source, Error: cause.Error()}
		r.event("", "", "", cancelled)
		r.projectEvent(cancelled)
	} else if ctx.Err() != nil {
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
			_ = r.finish(name, fmt.Errorf("gimbal: panic: %v", v))
			panic(v)
		}
	}()
	connection := effects.Connection(ctx)
	if connection.Secrets == nil && effects.Endpoint(ctx) == "" {
		connection.Secrets = backend.NewMemorySecrets()
		ctx = effects.WithConnection(ctx, connection)
	}
	return (&scope{run: r, node: r.rootNode}).do(ctx, func(ctx context.Context) error {
		if cause := r.rootCancellation(); cause != nil {
			return cause
		}
		return body(ctx)
	})
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
	r.event("", "", "", Complete{RecordingError: errString(errors.Join(r.recordingError(), r.observationError()))})
	r.recordFailure("flush run store", r.records.Flush(context.Background()))
	r.closeObservers()
	err = errors.Join(err, r.recordingError(), r.observationError())
	logf("run %s ended: %v", filepath.Base(r.dir), orNone(err))
	return err
}

// Closing and event callbacks share the lifecycle mutex. Late callbacks cannot
// reopen session logs or write through observation handles after shutdown.
func (r *run) closeObservers() {
	r.lifecycleMu.Lock()
	defer r.lifecycleMu.Unlock()
	r.closed = true
	defer func() { r.recordFailure("close run store", r.records.Close()) }()
	r.observationFailure("close observation", r.store.Close())
	r.closeSessions()
	if r.project != nil {
		r.observationFailure("close project log", r.project.close())
	}
	if r.writer != nil {
		r.observationFailure("close run log", r.writer.close())
	}
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

func (r *run) rootCancellation() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.rootCancel
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
		return false, fmt.Errorf("gimbal: no live session %q", sessionID)
	}
	s.mu.Lock()
	i := slices.IndexFunc(s.sessions, func(session *Session) bool { return session.id == sessionID })
	var session *Session
	if i >= 0 {
		session = s.sessions[i]
	}
	s.mu.Unlock()
	if session == nil {
		return false, fmt.Errorf("gimbal: no live session %q", sessionID)
	}
	return session.Steer(withSteerSource(context.WithValue(ctx, scopeKey{}, s), "person"), message)
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
		return fmt.Errorf("gimbal: no live loop %q", key)
	}
	if !s.queueMessage(message) {
		return fmt.Errorf("gimbal: the loop %q has stopped dispatching", key)
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
	if s == nil {
		r.mu.Unlock()
		return fmt.Errorf("gimbal: no live scope %q", key)
	}
	if cause == nil {
		cause = context.Canceled
	}
	if key == "" {
		if r.rootCancel != nil {
			r.mu.Unlock()
			return nil
		}
		r.rootCancel = cause
	}
	r.mu.Unlock()
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
		return fmt.Errorf("gimbal: no running turn %q", id)
	}
	session := path.Dir(id) // ids are <scope key>/<session name.N>/turn.N
	scope := path.Dir(session)
	if scope == "." {
		scope = "" // a session of the root scope
	}
	r.interactionMu.Lock()
	address := ""
	for candidate, active := range r.activeInteractions {
		if active.session.id == session {
			address = candidate
			break
		}
	}
	r.interactionMu.Unlock()
	if address != "" {
		err := r.acceptInputAtTurn(interactionInput{ID: string(runstore.NewRecordID()), Address: address, Stop: true, Message: errString(cause)}, id)
		if err == nil {
			r.event(scope, session, id, killedEvent(id, cause))
		}
		return err
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
	log.Printf("gimbal: "+format, args...)
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
func (r *run) observeLifecycle(_, _, _ string, _ LifecycleEvent, record json.RawMessage) error {
	if r == nil || r.store == nil {
		return nil
	}
	err := r.store.Lifecycle(record)
	r.observationFailure("observe lifecycle", err)
	return err
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
		r.observationFailure("observe event "+turn, err)
		return err
	}
	return nil
}

func (r *run) State() string {
	r.lifecycleMu.Lock()
	closed := r.closed
	r.lifecycleMu.Unlock()
	if closed {
		return "terminal"
	}
	return "active"
}
