# API error envelope and client-side HTTP status classes

## Purpose

This leaf covers unsuccessful HTTP responses and the SDK subclasses for invalid input, authentication, authorization, missing resources, and server-side validation. It emphasizes the shared observability fields and the operational distinctions an integration should preserve.

## Key concepts

- **`APIError` means the server returned an unsuccessful HTTP response.** It extends `TypeSafeError` and is the parent for the documented 400, 401, 403, 404, 422, 429, and 5xx classes. [APIError](https://docs.typesafe.ai/sdk/javascript/api/classes/APIError.md)
- **The error body is intentionally untyped.** `body` can be parsed JSON, response text, or `undefined` for an empty response; operational code must narrow it before reading fields. [APIError](https://docs.typesafe.ai/sdk/javascript/api/classes/APIError.md)
- **Every HTTP error carries response metadata.** `headers` exposes the HTTP headers, `status` preserves the numeric status code, and `requestId` reads `x-typesafe-request-id` when present. [APIError](https://docs.typesafe.ai/sdk/javascript/api/classes/APIError.md)
- **Status-to-subclass dispatch is centralized.** `APIError.fromResponse(status, body, headers)` constructs the status-specific error class and returns it as `APIError`. [APIError](https://docs.typesafe.ai/sdk/javascript/api/classes/APIError.md)
- **400 and 422 are distinct request failures.** `BadRequestError` means the request is invalid; `UnprocessableEntityError` specifically means request validation failed. Both inherit the same body, headers, request ID, and status envelope. [BadRequestError](https://docs.typesafe.ai/sdk/javascript/api/classes/BadRequestError.md) [BadRequestError](https://docs.typesafe.ai/sdk/javascript/api/classes/BadRequestError.md) [UnprocessableEntityError](https://docs.typesafe.ai/sdk/javascript/api/classes/UnprocessableEntityError.md)
- **401 and 403 separate identity from authority.** `AuthenticationError` means authentication failed; `PermissionDeniedError` means access is denied. [AuthenticationError](https://docs.typesafe.ai/sdk/javascript/api/classes/AuthenticationError.md) [PermissionDeniedError](https://docs.typesafe.ai/sdk/javascript/api/classes/PermissionDeniedError.md)
- **404 is a missing-resource response.** `NotFoundError` preserves the same constructor inputs and inherited observability fields as the other `APIError` subclasses. [NotFoundError](https://docs.typesafe.ai/sdk/javascript/api/classes/NotFoundError.md) [NotFoundError](https://docs.typesafe.ai/sdk/javascript/api/classes/NotFoundError.md)

## Citation bookmarks

- Complete `APIError` subclass list: [APIError](https://docs.typesafe.ai/sdk/javascript/api/classes/APIError.md)
- Constructor inputs: [APIError](https://docs.typesafe.ai/sdk/javascript/api/classes/APIError.md)
- Body/header/request-ID/status fields: [APIError](https://docs.typesafe.ai/sdk/javascript/api/classes/APIError.md)
- Status dispatch: [APIError](https://docs.typesafe.ai/sdk/javascript/api/classes/APIError.md)
- Invalid request: [BadRequestError](https://docs.typesafe.ai/sdk/javascript/api/classes/BadRequestError.md)
- Failed authentication: [AuthenticationError](https://docs.typesafe.ai/sdk/javascript/api/classes/AuthenticationError.md)
- Denied access: [PermissionDeniedError](https://docs.typesafe.ai/sdk/javascript/api/classes/PermissionDeniedError.md)
- Missing resource: [NotFoundError](https://docs.typesafe.ai/sdk/javascript/api/classes/NotFoundError.md)
- Server validation failure: [UnprocessableEntityError](https://docs.typesafe.ai/sdk/javascript/api/classes/UnprocessableEntityError.md)

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

- **Emit a safe structured incident:** record error subclass, numeric status, request ID when available, response content type, and a bounded/redacted representation of `body`; attach the Gimbal run/node ID locally. Start at [APIError](https://docs.typesafe.ai/sdk/javascript/api/classes/APIError.md).
- **Route operator guidance by subtype:** 401 → credential/authentication repair; 403 → account/permission repair; 400 or 422 → request/question validation; 404 → endpoint/model/resource configuration. The class semantics are at the status-specific bookmarks above.
- **Preserve the error envelope through fallback:** when a generative supervisor replaces a failed Jev call, retain the Jev status/request ID with the fallback result so the system does not hide an upstream reliability incident.

