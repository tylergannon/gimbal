# Worklog — pl-web (host wiring, remotes, UI)

- Stub `internal/promises` committed alone ("STUB promises API (drop on merge)").
  `Run` returns nothing in the stub; host ignores any return value anyway.
- First commit attempt failed the hook: `web/` tests need `web/build/skgo.manifest.json`;
  a fresh worktree must run `just build` before committing.
- Host: model resolution placed right after `promises.Open`, before the project is
  registered, so a bad `GIMBAL_PROMISES_MODEL` fails admission with nothing to unwind
  (plan put it just before `go p.cleanupExitedTasks()`, after registration). Host wraps
  the error with `GIMBAL_PROMISES_MODEL:` so the message does not depend on the package.
- `go build ./...` fails on plugins/* (no main) — pre-existing; use `just vet`.
