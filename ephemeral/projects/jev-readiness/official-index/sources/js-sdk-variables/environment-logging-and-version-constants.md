# JavaScript SDK environment, logging, and version constants

## Purpose

This leaf captures the JavaScript SDK’s exported configuration environment names, documented defaults, ordered logging-level constant, and runtime-visible package version. These constants are the stable handles for deployment configuration and operational correlation.

## Key concepts

- **Explicit client options override environment configuration.** `ENV` centralizes the supported environment-variable names rather than accepting arbitrary configuration keys. [ENV](https://docs.typesafe.ai/sdk/javascript/api/variables/ENV.md)
- **`TYPESAFE_API_KEY` supplies the required credential when `apiKey` is omitted.** The variable is a fallback configuration source, not an alternative credential type. [ENV](https://docs.typesafe.ai/sdk/javascript/api/variables/ENV.md)
- **`TYPESAFE_BASE_URL` selects the API root.** When neither explicit configuration nor the environment overrides it, the documented default is `https://api.typesafe.ai`. [ENV](https://docs.typesafe.ai/sdk/javascript/api/variables/ENV.md)
- **`TYPESAFE_DEFAULT_MODEL` selects the request default.** Its documented fallback is the moving alias `jev-latest`. [ENV](https://docs.typesafe.ai/sdk/javascript/api/variables/ENV.md)
- **`TYPESAFE_LOG_LEVEL` configures SDK logging.** The default is `warn`. [ENV](https://docs.typesafe.ai/sdk/javascript/api/variables/ENV.md)
- **`LOG_LEVELS` is a readonly ordered collection.** Its elements are typed as `LogLevel` and ordered from most to least verbose. [LOG_LEVELS](https://docs.typesafe.ai/sdk/javascript/api/variables/LOG_LEVELS.md)
- **The SDK exposes an exact build/version constant.** In this documentation snapshot, `VERSION` has the literal value `0.6.0`. [VERSION](https://docs.typesafe.ai/sdk/javascript/api/variables/VERSION.md)

## Citation bookmarks

- Environment precedence statement: [ENV](https://docs.typesafe.ai/sdk/javascript/api/variables/ENV.md)
- API-key variable: [ENV](https://docs.typesafe.ai/sdk/javascript/api/variables/ENV.md)
- Base-URL variable and production default: [ENV](https://docs.typesafe.ai/sdk/javascript/api/variables/ENV.md)
- Default-model variable and alias: [ENV](https://docs.typesafe.ai/sdk/javascript/api/variables/ENV.md)
- Log-level variable and default: [ENV](https://docs.typesafe.ai/sdk/javascript/api/variables/ENV.md)
- Logging-level ordering: [LOG_LEVELS](https://docs.typesafe.ai/sdk/javascript/api/variables/LOG_LEVELS.md)
- Exact SDK version: [VERSION](https://docs.typesafe.ai/sdk/javascript/api/variables/VERSION.md)

## Themes

- Deployment defaults are inspectable and overridable, but explicit in-process policy wins.
- Model selection and SDK version are separate axes: `jev-latest` may change while the JavaScript client remains `0.6.0`.
- Environment names form a small operational contract suitable for deployment validation.
- The version constant can anchor logs, traces, bug reports, and compatibility evidence.

## Gotchas

- Do not log or persist the value of `TYPESAFE_API_KEY`; record only whether a credential source was configured.
- An explicit option silently takes precedence by design. Environment-only inspection is insufficient when debugging the effective endpoint, model, or log level.
- `jev-latest` is an alias, not a reproducible model version. Evaluation evidence should also record the concrete model returned by the API when available.
- Changing `TYPESAFE_BASE_URL` redirects credentials and request state to another origin; validate the resolved destination before sending sensitive supervision traces.
- `LOG_LEVELS.md` states ordering but does not enumerate the literal values in this file or describe filtering/redaction behavior.
- `VERSION` exposes `0.6.0` but this slice has no release date, changelog, compatibility matrix, or guarantee tying the docs snapshot to an installed package.
- No environment variables for retry policy, timeout, default headers, custom fetch, or proxy configuration are documented in `ENV`.

## Task recipes

- **Validate deployment at startup:** require the API key source, validate the resolved base URL against an allowlist, make the model choice explicit for reproducible trials, and reject unsupported log-level values. Begin with [ENV](https://docs.typesafe.ai/sdk/javascript/api/variables/ENV.md).
- **Attach operational provenance:** record SDK `VERSION`, effective non-secret base URL, requested model configuration, concrete response model, and Gimbal run/node ID with each inference. The SDK version source is [VERSION](https://docs.typesafe.ai/sdk/javascript/api/variables/VERSION.md).
- **Separate stable and exploratory runs:** pin an explicit model for calibration/evaluation runs; reserve `jev-latest` for deliberately rolling experiments and record every observed concrete model.
- **Control logs independently of secrets:** set `TYPESAFE_LOG_LEVEL` explicitly or pass an overriding client option, keep SDK logging at the minimum necessary verbosity, and use application-owned redacted metrics for supervision observability. Start at [ENV](https://docs.typesafe.ai/sdk/javascript/api/variables/ENV.md).

