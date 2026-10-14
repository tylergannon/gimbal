# Persistent Mac release cache — 2026-10-14

correction: User approved removing remote mise cache transport and requires useful caches on the persistent Mac's local filesystem. Preserve installed tools and local caches; do not upload/download the runner's mise directory during release.

friction: v0.13.12 release run 38553157900 spent approximately 50s compressing/uploading 916,420,368 bytes after mise reported all tools already installed. Its mise action took 52s total. This is historical evidence for the change, not a measured new-release saving.

decision: Set cache=false for the existing mise action on the self-hosted Mac job. Keep tool installation/version verification, signing, notarization, platform jobs, and publication gates unchanged. Broader build deduplication and overlapping qualification remain recommendations, outside this patch.

decision: Pinned mise-action v5.1.2 declares cache=false disables remote cache reads/writes. Focused release preparation/dispatch tests passed (10 cases); actionlint v1.7.7 accepted the changed workflow; whitespace check passed. The next actual release must establish whether cache transport disappears and quantify elapsed savings; no new full application build was needed for this configuration change.
