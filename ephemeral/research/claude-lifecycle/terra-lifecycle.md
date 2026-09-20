# Terra: persistent listener, completion, and structured output

## Conclusion

The smallest viable contract is a **session-owned native Claude client, stream,
and one continuously running reader**, plus an explicit host-owned completion
decision. A native `result` with `subtype: success` means that one Claude
generation ended successfully. It does not prove that the workflow assignment
is complete: Claude can produce it while a background task remains and later
produce another success result whose `origin.kind` is `task-notification`.

For a bounded assignment, the host can mechanically require an explicit final
completion record for an assignment identifier, retain its outstanding work
set, and wait until every registered awaited item is terminal. The record's
truthfulness (including whether the model registered all work) remains a model
declaration; the host can only enforce its syntax, association, and accounting.
The native background-task snapshot/notification is useful evidence for
registered native tasks, but is not a general task-wide completion signal.
Persistent services must be an explicit non-awaited registration (or a scope
resource) with an explicit stop/ownership rule; otherwise a universal "wait
for all background work" deadlocks on an intended service. Scope cancellation
and session Close should resolve pending waiters as cancelled, never success.
Treat interruption/termination of all session-owned native work (especially
descendants) as a desired ownership policy to validate per platform and CLI,
not as a guarantee established by these probes.

## What must stay alive

These are separate lifetimes.

| Thing | Needed after a first result? | What closing does |
| --- | --- | --- |
| A `result` iteration boundary | Yes, the reader must continue past it | Returning from an iterator is only a host choice; it must not imply native completion. |
| SDK `Stream` | Yes, if it owns the queued input and reader | `Stream.Close` stops sends and receives on that stream, but leaves the SDK client connection active. |
| stdin | Yes, for later prompts/control requests | EOF ends input; it is not an idle marker. |
| SDK client/transport | Yes | `Client.Close` closes stdin and then waits briefly before killing the CLI process. |
| Claude CLI process | Yes, to receive native notification/resumption in the live session | Process death loses its live event channel and ends background work in the observed resume case. |
| native session transcript (`--resume`) | Sometimes | It restores conversation context only; it is not event replay or a guarantee that old native work survives. |

The pinned SDK says a `Stream` supports multiple rounds and `Messages()` runs
until its Close or context cancellation. Its implementation reads the client's
single shared message channel. Therefore one session should have **one reader**
which serializes prompt submission, event projection, result attribution, and
completion decisions; creating reader streams per `RunTurn` would race for
that shared channel. This is source-derived, not a claim that the SDK provides
durable reconnect.

Current Gimble instead creates a client and stream inside every `RunTurn` and
defers both closes ([`claude/claude.go`](../../../claude/claude.go), `RunTurn`).
It returns on the first successful `ResultMessage`; it therefore chooses
per-turn transport/process teardown. Its `onEvent` callback is supplied only
to that invocation, while `Session.turn` stamps it with that turn ID and clears
`activeEmit` when the invocation returns ([`session.go`](../../../session.go),
`turn`). A listener that keeps receiving while no caller is awaiting a turn has
no current turn callback/turn ID in this interface. A repair needs a durable
session-level event route and explicit late-result attribution; it cannot just
leave the current goroutine alive.

## Live evidence: waiting is not assignment completion

All probes used the authorized cheap model `claude-haiku-4-5-20251001` through
Claude CLI `2.1.270`, in `/private/tmp/gimble-317-terra/`.

1. With one native stream and a fixed startup schema `{kind, value}`, a
   `sleep 9` Bash task ran in the background. Claude first returned successful
   `{kind:"waiting", value:"INITIAL"}`. About 7.2 seconds later, the same
   process emitted `background_tasks_changed`, `task_notification`, then a
   second successful result with `origin.kind: "task-notification"` and
   `{kind:"complete", value:"TERRA_DONE"}`. A subsequent user prompt still
   worked in the same session. See
   `/private/tmp/gimble-317-terra/schema-late-notification/raw.jsonl` events
   51, 53-78, and 118.
2. Earlier controlled capture demonstrates the destructive boundary: after a
   result that had launched `sleep 35`, closing the client and later starting a
   resumed session produced `task_notification status:"stopped"`, summary
   "Background shell command didn't finish before the previous session ended",
   followed by a successful, empty task-notification result. The requested
   follow-up then ran separately. See
   `/private/tmp/gimble-317-investigation/sdk-structured/raw-1.jsonl` and
   `raw-2.jsonl`. This proves neither lossless delivery nor survival across
   that close/resume path.
