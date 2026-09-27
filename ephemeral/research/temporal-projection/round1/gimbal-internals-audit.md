# Gimbal internals audit: what a Temporal-shaped projection would have to deal with

Scope: source-only code audit of the `gimbal` checkout as of 2026-09-27
(`git log -1` HEAD at time of writing). No web research; every claim below
is either a direct citation (`file:line`) or marked `[inference]`. Read
against `ephemeral/research/temporal-projection/BRIEF.md`.

## 1. Runtime inventory

### 1.1 The one exported seam, and the two that don't exist

`HarnessAdapter` (`harness.go:14-40`) is the **only** exported interface a
different backend can implement without touching package `gimbal`. It has
five methods: `CreateSession`, `RunTurn`, `Steer`, `Fork`, `Close`. Every
adapter (`codex/`, `claude/`, `pi/`, `opencode/`, `agy/`) is a private
struct implementing it; `ModelBinding` (`harness.go:45-50`) just pairs one
adapter value with a model string and effort string.

There is **no equivalent interface for commands or services**.
`RunCommand`/`Check` call `os/exec.CommandContext` directly
(`command.go:241`), and `Service` calls `os/exec.Command("zsh", "-c",
command)` directly (`service.go:72`). Nothing named `CommandRunner` or
similar exists, and none of `command.go`/`service.go` take an adapter
argument. So "projecting to remote execution" is not a single seam
problem: agent turns already have a pluggable boundary; commands and
services do not, and adding one is exactly the kind of "new exported name"
`AGENTS.md`'s No-Wrappers rule reserves for Tyler to ask for by name. This
is a real, unavoidable design decision, not an implementation detail (see
§8 and the open questions).

The runtime state itself — `*run` (`run.go:67-83`) and `*scope`
(`scope.go:32-51`) — is unexported, holds live mutexes, goroutines,
channels, and OS handles, and is threaded through `context.Context` as a
raw pointer under an unexported key (`scopeKey{}`, `scope.go:27`,
`context.WithValue(ctx, scopeKey{}, s)` at `scope.go:158`, `group.go:40`).
It cannot cross a process boundary; nothing serializes it and nothing is
meant to. There is no swappable "runtime" interface behind `Run`/`Scope`/
`Group` the way `HarnessAdapter` is swappable behind `NewSession`/
`Generate` — the control-flow runtime is one concrete implementation.

### 1.2 Scope identity and node ids

- ctx carries a `*scope` under `scopeKey{}` (`scope.go:27,96-101`); `current(ctx)` is the only accessor and panics-by-returning-error when absent.
- A scope's `key` is `path.Join(parent.key, "name.N")`, computed by `(*scope).next(name)` under `s.mu` (`scope.go:104-110`); the ordinal `N` is `s.ordinals[name]++`, a per-scope map keyed by the literal name string.
- `RunCommand`/`Check`/`Service` reuse the exact same mechanism: `id := s.next(name)` (`command.go:207-209`, `service.go:46-48`), so a command's id is indistinguishable in shape from a child scope's or session's id (`lap.3/check.2`).
- `NewSession`/`Fork` call `scope.adopt(session)`, which does `session.id = s.next(session.name)` (`scope.go:119-125`, `session.go:57,630`).
- `Group.Go(name, fn)` computes `child := g.scope.child(name)` **synchronously**, in the caller's goroutine, before it spawns the child's goroutine with `g.wg.Go(...)` (`group.go:63-85`, specifically line 67 before line 68). So node-naming order for concurrent children is fixed by **source-visible call order**, not by goroutine scheduling — this is deterministic across two runs of the same source with the same inputs.
- `Iterate` (`iterate.go:12-40`) and `PromiseLoop.Tasks` (`loop.go:88-196`) each build and run their child scope **sequentially** in the calling goroutine, so their ordinals are trivially deterministic given the same `items`/planner decisions.

Net: **scope/session/command naming is deterministic** given (a) the same
source and (b) the same sequence of activity results (planner decisions,
task counts). The mechanism most people worry about (map iteration
order) is not actually in play here because ordinals are assigned at the
call site, synchronously, in source order.

### 1.3 Where nondeterminism actually lives

| Source | Location | Affects |
|---|---|---|
| `ulid.Make()` for the run id | `run.go:160` | Directory name only, not control flow |
| `ulid.Make()` for interview question ids | `run.go:319` | Id string only |
| `crypto/rand` for Claude/Pi/conversation-manager ids | `claude/claude.go:115`, `internal/pi/history/session_manager.go:723,747,1359`, `internal/conversation/manager.go:561` | Native session id strings only |
| `w.seq++` per-run event sequence | `event_persistence.go:46-53,71-74` | **Real interleaving** of concurrent scopes' records in `run.jsonl`; two runs of identical source with identical activity results can still produce different `run.jsonl` orderings across independent `Group` children, because this counter increments in real call order under a mutex, which depends on goroutine scheduling |
| Supervisor timer tick (`time.NewTicker(...every)`) | `supervise.go:138`, `supervise_jev.go:235` | *When* a supervisor's look happens relative to the worker's turn — genuinely wall-clock/real-time-dependent, and it can inject a `Steer` mid-turn |
| Jev screening (`jev.Client.Batch(...).Run`) | `supervise_jev.go:134-156` | Whether/when an automatic review-and-steer fires; this is an external network call with its own latency and a probability score, not deterministic replay data unless the score itself is durably recorded and treated as an activity result |
| `time.Now()` used directly in workflow bodies (not through an activity) | `internal/workflows/validateproduct/validateproduct.go:176,178,196,198,216,218` | Recorded `ElapsedSeconds`, i.e. workflow-body decisions computed from wall-clock reads outside any activity boundary |

