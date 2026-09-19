# Phase 2, the runs page

The promise: `/` is the runs list of `docs/design/specimens/Runs.html`,
built from the `RunsList` component already under `web/src/lib/run/`, fed
by a Go load, and the old landing page at `/` is gone.

The application is skgo: pages are Svelte, loads are Go under
`web/src/routes/`, and the generated `+page.server.ts`, `*.remote.ts`, and
`skgo_*_gen.go` files come from `go generate`, never by hand (see
`docs/web-app.md`). The workspace at `/runs/[runID]` exists and is being
changed by another lane at the same time: do not touch anything under
`web/src/routes/runs/`, `web/src/lib/run/`, or `run.go`.

## Claims

1. `/` renders `RunsList` with every run in the project: status, identity,
   workflow name, started and elapsed time, and the attention items for
   interviews waiting, from a Go load that reads the project's run tables
   (`internal/observation` has the readers; nothing under it changes).
2. Opening a run goes to `/runs/[runID]` as the existing links do.
3. The filters of the specimen work on the loaded rows in the page.
4. `just build`, `just vet`, and `just test` pass.

Done is the definition in the workflow. One task.
