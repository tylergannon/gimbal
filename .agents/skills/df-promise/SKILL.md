---
name: df-promise
description: "Create, run, resume, and verify repository Promise Loops: bounded goal loops that accumulate durable evidence toward a named promise and issue a README badge only when its fulfillment gates pass. Use when the user invokes df-promise, asks for a Promise Loop or repository warranty badge, or wants recurring security, brand, performance, memory, reliability, or compliance assurance with hourly, daily, continuous, or on-change revalidation."
---

# Promise

A Promise Loop is a specialized goal loop attached to one repository. It works
toward a precisely defined promise, accumulates evidence and resource usage over
bounded resumable runs, and earns a named README badge only when every
fulfillment gate passes. The badge is an attestation to a reviewed commit,
subject fingerprint, contract/tool fingerprint, evidence set, verifier, and
time—not a timeless claim about the repository.

The contract is runner-independent. CI may execute a cheap Promise, but a human,
local agent, lab, vendor, or planned audit campaign may execute another. Never
assume a scheduler, clean worker, artifact store, or spending authority exists;
declare the runner and economics before fulfillment work begins.

Examples include a blue-team vulnerability review, brand and typography
consistency, performance, memory, reliability, accessibility, and narrowly
defined compliance controls. Do not turn labels such as "secure" or "SOC 2"
into absolute claims. Define the exact scope, standard, freshness window,
evidence, and authority that make the promise defensible.

Resolve the bundled state helper dynamically so repository- and user-scoped
installs both work:

```bash
PROMISE_SKILL_DIR="$(find .agents ~/.agents ~/.codex -maxdepth 5 -type d -name df-promise 2>/dev/null | head -1)"
PROMISE_PROJECT_ROOT="$(git rev-parse --show-toplevel)"
PROMISE_HELPER="$(cd "$PROMISE_SKILL_DIR/scripts/promise" && pwd)"
test -f "$PROMISE_HELPER/go.mod" || { echo "df-promise helper not found" >&2; exit 1; }
GOWORK=off go run -C "$PROMISE_HELPER" . --project "$PROMISE_PROJECT_ROOT" --help
```

Run these commands from the target repository root. Do not vendor the generic
helper under `.promises/`; only promise-specific tools belong there. A
project-scoped skill installation is the supported way to pin the helper with
the repository for CI or another runner.

The helper is a self-contained Go module beside this skill. `GOWORK=off go run -C` selects
that module, while `--project` selects the target repository; capture the target
root before changing directories. The target's Go module and dependencies are
not used to build the helper. Go and Git are the runtime prerequisites.
In prose, `promise <command>` names the corresponding helper command below.

Core command forms are:

```bash
GOWORK=off go run -C "$PROMISE_HELPER" . --project "$PROMISE_PROJECT_ROOT" init <promise-id> \
  --name "Promise name" --statement "Exact attestable condition" \
  --include "src/**" \
  --gate inventory="Scoped inventory is covered" \
  --plan-step "Inventory the scoped subjects and stale coverage." \
  --runner "human-invoked local agent" \
  --evidence-mode internal --verifier "tools/check.sh" \
  --cost-band lt-10 --cost-basis "Local deterministic checks." \
  --max-run-cost-usd 9.99
GOWORK=off go run -C "$PROMISE_HELPER" . --project "$PROMISE_PROJECT_ROOT" validate <promise-id>
GOWORK=off go run -C "$PROMISE_HELPER" . --project "$PROMISE_PROJECT_ROOT" start <promise-id>
GOWORK=off go run -C "$PROMISE_HELPER" . --project "$PROMISE_PROJECT_ROOT" record <promise-id> <run-id> \
  --kind inspection --result pass --subject path/to/file --gate inventory \
  --evidence ".promises/<promise-id>/evidence/report.json" \
  --summary "What was inspected and what the evidence showed."
GOWORK=off go run -C "$PROMISE_HELPER" . --project "$PROMISE_PROJECT_ROOT" finish <promise-id> <run-id> \
  --verdict fulfilled --gate inventory=pass --verifier "tools/check.sh" \
  --cost-usd-total 0 \
  --summary "Why the contract is fulfilled."
GOWORK=off go run -C "$PROMISE_HELPER" . --project "$PROMISE_PROJECT_ROOT" status [<promise-id>]
GOWORK=off go run -C "$PROMISE_HELPER" . --project "$PROMISE_PROJECT_ROOT" retire <promise-id>
GOWORK=off go run -C "$PROMISE_HELPER" . --project "$PROMISE_PROJECT_ROOT" restore <promise-id>
GOWORK=off go run -C "$PROMISE_HELPER" . --project "$PROMISE_PROJECT_ROOT" badge
```

