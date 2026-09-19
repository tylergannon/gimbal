# Milestone 2: the application on the new components

Rough claims, to be sharpened once milestone 1 is done and seen. The
promise: the web application's pages are built from the milestone 1
components under `web/src/lib/run/`, driven by the run data the Go server
already serves, and the old viewer is gone.

Paths are relative to the repository root. The application is skgo: pages
are Svelte, loads and remote functions are Go under `web/src/routes/`, and
the generated `*.remote.ts`, `+page.server.ts`, and `skgo_remotes_gen.go`
files are not edited by hand (see `docs/web-app.md`).

## Claims

1. `/` is the runs list of `Runs.html`: every run in the project with its
   status, identity and time, the attention items for questions waiting,
   the filters, and a way to open a run. A Go load supplies the rows.
2. `/runs/[runID]` is the workspace of `Main.html`: the map on the left, the
   detail pane on the right, the topbar above. The page takes the snapshot
   the existing Go load hands over and the frames the store pushes after it
   (`web/src/lib/observation/index.ts` and `web/src/lib/sessionstate/`
   stay); the map and the pane update live as the run goes.
3. The map draws the run's registered workflow graph. Today no endpoint
   serves a graph to the page, and the root package keeps registered graphs
   in an unexported map (`graphs` in `run.go`): add the accessor and a Go
   load or remote function that returns the graph registered under the
   run's workflow name, as the TypeScript `Graph` type in
   `web/src/lib/workflow/types.ts`. A run whose graph is missing or no
   longer matches shows the lanes of `History.html`.
4. Steering a session uses the existing `steer` remote function; a loop
   message and wrap-up use `steerLoop`; an interview answer uses
   `answerInterview`. Each control reports whether the message landed, as
   the existing `Sent` and `Waiting` types say.
5. Stop turn calls the runtime's `KillTurn` and cancel run calls the
   runtime's kill of the run's root scope through new Go remote functions,
   behind the `CancelGuard` dialog. If the runtime lacks a way to cancel a
   whole run, file an issue and leave the button disabled with a tooltip
   saying so.
6. Live and disconnected states show in the topbar as the design draws
   them; a recorded run shows no live badge.
7. The old viewer under `web/src/lib/observation/*.svelte` and its stories
   are deleted; `index.ts` and its tests stay.
8. `just build`, `just vet`, `just test`, and `just e2e` pass; the e2e
   features under `e2e/features/` are updated to the new pages.
9. Nothing under `internal/observation/` changes except what claim 3 needs.