The important structural point: **almost all of the nondeterminism is
already on the activity side** (agent turns, `RunCommand`, external
network calls) — the side Temporal expects to isolate in activities.
The exceptions above (the event-sequence counter, timer-driven
supervisor looks, `time.Now()` in a workflow body) are the ones that
would need explicit handling; see §2 and §4.

### 1.4 Primitive-by-primitive process-local footprint

| Primitive | Goroutines | Files/dirs under run | Processes | ctx cancellation |
|---|---|---|---|---|
| `Run` | none itself; owns the whole tree | `runs/<ulid>.<name>/{run.jsonl,artifacts/,sessions/,commands/,scopes/}`, `project.jsonl` beside project dir (`run.go:160-190`) | none | root `context.WithCancelCause` (`scope.go:157-159` via `scope.do`) |
| `Scope` | none | none directly | none | child `context.WithCancelCause` (`scope.go:157-159`) |
| `Group` | one goroutine per `.Go` call (`group.go:68`) | none directly | none | shared group ctx, `cancel(nil)` on first non-`Killed` child error (`group.go:81-83`) |
| `Iterate` | none (sequential) | none | none | reuses parent ctx per item scope |
| `PromiseLoop` | none extra beyond its own child scopes/tasks; writes `scopes/<key>/backlog.md` (`loop.go:110-114,149-151`) | `scopes/<loop key>/backlog.md` | none | loop scope ctx; task scope ctx per lap |
| `NewSession` | none until first turn | `sessions/<id>.jsonl` opened lazily (`event_persistence.go:154-171`) | adapter-dependent, lazily (see §3) | none owned; closed via scope end |
| `Generate` | 0 supervisors: none; N supervisors: 1 ticker goroutine each (`supervise.go:136-165`) or Jev's async check goroutines (`supervise_jev.go:114-119,233-293`) | none directly (session log write happens per turn) | none directly (delegates to adapter) | own `context.WithCancelCause` per turn, registered in `run.turns` for `CancelTurn` (`session.go:317-319`, `run.go:291-301,427-442`) |
| `Steer` | none | none | none | none |
| `Fork` | none | none | adapter-dependent | none |
| `Set`/`SetJSON` | none | may spill to `artifacts/values/...` when over budget (`scope.go:298-330`, `artifact.go:86-128`) | none | none |
| `RunCommand`/`Check` | none | `commands/<id>/{stdout,stderr}.log` (`command.go:85-92,206-238`) | one `os/exec` process (`command.go:241`) | `exec.CommandContext`, `cmd.Cancel` kills the process on ctx cancel (`command.go:245-249`) |
| `Service` | 2: `wait()` and `stopWhenCancelled()` (`service.go:97-98`) | `commands/<id>/{stdout,stderr}.log` | one `os/exec` process, own process group (`service_process_unix.go`) | SIGTERM→5s→SIGKILL→5s teardown on scope end or ctx cancel (`service.go:202-267`) |
| `Interview` | none (blocks on a channel) | none directly (recorded as lifecycle events) | none | `waitInterviewAnswer` selects on ctx.Done() too (`run.go:329-345`) |
| `WithSupervisor` | see `Generate` row | none directly | none | supervisor's own `Generate`/`dispatch` calls, cancelled with the worker's turn (`supervise.go:130-135`, `supervise_jev.go:229-230,300-305`) |
| `WithScopeTemplate` | none | none | none | none — pure `text/template` render (`session.go:97-126,134-144`) |

