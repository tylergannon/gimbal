# Workflow discovery

[Back to TL;DR](../proposal.md)

**A project builds its workflows and descriptions together. The shared CLI and server discover that bundle without rebuilding Gimbal.** Local compilation and explicit refresh are the first delivery choice.

## Build and publish

A project's `.gimbal/workflows.json` lists workflow entry packages. Proposed `gimbal workflows refresh` runs source analysis, generates the runner main package in `generated/gimbal-runner/` in the project, builds it with `go build`, publishes the executable and sidecar command/design metadata under `.gimbal/builds/<build>/`, then atomically updates a current-build pointer. CLI and server read the same pointer. Edits take effect on refresh; a failed refresh leaves the last successful bundle selected.

A distributed backend may publish multiple OS/architecture executable variants under the same workflow build identity, or compile on the execution site. Preserve per-task execution requirements in the compiled metadata; see [execution placement](execution-placement.md). A local `go build` does not demonstrate mixed-platform execution.

The CLI constructs commands from metadata, preserving useful flags, help, validation, and role choices. The runner validates inputs again. The server uses its common frontend to render project-specific forms and designs. Prebuilt bundles can use this boundary without requiring a local compiler; their distribution is a later choice.

## Bind a run to its design

Identify workflow definitions by project, workflow name, and build identity. Save the selected design with the server's build metadata and reference that exact version from the run record. Two projects can use the same workflow name; editing a workflow does not change the diagram attached to an older run.

Keep `RegisterGraph`/`RegisteredGraph` local to the project runner. Replace the server's serving-binary registration check and name-only graph lookup with the saved design metadata.

## Runtime contract changes

Revise `Run`/`OpenRun` to accept assigned identity and directory for hosted execution. Direct library calls continue allocating their own identity and running in-process. The current unsupported-sharing caveat becomes explicit ownership of mutable run files, with immutable context objects shared as described in [storage](storage-and-remote.md).

All runs stop writing the unused `project.jsonl`; the server maintains its project index from run events. Update the recording and `LifecycleRecord.Seq` documentation for the single authoritative event history; retain producer sequence identity only where required for deduplication, not to preserve replaced log formats. Update the package and `Run` Godoc statements that currently require hosted workflows to be compiled into the Gimbal serving binary. These are intended changes to the present contract, not claims that this architecture already exists.

Current anchors: [entry analysis](../../../../internal/generate/entry.go), [workflow page generation](../../../../internal/generate/workflow_page.go), [hosted execution](../../../../internal/host/host.go).
