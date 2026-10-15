# Conversation environment routing

decision: Desktop shutdown detaches remote readers; it never implies native Stop or releases conversation resource ownership. Explicit scope cleanup is the end-session action.
correction: A task daemon's HTTP Interrupt reply acknowledges delivery, not native cessation. Only the original terminal result establishes Stopped; process cancellation already lives in ProcessResult and must not become an unexpected service-error return.
decision: Persist backend transport envelope ULIDs separately from conversation/request identities. The filesystem message transport rejects domain UUIDs or composite strings as request IDs.
decision: The browser receives a bounded nullable placement display DTO. Opaque route configuration and retained delivery envelopes remain in a private disk record, preserving both truthful projection and unknown plugin config.
decision: Classify placement by machine identity, not environment identity; an alternate directory on This Mac still uses the local harness.
correction: Scope cleanup success from an empty/restarted daemon cannot settle an outstanding original turn. End-session must read that turn's exact retained terminal fact, preserve completion that won the race, and leave missing outcomes owned and retryable. Pending local messages must not be admitted while End is in progress.
correction: The original observer and Stop reconciliation can receive the same retained completion concurrently. Settle under the manager mutex once, including the visible assistant message; request-row deduplication alone does not prevent duplicate chat replies.
correction: A conversation's cached HTTP binding can outlive an SSH tunnel's local port. Reacquire the immutable saved target for each operation, replacing derived handles without closing service-owned transports.
correction: Explicit messaging cleanup retries need a fresh control ID after an authoritative completed failure. Publication uncertainty retains the original ID; it never authorizes replacement work.
correction: Session admission uncertainty and turn submission are different facts. Persist attempted turn delivery separately, serialize the short admission section with End, and require an original turn result only when that turn could have been submitted.
correction: Register background operations before spawning, including local delivery, so manager Close joins operations that have not yet acquired their turn mutex. Journal errors must stop native effects before publication/admission/control.
