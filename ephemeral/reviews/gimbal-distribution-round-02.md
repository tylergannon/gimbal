# Adversarial review: Gimbal distribution round 02

**Review target:** current Gimbal distribution drafts in `/Users/tyler/.codex/worktrees/gimbal-distribution/gimbal`, against the user request, `ephemeral/research/distribution/plan.md`, and repository instructions.

**Scope:** standard CLI Homebrew and Linux binary releases; signed, notarized macOS app installation in `/Applications`; local signed proof before release automation activation.

**Evidence inspected:** current worktree status and all distribution implementation files, including `.goreleaser.yaml`, `.github/workflows/release.yml`, all `scripts/release-*`, signing and cask updater scripts, both entitlement plists, distribution instructions, `cmd/gimbal` metadata changes, native build configuration, and the relevant repository/plan sources. The parent reports Homebrew 7.0.9 parsed both generated Casks from a temporary tap, including the ARM64/macOS constraints and `/Applications/Gimbal.app` destination; that temporary tap was removed. The earlier Cask findings are resolved. The parent confirms there has been no push, PR, tag, tap publication, or CI activation. Local signed/notarized proof remains unavailable because the Developer ID identity/certificate service is unresolved; the extant build/UI evidence is unsigned only. I made no implementation changes and ran no checks.

## Findings

### 1. Issue — the implementation does not provide the separate CLI and app publishing credentials selected by the plan

The approved plan requires “separate CLI and desktop app tokens” (`ephemeral/research/distribution/plan.md:7`). The current workflow checks out the single tap using only `secrets.TAP_GITHUB_TOKEN` (`.github/workflows/release.yml:162-167`), then one script writes and pushes both `Casks/gimbal.rb` and `Casks/gimbal-app.rb` (`scripts/update-homebrew-casks.sh:20-51,54-62`). The distribution instructions likewise document only this one secret (`skills/gimbal-release/references/distribution.md`, “Automation”). Consequently the submitted implementation cannot demonstrate or enforce the credential split it claims to follow. Either implement the approved split in the publishing design or revise the plan to explain that a single repository-scoped tap credential is intentional; until then the plan and setup contract disagree.

## Outstanding activation gate

The requested Developer ID signed local build, accepted notarization, stapling, Gatekeeper check, and `/Applications` launch have not been demonstrated. This does not constitute an activated-pipeline finding because the workflow remains uncommitted/unpublished and no tag or tap update has occurred. Keep it inactive until the required local proof succeeds, as `ephemeral/research/distribution/plan.md:13` requires.

## Outcome

**material findings remain**
