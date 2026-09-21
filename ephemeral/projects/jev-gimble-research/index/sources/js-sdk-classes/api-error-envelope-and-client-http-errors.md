# API error envelope and client-side HTTP status classes

## Purpose

This leaf covers unsuccessful HTTP responses and the SDK subclasses for invalid input, authentication, authorization, missing resources, and server-side validation. It emphasizes the shared observability fields and the operational distinctions an integration should preserve.

## Key concepts

- **`APIError` means the server returned an unsuccessful HTTP response.** It extends `TypeSafeError` and is the parent for the documented 400, 401, 403, 404, 422, 429, and 5xx classes. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/classes/APIError.md:5-21`
- **The error body is intentionally untyped.** `body` can be parsed JSON, response text, or `undefined` for an empty response; operational code must narrow it before reading fields. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/classes/APIError.md:64-75`
- **Every HTTP error carries response metadata.** `headers` exposes the HTTP headers, `status` preserves the numeric status code, and `requestId` reads `x-typesafe-request-id` when present. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/classes/APIError.md:78-110`
- **Status-to-subclass dispatch is centralized.** `APIError.fromResponse(status, body, headers)` constructs the status-specific error class and returns it as `APIError`. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/classes/APIError.md:112-144`
- **400 and 422 are distinct request failures.** `BadRequestError` means the request is invalid; `UnprocessableEntityError` specifically means request validation failed. Both inherit the same body, headers, request ID, and status envelope. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/classes/BadRequestError.md:5-11` `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/classes/BadRequestError.md:54-116` `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/classes/UnprocessableEntityError.md:5-11`
- **401 and 403 separate identity from authority.** `AuthenticationError` means authentication failed; `PermissionDeniedError` means access is denied. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/classes/AuthenticationError.md:5-11` `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/classes/PermissionDeniedError.md:5-11`
- **404 is a missing-resource response.** `NotFoundError` preserves the same constructor inputs and inherited observability fields as the other `APIError` subclasses. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/classes/NotFoundError.md:5-52` `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/classes/NotFoundError.md:54-116`

## Citation bookmarks

- Complete `APIError` subclass list: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/classes/APIError.md:5-21`
- Constructor inputs: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/classes/APIError.md:23-62`
- Body/header/request-ID/status fields: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/classes/APIError.md:64-110`
- Status dispatch: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/classes/APIError.md:112-144`
- Invalid request: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/classes/BadRequestError.md:5-11`
- Failed authentication: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/classes/AuthenticationError.md:5-11`
- Denied access: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/classes/PermissionDeniedError.md:5-11`
- Missing resource: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/classes/NotFoundError.md:5-11`
- Server validation failure: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/classes/UnprocessableEntityError.md:5-11`

## Themes

- Failure classification is structured enough for metrics and operator guidance without requiring parsing message text.
- Response bodies remain schema-unknown; status subtype and request ID are the stable routing signals documented here.
- Authentication, authorization, malformed input, and domain validation are operationally different incidents even though they share one base envelope.

## Gotchas

- `body` is `unknown`; do not assume JSON or access `body.error` without runtime narrowing.
- `requestId` may be absent. Correlation code needs a local request/run identifier as a fallback.
- The class pages define meanings but not retryability. Do not add a blind outer retry around 400/401/403/404/422 failures; determine whether correction, credential refresh, configuration repair, or a documented retry policy is appropriate.
- 422 is server-side request validation and can still occur even though `systemOne` performs some local validation.
- `NotFoundError` does not state which resource was missing; inspect the untyped body and request context rather than assuming the model name or endpoint.

## Task recipes

- **Emit a safe structured incident:** record error subclass, numeric status, request ID when available, response content type, and a bounded/redacted representation of `body`; attach the Gimble run/node ID locally. Start at `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/classes/APIError.md:64-110`.
- **Route operator guidance by subtype:** 401 → credential/authentication repair; 403 → account/permission repair; 400 or 422 → request/question validation; 404 → endpoint/model/resource configuration. The class semantics are at the status-specific bookmarks above.
- **Preserve the error envelope through fallback:** when a generative supervisor replaces a failed Jev call, retain the Jev status/request ID with the fallback result so the system does not hide an upstream reliability incident.

