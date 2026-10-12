# Focused adversarial review: Gimbal self-hosted signing release round 05

**Review target:** final self-hosted macOS release draft and current GitHub release-environment settings in `/Users/tyler/.codex/worktrees/gimbal-distribution/gimbal`.

**Evidence inspected:** current `scripts/ci-signing.sh`, `scripts/release-macos.sh`, `scripts/sign-notarize-macos-cli.sh`, `.github/workflows/release.yml`, maintained distribution instructions, the accepted release plan, and read-only GitHub API state for the `release` environment, deployment branch policies, repository rulesets, and runner inventory. The release environment has one deployment policy: tag `v*.*.*` (policy id `62843103`); it has no required-reviewer protection rule. The repository rulesets endpoint currently returns an empty list. The runner inventory is empty, so the proposed `amritanandamayi` runner has not yet been registered. Parent reports successful local dedicated-Keychain signing/notarization and successful `just vet` and `just test`; those results were not independently rerun here. I accept the user's explicit choice of the named self-hosted Mac and its trust boundary; I did not treat self-hosting itself as a defect.

## Findings

### 1. Issue — a matching tag can execute arbitrary release code on the trusted self-hosted Mac without an approval gate

The macOS job runs on the persistent self-hosted machine for any matching tag ref (`.github/workflows/release.yml:69-71`). Its job-level condition checks only that the ref is a tag beginning with `v` and that a manual input matches the current ref (`release.yml:69`); the later CLI script validates semantic-version shape, but neither check restricts who may create or move that tag. The `release` environment currently allows tags named `v*.*.*` and has no reviewer protection rule, while the repository rulesets API returns no tag ruleset. Therefore any actor able to push a matching tag can cause that tag's checkout to execute on the owner's self-hosted Mac in the runner's user context and, after the signing step, with Apple signing/notarization secrets. A temporary Keychain limits credential persistence after normal cleanup; it does not make untrusted tagged source safe to execute on the host.

Before registering or publishing this self-hosted release path, restrict creation/update/deletion of `v*.*.*` tags to the release maintainer using a repository ruleset, and add a required reviewer to the `release` environment so the self-hosted job is held before its first step. The tag-only environment policy currently verifies only the ref name, not the actor or reviewed source. This matches GitHub's warning that self-hosted runners can be persistently compromised by workflow code: [GitHub Actions secure-use guidance](https://docs.github.com/en/actions/reference/security/secure-use).

## Runner and Keychain assessment

The current design's dedicated per-run Keychain is appropriate: identity lookup, every signing operation, and notary profile submission use the explicit `MACOS_KEYCHAIN_PATH`; cleanup deletes that Keychain and its temporary directory without changing the user's default Keychain search list. The workflow's `always()` cleanup covers normal success and step failures. The self-hosted path should also keep the runner out of PR workflows, use the dedicated `gimbal-release` label and a clean runner account, and verify Xcode 27/ARM64 immediately before registration. The `GITHUB_ACTIONS` check now accurately claims only that setup runs in Actions; it is not treated as a self-hosted isolation boundary.

## Outcome

**material findings remain**
