# Intake built-in recon

## Scope

This is a read-only design recon for a small CLI entry flow. The three local
skill documents were read as design references only:

- `.agents/skills/df-sprint-plan/SKILL.md`
- `.agents/skills/df-sprint-execute/SKILL.md`
- `.agents/skills/df-easy-loop-e2e/SKILL.md`

Their multi-agent ceremonies should not become the built-in's runtime model.
The useful distinction to retain is the amount of planning a request needs:
an immediately executable supervised turn, a plan followed by execution, or a
pre-existing sprint plan that only needs execution.

## Current entry points and API facts

- `cmd/main.go` has a direct `run(args, stdout, stderr, getenv)` switch for
  `run-prompt`; every other invocation enters the web server. This is the
  natural place to add named built-in routes. Keep each route as an explicit
  `if`/`switch`, with ordinary argument parsing in that route.
- `cmd/run_prompt.go` is the closest CLI pattern: a command-specific options
  struct, `flag.NewFlagSet`, explicit model resolution, a signal-bounded
  context, `web.NewRuntime(..., web.WithNoWeb())`, then
  `runtime.Run(..., func(ctx) error { ... })`. It prints the result and the
  run directory after completion.
- `internal/workflows/sprint/sprints.go` has the existing typed `sprint.Input`
  with JSON-schema generation and a direct `Sprint(ctx, Input)` entry. It
  shows the intended shape for structured inputs and uses `SetJSON(ctx,
  "input", in)` to make the request observable.
- `gimble.NewSession` owns one conversation in the current scope;
  `Generate[T]` is blocking and can attach `WithSupervisor`; `Fork` preserves
  conversation; `Loop` is the existing planner-directed dispatch primitive.
  There is no question or interactive prompt primitive in the root API.
- `gimble.Text` is sufficient for prose turns. A new structured output type
  must implement the existing `Output` contract (normally by polytype
  generation); no root-level export is needed for a command-private type.

## Smallest useful user flow

Use one command with a short, direct intake followed by one planning-choice
turn:

```text
gimble lfg [request text]
```

The request may be supplied as positional text, `--file`, or stdin. The CLI
should preserve the user's raw text and optionally label it as `goal`,
`design`, `claim`, or `detail` only if the user supplies that label. Do not
force a taxonomy on an unlabelled request.

Inside one `Run`, normalize the text into a small scoped brief and ask one
planner turn how much planning is warranted. Present exactly three routes:

1. `lfg`: one coding agent turn with one supervisor attached. This is the
   shortest path for a bounded request whose desired result and evidence are
   already clear.
2. `sprint-plan`: produce a plan/checklist first, then stop or hand the
   resulting local plan to execution according to the explicit command mode.
3. `sprint-execute`: consume an existing local sprint/checklist artifact and
   run the implementation loop.

Route the answer with a direct `switch` in the workflow/CLI. The selected path
should be visible in the run's scoped data. The `lfg` branch can be entirely
expressed today with `NewSession`, `Generate[gimble.Text]`, and
`WithSupervisor`; the other branches should call direct workflow code rather
than a generic registry or dynamic function dispatch.

The command names should remain explicit aliases for these three experiences:
`gimble lfg`, `gimble sprint-plan`, and `gimble sprint-execute`. If a single
entry command is preferred, `lfg` can own the choice and the two sprint names
remain explicit escape hatches. Avoid making `lfg` a high-level exported
wrapper in package `gimble`; it belongs in `cmd` or an internal workflow
package.

## Suggested typed shapes

Keep the input boundary concrete and command-owned. A useful internal shape is

```go
type Input struct {
	Request string `json:"request"`
	Kind    string `json:"kind"`    // goal, design, claim, detail, or empty
	Plan    string `json:"plan"`     // lfg, sprint-plan, sprint-execute
	Repo    string `json:"repo"`
	Model   string `json:"model"`
}
```

`Request` is the only required user content. `Kind` is descriptive metadata,
not a dispatcher. `Plan` is the selected route after the guidance turn (or an
explicit CLI override). Keep defaults at the CLI boundary, and record the
resolved value with `SetJSON`; do not ask the model to manufacture repository
paths or hidden defaults.

The guidance turn should return one small structured value, for example:

