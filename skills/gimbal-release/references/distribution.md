# Build distribution

Gimbal publishes the CLI/server separately from its desktop client. The
macOS builds currently require Apple Silicon and macOS 27. The desktop client connects
to a separately started server. Linux releases support AMD64 and ARM64.

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
against the independently running matching CLI. Signature checks alone do not
prove app launch or server reachability. Never report unsigned/ad-hoc probes as
Developer ID or notarization proof.

The standard CLI loads native Go plugins. It must retain CGO. macOS CLI signing
uses `scripts/macos-cli.entitlements.plist` to allow independently built plugins;
the desktop entitlement file grants only audio input. Standalone Mach-O CLI
binaries cannot have their notarization ticket stapled, so first Gatekeeper
assessment needs Apple's online ticket lookup. Downloaded plugins need their
own notarization or normal user approval. Plugins must match the host's exact
Go toolchain, dependencies, tags and build flags; inspect `go version -m` and
build plugins from the corresponding released source.
Releases pin Go 1.27.2 or newer: Go 1.27.1 has an upstream Mach-O plugin linker
bug on macOS 27. Match the exact patch version reported by the host binary.

## Automation

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
