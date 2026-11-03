# Adversarial review: Neon DSNEnv wiring and root build/start

Reviewer: Sonnet. Date: 2026-11-03. Target: worktree
`/Users/tyler/.codex/worktrees/1430/gimbal` on `codex/distributed-execution-slice`,
uncommitted changes on top of `4bcc3c92`. Source stage accepted at `1006a81d` and not
re-reviewed.

No caller instruction narrowing the review was present; the read-only and
artifact-path constraints were applied as operating limits.
No secret value was printed. Only variable names, counts and hashes were read.

## Evidence inspected

- Diff: `plugins/messaging-river/main.go`, `main_test.go`, `.gitignore`, the plan
  and worklog edits. New untracked: `neon.ts`, `package.json`, `package-lock.json`,
  `examples/config/river-neon.json`.
- Plan and the November 3 worklog amendment.
- Focused checks I ran:
  - `go vet ./plugins/messaging-river`: clean.
  - `gofmt -l plugins/`: clean.
  - `go test ./plugins/messaging-river -run 'TestNamespaceValidationIsPureAndStrict|TestEnvironmentDSNRejectsMissingAndMalformedWithoutDisclosure' -count=1`:
    both pass.
  - I did not rerun the build or the native/full suites.
- Runtime configs `neon-{controller,worker-a,worker-b}.json`: all `0600`, all use
  `DSNEnv: DATABASE_URL_UNPOOLED`, one shared queue `gimbal_neon_01a19a10`, with
  `worker-a` and `worker-b` identities.
- Hashes of local `bin/gimbal` and `bin/plugins/river.so` equal those on `amrita`
  (gimbal `7e82cc7b…`, river.so `b6ffabda…`). Both binaries are newer than the
  edited sources (14:57 against 14:56).
- Processes: local app (pid 77609, under `neon-env run`), local worker A (77573,
  `--env-file=.env.local`), and a worker on amrita. Each has
  `DATABASE_URL_UNPOOLED` in its environment (counted only).
- Build log `neon-build.log`: `just build`, then the plugin build, ends with
  `go build` of `bin/gimbal` and `bin/gimbal-command`. No error lines. I did not
  rerun it.
- Run `01M6HKPKG8R0GXRWJ41TS5MTGZ` records:
  - route: `TaskRoute` is `river` with `DSNEnv` (no secret stored)
  - `sessions.json`: `environment task-remote`, `worker_id worker-b`, workdir
    `/private/tmp/gimbal-dist-01a19a10…/work`, model `gpt-6-luna`
  - conversation: placement machine `worker-b`, `contact: connected`
  - the hostname `Amritanandamayi.local` appears in the session log, with one
    `session.tool.success`
  - the second human turn returns `NEON-C1-20261103-1505`, which is the token from
    the first turn
  - `model_calls.json`: three Luna calls across turns 1 and 2
- Stop evidence: request `327678e1…` is `canceled`, interrupt `96d8b459…` is
  `completed`, and the run log shows `turn_ended … interrupted:true`
  ("human requested interruption"). The run is still `running` and the
  conversation is `waiting`.
- A read-only `psql` count against Neon: `gimbal_message_events` has 234 rows from
  8 sources, including the run's executor operation ids, so the app database is in
  use over the unpooled endpoint.

## What is established

- `DSNEnv` is implemented as exactly-one-of `DSNFile` / `DSNEnv` in `parse`, the
  JSON schema (`oneOf`), and `open`. There is no fallback from env to file; the
  test covers `/must-not-fallback`.
- Validation stays pure: `ValidateConfiguration` does not read the environment
  (tested with `UNSET_RIVER_DSN`).
- Error messages are fixed strings. Parse failures do not echo the input; the test
  uses a `secret-marker` password.
- The route record stores the variable name, not the credential.
- A real native Luna conversation on the Neon-backed River queue ran on amrita's
  worker, kept first-turn context across a second human turn, and handled a
  Stop/interrupt while a command was in progress.
