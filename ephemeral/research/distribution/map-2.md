# Gimbal release distribution map 2

Research snapshot: 2026-10-12. Scope: read-only review of the current Gimbal source/build shape and primary vendor documentation. No source, account, or signing configuration was changed.

## Recommendation

Treat this as two releases sharing one version tag and GitHub Release page:

1. **`gimbal` CLI:** publish versioned macOS arm64 and Linux amd64/arm64 tarballs, with SHA-256 checksums, on GitHub Releases. Make a small `tylergannon/homebrew-tap` the supported Homebrew install path for the CLI (`brew tap tylergannon/tap && brew install gimbal`). Start with a hand-maintained formula that points at the release tarballs; automate tap updates only after the first end-to-end release is reliable. Do not submit to Homebrew Core as the first step.
2. **`Gimbal.app`:** publish a macOS arm64 app archive (and optionally a drag-install DMG) that is Developer ID signed, hardened-runtime enabled, notarized, and stapled. Add a separate `gimbal-app` Homebrew Cask only if Homebrew app installation is wanted; the CLI formula and the app cask serve different products.

Use GitHub-hosted macOS for the eventual signing/notarization job. Use `amritanandamayi` only for local build/sign proof if it is a trusted, controlled Mac; do not attach long-lived Apple signing secrets to a reused self-hosted Actions runner. Linux is a good runner for Linux artifacts, not for the Swift/Xcode app. Cross-compiling the Go CLI does not replace the Xcode native-app build.

## What the repository actually builds today

- `just build` runs `go mod tidy`, installs the `web/` pnpm dependencies, runs Go generation, builds the SvelteKit app, checks `web/build/skgo.manifest.json`, and compiles `bin/gimbal`. `web/server.go` embeds that generated build. A plain versioned `go install github.com/tylergannon/gimbal/cmd/gimbal@TAG` does not run that frontend pipeline, so it is not the release artifact path for the complete product.
- `go tool skgo native build --platform macos --preset real` builds the separate SwiftUI app and its Go presentation host. The pinned skgo native builder currently creates a Debug app, sets `CODE_SIGNING_ALLOWED=NO` when no development team is supplied, and hard-codes `ARCHS=arm64`/`ONLY_ACTIVE_ARCH=YES`. It invokes Xcode, Swift, Zig and the macOS SDK. The documented output is `native/build/macos-real/DerivedData/Build/Products/Debug/Gimbal.app`.
- The native target is macOS 27.0. This checkout's host has Xcode 27.0/SDK 27.0. Current GitHub runner docs list `xcode-27` as public preview; stable macOS 15/26 labels cannot be assumed to include the required SDK. Use `runs-on: xcode-27` only after proving it has the SDK and required toolchain, or use a controlled Mac with Xcode 27. Lowering the app deployment target to use older runners changes the supported OS contract and needs a product decision.
- The app config sets macOS deployment target 27.0 and bundle id `com.tylergannon.gimbal.real` for the `real` preset. It contains microphone and speech-recognition purpose strings. The builder's generated Info.plist currently defaults to `CFBundleVersion=1` and `CFBundleShortVersionString=0.1.0`; set these to the release version before signing the distributable app.
- `registry.go` and `cmd/gimbal/daemon.go` import Go's `plugin` package. Preserve this functionality in published CLI builds: build with cgo enabled. The existing task-environment reference's `CGO_ENABLED=0` example is explicitly a stripped Linux binary for a constrained environment, not a suitable full release binary. Linux plugins also need to be built with matching Go toolchain, source/dependency graph, tags, flags, and architecture.
- The app does not embed/start the independent Gimbal project server. Its native host proxies presentation requests to a separately running Gimbal server, defaulting to `127.0.0.1:8080`; package instructions must explain this prerequisite.

Local environment checked read-only: macOS 27.0.1, arm64, Xcode selected at `/Applications/Xcode.app/Contents/Developer`, Go 1.27.2. `security find-identity -v -p codesigning` found **zero valid signing identities**. The current local build can prove the unsigned build path; signing/notarization proof awaits an active Developer Program team and a Developer ID identity.

## Local machine proof before automation

First prove the user's local build path with the current release source and observe the actual app and CLI outputs:

```sh
just build
./bin/gimbal --help
go tool skgo native build --platform macos --preset real
open native/build/macos-real/DerivedData/Build/Products/Debug/Gimbal.app
```

The local app run is useful for verifying launch, server connection, web view and microphone permission behavior. It does **not** establish a distributable signature.