The live web instance is reached only through two narrow context hooks:
`live.FromContext`/`live.WithHook` (`internal/live/live.go:43-51`) gives
`Run` a `Controller` sink, and `observation.FromContext`
(referenced at `run.go:169`) gives it a `Store`. Neither is required —
`Run` works standalone with only `gimbal.Project` in ctx (`run.go:151-156`,
package doc `doc.go` "Called with only a Project context, Run executes in
the caller's process").

## 2. Generate internals and the activity-boundary question

One `Session.Generate[T]` call, without supervisors, does, in order:

1. `scopedPrompt` renders the visible scope context (default renderer or
   `WithScopeTemplate`) and appends it to the literal prompt constant
   (`session.go:87-126`, `scope.go:391-442`). This can synchronously write
   a content artifact under `artifacts/context/...` if the rendered text
   is too large (`scope.go:433-441`, `artifact.go:130-150`).
2. `dispatchRecorded` picks the no-supervisor path, `generate[T]`
   (`session.go:161-174,185-223`).
3. `generate[T]` loops up to `generateAttempts=3` times on an invalid
   result and up to `protocolAttempts=2` times on a specific Claude
   "unknown tool_use_id" string match (`session.go:182-184,203-213`) —
   **the re-ask text itself embeds the previous invalid answer**, so a
   retry is not idempotent input, it is a new prompt built from a prior
   activity's output.
4. `(*Session).turn` (`session.go:228-391`) lazily calls
   `adapter.CreateSession` on the first turn only (`session.go:248-256`),
   then always calls `adapter.RunTurn(turnCtx, native, prompt, schema,
   wrapped)` (`session.go:321`), where `wrapped` stamps every native event
   with a canonical session/message/event id (`session.go:443-507`) and
   writes it to the per-session log + the observation store
   (`event_persistence.go:154-187`).
5. The turn's ctx is independently cancellable by id through
   `run.turns` (`session.go:317-319`, `run.go:291-301,427-442`) — this is
   how an operator kills one turn without touching its scope or session.
6. `TurnStarted`/`TurnEnded` lifecycle records are written before/after
   (`session.go:264-271,353-390`), and usage is accumulated per step and
   republished as `session.usage.updated` synthetic events
   (`session.go:425-433`).

With supervisors attached, `supervise[T]` (`supervise.go:108-123`) instead:
spins one goroutine per supervisor that either ticks on `WithInterval`
(default 3 min, `supervise.go:79-81,138`) or, when `TYPESAFE_API_KEY` is
set, is driven by Jev screening of completed `session.reasoning.ended`
items (`supervise_jev.go:80-120,219-307`). Any objection is delivered by
calling `s.Steer` on the worker session from that goroutine
(`supervise.go:161`, `supervise_jev.go:208`). `Generate` returns only after
every supervisor goroutine is joined (`supervise.go:131-135`,
`supervise_jev.go:300-306`).

**Is "one Generate call including its supervisors" a clean activity
boundary?** Partly. The worker's own turn (steps 1-4, 6 above) is a clean
request/response unit: given (rendered prompt, schema, session identity,
workdir, model binding) it returns (validated JSON or error, usage). That
part maps cleanly onto a Temporal activity: `RunTurn` is already
request/response over `context.Context`, already cancellable, already
retried by hand inside `generate[T]` (which an activity/workflow split
would probably want to replace with Temporal's own retry policy instead
of the hand-rolled `generateAttempts`/`protocolAttempts` loops).

The supervisor path is not clean, because it is not request/response: it
is **a live side channel that must run concurrently with the still-open
worker activity and can push a message into it mid-flight**
(`Session.Steer`, `session.go:552-596`, which requires `s.running`,
`s.native`, `s.turnID` — all live, in-memory, adapter-connection state).
Temporal activities do not talk to each other directly; the only
sanctioned channels are activity heartbeats (workflow→activity is not a
thing) or a workflow issuing a second activity/signal. Projecting
"supervisor objects, steer lands inside the running turn" onto Temporal
means the *worker's RunTurn* activity would need to expose some kind of
external mutable channel (e.g. the workflow polls a queue and calls a
small "steer" activity that reaches back into the worker's still-running
activity via the same shared adapter process) — which only works if the
worker activity and the steer activity are pinned to the same host/worker
process holding the live adapter connection. This is the single biggest
mismatch found in this audit between Gimbal's execution model and
Temporal's, because it is concurrent-and-mutable, not the more usual
"long activity + heartbeat" shape.

Minimum inputs/outputs an activity boundary for the worker's own turn
would need: rendered prompt (string), JSON schema (raw), session identity
(role/native-id/workdir), model binding (adapter kind + model + effort),
and — because supervisors must be able to reach the same live turn —
either co-location with the worker's activity or an explicit "turn
handle" the orchestrator can address (mirroring what `run.turns` and
`Session.activeEmit` already do in-process, `session.go:244-246,294-296`).

## 3. Session lifecycle per adapter

| Adapter | Between-turn state | Resume mechanism | Cross-host resumable? | Fork |
|---|---|---|---|---|
| `codex/` | One shared **machine-level daemon** (`codex app-server`) reached over one shared WebSocket connection per adapter instance (`codex/codex.go:45-58,89-114`); a "thread" is server-side state, not process-per-turn | `thread/resume` re-subscribes an existing thread id; done automatically on redial (`codex/codex.go:85-147`) | **No** — the thread lives in one daemon process on one host; comment explicitly frames this as "the machine's one shared app-server daemon" (`codex/codex.go:2-9`); `Close` archives via `thread/archive` and an archived thread cannot be reused even via unarchive on codex-cli 0.153.4 (`codex/codex.go:149-161,413-455`) |
| `claude/` | **None** between turns — "Each turn launches Claude Code against the session id; the conversation is Claude Code's to keep" (`claude/claude.go:1-3`); `RunTurn` starts a fresh CLI process every call (`claude/claude.go:183-196`) | `--resume <id>` or `--fork-session <parent>` flags passed to a new process each turn (`claude/claude.go:171,176`) | **[inference]** depends on where the Claude Code CLI itself persists transcripts (not vendored in this checkout to verify); the adapter only mints a random UUID (`claude/claude.go:113-125`) and trusts the CLI to resume by it, so cross-host resumption requires that CLI-owned transcript store to be visible on the resuming host |
| `pi/` | **In-process** native session object, no subprocess at all ("Package pi implements Gimbal's HarnessAdapter over the native, in-process Pi session port... no subprocess is started", `pi/adapter.go:1-3`); session history lives under a workdir-relative directory via `history.Create(workdir, sessionDir, nil)` (`pi/adapter.go:125`) | Native `Session` object kept live in the adapter's map (`pi/adapter.go:37-42,60-68`) | **No** for the live object; the on-disk history directory under `workdir` could in principle be read elsewhere, but the live `*session.Session` (with its open state) cannot move processes |
| `opencode/` | One shared **local OpenCode server** per state dir, discovered/locked with a lock file (`opencode/server.go:20-25`, `opencode/process_unix.go:26` `unix.Flock`); adapter keeps a `*Client` per adapter-session pointing at it (`opencode/adapter.go:37-119`) | `client.CreateSession`/`client.Fork` against the shared server; a redial reconnects to the same server via its discovery file | **No** — server is a single local process reached over a local address, guarded by a `flock`-style lock file, i.e. host-pinned by construction |
| `agy/` | **None** between turns — each turn (or resumed turn) is its own `exec.Command` invocation (`agy/agy.go:56-57,262-264`); the CLI is told which conversation to resume by id | `-p <prompt> ... --add-dir <workdir>` plus resumed conversation id (`agy/agy.go:112-146,238-264`); **`Fork` is unsupported**: "agy does not expose /fork in print mode or provide another headless fork transport" and it returns an error (`agy/agy.go:168-175`) | **[inference]** same caveat as Claude: depends on where `agy`'s own conversation store lives; the workdir is passed explicitly via `--add-dir` each turn |

