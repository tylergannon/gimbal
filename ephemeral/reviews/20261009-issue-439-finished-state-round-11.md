# Adversarial review: issue 439 proposal, finished application state, round 11

Reviewer: Claude Fable 5.1, same session as rounds 01 to 10, 2026-10-09.

## Review target

The proposal tree rooted at `ephemeral/research/issue-439/proposal.md` at
commit `5c49b017` (`docs: define importable backend client packaging`,
clean working tree), assessed as the finished application it describes.
Reviewed against the same authoritative sources as the earlier rounds:

- `ephemeral/research/issue-439/requirements.md`
- `ephemeral/research/issue-439/issue.json`
- `AGENTS.md`, `docs/definition-of-done.md`, `go doc -all .`
- the current source on this branch, including the merged PR #440 worker
  (`plugins/codex-desktop/`, still at `1c3eb84c`)
- the unmerged runtime backend on `codex/temporal-container-backend` at
  `013559e85701cb424696b41fd396391a05a223ec`, which the proposal adopts
- the parallel draft and its request record in the gimbal-view exploration
  worktree

Operating constraints from the caller: read-only except this artifact; write
to the caller-supplied path under `ephemeral/reviews/`. Rounds 01 to 10 are
left untouched. No caller narrowing of subject matter or predicted verdict
was present.

## Evidence inspected

Change since round 10: `git diff 0d2ed902..5c49b017` touches only
`ephemeral/`: `topics/lean-runners.md:13` (new paragraph: the Temporal/Docker
client is extracted from `internal/execution` into a backend-owned module
`github.com/tylergannon/gimbal/backends/temporaldocker` with its own
`go.mod`; client and recovery plugin share one implementation; projects
import and pin it; consumer backends own their modules; the package does not
yet exist), `topics/ownership.md:51` (the server passes the recovery-contract
ID in a backend-specific `recovery_contract` configuration field when
constructing the typed recovery controller, which validates it before
returning), `:53` (adopts the draft's execution/recovery seam and record
contract, not its workflow-plugin loader or runner-side `plugin.Open`),
`topics/workflow-discovery.md:9,11` (the ID is derived from the built
executable's metadata, not from `workflows.json`; refresh builds the
project's resolved graph and reports missing dependencies, never edits
`go.mod`), and the worklog (round-10 entry). No Go, Node, `justfile`, or
`plugins/` change: `git diff 0d2ed902 --stat -- . ':!ephemeral'` is empty.

Verified for this round: neither this branch nor the pinned branch has a
`backends/` directory or a `go.work`; the repository already carries one
nested module, `examples/temporal/go.mod`, which depends on the root through
`replace github.com/tylergannon/gimbal => ../..` (line 56) and is tested by
`just` with `go -C examples/temporal test` (`justfile:34`); `just build`
builds only `./cmd/gimbal` (`justfile:11,29`); tags `v0.11.1` to `v0.12.1`
exist. `workflow-discovery.md:9` still describes the sidecar metadata as the
product of source analysis at refresh time. No stale phrasing from earlier
revisions remains in `ephemeral/research/issue-439/` (searched for the
runner-published incarnation, the configuration-reference wording, and the
"server/runner/backend-plugin" bundle). Proposal pages re-read in full:
`lean-runners.md`, `ownership.md`, `workflow-discovery.md`,
`delivery-and-review.md:8-16`, `backend-contract.md:11-16`,
`storage-and-remote.md`, `proposal.md`.

## Disposition of round 10 findings

1. **Client not importable from a project runner: resolved.**
   `lean-runners.md:13`, `workflow-discovery.md:11`: a separate module
   outside `internal/`, explicitly imported and pinned by the project, with
   Temporal and pgx dependencies kept in that module. The repository's
   `examples/temporal` module is a precedent for the layout.
2. **Alignment claim versus the draft's runner-side plugin loader:
   resolved.** `ownership.md:53` names what is adopted and what is not.
3. **Recovery-contract ID provenance: resolved.** `workflow-discovery.md:9`:
   derived from the linked implementation constant, not author-entered.

## Findings

No material findings remain. The following are nitpicks.

### 1. Nitpick: the recovery plugin and the server must be built from one source of the root module, and the proposal does not say how

The recovery `.so` is built from the `backends/temporaldocker` module, which
depends on the root `gimbal` module. For `plugin.Open` to succeed, the
`.so`'s copy of `gimbal` and every shared dependency must be built from the
same source and toolchain as the server binary (`lean-runners.md:15`). A
module that requires a tagged `gimbal` version will not match a server built
from a working tree, and `just build` today builds only `cmd/gimbal`. The
repository already shows the arrangement that works: `examples/temporal/go.mod:56`
uses `replace github.com/tylergannon/gimbal => ../..`. One sentence at
`lean-runners.md:13` saying the recovery plugin is built from the same
checkout as the server (via that `replace` or a workspace file) and by the
same `just build` would make "compatible server/recovery-plugin bundle"
(`ownership.md:51`) something a reader can produce.

### 2. Nitpick: a backend configuration field name is prescribed on a page that says no names are prescribed

`ownership.md:51` names "the backend-specific `recovery_contract`
configuration field", while `lean-runners.md:13` says "the parallel backend
design owns its API, so no additional constructor names are prescribed
here". The intent (use the existing configuration input rather than a new
capability-query method) is clear and right; drop the literal field name or
mark it as an example so the two pages agree about who names it.

### 3. Nitpick: "the built executable's metadata" needs one clause on how refresh obtains it, and the missing-dependency path lacks a demonstration

`workflow-discovery.md:9` says the sidecar is produced by source analysis at
refresh and, in the same paragraph, that the recovery-contract ID comes from
the built executable's metadata. Both can be true only if the generated
runner main emits its metadata when invoked, or the analyzer reads the
constant from the resolved backend module. Say which. Separately,
`workflow-discovery.md:11` makes refresh report a missing backend
dependency, and `delivery-and-review.md` has no bullet observing that
failure or a successful refresh of a project that pins the backend module;
one bullet would cover both.

## What the revision gets right

- The client/recovery split is now a concrete, importable package layout
  with an existing in-repo precedent, and dependencies land in the module
  that needs them.
- Refresh does not edit `go.mod` or install modules; the project owns its
  dependency graph.
- The alignment with the parallel draft is scoped to the seam and the
  record contract, and the divergence on packaging is stated.
- The recovery-contract ID is derived from the linked implementation, so a
  stale author-entered value cannot admit an incompatible runner.

## Outcome

only nitpicks remain
