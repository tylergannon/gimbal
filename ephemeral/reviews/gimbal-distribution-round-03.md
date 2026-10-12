# Adversarial review: Gimbal distribution round 03

**Review target:** current Gimbal distribution drafts in `/Users/tyler/.codex/worktrees/gimbal-distribution/gimbal`, against the user request, the current `ephemeral/research/distribution/plan.md`, and repository instructions.

**Scope:** standard CLI Homebrew and Linux binary releases; signed, notarized macOS app installation in `/Applications`; local signed proof before release automation activation.

**Evidence inspected:** current worktree status and the distribution implementation files: `.goreleaser.yaml`, `.github/workflows/release.yml`, `scripts/release-cli.sh`, `scripts/sign-notarize-macos-cli.sh`, `scripts/ci-signing.sh`, `scripts/release-macos.sh`, `scripts/update-homebrew-casks.sh`, both entitlements, distribution instructions, `cmd/gimbal` metadata changes, native build configuration, and relevant plan/research notes. The parent reports Homebrew 7.0.9 parsed the generated Casks from a temporary local tap: CLI is restricted to ARM64 macOS; the app is restricted to ARM64 macOS 27 and installs `Gimbal.app` into `/Applications`. The temporary tap was removed. The plan now explicitly calls for separate Cask names and one narrowly scoped tap-repository credential, matching the workflow. The parent confirms no push, PR, tag, tap publication, or CI activation has occurred. Available local app proof is unsigned; the Developer ID identity/certificate service remains unresolved. I made no implementation changes and ran no checks.

## Findings

No material findings remain in the current inactive draft. The earlier review observations about the Cask constraints and credential split are resolved by the current implementation and clarified plan.

## Outstanding activation gate

The requested local Developer ID signed build, accepted notarization, stapling, Gatekeeper assessment, and `/Applications` launch have not been demonstrated. This is a release sequencing blocker, not a defect in an activated workflow: the draft remains unpublished and inactive. Do not activate it or publish a tag/release/tap entry before that proof, as `ephemeral/research/distribution/plan.md:13` requires.

## Outcome

**no findings**
