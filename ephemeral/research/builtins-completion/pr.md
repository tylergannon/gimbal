Sprint Plan now discovers a project's local semantic index, retrieves cited prior art, and shares factual orientation before three independent drafts and cross-critiques. It presents the differences, interviews for missing decisions, preserves answers and fixed validation commands, and writes actual synthesis decisions with the final plan.

Adds `gimble index -from /cache -to /index -repo /project` to build a filesystem index with bounded topic routes and cited leaves. Automatic updates reread changed sources, remove deleted leaves and preserve unchanged leaves; audit checks freshness without modifying the index. Successful builds register the index for planning. Sources remain read-only, invalid paths/citations fail, and failed updates preserve the prior index. The initial reader supports UTF-8 files up to 256 KiB each and records unsupported sources as debt.

A live planning run also exposed agy's explicit tool `ERROR` state being ignored. Recognize that terminal error while retaining rejection of genuinely unfinished tools.

Validation: full Go tests, vet, golangci-lint, Gimble lint and production web build passed. Native Luna/Haiku/Gemini Flash evidence covers index build, source preservation, blind cited retrieval, paged retrieval, one-source incremental update plus deletion, stale/clean audit, recovery from a failed Gemini file lookup, and complete Sprint Plan orientation/draft/critique/interview/synthesis with exact answer and final-decision artifacts. Planning evidence and independent review are recorded in `ephemeral/attest/planning-index/README.md` and `ephemeral/reviews/planning-index-final-review.md`.

Closes #221.
