# JavaScript request options and retry policy

## Purpose

This leaf covers per-call overrides, cancellation, timeout semantics, and the SDK’s retry configuration. For continuous supervision, these controls determine whether a cheap semantic check remains bounded or silently becomes a long multi-attempt operation.

## Request options

- `RequestOptions` overrides client settings per call. [RequestOptions](https://docs.typesafe.ai/sdk/javascript/api/interfaces/RequestOptions.md)
- `headers?: Record<string,string>` are merged over `defaultHeaders`, so per-call values take precedence. [RequestOptions](https://docs.typesafe.ai/sdk/javascript/api/interfaces/RequestOptions.md)
- `retry?: Partial<RetryPolicy>` overrides retry behavior for one call; omitted fields inherit client settings. [RequestOptions](https://docs.typesafe.ai/sdk/javascript/api/interfaces/RequestOptions.md)
- `signal?: AbortSignal` cancels both the active request and pending retries. [RequestOptions](https://docs.typesafe.ai/sdk/javascript/api/interfaces/RequestOptions.md)
- `timeout?: number` is milliseconds per attempt; there is explicitly no total retry budget. [RequestOptions](https://docs.typesafe.ai/sdk/javascript/api/interfaces/RequestOptions.md)

## Retry behavior and defaults

- Partial retry overrides inherit unset fields from client or SDK defaults. [RetryPolicy](https://docs.typesafe.ai/sdk/javascript/api/interfaces/RetryPolicy.md)
- Connection failures—including interrupted response bodies—retry by default; `APIConnectionError` control defaults to `true`. [RetryPolicy](https://docs.typesafe.ai/sdk/javascript/api/interfaces/RetryPolicy.md)
- `APITimeoutError` retries by default (`true`). [RetryPolicy](https://docs.typesafe.ai/sdk/javascript/api/interfaces/RetryPolicy.md)
- Exponential backoff starts at `500ms`, doubles up to `5000ms`, and subtracts a random fraction up to `0.25` of each delay by default. [RetryPolicy](https://docs.typesafe.ai/sdk/javascript/api/interfaces/RetryPolicy.md)
- Retried HTTP statuses default to `408`, `429`, and `500-599`. [RetryPolicy](https://docs.typesafe.ai/sdk/javascript/api/interfaces/RetryPolicy.md)
- `maxRetries` counts retries after the initial attempt; default `2` permits up to three total attempts, while `0` disables retries. [RetryPolicy](https://docs.typesafe.ai/sdk/javascript/api/interfaces/RetryPolicy.md)
- Server `Retry-After` and `retry-after-ms` are honored by default up to `60000ms`; larger requested delays fall back to backoff. [RetryPolicy](https://docs.typesafe.ai/sdk/javascript/api/interfaces/RetryPolicy.md)

## Citation bookmarks

- Per-call inheritance and cancellation: [RequestOptions](https://docs.typesafe.ai/sdk/javascript/api/interfaces/RequestOptions.md)
- Per-attempt timeout caveat: [RequestOptions](https://docs.typesafe.ai/sdk/javascript/api/interfaces/RequestOptions.md)
- Retryable failure classes/statuses: [RetryPolicy](https://docs.typesafe.ai/sdk/javascript/api/interfaces/RetryPolicy.md) [RetryPolicy](https://docs.typesafe.ai/sdk/javascript/api/interfaces/RetryPolicy.md)
- Backoff defaults: [RetryPolicy](https://docs.typesafe.ai/sdk/javascript/api/interfaces/RetryPolicy.md)
- Retry count and server delay: [RetryPolicy](https://docs.typesafe.ai/sdk/javascript/api/interfaces/RetryPolicy.md)

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
