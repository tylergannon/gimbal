# 11-workflow-packaging

## Purpose and retrieval

- Audience: an author deciding how to package a Gimble workflow for repeated use,
  and an agent reviewing whether its generated CLI documents the workflow well.
  Coverage: authoring instructions, built-in workflow exposure, generator
  extraction, graph/role semantics, and portability limits. Authority is current
  source and current authoring guidance unless marked historical or proposal.
  Retrieval hints: `gimble-workflows`, `gimblegen`, `Command(defaults)`,
  `entryInfo`, `paramFields`, `Roles`, `RegisterGraph`, `--help`, `workflow_gen.go`.

## Authoring contract

- A workflow is ordinary Go in a package under `internal/workflows/`; its entry
  has the shape `func Name(ctx context.Context, env gimble.Env, params NameParams)
  error`, with workflow-owned arguments in a workflow-specific parameter type.
  [current instruction: `.agents/skills/gimble-workflows/SKILL.md:123-129`]
- `gimble.Env` is Gimble-owned and carries the absolute initial `WorkDir`; an
  author must not infer or duplicate that meaning in a user parameter.
  [current source: `env.go:3-6`; historical correction:
  `ephemeral/worklog/202609171047-workflow-roles.md:4-5`]
- The generator directive names both the entry and the CLI workflow name. With
  structured output, the Polytype directive precedes it; `go generate` writes
  `workflow_gen.go` beside the workflow. [current instruction:
  `.agents/skills/gimble-workflows/SKILL.md:123-137`]
- The authoring skill should require a documentation pass alongside code: write
  a package comment that explains the workflow's user-facing purpose and scope,
  an entry comment that says what it does and what it produces, and a comment on
  every exported parameter field that explains accepted input and its effect.
  This is directly actionable because those three comment locations feed the
  generated command's `Long`, `Short`, and flag usage. [current generator:
  `internal/generate/entry.go:14-22`, `internal/generate/entry.go:32-36`,
  `internal/generate/entry.go:70-77`, `internal/generate/entry.go:79-110`]
- Documentation should state help purpose, inputs, outputs, proof/observation
  expected, limits, and examples. Only purpose and input-field text currently
  reach CLI help; the remaining topics are authoring gaps identified below.
  [current generator: `internal/generate/entry.go:14-22`,
  `internal/generate/command.go:18-35`; proposal: this leaf]

## What the generator actually extracts

- `describe` validates that the entry has `context.Context`, `gimble.Env`, and at
  most one same-package named parameter type. It rejects a missing Env, foreign
  parameter type, or more than three parameters. [current source:
  `internal/generate/entry.go:32-63`]
- The entry's doc comment is split into a short synopsis using
  `go/doc.Package.Synopsis`, while the package comment is retained as the long
  description. Synopsis therefore means the first sentence/summary shape, not a
  separate hand-written CLI field. [current source:
  `internal/generate/entry.go:14-22`, `internal/generate/entry.go:32-36`]
- Package documentation is the first file-level `file.Doc.Text()` found in the
  loaded package, trimmed. A package with multiple package comments does not get
  an assembled description. [current source: `internal/generate/entry.go:70-77`]
- Parameter fields are read from the named struct in source order. Only exported
  fields become flags; each flag is kebab-cased from the Go field name, and the
  field's own doc group is trimmed into usage text. [current source:
  `internal/generate/entry.go:79-110`, `internal/generate/entry.go:131-140`]
- Supported flag types are `string`, `int`, `bool`, and `polytype.Optional[T]`
  around one of those types. Unsupported field types fail generation rather than
  being guessed or silently omitted. [current source:
  `internal/generate/entry.go:113-128`]
- Non-optional non-bool fields are marked required and the generated usage adds
  `(required)`; bools are never required. Optional fields are only assigned when
  their flag changed, preserving absence. [current source:
  `internal/generate/command.go:20-31`, `internal/generate/command.go:144-165`]
- The generated command copies `Summary` to Cobra `Short` and `Long` to Cobra
  `Long`, and uses each field comment as the flag's usage. [current source:
  `internal/generate/command.go:18-35`, `internal/generate/command.go:138-146`]
- The generated CLI always adds `--work-dir`, `--port`, `--uds`, and `--no-web`
  with fixed help strings. `--work-dir` defaults to `.`, while the web flags
  choose loopback TCP, a Unix socket, or no web app. [current source:
  `internal/generate/command.go:150-159`]
