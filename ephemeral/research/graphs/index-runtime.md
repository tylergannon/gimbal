# Route: runtime graph, runs, and commands

Use this route for observed run topology, scope/turn identity, supervision, fork/steer/kill, SSE/UI projection, or command execution. The runtime leaves describe current behavior; the graph shape and command boundary are recommendations.

- Runtime scope key and owner/execution scope: `runtime-index-leaf.md:7-15`.
- Group/Loop and supervision relations: `runtime-index-leaf.md:16-23`.
- Fork, steer, kill, reducer loss, and SSE limits: `runtime-index-leaf.md:24-34`.
- Current flat UI projection: `runtime-index-leaf.md:35-38`.
- Command observability gap and smallest probe: `runtime-index-leaf.md:39-42`; `runtime-source-evidence.md` for full source map.
- Proposed renderer-independent node/edge model: `recommendation.md:10-35`.
- Runtime/static identity boundary and command decision: `recommendation.md:83-97`.
- Delivery proof boundary (live/replay and unresolved mappings): `recommendation.md:105-107`; concrete slice proof: `delivery-slices.md:17-21`.

Adjacent route: [index-milestones.md](index-milestones.md) maps these gaps to #173, #120, and #197; [index-rules.md](index-rules.md) covers static scope provenance.
