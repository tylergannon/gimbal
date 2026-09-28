# Browser implementation design

Minimal design for the fifteen claims in `browser-evaluator-build.md`, revised
after `reviews/browser-design-round-01.md` through `browser-design-round-05.md`
(all material findings accepted), and the manager's shared-workdir finding.
One connector (`playwright-cli` 0.1.21), one
harness (Codex), no resource registry. The only new public names are
`NewBrowser`, `Browser` and `WithBrowser`. Backend packaging adds one config
field, `worker_binary`. The consumer image stays external, as assigned.

## 1. Public API (root package, new `browser.go`)

```go
// NewBrowser opens a browser named name in the ctx's environment and adopts
// it into the ctx's scope. workdir is the absolute directory every browser
// command runs in, and where its screenshots go. video is an absolute .webm
// path to record into, or "" for none. The browser is released when its
// scope ends. There is no Close.
func NewBrowser(ctx context.Context, name, workdir, video string) (*Browser, error)

// Browser is a live browser session owned by the scope that created it.
type Browser struct {
    id, name, workdir, video string // id = scope.next(name)
    session     string   // first 12 bytes of SHA256(runDir(ctx)+"/"+id), encoded as 24 hex characters
    wrapper     string   // workdir + "/.gimbal-browser-" + session: one per handle
    environment string   // environment name from ctx at creation ("" = host)
    owner       *scope
    createCtx   context.Context // context.WithoutCancel(ctx), used for close
    mu       sync.Mutex
    released bool
}

// WithBrowser gives this turn's agent working access to b.
func WithBrowser(b *Browser) AgentOption
```

`name` must be a compile-time constant. That is enforced by the graph
generator's diagnostic, as for `RunCommand` and `Service` (`expr.go`); there is
no lint rule for it. There is no URL parameter. The workflow `Set`s the URL,
and the agent navigates with `goto`. Connector details (the binary name,
`-s=`, `--idle-timeout`, the wrapper, video commands) live only in
`browser.go` (claim 3).

**One command per handle.** The wrapper path and the session both derive from
the run directory and scope-local handle id: SHA256(runDir(ctx)+"/"+id),
truncated to 12 bytes and encoded as 24 hex characters. Two browsers in one
workdir, including two runs with the same scope path, get distinct sessions,
wrappers and daemons. Test both cases.

## 2. Open and close, both bound to the environment

Both run through the internal `runCommand(ctx, s, name, workdir, command,
args)`. They use the existing environment resolution in `runCommand`, so they run
in the creation ctx's environment.
Hosted runs set `InEnvironment` on the root ctx, so this is the Docker worker.
They are recorded as ordinary `CommandStarted`/`CommandEnded` records.

**Open**, as the command `name`, is `zsh -c <script> zsh
<wrapper> <wrapper text> <session> <video>`. Values travel as positional
arguments, so nothing is quoted into the script:

```sh
set -e
printf '%s' "$2" > "$1" && chmod +x "$1"
playwright-cli -s="$3" open about:blank --idle-timeout=0
[ -z "$4" ] || playwright-cli -s="$3" video-start "$4" --cursor
```

`--idle-timeout=0` disables playwright-cli's one-hour headless idle shutdown
(probe README, "Runtime limitations"). The daemon then lives until the scope
closes it or its container is removed, so the scope owns the lifetime (claim 4).
The caller supplies an existing workdir, as validate-product already does.
The daemon is spawned detached in its own session/process group, so it survives
the worker's cleanup of the command's group. `docker run --init`, already used
by the backend, reaps adopted children. Open and close both use the declared
browser name: their distinct ordinals identify the commands, while the existing
run page can bind both records to the browser's declared service.

**The wrapper text** is built in Go with private `browserShellQuote` (the existing test-only `shellQuote` stays unchanged) for the workdir:

```sh
#!/bin/sh
for arg in "$@"; do
  case $arg in
    -s|-s=*|--session|--session=*) echo "browser: the session is fixed" >&2; exit 2 ;;
  esac
done
for arg in "$@"; do
  case $arg in
    -*) ;;
    open|attach|detach|close|close-all|kill-all|delete-data|video-start|video-stop|install|install-browser)
      echo "browser: $arg belongs to the workflow, not the agent" >&2; exit 2 ;;
    *) break ;;
  esac
done
cd '<workdir>' && exec playwright-cli -s=<session> "$@"
```

The first non-flag argument is the subcommand, and a lifecycle subcommand is
refused before playwright-cli runs. An agent cannot lose the recording by
calling `open` or `video-stop` through the command it was given. This is a
guard on the supplied command only. It is not a sandbox, because Codex runs
`danger-full-access` and can still call `playwright-cli` directly (§10). The
`cd` keeps relative screenshot paths and `.playwright-cli/` logs in the workdir.

Order in `NewBrowser`:

1. `current(ctx)` fails → return the error.
2. The scope has ended → return an error.
3. Build `b` and `owner.adoptBrowser(b)`, which errors if the scope is ending.
4. Run open. A nonzero exit or error → `b.close()` immediately (bounded,
   uncancelled), mark it released, and return `nil, errors.Join(openErr,
   closeErr)`. Partial startup and incomplete cleanup are errors (claim 6);
   failed cleanup must not be described as a confirmed release.