After the Apple account is active and the signing identity is installed in the login keychain, produce a release configuration from the same reviewed source. The skgo builder's generated project can be rebuilt as Release with an explicit Developer ID identity; confirm the project and output paths from the build log rather than relying on stale derived data. The builder currently uses arm64 only, so make the first app asset arm64. Ensure the final Info.plist contains the actual tag-derived marketing/build versions, then sign the completed `.app` with `Developer ID Application`, `--options runtime`, and secure timestamp. This app has no declared hardened-runtime exception entitlements; add none unless notarization/runtime proof establishes a concrete need. Sign nested executable code inside-out if the bundle contains it; do not use `codesign --deep` as the signing recipe.

The generated project name comes from the Go project name (`Gimbal` here). After the first native build generates `native/build/macos-real/Gimbal.xcodeproj`, the local Release build/sign shape is:

```sh
xcodebuild -project native/build/macos-real/Gimbal.xcodeproj \
  -scheme Gimbal -configuration Release -sdk macosx \
  -derivedDataPath native/build/macos-real/DerivedData build \
  CODE_SIGNING_ALLOWED=NO
APP="$PWD/native/build/macos-real/DerivedData/Build/Products/Release/Gimbal.app"
/usr/libexec/PlistBuddy -c "Set :CFBundleShortVersionString $VERSION" "$APP/Contents/Info.plist"
/usr/libexec/PlistBuddy -c "Set :CFBundleVersion $BUILD_NUMBER" "$APP/Contents/Info.plist"
codesign --force --options runtime --timestamp \
  --sign "$DEVELOPER_ID_APPLICATION" "$APP"
codesign --verify --deep --strict --verbose=2 "$APP"
spctl --assess --type execute --verbose=4 "$APP"
```

Set `VERSION` from the release tag without its leading `v`, set `BUILD_NUMBER` to a monotonically increasing CI run/build number, and set `DEVELOPER_ID_APPLICATION` to the keychain identity's full subject name. This manual final signing route avoids the current generated Xcode project's `CODE_SIGNING_ALLOWED=NO` default and does not require Xcode-managed profiles; the app has no App Store capability that needs one. If Xcode output contains nested code in a later release, sign each nested executable/framework first and the app last, then keep `--deep` only on verification.

The native builder also invokes `mise`, the pinned `zig` toolchain from the SDK's `mise.toml`, and `xcodegen`; current local PATH has all three. A CI image needs those installed and the versions resolved from the copied/pinned SDK config, plus the Go 1.27.1-or-newer toolchain required by this repo. The current Xcode 27 runner label is preview, so this dependency should be an explicit release job prerequisite, not silently approximated with Xcode 26.

