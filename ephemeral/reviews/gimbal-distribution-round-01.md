# Adversarial review: Gimbal distribution round 01

**Review target:** current Gimbal distribution drafts in `/Users/tyler/.codex/worktrees/gimbal-distribution/gimbal`, against `ephemeral/research/distribution/plan.md`, the original distribution request, and `AGENTS.md`.

**Scope:** standard CLI Homebrew and Linux binary releases; signed and notarized `/Applications` macOS app; local build/sign proof before release automation is activated.

**Evidence inspected:** complete current `.goreleaser.yaml`, `.github/workflows/release.yml`, `scripts/release-cli.sh`, `scripts/sign-notarize-macos-cli.sh`, `scripts/ci-signing.sh`, `scripts/release-macos.sh`, `scripts/update-homebrew-casks.sh`, entitlement plists, relevant `cmd/gimbal` changes, distribution instructions, plan, native app source/configuration, and worktree status. The Homebrew cask draft now includes ARM64 and macOS constraints for the CLI and ARM64/macOS 27 constraints for the app; earlier observations about those constraints are resolved and omitted. Local evidence available to this review is an unsigned Release app build and live UI check; signing preflight stopped because no valid Developer ID identity was available. The parent reports Apple membership is active but Apple's certificate service still rejects the team. The parent confirms the workflow has not been pushed or activated and no release tag or tap publication has been created. I made no implementation changes and ran no checks.

## Findings

### 1. Issue — the tap updater uses one credential where the approved plan requires separate CLI and app credentials

The plan explicitly selects “separate CLI and desktop app tokens” (`ephemeral/research/distribution/plan.md:7`). The workflow checks out the single tap with only `secrets.TAP_GITHUB_TOKEN` (`.github/workflows/release.yml:162-167`) and then one updater commits both `gimbal.rb` and `gimbal-app.rb` (`scripts/update-homebrew-casks.sh:19-51,54-62`). Thus the current publishing path cannot enforce the planned credential boundary: the same token that can change the CLI cask can also change the app cask, and vice versa. Implement the planned separation in the publishing design or explicitly revise the plan before treating this as its implementation.

## Outstanding activation gate

The requested local signed/notarized proof has not been demonstrated: the available run proves only an unsigned app build and live UI, and signing preflight currently fails for lack of a valid Developer ID identity. The plan forbids tag, release, tap publication, or signing-secret workflow activation before that proof (`ephemeral/research/distribution/plan.md:13`). The workflow remains an unpushed draft, so this is not a finding against an already activated pipeline; keep it inactive until local Developer ID signing, Apple notarization acceptance, stapling, and Gatekeeper verification pass.

## Outcome

**material findings remain**
