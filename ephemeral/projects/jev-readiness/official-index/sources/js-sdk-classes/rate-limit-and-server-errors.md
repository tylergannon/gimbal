# Rate-limit and server errors

## Purpose

This leaf isolates the HTTP failures most likely to be transient—429 and 5xx—while staying within what the class documentation actually guarantees about retry hints and exhausted retries.

## Key concepts

- **`RateLimitError` represents HTTP 429.** It extends `APIError`, so it carries untyped body, headers, optional request ID, and numeric status like every unsuccessful HTTP response. [RateLimitError](https://docs.typesafe.ai/sdk/javascript/api/classes/RateLimitError.md) [RateLimitError](https://docs.typesafe.ai/sdk/javascript/api/classes/RateLimitError.md) [RateLimitError](https://docs.typesafe.ai/sdk/javascript/api/classes/RateLimitError.md)
- **429 adds a parsed server delay.** `retryAfterMs` is the server retry delay in milliseconds, or `undefined` when the hint is absent or invalid. [RateLimitError](https://docs.typesafe.ai/sdk/javascript/api/classes/RateLimitError.md)
- **`InternalServerError` covers the entire 5xx family.** Its meaning is that the server failed to handle the request; the concrete numeric status remains available through the inherited `status` field. [InternalServerError](https://docs.typesafe.ai/sdk/javascript/api/classes/InternalServerError.md) [InternalServerError](https://docs.typesafe.ai/sdk/javascript/api/classes/InternalServerError.md)
- **Both classes retain the error body and correlation data.** Their inherited fields allow an integration to preserve response details and `x-typesafe-request-id` after the SDK’s retry policy is exhausted. [InternalServerError](https://docs.typesafe.ai/sdk/javascript/api/classes/InternalServerError.md) [RateLimitError](https://docs.typesafe.ai/sdk/javascript/api/classes/RateLimitError.md)
- **`systemOne` surfaces non-2xx only after retries.** Therefore a caught 429 or 5xx from the call represents the terminal result of the SDK-configured attempt policy, not necessarily the first response. [TypeSafeClient](https://docs.typesafe.ai/sdk/javascript/api/classes/TypeSafeClient.md)

## Citation bookmarks

- 429 class meaning: [RateLimitError](https://docs.typesafe.ai/sdk/javascript/api/classes/RateLimitError.md)
- Shared 429 error envelope: [RateLimitError](https://docs.typesafe.ai/sdk/javascript/api/classes/RateLimitError.md)
- `retryAfterMs` contract: [RateLimitError](https://docs.typesafe.ai/sdk/javascript/api/classes/RateLimitError.md)
- 5xx class meaning: [InternalServerError](https://docs.typesafe.ai/sdk/javascript/api/classes/InternalServerError.md)
- 5xx observability fields: [InternalServerError](https://docs.typesafe.ai/sdk/javascript/api/classes/InternalServerError.md)
- Post-retry throw boundary: [TypeSafeClient](https://docs.typesafe.ai/sdk/javascript/api/classes/TypeSafeClient.md)

## Themes

- Retry hints should feed centralized scheduling rather than ad hoc sleeps inside each supervised node.
- Terminal API errors should remain observable even when the workflow degrades gracefully to another decision path.
- HTTP class and numeric status are both useful: the class selects broad policy; status, body, and request ID support diagnosis.

## Gotchas

- `retryAfterMs` can be `undefined` when missing **or invalid**; code cannot distinguish those cases from this property alone.
- The class docs do not identify the source header, parsing rules, maximum accepted delay, or whether the SDK already honored the value during internal retries.
- The class docs do not guarantee that all 429/5xx responses are retried, nor state default attempt count or backoff. A caught error is documented as post-retry, but an additional outer retry risks multiplying attempts.
- `InternalServerError` spans all 5xx statuses; policies needing 500 versus 503 must inspect `status`.
- A fallback that hides repeated 429s can turn supervision load into a self-amplifying failure. Rate-limit metrics and concurrency controls remain necessary.

## Task recipes

- **Avoid retry multiplication:** configure retry policy in one layer, treat the surfaced `RateLimitError`/`InternalServerError` as exhausted under that layer, and permit any workflow-level retry only from a separately bounded budget. The post-retry contract is [TypeSafeClient](https://docs.typesafe.ai/sdk/javascript/api/classes/TypeSafeClient.md).
- **Honor a usable server delay:** when workflow policy allows another attempt, schedule it no earlier than a valid `retryAfterMs`; otherwise use the documented retry-policy fallback rather than inventing an unbounded wait. Start at [RateLimitError](https://docs.typesafe.ai/sdk/javascript/api/classes/RateLimitError.md).
- **Degrade supervision without hiding failure:** record status, request ID, retry hint, and fallback route; then skip intervention or invoke the configured generative/human lane according to the Gimbal workflow’s risk policy.

