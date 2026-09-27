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
