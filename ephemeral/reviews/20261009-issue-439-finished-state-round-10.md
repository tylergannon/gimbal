# Adversarial review: issue 439 proposal, finished application state, round 10

Reviewer: Claude Fable 5.1, same session as rounds 01 to 09, 2026-10-09.

## Review target

The proposal tree rooted at `ephemeral/research/issue-439/proposal.md` at
commit `0d2ed902` (`docs: separate project builds from recovery plugin
compatibility`, clean working tree), assessed as the finished application it
describes. Reviewed against the same authoritative sources as the earlier
rounds:

- `ephemeral/research/issue-439/requirements.md`
- `ephemeral/research/issue-439/issue.json`
- `AGENTS.md`, `docs/definition-of-done.md`, `go doc -all .`
- the current source on this branch, including the merged PR #440 worker
  (`plugins/codex-desktop/`, still at `1c3eb84c`)
- the unmerged runtime backend on `codex/temporal-container-backend` at
  `013559e85701cb424696b41fd396391a05a223ec`, which the proposal adopts
- the parallel draft and its request record in the gimbal-view exploration
  worktree (`temporal-docker-interface-design-2026-10-09.md`,
  `temporal-docker-interface-request-2026-10-09.md`)

Operating constraints from the caller: read-only except this artifact; write
to the caller-supplied path under `ephemeral/reviews/`. Rounds 01 to 09 are
left untouched. No caller narrowing of subject matter or predicted verdict
was present.

## Evidence inspected

Change since round 09: `git diff 2da9a85e..0d2ed902` touches only
`ephemeral/`: `topics/lean-runners.md:11` (the project runner "links the
execution client from its own module graph and never loads the server's Go
plugin"; compatible bundle is server plus recovery plugin; project build
metadata declares a recovery-contract ID checked before launch and
rechecked in records), `topics/ownership.md:49` (native names from a
canonical hash of the run/launch key and environment name, exact key
retained in labels), `:51` (one format/version initially; incompatible
persisted record is a named control failure), `topics/workflow-discovery.md:9`
(backend build metadata declares the linked client's recovery-contract ID;
hosted admission checks exact support), `topics/delivery-and-review.md:13`
(matching ID for the dead-runner check; incompatible ID or unloadable entry
rejected before work), and the worklog (round-09 entry; provenance
correction). No Go, Node, `justfile`, or `plugins/` change:
`git diff 2da9a85e --stat -- . ':!ephemeral'` is empty.

