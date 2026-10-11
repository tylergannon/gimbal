# Gimbal plugin and backend goal

**Install Gimbal once. Build project workflows and execution backends independently. Load them at startup, run ordinary Go workflows in the selected environment, and retain clear visualization and human controls.** Changing a plugin must not require rebuilding Gimbal or its frontend.

Gimbal owns scoped context, typed results, Go control flow, validation, supervision, and workflow meaning. Backends own their placement, provisioning, and transport choices.

## The state we need

- Project-owned native Go plugins provide workflow descriptions, schemas, validation, invocation, roles, and diagrams. The installed CLI/frontend consume these dynamically. Plugin development uses Go tooling; rebuild and restart is sufficient.
- Central can ask daemon A to spawn an independent workflow process; that workflow can use daemon B for tasks. A and B may coincide, and central can supply daemon capability. Tasks require no extra Gimbal worker-process layer.
- Observation delivery is replaceable while task results remain authoritative. Workflow and central receive the information each needs; context, workspaces, and actual artifact bytes remain accessible.
- The environment fulfills service needs, returns usable connection information, and releases scope-owned claims.
- Human observation, steering, answers, cancellation, and cleanup retry work across those boundaries. Central restart preserves independent work and restores control by identity.

## What 444 establishes

[PR #444](https://github.com/tylergannon/gimbal/pull/444), merged as `1e61d1ea`, delivers remote agent operations, commands, foreground services, and scope cleanup over HTTP. Its reported live evidence covers local/Docker execution, steering, cancellation isolation, central restart, workflow-death cleanup, cleanup retry, and command-output access.

**That is useful execution infrastructure. It is not yet the plugin/backend system above.** The plugin only supplies `Start(config) -> endpoint` and `Stop(config)`. Workflows remain built in and launch locally. The Docker daemon executes tasks, not remote workflow launches. Observations/artifacts require local or shared files. Service fulfillment remains a shell command. Startup discovery and external workflow registration are absent.

## What needs to happen

1. **Separate observation delivery next**, as already agreed. Include task-to-workflow events and workflow-to-central observations. Establish the remote A/workflow/B path with controls and artifacts; changing only browser SSE is insufficient.
2. **Connect plugin registration to invocation and execution.** External workflow descriptions and validated invocation must work in the installed CLI/frontend. An independently built backend must supply the execution path without core edits. Build from the existing operations rather than inventing a universal interface.
3. **Complete services and controls through that path.** Demonstrate useful service fulfillment, scope release, understandable ownership, and affected human controls. Repair the two concrete ownership/retention defects in the [assessment](backend-interface-assessment-2026-10-10.md).

**Completion:** an external project builds its workflow and backend, loads them into installed Gimbal, executes across the chosen locations, uses a service, remains observable/steerable through central restart, cancels one run independently, and retrieves results. Changing workflow inputs or output types requires only rebuilding the plugin and restarting. Two different workflow input types demonstrate that the host does not depend on project Go types.

Do not add crash recovery, auth, Temporal, another infrastructure backend, journals, fencing, guardians, a general resource system, or a new workflow language. This describes outcomes and proposed sequencing; no API or implementation is approved by this review.

[Assessment and source history](backend-interface-assessment-2026-10-10.md) · [Prior delivery evidence](local-docker-execution-delivery-2026-10-10.md)

## Current delivery scope correction

The user subsequently instructed: "leave pause for human input out of it for the time being." This delivery and its proof do not require an interview or human answer. Existing controls can remain, but the composed workflow must run without waiting for human input. Observation, steering, cancellation, central restart, services and result access remain in scope.

## JSON boundary clarification

The user requires no redundant marshal/unmarshal round trips: the workflow supplies its Go-derived JSON contract to the harness adapter; the adapter validates agent JSON before returning it to the workflow, which converts it into its concrete Go result. Preserve the raw JSON payload across the backend transport; do not reconstruct the workflow's result type at intermediate layers and marshal it again. Transport envelopes are separate from the result payload.
