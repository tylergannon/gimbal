# Claude native event and completion semantics

Observed 2026-09-20 with Claude Code 2.1.270, model
`claude-haiku-4-5-20251001`, and the pinned Go SDK
`github.com/tylergannon/claude-agent-sdk-go@v1.1.1-0.20260912021749-9a4ffeca77cc`.
Raw captures and probe sources are under `/private/tmp/gimbal-317-sol/`; earlier
captures used for the reconnect comparison are under
`/private/tmp/gimbal-317-investigation/`.

## What a result ends

A native `result` ends one Claude generation. It does not necessarily end the
process, SDK iterator, connected stream, conversation, background tasks, or the
host's assignment. On a persistent SDK stream the process and stdin/stdout stay
open, `Stream.Messages` keeps yielding until close or context cancellation, and
later prompts and automatic task notifications cause later generations and
results in the observed background-shell cases. The SDK documents and
implements this multi-round behavior in
`client.go:504-533,656-725`. `Client.Close`, separately, cancels the message pump
and closes the transport/process (`client.go:536-559`).

The observed forced-wait lifecycle was:

1. one prompt launched three background commands;
2. `background_tasks_changed` accumulated one, two, then three active tasks
   (`/private/tmp/gimbal-317-sol/raw.jsonl:43,46,55`);
3. Claude emitted a schema-valid waiting `result`, subtype `success`,
   `terminal_reason:"completed"`, `result_index:0` while all three remained
   active (`raw.jsonl:83`);
4. a finite command failed, producing `task_updated status:failed`, a
   `task_notification status:failed`, and a snapshot with the other two still
   active (`raw.jsonl:85-87`);
5. the other finite command completed, producing the analogous completed
   update/notification and a snapshot containing only the persistent service
   (`raw.jsonl:107-109`);
6. the automatic wakeup generation emitted another schema-valid result with
   `origin:{kind:"task-notification"}`, `terminal_reason:"completed"`, and
   `result_index:1` (`raw.jsonl:158`);
7. a subsequently sent prompt emitted `result_index:2` on the same process and
   stream (`raw.jsonl:178`).

The natural-completion probe did **not** instruct Claude to answer early. It
assigned one required finite dependency and one intentionally persistent
service, declared completion to require observing the finite output, and told
Claude to leave the service running. Claude launched both (`natural-raw.jsonl:
31,43`), did not emit an early result, received the finite completion while the
service remained (`natural-raw.jsonl:173-175`), then emitted its first result as
schema-valid `completed`, `terminal_reason:"completed"`, `result_index:0`
(`natural-raw.jsonl:231`). The remaining service therefore cannot itself mean
"waiting"; it is valid assignment state both before and after completion.

## Candidate completion rules

| Rule | Result |
| --- | --- |
| first `result` / subtype `success` | Fails: the forced waiting result has both. |
| `terminal_reason == completed` | Fails: both waiting and completed results carry it. The SDK type calls this a terminal reason for a result, not an assignment (`messages.go:547-574`). |
| valid `structured_output` exists | Fails semantically: the waiting result is independently schema-valid. Validation proves shape, not truth or task completion. |
| no active background tasks | Fails generally: an intentionally persistent service remains after valid assignment completion. Conversely, a task can exist outside the native background-task facility. |
| all observed background tasks terminal | Fails for the persistent-service case and cannot prove that the model discovered every required dependency. |
| `origin.kind == task-notification` | Useful attribution, not completion: it identifies an automatic wakeup generation, whose model answer may still say waiting and whose triggering task may have failed while others run. |
| `result_index` increased | Useful ordering within a process invocation, not completion. It does not identify the host prompt or semantic task boundary. |
| model returns explicit `state:completed` | Mechanically deterministic if made part of the host contract, but it proves only that Claude made that declaration. The host must still decide whether to trust it or validate external claims. |
| explicit blocking `TaskOutput` for a known finite task | Survives for that declared dependency: the earlier capture shows task completion before the sole result (`sdk-wait/raw-1.jsonl:51,73-75,94`). It does not solve unknown dependencies or persistent services. |

