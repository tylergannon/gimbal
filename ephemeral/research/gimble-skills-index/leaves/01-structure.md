# Leaf 01: workflow structure, ownership, and judgment

## Source coverage
- `AGENTS.md` (current instruction): the simplicity rule, no-wrapper rule, proof
  boundary, local prompt inputs, call-site naming, and repository layout.
- `doc.go` (current implementation contract): package purpose, ordinary Go,
  scopes/groups/iteration/planning, prompt/context behavior, command and
  cancellation semantics.
- `example_test.go` (current executable teaching evidence): one turn, Group,
  PromiseLoop, and Iterate shapes.
- `example_shapes_test.go` (current executable teaching evidence): fork/bake-off,
  critique, supervision, planner loops, worktrees, validation, and killed-turn
  recovery.
- `group.go` and `run.go` (current implementation): group cancellation/join,
  panic treatment, run lifetime, session cleanup, recording errors, and live
  controls.
- `ephemeral/research/api/API.md` (design record, explicitly non-authoritative
  where it differs from Godoc/examples): rationale, anti-patterns, scopes,
  context ownership, observability, commands, and open decisions.

## Proposed teaching topics
### 1. Author workflows as readable ordinary Go
- Teaching/decision: write a program-shaped workflow inside `Run`; keep tactics
  (research, bake-off, critique, retry, worktree, delivery) inline. Add an
  exported name only when an existing workflow needs it; do not hide meaning in
  `workflows.BakeOff`-style wrappers. [AGENTS.md:7-30]
- Audience: Author workflows.
- Evidence: `doc.go` says normal Go control flow is the workflow and runtime
  primitives are not named tactics. [doc.go:8-13]
- Evidence: the bake-off and critique examples make decisions, bounds, prompts,
  and result handling visible in their bodies. [example_shapes_test.go:127-188]
- Authority: current instruction + current implementation/examples; API design
  rationale is supporting historical/design evidence. [ephemeral/research/api/API.md:1-12]
### 2. Make graph identity visible at every node
- Teaching/decision: pass constant names at `Scope`, `Group`, `Group.Go`,
  `NewSession`, `Fork`, `PromiseLoop`, and `RunCommand` call sites. Names are
  graph node keys and static/runtime join points, not dynamic labels. [AGENTS.md:69-70]
- Audience: Author workflows; Build and release Gimble.
- Evidence: examples consistently use literal names (`draft`, `candidate`,
  `coder`, `round`, `work`). [example_test.go:62-80; example_shapes_test.go:145-164]
- Evidence: the design record explains that runtime-generated names hide nodes,
  while scope paths join static shape to runtime instances. [ephemeral/research/api/API.md:750-781]
- Authority: current instruction/implementation intent; static pass details are
  design-record material and may remain future tooling. [ephemeral/research/api/API.md:872-886]
### 3. Treat context as ownership, not a convenience value
- Teaching/decision: derive all work from the context whose lifetime owns it;
  pass child scope contexts into bodies and prompts; never sever the chain with
  `context.Background()` or retain a scope context in a struct/global. [ephemeral/research/api/API.md:117-145; ephemeral/research/api/API.md:291-309]
- Audience: Author workflows; Use Gimble.
- Evidence: `Project` puts project state in the root context, and `Run` requires
  that project context before creating a run. [run.go:32-36; run.go:122-151]
- Evidence: `doc.go` specifies cancellation propagation, scope/session closure,
  and prompt rendering from scope data. [doc.go:15-31]
- Authority: current implementation contract + design record; context linting is
  a proposed/static-analysis capability, not proof that every caller is linted.

### 4. Use scopes for data and lifetime; use Set deliberately
- Teaching/decision: a scope is both a bounded segment and the unit of visible
  data. Set values once per key in the owning scope; shadow in a child scope for
  revision; use `SetJSON`/typed outputs for structured values. [ephemeral/research/api/API.md:148-169; ephemeral/research/api/API.md:225-271]
- Audience: Author workflows; Use Gimble.
- Evidence: examples set repository, prompts, defects, worktree, and results in
  the scope that owns the next agent turn. [example_test.go:45-53; example_shapes_test.go:203-233; example_shapes_test.go:348-379]
- Evidence: `doc.go` states Generate appends rendered scope context explicitly,
  so the prompt source remains readable and constant. [doc.go:15-23]
- Authority: current contract/examples, with set-once and typed-value rationale
  from the design record.

### 5. Structured concurrency is an authoring obligation
- Teaching/decision: start workflow goroutines only with `Group.Go`; call
  `Wait` on every exit path before returning. Group failure cancels siblings,
  but operator `Killed` leaves siblings running; cancellation alone does not
  join. [group.go:19-33; group.go:58-96]
