# Runtime fetch, environment, and logging seams

## Purpose

This leaf captures the three runtime integration aliases exposed in the assigned corpus: custom `fetch`, named environment-variable values, and the closed logging-level set.

## Key concepts

- **A custom `Fetch` must look like global `fetch`.** Its documented input is a string, its optional second argument is `RequestInit`, and it must return `Promise<Response>`. [Fetch](https://docs.typesafe.ai/sdk/javascript/api/type-aliases/Fetch.md)
- **The fetch seam is narrow and platform-shaped.** Custom transports, instrumentation wrappers, test doubles, and proxy-aware implementations must ultimately preserve the Web `RequestInit`/`Response` contract. [Fetch](https://docs.typesafe.ai/sdk/javascript/api/type-aliases/Fetch.md)
- **`EnvVar` is derived from the exported `ENV` values.** It is `typeof ENV[keyof typeof ENV]`, so accepted environment names are centrally defined by `ENV`, not arbitrary strings at the type level. [EnvVar](https://docs.typesafe.ai/sdk/javascript/api/type-aliases/EnvVar.md)
- **Logging has five exact levels.** `LogLevel` is `debug | info | warn | error | off`; `off` disables logging. [LogLevel](https://docs.typesafe.ai/sdk/javascript/api/type-aliases/LogLevel.md)

## Citation bookmarks

- Complete custom-fetch signature: [Fetch](https://docs.typesafe.ai/sdk/javascript/api/type-aliases/Fetch.md)
- Environment-name derivation: [EnvVar](https://docs.typesafe.ai/sdk/javascript/api/type-aliases/EnvVar.md)
- Complete logging-level union: [LogLevel](https://docs.typesafe.ai/sdk/javascript/api/type-aliases/LogLevel.md)

## Themes

- Runtime substitution is possible without changing the typed decision API.
- Environment keys and logging verbosity are closed policy surfaces rather than unconstrained strings.
- The custom fetch boundary is the natural point for transport metrics and test fault injection, provided behavior remains fetch-compatible.

## Gotchas

- The documented `Fetch` input is `string`, narrower than the broadest platform `fetch` overloads. A custom implementation must at least support that exact SDK call shape.
- A fetch wrapper that drops `RequestInit.signal`, headers, or timeout-related behavior can silently break cancellation, authentication, tracing, or retry semantics.
- The alias does not document whether the SDK clones responses, how many times it invokes fetch under retries, or which response/body methods it requires.
- `EnvVar.md` does not list the actual `ENV` values, precedence, or whitespace handling; those must be retrieved from the `ENV` variable/client configuration docs.
- `LogLevel` specifies filtering vocabulary but not payload fields or secret redaction. `debug` should not be assumed safe for production traces.

## Task recipes

- **Instrument without changing semantics:** wrap fetch to record start/end, attempt correlation, status, and cancellation while forwarding the input and complete `RequestInit` and returning the original-compatible `Response`. Begin at [Fetch](https://docs.typesafe.ai/sdk/javascript/api/type-aliases/Fetch.md).
- **Build transport fault tests:** inject a custom fetch that deterministically returns selected responses or throws connection/abort errors, then verify the SDK’s typed error route and Gimbal fallback behavior.
- **Choose production logging explicitly:** set one of the five allowed levels, use `off` where the SDK logger cannot meet redaction requirements, and emit separately controlled application metrics keyed by safe request/run identifiers. Start at [LogLevel](https://docs.typesafe.ai/sdk/javascript/api/type-aliases/LogLevel.md).