- The existing pgx/River design is preserved: `pgx.ConnectConfig` on the pool's
  `ConnConfig` for the session advisory lock, pointed at the unpooled URL.
- `.env`, `.env.local`, `.neon` and `node_modules` are ignored. `.env.local` and
  `.neon` are `0600`.

## Findings

### 1. Issue: the production Neon credential is now ambient in the environment of processes that run model-driven shells

`internal/taskdaemon/process.go:81` builds task environments from
`os.Environ()`, and `internal/pi/shell/shell.go:94,206` does the same for agent
bash tools. Nothing scrubs `DATABASE_URL_UNPOOLED`. The old `DSNFile` design kept
the DSN in a `0600` file that only the plugin read. The new design puts a
read/write credential for the user's Neon `production` branch into the environment
of:

- the local app (77609),
- local worker A (77573),
- the amrita worker (89124).

Anything an agent runs there, for example `env`, can read the credential. The
credential is also visible to anything on the host that can read the process
environment.

Evidence limits:

- I confirmed the variable is in the three long-lived processes and traced the
  inheritance in code.
- I did not capture a live agent child to prove the shell sees it; none was running.

Impact: a prompt-injected or merely careless agent can exfiltrate or damage the
production database. The review requirement "do not print secrets" is undermined
by design.

Next action, one of:

- a plugin-level filter that removes the named `DSNEnv` variable from the
  executor and task environments, with a test, or
- an explicit decision to accept the exposure, or
- keep `DSNFile` for hosts that run agents.

### 2. Issue: live-test credential files are still on amrita and in `/tmp`

On amrita, in `/private/tmp/gimbal-dist-01a19a10.sK9NHy`:

- `neon.dsn` (a full Neon DSN, `0600`)
- `neon-worker.env`
- `app-postgres.dsn`
- `postgres.dsn`
- a stale `worker.log` showing the retired LAN database failing with "no route to
  host"

Locally, `/tmp/gimbal-distributed-owner-b/neon-worker.env` holds the same
variable. These are `0600`, but they persist in `/tmp` after the run.

Next action: delete them when the live check ends, with Tyler's approval. I made
no changes.

### 3. Nitpick: `examples/config/river-neon.json` is unrequested and under-explained

Nothing in the task or plan asks for an example file, and the repo rule is that
names and artifacts are added only when a workflow needs them. The file is plain
config, not a proof program, so it does not violate the "no specimen" rule.
However, the root `npm start` passes no plugin config. The wiring only works
because the route is supplied elsewhere (the UI or `--task-route`), and nothing
documents that. The example also hard-codes `worker-a`, `worker-b`,
`workflow-local` and `task-remote`.

Next action: either drop the file or state in the PR how it is used.

### 4. Nitpick: new behavior has thin test coverage

Tests cover validation and the missing/invalid env error. No test exercises a
successful `open` through `DSNEnv` (it is covered only by the live run), and
rejecting surrounding whitespace in `DSNFile` is a small behavior change for
existing configs. Both are acceptable given the live proof.

## Not established / limits on any completion claim

- This is not full D6–D10 completion, and I make no such claim.
- The tested path is one remote conversation, plus Stop on a running command.
  Remaining and not evidenced here:
  - **End:** the run is still `running`; I saw no End or cleanup completion.
  - **Restart and reconnect:** no worker or controller restart against Neon was
    shown.
  - **A to B workflow handoff:** the `workflow-local` to `task-remote` context-value
    handoff was not shown. This run is a single remote conversation.
  - **Attestation and the full original denominator:** not part of this evidence.
- The hostname is the model's own report plus the session log's tool success. The
  strongest independent evidence of remote execution is the `worker-b` placement,
  the amrita process and the hash match. That is adequate for this stage.
- The `Workdir` in the route record is the remote path while the project is
  local. That is as designed, but it is noted.

## Outcome

`material findings remain`

Findings 1 and 2 are credential-handling issues from the move to an environment
variable; the integration itself works as shown. The source change is otherwise
small and consistent with the accepted design.
