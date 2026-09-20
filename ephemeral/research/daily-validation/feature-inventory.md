# Gimble features to validate

Product snapshot: `3db61eb99397fa6e01898b0746529b5fefbd3e14`, checked against fetched `main` and the installed binary on 2026-09-19. This is an inventory of implemented behavior and proposed checks, not a report that those checks passed.

The [full feature research](features.md) supplies implementation references, provider prerequisites, and detailed checks. The tables below arrange that research around what a user can do and the primary mechanism for checking it. Six additional browser entries make visible interactions explicit rather than leaving them buried in implementation notes. IDs identify feature families; a daily suite will need multiple scenarios for families with several outcomes.

## Choose the testing mechanism

This inventory is a capability map for code review and automated test coverage,
not the assignment list for an agent focus group. Each row names the primary
mechanism for its specific claim; one capability can also appear incidentally in
a practical user workload.

- **Unit/integration:** precise contracts, data, error handling, ownership, and calculations.
- **E2E:** repeatable CLI/browser interactions and state transitions.
- **Live integration:** actual provider sessions and external process behavior.
- **Build checks:** compilation, generation, and generated-contract consistency.
- **User workload:** a useful job completed through Gimble, with task outcome and UX observations.

A separate code-review run can use all these entries to inspect implementation
and look for defects. Source inspection does not prove usability or replace
executing the relevant checks. The [user-testing direction](user-testing.md)
describes the practical, source-blind testing workflow; it should not mechanically
turn every row below into an agent task.

## CLI and built-in workflows

| ID | Feature | Primary mechanism | What daily use should demonstrate |
|---|---|---|---|
| CLI-001 | Serve a project | E2E | Start the built binary, open its page, use a selected/free port; verify headless and Unix-socket modes and shutdown. |
| CLI-002 | Run the workflow linter | Unit/integration | Accept valid workflow code and diagnose an intentionally invalid fixture. |
| CLI-003 | Discover active runs | E2E | Find the target's live runs; ignore unreachable owners; return an empty list when appropriate. |
| CLI-004 | Watch a run | E2E | Receive its initial snapshot and subsequent changes; report unavailable owners clearly. |
| CLI-005 | Steer a session or loop | E2E | Deliver an active-turn instruction, distinguish a dropped steer, and queue guidance for the next planner decision. |
| CLI-006 | Count tokens | Unit/integration | Return the expected count for a fixed text fixture; reject an unreadable file. |
| CLI-007 | Run one prompt | Live integration | Produce plain text or schema-valid output on a selected provider; respect timeout and invalid-input errors. |
| CLI-008 | Review code | User workload | Find a known defect in a disposable repository without modifying the target. |
| CLI-009 | Implement a bounded goal | User workload | Make a small change against a local definition of done, gather evidence, and obtain independent validation. |
| CLI-010 | Research a document | User workload | Produce local sources, indexes, and a document within budget; apply editorial feedback. |
| CLI-011 | Produce a summary pyramid | User workload | Create progressively shorter summaries from an existing document and index; preserve the central facts. |

Source detail: [CLI research](features-sources/topic-001/INDEX.md), [prompt execution](features-sources/topic-002/INDEX.md), [built-ins](features-sources/topic-003/INDEX.md).

## Browser: observe and control real runs