Repeat `--gate` and `--plan-step` as needed. `--human-signoff` also requires a
`human-signoff=...` gate so an automated result cannot silently stand in for a
person. When the contract requires per-run approval, `start` also requires a
non-empty `--approval-ref` such as an approval record, ticket, or signed plan.
The helper records this reference but cannot prove its external authority; the
operator must verify it before starting.

## Model selection

Before model preflight or substantive work, read [references/models.md](references/models.md)
and run the adjacent `scripts/resolve_models` for the target project. Apply
skill defaults, then `~/.config/diffusion/skills/models.yaml`, then the project's
`.diffusion/skills/models.yaml`. Use the resolved model and effort for every
role; the reference describes main-worker handoff and prevents recursive handoff.

```yaml
model_defaults:
  main:
    model: inherit
```

## Choose a mode

- **setup**: The user wants a new Promise Loop, or the named promise has no
  contract under `.promises/`. Run the required interview, plan the loop, create
  state, and build its tools. Setup does not earn a badge.
- **run or resume**: A contract exists and the user wants progress or scheduled
  revalidation. Execute one bounded run, resuming its active run id if present.
- **status**: Report contracts, active runs, coverage, evidence posture,
  resource totals, badge validity, expiry, and blockers without changing state.
- **reconfigure**: The promise, scope, gates, tools, budget, or cadence changes.
  Re-interview only the affected decisions, update the contract and plan, and
  invalidate the old badge until a fresh run fulfills the new mechanism.
- **retire**: Use `promise retire <promise-id>` to remove the badge and
  registry entry without deleting its contract, runs, or evidence. Finish an
  active run first. Use `promise restore <promise-id>` before running it
  again; a retired promise cannot validate as runnable, start, record, or finish
  runs. Restore reissues a retained attestation only when its scope, mechanism,
  and freshness are still valid; otherwise run the loop again.

Contract schema 2 adds required runner, economics, evidence mode, and verifier
fields. Migrate schema 1 contracts explicitly with `promise migrate
<promise-id>` plus the required runner, evidence, verifier, and cost flags shown
by `migrate --help`; do not invent cost or approval answers. Migration is
blocked while a run is active and regenerates `PROMISE.md` for review.

## Durable repository state

Use a tracked hidden directory at the repository root:

```text
.promises/
├── registry.json
├── .gitignore                  # ignores transient work and atomic scratch files
├── .work/                      # ignored registry lock and atomic scratch files
└── <promise-id>/
    ├── PROMISE.md              # human-readable contract and operating plan
    ├── contract.json           # machine-readable gates, scope, budget, cadence
    ├── state.json              # current state and lifetime resource totals
    ├── coverage.json           # latest result for each file/screen/subject
    ├── tools/                  # tools used only to fulfill this promise
    ├── evidence/               # small durable evidence or content-addressed pointers
    ├── runs/<run-id>/
    │   ├── run.json            # bounded run summary and exact/partial metrics
    │   └── events.jsonl        # append-only inspections, tests, findings, decisions
    └── .work/                  # ignored scratch: large, sensitive, or transient artifacts
```

This state belongs to the repository because the promise, evidence, badge, and
invalidation rules must survive agent sessions and be reviewable with the code.
Keep it compact: store summaries, hashes, source paths, request ids, and durable
artifact references. Never commit secrets, raw credentials, unnecessary source
copies, or large screenshot/video collections. Put transient material in
`.work/` and retain a hash or durable external reference when the contract needs
it.

Promise-local tools live in `.promises/<promise-id>/tools/`. They may inspect a
color palette, enumerate screens, run a scanner, or call another model, but they
must not become a general repository tool drawer. Declare every tool and its
purpose in `contract.json`; keep credentials outside the repository. When a
contract requires a named provider/model or independent reviewer, record the
actual identity and fail closed rather than silently substituting.

## Setup: interview before building

An initial setup interview is required. Reuse answers already supplied by the
user or repository; do not ask them twice. Resolve five decisions before writing
the contract:

1. **Claim and scope** — What exact condition is promised, what must it not
   imply, and which branches, paths, screens, environments, and exclusions are
   covered? Repository globs are root-relative: `*` stays within one segment,
   `**` spans directories, and `?` matches one non-separator character.
2. **Evidence and truth** — Which gates pass, who or what verifies them, where
   each gate's durable evidence comes from, and what changes or age make it
   stale? Which small promise-local tools produce or verify it, and which raw
   artifacts stay ignored or external? External or mixed evidence requires a
   freshness deadline.
3. **Execution** — Who or what invokes the run, in which environment, on what
   manual, change, or periodic trigger, and with what mutation or external-call
   authority? A cadence is not a runner.
4. **Economics** — What USD order-of-magnitude band covers a full run, what is
   the estimate basis and hard cap, and what approval or staging plan is needed?
   Also bound wall time, steps, tool calls, retries, and exact tokens when exposed.
5. **Publication** — What badge label and README link represent the historical,
   commit-pinned attestation without implying current universal health?

If a proposed promise cannot be falsified or proven with available evidence,
narrow it before proceeding. Prefer "no unresolved high-severity findings in
the reviewed inventory at fingerprint X" over "this repository is secure."

Use one full-run cost band: `lt-10`, `10-100`, `100-1k`, `1k-10k`,
`10k-100k`, `gte-100k`, or `unknown`. Bands of `100-1k` and above, plus
`unknown`, require a per-run approval reference and a positive USD cap. Bands of
`10k-100k` and above also require a staged execution plan. A `gte-100k` Promise
is a governed project: the loop records the approved plan and evidence; it does
not autonomously authorize the spend.
Do not incur material setup cost before that approval. During a run, record
direct USD cost at checkpoints, stop at the cap, and supply the exact total at
finish whenever `max_run_cost_usd` is configured.
If actual cost exceeds the cap, record the overrun as a blocked `blocker` event
and finish `budget-exhausted`; do not hide it or continue fulfillment work.

## Setup: plan and implement

After the interview:

1. Read `AGENTS.md`, relevant architecture/testing docs, existing CI, README
   badges, and any existing `.promises/` state. Preserve unrelated work.
2. Write a concise fulfillment plan with inventory, gate evidence, tool design,
   exact verifier, invalidation, runner, economics, budgets, and any schedule
   adapter. The plan must make interrupted and later runs resumable.
3. Initialize the contract with the helper. Use `init --help`; supply each gate
   as `--gate id=description`, at least one explicit repo-relative scope glob
   with `--include`, each plan step with `--plan-step`, and each promise-local
   tool as `--tool tools/path=purpose`.
4. Build declared tools only under `.promises/<promise-id>/tools/`. Prefer small
   deterministic programs with machine-readable output. Test them against
   representative fixtures or a safe repository sample.
5. Refine the generated `PROMISE.md` and `contract.json` when the interview
   requires detail the CLI flags cannot express. Keep both consistent.
6. Run `promise validate <promise-id>`. Commit the configuration and tools
   before a badge-bearing run unless the user explicitly accepts a dirty-scope
   review.
7. Add a scheduler only when the user asked for one and the execution target is
   known. A local automation, CI workflow, or external runner is an adapter to
   the same checked-in contract; it is not a second source of truth.

A scheduler is incomplete until its runner can resolve an installed, pinned
copy of the helper, such as the project-scoped skill installation above. Do not
assume the developer's global skill path exists in CI or on another machine.
External or mixed evidence is incomplete without a durable reference such as a
content hash, provider request id, signed report, or stable artifact URL.
The helper accepts a file under the Promise directory for internal evidence, or
an `https`, `s3`, `gs`, `artifact`, `provider`, or `request` URI (or
`sha256:<digest>`) for external evidence.

During the short interval after `init` and before declared tool files exist,
`status` remains readable and `validate --allow-incomplete-tools` can inspect the
rest of the setup. Final `validate` and `start` require every declared tool and
fail closed if one is missing. If a declared tool disappears during an active
run, `record` remains available for the blocker and `finish` can close the run as
`blocked` or `budget-exhausted`; `fulfilled` still fails closed.

"Continuous" means repeated bounded invocations with persisted state, not one
immortal agent process. Each invocation must end with a finished run or a
recorded checkpoint; the next `start` resumes an active run. Contract budgets
bound the active run across those invocations, while the scheduler separately
bounds each invocation. Avoid overlapping runs of the same promise. The helper
serializes state mutations so concurrent scheduler ticks fail cleanly instead
of racing registry, run, or badge writes.

