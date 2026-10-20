# Durable Interactive implementation review, round 01 (Opus)

## Review target

- Repository: /Users/tyler/.codex/worktrees/durable-interactive-spec/gimbal
- Range: `ed50f6bd..514d012f`. Started against pinned commit `5a009325`;
  extended at the requester's instruction to include `5a009325..514d012f`
  (5 files, +53/-6: `CancelTurn` routing of interactive turns through durable
  STOP, plus tests).
- Intent: `specs/durable-interactive.md` (nlspec), the
  `ephemeral/research/durable-interactive/DELIVERY-PLAN.md` delivery plan, and
  `AGENTS.md` at the target commit.
- Claimed observations: `/tmp/gimbal-durable-interactive-proof-context.md`,
  re-read after its final update (it names 514d012f as the final source).
- Scope was not narrowed. The review was performed directly, not delegated.

## Evidence inspected

- The skills: spec-writing, spec-review, adversarial-review.
- The spec in full, especially sections I (input delivery, finish, stop),
  J (scope/run termination), the Run contract, claims C1–C12 and L1–L5, and
  the budget table.
- Production sources at the target commits, read with `git show <commit>:path`
  because the worktree changed during the review:
  - Root package: `interactive.go`, `run.go`, `group.go`, `scope.go`,
    `session.go`, `command.go`, `event_persistence.go`, `service.go`.
  - Internal packages: `internal/runstate/store.go`, `internal/host/host.go`,
    `internal/runner/{runner,launch,daemon}.go`,
    `internal/conversation/{requests,manager}.go`,
    `internal/workflows/conversation/conversation.go`.
  - Admission: `gimballint` admission and effects, and
    `internal/generate/graph.go`.
  - Web: `web/src/routes/interactive.remote.go`, `control.remote.go`,
    `RunQuestions.svelte`.
  - Example: `cmd/examples/interactive/main.go`.
- Existing tests: `interactive_test.go`, `internal/generate/admission_test.go`.
- Budgets: handwritten production lines at 5a009325, counted with a scratch
  script that excludes tests, fixtures, mocks, stories and generated code.
  - J 1,048 / 1,400
  - E 7,753 / 9,000
  - I 574 / 900
  - H 7,151 / 8,500
  - P 13,924 / 14,800
  - A 12,262 / 13,200

  All are under their maximums. The +53/-6 final diff does not change that.
- Reproductions: written as scratch tests in a copy of 514d012f extracted
  outside the repository (`git archive 514d012f`). They use the package's own
  `interactive_test.go` harness. Nothing was added to the repository. The
  results are quoted under each finding.

## Additional diff 5a009325..514d012f

Before this diff, a Stop from the run view went through `CancelTurn`, which
canceled the underlying Generate directly, so Interactive returned a terminal
context-canceled error. That violated "does not … return an Interactive error
merely because Stop was requested" (spec, STOP paragraph after I).

At 514d012f, `run.CancelTurn` (`run.go:494`) sends a matching interactive turn
to `acceptInputAtTurn(..., Stop: true, ...)`. That function (`interactive.go:439`)
checks the expected turn under `interactionMu`, so an old control cannot stop a
later turn. That defect is fixed.

Nit, not ranked: each call mints a fresh `NewRecordID()`, so repeated Stop
clicks are idempotent only because of the turn check, not because of input
identity. The proof context states that this commit had no rebuilt binary and
no paid native or browser run.

## Findings

### 1. Critical: `Run` returns nil (success) at dormancy, contradicting the Run contract

- **Evidence:**
  - In `run.go:160-167` the suspension recover sets `r.suspended`, stores the
    run in `dormantRuns`, and sets `result = nil`.
  - The godoc at `run.go:133-134` says the same: "At an explicit durable wait
    it releases authored execution and returns nil while the run remains
    unfinished."
  - The spec (`specs/durable-interactive.md:442-451`) says the opposite: "Run …
    blocks until actual logical completion or settled cancellation/error.
    Dormancy does not return success … A direct Run without installed ingress
    remains pending at an Interactive wait until ingress is provided or its
    caller cancels."
