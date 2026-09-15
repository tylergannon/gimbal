---
name: gimble-workflows
description: Write, run, and read a Gimble workflow: an agent workflow as ordinary Go on the root package gimble (Run, Scope, Group, Loop, sessions, Generate, supervisors, kills). Use whenever asked to write or change a workflow, a bake-off, a critique round, a loop, a supervisor, or to run one and read its record.
---

# Gimble workflows

A workflow is ordinary Go. The API is the root package `gimble`; read
`go doc -all .` before writing, and never invent a name that is not there.
There are no tactics in the API: a bake-off, a critique round, a retry, a
worktree, a merge is written inline in the workflow that needs it, with
`Group`, `Generate`, and git through `os/exec`. Never propose a new exported
name; propose the program.

## Use a builtin

For existing coding work, start with `gimble work -repo /absolute/project
-goal "the desired result"`. It asks useful questions and recommends `lfg`
(one supervised worker), `plan` (three independent drafts, cross-critiques,
and a saved plan), or `sprint` (planner, supervised coders, and independent
validation). Each name is also a direct CLI command.

Supply a design with `-file design.md`, an implementation plan with `-plan
plan.md`, and boundaries with `-acceptance` and `-constraints`. File paths
resolve from `-repo`; repeat `-file` and `-check` for more context or checks.
Checks run in that repository. `plan` never implements; guided `work` offers
execution after planning. `-yes` accepts recommendations without questions.
Changes remain local and uncommitted unless Sprint explicitly receives
`-finish pr` or `-finish merge`.

The CLI prints request artifacts under `.gimble/requests` and records under
`.gimble/runs`. `-dry-run` previews without models or execution commands;
`-no-web` omits the live web server. The default models are Luna, Haiku,
and Gemini Flash; use `-model`, `-review-model`, and `-planning-model` to
select installed, authenticated native harness models. Use these workflows
as readable Go examples in `internal/workflows/` when writing another.

## What a run is

`gimble.Run(ctx, name, body)` runs `body` once and blocks until it returns.
The body is the workflow. Inside it, scopes form a tree: `Run` is the root,
`Scope`, each `Group.Go` child, and each `Loop` task open a child. A session
belongs to the scope that created it and is closed when that scope's body
returns, so a session made inside `Group.Go` is gone after that child. The
run's record is written under `<project>/runs/<id>/` as it goes. Every ctx
in the tree is a child of its parent's, so cancelling the run's ctx is how
it is stopped from outside; there is no stop method.

Ids are names with ordinals from the root: scope `work.1/task.2`, session
`work.1/task.2/coder.1`, turn `work.1/task.2/coder.1/turn.3`. These are the
ids on the page, in the log, and in a kill.

## Primitives

| Name | What it does | What to know |
| --- | --- | --- |
| `Project(ctx, dir)` | Puts the project dir in the ctx; runs land in `dir/runs/`. | Use `web.NewRuntime` instead for a run you watch. |
| `Run(ctx, name, body)` | The run and its root scope. | Join every goroutine before `body` returns. |
| `Scope(ctx, name, body)` | A named child scope; returns `body`'s error. | Values set in it are visible to its children, not its parent. |
| `Group(ctx, name)` | errgroup: `Go(name, fn)` per child, then `return group.Wait()`. | First error cancels the siblings; a killed child does not. Always `Wait`. |
| `Loop(ctx, name, goal, planner)` | The planner keeps a backlog and picks each next `Task`; `for ctx, task := range loop.Tasks {…}; return loop.Err()`. | Each task is a scope. What the body `Set`s is what the planner sees next. It ends by picking no task. |
| `NewSession(ctx, name, adapter, model, workdir)` | One conversation on one harness in one dir. Cannot fail; the process starts on the first turn. | Adapters: `codex.New()`, `claude.New()`, `agy.New()`. |
| `s.Generate[T](ctx, prompt, opts...)` | One blocking turn. `T` is `gimble.Text` for prose, or a polytype `Output` type whose schema is sent with the prompt. | Nothing is injected: put `ScopeText(ctx)` in the prompt yourself. A wrong-shaped answer is re-asked a bounded number of times. |
| `s.Fork(ctx, name)` | A new session with the conversation so far, in the same dir. | Read the code once, fork the readers. |
| `s.Steer(ctx, message) (landed, err)` | From another goroutine while `Generate` blocks: lands at the worker's next model call. | Dropped when no turn runs: `landed` false, `err` nil. The log's `steer` record says the same. |
| `Set(ctx, key, v)`, `SetJSON(ctx, key, v)`, `ScopeText(ctx)` | Record a scalar or a polytype value in the ctx's scope; render every value visible from it, outermost first. | `Set` returns nothing and panics on misuse: a key set twice in one scope instance, or a scope that has ended. Revise by shadowing in a child scope. |
| `WithSupervisor(session, instruction, opts...)`, `WithInterval(d)` | Options to `Generate`: a supervisor looks at what the worker did since its last look, every 3 minutes or `WithInterval`, and steers each objection in. | It never gates the result. Its own options are `opts`, so a supervisor can have a supervisor. |
| `Killed{Target, By, Reason}` | The cause an operator's kill puts on a scope's or a turn's ctx. | `errors.As(err, &killed)` on a `Generate` error, or `context.Cause(ctx)`. See below. |

