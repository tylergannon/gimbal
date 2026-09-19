# Context, scope, artifacts, and evidence

## Scope

- Audience: Author workflows. Authority: current code/instruction.
  `Run` supplies a scope through `context.Context`; `Set` and `SetJSON` write
  once per key in that scope. Use child scopes for revisions/shadowing, not
  mutable shared state. [scope.go:175-200]
- Audience: Author workflows. Authority: current code. A scope is a named
  lifetime: its body starts after the child context is installed and ends when
  the body returns; sessions created in it are closed before its context is
  cancelled. [scope.go:124-173]
- Audience: Author workflows. Authority: current code. Visible context walks
  ancestors, keeps the nearest definition per key, and renders outermost to
  innermost. A child can shadow a key without changing the parent's captured
  value. [scope.go:302-319] [scope.go:388-403]
- Audience: Author workflows. Authority: current test. Returning or failing
  from a child restores the parent prompt; a sibling receives the restored
  parent values and no child-only values. This is proven for default and
  `WithScopeTemplate` rendering. [scope_isolation_test.go:12-89]
- Audience: Author workflows. Authority: current code. `Set` is a programming
  error when there is no run scope, the scope ended, or the key already exists
  in that scope; it panics and names the key/scope. [scope.go:188-235]
- Audience: Author workflows. Authority: current code. Values are immutable
  snapshots (`scopeValue.raw` or an artifact descriptor); representation can
  change from inline bytes to a file, but the semantic value does not mutate.
  [scope.go:50-57] [scope.go:227-235] [scope.go:262-275]

## Prompt and structured output

- Audience: Author workflows. Authority: current code. `Generate` sends the
  caller prompt followed by `"\n\n"` and the current scope render. The same
  full prompt is recorded, supervised, retried, and passed to the adapter.
  Do not manually append scope text. [session.go:75-93]
- Audience: Author workflows. Authority: current code. `WithScopeTemplate`
  receives decoded `ScopeData` (`Values` plus key-addressable `By`) and its
  final rendered text is budgeted before dispatch. Use it to select fields,
  not to bypass the context budget. [scope.go:277-319] [session.go:95-120]
- Audience: Author workflows. Authority: current code. Typed outputs send a
  schema, validate the raw result, decode JSON, and re-ask up to three total
  attempts when validation/decoding fails. `Text` sends no schema and returns
  the final message. [session.go:62-83] [session.go:163-195]
- Audience: Author workflows. Authority: current code. Runtime-built planner
  and supervisor prompts call internal dispatch directly, avoiding duplicate
  scope injection; those dynamic prompts still need explicit budget treatment.
  [session.go:150-160]

## Context budget and local files

- Audience: Author workflows and build/release maintainers. Authority: current
  code/design. Automatic scope context is limited to 15,000 `o200k_base`
  tokens, with a provisional 4,000-token per-entry ceiling; headings,
  previews, references, and template output count. [artifact.go:20-47]
  [ephemeral/scoped-context-plan.md:73-117]
- Audience: Author workflows. Authority: current code. Small values render
  inline. Large values spill to run-owned artifacts; aggregate overflow keeps
  bounded head/tail excerpts, omission markers, and absolute paths. If the
  index itself is too large, a bounded index artifact points to every complete
  value. [scope.go:321-364] [artifact.go:172-240]
- Audience: Author workflows. Authority: current test. The budget is checked
  with Unicode validity, aggregate overflow, many-reference index fallback,
  custom templates, and previous planner-task context; complete sentinels are
  recoverable through the emitted file path. [context_artifact_test.go:122-260]
- Audience: Author workflows. Authority: current code. Structured artifacts
  are complete valid JSON; scalar artifacts are complete text. Reads decode
  the artifact back to the original semantic value, so a workflow can safely
  use `scopeData` after spilling. [artifact.go:243-274]
- Audience: Build/release. Authority: current code. Artifact paths are encoded
  and checked for run containment; writes use temp files, sync, atomic hard
  link, and cleanup, and immutable files refuse overwrite. [artifact.go:49-128]
- Audience: Build/release. Authority: current design/instruction. Runs are
  storage-local: structured records use run-relative paths, while prompts and
  scalar returns use absolute paths. Do not promise relocation or resume after
  moving a run. [ephemeral/scoped-context-plan.md:16-17]
