# Tyler's direction for the port, 2026-09-25

Given by Tyler to the Claude session that took over this worktree after
ChatGPT/Codex stopped working on his Mac (every Codex request fails with a
server-side 401). Where this file and [PLAN.md](PLAN.md) disagree, this file
wins. [MODULES.md](MODULES.md) and [WORKFLOW.md](WORKFLOW.md) still hold
unless changed here.

## What we are building

A Go port of the Pi harness that runs agent loops **in process**. The whole
point: a Gimbal workflow must not spawn a separate process for each agent run
through Pi. Each session is goroutines inside the Gimbal binary, called
directly, never by shelling out to a CLI. Tool commands such as `bash` still
start ordinary child processes; the agent itself does not.

## How to port

- **A near-straight copy.** Translate the pinned TypeScript module by module
  into Go. Do not redesign. The one deliberate translation is concurrency:
  TypeScript promises and async iteration become goroutines, `context`
  cancellation and `errgroup` (`golang.org/x/sync`, already in `go.mod`).
- **Upstream tests are the specification.** Pi ships many TypeScript unit
  tests. Translate the relevant ones beside each Go package. The more
  translated tests, the better.
- **Where upstream has no test,** prove behavior by running the Go harness
  and the TypeScript Pi CLI repeatedly on the same prompts through the
  Diffusion Router and comparing results.

## Who does what

- **Top-level session (Claude, this worktree):** owns the workflow, makes sure
  it runs the module assignments in parallel, integrates, and is personally
  responsible for the result actually working. Final acceptance is what this
  session observes, not what workers report.
- **Coders:** open-weight models through Gimbal's existing Pi harness on
  `main` (`pi/diffusion/...`, merged in #392). Candidates are
  `deepseek-4.1-flash` and `glm-5.3`; pick with a cheap head-to-head on a real
  assignment, not by guess.
- **Planning and review, where needed:** Claude Sonnet.
- **No Codex or ChatGPT roles** until OpenAI fixes the account.

## Temporary CLI

Build a small CLI for the Go harness now so it can be driven and compared
against the TypeScript CLI. It is temporary: put it where it is obviously
disposable (`cmd/pigo/`) and delete it when the harness is wired in.
Long term there is no separate CLI. The Go harness is reached through
`gimbal run-prompt` and through the in-process harness binding that
workflows use, replacing the RPC adapter from #392.

## Diffusion Router key

Tyler supplies a temporary `DIFFUSION_API_KEY` for this job and will delete it
at the end of the day. Use it freely for live runs and final validation. Keep
it outside the repository (never committed, never in tracked files).

## Done means

The workflow ran every assignment, the pieces are integrated, the translated
upstream tests pass, and this session has seen the Go harness do the same
things the TypeScript Pi does on real Router-backed coding tasks: read,
edit, run commands, continue a conversation, steer, cancel, fork, compact,
return schema-valid output. It also runs inside Gimbal with no Pi child
process per session.
