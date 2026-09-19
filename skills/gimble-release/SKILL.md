---
name: gimble-release
description: >
  Build and deliver changes to the Gimble application. Covers development
  priorities, proof, generated frontend assets, merge, local installation,
  and refreshing Gimble instructions in installed skills and plugins.
---

# Build and release Gimble

Deliver the requested behavior and establish that it works. Gimble currently
serves agents on this machine; local installation is the distribution target.
For writing a workflow, use [Author workflows](../gimble-workflows/SKILL.md).
For operating one, use [Use Gimble](../gimble-runs/SKILL.md).

## Priorities and working method

Work in the task worktree. Read its `AGENTS.md`, `docs/definition-of-done.md`,
and the relevant requirement. Use Godoc for the API; use research and historical
reviews to understand decisions. Materialize remote requirements locally before
giving their absolute paths to agents.

Keep implementation as simple as the requirement permits. Workflow tactics
stay visible in ordinary Go. Add no unrequested features, compatibility layers,
or abstractions. A hypothetical edge-case concern needs a reasonable actual
failing test before it becomes repair work.

Give agents outcomes and relevant facts. Scope coaches can steer planner,
worker, and validator against over-engineering; they are advisory. Review
findings are assessed against requested behavior rather than automatically
expanding the goal or becoming release blockers.

The repository's completion standard is requested functionality implemented
and seen working. Its 90–95% completion guidance permits deferring remaining
quirks; it does not permit declaring a required failing check successful.
Distinguish unmet requirements and invalid proof from optional improvements.
File genuinely deferred work rather than extending delivery indefinitely.

## Build the application

```sh
just build
```

The current recipe prepares Go and frontend dependencies, runs generation,
formats generated frontend output, builds Svelte, and builds `bin/gimble`.
The binary embeds the web application: `go install ./cmd/gimble` alone does
not prepare missing or stale frontend assets.

Generators own generated files. Regenerate and inspect their diff rather than
patching generated output by hand. Preserve Gimble-owned application code when
updating a scaffold. Follow the existing application composition so tests and
the built binary exercise the same server.

## Check and demonstrate the change

Run required repository checks appropriate to the change:

| Check | Purpose |
| --- | --- |
| `just vet` | Go vet, binary build, and Gimble lint. |
| `just test` | Go and frontend unit tests. |
| `just fmt-check` | Frontend formatting. |
| `just e2e` against a running server | Browser behavior through Playwright. |

A documentation-only edit needs instruction and link checks. For application
behavior, run the real path the claim concerns: a workflow, separate CLI
processes for external steering, or the browser for a UI change. Use cheap
models for live demonstrations and name the model.

An independent validator inspects the work and whether its evidence establishes
the definition of done. Worker summaries and aggregate green gates cannot
establish unrelated behavioral claims. After relevant checks pass, repeat them
only when a change, failure, or unresolved concern warrants it.

In this repository, repeatable checks belong beside the code they test. Do not
commit proof programs, screenshots, logs, or run output. Describe observations
in the conversation or PR. Other projects using Gimble retain their own policy
for keeping or sharing evidence.

## Review CLI documentation

For an added or changed built-in, read its rendered help as a caller. It must
explain purpose, inputs, meaningful defaults, outputs or changes, proof and
completion expectations, limits, and a useful example at suitable detail.

The entry function's doc synopsis supplies short help; package documentation
supplies long help; parameter field comments supply flag help. Check the built
command's workflow list and affected workflow help. A graph or output schema
does not document the invocation for its caller.

## Merge and refresh the local installation

Commit and push meaningful work; follow repository review and squash-merge
practice within the user's authorized scope. Retain the task worktree when the
user has asked to keep it. After merging a feature or fix:

1. Fast-forward the main checkout and build the merged source with `just build`.
2. Install it with `go install ./cmd/gimble`.
3. Update affected instructions in their maintained skill or plugin source and
   refresh those packages' installed copies, including supporting references.
4. Resolve the executable with `command -v gimble`. Check `gimble --help`,
   `gimble run --help`, and affected workflow help; exercise the relevant
   installed behavior when a behavioral change requires it.
5. Compare installed instruction files with their maintained source. A
   successful install command alone does not establish that the copies match.

The maintained skill source is `skills/` in this repository.
The shared local installation location is `~/.agents/skills/`. The Gimble set is
`gimble`, `gimble-workflows`, `gimble-release`, and `gimble-runs`. For an affected
plugin, inspect its actual installation and use its update mechanism; preserve
unrelated local configuration. Report when a new session is needed to load
updated instructions.

Local release needs no binary signing or scalable distribution system. The
existing docs-site CI and model-price patch automation have their own scope;
they do not imply a general CLI publishing pipeline.

## Report delivery

State the behavior observed, relevant checks, merge status, installed binary
path, and which installed instructions were refreshed. Separate local proof,
merged source, and installed behavior. Name unmet requirements and deferred
issues plainly.
