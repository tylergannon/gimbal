---
name: gimble-runs
description: >
  Run, observe, and steer Gimble workflows from the CLI. Use when an agent
  needs to find a running workflow, follow its activity, or send instructions
  to one of its sessions or loop planners. For writing Go workflows, use
  gimble-workflows.
---

# Gimble runs

Use the installed `gimble` binary. `gimble run --help` lists the built-in
workflows; each workflow's `--help` lists its inputs and model flags.

```sh
gimble run review --work-dir /abs/repository --goal "Review the parser changes" --no-web
```

The workflow command stays running until its work finishes. Keep that process
alive while observing or steering it from another shell. Omit `--no-web` to
also open the browser listener; use `--port 0` to choose an available port.

## Observe and target

```sh
gimble runs --work-dir /abs/repository
gimble watch <run-id> --work-dir /abs/repository
```

Discovery uses that repository's `.gimble` directory and contacts its running
instances. Use the returned run ID. The watch stream contains the current
snapshot and subsequent changes, including sessions and turns. A running
run's turn with no end time identifies an active session. Copy its session
ID exactly; a role name or a turn ID is not a session ID.

```sh
gimble steer <run-id> --work-dir /abs/repository --session <session-id> "Investigate the parser test before changing the frontend."
gimble steer <run-id> --work-dir /abs/repository --loop <loop-scope-id> "Prioritize completing the CLI."
```

Session steering reaches that session's active turn. `landed: false` means
the message was dropped because no turn received it; it is not saved for a
future turn. Loop steering queues the message for the next planning decision.
Neither acknowledgment proves the agent followed the instruction. Continue
observing to establish the effect.

Run records remain under `.gimble/runs/<run-id>/` after the process exits,
but that process can no longer receive steering. Use `gimble` in the repository
to serve the saved runs in the browser, or read the tables and transcripts
as described in `gimble-workflows`.

Report the run and target session or loop, the delivery result, and what you
actually observed afterward. Keep undemonstrated effects explicit.
