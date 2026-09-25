# Issue 326: Initial standalone history viewer renders Error 500 before becoming usable later

Source: https://github.com/tylergannon/gimbal/issues/326
Observed: 2026-09-20 on Gimbal build `a41c8b7e6e2d4289dfcdf6c15c1fb99a75a22ee7`

The initial standalone history-viewer page displayed `Error 500 / Internal Error`
beneath the normal navigation. The same viewer was usable later in the session.

The evaluation harness started `gimbal --port 43840` in a newly created
Scrabbler worktree and checked readiness with
`curl --fail --silent http://127.0.0.1:43840/`. The readiness check succeeded.
The recorded browser then visited the viewer; its first retained screenshot
shows the error. The evaluator subsequently used the delegated run's own
listener on 43841 and returned to the 43840 history viewer after the target
completed.

A fresh worktree/startup view is a reproduction lead, not a confirmed root
cause. The screenshot has no stack trace; the precise failing request,
repeatability, and server-side cause remain unknown. The readiness check itself
did not return 500.

Expected: the initial viewer renders a usable page, including a valid empty
state when no history exists, without an unexplained server error. Diagnose the
observed startup failure and add a focused regression check once its cause is
known.

The prescribed Gimbal product/process model is a single process only. Any
evidence of ports 43840 and 43841 or multiple runtimes must be explained against
that requirement; it must not be assumed to be intended architecture.

Target run: `01M2ZTP8DFJQAYF7QSK36AYPHH.implement`.
Evaluator: `gpt-5.6-sol:medium`; implementation roles: `claude-sonnet-5:high`.
