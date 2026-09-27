# R1.2: Session durability — what survives a worker pod death, per harness

Date: 2026-09-27. Scope: risk R1.2 from `round1/SYNTHESIS.md` — for each
harness Gimbal drives, what conversation state survives a worker pod dying,
whether a session can resume on another pod, and what that means for
Finalist 1's three candidate placements. Gimbal claims are source-only,
cited file:line, read directly against the checkout at `/home/user/gimbal`.
Harness claims are verified against official docs and upstream source,
fetched today (2026-09-27) unless noted; every claim below says which kind
it is.

## Verdict

**None of the five harnesses supports Finalist 1's placement (a) — stateless
worker pods with session state on shared storage, keyed by session id — as
Gimbal's adapters are coded today.** Three (Codex, OpenCode, Pi) are pinned to
one live local process by Gimbal's own adapter design regardless of what the
harness's on-disk format could support elsewhere; the other two (Claude
Code, agy) spawn a fresh process per turn and could in principle resume on a
different host if their transcript store is on shared storage at the same
workdir key, but even there a turn that dies mid-flight leaves a half-written
native transcript Gimbal has no protocol to detect or repair before
resubmitting. **Placement (b) — one long-lived pod per Gimbal session,
workdir and harness state local, retry-the-turn as the only recovery — is
the only placement every adapter already fits without new Gimbal-side
design**, and even there a retried turn depends on the workdir surviving on
the same pod, since none of Gimbal's `run.jsonl` facts (`TurnEnded`,
`CommandEnded`) capture the file or git state a turn left behind.

## Per-adapter findings

### Claude Code (`claude/`)

**Storage and version (docs, code.claude.com/docs/en/sessions, read
2026-09-27).** Claude Code stores transcripts as JSONL at
`~/.claude/projects/<project>/<session-id>.jsonl`, where `<project>` is the
working directory's absolute path with non-alphanumerics replaced by `-`.
Storage moves with `CLAUDE_CONFIG_DIR`; retention is `cleanupPeriodDays`
(default 30 days). The docs page gates several behaviors by CLI version up
to v2.1.281, so it reflects a build in that neighborhood, not one pinned
number. Gimbal wraps the CLI through a forked Go SDK pinned at
`github.com/tylergannon/claude-agent-sdk-go v1.1.1`
(`claude/SDK.md:1-9`), mints its own session id
(`claude/claude.go:113-125`), and hands it to the CLI via `--session-id`,
`--resume`, or `--fork-session` (`claude/claude.go:169-177`) — Gimbal never
reads the CLI's own transcript store, it only trusts the CLI to resume by
the id it was given.

**Cross-machine resume.** The docs state resuming by id searches "the current
project directory and its git worktrees first, then every other project on
this machine" — scoped to one machine by default, no documented cross-machine
path. The `<project>` key derives purely from the absolute workdir path, not
a hostname, so **if the exact `.jsonl` file is copied onto another host at
the identical workdir path, resume should locate it** — inference from the
key's construction, not a documented guarantee. The same page warns "the
entry format is internal to Claude Code and changes between versions,"
so a resuming host on a different CLI build than the one that wrote the file
is a real risk.

