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