- Audience: Build/release. Authority: current test/design. A spilled value
  remains readable after scope closure; live observation, finished tables, and
  missing-table log rebuild must converge on the descriptor. Failed `Set`/
  `SetJSON` artifact writes panic rather than publishing a broken pointer or
  retaining unlimited bytes. [context_artifact_test.go:16-120]

## Command evidence

- Audience: Author workflows. Authority: current code. `RunCommand` names each
  invocation by scope key plus ordinal, writes stdout and stderr to separate
  run-owned files while the process runs, and preserves partial output on
  nonzero exit, cancellation, or capture failure. [command.go:84-141]
- Audience: Author workflows. Authority: current code. Streams at or below
  64 KiB return exactly; larger streams return bounded head/tail text with an
  omission marker and an absolute `Complete output` path. The command's exit
  code is data; `err` means start, cancellation, or capture failure.
  [command.go:17-19] [command.go:84-99] [command.go:52-82]
- Audience: Build/release. Authority: current design/instruction. Evidence
  should be inspected from the run-owned files, and capture errors must be
  recorded as failures. Do not infer a resident-memory bound merely because an
  artifact exists; tests must assert bounded returned previews and file
  fidelity. [ephemeral/scoped-context-plan.md:125-147]

## Established methodology and gotchas

- Audience: Build/release. Authority: current design/instruction. Go tests are
  the primary evidence across real Run, prompt, filesystem, subprocess,
  observation, and replay boundaries; report one real workflow's behavior and
  commit no proof program, screenshot, run log, or other proof artifact.
  [ephemeral/scoped-context-plan.md:225-236]
- Audience: Build/release. Authority: current design. The focused proof matrix
  crosses prompt assembly, filesystem storage, subprocess capture, observation,
  and replay; keep it behavioral and bounded rather than adding a public
  artifact API, summarizer, eviction framework, or workflow wrapper.
  [ephemeral/scoped-context-plan.md:165-189] [ephemeral/scoped-context-plan.md:225-239]
- Audience: Author workflows. Authority: current code. Once a child value has
  been injected into a native session turn, closing the scope cannot erase the
  provider conversation history; the isolation guarantee applies to later
  injected context. [ephemeral/scoped-context-plan.md:119-123]
- Audience: Author workflows. Authority: current code. A session belongs to
  its creating scope and cannot generate after that scope ends; `Fork` keeps
  the parent's conversation and binding, then becomes independent.
  [session.go:15-17] [session.go:574-600]
- Audience: Build/release. Authority: current code. Canonical event IDs and
  normalized native references are validated, and unsupported native-ref keys
  are rejected. Provider event data must carry a native session ID and a
  matching provider reference. [session.go:409-472]

## Retrieval hints

Search `scope`, `shadow`, `visibleValues`, `SetJSON`, `WithScopeTemplate`,
`contextTokenLimit`, `artifactAbsolute`, `Complete value`, `RunCommand`,
`Complete output`, `ValueSet`, `replay`, and `scope isolation` together.
For authoring questions retrieve the Scope and Prompt sections first. For
release or proof questions retrieve Local files, Command evidence, and
Established methodology together.

## Coverage, conflicts, and gaps

- Covered: prompt assembly, typed output validation/retry, scope lifetime and
  shadowing, immutable values, token budgeting, artifact containment/lifetime,
  command capture and bounded returns, event/reference evidence, and tests.
- `docs/context-budget.md` is authoritative only for the separate Claude Code
  host-session cost model (measured ~53.5k baseline; tool schemas dominate),
  not Gimble's 15,000-token scope budget. [docs/context-budget.md:1-8]
  [docs/context-budget.md:43-59]
- The proposal's “What exists” and command-buffering description are historical
  context; current `command.go` already streams to files and bounds large
  returns. [ephemeral/scoped-context-plan.md:19-34] The worklog records the
  settled command-return and token-counter decisions; treat it as historical
  decision evidence, not API authority. [ephemeral/worklog/scoped-context-plan.md:3-14]
- Gap: these sources do not document adapter-specific provider context windows,
  full planner implementation, or browser/UI rendering of artifact descriptors.
