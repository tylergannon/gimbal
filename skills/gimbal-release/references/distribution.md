# Build distribution

Gimbal publishes the CLI/server separately from its desktop client. The
macOS desktop client currently requires Apple Silicon and macOS 27 and connects
to a separately started server. Linux releases support AMD64 and ARM64.

## Local qualification

Run `just build` to prepare the embedded web application. Build an optimized
unsigned desktop candidate with:

```sh
scripts/release-macos.sh --build-only --version 0.13.0 --build-number 1
```

The script generates the pinned SKGo native project, then rebuilds it in Xcode
Release configuration. `--skip-generate` reuses a freshly prepared native
project; use it only when its SDK, Go archive and generated Swift bindings match
the candidate source. Generated artifacts stay out of commits.

To qualify distribution, use a valid Developer ID Application identity in
Keychain and a `notarytool` Keychain profile:

```sh
export MACOS_SIGNING_IDENTITY='Developer ID Application: NAME (TEAM_ID)'
export MACOS_NOTARY_PROFILE=gimbal-release-notary
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

## Automation

`.github/workflows/release.yml` builds stable semantic-version tags. Linux runs
on native Ubuntu AMD64/ARM64; Mac assets use the hosted `xcode-27` ARM64 image,
which supports the app's SDK requirement and is currently public preview.
Use the offered self-hosted Mac only if an actual hosted-runner limitation is
established, after checking its OS, Xcode, architecture and access controls.

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
- `TAP_GITHUB_TOKEN`: narrowly scoped Contents write access to the tap repository.

User credential creation/entry stays in the requesting chat. Never put secret
bytes in prompts, source, worklogs or ordinary command output. CI imports the
identity into a temporary Keychain, registers its notary profile, and removes
private files and the Keychain afterward. Restrict the release environment to
trusted version tags. Linux builds do not require Apple credentials.

After assets and tap entries are published, install the CLI with
`brew install --cask tylergannon/gimbal/gimbal`, and the desktop client with
`brew install --cask tylergannon/gimbal/gimbal-app`. Linux users download the
architecture-specific tarball from GitHub Releases, verify `checksums.txt`, and
install its `gimbal` executable on PATH. Record the actual release URL and
install/launch observations; prepared scripts and a queued workflow are not a
published release.
