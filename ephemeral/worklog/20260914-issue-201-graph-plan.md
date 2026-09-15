# Issue 201 graph implementation handoff

correction: The repository uses Go 1.27.1 with generic methods on concrete types. Session.Generate[T] exists and a local compiler probe accepts the proposed Runtime.Run[T] shape; do not substitute older Go limitations for the checked toolchain contract.

decision: Issue 201's owner comment at 2026-09-14T23:10:57Z supersedes both the JSON-manifest delivery in its body and dynamic-key permission in pinned research. The handoff uses generated Go graphs and preserves constant-key linting while making the builtin's fixed checks explicit.

doc_bug: The owner's proof list attributes parallel children and nested supervision to the builtin sprint, but the inspected sprint has neither. The plan explicitly assigns those claims to fixtures and the separate caller example instead of inventing extraction output or changing the sprint's behavior.

friction: Research linked from issue 201 is on the graph-research branch and absent from this worktree. Cache the issue including comments and the three relevant files from pinned commit ecfa2d8 beside the handoff so implementation does not lose the superseding comment or require remote prompt sources.

decision: The proposed graph keeps scope containment, session ownership, control endpoints, and supervision separately typed. Static helper expansion IDs retain lexical source identity and call context; repeated runtime tasks never duplicate source templates.

correction: Tyler explicitly intends a statically mappable workflow aesthetic: as a rule of thumb, lint against anything that obscures possible source structure, including dynamic task dispatch, just as with dynamic Set keys. Unknown iteration counts and []Task contents are primitive runtime exceptions. Prefer explicit workflow source over more elaborate analysis; treating arbitrary dynamic dispatch as a capability to accommodate misstates the intended product. Recorded in AGENTS.md and the implementation handoff.

decision: Updated evaluation of tradeoff (1): endorse static structural recovery as an intentional workflow authoring rule; recommend no relaxation. The earlier concern over accommodating arbitrary idiomatic Go dispatch was misplaced for Gimble's explicit aesthetic.

correction: Tradeoff (2) assumed runtime discovery and led to redundant graph registration and workflow binding. Tyler instead wants an explicit build-time list of workflow types/Inputs, concrete generated remote bindings, and an authored Svelte page and link for each web-startable workflow. Replace WithWorkflow/registration in the handoff; preserve typed direct Run.

decision: Local pinned polytype v1.0.0 codegen API already selects JSON Schema, Go JSON, TypeScript, and devalue outputs. skgo v0.4.1 already generates concrete remote decoders and TS callers via polytype. Generate the per-workflow declarations and reuse that pipeline; do not invent a second codec or remote protocol. Emitted remote declarations must use skgo's .remote.go naming.

decision: Tyler selected a sealed Operation interface for point (3). Replaced the handoff's OperationKind plus nullable payloads with 17 concrete value variants, each declaring operation() directly. Shared Site metadata is embedded without the sealing method. Use polytype.SealedUnion[Operation]("kind", polytype.Snake), matching the lifecycle union convention. The full proposed Graph passed JSON/devalue round trips, invalid-variant rejection, and generation idempotence with pinned polytype v1.0.0; TypeScript output was inspected, not compiled.

correction: Tyler selected a nested workflow description, not the proposed flat CFG. Ordered bodies contain AgentCall, Command, Set, SetJSON, Subgraph, and relevant Exit; subgraphs are Sequence, Condition, Loop, Scope, and Group. Preserve relevant structure and context writes without modeling all Go. Removed the obsolete flat production declaration from the handoff and recorded the accepted semantics in ephemeral/issue-201/graph-model.md and AGENTS.md.

decision: Supervision is an unordered hierarchy beside each watched call. Higher supervisors watch the immediate supervisor's looks. Ancestor session ownership controls conversation lifetime; attachment/execution/visual placement follows the watched child call. Repeated appearances retain one session and accounting identity; task-local creation instead gives fresh conversations. Shared sessions still admit one active turn at a time.

constraint: The natural model recurses through both subgraph bodies and supervisor children, which pinned polytype v1.0.0 rejects. Recursive support versus reference encoding, and the concrete Group shape preserving intervening caller work, remain explicit open type decisions. The earlier 17-variant codec proof is historical, not verification of this new model. No extractor, runtime, UI, or polytype implementation was changed in this codification.

friction: During the documentation update, shared Git config reported core.bare=true and ordinary worktree commands failed. Explicit --git-dir and --work-tree paths restored access without changing shared repository configuration. Document consistency checks and git diff --check passed; no implementation tests were appropriate for this docs-only update.

correction: Tyler rejected further program modeling in point (4). Scoped groups of agent calls in the right order suffice; removed the proposed Group launch/join and os/exec Start/Wait reconstruction requirements. Command result detail is secondary. Record one approved opinionated Gimble command function and lint against direct os/exec in workflow code; its name/signature/result contract remain to be designed. Updated AGENTS.md to remove the contradictory instruction to author workflow git calls through os/exec. This is a design update, not an API/linter implementation.
