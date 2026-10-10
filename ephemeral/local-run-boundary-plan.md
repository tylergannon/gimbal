# Implementation plan: local run boundary

2026-10-10. Ready for implementation handoff; execution awaits Tyler's go-ahead.
This incorporates the decisions from the recovery conversation. Consult older
material only when it answers a concrete question; it does not add scope.

## Result

Start an existing stock workflow from CLI JSON input. It runs in its own process.
Restart the server; the same run remains visible and controllable in the UI and
CLI. Delete the compiled execution path and its experimental consumers.

One delivery, with an early working checkpoint. This does not deliver Docker,
external workflow loading, or web invocation.

## Ownership and contracts

| Owner | Responsibility |
| --- | --- |
| Workflow | Concrete input parameters and ordinary Go workflow body |
| Shared invocation code | Pure Go descriptions, input validation/adapters, name-to-body dispatch |
| Run process | Existing gimbal.Run, workflow stack, agents, commands, run records, controller, cleanup |
| Daemon | Project admission, child launch, discovery of known runs, observation/control API |
| CLI | Accept JSON and submit it; retain named flags only where they translate simply into that request |
| Web | Dynamic lookup by workflow identity, observation and existing controls; invocation deferred |

Anything the web consumes must be pure Go data that can be returned by the plugin
interface. Shared invocation code does not import web routes or skgo. This does
not require implementing external plugin loading now. Keep existing Generate[T],
typed outputs, validation/re-asks, supervision, branching, and session semantics.
Do not introduce another semantic runtime.

**Start.** Carry workflow name, input JSON, project/workdir, role selections and
existing conversation association. Validate before launch, preserving current
required-field rules and omitted versus explicit values. Generate one typed
adapter/dispatch path from existing workflow declarations. Replace the old web
execution closures rather than retain parallel start implementations.

The daemon launches the child; the child uses normal run-ID/directory creation
and establishes its control endpoint. It returns one startup response with the run
ID before workflow/provider work. The daemon returns that ID to the CLI.
No reservation/commit/confirmation handshake.

**Lifetime.** The child runs independently of the daemon's context, signals and
output descriptors. Reuse the existing executable with a run-process entry that
does not start the web application or another project-owning server. The daemon
keeps the project ownership lock; each child records how to reach its controller
in its own run directory. A replacement daemon discovers known runs from project
state, without OS process-list searches. Normal server shutdown neither cancels
started runs nor waits for workflow completion.

**Controls.** Preserve start, list, snapshot, events, artifacts, agent steering,
loop steering, interview answers, and run/scope/turn cancellation through the run
API. A child remains controllable while existing cleanup is pending. Preserve
useful existing errors; do not create another control state machine.

**Observation.** Preserve existing observation behavior across processes. Brief
lag is acceptable. Demonstrate that activity advances and completion arrives.

The child writes run records; the daemon reads them. This applies to run pages,
lists, conversation status and child-run monitoring. Reuse existing observation
formats and cursor/reset behavior. Display the graph belonging to the run.
Socket loss alone does not establish process death; use RecordTerminated only
after establishing that the writer stopped. Do not resume a dead workflow.

## Work and first checkpoint

After authorization, create a fresh Gimbal task worktree from current main.
Do not transplant the rejected implementation. Reference a specific old fragment
only when it answers a concrete implementation question.

