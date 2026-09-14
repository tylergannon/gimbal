# Unify workflow model objects and expose loop judge and evaluator roles

URL: https://github.com/tylergannon/gimble/issues/53
State: closed
Milestone: None
Updated: 2026-09-04T22:17:36Z

## Outcome

Give every model-driven part of a workflow one model-selection contract, and make the loop's item judge and goal evaluator visible as named roles. Authors should be able to predict which model executes each turn without knowing engine internals or reconstructing inheritance rules.

This issue was rewritten on 2026-09-04 at Tyler's request after reviewing the original agent-authored proposal against Attractor's goals. The decisions below supersede the original alternative shapes and loop-only scope.

## Decision record

**Tyler accepted:** Keep the checklist loop and its arbitrary body subgraph; expose `item_judge` and `goal_evaluator`; use one `model` shape everywhere; replace model selections atomically; preserve the deliberate independent judge default and normal evaluator fallback; validate before harness activity; migrate strictly and prove the actual loop behavior.

**Tyler explicitly added:** An optional `version` key. Model names identify their providers, so workflow authors should not select a provider. Third-party model routers are outside the current scope.

**Agent execution clarifications:** The precise string/version/default rules and validation matrix below make that design executable. They are implementation guidance under the accepted design, not additional decisions attributed to Tyler. Internal package names and data structures remain implementer choices.

## Why this change

The loop currently hides item-judge selection in generic `llm_*` fields while giving its evaluator differently prefixed fields. The editor describes their inheritance incorrectly. More fundamentally, `graph/parse.go` independently inherits model, provider, and effort for ordinary agents and supervisors too: selecting another model can retain the previous provider or effort. Fixing just the loop would leave two selection contracts in one workflow.

The loop roles serve different purposes:

| Role | Responsibility |
| --- | --- |
| `item_judge` | Inspect the evidence for one checklist item and return pass/fail. |
| `goal_evaluator` | Inspect whether the overall definition of done holds; revise open work when needed and return done/not_done. This is also the replanning role. |

Keep the engine responsible for validation ordering, item marking, frames, and traversal. Do not make authors reproduce this protocol by wiring ordinary nodes. Preserve existing prompts, tools, fresh-context policy, evaluator cadence, verdicts, and loop semantics.

## Authored contract

```yaml
defaults:
  model:
    name: fable
    effort: high

start: items
nodes:
  - id: items
    type: loop
    checklist: work.md
    item_judge:
      model:
        name: flash
        effort: medium
    goal_evaluator:
      model:
        name: fable
        version: "5.1"
        effort: high
    edges:
      loop: implement
      exit: success

  - id: implement
    type: agent
    prompt: Implement the current checklist item.
    edges:
      - to: items
```

One closed model object, with no string shorthand:

| Key | Contract |
| --- | --- |
| `name` | Required, nonblank string: a maintained model alias/family or recognizable provider-native model ID. |
| `version` | Optional, nonblank string. Selects an exact supported release for that name; never a number, version range, or silent fallback request. |
| `effort` | Optional explicit reasoning effort. Retain the currently supported `low`, `medium`, `high` vocabulary for this migration. |

`provider` is not an authored key. Provider and harness are derived resolution results. Reject names whose provider cannot be determined; do not introduce a default provider fallback, provider override, profile registry, or router abstraction.

Use the identical object at every model-selection location:

- `defaults.model`
- `agent.model`, `supervisor.model`, and `fan_in.model`
- `fan_out.model`, retaining its existing meaning as a branch-agent template, and `branches[].agent.model`
- `loop.item_judge.model` and `loop.goal_evaluator.model`

The notation above names node types, not literal YAML wrapper keys. A command node executes no model and must still reject model configuration. A loop has no ambiguous top-level `model`; its roles own their selections. The optional role objects currently admit only `model`; omission or an empty role object uses that role's default.

### Atomic defaults

