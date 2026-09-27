# Temporal backend experiment

scope: Tyler explicitly requested a goal and implementation experiment to discover the API. Confirmed ordinary-Go control plus Temporal activities; persistent Docker volumes and Postgres; portable session snapshots and cold offload deferred. Live provider execution not required.
finding: Local Temporal CLI 1.9.1 / server 1.32.0 supports standalone activities. Docker Desktop started successfully; VM has about 2 GB RAM and 3 CPUs, so keep infrastructure small.
decision: Run existing harness adapters inside environment workers and proxy HarnessAdapter; introduce a provider-neutral process handle for commands/services. No virtual OS or provider-specific fields in root API.
finding: Official Codex authentication guidance warns against shared rotating auth caches across machines. Reserve one mutable credential directory per active container; preserve refreshed state and never reseed automatically.

## Implementation handoff

Tyler revised the goal: use Gimbal for coding with lower models, make the
orchestration layer work before polishing abstractions, then do a second seam
refactoring pass. Direct coding stopped after the provisional execution.go
scaffold. Built-in implement run 01M3GW34S2KBBS97EKV8PQ32YS.implement uses Sonnet 5
for planning, QA and scope critique, GPT-6 Luna for coding. First bounded outcome
is a real Docker/Temporal command path with Postgres bootstrap and events.

Tyler then proposed backend-provided HarnessAdapter objects, with orchestration
as middleware around stock harnesses. This fits the current environment-supplied
adapter draft. Keep commands/services/lifecycle beside the adapter and avoid a
second generalized callback API. Direction recorded in the scope brief and
queued to the Gimbal planner; active coder was steered to integrate the existing
RunCommand path rather than only produce a backend-only demonstration.

Live slice: `temporal server start-dev --ip 0.0.0.0`; `docker compose -f docker-compose.temporal.yaml up -d postgres` (5433); Docker Hub metadata timed out, so I cross-compiled `cmd/gimbal-worker` for linux/arm64 into the cached Ubuntu runtime image. Through `gimbal.Run` + `InEnvironment` + `RunCommand`, the Docker worker ran `sh -c "printf 'from-temporal-worker\\n' > shared.txt"`, then `cat shared.txt` returned `from-temporal-worker\n`. Postgres held the owner/queue bootstrap row and two command events. After `DROP TABLE gimbal_command_events`, `sh -c 'printf command-still-success'` returned exit 0 with nil error and emitted a recording-degraded message.

## Delegation friction to carry forward

The coder began preparing another Claude review even though the built-in
implement workflow already runs Sonnet QA. Steering removed that duplicate
review setup; future outcome briefs should explicitly rely on the workflow's
own validator. The initial Dockerfile image build encountered registry metadata
timeouts. The real command path ran using a Linux ARM64 worker cross-compiled
on the host and placed in a cached Ubuntu image; do not confuse that evidence
with verification of the proposed Dockerfile build. Review diagnostics that pull
uncached images repeat the same external blocker; prefer cached images with
pull disabled while continuing functional work.

## First slice checkpoint

Built-in implement run 01M3GW34S2KBBS97EKV8PQ32YS.implement completed. Its command
check ran go build ./... and go test ./... successfully. Sonnet independently
ran the real Docker/Temporal write/read and cancellation paths, observed Postgres
bootstrap/event rows, and verified backend cleanup removed the worker and its
bootstrap row. It approved this slice with two gaps to carry forward: the
Dockerfile build remains unverified because of registry access, and a worker
with a bad Postgres address can crash after docker run succeeds while the caller
waits too long. The next worker/harness slice must add bounded actual readiness
and a clear startup failure; successful Docker creation alone is insufficient.

## Image-pull blocker narrowed

After the first checkpoint, pulling golang:1.27.1-alpine succeeded with an empty,
task-local Docker configuration and the explicit Docker Desktop Unix socket.
The earlier failures therefore were not proof that Docker Hub was unavailable;
the normal local credential-helper path was involved. The isolated public-pull
configuration leaves the user's Docker settings and credentials untouched.
A normal Dockerfile build is now being checked with that configuration.
Next built-in implement run: 01M3GZ3PQ6DPHMWW7TF9AY4ZC2.implement (same
Sonnet 5 planning/validation and GPT-6 Luna coding mapping).

