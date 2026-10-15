# Conversation environment routing

decision: Desktop shutdown detaches remote readers; it never implies native Stop or releases conversation resource ownership. Explicit scope cleanup is the end-session action.
correction: A task daemon's HTTP Interrupt reply acknowledges delivery, not native cessation. Only the original terminal result establishes Stopped; process cancellation already lives in ProcessResult and must not become an unexpected service-error return.
decision: Persist backend transport envelope ULIDs separately from conversation/request identities. The filesystem message transport rejects domain UUIDs or composite strings as request IDs.
decision: The browser receives a bounded nullable placement display DTO. Opaque route configuration and retained delivery envelopes remain in a private disk record, preserving both truthful projection and unknown plugin config.
decision: Classify placement by machine identity, not environment identity; an alternate directory on This Mac still uses the local harness.
correction: Scope cleanup success from an empty/restarted daemon cannot settle an outstanding original turn. End-session must read that turn's exact retained terminal fact, preserve completion that won the race, and leave missing outcomes owned and retryable. Pending local messages must not be admitted while End is in progress.