**Close**, as the command `name`, runs on
`context.WithTimeout(b.createCtx, 30*time.Second)`. The creation ctx has
cancellation removed but keeps its values, so the environment binding and the
scope key still hold. It calls `playwright-cli` directly, not the wrapper,
which refuses these subcommands. The command is
`zsh -c <script> zsh <session> <video>` in `workdir`:

```sh
rc=0
if [ -n "$2" ]; then playwright-cli -s="$1" video-stop || rc=1; test -s "$2" || { echo "video missing or empty: $2" >&2; rc=1; }; fi
playwright-cli -s="$1" close || rc=1
exit $rc
```

`video-stop` runs before `close`, because the probe showed that `close` while
recording writes no file at all. `video-stop` returns only after the WebM is
written, and `test -s` checks it, so the recording is final before `close`.
Close always attempts `close`, even when `video-stop` fails. It is idempotent
through `released`. A nonzero exit is an error naming the browser. No
`kill-all` is used, because it would reach other sessions.

**Scope hook.** `scope` gains `browsers []*Browser` and `adoptBrowser`. In
`scope.end()`, browsers close after `s.ended = true` and before services stop
and sessions close, so the application under test is still up while recording
stops. Browser close errors are joined with `serviceErrs` into the scope's
error, like service cleanup errors (claim 6), not diverted to `CloseError`.
`Run` closes the backend only after the root scope ends, so the environment is
still resolvable at close.

### Dependent work stops before release

Close runs after the body returns, and the body's `Generate` calls have
returned. That is only a join if a returned turn has really stopped. Each path
below either stops it or says it did not.

- **Temporal backend, unconfirmed cancellation** (`internal/execution`). In
  `harnessProxy.RunTurn`'s `ctx.Done()` branch, the cancellation is confirmed
  when the activity completes within the confirmation wait with a result, a
  Temporal cancellation, or an application failure other than the private
  `turnUnconfirmedError` defined below. Preserve any completion error in the
  returned error. No completion by the deadline, that private unconfirmed
  failure, or a transport/server timeout without native confirmation is
  unconfirmed. This same classification applies on both completion paths.
  For an unconfirmed turn, the controller cannot reach it any other way, so it
  takes the smallest action that stops it. It
  removes that environment's worker container with the bounded `docker rm -f`
  that `Backend.Close` already runs. Factor out that per-container removal,
  including the bootstrap-row delete, as `Backend.removeWorker(name)`. It
  also marks the `Environment` removed. The returned error joins `ctx.Err()`,
  the Cancel error, the cause, and the removal result.
  `Process.Wait`'s unconfirmed branch (`temporal.go`, "activity cancellation
  was not confirmed") does the same, since an open or close command can be
  what was cancelled.
  Apply the same bounded acknowledgement and environment-removal rule to
  `harnessProxy.request` (create/fork/steer/close) and `Process.Stop`; an
  unconfirmed timeout there must not leave the environment available for work.
- **Confirmation timing.** Replace the independent 10-second literals with the
  existing shared `activityWaitTimeout`, initialized to four times a shared
  `activityHeartbeatTimeout` of 15 seconds. Use that heartbeat constant for
  command and harness activities. The resulting 60 seconds covers the SDK's
  maximum 12-second heartbeat throttle, an outstanding Codex turn/start request
  (up to 30 seconds), interrupt (5 seconds), drain (10 seconds), and 3 seconds of
  margin. This bounds exceptional acknowledgement delay; normal completion
  returns immediately when confirmation arrives. Keep these timing assumptions
  together in a comment; a provider stop-budget change requires updating this bound. Add a
  real Temporal integration test with no model credentials: cancel a running
  command activity after its first heartbeat, await confirmed termination,
  then run another command in the same environment. It must remain usable.
  Retain separate fault tests for genuinely unconfirmed cancellation/removal.
- **After removal.** Removing the container ends the Codex daemon and its turn,
  the browser daemon, and every command in the environment. `Start`, `RunTurn`
  and harness requests on a removed `Environment` fail immediately with
  `execution: environment %q was removed after an unconfirmed cancellation`.
  The browser close that follows therefore fails with that error in the scope
  error. The browser ended with the container, its recording is lost, and the
  error says both. New work in the same environment also fails with errors
  naming the removal. Already-running operations append that same removal
  cause when their completion/timeout arrives, by checking the environment
  state before returning; preserve their original failure too. That is the cost
  of a worker that cannot confirm a cancellation, and there is no restart. If `docker rm` itself fails, the
  environment is still marked removed, so no more work is sent. The error says
  the worker may still be running, the container stays on `b.containers`, and
  `Backend.Close` tries to remove it again. Release is not claimed in that case.
- **Codex adapter** (`codex/codex.go`). Have `readTurn` report whether it
  consumed this turn's terminal notification, independently of `ctx.Err()`.
  Keep that fact even when projecting that terminal event returns an error.
  If the first read saw a terminal, return its result/error (or cancellation)
  without interrupting or draining for a second copy of the event. If it
  returned without a terminal for any reason, including an interactive request
  refusal or projection failure with a live ctx, interrupt and drain using the
  existing bounded control contexts. The only terminal is a matching
  `turn/completed` for this turn. Give terminal draining an explicit private
  10-second budget (separate from the 5-second control-RPC timeout).
  Draining keeps consuming nonterminal error
  notifications, declines interactive requests, and records projection errors
  without stopping the drain, until that terminal, connection loss, or the
  deadline. It must not reuse readTurn's early-return behavior for ordinary
  messages. A terminal seen during drain confirms the stop even if the
  interrupt RPC raced with completion. Return the original error after a
  confirmed stop. An interrupt acknowledgement alone is not confirmation.
  Check cancellation before sending turn/start. Once sent, await its
  acknowledgement on a bounded 30-second context independent of the turn ctx.
  If the turn ctx is then cancelled, use the acknowledged turn id to interrupt
  and drain normally. A definite start rejection is an ordinary failure.
  Transport loss, acknowledgement timeout or an invalid acknowledgement after
  the request may have started a turn remains unconfirmed unless it can be
  identified and stopped; never label that a clean cancellation.
