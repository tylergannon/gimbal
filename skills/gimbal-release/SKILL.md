---
name: gimbal-release
description: >
  Build and deliver changes to the Gimbal application. Covers development
  priorities, model catalog refreshes, proof, generated frontend assets,
  merge, local installation, and refreshing installed Gimbal instructions.
---

# Build and release Gimbal

Deliver the requested behavior and establish that it works. Gimbal ships a
CLI/server and a separate macOS desktop client.
For writing a workflow, use [Author workflows](../gimbal-workflows/SKILL.md).
For operating one, use [Use Gimbal](../gimbal-runs/SKILL.md).

## Priorities and working method

Work in the task worktree. Read its `AGENTS.md`, `docs/definition-of-done.md`,
and the relevant requirement. Use Godoc for the API; use research and historical
reviews to understand decisions. Materialize remote requirements locally before
giving their absolute paths to agents.

Keep implementation as simple as the requirement permits. Workflow tactics
stay visible in ordinary Go. Add no unrequested features, compatibility layers,
or abstractions. A hypothetical edge-case concern needs a reasonable actual
failing test before it becomes repair work.

Give agents outcomes and relevant facts. Scope coaches can steer planner,
worker, and validator against over-engineering; they are advisory. Review
findings are assessed against requested behavior rather than automatically
expanding the goal or becoming release blockers.

The repository's completion standard is requested functionality implemented
and seen working. Its 90–95% completion guidance permits deferring remaining
quirks; it does not permit declaring a required failing check successful.
Distinguish unmet requirements and invalid proof from optional improvements.
File genuinely deferred work rather than extending delivery indefinitely.

## Build the application

```sh
just build
```

The current recipe prepares Go and frontend dependencies, runs generation,
formats generated frontend output, builds Svelte, checks that
`web/build/skgo.manifest.json` exists, and builds `bin/gimbal` with `web/build/`
embedded directly using `embed.FS`. Generated frontend files stay local and are
not committed. Run the built CLI as `./bin/gimbal`. Published versions are
installed from the Homebrew tap; a bare versioned `go install` does not build
or include the console.

Generators own generated files. Regenerate and inspect their diff rather than
patching generated output by hand. Preserve Gimbal-owned application code when
updating a scaffold. Follow the existing application composition so tests and
the built binary exercise the same server.

## Optional web authentication

The root serving command accepts `--auth-file /absolute/auth.json`; custom hosts
use `web.WithAuthFile`. The file selects a provider and users plugin by Registry
name and one explicit public origin. Build native `plugins/auth-github` and
`plugins/auth-files` with the same toolchain/source as the host. Credentials are
environment references. HTTPS is required off loopback; explicit localhost HTTP
works for local login. Provision identities through `gimbal user add` on the
running instance's Unix control socket; login never creates a user. The README
contains the file shape and invocation. Missing configuration/plugins/storage
fail startup. Sessions last one week. The macOS shell keeps its Gimbal session in Keychain,
scoped to the configured server; its proxy supplies that login to web pages and
native recording commands. Sign-out revokes the session and clears the saved
credential. The separate task listener remains outside web authentication.

## Build the macOS workspace

After `just build`, stage the development CLI in the app resources and build
the native microphone application:

```sh
mkdir -p native/Resources
install -m 755 bin/gimbal native/Resources/gimbal
go tool skgo native build --platform macos --preset real
```

The app needs that bundled CLI to start its default local server. Open
`native/build/macos-real/DerivedData/Build/Products/Debug/Gimbal.app` as a macOS
application so macOS attributes microphone and speech permissions to Gimbal.
The `synthetic` preset provides deterministic speech revisions for development.

The macOS app starts its bundled Gimbal CLI as a managed local server by
default. Its central project catalog and instance control state live in
`~/.gimbal/desktop`; each project's runs and conversations remain under that
project's `.gimbal` directory. Set `GIMBAL_SERVER_URL` before app launch to
proxy an external server. In that mode the app does not start, stop, or clean up
the external server. Closing the app window leaves the app and its bundled
server running; Command-Q stops the app-owned server. Project-file storage
reuses `uploadArtifact.r2` in `~/.gimbal/config.json`, with the existing
`GIMBAL_R2_*` environment overrides.
Metadata retains the bucket and object key; viewing generates a new temporary
URL.

## Publish builds

Use the release scripts and GoReleaser configuration in the repository. Read
[distribution.md](references/distribution.md) for local signing setup, runner
requirements, credentials, artifact names and publication commands.

Prove a Release app locally with Developer ID signing, accepted notarization,
stapling, and launch from `/Applications` before activating automated releases.
An unsigned build proves compilation and launch only. Verify the app starts its
bundled server in default mode and that `GIMBAL_SERVER_URL` selects an externally
managed server without transferring ownership to the app.

The standard CLI uses Go plugins, so published builds enable CGO on native
macOS ARM64 and Linux AMD64/ARM64 runners. The macOS CLI needs the tested
library-validation exception for user-built plugins; the desktop app does not.
GoReleaser packages CLI archives; Apple tools sign and notarize Mac artifacts.
The initial Linux channel is GitHub Release downloads, with SHA-256 checksums.

