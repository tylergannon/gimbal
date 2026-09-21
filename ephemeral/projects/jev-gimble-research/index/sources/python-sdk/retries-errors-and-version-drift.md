# Python SDK retries, errors, and version drift

## Purpose

Operational reference for bounding Jev calls, classifying failures, choosing retry behavior, logging enough evidence for diagnosis, and protecting a Gimble supervision integration from rapid SDK contract changes.

## Key concepts

- `RetryPolicy` defaults to two retries after the initial attempt, exponential delay from 0.5 seconds capped at 5 seconds, 25% subtractive jitter, retryable HTTP statuses 408, 429, and all 5xx, respect for retry headers, retries for connection and timeout errors, and a 30-second total retry budget. [/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/python/api/retries.md:38-46] [/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/python/api/retries.md:100-100] [/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/python/api/retries.md:140-232] [/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/python/api/retries.md:272-352] [/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/python/api/retries.md:378-430]
- Backoff doubles per attempt up to the cap; jitter randomly subtracts a configured fraction. A zero backoff disables delay. The total retry budget includes the initial attempt and delays, and the SDK stops before a delay that would reach or exceed it, re-raising the last error. [/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/python/api/retries.md:100-180] [/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/python/api/retries.md:182-220] [/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/python/api/retries.md:428-430]
- Retry behavior is configurable at client level and overridable per call; `max_retries=0` disables retries. Invalid API keys fail during client creation before any request or retry. [/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/python/usage.md:206-230]
- Beyond built-in HTTP/connection/timeout rules, a set of exception types and an exception predicate can opt additional failures into retrying. These are additive, so a permissive predicate can unintentionally retry deterministic validation/programming failures. [/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/python/api/retries.md:354-376]
- All SDK failures derive from `TypeSafeError`. `TypeSafeAPIError` represents an unsuccessful HTTP response and retains status, parsed JSON or text body, headers, sanitized endpoint, and optional request ID. Specialized subclasses cover 400, 401, 403, 404, 422, 429, and 5xx. [/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/python/api/exceptions.md:38-60] [/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/python/api/exceptions.md:62-148] [/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/python/api/exceptions.md:150-216]
- `TypeSafeRateLimitError.retry_after_ms` exposes the server-requested wait when present. RetryPolicy can honor `Retry-After` and `retry-after-ms`; the docs do not define precedence between those headers and exponential backoff. [/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/python/api/exceptions.md:190-208] [/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/python/api/retries.md:234-272]
- A connection error means no HTTP response. A timeout error is also a connection error and Python `TimeoutError`, carrying the float or `httpx2.Timeout` used. A response-validation error instead represents a successful HTTP response whose required body is missing/invalid and exposes a dotted offending `field_path`. [/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/python/api/exceptions.md:218-248] [/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/python/api/exceptions.md:250-288]
- Version history is unusually compressed: initial public release v0.5.7 on 2026-09-14, breaking Score criteria change plus type/error/pickle changes in v0.6.0 on 2026-09-15, serialization migration and `response_model` in v0.7.0 on 2026-09-18, then API-key/logging fixes and gateway examples in v0.7.1 on 2026-09-21. [/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/python/changelog.md:11-83]

## Important citation bookmarks

- Retry defaults in one constructor signature: [/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/python/api/retries.md:38-60]
- Retryable status defaults: [/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/python/api/retries.md:222-232]
- Retry headers and connection/timeout switches: [/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/python/api/retries.md:234-352]
- HTTP error diagnostic fields: [/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/python/api/exceptions.md:54-148]
- Transport versus response-validation failures: [/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/python/api/exceptions.md:218-272]
- Full current changelog: [/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/python/changelog.md:11-83]

## Themes

- **Two independent time bounds:** each HTTP operation has a timeout; the retry policy has a total call budget.
- **Retry only transient faults:** statuses, connectivity, and timeouts are enabled by default; local input and response-shape errors should drive configuration/schema repair.
- **Diagnostics are structured:** exception subclasses plus status, body, endpoint, request ID, timeout, and field path support machine routing.
- **Pin during early churn:** three releases with two breaking changes landed within one week of the initial public release.

## Gotchas and version limitations

- The default can make up to three network attempts (initial plus two retries), not two total attempts. [/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/python/api/retries.md:62-100]
- Per-operation timeout does not by itself bound a whole call with retries; use `RetryPolicy.timeout` for a total budget. Conversely, setting only the retry budget does not document how an individual operation is divided across connect/read/write/pool phases.
- Because 5xx and 429 are retryable by default, a supervision loop's observed latency and provider load can exceed the single-attempt profile. The response `Usage` type exposes no attempt count.
- v0.6.0 changed `Score.criteria` from an integer-keyed dictionary to an ordered sequence; code or stored configs using the earlier shape are incompatible. [/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/python/changelog.md:49-64]
- v0.7.0 replaced `msgspec` with Pydantic and added `response_model`; serialization, equality, validation, or error details may differ across that boundary. [/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/python/changelog.md:27-47]
- v0.7.1's early key validation and exclusion from logged exceptions is security-relevant; do not assume earlier versions have the same safeguard. [/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/python/changelog.md:11-25]

## Task recipes

### Bound continuous supervision latency

1. Choose an HTTP operation timeout from the maximum acceptable single-attempt stall.
2. Choose `max_retries` only for the failure classes where a delayed supervisory judgment remains useful.
3. Set `RetryPolicy.timeout` below the Gimble checkpoint's overall deadline, allowing time for local action after Jev returns.
4. Keep retry-header honoring for shared-service backpressure, but record latency and final exception class.
5. For time-critical steering, consider `max_retries=0` and degrade to deterministic policy or human review instead of delivering stale coaching.

### Route failures without recursive agent diagnosis

1. Catch response-validation error first and log `field_path`, request ID, endpoint, status, and SDK/model version; quarantine the payload rather than retrying it with a broad predicate.
2. Catch rate-limit error and record `retry_after_ms`; allow bounded policy retry or reduce supervision frequency.
3. Catch authentication/permission/bad-request/unprocessable errors as configuration/input defects and alert once, not per trace.
4. Catch timeout/connection/5xx as transient availability failures; continue the agent under the explicitly selected fallback policy.
5. Never log credentials or unredacted state bodies as part of the exception event.

### Upgrade the SDK safely

1. Pin the installed SDK version and snapshot the question/response shapes exercised by the integration.
2. Read every intervening changelog entry, with special attention to Score criteria (v0.6.0) and the Pydantic migration (v0.7.0).
3. Run contract tests for mixed question objects/dicts, custom response models, nullable usage, unknown fields/kinds, exception pickling, and retry budgets.
4. Run a small live comparison on a fixed trace corpus, persisting actual returned model identity; a green serialization test does not prove equal judgments.

## Gaps

- The docs do not define retry-header parsing limits, precedence, malformed-value behavior, or whether retry delays can exceed `backoff_max`.
- There is no idempotency/billing guarantee for retried calls and no retry-attempt metadata in successful responses.
- No documented circuit breaker, bulkhead, concurrency limiter, or cancellation API exists.
- The changelog begins only at v0.5.7 and contains no migration guide, support window, Python-version matrix, or compatibility table with API/model versions.
- The assigned source inventory says 12 Markdown files including an overview, but only 11 Markdown files exist recursively under `pages/sdk/python`; no standalone overview source was available to index.