- **Unconfirmed error across the fixed activity boundary.** If neither read
  sees a terminal, return a private Codex error type `turnUnconfirmedError`
  whose text includes the turn and the failures, but which does not unwrap to
  `context.Canceled`. Keep that concrete error intact through the worker.
  Temporal's existing Go error conversion carries its type as the application
  failure type `turnUnconfirmedError`. The Codex-only proxy recognizes that
  exact wire error type with `errors.As` to Temporal's `ApplicationError` and
  removes the environment on either completion path: cancelled ctx or live ctx.
  All other completed provider errors remain ordinary confirmed failures.
  This is one private error in the existing Codex activity contract, not a new
  exported Gimbal API or a generic harness protocol. Test its real SDK failure
  conversion and the proxy's treatment on both paths. On the host backend the
  same unconfirmed error remains visible; it does not authorize killing a
  shared host daemon.
- **Surfacing** (`session.go`, `Session.turn`). Preserve the original adapter
  error before either stopped-path assignment. At the final stopped return,
  join it with the stop cause and terminal-event recording error; record this
  same final joined text in `TurnEnded.Error`. Avoid adding a duplicate bare
  `turnCtx.Err()` but retain any additional failure detail. The earlier
  `err = stopped` and later `errors.Join(stopped, terminalErr)` must not erase
  it. Test both the returned error and the emitted turn record. A confirmed
  cancellation without additional failures still reads as before, and
  `errors.Is(err, context.Canceled)` remains true.

## 3. WithBrowser: explicit, turn-scoped access

`options` gains `browsers []*Browser`. The access text is added in
`dispatchRecorded`, not `Generate`. Worker turns and supervisor looks (which
call `dispatch` with `sup.opts`) then share one path, and each gets access only
when WithBrowser is in its own options:

```go
func dispatchRecorded[T Output](ctx context.Context, s *Session, prompt string, opts []AgentOption, started *TurnStarted) (T, error) {
    o := apply(opts)
    ask, err := browserAccess(ctx, s, prompt, o.browsers) // prompt + "\n\n" + access block per browser
    if err != nil { ... }
    if len(o.supervisors) > 0 {
        return supervise[T](ctx, s, ask, o.supervisors, started)
    }
    return generate[T](ctx, s, ask, nil, typeName, started)
}
```

Keep the existing supervision signatures and transcript unchanged. Supervisors
see the worker's task, browser instructions and browser commands as material to
review. Those quoted grants belong to the worker; they are not the supervisor's
own grant. The supervisor receives its own browser access block only when its
`WithSupervisor(..., WithBrowser(b))` options provide it. No transcript filtering
or separate task parameter is needed, and this is an instruction boundary,
not a sandbox or a confidentiality claim.

`browserAccess` checks each browser before any model call and returns an error:

- The browser is released, or its owner scope has ended.
- The ctx's scope is not the owner or a descendant of it (walk `parent`).
- `b.environment != s.environment`. The session keeps its own role and
  environment binding, and WithBrowser never rebinds it.

The access block is a fixed constant, formatted with this handle's wrapper
path and workdir:

```text
You have a browser that is already open for you. Drive it only
with the shell-quoted command <wrapper>, for example `<quoted-wrapper> goto <quoted-url>`,
`<quoted-wrapper> snapshot`, `<quoted-wrapper> click <ref>`,
`<quoted-wrapper> screenshot --filename=<quoted-screenshot-path>`. Save screenshots in
<workdir>. The workflow opens, records and closes this browser; the command
refuses open, close, video and session commands. Do not call playwright-cli
directly.
```

For `Generate`, `TurnStarted.Prompt` stays the workflow prompt as written. The existing
`session.inbox.enqueued` record in `Session.turn` receives the actual final
prompt, including browser instructions; preserve that existing path so the
delivered instructions remain inspectable without changing the page.

`PromiseLoop` passes its opts to `dispatch`, so a planner could be granted a
browser. That falls out of the shared path and needs no code. `Interview`
passes nil.

## 4. Graph generator

- `internal/generate/expr.go` `gimbalOperation` gets a case `"NewBrowser"` →
  `emitService(workflow.Service{Source, Name: <arg 1 constant>})`. A
  non-constant name is the same diagnostic the `RunCommand` and `Service`
  cases give. Like a Service, it is not an ordered operation, and its lifetime
  is owned by the containing body. There is no new graph type.
- `internal/generate/read.go` `isOperation` also lists `"NewBrowser"`, so
  `holdsOperation` walks a helper or function literal whose only operation is
  `NewBrowser`.
- `WithBrowser` inside Generate's options is ignored like other non-supervisor
  options. Add cases to the existing expr and read tests.

