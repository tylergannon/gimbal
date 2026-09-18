# Design handoff findings

friction: The new worktree started detached at PR #246's original head. A normal rebase attempted to replay its 13 already-squashed commits. Aborted, verified the entire commit list against merged PR #246, and rebased only the empty post-PR range onto origin/main (0908812), preserving the old commits in existing history.

decision: Tyler's four requested design capabilities include an interactive generated-workflow map now, not as a later optional enhancement. Prepared a bounded design brief without changing production UI or prescribing a graph library.

doc_bug: docs/web-app.md calls several features existing or planned using outdated status. Current home is informational; run page has no graph in its load data; interviews and steering are implemented. Designers must distinguish confirmed data surfaces from aspirations.

decision: The compiled graph is not stored with historical runs. The handoff explicitly distinguishes workflow definition from execution instances and calls out missing/mismatched graph handling without inventing a persistence or mapping scheme.