**Mid-turn failure.** The docs describe Claude Code's own crash recovery
precisely: a tool still running when the process ended "doesn't finish or
run again when you resume... Claude sees the call marked as cut off before
its result was recorded and is told to check whether it took effect before
running it again," unless `CLAUDE_CODE_RESUME_INTERRUPTED_TURN` is set.
Gimbal's own `RunTurn` always either starts fresh or issues `--resume
sessionID` for a later call on the same id (`claude/claude.go:169-181`); a
retry after a pod death would call `--resume` with the same id and prompt,
landing inside this documented recovery path — a reasonable fit, since the
model is told to double-check its own prior side effects. Gimbal records
`TurnEnded` only after `waitTurn` returns (`session.go:362,370,382,388`), so
a crash mid-turn leaves no `TurnEnded`, and nothing detects that a retry is
actually a resume-after-crash rather than a fresh submission.

**Placement fit** (see table): the adapter's own bookkeeping
(`a.sessions map[string]*session`, `claude/claude.go:33-38`) lives in one
process's memory, but the actual conversation state is Claude Code's own
file, not Gimbal's — the easiest of the five to externalize.

### Codex (`codex/`)

**Storage and version (openai/codex, github.com/openai/codex, HEAD
`985cf47a4eb6084b2ff6b30ebdb1216acda85bb4`, 2026-09-27, cloned and read
directly this session).** Rollouts are stored at
`~/.codex/sessions/YYYY/MM/DD/rollout-<UTC-timestamp>-<thread-uuid>.jsonl`
(`codex-rs/rollout/src/list.rs:438`, `codex-rs/rollout/src/recorder.rs:82-83`),
one immutable file per (thread, rollout) generation, keyed by the thread
UUID — the same id Gimbal's `CreateSession` returns as the Gimbal session id
(`codex/codex.go:198-208`). A separate SQLite index under `CODEX_HOME` (a
`state` crate, migrations under `codex-rs/state/migrations/`) mirrors thread
metadata and the currently selected rollout path per thread. `codex-cli`'s
`package.json` at this HEAD reports `"0.0.0-dev"` (a dev/main snapshot, not a
tagged release); Gimbal's own comments pin its tested behavior to `codex-cli
0.153.4` (`codex/codex.go:151,159`), and the SQLite-backed thread-store found
at today's HEAD sits under an "Unreleased" `CHANGELOG.md` entry — **whether
this architecture existed at 0.153.4 is unverified; version drift between
what Gimbal tested and current `main` is a real open question.**

**Cross-machine resume.** `thread/resume`'s resolver tries, in order: (1) a
live writer — in-process daemon state — (2) SQLite's recorded rollout path
for the thread, (3) a filesystem fallback scan of `~/.codex/sessions`
(`codex-rs/thread-store/src/local/thread_rollout_resolver.rs:65-120`). With
no live writer — exactly the "resume after pod death" case — resolution is
disk-based, and `cwd` is an explicit override parameter that a
reconstructed-from-disk thread simply adopts (mismatch is only checked
against an already-*running* thread,
`codex-rs/app-server/src/request_processors/thread_processor.rs:139-146`;
`codex-rs/app-server-protocol/src/protocol/v2/thread.rs:388-390`). So
**Codex's own persistence model is designed to resume from disk on a
different daemon process**, given a shared `CODEX_HOME` — intentional in the
source, but not verified live, and SQLite/WAL over a shared network
filesystem (EFS/NFS) is a widely known fragile combination for concurrent
writers (general risk, not stated in Codex's source). **Gimbal's own adapter
is a stronger constraint than the harness's design**: it dials one
machine-scoped Unix socket (`codex/rpc.go:77-114,145-160`), inherits the
ambient `CODEX_HOME` with no override (`codex/rpc.go:162-171`), and its
package comment states the intent plainly: "the machine's one shared
app-server daemon," never "a private app-server process" (`codex/codex.go:2-9`)
— a Gimbal choice, not a hard limit of Codex's persistence.

**Mid-turn failure.** If the pod (and its daemon) dies mid-turn, nothing
equivalent to Claude Code's "cut off tool call" marker is guaranteed; the
rollout JSONL simply ends without a `turn/completed` line. Gimbal's own
`readTurn` only returns on that notification (`codex/codex.go:490-587`), so
Gimbal has no code that inspects the rollout file for an unfinished prior
turn before resubmitting.

**Placement fit** (see table): `codex app-server daemon start` is a
machine-local singleton by design and Gimbal never overrides `CODEX_HOME`,
so today's code does nothing to make shared-storage placement real, even
though Codex's own resolver is built for it.

### agy — Google Antigravity (`agy/`)

**Storage and version (google-antigravity/antigravity-cli,
github.com/google-antigravity/antigravity-cli, HEAD
`6dadd6227a49905f475d22b7f0afe59493229595`, 2026-09-25, `CHANGELOG.md` read
directly this session — this repo ships only docs/changelog/examples, not the
CLI's source, so these are the vendor's own release notes, the strongest
public source available). Current released version per the changelog is
**1.2.11**. Conversation history lives under the app data root `~/.gemini/`
(entries name `~/.gemini/config/plugins`, `~/.gemini/antigravity-cli/`,
`.../cache/`, `.../settings.json`). An earlier entry records "Added SQLite
(.db) conversation support and will be CLI's conversation format" and a fix
for "scanning SQLite database files (`.db` and `.db-wal`)" — each
conversation is a per-conversation SQLite database with WAL files,
consistent with a separately fetched (unverified, WebFetch-summarized) docs
claim that conversations are keyed by absolute workspace path in a
`.../cache/last_conversations.json`-style cache and `.../cache/projects.json`.
The changelog also records a bug directly on point: "Fixed a
conversation-history database corruption risk where checking the database
file for write access could silently drop file locks held by concurrent CLI
processes on the same file" (v1.2.9) — this exact class of file-locking
fragility has already bitten this harness once, on local disk.

**Cross-machine resume.** Gimbal's own adapter passes `--conversation <id>`
and `--add-dir <workdir>` on every resumed turn (`agy/agy.go:238-252`); the
id is the only handle Gimbal holds (`agy/agy.go:99-107,120-126`), and `Fork`
is explicitly unsupported in print mode: agy's CLI has no `/fork` or headless
fork transport (`agy/agy.go:171-176`, Gimbal's own comment, a firsthand
implementer's statement about the CLI's real capability). Whether a
`--conversation <id>` resume succeeds when the `~/.gemini/` state tree is
copied to a new host at the same workdir path was not verified live; the
changelog's own project-mapping-by-workspace-path design, if the earlier
unverified docs claim is accurate, would make this plausible the same way as
Claude Code's cwd-keyed project directory — the weakest-sourced claim in
this note, flagged again below.

**Mid-turn failure.** `RunTurn` spawns one `agy` process per attempt
(`agy/agy.go:262-296,353-392,463-487`); a pod death leaves the child gone
with no clean `result` event written to its own store. Gimbal's retry loop
(`a.run`, `agy/agy.go:195-215`) only handles a steer-triggered reprompt
inside one still-live process, not a resume of a conversation whose process
is already dead — a retry after a pod death is a brand-new `agy -p
--conversation <id>` invocation with the original prompt.

**Placement fit** (see table): Gimbal's adapter holds no host-pinned daemon
(each turn is its own process, `agy/agy.go:262`), so the constraint, if any,
is entirely inside agy's own `~/.gemini/` store and its file-locking history.

### OpenCode (`opencode/`)

**Storage and version (sst/opencode, github.com/sst/opencode, HEAD
`b471c2b4495747353af768fbf2e0790c9d820ce2`, 2026-09-26, `packages/opencode`
version `1.18.32`, cloned and read directly this session).** Sessions are
rows in a single shared SQLite database at `Global.Path.data/opencode.db`,
i.e. `~/.local/share/opencode/opencode.db` by default
(`packages/core/src/global.ts:1-30`,
`packages/core/src/database/database.ts:41-53`), opened with `PRAGMA
journal_mode = WAL` (`database.ts:27`). A `SessionTable` row is scoped to a
`ProjectTable` row keyed by the project's worktree/git identity, with a
`directory`/`path` pair recording the actual cwd relative to that worktree
(`packages/opencode/src/session/session.ts:81-127,171-172,506-982`,
`packages/opencode/src/project/project.ts:196-306`) — materially different
and newer than the file-per-session JSON older releases used (still
referenced by open community issues, e.g. `sst/opencode#3026`,
`anomalyco/opencode#36178`, evidence the migration caused real breakage).
Gimbal's own package comment calls its client "**Gimbal's legacy OpenCode
harness**" (`opencode/adapter.go:69`); the generated REST client
(`opencode/types.json`, OpenAPI version `"1.0.0"`) may target an older
server surface than 1.18.32 — **unverified, should be smoke-tested before
any placement decision leans on the storage model above.**

