# Design handoff findings

friction: The new worktree started detached at PR #246's original head. A normal rebase attempted to replay its 13 already-squashed commits. Aborted, verified the entire commit list against merged PR #246, and rebased only the empty post-PR range onto origin/main (0908812), preserving the old commits in existing history.

decision: Tyler's four requested design capabilities include an interactive generated-workflow map now, not as a later optional enhancement. Prepared a bounded design brief without changing production UI or prescribing a graph library.

doc_bug: docs/web-app.md calls several features existing or planned using outdated status. Current home is informational; run page has no graph in its load data; interviews and steering are implemented. Designers must distinguish confirmed data surfaces from aspirations.

decision: The compiled graph is not stored with historical runs. The handoff explicitly distinguishes workflow definition from execution instances and calls out missing/mismatched graph handling without inventing a persistence or mapping scheme.

correction: A prompt with repository pointers was not a sufficient design package. Tyler requested concrete graph/data examples and varied, visually evidenced research before handing off to Claude Fable. Added explicitly fictional runtime specimens alongside a source-derived graph; assigned three Luna research lanes.

decision: Tyler explicitly included both stopping a running turn and cancelling a run in the first design. Their missing web controls are an engineering gap, not an exclusion from design scope.

friction: Two research lanes could discover public image links but could not render them. The parent inspected Studio, Langfuse, Chrome, Blender, and Sentry references in Chrome and corrected unsupported visual claims (including Sentry tabs absent from the screenshot). The execution lane directly inspected its three images. Captions alone must not be labeled visual inspection.

decision: Curated eight unique products into three interaction directions, deduplicating Temporal across lanes. Reference screenshots are public embeds with source attribution, not locally committed assets. Final layout and deliverable remain a user/design choice; all concepts retain the required map and active controls.