Two forks in one workdir cannot run turns at once — this is stated as a
design intent in `ephemeral/research/api/API.md:111-112` ("Two forks in
the same workdir cannot run turns at once") but is **not enforced by any
lock in the current code**: nothing in `scope.go`, `session.go`, or any
adapter takes a workdir-level mutex or file lock across sessions.
`[inference]` the constraint is really about the *native harness/git
tooling* not tolerating two live agents editing the same tree
concurrently (e.g. lock files, index contention), not something Gimbal
polices. For placement onto remote workers, this means: **nothing today
stops a Temporal-driven scheduler from placing two forked sessions of one
workdir on two different workers**, which would actually be worse than
today's single-host behavior (two hosts touching the same working tree
over some shared filesystem) unless the projection explicitly serializes
same-workdir sessions onto one worker/session affinity, which none of the
current code does for it automatically.

The concrete daemons/servers above (`codex app-server`, the OpenCode
server) and the in-process Pi session are **host-pinned resources by
construction** — a Temporal worker pool would need "one Kubernetes pod =
one persistent local daemon + one checked-out workdir", with session
affinity keyed to (workdir, harness) pairs, not stateless worker pods.

## 4. Workflow-body side effects (outside `Generate`)

Counting only statements in the workflow's own body — not inside
`Generate`/`RunCommand`/`Check` calls themselves, which are already the
observed activity boundary.