Use a protected local keychain profile with an App Store Connect API key for notarization (Apple ID plus app-specific password is supported, but the API-key flow avoids a user's 2FA password). The success path is:

```sh
codesign --verify --deep --strict --verbose=2 /path/to/Gimbal.app
spctl --assess --type execute --verbose=4 /path/to/Gimbal.app
ditto -c -k --keepParent /path/to/Gimbal.app /tmp/Gimbal-notary.zip
xcrun notarytool submit /tmp/Gimbal-notary.zip --key /secure/AuthKey_XXXXXX.p8 \
  --key-id "$APPLE_NOTARY_KEY_ID" --issuer "$APPLE_NOTARY_ISSUER_ID" --wait
xcrun stapler staple /path/to/Gimbal.app
xcrun stapler validate /path/to/Gimbal.app
```

For a local keychain profile, `xcrun notarytool store-credentials gimbal-notary --key /secure/AuthKey_XXXXXX.p8 --key-id "$APPLE_NOTARY_KEY_ID" --issuer "$APPLE_NOTARY_ISSUER_ID"` stores the API-key profile in Keychain; the corresponding submit uses `--keychain-profile gimbal-notary`. For noninteractive CI, passing `--key`, `--key-id`, and `--issuer` directly is supported. If publishing the CLI on macOS as a direct download, sign that Mach-O executable too and submit a zip containing it for notarization; Homebrew checksum verification alone is not a substitute for a Developer ID signature when users execute a quarantined downloaded binary.

Then copy the stapled app to `/Applications/Gimbal.app`, launch it, confirm Gatekeeper accepts it without an override, verify the bundle identifier/version and permission prompts, connect it to the independently running server, and exercise a real microphone recording. Test the exact downloaded archive/DMG with quarantine metadata on a clean macOS account or machine before release. A successful `notarytool` response alone is not the install proof; inspect the notary log on rejection and keep the submission id/log with release diagnostics.

Apple membership renewal and Account Holder access are required to create Developer ID certificates. The certificate is `Developer ID Application` for the app/CLI if signing the CLI separately, not `Mac App Distribution`; a `.pkg` would need a distinct `Developer ID Installer`. Exporting the signing identity means exporting its private key into a password-protected PKCS#12 (`.p12`) file. Treat both file and password as secrets and send them through different protected paths. Notary API key material is a `.p8` private key plus Key ID and Issuer ID. The parent agent is handling the user's account path; do not ask for or paste secret material into chat.

## Release automation after local proof

Use a tag-triggered workflow for `v*` tags (or an explicit protected manual release workflow), with a protected GitHub `release-signing` environment restricted to release tags and required reviewer approval. Do not expose signing or notarization credentials to pull-request or general push jobs. Give the publish job only `contents: write`; use `GITHUB_TOKEN` for the same-repository Release. Pin Actions to full commit SHAs, avoid executing fork code in the privileged signing job, and keep the app signing/notarization job separate from ordinary test/build jobs.

Suggested job boundaries:

1. **Verify source/tag:** ensure the tag resolves to the reviewed commit; run repository release checks; record tag and commit.
2. **CLI builds:** build the embedded-web CLI with the repository's complete frontend build pipeline, then produce cgo-enabled platform binaries. Use pinned Linux runner images so glibc compatibility is stable. Use a native `ubuntu-24.04-arm` runner for Linux arm64; use `ubuntu-24.04` for amd64. Do not set `CGO_ENABLED=0`. Re-link after `just build` with `go build -ldflags "-X main.version=${VERSION} -X main.commit=${GITHUB_SHA} -X main.date=${BUILD_DATE}" -o bin/gimbal ./cmd/gimbal`, with `BUILD_DATE=$(date -u +%Y-%m-%dT%H:%M:%SZ)`. Smoke-test `gimbal --help`, `gimbal --version`, and the web listener on each native runner; for plugin support, additionally verify a compatible plugin loads on the matching Linux target. Produce separate archives, e.g. `gimbal_<version>_darwin_arm64.tar.gz`, `gimbal_<version>_linux_amd64.tar.gz`, `gimbal_<version>_linux_arm64.tar.gz`, plus `SHA256SUMS`.
3. **macOS app:** on `xcode-27` arm64 (currently public preview), checkout the same tag, run the native build, produce the Release app, set the tag-derived bundle versions, Developer ID sign with hardened runtime, verify, submit for notarization, wait for acceptance, staple and validate, then create the final zip/DMG. Use a temporary keychain and remove it in an `always()` cleanup. Secrets belong to the protected environment. `notarytool` submission output and notary log are release diagnostics.
4. **Publish:** only after every required artifact job succeeds, create the GitHub Release and upload assets/checksums. Avoid draft releases that look installable before notarization is finished. Verify downloaded checksums and test the release assets on clean Linux and macOS targets.
5. **Homebrew tap:** formula installs the CLI archive for OS/architecture, verifies the archive's SHA-256 and exposes only `gimbal`. For a cask, download the app archive/DMG, install `Gimbal.app` into `/Applications`, and rely on the Developer ID/notarization plus checksum. Start with manual version/hash updates in a small tap repository; if automating later, use a narrowly scoped GitHub App installation token or a separate tap-specific credential, not a personal classic PAT.

Linux release scope should begin with amd64 and arm64 only if both exact cgo-enabled artifacts can be built and smoke-tested. On Linux, `plugin.Open` is a supported platform feature, but a Linux plugin must match the host Go build; release notes should distinguish support for built-in workflows from operator-supplied `.so` plugins and state that Go-version/build compatibility requirement. Avoid promising Alpine/musl compatibility from glibc-built archives unless a separate musl build and test is added.

## Runner trade-offs

| Option | Good for | Trade-off | Recommendation |
| --- | --- | --- | --- |
| GitHub-hosted macOS | Xcode/Swift build, app signing/notary upload, clean per-job machine | Higher per-minute cost than Linux and slower cold builds; Xcode 27 label is currently public preview | Use `xcode-27` for the macOS app release after validating the SDK/tools on that image. The app is arm64-only today. |
| `amritanandamayi` self-hosted | Immediate local build/sign proof if it is a Mac with the needed Xcode/SDK and cert | Need confirm OS/arch, admin, durable access and clean runner lifecycle; a persistent runner can be compromised by workflows, including through environment secrets | Use only for local proof or a one-off, closely controlled job. Do not register the everyday machine as a long-lived runner holding Apple keys. |
| Linux build/cross-build | CLI artifacts; Linux amd64/arm64 are available as native hosted runner labels | Cannot produce/test the SwiftUI app or notarize/staple an app bundle; cgo plugin support means cross builds need target C toolchains | Build Linux CLI natively on amd64 and arm64 Linux hosted runners. Do not use Linux to stand in for Xcode app packaging. |

For a public repository, GitHub's guidance says self-hosted runners should almost never execute arbitrary public-repo PR jobs: the runner is not a fresh isolated VM and can be persistently compromised. If the team eventually chooses self-hosted release signing for cost/control reasons, limit the runner group to this repository and release workflow, never run PR code there, use an ephemeral clean VM/host when possible, keep no unrelated credentials/network access on it, and still require release-environment approval. GitHub environments do not isolate a self-hosted machine.

## Risks / failure handling

- **Apple account/team inactive:** stop before creating/exporting a certificate; no notarization route exists until the membership is active. Current machine check finds no valid identity.
- **Notary rejection:** retain the submission ID, download the notary log with `xcrun notarytool log`, repair signing/runtime/entitlements or embedded-code issues, rebuild and sign, and submit a new exact artifact. Do not publish an unnotarized fallback as a macOS install.
- **Release artifact drift:** tie every asset to one version tag and commit, include checksums, and verify downloads before publishing; never mix the Debug app with Release CLI artifacts.
- **App target boundary:** current target is macOS 27.0 and arm64. If distribution should include older macOS or Intel, that is a source/toolchain feasibility decision before promising support; current config does not substantiate those claims.
- **CLI frontend absent:** any release path using only `go install @tag` produces an incomplete CLI relative to `just build`. Keep the committed/generated frontend story and release workflow aligned.
- **glibc/plugin mismatch:** build Linux assets on stable, pinned glibc runner images, test at the oldest supported distro, publish a plugin build recipe with the same exact Go toolchain, and do not silently claim CGO-free/static portability.
- **Formula write credential:** do not grant the Gimbal release workflow broad write access to a separate tap. Keep formula updates manual initially or use a narrowly scoped app credential.

## macOS CLI plugin-signing result

The standalone CLI needs one exception to hardened runtime if macOS users are to keep loading locally built Gimbal Go plugins. The CLI imports `plugin` and scans user-selected plugin directories (`registry.go`, `cmd/gimbal/daemon.go`); the native `Gimbal.app` shell does not load these plugins.

Apple's current rule: hardened runtime enables library validation by default. It permits Apple-signed code or libraries signed by the host's same Team ID. The `com.apple.security.cs.disable-library-validation` entitlement is specifically the exception for loading plug-ins/libraries without the host Team ID; Apple notes this triggers additional Gatekeeper checks. Apple's notarization guidance says to declare it for real plug-in hosts and not executables that do not host plugins. This makes the entitlement appropriate on the CLI only. It must not be placed on `Gimbal.app` or indiscriminately on every executable.

### Local runtime probe

On macOS 27.0 arm64 with Go 1.27.2, I built the actual CLI and existing `plugins/messaging-filesystem` using the same checkout/toolchain and default build flags:

```sh
go build -o /tmp/gimbal-plugin-probe.bmUy4l/gimbal ./cmd/gimbal
go build -buildmode=plugin -o /tmp/gimbal-plugin-probe.bmUy4l/plugins/filesystem.so ./plugins/messaging-filesystem
```

The Go linker marks the plugin Mach-O with an ad-hoc signature and no Team ID. The unsigned CLI loaded it and started the actual Gimbal server. Copying the CLI and signing only the host with `codesign --force --sign - --options runtime` caused `plugin.Open` to fail with `mapping process and mapped file (non-platform) have different Team IDs`. Signing the same CLI with hardened runtime and an entitlement plist containing only:

```xml
<key>com.apple.security.cs.disable-library-validation</key>
<true/>
```

made the same plugin load and the server start. `codesign --verify --strict` passed, the code directory reported `adhoc,runtime`, and entitlement inspection returned only `disable-library-validation=true`. This demonstrates runtime behavior on this host only. The ad-hoc signature is not a Developer ID signature, and no Developer ID or Apple notarization proof was possible because the local keychain contains zero valid signing identities.

The successful Gimbal invocation was:

```sh
/tmp/gimbal-plugin-probe.bmUy4l/gimbal-runtime-with-lv-exception \
  --plugin-dir /tmp/gimbal-plugin-probe.bmUy4l/plugins --no-web \
  --project /tmp/gimbal-plugin-probe.bmUy4l/project-with-lv \
  --instance-dir /tmp/gimbal-plugin-probe.bmUy4l/state-with-lv
```

It printed `control socket listening on /tmp/gimbal-plugin-probe.bmUy4l/state-with-lv/<socket>.sock`. With the same host signed with hardened runtime and no entitlements, it exited immediately from `plugin.Open` with the Team ID validation error before creating the socket.

### Release design for the CLI

Sign the final macOS CLI Mach-O with `Developer ID Application`, hardened runtime, secure timestamp, and exactly the library-validation entitlement; do not add `get-task-allow`, JIT, or unsigned-executable-memory exceptions absent separate evidence. An auditable XML plist can be generated in the release job or kept as a CLI-specific file:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
  <key>com.apple.security.cs.disable-library-validation</key><true/>
</dict></plist>
```

```sh
plutil -lint "$CLI_ENTITLEMENTS"
codesign --force --options runtime --timestamp \
  --entitlements "$CLI_ENTITLEMENTS" \
  --sign "$DEVELOPER_ID_APPLICATION" "$CLI_BIN"
codesign --verify --strict --verbose=2 "$CLI_BIN"
codesign -d --entitlements :- "$CLI_BIN"
```

Submit a zip containing the signed CLI with `xcrun notarytool submit ... --wait`; read and retain the notary log. Apple currently says notarization tickets are created for standalone binaries but cannot be stapled to a standalone Mach-O or ZIP. Gatekeeper finds the CLI ticket online at launch, so test the exact downloaded artifact with network access and quarantine behavior on a clean Mac. Do not use quarantine removal as a substitute. A distributed quarantined third-party plugin has a separate requirement: Apple says macOS 10.15+ loads quarantined plugins only if they are notarized, otherwise the user must explicitly approve it. Plugins built locally by the user are not internet-quarantined.

Keep the native SwiftUI `.app` signing path separate and without this entitlement: it links its Go presentation host into the bundle but does not call Gimbal's Go plugin loader. This avoids widening the app's dynamic-code trust boundary. The CLI already documents plugins as trusted native code requiring the same Go toolchain/source compatibility, so release notes should retain that operator trust requirement.

## Primary references

- Repository: [Gimbal release skill](../../skills/gimbal-release/SKILL.md), [`Justfile`](../../Justfile), [`native/skgo-native.json`](../../native/skgo-native.json), [`native/host/main.go`](../../native/host/main.go), [`registry.go`](../../registry.go), [`cmd/gimbal/daemon.go`](../../cmd/gimbal/daemon.go), and [`task-environments.md`](../../skills/gimbal-runs/references/task-environments.md).
- Apple: [Notarizing macOS software](https://developer.apple.com/documentation/security/notarizing-macos-software-before-distribution), [custom notarization workflow / `notarytool` / standalone ticket stapling limits](https://developer.apple.com/documentation/security/customizing-the-notarization-workflow), [notarization requirements and valid Developer ID certificate](https://developer.apple.com/documentation/security/resolving-common-notarization-issues), [Disable Library Validation entitlement](https://developer.apple.com/documentation/BundleResources/Entitlements/com.apple.security.cs.disable-library-validation), [Developer ID certificates](https://developer.apple.com/help/account/certificates/create-developer-id-certificates/), [sharing signing identities](https://developer.apple.com/documentation/Xcode/sharing-your-teams-signing-certificates), and [distribution signing / avoid `--deep` when signing](https://developer.apple.com/documentation/xcode/creating-distribution-signed-code-for-the-mac).
- GitHub: [hosted runner images and architectures](https://docs.github.com/en/actions/reference/runners/github-hosted-runners), [compromised-runner risk](https://docs.github.com/en/actions/concepts/security/compromised-runners), [environment secrets/protection](https://docs.github.com/en/actions/concepts/workflows-and-actions/deployment-environments), and [Actions runner pricing](https://docs.github.com/en/billing/reference/actions-runner-pricing).
- Homebrew: [formula cookbook](https://docs.brew.sh/Formula-Cookbook.html), [maintaining a tap](https://docs.brew.sh/How-to-Create-and-Maintain-a-Tap), and [cask cookbook](https://docs.brew.sh/Cask-Cookbook).
- Go: [`plugin` package platform and compatibility constraints](https://pkg.go.dev/plugin), [build and install commands](https://go.dev/doc/tutorial/compile-install).