- **Reproduction** (scratch test at 514d012f): Interactive with no input,
  called through `Run` once. Output:
  `Run returned nil while state=waiting and EndedBy=""`.
  The existing tests in `interactive_test.go` assert that this nil return
  happens, so the deviation is built into the suite.
- **Impact:**
  - Every direct caller of the public API sees success for an unfinished run.
  - `gimbal.Run(...); exit 0` reports a completed workflow that has not
    completed.
  - Each host has to re-implement the contract by polling `State()=="waiting"`
    and `Runnable()` every 100 ms:
    - `internal/runner/runner.go:176-188`
    - `internal/host/host.go:428-440`
  - That polling loop does not consult `ctx.Done()`
    (`internal/runner/runner.go:180`), so the spec's "caller's deadline keeps
    running during dormancy" depends on each host noticing separately.
  - This is the central public behavior of the feature (L-level Run claim), and
    it is inverted.

### 2. Critical: an in-flight steer is redelivered as a new turn's prompt (duplicate delivery)

- **Evidence:**
  - `interactive.go:496-499` records the input with `Disposition = "pending"`.
  - `interactive.go:521-523` releases `interactionMu`, calls
    `active.session.Steer(...)`, and only afterwards records `delivered` or
    `uncertain` (`interactive.go:524-532`).
  - Nothing marks the input claimed, in flight, or uncertain before the provider
    receives it.
  - If the turn ends while `Steer` is in flight, the post-turn loop
    (`interactive.go:245-259`) and the next iteration's `pendingInput`
    (`interactive.go:372`) still see a `pending` input and send it as the next
    turn's prompt.
  - The same happens after a crash between the provider accepting the steer and
    `recordFact`: a restarted activation sees `pending`.
  - The spec (`:351-353`) says: "If delivery is uncertain, preserve that
    uncertainty and reconcile before redelivery. Neither a missing
    acknowledgement nor a restarted activation establishes that the message was
    not delivered."
  - C7 says: "uncertain delivery is not blindly repeated".
- **Causal sequence:**
  1. The human sends "two" during turn 1.
  2. `acceptInputAtTurn` commits `pending`, unlocks, and enters
     `Steer` (provider accepts).
  3. Turn 1 returns chat; Interactive relocks, finds "two" still `pending`, and
     dispatches turn 2 with "two" as its prompt.
  4. `Steer` returns `landed=true` and the disposition becomes `delivered`, but
     too late.
- **Reproduction** (scratch test at 514d012f): an adapter whose `Steer` blocks
  until `RunTurn` has returned. Output:

  ```
  steered=["EXTRA"]
  turn 1 prompt contains EXTRA: false
  turn 2 prompt contains EXTRA: true
  input "EXTRA" was delivered twice: once by steer, once as turn 2's prompt
  ```

- **Impact:** the agent receives the human's message twice: it has already been
  steered into the native thread and it is sent again as a turn. This breaks the
  delivery guarantee the inbox exists to provide. A crash makes it
  deterministic, not only racy.

### 3. Issue: Finish does not close ingress; later messages are accepted and acknowledged

- **Evidence:**
  - `acceptInputAtTurn` (`interactive.go:439-504`) rejects only when
    `r.State()=="terminal"` or the owner scope or run has ended. It does not
    check for an accepted Finish pending at the same address.
  - The absent-execution path `runstate.SubmitInteractive`
    (`internal/runstate/store.go:62-`) checks only that the scope is known, not
    ended, and has no ID conflict. It also has no Finish check.
  - Undelivered messages become `unprocessed` only in `closeInteractionInputs`
    (`interactive.go:567-590`). That runs at scope or run end, not at human
    completion: the human-finish return at `interactive.go:261-266` does not
    mark them.
  - The spec (`:373-382`) says: "New messages after the durable finish boundary
    are rejected as closed … Finish closes ingress to new ordinary messages …
    Earlier accepted but undelivered messages become visibly unprocessed at
    human completion."