| ID | Feature | Primary mechanism | What daily use should demonstrate |
|---|---|---|---|
| WEB-001 | Runs list, filters, search, attention | E2E | Find live and past runs; filter/search; locate a pending interview. |
| WEB-002 | Workflow map | E2E | Read scopes, branches, concurrent work, and declared steps that have not executed. |
| WEB-003 | Scope details | E2E | Select a scope and read its task, recorded values, and references to large values. |
| WEB-004 | Session details | E2E | Read the correct provider, model, role, and session attribution. |
| WEB-005 | Turn and supervisor details | E2E | Inspect the actual prompt, outcome, timing, usage, and supervisor activity. |
| WEB-006 | Streaming transcripts | E2E | Follow messages, reasoning, and tool calls/results while work runs, then read them afterwards. |
| WEB-007 | Steer a turn | E2E | Submit guidance and distinguish accepted delivery from a turn that ended too soon. |
| WEB-008 | Steer or wrap up a loop | E2E | Queue a message and observe the planner consume it; request an end to dispatch. |
| WEB-009 | Answer an interview | E2E | See the question, answer it, and observe progress; end the interview with an empty answer. |
| WEB-010 | Stop a turn or cancel a run | E2E | Observe interruption/cancellation and completed cleanup; dismiss the cancellation dialog without cancelling. |
| WEB-011 | Navigation, loading, and empty states | E2E | Navigate among Conversations, Runs, and About; follow direct links; retain usable cards on refresh. |
| WEB-012 | Find work within a run | E2E | Search the run and jump to current activity; reveal the corresponding map selection. |
| WEB-013 | Navigate a large map | E2E | Zoom, fold/unfold a scope, and select an instance of repeated work without confusing its details. |
| WEB-014 | Command and service inspection | E2E | Read command exit/output; inspect a service under its owning scope and its actual process records. |
| WEB-015 | Connection and unavailable-graph states | E2E | Show connection loss/recovery; explain missing or mismatched graphs rather than showing misleading work. |

Source detail: [browser research](features-sources/topic-004/INDEX.md), [controls](features-sources/topic-005/INDEX.md). Additional interaction references: [browser scenarios](../../../e2e/features/generated-app.feature), [layout](../../../web/src/routes/+layout.svelte), [run page](../../../web/src/routes/runs/[runID]/+page.svelte), [top bar](../../../web/src/lib/run/Topbar.svelte), [map](../../../web/src/lib/run/Map.svelte), [detail pane](../../../web/src/lib/run/DetailPane.svelte).

## Conversations and provider support

| ID | Feature | Primary mechanism | What daily use should demonstrate |
|---|---|---|---|
| CONV-001 | Conversation worktrees | Unit/integration | A new conversation gets its own Git branch/worktree and work happens there. |
| CONV-002 | Saved conversations | E2E | Messages, metadata, and linked runs remain readable after reload. |
| CONV-003 | Restart recovery | E2E | Saved state loads without silently restarting work; interrupted work is represented honestly. |
| CONV-004 | Launch workflows from chat | E2E | Start review/implementation through the agent, follow the run link, and receive its result. |
| CONV-005 | Conversation browser interaction | E2E | Select a provider/model, create and reopen a conversation, send messages, and see busy/error/history states. |
| HARN-001 | Claude integration | Live integration | Run a real cheap-model turn and inspect its text, tools, usage, and cancellation. |
| HARN-002 | Codex integration | Live integration | Run through the shared app-server and verify session lifecycle without disrupting other clients. |
| HARN-003 | Gemini/agy integration | Live integration | Run in print mode, validate structured output, and expose harness errors honestly. |
| HARN-004 | Model and effort selection | Live integration | Resolve supported aliases and explicit models; reject unsupported combinations. |
| HARN-005 | Codex conversation continuation | Live integration | Restart Gimble, send another message, and verify the same native conversation retains earlier context. |
| HARN-006 | Fork a session | Live integration | On supported providers, fork shared history into independent continuations; agy reports its unsupported operation. |
| HARN-007 | Provider-specific steering | Live integration | Demonstrate actual instruction delivery on each supported provider, not only a successful control response. |

Source detail: [conversations](features-sources/topic-007/INDEX.md), [harnesses](features-sources/topic-008/INDEX.md), [resume/fork/steering](features-sources/topic-009/INDEX.md), [conversation UI](../../../web/src/routes/conversations/+page.svelte).

## Workflow library behavior

These features need real workflow execution. Prefer exercising them in existing workflows; repeatable missing cases belong in package tests or ordinary maintained workflows, following repository policy. A browser walkthrough alone cannot establish these contracts.

