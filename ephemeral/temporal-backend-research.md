# Practical findings for the experiment

Research findings are implementation input, not additional product requirements.

## Temporal

Installed CLI: /opt/homebrew/bin/temporal 1.9.1, embedded server 1.32.0.
The host CLI dev server can save Docker VM memory (Docker currently has 2 GB).
SDK go.temporal.io/sdk v1.49.0 supports standalone activities; ordinary Go need
not be translated to a Temporal Workflow. Use ExecuteActivity, with a unique
activity ID and MaximumAttempts:1 for mutating operations. Cancelling a Get
wait does not cancel its activity: request cancellation explicitly on an
independent context, heartbeat the activity, and propagate cancellation to the
child process. A unique queue routes work but does not itself establish worker
or filesystem ownership. Do not set the whole worker to concurrency one if
that prevents supervisor or steering calls during an agent turn.

Sources: https://docs.temporal.io/develop/go/activities/standalone-activities
and the pinned SDK source.

## Harness and filesystem

The smallest seam appears to be a HarnessAdapter proxy in the ordinary Go
controller and real existing harness adapters in the worker. Planner and
supervisor agents already use that interface. A separate steering/control
path must remain responsive during long turns. Preserve the same absolute
worktree and run-directory paths in mounts: prompts include absolute artifact
and supervisor-history paths. A persistent native state directory does not
reconstruct the current adapter's in-memory maps after worker process death;
do not claim transparent crash resume.

The existing observation contract in docs/definition-of-done.md says observation
may degrade but must not override successful execution. Surface delivery gaps;
do not turn a successfully completed command/turn into execution failure solely
because recording failed. This first experiment keeps one controller/web owner
for canonical roll-ups while Postgres retains worker events.

## Credentials

Official Codex CI guidance explicitly warns against concurrent runners sharing
mutable auth.json: another refresh can invalidate a runner's token lineage.
Use a dedicated writable state directory owned by one live worker container,
preserve refreshed writes, and never reseed it at every startup. Fail clearly
on conflicting ownership rather than inventing a refresh scheduler. API-key
mode avoids that OAuth lineage issue. No copying this machine's provider state
automatically and no secret bytes in Temporal payloads, bootstrap rows, events,
images or Git. Operator-supplied secret files/references are sufficient.
Claude accepts ANTHROPIC_API_KEY or CLAUDE_CODE_OAUTH_TOKEN; choose one mode.

Sources: https://learn.chatgpt.com/docs/auth/ci-cd-auth,
https://learn.chatgpt.com/docs/auth,
https://code.claude.com/docs/en/authentication.

## User's execution preference

Use Gimbal to implement bounded outcomes with Sonnet planning/validation and
Luna coding. Get the orchestration path functioning before polishing the seam;
then perform a second refactor pass. Focus on running built-in workflows and
useful unit tests. No extensive proof machinery or unrelated UI changes.

## Worktree mount detail

This experiment itself uses a Git worktree. Its .git file points to metadata in
the parent checkout's .git/worktrees directory. Mounting only the worktree leaves
Git broken inside a worker even when its source files are present. The backend's
mount configuration/startup must make the required Git metadata accessible at the
referenced paths for built-in coding workflows. Do not broadly mount the user's
home or provider-login directories as a shortcut.
