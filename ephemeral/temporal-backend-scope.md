# Temporal container backend experiment

Tyler requested working proposed code for both a small Gimbal execution API and
an initial orchestration backend. Live provider execution/deployment is not
required. He selected ordinary Go workflow control dispatching Temporal
activities, with persistent Docker volumes and Postgres first. Portable native
session export/restore and two-day object-store offload are deferred.

Implement named environments shared by commands, services and agent sessions.
Existing real harness adapters run inside worker containers; a remote adapter
implements the existing HarnessAdapter interface. Planners and supervisors use
that same path. Supervisors must share their principal's environment. The root
package must not know Docker images, secrets, Temporal queues or Postgres DSNs.
The run owns an io.Closer backend and closes it after scope cleanup.

Backend constructors own environment/role configuration. On resolution they
persist a worker bootstrap row and start the environment's Docker container.
The worker loads its row, configures its allowed harness adapters, then polls a
unique Temporal queue. Activity calls carry operation/session/role identities;
provider credentials stay out of activity payloads and database events.

Use standalone activities with automatic retry disabled for mutating work.
Cancellation must explicitly reach the Temporal activity and its process.
Steering must work independently of long-running turn activities. Persist
worker events in Postgres and stream them through the existing Gimbal callback
so local canonical roll-ups/supervision continue to work. One persistent
control/web process remains the observation authority for the experiment;
database-backed replicated frontend reducers are not part of this slice.

Docker setup must preserve absolute project/run paths in worker mounts because
existing prompts reference them. The initial backend uses shared mounted
workspace storage for the consistency promise, without Git copying or recursive
blob storage. Persistent native state is retained, but lost worker processes
do not transparently restore adapter maps or ordinary Go control flow.

Credential references are backend configuration. Dedicated mutable Codex login
state has one container owner, survives refreshed writes, and is not repeatedly
seeded from a stale host copy. API keys/OAuth token inputs are operator-supplied
secret files; implementation must not copy this machine's login automatically.

Verify compilation, targeted tests with fake harnesses, Temporal dispatch and
Docker/Postgres protocol tests where feasible. User waived a live model run.
Review scope is this experiment, not a production orchestrator or a claim that
every prior research hypothesis is already implemented.

## Backend-provided harnesses (Tyler's latest direction)

Prefer letting the backend supply HarnessAdapter objects. The remote adapter
can be middleware/decorator around stock provider adapters, keeping transport,
routing, event durability and cancellation behind the existing harness methods.
Avoid a separate generalized orchestration callback API in the runtime. Commands,
services and lifecycle still share the named environment alongside its supplied
harness. Get the command path working, then assess how much additional API can
be eliminated in the second integration/refactoring pass. This is a simplification
direction, not a requirement to force commands into the harness interface.

## Harness/worktree binding clarification

Tyler referenced StrongDM Attractor's tool execution environment as inspiration,
not a required interface: https://github.com/strongdm/attractor/blob/main/coding-agent-loop-spec.md#4-tool-execution-environment.
He proposes supplying a harness and environment bound to the project/worktree.
The important relationship is the same live workspace and services, not merely
identical path strings on unrelated machines. Favor obtaining the adapter from
its environment so the backend owns compatibility. For stock Claude/Codex,
running the real adapter and agent process in the same container as the worktree
is sufficient for this experiment; do not invent interception of every native
agent tool or copy Attractor's read/write/grep/glob API without a concrete need.

## Session failure semantics

Preserve ordinary workflow error handling: container/database/network failures
while resolving an environment are operational errors, returned from the action
that needs it. They must not become NewSession panics merely by weakening its
existing lazy-start contract. Defer remote setup to the existing native-session
initialization/first action path where practical. Missing static role bindings
may retain their existing programming-error behavior. Environment identity still
needs to survive forks and permit clear supervisor compatibility checks.
