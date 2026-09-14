# Live run registry: Runtime.Steer, KillScope, KillTurn by id (Go half; page half after #173)

URL: https://github.com/tylergannon/gimble/issues/176
State: closed
Updated: 2026-09-14T00:58:49Z

The Go half of "steer or kill any agent from the page". The page half waits until the usage tree (#173) lands, because it edits the run route that #173 rewrites.

## Facts at `ad6f2f6`

- `web.Runtime` (`web/runtime.go:22`) holds a ctx, a dir, and an address. It does not know which runs are live; `Runtime.Run` calls `gimble.Run` and blocks.
- Steer needs a `*Session` (`Session.Steer`); the kill functions from the cancel-by-id issue need a `*run`. The page has neither. It has a run id, scope keys, session ids, and turn ids from the observation store.

## Change

- `Runtime` keeps `live map[runID]*liveRun` filled by `Runtime.Run` for the duration of the run. The root package exposes what the runtime needs through unexported hooks in `run.go` (a `Handle`-style value returned to `web`, or an internal package), not through new exported names on `Session`.
- Three methods on `Runtime`, by id, each returning an error for an unknown or finished target:

  ```go
  func (r *Runtime) Steer(ctx context.Context, runID, sessionID, message string) error
  func (r *Runtime) KillScope(runID, scopeKey, by, reason string) error
  func (r *Runtime) KillTurn(runID, turnID, by, reason string) error
  ```

  Steer records `Source: "person"` (the existing `steerSource` value) so the log and the page can tell an operator's steer from a supervisor's. Kill records the `Killed` lifecycle event with `By`.
- A session is found by its id (`lap.3/coder.1`) through the scope table; the session list already lives on the scope (`scope.sessions`).

## Done when

- A Go test starts a run under `web.NewRuntime` with the fake adapter, steers a session by id from another goroutine, kills a turn by id, kills a scope by id, and reads all three from the run log.
- A live check on the cheap tier where a steer from a second goroutine lands (depends on #107 to say so) and a kill ends the turn. Log under `ephemeral/attest/registry/`.
- Nothing under `web/src/` changes.

