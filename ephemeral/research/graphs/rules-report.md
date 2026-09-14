# Gimble graph and static-lint business rules

Integration note after Tyler's follow-up: this report audits the earlier design's rules. Tyler now explicitly prefers incomplete linting and permits dynamic Set keys; mandatory-constant recommendations here are not the current shipping gate. See `delivery-slices.md` and the separate `keys-recommendation.md` case study.

Audit date: 2026-09-14. Scope: current root package, workflow examples/tests,
API/design docs, and issue #162. No product files were changed.

## Rules the contract establishes

- A run establishes the root scope. A scope is a function-bound span; its ctx
  carries scope identity and its `parent` chain carries ancestry. Returning
  closes sessions and cancels the scope. `ScopeText` shows nearest values with
  outer scopes first; `Generate` injects nothing (`doc.go:13-18`,
  `scope.go:29-64,76-125,184-201`; `API.md:155-219`).
- `Set`/`SetJSON` writes are snapshots and set-once per scope instance. A child
  scope may shadow a parent key. Duplicate, ended-scope, and no-scope writes
  are programming errors (`scope.go:145-182`; `API.md:220-255`).
- Scope and graph names and value keys are stable compile-time constants. The
  documented static pass must reject dynamic names/keys (`API.md:246-247,
  275-293`). The current runtime still accepts strings, so this is a missing
  lint rather than existing runtime enforcement (`scope.go:52-59,145-182`).
- Sessions belong to the scope that creates them. A scope closes all owned
  sessions before cancelling its ctx; post-end Generate/Fork is rejected. Fork
  makes an independent child session in the current ctx, preserving the
  conversation and workdir (`scope.go:67-125`; `session.go:14-47,410-420,
  487-512`; `API.md:90-99,180-192`).
- `Group` is the only workflow concurrency primitive. `Group.Go` creates a
  child scope; `Wait` joins all children and then ends the group. Every return
  path must wait. First ordinary error cancels siblings; operator `Killed`
  errors are returned but do not abort siblings (`group.go:19-98`; `API.md:167-179,
  433-510`).
- `Loop.Tasks` creates one loop scope and one fresh task scope per yielded task;
  `task` is reserved and stored before user code. The task ctx ends before the
  next planner decision; local values become planner feedback
  (`loop.go:66-165`; `API.md:515-598`).
- Supervisors are turn-scoped sessions attached at the call site. They inspect
  bounded new transcript material, steer objections, and are cancelled/joined
  when the worker ends; they never gate the worker result (`supervise.go:72-112`;
  `API.md:598-717`).
- Static graph extraction should find `Scope`, `Group`, `NewSession`, `Fork`,
  `Loop`, `Each`, `WithSupervisor`, `Set`, and `Get`, trace contexts through
  parameters/captures and recognized `context.With*`/Gimble derivations, and
  add explicit data edges for constant writes and `ScopeText`/Get reads
  (`API.md:856-870`). Conditional scope branches remain possible scope sets;
  interface calls require possible implementations.

## Runtime-enforced versus static-only

Runtime currently maintains scope lookup and the parent ancestry chain; it
enforces set-once, ended/no-scope panics, session ownership cleanup, post-end
Session rejection, one active turn per Session, fork parent recording, Group
cancellation/join, and Loop task cleanup. `Generate`/`Fork` do not compare the
caller ctx's scope ancestry with the Session's owner; owner enforcement happens
when the owner's scope ends and marks the Session closed. Runtime does not
enforce: compile-time constant names/keys, raw Go
goroutine bans, Wait-on-every-path, ctx escape to structs/globals, stable graph
extraction, or static session-use ownership. Dynamic keys are accepted, and
the graph cannot promise a stable data node for them.

`SetJSON` gets its type restriction from `Output`; generic type checking rejects
ordinary arbitrary structs/maps. JSON encoding itself can panic, which is
runtime behavior, not a graph lint (`scope.go:20-24,145-160`; `API.md:220-235`).

## Issue #162 corrections

Issue #162's five checks are a useful subset but need these changes before
serving as the full contract; details and evidence are in
`rules-issue-162-audit.md`:

1. Keep duplicate-key detection, but compare normalized scope provenance, not
   only equal SSA ctx values. Context.With* creates new SSA values while
   preserving Gimble scope identity.
2. Replace callback-parameter equality with context provenance. Derived child
   contexts can be valid; helper calls and aliases require summaries or an
   explicit unknown result.
3. Flag raw workflow goroutines regardless of whether nested under a
   `Group.Go` callback; only Gimble Group.Go is allowed to own workflow
   goroutines.
4. Keep reserved `task`, recognizing aliases/derivations of task ctx, not only
   the immediate range body parameter.
5. Expand direct Background/TODO detection to aliases, derived contexts, and
   forbidden struct/global storage. Do not flag outer setup contexts that are
   passed into Run and replaced by Run's root ctx.
6. Add the omitted mandatory constant-key/name rule for all graph-bearing APIs.

Concrete current counterexample: `ephemeral/attest/loop-practice/group-in-task/main.go:163`
uses `fmt.Sprintf("attempt %d", i+1)` as a Set key. Runtime accepts this, while
`API.md:275-293` requires a build-failing lint because a static graph cannot
name the data node. Current correct Loop, Group, fork, and task examples are
listed in `rules-issue-162-audit.md`.

## Feasibility and boundaries

The official `golang.org/x/tools/go/analysis` API supports an analyzer with
`inspect` and `buildssa` requirements, diagnostics, SSA results, and
serializable package/object facts. `analysistest` supports `// want` fixtures;
`singlechecker`/`multichecker` provide drivers. Local snapshots with URLs and
fetch dates are `rules-go-analysis.md`, `rules-go-inspect.md`, and
`rules-go-analysistest.md`.

The first implementation can prove in-package direct calls, callbacks,
assignments, branches, closures, context derivations, and range-over-function
task bodies. Cross-package helpers/interfaces, function values, and context or
session values hidden in structs need facts or conservative unknown edges.
The analyzer should not claim runtime behavior: use it for definite source
violations and graph declarations. Dynamic keys are a static violation under
the contract; runtime remains a backstop if the gate is bypassed or a computed
key reaches an unmodeled path. Post-end behavior and unknown helper flows stay
runtime checks. Make the repository gate treat diagnostics as fatal because the
analysis framework leaves severity to its driver (`rules-go-analysis.md`;
`API.md:275-279`).

See `rules-source-excerpts.md` for copied contract/runtime excerpts and
`rules-index-leaf.md` for semantic-index routes.
