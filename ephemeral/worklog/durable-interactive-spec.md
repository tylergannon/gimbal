# Durable Interactive spec authoring

decision: User requests durable interaction and resumption, replacing Interview
with Interactive; SERVICE lifecycle and restoration remain explicitly deferred.

decision: Three read-only subagents mapped execution/lifetimes, interactions and
conversation UI, and indexed DBOS prior art before beginning the interview.
Current facts and source anchors are in
ephemeral/research/durable-interactive/SOURCE-MAP.md.

correction: Cached DBOS routing prose overstates memory release while waiting;
actual Go Recv retains a waiting call stack. Its HITL publish and recv are also
separate operations. Neither establishes Gimbal's requested suspension or
atomic reply-and-wait contract.

decision: Existing runstore supplies stable node identities and atomic committed
batches, but Open explicitly does not resume execution. Recorded results must
not be mistaken for working replay, and current persistence tests do not prove
resumed-work deduplication.

decision: Human messages arriving during an active agent turn steer that turn
(Tyler: "steer."). Durable inbox acceptance still precedes delivery.

decision: Human archive is successful completion. Tyler prefers a signal that
makes the workflow exit Interactive and actually finish normally, and permits
deferring that signal if it is hard. Proposed finish input shares the required
durable inbox; handling archive during an active turn remains open.

decision: Archive interrupts an active turn unless doing so would leave
unrecoverable or corrupt state. Interruption is turn-scoped; the workflow
continues normally after safe cessation. An unconfirmed stop remains
recoverable/unresolved rather than being reported as successful completion.

correction: Tyler forbids direct I/O, clocks, randomness and equivalent
nondeterministic operations in workflow functions. They must use the library;
an author promise that arbitrary operations are replay-safe is insufficient.
Ordinary deterministic Go control flow remains. A possible Activity concept
is future work, not part of this change.

correction: Batch independent spec questions where plausible; Tyler explicitly
overrides the grilling skill's one-question-at-a-time interview format to
avoid unnecessarily prolonged authoring.

decision: Tyler confirms Interactive returns the ordered transcript plus an
error and that runs from different projects may share a host within the same
execution environment, with independent run state and bindings.

decision: Proposed cells separate durable records/inbox, execution/replay,
interaction protocol, shared hosting, presentation, and workflow admission.
Physical handwritten-source baselines were measured before proposing budget
tripwires. The existing linter follows source-visible helpers but does not yet
enforce the new library-only effects rule.

decision: Tyler approves InteractiveTranscript/InteractiveMessage and the
ordered role/message shape. Support both initial entry modes eventually;
choose the easiest correct starting default and do not over-engineer it.

correction: Native agent resource handling explicitly has NO POLICY and NO
tests purporting to enforce one. Choose the easiest default that behaves
correctly. Do not turn detach/retain/eviction/process counts into requirements
or frame this as a policy-design task deferred to implementation.

decision: Full review draft adds durable ingress/transition, replay, hosting,
presentation and admission contracts and routed behavioral claims. One open
public-contract defect remains: generic end and human finish both return nil,
but conversation workflow code must distinguish them to remain interactive
until archive. Proposed termination provenance needs human alignment; it has
not been silently added to the approved public types.

decision: Author's six-pass spec review is recorded in
ephemeral/research/durable-interactive/REVIEW.md. It leaves termination
provenance as an explicit public-contract finding. Cell budgets and derived
contracts remain proposed; no implementation or live proof is claimed.

decision: Tyler approves EndedBy ("agent" or "human") on InteractiveTranscript
("love it"). The result durably distinguishes normal agent end from human
finish. The conversation uses an ordinary loop at one Interactive call site,
returning success for human finish and re-entering a durable input wait after
agent end. Pending input and finish signals retain identity across re-entry;
no autonomous agent-end loop or startup-mode framework is required.

decision: Rechecked the EndedBy amendment against all six spec-review passes.
The author's review has no unresolved material finding. The completed draft
is ready for final review of cells/budgets/derived contracts; independent
review and implementation acceptance have not been claimed.

decision: Tyler authorizes an independent Claude Opus nlspec review followed
by implementation in a Sol 6.1 subagent (possibly two), explicitly asking to
avoid excessive splitting and repeated proof. Plan one end-to-end implementer
and one shared final acceptance pass; the manager owns specification/review
adjudication and does not duplicate unchanged broad checks.

friction: Opus review run 01M5BDHYRPKYNJGQBKZK263ZF7.run-prompt failed on
ENOSPC before producing a review artifact. Its logs were only about 2 MB; the
data volume had roughly 117 MB free. Reclaimed 8 GiB of rebuildable Go cache
entries older than 48 hours, leaving recent cache and source/session/app data
alone; measured free space then reached about 9 GiB. No failed run is counted
as an independent review.

decision: Fresh Opus review run 01M5BDTDK25XKQEFXHHWCNGXCQ.run-prompt targets
ephemeral/reviews/durable-interactive-opus-spec-round-02.md, through Gimbal
run-prompt with the unversioned opus alias (resolved Claude Opus 5.5). It reads
the spec-writing, spec-review and adversarial-review skills. Its run logs are
outside the repository at /tmp/gimbal-durable-interactive-opus-spec-r02.

decision: Existing built-in workflows contain direct I/O and clock calls.
The implementation plan must account for their migration through library
operations rather than granting exceptions or hiding effects in helpers.


decision: Opus round 02 found five legitimate spec gaps (stable ingress versus
occurrence identity, unconstrained first-entry choice, missing built-in effect
migration, executable identity and incomplete source ownership/counting).
Adjudication is in the author's review. Promoted the maintained nlspec to
specs/durable-interactive.md and kept the old ephemeral SPEC as a pointer.
The human approved full RunCommand caller output with bounded observation.
First entry uses the delegated easiest correct human-first default.

decision: Final proposal uses two Sol 6.1 workers in one shared checkout and
one delivery unit: runtime/hosting/presentation and authoring/generation/built-in
migration. One final validation pass follows combined source and generation,
not repeated broad proof per subtask. Manager owns spec and integration.
Fresh Opus run 01M5BEMKTNQKNMWWBQ1F9C8J1A.run-prompt reviews the complete
contract and plan at ephemeral/reviews/durable-interactive-opus-spec-round-03.md.


decision: Opus round 03 completed after 9m14s. It verified the original fixes
and corrected counts, and exposed further boundary choices. Native capability
and transcript membership have direct resolutions recorded in REVIEW.md.
Asked two batched material questions: retain required builds for old-run
recovery across upgrades; reject authored unwind/timing constructs and remove
unsupported turn timeouts for the initial library-only delivery. Do not assume
approval or launch dependent implementation while these remain unanswered.
