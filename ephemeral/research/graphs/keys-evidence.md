# Evidence on constant Set keys

Audit date: 2026-09-14. This note answers the bounded question whether every
`Set`/`SetJSON` key should be a compile-time constant while the graph and lint
are still being established.

## What the API actually guarantees

`Set` and `SetJSON` share one per-scope key namespace. A write is a snapshot and
the second write to the same key in one scope panics; a child scope is the
documented way to revise or repeat a value (`scope.go:140-182`,
`ephemeral/research/api/API.md:237-255`). The design also says names and keys
are compile-time constants and proposes making a non-constant key a build
failure (`ephemeral/research/api/API.md:243-247,275-293`). The runtime does not
enforce that rule: it stores a runtime string in `map[string][]byte`
(`scope.go:31-42,163-181`; `ephemeral/research/graphs/rules-report.md:45-60`).

The graph proposal only treats constant keys as inspectable data metadata and
explicitly says unresolved alternatives should remain visible
(`ephemeral/research/graphs/recommendation.md:15-24,69-75`). Thus a dynamic key
is a loss of static field identity, but it need not make the entire workflow
unextractable.

## Observed dynamic uses

| Site | Why the key is dynamic | Natural shape if a constant outer key is required | Cost or semantic change |
| --- | --- | --- | --- |
| `internal/workflows/sprint/sprints.go:252-260` | A fixed two-element `checks` list is indexed to record each command as `repository check 1` and `repository check 2`; the list is declared at `sprints.go:55`. | One `Set(ctx, "repository checks", []string{...})`, or a generated `SetJSON` aggregate containing check name, exit code, and output. | One ScopeText field replaces two headings; an aggregate preserves values but changes presentation. It also delays the write until the loop ends, so a later command error or cancellation can lose earlier results that the current code has already recorded. A deferred partial flush is possible but adds complexity. Unrolling two fixed calls preserves headings but couples the workflow to the current list. |
| `ephemeral/attest/loop-practice/group-in-task/main.go:151-178` | Three parallel attempts return independently, and the parent records `attempt 1`, `attempt 2`, etc. after `Group.Wait`; the attempt number is runtime data. | `Set(ctx, "attempt results", []string{...})` or a generated slice of `{attempt, outcome}` records. Alternatively set constant `outcome` in each existing `Group.Go("attempt", ...)` child. | Child writes naturally place data under the attempt scopes, but `Loop` carries only the task scope’s direct local values to its next planner prompt (`loop.go:131-140`; `scope.go:204-215`). A parent aggregate is needed to preserve current planner feedback. |
| `ephemeral/review/codex-loop-api/main.go:131-140` | Final checks are selected by a slice of filenames, and the filename is used as the key. | A constant `final checks` aggregate, or two explicit constant writes if the set is permanently fixed. | An aggregate is slightly less convenient to inspect by heading; explicit writes are harmless only while the set stays fixed. |

These are not all the same case. The sprint and final checks are small fixed
sets whose dynamic names are mostly presentation. The attempt keys describe a
runtime-indexed collection, where making one scope or one key per result is a
real modeling choice. The API’s own bake-off example already treats attempts
as repeated child scopes and says that this is what makes the page show N
attempts (`ephemeral/research/api/API.md:456-510`).

## Aggregates and payload keys

The static rule should concern the outer storage key, such as
`"attempt results"`. Dynamic identifiers inside an aggregate are ordinary
data. A JSON object with runtime map members, or a slice of records carrying an
`id`, does not erase the outer graph node. Today `SetJSON` accepts only a
polytype `Output`; arbitrary structs and maps are rejected by the generic
contract (`scope.go:20-24,149-153`; `ephemeral/research/api/API.md:220-235`;
`ephemeral/research/graphs/rules-report.md:58-60`). A `Results` example must
therefore have generated `Schema` and `ValidateJSON` methods before it can be
passed to `SetJSON`, and map support in the current polytype grammar must be
verified rather than assumed. Existing generated output already demonstrates
slice fields (`example_shapes_test.go:85-98`). If map members are unavailable,
a generated slice of records is the natural fallback. That limitation should
not be solved by inventing one `Set` key per map member.

## Existing per-iteration API boundary

The documented `Each` iterator would create a fresh named scope per item, but
it is design text, not an available API: it is listed as not yet built in
`ephemeral/research/api/SPRINTS.md:89-91`, while the API describes it as future
work in `ephemeral/research/api/API.md:257-270`. `Scope` is available and can
wrap a plain-loop body with a constant name; runtime ordinals then distinguish
instances. However, a nested scope’s values are not automatically promoted to
the parent task’s local text (`scope.go:204-215`), so using it to replace a
dynamic parent key changes what the next Loop planner sees unless the workflow
also writes a parent aggregate.

## What the current issue actually asks

Issue #162 deliberately skips non-constant keys in its first analyzer shape
(`ephemeral/research/graphs/issue-162.md:10-20`) and calls for a clean report on
the existing sprint and loop inputs (`issue-162.md:22-35`). The later graph
recommendation notes that adopting a hard constant-key gate would require a
sprint migration (`ephemeral/research/graphs/recommendation.md:91-95`). That
is evidence for sequencing the misuse checks independently from a universal
dynamic-key prohibition.
