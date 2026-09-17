---
name: gimble-workflows
description: Write, run, and read a Gimble workflow: an agent workflow as ordinary Go on the root package gimble (Run, Scope, Group, Loop, sessions, Generate, RunCommand, supervisors, kills). Use whenever asked to write or change a workflow, a bake-off, a critique round, a loop, a supervisor, or to run one and read its record.
---

# Gimble workflows

A workflow is ordinary Go. The API is the root package `gimble`; read
`go doc -all .` before writing, and never invent a name that is not there.
There are no tactics in the API: a bake-off, a critique round, a retry, a
worktree, a merge is written inline in the workflow that needs it, with
`Group`, `Generate`, and git through `RunCommand`. Never propose a new
exported name; propose the program.

## What a run is

`gimble.Run(ctx, name, models, body)` runs `body` once and blocks until it returns.
The body is the workflow. Inside it, scopes form a tree: `Run` is the root,
`Scope`, each `Group.Go` child, and each `Loop` task open a child. A session
belongs to the scope that created it and is closed when that scope's body
returns, so a session made inside `Group.Go` is gone after that child. The
run's record is written under `<project>/runs/<id>/` as it goes. Every ctx
in the tree is a child of its parent's, so cancelling the run's ctx is how
it is stopped from outside; there is no stop method.

Ids are names with ordinals from the root: scope `work.1/task.2`, session
`work.1/task.2/coder.1`, turn `work.1/task.2/coder.1/turn.3`, command
`work.1/task.2/check.1`. These are the ids on the page, in the log, and in
a kill.

## Primitives

| Name | What it does | What to know |
| --- | --- | --- |
| `Project(ctx, dir)` | Puts the project dir in the ctx; runs land in `dir/runs/`. | Use `web.NewRuntime` instead for a run you watch. |
| `Run(ctx, name, models, body)` | The run and its root scope, with a model binding for every role it uses. | Join every goroutine before `body` returns. |
| `Scope(ctx, name, body)` | A named child scope; returns `body`'s error. | Values set in it are visible to its children, not its parent. |
| `Group(ctx, name)` | errgroup: `Go(name, fn)` per child, then `return group.Wait()`. | First error cancels the siblings; a killed child does not. Always `Wait`. |
| `Loop(ctx, name, goal, planner)` | The planner keeps a backlog and picks each next `Task`; `for ctx, task := range loop.Tasks {…}; return loop.Err()`. | Each task is a scope. What the body `Set`s is what the planner sees next. It ends by picking no task. |
| `NewSession(ctx, role, workdir)` | One conversation using the run's model binding for that cognitive role. Cannot fail; the process starts on the first turn. | Prefer Gimble's `Role...` constants; applications may define additional `WorkflowRole` constants. |
| `s.Generate[T](ctx, prompt, opts...)` | One blocking turn. `T` is `gimble.Text` for prose, or a polytype `Output` type whose schema is sent with the prompt. `prompt` must be a compile-time string constant (GIMBLE108); `Generate` appends the ctx scope's rendered context to it itself, as `prompt + "\n\n" + context`. | Put the run's data into the scope with `Set`/`SetJSON` before the call. A wrong-shaped answer is re-asked a bounded number of times. |
| `s.Fork(ctx, name)` | A new session with the conversation so far, in the same dir. | Read the code once, fork the readers. |
| `s.Steer(ctx, message) (landed, err)` | From another goroutine while `Generate` blocks: lands at the worker's next model call. | Dropped when no turn runs: `landed` false, `err` nil. The log's `steer` record says the same. |
| `RunCommand(ctx, name, workdir, command, args...)` | Runs one command and blocks until it exits: `(exitCode, stdout, stderr, err)`. | A nonzero exit is not an error; `err` is a command that could not start or was cancelled, exit code -1. Every command a workflow runs goes through it, never `os/exec`: it is recorded in the scope. |
| `Set(ctx, key, v)`, `SetJSON(ctx, key, v)` | Record a scalar or a polytype value in the ctx's scope; `Generate` renders every value visible from it, outermost first, as the context it appends to the prompt. | `Set` returns nothing and panics on misuse: a key set twice in one scope instance, or a scope that has ended. Revise by shadowing in a child scope. |
| `WithScopeTemplate(tmpl)` | An option to `Generate`: the scope is rendered for that one call through the `text/template` text `tmpl`, whose argument is a `gimble.ScopeData` (`Values` outermost first, `By` keyed; each has `Key`, `Value`, `Text`). | `tmpl` must be a string constant or a `//go:embed` variable (GIMBLE109), so what the agent is sent stays readable in the source; a long template reads better as a file. Gimble parses each text once. A template that cannot be parsed or rendered is the error `Generate` returns. |
| `WithSupervisor(session, instruction, opts...)`, `WithInterval(d)` | Options to `Generate`: a supervisor looks at what the worker did since its last look, every 3 minutes or `WithInterval`, and steers each objection in. `instruction` must be a compile-time string constant too (GIMBLE108). | It never gates the result. Its own options are `opts`, so a supervisor can have a supervisor. |
| `Killed{Target, By, Reason}` | The cause an operator's kill puts on a scope's or a turn's ctx. | `errors.As(err, &killed)` on a `Generate` error, or `context.Cause(ctx)`. See below. |

