# Python SDK client lifecycle and call contract

## Purpose

Task-focused reference for choosing the synchronous or asynchronous Python client, constructing and closing it safely, issuing `system_one` calls, selecting models, using gateways, and integrating the SDK into a long-running supervision service. The public API page is only a link hub; the behavioral contracts live in the client and usage pages. [/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/python/api.md:5-16]

## Key concepts

- `TypeSafeClient` and `AsyncTypeSafeClient` expose the same logical resources: `system_one` and a cached `models` accessor. Async calls and model listing must be awaited; sync calls do not use `await`. [/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/python/api/clients/sync.md:112-145] [/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/python/api/clients/async.md:119-159]
- Prefer context managers: `with TypeSafeClient()` for sync and `async with AsyncTypeSafeClient()` for async. Explicit `close()`/`aclose()` releases network resources **and closes a caller-supplied HTTP client**, so shared-client ownership must be designed deliberately. [/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/python/api/clients/sync.md:265-273] [/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/python/api/clients/async.md:277-287]
- Client configuration accepts API key, default model, retry policy, per-operation timeout, headers, a custom transport or HTTP client, and base URL. `transport` and `http_client` are mutually exclusive. Explicit options outrank environment variables; whitespace-only environment values are ignored. [/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/python/api/clients/sync.md:42-90]
- `system_one(state, questions, ...)` accepts text, JSON object, or array state plus a nonempty mapping of answer names to question objects or raw dictionaries. Per-call model, retry, and timeout override client defaults. [/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/python/api/clients/sync.md:171-191] [/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/python/api/clients/sync.md:202-224]
- Calls accept a Pydantic `response_model`; absent one, the return is `SystemOneResponse` with typed answers, model identity, and token usage. A custom model may be a `SystemOneResponse` subclass or a completely independent `BaseModel`, so standard metadata is not automatically present unless that model declares it. [/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/python/usage.md:75-126] [/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/python/api/clients/sync.md:195-209]
- `models.list()` returns account-visible model metadata and supports per-call retry, timeout, and extra headers. Authentication, SDK identification, and `Accept` headers remain protected from `extra_headers` overrides. [/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/python/api/clients/sync.md:287-325]
- Model selection can be fixed at client construction or overridden per call. The documented default is the moving alias `jev-latest`; `TYPESAFE_DEFAULT_MODEL` can change the process default. For reproducible supervision evidence, persist `result.model` and prefer an explicit versioned model when the service supports one. [/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/python/api/constants.md:41-51] [/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/python/api/constants.md:77-87] [/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/python/usage.md:128-146]
- Alternative endpoints can be configured with `base_url` or `TYPESAFE_BASE_URL`; OpenRouter and Vercel examples change API key, endpoint, and model identifier together. Compatibility requires the alternative endpoint to implement TypeSafe's OpenAPI contract. [/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/python/usage.md:148-204]

## Important citation bookmarks

- Minimal async and sync calls: [/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/python/usage.md:11-73]
- Constructor precedence, ownership, and errors: [/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/python/api/clients/async.md:46-90]
- Complete call parameters and return/exception contract: [/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/python/api/clients/async.md:181-234]
- Model discovery: [/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/python/api/clients/async.md:289-341]
- Environment variables and API-key validation: [/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/python/usage.md:263-278]

## Themes

- **Parity across concurrency styles:** sync and async expose equivalent semantic operations, making event-loop fit the main selection criterion.
- **Configuration hierarchy:** per-call overrides client options, which override environment defaults; record effective values with run evidence.
- **Resource ownership:** client lifetime is part of correctness because close propagates into injected transports/clients.
- **Typed core, explicit escape hatches:** Pydantic models cover stable contracts while raw dictionaries and `extra_body` permit API-forward experimentation.

## Gotchas and version limitations

- `extra_body` is a shallow, last-write-wins merge performed after `state`, `model`, and `questions`; collisions can replace those core values, and nested objects are replaced rather than deep-merged. Treat it as privileged configuration, not ordinary metadata. [/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/python/api/clients/sync.md:192-200]
- Forward compatibility is deliberately lossy: unknown answer kinds are warned about and skipped from typed collections, though still available in `raw_http_response`; unknown fields on recognized responses are ignored. [/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/python/usage.md:280-345]
- `TYPESAFE_LOG_LEVEL` is applied once at import. Debug logs redact recognized secret **headers**, but request and response bodies are not redacted; agent transcripts or customer data placed in state can therefore leak to logs. [/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/python/usage.md:247-261]
- The SDK default HTTP operation timeout is 10 seconds, distinct from the retry policy's total-budget timeout. [/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/python/api/constants.md:89-99]
- API keys are normalized only at the ends; empty, internally spaced, control-character, and non-ASCII values fail client construction, and an explicitly empty key does not fall back to the environment. [/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/python/usage.md:267-278]

## Task recipes

### Embed Jev in an async Gimble-side supervision service

1. Construct one `AsyncTypeSafeClient` for the service lifetime, explicitly setting model, HTTP timeout, and retry policy.
2. Enter it with `async with` (or guarantee `aclose()` at service shutdown); do not inject an HTTP client that another subsystem expects to remain open.
3. For each checkpoint, await `system_one` with structured trace state and a named, nonempty question map.
4. Persist the API `request_id`, returned `model`, token usage, primitive answers, and the supervisory action taken.
5. Keep debug body logging off unless the trace has been redacted.

### Make supervision runs reproducible

1. Call `models.list()` and resolve the intended explicit model name before starting a run.
2. Pass that name at client or call level instead of relying silently on `jev-latest`.
3. Use a `SystemOneResponse` subclass when adding domain fields so model, usage, answer groups, request ID, and raw response remain accessible.
4. Store effective endpoint and retry configuration alongside run evidence, but never the API key.

### Trial a newly released request field safely

1. Confirm the field is supported by the target API/gateway.
2. Place only that field in `extra_body`; reject keys named `state`, `model`, or `questions` before calling.
3. Inspect `raw_http_response` for response kinds the installed SDK cannot yet type.
4. Upgrade the SDK once first-class support is available; the docs call raw unknown fields an escape hatch, not the preferred steady state. [/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/python/usage.md:284-339]

## Gaps

- No thread-safety/concurrent-request guarantee, connection-pool sizing guidance, cancellation semantics, or shutdown-race behavior is documented.
- No idempotency-key facility is described; whether a retried inference is billed or logged more than once is unspecified.
- No model capability schema is returned beyond name, prose description, and release date.

