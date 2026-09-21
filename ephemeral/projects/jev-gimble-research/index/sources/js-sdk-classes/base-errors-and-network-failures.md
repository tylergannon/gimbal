# Base errors, connection failures, timeouts, and caller aborts

## Purpose

This leaf captures the non-HTTP-status side of the JavaScript SDK error hierarchy and the distinctions an operational integration must preserve between network delivery failure, elapsed-time failure, and intentional cancellation.

## Key concepts

- **All SDK errors share `TypeSafeError`.** It extends the platform `Error` and is the direct base for `APIConnectionError`, `APIError`, and `APIUserAbortError`. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/classes/TypeSafeError.md:5-18`
- **Connection failure covers both request transport and response-body delivery.** DNS, TLS, closed connections, and analogous failures become `APIConnectionError`; the constructor can preserve an `ErrorOptions` cause. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/classes/APIConnectionError.md:5-15` `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/classes/APIConnectionError.md:17-43`
- **Timeout is a specialized connection failure.** `APITimeoutError` means the full response did not arrive within the timeout, extends `APIConnectionError`, and records the configured timeout in `timeoutMs`. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/classes/APITimeoutError.md:5-12` `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/classes/APITimeoutError.md:13-51`
- **Caller cancellation is a distinct branch.** `APIUserAbortError` is raised when an `AbortSignal` cancels the request and extends `TypeSafeError` directly rather than `APIConnectionError`. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/classes/APIUserAbortError.md:5-12`
- **Constructors support causal error chaining.** The base and all three transport/cancellation constructors accept `ErrorOptions`, enabling a lower-level cause to be retained. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/classes/TypeSafeError.md:19-47` `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/classes/APIUserAbortError.md:13-39`

## Citation bookmarks

- Base hierarchy: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/classes/TypeSafeError.md:5-18`
- Connection delivery semantics: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/classes/APIConnectionError.md:5-15`
- Connection constructor/default message: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/classes/APIConnectionError.md:17-43`
- Timeout inheritance and `timeoutMs`: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/classes/APITimeoutError.md:5-51`
- AbortSignal cancellation: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/classes/APIUserAbortError.md:5-39`

## Themes

- Transport instability and deliberate lifecycle cancellation must not be conflated.
- A timeout is specifically failure to receive the complete response, not merely failure to receive headers.
- Error inheritance supports coarse catches while subclasses support precise operational accounting.

## Gotchas

- Catching `APIConnectionError` also catches `APITimeoutError`; test the timeout subtype first when metrics or fallback policy differs.
- Caller abort is not a server or connectivity fault. Treating it as retryable can revive work the owning Gimble run intentionally cancelled.
- A response-body connection failure can occur after a request reached the server; although the Jev operation is decision-only, retry accounting should still distinguish it from pre-connect DNS/TLS failure.
- These class pages do not specify retryability, retry counts, or whether the SDK wraps every platform `AbortError`; only the resulting SDK class meanings are documented.

## Task recipes

- **Implement operational classification:** handle `APIUserAbortError` as expected cancellation; `APITimeoutError` as budget exhaustion; other `APIConnectionError` instances as delivery failures; `APIError` separately as an HTTP response. Start with the hierarchy at `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/classes/TypeSafeError.md:13-18`.
- **Cancel with run ownership:** attach a Gimble run/node lifecycle `AbortSignal` to the call and suppress automated retry or escalation after an intentional abort. The documented cancellation class is `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/classes/APIUserAbortError.md:5-12`.
- **Measure timeout correctly:** record `timeoutMs` and attempt count separately; the client’s timeout is per attempt and `APITimeoutError` means the full body missed that boundary. Start at `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/classes/APITimeoutError.md:5-51`.

