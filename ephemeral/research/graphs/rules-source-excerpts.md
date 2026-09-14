# Gimble contract and runtime excerpts

All line references are to the current checkout at `/Users/tyler/.codex/worktrees/7b5c/gimble` on 2026-09-14. Excerpts are copied from the named files so the graph-rules report has a local, stable evidence leaf.

## Public contract

Source: `doc.go:13-18`

```go
// Project places the project directory in a context. Run uses that context
// to establish the root scope and durable record for one workflow run.
//
// Workflows use normal Go control flow. Scope names a bounded segment of work;
// Group provides an observable form of errgroup-style concurrency; Loop yields
// planner-selected tasks. Gimble supplies these runtime primitives, not named
// tactics: retries, critique rounds, bake-offs, and delivery methods remain
// visible in the workflow that needs them.
//
// NewSession creates a conversation owned by the current scope. Generate runs
// a blocking turn and returns either Text or a schema-bearing Output. Set,
// SetJSON, and ScopeText let the workflow explicitly choose which scoped data
// it places in a prompt; Generate does not inject context implicitly.
```

Source: `scope.go:29-74,127-182,184-201`

```go
// scope is one instance of a named span of the workflow. Nothing is in the
// ctx but a pointer to it.
type scope struct {
    run *run
    parent *scope
    key string
    cancel context.CancelCauseFunc
    mu sync.Mutex
    ordinals map[string]int
    keys []string
    values map[string][]byte
    sessions []*Session
    ended bool
}

func (s *scope) next(name string) string {
    s.ordinals[name]++
    return path.Join(s.key, fmt.Sprintf("%s.%d", name, s.ordinals[name]))
}

func (s *scope) child(name string) *scope { ... parent: s, key: s.next(name) ... }

func (s *scope) adopt(session *Session) {
    session.id = s.next(session.name)
    session.closed = s.ended
    s.sessions = append(s.sessions, session)
}

func Scope(ctx context.Context, name string, body func(context.Context) error) error {
    parent, err := current(ctx)
    if err != nil { return err }
    return parent.child(name).do(ctx, body)
}

func Set[V ~string | ~int | ~bool | ~[]string](ctx context.Context, key string, value V) {
    store(ctx, key, encode(key, value))
}
func SetJSON[V Output](ctx context.Context, key string, value V) {
    store(ctx, key, encode(key, value))
}

func store(ctx context.Context, key string, raw []byte) {
    s, _ := ctx.Value(scopeKey{}).(*scope)
    if s == nil { panic("... no scope ...") }
    s.mu.Lock(); defer s.mu.Unlock()
    if s.ended { panic("... after it ended") }
    if _, ok := s.values[key]; ok { panic("... already set ...") }
    s.values[key] = raw
    s.keys = append(s.keys, key)
    s.run.event(s.key, "", "", ValueSet{Key: key, Value: JSONText(raw)})
}

func ScopeText(ctx context.Context) string {
    shown := make(map[string]bool)
    for s, _ := ctx.Value(scopeKey{}).(*scope); s != nil; s = s.parent {
        // nearest scope wins; sections are rendered outermost first
    }
}
```

Source: `scope.go:92-125`

```go
func (s *scope) end() {
    s.run.removeScope(s)
    s.ended = true
    for _, session := range s.sessions {
        session.closed = true
        if session.native != "" { session.adapter.Close(closeCtx, session.native) }
        s.run.event(s.key, session.id, "", SessionClosed{...})
    }
    s.cancel(nil)
}
```

## Session ownership, fork, and turn lifetime

Source: `session.go:14-47,410-420,487-512`

```go
type Session struct {
    adapter HarnessAdapter
    name, model, workdir, id string
    native string
    turns int
    running, closed bool
}

func NewSession(ctx context.Context, name string, adapter HarnessAdapter, model, workdir string) *Session {
    s := &Session{...}
    if scope, err := current(ctx); err == nil {
        scope.adopt(s)
        scope.run.event(scope.key, s.id, "", SessionCreated{...})
    }
    return s
}

func (s *Session) usable() error {
    switch {
    case s.id == "": return ...not created in a run
    case s.closed: return ...used after its scope ended
    case s.running: return ...already running a turn
    }
}

func (s *Session) Fork(ctx context.Context, name string) (*Session, error) {
    scope, err := current(ctx)
    ... check s.usable(); adapter.Fork(...)
    scope.adopt(fork)
    scope.run.event(scope.key, fork.id, "", SessionCreated{Parent: s.id})
    return fork, nil
}
```

`Generate` calls `s.usable()` and blocks through `adapter.RunTurn`; the turn
has a child cancellation context and is removed from the run's live turn table
after the adapter returns (`session.go:63-74,111-124,189-210`). The caller's
ctx is not checked against the session's creating scope, so using a parent
session in a child context is valid and using a child-owned session with a
different live context can run until ownership cleanup; only post-end use is
runtime-rejected.

## Group and Loop

Source: `group.go:19-43,58-98`

```go
func Group(ctx context.Context, name string) *group {
    parent, err := current(ctx)
    if err != nil { return &group{err: err} }
    g := &group{scope: parent.child(name)}
    g.ctx, g.scope.cancel = context.WithCancelCause(context.WithValue(ctx, scopeKey{}, g.scope))
    g.scope.run.addScope(g.scope)
    g.scope.run.event(g.scope.key, "", "", ScopeBegan{Name: name})
    return g
}

func (g *group) Go(name string, fn func(context.Context) error) {
    child := g.scope.child(name)
    g.wg.Go(func() { child.do(g.ctx, fn) /* first error cancels group */ })
}

func (g *group) Wait() error {
    g.wg.Wait()
    g.scope.run.event(g.scope.key, "", "", ScopeEnded{...})
    g.scope.end()
    return g.err
}
```

Source: `loop.go:66-165`

```go
func Loop(ctx context.Context, name, goal string, planner *Session) *loop { ... }

func (l *loop) Tasks(yield func(context.Context, Task) bool) {
    parent, err := current(l.ctx)
    ...
    parent.child(l.name).do(l.ctx, func(ctx context.Context) error {
        // planner turn
        taskScope := loopScope.child("task")
        taskCtx := context.WithValue(ctx, taskKey{}, task)
        taskScope.do(taskCtx, func(ctx context.Context) error {
            store(ctx, "task", raw)
            more = yield(ctx, task)
            previous = taskScope.localText()
            return nil
        })
        if !more { return nil }
    })
}
```

Each task callback receives a fresh task scope, which owns the values and
sessions created through its ctx; it is ended before the next planner decision.
The implementation reserves the `task` key in that scope (`loop.go:133-140`).
The planner session is supplied by the workflow and is not automatically
created or re-owned by `Loop`.

## Static graph contract

Source: `ephemeral/research/api/API.md:275-293,856-870`

The documented lints are: duplicate constant Set keys in one scope; Set in a
plain loop on a ctx from outside the body; non-constant keys or names; raw
goroutines/WaitGroups/errgroup in workflow code; Group not joined on every
return path; and ctx stored in a struct/global or derived from
`context.Background()` on a node/value path. The static pass should find
`Scope`, `Group`, `NewSession`, `Fork`, `Loop`, `Each`, `WithSupervisor`, `Set`,
and `Get`, trace ctx ancestry, and add data edges from constant keys to prompt
reads. Its documented ctx model allows parameters and lexical captures, forbids
struct storage, and recognizes the `context.With*` family and Gimble functions;
interface calls produce possible implementations and conditional scopes
produce possible scope sets.
