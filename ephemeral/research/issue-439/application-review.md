# Application architecture review: issue 439 proposal (fresh read)

Reviewer: Claude Fable 5.1, fresh session, 2026-10-09.

## Review target

The finished application described by `ephemeral/research/issue-439/proposal.md`
and its six topic files (`ownership.md`, `interchange.md`, `lean-runners.md`,
`workflow-discovery.md`, `storage-and-remote.md`, `delivery-and-review.md`) at
commit `894f71a4`, assessed as a design: is it idiomatic, are the seams
sound and testable, and what are the material risks.

Authoritative sources used: `requirements.md`, `issue.json` (issue 439 body),
`AGENTS.md`, `docs/definition-of-done.md`, `go doc -all .`, and the current
source at `37f8e3a9` (the proposal's audit baseline, identical for the files
below).

Operating constraints from the caller: read-only except this artifact; write
to the caller-supplied path. The adversarial-review skill expects artifacts
under `ephemeral/reviews/`; the caller's explicit path and "do not edit any
other files" were followed instead, and no second copy was written. No caller
narrowing of subject matter was present.

## Evidence inspected

- `internal/host/host.go` (project admission, `owner.lock` via `flock`,
  `Project.Run`/`Start`/`OpenCompiledRun`, in-process controller hooks)
- `internal/host/compiled.go` (`compiledController`, hosted cancellation)
- `internal/live/live.go` (`Controller`, `Runs` table, retained cleanup)
- `internal/observation/{durable,registry,replay,subscribe}.go` (snapshot +
  delta journal with per-delta fsync, `Join(stream, position)`, bounded
  subscriber backlog, `open()` rebuilding tables from `run.jsonl`)
- `run.go` lines 30-75 and 505-545 (graph registration, `CancelHostedRun`);
  `event_persistence.go` (writers never sync)
- `web/control.go`, `web/submit.go`, `web/server.go`,
  `web/src/routes/workflow_start.go` (socket control, instance selection,
  `Project.Start` with a compiled closure)
- `cmd/gimbal/main.go` and `cmd/gimbal/runs.go` (`--instance-dir` default
  `.gimbal`, discovery under `<project>/.gimbal/control/`)
- `opencode/process_unix.go`, `agy/process_unix.go`,
  `service_process_unix.go`, `internal/pi/shell/process_unix.go` (every agent
  and service child is spawned with `Setpgid`; opencode already verifies PID
  identity with `ps lstart`)
- `go list -deps`: root `gimbal` has 239 packages and no skgo/web; `cmd/gimbal`
  has 519
- Prior rounds 01-09 under `ephemeral/reviews/`, to avoid re-litigating
  resolved points

## Short answers

**Is it idiomatic?** Yes, for this codebase and for Go. HTTP over Unix
sockets, newline-delimited JSON, `flock` lifetime markers, resumable
`(stream, position)` cursors, bounded subscriber buffers, and one process per
unit of ownership are all already how this tree works. The server-opens-the-
subscription choice is the right one for the stated ownership requirement:
the server is the only party that must know where every run is, so it is the
one that dials. The runner stays dumb: it serves its journal and a handful of
controls. The journal doubling as the backpressure queue is the cleanest part
of the design.

**Are the seams sound, isolable, validatable?** Mostly. The runner seam is
excellent: it is `gimbal.Run` plus a recorder plus a small HTTP handler, and
it can be exercised with `curl` against a socket. The server seam (ownership
registry + launcher + projection) is testable with any fake HTTP runner. Two
seams are weaker: ownership identity is keyed on a directory that defaults to
the current working directory (finding 1), and "kill" is designed as
cooperative cancellation with an open-ended waiting state (finding 2).

**Most significant risks.** In order: a runner that is alive but owned by a
registry the new server is not reading; a hung runner or orphaned agent
children that cooperative cancellation cannot reach; and storage multiplicity
that will make every later change to recording twice as expensive. The first
two violate the user's clarified requirement that independent processes never
become unmanaged. All three have cheap mitigations.

## Findings

### 1. Issue: a still-running runner becomes an unmanaged "direct run" whenever the server starts with a different instance directory

**Where.** `topics/ownership.md:9` keys ownership on an `instance-id` file
under `--instance-dir`. `topics/ownership.md:45` has the replacement server
scan `<project>/.gimbal/runs/`, skip IDs in *its* ownership registry, and
import everything else as direct-run history. `cmd/gimbal/main.go:231` and
every generated command default `--instance-dir` to `.gimbal` relative to the
current directory.