3. A task that called `TaskOutput(block=true)` before ending produced a single
   non-notification successful result only after the task notification
   `status:"completed"`; see
   `/private/tmp/gimble-317-investigation/sdk-wait/raw-1.jsonl`. That is a
   useful in-turn workaround, not a general persistent-service policy.

One loss-minimization option is for the listener to persist enough native
identity before projection with a durable monotonic host sequence: native
`session_id`, result UUID/index, notification task ID, and origin. This is a
proposed host design, not an established Gimble API requirement; the repository
has deliberately not adopted general raw retention. The listener must still
associate a result with a submitted prompt only after its native evidence
identifies it. `origin` distinguishes the demonstrated automatic completion
result from an ordinary user result.
It does not identify the host assignment, and `result_index`, stop reason,
idle time, a task snapshot, and a schema-valid payload have counterexamples.
For example, a model may declare waiting with no native task, finish while a
persistent service lives, or omit a task registration entirely.

On context cancellation, current adapter behavior calls `InterruptWithReceipt`
and waits a five-second grace interval; receipts can name prompts still queued.
The listener must mark those prompts as queued/cancelled according to a
receipt, rather than assume interruption made later results impossible. A
notification racing the next prompt must be consumed by the one serialized
reader, recorded as notification-originated, then wake the assignment waiter;
it must not be handed to the next `Generate` merely because it arrived first.

## Structured output: observed constraint, not claimed protocol impossibility

The adapter currently maps each requested schema to CLI `--json-schema` when
it creates the process. The pinned Go SDK exposes `OutputFormat` as a client
option and has no public `SetOutputFormat`. Its wire `initialize` request does
contain `jsonSchema`, so this was tested directly rather than inferred from the
convenience API.

In a live CLI 2.1.270 process, I sent these native control requests in order:

1. `initialize(jsonSchema=A)` where A requires exactly `{a:string}`: the CLI
   acknowledged success and the first response was `{a:"FIRST"}`.
2. `initialize(jsonSchema=B)` where B requires exactly `{b:string}`: the CLI
   again acknowledged success, but the next answer was constrained to A,
   `{a:"{\\"b\\": \\"SECOND\\"}"}`.
3. `initialize(jsonSchema:null)`: literal JSON null (not omission) also
   acknowledged success, but a request for plain text returned
   `{a:"CLEAR_THIRD"}`.

See `/private/tmp/gimble-317-terra/control-schema/raw.jsonl`. A second probe
started a background task under A, received its ordinary waiting result, sent
an acknowledged `initialize(jsonSchema=B)` while idle, then received the late
notification result as `{a:"COMPLETE"}`. See
`/private/tmp/gimble-317-terra/control-schema-notification/raw.jsonl`.

This establishes **no verified schema-switch or clear path on this CLI
version**, including during the notification gap. It does not prove no future
or undocumented protocol path exists. A session-owned process consequently
cannot presently promise distinct native schemas for arbitrary successive
`Generate[T]` calls.

One possible design direction, not demonstrated behavior or an implementation
recommendation to adopt without product direction, is one stable native
envelope such as `{assignmentID, phase: waiting|completed|failed, payload}`.
Gimble could validate `payload` against each caller's T after receiving it.
That preserves native validation of the envelope and works with a persistent
process, but gives up native validation of the caller's exact T and still does
not make `completed` truthful by itself. A fixed per-session result type or a
process boundary for a changed schema are the other honest choices.

## Recovery boundaries and unresolved assumptions

`--resume` starts a new process with stored conversation/session context. It
does not reconnect to the old stdout stream, recover unrecorded events, keep
stdin open, or give exactly-once delivery. Persisted transcripts/output files
can reconstruct a partial record or support a compensating re-check, but not
prove the old event history was complete. A host crash after executing an
external effect but before recording an event also remains ambiguous.

Unresolved: the exact upstream guarantee for `origin`, task-status ordering,
and notification redelivery across every CLI version; behavior when several
notifications and user prompts are queued concurrently; and whether a future
CLI exposes a documented dynamic output-format control. These need versioned
live tests before becoming contract promises.

## Evidence locations

- Gimble current teardown/result handling: `claude/claude.go` (`RunTurn`,
  `waitTurn`) and `session.go` (`turn`).
- Pinned SDK stream/control/transport source:
  `/Users/tyler/go/pkg/mod/github.com/tylergannon/claude-agent-sdk-go@v1.1.1-0.20260912021749-9a4ffeca77cc/client.go`,
  `messages.go`, `protocol.go`, and `transport.go`.
- New bounded probes and captures:
  `/private/tmp/gimble-317-terra/schema_late_notification.py`,
  `control_schema.py`, `control_schema_notification.py`, and their sibling
  capture directories. All launched processes were terminated by their probe
  finally blocks.
