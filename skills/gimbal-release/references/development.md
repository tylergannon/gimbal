# Source development

Use `just dev` for the normal Go/frontend edit loop. For a source CLI or a native
candidate that needs embedded frontend assets, use `just build`. It installs
frontend dependencies, generates Go/TypeScript/Swift bindings, builds the web
bundle and embeds `web/build/` in `bin/gimbal`. Generators own their output;
regenerate rather than hand-editing it. Generated build assets stay local.

`just app` builds a Debug macOS app with its matching CLI in Resources. For native
microphone validation, use `just app-validate`: it signs the built app with an
existing Developer ID Application certificate, using the release identity and
entitlements. `MACOS_SIGNING_IDENTITY` chooses the exact identity; when omitted,
the single valid Developer ID identity is used. Keep that identity and bundle ID
stable across validation builds. An ad-hoc build identifies a code hash, so its
microphone/speech permission grants cannot be assumed to survive rebuilds. Do
not rename the app or reset macOS privacy permissions to get around this.

Launch
through `just app-run /absolute/disposable/catalog` to isolate its instance state.
The current app is under `native/build/macos/DerivedData/Build/Products/Debug/`.
For optimized/signed distribution probes, read [distribution.md](distribution.md).

The app normally owns its bundled CLI server. Closing its window leaves that
server running; Command-Q stops it. `GIMBAL_SERVER_URL` selects an external
server and leaves its lifetime externally owned. Real recording needs the native
app so microphone/speech permissions are attributed correctly. The synthetic
preset supplies deterministic speech for development.

Run checks for the changed contract: affected Go packages, frontend units or
browser behavior, generated-output/freshness checks, or the actual native path.
`just check-cli-release` is a separate real-executable gate using the already
built CLI. `just lint-go`, `just vet`, `just test`, `just fmt-check`, and
`just check-web` are explicit repository-wide checks owned by the main release
gate. Invoke them locally when the change warrants that coverage, not as a
ritual at every commit/merge/tag milestone.

When published skills changed, refresh from this repository's maintained skill
root, preserving supporting references and installer symlink mode:

```sh
vp dlx -- skills add https://github.com/tylergannon/gimbal/tree/main/skills \
  --global --agent codex claude-code --skill '*' --yes
```

Scope discovery to `/skills`; repository-root discovery includes internal skills.
Compare affected installed files to source, and report whether a new session is
needed. For optional web authentication and server configuration, use the current
repository README rather than reconstructing its settings from examples.
