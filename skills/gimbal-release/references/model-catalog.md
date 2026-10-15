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
fetched there). The hand-kept exception is Gemini `flash`, in
`internal/modelalias/modelalias.go`.

The scheduled `Refresh model catalog` Actions job runs the same command daily
and can be started manually (`workflow_dispatch`). It reads `DIFFUSION_API_KEY`
and `GIMBAL_PR_CREATION_TOKEN` from Doppler `gimbal/stg` through the main-only
preparation environment, then opens a PR for the generated files; it does not
publish a release. Branch pushes use the checkout's default GitHub token;
`gh pr create` uses the dedicated `GIMBAL_PR_CREATION_TOKEN`, restricted to this
repository with Pull requests read/write and mandatory Metadata read-only. PR
creation with that token allows the PR event to trigger downstream Actions.

Review new and removed model IDs before merging. Aliases follow the catalog, but
built-in role defaults in `internal/builtin/defaults.json` and other configured
defaults, help, and skills name explicit models: update those to the new version
where it applies, keeping each role's intended model tier. Check the rendered
`gimbal run` role flags, not only the JSON. Tests for aliases should cover
resolution and overrides without fixing a moving shorthand to today's version.