**Failure.** Server A runs from `/work/proj` (instance dir `/work/proj/.gimbal`)
and launches runners. A is stopped; the runners keep going, as designed
(`ownership.md:29`). Tyler restarts from `/work` with `--project /work/proj`,
or with a fresh `GIMBAL_INSTANCE_DIR`. Server B gets a new `instance-id` and an
empty registry, the project `owner.lock` is free, so B admits the project,
scans `runs/`, finds no owned IDs, and imports the live runners' directories
as finished direct runs. They are now invisible as live, uncontrollable, and
`cancel all` on B does nothing to them. That is precisely the state the
requirement forbids. Today's `open()` (`internal/observation/replay.go`) also
writes `*.json` tables and `observation.json` into the directory it imports,
which the design says is exclusively runner-owned.

**Mitigation (no new infrastructure).** `runner.json` already names the owner
instance. The import scan must treat a directory holding `runner.json` whose
owner is not this instance as foreign-owned: list it, show the owner id and
endpoint, refuse to import it as history while its `process.lock` is held,
and never write into it. A unit test in the registry package: create a run
dir with `runner.json` for owner X and a held lock, admit as owner Y, assert
the row is "foreign-owned, live" and the directory is byte-identical after
admission. Consider also making the instance directory absolute and warning
when `instance-id` is created fresh while the project already has runner
directories naming another owner.

### 2. Issue: the requirement says "kill"; the design delivers cooperative cancellation with an unbounded waiting state, and agent children are outside the runner's process group

