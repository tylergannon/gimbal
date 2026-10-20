# Independent spec and plan review: durable Interactive execution (round 03)

Reviewer: Claude Opus 5.5, independent adversarial pass. I edited only this
file.

## Target

- nlspec: `specs/durable-interactive.md` (578 lines).
- Delivery plan: `ephemeral/research/durable-interactive/DELIVERY-PLAN.md`
  (139 lines).
- Both as committed at `5c0c419f` (`docs: resolve durable interaction spec
  review and delivery boundaries`). That commit landed while this review was
  in progress. The working tree was clean when I re-read it, and the spec and
  plan text matched what I had reviewed.

The launch prompt names the targets, the reference documents and the output
path. It does not narrow scope, predict findings or ask for a verdict, so no
caller instruction was ignored. The restriction to edit only this file is a
valid operating constraint.

## Evidence inspected

- Standards: `spec-writing/SKILL.md` and `notation.md`, `spec-review/SKILL.md`,
  and `adversarial-review/SKILL.md`.
- Repository instructions: `AGENTS.md`, in particular "The rule", "No
  wrappers", "Build what was asked", "Proof is what you saw" and "Working
  here".
- Spec set: the nlspec, the plan, `SOURCE-MAP.md`, `REVIEW.md` (including its
  round-02 adjudication), the pointer `SPEC.md`, the worklog
  `ephemeral/worklog/durable-interactive-spec.md`, and the round-02 review.
  I also diffed the nlspec against the round-02 text at `13ee0140`.
- Prior art: the token-cache entrypoint and the DBOS facts that
  `SOURCE-MAP.md` cites, which I used for its stated limits.
- Current source:
  - Public API: `go doc -short .`, `go doc . HarnessAdapter`, `Session`,
    `CommandEnded`, `Check` and `Steer`.
  - Root package: `command.go` (capture, excerpt and intent recording),
    `run_recording.go:97-164`, `group.go:40-99`, `run.go:260-300`,
    `interview.go:50-80` and `registry.go:128-169`.
  - Hosting: `internal/host/host.go:330-410`, `internal/host/routes.go:95-160`,
    `internal/runner/{launch,runner,daemon}.go`,
    `internal/invocation/invocation.go` and `cmd/gimbal/main.go:262-340`.
  - Remote backend: `backend/backend.go:121-162`.
  - Conversations: `internal/conversation/{manager,requests,binding,remote}.go`.
  - Adapters: `codex/codex.go:370-420`, `claude/claude.go:111-136,269-276`,
    and `CreateSession` in `pi`, `agy` and `opencode`.
  - Maintained workflows: `internal/workflows/{validateproduct,researcheval,researchdocument}`
    and `cmd/examples/**`.
- Mechanical recount of all six cell baselines under the spec's counting rule
  (script in my scratchpad, not committed).

## Round-02 disposition

All five round-02 findings were materially addressed in the text:

- F1 (ingress identity): a stable run/scope/name address was added, with
  occurrence claims and an `unprocessed` disposition.
- F2 (entry mode): entry is human-first for every occurrence.
- F3 (authoring contract): admission covers all workflows, with RunCommand as
  the effect path, migration ownership, and rules on concurrency and map
  iteration.
- F4 (code identity): `SameCodeIdentity` is now the executable digest plus
  plugin digests.
- F5 (ownership and counting): ownership sets and the counting rule were
  corrected.

The findings below are new, or are consequences of those resolutions.

## Pass summary

| Pass | Result |
| --- | --- |
| Consistency | Defect: "Conversation workflows remain interactive until the human archives them" contradicts the rule that any rebuild leaves dormant runs permanently blocked (F1). Otherwise terminology is stable. Interaction address, occurrence, disposition and EndedBy are used the same way across prose, pseudocode and claims. |
| Claim traceability | Every cell routes at least one claim, and every external seam has a live claim. L1 assumes a native-resume capability that only one of the five offered harnesses has, and that no cell owns (F2). |
| Seam audit | All six cells have a responsibility, boundary, contract and budget, and their reasons to change differ. Baselines reproduce exactly: J 913, E 3,923, I 102, H 5,329, P 13,787, A 11,526. P reproduces only if TypeScript files with a `// Code generated … DO NOT EDIT.` header are excluded. The rule (`specs/durable-interactive.md:128-131`) defines that header only for Go, and the P set names files that are entirely skgo/polytype output (`web/src/lib/workflow/types.ts`, `*.remote.ts`, `+page.server.ts`). This is a nitpick: extend the header rule to TypeScript. Adapter packages (`claude/`, `agy/`, `pi/`, `opencode/`, `codex/`) that L1 needs belong to no cell (F2). |
| Claim classification | 12 cell-local and 4 integration claims. The labels and routing are plausible. The decomposition is not dominated by integration claims. |
| Derivability | Spanning failures: how suspension releases Go execution relative to authored `defer`/`recover` (F3); whether context deadlines and cancellation observations are admitted, and what they mean across replay and dormancy (F4); which messages a returned transcript contains (F5). A lower-severity gap below the five-finding cut: whether workflows placed on a non-local workflow route get durable ingress. `backend.Execution` (`backend/backend.go:133-148`) has no input-submission operation, and the spec does not say whether remote workflow placement is in scope. |
| Pump hygiene | Partly satisfied. The worklog now records the human approving complete RunCommand output (worklog lines 116-117). Rebuild-blocking of dormant runs appears only as the author's adjudication of round-02 F4 (`REVIEW.md:60-62`), with no recorded human decision. Its product consequence is material (F1). |