| Work | Existing source to change | Required result |
| --- | --- | --- |
| Shared invocation and CLI JSON | internal/generate/entry.go, stock.go, cmd/gimbal/*_gen.go, workflow parameter types | One generated invocation contract; CLI start no longer depends on a web form |
| Independent run and daemon forwarding | cmd/gimbal/, internal/host/host.go, internal/live/live.go, run.go, web/control.go | Start returns the child's ID; restart preserves execution and controls |
| Observation and consumer migration | internal/observation/, run/project page loaders, web control remotes, CLI controls/observation, internal/conversation/, internal/workflows/researcheval/ | Existing observers and child-run consumers work through the new boundary |
| Remove web invocation | Per-workflow start remotes/generation and launch affordances | No web start path invokes a workflow in-process; existing run controls remain |
| Remove compiled execution | compiled_*.go, compiler/, compiled host/web adapters, internal/temporalgen/, internal/experiments/instrumented/, examples/temporal/, exclusively dependent code/tests | No compiled execution path; ordinary workflow/schema/graph generation still works |

Paths identify the expected change surface, not a requirement to create packages
or rewrite every file in those directories. Delete shared helpers only when no
remaining consumer needs them.

**First checkpoint:** one stock workflow, started through the actual CLI JSON
adapter, runs independently, appears in the existing UI, survives server restart,
and can be steered and cancelled. This is product code on the delivery branch,
not a temporary dispatch table or separate prototype. Show the working behavior
and diff before scaling the migration.

**Finish:** apply the same path to remaining stock workflows and consumers;
complete the deletions; run the checks below. Do not turn this into separately
designed infrastructure projects.

## Acceptance

| Behavior | Evidence |
| --- | --- |
| JSON invocation and validation | Real CLI start returns the run ID; focused tests reject invalid input before admission and preserve supported input/role semantics |
| Independent lifetime | With a real provider turn active, stop/restart the server normally and abruptly; observe the same run continuing and steer it |
| Controls by identity | With two runs active, cancel one and let the other complete; focused tests exercise interviews, loop steering, scope/turn cancellation and retained cleanup |
| Observation across processes | In the running UI and CLI, activity advances and completion arrives; lists and conversation status catch up; artifacts and the run's graph remain available |
| Existing child workflow behavior | Bounded completion and timeout/cancellation exercise through research-eval's changed path; no full quality benchmark |
| Runner exit | A confirmed dead runner is reported truthfully without resuming it |
| Integrated application | Relevant package tests, normal generation/build checks and regression suite pass after migration and deletions |

Use existing tests and small package-local additions. Use the cheapest suitable
configured provider for live work; checks that do not need a provider do not get
one. State revision, commands and observed results in the PR. No proof framework,
paid benchmark, committed transcripts or runtime artifacts.

## Limits and completion

The size tripwire remains **2,000 added handwritten lines including tests and
frontend changes**, measured against the delivery base. Report generated additions
and deletions separately. This is a stop-and-assess threshold, not an estimate or
target; do not hide additions behind deletion totals or generated wrappers.

At the early checkpoint and final code review, assess correctness and necessity:
what requested behavior fails, and what can be deleted without losing it?
Reviewer remedies do not automatically enter scope. Bring a scope change or budget
breach to Tyler with the concrete diff and smallest alternative. Routine internal
choices remain the implementer's responsibility.

Excluded: web invocation; Docker or another backend; runtime plugin discovery and
external authoring tools; generic form/CLI generation; auth; crash resume; abrupt
death during startup handoff; special handoff/shutdown protocols; event databases;
new delivery guarantees; deployment management. Issue #439's remaining external
plugin experience is later work, not claimed complete here.

Completion means the agreed behavior is demonstrated, obsolete paths removed,
the reviewed PR merged, and the root checkout synchronized. Until implementation
is authorized, this remains a plan; no runtime outcome is claimed.

## Targeted references

Source inspected at Gimbal 1c3eb84c94c5cce42a7f78018ca51e285e789dc1:
internal/host/host.go for admission/start/lifetime; internal/live/live.go for
controls; internal/observation/registry.go for replay-once caching;
internal/observation/terminated.go for terminal settlement;
internal/workflows/researcheval/researcheval.go for child observation/cancellation.

The [Fable review](reviews/20261010-local-run-boundary-fable-round-01.md) describes
the earlier proposal. [Discussion and decisions](local-run-boundary-review-disposition-2026-10-10.md)
distinguish its suggestions from subsequent user decisions. Consult either only
when needed; neither adds requirements to this handoff.