## 5. Linter: GIMBAL110-BROWSER/ESCAPE

Message: `[GIMBAL110-BROWSER/ESCAPE]: a *gimbal.Browser must not outlive the
scope that created it; keep it in that scope's body and pass it to calls or
WithBrowser.`

**Tracked by type.** A browser value is any expression whose type is, or
contains, `*gimbal.Browser` (pointer, slice, array, map, chan, struct, or func
result). What produced it does not matter: `NewBrowser`, a helper's result, a
parameter, an alias.

**Owner body.** The owner body of a browser-typed local or parameter is the
body that declares it. That is a function body or a nested lifetime body from
the analyzer's `boundaries`: a `Scope` callback, a `Go` callback, or the body
of a `range gimbal.Iterate(...)` loop. A browser-typed call result that is not
bound belongs to the body containing the call.

**Options that retain a handle.** A direct `WithBrowser(b)` result also carries
b's owner even though its Go type is `AgentOption`. Propagate this ownership
through local assignments/aliases and locally constructed `[]AgentOption`
literals and `append` results. A Gimbal option constructor also carries the
ownership of its browser-bearing arguments, including
`WithSupervisor(reviewer, instruction, WithBrowser(b))` and a spread of locally
tracked options into that call. Test both direct outer assignment and outer
slice append of that result as errors, and its use within the owner as allowed.
Such option aggregates are allowed within the
owner body; outward assignment, capture, return across a local owning scope,
or storage follows the same escape checks. Preserve known owner information
through aliases instead of treating a fresh inner alias as a new resource.
In particular, the review's `opts = append(opts, WithBrowser(b))` assignment
from a child scope to an outer options slice must be diagnosed. This is local
flow tracking, not general interprocedural closure analysis: document that
opaque helper-produced AgentOptions and unknown container mutations are outside
the static guarantee and remain subject to runtime owner checks.

Reported:

1. Assignment (`=`) of a browser value to a variable declared outside the
   value's owner body: an outer local, a captured variable, a named result, or
   a package variable. Declaring it with `:=` or `var` in a body is fine.
2. Storing a browser value into a field, index or map element, or through
   `*p`, sending it on a channel, passing it to `append`, or using it as a
   composite-literal element or field. Locally tracked AgentOption aggregates
   are the explicit exception described above: retain ownership on the aggregate
   and diagnose escape, rather than rejecting safe local option assembly.
3. Converting a browser value to an interface type, explicitly or implicitly:
   by assignment, as a call argument, in a return, or as a composite element.
   An escape through `any` would lose the type.