- For each role returned by the graph, the CLI adds a `--<role>` string flag.
  Its help is fixed as “the model for role …, as model or model:effort”; a
  caller-supplied default makes it optional, while an empty default makes it
  required. [current source: `internal/generate/command.go:33-35`,
  `internal/generate/command.go:151-156`]
- `gimblegen` itself accepts only `-entry`, `-name`, and `-o`; `-entry` and
  `-name` are required and output defaults to `workflow_gen.go`. [current source:
  `internal/generate/gimblegen/main.go:23-38`]

## What is generated and registered

- `Source` loads one package, extracts the graph and command metadata, checks flag
  collisions, formats the result, and writes one generated file. It temporarily
  overlays that output as an empty package clause so an old generated file does
  not prevent the next generation from reading current source. [current source:
  `internal/generate/source.go:16-24`, `internal/generate/source.go:33-50`]
- The generated file declares `Graph`, emits `func init() { gimble.RegisterGraph(Graph) }`,
  and emits `Command(defaults)`. Registration is graph registration only; it does
  not add a CLI subcommand by itself. [current generator:
  `internal/generate/command.go:118-124`]
- A built-in CLI must explicitly import the workflow package and add its generated
  command to `gimble run`. The current binary does that for `review` in one line.
  [current source: `cmd/gimble/workflows.go:7-10`, `cmd/gimble/workflows.go:23-31`]
- The root CLI describes `run` as workflows built into this binary; the generated
  command is therefore not automatically available merely because its graph was
  registered. [current source: `cmd/gimble/workflows.go:23-29`; current source:
  `cmd/gimble/main.go:112-119`]
- The generated `RunE` resolves the work directory, binds role models, creates a
  runtime, builds `gimble.Env`, and invokes the entry. [current generator:
  `internal/generate/command.go:166-192`]

## Graph and role authoring implications

- The graph is source shape: scopes, sessions, agent calls, commands, and values
  written, in source order. It deliberately omits adapters, models, reasoning
  effort, results, timings, and costs because those belong to the run record.
  [current design/source: `workflow/graph.go:5-14`]
- Every graph node has source file and line; diagnostics record unreadable shape
  honestly, and the extractor never completes a hole by guessing. [current
  design/source: `workflow/graph.go:16-30`; current source:
  `internal/generate/graph.go:1-4`, `internal/generate/source.go:79-94`]
- `Graph.Roles()` walks all operation bodies in source order, includes every
  `NewSession`, excludes forks, and de-duplicates names. A role flag therefore
  comes from actual graph session creation, not from a manually maintained role
  list. [current source: `workflow/roles.go:3-38`]
- A fork inherits its parent's model binding; its name is a session name in the
  graph but does not create a role flag. [current design: `workflow/graph.go:46-55`;
  current source: `workflow/roles.go:9-16`]
- Prompts are captured verbatim in `workflow.AgentCall`, while interviews,
  command names, sets, loops, groups, and conditions capture their shape and
  source location. [current design: `workflow/graph.go:57-80`,
  `workflow/graph.go:93-181`]
- The authoring skill should tell authors to make prompts, role names, and command
  names readable because the graph is the no-model way to inspect workflow intent;
  it should not imply that graph metadata is a full user manual. [current
  instruction: `.agents/skills/gimble-workflows/SKILL.md:146-150`; current design:
  `workflow/graph.go:5-14`]

## Built-in examples and documentation evidence

- `review` has package, parameter type, field, result, and entry comments. Its
  generated help consequently exposes a useful purpose, required `--goal`, and
  generic role/model and runtime flags. [current source:
  `internal/workflows/review/review.go:1-26`; generated output:
  `internal/workflows/review/workflow_gen.go:34-55`]
- `interview` documents its package, parameter, field, and entry, and its CLI
  exposes `--topic` plus the interviewer model flag. [current source:
  `internal/workflows/interview/interview.go:1-21`; generated output:
  `internal/workflows/interview/workflow_gen.go:34-55`]
- `implement-interview` demonstrates richer input comments: absolute requirements
  file, reference directory, and maximum task count. Its generated CLI exposes
  those as required flags, but its package/entry help still only describes the
  broad activity and does not state outputs, proof, limits, or an invocation.
  [current source: `internal/workflows/implementinterview/implementinterview.go:1-5`,
  `internal/workflows/implementinterview/implementinterview.go:24-45`;
  generated output: `internal/workflows/implementinterview/workflow_gen.go:91-156`]
- Existing CLI tests prove flag presence, requiredness, and model defaults for
  `review`; they do not assert that help explains outputs, proof, limits, or
  examples. [current test: `cmd/gimble/workflows_test.go:23-66`]

