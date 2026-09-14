# #108: Codex adapter keeps an idle app-server for a session that never runs a turn

https://github.com/tylergannon/gimble/issues/108

The Codex adapter keeps the `codex app-server` process that started or forked a thread until that thread's first turn. A session created or forked but never used keeps its process until the program exits. In the sprint workflow every fork is used, so this has not bitten yet.
