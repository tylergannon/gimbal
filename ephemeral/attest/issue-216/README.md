# Issue 216 attestation

Head: `e3454ad` (`Add RunCommand and record each command in its scope (#216)`).
Program: `ephemeral/attest/issue-216/main.go`. Model: Codex `gpt-5.6-luna`
through the shared app-server daemon, one turn.

```sh
go run ./ephemeral/attest/issue-216          # the run: run.txt, run-stderr.txt
go run ./ephemeral/attest/issue-216 -read    # a new process reads it back: read.txt
```

Run `01M2KE9AADWNR8EQDAG53ERYRN.issue-216`, 7.0 s, no error. Its durable
local record is `ephemeral/attest/issue-216/.gimble/runs/01M2KE9AADWNR8EQDAG53ERYRN.issue-216`
(not committed).

Claim: each outcome is its own record, in the scope it ran in.
`commands.json` and the observation endpoint hold, for the four outcomes:

| id | scope | outcome |
| --- | --- | --- |
| `version.1` | root | `go version`, exit 0, stdout `go version go1.27.1 darwin/arm64` |
| `retry.1/check.1` | `retry.1` | exit 3, stderr `not ready: … has no file named ready`, no error |
| `missing.1` | root | exit -1, error `exec: "gimble-no-such-tool": executable file not found in $PATH`, not interrupted |
| `sleep.1` | root | exit -1, error `context deadline exceeded`, interrupted, 969 ms |

Claim: a failed command and its retry are two attempts, and parallel
commands stay distinct. `retry.1/check.1` exited 3 and `retry.1/check.2`
exited 0 in the same scope. `attempts.1/attempt.1/check.1` exited 1 while
`attempts.1/attempt.2/check.1` exited 0; neither attempt scope, the group,
nor the run is marked failed (`scopes.json`: every scope `ended` with no
error; `run.json`: `completed`).

Claim: ordering and placement sit alongside agent turns. `records.txt` is
the run log's command, scope, and turn records in `seq` order: each
`command_started`/`command_ended` pair is placed on its scope key, the two
parallel attempts interleave (seq 18 to 21), and the agent's
`reader.1/turn.1` follows at seq 26.

Claim: an agent inspects the output. `gpt-5.6-luna` was given what the
commands returned and answered (run.txt):

```text
Go version: go1.27.1 (darwin/arm64).
First check: no file named `ready`; second check passed.
Missing tool did not run because it was not found in `$PATH`.
Sleep did not finish because its one-second context deadline expired.
```

Claim: observation serves the records live and finished, and a restart
reads the same facts. `GET /api/runs/<id>` returned all seven commands
while the run was still `running` (before the agent's turn), again from the
same process after it `completed`, and from a new process started with
`-read` (`read.txt`), which also found the endpoint's rows identical to the
`commands.json` the run wrote: `true`.

A command only constructed produces no record: RunCommand is the only way
a workflow runs a command, and the record is written by that call.
