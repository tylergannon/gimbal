# Assessment of Gimbal PR 444 against the plugin goal

**444 materially advances execution and cleanup. It makes limited progress on plugin extensibility and does not yet establish the requested distributed topology. Keep the useful implementation; finish the missing connections instead of treating the narrower delivery plan as the whole goal.**

Reviewed implementation: Gimbal `1e61d1ea46cedea997ed575d74bce7cd76f0ccac`. The present review inspected source and ran focused checks. Live provider/UI claims below come from the earlier delivery session and PR, not a new provider run during this review.

[Goal and proposed next delivery](backend-goal-and-next-delivery-2026-10-10.md)

## What the plugin provides

The native loader looks up two ordinary Go functions:

```go
Start(context.Context, json.RawMessage) (string, error)
Stop(context.Context, json.RawMessage) error
```

`Start` returns a task-daemon HTTP URL. The caller passes that URL as `--task-endpoint`. The Docker implementation provisions a container and waits for `/health`; `Stop` removes the configured container name. This is a small, understandable provisioning interface. It avoids reflection-driven composition and keeps Docker out of the workflow runtime.

The endpoint yields a real execution interface: `/tasks/session`, `turn`, `process`, `poll`, `interrupt`, `steer`, `close`, `cleanup`, and `state`. Gimbal supplies the client and handlers. Sessions include fork through a parent identity; process requests distinguish commands and resident services. This covers much more than provisioning, but the plugin itself does not return an execution implementation, observation source, artifact accessor, or workflow-launch capability.

A different backend could implement the same HTTP protocol, including a facade over queue workers. That is legitimate protocol-based extensibility; absence of a Go `Backend` interface alone is not a defect. However, today's protocol and consumer assumptions require the fixed polling path and shared absolute filesystem paths. Its types live in `internal/effects`, and its implementation is not yet a supported external backend contract. Replacing these assumptions requires core work, not merely another provisioning plugin.

