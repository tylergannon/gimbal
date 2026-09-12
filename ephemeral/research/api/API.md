# Gimble API design record

This file preserves the decisions, rationale, and open questions that shaped
Gimble. It is not the current public contract; the root package's Godoc and
compiling examples define the API that exists.

The rule: **as simple as possible.** Add a name only when a workflow that
exists needs it. Everything else is ordinary Go written in the workflow.

The research behind these decisions is in `gemini/`, `claude/`, and
`chatgpt/` next to this file. The old code in `ephemeral/legacy/` is
inspiration, not the API.

## Sessions and turns

```go
// The adapter holds the agent-specific code (for example, Codex, Claude, or
// another harness). It is untyped: a raw JSON Schema goes in, raw JSON comes
// out.
type HarnessAdapter interface { /* ... */ }

// Session is a concrete struct wrapping one adapter session.
type Session struct { /* adapter, session id, model, workdir */ }

// NewSession creates a session in the scope the ctx is in, named for the
// graph. It cannot fail: the agent process starts on the first turn, and
// the adapter carries the harness-specific config.
func NewSession(ctx context.Context, name string, adapter HarnessAdapter, model, workdir string) *Session

// Generate runs one turn and blocks until it ends. T is a polytype-generated
// type: its schema is sent with the prompt, and the result is validated
// once, here, and decoded into T. A failed validation is an error. For
// gimble.Text no schema is sent and the result is the final message. The
// options attach supervisors (see Supervisors).
func (s *Session) Generate[T Output](ctx context.Context, prompt string, opts ...AgentOption) (T, error)

// Steer injects a message into the turn that is running on this session.
// It is called from another goroutine while Generate blocks. The message
// lands at the worker's next model call. If no turn is running the message
// is dropped, and Steer returns nil either way; the log records whether it
// landed. The error is for a harness that could not be reached.
func (s *Session) Steer(ctx context.Context, message string) error

// Interrupt stops the turn that is running on this session. Generate
// returns with an error. Cancelling Generate's context does the same thing;
// the method is for callers that hold the session but not the context.
// With no turn running it does nothing and returns nil, for the same
// reason Steer does not error: the caller can always lose that race.
func (s *Session) Interrupt(ctx context.Context) error

// Fork returns a new session, named for the graph, in the same workdir,
// with the same conversation so far. The two sessions are independent
// after that.
func (s *Session) Fork(ctx context.Context, name string) (*Session, error)
```

- Session is a concrete struct, not an interface, so it can have a generic
  method (Go 1.27 allows generic methods on concrete types, never on
  interfaces). Unit tests fake the `HarnessAdapter` underneath; proof comes
  from live runs.
- Sessions are created with `gimble.NewSession(ctx, name, adapter, model,
  workdir)`. The run is in the ctx, and so is the scope the session belongs
  to (see Scope). `workdir` is a path. Gimble has no worktree primitive: a
  workflow that wants a candidate in its own worktree runs `git worktree
  add` at the top of the scope body, removes it in a `defer`, and hands the
  path to `NewSession`, so the worktree ends with the scope like everything
  else.
- One method for every turn. A prose turn is `Generate[gimble.Text]`, a
  polytype type Gimble ships, and for it the adapter sends no schema and
  returns the final message. This is where AI SDK 6 landed too: it
  deprecated `generateObject` and left one `generateText`, whose `output`
  setting decides whether the result is text or a validated object. The
  type parameter is that setting.
- Validation happens once, in `Generate`, with `T`'s `ValidateJSON`, and a
  failure is an error. The harnesses enforce the schema natively, so it is
  rare, and a retry is a follow-up on the same session when a live run
  shows one is needed.
- A follow-up is another `Generate` on the same session. There is no
  `FollowUp`.
- `Steer` is a primitive, not an option. Supervisors are built on it, and a
  workflow can call it directly (a human at the console, a watchdog on a
  timer). The legacy adapters implement it: Codex through native
  `turn/steer` on the active turn, Claude by sending on the live SDK
  session, and agy, which has no native steer, by interrupting the process
  and replaying the message on the resumed turn. It never errors for want
  of a turn: a reviewer or a person can always lose the race with the turn
  ending, and an error there would report timing, not a bug. The log says
  whether each steer landed or was dropped, and the page shows whether a
  turn is running before anyone types.
- `Fork` exists because loading a session with research costs tokens and
  minutes, and a fork pays that once. Prime one session, then fork it for
  the coder and the validator so both start already knowing the codebase;
  or research, fork, attempt, and on failure attempt again from the fork
  instead of from scratch. Forks share one harness, so priming that has to
  cross models is a doc both sessions read, not a fork. Two forks in the
  same workdir cannot run turns at once. A fork inherits its parent's
  workdir: the conversation is about that tree and the transcript is full
  of its paths. Whether a harness forks natively or resumes from a copied
  transcript is the adapter's business, not the API's.

## Context

Every operation takes a `context.Context` and stops when it is cancelled.
No cancel channels, no done markers. A cancelled `Generate` interrupts the
native turn and returns `ctx.Err()` (the legacy adapters do this today, and
it was live-attested: a SIGINT stops a Codex turn within seconds).

The rule for which context to derive from: **work derives from the
lifetime that owns it, not from whoever triggered it.**

