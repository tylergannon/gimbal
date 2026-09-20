# Sprint 002 (Claude draft): Claude Generate waits through background completion

Bug 317. Plan only; no product code changes here.

## Pyramid Index

- L0: A Claude `Generate` returns only on a declared terminal answer, proven
  by two real sequential `Generate` calls with different types on one Session.
- L1: state envelope around T; one process per adapter turn, read past waiting
  results; native wakeup only; next call resumes in a fresh process with its
  own schema; cancellation and failure never promote a waiting answer.
- L2: Contract, Boundaries, Acceptance workflow, Definition of done, Failure
  cases, Unresolved.

## Outcome

Today `waitTurn` (`claude/claude.go:329-336`) returns the first native
`result` with subtype `success`. That result can be Claude saying "I'll wait",
after which `RunTurn`'s deferred `client.Close` kills the background work and
the next call reads an orphan empty result. After this sprint the first result
is no longer special: the adapter keeps the same process and reader until the
model declares the assignment complete, then closes. The public API is
unchanged: `RunTurn`, `Generate[T]`, `TurnResult` keep their shapes.

## Contract

No native field separates waiting from done (`sol-semantics.md` § Candidate
completion rules), so the declaration is ours, carried in the native schema.

**Envelope.** For every Claude turn the adapter sends, as `--json-schema`:

```json
{"type":"object","required":["state"],"additionalProperties":false,
 "properties":{"state":{"enum":["waiting","completed"]},"value":<T's schema, verbatim>}}
```

T's schema is embedded unmodified, so native validation of T still happens.
For `Text` the `value` schema is `{"type":"string"}`. A constant paragraph in
the adapter, appended to the prompt, states the rule: answer `waiting` only
while something you started will wake you; answer `completed` with `value`
once the whole assignment is done, not merely the last notification.

**Reading rules**, in order, for each native `result` in the process:

1. subtype not `success` → the call fails (as now), whatever came before.
2. `success` without `structured_output` → not an answer (the orphan result
   seen on resume). Emit it for observation, keep reading.
3. `state:"waiting"` → emit it, keep reading. Nothing is sent to Claude.
4. `state:"completed"` → `TurnResult.Output` is `value`. A missing `value`
   yields `null`, which fails `ValidateJSON` and takes Generate's existing
   bounded re-ask. Then close the process.

**Attribution.** A process receives exactly one host prompt (plus any steers
of that same call), so every result it emits belongs to this logical call;
`origin` and `result_index` are recorded for observation, not used to decide.
Rule 2 covers the one known cross-call leak. This is the honest limit: the
declaration is a model claim. Field parsing tells us Claude *said* completed;
truth is checked by the workflow against evidence it owns.

**Cancellation and failure.** The waiting value is never stored as a candidate
result, so there is nothing to promote; see Failure cases.

## Implementation boundaries

- `claude/claude.go`: envelope construction, the prompt constant, `waitTurn`
  loop per the rules, unwrap `value`. `client.Close` stays deferred in
  `RunTurn`, which now simply returns later.
- `claude/events.go`: no structural change. `p.report` is already overwritten
  per result and Claude's `modelUsage` is cumulative per process, so the last
  result's report covers the call. Confirm, do not redesign.
- `session.go`: untouched. A re-ask remains a new adapter turn in a fresh
  resumed process; ownership is per adapter turn, not per `generate` loop.
- Regression tests beside the adapter (`claude/claude_test.go`): scripted
  message sequences into the wait loop, one per adapter-owned row of Failure
  cases, plus waiting→completed returning the second value.
- Out: session-lifetime process, a server, disabling background tasks,
  unrestricted payloads, service survival across calls, Codex, new exported
  names, dashboards, raw retention.

## Temporary acceptance workflow

Lives in `/private/tmp/gimble-317-acceptance/` with its logs; never committed.
A Go program using real `gimble.Run`, one `NewSession` on
`claude-haiku-4-5-20251001`, under a `context.WithTimeout` (about 5 minutes).

1. The program mints two random tokens. `TOKEN_A` goes in prompt 1 only.
   `TOKEN_B` is never shown to the model: the program writes a script outside
   the workdir that sleeps ~45s, writes a `done` marker with a timestamp, and
   prints `TOKEN_B`.
2. `Generate[JobReport]` (fields: `output string`, `exitCode int`): run the
   script with Bash `run_in_background: true`, do not poll, do not block on
   TaskOutput, say you are waiting, report the script's output when notified,
   and remember `TOKEN_A`.
3. `Generate[Recall]` (fields: `token string`, `count int`; incompatible with
   `JobReport`): "what token did I ask you to remember?" with no token in the
   prompt, scope, or any file.

The program asserts, from the Session's own event stream and return values:

- a `waiting` result was observed **before** the marker's timestamp, and
  Generate 1 had not returned at that moment (waiting genuinely exercised);
- the marker timestamp precedes Generate 1's return, and
  `JobReport.output` contains `TOKEN_B` (value reflects real task output;
  elapsed time alone proves nothing);
- the completing result carries `origin.kind == "task-notification"`, and the
  transcript shows no host message, no TaskOutput call, and no polling reads
  between waiting and completion;
- `Recall.token == TOKEN_A`, and the native conversation ID is identical
  across both calls while the process IDs differ.

Any assertion failure, provider error, or deadline exits nonzero after
killing the script and closing the run. If Haiku never emits `waiting` (it
blocked, or finished synchronously), that is **fixture failure, exit nonzero**,
not a pass. Against current `main` this must fail: Generate 1 returns before
the marker exists. The root agent records that baseline.

## Definition of done

1. The workflow passes on the fixed build and failed on the baseline, same
   program, unedited between the two.
2. The adapter regression tests above pass; `go test ./...` is green.
3. A validator reads the program and raw logs and confirms each assertion is
   evidenced by native events, not by timing or by the model's prose, and
   that neither the program nor the prompts were softened to pass.

## Failure cases

| Case | Required behavior |
| --- | --- |
| Waiting, then ctx cancelled | `ctx.Err()`; never the waiting text |
| Waiting, then provider error result | Error; process closed |
| Waiting, then process dies / EOF | "stream ended without a result" |
| Background task exits nonzero | Not an adapter error; Claude wakes and reports it through T |
| `completed` without `value`, or invalid T | Existing bounded re-ask |
| Orphan empty result at start of a resumed process | Skipped, still observed |
| Waiting with nothing that will wake it | Pends until caller's ctx ends (see Unresolved) |
| Persistent service left running at `completed` | Dies with the process; accepted this sprint |

## Unresolved decisions

1. **Envelope on `Text` too?** Recommended yes: issue 317's failing turns were
   text turns. Cost: text answers arrive as structured output. Validator must
   see one text-turn regression test either way.
2. **A `failed` state?** Recommended no. T already carries task failure; a
   third state adds an error path nobody asked for.
3. **Waiting with no active native task.** Failing from a
   `background_tasks_changed` snapshot contradicts "observation may degrade;
   execution must not". Recommended: log it, let ctx bound it.
4. **Does Claude's native validator accept an embedded arbitrary T schema**
   (`$defs`, `$ref` at a non-root path)? Assumed yes from probe A's envelope;
   must be seen with one generated Gimble schema before building on it.
5. **Steers during waiting** produce extra results in the same process. The
   rules classify them by `state`, so no special case is planned; unverified.
6. **Orderly close**: stdin close first, SIGTERM fallback, per
   `generate-lifetime.md`. Whether the SDK's `Close` already does this is
   unchecked.
