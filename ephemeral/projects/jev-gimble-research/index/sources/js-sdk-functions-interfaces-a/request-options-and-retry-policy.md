# JavaScript request options and retry policy

## Purpose

This leaf covers per-call overrides, cancellation, timeout semantics, and the SDK’s retry configuration. For continuous supervision, these controls determine whether a cheap semantic check remains bounded or silently becomes a long multi-attempt operation.

## Request options

- `RequestOptions` overrides client settings per call. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/interfaces/RequestOptions.md:5-8`
- `headers?: Record<string,string>` are merged over `defaultHeaders`, so per-call values take precedence. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/interfaces/RequestOptions.md:9-20`
- `retry?: Partial<RetryPolicy>` overrides retry behavior for one call; omitted fields inherit client settings. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/interfaces/RequestOptions.md:21-32`
- `signal?: AbortSignal` cancels both the active request and pending retries. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/interfaces/RequestOptions.md:33-44`
- `timeout?: number` is milliseconds per attempt; there is explicitly no total retry budget. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/interfaces/RequestOptions.md:45-55`

## Retry behavior and defaults

- Partial retry overrides inherit unset fields from client or SDK defaults. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/interfaces/RetryPolicy.md:5-8`
- Connection failures—including interrupted response bodies—retry by default; `APIConnectionError` control defaults to `true`. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/interfaces/RetryPolicy.md:11-20`
- `APITimeoutError` retries by default (`true`). `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/interfaces/RetryPolicy.md:21-32`
- Exponential backoff starts at `500ms`, doubles up to `5000ms`, and subtracts a random fraction up to `0.25` of each delay by default. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/interfaces/RetryPolicy.md:35-68`
- Retried HTTP statuses default to `408`, `429`, and `500-599`. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/interfaces/RetryPolicy.md:69-80`
- `maxRetries` counts retries after the initial attempt; default `2` permits up to three total attempts, while `0` disables retries. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/interfaces/RetryPolicy.md:81-92`
- Server `Retry-After` and `retry-after-ms` are honored by default up to `60000ms`; larger requested delays fall back to backoff. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/interfaces/RetryPolicy.md:93-115`

## Citation bookmarks

- Per-call inheritance and cancellation: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/interfaces/RequestOptions.md:5-44`
- Per-attempt timeout caveat: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/interfaces/RequestOptions.md:47-55`
- Retryable failure classes/statuses: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/interfaces/RetryPolicy.md:11-32` `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/interfaces/RetryPolicy.md:71-80`
- Backoff defaults: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/interfaces/RetryPolicy.md:35-68`
- Retry count and server delay: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/interfaces/RetryPolicy.md:83-115`

## Themes

- **Per-call control:** high-frequency routine checks and latency-sensitive steering checks can use different timeout/retry policies without separate clients.
- **Cancellation is end-to-end:** use one AbortSignal for the current attempt and its queued retry delays.
- **Retry policy is explicit data:** store effective policy with run evidence so long-tail latency and duplicate attempts are explainable.
- **Server pacing participates:** 429 handling honors bounded server delay before local exponential backoff.

## Gotchas and gaps

- `timeout` applies per attempt and there is no total retry budget. With default two retries, wall time can approach three attempt timeouts plus backoff or accepted server delay.
- Retrying timeouts/connection errors may repeat a request after the server performed work but the response was lost. The assigned docs do not state idempotency or request-deduplication guarantees.
- Default retry statuses include every 5xx. Some failures may be persistent semantic/configuration errors despite their status; retries add latency without benefit.
- Jitter is subtractive (`delay - up to fraction`), not documented as symmetric/full jitter. Capacity planning should use the actual policy rather than generic exponential-backoff assumptions.
- `maxRetryAfterMs=60000` can make a supervision check wait up to a minute before another attempt unless a task-level AbortSignal or deadline intervenes.
- Per-call headers override defaults. Accidentally overriding authorization/content headers or leaking run data through custom headers is possible; the interface documents merge precedence but not protected header names.
- The docs give configuration, not retry telemetry hooks, attempt callbacks, or a total-deadline option.

## Task recipes

1. **Bound an interactive check:** create a task-level `AbortController` deadline; pass its signal; use a short per-attempt timeout and low/zero retries when stale coaching is worse than a missed result.
2. **Run background classification:** allow bounded retries for 408/429/5xx, persist attempt count and effective policy, and keep workflow concurrency below service limits.
3. **Honor a true total budget:** compute a wall-clock deadline outside the SDK and abort it; do not treat `timeout` as total duration.
4. **Test retry semantics:** simulate connection interruption, timeout, 429 with both retry headers, a long retry header, and 5xx; verify attempts, delays, cancellation during backoff, and final error identity.
