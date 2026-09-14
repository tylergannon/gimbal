# Steer does not report whether it landed

URL: https://github.com/tylergannon/gimble/issues/107
State: closed
Updated: 2026-09-14T01:00:03Z

`HarnessAdapter.Steer` returns only an error. `Session.Steer` knows a steer was dropped when no turn is running, but it cannot tell when a steer races the end of a turn inside the adapter. The Codex adapter swallows the `turn/steer` error in that case. Sprint 2's steer event needs a landed-or-dropped field, and Sprint 4 draws it.

