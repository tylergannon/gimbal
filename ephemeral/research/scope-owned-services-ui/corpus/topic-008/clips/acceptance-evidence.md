# Acceptance evidence clip

Use the copied source files for the implementation seam; this clip identifies
the observations a proof run must make.

- `issue-284.md:14-27` requires root, nested, and iteration ownership while
  excluding runtime process and dependency semantics.
- `workflow-graph.go.txt:21-44,93-135` shows the current graph's source-bearing
  sealed operation union, `Command` conflation, nested `Scope`, and static
  `Iterate` body.
- `generate-graph.go.txt:130-138` preserves module-relative forward-slash file and
  line; `generate-source.go.txt:79-99,127-139` emits those fields but currently
  serializes every service as `workflow.Command`.
- `workflow-types.ts.txt:21-52,100-121,270-310` is generated from that Go shape;
  `run-layout.ts.txt:209-245,309-330,479-495` makes commands ordered visual nodes
  and scopes containers; `run-node.svelte.txt:38-86` supplies labels, icons,
  keyboard focus, and selection behavior.
- `generate-graph-test.go.txt:14-80` already proves nested scope, iteration, and
  command extraction. Extend the observation with root/child/item services,
  then inspect generated JSON and the browser map rather than treating a green
  test gate as proof of the visual distinction.
