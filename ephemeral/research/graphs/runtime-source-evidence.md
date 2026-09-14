# Gimble graph/runtime evidence

This is a line-numbered source map for the graph feasibility report. Line
numbers are from the checked-out revision on 2026-09-14 and were produced with
`nl -ba`; they are citations, not copied source.

## Runtime identity and containment

- `scope.go:29-43` defines one runtime scope instance. It stores its parent,
  an ordinal path key, child/session ordinal counters, values, owned sessions,
  and ended state.
- `scope.go:52-65` increments one ordinal per child name and joins it into a
  slash path (`name.1`); this is the source of runtime instance identity.
- `scope.go:67-74` assigns a session ID below the creating scope and records
  ownership in that scope. The ordinal counter is shared by child scopes and
  session names.
- `scope.go:76-89` emits `ScopeBegan` and `ScopeEnded` around the body and
  creates the scope context. `scope.go:92-125` removes the scope before
  closing owned sessions, records `SessionClosed`, and cancels the context.
- `scope.go:132-137` makes a named child scope. `scope.go:140-182` makes
  `Set`/`SetJSON` values scope-local, one-write-per-key, and emits `ValueSet`;
  duplicate, ended-scope, and no-scope use panic.
- `scope.go:184-201` resolves `ScopeText` by walking the parent chain and
  taking the nearest value for each key. Reads have no lifecycle event.
- `session.go:14-35` separates the Gimble session ID/owner from its native
  harness session, turns, and running state. `session.go:37-47` emits
  `SessionCreated` when adopting into the current scope.
- `session.go:111-151` gives each turn an ordinal ID and records the scope in
  which it started. `session.go:189-203` registers a running turn for targeted
  cancellation and removes it when the adapter returns.
- `group.go:19-43` creates a named concurrent scope. `group.go:58-84` makes
  each `Go(name, fn)` a child scope and cancels siblings for ordinary errors;
  `group.go:87-98` waits for every child and ends the group scope.
- `loop.go:82-164` creates the loop scope, emits planner decisions, and makes
  each selected task a `task` child scope carrying structured task data. A
  killed task is recorded as failed feedback while the loop may continue.

## Lifecycle relations

- `events.go:56-85` defines scope and value lifecycle events.
- `events.go:87-105` defines session creation/closure; `SessionCreated.Parent`
  is the persisted fork relation.
- `events.go:107-137` defines turn start/end and `SuperviseAttached`, including
  reviewer session, worker turn, instruction, and interval.
- `events.go:139-161` defines `Steer` (target/source/message/landed) and
  `Killed` (target/by/reason), which are non-containment control relations.
- `events.go:174-181` defines durable `Complete`; absence means the run log
  did not finish durably.
- `run.go:98-172` creates a ULID-plus-name run directory and writes lifecycle
  records; `run.go:215-242` maintains live scope and turn indexes.
- `run.go:248-305` resolves person steering by session owner scope and
  cancellation by full scope key or turn ID. Scope cancellation affects all
  descendants; turn cancellation affects one turn while retaining scope and
  session.
- `supervise.go:75-87` emits the attachment relation before work starts;
  `supervise.go:89-129` runs reviewer looks concurrently, lets nested
  `Generate` happen, and steers objections to the worker. An attachment is not
  a completion gate.

## Persistence and current graph loss

- `events.go:183-206` puts sequence, timestamp, scope, session, and turn on
  lifecycle/agent records. Placement is sufficient to reconstruct containment
  and turn execution location.
- `internal/observation/rows.go:50-93` stores scope instances, session owner
  scope plus fork parent, and turn execution scope separately. This supports
  owner-versus-execution edges.
- `internal/observation/store.go:140-182` decodes a flat lifecycle record.
- `internal/observation/store.go:203-280` reduces run, scope, value, planner,
  session-created, and turn events into tables. The switch has no cases for
  `session_closed`, `supervise_attached`, `steer`, `killed`, or `complete`, so
  those relations remain in `run.jsonl` but are absent from `RunSnapshot`.
- `internal/observation/store.go:289-365` writes changed rows and publishes
  only the six table row kinds; `internal/observation/store.go:368-420` emits
  native transcript events with placement sidecars.
- `internal/observation/snapshot.go:9-23` defines run statuses and the single
  ended scope status. A successful scope is intentionally `ended`, and native
  events cannot decide workflow status.
- `internal/observation/store.go:44-74` shows the current store is six tables,
  transcripts, model-call facts, and subscriptions; no graph/edge table is
  present.
- `internal/observation/http.go:17-27` exposes only snapshot and SSE routes.
  `internal/observation/http.go:48-120` sends one complete snapshot followed
  by ordered row/totals/event frames, with no cursor or graph-specific frame.

## Current UI and native projection boundary

- `docs/web-app.md:80-100` names the scope-key graph spine and records the
  current status: F3 graph is still a flat list, F10 edges and F13 static
  template are designed/later, and F11 usage timeline is in progress.
