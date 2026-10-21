# Source development

Use `just dev` for the normal Go/frontend edit loop. For a source CLI or a native
candidate that needs embedded frontend assets, use `just build`. It installs
frontend dependencies, generates Go/TypeScript/Swift bindings, builds the web
bundle and embeds `web/build/` in `bin/gimbal`. Generators own their output;
regenerate rather than hand-editing it. Generated build assets stay local.

`just app` builds a Debug macOS app with its matching CLI in Resources. For native
user testing, use `just app-validate`: it signs the built app with an
existing Developer ID Application certificate, using the release identity and
entitlements. `MACOS_SIGNING_IDENTITY` chooses the exact identity; when omitted,
the single valid Developer ID identity is used. Keep that identity and bundle ID
stable across validation builds. An ad-hoc build identifies a code hash, so its
Documents, microphone and speech permission grants cannot be assumed to survive rebuilds. Do
not rename the app or reset macOS privacy permissions to get around this.

Start each user-testing task by recording the existing authorized test app's
absolute path, `CFBundleIdentifier`, signing identity and designated requirement.
Reuse that context across candidates. Do not make commit-named app copies, invent
bundle IDs, relabel/re-sign ad hoc for each revision, or overwrite the installed
stable release. Stage candidate builds separately and replace the established
test target only at an agreed handoff when its tester is idle. If the established
signing identity is unavailable, report that blocker instead of falling back to
ad-hoc signing or manufacturing certificates. Use the existing
`MACOS_SIGNING_IDENTITY`/`scripts/sign-native-dev.sh` workflow; never weaken its
designated requirement or change macOS permissions to avoid a prompt.

If a File Provider workspace keeps restoring `com.apple.FinderInfo` on generated
app output and signing rejects that metadata, use one fixed staging location
outside File Provider storage. Copy only the generated bundle with
`ditto --noextattr --norsrc`, then run `scripts/sign-native-dev.sh` on that staged
bundle. This is build-artifact preparation, not a privacy reset. Keep the staging
and launch paths stable; move the signed candidate to the established test target
only at the tester's idle handoff, never over its running bundle.

Version/build numbers and visible source provenance are separate from privacy
identity. `CFBundleVersion` alone does not explain TCC changes: compare the actual
signing requirement, identifier and path. Debug metadata defaults to `0.1.0 (1)`;
release packaging supplies the official version. Keep one stable test display
name, report it as unreleased, and use the existing machine/build source readout
for commit provenance instead of changing bundle metadata for every commit.

Launch
through `just app-run /absolute/disposable/catalog` to isolate its instance state.
The built app is under `native/build/macos/DerivedData/Build/Products/Debug/`.
`app-run` only opens the prepared app; it does not replace `app-validate` or prove
that the existing privacy identity was preserved. Verify the actual running PID,
bundle path, isolated catalog and source before the first user action.
For optimized/signed distribution probes, read [distribution.md](distribution.md).

The app normally owns its bundled CLI server. Closing its window leaves that
server running; Command-Q stops it. `GIMBAL_SERVER_URL` selects an external
server and leaves its lifetime externally owned. Real recording needs the native
app so microphone/speech permissions are attributed correctly. The synthetic
preset supplies deterministic speech for development.

## Native test ownership and cleanup

The tester owns the lifetime of each app it launches. Before launching, record
the existing Gimbal app and bundled-server PIDs, parent PIDs, executable paths,
catalog arguments, and canonical project paths. Reuse an existing test instance
only when its path, catalog, source and ownership match the agreed test slot.
`open -n` explicitly creates another instance; `just app-run` is a launcher, not
a qualification workflow with automatic shutdown.

Register cleanup before the first launch. Scripted or supervised tests need a
`finally`/`EXIT`, `INT`, and `TERM` cleanup path, including failed assertions and
interrupted work. Scope it to the exact app/server PIDs created by that test and
recheck their executable and catalog ownership before acting. Never use
`killall Gimbal`, a broad `pkill`, or terminate unrelated instances. For manual
agent testing, make cleanup the final required step and the first action after
resuming an interrupted test; use a bounded cleanup guard for unattended probes.

On success, failure, or cancellation, use normal **Quit Gimbal / Command-Q** on
the owned instance. Closing its window, closing a view, or reporting “GUI
ownership released; app idle” does not stop it or its server. If actual unsaved
work or a confirmation prevents quitting, preserve it and report the remaining
PID, path, catalog and blocker. Keep stored catalogs, documents and history.
Leave an app running only for an explicitly accepted handoff to a named next
tester, who assumes cleanup ownership; report that transfer rather than declaring
the test cleaned up. Honor an explicit user request to quit all instances,
including the installed app, without deleting their data.

After Quit, verify from read-only process inspection that the recorded app PID
and its owned server exited, and compare against the pre-test inventory. Do not
query the closed app through computer-use `getApp` or `getAXState` to verify
shutdown: those calls can launch it again, without the disposable catalog
override. Do not finish while an owned qualification instance remains running.
Report the before/after PID and path inventory with any preserved instances.

The project lock is separate from this cleanup responsibility. The host
canonicalizes a project path with `Abs` and `EvalSymlinks`, then holds an exclusive
nonblocking lock on that project's `.gimbal/owner.lock` while it owns the project.
It prevents concurrent ownership of the same project, including path aliases;
it is not a global single-app lock or a lock on the catalog directory. Distinct
projects can therefore have separate native instances. A lock check must attempt
the same canonical project from two owners, observe refusal while the first
owns it and admission after it quits, then clean up both instances.

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
