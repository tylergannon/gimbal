# Model catalog refresh

Gimbal promises to stay current with the latest models. `just models` (needs
`DIFFUSION_API_KEY`) downloads the models.dev catalog and the Diffusion Router's
`/v1/models`, then regenerates `internal/observation/model_prices_gen.go`,
`internal/modelalias/aliases_gen.go`, and `pi/diffusion_models_gen.go`. The
tracked alias families (haiku, sonnet, opus, fable, luna, terra, sol, astra,
gpt, deepseek-flash, glm, glm-flash, glm-vision) are listed in
`internal/modelcatalog/main.go`; each unversioned alias resolves to the newest
version found, so a new version of a tracked family needs no hand edit. To track
a new family, add a line to that list (a new provider also needs its models
fetched there). The hand-kept exceptions are Gemini `flash` and `pro`, in
`internal/modelalias/modelalias.go`.

The scheduled `Refresh model catalog` Actions job runs the same command daily
and can be started manually (`workflow_dispatch`). It reads `DIFFUSION_API_KEY`
and `GIMBAL_PR_CREATION_TOKEN` from Doppler `gimbal/stg` through the main-only
preparation environment, then opens a PR for the generated files; it does not
publish a release. Branch pushes use the checkout's default GitHub token;
`gh pr create` uses the dedicated `GIMBAL_PR_CREATION_TOKEN`, restricted to this
repository with Pull requests read/write and mandatory Metadata read-only. PR
creation with that token allows the PR event to trigger downstream Actions.

Review new and removed model IDs before merging. Role defaults, examples and
skills use unversioned aliases, with effort when needed (for example `sol:high`,
`astra:medium`, `flash:medium`, or `luna`). They must not pin provider-native
versions. A model refresh changes the central alias catalog; it does not require
editing role defaults or examples. Gemini Flash and Pro version pointers live
in `internal/modelalias/modelalias.go`. Resolve aliases at provider boundaries.
Keep explicit model IDs only where behavior truly depends on that exact version,
such as provider capability tests and historical fixtures. Tests for aliases
must cover resolution and overrides without fixing a moving shorthand to today's
version. Check rendered `gimbal run` role flags show aliases.
