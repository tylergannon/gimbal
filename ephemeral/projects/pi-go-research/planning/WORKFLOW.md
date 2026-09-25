# Dedicated Gimbal delivery workflow

Implement ordinary Go in `internal/workflows/piport/`, using today's working Codex/Claude/Gemini harnesses. The port must not depend on itself to build. Its purpose is to deliver the assignments in [MODULES.md](MODULES.md), with visible independent branches, real gates and one integration owner. This document is a design, not runnable or compiled code.

## Shape

Inputs are the accepted local plan, source roots/pins, integration worktree/commit, remaining assignment IDs, role bindings, concurrency limit and maximum revision attempts. Assignment descriptions are local data, not a new generic workflow framework. Sessions receive absolute paths to materialized handoffs and source; no agent must fetch an issue to learn its assignment.

Write the stages directly in the workflow:

1. Verify source pins, clean integration baseline, required tools and assignment set. Materialize handoffs outside the tracked tree. Contracts are already accepted or run the explicit contract stage first.
2. Create each ready worker worktree from the exact same integrated commit with a unique `codex/` branch. Never reset/recreate a pre-existing branch blindly.
3. Declare a `Group` for that stage and explicit, constant-named children such as `openai`, `anthropic`, `google`, `configuration`, `read-tools`, `edit-tools`, `shell`, `compaction`. Each callback contains its implementation/test/review/revision steps visibly. Join the whole group.
4. Inspect results and changed paths. Integrate accepted commits serially in declared dependency order. Check command error AND exit status after each Git operation/check. Stop on rejected required work, conflicts or integration failure; retained branches are the repair inputs.
5. Start dependent stages from the new tested integration commit. Assemble session and adapter, then run independent integrated qualification and the real Router task. Completion requires every assignment required for the selected milestone, not merely an empty pending list.

Use `Iterate` for bounded attempts and serial integration items. Keep names constant at each callsite. A cancellable semaphore inside callbacks may cap concurrent active work; it controls capacity without replacing `Group` ownership or hiding orchestration. Review sessions also count against capacity. Session acquisition/release must not deadlock by holding a worker capacity slot while waiting for an additional reviewer slot.

**Why explicit children:** today's graph extractor requires a `Group.Go` call in the body that declared its group. `internal/generate/expr.go` diagnoses `Go on a group declared outside this body is not read`; `stmt.go` gives ordinary range loops a separate repeat body. Consequently, `Group` outside a `for assignment` loop with `Go` inside it is not our executable design. Do not fix the generator as part of this port. This finite workflow can name its actual branches directly. Verify the generated graph before running it.

## One branch

Each branch uses a worker session and an independent reviewer session; a scope coach is optional, not another mandatory approval layer. For at most three attempts:

- Ask the worker to read and implement its local assignment. It knows its allowed files and frozen contracts, and that other workers own other packages. It must not revert or edit their work.
- Run package build/tests/race checks appropriate to the change with `RunCommand`. Check process-start errors and nonzero exit status explicitly. `Check` adds useful diagnostic context but a nil error does not establish exit zero.
- Compare the full changed-path set against ownership, including staged, unstaged, untracked and committed changes since the baseline. A passing test does not excuse an out-of-scope file.
- Have the fresh reviewer inspect the actual diff, original TypeScript and relevant tests; run focused checks independently. Require concrete unmet behavior, not style preference or guessed parity.
- If tests, ownership and review pass, record the candidate commit for serial integration. Otherwise write actual failure/findings to the local handoff and revise. Never replace the acceptance condition with a model's assertion of success.

An ordinary exhausted implementation rejection records a rejected result and returns nil from its child so unrelated workers can finish; the parent refuses overall success after `Wait`. Infrastructure errors/cancellation propagate normally, and `Group` cancels siblings on the first ordinary error. Always join before cleanup. An operator-killed child is not a successful assignment even when siblings continue. Protect shared result collection, or use disjoint fixed result slots.

Keep candidates' unrelated successful work when a sibling fails. No automatic rollback of others' branches, conflict invention, or edits to make a failing test disappear. Cleanup cancels/joins agent sessions and processes before any worktree removal, using bounded cleanup independent of an already-cancelled run context. Failed worktrees remain available for diagnosis.

## Integration and restart

Only the integration owner modifies module dependencies, attribution, binding/registration and generated files. Ownership avoids most content conflicts; it does not make semantic or Git conflicts impossible. Serially cherry-pick a candidate, run the relevant integrated checks, and advance the accepted baseline only when they pass. On failure, retain the last accepted commit and candidate state and give the owning package the concrete defect.

A restart receives explicit remaining IDs and inspects retained integration commits and worktrees. Verify previously accepted commits are ancestors of the integration baseline, their required checks still apply, and all required assignments are accounted for. A tag or old reviewer message is not sufficient. Resume failed work deliberately; do not infer transparent Gimbal run resumption from native session persistence.

## Demonstrating the workflow

Before launching the port, build/register the workflow, regenerate/lint its graph and inspect that all package branches are visible. A running older binary cannot discover newly written workflow code. Run a tiny two-branch disposable implementation using `gpt-5.6-luna` or Claude Haiku; observe both branches, a deliberately rejected candidate, joined completion and serial integration. Verify rejection cannot become a successful run. Temporary handoffs/logs stay outside tracked source; repeatable checks belong in package tests. Report the model and observations.

Frontier judgment owns shared contracts, lifecycle assembly and final acceptance. Routine independent modules may use cheaper coding models, with escalation for demonstrated difficulty. The chosen capacity controls actual concurrency; twenty assignments are not twenty permanently running agents.
