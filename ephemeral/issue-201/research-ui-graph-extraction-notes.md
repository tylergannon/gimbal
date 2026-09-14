# Graph extraction — UI-informed issue addition

Prepared 2026-09-14 for the graph-extraction issue. The live issue inventory did not identify a dedicated extraction issue; #162 is the bounded linter and #173 is the runtime usage view. The user has been asked for the intended target. This is a ready-to-apply addition, not a claim that an issue was updated.

## Outcome

The generated graph should support a scope-aware workflow map, a run overlaid on that map, and navigation to the same observed instance in a timeline and inspector. It describes semantics, not layout coordinates. A useful partial manifest is acceptable; the viewer must work from observed runtime structure when a manifest is absent.

See [UI design brief](ui-design-brief.md) and [earlier extraction recommendation](recommendation.md). This addition refines what the UI needs; it does not expand Beta linting into whole-program analysis.

## Semantic information to retain

| Information | Why the viewer needs it |
| --- | --- |
| Region identity, parent, and kind: scope, group, loop | Collapse/focus the real scope structure and distinguish concurrency from repetition |
| Operation sites: agent calls, command execution sites, planning calls, supervisor attachments/look sites where identifiable | Show the whole workflow rather than only agent sessions |
| Supported control relations: sequence, branches/joins, parallel launch/join, repetition/exit | Explain possible order without inventing dependencies from source placement |
| Display/source order separate from control edges | Stable readable layout, including siblings that can execute concurrently |
| Region entry/exit and cross-boundary endpoints, derivable from semantic edges where possible | Collapsing a scope must preserve external connections |
| Session ownership and operation execution scope as separate relations | Ancestor-owned sessions may be used in child scopes; the inspector must not misattribute ownership |
| Supervisor session role and attachment target | A supervisor watches a turn; nested supervision watches a look turn |
| Typed fork/call provenance where resolved | Optional inspection without adding every relation to the default map |
| Source anchor, containing function, static site identity, and source/build version | Source navigation, repeat-template identity, and compatibility checks with a run |
| Known constants, dynamic expressions, and unresolved coverage with reason | Useful partial visualization without outlawing dynamic Set keys or silently dropping code |

This is a vocabulary requirement, not a demand to resolve every row everywhere in the initial extractor. Mark unsupported sites/regions and keep the surrounding graph. A normal JSON manifest is sufficient; a Go literal can be another encoding of the same semantic information if needed. Do not require two encoders for the first delivery.

## Program sites and run instances

A source operation may execute zero, once, or many times. A loop template therefore exists once in the manifest; its observed instances belong to the run. Keep static site IDs separate from scope/session/turn/event instance IDs. Preserve the manifest version used by a run, rather than attaching old events to whichever source is currently checked out.

Exact correlation is a separate runtime integration concern. Existing keys may support a qualified match in simple cases. Repeated calls, helper reuse, or ambiguous dynamic keys must remain aggregate/unmatched until instrumentation gives a stronger identity. Do not infer a unique callsite from a label shared by multiple sites; do not introduce reflection or runtime.Caller.

Static extraction cannot supply actual loop counts, task names selected by an agent, branch outcomes, exact invocation targets in arbitrary Go, process exit results, or supervisor steering delivery. The runtime record supplies observations. The manifest supplies supported possible structure. A static path not observed is not automatically “skipped”: it may be future work, unreachable this time, or unrecorded.

## Commands

Represent recognizable command operations in the static shape now. Distinguish constructing an `exec.Cmd` from starting/waiting for execution, and do not claim the exact command string where arguments are dynamic. Support source anchors and expression labels for partial cases.

Visualization of a command's actual start/end/output/exit requires an observation boundary. Existing named scopes and recorded result values can supply a first useful view where workflows use them, but scope duration is not automatically process duration. Do not make a new command wrapper or tracing API a prerequisite of graph extraction. Unobserved static command sites still belong in Program.

## Supervision

Retain attachment target semantics independently from control flow and containment. An attachment can exist without a look occurring. A look can occur without steering; attempted steering can land or be dropped. A higher supervisor can watch a lower supervisor's look. Source extraction should represent the known attachment structure; runtime events supply actual looks and delivery outcomes.

The UI will show nearby supervision by default and reveal larger cross-scope relations on selection. It must not have to reverse-engineer these relationships from names or treat all edges as ordinary dependencies.

## Concrete acceptance examples

- A workflow with sequential phases, two parallel child scopes, a join, and a repeat/exit can be rendered with meaningful scope boundaries and possible order. Display order does not falsely serialize parallel siblings.
- A command followed by an agent validator appears as two different operation kinds. A constructed-but-never-run command is not portrayed as an observed process.
- Two same-named operations in different scopes remain distinguishable. One site executed repeatedly yields one template and multiple run instances.
- A session owned by an ancestor and used in a child scope retains both relationships.
- A worker watched by a supervisor whose look is itself supervised preserves both attachment targets.
- A dynamic key or unresolved helper leaves a labeled partial region/site instead of failing extraction or disappearing without explanation.
- Collapsing a region preserves external connection endpoints and enough identity to restore its contents. Coordinates, colors, viewport, and selection are viewer concerns.

Use small source fixtures with expected semantic assertions for extraction, and independently inspect one representative builtin workflow's generated manifest. Avoid snapshots that merely freeze incidental node coordinates. Runtime overlay tests must use actual observation records for correlation and outcomes.

## Keep the early delivery bounded

Start with typed AST/control-flow information and recognizable workflow constructs; only add deeper analysis when an existing workflow needs it. Caller/callee relations are optional navigation, not required default edges. Do not require exhaustive call graphs, alias/lifetime proofs, arbitrary Go evaluation, data lineage, or critical-path analysis. Do not make constant Set keys mandatory.

The bounded linter remains in Beta (#162). The UI can first ship an observed-run map with linked details/timing; richer Program views arrive with extraction coverage. The necessary early commitment is to keep graph semantics and observed identities distinguishable so these increments fit together.
