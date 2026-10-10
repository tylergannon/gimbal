# Adversarial review: issue 439 proposal, finished application state, round 09

Reviewer: Claude Fable 5.1, same session as rounds 01 to 08, 2026-10-09.

## Review target

The proposal tree rooted at `ephemeral/research/issue-439/proposal.md` at
commit `2da9a85e` (`docs: bound backend recovery to compatible deployment`,
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
  worktree: `temporal-docker-interface-design-2026-10-09.md` and
  `temporal-docker-interface-request-2026-10-09.md`

Operating constraints from the caller: read-only except this artifact; write
to the caller-supplied path under `ephemeral/reviews/`. Rounds 01 to 08 are
left untouched. No caller narrowing of subject matter or predicted verdict
was present.

## Evidence inspected

Change since round 08: `git diff 1eadeab0..2da9a85e` touches only
`ephemeral/`: `topics/backend-contract.md:13` (records keyed by the
server-assigned run/launch identity), `topics/delivery-and-review.md:13`
("restart the same compatible installed server/backend deployment"; reject
new admission when the recovery entry cannot load; drain before changing
the installed deployment), `topics/lean-runners.md:11` (local backend
compiled in; "the runner links or loads its execution client and the server
loads its typed recovery entry"; "Instance deployment selects and validates
the compatible server/runner/backend-plugin bundle"; "no per-run artifact
store"), `topics/ownership.md:49-53` (server writes the backend reference
and `(RunID, LaunchID)` to `owner.json` before launch; the key is passed to
the backend before the first `Resolve`; one compatible installed deployment;
restart of the same deployment supported, mixed-version upgrades not
promised), `topics/storage-and-remote.md:11`, and the worklog (round-08
entry: accepted the deployment-clarity gap, rejected the cross-version
redeploy assumption as deferred by `requirements.md:9`, simplified identity
to the server-assigned key). No Go, Node, `justfile`, or `plugins/` change:
`git diff 1eadeab0 --stat -- . ':!ephemeral'` is empty.

Verified for this round: `requirements.md:9` ("must not be gated on
seamless application upgrades"), `:13` (durable kill-one and kill-all
through restart), and the compiled-distributed-backends paragraph
("consumer-owned backends rather than the central server");
`workflow-discovery.md:9` (`gimbal workflows refresh` generates the runner
main in the project and builds it with `go build`; prebuilt bundles are a
later choice); the backend request record (Tyler: "Take the existing
temporal+docker implementation. What architecture (be specific -- actually
define the interface) supports that backend"); the draft's packaging row
(line 317: "Startup workflow/backend plugins | Separate bounded
packaging/API work | ... reproducible plugin/runner builds") and line 213
("Go plugin compatibility is a runner packaging constraint");
`internal/execution/temporal.go:213` (`gimbal_environments` primary key
`name`, `owner_id` column), `:296-319` (`environmentIdentity(b.owner,
name)`; duplicate owner rejected), `:335-343` (`environmentIdentity` embeds
`owner[:8]` and `owner[:12]` in the Docker container and volume names).
Proposal pages re-read in full: `ownership.md`, `lean-runners.md`,
`workflow-discovery.md`, `backend-contract.md:11-16`,
`delivery-and-review.md:8-16`, `storage-and-remote.md`.

## Disposition of round 08 findings

1. **Go-plugin recovery bound to a build-identical server: resolved as a
   scoped decision, with a correction to my argument.** The revision makes
   the rule explicit: one compatible installed deployment, same-deployment
   restart supported, drain and resolve cleanup before upgrade, admission
   refused and ownership preserved when the entry cannot load
   (`ownership.md:51`, `delivery-and-review.md:13`). `requirements.md:9`
   defers seamless upgrades, so that scoping is legitimate. I also withdraw
   the round-08 claim that "there is exactly one backend, and it is Gimbal
   code": `requirements.md` requires support for consumer-owned backends,
   so an extension point for recovery is justified and static linking of
   Gimbal-internal code alone cannot serve it. What remains open is the
   compatibility rule for project-built runners; see finding 1.
2. **Runner-published incarnation duplicates the server key: resolved.**
   `ownership.md:49`, `backend-contract.md:13`. `owner.json` is now
   server-written only.
3. **Local backend needs no plugin: resolved.** `lean-runners.md:11`.

## Findings

### 1. Issue (incomplete requirement): "one compatible server/runner/backend-plugin bundle" is not achievable for runners that each project builds with its own module graph, and the proposal does not say which side gives

**What the proposal requires.** `ownership.md:51`: "Use one compatible
installed server/runner/backend-plugin deployment." `lean-runners.md:11`:
"the runner links or loads its execution client and the server loads its
typed recovery entry. ... Instance deployment selects and validates the
compatible server/runner/backend-plugin bundle."

**What the proposal says about runners.** `workflow-discovery.md:9`: the
runner main is generated in the project and built there with `go build`,
against the project's `go.mod`, Go toolchain, and pinned Gimbal and
provider versions. Instance deployment does not control that build.

**Why the two cannot both hold as written.** If the runner *loads* the
backend `.so`, Go's plugin rule (host and plugin must share the toolchain
and identical versions of every common package, `lean-runners.md:13`,
`issue.json`) becomes a constraint on every project's `go.mod`: each must
pin exactly the versions the installed plugin was built with and use the
same toolchain, and a project that upgrades Gimbal breaks its own hosted
runs until the instance's plugin is rebuilt. If the runner *links* the
backend client from its own module graph, then the code that writes the
resource records (`gimbal_environments` rows, container and volume names,
activity identities) is the project's version while the code that reads
them for recovery is the instance's `.so`; "mixed-version upgrades are not
promised" (`ownership.md:51`) does not cover this skew, because it is
permanent and per project, not an upgrade event. In both readings
"compatible bundle" has no owner who can validate it, and the demonstration
at `delivery-and-review.md:13` runs one project against its own instance
and would not observe the failure.

**Impact.** Recovery of a dead runner's environments is the requirement
`backend-contract.md:13` exists for. Under the first reading it fails at
plugin open for any project whose dependency set differs; under the second
it fails at record decode or name derivation when the versions diverge.
Either failure is silent until a runner dies.

**Smallest response.** State which side is fixed. The simplest consistent
rule is: the runner never loads a Go plugin; it links the backend client
from its own module graph, and the recovery contract between runner-written
records and the server-side controller is a versioned record schema (the
draft already plans "a schema version bump") that the controller checks
before acting, refusing with a visible control limitation on mismatch. Then
"compatible bundle" shrinks to server plus installed recovery entry, which
instance deployment does control, and `workflow-discovery.md:9` needs no
toolchain constraint. Add to `delivery-and-review.md:13` a project built
against a different Gimbal patch version whose dead-runner environment is
still recovered or visibly refused.

### 2. Nitpick: the worklog attributes the Go-plugin architecture to a user request that the request record does not contain

The round-08 worklog entry calls it "the coordinated user-requested backend
plugin architecture". The authoritative request for the backend task asks
for a concrete interface that supports the existing Temporal/Docker
backend; it does not mention plugins. The draft itself lists "Startup
workflow/backend plugins" as "Separate bounded packaging/API work" (draft
line 317), that is, its own proposal. Under `AGENTS.md` a new exported
name exists only when Tyler asks for it by name, so the provenance matters;
the proposal should say the `.so` packaging is the parallel task's proposed
mechanism for a required consumer-backend extension point, which is
accurate and sufficient.

### 3. Nitpick: the pinned backend derives Docker names from the owner string, so the server-assigned key needs a stated shape

`environmentIdentity` builds the container name from `owner[:8]` and the
state volume from `owner[:12]` (`temporal.go:340-341`), and both must be
valid Docker identifiers. A `(RunID, LaunchID)` key passed as the owner
must therefore be at least twelve Docker-safe characters, or the backend
must hash it before use. One clause at `ownership.md:49` saying the backend
derives native names from a hash of the key, not from the key's prefix,
prevents an integration that truncates two IDs into a collision.

## What the revision gets right

- Recovery identity is now the server-assigned key that `owner.json`
  already carries, with one writer, and the pinned fresh-UUID behavior is
  named as the thing to replace.
- The deployment rule is explicit and consistent with the deferral of
  seamless upgrades: same-deployment restart, drain before upgrade,
  admission refused with ownership preserved when the entry cannot load.
- The local backend is stated to need no plugin or artifact, and the
  per-run artifact cache is explicitly rejected.
- The worklog records what was rejected from round 08 and why, which is the
  behavior the review process asks for.

## Outcome

material findings remain
