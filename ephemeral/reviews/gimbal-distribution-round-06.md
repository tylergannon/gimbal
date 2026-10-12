# Adversarial review: Gimbal self-hosted release round 06

**Review target:** current Gimbal distribution release workflow, with emphasis on the self-hosted signing runner and external GitHub tag/environment protections.

**Evidence inspected:** current release workflow, signing and notarization scripts, maintained distribution instructions, and read-only GitHub API state for repository collaborators, release tag rulesets, and the `release` environment. The active `Gimbal release tags` ruleset applies to `refs/tags/v*.*.*`, protects creation, update, and deletion, and grants bypass only to user `tylergannon` (ID 50593). The collaborator inventory contains only `tylergannon`, with admin/write access. The `release` environment permits only `v*.*.*` tag refs. The runner inventory is currently empty; runner registration remains a subsequent setup action. Parent reports local dedicated-Keychain signing/notarization and successful `just vet`/`just test`; I did not rerun those checks.

## Findings

No material findings remain. The round-05 concern that arbitrary matching tags could run on the self-hosted Mac is resolved by the active tag ruleset: only the owner, who is also the sole collaborator and explicitly authorized release authority, can create/update/delete those tags. The environment's tag-name policy and the workflow's same-ref dispatch condition provide consistent additional checks. Requiring this same owner to approve each release again would add a redundant gate without strengthening the stated trust boundary.

The dedicated per-job Keychain remains appropriate for the self-hosted runner: the scripts direct identity lookup, signing and notarization to that Keychain, delete it and its temporary directory in the workflow cleanup step, and leave the user's default Keychain search list untouched. The release workflow does not route pull-request jobs to the `gimbal-release` runner.

## Outcome

**no findings**