- Audience: Author workflows; Use Gimble.
- Evidence: `ExampleGroup` submits children and returns `group.Wait()`. [example_test.go:62-81]
- Evidence: API explicitly rejects raw `go`, `sync.WaitGroup`, and `errgroup` in
  workflow packages because an unjoined child violates scope lifetime. [ephemeral/research/api/API.md:173-224; ephemeral/research/api/API.md:291-305]
- Authority: current implementation + current examples; API is design rationale.

### 6. Place sessions where their lifetime and workdir belong
- Teaching/decision: sessions belong to their creating scope and close when that
  scope ends. Create cross-iteration state outside an iterator; create per-task
  sessions inside its task scope. Fork when shared conversation is useful, and
  make worktree setup/removal ordinary commands in the owning scope. [ephemeral/research/api/API.md:196-220]
- Audience: Author workflows; Use Gimble.
- Evidence: planner loop researches once, forks planner, and creates task coders
  in task scopes. [example_shapes_test.go:278-323]
- Evidence: worktree example defers removal with `context.WithoutCancel` so
  cleanup still runs after group cancellation. [example_shapes_test.go:332-381]
- Authority: current examples + design rationale; exact adapter behavior is
  current implementation territory.

### 7. Keep adaptive judgment in the workflow, not in a primitive
- Teaching/decision: use `Iterate` for a known finite collection and
  `PromiseLoop` when a planner adaptively chooses tasks; range tasks and check
  `loop.Err()` afterward. A reviewer finding is input to workflow judgment, not
  an automatic order. [ephemeral/research/api/API.md:970-982; example_shapes_test.go:184-188]
- Audience: Author workflows; Use Gimble.
- Evidence: examples show explicit round bounds, defect decisions, planner
  feedback via `Set`, and loop termination by an empty task choice. [example_shapes_test.go:203-240; example_shapes_test.go:280-329]
- Authority: current decision in API design record plus executable examples;
  current exported signatures should be checked against Godoc before release.

### 8. Separate turn errors, command answers, cancellation, and cleanup
- Teaching/decision: return ordinary errors from failed turns; inspect typed
  `Killed` when recovery is intended; treat a command's nonzero exit as its
  answer and `err` as failure to execute; propagate `loop.Err`; let `Run` join
  body, close, and recording failures. [ephemeral/research/api/API.md:38-43; ephemeral/research/api/API.md:941-967]
- Audience: Use Gimble; Build and release Gimble.
- Evidence: killed-turn example retries the same session after recording who and
  why killed it; Group treats recovered child panic as an error. [example_shapes_test.go:441-506; group.go:46-83]
- Evidence: `Run` closes sessions before terminal records and returns body error
  joined with `CloseError` and recording failures. [run.go:122-135; run.go:215-239]
- Authority: current implementation + current executable evidence; API command
  details are design-record contract and should not outrank compiled Godoc.

### 9. Build/release around generated shape and real proof
- Teaching/decision: build the binary with generated graph registration; do not
  hand-edit generated files. Validate repeatable behavior with package tests,
  then run the real workflow and report what was observed. Keep ephemeral notes
  out of production proof artifacts. [AGENTS.md:39-50; AGENTS.md:71-78]
- Audience: Build and release Gimble.
- Evidence: `RegisterGraph` is intended to be called only by generated init code;
  duplicate workflow names panic. [run.go:38-57]
- Authority: current repository instruction + current implementation. The static
  analyzer/graph details in API are proposal/design record until implemented.

## Retrieval hints
- “ordinary Go / no wrapper / inline tactic” -> this leaf topics 1 and 7;
  inspect `example_shapes_test.go` before suggesting a new primitive.
- “scope / context / set / prompt” -> topics 3 and 4; compare `doc.go` with
  API scope rules and cite the current contract first.
- “parallel / wait / cancellation / panic” -> topic 5 plus `group.go`.
- “planner / fixed items / loop error” -> topic 7 and API current decision.
- “release / generated graph / proof” -> topic 9, `run.go:RegisterGraph`, and
  AGENTS proof rules; do not treat a green aggregate gate as runtime evidence.

## Gotchas, conflicts, and gaps
- `API.md` declares itself a design record, not the public contract; Godoc and
  compiling examples win when names or behavior differ. [ephemeral/research/api/API.md:1-5]
- API examples use historical names such as `Each`/`Loop`, while current
  examples and package docs use `Iterate`/`PromiseLoop`; teach the current names
  and label older passages as design history. [doc.go:8-13; ephemeral/research/api/API.md:273-286; ephemeral/research/api/API.md:970-982]
- `Run` panics on misuse and on a recovered child panic after recording terminal
  state; callers should not flatten these into ordinary validation errors. [run.go:185-198; run.go:201-213]
- `Steer`/`Interrupt` and live cancellation are operator surfaces; a workflow
  should use them only with an explicit lifetime owner and should expect races
  (a steer may be dropped). [ephemeral/research/api/API.md:45-57; run.go:329-405]
- Open design items include restart, run-directory discovery, and compacting;
  do not teach them as existing APIs. [ephemeral/research/api/API.md:984-1013]
