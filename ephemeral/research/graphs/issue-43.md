# fan_out/fan_in shape: object branches are one node, and the branch-template fields have no honest home

URL: https://github.com/tylergannon/gimble/issues/43
State: closed
Milestone: None
Updated: 2026-09-13T16:46:57Z

Recorded from the #41 discussion. **Not** part of #41, which is renames only.

## What is actually restricted

A branch can be an arbitrary subgraph. This validates today:

```yaml
- {id: fan,   type: parallel, branches: [left, right]}
- {id: left,  type: tool, tool_command: "true", on_success: left2}
- {id: left2, type: tool, tool_command: "true", on_success: join}
- {id: right, type: tool, tool_command: "true", on_success: join}
- {id: join,  type: parallel.fan_in, prompt: judge, edges: [{to: success}]}
```

So the fan_in does not have to sit immediately after the fan_out, and parallel subgraphs are already supported. The restriction is narrower than it looks:

- **String branches** name nodes you declared, so each branch may be any subgraph.
- **Object branches** (the `codergen:` shorthand, as in `examples/loops/bake-off.yaml`) synthesize exactly one agent node each. A branch in that form cannot be more than one node.

## The template problem

With object branches, the fan_out node's own `prompt`, `fidelity`, `thread_id`, `max_retries` and model fields are copied onto every synthesized branch (`resolveParallelCodergen`, `graph/parse.go:120-143`). So `prompt` on a fan_out means "the prompt each branch gets", which is not what `prompt` means on any other node type. #41 renames the template `edges` to `branch_edges` so that `edges` has one meaning, and deliberately leaves the rest alone.

A shape fix would give these a named home, for example a `branch_defaults` block, so nothing template-shaped hides in a field that looks like the node's own.

## Open questions, not decided

- Should object branches be able to be a subgraph rather than one node, or should the shorthand be dropped in favor of declaring branch nodes?
- Should one node define both the fan-out and the fan-in, given the shape is restricted anyway?
- Do the template fields move into `branch_defaults`?