## Precise documentation gaps and portability friction

- Gap: no generated help field describes outputs. Result type comments and JSON
  field comments are used for model schema/prompt text, not Cobra command help.
  [current source: `internal/workflows/review/review.go:19-23`;
  current instruction: `.agents/skills/gimble-workflows/SKILL.md:53-60`]
- Gap: no generated help field describes proof or observation. Runtime records
  prompts/events, but CLI help does not explain what a user should watch or what
  counts as demonstrated. [current instruction: `.agents/skills/gimble-workflows/SKILL.md:146-150`;
  current source: `internal/generate/command.go:138-159`]
- Gap: no generated help field describes operational limits such as maximum
  planner tasks, validation behavior, absolute-path requirements, or failure
  conditions. Those facts remain buried in ordinary entry code and field text.
  [current source: `internal/workflows/implementinterview/implementinterview.go:44-57`,
  `internal/workflows/implementinterview/implementinterview.go:145-162`]
- Gap: no generated help field offers a runnable example. The authoring skill
  shows generic review invocations, but a workflow's own package or entry docs
  cannot add examples to Cobra help. [current instruction:
  `.agents/skills/gimble-workflows/SKILL.md:139-144`; current generator:
  `internal/generate/command.go:138-146`]
- Gap: role help cannot explain a role's purpose or model selection policy; it is
  always generated from the same generic sentence. Role constants are typed
  cognitive skills, but their descriptions are not consumed by the generator.
  [historical decision: `ephemeral/worklog/202609171047-workflow-roles.md:1-3`;
  current generator: `internal/generate/command.go:33-35`]
- Gap: package docs use the first file comment rather than a deliberate exported
  CLI documentation block, and entry synopsis is reduced to a one-sentence
  synopsis. Long, structured help cannot currently be authored through a named
  documentation contract. [current source: `internal/generate/entry.go:32-36`,
  `internal/generate/entry.go:70-77`]
- Gap: there is no generator lint requiring the six documentation topics above,
  and current tests check only flags/defaults. A future authoring requirement
  should add such a check or a documented convention before claiming built-in
  workflows are well documented via CLI. [current test: `cmd/gimble/workflows_test.go:23-66`;
  proposal: this leaf]
- Portability friction: generated commands import `internal/binding` and `web`
  in addition to the public root package, so the generated command is coupled to
  a Gimble repository/module layout. [current generator:
  `internal/generate/command.go:102-115`]
- Portability friction: the generator identifies Gimble calls by the exact module
  path `github.com/tylergannon/gimble`, and package loading expects a Go package
  in a directory with its module/dependency graph. A fork or external module
  cannot casually rename the import path and still be extracted. [current source:
  `internal/generate/graph.go:19-35`; `internal/generate/read.go:17-23`]
- Portability friction: generated `init` graph registration is usable wherever
  the package compiles, but CLI availability still requires an application to
  import the package and add `Command(defaults)` to its own root command. This is
  explicit integration work for external authors and built-ins alike. [current
  source: `internal/generate/command.go:118-124`; `cmd/gimble/workflows.go:23-31`]
- Build friction: `go generate` needs the module's registered tools and Polytype
  generation for structured outputs; the repository pins those tools in `go.mod`.
  [current source: `go.mod:48-56`; current instruction:
  `.agents/skills/gimble-workflows/SKILL.md:53-60`]
- Build friction: the generator writes into the package directory and requires a
  compilable package load (apart from its own temporary generated-file overlay),
  so a missing dependency or invalid non-generated source blocks generation.
  [current source: `internal/generate/source.go:21-24`,
  `internal/generate/graph.go:28-49`]

## Recommended authoring topics

- Require package/entry/field comments with purpose, inputs, outputs, proof,
  limits, and one concrete CLI example; treat the current extraction as the
  minimum baseline, not evidence that all six topics are exposed. [proposal,
  grounded in current extraction: `internal/generate/entry.go:14-22`,
  `internal/generate/entry.go:32-36`, `internal/generate/entry.go:79-110`]
- Document every role's purpose where users choose a model, and document that
  role flags are generated from `NewSession` names and forks do not add flags.
  [current source: `workflow/roles.go:3-38`; current design:
  `workflow/graph.go:46-61`]
- For each built-in, prove `gimble run --help` and `gimble run <name> --help`,
  then inspect that the visible text answers what it does, what to pass, what it
  returns or changes, how to tell it worked, and where it stops. [current
  instruction: `.agents/skills/gimble-workflows/SKILL.md:139-150`; proposal: this
  leaf]
