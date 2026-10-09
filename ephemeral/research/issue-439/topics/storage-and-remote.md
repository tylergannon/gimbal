# Storage and remote execution

[Back to TL;DR](../proposal.md)

**The server persists ownership and its archive; each runner persists its execution records.** Remote operation must move the data through the protocol rather than assume shared paths.

## File ownership

| Owner | Durable material |
| --- | --- |
| Server instance | Ownership records, stop intent, run index, received event prefixes, saved build/design metadata. |
| Runner | `events.jsonl`, existing run/session logs, `runner.json`, the launcher-created `process.lock` handed to the runner, and stdout/stderr in its assigned run directory. |
| Shared immutable context store | Content-addressed values referenced by local runs. Mutable materialization caches stay local to their consumers. |

The server's received journal is at `projects/<project>/runs/<run>/events.jsonl` under its instance directory, beside the [ownership record](ownership.md#record-ownership-before-launch). This archive is distinct from the runner's source journal. Server restart replays its own prefix and retrieves any missing suffix. Retained ownership is not inferred from either journal's last status.

The launcher supplies context-store and cache paths so the runner constructs `ContextAccess` locally. Immutable content-addressed objects may be shared by local runs; mutable run files remain exclusively owned. Existing local artifact serving can keep `ReadObject` and its check that the requested object belongs to that run's references.

## Other machines and containers

Remote runners use their own object storage and agent-visible materialization paths. Artifact requests transfer referenced objects into the server archive; a path on the runner is not a path the server can open.

Runner storage must survive container exit until journal and artifact import completes. Remote launching, archive transfer completion, and deletion policy need implementation before claiming remote support. Separate processes and sockets merely preserve the opportunity.

Reachable authenticated HTTPS or a tunnel can carry the same HTTP requests and event stream. Outbound-only networks may need a runner-initiated tunnel. That changes connection establishment while preserving the run identities, journal positions, controls, and durable server ownership.

Mixed-version compatibility and seamless application upgrades remain later work. Ordinary server restart ownership and cancellation are required now. Reject incompatible protocols explicitly and retain the affected run in the ownership registry with its control limitation visible.
