# Automatic release tag authorization

friction: The first checked-main delivery reached tag preparation but immutable version-tag creation rules rejected GITHUB_TOKEN. A user creation-only bypass did not authorize that integration's push. Use a repository-scoped SSH deploy key whose identity can be granted creation-only bypass; preserve owner-only tag update/delete rules.

decision: Put the key only in the main-only release-preparation environment/job. Checks retain the read-only token with no signing/preparation environment. SSH pushes create events, so skip the already-checked bot-authored Release-Source metadata and make the existing release workflow dispatch-only to avoid duplicate builds.

decision: Deliberate minor/major tags now require an explicit Release dispatch at the same tag. Existing tagged signing/environment and publication steps remain intact.