Select the first whole model object at the applicable precedence level, then resolve it. Never fill its absent keys from a lower-precedence model object.

| Consumer | Selection precedence |
| --- | --- |
| Ordinary agent, supervisor, fan-in | Node selection, then pipeline selection, then system selection. |
| Synthesized branch agent | Branch override, then fan-out template, then pipeline selection, then system selection. |
| Item judge | Explicit role selection, then the independent `{name: flash, effort: medium}` default. Pipeline model defaults do not apply. |
| Goal evaluator | Explicit role selection, then pipeline selection, then system selection. |

After choosing the object, resolve omitted version from that name's maintained default release. Resolve omitted effort from the selected model's documented policy: preserve Flash's medium default, native effort fixed by an explicitly pinned ID where applicable, and the existing high default otherwise. An explicit effort overrides a model policy default, but cannot contradict an explicitly pinned native ID's fixed effort.

For example, replacing `{name: fable, version: "5", effort: low}` with `{name: flash}` must not carry version `5` or effort `low` into Flash. A version-only or effort-only object is invalid because `name` is required. Changing only effort therefore repeats the model name. Non-model defaults continue to behave as before.

### Versions and effort

- Omitted version chooses Tractor's maintained default release for that model name; an explicit version pins the requested release with no substitution.
- Preserve provider-native IDs and existing version-bearing aliases as escape hatches. An already versioned name must not also take a separate `version`, even if it looks redundant rather than contradictory.
- Version resolution must use known family mappings or supported native naming rules, not blindly concatenate strings. Include real pinning support, not an accepted-but-ignored field; exercise at least two existing supported releases in resolver tests.
- Native IDs with a recognized provider remain usable without enumerating every provider model in Tractor. Static resolution does not promise remote model availability or account access.
- Resolve one effective effort and translate it to a supported harness invocation. If the harness encodes effort in its model ID, use the supported translation. Never assume removing a suffix yields a valid model ID.
- Reject contradictory or unsupported explicit selections before execution. No adapter may silently drop an authored effort while reporting it as effective.

## Shared resolution and observability

Use one resolution path across validation and execution, including CLI-started and MCP-started workflows, ordinary turns, supervisors, synthesized branches, and both loop roles. Resolve every declared selection before any harness session or harness log is created, not only nodes sharing a thread or nodes encountered during traversal.

Preserve enough authored provenance to diagnose the offending location and explain defaults; do not eagerly flatten defaults and then attempt to reconstruct their source. Existing thread/harness compatibility checks must consume the same resolved values as execution.

CLI validation/inspection and the editor must expose each effective selection: node/role, authored name and version where present, resolved native model, effective effort, derived provider, harness, and the source of the selected default. Reuse the current inspection surfaces where practical; no separate service or execution-plan framework is required. Log records must identify the role and agree with the actual native request.

Keep static invalidity separate from execution-environment failures: validation can reject an unknown provider route or unsupported local mapping without checking credentials, launching binaries, or querying a provider's live catalog.

## Strict migration

Replace `llm_model`, `llm_provider`, `reasoning_effort`, `evaluator_llm_model`, `evaluator_llm_provider`, and `evaluator_reasoning_effort` wherever they are superseded in authored workflows, including defaults, templates, and branch overrides. Reject old keys with diagnostics pointing to the new location; no compatibility period or silently supported aliases for the old fields. Continue emitting derived provider/model information in runtime records; those are not authored selectors.

Update the Go graph types and generated JSON/YAML schemas/checksums, parser, resolver, engine consumers, lint, editor authoring and summaries, shipped examples, first-party skills/reference material, and normative documentation. Historical evidence can retain the syntax it actually tested if clearly historical. Follow repository schema-generation instructions; do not hand-maintain a competing schema.

Update the README's explanation of completion: command checks supply evidence, item judgments assess it, and goal evaluation determines whether the promised work is satisfied. An exit code alone does not describe checklist-loop completion.