## Check and demonstrate the change

Run required repository checks appropriate to the change:

| Check | Purpose |
| --- | --- |
| `just vet` | Go vet, binary build, and Gimbal lint. |
| `just test` | Go and frontend unit tests. |
| `just fmt-check` | Frontend formatting. |
| `just e2e` after `just build` | Browser behavior against the built binary on an isolated port. Set `BASE_URL` to use an existing server. |

A documentation-only edit needs instruction and link checks. For application
behavior, run the real path the claim concerns: a workflow, separate CLI
processes for external steering, or the browser for a UI change. Use cheap
models for live demonstrations and name the model.

An independent validator inspects the work and whether its evidence establishes
the definition of done. Worker summaries and aggregate green gates cannot
establish unrelated behavioral claims. After relevant checks pass, repeat them
only when a change, failure, or unresolved concern warrants it.

In this repository, repeatable checks belong beside the code they test. Do not
commit proof programs, screenshots, logs, or run output. Describe observations
in the conversation or PR. Other projects using Gimbal retain their own policy
for keeping or sharing evidence.

## Review CLI documentation

For an added or changed built-in, read its rendered help as a caller. It must
explain purpose, inputs, meaningful defaults, outputs or changes, proof and
completion expectations, limits, and a useful example at suitable detail.

The entry function's doc synopsis supplies short help; package documentation
supplies long help; parameter field comments supply flag help. Check the built
command's workflow list and affected workflow help. A graph or output schema
does not document the invocation for its caller.

## Model catalog refresh

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
from the Doppler config that the repository secret `DOPPLER_TOKEN` grants, then
opens a PR for the generated files; it does not publish a release. The
repository's Actions setting permits bot-created PRs; the job grants only
contents and pull-request write access.

Review new and removed model IDs before merging. Aliases follow the catalog, but
built-in role defaults in `internal/builtin/defaults.json` and other configured
defaults, help, and skills name explicit models: update those to the new version
where it applies, keeping each role's intended model tier. Check the rendered
`gimbal run` role flags, not only the JSON. Tests for aliases should cover
resolution and overrides without fixing a moving shorthand to today's version.

Before tagging a release, build from a checkout of the pushed commit with
`just build`, run `./bin/gimbal`, and confirm its web page serves. After
tagging, repeat from a checkout of the exact tag. For this source proof, run
the checkout's binary directly; do not replace a Homebrew-managed executable
with `go install`.

## Merge and refresh the local installation

Commit and push meaningful work; follow repository review and squash-merge
practice within the user's authorized scope. Retain the task worktree when the
user has asked to keep it. Every release must update `CHANGELOG.md`: move the
relevant entries from `Unreleased` under a version heading with the release date,
update comparison links when tags exist, and leave a new empty `Unreleased`
section for subsequent work. Update affected skill and plugin instructions in
their maintained source before merging. After every merge, build the updated
source for verification and refresh the Gimbal skills. When a version is
published, update installed Gimbal executables through the public Homebrew tap;
do not overwrite a Homebrew-managed CLI with `go install`:

1. Fast-forward the main checkout and build the merged source with `just build`.
2. For a published version, upgrade the CLI and desktop app from the tap:

   ```sh
   brew upgrade --cask tylergannon/gimbal/gimbal
   brew upgrade --cask tylergannon/gimbal/gimbal-app
   ```

   Use `brew install --cask` for a cask that is not installed yet. Keep
   `./bin/gimbal` for source-checkout proof before publication.
3. Reinstall the published skills, including their supporting references:

   ```sh
   vp dlx -- skills add https://github.com/tylergannon/gimbal/tree/main/skills \
     --global --agent codex claude-code --skill '*' --yes
   ```

   The source URL scopes discovery to this repository's `skills/` directory;
   using the repository root would also discover internal `.agents/skills`.
   Keep the installer's default symlink mode. Refresh any affected plugin
   through its own installation mechanism as well.
4. Resolve the executable with `command -v gimbal`. Check `gimbal --help`,
   `gimbal run --help`, and affected workflow help; exercise the relevant
   installed behavior when a behavioral change requires it.
5. Compare installed instruction files with their maintained source. A
   successful install command alone does not establish that the copies match.

The maintained skill source is `skills/` in this repository.
The shared local installation location is `~/.agents/skills/`. The Gimbal set is
`gimbal`, `gimbal-workflows`, `gimbal-release`, and `gimbal-runs`. For an affected
plugin, inspect its actual installation and use its update mechanism; preserve
unrelated local configuration. Report when a new session is needed to load
updated instructions.

The documentation site is maintained in Gimbal View. Model-catalog refreshes
open source PRs; release publication is a separate tagged workflow.

## Report delivery

State the behavior observed, relevant checks, merge status, installed binary
path, and which installed instructions were refreshed. Separate local proof,
merged source, and installed behavior. Name unmet requirements and deferred
issues plainly.