| ID | Feature | Primary mechanism | What execution should demonstrate |
|---|---|---|---|
| RT-001 | Scopes and ownership | Unit/integration | Nested scope identities, cleanup, cancellation, and session lifetime match the workflow. |
| RT-002 | Scoped context | Unit/integration | Parent values are visible; child shadowing works; duplicate writes in one scope are rejected. |
| RT-003 | Large context values | Unit/integration | Large values become local file references while the full content remains available. |
| RT-004 | Concurrent groups | Unit/integration | Siblings run concurrently, join before return, and respond to an error/cancellation. |
| RT-005 | Finite iteration | Unit/integration | Every item gets a scope and its resources close before the next item. |
| RT-006 | Adaptive PromiseLoop | Unit/integration | Planner decisions dispatch tasks, update the backlog, consume feedback/steers, and distinguish stopping from fulfillment. |
| RT-007 | Scope-owned services | Unit/integration | A foreground service survives for its owning scope; premature exit fails it; normal exit/cancellation cleans up. |
| RT-008 | Run commands | Unit/integration | Preserve exit status, stdout/stderr, execution errors, and full large-output artifacts. |
| RT-009 | Record command evidence with Check | Unit/integration | A nonzero command result remains visible evidence; failure to execute remains an error. |
| RT-010 | Typed generation and templates | Unit/integration | Validate generated output, bound invalid-response retries, and render scoped context/custom templates correctly. |
| RT-011 | Non-gating supervision | Unit/integration | Observe worker activity at the configured interval, deliver objections, and stop supervising with the worker. |
| RT-012 | Interviews | Unit/integration | Generate a question, wait for its answer, carry dialogue forward, and end/cancel cleanly. |

Source detail: [scopes/context](features-sources/topic-010/INDEX.md), [loops](features-sources/topic-011/INDEX.md), [services/commands](features-sources/topic-012/INDEX.md), [generation/interviews](features-sources/topic-013/INDEX.md), [supervision](../../../supervise.go).

## History, usage, and authoring tools

| ID | Feature | Primary mechanism | What execution should demonstrate |
|---|---|---|---|
| PERS-001 | Run lifecycle journal | Unit/integration | Ordered, complete lifecycle records, including final status and recording errors. |
| PERS-002 | Shared project journal | Unit/integration | Concurrent runs leave their milestone records; consumers can order them by timestamp. |
| PERS-003 | Typed event schemas | Unit/integration | Produced lifecycle records conform to their generated event schemas. |
| PERS-004 | Provider transcript records | Unit/integration | Session logs retain provider messages/tools and their attribution separately from lifecycle records. |
| OBS-001 | Durable observation tables | Unit/integration | Run, scope, session, turn, usage, command, and interview facts survive process exit. |
| OBS-002 | Live updates and checkpoints | E2E + integration | A connected/reconnecting observer reaches the current state without duplicate or missing changes. |
| OBS-003 | Historical run reading | E2E + integration | Reopen recorded runs without resuming agents; exercise missing-table recovery only on copied fixtures. |
| OBS-004 | Usage and cost totals | Unit/integration | Correctly aggregate reported tokens; price supported models and distinguish unavailable prices from zero cost. |
| LINT-001 | Workflow authoring rules | Unit/integration | Reject dynamic dispatch, invalid keys/context, and unsupported prompt/template shapes with useful diagnostics. |
| BUILD-001 | Schema/type generation | Build checks | Generate the current Go/JSON/TypeScript contracts without manual generated-file edits. |
| BUILD-002 | Workflow graph generation | Build checks | Generate the declared scopes, nodes, roles, supervisors, and service ownership from ordinary Go. |
| BUILD-003 | Web binding generation | Build checks | Generate matching Go/Svelte remote interfaces. |
| BUILD-004 | Build the complete product | Build checks | Build a runnable binary containing the actual web application from the chosen revision. |

Source detail: [persistence](features-sources/topic-014/INDEX.md), [observation/pricing](features-sources/topic-015/INDEX.md), [lint/build](features-sources/topic-016/INDEX.md).

## Limits that must remain visible

- The inventory has 63 feature-family entries, including overlapping CLI/library views of the linter and steering. It is not 63 independent tests or a coverage percentage.
- Codex, Claude, and agy differ: a Gemini-driven validator still needs real Codex and Claude runs to validate those integrations. Provider credentials and harness availability are prerequisites, not passes.
- Generic browser workflow-start forms, historical backlog scrubbing, standalone pre-run previews, and the complete originally proposed cross-node relation display are not current capabilities. Existing compiled graphs and watcher displays should still be checked.
- Interviews and steering need a second actor; browser closure must not end a workflow. Destructive cases, implementation runs, restarts, and cancellation belong in disposable projects owned by the validation run.
- There is currently no built-in feature-file validator, daily scheduler, or automatic video artifact viewer established by this research. Those are proposed work.
