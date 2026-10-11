# Independently built workflows and backends

Deliver one installed host that discovers native plugins at startup, submits an
ordinary Go workflow to daemon A, and lets that workflow execute agent turns,
commands and a useful service at daemon B. Central remembers run identity and
routes; A owns the independent workflow process; B owns native tasks. Rebuilding
a project's plugin and restarting changes its invocation input and typed agent
outputs without rebuilding the host or frontend.

This proposal follows the pinned October 10 goal, assessment and issue 439. It
is not implementation evidence. Source compatibility with replaced interfaces
is unnecessary. The user's subsequent correction excludes human-input pauses
and interview proof from this delivery.

## 1. Go-only authoring, registration and invocation

Root package `gimbal` owns `Registry`, `WorkflowDefinition` and `BackendDefinition`.
A workflow definition carries name, help, input JSON Schema, ordered field
presentation, role defaults, source-page data, `workflow.Graph`, and
`Prepare(json.RawMessage) (Invocation, error)`, where
`Invocation` is `func(context.Context, Env) error`. Prepare validates input and
returns a closure containing concrete plugin-owned parameters without starting
work. The authored entry remains `func(context.Context, Env, Params) error`.
There is no new workflow return-value convention or mandatory output schema.
Existing `Generate[T]` schemas and typed consumption remain plugin-owned;
existing scope results and artifacts remain retrievable. Its raw-JSON validation
boundary is specified below; existing bounded re-ask behavior is preserved.

Provide `gimbal generate --dir . --entry Review --name review` as the supported
external generation entry. It calls the existing `internal/generate` extraction
from the installed binary, adapted to the caller's Go module rather than the
built-in inventory. It invokes `go tool polytype --validate` in the workflow
package; the external `go.mod` must declare the Polytype tool and pin its version
to the installed host's build. Run inside the project module with its matching
Go toolchain on PATH. It emits a concrete definition,
input schema/validator/adapter, field metadata and source/graph data into that
workflow package. Generate agent output codecs/schemas from the same project Go
types through the supported generation path. Authors maintain ordinary Go,
field comments and role choices, not a second declaration of their types. The
plugin package exports `Register(*gimbal.Registry) error` and registers generated
definitions plus its backend definition. Generation/build needs Go tooling only;
no project Svelte generation, Node or host checkout modifications.

Generated workflows no longer call `RegisterGraph` from `init`. Their graph
travels in the definition; registry insertion validates duplicates before
installing graph/catalog data. Migrate built-ins to this same path and remove
the private invocation adapters map. Reject duplicate workflow/backend names,
malformed definitions and missing required role defaults with the source plugin
filename in the error. Do not retain a second registration authority that can
panic before the registry checks names.

The adapter validates schema constraints before decoding and preserves existing
semantic checks (nonblank required strings, positive required integers), returning
field-addressable errors before admission. Invoke Prepare again on A before
starting its runner. Keep supported input types to string/int/bool/optional;
reject unsupported shapes at generation with a useful error. Preserve omitted
optional values versus explicit zero, false, empty and null according to the
declared type. Derive flags, fields and JSON names from the same input contract.

Central, daemon A and runner processes load sorted `.so` paths from repeatable
configured plugin directories before accepting work. Fail their startup clearly
on incompatible plugins rather than silently expose a partial catalog. Pass
configured directories to local runner children. A's plugin directories are
configured paths on A; central does not transmit its local plugin path for remote
execution. Deploy matching project builds at central and A explicitly. Native
plugins require matching toolchain, platform, build flags and shared package
revisions. The external `go.mod` pins Gimbal and shared dependencies, including
transitive shared packages, to versions matching the installed build; `go.sum`
verifies downloads but does not select versions. No hot reload or cross-version
ABI promise.

CLI control commands load no plugins. `gimbal run` fetches serializable workflow
descriptions from the selected central's existing control interface, then builds
commands, useful parameter flags, role choices and help from that catalog. It
also accepts `--input` JSON/file. Central's adapter is authoritative for submitted
input validation. `runs`, `watch`, `cancel` and `steer` remain usable despite a
stale plugin file on the CLI machine. Generation also loads no plugins.

