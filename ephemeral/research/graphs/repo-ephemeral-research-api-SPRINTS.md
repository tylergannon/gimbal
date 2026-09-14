# Building Gimble: four sprints

The root package's Godoc defines the current public API. `API.md` beside this
file is the design record behind it; this file records delivery order and
historical sprint scope.

0. The skgo app, generated. Done 2026-09-10.
1. The API, all of it.
2. The events, into files.
3. The web app, first pass: see everything.
4. The web app, second pass: touch everything.

Sprint 1 is built by hand. Every sprint after it is built by `cmd/sprint`,
the sprint workflow, written in Sprint 1 against the finished API and not
rewritten after: from Sprint 2 on, building Gimble is a run of Gimble.

Godoc is the API contract. `API.md` preserves the decisions and reasons. The
reports beside it are done with.

Rules: as simple as possible, no name that `API.md` does not have, tactics
inline in the workflow. Attestation runs on the cheap tier (`gpt-5.6-luna`,
Haiku, flash); real build runs on whatever Tyler picks. Legacy code is
rewritten against the new contract, not spliced.

## Sprint 0: the app, generated

Done. `skgo new --build-tool=just` made `web/` (the SvelteKit app),
`generated/` (`go generate ./...` writes it), `web/server.go` (the one
`NewHandler`), `cmd/main.go` (the binary), `Justfile`, `e2e/`. The root
package is `gimble`, the API; the page's Go imports it, so `Serve` lives
in `web`. `AGENTS.md` carries the rules. `just build` builds it and
`./bin/gimble` serves the starter page.

## Sprint 1: the API

Ship: `go get github.com/tylergannon/gimble` and write any workflow in
`API.md`, against both harnesses, with nothing on disk and no page.

- `HarnessAdapter`, from the legacy `harness/contract.go` without
  `Compact`: create session, run turn with schema and event callback,
  steer, interrupt, fork. Both adapters ported from `ephemeral/legacy/
  harness/`: Codex (native steer, cheapest model) and Claude Code.
- `NewSession`, `Generate[T]`, `Text`, `AgentOption`, `Steer`, `Interrupt`,
  `Fork`. Validation once in `Generate`; a cancelled `Generate` interrupts
  the native turn and returns `ctx.Err()`; a steer with no turn returns
  nil and is marked dropped; a session past its scope errors.
- `Scope`, `Set`, `SetJSON`, `Get`, `GetJSON`, `ScopeText`, `Each`,
  `Output`. In this sprint because `NewSession`, `Group.Go`, and `Loop`
  all open scopes and sessions close with theirs; keys with ordinals are
  what Sprint 2 places every event by. The data half is small and the
  sprint workflow wants `ScopeText` in its prompts. If anything is cut
  from this sprint, it is the data half, last.
- `Group`: `Go` is a child scope, `Wait` joins and ends the group, the
  first error cancels the rest.
- `WithSupervisor` and `WithInterval`, the loop in `API.md` verbatim, over
  the worker's in-memory event stream: a look every three minutes when
  there is something new, objections become steers, no gate. A supervisor
  takes the same options, so it can be supervised. `Review` is
  `struct{ Objections []string }`.
- `Loop(ctx, name, goal, planner)`, `Tasks`, structured `Task`, `Err`.
  Backlog file: markdown with YAML frontmatter, immutable goal, and revisable
  tasks. Per dispatch: reload, ask the planner, yield a child scope containing
  the selected task, and carry that scope's recorded result into the next
  decision. Validation is ordinary workflow code. Ending dispatch never marks
  the goal fulfilled.
- `Run` and `Project`. `Project` is the runs directory only; `Run` opens
  the root scope, calls the body, ends it. No `Serve`, no `Start`.
- `cmd/sprint`: `sprint.Sprint(ctx, in Input)` in `sprint/sprints.go`,
  gated as `docs/definition-of-done.md` says. A researcher primed once on
  `API.md` and the code; a `Loop` whose goal is the sprint's section of
  this file with the definition of done, and whose planner is forked from
  the researcher; each task gets a coder forked from the researcher, one
  supervisor that steers and never gates, explicit task validation, and the
  repository checks. Passing work is committed and failed work remains in the
  tree for the planner to respond to. When the planner is done, a validator
  checks that the sprint is legitimately demonstrated; its objections are
  another loop. Then the
  planner files what is left as issues and merges the branch with `gh`. No
  bake-off: it multiplies every assignment's cost, and a sprint does not need it.
