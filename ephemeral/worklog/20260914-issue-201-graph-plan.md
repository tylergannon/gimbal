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
