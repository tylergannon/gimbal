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

`.github/workflows/auto-release.yml` selects PR checks by changed paths. Docs run
instruction/link checks; automation runs focused script/hook tests; application
changes run affected unit tests. Non-documentation main changes run quick
preflight before reserving a candidate tag. The tagged workflow runs one complete
analysis, test, formatting and Svelte gate against its built Linux amd64 candidate
before publication, alongside native packaging. Main scope is
compared with the latest published tag, so coalescing cannot hide undelivered
application changes. Documentation-only changes do not publish an app. It moves Unreleased entries
into the next release changelog, records the candidate source SHA, and atomically
pushes the release metadata commit and tag. If main advanced during checks or
push, that stale candidate is skipped; the newer push is checked instead.
Release metadata uses a repository-scoped writable SSH deploy key fetched at
job runtime from Doppler `gimbal/stg`, secret `RELEASE_TAG_SSH_KEY`. Permit the
DeployKey actor class to bypass only version-tag creation; this bypass covers
**all repository deploy keys**. Keep version-tag update/deletion owner-only.
The read-only check job has no preparation environment or Doppler token.
The preparation job's `GITHUB_TOKEN` has contents read and actions write solely
to dispatch/wait for the existing build.

### Doppler access and GitHub environment guards

Doppler `gimbal/stg` is the source of truth for release credentials. The pinned
Doppler Secrets Fetch action masks fetched secret values; consumers use its
individual step outputs rather than injecting every secret into all processes.
Each fetch verifies the returned project/config before using its outputs.

Retain the GitHub environments as ref restrictions and deployment records:

- `release-preparation`: main only; release tagging and model-catalog refresh.
- `release`: version tags only; Apple signing/notarization and Homebrew updates.

Each environment stores only `DOPPLER_TOKEN`, the existing read-only service
token for `gimbal/stg`. Keep it out of repository-wide secrets so PR jobs cannot
access the release config. The shared token/config grants the trusted jobs read
access to all of `stg`; these environment guards restrict which refs may use it,
not individual Doppler secret permissions. Per-job consumers receive only the
values they need. Signing credentials and SSH private keys belong in Doppler,
not in GitHub environment secret copies. The temporary import workflow and
write token are no longer needed after migration.

### Release preparation key and rotation

The current repository deploy key is **Deploy**, GitHub key ID `166363428`,
with Ed25519 fingerprint
`SHA256:JpeaWBxFTi4w4ekBE7Jmz/dtPwP/kK9t3QWfbNrAUrY`. Its private value is
`RELEASE_TAG_SSH_KEY` in Doppler `gimbal/stg`. It has repository write access;
the key itself is not restricted to main. Record only its public identity here.
The previous key `166320859` was removed by the user during the Doppler cutover.
[Issue #516](https://github.com/tylergannon/gimbal/issues/516) tracks verification.

To rotate without interrupting preparation:

1. Let active release preparation finish.
2. Generate a replacement outside Git and add its public key as a writable
   deploy key on this repository. Keep the existing creation-only bypass.
3. Replace Doppler `gimbal/stg`'s `RELEASE_TAG_SSH_KEY` with the new private key.
4. Observe trusted-main tag preparation using the replacement, then remove the
   previous repository deploy key and update this public identity record.
5. Delete temporary local private-key files. Apple signing, notarization and
   Sparkle credentials remain unchanged during this SSH-key rotation.

SSH pushes trigger main workflows. The scope selector compares delivered source
with main, so the release metadata commit becomes a documentation-only change
after publication and cannot trigger another app release. PR events run scoped checks; manual main retries use the same release gate.
`release.yml` is dispatch-only: the preparation job selects **both** its ref and
tag input as the newly created tag, preventing competing tag-push and explicit
builds. Major version increments are forbidden while Gimbal is pre-release.
For a deliberately created minor version tag, explicitly run
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

`just build-assets` prepares generated source and frontend once. The release
workflow shares these as a revision-bound `release-inputs` artifact; consumers
restore it before using `RELEASE_PREPARED_INPUTS=1` with `scripts/release-cli.sh`.
A manual CLI build prepares those inputs itself. GoReleaser compiles the versioned
native target; the app bundles that CLI and prepares its Xcode project without
another frontend or Debug build. The Mac keeps compiler/tool caches locally;
`GIMBAL_DERIVED_DATA` selects its persistent Xcode cache. `GIMBAL_BUILD_JOBS=4`,
`GOFLAGS=-p=4`, and `GOMAXPROCS=4` bound native/Go work, with build steps at reduced
CPU priority. Mac packaging requires signing credentials and never substitutes
unsigned release assets. The workflow collects every required platform and Mac
artifact before creating the GitHub Release and updating the dedicated
`tylergannon/homebrew-gimbal` tap. Signing qualification is needed when the distribution path changes; successful
unchanged qualification is not repeated for every source merge.

### Fixed candidate artifact

Release assembles one final candidate even with `publish=false`. Its name is
`release-candidate-<run_id>-<run_attempt>`. The `candidate` job exposes
`artifact-id`, `artifact-digest`, `artifact-url`, `artifact-name` and the assembly's
`run-attempt`; its workflow summary records those upload identifiers alongside
repository, source SHA, tag and run identity. The artifact ID and digest belong
outside the bundle because they are assigned after its upload. Consumers select
that immutable artifact ID, rather than looking up a generic artifact name.

The directory contains the three `gimbal_<version>_<platform>.tar.gz` files,
`macos/Gimbal-<version>-macos-arm64.zip`, the matching DMG, `macos/appcast.xml`,
`checksums.txt` and `candidate.json`. The manifest has `schema_version: 1`,
`repository`, `source_sha`, `tag`, `run_id`, `run_attempt`, and a `files` array of
`{name, sha256}` for the seven publication files. It excludes its own content
and its upload identity. The checksum file retains the existing five-archive
publication format. App payloads and the signed feed are copied unchanged.

The existing `release-assets` helper offers `assemble-candidate --input DIR
--output NEW_DIR` and `verify-candidate --input DIR`. Both require `--repository
OWNER/REPO --source-sha FULL_SHA --tag v0.x.y --run-id ID --run-attempt N` and
emit the manifest JSON on stdout after successful identity/content checks.
Verification rejects changed, missing, extra or nonregular files, duplicate or
unsafe names, mismatched identity, misleading checksums and a feed with a
different version, repository, tag or ZIP size. It checks that the existing feed
signature is present; it does not perform signature trust or native acceptance
checks. Assembly refuses to overwrite an existing directory.

The normal publisher still waits for source qualification and platform builds.
It downloads the final artifact by ID and verifies its identities and hashes,
then uploads those files without recomputing checksums or regenerating packages
or appcast. A publisher retry retains the assembly's producing attempt. ID-based
downloads use `merge-multiple: true` to place content directly in the destination
([Actions download implementation](https://github.com/actions/download-artifact/blob/v4/src/download-artifact.ts)).

Candidate metadata contains no acceptance verdict. Assembly depends on platform
builds; an uploaded candidate alone does not establish source qualification or
product acceptance. Artifacts retain the existing **two-day lifetime**, so this
does not provide a durable handoff beyond that expiry. Observation, scheduling,
candidate selection and authority to promote are separate follow-up work.

Release credentials belong in Doppler `gimbal/stg`:

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