## Findings

### F1 — critical — Every rebuild permanently strands every open conversation, and a blocked run has no way to be archived or ended

Passes: consistency, pump hygiene. Location:
`specs/durable-interactive.md:30-31`, `395-410`, C4 (`507`), L1 (`516`).

The spec makes two commitments:

- "Conversation workflows remain interactive until the human archives them"
  (`30-31`).
- `SameCodeIdentity` is the executable digest plus the sorted digests of the
  loaded plugins. A dormant run whose identity differs "is visibly blocked
  awaiting its matching build … retention/restoration of old builds are not
  promised" (`395-407`).

L1 now requires that blocking (`516`).

Current source makes the scope of this concrete:

- The host loads one owner-wide plugin set, which every project's
  invocations inherit (`cmd/gimbal/main.go:304,335`,
  `internal/host/host.go:338-340`). Any plugin rebuild therefore changes the
  identity of every project's runs.
- The plan places the ordinary conversation workflow among the built-ins
  (`DELIVERY-PLAN.md:13-15`). It is linked into the `gimbal` executable, so
  every `just build`, `go install` or release changes the executable digest of
  every conversation.
- Today, conversations survive restarts and rebuilds. They reload from
  persisted state and resume the native session lazily
  (`internal/conversation/manager.go:216-234`).

Under this spec, every open chat tab becomes blocked after each Gimbal
upgrade. Nothing builds or keeps the old artifact, so "awaiting its matching
build" means forever in practice.

The blocked state also has no exit:

- Archive is defined as a finish signal that makes `Interactive` return, so
  the workflow completes "through its ordinary return path" (`39-43`,
  `446-447`). That requires running the workflow code the host refuses to
  run.
- "Explicit cancellation retain[s] their normal lifetime behavior" (`390-391`),
  which also unwinds through workflow scopes.
- The spec defines no way to close, abandon or terminally resolve a run that
  cannot be reconstructed. Its ingress keeps accepting messages (`266-279`),
  and they stay pending indefinitely.

So blocked tabs pile up and stay unfinished: neither interactive nor
archivable. That contradicts `30-31` and the archive goal.

The round-02 review asked for a human decision on exactly this product
behavior. The record shows only the author's adjudication ("F4 accepted with
the stated same-code boundary", `REVIEW.md:60-62`). There is no human
decision in the worklog, where the other public-behavior changes in this
round were recorded.

Impact: the delivery would ship a guaranteed regression in conversation-tab
continuity on every release, with no recovery path. L1 would require
demonstrating that regression. Implementers cannot invent the missing exit
without a contract decision. The options are a terminal "abandon" outcome, a
blocked-run archive that records no successful return, or keeping old builds,
and each changes P, H and the archive claim.

Proposed amendment (HITL):

1. Bring the rebuild consequence to Tyler explicitly, including built-in
   conversations being stranded by every Gimbal release.
2. Define what a human can do with a blocked run, and its terminal outcome:
   for example, an explicit "abandon" that is distinguishable from success and
   cancellation.
3. Say whether the identity is the host-wide plugin set or only the
   artifacts that define the run's workflow.
4. Or narrow the identity enough that unrelated rebuilds do not block runs,
   and accept the stated replay risk.

Then align `30-31`, C4, L1 and P's honesty claim with that decision.

### F2 — critical — L1 needs native session resume after restart, which only Codex supports, through a conversation-only interface that no cell owns

Passes: claim traceability, seam audit, derivability. Location:
`specs/durable-interactive.md:59-61`, `370-373`, `521-523`, exclusion
`533-538`, L1/L3/L4 (`516-519`), `153-157`.

