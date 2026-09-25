# Recovered Go library POC and authoring sketches

Recovered and checked on 2026-09-09 after Tyler identified that a new proposal
had missed the earlier implementation. Start here before designing another
Go orchestration API. The shipped Gimbal graph engine remains on `main`;
the Go POC exists on a preserved, unmerged branch.

## Read in this order

1. [Actual workflow source](POC-WORKFLOWS.md): the 88-line orchestration file
   after input/prompt separation, copied unchanged from the branch.
2. [Concurrency shapes](CONCURRENCY-SHAPES.md): the earlier proposed
   critique-circle and bake-off algorithms using ordinary `errgroup` and
   synchronous typed operations. This document is restored unchanged;
   its Tractor names and present-tense POC status refer to that branch.
3. The removed Go-library sketches recorded the later proposal for
   roles, arbitrary Polytype inputs, an additive migration, and a curated
   catalog. Tyler rejected its level of indirection. Its API names are
   hypothetical and need revision against the earlier work.

## What was implemented

The `program` library already contains typed `Codergen[T]`, command execution,
command-plus-inference validation, and
`Loop(...) iter.Seq2[Iteration, error]` over the existing checklist format.
The iterator owns validation and `done` reconciliation; ordinary Go owns
implementation, review, repair, and nesting. Validation is a callback, so
callers can also supply ordinary Go validation functions.

`program/workflows` implements sprint execution, nested chapter execution,
and delivery. Input validation/dispatch, configuration/defaults, prompts,
and typed step adapters are separate from the workflow algorithms.
`tractor program schema <name>` and `tractor program run <name> --input ...`
already formed a separate argument-schema/invocation path beside the graph
CLI. These are commands in the historical POC, not current Gimbal commands.

The POC is not complete graph-engine parity. Scoped automatic context,
concurrent call ownership/isolation, supervisors, native-session continuity,
and a visualizer remained follow-ups. Its input schemas were handwritten;
workflow-owned roles and generated Polytype argument schemas are subsequent
direction. The concurrency sketches explain the concrete per-call
cancellation and invocation-identity problems to resolve before parallel use.

## Recover the complete source

The verified backup is `/Users/tyler/src/gimbal-history-backup-20260909.bundle`.
Its `refs/heads/codex/programmatic-workflow-research` points to
`4906a98c44930763597ebe015d5b996c120bc1eb`. This local backup is not a published
Gimbal branch; these Markdown copies keep the authoring examples available
without it. The full source requires access to the backup.

Clone into a separate inspection directory; do not import the old repository
history or its research corpus into current Gimbal:

```sh
git clone --single-branch --branch codex/programmatic-workflow-research \
  /Users/tyler/src/gimbal-history-backup-20260909.bundle \
  /private/tmp/gimbal-go-poc-inspection
```

Relevant commits:

- `983ebc6bf57ca0a7d40a6b55d3daec0269861cbe`: initial Go POC.
- `13961c504bde31e2edd8e06f95e497ba4e110593`: frozen implementation recorded by
  the historical proof report.
- `ae7af1f56548ac866f99f21217b3754b32e15786`: separate workflow algorithms from
  input and prompt plumbing.
- `b04cd90d5a189dae8cf4dcaec60c740a27afb706`: ordinary-Go concurrency sketches.

In the recovered checkout, inspect `program/`, `cmd/tractor/program.go`,
and `ephemeral/projects/tractor/programmatic-workflows/POC.md`. The last file
records the original choices, invocation examples, limitations, and historical
live-run results. The referenced temporary proof artifacts no longer exist;
source recovery was verified, but those runtime results were not reverified.

## How to use this as the next starting point

Tyler's established authoring requests were Go iterators, visible ordinary
control flow, separate input/prompt plumbing, automatic scoped context, and
ordinary goroutines/errgroup for concurrency. The next sketches should build
from the recovered code and those requests. Keep the newer role/input/catalog
direction distinct from its rejected API spelling; none of its invented
identifiers is an accepted interface merely because it appears in the index.

## New Go source examples

For actual Go programs to read and run in the current checkout, see
the stubbed workflow examples that were removed on 2026-09-10.
Bake-off and critique circle use ordinary `errgroup`; sprint execution uses
a Go iterator with visible implementation/review/repair control flow. Inputs,
prompts, and demo responses live in separate files.

These are new examples of the authoring shape, not a restoration of the native
runtime. Agent and command responses are canned; workspaces and integration
are symbolic; checklist state is in memory. Catalog discovery, generated
Polytype schemas, automatic context assembly, and full runtime parity remain
outside them. The historical POC and its provenance above remain the starting
point for native execution work.
