# Authority, conflicts, and unimplemented ideas

Use this route when two sources appear to disagree or a proposed topic risks
teaching an unavailable feature. Open the relevant leaf before the original
source; its authority label explains what the citation can establish.

| Question or conflict | Resolution and evidence route |
| --- | --- |
| Does a design sketch define the API? | Current Godoc/code and compiling examples define behavior. `API.md` preserves rationale and contains superseded names. [Structure](../../leaves/01-structure.md) |
| Is a planner stopping the same as success? | No. Dispatch, task assessment, deterministic checks and overall fulfillment have different authority. [Planning](../../leaves/03-planning.md) |
| Can a reviewer or coach add requirements? | Findings are evidence to assess against the goal. Taste steers; it does not create completion gates. [Methodology](../../leaves/05-methodology.md) |
| Is `df-promise` the runtime `PromiseLoop`? | No. The skill/helper describes an evidence-backed repository promise; the runtime primitive dispatches tasks. [Promises](../../leaves/08-promises.md), [planning](../../leaves/03-planning.md) |
| Must every task have a chapter, sprint, badge or ledger? | No. Chapter context is optional; planning artifacts apply to their selected method. [Chapters and sprints](../../leaves/09-chapters-sprints.md) |
| Does presence of a `df-*` skill imply a Gimble built-in? | No. Check actual CLI registration/help. [Operations](../../leaves/07-operations.md), [packaging](../../leaves/11-workflow-packaging.md) |
| Does historic Sprint 001 describe the current run store? | Its checkpoint design is historical. Use current observation/store code for record authority, replay, and file layout. [Operations](../../leaves/07-operations.md) |
| Does generic proof guidance require committing artifacts here? | Gimble's repository instructions prohibit committing run/proof output. Other consumer projects retain their own artifact policies; do not export Gimble's repository rule as a universal restriction. [Methodology](../../leaves/05-methodology.md), [promises](../../leaves/08-promises.md) |
| Does `--no-web` remove frontend build requirements? | It disables the browser listener during use. Building an embedded application still has frontend/generated prerequisites. [Operations](../../leaves/07-operations.md), [build](../../leaves/06-build-release.md) |
| Does a local build update what other agents execute? | `bin/gimble` and the installed executable are distinct. The user requires post-merge install plus affected skill/plugin refresh. [Build and release](../../leaves/06-build-release.md) |
| Is help generated from workflow comments today? | Consult the generator findings; distinguish what is extracted now from the user's richer documentation requirement. [Workflow packaging](../../leaves/11-workflow-packaging.md) |
| Should every old workflow pattern be copied? | Preserve the reasoning and useful shapes; old implementations are inspiration, not the restarted API. [Historical teachings](../../leaves/12-historical-teachings.md), [delivery methods](../../leaves/10-delivery-loops.md) |

## Editorial rule

The synthesis should teach decisions that generalize: who owns context, what
counts as evidence, how a planner receives feedback, and when work should stop.
Do not elevate one historical acceptance fixture, model choice, counter, cost
threshold, stage count, or workaround into a universal workflow requirement.
Name unresolved capability gaps. Do not repair them by documenting imagined
commands or by adding production changes during this research task.

The semantic-index helper's benchmark is also evidence with limits: its
current scoring can count existing expected files without query-directed
retrieval. No retrieval benchmark is claimed; the index is a working source-finding aid.
