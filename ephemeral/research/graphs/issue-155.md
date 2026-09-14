# Session info in the ported reducers never receives cost and tokens

URL: https://github.com/tylergannon/gimble/issues/155
State: closed
Updated: 2026-09-13T17:35:31Z

Both ported reducers (`internal/sessionstate/reduce.go`, `web/src/lib/sessionstate/index.ts`) fold `session.usage.updated` into a session's info record only when that record exists. OpenCode creates it by API sync; Gimble emits no event that creates it, so `info` stays empty and the reducers' `cost`/`tokens` slots are never filled.

For #149 the page reads the observation store's per-session usage map instead (`RunInfo.Usage`), which is correct and live. If a consumer of the reducer snapshot alone needs session totals, decide whether Gimble should emit a session-created event that seeds info, or whether the observation map is the contract.