4. A function literal that references a browser-typed variable declared
   outside it is reported, with these exceptions. It may be a `Scope` callback,
   the body of a `range gimbal.Iterate` loop, or the callback of `g.Go` where
   `g` is a group variable (`gimbal.Group`'s result) declared in that
   variable's owner body or a body nested in it. Every other literal is
   reported: a deferred or stored closure, a callback to another API, or `Go`
   on a group from an outer body.
5. Any reference inside a `go` statement.
6. A package-level variable, or a struct field declared in a workflow package,
   whose type contains `*gimbal.Browser`.

Allowed: local declarations, passing it as an argument whose parameter is
browser-typed, `WithBrowser(b)`, `_ =`, the captures in rule 4, and returning it
from a function. A returned browser is bound in the caller, and the rules apply
there. A helper cannot hand out a browser from a scope it opened without an
outward assignment (rule 1) or a capture (rule 4).

The review's example yields two diagnostics. `kept = b` is rule 1, because
`kept` is declared in `Workflow`, outside `owner`'s callback. The `later.Go`
literal is rule 4, because `later` is declared outside it. Both go into
testdata verbatim.

Not supported, and documented in `rules.md`: reflection, `unsafe`, cgo, and the
opaque option-flow forms described above.
The runtime owner check in §3 is the backstop. Add `"NewBrowser"` to
`isWorkflowOperation`. Testdata goes in `testdata/src/browserchecks/` with one
`// want` per reported form and unflagged cases for each allowed form. Stub
`NewBrowser`, `Browser`, `WithBrowser`, `Scope`, `Group` and `Iterate` in the
testdata `github.com/.../gimbal` package as needed.

## 6. Evaluator migration (`internal/workflows/validateproduct`)

This lands after §1–5. Services and readiness stay as they are, in the root
body before the testers. Each tester creates its own browser in its own scope.
The three explicit slots stay; tester2 and tester3 repeat this with indexes 1
and 2:

```go
users.Go("tester1", func(ctx context.Context) error {
    gimbal.Set(ctx, "assignment file", suite.Workloads[0].AssignmentFile)
    gimbal.Set(ctx, "screenshots directory", dirs[0])
    gimbal.Set(ctx, "product URL", suite.Workloads[0].URL)
    var taskErr error
    scopeErr := gimbal.Scope(ctx, "browser-session", func(ctx context.Context) error {
        browser, err := gimbal.NewBrowser(ctx, "browser", dirs[0], filepath.Join(dirs[0], "video.webm"))
        if err != nil {
            return err
        }
        tester := gimbal.NewSession(ctx, "product-operation", suite.Workloads[0].Workdir)
        start := time.Now()
        text, err := tester.Generate[gimbal.Text](ctx, userPrompt, gimbal.WithBrowser(browser))
        reports[0].ElapsedSeconds = time.Since(start).Seconds()
        if err == nil {
            feedback, feedbackErr := tester.Generate[gimbal.Text](ctx, experiencePrompt)
            text += "\n\n" + feedback
            err = feedbackErr
        }
        taskErr = errors.Join(err, os.WriteFile(reports[0].Report, []byte(text), 0644))
        return nil // retain the task failure separately from browser startup/cleanup
    })
    // The scope has ended: video-stop ran, the WebM is non-empty, the browser is closed.
    var encodeErr error
    if scopeErr == nil {
        encodeCtx, stop := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Minute)
        encodeErr = videoError(gimbal.RunCommand(encodeCtx, "encode-video", dirs[0], "ffmpeg", ffmpegArgs(dirs[0], reports[0].Video)...))
        stop()
    }
    err := errors.Join(taskErr, scopeErr, encodeErr)
    turns[0] = err
    reports[0].Error = errorText(err)
    return nil // one tester's failure never cancels its siblings
})
```

`ffmpegArgs` holds today's arguments, and `videoError` folds the exit code
and stderr into an error. Both are ordinary unexported helpers. Recording is
finalized before processing (claim 14), because `Scope` returns only after
`scope.end` has run `video-stop` and `test -s`. ffmpeg then runs in the
environment, where the image's ffmpeg has libx264. This first evaluator slice bounds each
encode at two minutes; unusually long or concurrent recordings can exceed that
budget and return an encoding error while retaining the finalized WebM.
Task failure, including confirmed cancellation, does not discard a successfully
finalized recording: the task error is retained while bounded encoding runs
outside cancellation. Browser startup/cleanup failure prevents processing and
is reported. A removed environment cannot supply a finalized video, and its
failure remains explicit. Keep the per-turn failure in its existing turn record
and in the workload/run result even though the inner resource scope returns only
its startup/cleanup failure.

**Inputs visible to the agents.** Before the testers start, one command
`check-inputs` runs in `output`:
`zsh -c 'for f; do test -r "$f" || { echo "not readable in the environment: $f" >&2; exit 1; }; done' zsh <guides...> <assignment files...>`.
A guide or assignment outside the mounts then fails the run with its path. It
is not left unreadable to a tester that passed host-side validation.

Delete these:

- the `exec.LookPath` calls for the driver and ffmpeg;
- the `opened`/`recording` arrays, the `names` slice and the `record-browser` command;
- the `closeBrowsers` closure, its defer and its call after `Wait`;
- the host `exec.CommandContext` ffmpeg loop;
- the "browser command" `Set`;
- `Suite.PlaywrightCLI` and its `playwright_cli` config key.

`userPrompt` loses its browser-command wording, because the access block
supplies it. It tells the agent to `goto` the "product URL" value first.
The screenshot directory also contains the connector's hidden wrapper and
`.playwright-cli/` logs. Those are expected operational files, not screenshot
evidence; visual review consumes the image files.

**Report-only mode.** `issue_repo` becomes optional. It is validated with the
same pattern only when it is non-empty. Then:

```go
if suite.IssueRepo == "" {
    findings, err = triage.Generate[gimbal.Text](ctx, reportPrompt)
} else {
    gimbal.Set(ctx, "issue repository", suite.IssueRepo)
    findings, err = triage.Generate[gimbal.Text](ctx, triagePrompt)
}
```

`reportPrompt` is a new constant. It asks for the same findings.md content,
with screenshots cited by local path. It says not to use `gh`, create issues,
or upload anything. Live validation runs with `issue_repo` omitted.

**Package doc.** The prerequisites become the environment's: `playwright-cli`
with its configured browser, `ffmpeg` with libx264, and `zsh`. `gh` and
`gimbal upload-artifact` are needed only when `issue_repo` is set. On the
Docker/Temporal backend all three roles must be bound to Codex models. The
example is the invocation in §8. `Params.SuiteFile`'s comment says the issue
repository is optional.

**Generated files.** The graph changes, so `go generate ./...` must produce a
diff: `workflow_gen.go`, because `record-browser`, the browser `Set`s and the
"browser command" Condition go, while `check-inputs`, the tester scopes,
browser nodes and `encode-video` arrive and every `Source` line shifts; and
`cmd/gimbal/validateproduct_gen.go`, which embeds the package doc and the
`--suite-file` help. Commit every file the generator changes. No diff would
mean the generator did not see the migration.

Tests:

- The fake `playwright-cli` is placed on PATH instead of `s.PlaywrightCLI`.
  It must accept `-s=`, `open ... --idle-timeout=0`, `video-start`,
  `video-stop` and `close`. `video-stop` writes the file.
- The case "one browser close failure preserves both videos" now asserts that
  the failing tester's error is in the run outcome, and that the other two
  produced mp4s.
- New case: report-only (no repo) never sends `triagePrompt`.
- New cases: a failed turn and a confirmed cancelled turn still produce MP4
  after successful recording finalization, while retaining their task errors.
- New case: a guide outside the readable set fails at `check-inputs`.
- The config test "missing repository" becomes "empty repository is
  report-only", and "malformed repository" still errors.
- `TestScreenshotClaimsStayGrounded` also covers `reportPrompt`.

## 7. Packaging (track B)

**Worker binary, injected separately (claim 7).** Add a field to
`execution.Config`:

```go
WorkerBinary string `json:"worker_binary"` // absolute host path to a static linux gimbal-worker
```

`New` requires it: absolute, a regular file, executable. Otherwise it returns
an error naming `worker_binary`. `startWorker` adds `-v
<WorkerBinary>:/opt/gimbal/gimbal-worker:ro --entrypoint
/opt/gimbal/gimbal-worker` and keeps the `worker` argument, so the image's own
entrypoint is irrelevant. The consumer image never contains the worker.
`cmd/gimbal`'s `serverFlags.options` required-field check adds `worker_binary`.

**Startup validation, no protocol version.** The supported recipe builds the
worker from the same checkout as the controller, so there is no version
mechanism. Startup validates the binary as it is. `New` checks the file, and
the existing readiness wait requires the worker's Temporal poller with the
owner identity. A wrong-architecture or broken binary never polls. The
readiness failure already carries `docker logs` (for example `exec format
error`) and removes the container. Its message also names `worker_binary`.

**Worker build artifact.** Operator requirements: Linux, the Docker server's
architecture (arm64 on this Docker Desktop host), static (`CGO_ENABLED=0`),
and the same checkout as the `gimbal` controller binary. The Justfile gets:

```just
worker-binary out="bin/linux/gimbal-worker":
    CGO_ENABLED=0 GOOS=linux GOARCH="$(docker version -f '{{{{.Server.Arch}}')" go build -o {{out}} ./cmd/gimbal-worker
```

`bin/` is already gitignored. The worker needs no web build.

**Consumer image: external.** The prepared image (claim 8) is
`gimbal-browser-evaluator:local`
(`sha256:88eafaea22c20a9250c44ac98505c0a3b97e06168cf8e42a3e658abb572e720f`,
linux/arm64). It is built from `/private/tmp/gimbal-browser-build.AfTh8P/image/`,
and `image/README.md` records its build command and probe. It uses
`node:24.21.0-bookworm-slim`, `@openai/codex@0.157.1`, `@playwright/cli@0.1.21`
with `chromium-headless-shell` in `/ms-playwright`, and
`PLAYWRIGHT_MCP_CONFIG=/etc/playwright-cli/cli.config.json` (`browserName:
chromium`, headless). That setting is required: without it the CLI picks the
`chrome` channel, which does not exist for Linux arm64. It also has
`http-server@14.1.1`, TodoMVC in `/opt/todomvc`, apt `ffmpeg` (libx264),
`zsh`, `git`, `procps` and `curl`, and no Gimbal binary. Worker commands and the
Codex shell inherit the image environment, since the worker sets no `Env`.
Track B builds and tracks no image.

**The old baked-worker recipe goes.** Delete `Dockerfile.gimbal-worker` and the
compose `worker` service (profile `manual-worker`), which built the worker into
an image. `docker-compose.temporal.yaml` keeps Postgres. The integration tests
read `GIMBAL_TEST_WORKER_IMAGE` and `GIMBAL_TEST_WORKER_BINARY` with
`requiredEnv`, with no defaults naming deleted images. The Codex lifecycle test
needs an image with Codex, such as the evaluator image. Update the `temporal.go`
doc comment and `ephemeral/temporal-backend-api.md` "Local startup" to use
`just worker-binary` plus an external image.

**Secrets.** The only supported mechanism is the existing
`secret_files: {"OPENAI_API_KEY": "<file>"}`, mounted read-only. Real
authentication waits on the user's choice of that file. No host login state
(`~/.codex`, browser profiles) is copied or mounted, and no new auth API is
assumed. With Codex roles configured, missing credentials fail worker startup
before it polls Temporal. No product service, browser or evaluation operation
runs in that case. The external image-only probe is separate evidence and is
not a pre-auth Docker/Temporal evaluator run.

## 8. Live configuration and invocation (claims 12, 13, 15)

All shared data is under canonical `/private/tmp` paths, because
`CanonicalProject` resolves symlinks and mounts are compared as paths.

**Backend config**, `/private/tmp/gimbal-browser-build.AfTh8P/execution.json`,
is the manager's draft with two corrections. `artifact_dir` is dropped:
`ArtifactDir` is `json:"-"`, so the factory sets it and the key is silently
ignored. `mounts` is narrowed to the evaluation directory, so `temporal.db`,
the image context and the worker binary are not writable by agents:

```json
{
  "environment": "browser-evaluation",
  "docker_image": "gimbal-browser-evaluator:local",
  "worker_binary": "/private/tmp/gimbal-browser-build.AfTh8P/gimbal-worker-linux-arm64",
  "temporal_address": "127.0.0.1:7233",
  "postgres_dsn": "postgres://gimbal:gimbal@127.0.0.1:5433/gimbal?sslmode=disable",
  "worker_temporal_address": "host.docker.internal:7233",
  "worker_postgres_dsn": "postgres://gimbal:gimbal@host.docker.internal:5433/gimbal?sslmode=disable",
  "mounts": ["/private/tmp/gimbal-browser-build.AfTh8P/evaluation"]
}
```

`secret_files` is added when the user chooses the key file (§7).

**Mounts.** The one configured mount,
`/private/tmp/gimbal-browser-build.AfTh8P/evaluation`, holds all of these:

- inputs: `suite.yaml` and `task.md`, plus any guides, which must be placed
  here;
- the workspace: `workspace/`, the workload `workdir` and the tester session
  workdir;
- outputs: `results/`, the `output_dir`, which receives `user-testing-*/` with
  screenshots, `video.webm`, `video.mp4`, reports, `visual-review.md` and
  `findings.md`;
- run context files: `.gimbal/runs/<id>/`, with spilled `Set` values and
  artifacts. That directory is the project state dir, which the hosted factory
  also mounts through `WorktreeMounts`.

`worker_binary` and the secret file are mounted read-only by the backend.

**Suite**, `/private/tmp/gimbal-browser-build.AfTh8P/evaluation/suite.yaml`,
is used as is. It has one workload, `shopping-list`, with `task.md` and
`workspace/`. It starts `http-server /opt/todomvc -a 0.0.0.0 -p 7002 -c-1
--cors`, checks readiness with `curl -fsS http://127.0.0.1:7002/`, and sets
`url: http://127.0.0.1:7002/`, a 15-minute timeout and `output_dir:
./results`. It has no `issue_repo` (report-only) and no `playwright_cli`.
Relative paths resolve from the canonical suite path. The server and the
browser share the worker container, so the loopback URL works.

**Startup**, from the worktree root, one foreground process per terminal:

```sh
temporal server start-dev --ip 0.0.0.0 --db-filename /private/tmp/gimbal-browser-build.AfTh8P/temporal.db
docker compose -f docker-compose.temporal.yaml up -d postgres
just build                                                                       # bin/gimbal, the controller
just worker-binary /private/tmp/gimbal-browser-build.AfTh8P/gimbal-worker-linux-arm64   # same checkout
./bin/gimbal --execution-config /private/tmp/gimbal-browser-build.AfTh8P/execution.json \
  --instance-dir /private/tmp/gimbal-browser-build.AfTh8P/instance \
  --project /private/tmp/gimbal-browser-build.AfTh8P/evaluation
```

**Invocation**, against that running instance:

```sh
./bin/gimbal run validate-product \
  --instance-dir /private/tmp/gimbal-browser-build.AfTh8P/instance \
  --project /private/tmp/gimbal-browser-build.AfTh8P/evaluation \
  --suite-file /private/tmp/gimbal-browser-build.AfTh8P/evaluation/suite.yaml \
  --product-operation gpt-5.6-luna \
  --product-visual-review gpt-5.6-luna \
  --product-triage gpt-5.6-luna \
  --follow
```

All three role flags are required on this backend. The defaults (Claude Opus,
Gemini Flash, GPT-6 Astra) are not all Codex bindings, and `execution.New`
rejects any role that is not bound to Codex.

**Bounded supervision exercise (claims 10-11).** Add the opt-in
`TestSupervisedTurnIntegration` beside the existing real execution integration
tests. It runs ordinary `gimbal.Run` with principal and supervisor sessions in
the same configured environment/workdir, both using Codex `gpt-5.6-luna`, and
the existing supervision API. Give the principal a bounded workspace task
that remains active long enough for a look. The supervisor must inspect live
activity/workspace data, return an objection, and the principal must receive
that steering before finishing. Assert overlapping turns, shared environment
and workspace, activity reaching the look, and landed steering; retain useful
test diagnostics. This is a maintained integration test, not another product
workflow or committed proof program. It uses the existing operator environment:

```sh
GIMBAL_EXECUTION_INTEGRATION=1 \
GIMBAL_TEST_WORKER_IMAGE=gimbal-browser-evaluator:local \
GIMBAL_TEST_WORKER_BINARY=/private/tmp/gimbal-browser-build.AfTh8P/gimbal-worker-linux-arm64 \
GIMBAL_TEST_TEMPORAL_ADDRESS=127.0.0.1:7233 \
GIMBAL_TEST_POSTGRES_DSN='postgres://gimbal:gimbal@127.0.0.1:5433/gimbal?sslmode=disable' \
GIMBAL_TEST_OPENAI_API_KEY_FILE=/operator/chosen/key-file \
go test ./internal/execution -run '^TestSupervisedTurnIntegration$' -count=1 -v
```

The key path is an operator input, not a supplied credential. Do not run this
authenticated check until that input exists. The separate real-Temporal
command cancellation test requires no model credentials and uses the same
infrastructure environment without the key-file variable.

## 9. Claims map

| # | Where it is met |
|---|---|
| 1 | `NewBrowser(ctx, name, workdir, video) (*Browser, error)` §1 |
| 2 | `WithBrowser` access block with this handle's own wrapper, plus checks §1, §3 |
| 3 | Connector only in `browser.go`; the workflow never names playwright-cli §1, §6 |
| 4 | Adopted by the creation scope; `--idle-timeout=0`; outlives each Generate; released only in `scope.end` §2 |
| 5 | GIMBAL110, tracked by type §5; runtime owner check §3 |
| 6 | Close in `scope.end` before services and sessions. An unconfirmed cancellation removes the worker before close. Every failure joins the scope error §2 |
| 7 | `worker_binary` read-only mount plus entrypoint; file check plus readiness §7 |
| 8 | External probed image `gimbal-browser-evaluator:local` §7 |
| 9 | Unchanged: Go runs on the controller; only commands and turns cross the backend |
| 10 | Unchanged: worker and supervisor sessions share one environment and workdir (`validateSupervisorBindings`) |
| 11 | Unchanged: supervise looks and steer; browser activity remains observable, with explicit per-agent grants §3 |
| 12 | Scope context plus one mount covering inputs, workspace, run files and outputs; `check-inputs` §6, §8 |
| 13 | Migrated validate-product on Docker/Temporal, report-only, invocation §8 |
| 14 | `video-stop` + `test -s` before `close`, and `encode-video` after the scope §2, §6 |
| 15 | The tester body reads top to bottom; one config, one suite, one invocation §6, §8 |

## 10. Edit ownership and order

Tracks A and B are independent and can proceed in parallel. Track C starts
after A merges. The manager owns the external files under
`/private/tmp/gimbal-browser-build.AfTh8P` (config, suite, image), integration,
commits and live validation (Codex `gpt-5.6-luna`). No two tracks edit the
same file.

**A: browser core, generator and linter**

- `browser.go` (new) and `browser_test.go` (new). The tests use a fake backend
  and runtime and cover:
  - an open failure closes the browser and returns an error;
  - close runs before services, and its error is in the scope error;
  - the environment, owner and released checks;
  - an access block is appended by each agent's own options; supervisors also
    observe the worker's instructions in their quoted task/transcript;
  - two browsers in one workdir get distinct wrappers and sessions;
  - the wrapper refuses lifecycle subcommands and `-s`;
  - open passes `--idle-timeout=0`.
- `scope.go`: `browsers`, `adoptBrowser`, the `end` hook.
- `supervise.go`: `options.browsers`; keep existing supervision signatures.
- No `supervise_jev.go` signature change is needed.
- `session.go`: `dispatchRecorded`, and the joined adapter error in `Session.turn`,
  with a test.
- `doc.go`: one paragraph.
- `internal/generate/expr.go`, `internal/generate/read.go`, and their tests.
- `internal/gimballint/analyzer.go`, `rules.md`, `testdata/src/browserchecks/`,
  and the testdata gimbal stub. Include ownership propagation through
  WithSupervisor options and spread option slices.

**B: backend packaging and remote cancellation**

- `internal/execution/temporal.go`: `WorkerBinary`, `startWorker`, readiness
  message, `removeWorker`, the removed-environment state, and `Process.Wait`'s
  unconfirmed branch, `Process.Stop`, and shared cancellation/heartbeat timing.
- `internal/execution/harness.go`: `RunTurn`'s confirmed/unconfirmed decision
  plus `request` (create/fork/steer/close) and the removed-environment refusal.
- `internal/execution/*_test.go`, including `temporal_integration_test.go`
  (required image and binary env). Test that an unconfirmed turn removes the
  container and that later operations are refused.
  Also test confirmed cancellation against the real Temporal dev server without
  a model, and verify a subsequent command still runs in the same environment.
  Add `TestSupervisedTurnIntegration` for the separate authenticated exercise
  described in §8; an unset credential must skip honestly, not count as proof.
- `codex/codex.go` and `codex/codex_test.go`: the unconfirmed-interrupt error.
  Cover the finish-versus-interrupt race: a terminal turn remains confirmed
  when an interrupt reports that it already finished, and cancellation from
  the first terminal event's callback must not await a second terminal.
  Cover a nonterminal interactive refusal with a live ctx: interrupt/drain
  must occur before return, with an unconfirmed error if no terminal follows.
  Cover cancellation during turn/start: await its id, then interrupt/drain.
  Cover a genuinely ambiguous start acknowledgement so it cannot silently
  become a clean cancellation while native work may still run.
  Cover error {willRetry:false} and interactive requests during drain followed
  by turn/completed: these must remain confirmed and retain the environment.
- `cmd/gimbal/main.go` and `cmd/gimbal/execution_config_test.go`: the
  `worker_binary` required field.
- `Justfile` (`worker-binary`), delete `Dockerfile.gimbal-worker`, and remove
  the `worker` service from `docker-compose.temporal.yaml`.
- `ephemeral/temporal-backend-api.md`: local startup.

**C: evaluator (after A)**

- `internal/workflows/validateproduct/{validateproduct.go, config.go,
  prompts.go, validateproduct_test.go, config_test.go}`.
- Run `go generate ./...` and commit its diff, at least
  `internal/workflows/validateproduct/workflow_gen.go` and
  `cmd/gimbal/validateproduct_gen.go` (§6).

## 11. Known limits

- The wrapper guards the supplied command. It does not sandbox Codex, which
  runs `danger-full-access` and could call `playwright-cli` directly (§2, §3).
  Browser-driving commands such as `tab-close` or `run-code` can also end a
  page, and value-taking global flags can bypass the simple subcommand scan.
  This is not a promise to police every browser action.
- An unconfirmed cancellation on the Temporal backend ends the whole
  environment: other turns, commands and browsers in it fail with errors naming
  the removal. There is no restart.
- On the host backend, Gimbal does not own the machine's shared Codex daemon.
  An unconfirmed interrupt is returned as an error in the scope's result, and
  the browser is still closed. A turn that ignores the interrupt keeps running
  until Codex ends it. It cannot reopen the browser through its wrapper.
- On the host backend, a daemon that refuses `close` stays running, because the
  idle timeout is disabled. The close error names its session.
- One connector and one harness only; the Temporal backend is Codex-only.
- Supervisor context is bounded (lookIntro clips the task), with no promise
  of full-scope inheritance.
- Worker loss is not recovered: the browser daemon dies with the container,
  and the close error surfaces.
