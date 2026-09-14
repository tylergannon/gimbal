# Wrap a real process manager: services as graph nodes, edges as requirements

URL: https://github.com/tylergannon/gimble/issues/76
State: closed
Milestone: None
Updated: 2026-09-08T03:12:02Z

## What this replaces

`.tractor/run`, shipped in #60, is a single shell string the engine starts and kills. It was the wrong shape. Tyler's ruling: **wrap a real process manager**, which is what he wanted from the start.

An opaque command cannot be queried, and that is not a missing feature — there is nothing inside it to name. It also cannot be made current: making a running application reflect the tree means restarting *something*, and a black box has no somethings.

The concrete failure this comes out of: a `sprint-execute` run reached stage 128 with no application running for 127 of them. `startService` is called from `validateSet`, which only runs when the loop node is arrived at; that run arrived once, at stage 1, and `defer app.stop()` killed it on the way out. The `implement`/`review` cycle then spun against nothing. The run record could not show this, because the only timeline event that mentions the service is `ServiceUnavailable` on the failure branch — a clean start, a service never started, and a repository with no service all emit the same silence.

## The design

**Services are nodes. An edge to a service node is a requirement, not an instruction.**

Tyler's rulings, in his words as decisions:

1. Entering a node sets the requirement: every service it requires must be up and healthy before the node's work begins.
2. Leaving the node lifts the requirement and does nothing else. No stopping, no restarting, no monitoring, no polling.
3. Any number of named services; a node may require one or more of them by holding an edge to each.

Everything the earlier sketch spelled as a verb falls out of this. `up` is what the engine does when a requirement is unmet. `down` is run teardown. `restart` is deferred — see *What this does not fix*.

### Lifecycle

| Moment | Engine | Overmind |
| --- | --- | --- |
| Entering a node with requirements | Ensure each required service is running and healthy. Wait. **Fail the node if it cannot be.** Never a question put to a model. | `status`, then `start -D` or `restart <svc>` |
| While the node runs | Nothing | — |
| Leaving the node | Lift the requirement. No action on the service. | — |
| Entering a node with no requirements | Nothing. A workflow declaring no services runs exactly as today. | — |
| A required service dies mid-run | Nobody notices until the next node that requires it, whose entry brings it back. Crash recovery with no monitor. | `restart <svc>` |
| Run end | Tear the instance down unconditionally, whatever the graph did | `quit` |

### Shape

One service node per Procfile process. The Procfile already names them, so the arbitrary number of named services comes for free, and each is individually addressable.

```yaml
nodes:
  - id: web                   # matches a Procfile process name
    type: service
  - id: worker
    type: service

  - id: implement
    type: agent
    edges:
      - { to: review }        # control flow
      - { to: web }           # requirement

  - id: review
    type: agent
    edges:
      - { to: sprints, condition: ... }

  - id: sprints
    type: loop
    edges:
      - { to: web }
      - { to: worker }        # a node may require several
```

The disambiguation is structural rather than a new field: **an edge whose target is a `service` node is a requirement; every other edge is control flow.** Control flow never enters a service node, and a requirement never advances the run. Lint rules follow: every service node names a process that exists in the Procfile, and nothing routes control into one.

### Adapter

```go
type Manager interface {
	Up(ctx context.Context, services []string) error
	Healthy(ctx context.Context, service string) (bool, error)
	Status(ctx context.Context) ([]ProcessState, error)
	Down(ctx context.Context) error
}
```

**Recommendation on `Healthy`: the adapter owns what healthy means, and answers with the best its orchestrator can give.** No engine branch, no per-service configuration, no check command in the workflow.

For Overmind that is: `status` reports the process running, and — because the adapter is the thing that assigned `OVERMIND_PORT` — if the service was given a port, that port accepts a connection. The adapter knows the port, so this costs nothing and keeps the readiness signal that `.tractor/run` had. A worker with no port is healthy when it is running, which is all anyone can say about it.

A future Compose adapter would answer from the container's own `healthcheck`, which is better, and the interface does not change. Getting this exactly right matters much less than getting it working; if a real false pass shows up that a per-service check command would have caught, add it then.

## What Overmind 2.5.1 actually does

Probed directly while designing this, not read from docs.

- `overmind stop web` then `overmind restart web` toggles a single process in place — `running` → `dead` → `running` with a fresh PID, siblings untouched. Per-service lifecycle works.
- **Without `--any-can-die`, one process going away quits the whole instance** — the socket is deleted and the siblings die with it. Reproduced on a crash and again on an ordinary `stop`, despite the help text saying `stop` does not quit Overmind. The flag is mandatory for this design, not a preference.
- `--processes` selects which processes launch, but only at `start`; a process not launched then cannot be introduced later. So the adapter starts the whole Procfile once and toggles from there.
- There is no health check anywhere in Overmind or in a Procfile. `status` reports that a PID is alive.
- Everything runs inside a tmux session, so tmux becomes a hard dependency for any repository that declares a service.

## Also removes `TRACTOR_URL`

`.tractor/run` exported `PORT` and `TRACTOR_URL`. `TRACTOR_URL` should never have existed: it hardcodes HTTP, loopback, no base path, and that the address is a URL at all. The workflow decides what the port means.

The engine picks a free base port and passes `OVERMIND_PORT` and `--port-step`. Each process reads `$PORT` from its own environment. Nothing else is exported.

## Observability

Reading a process table on every node entry is what makes the state askable. The timeline gains an event per node entry naming which services were required and what state they were found in, and `tractor status` grows a services block:

```
services   overmind · Procfile · base port 55810
  web       pid 69685   running    required by implement, sprints
  worker    pid 69608   running    required by sprints
  checked   entering `sprints` at stage 11 — web accepted in 0.4s
```

## What this does not fix

A service that is only ever ensured *up* keeps serving whatever it booted with. `status` reports it running the whole time, because it is. A loop check at stage 12 then validates code nobody wrote — the false pass this whole area exists to prevent, arriving by a different road.

Restarting when the tree changes is what closes it, and that is deliberately out of scope here. This is a considered trade, not an oversight: the requirement primitive alone is already far better than 127 stages against nothing, and it is the smaller thing to build first. Workflows adopting this early should either have a Procfile process that builds before it serves, or a service that reads the tree live.

## Scope

Not building process orchestration. The engine holds an adapter over one manager and never supervises a process itself. If the six verbs above start growing, that is the signal to stop.

## Related

- #70 — supersedes it. The fix filed there was to export `TRACTOR_URL` into agent turns; the answer is instead that a required service is alive across the nodes that require it, and `TRACTOR_URL` goes away. Recommend closing #70 when this lands.
- #65 item 6 — the service scope question, answered here.
- #67 — an agent can repair `.tractor/run` mid-run. Moot once the file is gone, though the same hole exists for a Procfile.
- #60 — what this replaces.