**Cross-machine resume.** Gimbal's own `server.go` starts or discovers
exactly one local `opencode serve` process per Gimbal-owned discovery
directory (default `~/.gimbal/opencode`, overridable via
`GIMBAL_OPENCODE_DIR`), guarded by a `flock`-style lock file and a
`server.json` recording that process's PID, URL, and credentials
(`opencode/server.go:20-28,64-123,282-301`). **This discovery/lock directory
is not the same directory as OpenCode's own session data** (`XDG_DATA_HOME`),
which the launched `opencode serve` process still inherits unchanged from
`os.Environ()` (`opencode/server.go:203-210,374-386`) — Gimbal never
redirects it. Two worker pods pointed at the same `GIMBAL_OPENCODE_DIR` would
each try to become "the" server and contend for the flock; if `XDG_DATA_HOME`
were also shared they would additionally contend for one SQLite file under
WAL, not validated for concurrent multi-host access by anything read this
session.

**Mid-turn failure.** `RunTurn` posts one synchronous `client.Prompt` and, on
`Steer`, calls `client.Abort` (`opencode/adapter.go:124-221,225-262`). If the
worker pod dies mid-`Prompt`, the in-flight message is orphaned server-side
with nothing left to abort it cleanly; a resuming instance has no record of
the dead turn, since that bookkeeping lived only in the dead process's
memory (`adapterSession.active`, `opencode/adapter.go:32-52`). `server.go`
starts `opencode serve` as a child process of the Go binary
(`opencode/server.go:180-229`), normally co-located with the adapter, so a
pod death kills both — there is no "server survived, Go process died" case.