- `docs/web-app.md:146-159` prioritizes the first pass around operator/replay
  views and deliberately keeps workflow editing and multi-project out of the
  page.
- `web/src/routes/runs/[runID]/page.server.go:29-51` serializes one Go
  `RunSnapshot` for SSR. `web/src/routes/runs/[runID]/+page.svelte:1-11`
  passes it directly to `RunViewer`.
- `web/src/lib/observation/index.ts:10-51` mirrors the six tables and
  transcripts. `web/src/lib/observation/index.ts:53-203` applies snapshot,
  row, totals, and native event frames; no graph nodes or edges are modeled.
- `web/src/lib/observation/RunViewer.svelte:13-30` sorts scopes and turns from
  table rows. `RunViewer.svelte:62-87` renders one section per scope and one
  invocation per turn, with no graph layout, edge overlay, or overlap view.
- `web/src/lib/observation/SessionTimeline.svelte:7-44` is a per-turn native
  message projection. It is a transcript timeline, not workflow topology.

## Workflow-owned external commands

- `internal/workflows/sprint/sprints.go:209-280` runs a worker, records result
  values, calls validation commands, asks a validator, runs repository checks,
  and commits. It explicitly records command summaries with `Set`.
- `internal/workflows/sprint/sprints.go:284-310` implements commands with
  ordinary `exec.CommandContext("sh", "-c", text).CombinedOutput`, returning
  exit code and output to the caller. There is no Gimble command lifecycle
  event, start timestamp, stderr split, or process node.
- `example_shapes_test.go:357-391` demonstrates the same workflow-owned
  validation pattern. `example_shapes_test.go:314-352` uses ordinary `os/exec`
  for worktree setup and cleanup.

## Static design contract and limits

- `ephemeral/research/api/SPRINTS.md:96-127` defines the persisted lifecycle
  event vocabulary, prefix-tree/ordinal/interval proof, and one-writer log.
  `SPRINTS.md:129-150` places the graph/timeline browser work in Sprint 3;
  `SPRINTS.md:152-170` puts steer/interrupt/cancel and explicit edges in Sprint
  4; `SPRINTS.md:172-180` defers the static pass and lints until after the web
  passes.
- `docs/definition-of-done.md:7-30` makes seen-working behavior and agent
  validation the gate; tests or commands alone are evidence to inspect, not a
  claim by themselves. `docs/definition-of-done.md:41-54` confirms supervisors
  steer and never gate results.
- `ephemeral/research/api/API.md:720-732` specifies runtime instances plus a
  future static source pass, with Go execution remaining the authority.
- `API.md:734-768` says names are supplied at call sites, scopes provide exact
  containment, and sessions/reviewers are nodes. `API.md:747-759` makes the
  name path the static/runtime join key and rules out reflection and
  `file:line` identity.
- `API.md:779-819` defines ordinal instance keys, prefix containment, sequence
  order, overlap/concurrency, separate session-owner versus turn-execution
  placement, and non-prefix fork/supervise/steer relations.
- `API.md:821-851` enumerates lifecycle and agent events; these are richer than
  the current observation table reducer.
- `API.md:856-870` describes the intended SSA pass and its limits: context
  parameters/lexical capture, known `context.With*` and Gimble calls,
  `Background` chain breaks, interface calls may resolve to multiple
  implementations, and conditionals produce possible scope sets.
- `API.md:275-293` lists intended static lints for duplicate Set keys, plain
  loops, nonconstant names/keys, raw goroutines, missing Group waits, and
  escaping contexts. This is a design record, not an implemented analyzer.
- `API.md:433-510` states Group is the only workflow concurrency primitive and
  repeated candidate scopes/instances are intentional. `API.md:598-718`
  defines nested supervision as ordinary sessions/turns plus attachment and
  steer relations.

## Official Go analyzer references downloaded locally

- `runtime-go-analysis.html` was fetched 2026-09-14 from
  `https://pkg.go.dev/golang.org/x/tools/go/analysis?tab=doc`. The official
  package defines `analysis.Analyzer`, a `Pass` with syntax/type/source-set
  inputs, `ResultOf` dependencies, and drivers such as `singlechecker` and
  `multichecker`.
- `runtime-go-ssa.html` was fetched 2026-09-14 from
  `https://pkg.go.dev/golang.org/x/tools/go/ssa?tab=doc`. The official package
  exposes SSA for packages/types/functions and documents building it from
  `go/packages`/`ssautil`; source-to-SSA correspondence remains incomplete.
  SSA makes control flow inspectable but cannot manufacture runtime timings,
  dynamic command arguments, or missing call-site identity.

## Verification

- The first `go test ./...` passed for all non-web packages but the `web`
  package failed because `skgo.manifest.json` was absent. `just build` then
  passed, installing web dependencies, running generation, building the Svelte
  app, and building `bin/gimble`; a second `go test ./...` passed for every
  package, and `go vet ./...` passed. The build generated artifacts outside
  this research leaf; no implementation was made here.
- No screenshot was attached in the received task. The UI evidence above is
  from repository code and `docs/web-app.md`/`API.md`.
