# Recommendation: Set keys

**Current decision supersedes the earlier advisory recommendation below:** Set/SetJSON keys must be compile-time constants and nonconstant keys hard-fail lint. The builtin sprint is the required negative case. See constant-context-key-rule.md and #162. The following study preserves the earlier reasoning; its recommendation to allow dynamic keys is no longer the shipping policy.

Recommend constant outer keys as the normal authoring style, but do not make
every dynamic `Set`/`SetJSON` key a build-blocking error in the first analyzer.
Keep #162 focused on definite scope misuse and duplicate writes, as its current
shape already does by skipping non-constant keys (`ephemeral/research/graphs/issue-162.md:10-20`).
The graph should retain a dynamic write site as an unresolved or aggregated
value operation with its source location; the static result must say that its
runtime field names are unknown. This follows the graph proposal’s requirement
to keep unresolved alternatives explicit (`ephemeral/research/graphs/recommendation.md:69-75`)
and avoids claiming that an incomplete static field map is a runtime failure.

The constant preference is still valuable. A constant outer key gives the page,
`ScopeText`, and future generated data edges a stable field identity, while the
runtime already gives set-once and scope-lifetime protection
(`scope.go:140-182`; `ephemeral/research/api/API.md:237-255`). Prefer constants
for structural names where exact graph identity is a supported analyzer
contract; leave unknown or unsupported dynamic sites unresolved rather than
introducing a blanket authoring ban. For `Set`/`SetJSON`, treat dynamic keys as
a disclosed precision loss until representative migrations show that a hard
gate does not distort ordinary workflows.

This is a recommendation for the first delivery, not a universal claim that
arbitrary workflows can be rewritten naturally. The repository’s own evidence
contains both a simple fixed-set case and runtime-indexed results
(`internal/workflows/sprint/sprints.go:252-260`; `ephemeral/attest/loop-practice/group-in-task/main.go:151-178`).

## Design sketches

### 1. Sprint checks: aggregate the fixed list

Current code uses an index to make two headings:

```go
for i, check := range checks {
	code, output, err := command(ctx, in, check)
	if err != nil { return err }
	gimble.Set(ctx, fmt.Sprintf("repository check %d", i+1), commandText(check, code, output))
}
```

The smallest constant-key rewrite keeps the loop and the planner-visible task
scope, while moving the dynamic identity into data:

```go
var evidence []string
for _, check := range checks {
	code, output, err := command(ctx, in, check)
	if err != nil { return err }
	evidence = append(evidence, commandText(check, code, output))
	if code != 0 { passed = false }
}
gimble.Set(ctx, "repository checks", evidence)
```

This is a reasonable migration because `checks` is currently exactly two
fixed commands (`internal/workflows/sprint/sprints.go:55,252-260`). It changes
two `ScopeText` headings into one aggregate, so it should be accepted only if
that presentation change is acceptable. It also changes failure behavior:
because the current code sets each result immediately, a later command failure
or cancellation leaves earlier evidence in the run; a final aggregate write
would lose it. A deferred partial flush could preserve that evidence, but adds
complexity and must still honor Set-once. A generated `SetJSON` slice of records
is preferable when exit code and command need structured fields. The aggregate
key is constant; any command names or future map members remain payload data.

### 2. Parallel attempts: preserve runtime identity as scopes or records

Current code waits for all `Group.Go("attempt", ...)` children and then writes
`attempt 1`, `attempt 2`, and `attempt 3` into the parent task
(`ephemeral/attest/loop-practice/group-in-task/main.go:151-178`). There are two
natural constant-key designs:

```go
// Inside each existing Group.Go child, after Generate:
gimble.Set(attemptCtx, "outcome", outcome)
```

This makes the already-existing repeated `attempt` scope carry its own stable
field. It is the best page model, but it moves the values out of the task’s
direct local values. Since Loop uses those direct values for the next planner
decision (`loop.go:131-140`; `scope.go:204-215`), preserve planner behavior by
also recording one parent aggregate after `Wait`:

```go
gimble.Set(ctx, "attempt results", attemptRecords)
```

That duplicate projection is extra code and should be used only when both page
identity and planner feedback matter. If only feedback matters, omit the child
write and use the aggregate alone. A per-attempt `Scope(ctx, "attempt", ...)`
would be legal with today’s API, but it adds nesting around work that already
has `Group.Go` scopes and still needs the aggregate for planner visibility.

### 3. Dynamic JSON members: keep the outer graph key fixed

For a workflow that collects results by a runtime id, prefer one stable field:

```go
type Results struct {
	ByID map[string]Result `json:"by_id"`
}

gimble.SetJSON(ctx, "results", Results{ByID: byID})
```

The sketch is schematic: `Results` needs polytype-generated `Schema` and
`ValidateJSON` methods before `SetJSON` accepts it, and map support in that
grammar must be confirmed. If it cannot represent that map, use a generated
slice such as `[]ResultRecord{{ID: id, ...}}` under the same constant `"results"`
key. Existing output shapes already use slices in generated records
(`example_shapes_test.go:85-98`), while current `SetJSON` rejects arbitrary
maps (`ephemeral/research/api/API.md:220-235`). Do not force one scope or one
top-level key for each member solely to satisfy static extraction.

## Is sprint complexity unnecessary?

The specific dynamic repository-check key is unnecessary complexity: the set is
small and fixed, so an aggregate or two explicit writes can be reviewed easily.
That does not establish that dynamic keys are unnecessary in workflows. The
parallel-attempt example has runtime cardinality and useful attempt identity;
the final-check example has a dynamic filename set
(`ephemeral/review/codex-loop-api/main.go:131-140`). A universal rule would
push authors toward extra scopes, duplicated aggregates, or altered planner
feedback merely to satisfy a static node inventory.

## Revisit criteria

Make dynamic keys build-blocking later only if all of the following evidence is
available:

1. A representative workflow corpus shows that fixed-set dynamic writes can be
   migrated to aggregates or constants without changing planner prompts,
   page usefulness, or source readability.
2. The API has a supported aggregate path for the needed values, including
   generated slice records and, if required by real workflows, map-valued JSON
   fields. `Each` should not be assumed until it exists; it is currently
   documented but unimplemented (`ephemeral/research/api/API.md:257-270`;
   `ephemeral/research/api/SPRINTS.md:89-91`).
3. Graph consumers demonstrate that an unresolved dynamic write makes the view
   materially unusable, rather than merely less precise. Until then, retaining
   the source site plus `dynamic key` is honest and sufficient for a proposed
   template graph (`ephemeral/research/graphs/recommendation.md:75,103-105`).

If those experiments fail—especially if planner-visible semantics require
parent projection after every nested result—keep the rule advisory or scoped
to graph-critical fields. Do not claim arbitrary workflows can be rewritten
naturally from the current evidence.
