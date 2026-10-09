# Storage and remote execution

[Back to TL;DR](../proposal.md)

**The server persists ownership; the backend owns one logical event history per run.** Remote operation must move the data through the protocol rather than assume shared paths.

## File ownership

| Owner | Durable material |
| --- | --- |
| Server instance | Ownership records, stop intent, run index, received event prefixes, saved build/design metadata. |
| Backend execution | Authoritative event history and artifact objects. The local implementation uses `events.jsonl`, `runner.json`, the launcher-created `process.lock`, and stdout/stderr in its assigned run directory. |
| Shared immutable context store | Content-addressed values referenced by local runs. Mutable materialization caches stay local to their consumers. |

When a replica is needed, the server stores it at `projects/<project>/runs/<run>/events.jsonl` under its instance directory, beside the [ownership record](ownership.md#record-ownership-before-launch). When the authoritative journal is directly accessible, retain a reference instead of a second byte copy. Server restart replays the available committed prefix and retrieves the missing suffix through the backend. Retained ownership is not inferred from a journal's last status. UI snapshots and deltas are rebuildable projections, not additional event authorities.

The launcher supplies context-store and cache paths so the runner constructs `ContextAccess` locally. Immutable content-addressed objects may be shared by local runs; mutable run files remain exclusively owned. Existing local artifact serving can keep `ReadObject` and its check that the requested object belongs to that run's references.

## Other machines and containers

Compiled distributed execution follows the [backend contract](backend-contract.md). A runner process, JSONL file, or common filesystem is an implementation choice, not the public definition of a run.

Remote runners use their own object storage and agent-visible materialization paths. Artifact requests transfer referenced objects into the server archive; a path on the runner is not a path the server can open.

Runner storage must survive container exit until journal and artifact import completes. Remote launching, archive transfer completion, and deletion policy need implementation before claiming remote support. Separate processes and sockets merely preserve the opportunity.

Reachable authenticated HTTPS or a tunnel can carry the same HTTP requests and event stream. Outbound-only networks may need a runner-initiated tunnel. That changes connection establishment while preserving the run identities, journal positions, controls, and durable server ownership.

Mixed-version compatibility and seamless application upgrades remain later work. Ordinary server restart ownership and cancellation are required now. Reject incompatible protocols explicitly and retain the affected run in the ownership registry with its control limitation visible.
