Remaining work split from #135, which shipped delayed/sequential tool attribution (#202), live Claude rendering (#202), and socket reconnect validation (#207).

Codex CLI 0.153.4 exposes experimentalRawEvents only on thread/start. Native thread/fork and thread/resume after daemon restart/thread unload cannot enable exact raw response events. The adapter's extra field is ignored. Socket-only reconnect of a still-loaded thread does preserve the setting and is validated.

This also leaves incomplete model-call accounting on the unsupported paths: a tool call followed by a final-answer call can produce one turn-finalized row retaining only the first call's usage.

Completion requires upstream native support (or a verified supported native mechanism), wiring it into Gimble, and a cheap live production-handler run demonstrating native fork and resume after restart/unload. Inspect exact native response IDs, correct tool/final row ownership, and complete per-call and turn usage. Use an isolated daemon for lifecycle experiments. Reuse existing validation; do not infer boundaries from usage notifications, fabricate response IDs, or reconstruct history.

Prior findings and evidence: https://github.com/tylergannon/gimble/issues/135#issuecomment-5670342169 and https://github.com/tylergannon/gimble/pull/207#issuecomment-5671101861 . This is deferred upstream-dependent work, not delivered functionality.
