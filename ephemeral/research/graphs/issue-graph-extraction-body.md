Workflow authors and the run viewer need a machine-readable description of a workflow's shape: scopes, operations, possible control flow, and supervision. The runtime tree alone describes what was observed; it cannot show unexecuted alternatives or the complete program structure.

Generate an inspectable JSON graph from ordinary Go workflow source through `go generate`. Start with the builtin sprint and one small workflow in a caller module. A useful partial graph is acceptable: preserve dynamic expressions and mark unresolved sites rather than restricting valid workflows or silently omitting them.

## First delivery

- Preserve scope regions and their kinds, including groups and loops. A repeated source template appears once; actual iterations belong to a run.
- Identify agent-call and command-execution sites, plus supervisor attachments where statically recognizable. Distinguish constructing an `exec.Cmd` from executing it.
- Retain supported sequence, branch, parallel launch/join, and repeat/exit relations. Keep display/source order separate from execution dependencies.
- Keep session ownership separate from a turn's execution scope: an ancestor-owned session may be used in a child scope.
- Represent supervision as its own relation to the watched turn, including a supervisor whose look turn is itself supervised. Attachment does not imply a look actually occurred.
- Include source anchors, static site identities, and source/build version information. Keep these separate from runtime scope/session/turn instance IDs.
- Follow the statically resolved local helpers needed by the builtin example. Preserve unresolved targets, dynamic keys, and dynamic arguments with an expression/source label and a reason for incomplete coverage.

The graph describes semantics, not coordinates, colors, or viewport state. It must retain enough containment and cross-boundary endpoint information for the viewer to collapse a scope and restore it without losing its external connections. JSON is enough for the first delivery; a second Go-literal encoder is not required.

## Why the UI needs these distinctions

The intended viewer has a scope-aware map, a linked run timeline, and an inspector that preserves selection. It must support commands, parallel work, repeated tasks, and nested supervision alongside agent calls.

For example, a user can inspect task 3's failed command while task 4 is running, without the whole run appearing terminally failed. A source template and a runtime instance therefore cannot be the same identity. A static site without an observed event is not automatically skipped: it could be future work, an untaken branch, or unrecorded work.

Runtime events supply actual intervals, outcomes, loop counts, supervisor looks, and steering delivery. Static command discovery does not supply process telemetry. Existing named scopes with recorded command results can support a first useful observation, but scope duration is not automatically process duration. Exact source-to-run correlation remains a separate integration step; ambiguous matches must stay ambiguous.

## Acceptance

- `go generate` produces inspectable output for the builtin sprint and a small caller-module workflow. Document how callers select a workflow for extraction and invoke generation.
- Source fixtures demonstrate sequential phases, parallel children with a join, and repeat/exit structure without falsely serializing siblings.
- A command followed by an agent validator produces distinct operation kinds. A command only constructed in source is not claimed as an observed process.
- Same-named sites in different scopes remain distinguishable, and repeated execution does not duplicate the source template.
- Fixtures preserve ancestor session ownership versus child execution scope, and both targets in nested supervision.
- Dynamic keys and an unsupported indirect call yield useful partial output with explicit unresolved coverage.
- Inspect the builtin manifest against its source. Tests assert semantic relationships rather than incidental layout or node coordinates.

## Boundaries and related work

This is separate from **#162**, the bounded Set/SetJSON linter, which remains in Beta. It is also separate from **#173**, the observed scope/time/token/cost view. Neither requires complete graph extraction to ship. The viewer can begin with observed runtime structure and gain a Program view when a compatible manifest exists.

Do not ban dynamic Set keys, add high-level workflow wrappers, use reflection/runtime.Caller, or attempt exhaustive ownership proofs, arbitrary Go evaluation, whole-program pointer analysis, general recursive helper expansion, or critical-path analysis. Caller/callee links are optional inspection aids, not required default edges. This issue does not include building the UI or exact runtime instrumentation.

## Research and design input

The research is cached as flat files in `ephemeral/research/graphs`, including a Go feasibility probe, independent source-rule reviews, downloaded UI references, and a pinned Tenacious archive. The following links are pinned to the reviewed research commit:

- [Extraction feasibility and recommended graph model](https://github.com/tylergannon/gimble/blob/ecfa2d8/ephemeral/research/graphs/recommendation.md)
- [UI-informed semantic requirements](https://github.com/tylergannon/gimble/blob/ecfa2d8/ephemeral/research/graphs/ui-graph-extraction-notes.md)
- [Design brief and drill-down states](https://github.com/tylergannon/gimble/blob/ecfa2d8/ephemeral/research/graphs/ui-design-brief.md)
- [Research index](https://github.com/tylergannon/gimble/blob/ecfa2d8/ephemeral/research/graphs/INDEX.md)
