# Source leaf: earlier Go concurrency sketches

## Purpose

Recovered original design for critique circles and bake-offs, using ordinary
goroutines/errgroup with synchronous typed operations. These are proposed
algorithms; the recovered POC was sequential.

## Key concepts

- Tyler's pseudocode-first request and ordinary-Go starting point:
  `ephemeral/projects/gimbal/programmatic-workflows/CONCURRENCY-SHAPES.md:1-21`.
- Shared-proposal critique, adjudication, revision, and competing-draft variant:
  `ephemeral/projects/gimbal/programmatic-workflows/CONCURRENCY-SHAPES.md:23-74`.
- Isolated bake-off trials, failed candidates as data, joining, and explicit
  selection separate from integration:
  `ephemeral/projects/gimbal/programmatic-workflows/CONCURRENCY-SHAPES.md:76-115`.
- Scoped context, native call identity, workspace isolation, observation,
  and the POC's cancellation/serialization limitations:
  `ephemeral/projects/gimbal/programmatic-workflows/CONCURRENCY-SHAPES.md:117-136`.
- Historical comparison to Temporal/Genkit and the proposed next implementation:
  `ephemeral/projects/gimbal/programmatic-workflows/CONCURRENCY-SHAPES.md:138-172`.

## Retrieval recipes

- For “do we need fan-out/fan-in or Future primitives?”, read lines 10-21,
  then the concrete algorithms at lines 23-115.
- For safe concurrent calls, read lines 117-136 before implementing these shapes.
- For provenance and current status, use [POC recovery](poc-recovery.md).
