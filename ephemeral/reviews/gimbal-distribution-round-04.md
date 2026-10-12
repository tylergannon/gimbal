# Focused adversarial review: Gimbal signing runner round 04

**Review target:** current macOS signing/notarization workflow and scripts in `/Users/tyler/.codex/worktrees/gimbal-distribution/gimbal`, with maintained `skills/gimbal-release/references/distribution.md`.

**Evidence inspected:** complete current `scripts/ci-signing.sh`, `scripts/release-macos.sh`, `scripts/sign-notarize-macos-cli.sh`, `.github/workflows/release.yml`, and distribution instructions; relevant release plan and repository instructions. Parent reports local signing and notarization succeeded with the dedicated Keychain, and `just vet` and `just test` exited 0. The workflow still selects GitHub-hosted `xcode-27`; no runner switch was requested or made. Current GitHub runner documentation lists `xcode-27` as a public-preview ARM64 macOS runner. GitHub's security guidance says self-hosted runners are not guaranteed to be clean/ephemeral and can be persistently compromised; its runner reference recommends ephemeral self-hosted runners rather than persistent ones. I made no source changes and did not rerun proof.

## Findings

### 1. Issue — the signing script's hosted-runner guard also accepts self-hosted runners

`scripts/ci-signing.sh:31` checks only `GITHUB_ACTIONS=true` before accepting Apple secrets, while its error text and closing status message claim setup is restricted to ephemeral GitHub-hosted runners. `GITHUB_ACTIONS` is true on both GitHub-hosted and self-hosted Actions runners. If the planned fallback changes `.github/workflows/release.yml:70` to a self-hosted label, the guard will still pass on a persistent runner; the dedicated Keychain and `always()` cleanup do not make that machine ephemeral or reset unrelated host state. This conflicts with the distribution instructions' recommendation to use self-hosting only after verifying access controls and with GitHub's self-hosted runner security guidance. Keep `xcode-27` as the default. If self-hosting becomes necessary, use a one-job ephemeral runner/clean disposable host and make the script check the actual runner environment (for example `RUNNER_ENVIRONMENT`) rather than `GITHUB_ACTIONS` alone. GitHub guidance: [secure use of GitHub Actions](https://docs.github.com/en/actions/reference/security/secure-use) and [self-hosted runner reference](https://docs.github.com/en/actions/reference/runners/self-hosted-runners).

### 2. Issue — release secret access still depends on an unverified GitHub environment setting

The workflow attaches signing secrets to `environment: release` (`.github/workflows/release.yml:71,96-105`) and exposes an unrestricted manual dispatch input for a tag (`.github/workflows/release.yml:6-11`). The YAML does not restrict that environment to trusted version tags or require an approval; the maintained instructions explicitly require restricting it to trusted version tags (`skills/gimbal-release/references/distribution.md:82-83`). GitHub environment branch/tag rules and required reviewers are external repository settings. Until those settings are verified, a workflow run from an untrusted ref can reach the same release environment and its signing/notarization secrets. Before adding secrets or activating publication, restrict the environment to protected version tags and use required reviewer approval if the release policy calls for it. GitHub documents these protections under [deployment environments](https://docs.github.com/en/actions/concepts/workflows-and-actions/deployment-environments).

## Runner and Keychain assessment

A dedicated Keychain is appropriate for both local qualification and unattended hosted signing because it makes identity lookup, `codesign`, and `notarytool` use one explicit credential store without mutating the user's default search list. It is particularly important on a self-hosted host, but is not sufficient isolation for a persistent runner: GitHub warns that a later or concurrent job may inspect process arguments and retained machine state. `security import` currently passes the PKCS#12 password through `-P` (`scripts/ci-signing.sh:59`), which is acceptable on the isolated hosted VM path but increases exposure on a shared persistent host.

No self-hosted switch is justified now. `xcode-27` matches the app's macOS 27 ARM64 target and avoids storing signing material on the user's long-lived machine. Reconsider self-hosting only for a demonstrated hosted-runner limitation; then qualify its architecture/Xcode first and use an ephemeral one-job runner with full machine disposal, restricted runner groups/ref access, and the same temporary Keychain cleanup.

## Outcome

**material findings remain**