Replace the static workflow list import and built-in-only page lookup with
runtime catalog data through existing page load/query machinery. Add one shared
compiled Run form driven by the supported field metadata, with existing readable
field controls and a form remote spread on the visible form. Use a fixed
skgo envelope with workflow/project/workdir strings, `Inputs []LaunchField`
and role overrides as a slice of named structs, never maps. `LaunchField` has
`Name, Value string` and `Include bool`; real controls submit indexed fields
such as `inputs[0].name`, `inputs[0].value` and `inputs[0].include`. Convert on the
server using catalog metadata, rejecting unknown/duplicate names. An optional
field's Include checkbox distinguishes omission from explicit empty/false/zero
without JavaScript; required fields are always included. Boolean values use
explicit true/false controls, and null is accepted only where the declared type
allows it. Map conversion/adapter issues to `inputs[i].value`, the exact visible
control's form path. No hidden JSON packing is needed. skgo projects named-struct
slices and its browser form decoder reads indexed fields; probe generation and
native no-JS submission. Its separate Go SubmitForm encoder does not support
slice fields, so the CLI continues using the control interface, not that client.
No project Go types or handwritten browser endpoints enter generated bindings.
Preserve the existing guide and diagram renderers. No general JSON Schema form builder.

## 2. Concrete backend boundary and package ownership

Public package `backend` owns concrete wire DTOs and interfaces, importing only
the standard library. Root `gimbal` imports `backend`; the reverse import is
forbidden. Move the current `internal/effects` request types there, retaining
`{Run, Scope, Task}` identity. Root/runtime bridges decode native agent-event and
result JSON into Gimbal types. `internal/observation` alone decodes existing
snapshot/delta JSON; the public package must not import its private types.

A registered backend definition has name, configuration JSON Schema, a validating
configuration adapter, and this factory:

```go
Connect(context.Context, json.RawMessage) (backend.Connection, error)
```

`backend.Connection` contains independently supplied execution, task-event and
run-observation interfaces. Their implementations may share a connection but
execution cannot depend on successful event delivery. The backend plugin chooses
implementations through its validated configuration. No separate named event
factory registry is required; replacing either source needs no core edits.

| Capability | Concrete meaning |
| --- | --- |
| Task execution | Existing session/fork, turn, process, terminal result, interrupt, steer, close, scope cleanup and pending-ownership operations. |
| Task events | Native event JSON batches by task identity and cursor; no terminal result/status fields. |
| Workflow execution | Independent runner admission, run status and existing control/cleanup operations by run identity. |
| Run observations | Existing snapshot and committed deltas as opaque `json.RawMessage`, with a cursor and explicit unavailable/gap response. |
| Bytes | Stream explicit owner-scoped task files and referenced command/context/scope-result artifacts. |
| Service fulfillment | Scope-owned foreground command, requested TCP port, returned connection URL usable by its declared consumer. |

Execution exposes the concrete task/workflow/bytes/service operations above;
avoid `any`, arbitrary method-name dispatch or a general resource interface.
The initial HTTP implementation has separate result and events endpoints.
A serializable route carries backend name, configuration and environment identity,
not Go interfaces. Invocation holds separate workflow and task routes; each
process resolves them through its registry. This delivery needs one task route
per run. Register `local` as the default backend: plain `gimbal` and `gimbal run`
launch the existing independent local workflow child, whose local backend runs
native tasks directly. No standalone daemon is required and no extra Gimbal
process is created per task. Central `--task-listen` optionally exposes the same
daemon task/workflow handlers used by A, so either can supply A/B capabilities.
A and B may coincide. No scheduler is added.

## 3. JSON result boundary and observation

Go output type → generated JSON Schema → harness request → raw agent JSON →
adapter schema validation → unchanged raw JSON → one workflow decode into T.
Today `session.go` validates after `HarnessAdapter.RunTurn`, and `decodedOutput`
combines validation with typed decoding. Change the public HarnessAdapter contract:
a successful native RunTurn MUST validate structured Output before returning.
Add the shared helper
`gimbal.ValidateTurnResult(schema json.RawMessage, result TurnResult) error`;
it performs raw JSON syntax/schema validation (JSON-string validation for Text's
nil schema) using the existing schema library. Each of the five native adapters
(codex, claude, pi, agy, opencode) calls it once at its successful return boundary,
returning the same result plus any invalid-output error. External adapters have
the same obligation. This is an explicit contract, not runtime adapter detection.
At B the native adapter validates before transport; the remote adapter only
forwards raw Output, usage and invalid-output classification, without validating
again or reconstructing a project struct. Envelope encoding is distinct from
interpreting the result; carry opaque Output bytes unchanged through the wire.

