# Issue 177 proof: a fresh session writes a compiling workflow from the skill

Date: 2026-09-13 19:04 local, branch `claude/issue-177-workflow-skill` rebased
on `origin/main` at `b7ce435` (#176 registry, #159 infallible Set, #189 Steer
landed in).

Issue 177 asks for a fresh Sonnet session. The Claude CLI on this machine is
logged out (`claude -p` answers "OAuth session expired and could not be
refreshed"), so the session ran on Codex instead:

    codex exec -m gpt-5.6-luna -C <worktree> --sandbox workspace-write \
        -o ephemeral/attest/skill/output.txt - < ephemeral/attest/skill/prompt.txt

Model: `gpt-5.6-luna`. The session was given only the repository and the
prompt in `prompt.txt`, which points it at
`.agents/skills/gimble-workflows/SKILL.md` and asks for "a bake-off of two
Codex sessions judged by a Claude session", written once, with no Go
compiled or run by the session.

Files:

- `prompt.txt`: the prompt, verbatim.
- `transcript.txt`: the whole `codex exec` transcript, unedited. The session
  ran five commands, every one a read (`sed`, `rg`, `cat`) of the skill,
  AGENTS.md, the examples, `cmd/sprint/main.go`, the adapters, and
  `web/runtime.go`, then one write. No `go build`, `go vet`, `go test`, or
  `gofmt` appears in it.
- `output.txt`: its final message, the path of the file it wrote.
- `bakeoff/main.go`: the workflow it wrote, untouched.
- `build.txt`: `go build` and `go vet` of that file, exit 0, and `gofmt -l`
  empty, run afterwards by the orchestrator.

Result: the first draft compiles. What it wrote follows the skill: a
worktree per Codex candidate, the absolute worktree path in the prompt,
`git status --porcelain` in the repository after the Group, a Claude judge
on `claude-haiku-4-5-20251001` told to read the diffs and not the
candidates' summaries, `web.NewRuntime` with a port flag, Ctrl-C through
`signal.NotifyContext`, and the cheap models by name. It was not run live:
the issue's proof is that the file compiles on the first try.
