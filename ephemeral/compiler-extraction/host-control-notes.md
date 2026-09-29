# Hosted cancellation and retained artifacts

- decision: Stage 2 keeps the hosted entry/configuration seam in internal/host, with required backend callback, bounded timeout, initial snapshot and separate worker/host materialization directories. Public web signatures remain unpublished.
- correction: Backend notification can call finish before the console delivery callback returns. The raw compiled finish now serializes with the bounded delivery attempt; consumer FinishCancelled waits for actual root cancellation before draining. Without both ordering points, cleanup could close the observation writer before delivery status was saved or lose the reserved operator cause.
- decision: Root kill deduplication does not suppress delivery retry while the local run remains live. Once local finish removes the controller, retained history has no retry control; persistent post-finish backend control is not supplied by this local owner seam.
- friction: New observation fields must follow Polytype optionality contracts. Use Optional[T] with omitzero for optional structured fields; ordinary omitempty pointers fail skgo regeneration even when direct Go tests pass.
- decision: Artifact resolution checks the retained run's logical references, reads and verifies immutable storage, then materializes into a distinct host cache. A surviving cache cannot conceal a missing retained object.
