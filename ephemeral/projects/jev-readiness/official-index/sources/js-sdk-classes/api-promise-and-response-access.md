# `APIPromise`: parsed results, raw responses, and request correlation

## Purpose

This leaf describes the SDK’s promise wrapper and the ownership rules for parsed data versus raw HTTP responses. It is the key source for response buffering, request-ID access, and avoiding double consumption.

## Key concepts

- **`APIPromise<T>` is both a standard promise and an HTTP-aware wrapper.** It extends `Promise<T>` while retaining access to the response; non-2xx responses reject with `APIError`, including calls through `asResponse()`. [APIPromise](https://docs.typesafe.ai/sdk/javascript/api/classes/APIPromise.md)
- **`asResponse()` returns a raw `Response`, not an early streaming handoff.** The SDK buffers the full body under the request timeout before handing the response to the caller; subsequent body reading is caller-owned. [APIPromise](https://docs.typesafe.ai/sdk/javascript/api/classes/APIPromise.md)
- **Raw and parsed consumption are mutually exclusive on the same promise.** After choosing `asResponse()`, the caller must not also `await` the parsed result from that `APIPromise`. [APIPromise](https://docs.typesafe.ai/sdk/javascript/api/classes/APIPromise.md)
- **`map()` transforms parsed data while preserving HTTP association.** It returns another `APIPromise<U>`, shares the HTTP response, and performs a single body parse. [APIPromise](https://docs.typesafe.ai/sdk/javascript/api/classes/APIPromise.md)
- **Ordinary Promise chaining loses the specialized return type.** `catch`, `finally`, and `then` are documented as returning ordinary `Promise` values, whereas `map` returns `APIPromise`; callers needing response metadata should not chain it away before calling the response-aware helpers. [APIPromise](https://docs.typesafe.ai/sdk/javascript/api/classes/APIPromise.md) [APIPromise](https://docs.typesafe.ai/sdk/javascript/api/classes/APIPromise.md)
- **`withResponse()` is the safe combined path.** It returns parsed data, the HTTP response, and the request ID together. [APIPromise](https://docs.typesafe.ai/sdk/javascript/api/classes/APIPromise.md)

## Citation bookmarks

- Promise/error contract: [APIPromise](https://docs.typesafe.ai/sdk/javascript/api/classes/APIPromise.md)
- Construction from response promise and parser: [APIPromise](https://docs.typesafe.ai/sdk/javascript/api/classes/APIPromise.md)
- Raw response buffering and ownership: [APIPromise](https://docs.typesafe.ai/sdk/javascript/api/classes/APIPromise.md)
- Single-parse transformation: [APIPromise](https://docs.typesafe.ai/sdk/javascript/api/classes/APIPromise.md)
- Parsed data plus response/request ID: [APIPromise](https://docs.typesafe.ai/sdk/javascript/api/classes/APIPromise.md)

## Themes

- Observability metadata can travel with typed application data.
- Full-response buffering makes the SDK API deterministic but rules out treating `asResponse()` as a streaming primitive.
- One body has one owner; use the combined helper when both data and metadata are required.

## Gotchas

- `asResponse()` still rejects on non-2xx; it is not a bypass for inspecting an error response as a successful raw `Response`.
- The entire body must arrive before `asResponse()` resolves. Large responses and slow body delivery consume the same request timeout and memory budget.
- Do not both call `asResponse()` and await the parsed result on the same promise.
- Calling `.then()`, `.catch()`, or `.finally()` produces an ordinary `Promise`; use `map()` if later code still needs the specialized response helpers.
- The page does not state whether `withResponse().requestId` can be absent on successful responses; error request IDs are explicitly optional elsewhere.

## Task recipes

- **Correlate every supervision inference:** use `withResponse()` and record request ID, model/token usage from the parsed result, latency, and the Gimble run/node identifier together. Start at [APIPromise](https://docs.typesafe.ai/sdk/javascript/api/classes/APIPromise.md).
- **Transform without losing transport metadata:** call `map()` to derive a smaller domain value, then use the returned `APIPromise<U>` for response-aware handling. Start at [APIPromise](https://docs.typesafe.ai/sdk/javascript/api/classes/APIPromise.md).
- **Inspect a successful raw body:** select `asResponse()` once, read the caller-owned body once, and do not await the parsed result. Start at [APIPromise](https://docs.typesafe.ai/sdk/javascript/api/classes/APIPromise.md).