| Workflow | os/exec/time/rand calls in body | go/defer/select/type-switch/sync | Classification |
|---|---|---|---|
| `review/review.go` (26 lines of body) | **0** | none | — |
| `implementation/implementation.go` | `os.ReadFile` line 72 (reads the outcomes JSON before any scope exists); `filepath.Join`/`Abs`/`IsAbs` in `absoluteFrom` lines 158-166 (pure, no-op since `env.WorkDir` is already absolute) | none | `os.ReadFile` at line 72 is **(c) genuinely hard**: it is how the workflow receives dynamic input, done by direct disk read inside the body rather than as a typed parameter — the natural Temporal-shaped fix is to parse the outcomes file at the CLI/route layer (already true for every other stock workflow's `Params`) and pass `[]string` in, not read it in-body |
| `researchdocument/researchdocument.go` | `os.Executable()` line 143; `os.MkdirAll` ×6 (lines 147,150,186,189,348,351); `filepath.WalkDir` in `verifyResearchFloor` line 407, called from inside 5 `Group` children (lines 211,230,249,268,287) and from the editorial round (line 369); `os.Stat` via `requireNonemptyFile` called at lines 304,314,382 | `gimbal.Group` with 5 real `.Go` children (lines 197-296); no raw `go`/`defer`/`select`/`sync`/type-switch | `os.Executable()` is **(c) hard**: it resolves *this host's* running binary path and bakes it into scope context as `"token counter executable"` (line 159) for later `RunCommand` calls — on a remote worker the equivalent binary must be resolved on that worker, not carried from the orchestrator. `MkdirAll` is **(a)** trivially movable into a command activity (`mkdir -p`). `WalkDir`/`Stat` reading agent-produced files directly are **(c) hard**: they inspect live filesystem state the worker turn just wrote, so they must run wherever that workdir actually is, i.e. become their own activity (or be redone as `Check`) rather than a raw workflow-body syscall |
| `pyramidsummary/pyramidsummary.go` | `os.Executable()` line 124; `os.MkdirAll` line 121; `os.Stat` via `requireNonemptyFile` called at lines 115,118,178,193,208,223,238,253,282 (9 sites) | `gimbal.Group` twice, 6 children each (lines 164-257, 320-405); fixed-size arrays `var slots [6]levelSpec` (line 160), no raw `go`/`defer`/`select`/`sync`/type-switch | Same classification as researchdocument: `os.Executable()` **(c)**, `MkdirAll` **(a)**, `os.Stat` reads of agent output **(c)** |
| `validateproduct/validateproduct.go` | `exec.LookPath` ×2 (lines 86,90, resolving `playwright-cli` and `ffmpeg` on **this** host); `os.MkdirAll` ×2 (94, and once per workload inside the loop at 145); `os.MkdirTemp` line 97 (nondeterministic directory name baked directly into workflow state); `context.WithTimeout` lines 101,154 (real wall-clock deadlines set directly in the body); `exec.CommandContext` called **directly, bypassing `gimbal.RunCommand` entirely**, at line 122 (browser teardown) and line 232 (ffmpeg video conversion); `os.WriteFile` ×6 (130,184,204,224,252,258); `os.Rename` in `writeJSON` line 270; `time.Now()`/`time.Since()` ×3 pairs (176/178, 196/198, 216/218) | 2 `defer` closures over shared state (lines 108, 135); `gimbal.Group` with 3 real children (170-227) closing over shared arrays `reports`, `turns [3]error`, `opened`/`recording [3]bool` (each goroutine only touches its own index, so this is not a data race, but it is unsynchronized shared mutable state that a naive Temporal translation could not leave as real goroutines + shared arrays) | The two raw `exec.CommandContext` calls (122, 232) are the single most consequential finding here: they run real processes (browser teardown, ffmpeg) **entirely outside the run log / graph / page** — invisible to `internal/generate`'s static reader, invisible to `internal/observation`, and therefore invisible to whatever a Temporal projection would otherwise treat as "the set of activities". `exec.LookPath` and `os.MkdirTemp` are **(c) hard**: both are genuinely host-specific (PATH contents, temp-dir entropy) and would need to become activity results, not in-body computation. `time.Now()` for elapsed measurement is the textbook case Temporal solves with `workflow.Now()`; used raw here it is **(c)** |

Totals: review 0, implementation 1 hard OS call, researchdocument ~8 hard
+ 6 movable, pyramidsummary ~10 hard + 1 movable, validateproduct ~9 hard
(2 of which are wholly unobserved subprocess launches) + several movable.
No stock workflow uses a raw `select`, `type switch`, or a bare `go`
statement in its body — every concurrency construct routes through
`gimbal.Group`, which `gimballint`'s GIMBAL105 rule enforces
(`internal/gimballint/rules.md:101-119`). `validateproduct.go` is the only
one with `defer` in the workflow body, and the only one whose concurrent
children read/write shared plain-Go state (arrays) rather than only their
own scope.

## 5. The static reader (`internal/generate`) and `workflow.Graph`

### 5.1 What it reads

`internal/generate` loads the workflow package with `go/packages` +
`go/types`, finds the entry function (`internal/generate/entry.go:33-52`
requires signature `(ctx, gimbal.Env, <=1 params struct)`), and walks its
body statement-by-statement (`internal/generate/stmt.go:12-56`). The
switch in `internal/generate/expr.go` recognizes exactly these call
names as shape: `NewSession` (133), `Fork` (146), `Generate` (165),
`Interview` (185), `RunCommand` (202), `Service` (211), `Check` (220),
`Set`/`SetJSON` (229), `Group` (248), `PromiseLoop` (284), plus
`WithSupervisor` as an option (383). `Iterate` is recognized as a special
range shape (`internal/generate/read.go:48`). **`Steer` and
`WithScopeTemplate` are not read at all** — they never appear in either
switch, so a workflow that calls them gets no diagnostic and no graph
entry; they are purely runtime behavior invisible to static analysis.

`ast.IfStmt`/`ast.SwitchStmt` become `workflow.Condition`/`Branch`
(`internal/generate/stmt.go:150-201,240-272`) but only the **branch
shape** (case text as written, whether it exits) is captured — the
**condition's boolean value at runtime is never in the graph**. A `for`/
`range` loop that holds an operation becomes `workflow.Repeat` with
`Cond` as the literal loop header text (`internal/generate/stmt.go:313-320`,
`workflow/graph.go:153-160`) — "how many times it runs is the run's
record", per the doc comment on `Repeat`.

Unread constructs are recorded as diagnostics, never guessed:
`ast.TypeSwitchStmt`, `ast.SelectStmt`, `ast.GoStmt`, `ast.DeferStmt`
(`internal/generate/stmt.go:46-53`), each only reported when it actually
holds a Gimbal call (`internal/generate/stmt.go:59-65`). A raw goroutine
holding a `Set`/`SetJSON`/`Check` is separately caught earlier as
GIMBAL105 by `gimballint`, not by the graph extractor.

### 5.2 What `workflow.Graph` captures

`workflow/graph.go:21-217` is a sealed union of source-order
`Operation`s: `Session`, `AgentCall` (prompt is the constant, not the
rendered scope text — "the scope's context the run appends to it is the
run's record", `workflow/graph.go:69-71`), `Interview`, `Command`, `Set`,
`Scope`, `PromiseLoop`, `Iterate`, `Repeat`, `Group`, `Condition`. Its own
doc comment is explicit about the boundary: "It is not a model of the
program. Ordinary Go calculation and error handling are absent... nothing
the runtime already records belongs here" (`workflow/graph.go:5-14`).

**Is the graph sufficient to drive an interpreter? No**, and the code
does not claim it is. Missing, by design:
- Condition branch *choice* at runtime (only shape, not which branch runs).
- Loop trip counts for `Repeat` and item counts for `Iterate` (runtime data).
- Any data flow: which value a `Set` key holds, what an `AgentCall`
  actually returned, what a `PromiseLoop` planner actually decided beyond
  "a task ran" — all "the run's record" (the JSONL logs / observation
  tables), never the graph.
- Anything reached only via `Steer`, `WithScopeTemplate`, or a helper the
  extractor could not follow (GIMBAL101's exact boundary,
  `internal/gimballint/rules.md:8-31`).

So a projection built purely from `workflow.Graph` would need to re-derive
control flow and data flow from somewhere else — realistically, from
re-running the actual Go source as the orchestrating logic (with activity
calls substituted underneath `HarnessAdapter`/command execution), not
from interpreting the graph as a program. The graph is a **shape
descriptor for the page and the lint tool**, not an executable IR.

### 5.3 `gimballint` (GIMBAL10x) rules

| Rule | What it forbids |
|---|---|
| GIMBAL101 | A worker callable selected from a collection or passed as an opaque function value; only direct, source-visible calls (including package-local helpers) are followed |
| GIMBAL102 | A `Set`/`SetJSON`/`Check` key that is not a compile-time string constant |
| GIMBAL103 | A second `Set`/`SetJSON`/`Check` on the same constant key in the same scope context, including via a context captured outside a loop/`Iterate`/`PromiseLoop.Tasks` |
| GIMBAL104 | Writing through a different context than the one a `Scope`/`Group.Go`/`Iterate`/`PromiseLoop.Tasks` callback received |
| GIMBAL105 | `Set`/`SetJSON`/`Check` inside a raw `go` statement instead of a named `Group` child |
| GIMBAL106 | Writing the reserved key `"task"` through a `PromiseLoop.Tasks` yielded context |
| GIMBAL107 | `Set`/`SetJSON`/`Check` called with `context.Background()`/`context.TODO()` directly |
| GIMBAL108 | A `Generate`/`WithSupervisor` prompt/instruction argument that is not a compile-time constant expression |
| GIMBAL109 | A `WithScopeTemplate` argument that is not a constant or a `//go:embed` variable |

(`internal/gimballint/rules.md:8-211`, one exemption: `cmd/gimbal/run_prompt.go`
is allowed a non-constant prompt for GIMBAL108, `rules.md:172-174`.) These
rules already push workflow authors toward exactly the shape a
Temporal-style projector would want (named, source-visible, constant
prompts/keys/instructions) — they are a good foundation, but they say
nothing about `RunCommand`'s or `Service`'s command strings, nothing about
raw `os`/`exec`/`time` calls in the body (§4's findings are entirely
outside gimballint's current coverage), and nothing about `Steer` or
`WithScopeTemplate` at all.

## 6. Observation/record: what the page needs, and what a remote worker must ship back

One run directory (`runs/<ulid>.<name>/`) is fully self-contained
(`internal/observation/doc.go:10-14`: "a finished run's directory is
self-contained and greppable with jq"):

- `run.jsonl` — the one authoritative lifecycle log, one writer for the
  run's life, gap-free `Seq` (`event_persistence.go:41-53`,
  `LifecycleRecord.Seq` doc at `events.go:265-270`).
- `sessions/<session id>.jsonl` — one `AgentRecord` per native harness
  event, opened lazily per session (`event_persistence.go:154-171`).
- `commands/<command id>/{stdout,stderr}.log` — full captured streams for
  every `RunCommand`/`Check`/`Service` invocation (`command.go:85-92`).
- `artifacts/...` — spilled scope values, content artifacts (rendered
  scope text, planner backlog, etc.) written content-addressed under
  `context/` or path-addressed under `values/` (`artifact.go:86-150`).
- Eight roll-up tables written as flat JSON files beside the log —
  `run.json`, `scopes.json`, `sessions.json`, `interviews.json`,
  `turns.json`, `turn_usage.json`, `model_calls.json`, `commands.json`
  (`internal/observation/files.go:15-27,102-123`) — rewritten atomically
  (temp file + rename, `internal/observation/files.go:137-161`) every time
  they change, so **a directory holding only the two JSONL logs can be
  fully reconstructed** by replay (`internal/observation/replay.go:20-79`,
  the fallback path when any of the eight table files is missing).
- `project.jsonl` beside the project directory, one per concurrently
  active run, `Seq` always 0 there, ordered by `Time` instead
  (`event_persistence.go:41-53`, `events.go:265-270`).

The live page gets updates over Server-Sent Events
(`internal/observation/http.go:87-91`, `Content-Type: text/event-stream`),
fed by `Store.Join`'s bounded delta subscription
(`internal/observation/subscribe.go:23-100`, bounded to 512 frames / 4 MiB
per subscriber, `internal/observation/subscribe.go:13-16`). The
`live.Controller` interface (`internal/live/live.go:18-33`) is the only
way the page can act back on a run: `AnswerInterview`, `Steer`,
`SteerLoop`, `CancelScope`, `CancelTurn` — all keyed by the exact same
scope-key/session-id/turn-id strings §1.2 describes.

**What a remote worker would have to ship back for the page to keep
working unchanged**: exactly the same `AgentEvent` stream per turn (fed
through the same `onEvent`/`wrapped` stamping pipeline,
`session.go:274-391`) and the same `CommandStarted`/`CommandEnded`
lifecycle pair per command, tagged with the same scope/session/turn id
strings the orchestrator's control flow already computed. Nothing about
the store or the page needs to change **if** the orchestrator process
(the one actually executing the Go workflow body, whether that is
literally `gimbal.Run` or a Temporal-workflow-side equivalent) remains the
one place that owns `*run`/`*scope` and therefore the one place that calls
`r.event`/`r.sessionEvent`/`r.eventResult`. A worker executing a turn or a
command remotely only needs to get its output (events, exit code,
stdout/stderr, usage) back to that orchestrator — as an activity result,
or as heartbeat-carried partial events for the live-streaming case, since
`onEvent` fires per native event today, not once at the end
(`session.go:275-293`, `command.go` streams to files "while it is
running"). Long `RunTurn`/`RunCommand`/`Service` calls without any partial
reporting mechanism would otherwise silently break live streaming even if
the final result eventually arrives correctly.

## 7. The hosted path and where a Temporal switch would go

`gimbal run <workflow>` is a generated Cobra command
(`internal/generate/command.go:92-149`, the `clientCommandTemplate`). It
never runs the workflow itself. It builds a typed `Start<Entry>Input`,
finds the **already-running** instance via
`web.SelectedFormClient(instanceDir, project)`
(`internal/generate/command.go:141`), and calls the generated SKGO Form
remote over that connection. `follow` optionally then polls
`web.Follow` for the terminal run row (`internal/generate/command.go:145`).

Instance discovery is a Unix-domain-socket rendezvous: each running
instance writes `<instance-dir>/control/<control-id>.json` with
`{pid, socket, project}` (`internal/host/host.go:165-183`); the CLI reads
every file in that directory, probes `/control/live` over the socket, and
requires **exactly one** live match (`web/submit.go:72-113`). The Form
client then dials that Unix socket directly
(`web/submit.go:57-66`).

Server-side, the generated remote (`web/src/routes/review_start.remote.go:24-36`
for `review`) calls the shared `startWorkflow` helper
(`web/src/routes/workflow_start.go:25-79`), which: validates paths, binds
models via `internal/binding`, calls `host.OwnerFrom(ctx)` to get the
`*host.Owner`, admits/looks up the project (`owner.AdmitProject`,
`internal/host/host.go:70-130` — this takes an exclusive `flock` on
`<project>/.gimbal/owner.lock`, one instance per project, ever), and
finally calls `project.Start(name, workDir, conversation, models, body)`
(`web/src/routes/workflow_start.go:67`).

`Project.Start` (`internal/host/host.go:253-297`) launches
`p.Run(ctx, name, models, body)` **in a goroutine inside the same
server process**, waits only long enough to learn the run id, and returns
immediately — the workflow body keeps running after the HTTP request that
started it completes. `Project.Run` (`internal/host/host.go:201-244`) is a
thin wrapper that puts the registry/runs table in ctx and calls
`gimbal.Run(gimbal.Project(runCtx, p.dir), name, models, body)` directly
— **`body` here is the literal generated closure that calls the
package-level workflow function** (e.g. `review.Review(...)`,
`web/src/routes/review_start.remote.go:31-35`).

**There is no submission boundary today** — "submit" and "execute" are
the same call, in the same process, on the machine that answered the
control socket. The natural point for a "submit to Temporal instead"
switch, with the least code, is exactly `Project.Run`
(`internal/host/host.go:201-244`): today it does
`gimbal.Run(gimbal.Project(runCtx, p.dir), name, models, body)`; a
Temporal-backed variant would instead start a Temporal workflow execution
whose workflow function performs the equivalent of the body — either (a)
the literal same Go function, if and only if its use of `Group`/
supervisors/timers can be made to satisfy Temporal's determinism
constraints (see §1.3/§2's finding that real goroutines + real mutexes are
not directly portable to Temporal's cooperative-coroutine model), or (b) a
generated Temporal workflow driven by `workflow.Graph` plus a real
Go interpreter loop, which §5.2 shows the graph alone cannot support
without also carrying data flow. Either way, `host.Project`'s other
responsibilities — project admission/locking, the `observation.Registry`,
the `live.Runs` table, conversation association — are orthogonal to where
the workflow body actually executes and would not need to change.

## Facts that constrain the design

1. `HarnessAdapter` is the only pluggable execution backend; `RunCommand`/`Check`/`Service` are hardwired to local `os/exec` with no interface at all (`command.go:241`, `service.go:72`).
2. Scope/session/command identity (`lap.3/check.2`-style keys) is deterministic given the same source and the same activity results, because ordinals are assigned synchronously at the call site (`scope.go:104-116`, `group.go:63-85`), not by goroutine scheduling.
3. The per-run event `Seq` counter (`event_persistence.go:46-53`) is a real-time-ordered counter across concurrently running `Group` children; two runs with identical activity results are not guaranteed to produce byte-identical `run.jsonl` ordering across independent branches.
4. `*run`/`*scope` are unexported, hold live mutexes/goroutines, and are threaded through `context.Context` as raw pointers (`scope.go:27,32-51`) — they cannot be serialized across a process boundary; only `HarnessAdapter` calls can.
5. `Session.Steer` requires live, in-memory state (`s.running`, `s.native`, `s.turnID`, `s.activeEmit`, `session.go:552-596`) — supervision is a concurrent side channel into a running turn, not a request/response call, and does not map onto a plain Temporal activity boundary without co-location of the "steer" path and the worker's turn.
6. Three of five adapters (`codex`, `opencode`, `pi`) depend on a host-pinned local daemon/server or in-process object that a session cannot be moved off of (`codex/codex.go:2-9`, `opencode/server.go`, `pi/adapter.go:1-3`); the other two (`claude`, `agy`) spawn a fresh process per turn but rely on the harness's own transcript store being reachable on whichever host resumes the session.
7. `agy.Fork` is not supported at all (`agy/agy.go:168-175`).
8. Nothing in the current code enforces "two forks of one workdir cannot run turns concurrently" (documented intent only, `ephemeral/research/api/API.md:111-112`); a naive Temporal scheduler could violate it by placing forked sessions on different workers.
9. `workflow.Graph` deliberately excludes runtime data and branch/loop decisions (`workflow/graph.go:5-14`); it cannot drive an interpreter by itself (§5.2).
10. `Steer` and `WithScopeTemplate` are entirely invisible to the static reader (`internal/generate/expr.go`, `read.go`) — no graph node, no gimballint rule references them.
11. Every stock workflow's OS-touching code in the *body* (outside `Generate`/`RunCommand`) reads or writes the local filesystem directly at least once (`os.ReadFile`, `os.Executable`, `os.MkdirAll`, `os.Stat`, `filepath.WalkDir`, `os.MkdirTemp`) — none of it currently goes through any recorded/observed boundary.
12. `validateproduct.go` calls `exec.CommandContext` directly, twice, entirely outside `gimbal.RunCommand` (`validateproduct.go:122,232`) — those subprocess runs are invisible to the run log, the graph, and the page today, independent of any Temporal question.
13. `os.Executable()` is used in two stock workflows to resolve a token-counting helper binary and is baked into scope context as a path string (`researchdocument.go:143-146,159`, `pyramidsummary.go:124-127,148`) — this path is meaningless on a different host.
14. Supervisor timing (`WithInterval`, default 3 minutes) and Jev screening are wall-clock/network-driven, not deterministic (`supervise.go:79-81,138`, `supervise_jev.go:134-156`).
15. The generated CLI (`gimbal run ...`) already treats "submit" and "execute" as remote-procedure-call-shaped (Unix socket, discovery file, typed Form input, `web/submit.go`), even though today both ends are the same process — this shape is reusable for a Temporal submission path with minimal change at `internal/host/host.go:201-244` (`Project.Run`).
16. `Project.Start` returns as soon as the run id is known and the workflow body keeps running detached from the originating request (`internal/host/host.go:253-297`) — the hosted path already assumes fire-and-forget/long-running semantics compatible with Temporal's model.
17. Eight flat JSON roll-up tables plus two JSONL logs plus per-session JSONL plus per-command stdout/stderr files are the complete durable contract the page depends on (`internal/observation/files.go:15-27`); anything a remote worker produces must arrive at the orchestrator in a form that can still populate exactly these files, because the page's only knowledge of a run is this file set (or its replay, `internal/observation/replay.go:20-79`).
18. Live streaming to the page is per-native-event, not per-final-result (`session.go:275-293`); a remote activity model that only returns a final result at the end of a long turn would silently break live updates unless it also streams partial events back (e.g. via heartbeats).
19. `gimballint`'s GIMBAL10x rules already push workflows toward source-visible, constant-keyed, constant-prompted code (`internal/gimballint/rules.md`), which is a good precondition for any generated projection, but they cover none of the raw OS/time/exec calls found in §4.
20. There is exactly one admission lock per project (`owner.lock` flock, `internal/host/host.go:90-100`) and exactly one instance is expected to answer discovery for it (`web/submit.go:109-112`) — any remote-execution design has to decide whether that single-owner-per-project invariant still holds when workers, not just the orchestrator, touch the project's checkout.

## Questions only Tyler can answer

1. Is a `CommandRunner`/similar interface (parallel to `HarnessAdapter`) something you're willing to name and add, or should `RunCommand`/`Check`/`Service` stay local-only and a projected workflow instead run *all* control flow (including these calls) on a worker colocated with the checkout?
2. For the supervisor side channel (§2, §8.5): is it acceptable for supervision to require worker/activity co-location (steer and worker turn pinned to the same process), or does supervision need to be redesigned around Temporal signals/queries instead of live `Steer`?
3. Should a Temporal projection literally run the workflow's Go source as the Temporal workflow function (accepting the `Group`→real-goroutines mismatch and rewriting `Group` to use `workflow.Go`/`workflow.Selector` under a build tag or generated variant), or should it always go through code generation from `workflow.Graph` plus a second, richer extraction pass that also captures data flow and branch decisions?
4. Given `os.Executable()`/`exec.LookPath`/raw `exec.CommandContext` calls already exist in shipped stock workflows (§4), do those count as bugs to fix under "no code" review before any projection work starts, or as separate, allowed technical debt this research should just document?
5. Is per-workdir/per-session-affinity scheduling (one Kubernetes pod holding one persistent local daemon and one checked-out repo) an acceptable permanent constraint for codex/opencode/pi-backed roles, or is a stateless-worker model actually required, which would mean each of those three adapters needs a materially different remote-attach design?
6. Should `agy`'s missing `Fork` support block it from any role that a projected workflow forks, or is that an existing limitation orthogonal to this work?
7. Does the "two forks in one workdir" constraint (currently unenforced, §3) need to become an actual lock in `gimbal` itself before remote scheduling is safe, or is the answer to make workdir placement a scheduling policy that a Temporal-side "worktree" tactic enforces by construction (one worktree per session, never shared)?