- The process has a root context. SIGINT cancels it, and every run's
  context derives from it.
- The HTTP server lives as long as the process and sets
  `http.Server.BaseContext` to return the root, so every request context is
  a child of root and dies with it.
- A steer from a handler is a short call; `r.Context()` is fine.
- A run started from a form handler on the page derives from the
  project's root, not the request's (that is `Start`, under Run). A
  dropped browser tab must not kill twenty minutes of work.
- An observer streaming events derives from the request. Losing the client
  ends the stream, not the turn.
- "Cancel when either of two contexts dies" is `context.AfterFunc`, not a
  merged-context type:

  ```go
  ctx, cancel := context.WithCancel(root)
  defer context.AfterFunc(other, cancel)()
  ```
- No Futures. No ACP; Gimble has its own agent drivers.
- Names (`Generate`, `NewSession`, `Output`) are placeholders.

## Scope

A scope is a named span of the workflow: a segment, a lap, one candidate's
attempt. It is the unit of structure the page draws and the unit of data
the workflow hands to agents. The ctx says which scope you are in; the run
owns what is in it.

```go
err := gimble.Scope(ctx, "sprint", func(ctx context.Context) error {
	gimble.Set(ctx, "language", "go")                    // scalars
	gimble.SetJSON(ctx, "research", research)            // polytype-generated types
	lang, ok := gimble.Get[string](ctx, "language")      // nearest scope up the chain
	r, ok := gimble.GetJSON[Research](ctx, "research")   // validated, then decoded
	prompt := task.Description + "\n\n" + gimble.ScopeText(ctx) // the chain, rendered for a prompt
	// ...
	return nil // the scope ends here: its sessions are closed, then its ctx is cancelled
})

g := gimble.Group(ctx, "bakeoff") // a concurrent scope
g.Go("attempt", func(ctx context.Context) error { /* ... */ return nil })
err = g.Wait() // joins the goroutines, then ends the group's scope
```

Identity is in the ctx; data is in the run.

