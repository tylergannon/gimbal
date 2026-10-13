# Build distribution

Gimbal publishes the CLI/server separately and also bundles it in the desktop
app. The macOS builds currently require Apple Silicon and macOS 27. By default,
the desktop app starts and manages its bundled CLI server. Its central project
catalog and instance control state live in `~/.gimbal/desktop`; project runs
and conversations remain under each project's `.gimbal` directory. Setting
`GIMBAL_SERVER_URL` before app launch selects an external server, which the app
does not start or manage. Closing the app window leaves the app-owned server
running; Command-Q stops it. Linux releases support AMD64 and ARM64.

## Local qualification

Run `just build` to prepare the embedded web application. Build an optimized
unsigned desktop candidate with:

```sh
scripts/release-macos.sh --build-only --version 0.13.0 --build-number 1
```

The script generates the pinned SKGo native project, then rebuilds it in Xcode
Release configuration. Its Xcode package fetches use netrc authorization and
system Git so the public Sparkle dependency does not request login Keychain
credentials. `--skip-generate` reuses a freshly prepared native
project; use it only when its SDK, Go archive and generated Swift bindings match
the candidate source. Generated artifacts stay out of commits.

To qualify distribution, use a valid Developer ID Application identity in
Keychain and a `notarytool` Keychain profile:

```sh
export MACOS_SIGNING_IDENTITY='Developer ID Application: NAME (TEAM_ID)'
export MACOS_NOTARY_PROFILE=gimbal-release-notary
export MACOS_KEYCHAIN_PATH="$HOME/.config/gimbal-release/release.keychain-db"
export SPARKLE_ED_KEY_FILE="$HOME/.config/gimbal-release/sparkle-ed25519.key"
scripts/release-macos.sh --release --version 0.13.0 --build-number 1
```

Replace the example version with the actual release version. The script verifies
signatures, requires accepted notarization, staples and assesses the app, then
produces a signed and stapled DMG with an Applications link and a ZIP containing
the stapled app. Install the candidate in `/Applications` and observe its window
with the bundled-server default. Also launch with `GIMBAL_SERVER_URL` pointing
at a test server; verify that the app proxies to it and does not stop it on
quit. Signature checks alone do not prove app launch or server reachability.
Never report unsigned/ad-hoc probes as Developer ID or notarization proof.

The standard CLI loads native Go plugins. It must retain CGO. macOS CLI signing
uses `scripts/macos-cli.entitlements.plist` to allow independently built plugins;
the desktop entitlement file grants only audio input. Standalone Mach-O CLI
binaries cannot have their notarization ticket stapled, so first Gatekeeper
assessment needs Apple's online ticket lookup. Downloaded plugins need their
own notarization or normal user approval. Plugins must match the host's exact
Go toolchain, dependencies, tags and build flags; inspect `go version -m` and
follow the source workspace recipe below.
Releases pin Go 1.27.2 or newer: Go 1.27.1 has an upstream Mach-O plugin linker
bug on macOS 27. Match the exact patch version reported by the host binary.

## Plugin builds

Published CLIs use `-trimpath` to remove machine-specific source paths from Go's
package identity. Their plugins need the same flag, exact Go patch version,
shared dependency versions, build tags and CGO environment. Build the bundled
filesystem plugin from a checkout of the host's release tag:

```sh
git clone --branch v0.13.2 --depth 1 https://github.com/tylergannon/gimbal.git /abs/gimbal-source
CGO_ENABLED=1 GOTOOLCHAIN=go1.27.2 go -C /abs/gimbal-source build -trimpath -buildmode=plugin \
  -o /abs/plugins/filesystem.so ./plugins/messaging-filesystem
```

Use the actual installed host version in place of the example tag. An external
plugin project must also use that source checkout as a workspace main module.
A plain module-cache dependency has a different package identity from the
release host even when its version matches. Keep the workspace in a separate
directory and select it explicitly. Use canonical absolute paths for the
workspace entries, as returned by `realpath`:

```sh
mkdir -p /abs/plugin-workspace
GOTOOLCHAIN=go1.27.2 go -C /abs/plugin-workspace work init /abs/gimbal-source /abs/my-plugin
GOWORK=/abs/plugin-workspace/go.work CGO_ENABLED=1 GOTOOLCHAIN=go1.27.2 \
  go -C /abs/my-plugin build -trimpath -buildmode=plugin -o /abs/plugins/my-plugin.so .
```

The plugin project's dependencies must preserve the host's shared versions;
Go workspace version selection can otherwise upgrade them. Build for the
host's native OS/architecture with its matching C compiler environment. A
local development host built without `-trimpath` needs plugins built with that
same flag choice and source checkout. The release packaging gate compiles an
external plugin in a separate source workspace and loads it with the actual
archive's CLI before permitting publication.

## Automation

