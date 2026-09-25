# Antigravity harness and run-prompt

decision: Validation is the real `gimbal run-prompt` command invoking `agy`, with the emitted result and durable Gimbal events inspected; unit tests and builds are supporting checks.
decision: Port legacy behavior by hand against the current HarnessAdapter and event contracts; legacy is inspiration, not API authority.
friction: A fresh worktree's `go test -race ./...` fails in web tests because the embedded build lacks `skgo.manifest.json` -> run `just build` before the full Go suite so generated web assets exist.
decision: `agy` 1.2.1 rejects `/fork` in print mode and exposes no headless fork flag, so the adapter returns an explicit unsupported error instead of aliasing two Gimbal sessions to one mutable conversation.
decision: The live Flash-low proof used `gimbal run-prompt` to create and read `agy-proof.txt`, returned schema-valid JSON, projected the real tool and text steps into observation state, and ended the durable run with `recording_error: ""`.
correction: Independent validation found that native session bootstrap hid a paid turn, cancellation could miss a process after stdout closed, explicit non-Gemini effort was discarded, and the proof predated the last projector change -> bind the native conversation lazily on the first visible turn, select on cancellation through process exit, reject unsupported effort, and replace the proof at the final head.
correction: The first lazy-binding fix used the adapter UUID in agy NativeRef, losing the real conversation ID -> provider events use the learned agy conversation ID while Gimbal canonicalizes them to the adapter-owned session.
correction: Live structured output exposed agy's terminal `finish` tool as ACTIVE followed directly by a result envelope -> settle that specific tool from the result and reject other unsettled steps.
