# JavaScript client configuration and `systemOne`

## Purpose

This leaf captures the JavaScript SDK’s main operational surface: construction precedence, inspectable client configuration, typed `systemOne` requests, per-call overrides, and the documented validation/transport failure boundary.

## Key concepts

- **Configuration has an explicit precedence chain.** Constructor options override environment variables, which override SDK defaults; empty and whitespace-only environment values are ignored. Construction can fail when the API key is missing, configuration is invalid, or the runtime is unsupported. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/classes/TypeSafeClient.md:9-36`
- **The resolved operational configuration is inspectable and readonly.** The client exposes a trailing-slash-normalized `baseURL`, per-request `defaultHeaders`, `defaultModel`, and the `fetch` implementation. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/classes/TypeSafeClient.md:38-85`
- **Logging, available models, retries, and timeouts are first-class client state.** `logger` is filtered to `logLevel`; `models` represents models available to the account; `retry` reflects constructor overrides; `timeout` is measured per attempt in milliseconds. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/classes/TypeSafeClient.md:88-144`
- **`systemOne` supports textual or structured state.** Its generic question map `Q` extends `Questions`; the request carries state, questions, and an optional model override. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/classes/TypeSafeClient.md:146-170`
- **Every call can override operational policy.** `RequestOptions` supplies per-call timeout, retry, headers, and cancellation settings. The return value is `APIPromise<SystemOneResult<Q>>`, typed by question name and criteria and carrying model/token usage. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/classes/TypeSafeClient.md:172-182`
- **The failure boundary distinguishes local validation, exhausted API/transport retries, and caller cancellation.** Empty questions or score criteria with fewer than two entries fail validation; non-2xx responses, connection failures, and timeouts are surfaced after retries; caller abort is a separate failure. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/classes/TypeSafeClient.md:184-198`

## Citation bookmarks

- Constructor precedence and throws: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/classes/TypeSafeClient.md:9-36`
- Endpoint/model/header/fetch properties: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/classes/TypeSafeClient.md:38-85`
- Logger, models, retry, and per-attempt timeout: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/classes/TypeSafeClient.md:88-144`
- `systemOne` type signature and inputs: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/classes/TypeSafeClient.md:146-176`
- Result and complete throws list: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/classes/TypeSafeClient.md:178-198`
- Minimal Noul invocation: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/classes/TypeSafeClient.md:200-208`

## Themes

- Configuration and policy are visible to application code rather than hidden in a singleton.
- Global defaults establish a baseline; per-call options let a supervision probe choose a tighter timeout, retry budget, cancellation scope, or headers.
- Typed result keys connect static question definitions to code-owned routing.

## Gotchas

- `timeout` is **per attempt**, so wall-clock latency may exceed the configured number when retries occur. The class page does not state the maximum total elapsed time.
- These class docs expose a `RetryPolicy` but do not state default attempt count, backoff, retryable statuses, retry-header handling, or whether caller abort can occur during backoff; those details must be retrieved from the retry/interface documentation.
- Constructor precedence can make a local explicit option silently override a production environment value by design. Record the resolved non-secret configuration when diagnosing environment drift.
- The page says unsupported runtimes can fail construction but does not enumerate supported runtimes or required `fetch` features.
- `defaultHeaders` are sent with every request. The page does not describe redaction behavior in logs; do not assume custom authorization or trace headers are safe to log.

## Task recipes

- **Create a bounded supervision client:** construct one client with explicit base URL/model/retry/timeout/logger values, then log only the resolved non-secret properties. Start at `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/classes/TypeSafeClient.md:21-36` and `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/classes/TypeSafeClient.md:38-144`.
- **Give each probe its own latency budget:** pass per-call timeout/retry/cancellation options so a low-priority classifier cannot stall the main workflow. Start at `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/classes/TypeSafeClient.md:172-198`.
- **Fail before network I/O:** validate that the question map is non-empty and each Score has at least two criteria during program startup or test initialization. The documented SDK failure is `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/classes/TypeSafeClient.md:184-186`.