Verified for this round: the pinned backend client is the package
`internal/execution` (`temporal.go` imports `github.com/tylergannon/gimbal`,
`github.com/jackc/pgx/v5/pgxpool`, `go.temporal.io/sdk/client`,
`go.temporal.io/sdk/temporal`, `github.com/google/uuid`); the pinned
`go.mod` requires `github.com/jackc/pgx/v5 v5.11.0`, `go.temporal.io/api`,
`go.temporal.io/sdk`; this branch's `go.mod` has the two Temporal modules
and no pgx. The proposal's generated runner main lives in the project
(`workflow-discovery.md:9`: "generates the runner main package in
`generated/gimbal-runner/` in the project, builds it with `go build`"). The
draft's packaging section (line 179: the host "load[s] workflow + backend
plugins; validate two configs"; line 232: "At startup, `plugin.Open` each
configured `.so` path and `Lookup("GimbalBackend")` or
`Lookup("GimbalWorkflow")`"; line 321: "Adapt
`internal/execution/{temporal,worker,harness,mounts}.go` into the backend
package/plugin"). Proposal pages re-read in full: `lean-runners.md`,
`ownership.md`, `workflow-discovery.md`, `delivery-and-review.md:8-16`,
`backend-contract.md:11-16`, `storage-and-remote.md`.

## Disposition of round 09 findings

1. **Compatible bundle unachievable for project-built runners: resolved.**
   The runner links its client and never loads the plugin; the bundle is
   server plus recovery plugin; a recovery-contract ID bridges the record
   boundary with one format initially and no matrix (`lean-runners.md:11`,
   `ownership.md:51`, `workflow-discovery.md:9`,
   `delivery-and-review.md:13`). See finding 1 for the consequence the
   resolution does not name.
2. **Worklog provenance: resolved.** The round-08 entry is corrected and the
   round-09 entry states the plugin mechanism is the parallel task's
   proposed implementation of a required extension point.
3. **Docker names from raw ID prefixes: resolved.** `ownership.md:49`.

## Findings

### 1. Issue (incomplete requirement): "the project runner links the execution client from its own module graph" is impossible with the client where the pinned branch puts it, and the proposal does not name the public package and dependency surface that makes it possible

**The claim.** `lean-runners.md:11` and `ownership.md:51`: the project
runner links the backend execution client from the project's module graph.
`workflow-discovery.md:9`: the runner main is generated and built inside the
project module.

**The constraint.** Go's `internal` rule allows a package under
`internal/` to be imported only by code rooted at the parent of that
`internal` directory. The pinned client is `internal/execution` in module
`github.com/tylergannon/gimbal`. A project's `generated/gimbal-runner/`
is in the project's module and cannot import it. The pinned branch works
because its callers are `cmd/gimbal/main.go` and `web/runtime.go`, inside
the Gimbal module, which is exactly the arrangement the proposal replaces.

**What follows that the proposal must say.** The execution client has to
become an importable package with an exported constructor and
configuration type (the draft already names this as "adapt
`internal/execution/...` into the backend package/plugin", line 321). That
is a new exported API surface, which under `AGENTS.md` exists only when
asked for by name, and it pulls `pgx`, the Temporal SDK, the Docker CLI
wrapper, and the worker protocol into the module graph of every project
that uses the remote backend, not just into Gimbal. The proposal's
"selected execution-backend support" (`lean-runners.md:9`) and "links the
execution client" (`:11`) read as if that package exists. Neither page
names the package, says who owns it (Gimbal module, a separate module per
backend, or the consumer), or notes that the project's `go.mod` acquires
those dependencies. `storage-and-remote.md`, `workflow-discovery.md`, and
the TL;DR are silent too.

**Impact.** Whoever implements `gimbal workflows refresh` discovers at the
first `go build` that the client cannot be imported, and must then make a
packaging decision the proposal has deferred without saying so: a public
package in the Gimbal module (adds those dependencies to every Gimbal
consumer's graph), or a separate module per backend (then "selected
execution-backend support" means a `go.mod` requirement the generator must
add). The two choices give different project `go.mod` files and different
answers to "what does a lean runner link".

**Smallest response.** One sentence in `lean-runners.md:11`: the remote
backend's execution client is an importable package outside `internal/`
(name the intended location: a separate module per backend is the one that
keeps those dependencies out of projects that only run locally), its
exported constructor and config type are the client surface, and
`gimbal workflows refresh` adds that requirement to the project build when
`workflows.json` selects the backend. Add the same note to
`workflow-discovery.md:9`.

### 2. Nitpick: the proposal's runner packaging now differs from the draft it says it is aligned with, and the difference is not stated

`ownership.md:53` says recovery is "aligned with the parallel API draft,
whose interface remains owned by that task". The draft's packaging has the
host "load workflow + backend plugins" and `plugin.Open` both at startup
(draft lines 179, 232), that is, a workflow `.so` and a backend `.so`
loaded into one Gimbal process. The proposal now has a project-built runner
binary that links workflow code and the backend client, with the server
loading only a recovery `.so` (`lean-runners.md:11`). Both can be right
for their purposes, but a reader following the alignment claim would
expect the runner to load plugins. One sentence saying which parts of the
draft are adopted (the `Resolve`/`Harness`/`Start`/`Stop` seam, the
`RunControl` recovery shape, the record schema) and which are not (the
workflow-plugin loader and the runner-side `plugin.Open`) would keep the
two documents from being read as the same design.

### 3. Nitpick: the recovery-contract ID is a property of the linked client version, and the proposal should say where the generator reads it from

`workflow-discovery.md:9`: "Backend build metadata declares the linked
client's recovery-contract ID." The ID is a constant in the client package
that the project build happens to resolve, not something the project
author writes. The generated main can print or embed it at build time so
the sidecar metadata is derived from the actual linked version rather than
from a value an author could leave stale. One clause saying the generator
derives the ID from the built runner, not from `workflows.json`, closes the
gap between "declares" and "is".

## What the revision gets right

- The runner no longer loads a Go plugin, so Go's toolchain-matching rule
  constrains only the server and its installed recovery entry, which
  instance deployment controls.
- The record boundary is a single checked ID with one format and no
  compatibility matrix, which is the smallest thing that makes a mismatch
  visible instead of silent.
- Native names derive from a hash of the full key with the exact key
  retained in labels.
- The worklog states plainly what was accepted, what was rejected, and
  corrects its own earlier provenance claim.

## Outcome

material findings remain