Shrink `Output` to `Schema()`; Generate decodes accepted bytes once into T, with
no redundant schema validation or custom ValidateJSON extension. A failed typed
decode is an invalid result and uses the same bounded re-ask loop. An exported
invalid-output error classification in `gimbal`, matched with `errors.Is`, is
shared by native adapters, the remote client and Generate; the remote client
reconstructs it from the wire classification. Keep the
existing bounded Generate re-ask loop, carrying invalid-output classification
across the wire; do not add an adapter retry loop. Its internal decoded-acceptance
path lets `validatePlan` check the already-decoded plan and retain semantic
re-asks without another decode/schema check. Keep generated ValidateJSON methods
for actual consumers such as invocation input and event validators/tests; do not
delete unrelated validators merely because Generate stops calling them. SetJSON
and other Output users adopt the smaller constraint; retain their actual checks
where independently needed.

Split `effects.Wait`, which currently polls events and result together, into an
authoritative result waiter and independent event reader. Failed/slow event
reads expose an observation gap and bounded reconnect attempts without
interrupting work or changing a successful terminal result. Supervision consumes
received events with its existing timed fallback. Stop the event reader after
a bounded final drain when the result arrives; it cannot delay completion
indefinitely. Cancellation explicitly interrupts through execution. Reconcile
lost admission replies through execution status, never event cursors.

A serves its runner's existing observation snapshot/deltas. Central installs a
source adapter where `observation.Store.refresh` currently opens local paths.
Resume a valid cursor or replace the snapshot after an explicit gap. Reuse the
existing durable observation files; add no log/journal or recovery subsystem.
Central disconnection does not affect execution. Existing browser SSE/reducers
continue to consume the same projection.

## 4. Independent ownership and central restart

Extract local `exec ... run-process` admission from `host.Project.Start` into the
workflow execution implementation. A exposes it over HTTP and launches the
installed runner with A's plugin/workspace settings. Keep independent process
lifetime (`Setsid`); an admission request context does not own the child. A knows
its children through returned identity and direct handles. B directly runs its
native provider sessions and commands.

Central assigns the run ID and writes its OWN project-local
`.gimbal/routes/<id>.json` before admission, containing A/B routes, workflow,
placement labels and admission status. This record is distinct from A's run
directory, observation files and controller, including for local runs: the
sidecar does not precreate the directory that `gimbal.Run` owns. Central listing,
`Registry.Snapshot` and `Runs.Resolve` read these local records and construct the
remote source/controller adapters; they never expect A's files in central's
filesystem. Thread the assigned ID through `gimbal.Run` and runner admission.
A rejects duplicate execution under that identity and
answers admission-status reads after a lost reply; do not blindly replay a start.
Persist placement/ownership labels for the run page. No OS process-list discovery.

A proxies steer, loop, scope/turn cancel and cleanup operations to the existing
runner controller. Central reconstructs `live.Runs.Resolve` from retained routes
after restart. Persist enough existing project catalog metadata to reopen known
projects/runs without resubmission. Central shutdown does not release A or B.
Existing answer/control plumbing may remain where naturally shared, but adding
or proving human-input pauses is excluded.

Only A's child-exit observation or its local lifetime lock establishes runner
death. A requests run-scope cleanup at B and retains failures for retry, reachable
even after the runner exits. An unreachable A means unavailable control, not a
dead workflow. Expose cleanup pending truthfully. A/B daemon crash recovery and
seamless upgrades are outside this delivery's central-restart demonstration.

## 5. Files and useful services across different filesystems

Central has its own local project/catalog directory for registry and route
sidecars, distinct from A/B execution workdirs. Each execution environment supplies
its own workspace root and validates its workdir. Central must not `os.Stat` a remote directory. Distinguish project/workspace
identity from environment-local paths. Repositories are explicitly provisioned
at A and B; arbitrary repository synchronization is not required.

Transfer Gimbal-owned context bytes and explicitly provided task files before
remote dispatch, then put B-local paths in the prompt. Backend workdir mapping
supplies B's repository location. Do not sweep the filesystem/provider state or
claim transparent rewriting of arbitrary absolute paths in workflow prose.

Replace shared `effects.Process.Stdout/Stderr` filenames with artifact identities.
B owns captures and returns references; the workflow reads actual bytes and
records existing excerpts/links. Copy command/context/scope-result bytes needed
after retirement to A before acknowledging consumption at B. Central's artifact
handlers resolve the run's byte source and preserve referenced-artifact checks,
not local opens of remote filenames. Shared run directories are unnecessary.