L1 requires "native session continuity" after a host restart, using
whichever cheap alias the harness supports: `luna`, `haiku` or `flash`
(`521-523`). E must reconstruct a session binding "including native identity"
(`371-372`).

The public `HarnessAdapter` (`harness.go:25-60`) can only `CreateSession`,
`RunTurn`, `Steer`, `Fork` and `Close`. It has no way to rebind a recorded
native session in a fresh process.

The only resume path today is the unexported `persistentSessionAdapter`
(`internal/conversation/manager.go:126-131`), with `ResumeSession` and
`DetachSession`. Only `codex/codex.go:377` implements `ResumeSession`, and its
doc says "workflow sessions do not use this lifecycle" (`codex/codex.go:376`).

Claude mints an adapter session ID into an in-memory map
(`claude/claude.go:125-136`) and rejects any ID it does not hold
(`claude/claude.go:269-276`). After a restart, a reconstructed Claude session
cannot run a turn. Conversations already fail in that case for non-Codex
providers ("this provider cannot resume a saved native session",
`internal/conversation/requests.go:179-181`), yet the conversation setup
offers claude, codex, agy, opencode and pi
(`internal/conversation/manager.go:379`).

The spec replaces the conversation runtime with ordinary workflow sessions
(`424-426`). It does not say:

- which harnesses must support restart continuity;
- through which contract: a new method on the public `HarnessAdapter`
  interface, which is a public-surface change the plan's "no new public names"
  rule (`DELIVERY-PLAN.md:30-31`) does not cover, or the private conversation
  interface carried into E;
- what a session on a harness without resume does after a restart (visibly
  unresolved, or failed).

The native-resource exclusion (`533-538`) does not settle any of this.
Restoring a binding is a correctness capability, not a detach or retain
policy. The adapter packages belong to no cell. `153-157` requires such
changes to be "assigned and budgeted before implementation", and neither the
spec nor the plan assigns them.

Impact: L1, L3 and L4 can pass only on Codex. With `haiku` or `flash` they
fail, or they pass only after unbudgeted adapter work and an unapproved
interface change. Two implementers will diverge between a public interface
change, a private type assertion, and a Codex-only feature.

Proposed amendment (HITL):

1. Name the harnesses required to provide restart continuity in this delivery.
   If only Codex, require the live claims to run on `luna`.
2. Decide the resume contract: either a public `HarnessAdapter` method
   (Tyler must approve it by name) or an internal optional capability.
3. State the visible outcome for harnesses without resume.
4. Assign the affected adapter packages to a cell with a budget.

### F3 — critical — How suspension releases Go execution is unspecified, and the choice decides whether authored `defer` and `recover` break the wait contract

Passes: derivability, seam audit (A/E boundary). Location:
`specs/durable-interactive.md:10-12`, `73-76`, `376-393`, `449-457`, C5
(`508`).

The goal is to "remain logically unfinished at an explicit wait while
releasing active execution" (`10-12`). A dormant run "can release its
activation" (`383-386`). Waiting "cannot trigger Group's error cancellation,
finish a scope, or allow code after an unfinished join to run" (`378-381`).
Suspension "does not run normal scope teardown" (`389`).

The spec covers library scope teardown only. It never says how an
activation's goroutines stop running when a run goes dormant. That question
has two divergent answers:

1. **Park the goroutines.** Nothing is released, and a long-lived shared host
   accumulates one parked stack per open conversation tab. "Releasing active
   execution" becomes nominal.
2. **Unwind them**, by panic or `runtime.Goexit`. Go then runs every authored
   `defer` on the way out:
   - `validateproduct.go:108,135` writes a report and closes browsers.
   - `researcheval.go:169` deletes the suite directory.
   - After migration, these become RunCommand calls. They would run at
     suspension and record new operations, which is the teardown C5 forbids.
   - An authored `recover()` can swallow the unwind and continue past the
     wait, violating `378-381`.
   - Group itself recovers child panics into a `panicError` that cancels
     siblings (`group.go:70-85`).
   - `Run.root` records a terminal end before re-panicking (`run.go:273-279`).

   Every one of these paths must be special-cased, and A's admission rule
   must then forbid or constrain authored `recover` and effectful `defer`.

A's admission contract (`449-457`) and E's suspension contract (`376-393`)
both depend on this choice, and neither states it. C5 can be "demonstrated"
under either answer while meaning different things.

Proposed amendment (HITL): state whether a dormant activation's Go
execution is retained or unwound. If unwound, state the rules for authored
`defer` and `recover` at suspension: admitted, rejected, or run only on actual
completion. Add the corresponding admission rule to A and the corresponding
observation to C5.

