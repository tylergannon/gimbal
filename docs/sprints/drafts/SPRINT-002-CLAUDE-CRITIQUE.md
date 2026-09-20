# Sprint 002 critique (Claude)

## 1. Is waiting genuinely witnessed?

The fixture's core witness is sound: tap-stamped first result < `completion.json`
stamp <= host `first_returned`, plus a `task_notification` between results and a
single background Bash. That is ordering, not elapsed time. Gaps:

- **Assertion order.** `run.py:23-25` asserts ordering before `len(res)>=2`. A
  one-result run fails as "ordering invalid", not `INVALID FIXTURE`.
- **String witness.** `'WAITING_317' in json.dumps(...)` accepts any first
  result mentioning the string. Assert the declared state is waiting.
- **Witness does not survive the repair.** Codex's waiting branch "carries no
  final value", so `WAITING_317` has nowhere to go and the repaired run reports
  `INVALID FIXTURE`. T also has `phase: waiting|completed`, so two waiting
  vocabularies compete (envelope `completed` wrapping `phase: waiting` is
  schema-valid). Key the witness on the adapter's own declaration; the fixture
  cannot stay unchanged across baseline and repair unless designed for it now.
- **Baseline classification.** `run.py:18` exits with the workflow's code before
  any trace check. Auth failure, timeout, and `EARLY_RETURN` are all exit 1.
  Codex asks to record the failing assertion; Gemini's "fails prior to the fix"
  accepts any failure. Require `EARLY_RETURN` plus a first trace showing
  background launch then a waiting result.
- **Gemini's 6s sleep** is below the observed 10s to the waiting result, so
  waiting is never elicited. Its "return time after completion" check is the
  elapsed-time proof the intent rejects, and no observer is named.
- **Vacuous liveness check.** `os.kill(a['pid'],0)` runs after the workflow
  exits. Compare the first tap's `stdout_closed` to the second launch's `started`.

## 2. Does the terminal schema preserve refs?

Neither draft is concrete; Gemini is silent.

- `$ref: "#/$defs/X"` inside T is root-relative. Embedded under
  `properties.value` it resolves against the envelope root and breaks. State the
  rule (hoist `$defs` with collision handling, strip nested `$schema`) and test
  it beside the adapter with a T that uses `$defs`.
- The fixture's types have no refs, so live acceptance never exercises this.
  Give one fixture type a `$ref`.
- Three states imply a top-level `oneOf`. Structured output rides a tool input
  schema, which must be a top-level object; top-level `oneOf/anyOf` is likely
  rejected. Nest the union under one property, or host-check "completed implies
  value" as a bounded re-ask (validate to retry), not a fatal error.
- Gemini's Option B cannot work: arbitrary T has no state field, and requiring
  one is a public contract change. The fixture's `phase` is an accident. Drop B.

## 3. Can stale notifications be misattributed?

Yes, under Codex's contract as written. The token is "constrained in the native
schema", and `--json-schema` is fixed at process launch (`claude.go:166-168`).
An orphan wakeup generation in the resumed process is therefore forced to emit
the current token before it has seen the new prompt. A schema-const token equals
process identity and proves nothing. Put the token only in the prompt text,
constrain shape not value, and treat a mismatch as non-terminal. Unit test: an
orphan `completed` result arriving before the prompt's turn must not return.

Gemini's "matched to active turns" names no mechanism; the intent requires a
concrete contract.

## 4. Do failure semantics match scope?

- **Codex's `failed` state is new behavior.** It lets the model route around T
  and turns typed answers into Go errors for every caller. The intent covers
  cancellation and provider failure only. Two states suffice.
- **Gemini failure mode 1** fails the turn on a nonzero background exit. That is
  field parsing as semantic truth; Claude may legitimately report it inside T.
- **Waiting with nothing pending** hangs forever: stdin stays open, so no EOF
  arrives. Neither draft addresses it. Decide: caller deadline only, or one
  bounded re-ask.
- **Validation retries** (`session.go:176`) start a fresh process and kill live
  background tasks. Codex flags it; Gemini does not. Gemini also omits `Text`,
  which has no schema to carry state.
- **Fixture cleanup.** `subprocess.run(timeout=220)` kills only `go run`, not the
  compiled binary, Claude, or `slow.py`. Kill a process group, or cleanup is
  unmet on the timeout path.
- **Recall isolation.** `gimble.Project(ctx, dir)` and the session workdir are
  the same directory, so run records holding the token are agent-visible. The
  no-tools assert covers it; separate directories remove the question.