There is no undocumented native field in these captures that distinguishes the
forced waiting result from an authoritative completion. Both can have success,
`stop_reason:"tool_use"`, `terminal_reason:"completed"`, valid structured
output, no queued turns, and live background tasks. Their discriminating field
was the model-authored `state`, which was requested by the probe and is not an
independent provider judgment.

The SDK also defines special terminal reasons including `background_requested`
and `tool_deferred` (`messages.go:560-565`). These may identify their particular
provider yield paths: the SDK comment says `background_requested` means the turn
itself was moved to a background task. They are not a complete waiting detector.
The demonstrated conversational waiting path used neither and instead emitted
`terminal_reason:"completed"` while its background work remained active.

Structured output is generated and validated per generation, not accumulated
as one task-wide object. The forced waiting object and later completion object
are separate full values (`raw.jsonl:83,158`). A later ordinary prompt also
gets a separate full value (`raw.jsonl:178`). A reconnect exposes the sharp
edge: after the old process was closed, resume emitted a stopped-task
notification followed by an empty successful task-notification result with no
`structured_output`, `terminal_reason`, or model turns before the newly queued
prompt produced its schema-valid result (`/private/tmp/gimbal-317-investigation/
sdk-structured/raw-2.jsonl:1,4,28`). Thus a configured schema does not guarantee
that every native result has structured output. The adapter currently rejects
missing structured output only after it has already selected the first result
(`claude/claude.go:206-222`).

## Races and failure semantics

Task snapshots are ordered observations, not an atomic completion barrier. In
the forced-wait capture the failure wakeup began before the success notification
arrived; the resulting model generation incorporated both and returned only
after both finite tasks were terminal (`raw.jsonl:85-109,158`). Another timing
could let it answer after the first notification while another finite task is
still running. Sending a host prompt during such a wakeup adds another producer
to the same stream. The observed protocol supplies UUIDs for individual events,
task IDs/tool-use IDs for task notifications, `origin` for task-notification
results, and monotonically increasing `result_index`, but no prompt ID on a
result. FIFO observation plus origin can classify these observed cases; it
cannot establish an exactly-once mapping between a host prompt and a result
across notification races, close, and resume.

Failure of a background task is a notification, not automatically an error
result. The failed command reported exit 7 (`raw.jsonl:86-87`), but the later
generation still emitted subtype `success` because the model successfully
produced a response (`raw.jsonl:158`). Assignment failure therefore requires a
workflow/host contract over which tasks are required and how their statuses
affect the task-wide outcome.

Closing after the first result is destructive to in-process background work.
The reconnect capture reports the old command as stopped because the previous
session ended, then emits the empty orphan result before the requested answer
(`sdk-structured/raw-2.jsonl:1,4,28`). Conversation resume reconstructs context
and surfaces this status; it is not lossless continuation of the old process,
stream, or task. The task output path can preserve output already written, but
does not recreate a killed computation. No observed primitive provides
exactly-once recovery.

## Smallest supported solution

Keep one client/process and one `Stream.Messages` consumer alive for the Gimbal
session rather than closing it after each result. Treat every native result as
the end of a generation, record its origin/index/usage, and continue listening
through the background-shell task notifications observed here. Serialize or
explicitly queue host prompts so observed notification generations cannot be
mistaken for prompt answers; other notification mechanisms require their own
evidence before assuming the same lifecycle.

The missing minimum declaration is task-wide completion owned by the workflow
or prompt contract, for example a required structured `state` (`waiting`,
`completed`, `failed`) plus the final task-wide value. A `completed` declaration
is mechanically recognizable; it remains a model claim and should be checked
against any deterministic workflow-owned evidence. When the workflow knows a
finite dependency, requiring Claude to block on `TaskOutput` avoids the early
result for that dependency. Persistent services must be declared as services,
since waiting for all background tasks would never finish them.

Unresolved: whether newer CLI versions add prompt identity or stronger task
lifecycle correlation; whether schema mutation works through a path beyond the unsuccessful
reinitialization/clearing probes documented in [Terra’s investigation](terra-lifecycle.md); the exact FIFO guarantee when a host prompt and notification arrive
simultaneously; whether all background mechanisms emit the same task events;
and whether task output files have a documented retention/recovery contract.
