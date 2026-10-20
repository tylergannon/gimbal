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
