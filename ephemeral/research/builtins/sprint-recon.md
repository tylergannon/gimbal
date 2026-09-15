# Built-ins recon: Sprint and supervised LFG

Date: 2026-09-14

## Current integration surface

- `internal/workflows/sprint/sprints.go` is the only current first-party
  built-in workflow. `Sprint(ctx, Input)` selects real Codex/Claude adapters;
  the unexported `run` accepts adapters so tests and `-dry-run` can avoid model
  calls. `goalText`, `validationSection`, `runTask`, `command`, and `git` are
  the seams for input loading, validation, task execution, and repository
  mutation.
- `cmd/sprint/main.go` is the CLI composition point. It maps flags into
  `sprint.Input`, sets `Repo` from the current directory, creates the web
  runtime, and invokes `runtime.Run(..., sprint.Sprint)`. New user-facing
  built-ins need a command/subcommand or explicit selector here; the existing
  `-sprint`/`-issue` flags are tightly coupled to this workflow.
- `internal/workflows/sprint/schema.go`, generated
  `jsonschema_gen.go`, and `jsonschema/*.json` are generated-input seams.
  Any richer `Input` must be changed in `sprints.go` and regenerated with
  `go generate ./...`; generated files are not hand-edit targets.
- There is no `builtins` package or generic orchestrator in this tree. The
  minimal placement is to keep the sprint implementation in its package and
  add a sibling ordinary-Go workflow package/file only when a named built-in
  is required.

## Public API constraints

`go doc -all .` is authoritative. Relevant available operations are:

- `Run`/`Scope` define ownership and close sessions at scope end.
- `Group(ctx, name)` opens an observable concurrent scope; each `Go` child
  gets its own scope and `Wait` is mandatory before returning.
- `NewSession`, `Fork`, `Generate[T]`, `Steer`, `Interrupt`, and
  `WithSupervisor`. A supervisor is attached to one turn, periodically looks
  at the worker stream, and steers objections; it never gates the worker
  result. Supervisor options can themselves contain supervisors.
- `Set` supports only scalar/string-slice values. `SetJSON` supports a
  polytype-generated `Output`. `ScopeText` is the explicit prompt context;
  `Generate` injects no data implicitly. There is no arbitrary context map or
  implicit prompt enrichment.
- `Loop(ctx, name, goal, planner)` yields structured `Task` values. Its
  backlog goal is immutable, tasks are revisable, the yielded task scope
  carries task data and local feedback, and `Loop.Err()` is separate from task
  validation/fulfillment.
- Sessions belong to their creating scope. Forks inherit the workdir and
  conversation; concurrent turns sharing a workdir/session lineage need care.
  Worktree creation/removal remains ordinary `os/exec`/`defer` in the workflow.

## Existing Sprint roles and data flow

The current sequence is researcher (prime once) -> planner (fork) -> repeated
`round` scopes containing a `sprint` Loop -> coder fork per task plus one Claude
supervisor -> optional task command -> validator assessment -> repository
`go vet ./...` and `go test ./...` -> commit only if all evidence passes. After
the loop, a validator checks the whole goal (up to three rounds), then the
planner receives a merge prompt and is instructed to file issues, push, open,
and squash-merge a PR.

`Input` currently contains `Sprint`, `Issue`, `Repo`, `Model`, `ReviewModel`,
`Tasks`, and `DryRun`. It has no supervisor policy, scoped seed data, planning
role/model controls, parallelism, or execution-stage selection. `goalText`
reads a Sprint section from `ephemeral/research/api/SPRINTS.md`, or resolves
an issue number through `gh` and writes a local `.gimble/issues/N.md` before
prompting agents. The goal includes the fixed definition-of-done text, while
validator findings are placed in a child scope as `Set` data and shown through
`ScopeText`; findings do not mutate the immutable goal.

The current implementation is also Gimble-specific in its prompts and
assumptions: `researchPrompt` names Gimble and always asks for
`AGENTS.md`/`ephemeral/research/api/{API,SPRINTS}.md`; `done` hardcodes
`docs/definition-of-done.md`, `go vet ./...`, and `go test ./...`; task commit
messages say `Sprint N`; and `mergePrompt` always asks the planner to use `gh`
to file, push, open, and squash-merge. A general usable built-in needs an
explicit input contract instead of silently retaining those defaults.