Structured output: a struct whose field comments are the descriptions the
model reads, with `//go:generate go tool polytype --validate` in the package
and a `//go:build jsonschema` stub file that `polytype.Declare`s each type,
as `internal/workflows/sprint/schema.go` does. A field comment is prompt
text: write it as an instruction.

## Kills

An operator holds ids from the page, not pointers, and reaches a live run
through the web runtime:

```go
landed, err := runtime.Steer(ctx, runID, "work.1/task.2/coder.1", "look at the tests first")
err = runtime.KillTurn(runID, "work.1/task.2/coder.1/turn.3", "tyler", "editing the wrong file")
err = runtime.KillScope(runID, "work.1/task.2", "tyler", "off the rails")
```

A steer from the page is recorded with `source: "person"`; a supervisor's
with its session id. A kill puts a `gimble.Killed{Target, By, Reason}` cause
on every ctx under the target: `errors.As(err, &killed)` on the `Generate`
error tells a kill from an ordinary failure, and `context.Cause(ctx)` shows
it anywhere below. A killed turn ends only that turn: the session, its
scope, and the loop keep running, and the body decides whether to re-ask.
A killed scope closes its sessions; its `Group` siblings run on; a `Loop`
records the task failed with the reason and the planner sees it on the next
lap. An unknown or finished id is an error. Write the workflow to handle the
cause; the sending is the operator's.

## The shapes

Each is a compiling `Example` in the root package (`example_test.go`,
`example_shapes_test.go`); copy the shape, not the fake harness.

| Shape | Example | The point |
| --- | --- | --- |
| One turn | `Example` | `Set` the goal, one session, one `Generate` with `ScopeText`. |
| Fork and bake-off | `Example_bakeOff` | One researcher reads; two forks propose in a `Group`; a judge on another harness picks by reading the proposals. |
| Critique round | `Example_critiqueRound` | Critic reads the file against the code; writer takes or rejects each finding; at most two rounds. |
| Supervised worker | `Example_supervisedWorker` | `WithSupervisor` on the turn; the worker's own answer comes back. |
| Loop with a planner | `Example_loopWithPlanner` | Planner forked from the researcher; a coder per task; `Set` the result for the planner. |
| Worktree per candidate | `Example_worktreePerCandidate` | `git worktree add` per candidate, absolute path in the prompt, `git status` in the repo afterwards. |
| Validation command | `Example_validationCommand` | The check is chosen before the coder starts; exit code is the verdict; output goes back to the coder. |
| Killed by an operator, loop recovers | `Example_killedTurn` | `runtime.KillTurn` by id mid-turn; the task's `Generate` returns the `Killed`; the body re-asks the same session and the loop goes on. |

## Run and watch

A workflow is a `package main` (see `cmd/sprint/main.go`):