### F4 — issue — Context deadlines, sleeps and cancellation observations are neither admitted nor forbidden, and maintained workflows depend on them with no library replacement

Passes: derivability, claim traceability. Location:
`specs/durable-interactive.md:80-102`, `449-457`, exclusion `539-540`, C12
(`515`); `DELIVERY-PLAN.md:66-73`.

The spec forbids "clocks … and equivalent operations" in workflow functions
and helpers. It routes "clock reads" through RunCommand, and it adds no
clock API (`80-97`). It does not mention `context.WithTimeout`,
`WithDeadline`, `time.Sleep`, tickers or `ctx.Err()`. Each of these is a
clock read or an unrecorded timing observation that can change control flow.
The maintained workflows that A must migrate depend on them:

- **Deadlines around agent turns and phases:**
  - `researcheval.go:337,344,379,417,480` (turns of one minute to N minutes);
  - `validateproduct.go:101,163`.
- **Sleep backoff:** `researchdocument.go:406,449,606,649`.
- **Polling another run with a ticker, `ctx.Err()` and an internal
  CancelScope control:** `researcheval.go:693-722`.

RunCommand can wrap a shell `timeout` or `sleep`. Nothing in the existing
library puts a deadline on a `Generate` turn, and the exclusions forbid adding
one.

The library is also already entangled with deadlines. RunCommand records the
wall-clock `ctx` deadline in its durable intent
(`command.go:213` via `operationDeadline`, `remote_execution.go:188`). A
replay under a recomputed `WithTimeout` therefore presents different intent
input for the same operation. The spec also does not say whether a deadline
that spans a durable wait keeps running while the run is dormant.

Two readers diverge:

- One rejects deadlines and removes the turn timeouts from researcheval and
  validateproduct. That changes their cost and time bounds with no approved
  replacement.
- The other admits deadlines and leaves replay and dormancy semantics
  undefined.

Either way, C12's "maintained built-ins/examples pass the same admission
checks after migration" cannot be judged against one rule.

Proposed amendment (HITL): state whether `context` deadlines and timeouts,
and `ctx.Err()`/`ctx.Done()` checks, are admitted in workflow code. If they
are admitted, state what a deadline means across replay and dormancy (for
example, wall-clock, recorded at creation) and how recorded intent treats it.
If they are rejected, name the migration for turn timeouts using existing
operations, or accept losing them in the affected workflows.

### F5 — issue — Which messages a returned transcript contains is underdetermined at the public contract

Passes: derivability, consistency. Location:
`specs/durable-interactive.md:33-37`, `214-217`, `266-273`, `322-325`,
`332-338`; C6 (`509`).

The spec says the transcript "contains durably accepted human messages and
committed assistant chat replies" (`322-323`). It also says "the transcript of
each call covers that call's interaction" (`216`). Pending input may be
accepted during one occurrence and delivered in the next (`266-270`,
`336-337`). At human finish, undelivered messages become "visibly unprocessed
… never acknowledged as delivered" (`334-335`). Delivery can also remain
`unresolved` (`313-314`).

That leaves three open questions:

1. Does a transcript include a user message that was accepted but finished
   as `unprocessed`, or is still `unresolved`, which the agent may never have
   seen? "Accepted" says yes; "never acknowledged as delivered" suggests no.
2. Does a message accepted during occurrence N but delivered in N+1 belong to
   N's transcript, N+1's, or both?
3. Is "recorded order" acceptance order or delivery order when the two
   differ?

Downstream workflow code consumes the transcript as conversation history. The
replaced Interview fed its transcript to an implementer, as
`implementinterview` does. A transcript that includes messages the agent
never received misreports the conversation. C6 cannot settle these questions
because it only asserts "ordered transcripts".

Proposed amendment: define transcript membership by delivery disposition
and occurrence. For example: "contains user messages delivered to this
occurrence's turns, in acceptance order, plus its committed replies;
unprocessed or unresolved inputs are excluded and visible only in the
conversation view". Add that rule to C6.

## Plan notes

The delivery plan is consistent with the nlspec and with the latest worklog
decision (two workers, one delivery unit, one final validation pass). It
correctly forbids proof artifacts, resource-policy tests and nlspec edits by
implementers. It inherits F1–F5 directly:

- Its control-flow sketch and work items 1–3 cannot be completed against the
  live claims until F2's adapter ownership and contract are decided.
- Work item 5's migration depends on F3 and F4.
- "No new root public names" (`DELIVERY-PLAN.md:30-31`) does not cover a new
  method on the public `HarnessAdapter`, which F2 may require.

## Outcome

material findings remain
