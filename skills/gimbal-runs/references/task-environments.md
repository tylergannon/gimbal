# Standalone workflow and task daemons

A workflow runs as ordinary Go in its host. `--task-endpoint` sends its
agent sessions, commands, and foreground services to a selected task daemon.
The workflow retains its observations and controls. Results remain authoritative
when event delivery is unavailable.

Start a daemon with `gimbal daemon --listen 127.0.0.1:8081`. Central can also
include one with `--task-listen 127.0.0.1:8081`. Use a separate daemon when its
work must survive central restarting. Configure providers in the task daemon's
environment. Use loopback or a private network: the endpoint accepts trusted
execution requests.

Add `--task-endpoint http://127.0.0.1:8081` to a workflow invocation. The daemon
uses its own workspace and `--workdir`. Context and command artifacts move as
bytes; the workflow and task daemon need no shared run directory. Provision
repositories explicitly. `--plugin-dir` supplies compatible native workflow
plugins and `--workspace-root` restricts admitted workflow projects. A remote
workflow can use `--workflow-endpoint` and `--workflow-project` together with a
separate `--task-endpoint`.

Standalone task daemons have a private in-memory secrets backend. Producers
publish declared string outputs to scoped stores. Consumers declare required
stores and names; programs receive the visible store values as environment
variables, and agents receive an operation-scoped callback. Values do not
persist across daemon restart or cross daemon boundaries. Foreground services
signal readiness by atomically writing ENV_FILE, even when it is empty. Finite
producers publish after successful exit.

Scope cleanup closes owned sessions and stops owned commands and services,
preserving enclosing scopes and other runs. Failed cleanup remains visible and
retryable with `gimbal cancel RUN_ID` or the console's **Retry cleanup** control.
The daemon retains session/task indexes and reuses the executor's recorded
operations after restart. Unavailable native outcomes are reported without
blind redispatch. This does not establish SERVICE restoration or restore the
in-memory secrets described above.

Named task environments use `OrchestratorBackend` and a messaging route instead
of direct task endpoints. See the automatic named-environment example in the
[parent skill](../SKILL.md). Orchestration starts a scope-assigned daemon, waits
for readiness, and releases it after native cleanup. Infrastructure setup belongs
to that backend's configuration.

Published CLIs use `-trimpath`. Build their plugins with that flag and the
released source workspace described in
[distribution](../../gimbal-release/references/distribution.md#plugin-builds).