## Current test seams

- `internal/workflows/sprint/sprints_test.go` uses a local `taskAdapter` and
  tests that task assessment gates commits, plus dry-run prompt ordering,
  absolute issue paths, schema visibility, findings feedback, and no repository
  mutation.
- Root `gimble_test.go` already covers fake-adapter behavior for scopes/data,
  groups, forks, supervisors (including nested supervisors), Loop structured
  tasks/feedback, cancellation, and event records. These are runtime proofs,
  not built-in workflow tests.
- `ephemeral/attest/just-attest` and
  `ephemeral/attest/loop-practice/supervisor-objects` are runnable shapes for
  supervised turns and should be treated as evidence/examples rather than API
  seams.
- `dryrun.go` is useful for any new prompt phases: it prints prompts and
  schemas without models or commands. Its answer generator is intentionally
  illustrative, so dry-run output cannot prove real planning or execution.

## Minimal implementation recommendation

1. Revise `sprint.Input` in place around the requested richer contract. The
   likely fields are `Goal`, `Plan` (local plan/spec path), `Acceptance`,
   `Constraints`, `ContextFiles`, `Checks`, `SupervisorInterval` (seconds),
   and an explicit finish policy (`local`, `pr`, or `merge`), alongside role
   models and task limits. Keep `Sprint`/`Issue` only as compatibility inputs
   if the parent task requires them; otherwise derive one normalized goal
   source. Validate mutually exclusive/required fields before creating
   sessions. Store seed data once in the run/round scope with `Set`/`SetJSON`,
   and append `ScopeText` explicitly to prompts. Keep repository paths and
   command checks workflow-owned.
2. Make a supervised LFG built-in as a small ordinary-Go workflow that creates
   a worker and reviewer in the owning scope, calls one `Generate` with
   `WithSupervisor`, and records the result. If LFG means launch/fan-out/gather,
   express fan-out with `Group` and gather after `Wait`; do not add a reusable
   high-level helper or runtime primitive.
3. Extend Sprint's planning phase inline: prepare the shared research/session
   context, fork independent planner sessions, run draft turns concurrently in
   a `Group`, run cross-critique turns against the saved draft text, then do a
   human refinement turn if the input supplies one and a synthesis turn that
   feeds the existing `Loop`. Keep the existing structured `Task` backlog as
   the execution contract; do not make planning agents write `done` markers.
4. Extend execution inline around the existing Loop/task seams: requirements
   (if requested by input) -> planning artifacts -> coder task scopes with
   supervisors -> deterministic command/query validation -> final validator.
   Reuse `runTask`'s commit gate and `Loop.Err` distinction. Parallel work must
   have independent scopes/worktrees and must join before merge/validation.
5. Update `cmd/sprint/main.go` flags and generated schema together. Preserve
   dry-run as a no-model/no-command viewer and add focused fake-adapter tests
   for phase ordering, prompt inclusion of scoped data, supervisor attachment,
   draft fan-out/join, and failed deterministic checks preventing commits.

## Pitfalls

- Do not represent supervisor objections as task failure: `WithSupervisor`
  steers and never gates. Only deterministic checks and validator evidence
  should block a commit or finish.
- Do not let parallel planners mutate the same backlog or worktree. Capture
  each draft in scoped data or explicit local files, then have critique/synthesis
  read those artifacts after `Group.Wait`.
- A `Set` key is write-once per scope. Use child scopes for revisions or one
  structured `SetJSON` value; do not overwrite a seed or validator finding in
  the same scope.
- `ScopeText` is unbounded and explicit. Pass only the needed draft, critique,
  requirements, and task evidence; otherwise prompts and supervisor look
  payloads can balloon.
- `Fork` preserves conversation and workdir, but does not create isolation.
  Worktree lifecycle and concurrent filesystem safety remain workflow code.
- Keep remote GitHub operations in the agent prompt/ordinary workflow boundary
  already established by `mergePrompt`; do not hide `gh` behind a new runtime
  abstraction.
- Generated schemas and any `go generate` output must be regenerated, while
  `go doc -all .` and the root API remain unchanged unless a genuinely missing
  primitive is demonstrated.
