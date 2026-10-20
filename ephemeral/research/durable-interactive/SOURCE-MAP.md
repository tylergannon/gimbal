# Source map for durable Interactive execution

Read-only mapping at Gimbal `ed50f6bd` by three subagents: execution/lifetimes,
interaction/conversations, and indexed token-cache prior art. No live execution
or implementation claims were established.

## Current code

Paths below are repository-relative. Godoc is the public contract; the API
design record is historical rationale.

| Concern | Evidence | Implication |
| --- | --- | --- |
| Ordinary workflow entry | `run.go:142-194` creates a run, executes the body, completes it | No execution reconstruction entry exists |
| Portable identity | `runstore/store.go:45-86`; `runstore/directory/directory.go:370-396` | Stable parent/kind/name/ordinal slots can identify replayed work |
| Committed transitions | `runstore/directory/directory.go:471-593` | Whole batches are validated, framed and committed; record IDs are idempotent |
| Recording recovery | `runstore/store.go:1-3`; `runstore/directory/directory.go:198-297` | Opening repairs records; it explicitly does not resume execution |
| Single writer | `runstore/directory/directory.go:177-196,676-709` | Durable inbox acceptance must account for an active run's exclusive writer |
| Completed actions | `run_recording.go:97-164`; `session.go:296-318,439-472` | Intent, results and native bindings are recorded, but operations do not consult them to skip execution |
| Scope context | `scope.go:114-119,344-368`; `event_persistence.go:138-165` | Reconstruct values and operation ordinals; current live state is not a continuation |
| Concurrency | `group.go:65-98` | Child errors cancel siblings and Wait joins children; suspension needs distinct semantics |
| Scope end | `scope.go:184-304` | Return/cancellation currently ends scopes, stops owned work and clears values; suspension cannot use normal teardown |
| SERVICE boundary | `service.go`; `remote_execution.go`; `scope.go:252-304` | Leave services and claims untouched on suspension; restoration is outside scope |
| Hosted execution | `internal/host/routes.go:104-136`; `internal/runner/launch.go:16-54` | Hosted workflows currently launch independent OS processes |
| Launch inputs | `internal/invocation/invocation.go:19-32,69-107` | Reconstruction needs invocation, bindings and code identity; graph.json alone is display metadata |
| Interview | `interview.go:13-80`; `run.go:363-411`; `web/src/routes/interview.remote.go:27-47` | Existing Q/A API waits in memory and answers route to a live controller |
| Conversation manager | `internal/host/host.go:140-155`; `internal/conversation/manager.go:53-81,113-155` | Separate runtime today, rather than an ordinary workflow |
| Conversation delivery | `internal/conversation/requests.go:44-240` | Journals admission, steers active turns or queues serialized turns, calls prose RunTurn directly |
| Conversation restart | `internal/conversation/manager.go:216-234`; `requests.go:178-203` | Native binding resumes lazily; interrupted requests become unresolved |
| Submission identity | `internal/conversation/requests.go:60-66` | Identical request ID reuse is idempotent; conflicting reuse fails |
| Archive gap | `web/src/lib/workspace/ProjectWorkspace.svelte:180-186`; `internal/conversation/remote.go:703-823` | Tab close only hides presentation; remote cleanup releases resources; neither defines human archive |
| Tagged unions | `schema.go:34`; `workflow/schema.go:14`; `workflow/polytype_gen.go:452-510` | Existing Polytype sealed unions generate flat discriminator objects |

Replacement reaches the public API and generated schemas, lifecycle records
and observation projections, controller and UI delivery, workflow extraction
and diagrams, the example workflow, and the separate conversation runtime.

Representative production LOC baselines (not approved cell budgets): run 576,
scope 646, group 99; run recording 208, session 902, command 385; runstore
contract 117 and directory implementation 796; runner files 615 total, host
routes 262, invocation 108; conversation manager 779, requests 411, remote 962;
observation store 867, registry 224, durable projection 235. Generated code and
tests need explicit treatment when assigning mechanical budgets.

Existing tests worth carrying into claims: directory writer exclusion/atomic
batches/interrupted tails/lost acknowledgements; root persistence failure and
scope lifetime tests; Interview routing/cancellation tests; conversation
request identity, messaging and remote uncertainty tests; browser conversation
and interview integration tests. `run_recording_test.go:203` proves behavior
within one execution, not deduplication after replay.

## Indexed prior art

Entrypoint: `/Users/tyler/src/gimbal-token-cache/index/README.md`. Followed
messaging/waits, persistence, recovery/ownership and determinism/concurrency
routes through their leaves into actual cached sources and tests.

The primary paths below are relative to `/Users/tyler/src/gimbal-token-cache/`.
Cached DBOS Go revision: `339705771d7e198c5bac9e213903820c0575e6c1`.
Cached DBOS docs revision: `5c118aacd4b07a8988c63041cf3e61482299403a`.
These are prior-art facts, not dependencies or Gimbal requirements.

| Pattern | Primary evidence |
| --- | --- |
| Return recorded receive before waiting | `repos/dbos-transact-golang/dbos/workflow.go:3552-3565` |
| Consume message and checkpoint receive transactionally | same file `3592-3609` |
| Subscribe before reading pending messages | `repos/dbos-transact-golang/dbos/internal/sysdb/system_database.go:4503-4529` |
| Durable message address and consumption identity | same file `4532-4555` |
| Poll durable state when notification is missed | same file `4455-4499` |
| Return completed step result before executing body | `repos/dbos-transact-golang/dbos/workflow.go:2805-2819` |
| Allocate concurrent operation identity before launch | same file `3120-3142` |
| Fence stale checkpoint writes by owner | `repos/dbos-transact-golang/dbos/internal/sysdb/system_database.go:3052-3064` |
| Racing receives consume once | `repos/dbos-transact-golang/dbos/workflows_test.go:5871-5912` |
| Replayed send avoids duplicate message | same file `6420-6448` |

Important limits:

- The route's claim that dormant conversations hold no process memory is too
  strong: DBOS Go Recv calls waiter.Wait (`workflow.go:3585-3586`) which blocks
  in a select loop (`system_database.go:4455-4501`). It does not demonstrate
  release of active Go execution.
- The HITL example publishes an event and receives separately
  (`repos/dbos-docs/docs/ai/hitl.md:47-50`). It does not demonstrate Gimbal's
  stronger atomic reply-plus-wait requirement.
- Message ordering uses millisecond timestamps (`system_database.go:4532-4547`);
  its test explicitly leaves same-millisecond order unspecified
  (`workflows_test.go:5973-5983`). A total inbox order needs its own contract.
- Fencing durable writes does not make external effects execute once:
  `workflows_test.go:4152-4196` deliberately executes a body twice.

## Decisions not settled by source

Replay safety of ordinary Go observations/effects; safe recovery versus an
unresolved in-flight action; active-turn input routing and batching; admission
identity/order and terminal races; human archive versus agent end; Interactive
return/history contract; and the placement boundary of shared Go capacity.
SERVICE restoration is deliberately deferred and must not be reopened as an
implicit replay prerequisite.