Sources: [plugin loader](https://github.com/tylergannon/gimbal/blob/1e61d1ea46cedea997ed575d74bce7cd76f0ccac/cmd/gimbal/daemon.go#L50), [Docker plugin](https://github.com/tylergannon/gimbal/blob/1e61d1ea46cedea997ed575d74bce7cd76f0ccac/plugins/docker/main.go#L30), [effect messages and client](https://github.com/tylergannon/gimbal/blob/1e61d1ea46cedea997ed575d74bce7cd76f0ccac/internal/effects/client.go#L14), [HTTP handlers](https://github.com/tylergannon/gimbal/blob/1e61d1ea46cedea997ed575d74bce7cd76f0ccac/internal/taskdaemon/http.go#L20).

## Progress toward the requested system

| Capability | What exists after 444 | Assessment |
| --- | --- | --- |
| Ordinary Go and shared agent semantics | The original workflow executes; context, typed decoding, validation/re-asks, supervision, and workflow attribution remain in Gimbal. Native harness operations cross the task boundary. | Substantial reusable progress. No second workflow program or new language. |
| Independent workflow lifetime | 443 established separate local workflow processes. 444 retains owner routing and cleans up remote effects after workflow death. Earlier live work survived central restart and regained steering. | Preserve this; distinguish 443's foundation from 444's additions. |
| Remote workflow and task placement | `Project.Start` executes the local binary as `run-process`. `gimbal daemon` exposes task handlers only. One task endpoint is selected per run. | Remote tasks work. Central-to-remote-daemon workflow launch and multiple task placements within a run are not supplied. The requested A/workflow/B topology is incomplete. |
| Independently built plugins | A manually selected `.so` supplies environment Start/Stop. Built-in workflow imports, adapters, commands, and catalog remain. | No startup discovery across configured directories, external workflow registration, or plugin-only workflow edit cycle. These were deferred; they remain central to the goal. |
| Observation choice | Task polling returns both native events and terminal result. Workflow records are written locally and followed by central. | Useful working delivery, but neither event path is independently selectable. The workflow is an event consumer for supervision, not just an event producer for the UI. |
| Workspace and artifact access | Task daemon writes capture files created at the workflow location; central opens local run files. | Works for the shared/bind-mounted configuration. It does not establish backend-owned transfer or retrieval across independent filesystems. |
| Service fulfillment | `Service` forwards a zsh command with a `Service` flag. The exercise communicates its address through a workspace file. Scope cleanup releases it. | Real service use and anti-claim. No backend-managed resource requirement or returned connection-information contract yet. |
| Human oversight | Existing graph and agent activity are reused; cleanup retry and full command-output links were added and observed during delivery. | Useful improvement. There is no first-class environment/host identity in the observation model; the UI cannot yet explain distributed placement and ownership directly. Remote-workflow interviews and loop controls were not demonstrated. |
| Lean remote daemon | Task mode starts only its task server, but the command still belongs to the binary importing the web application and embedded console. | No runtime-only build boundary or measured lean-equivalence claim is established by this review. Do not infer resident memory from binary size alone. |

Source anchors: [local workflow launch](https://github.com/tylergannon/gimbal/blob/1e61d1ea46cedea997ed575d74bce7cd76f0ccac/internal/host/host.go#L246), [fixed remote harness selection](https://github.com/tylergannon/gimbal/blob/1e61d1ea46cedea997ed575d74bce7cd76f0ccac/internal/invocation/invocation.go#L57), [built-in inventory](https://github.com/tylergannon/gimbal/blob/1e61d1ea46cedea997ed575d74bce7cd76f0ccac/internal/builtin/workflows.go#L19), [shared capture files](https://github.com/tylergannon/gimbal/blob/1e61d1ea46cedea997ed575d74bce7cd76f0ccac/remote_execution.go#L57), [artifact reads](https://github.com/tylergannon/gimbal/blob/1e61d1ea46cedea997ed575d74bce7cd76f0ccac/internal/observation/artifacts.go#L41), [service meaning](https://github.com/tylergannon/gimbal/blob/1e61d1ea46cedea997ed575d74bce7cd76f0ccac/service.go#L25).

The narrower plan explains why some work was deferred. It does not establish that the plugin/backend goal was achieved. The first-stride discussion explicitly included workflow placement as well as task placement; the implementation reaches only the latter remotely.

## Two concrete implementation defects

1. **Environment release does not verify ownership.** `Stop` runs `docker rm -f <configured name>` without a returned resource identity or ownership check. A temporary test substituted a Docker executable: Start rejected an already-used name, then Stop still issued removal for that name. This is a stubbed command-boundary demonstration, not deletion of a real unrelated container. The contract should release the environment actually owned by this provisioning operation. [Source](https://github.com/tylergannon/gimbal/blob/1e61d1ea46cedea997ed575d74bce7cd76f0ccac/plugins/docker/main.go#L70).
2. **Completed task payloads have no retirement path.** Cleanup marks tasks released but does not remove tasks, events, results, or scope tombstones. A temporary package check completed a turn and root cleanup: pending resources were zero, but one task, its event, and its result remained. Source contains no expiry/removal path, so repeated runs accumulate retained data for the daemon's lifetime. This needs bounded retention consistent with remaining readers, not a new history system. [Event retention](https://github.com/tylergannon/gimbal/blob/1e61d1ea46cedea997ed575d74bce7cd76f0ccac/internal/taskdaemon/daemon.go#L151), [cleanup](https://github.com/tylergannon/gimbal/blob/1e61d1ea46cedea997ed575d74bce7cd76f0ccac/internal/taskdaemon/daemon.go#L248).

`go test ./internal/taskdaemon ./internal/runner ./plugins/docker` passed at the reviewed revision; the Docker plugin had no existing tests. Both temporary review checks passed and were removed. No runtime implementation was changed.

## Preserve the decisions that keep this tractable

Keep workflow code ordinary and typed. Keep operation identity, scope ownership, authoritative results, and human control meanings stable. A backend may choose transport, queue, worker arrangement, and supporting infrastructure; Gimbal need not implement every such choice or guarantee arbitrary compatibility among them.

Separate observation delivery next, including task-to-workflow consumption. Use the existing concrete operations as the starting point for the backend boundary. Complete one external workflow/backend integration before adding infrastructure varieties. Remote workflow launch, artifact access, service fulfillment, and plugin discovery must eventually compose into that single usable path; a loader test or another successful Docker task alone cannot establish completion.

Do not revive the discarded compiler, Temporal requirement, mutation journals, fencing, guardians, recovery system, auth work, or exhaustive compatibility framework. The two concrete defects above are repairs to current behavior, not permission to revive that machinery.

## History needed by the next chat

Read the goal file first. These chats explain the decisions if a specific question remains:

- [Find promise reevaluation report](codex://threads/01a11ec8-34ea-7da1-a7b5-75ac2ceef795): product priorities included scoped context, ordinary Go decisions, bring-your-own-backend, human-agent workflow editing, and a frontend for observation/control. Remote execution was desired, while project portability was the immediate concern.
- [Explore project-owned Gimbal flows](codex://threads/01a1226b-3788-7083-9afd-996b32b537a2): independent project builds, native Go libraries, multiple plugin directories, non-generic registration, schema/validation, and a dynamic fixed frontend. [Issue 439](https://github.com/tylergannon/gimbal/issues/439) records that outcome; signature sketches are not a frozen API.
- [Clarify pluggable backend lifecycle](codex://threads/01a12320-6e58-78b2-bfd6-0e56572ef407): backend choice must not require a per-workflow compiler or impose backend-specific reliability infrastructure in shared Gimbal. Temporal was an example and later rejected, not the product target.
- [Assess recovery plan scope](codex://threads/01a1273b-8f9f-7b61-8bb1-7c0c843b1c51): preserve existing observations across processes; brief lag is acceptable. Do not turn completeness into endless planning or additional reliability requirements.
- [Find the next Gimbal step](codex://threads/01a1280d-f6a9-7273-91a4-cca55354ae91): central, daemon A, independent workflow, and optional task daemon B; no extra Gimbal process per task. Services and event delivery are backend responsibilities to resolve against concrete use.
- [Review thread and suggest direction](codex://threads/01a1285b-beef-7763-822c-b0bdbd5ced29): bounded local/Docker plan and implementation; message-bus separation explicitly next. [Delivery readout](local-docker-execution-delivery-2026-10-10.md) retains named checks and their revision.

Do not adopt old assistant proposals as user requirements. This assessment and goal handoff are written outcomes; implementation has not been authorized by the present review request.
