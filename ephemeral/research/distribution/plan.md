# Gimbal build publication

User request, 2026-10-12: publish the standard Gimbal CLI through Homebrew, provide Linux binaries, and ship a signed macOS application installable in /Applications. Prove local build and signing before activating release automation. Four Luna agents independently mapped the entire process; map-1.md through map-4.md retain their findings and disagreements.

## Decisions

Use GoReleaser OSS for versioned CLI archives and checksums. Build on native macOS ARM64 and Linux AMD64/ARM64 runners with CGO enabled: the standard CLI loads Go plugins. Linux starts with conventional GitHub Release tarballs; no new apt/rpm repository is needed. Current GoReleaser recommends a binary Homebrew Cask for precompiled CLIs and deprecates its old Formula publisher. Use a dedicated public Homebrew tap, separate Cask names (`gimbal` for the CLI and `gimbal-app` for the desktop), and exact archive checksums. One narrowly scoped tap-repository credential updates both Casks.

Use Apple's tools to build, Developer ID sign, notarize and staple the existing SwiftUI/macOS app. The app is currently ARM64/macOS 27 and connects to the independent Gimbal server. Packaging preserves that ownership boundary; it does not start a new server or promise Intel/older-macOS support. Rebuild the generated Xcode project in Release configuration rather than publishing the generator's unsigned Debug output.

GitHub's hosted xcode-27 runner matches the native target and is the initial automation choice. It is public preview. The offered amritanandamayi self-hosted Mac is a fallback if qualification reveals an actual hosted-runner limit. The SwiftUI/cgo app requires Apple's macOS SDK and tools; Linux is a CLI build platform, not a native app signing platform.

No tag, release, tap publication or signing-secret workflow activation precedes local signing/notarization proof. Draft implementation can be reviewed on the task branch while account setup proceeds. Published jobs fail rather than falling back to unsigned assets.

## Secrets and account setup

Local signing needs a Developer ID Application certificate and its private key in Keychain. Local notarization uses a notarytool Keychain profile. Hosted automation needs the exported encrypted PKCS#12 identity and password, a narrowly scoped notarization credential, and separate repository write access to the Homebrew tap. Secret bytes never enter research notes, chat, source or ordinary build logs. Required account actions and user entry are handled in the requesting chat.

Membership renewal was completed by the user during this session. The account now shows renewal through October 12, 2027. The certificates service initially continued rejecting the same team as ineligible after refresh; distinguish that service response from unpaid membership.