## Run or resume the loop

1. Run `promise validate <promise-id>` and read `PROMISE.md`, `contract.json`,
   `state.json`, recent run summaries, and relevant coverage. Do not ingest old
   raw artifacts unless the current gap requires them.
2. Start with `promise start <promise-id>`. If an active run exists, the
   helper returns its run id for resumption. By default, a dirty scoped worktree
   is rejected so the reviewed commit remains meaningful. If the contract and
   user explicitly allow `--allow-dirty`, the dirty paths are recorded and any
   resulting badge is visibly marked `dirty`.
3. If the runtime exposes a goal-loop primitive and this invocation explicitly
   asks to run the Promise Loop, create or resume a goal for this bounded run.
   The goal is orchestration state, not proof of fulfillment.
4. Determine the smallest uncovered or stale slice from the subject inventory,
   commit history, `coverage.json`, and invalidation rules. Review that slice
   deeply using the promise-local tools and the contract's permitted authority.
5. Record each material inspection, test, visual review, finding, blocker,
   decision, and verification with `promise record`. Use `--gate` for every
   gate the event proves and attach a durable artifact path, hash, URL, or
   provider request id with `--evidence`. A fulfilled run requires passing
   evidence linked to every contract gate.
6. Repeat inventory → inspect → record → assess until all gates pass, a budget
   is reached, authority is missing, or evidence blocks progress. Fix findings
   only when the contract and current user request authorize mutation.
7. Run the promised verifier. Its stable finish identity must exactly match the
   contract; record tool/model versions or mechanism hashes in the evidence.
   Record human sign-off explicitly and never infer it from automated checks.
8. Finish with one result per contract gate and one of `fulfilled`,
   `not-fulfilled`, `blocked`, or `budget-exhausted`. Supply exact runtime totals
   for steps, tool calls, and tokens when the host exposes them. Otherwise the
   helper records partial or unavailable token accounting rather than inventing
   a number.

Wall time accumulates active segments across resumed invocations. On resume, the
helper closes an interrupted segment at its last recorded activity and excludes
the idle gap, so record a final checkpoint before yielding. A fulfilled run
requires recorded evidence, all gates passing, unchanged subject and mechanism
fingerprints, a named verifier, and respected budgets. Non-fulfillment is a
valid run outcome and must not be hidden by retries.

## Badge truth

The helper manages only the README block between
`df-promise-badges:start` and `df-promise-badges:end`. It emits one green badge
per currently valid promise, linked to that promise's `PROMISE.md`, with the
fulfillment date and reviewed commit in the badge text. Setup, test execution,
or a merely active goal never earns a badge. Without a live runner, the badge is
a historical attestation to that commit and evidence—not a self-refreshing CI
status indicator.

Scoped content, the promise contract, or promise-local tools changing makes the
attestation stale. A freshness deadline can also expire it. A later
`not-fulfilled` run or any failed gate withdraws the prior attestation; a merely
blocked or budget-exhausted run preserves it when the scope, mechanism, and
freshness remain valid. Re-run the loop and use `promise badge` to reconcile
the managed block. Evidence-only commits under `.promises/` and edits confined
to the managed badge block do not alter the subject fingerprint, so the
attestation can be committed without invalidating itself.

## Domain patterns

For a blue-team vulnerability promise, inventory the attack surface and changed
files, combine deterministic scanners with source inspection, log each reviewed
file and commit, track findings through disposition, and require a final
security-specific verification gate. Default to read-only review; remediation
and disclosure need separate authority.

For a brand and typography promise, maintain a meaningful-screen manifest,
capture reproducible screenshots, test design tokens/CSS and palette rules,
perform visual review against explicit brand criteria, and require human taste
acceptance when the promise says it represents brand quality. DOM/CSS tests do
not substitute for visual evidence, and model review does not substitute for a
named human gate.

## Completion report

Report the promise id and statement, reviewed commit/fingerprint, coverage
advanced, tools and verifiers actually used, gate results, findings/blockers,
runner, cost band, cap, approval reference, run and lifetime resource metrics,
badge status, evidence paths, and next invalidation or scheduled run.
Distinguish configured, executed, fulfilled, badge-issued, scheduled, and
human-accepted states.