Service fulfillment is deliberately bounded: ordinary workflow code requests a
foreground command and TCP service port, and receives an owned service identity
and connection URL. The backend allocates/maps the port and returns an address
reachable by the declared consumer (A in the proof). Fail if that topology cannot
be fulfilled; B's `localhost` is not an answer usable at A. The workflow checks
readiness through that URL. This supports a useful HTTP service without a
resource-requirement language, database catalog or dependency framework. Record
location and scope ownership without credentials. Scope exit releases the
service; failed cleanup remains owned and retryable.

## 6. Docker ownership and bounded retention

Docker Start returns an ownership receipt: created container ID, fresh ownership
label and endpoint. Store runtime receipts outside Git. Attach the label during
`docker run`; Stop inspects/removes only the returned ID after checking its label.
A conflicting name returns no receipt and cannot be removed by Stop. If readiness
fails after creation, return/persist its receipt with the error so the actual
owned container can be released. Already absent succeeds; ownership mismatch
refuses removal.

Use one completed-payload policy: after reading the terminal result and copying
required bytes, the workflow acknowledges consumption. Delete event/result/capture
payloads once acknowledged and native resources are released. Otherwise expire
them five minutes after completion, with a distinguishable expired read response;
this bounds abandoned-reader retention duration. Readers must finish within that
window, and in-flight reads hold their already-acquired data until completion.
Never expire active tasks or cleanup-pending ownership. A released run's compact
closed-scope markers expire after the same window. There is no new active-run
registration, admission-reservation or persistent retirement subsystem. The
runtime never dispatches new work for a scope after closing it, and does not
replay old mutations after its supported lifetime.

## 7. Boundaries and proof

Implementation seams:

- Authoring/catalog: public root registry/definitions, `gimbal generate` and
  `internal/generate/{source,stock}.go` plus extraction/Polytype integration;
  remove generated init registration, migrate `internal/invocation` and built-ins;
  runtime CLI catalog and dynamic workflow list/detail/form.
- Execution/observation: public backend DTOs/interfaces, `internal/effects`,
  `internal/taskdaemon`, `remote_execution.go`, `service.go`, `harness.go`/`session.go` raw-result boundary
  and native adapter validation, public run-ID path
  in `run.go`, `internal/runner`, `internal/host`, and
  `internal/observation/{follow,registry,artifacts}.go`.
- Environment/presentation: Docker receipt release, daemon admission/control,
  placement/ownership display and existing result/artifact access. Generated files
  remain generator-owned; remove replaced static/fixed-path assumptions.

Package tests cover generated validation and optional-value meaning, duplicate
registration without init panics, stale-plugin-independent CLI controls,
execution success despite failed events, restart routes, actual byte content,
service release, Docker refusal and acknowledged/expired task retention while
active work and cleanup-pending ownership remain. Verify invalid local/remote
agent JSON causes the same bounded re-ask, valid raw bytes reach one typed decode,
and no duplicate validation occurs. Exercise plain local launch plus optional
central daemon handlers and no-JS indexed-field submission with addressable errors. Tests belong beside packages.

Live proof begins with an external Go module. Generate two workflows from source:
first parameters contain a required goal string and optional boolean; second
contains a positive count and service label string. Use their distinct concrete
Params and `Generate[T]` output structs; the host does not import project types. Generate/build only
through the supported Go path. Invoke both through runtime CLI flags and the
shared form; inspect invalid-input errors, defaults, roles, guides and diagrams.
Change the second Params and Generate output struct, rebuild only the plugin,
restart and prove the host accepts the new input and the workflow validates and
consumes the new typed agent result. No new workflow return-value schema.

Run A and B with separate workspace/run roots. Execute a real cheap-provider turn
at B (name `gpt-5.6-luna`, Haiku or Gemini flash), transferred context, a command
whose output exceeds the excerpt, and a backend-fulfilled HTTP service. Read the
service from A through the returned URL and exact artifact bytes through central.
Observe the graph and A/B/scope ownership in the running UI. Interrupt only event
delivery: confirm authoritative results and controls work, then observe a gap or
resynchronized projection after reconnection.

While two runs are active, restart only central. Reopen without resubmission;
steer the agent and loop, cancel a turn/scope, retry a deliberately failed cleanup,
and cancel one run while the other completes. Confirm service release and
retrievable scope results/artifacts after task retirement. Check same-name
unowned Docker refusal without deleting that container. An independent validator
assesses the real evidence. Report revision, models, observations and limits in
PR/chat; commit no transcripts, screenshots, run output or temporary proof code.

Explicit deployment, matching native builds and mapped workspaces are accepted
costs. No auth, Temporal, recovery system, journals, fencing, guardians, workflow
compiler, extra infrastructure backend or general resource framework is added.