- A scope is a function. It begins when `Scope` calls the body and ends
  when the body returns. There is no handle to close, because a ctx cannot
  know when it goes out of scope and a function boundary can. Ending
  cancels the ctx, and nothing created in the scope outlives it: goroutines
  were joined by their group before the return, sessions are closed, the
  ctx is dead. That is the context rule made mechanical, and it is the rule
  structured concurrency settled on (Trio's nurseries, Kotlin's coroutine
  scopes, Swift's task groups). Cancelling alone bounds nothing in Go,
  since a cancelled ctx is only a signal; waiting is what bounds a
  lifetime, and `Wait` is where it happens.
- `Group` is the concurrent scope, and it is explicit, the way `errgroup`
  is. `g := gimble.Group(ctx, name)` opens it; `g.Go(name, fn)` starts a
  goroutine in a child scope of its own; `g.Wait()` joins them all and ends
  the group. The first error a goroutine returns cancels the group's ctx,
  interrupting the others' turns, and comes back from `Wait`; a goroutine
  whose failure must not stop its siblings returns nil and reports on a
  channel, as the bake-off below does. `g.Go` is the only way a workflow
  starts a goroutine; raw `go`, `sync.WaitGroup`, and `errgroup` are for
  Gimble's own code. Explicit rather than a `Go` that finds its group
  through the ctx, because then a helper cannot quietly attach goroutines
  to a caller's scope and the reader sees the join; Trio passes nurseries
  explicitly for the same reason. That every group is waited on every path
  before its function returns is a lint of the `lostcancel` shape.
- Sessions belong to the scope that created them and are closed when it
  ends. A session is a process and a workdir lease, the resource that
  leaks worst, and without this rule nothing ends one before the run
  does. With it, every agent process has a definite end the page can show,
  a workdir is known to be free at scope end, and a session that escapes
  its scope errors on its next `Generate` instead of running as a zombie.
  `Fork` loses nothing: create the researcher in the parent, prime it in a
  child (the priming turn is bounded by the child, the session by the
  parent), fork it later. A session created in a lap and wanted in the
  next lap is a smell; cross-lap state is made in the loop's scope, as the
  coder is in the Loop example. There is no `Close` on `Session`; Gimble
  calls it. Turns are bounded already: `Generate` blocks, and a
  goroutine's turn is bounded by its group's `Wait`.
- What the rule forbids, and what to do instead. None of it loses
  expressiveness; the fix is always to make the thing in the scope that
  should own it.
  - Fire and forget past your own scope. A watchdog for a turn lives in
    the turn's scope; the server lives in the run.
  - Returning while children run. `Wait` comes before the return, on every
    path, and blocks until they stop. A child that ignores ctx blocks
    forever, the same as a `Wait` does today; the harnesses abide ctx, so a
    turn stops in seconds.
  - A producer in one scope feeding a consumer in a sibling scope. They
    share a parent, and the join is explicit.
  - A goroutine or a session that should outlive a lap. Make it in the
    loop's scope, outside the range.
  - The cost that actually bites: a goroutine you forgot keeps its group's
    `Wait` blocked until it finishes instead of leaking. The page shows the
    group still open with a running child, which is the right failure.
- Nothing is stored in the ctx but a pointer to the scope. The values live
  in a store the run owns, keyed by scope, and every `Set` is a log event,
  so the page shows scope contents with no new endpoint. This is the line
  OpenTelemetry draws: the span is in the ctx and immutable there, and its
  attributes are written freely. The run itself is
  in the root ctx, which is why `NewSession`, `Loop`, and `Scope` are
  package functions and no run handle is threaded through anything.
- `Get` reads the nearest scope up the chain that has the key, like a
  variable: an inner scope shadows an outer one. `ScopeText` renders the
  chain the same way, innermost wins, and the workflow puts it in the
  prompt itself. `Generate` injects nothing.
- Two `Set` functions, because a Go constraint cannot mix a type set with
  an interface that has methods. `Set` takes `~string | ~int | ~float64 |
  ~bool | ~[]string`, tilde so a named string type works, and derives
  their schema. `SetJSON` takes polytype-generated types, the same `Output` that
  `Generate` decodes into, so a value carries its schema and the field
  descriptions an agent reads. An arbitrary struct or a map does not
  compile, which is the point.

  ```go
  // Output is what polytype generates, and what Generate, SetJSON, and
  // GetJSON require.
  type Output interface {
  	Schema() json.RawMessage
  	ValidateJSON([]byte) error
  }
  ```

- Set-once per key per scope instance. The second `Set` of a key errors,
  naming the scope. That turns two goroutines racing on a key into a bug
  report, and it means a value cannot change under a turn that is reading
  it. Revision is shadowing: enter a child scope and set the key there.
  In-place replacement waits for a workflow that needs it.
- `Set` marshals at the call, so a value is a snapshot. A pointer mutated
  afterwards changes nothing, and agents, the page, and the files all see
  the same bytes.
- Keys and scope names are compile-time constants, like node names.
- `Set` on an ended scope, or on a ctx with no scope, is an error. The
  first can only happen through a ctx stored somewhere it should not be;
  the second is a chain severed by `context.Background()`. Both should be
  loud. `Get` on an ended scope is fine; the values are the record.

**Iterators own scopes.** A plain `for` body cannot end a scope per
iteration, so a `Set` inside a plain loop hits set-once on the second
pass. Gimble's iterators make the range body the scope's function: a fresh
scope every iteration, ended when the body returns. `Loop` does this per
lap, and there is a trivial one for a fixed slice:

```go
for ctx, prompt := range gimble.Each(ctx, "attempt", prompts) {
	// ctx is a child scope named "attempt", ended when this body returns
}
```

`Each` is built with `Scope`, before `Loop`: it is the iterator with
nothing to decide, and `Loop` is the least settled thing in this doc.

A group made inside a lap is waited inside the lap. One that should
outlive laps is made in the loop's scope, outside the range.

**Lints.** The rules above are checkable statically, in one analyzer over
the ssa form, and the runtime errors stay as the backstop for what it
cannot see. Every lint fails the build; there is no warning level, because
a name built at runtime silently loses a node from the page, which is worse
than a red build:

- Two `Set` calls with the same constant key under the same scope, with no
  scope between them.
- A `Set` inside a plain `for` body on a ctx from outside the body. Use
  `Each`, or a `Scope` per iteration.
- A non-constant key or name.
- A raw `go` statement, `sync.WaitGroup`, or `errgroup` in a workflow
  package. Goroutines are `Group.Go`.
- A `Group` not waited on every path before its function returns, the
  `lostcancel` shape. This is what makes the bound hold.
- A ctx stored in a struct or a global, or one from `context.Background()`
  on a path that creates nodes or sets values, which is what the existing
  `contextcheck` linter flags. With a scope's ctx never escaping its
  function, "set after end" cannot happen.

**Files.** Later, and without changing the API: the store is written
through to the run directory. Go keeps reading from memory, so there is no
filesystem read race on the Go side; the files are for agents, the page,
and, one day, restart. Layout is the scope key from Events, as in
`run/scopes/lap.3/attempt.2/research.json`, each segment a scope name
with its ordinal. The runtime is the only writer;
agents and the page read. Writes are atomic, temp file then rename, and a
scope's directory is frozen when the scope ends. The directory tree is the scope
tree is the graph. Once files exist, `ScopeText` can reference a large
value by path instead of inlining it.

## Run

A run is the root scope, and there is a runtime object behind it. The
workflow never holds it.

```go
// Run starts one run of a workflow and blocks until the body returns. The
// run's ctx derives from the caller's, so main can put a deadline on it.
func Run(ctx context.Context, name string, body func(ctx context.Context) error) error

// Start is for a caller that must not own the run, such as a form handler
// on the page: the run's ctx derives from the project's root, not the
// request's, and Start returns the run id at once.
func Start(ctx context.Context, name string, body func(ctx context.Context) error) (string, error)

// NewRuntime owns the project directory and starts its web application. It is
// in package web, beside the embedded build, because the page's remote
// functions import gimble and gimble cannot import the page.
func NewRuntime(ctx context.Context, dir string, opts ...Option) (*Runtime, error)

func WithPort(port int) Option
func WithUDS(path string) Option
func WithNoWeb() Option
```

The process is a server first. It serves the project's runs directory for
its whole life, live runs and past ones alike, and it hosts the workflows
compiled into it. `Run` and `Start` create a run inside it: a directory
under the project's runs, the event log, the session registry, the scope
store. They register the run with the server as live, open the root scope,
and call the body. When the body returns, the root scope ends like any other,
sessions closed and ctx cancelled, the log gets its final event, and from
then on the run is served the way every past run is: from its log.

How a run starts is ordinary Go in the workflow's `main`. There is no CLI
in Gimble:

```go
func sprint(ctx context.Context, in SprintInput) error {
	gimble.SetJSON(ctx, "input", in) // the page shows it as the root scope's data
	// ...
	return nil
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	runtime, err := web.NewRuntime(ctx, ".gimble")
	if err != nil {
		log.Fatal(err)
	}
	err = runtime.Run(ctx, "sprint", func(ctx context.Context) error {
		return sprint(ctx, SprintInput{Goal: os.Args[1]})
	})
	if err != nil {
		log.Println(err)
	}
	<-ctx.Done() // keep serving until SIGINT; the page can start more runs meanwhile
}
```

- The process stays up after the body returns, so the page does not die
  under the person looking at it. A headless caller returns instead of
  waiting on the ctx.
- `web.Runtime` directly owns the project and assembled web application; the
  root `gimble` package does not import or expose web assembly. Workflow tests
  can still call `Project` and `Run` with fake adapters and no web listener.
- The page starts runs through one form per workflow, typed with the
  workflow's own input. It is a `skgo.Form` beside the workflow's Svelte
  page, and this is all the code there is:

  ```go
  func startSprint(ctx context.Context, in SprintInput) (string, error) {
  	return gimble.Start(ctx, "sprint", func(ctx context.Context) error { return sprint(ctx, in) })
  }

  var _ = skgo.Form(startSprint)
  ```

  Nothing untyped crosses the wire: no map, no raw JSON. The argument is
  the type the body takes, a polytype type like anything else a workflow
  sets or generates, so the compiler holds the handler to the workflow. skgo generates the page's TypeScript from the Go signatures,
  and its own example has a test that renames a Go field on the wire and
  requires the Svelte type check to fail naming the page that read it, so
  the form cannot drift from the input either. The page's controls are
  named by the input's fields, and an agent can write that page from the
  Go type and its doc comments. One function and one page per workflow
  does not scale to hundreds and does not need to; if it ever hurts, the
  fallback is one `startRun` whose argument is a sealed union of every
  input type, which polytype supports as long as the variants share a
  package. A binary that leaves every start to the page is this `main`
  without the `Run` call.
- A run id is a timestamp and then the name, as in
  `20260910-140322.sprint`: sortable in a listing, readable in a URL,
  unique enough for one machine. The runs directory is `.gimble/runs/` in
  the project being worked on, ignored by git.
- A session id is the key of the scope that created it plus the session's
  name and ordinal there, as in `lap.3/coder.1` (see Events); the
  harness's native id is logged as an attribute. The remote functions take
  the run and the session together, so nothing needs to be unique across
  runs.

The wiring between workflow state and the server is the ctx, in both
directions:

- Every package function (`Scope`, `Group`, `NewSession`, `Loop`, `Set`)
  reads the current scope from its ctx, and the scope knows its run. The
  run is not a second parameter because the scope is already there and the
  run is its root; passing both would be the same pointer twice.
- Every remote function (`watchRun`, `steer`, and the rest) gets a request
  ctx that is a child of the process root, which carries the project: the
  runs directory and the registry of live runs. A watch tails a log from
  disk, live or past, and stops at the run's final event. `steer`,
  `interrupt`, and `cancelRun` need a live run and error on a past one.
- Inside the run, the log has one writer and any number of readers, and
  the registries are maps under a mutex. Nothing is global, so two runs in
  one process do not see each other.
- The log is files in the run directory: one for the run and one per
  session, at paths an agent can be told. A workflow has no Go API for
  events; a reviewer or a post-mortem agent that should know the event
  stream is told where it is in its prompt. Token counts and timing are
  logged per turn from the start, since the harnesses report them.
- The server has its own tests, in the three layers skgo's example uses:
  the remote functions as plain Go over a fixture runs directory (a past
  run replays, a fake live run pushes), the assembled handler over the
  embedded build under `httptest`, and a browser suite for the graph page.

## Concurrency: Scope and Group

Gimble has no built-in tactics for running agents in parallel. There is no
`BakeOff`, no `Race`, no `Parallel`, no `FirstK`. The legacy tree has
bake-off helpers and sketches; they are the pattern this section replaces.
What Gimble has is `Group`, the only way a workflow starts goroutines, with
`Wait` as the join. A workflow that needs a bake-off writes it out, like
this, so the reader sees every decision: how many candidates, what counts
as a finisher, when the losers stop.

**Wait for all of them** (the first error cancels the rest):

```go
var toaster Toaster
var car Car
g := gimble.Group(ctx, "build")
g.Go("toaster", func(ctx context.Context) (err error) { toaster, err = s1.Generate[Toaster](ctx, "make me a toaster"); return })
g.Go("car", func(ctx context.Context) (err error) { car, err = s2.Generate[Car](ctx, "make me a car"); return })
if err := g.Wait(); err != nil { // the first error cancelled the other turn
	return err
}
```

**Take the first k finishers** (`k = 1` is "first one"):

```go
type outcome struct {
	res Result
	err error
}
ctx, cancel := context.WithCancel(ctx) // to cut the losers before Wait
defer cancel()
g := gimble.Group(ctx, "bakeoff")
results := make(chan outcome, len(prompts)) // buffered: senders never block

for _, prompt := range prompts {
	g.Go("attempt", func(ctx context.Context) error {
		// One session per turn in flight. They share the name: the name is the
		// node, and the page stacks the instances under it.
		s := gimble.NewSession(ctx, "candidate", adapter, model, workdir)
		res, err := s.Generate[Result](ctx, prompt)
		results <- outcome{res, err} // a failure travels on the channel,
		return nil                   // because returning it would cancel every other candidate
	})
}

var winners []Result
for range prompts {
	o := <-results
	if o.err != nil {
		continue // a failed candidate is not a finisher
	}
	winners = append(winners, o.res)
	if len(winners) == k {
		break
	}
}
cancel()                         // interrupt the losers' turns
if err := g.Wait(); err != nil { // let them finish shutting down
	return err
}
```

Why each line is there:

- The channel holds every result, so a losing goroutine never blocks on send.
- `cancel()` interrupts the losers' native turns; `Wait()` keeps the
  function from returning while their agent processes are still exiting.
- The goroutines return nil on a failed candidate. An error returned to the
  group cancels the siblings, and one failed candidate must not do that,
  so the failure travels on the channel instead. In the wait-for-all case
  the error is returned, because there a failure is the group's failure.
- The loop reads at most `len(prompts)` results, so it cannot hang when too
  many candidates fail. `len(winners) < k` means it came up short.
- A session runs one turn at a time, so concurrent turns need their own
  sessions.
- Each attempt is a scope of its own, so what it sets never collides with
  a sibling's, and the page shows N attempts under the bake-off.

When the candidates return different types, use one channel per type and a
`select`, written in the workflow that needs it.

## Loop

The [Promises, dispatch, and completion](LOOP.md) record explains the design.
`Loop` is adaptive dispatch: a planner chooses the next useful assignment from
the goal, current evidence, priorities, and dependencies. The workflow chooses
and prepares the planner Session.

```go
loop := gimble.Loop(ctx, "sprint", goal, planner)
for ctx, task := range loop.Tasks {
	implement(ctx, coder, task)
}
if err := loop.Err(); err != nil {
	return err
}
```

The yielded value is the assignment, rather than iterator bookkeeping:

```go
type Task struct {
	Name             string
	Description      string
	DefinitionOfDone string
	Validation       struct {
		Command string
		Query   string
	}
}
```

`Description` states the desired result and necessary non-obvious facts. It
does not prescribe the worker's approach. `DefinitionOfDone` describes success
for this assignment, which may be an evidence-supported investigation rather
than fulfillment of the whole goal. Validation is a request for evidence, not
the result itself; either field may be empty.

The backlog is markdown with YAML frontmatter containing the immutable `goal`
and a structured `tasks` list. It lives under the Loop scope in the run
directory. On each dispatch, the planner receives the backlog, visible scoped
context, and values recorded by the preceding task. It edits the backlog and
returns the next Task exactly as recorded there, or returns no task to end
dispatch. Loop rejects a changed goal, malformed task, or disagreement between
the returned Task and the backlog rather than yielding ambiguous work.

The planner is asked to choose the assignment with the greatest concrete gain
toward the overall Definition of Done, sized for one worker to understand,
complete, and demonstrate in one working session. A nonblocking defect in an
earlier phase does not impose a phase gate; it remains in the backlog while the
planner may choose later work with greater value.

Each yielded context is a child `task` scope. Loop stores the structured Task
there before calling the body. The workflow records the worker result and any
validation evidence with `Set` or `SetJSON`; Loop explicitly projects those
local values into the next planner prompt without promoting them into the
parent scope. The task and planner decision are also structured in the durable
run events, so later backlog edits do not rewrite what was dispatched.

Details:

- Loop does not execute `Validation.Command`, select a validator for
  `Validation.Query`, or infer success from the body returning. The workflow
  performs validation visibly in ordinary Go and records the actual result.
  A deterministic check is binary; agent judgment uses the repository's
  90-95% release standard.
- When the planner returns no assignment, `range` falls through. This means
  dispatch ended; it does not attest that the enclosing promises are fulfilled.
- When a task body returns, its sessions are closed and its ctx is cancelled;
  a group the body made was waited before that, by the lint. The page does not
  cancel tasks, or any scope; it interrupts a session's turn or cancels the
  whole run. A body that treats an interrupted turn as a fact and records it
  gives the planner evidence to plan from; ordinary Go control flow can instead
  end the workflow.
- `Err` is checked after the range, the way `bufio.Scanner` does it, because
  the yielded pair is the context and Task. Loop errors report persistence,
  malformed planner data, cancellation, or planner harness failures. Failed
  task validation is recorded feedback, not a Loop error.
- Limits, retry policy, validation execution, and overall completion remain
  visible in the enclosing workflow.

## Supervisors

`WithSupervisor` attaches a supervisor to a turn. A supervisor is a
session of its own and an instruction saying what to watch for.

```go
func (s *Session) Generate[T Output](ctx context.Context, prompt string, opts ...AgentOption) (T, error)

// AgentOption is an argument to Generate. The same options are a
// supervisor's own where WithSupervisor attaches it.
type AgentOption func(*options)

// opts are the supervisor's: WithInterval, and WithSupervisor for
// supervisors of its looks.
func WithSupervisor(supervisor *Session, instruction string, opts ...AgentOption) AgentOption

// WithInterval is how often a supervisor looks; three minutes by default.
func WithInterval(every time.Duration) AgentOption
```

```go
coder := gimble.NewSession(ctx, "coder", adapter, model, workdir)
taste := gimble.NewSession(ctx, "taste", adapter, cheap, workdir)

res, err := coder.Generate[Result](ctx, task.Description+"\n\n"+gimble.ScopeText(ctx),
	gimble.WithSupervisor(taste, "don't let it over-engineer", gimble.WithInterval(time.Minute)),
)
```

Supervisors watch the turn while it runs and steer the worker when they
object. Steering is the point: a coding turn runs twenty minutes, and a
supervisor that first speaks at the end watches the worker build the
plugin system and then pays a second turn to tear it out.

What `Generate` does when supervisors are attached. This is the whole
definition; if supervision ever needs more than this, inline it instead.

```go
transcript := newTranscript() // the worker's events, appended as they arrive

// The worker's turn, in the background so the supervisors can run beside it.
var res T
var err error
done := make(chan struct{})
go func() {
	defer close(done)
	res, err = s.generate[T](ctx, prompt, transcript.append)
}()

// Each supervisor on its own clock: look at what is new, steer on
// objection, stop when the turn ends.
var wg sync.WaitGroup
for _, sup := range supervisors {
	wg.Go(func() {
		for sup.tick(done) { // at its interval until the turn ends
			events := transcript.sinceLastLook(sup)
			if len(events) == 0 {
				continue
			}
			// A look is a turn with the supervisor's own options.
			review, _ := sup.session.Generate[Review](ctx, look(sup.instruction, prompt, events), sup.opts...)
			if len(review.Objections) > 0 {
				s.Steer(ctx, strings.Join(review.Objections, "\n"))
			}
		}
	})
}
<-done
wg.Wait()
return res, err
```

- Supervisors are attached per turn, not per session. The instruction is
  about the task, and the reader sees at the call site who is watching. A
  session carries no hidden supervisors. What is per session is the
  supervisor itself: its model, its workdir, and its memory of earlier
  looks. A workflow that wants the same watchers on every turn passes the
  same options every time.
- The instruction is the supervisor's prompt, written at the call site, so
  what a supervisor objects to is whatever its caller told it to watch for.
- A review is `struct{ Objections []string }`. An empty list is no
  objection; there is no `Approved` flag, for the same reason `Task` has
  no `Done`.
- Cadence is a clock: every three minutes, or the supervisor's
  `WithInterval`. A tick with nothing new since the last look is skipped,
  and a turn shorter than the interval is never looked at. Supervisors do
  not have to run all the time.
- Supervisors steer; they never gate. There is no final look and no
  follow-up round, and `Generate` returns the worker's result as it is.
  Work is gated on the definition of done, which a validator checks
  (`docs/definition-of-done.md`), not on what a supervisor thinks of it.
  The first version looked after every tool result, took a final look at
  the result, and made its objections a follow-up turn, for up to three
  rounds. Its first live run made 53 looks and 17 steers in eleven
  minutes, most of them about code quality.
- A steer lands at the worker's next model call, same as a person talking
  over its shoulder. The supervisor judged a snapshot that is now a few
  seconds old; that is fine. If the turn ended first the steer is dropped.
- Supervisors are sessions, so one remembers its earlier looks and can say
  "you still have not removed the plugin system."
- Agents and supervisors share one option type. A supervisor's options are
  `AgentOption`s and a look is a `Generate` with them, so any supervisor
  can be supervised: `WithSupervisor(taste, "don't let it over-engineer",
  WithSupervisor(lead, "don't let it nitpick"))`. `WithInterval` means
  something only among a supervisor's options. The overload is for now.
- A supervisor's first look carries the instruction and the worker's
  prompt; every look carries what the worker did since the last one. Tool
  output is shortened; supervisors read files in the workdir themselves.
- Every look is an agent turn. Supervisors should be on the cheap tier.
- Why a built-in when the bake-off is not: the bake-off's decisions differ
  per workflow (how many, what counts as a finisher, when losers stop).
  Supervision's mechanics never differ; its decisions are who supervises,
  what they watch for, and how often, and those are the arguments. And it
  needs the event stream, which a workflow does not otherwise see.

Hierarchy is composition, not a third shape. A `Loop` body that calls
`Generate` with `WithSupervisor` is a manager over a supervised worker; a `Loop`
whose body runs a `Loop` is a manager over leads. The one thing these do not
give is an agent that delegates to agents on its own, without a Go program
in between. That would be `Generate` exposed to the agent as a tool, not a
new Go shape, and it waits for a workflow that needs it.

## Observability

The process serves a web page for every run in the project, live or past,
that shows the graph of agents: every scope with its values, rendered the
way `ScopeText` hands them to an agent inside it, every session, its
turns, the reviewers watching each turn, every steer, and the transcript
behind each node. A person at that page can steer or interrupt any
session through the same `Steer` and `Interrupt` the reviewers use, and
cancel a run.

The graph has two sources. The runtime supplies what happened: every
instance, and every edge, because structure is declared in scopes. A static
pass over the source supplies the same shape before it has run.

**Nodes are named at the call site.** Everything that is a node in the
graph takes a name where it is created, and everything that groups nodes is
a scope:

```go
err := gimble.Scope(ctx, "sprint", body)
g := gimble.Group(ctx, "bakeoff")
g.Go("attempt", body)
coder := gimble.NewSession(ctx, "coder", adapter, model, workdir)
validator, err := researcher.Fork(ctx, "validator")
loop := gimble.Loop(ctx, "sprint", goal, planner)
```

- The name identifies the node, not the instance. Five bake-off candidates
  are five sessions all named `candidate`; a loop body that creates a coder
  every lap creates N sessions named `coder`. The runtime tells instances
  apart by session id and scope instance, and the page draws them as cards
  stacked under the node. So names are compile-time constants, and the
  static pass checks that. A name built at runtime would hide a node from
  it.
- The name is the join key between the static pass and the log; precisely,
  the path of names from the root, which Events below calls the node key.
  Nothing else would work: the runtime cannot know its call site without
  reflection, which is out, and a `file:line` key would churn on every
  edit anyway. A name at the call site is also visible to whoever reads the
  workflow.
- Structure comes from scopes. A node is created with a ctx, the ctx says
  which scope it is in, and scopes nest. So containment is exact at
  runtime, for one line per segment, and a loop's laps and a bake-off's
  attempts are scopes that `Loop`, `Each`, and `Group.Go` make. Nothing is
  inferred. A session created in one scope and used in another shows its
  turns where they ran, which is information, not a problem.
- Reviewers are sessions, so they are nodes like any other. A person
  steering from the page is a supervisor node too, one the runtime adds
  itself since it is not in the source.

**The runtime** writes to the log what happened, as the events listed
below, and that is the whole live graph.

**Events** are of two kinds. Lifecycle events are Gimble's own and go in
the run's log; the project log carries the run-level ones, which is the
server-wide stream. Agent events are the harness's, one file per session
under the run. Both kinds carry the same placement, and the placement is
what draws the graph.

**Keys.** Every scope instance has a key: the chain of scope names from
the run's root, each with the ordinal the runtime gave that instance under
its parent, as in `lap.3/bakeoff.1/attempt.2`. The root's key is empty. A
session's id extends the key of the scope that created it, `lap.3/coder.1`,
and a turn's id extends its session's, `lap.3/coder.1/turn.2`. Every event
carries `seq`, `time`, the key of the scope it happened in, and the session
and turn ids when it has them. That is the whole structure:

- Containment is prefix. An event is inside every scope whose key prefixes
  its own, and the scope tree is the prefix tree of the keys in the log.
  No event names its parent.
- Node identity is the key with the ordinals stripped, `lap/bakeoff/attempt`.
  Every instance with the same node key stacks under one card, and the
  node key, a path of constant names, is the join key with the static
  pass. Two sessions both named `coder` under different scopes are
  different nodes.
- Peers share a parent key. Same node key under one parent means repeated
  instances (laps, attempts); different node keys under one parent are the
  different things that scope did (coder, validator).
- Sequence within a scope instance is `seq` order, and it is a real
  sequence: a scope's body is one goroutine, because `Group.Go` is the
  only way to start another and every `Go` is a child scope. So two
  sibling scopes whose began-ended intervals overlap ran concurrently, in
  a group, and siblings that do not overlap ran one after another, in
  laps or plain calls. The page tells a bake-off from a loop by the
  intervals alone.
- A turn's events carry the key of the scope where the turn ran, which
  may be below the scope that created its session; the session id says
  where it was created. That is how a coder made in the loop's scope shows
  a turn in each lap.
- Data is the same rule. A `Set` carries its scope key; the value a turn
  saw for a key is the nearest prefix's `Set` before the turn's `seq`. The
  page renders what any agent was shown by walking the prefix chain, with
  no event for reads.

Three relations are not prefixes, and they are fields on events that
exist anyway, not new events: a forked session's parent; a supervisor
attachment's reviewer session and worker turn; and a steer's source and
target. Supervision is also the one concurrency inside a single scope:
reviewer turns overlap the worker's, in Gimble's own code, marked by the
attachment. Any other overlap inside one scope key is a bug.

Lifecycle events, ours:

- run: started (name), ended (error or none), cancelled (by whom).
- scope: began (name), ended (error or none). Groups, laps, and attempts
  are scopes; a lap's begin carries its task.
- loop, per lap: each command run with its exit code, and the planner's
  decision.
- set: key, value.
- session: created (name, adapter, model, workdir, parent for a fork),
  closed.
- turn: started (prompt, output type), ended (result or error, tokens,
  duration, interrupted or not). A validation failure is an ended with an
  error.
- supervise: attached (supervisor session, worker turn, instruction,
  interval). Looks are turns on the supervisor's session, so they need no
  event of their own.
- steer: target session, message, source, landed or dropped. interrupt:
  session, source. cancel: run, source.

Agent events, the harness's:

- user message (the prompt, and each steer as it lands), assistant
  message, thinking, tool call, tool result.
- a delta: a part of a message, a thought, or a tool input still
  accumulating, naming the id of what it extends. The log stores what the
  harness emits at the grain it emits it, and the page coalesces by id, so
  a live watch and a replay are the same code.
- usage per model call (tokens, duration); the turn's ended event sums it.
- a harness error or retry; an approval request, which the page may one
  day answer; and a nested transcript when a harness runs a subagent
  inside a tool call, the one shape that is not flat.

What each event carries beyond its placement is settled by the page as it
is drawn.

**The static pass** draws the same graph before it has run: the template
the page shows with nothing lit up yet. Over the ssa form of each `Run` or
`Start` body it finds every `Scope`, `Group`, `NewSession`, `Fork`, `Loop`, `Each`,
`WithSupervisor`, `Set`, and `Get`, with their constant names and keys, and
traces each ctx from the scope that made it to the calls that receive it.
Constant keys add data edges: this scope writes `research`, that node's
prompt reads it. The limitations are Go's existing ctx conventions: ctx
travels as a parameter or a lexical capture, never in a struct; it is
derived only through the `context.With` family and Gimble's own functions,
so a `context.Background()` severs the chain; a call through an interface
resolves to every implementation; and conditional scoping yields a set of
possible scopes, not an error. The same analyzer hosts the lints under
Scope. It is not needed for the live view, so it comes after the server,
and it needs no agent: what an agent was going to guess about structure is
now declared.

The server is a `tylergannon/skgo` app: SvelteKit pages, Go remote
functions beside them, one binary. The known surface so far is six remote
functions shared by every workflow, plus one form per workflow that starts
it (see Run), and there will be more as the page finds what it needs:

```go
// watchRuns is the server-wide stream: every run in the project, live and
// past, pushed now and again whenever a run starts or ends.
func watchRuns(ctx context.Context, yield func([]RunInfo) error) error

// watchRun replays every event of one run from its log, then pushes each
// new one until the run ends or the browser goes away. The page draws the
// graph from these. A past run replays and stops at its final event.
func watchRun(ctx context.Context, runID string, yield func(Event) error) error

// watchSession is the zoomed-in view: one session's turns and transcript,
// replayed then live.
func watchSession(ctx context.Context, runID, sessionID string, yield func(Event) error) error

// steer and interrupt call the Session methods of the same name on a live
// run, and error on a past one. ctx is the request's; a turn does not end
// because a tab closed.
func steer(ctx context.Context, runID, sessionID, message string) error
func interrupt(ctx context.Context, runID, sessionID string) error

// cancelRun cancels a live run's root ctx: every turn is interrupted, every
// scope ends, the body returns. The page cancels whole runs and interrupts
// single turns, and nothing in between; cancelling one scope of a running
// workflow is a knob nobody has asked for.
func cancelRun(ctx context.Context, runID string) error
```

- The watches read logs from disk, so a past run and a live one go through
  the same code, and the log's final event is what tells a watch to stop.
  `Generate` writes events there, and the log feeds the reviewers'
  transcripts, the page, and the post-mortem. The workflow does not see
  events through Go; an agent that should see them is told the path (see
  Run).
- A steer from the page is recorded like a reviewer's steer, attributed to
  the person and marked landed or dropped. In the graph the person is one
  more supervisor node, attached to every session at once. `Steer` takes
  no source argument; the run attributes by call path (supervision knows
  its reviewer, the `steer` command knows it is the console, anything else
  is the workflow).
- Contexts follow the rule above. A live query's `ctx` is the request's, so
  a watch returns when the browser leaves; `steer`, `interrupt`, and
  `cancelRun` are short calls on the request context; nothing the page
  does can end a turn except `interrupt` and `cancelRun`.
- Build order: the API first, all of it, then the events into files, then
  the page in two passes. The sequence is `SPRINTS.md` beside this file.
  The event schema is still answered by what the page needs to draw, which
  is why the page's second pass may grow `Event`.

## Still open

- `Compact`. Open-minded. What is wanted is not the harness's in-place
  compaction but Gimble's own: a chosen prompt that extracts a chosen
  shape of what the session knows. That program exists with today's
  primitives: `Generate[Handoff]` on the old session, `SetJSON` of the
  result in a scope, a new session whose first prompt carries
  `ScopeText`. It puts the handoff on the page as scope data, which a
  native compact never would. A `Compact` name waits for a workflow to
  show that program is too clunky to write out.
- Restart. A rerun is a new run, and resumability is an aspiration the
  design does not bend around. If it comes, it is a new run seeded from an
  earlier one by id, and set-once meets it then.
- How a workflow learns its run directory to tell an agent about the log.
  One function on the ctx when the first workflow needs it.
- The template: how much of it the page shows before a run, once the static
  pass exists.
- Supervisors: the wording of the prompt sent to a supervisor each look
  (the instruction and the worker's prompt on the first look, then what
  the worker did since the last one, asking for objections). Prompt
  engineering against real looks, not API.
- Hierarchy. Wanted: a CEO that hears from supervisors about groups of
  agents, deals only in summaries, steers the bottom-level work indirectly,
  and thinks about strategy and pace rather than tactics. Most of it is
  already here: a top `Loop` whose planner is the CEO, whose laps run the
  leads' loops; the leads' backlog files are the summaries, and agents
  edit those files, so the CEO steers a lead by editing its file and the
  iterator reads it next lap; pace comes from the run log, which an agent
  told the path can read. What it needs beyond that is decided when the
  first one is written, after `Loop` exists.
