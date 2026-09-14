package gimble

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/oklog/ulid/v2"
	"github.com/tylergannon/gimble/internal/observation"
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

type run struct {
	dir       string // <project>/runs/<id>
	writer    *eventWriter
	project   *eventWriter
	store     *observation.Store
	mu        sync.Mutex
	sessions  map[string]*eventWriter
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

// Run starts one run of a workflow and blocks until the body returns. The
// run's ctx derives from the caller's, so main can put a deadline on it.
// The run is the root scope: when the body returns, each session it created
// is actually released through its adapter's Close, then its ctx is
// cancelled.
// The body must join its concurrent work before returning. Run then finishes
// every log it owns and returns the body's error joined with a *CloseError
// aggregating every session's Close failure (nil when there were none) and
// with the first recording failure, if any; cancellation alone is not
// completion.
func Run(ctx context.Context, name string, body func(ctx context.Context) error) error {
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
	r := &run{dir: dir, writer: w, sessions: make(map[string]*eventWriter)}
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
	err = (&scope{run: r}).do(ctx, body)
	if ctx.Err() != nil {
		cancelled := RunCancelled{Name: name, Source: steerSource(ctx), Error: ctx.Err().Error()}
		r.event("", "", "", cancelled)
		r.projectEvent(cancelled)
	}
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
	r.recordFailure("close run log", w.close())
	err = errors.Join(err, r.recordingError())
	logf("run %s ended: %v", id, orNone(err))
	return err
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