**Placement fit** (see table): (a) needs both OpenCode's own SQLite database
(not just Gimbal's discovery directory) on validated concurrent-safe shared
storage, and arbitration over which pod's `opencode serve` owns a session —
neither exists today. (c) also pays a real ~15s startup cost per turn
(`startupTimeout`, `opencode/server.go:26`).

### Pi (`pi/`, `internal/pi/`)

**Storage.** Gimbal's Pi adapter is a from-scratch, in-process, native Go
port — not a subprocess wrapper around any upstream CLI (package comment,
`pi/adapter.go:1-3`) — so there is no external CLI version to check; the
port's own doc states it is "a semantic port of
`packages/coding-agent/src/core/session-manager.ts`... at upstream pin
`d6af72e1857cfb10b41d8ff8e69f0d72b4cf6d31`," and that "callers pass the
session directory (typically under a Gimbal state root) explicitly. Nothing
here reads `~/.pi`" (`internal/pi/history/doc.go:1-16`). In practice,
`Adapter.CreateSession` creates that directory with `os.MkdirTemp("",
"gimbal-pi-")` — an **ephemeral OS temp directory, not under the session's
actual workdir** (`pi/adapter.go:84-96`); `history.Create` then writes one
append-only file per session at `<sessionDir>/<timestamp>_<sessionUUID>.jsonl`
(`internal/pi/history/session_manager.go:82-113,202-210`), with `workdir`
recorded only as a header field for later matching, not the storage location
itself.

**Cross-machine resume.** There is no resume path in Gimbal's Pi adapter:
`CreateSession` always calls `history.Create` — a brand-new file — never
`history.Open` or `history.ContinueRecent` (both exist in
`internal/pi/history/session_manager.go:125-178` but are unused by the
adapter); `Close` deletes the entire temp root with `os.RemoveAll`
(`pi/adapter.go:418-439`). `Fork` is served purely from the in-process
`*session.Session`'s own deep copy (`native.Fork(ctx)`, `pi/adapter.go:398-415`),
not from anything on disk. **The underlying `internal/pi/session` package
does support reopening a persisted session from its history file** —
exercised directly by `TestPersistenceReopen`
(`internal/pi/session/runtime_test.go:14-40`) — so cross-pod resume is a
capability the lower-level code already has and tests, not one the Pi
adapter exposes to Gimbal workflows today.

**Mid-turn failure.** Because the temp directory lives outside the workdir
and is deleted on any `Close`, and a pod death never calls `Close`, a dead
pod's JSONL history file is an orphaned file in that pod's local `/tmp` —
worthless for recovery unless the pod's entire local disk (not only the
shared workdir volume) were itself durable and reachable, which nothing in
Gimbal's design provides. The live `*session.Session` object — its open
provider stream, its in-flight `native.Prompt(ctx, ...)` — is pure
Go-process memory; a pod death loses it outright. **Pi has the simplest
failure mode of the five: total loss, always, with nothing partial to detect
or repair.**

**Placement fit** (see table): (a) needs the adapter redesigned to persist
under a durable, Gimbal-provided root instead of `os.TempDir()`, and to wire
`Open`/`ContinueRecent` into `CreateSession` — neither exists today, though
the lower-level package already supports the reopen half. (c) breaks `Fork`,
since forking only works from the still-live in-process object.

## Placement table

**Yes**: works with today's adapter code plus only infra changes (e.g.
mounting shared storage). **Partial**: the harness's own design would allow
it, but Gimbal's adapter actively prevents it (needs adapter changes, not
just infra). **No**: neither the harness nor the adapter supports it.

