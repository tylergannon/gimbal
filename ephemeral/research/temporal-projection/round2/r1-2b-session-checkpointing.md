# R1.2 addendum: session state is portable at turn boundaries

Date: 2026-09-27. Tyler's challenge to r1-2's verdict: can session data be
stored in a bucket and downloaded so a later `Generate` continues the same
conversation on another pod? Answer: yes. r1-2 conflated "Gimbal's
adapters pin sessions to a process" (true) with "sessions cannot move"
(false). The blocker is adapter design, not harness formats.

## Verified today: Claude Code

Hands-on, Claude Code 2.1.283, haiku, in the scratchpad (not committed):

- Turn 1 with an explicit session id in config dir A, cwd W. Transcript at
  `A/projects/<cwd-mangled>/<uuid>.jsonl`, 26 lines, 212 KB.
- Copied only that file into empty config dir B at the same relative path.
- `--resume <uuid>` from B, cwd W: answered the codeword correctly. The
  file grew in place; A's copy untouched.
- Same from a different cwd W2: also correct. Resolution is by session id
  across all project directories; cwd need not match, and the continuation
  is appended to the file where it already lives.
- `--fork-session` from the copied file: worked, wrote a new file, left the
  original unmodified.
- Nothing else under the config dir was needed.

So for Claude Code a session checkpoint is one JSONL file. Upload after
each turn, download before the next, resume by id. Pin the CLI version in
the image: the format is documented as internal and version-dependent.

## By inference from r1-2's source reads, per harness

| Harness | Checkpoint unit | Restore | Confidence |
|---|---|---|---|
| Claude Code | one `<uuid>.jsonl` | copy file, `--resume` | verified |
| Codex | `CODEX_HOME` rollout `.jsonl` for the thread plus the SQLite index (resolver falls back to a filesystem scan when the index is stale) | set `CODEX_HOME` per run, start app-server, `thread/resume` | designed for it in source; not run |
| agy | per-conversation `.db` plus `.db-wal` under `~/.gemini/antigravity-cli/` | copy at same path, `--conversation <id>` | plausible; closed CLI, locking bug history |
| OpenCode | `opencode.db` (WAL) plus project row | set `XDG_DATA_HOME` per run, start `opencode serve`, resume by id | plausible; version skew with Gimbal's legacy client |
| Pi | Gimbal's own history file | `history.Open`/`ContinueRecent` exist and are tested; adapter writes to `os.MkdirTemp` and never reopens | Gimbal code, fully in our control |

## What changes in the design

1. A Gimbal session checkpoint is {harness state for that session id,
   workdir delta}. Take it at turn boundaries only: mid-turn state (daemon
   memory, half-written rollout, open WAL) is not consistent. Durability
   granularity is therefore "lose at most the in-flight turn", which is
   exactly what activity retry gives.
2. The workdir delta: git base commit plus `git diff` and untracked files
   (small), or the whole tree (large, includes caches). A per-run EFS
   volume avoids uploads entirely; a bucket is for cold storage and
   retention. Either fixes r1-2's idempotency gap: the checkpoint is the
   record of what a turn left behind.
3. Adapter changes needed: redirect state roots per run (`CLAUDE_CONFIG_DIR`,
   `CODEX_HOME`, `XDG_DATA_HOME`, Pi's root) to the run's volume; a
   daemon per pod instead of "the machine's one daemon"; Pi wired to
   reopen. Record the native session id (r1-4 §3).
4. Placement: pod-per-run remains the hot path. Checkpointing makes a
   replacement pod able to continue, so a Standalone Activity retry can
   land on any worker that restores first. Sticky by default, restore on
   failure.
5. Costs: harness state is KB to low MB per turn; the workdir delta is the
   variable part. One writer per session at a time (lease via the run's
   task queue having one poller).
6. Not portable: a turn in flight, the live daemon connection Steer uses
   (re-established after restore), agy `Fork` (unsupported anyway).

## Still to verify

Codex `CODEX_HOME` relocation end to end; agy conversation copy; OpenCode
DB copy against the client version Gimbal uses; Pi reopen through the
adapter. Each is a short test once the CLIs and keys are available.