Structured output: a local struct whose field comments are the descriptions
the model reads. Declare it to polytype in a `//go:build jsonschema` file
beside the workflow, as the root package's `schema.go` does: a panic stub for
`Schema` and `ValidateJSON`, then `polytype.Declare(T.Schema)`. The
workflow's `//go:generate go tool polytype --validate` line, placed before its
gimblegen directive, writes the real methods. Pass an undeclared type to
`Generate` and the compiler names the missing method. A field comment is
prompt text: write it as an instruction.

## Kills

An operator holds ids from the page, not pointers, and reaches a live run
through the web runtime:

```go
landed, err := runtime.Steer(ctx, runID, "work.1/task.2/coder.1", "look at the tests first")
err = runtime.KillTurn(runID, "work.1/task.2/coder.1/turn.3", "tyler", "editing the wrong file")
err = runtime.KillScope(runID, "work.1/task.2", "tyler", "off the rails")
err = runtime.SteerLoop(runID, "work.1", gimble.WrapUp)
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

A message to a `Loop` is not a steer of a turn: a planner is not always in
one, so `SteerLoop` holds it for the loop's next planning decision instead
of dropping it, and the loop records it as landed once the planner has read
it. `gimble.WrapUp` is the message that means end dispatch there; anything
else in prose is the planner's to weigh. The run page offers both on the
card of a loop that is still dispatching.

## The shapes

Each is a compiling `Example` in the root package (`example_test.go`,
`example_shapes_test.go`); copy the shape, not the fake harness.

| Shape | Example | The point |
| --- | --- | --- |
| One turn | `Example` | `Set` the goal, one session, one `Generate` with a constant prompt. |
| Fork and bake-off | `Example_bakeOff` | One researcher reads; two forks propose in a `Group`; a judge on another harness picks by reading the proposals. |
| Critique round | `Example_critiqueRound` | Critic reads the file against the code; writer takes or rejects each finding; at most two rounds. |
| Supervised worker | `Example_supervisedWorker` | `WithSupervisor` on the turn; the worker's own answer comes back. |
| Loop with a planner | `Example_loopWithPlanner` | Planner forked from the researcher; a coder per task; `Set` the result for the planner. |
| Worktree per candidate | `Example_worktreePerCandidate` | `git worktree add` per candidate, absolute path in the prompt, `git status` in the repo afterwards. |
| Validation command | `Example_validationCommand` | The check is chosen before the coder starts; exit code is the verdict; output goes back to the coder. |
| Killed by an operator, loop recovers | `Example_killedTurn` | `runtime.KillTurn` by id mid-turn; the task's `Generate` returns the `Killed`; the body re-asks the same session and the loop goes on. |

## Run and watch

A workflow is a package under `internal/workflows/` whose entry is
`func Name(ctx context.Context, env gimble.Env, params NameParams) error`.
`env` is Gimble-owned and holds the absolute initial `WorkDir`; `params` and
its workflow-specific type contain only arguments belonging to that workflow.
The entry has the directive
`//go:generate go run github.com/tylergannon/gimble/internal/generate/gimblegen -entry Name -name name`,
placed after its polytype directive when it has structured outputs.
`go generate` runs the independent generator in `internal/generate/` and
prints `workflow_gen.go` beside the workflow: the graph, which registers
itself, and the workflow's `Command(defaults)`, a Cobra subcommand you can read: one
`--work-dir`, which defaults to the current directory; one flag per field of
the parameter struct, named from the field with its doc comment as help, required unless the
field is a `polytype.Optional` (a bool is never required); one `--<role>` flag per role the graph
names, defaulting to the model supplied by the application and required when
that default is empty; and `--port`, `--uds`, `--no-web`. One line in `cmd/gimble/workflows.go` adds it to `gimble run`. Then:

```sh
gimble run --help
gimble run review --help
gimble run review --work-dir /abs/repository --goal "find correctness bugs"
gimble run review --work-dir /abs/repository --goal "find correctness bugs" --code-review gpt-5.6-luna:high
```

The log's first line is `gimble: run <id> started in <dir>`; the page is
`http://127.0.0.1:8080/runs/<id>`, live while it runs and after. Ctrl-C
cancels the ctx, which interrupts every turn. To read a workflow's prompts
without a model, read its graph: every prompt is in it, verbatim. `just vet`
and `just test` are the repository's checks.

## Read the record

Under `<project>/runs/<id>/`:

- `run.jsonl`: one `LifecycleRecord` per line, gap-free `seq`, with `scope`,
  `session`, `turn`, and `event.kind` in `run_started`, `scope_began`,
  `scope_ended`, `session_created`, `session_closed`, `turn_started`
  (the prompt and `output_type`), `turn_ended` (result, error, usage per
  model, duration, `interrupted`), `command_started` (the command's `id`,
  what ran, the workdir), `command_ended` (`exit_code`, `stdout`, `stderr`,
  error, `interrupted`), `value_set`, `steer`, `supervise_attached`,
  `planner_decision`, `killed`, `run_cancelled`, `run_ended`, and last
  `complete`; no `complete` means the run did not finish durably.
- `sessions/<scope>/<session>.jsonl`: the harness's own events for that
  session, one `AgentRecord` per line.
- `scopes/<loop key>/backlog.md`: a `Loop`'s backlog as the planner last
  revised it.
- `run.json`, `scopes.json`, `sessions.json`, `turns.json`, `turn_usage.json`,
  `model_calls.json`, `commands.json`: the seven tables the page reads, kept
  current as the run goes.
- `commands/<command id>.stdout` and `.stderr`: a command's whole stream when
  it is longer than the 64 KiB its record keeps; the record holds the tail
  and names the file.
- `../../project.jsonl`: every run's lifecycle records in one file, ordered
  by `time`, not `seq`.

Every prompt sent: `jq -r 'select(.event.kind=="turn_started") | .turn, .event.prompt' run.jsonl`.
What each turn cost: `jq -c 'select(.event.kind=="turn_ended") | {turn, usage: .event.usage, ns: .event.duration}' run.jsonl`.
Who killed what: `jq -c 'select(.event.kind=="killed")' run.jsonl`.
How each command ended: `jq -c 'select(.event.kind=="command_ended") | {id: .event.id, exit: .event.exit_code, error: .event.error}' run.jsonl`.

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
- A harness adds its own context: Codex reads the user's global
  instructions, and a Codex session in a workflow once wrote a worklog
  nobody asked for. The prompt is not everything the agent sees.
- Size a supervisor's `WithInterval` to the worker's step. Eight seconds
  against three-second worker steps gave 29 supervisor turns for 3 worker
  turns and three quarters of the run's input tokens.
- Cheap models for any live run you start to see something work: Codex
  `gpt-5.6-luna`, Claude `claude-haiku-4-5-20251001`, Gemini
  `gemini-3.8-flash-low`. Say which model a run used.
- Proof is running the real thing and saying what you saw, in the chat or
  the PR description. Write no proof program and commit nothing a run
  produced: no `ephemeral/attest/`, no `result.md`, no logs, no dumps.
- Never scan from `/` or `$HOME`, in a prompt or in the workflow: name the
  directory.
- Nothing under `docs/`, `internal/observation/`, `web/src/`, or a generated
  skgo file changes for a workflow.
