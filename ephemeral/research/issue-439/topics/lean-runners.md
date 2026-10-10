# Lean runners

[Back to TL;DR](../proposal.md)

**Build a small project runner separately from the CLI/server executable. Keep both the website and historical UI state out of the runner.** Loading the website as a Go plugin buys us nothing needed for this boundary.

## What gets linked

The project entry point links its workflow code, Gimbal execution runtime, required adapters and selected execution-backend support, recorder, and small HTTP control handler. It excludes `web`, skgo, frontend assets, the built-in workflow catalog, and the source analyzer. Typed `Generate[T]` schemas and validation stay with the project executable.

The local backend is compiled into the runner and server; it needs no plugin or separate artifact. For an installed backend extension, the project runner links the execution client from its own module graph and never loads the server's Go plugin; the server loads its installed typed recovery entry. The server does not need every provider's clients or project workflow code to recover a run. Instance deployment selects and validates the compatible server/recovery-plugin bundle and passes the selected configuration to the runner. Project build metadata declares the client's recovery-contract ID; the server checks exact support before launch, and recovery rechecks the ID in persisted records. This record boundary does not require the project's Go module graph to match the server's plugin build. Keep that installation in place through active runs and pending cleanup; stop/drain before upgrading. There is no per-run artifact store. This proposed backend integration is distinct from turning the website into a plugin and is not an already implemented loader.

A server-mode flag changes runtime behavior; it does not exclude linked packages or their initialization. Separate builds make the exclusion explicit. Go plugins add toolchain and shared-dependency matching constraints, described in the [official documentation](https://pkg.go.dev/plugin).

The root Gimbal library already excludes web/skgo/builtin dependencies in its current import graph. It does still depend on `internal/observation`, and even a direct run creates the observation store. The split therefore requires changing runtime observation ownership, not merely choosing another `main` package.

## What stays in memory

The current observation store retains completed turns and transcript projections. Move that reduction to the central server. The runner retains active workflow/adapter state and bounded recording/streaming buffers; its historical record lives on disk.

This does not promise a fixed total RSS: workflow values and agent subprocesses have their own memory needs. Measure runner memory separately from those subprocesses, both idle and after a long transcript. The intended result is that browser history and accumulated UI projections do not grow every satellite's heap.

## Current implementation anchors

- [Run setup and observation handover](../../../../run.go)
- [Observation store and transcript maps](../../../../internal/observation/store.go)
- [Web application assembly](../../../../web/server.go)

See [storage](storage-and-remote.md) for immutable context objects and [delivery](delivery-and-review.md) for the evidence required before claiming the memory boundary works.