- `Justfile` gains `vet`, `test`, `attest`. Tests: every primitive over a
  fake adapter. `just attest`: one workflow on `gpt-5.6-luna` and Haiku
  that touches every primitive once (a scope with data, a schema turn, a
  text turn, a steer that lands and one that drops, an interrupt, a fork,
  a group of two, a supervised turn with an objection, a loop of two tasks)
  and prints what it saw.

Proof: Sprint 2 is built by `cmd/sprint`.

Built 2026-09-10 as far as the sprint workflow needs: not yet
`Session.Interrupt`, `Each`, `Get`, `GetJSON`, or `just attest`, because
the sprint workflow calls none of them.

`API.md`: Sessions and turns, Context, Scope, Concurrency, Loop,
Supervisors, Run (minus Serve and Start).

## Sprint 2: events into files

Ship: every run leaves a complete record on disk that a program or an
agent can read while it runs and after.

- `<dir>/runs/<id>/` with id `20260910-140322.sprint`; `run.jsonl` for
  lifecycle events; `sessions/<session id>.jsonl` for agent events, at the
  grain the harness emits them; `<dir>/project.jsonl` for run started,
  ended, cancelled; `scopes/<loop key>/backlog.md` for a loop's file.
- The typed `Event`: `seq`, `time`, scope key, session and turn ids when
  present, then the kind. Lifecycle: run started, ended, cancelled; scope
  began, ended (a task scope's began carries its task); planner decision;
  set; session created (name, adapter, model,
  workdir, parent), closed; turn started (prompt, output type), ended
  (result or error, tokens, duration, interrupted); supervise attached;
  steer (target, message, source, landed or dropped); interrupt. Agent:
  user, assistant, thinking, tool call, tool result, delta naming what it
  extends, usage, harness error, approval request, nested transcript.
- One writer per run, atomic appends, a final event that marks the log
  complete. Supervisors read the same stream the file is written from.
- A function on the ctx for the run directory, if the sprint workflow
  wants to tell a post-mortem agent where the log is. That is the
  workflow that was going to decide it.
- Tests: a fake-adapter run of the attest workflow produces a log; the
  log's prefix tree is the scope tree; ordinals, intervals, and the three
  explicit relations are all present; a second reader tails it live and
  stops at the final event.

Proof: Sprint 3 is built by `cmd/sprint`, and the run it leaves in
`.gimble/runs/` is the first fixture the page replays.

`API.md`: Observability (Events, Keys), Run (ids, the log).

## Sprint 3: the web app, first pass

Ship: open a browser and see every run in the project, live and past,
drawn as the graph.

- `web.NewRuntime` and `gimble.Start`. `cmd/main.go` becomes `cmd/sprint`:
  with an argument it runs; without, it serves and waits. The form for
  the sprint workflow beside its page, `skgo.Form(startSprint)`, controls
  named by `Input`'s fields. The starter routes go.
- `watchRuns`, `watchRun`, `watchSession`, tailing the files from disk.
- Pages: runs list; the run graph, scopes from key prefixes with their
  data and what `ScopeText` would say, siblings on a timeline with
  overlaps drawn as concurrent, sessions stacked under their node, turns
  with tokens and duration, transcripts coalesced by delta id, replayed
  then live; the loop's backlog as the planner left it after each dispatch.
- Tests: skgo's three layers over `testdata/runs/` seeded with Sprint 2's
  real run; the typed-drift test on the form.

Proof: Sprint 4 is built by `cmd/sprint` started from the form and watched
live, every candidate side by side.

`API.md`: Run (Serve, Start, the form), Observability (remote functions).

## Sprint 4: the web app, second pass

Ship: a person at the page is one more supervisor.

- `steer`, `interrupt`, `cancelRun`. A steer box on every session card, a
  turn-running indicator so a person can see whether a steer will land,
  cancel on the run.
- Edges: supervise attachments (reviewer to worker turn), fork edges,
  steers drawn from source to target and marked landed or dropped, the
  person as a supervisor node attached to every session.
- Whatever the first pass showed was missing. The event fields are settled
  by what this page needs to draw, so `Event` may grow here.
- Tests: the three layers again; a browser test that steers a fake live
  run and sees the steer land.

Proof: everything after this is built by `cmd/sprint` with Tyler steering
it from the page.

`API.md`: Observability (steer attribution, the person as a node).

## After that, built by the Loop, in the order something needs them

- The static pass and the lints under Scope, every lint failing the build.
- Write-through of scope values to `runs/<id>/scopes/<key>/` and
  `ScopeText` referencing large values by path.
- `Compact` and restart: still open, see `API.md`.
- Hierarchy, which Tyler asked to be reminded of once `Loop` exists: a
  top `Loop` whose planner is the CEO and whose assignments run sprint loops. A
  program, not a primitive. Its first job is running this list.
