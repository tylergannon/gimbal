# Research brief: projecting Gimbal workflows into other execution environments

Date: 2026-09-27. Owner: Tyler. Research led by Claude (Fable 5.1) with
sub-agents. This directory is notes only (see AGENTS.md: nothing under
`ephemeral/` is code).

## The question

Can Gimbal use static analysis and code generation to project a workflow,
written as ordinary Go against the `gimbal` package, into a different
execution environment, and how? The first target is Temporal: a Gimbal
workflow should be deployable as a Temporal workflow, with the agent turns
(and commands, services, worktrees) carried out by workers running in a
per-project Kubernetes namespace on EKS, from defined container images,
with secrets distributed to them and repositories cloned through a GitHub
App. The workflow source itself must stay simple and locally runnable, as
today. Consumers (Tyler first, with Temporal) may supply templates that do
the transformation and generate the linkage.

## What Gimbal is (read `go doc -all .` in the checkout for the contract)

- Workflows are Go functions: `func X(ctx context.Context, env gimbal.Env, params P) error`.
- Primitives: `Run`, `Scope`, `Group` (errgroup-shaped; `g.Go(name, fn)` is the
  only way a workflow starts a goroutine), `Iterate`, `PromiseLoop` (planner
  selects tasks adaptively), `NewSession(ctx, role, workdir)`,
  `session.Generate[T](ctx, prompt, opts...)` (one blocking agent turn, T is
  `gimbal.Text` or a polytype-generated schema type), `session.Steer`,
  `session.Fork`, `Set`/`SetJSON` (scope-local values rendered into prompts),
  `RunCommand`, `Check`, `Service` (a foreground process owned by a scope),
  `Interview` (human in the loop), `WithSupervisor` (a second session that
  watches the worker's transcript and steers it mid-turn).
- A session is a harness process (Codex, Claude Code, Pi, OpenCode, agy) in a
  workdir on the local machine. Agent turns take minutes to an hour.
- Prompts and scope keys are compile-time constants; every node is named at
  its call site. The run log (run.jsonl, per-session transcripts, command
  stdout/stderr files) is the durable record and feeds the live web page.
- `internal/generate` already does static analysis: it loads the workflow
  package with go/packages + go/types, walks the entry function, and extracts
  a `workflow.Graph` (scopes, groups, loops, sessions, turns, commands),
  recording constructs it cannot read (select, go, defer, type switch) as
  diagnostics rather than guessing. `internal/gimballint` enforces rules
  (constant prompts, etc.). Generated code registers the graph, a SKGO form
  handler, and a CLI command.
- Workflows are compiled into the `gimbal` binary; a hosted instance runs them.

## Rules for every note written here

- Prefer official docs, then heavily-starred repos, then well-known
  engineers. Give the URL of every source you actually read, with the date
  or version you saw. Say which claims you verified against a source and
  which are inference.
- Note versions and dates explicitly; edge versions matter.
- No code. Prose and short tables. Be terse; substance stays, fluff dies.
- Push back. If the premise is wrong, say so with evidence.