| Adapter | (a) Stateless pods + shared storage keyed by session id | (b) One long-lived pod per Gimbal session | (c) One pod per turn |
|---|---|---|---|
| Claude Code | Partial — Claude Code's own file store is cwd-keyed and plausibly portable if copied to the same absolute path; Gimbal's own per-process session bookkeeping would need externalizing | Yes | Yes — already closest to "spawn per turn" |
| Codex | Partial — Codex's own resolver is designed to reconstruct from disk, but Gimbal pins to one machine-scoped daemon socket and never redirects `CODEX_HOME`; SQLite-over-shared-storage safety unverified | Yes | Yes, mechanically — but discards the shared-daemon/warm-MCP design the adapter is built around |
| agy (Antigravity) | Partial/unverified — no Gimbal-side daemon to block it, but agy's own `~/.gemini/` SQLite store has a documented history-of-record file-locking bug class | Yes | Yes |
| OpenCode | No — Gimbal's flock-guarded single local server plus unvalidated SQLite-WAL-over-shared-storage both block it | Yes | Partial — mechanically fine, ~15s startup cost per turn |
| Pi | No — ephemeral `os.TempDir()` root, deleted on `Close`, no resume path wired in the adapter (though the lower package supports reopen) | Yes — the only placement that fits Pi's design | Partial — fine for a fresh turn, breaks `Fork` |

## What a retry of a turn must do (every adapter)

1. **Re-send the same prompt as a new native turn/process/RPC**, not resume
   mid-flight — none of the five adapters can detect or reattach to a
   still-running native turn from a different process; each treats "the
   process/daemon connection is gone" as "the turn ended," whether via
   `ctx.Err()` (Claude, agy, OpenCode, Pi) or a dead WebSocket (Codex).
2. **Accept that the native conversation may already contain a half-finished
   assistant turn** for Claude Code and Codex, whose persistence writes
   incrementally; Claude Code's documented behavior explicitly plans for
   this (the cut-off-tool-call marker), but Codex's rollout format surfaces
   no equivalent marker that Gimbal's `readTurn` inspects.
3. **Not double-count usage**: `TurnEnded.Usage` is written only on a
   completed turn (`session.go:362-388`); a retry's own `TurnEnded` is a
   second, independent record — nothing merges the two into "one logical
   turn, two attempts."
4. **Handle a git index lock or a half-applied edit already sitting in the
   workdir.** No code in `command.go`, and none of the five stock workflows
   under `internal/workflows/`, clears a stale `.git/index.lock`; none call
   `git` directly at all (confirmed by grep; the only `RunCommand` calls
   found are `count-tokens` and a browser/ffmpeg pipeline,
   `pyramidsummary.go:465`, `researchdocument.go:323`,
   `validateproduct.go:163`). `AGENTS.md` names "a worktree, a merge" done
   "with `Group`, `Generate`, and git through `RunCommand`" as the expected
   shape for workflows not yet written, so this gap is real for the future,
   and no Gimbal code addresses it yet.
5. **Re-resolve host-specific values computed in the workflow body before
   the crash**: `os.Executable()` for a token-counter path, `exec.LookPath`
   for `playwright-cli`/`ffmpeg`, `os.MkdirTemp` for a nondeterministic
   directory (round-1 audit §4). None are re-derivable from `run.jsonl`;
   today's bodies recompute them unconditionally rather than memoizing, so
   this is already safe under retry-by-re-run.

## Idempotency: what's captured, what must be reconstructed from the workdir

**Sufficient to skip a completed node on resume:** a `TurnEnded` (`Result`,
`Error`, `Usage`, `Duration`, `Interrupted`, `events.go:144-156`) or
`CommandEnded` (`ExitCode`, `Stdout`/`Stderr` plus full-stream file
references, `Error`, `Interrupted`, `Duration`, `events.go:190-209`) record
in `run.jsonl`, for a node's deterministic id (`lap.3/check.2`-style, stable
per round-1 audit §1.2), tells a resuming orchestrator that node's logical
outcome unambiguously — enough to decide "do not re-invoke this harness call
or subprocess."

**Not captured:** neither event records a diff, a file list, or a git commit
hash. A turn's or command's writes to the workdir exist only in the workdir
itself — no claim-check or content-addressed capture of "what changed" is
anywhere in `events.go`. "Skip it, `TurnEnded` says it's done" is only
correct if the working tree is still in the exact state that turn left it —
which needs either the workdir surviving on the same local disk (placement
(b)), or the workdir on shared durable storage the new pod mounts unchanged
(an infra requirement nothing in Gimbal arranges or verifies), or a
workdir-delta capture mechanism, which does not exist. Under placement (c) —
re-clone each turn — a "done" record is actively misleading unless the
re-clone also replays every commit or write those earlier nodes made, which
nothing in Gimbal does automatically and none of the five stock workflows
are written to support (none call `git commit` at all).

