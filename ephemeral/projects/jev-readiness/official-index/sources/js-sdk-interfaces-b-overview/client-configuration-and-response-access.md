# JavaScript client configuration and raw response access

## Purpose

This leaf covers environment/config precedence, model and transport defaults, browser-key safety, logging exposure, retry/timeout defaults, and the raw-response wrapper needed for request correlation and HTTP inspection.

## Configuration precedence and defaults

- `TypeSafeClientConfig` resolves explicit values before environment variables, then SDK defaults. [TypeSafeClientConfig](https://docs.typesafe.ai/sdk/javascript/api/interfaces/TypeSafeClientConfig.md)
- `apiKey` falls back to `TYPESAFE_API_KEY`; `baseURL` falls back to `TYPESAFE_BASE_URL`, then `https://api.typesafe.ai`. [TypeSafeClientConfig](https://docs.typesafe.ai/sdk/javascript/api/interfaces/TypeSafeClientConfig.md)
- Browser use is refused unless `dangerouslyAllowBrowser` is set, because it exposes the API key to page users; default is `false`. [TypeSafeClientConfig](https://docs.typesafe.ai/sdk/javascript/api/interfaces/TypeSafeClientConfig.md)
- Default headers can be configured, with per-call headers taking precedence. [TypeSafeClientConfig](https://docs.typesafe.ai/sdk/javascript/api/interfaces/TypeSafeClientConfig.md)
- `defaultModel` falls back to `TYPESAFE_DEFAULT_MODEL`, then the moving alias `jev-latest`. [TypeSafeClientConfig](https://docs.typesafe.ai/sdk/javascript/api/interfaces/TypeSafeClientConfig.md)
- A custom `fetch` supports transport configuration/testing; otherwise global `fetch` is used. [TypeSafeClientConfig](https://docs.typesafe.ai/sdk/javascript/api/interfaces/TypeSafeClientConfig.md)

## Logging, retries, and timeout

- A custom `Logger` is filtered to `logLevel` and above; default logger is prefixed `console`. [TypeSafeClientConfig](https://docs.typesafe.ai/sdk/javascript/api/interfaces/TypeSafeClientConfig.md)
- Log level falls back to `TYPESAFE_LOG_LEVEL`, then `warn`. `info` logs request summaries; `debug` adds headers and bodies. Known credential headers are redacted, but bodies are not. [TypeSafeClientConfig](https://docs.typesafe.ai/sdk/javascript/api/interfaces/TypeSafeClientConfig.md)
- Client retry accepts `Partial<RetryPolicy>` and inherits omitted fields from SDK defaults. [TypeSafeClientConfig](https://docs.typesafe.ai/sdk/javascript/api/interfaces/TypeSafeClientConfig.md)
- Client timeout defaults to `10000ms` per attempt, with no total retry budget. [TypeSafeClientConfig](https://docs.typesafe.ai/sdk/javascript/api/interfaces/TypeSafeClientConfig.md)

## Parsed data plus HTTP response

- `WithResponse<T>` packages parsed data with the HTTP response and request ID. [WithResponse](https://docs.typesafe.ai/sdk/javascript/api/interfaces/WithResponse.md)
- `data: T` is the parsed response body. [WithResponse](https://docs.typesafe.ai/sdk/javascript/api/interfaces/WithResponse.md)
- `requestId` comes from `x-typesafe-request-id` and may be `undefined`. [WithResponse](https://docs.typesafe.ai/sdk/javascript/api/interfaces/WithResponse.md)
- `response` is the native `Response`, but its body has already been consumed by parsing. [WithResponse](https://docs.typesafe.ai/sdk/javascript/api/interfaces/WithResponse.md)

## Citation bookmarks

- Config precedence and credentials: [TypeSafeClientConfig](https://docs.typesafe.ai/sdk/javascript/api/interfaces/TypeSafeClientConfig.md)
- Model and transport defaults: [TypeSafeClientConfig](https://docs.typesafe.ai/sdk/javascript/api/interfaces/TypeSafeClientConfig.md)
- Logging exposure: [TypeSafeClientConfig](https://docs.typesafe.ai/sdk/javascript/api/interfaces/TypeSafeClientConfig.md)
- Retry and timeout: [TypeSafeClientConfig](https://docs.typesafe.ai/sdk/javascript/api/interfaces/TypeSafeClientConfig.md)
- Request ID and consumed response: [WithResponse](https://docs.typesafe.ai/sdk/javascript/api/interfaces/WithResponse.md)

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
