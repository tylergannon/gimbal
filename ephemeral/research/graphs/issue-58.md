# Runtime-configurable model presets: name the role, not the model

URL: https://github.com/tylergannon/gimble/issues/58
State: closed
Milestone: None
Updated: 2026-09-13T16:47:06Z

## Summary

Let a node name a role rather than a model, and let the operator decide what each role resolves to at runtime.

```yaml
presets:
  manager: fable
  workhorse: claude-sonnet-5
  heavy: claude-opus-5
  validation: gpt
  evidence: flash
```

```yaml
- id: implement
  type: agent
  model: workhorse
```

## Why

**The same mapping is retyped in every file.** The five workflows in `examples/workflows/` use four roles between them — a manager for planning and for every loop's goal evaluator, a workhorse for ordinary turns, a validator that always runs on the other provider from the coder, and a cheap judge for evidence. That convention is currently spelled out node by node in five files. Changing the workhorse means editing every node that has one.

**Roles are stable; models are not.** A pipeline that says `claude-sonnet-5` is stale the next time the family moves. A pipeline that says `workhorse` is not.

**Operators differ.** Someone without Codex access, someone who wants the heavy model everywhere, someone running as cheaply as possible, and CI all want the same graph with different models. Today the only way to get that is to fork the file.

**The engine already thinks in roles.** `engine/model_selection.go` has `RoleItemJudge` and `RoleGoalEvaluator` with their own system defaults, and `tractor inspect-models` already prints every effective selection with its source. Presets generalize a mechanism that exists rather than adding a parallel one.

## Alias gap

`gpt`, `flash`, and `fable` are maintained aliases. There is no `opus` or `sonnet`, so the workhorse role cannot be named without a provider-native ID — `claude-sonnet-5` or `claude-opus-5` spelled out. Worth adding those aliases regardless of whether presets land, since a preset table wants short names on both sides.

## Acceptance criteria

1. A node's `model` may name a preset instead of a model selection, and the two forms are distinguishable at parse time.
2. Presets resolve from operator configuration, and a pipeline may override any of them in its defaults for the cases where the graph genuinely requires a specific model.
3. `tractor inspect-models` reports the preset name alongside what it resolved to and where that resolution came from, the way it already reports selection sources.
4. An unknown preset name is a lint error at `start_run`, not a fallback to a default.
5. The existing internal roles resolve through the same table, so the item judge and goal evaluator are configurable the same way as everything else.
6. Pipelines that name models directly keep working unchanged.
7. A run records the resolved model for every node in its manifest, so a run stays readable after the operator's preset table changes.

## Note

This pairs with #56. Skills make a node's instructions portable; presets make its model portable. Between them a pipeline file stops encoding one machine's setup.

