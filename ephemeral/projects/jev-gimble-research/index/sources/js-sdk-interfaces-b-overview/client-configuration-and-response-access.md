# JavaScript client configuration and raw response access

## Purpose

This leaf covers environment/config precedence, model and transport defaults, browser-key safety, logging exposure, retry/timeout defaults, and the raw-response wrapper needed for request correlation and HTTP inspection.

## Configuration precedence and defaults

- `TypeSafeClientConfig` resolves explicit values before environment variables, then SDK defaults. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/interfaces/TypeSafeClientConfig.md:5-8`
- `apiKey` falls back to `TYPESAFE_API_KEY`; `baseURL` falls back to `TYPESAFE_BASE_URL`, then `https://api.typesafe.ai`. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/interfaces/TypeSafeClientConfig.md:9-32`
- Browser use is refused unless `dangerouslyAllowBrowser` is set, because it exposes the API key to page users; default is `false`. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/interfaces/TypeSafeClientConfig.md:33-44`
- Default headers can be configured, with per-call headers taking precedence. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/interfaces/TypeSafeClientConfig.md:45-56`
- `defaultModel` falls back to `TYPESAFE_DEFAULT_MODEL`, then the moving alias `jev-latest`. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/interfaces/TypeSafeClientConfig.md:57-68`
- A custom `fetch` supports transport configuration/testing; otherwise global `fetch` is used. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/interfaces/TypeSafeClientConfig.md:69-80`

## Logging, retries, and timeout

- A custom `Logger` is filtered to `logLevel` and above; default logger is prefixed `console`. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/interfaces/TypeSafeClientConfig.md:81-92`
- Log level falls back to `TYPESAFE_LOG_LEVEL`, then `warn`. `info` logs request summaries; `debug` adds headers and bodies. Known credential headers are redacted, but bodies are not. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/interfaces/TypeSafeClientConfig.md:93-106`
- Client retry accepts `Partial<RetryPolicy>` and inherits omitted fields from SDK defaults. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/interfaces/TypeSafeClientConfig.md:107-118`
- Client timeout defaults to `10000ms` per attempt, with no total retry budget. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/interfaces/TypeSafeClientConfig.md:119-129`

## Parsed data plus HTTP response

- `WithResponse<T>` packages parsed data with the HTTP response and request ID. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/interfaces/WithResponse.md:5-13`
- `data: T` is the parsed response body. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/interfaces/WithResponse.md:15-25`
- `requestId` comes from `x-typesafe-request-id` and may be `undefined`. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/interfaces/WithResponse.md:27-38`
- `response` is the native `Response`, but its body has already been consumed by parsing. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/interfaces/WithResponse.md:39-49`

## Citation bookmarks

- Config precedence and credentials: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/interfaces/TypeSafeClientConfig.md:5-44`
- Model and transport defaults: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/interfaces/TypeSafeClientConfig.md:45-80`
- Logging exposure: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/interfaces/TypeSafeClientConfig.md:81-106`
- Retry and timeout: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/interfaces/TypeSafeClientConfig.md:107-129`
- Request ID and consumed response: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/interfaces/WithResponse.md:27-49`

## Themes

- **Explicit configuration wins:** production code can pin credentials, endpoint, model, transport, logging, retry, and timeout independent of ambient process settings.
- **Keep API keys server-side:** browser enablement is deliberately named dangerous, not a normal deployment path.
- **Observe at the HTTP seam:** request ID and response headers/status complement parsed model results for operational diagnosis.
- **Model pinning versus alias convenience:** `jev-latest` eases upgrades but weakens repeatability unless actual result model is recorded.

## Gotchas and gaps

- `debug` logs request and response bodies without redaction. Supervision state can contain source code, user text, credentials embedded in documents, or sensitive operational data; do not enable debug casually.
- Known credential headers are redacted, not necessarily every sensitive custom header. Review custom header names and logger handling.
- `dangerouslyAllowBrowser` exposes the API key to page users. A browser-based Gimble view should call a server-side boundary, not instantiate this client with a secret.
- Default `jev-latest` can change behavior without a dependency update. Pin for reproducible experiments or store returned model and rerun stability gates.
- The 10-second timeout is per attempt, not a total SLA; retries can extend wall time significantly.
- `WithResponse.response` cannot be reparsed because its body is consumed. Read status and headers; use `data` for content.
- `requestId` is optional, so local correlation IDs are still required.

## Task recipes

1. **Production client:** provide explicit API key through a secret store, pin base URL/model where reproducibility matters, inject a redacting logger, and keep browser use disabled.
2. **Test transport:** inject a fake `fetch` to assert exact payloads, header precedence, retry behavior, and response/request-ID handling without network calls.
3. **Operational evidence:** request response metadata, record local correlation ID plus TypeSafe request ID/status/headers, parsed result model/usage, effective config, and final failure class.
4. **Safe logging:** default to `warn`; at `info`, emit only bounded summaries; enable `debug` only in controlled fixtures with synthetic bodies.