`.github/workflows/auto-release.yml` checks each pushed main revision with the
normal build, vet, test, formatting and Svelte gates. It moves Unreleased entries
into the next patch changelog, records the checked source SHA, and atomically
pushes the release metadata commit and tag. If main advanced during checks or
push, that stale candidate is skipped; the newer push is checked instead.
Release metadata uses a repository-scoped writable SSH deploy key from the
main-only `release-preparation` environment. Configure its `RELEASE_TAG_SSH_KEY`
secret and permit that deploy key to bypass only the version-tag creation rule;
keep version-tag update/deletion owner-only. The read-only check job has no
preparation environment or SSH key. The preparation job's `GITHUB_TOKEN` has
contents read and actions write solely to dispatch/wait for the existing build.
Apple credentials remain in the separate tag-restricted `release` environment.

SSH pushes trigger main workflows, so the check job skips bot-authored commits
carrying `Release-Source:` metadata. PR and manual events still run checks.
`release.yml` is dispatch-only: the preparation job selects **both** its ref and
tag input as the newly created tag, preventing competing tag-push and explicit
builds. For a deliberately created minor/major version tag, explicitly run
`gh workflow run release.yml --ref vX.Y.Z -f tag=vX.Y.Z` after pushing that tag.

Automatic releases wait for the tagged workflow under a shared concurrency
group, keeping publication ordered while pending main pushes coalesce. A rerun
reuses the source's tag and an active or successful build. A failed tagged build
is dispatched again at the same tag; existing release assets are replaced before
retrying the Homebrew update. Use the **Automatic release** Actions workflow's
manual dispatch on main to retry a failed delivery without a new source commit.

`.github/workflows/release.yml` builds stable semantic-version tags. Linux runs
on native Ubuntu AMD64/ARM64; Mac assets use the repo-scoped self-hosted
`amritanandamayi` runner with the `gimbal-release` label, Apple Silicon,
macOS 27 and Xcode 27. The signing job runs only for version-tag refs,
including manual dispatches selected on that same tag. Keep this runner out of
pull-request workflows; run it under its user LaunchAgent with the pinned
toolchain available on PATH.

`scripts/release-cli.sh` runs the full frontend build and GoReleaser OSS for its
native target. Mac packaging requires signing credentials and never substitutes
unsigned release assets. The workflow collects every required platform and Mac
artifact before creating the GitHub Release and updating the dedicated
`tylergannon/homebrew-gimbal` tap. Automation activation follows local signed
qualification, not merely config validation.

Secrets belong in the GitHub `release` environment:

- `DEVELOPER_ID_CERTIFICATE_P12_BASE64`: exported certificate and private key.
- `DEVELOPER_ID_CERTIFICATE_PASSWORD`: PKCS#12 encryption/import password.
- `APPLE_TEAM_ID`: identity team identifier.
- `APP_STORE_CONNECT_KEY_ID` and `APP_STORE_CONNECT_ISSUER_ID`: API key identifiers.
- `APP_STORE_CONNECT_API_KEY_P8_BASE64`: downloaded team API private key.
- `TAP_DEPLOY_KEY`: SSH deploy private key with write access only to the tap.
- `SPARKLE_ED_PRIVATE_KEY`: Sparkle EdDSA private key matching `SUPublicEDKey`
  in `native/skgo-native.json`. Back it up privately; future updates require it.

The desktop client uses Sparkle 2.10.0. It checks daily by default and presents
the standard install-and-relaunch prompt. **Gimbal > Check for Updates…** starts
a manual check. `SUAutomaticallyUpdate` is false, so background checks do not
silently replace the app. Sparkle is linked only on macOS through the local
`native/Updates` Swift package.

The release script runs Sparkle's `generate_appcast` against the final notarized,
stapled ZIP and publishes `appcast.xml` with the other release assets. The app
reads `https://github.com/tylergannon/gimbal/releases/latest/download/appcast.xml`;
each feed points at the immutable ZIP URL for its version tag. The feed becomes
live when the stable GitHub Release is published. Keep `CFBundleVersion` build
numbers increasing; Sparkle compares those numbers, rather than the displayed
marketing version. Local test releases must use a separate feed.

User credential creation/entry stays in the requesting chat. Never put secret
bytes in prompts, source, worklogs or ordinary command output. CI imports the
identity into a temporary Keychain, registers its notary profile, and removes
private files and the Keychain afterward. `MACOS_KEYCHAIN_PATH` selects the exact
Keychain for identity lookup, every `codesign` operation, and all `notarytool`
profile lookups/submissions. It is optional for local commands that intentionally
use the default Keychain. CI sets it to the temporary release Keychain and does
not change global Keychain search settings. Export password-protected PKCS#12
identities with Apple's compatible MAC SHA-1 and PBE-SHA1-3DES encryption; newer
OpenSSL defaults may not import into macOS Keychain. Restrict the release
environment to trusted version tags. Linux builds do not require Apple credentials.

After assets and tap entries are published, install the CLI with
`brew install --cask tylergannon/gimbal/gimbal`, and the desktop client with
`brew install --cask tylergannon/gimbal/gimbal-app`. Linux users download the
architecture-specific tarball from GitHub Releases, verify `checksums.txt`, and
install its `gimbal` executable on PATH. Record the actual release URL and
install/launch observations; prepared scripts and a queued workflow are not a
published release.
