# JavaScript SDK environment, logging, and version constants

## Purpose

This leaf captures the JavaScript SDK’s exported configuration environment names, documented defaults, ordered logging-level constant, and runtime-visible package version. These constants are the stable handles for deployment configuration and operational correlation.

## Key concepts

- **Explicit client options override environment configuration.** `ENV` centralizes the supported environment-variable names rather than accepting arbitrary configuration keys. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/variables/ENV.md:5-13`
- **`TYPESAFE_API_KEY` supplies the required credential when `apiKey` is omitted.** The variable is a fallback configuration source, not an alternative credential type. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/variables/ENV.md:15-23`
- **`TYPESAFE_BASE_URL` selects the API root.** When neither explicit configuration nor the environment overrides it, the documented default is `https://api.typesafe.ai`. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/variables/ENV.md:25-33`
- **`TYPESAFE_DEFAULT_MODEL` selects the request default.** Its documented fallback is the moving alias `jev-latest`. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/variables/ENV.md:35-43`
- **`TYPESAFE_LOG_LEVEL` configures SDK logging.** The default is `warn`. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/variables/ENV.md:45-53`
- **`LOG_LEVELS` is a readonly ordered collection.** Its elements are typed as `LogLevel` and ordered from most to least verbose. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/variables/LOG_LEVELS.md:5-11`
- **The SDK exposes an exact build/version constant.** In this documentation snapshot, `VERSION` has the literal value `0.6.0`. `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/variables/VERSION.md:5-9`

## Citation bookmarks

- Environment precedence statement: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/variables/ENV.md:5-13`
- API-key variable: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/variables/ENV.md:15-23`
- Base-URL variable and production default: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/variables/ENV.md:25-33`
- Default-model variable and alias: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/variables/ENV.md:35-43`
- Log-level variable and default: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/variables/ENV.md:45-53`
- Logging-level ordering: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/variables/LOG_LEVELS.md:5-11`
- Exact SDK version: `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/variables/VERSION.md:5-9`

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

- **Validate deployment at startup:** require the API key source, validate the resolved base URL against an allowlist, make the model choice explicit for reproducible trials, and reject unsupported log-level values. Begin with `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/variables/ENV.md:11-53`.
- **Attach operational provenance:** record SDK `VERSION`, effective non-secret base URL, requested model configuration, concrete response model, and Gimble run/node ID with each inference. The SDK version source is `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/variables/VERSION.md:5-9`.
- **Separate stable and exploratory runs:** pin an explicit model for calibration/evaluation runs; reserve `jev-latest` for deliberately rolling experiments and record every observed concrete model.
- **Control logs independently of secrets:** set `TYPESAFE_LOG_LEVEL` explicitly or pass an overriding client option, keep SDK logging at the minimum necessary verbosity, and use application-owned redacted metrics for supervision observability. Start at `/Users/tyler/.codex/worktrees/14b6/gimble/ephemeral/projects/jev-gimble-research/token-cache/official-docs/pages/sdk/javascript/api/variables/ENV.md:45-53`.