**Where.** `topics/ownership.md:21` ("A timeout alone does not prove
death"), `:27` (`unreachable` is never resolved by the server), `:41`
("Force-killing an unresponsive process is separate ... and must not signal a
reused PID"). `requirements.md`: "so it can kill one run or all its runs".

**Failure.** A runner wedged in an adapter call (or whose socket handler is
stuck) holds `process.lock`, answers nothing, and keeps its saved stop intent
forever. The design names force-kill but gives it no operation, no identity
check, and no place. Separately, every agent and service child is started
with `Setpgid` (`opencode/process_unix.go:44`, `agy/process_unix.go:12`,
`service_process_unix.go:12`, `internal/pi/shell/process_unix.go:19`), so
"runners use their own OS session/process group" (`ownership.md:29`) puts only
the runner itself in that group. Killing the runner's group does not reach a
running `claude`, `agy`, or service process, and a runner that crashes leaves
them orphaned with nobody recording that fact. Today that is also true when
the server crashes, so it is not a regression, but the proposal's cancel-all
reporting ("cessation confirmed") cannot be truthful for that case.

**Mitigation.** Record the runner's PID plus a start-time token in
`runner.json`; the tree already has the verification pattern in
`opencode/process_unix.go` `processIdentity`. Add one control on the server,
"force stop", that verifies the token, signals the runner, and then marks the
row "killed, cleanup unresolved" rather than "ceased". For children, either
start the runner with `Setsid` and have its adapters record each child's PID
and start token in the journal (the `CommandStarted` record already exists for
services; adapters would add one line), so the force path can enumerate and
verify them, or accept the orphan and say so in the per-run report. Test: start
a runner whose workflow runs `sleep 600` as a service, `kill -STOP` the
runner, cancel-all, assert the row stays pending; force-stop, assert the
runner is gone and the row names the still-live child PID.

### 3. Issue: five on-disk recordings of one run; the repository rule is to delete what is replaced

**Where.** `topics/interchange.md:27` adds `events.jsonl` beside the retained
`run.jsonl` and `sessions/*.jsonl` "to supply a single ordered replay source"
while keeping the old logs for "human-readable records and existing per-log
`Seq` meanings". `topics/storage-and-remote.md:15` has the server keep a full
byte copy of every local runner's `events.jsonl`, then
`observation.json` and `observation-deltas.jsonl` on top of that
(`internal/observation/durable.go`). `interchange.md:21` already concedes that
direct reads "remove an IPC hop and a second local copy".

**Impact.** The runner syncs two journals per batch; the server syncs two
more. Every future change to a lifecycle or agent payload touches three
writers and two replayers. `AGENTS.md`: "As simple as possible", "No backwards
compatibility ... delete what is replaced." Keeping `run.jsonl` and the session
logs because of their existing `Seq` semantics is a compatibility reason, not a
requirement. The server-side copy exists only so that local and remote runners
look identical to the server's reducer; location uniformity is a transport
property that the subscription already provides.

**Mitigation (a decision, not software).** Make `events.jsonl` the runner's
only ordered record and have `open()` replay it; session logs, if kept at all,
become a derived human view, not a second authority. For a local runner, let
the server's archive be a reference to the runner's directory (it is under the
project the server admitted) rather than a copy; copy only when the runner's
storage is not server-readable. The subscription remains the one live
boundary either way, so nothing about remote readiness is lost. If Tyler
prefers the copy for crash isolation, say that reason in the proposal; "same
observation boundary" does not require it.

### 4. Nitpick: the observation cursor in `owner.json` duplicates the archive's tail

`topics/ownership.md:11` stores "the observation cursor" in the per-run
ownership record, and `interchange.md:31` advances it after each persisted
batch. That rewrites an atomically-renamed JSON file per batch and creates a
second source of truth for a value that is the last position in the server's
`events.jsonl`. Derive the cursor from the archive on restart and keep
`owner.json` for slow-changing facts (identity, state, intent).

### 5. Nitpick: hosted run identity should ride the context like `Project`, and sockets need a file mode

`topics/workflow-discovery.md:21` says "Revise `Run`/`OpenRun` to accept
assigned identity and directory" without saying how. The tree's idiom is
`gimbal.Project(ctx, dir)`; the assigned run ID and directory belong in the
same context value rather than new parameters or exported names
(`AGENTS.md`: a new exported name only when asked for). Separately, runner
sockets will live under `<project>/.gimbal/runs/<run>/` which `host.go`
creates with mode `0755`; state the socket's mode (`0600`) and its parent's
so a second local user cannot steer or cancel runs. This is not worse than
today's instance socket, but the proposal is the place to fix it.

## What the design already provides, and what still needs a decision

Already provided and well-reasoned:

- Durable ownership recorded before spawn, with the `process.lock`
  inheritance trick closing the spawn-acknowledgment window. The mechanism is
  correct on Linux and macOS (`flock` follows the open file description;
  `os/exec` `ExtraFiles` duplicates it into the child; the child sets
  close-on-exec). Finding 2 only adds the force path it lacks.
- Pull subscriptions with the journal as the queue. Backpressure is on disk,
  memory is bounded, and the runner never waits on an observer. This maps onto
  the existing `Store.Join(stream, position)` and `Store.Lifecycle`/`Store.Event`
  entry points nearly one-to-one, so the server's reducer changes little.
- Cancel-all serialized with launch admission, intents persisted before
  acknowledgment, acceptance reported separately from cessation. This is the
  correct shape for an at-least-once control.
- Control results journaled under a command ID, so a lost HTTP reply is
  recoverable from the stream and steers are never auto-replayed.
- The lean-runner claim is cheap to prove: the root library already links no
  web code (239 packages versus 519 for the CLI), so a `go list -deps`
  assertion in the runner's package settles the dependency half; the memory
  half needs the measurement the proposal already lists.
- The plugin question is answered correctly: a process boundary couples on
  the wire protocol only, whereas a plugin couples on toolchain and every
  shared dependency version.

Needs a decision or remains uncertain:

- Findings 1 and 3 are decisions, not software.
- Remote transport direction. Server-dials-runner needs an inbound-reachable
  listener per runner. The proposal's "runner-initiated tunnel" later reverses
  connection initiation, which an HTTP/1.1 client cannot do without a
  multiplexer. Positions, identities, and controls survive that change;
  connection-establishment code does not. Keep the `endpoint` field in
  `owner.json` an opaque URL-like string now so nothing deeper assumes a
  socket path.
- Who runs `go build`. The proposal picks local compilation first, which is
  consistent with the issue's "possible installation choice", but a project
  must depend on a `gimbal` version whose runner protocol the server accepts.
  The identity/status reply carries protocol identity, so the rejection path
  exists; the proposal should say that the project's `go.mod` pin is the
  compatibility unit.
- Agent-child ownership on runner crash (finding 2) is acknowledged as
  "retain unresolved cleanup separately" but has no operator path.

## Validation that would settle the claims cheaply

- Crash consistency of the journal: `kill -9` a runner mid-stream, assert the
  server's received prefix equals the runner file's synced prefix and that
  the last line is complete. One test beside the recorder.
- Restart gap and duplicate check: stop the server at a random cursor,
  restart, assert the archive equals the runner journal prefix with no
  repeated positions.
- Foreign-owner admission (finding 1) and force-stop (finding 2) tests as
  described above.
- A `go list -deps` test in the runner `main` package asserting no
  `web`, `skgo`, or `internal/builtin` import.
- The restart-and-cancel demonstration in `delivery-and-review.md` is the
  right end-to-end proof; add the two-instance-directory case to it.

## Outcome

material findings remain