```go
ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
defer stop()
runtime, err := web.NewRuntime(ctx, filepath.Join(repo, ".gimble"), web.WithPort(8080))
err = runtime.Run(ctx, "bakeoff", func(ctx context.Context) error { ... })
```

The log's first line is `gimble: run <id> started in <dir>`; the page is
`http://127.0.0.1:8080/runs/<id>`, live while it runs and after. Ctrl-C
cancels the ctx, which interrupts every turn. `web.WithNoWeb()` runs without
the page. `go run ./cmd/sprint -dry-run -issue <file>` shows the sprint
workflow's prompts and schemas without calling a model; a new workflow that
wants that writes a fake `HarnessAdapter` the way `internal/workflows/sprint/dryrun.go`
does. `just vet` and `just test` are the repository's checks; `just attest`
runs the live attestation on the cheap tier.

## Read the record

Under `<project>/runs/<id>/`:

- `run.jsonl`: one `LifecycleRecord` per line, gap-free `seq`, with `scope`,
  `session`, `turn`, and `event.kind` in `run_started`, `scope_began`,
  `scope_ended`, `session_created`, `session_closed`, `turn_started`
  (the prompt and `output_type`), `turn_ended` (result, error, usage per
  model, duration, `interrupted`), `value_set`, `steer`, `supervise_attached`,
  `planner_decision`, `killed`, `run_cancelled`, `run_ended`, and last
  `complete`; no `complete` means the run did not finish durably.
- `sessions/<scope>/<session>.jsonl`: the harness's own events for that
  session, one `AgentRecord` per line.
- `scopes/<loop key>/backlog.md`: a `Loop`'s backlog as the planner last
  revised it.
- `run.json`, `scopes.json`, `sessions.json`, `turns.json`, `turn_usage.json`,
  `model_calls.json`: the six tables the page reads, kept current as the run goes.
- `../../project.jsonl`: every run's lifecycle records in one file, ordered
  by `time`, not `seq`.

Every prompt sent: `jq -r 'select(.event.kind=="turn_started") | .turn, .event.prompt' run.jsonl`.
What each turn cost: `jq -c 'select(.event.kind=="turn_ended") | {turn, usage: .event.usage, ns: .event.duration}' run.jsonl`.
Who killed what: `jq -c 'select(.event.kind=="killed")' run.jsonl`.

## Rules a workflow author meets

- The validator's findings never become the goal. They go to the planner as
  information (`Set(ctx, "what the validator did not see working", …)`),
  never appended to the goal or turned into tasks by the workflow. A
  finding is a claim to investigate; the implementer decides.
- The judge's input differs in kind from the actor's output. A validator or
  judge reads files, runs commands, and watches the software; it never reads
  the worker's summary as evidence. Prompt it to read the thing, not the
  report.
- An agent can repair what it is judged against. Choose the validation
  command and the thing it starts before the coder's turn, run it from the
  workflow, and tell the coder the check stays as it is.
- Absolute workdirs in prompts. Every path in a prompt is absolute, the
  session's `workdir` is absolute, and a worktree workflow checks
  `git status --porcelain` in the repository after the turn: an agent told a
  bare filename writes to the repository root.
- Information lands locally. A workflow that needs a GitHub issue writes its
  text to a file first and names the path in the prompt: "Read and implement
  the issue in /abs/path/168.md." No prompt points an agent at a remote
  source.
- Prompts are plain English: what to read, what to do, what to leave
  uncommitted, what to answer with. No boilerplate about being an agent. A
  validator's prompt is one line plus the context it needs.
- Cheap models for attestation: Codex `gpt-5.6-luna`, Claude
  `claude-haiku-4-5-20251001`, Gemini `gemini-3.8-flash-low`. Say which
  model a run used. Proof of a workflow is a live run and its record under
  `ephemeral/attest/<issue>/`, never a unit test alone.
- Never scan from `/` or `$HOME`, in a prompt or in the workflow: name the
  directory.
- Nothing under `docs/`, `internal/observation/`, `web/src/`, or a generated
  skgo file changes for a workflow.