The checkpoint Dockerfile build completed successfully with the isolated config:
gimbal-command-worker:dockerfile-check (de37538473a9). It compiled the worker in
golang:1.27.1-alpine and installed bash/git in alpine:3.22. This resolves the
previous Dockerfile-build gap for checkpoint 0b0373c4. The image currently has
only the command worker; provider CLI installation belongs to the next slice.

## Readiness task and workflow handoff correction

Sonnet independently confirmed the bad-worker startup now fails in under a
second with environment-specific diagnostics, cleans its container/bootstrap
row, and allows a healthy same-name retry through RunCommand. Build and existing
Go tests passed. This completes readiness only.

The built-in implement validator marked the broader outcome passed after only
this first planner-selected readiness task, so the workflow advanced before
harness support existed. Manager explicitly corrected outcome 2's planner:
missing remote harnesses, role bootstrap, provider state/secrets and event
streaming remain required. Future validation must assess the entire selected
outcome, not only its first task. No completion claim is inherited from that
incorrect outcome-level approval.

## Environment-bound session seam

Readiness checkpoint 2a6de3d8 passed the lint gate after replacing the probe
callback with a private named-method interface. The session task now captures
environment identity at NewSession, resolves the backend adapter lazily on first
action, returns operational setup failures normally, preserves identity across
forks, and rejects incompatible supervisor placement. Sonnet independently
accepted those changes with fake-harness tests and passing build/Go/lint checks.
No real remote harness session has been demonstrated yet; the backend Harness
method remains nil until the next task. Explicit QA steering kept the overall
outcome open so the remaining remote harness/service/invocation work proceeds.

## Disk exhaustion interrupted remote harness qualification

The management process reported run 01M3GZ3PQ6DPHMWW7TF9AY4ZC2.implement
terminal at 2026-09-27 03:13:43 local: no space left on device while writing the
active coding session log. The saved turns file consequently still shows an
active turn; it is stale. The instance itself (PID 82269) remains alive.
Remote harness changes are uncommitted and not independently accepted.

Before interruption, the provider-bearing image built with Codex 0.157.1.
A real lifecycle probe exposed missing procps (BusyBox ps lacks lstart), then
a need to start the Codex daemon during worker initialization. Both changes
are in the worktree, but the final image rebuild/lifecycle probe did not finish.
Do not treat those paths as passing. Earlier Go build/test passed before the
last initialization edit.

Host disk had only about 120 MB free; Docker metadata began returning I/O
errors. Clearing regenerable Go build cache recovered space. Docker restart
requires coordinating with the user because unrelated merge-herder-db-1 also
runs there. No Docker reset/prune or unrelated data deletion is authorized.
The next run must finish the harness, then services, built-in invocation, and
the requested API simplification pass. Keep each outcome bounded so narrow
task approval cannot silently skip a broader required outcome.

## Auth startup corrected and independently checked

Recovery run 01M3H2FQJYP6543Z2AZAMREVNN.implement completed its first bounded
auth/daemon task. The coder traced Codex 0.157.1 app-server disabling environment
API-key authentication; worker startup now calls codex login --with-api-key
using stdin only when its persistent auth.json is absent, and bounds daemon
startup to 30 seconds. Existing auth.json is retained. Sonnet independently
observed real CLI login with a dummy key and idempotent daemon startup in
temporary isolated homes, and reproduced package build/vet/tests (12 pass,
2 Docker integration tests skipped). No authenticated provider turn was run.

The full harness outcome remains unaccepted because its real container proxy
lifecycle has not succeeded yet. Earlier command-only readiness did pass live;
the recovery reviewer overstated that gap as all readiness having only skipped.
The lifecycle test also needs an explicit key requirement or a deliberate dummy
key before its next Docker run, since startup now correctly rejects absent auth.
Host build-cache cleanup recovered roughly 55 GB; one concurrent writer caused
a harmless directory-not-empty cleanup error. No further cache cleanup needed.
Docker metadata I/O errors persisted after disk recovery; restart approval is
still pending. Continue independent services/CLI implementation and retain live
qualification for the final integration pass.