## Execution order

1. Implement the common selection type and resolver with the version/default/effort matrix. Establish the single resolution path before changing consumers independently.
2. Migrate all graph consumers, preflight, and strict parsing; preserve existing loop and thread behavior.
3. Regenerate schemas and migrate editor, examples, skills, and documentation together. Validate every shipped runnable example.
4. Demonstrate the observable behavior below through the running CLI and editor. Follow repository verification requirements and bind each proof claim to the tested commit and inspectable artifact links in the PR.

Implementation entry points: `graph/graph.go` and `graph/parse.go` (authored contract and inheritance); `internal/modelalias/modelalias.go` (shared resolution); `engine/codergen.go`, `engine/supervisor.go`, and `engine/loop.go` (turn consumers); `lint/` and `cmd/tractor/root.go` (preflight); `harness/agy/adapter.go` (native effort translation); `web/editor/src/lib/model.ts` and `scene.ts` (authoring metadata and summaries); `docs/spec.md` (normative contract). Recheck current code rather than treating old line numbers as authority.

## Acceptance criteria

- [ ] Every model-capable authoring location above admits exactly the same model shape; command nodes and ambiguous loop-level model keys reject it.
- [ ] JSON and YAML accept omitted selections, name-only selections, explicit string versions, explicit efforts, known aliases, and recognizable native IDs; reject empty/missing names, numeric versions, partial replacement objects, unknown keys, authored provider, and every superseded field.
- [ ] Resolver tests demonstrate default release versus explicit release, at least two supported release pins, unsupported version failure without fallback, and rejection of version plus an already versioned name.
- [ ] Defaults tests cover ordinary nodes, supervisors, both branch precedence levels, and both loop roles; no provider, version, or effort leaks from a replaced selection. The judge stays independent of pipeline model defaults.
- [ ] Supported effort translation and explicit conflicts are tested at the native request/argument boundary, including the agy effort-bearing ID case. Reported effective effort matches the actual invocation.
- [ ] Validation and run startup through CLI and MCP use the same resolution/preflight. Invalid configured selections fail before any harness activity, including hidden roles, supervisors, and synthesized branches. Existing shared-thread checks remain correct.
- [ ] Generated schemas/checksums, editor controls and summaries, docs, and all shipped runnable examples agree with the new contract. Demonstrate editor load/edit/save preserving optional version and whole-object defaulting without reintroducing old fields; include the saved workflow and visual evidence of effective selections.
- [ ] Effective selections and their provenance are inspectable before execution and identifiable by role in real run evidence.
- [ ] Existing loop behavior is preserved, including validation failure, done-state reconciliation, evaluator replanning, nesting, and visit limits. This change does not alter those contracts.
- [ ] A real CLI workflow demonstrates: a worker produces inadequate evidence; the item judge rejects it; a repair is accepted; the goal evaluator sees that passing item checks still leave the overall goal unmet, leaves new/open work and returns `not_done`; subsequent work satisfies the goal and the evaluator returns `done`.
- [ ] The live proof uses distinguishable worker/judge/evaluator selections, exercises an explicit version pin, and records native provider/model/effective-effort evidence for each role. Correlate that evidence with the displayed resolved configuration. A simulated backend or successful routing alone does not satisfy this criterion.

## Boundaries and related issues

This intentionally broadens the original loop-only scope. Coordinate with #11 so aliases, versions, provider inference, and entry-point consistency have one implementation rather than competing fixes. Preserve the behavioral separation introduced by #33 and #37; follow #41's strict authored-schema migration precedent.

Do not redesign fan-out topology/template placement (#43), add programmable role subgraphs, change checklist validation or evaluator behavior, add model benchmarks, or claim general judgment quality from the live fixture. A version pin cannot guarantee a provider continues serving an old release. Internal harness provider fields remain necessary routing results; removing them is not a goal.