```go
type Guidance struct {
	Plan   string `json:"plan"`
	Reason string `json:"reason"`
}
```

The prompt must constrain `Plan` to the three literal choices and ask for a
short reason. Validate the choice in ordinary Go immediately after
`Generate[Guidance]`, then `switch` on it. If polytype supports an enum/tag in
the selected version, use it; otherwise the explicit Go check is the truthful
boundary. Do not use reflection, a map of handlers, or a generic command
registry.

The separate planning worker should use its own plain Go input rather than
passing this intake type across package boundaries:

```go
type Input struct {
	Repo          string
	Goal          string
	Acceptance    string
	Constraints   string
	ContextFiles  []string
	Model         string
	ReviewModel   string
	PlanningModel string
	OutputDir     string
}

func Plan(ctx context.Context, in Input, reader *bufio.Reader, writer io.Writer) (string, error)
```

Keep that `Input` local to the planning package. Generated polytype schemas
are for a package's own generated types; external package types are unsupported
in the current setup. The CLI can construct this input field by field from the
intake state. `reader` and `writer` make human clarification explicit without
inventing a Gimble question primitive, and the worker can use one typed model
question at a time internally.

## Questions representation

There is no `request_user_input` API in the current Gimble package. Do not
pretend a model-generated question is an interactive runtime primitive.

For this first flow, avoid questions entirely: collect one raw request and
have the guidance turn choose the smallest sufficient route. If a later
requirement truly needs clarification, represent it as a typed model output
and loop in ordinary Go:

```go
type Question struct {
	Prompt  string   `json:"prompt"`
	Choices []string `json:"choices"`
}

type IntakeTurn struct {
	Question *Question `json:"question,omitempty"`
	Plan     string    `json:"plan,omitempty"`
}
```

The CLI prints `Prompt` and lettered `Choices`, reads one answer, stores the
verbatim answer in a new scope with `Set`, and generates the next turn. Bound
the number of clarification turns in ordinary Go. This shape is only a future
extension: `Question` needs generated schema support and an explicit
answer-validation rule before it should ship. A slice of questions in one
response would make the CLI harder to drive and weaken the one-question
interaction used by the referenced easy-loop design.

## Planning branch shape

The three branches should share only the intake normalization and the direct
route switch:

```go
switch plan {
case "lfg":
	worker := gimble.NewSession(ctx, "worker", adapter, model, repo)
	reviewer := gimble.NewSession(ctx, "supervisor", reviewAdapter, reviewModel, repo)
	_, err := worker.Generate[gimble.Text](ctx, brief,
		gimble.WithSupervisor(reviewer, supervisionInstruction))
	return err
case "sprint-plan":
	return runSprintPlan(ctx, input)
case "sprint-execute":
	return runSprintExecute(ctx, input)
default:
	return fmt.Errorf("unknown planning route %q", plan)
}
```

The names above are schematic call sites, not a recommendation to add
exported wrappers. In the implementation, keep the plan and execute bodies
inline or as unexported functions in the command/internal package, following
the repository's no-wrapper rule. The existing `internal/workflows/sprint`
package is a useful implementation reference, but its current `Sprint` is a
full sprint/issue builder with fixed researcher/planner/validator behavior; it
is not itself a drop-in intake router.

## Boundaries and proof

- Prove `lfg` with a fake adapter: the worker receives the normalized request,
  the supervisor is attached to that turn, and the run records the selected
  route and result.
- Prove guidance with a fake structured response for each literal route,
  including rejection of an unknown route. A passing schema decode alone does
  not prove the branch ran.
- Keep sprint planning and execution proof separate. A generated checklist is
  not evidence that implementation ran; a green repository check is not
  evidence that the requested behavior was seen working.
- Do not make web serving, GitHub issue fetching, multi-agent fan-out, or a
  persistent conversation prerequisite for the first intake path. Existing
  `run-prompt` already shows how to keep logs durable without opening the web
  UI.

## Recommendation

Start with `gimble lfg` and a typed `Input` carrying raw request metadata plus
one typed `Guidance` output. Route by a direct switch. Make the one-turn
supervised branch the only fully implemented first slice; add the two sprint
branches as explicit commands that consume/produce local artifacts when their
contracts are ready. This preserves the user's simple entry experience while
staying inside the current Gimble API and no-wrapper rule.
