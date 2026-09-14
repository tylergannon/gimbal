# Pipeline schema: nomenclature is inconsistent and in one case collides with itself

URL: https://github.com/tylergannon/gimble/issues/41
State: closed
Milestone: None
Updated: 2026-09-03T23:57:07Z

The authored surface uses several different words for one idea, one word for two different ideas, and in one place hides a template inside a field that looks like routing. Nothing about the names teaches the model behind them.

## Ruling (Tyler, 2026-09-03)

**Node types are renamed.** `parallel.fan_in` in particular is a literal type string today.

| Now | New |
|---|---|
| `codergen` | `agent` |
| `tool` + `tool_command` | `command` + `command` |
| `parallel` | `fan_out` |
| `parallel.fan_in` | `fan_in` |

**All routing moves under `edges`.** Every route out of a node lives under one key, so where a node can go is always found in the same place.

| Node | Now | New |
|---|---|---|
| `agent` | `edges: [{to, condition}]` | unchanged |
| `fan_in` | `edges: [{to, condition}]` | unchanged |
| `command` | `on_success`, `on_error` | `edges.success`, `edges.error` |
| `loop` | `body`, `on_done` | `edges.loop`, `edges.exit` |
| `fan_out` | `branches`, `edges` (branch template) | `branches` unchanged, `edges` renamed `branch_edges` |

`edges` is a list on `agent` and `fan_in` and a mapping elsewhere. That is intended, and each node type already declares its own properties, so no union is needed. The rule: **`edges` holds the routes out of this node, keyed by the condition when the engine knows the condition, listed with prose conditions when an agent must judge it.** `success`, `error`, `loop`, `exit` and `branches` are the engine-known conditions.

`supervises` stays out of `edges`. It is a scope, not a route.

## Two defects this fixes

**`body` collides with itself.** A loop node's `body` is its edge to the first node of a lap. A checklist file's markdown body is the definition of done, which is what the #33 evaluator reads. Same word, two meanings, one feature. `edges.loop` removes the collision.

**`edges` on a fan_out is not routing.** `graph.RoutingTargets` for a parallel node returns only `BranchIDs()`. The node's `edges` field is never a route out of it: `resolveParallelCodergen` (`graph/parse.go:120-143`) copies `edges`, `prompt`, `fidelity`, `thread_id`, `max_retries` and the three model fields onto each synthesized branch node.

`branches` stays a top-level field and does **not** move under `edges`: every branch runs, so they are not alternatives the way every other `edges` entry is.

To keep `edges` from meaning two different things, the fan_out node's template edges are renamed `branch_edges`. That is a rename and nothing else; the field keeps doing exactly what it does today.

The other template fields on a fan_out (`prompt`, `fidelity`, `thread_id`, `max_retries`, the model fields) keep their current names and behavior. Giving them an honest home is a shape change, not a rename, and is out of scope here.

## Also worth renaming

`on_done` reads as if it were about the #33 evaluator's "done" verdict rather than about leaving the graph. `edges.exit` says what it does.

## Changes

Type strings and field names above, across `graph/`, `lint/`, `engine/`, the generated `graph/jsonschema/Graph.json` and its checksum, `docs/spec.md`, `src/content/docs/`, `llms.txt`, `skills/tractor/`, and every file under `examples/`.

## Open

Whether the old type strings and field names keep parsing as deprecated aliases for one release, or break outright. Not yet ruled.



Scope note: this issue is renames only. No functional or shape change to fan_out/fan_in. See the follow-up issue for the shape questions.

