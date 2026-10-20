# Durable Interactive: one delivery unit

Status: finalized proposal pending the independent Opus spec/plan rereview. Do not
start implementation until the manager resolves that review and finalizes
this handoff. This document is a per-slice technical plan; `specs/durable-interactive.md` holds the durable contract.

## Ownership and shape

Use two Sol 6.1 implementers in this shared worktree, one delivery unit:

- Runtime owner: J/E/I/H/P, including runtime tests, shared host, native adapter
  continuity where required, message/archive ingress and application views.
- Authoring owner: A, all built-ins/examples, generation/admission tests and
  generated source. Own the ordinary built-in conversation workflow and its
  inventory entry; coordinate its host-facing invocation with the runtime owner.
- Manager: authoritative nlspec, source map, reviews, this plan and worklog;
  adjudicate construction blockers and perform one final acceptance pass.

Do not spawn more agents. Other work exists in the checkout: do not revert
others' edits. Keep each owner's focused checks local; runtime owner builds
and runs final broad checks after the authoring owner has finished generation.
The manager reuses that evidence and observes the real app, rather than running
parallel duplicate full suites. Coordinate changed generation inputs before the
single final generation pass.

Deliver one vertical feature across the existing runtime, shared host and
conversation UI. There is one shared validation/acceptance pass for the final
implementation, not a suite and separate review for every internal task.
Keep actual implementation choices in ordinary Go and existing modules. No
new root public names beyond the approved Interactive, InteractiveTranscript,
InteractiveMessage and EndedBy contract.

## Control flow to change

```diff
 project admission / workflow daemon
-  independent workflow subprocess per invocation
-  separate conversation adapter manager
+  shared host admits saved ordinary workflow invocation
+    Run creates or reconstructs the same recorded run
+      library operations reuse recorded completed outcomes
+      Interactive
+        typed agent result -> committed reply plus durable wait
+        durable input -> steer active turn or schedule continuation
+        human finish -> safe interruption -> normal return
+      Group preserves waiting branches and runnable siblings
+    no runnable branch -> dormant logical run without scope completion
 observation and chat controls
-  answer depends on a live in-memory Interview waiter
+  submit durable input by run/interaction identity
+  project committed state and normal archive completion
```

## Countable source scope

| Cell | Existing source to change or replace |
| --- | --- |
| J | runstore contract and directory journal; use its existing identity/batch seam |
| E | run/scope/group/session/command and recording/context reconstruction; lifecycle schema |
| I | replace interview.go and its transcript/decision generation with Interactive and durable input handling |
| H | host, runner, invocation and conversation ownership; preserve project/environment routing |
| P | observation reduction and conversation/interaction routes and components |
| A | workflow extraction/linting, schemas/graphs and the built-in workflows affected by the library-only rule |

Before adding production files outside the spec's measured ownership sets,
report the exact paths and their cell to the manager for budget assignment.
The existing built-in workflow source contains direct I/O and clock calls;
they cannot be hidden in helpers or exempted from the new authoring rule.
Use existing library operations for those effects. Initial preparation is
not an excuse to reread mutable inputs during replay. RunCommand returns full
captured stdout/stderr to its caller and on replay; display excerpts remain
bounded. The user explicitly approved this contract change. No file/clock API
or Activity is added. First entry is human-first for every occurrence.

Same-code recovery compares executable plus loaded plugin digests. A changed
build visibly blocks old runs, preserving their inbox and outcomes; do not
build a multiversion execution manager. The stable interaction address omits
occurrence ordinal, while operation results retain it. Native resource handling
has no policy.

## Work items and acceptance denominator

1. Implement recorded activation/reconstruction and the necessary durable
   inbox/transition storage, reusing existing IDs, ownership and atomic
   batches. Preserve completed outcomes, bindings and scoped context before
   dispatch; unresolved native work is never blind-replayed.
2. Replace Interview end to end with the approved Interactive call and
   generated chat/end union, ordered transcript, durable EndedBy, steering,
   durable waits and finish signal. Keep input identity through races and
   conversation re-entry; agent end cannot cause an autonomous spin loop.
3. Wire shared hosting and the ordinary conversation workflow through the
   real application's existing project and placement paths. Persist enough
   invocation/code information for same-code reconstruction and startup
   discovery. Preserve services/claims on suspension without defining their
   restoration behavior.
4. Replace the app's live-waiter admission path with durable message/archive
   ingress and project the real unfinished/terminal/recovery states. Read
   web idioms and Svelte factoring instructions before UI changes. Keep the
   frontend small; do not redesign unrelated chat/workspace surfaces.
5. Enforce library-only effects and observations for accepted workflow
   bodies/helpers, update affected built-ins through existing library calls,
   and regenerate maintained output. Remove replaced Interview code and
   parallel conversation execution instead of adding compatibility paths.
6. Demonstrate routed C1-C12 and L1-L4 claims. Add meaningful package-adjacent
   tests for the changed contracts, finish one consolidated build/check
   cycle, then observe the real hosted application with an inexpensive
   unversioned native model alias and disposable catalog. Report the actual
   model and observations, including any unmet claim. Commit and push
   interesting milestones with Conventional Commit titles.

These are internal work items of one slice, not separate delivery gates. An
error behavior named in the nlspec belongs in this slice. Fix actual failures;
do not broaden the task for hypothetical improvements.

## Checks and proof

During implementation, run focused checks that answer concrete questions.
At the final coherent head, generate/build once and run the affected backend,
frontend, analysis and formatting checks required by the repository. Record
the tested head and exact outcomes in the report to the manager. The manager
will reuse valid evidence rather than rerun unchanged broad suites. A changed
input or a new failure justifies rerunning the affected check.

Use the actual maintained app/workflow paths for the live demonstrations.
Unit stand-ins do not prove native continuity or browser behavior. Do not
commit proof programs, transcripts, screenshots, run logs or results dumps;
report observed proof in chat/PR, with repeatable checks beside their code.

Native agent resource handling has NO POLICY. Do not add detach/retain,
eviction, adapter-close-count or native-process-count tests asserting one.
Prove continuity and correctness without prescribing those tactics. SERVICE
restoration/rechecks remain deferred. Supporting both entry modes and a new
Activity concept are not required work.

If a budget is exceeded, a deferred decision is encountered or the work
requires an underivable seam/public contract, report it to the manager with
the concrete construction blocker. Do not amend the nlspec yourself. Continue
independent authorized work where possible. Do not remove, skip or weaken a
failing acceptance check.
