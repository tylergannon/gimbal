# Nested Struct Field Paths and Gimbal Parameter Shapes

## Polytype and SKGO Form Nested Path Support

SvelteKit's form convention supports nested object paths on the wire:
- In SvelteKit's `form-utils.js:16-18, 95-110`, controls named `author.name` or `params.goal` are split by `split_path` and assigned via `deep_set` into nested POJOs.
- SKGO mirrors this in `internal/formdata/convert.go:213-250` (`setNested`) and `decode.go:177-212` (`assignObject`).
- When a form submission contains dotted paths (e.g. `params.goal` or `models.code-review`), `convert_formdata` and `ParseWith` construct a `*devalue.Object` hierarchy where top-level keys map to nested `*devalue.Object` instances.
- In `internal/formdata/decode.go:188-191`, `assignObject` looks up struct fields matching wire names. If a struct field is itself a struct (e.g. `Params Params`), `assign` recurses into `assignObject` for that child struct.
- In Polytype's generated devalue codecs (`polytype/devalue/codegen/decode.go:207-233`), nested struct fields are lowered into `decodeObjectInto` calls using selector chains on the parent value (e.g. `target.Params.Goal`).

## Gimbal's Workflow Parameter Shapes

Across the five stock built-in workflows in Gimbal:
1. `review` (`internal/workflows/review/review.go`):
   - `Goal string`
2. `implement` (`internal/workflows/implementation/implementation.go`):
   - `OutcomesFile string`
   - `MaxTasksPerOutcome int`
3. `validate-product` (`internal/workflows/validateproduct/validateproduct.go`):
   - `SuiteFile string`
4. `research-document` (`internal/workflows/researchdocument/researchdocument.go`):
   - `Goal string`
   - `ResearchDir string`
   - `Output string`
   - `TokenBudget int`
   - `MinSourcesPerTopic polytype.Optional[int] json:",omitzero"`
   - `MaxEditorialRounds polytype.Optional[int] json:",omitzero"`
5. `pyramid-summary` (`internal/workflows/pyramidsummary/pyramidsummary.go`):
   - `Goal string`
   - `SemanticIndex string`
   - `LargestDocument string`
   - `OutputDir string`
   - `LargestTokenBudget polytype.Optional[int] json:",omitzero"`

### Struct Conventions for Shared Start Requests

For generated workflow-start remotes (`implementation-plan.md:115-120`), each request must contain:
1. Owning project directory: `Project string` (`json:"project"`)
2. Execution workdir: `WorkDir string` (`json:"work_dir"`)
3. Optional conversation ID: `Conversation polytype.Optional[string]` (`json:"conversation,omitzero"`)
4. Role model override fields: `Role<RoleName> polytype.Optional[string]` (`json:"role_<name>,omitzero"`)
5. Workflow-specific parameters:
   - Either flat top-level fields (e.g. `Goal`, `OutcomesFile`, `TokenBudget`)
   - Or an embedded/nested `Params` struct (e.g. `params.goal`, `params.outcomes_file`)

Because all workflow parameters across all five workflows are flat scalars (`string`, `int`, `polytype.Optional[int]`), either flat naming or a single-level nested struct `Params` matches both Polytype's typegrammar and SKGO's form decoding without requiring complex recursive data structures, maps, or file uploads.
Flat fields or single-level nested `Params` ensure straightforward SvelteKit form input bindings (`name="params.goal"` or `name="goal"`).
