# Issue 326 determination

## Determination

Issue 326 is not evidence that the prescribed single-process Gimbal viewer fails on an empty project. The retained first browser snapshot is a healthy `Empty project / No runs yet` page. The 500 appeared only after the validation assignment launched a second Gimbal runtime on port 43841 while the standalone viewer on 43840 continued polling the same project directory. That topology violated the single-process model Tyler confirmed for this investigation.

The failure is nevertheless understandable from current source. Every two seconds, the viewer invalidates the runs page. A live run owned by the same runtime is served from its in-memory registry. A run owned by another process is absent from that registry, so the page reconstructs it from disk. That reconstruction is not a safe read of an active foreign writer:

- It replays session JSONL files which may be observed after only part of the current record has been appended. A JSON decode error aborts the entire runs page.
- Before the writer's first 64-delta checkpoint, the foreign reader can write `observation.json` using its own random stream ID. A later read can combine that snapshot with the writer's delta file and fail with `observation: non-contiguous durable delta at 1`.

The retained artifacts cannot distinguish these mechanisms because skgo's data-request error path sanitizes the cause to `500 Internal Error` and, without a configured error handler, writes no underlying error to service stderr. The screenshot records the result, not the failed request or raw response. The original missing-table explanation is contradicted: all eight observation tables existed from run initialization, roughly 14.6 seconds before the screenshot.

This behavior was enabled deliberately by PR #283's foreign-run disk fallback and active-foreign-run test, then exercised operationally by PR #306's standalone-viewer/delegated-listener guidance. The test serializes mutations and renders, so it proves intended foreign-run semantics without testing concurrent file I/O. That is implementation and workflow drift away from the now-settled single-process model.

## Recommended disposition

Do not repair Issue 326 by making foreign active-run replay tolerant. That would preserve an invalid process model and risks hiding real corruption. Reframe the issue as a validation-topology defect, remove the two-listener guidance, and make one Gimbal runtime own both workflow execution and live observation.

Before declaring the supported path race-free, run one focused reproduction: hold a single-process run after its directory and `run.jsonl` are published but before its observation store is registered, then request the runs page. Source inspection proves this narrow publication window exists. Issue 326 does not prove it fired, and normal post-registration requests bypass disk entirely. The result should decide whether initialization order also needs a small same-process fix. A separate dual-process reproduction is useful only as forensic confirmation of which unsupported race produced the historical 500.