- **Reproduction** (scratch test at 514d012f): wait, accept a Finish, then
  accept a message. Output:
  `message after durable finish was acknowledged instead of rejected as closed`.
- **Impact:**
  - The UI and the API answer "accepted" for a message that will never be
    processed.
  - In an ordinary workflow that calls `Interactive` again at the same stable
    address (the conversation workflow does, after an agent end), the
    post-Finish message stays `pending` and can be consumed by the next
    occurrence. That is the opposite of closure.

### 4. Issue: Stop racing turn completion silently discards a completed native reply

- **Evidence:**
  - At `interactive.go:268-277`, if a Stop was accepted and
    `turnErr == nil`, the loop commits a wait and `continue`s. The successfully
    decoded `output` (chat or end) is dropped.
  - The `chat` and `end` handling at `interactive.go:293-310` is never reached.
  - A Stop accepted after the provider produced its final message, but before
    `dispatch` returned, causes `active.cancel` (`interactive.go:518`) to have
    no effect, while the Stop input is still present in `pending`.
  - The spec says STOP "after confirmed cessation, records a wait" (`:393-395`)
    and the Stop row (`:568`) says "returning to wait after confirmed
    cessation". Here there was no cessation. The turn completed, and its
    committed native reply exists in the provider thread but not in the
    transcript.
  - An agent `end` in the same window is converted into another wait, so the
    agent's decision is lost.
- **Impact:**
  - The transcript and the native thread diverge. The next turn's model sees an
    assistant reply that the human and the returned transcript never saw.
  - A clicked Stop can override an agent end that had already happened.
- **Reproduction recipe** (not run): an adapter whose `RunTurn` waits on a
  channel after deciding on chat. Accept Stop through `acceptInputAtTurn`
  with the current turn ID, then release the channel. The transcript has no
  assistant message for that turn.

### 5. Issue: after any suspension the live-runtime release is never called

- **Evidence:**
  - `run.go:171-175` skips `release()` when the run suspended.
  - On re-entry, `beginRun`'s dormant path (`run.go:207-216`) returns
    `func() {}` as the release. The closure from the first activation
    (`run.go:290-293`) is no longer referenced.
  - So for any run that waited even once, the hook's deregistration never runs,
    even after the run completes. The code comment at `run.go:286-289` says the
    run "leaves the runtime's table as soon as the body returns".
  - The hosts register through `live.WithHook`: `p.runs.Hook` in
    `internal/host/host.go:418`, and `internal/runner/runner.go:106`.
- **Impact:**
  - Completed interactive runs stay in the in-process runtime table, the source
    of `runs.InProgress` and controller resolution, for the life of the
    process.
  - Run views and the web remotes keep resolving a terminal controller as
    in progress.
  - The table grows with every conversation in a long-lived shared host.
- **Reproduction recipe** (not run): count hook registrations and releases
  across `Run` → wait → input → `Run` → complete. Expect one registration and
  zero releases.

## Residual risks (not ranked)

- `host.WorkflowInput` falls back to `runstate.SubmitInteractive`
  (`internal/host/host.go:170-172`) without scheduling recovery. It relies on
  the startup invocation scan having already respawned the run. The daemon path
  (`internal/runner/daemon.go:203-205`) does call `recoverRun`.
- Dormant re-entry reuses the in-memory `*run` and ignores the `models` passed
  to the new `Run` call (`run.go:207-216`). The spec says re-entry uses "the
  supplied body/bindings".
- Root cancellation for a dormant run is held only in memory.
- `internal/conversation/manager.go` defaults Codex to the hard-coded
  `gpt-6-luna`. AGENTS.md requires unversioned aliases. Whether this default is
  new in the range was not established.
- Gates: the proof context assembles a green result from several partial
  attempts and says it is "not described as one uninterrupted green command".
  The final 514d012f routing fix had no rebuilt-binary, native or browser run.

## Outcome

material findings remain
