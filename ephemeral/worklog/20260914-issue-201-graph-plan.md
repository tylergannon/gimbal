# Issue 201 graph implementation handoff

correction: The repository uses Go 1.27.1 with generic methods on concrete types. Session.Generate[T] exists and a local compiler probe accepts the proposed Runtime.Run[T] shape; do not substitute older Go limitations for the checked toolchain contract.

decision: Issue 201's owner comment at 2026-09-14T23:10:57Z supersedes both the JSON-manifest delivery in its body and dynamic-key permission in pinned research. The handoff uses generated Go graphs and preserves constant-key linting while making the builtin's fixed checks explicit.

doc_bug: The owner's proof list attributes parallel children and nested supervision to the builtin sprint, but the inspected sprint has neither. The plan explicitly assigns those claims to fixtures and the separate caller example instead of inventing extraction output or changing the sprint's behavior.

friction: Research linked from issue 201 is on the graph-research branch and absent from this worktree. Cache the issue including comments and the three relevant files from pinned commit ecfa2d8 beside the handoff so implementation does not lose the superseding comment or require remote prompt sources.

decision: The proposed graph keeps scope containment, session ownership, control endpoints, and supervision separately typed. Static helper expansion IDs retain lexical source identity and call context; repeated runtime tasks never duplicate source templates.