## Remaining unknowns

- Whether Codex's disk-based `thread/resume` fallback actually succeeds
  end-to-end from a second daemon pointed at a shared `CODEX_HOME`: read
  from source, not run live; no report of anyone exercising this handoff was
  found either.
- Whether `codex-cli 0.153.4`, the version Gimbal tested against, has the
  SQLite-backed `thread-store`/`state` crate found at today's `main` HEAD.
- Whether agy's workspace-path-keyed conversation cache (found only through
  a WebFetch summary of Google's docs, not source — the CLI is closed and
  the public repo ships only docs/changelog) is accurate, and whether
  copying that cache plus the `.db`/`.db-wal` files to a new host at the
  same workdir path actually resumes.
- Whether OpenCode's REST client Gimbal generated against
  (`opencode/types.json`, OpenAPI version `"1.0.0"`) matches the current
  1.18.32 server closely enough that this note's findings are even the
  version Gimbal's adapter talks to — "the legacy OpenCode harness" comment
  suggests it may not be.
- Whether SQLite in WAL mode genuinely breaks under concurrent multi-host
  access on AWS EFS specifically (general NFS/SQLite-WAL fragility is
  well-known lore; no source found here tests EFS for OpenCode's or agy's
  database files specifically).

## Sources

- `code.claude.com/docs/en/sessions`, read 2026-09-27 — official Claude Code
  docs: transcript storage path, retention, resume scope, mid-turn-crash
  recovery, version gates through v2.1.281.
- `github.com/openai/codex`, HEAD `985cf47a4eb6084b2ff6b30ebdb1216acda85bb4`,
  cloned and read directly 2026-09-27 — `codex-rs/rollout/src/list.rs`,
  `codex-rs/rollout/src/recorder.rs`,
  `codex-rs/thread-store/src/local/thread_rollout_resolver.rs`,
  `codex-rs/app-server-protocol/src/protocol/v2/thread.rs`,
  `codex-rs/app-server/src/request_processors/thread_processor.rs`,
  `codex-cli/package.json`, `CHANGELOG.md`. Supporting:
  `learn.chatgpt.com/docs/app-server` (redirected from
  `developers.openai.com/codex/app-server`), fetched 2026-09-27, corroborates
  but does not add cross-machine detail.
- `github.com/sst/opencode`, HEAD `b471c2b4495747353af768fbf2e0790c9d820ce2`,
  package version `1.18.32`, cloned and read directly 2026-09-27 —
  `packages/core/src/global.ts`, `packages/core/src/database/database.ts`,
  `packages/core/src/database/path.ts`,
  `packages/opencode/src/session/session.ts`,
  `packages/opencode/src/project/project.ts`.
- `github.com/google-antigravity/antigravity-cli`, HEAD
  `6dadd6227a49905f475d22b7f0afe59493229595`, `CHANGELOG.md`/`README.md` read
  directly 2026-09-27 (repo ships only docs/changelog, not CLI source — the
  vendor's own release notes, the strongest available public source).
  Weaker, WebFetch-summarized supporting source, not raw-verified:
  `antigravity.google/docs/cli/conversations/` and
  `.../docs/cli/commands/resume/`, fetched 2026-09-27, for the
  workspace-path-keyed cache and per-conversation SQLite claims.
- Gimbal source, read directly this session: `claude/claude.go`,
  `claude/SDK.md`, `codex/codex.go`, `codex/rpc.go`, `agy/agy.go`,
  `opencode/adapter.go`, `opencode/server.go`, `pi/adapter.go`,
  `internal/pi/history/doc.go`, `internal/pi/history/session_manager.go`,
  `internal/pi/session/runtime_test.go`, `events.go`, `session.go`,
  `command.go`, and the three stock workflows under `internal/workflows/`
  that call `RunCommand` directly (`pyramidsummary.go`,
  `researchdocument.go`, `validateproduct.go`).
- `ephemeral/research/temporal-projection/round1/gimbal-internals-audit.md`
  §3 and §4, and `round1/SYNTHESIS.md`, read in full before this note was
  written; every repeated audit finding was independently re-verified
  against the cited source in this session, not taken on the audit's word
  alone.
