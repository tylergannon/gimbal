# Build and release route

Use this route when changing and delivering the Gimble application itself.
Its priorities and methodology precede the shell commands.

| Maintainer's decision | Read next |
| --- | --- |
| What makes requested work done? Which findings should block it? | [Requirements, validation, review and proof](../../leaves/05-methodology.md) |
| How do we keep coaches from turning taste into release gates? | [Supervision and scope](../../leaves/04-supervision.md) |
| Which contract and examples define the current API? | [Structure and current authority](../../leaves/01-structure.md) |
| What must build first? Which files are generated? Which checks apply? | [Build, hooks, embedding and CI](../../leaves/06-build-release.md) |
| What should be visible through the real installed CLI and browser? | [Operations and runtime behavior](../../leaves/07-operations.md) |
| Does a workflow's installed help accurately document its contract? | [Generated workflow documentation](../../leaves/11-workflow-packaging.md) |
| How do existing planning/review methods support delivery without adding ritual? | [Delivery loop methods](../../leaves/10-delivery-loops.md) |

Current user direction adds a concrete post-merge responsibility: rebuild
required assets, install Gimble locally with `go install ./cmd/gimble`, update
affected plugin/skill instructions and their installed copies, then verify the
installed experience. This is a requested operating practice, not a claim that
the repository already automates it. Local-machine distribution is sufficient.

Proof must match the change. The steering increment's separate-process live
test is a useful example for steering, not a mandatory fixture for every
release. Check the [authority route](../authority/index.md) before importing
generic artifact-upload, review-loop, versioning or publication requirements.
