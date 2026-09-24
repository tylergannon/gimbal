# JavaScript logging and model metadata

## Purpose

This leaf covers SDK observability hooks and account-visible model metadata. The surfaces are intentionally small: a console-compatible four-level logger, a model listing resource, and model cards containing only name, description, and release date.

## Logging surface

- `Logger` is described as console-compatible and accepts a string message followed by structured values. [Logger](https://docs.typesafe.ai/sdk/javascript/api/interfaces/Logger.md)
- `debug(message: string, ...args: unknown[]): void` provides the lowest-level hook. [Logger](https://docs.typesafe.ai/sdk/javascript/api/interfaces/Logger.md)
- `error(message: string, ...args: unknown[]): void` has the same synchronous shape. [Logger](https://docs.typesafe.ai/sdk/javascript/api/interfaces/Logger.md)
- `info(message: string, ...args: unknown[]): void` has the same synchronous shape. [Logger](https://docs.typesafe.ai/sdk/javascript/api/interfaces/Logger.md)
- `warn(message: string, ...args: unknown[]): void` completes the four-level interface. [Logger](https://docs.typesafe.ai/sdk/javascript/api/interfaces/Logger.md)

## Model discovery

- `Models` is the Models API resource. `list(options = {})` lists models available to the account and returns `APIPromise<ModelCard[]>`; the same per-call `RequestOptions` used elsewhere applies. [Models](https://docs.typesafe.ai/sdk/javascript/api/interfaces/Models.md)
- `ModelCard` contains three readonly strings: `description`, `name`, and `release_date`. [ModelCard](https://docs.typesafe.ai/sdk/javascript/api/interfaces/ModelCard.md)

## Citation bookmarks

- Logger contract: [Logger](https://docs.typesafe.ai/sdk/javascript/api/interfaces/Logger.md)
- All log levels: [Logger](https://docs.typesafe.ai/sdk/javascript/api/interfaces/Logger.md)
- Account model listing: [Models](https://docs.typesafe.ai/sdk/javascript/api/interfaces/Models.md)
- Model-card fields: [ModelCard](https://docs.typesafe.ai/sdk/javascript/api/interfaces/ModelCard.md)

## Themes

- **Injectable observability:** console compatibility allows a thin adapter into an existing structured logger without an SDK-specific logging framework.
- **Model inventory is account-scoped:** discover what the credential can access instead of hard-coding a global catalog assumption.
- **Snapshot metadata:** record model name and release date beside supervision evaluations so drift investigations have a local reference.

## Gotchas and gaps

- The logger contract has no event schema, context object, child logger, redaction hook, correlation/request ID, or async return. `...unknown[]` is flexible but does not guarantee structured fields.
- The assigned docs do not say which SDK events use which log level, whether bodies/headers are logged, or whether secrets are redacted. Treat log payloads as potentially sensitive until verified from implementation/runtime.
- `ModelCard` has no modality, context limit, supported primitives, vision/image capability, pricing, lifecycle status, alias target, or deprecation metadata. The JavaScript model-listing API therefore cannot, from this interface alone, answer whether Jev supports vision.
- `release_date` is typed only as `string`; format/timezone semantics are undocumented here.
- `Models.list()` has no pagination parameters in the shown signature, but the docs also do not explicitly guarantee an unpaginated/full inventory beyond its `ModelCard[]` result.

## Task recipes

1. **Bridge logging safely:** implement the four methods with a project logger; attach run/call identifiers in the adapter; redact authorization headers, source documents, and question state before emission.
2. **Snapshot available models:** call `models.list()` at a deliberate discovery/admin boundary, persist name/description/release date, and diff snapshots before adopting a new alias or version.
3. **Do not infer vision:** use model description only as a discovery hint; require an authoritative capability document or a bounded live multimodal probe before claiming image support.
4. **Correlate retries:** log attempt number, failure class/status, chosen delay, cancellation, and final outcome in application instrumentation if the SDK does not expose these fields directly.
