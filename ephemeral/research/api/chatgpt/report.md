> ChatGPT deep research, received 2026-09-10. The inline `cite…turn…` markers point to ChatGPT session-internal search results and cannot be mapped to individual sources; the Sources section at the end is the source list ChatGPT gave afterward.

# Go Agent Orchestration: API Research and Recommendation

## Bottom line

I would build the small Go façade rather than adopt an existing agent framework wholesale, and I would make **ACP one backend adapter rather than the abstraction itself**.

The core model I recommend is:

```go
package gimbal

import (
	"context"
	"encoding/json"
	"iter"
)

type Session interface {
	ID() string
	Prompt(
		context.Context,
		string,
		...PromptOption,
	) (Turn, error)
}

type Turn interface {
	ID() string

	// Events observes progress while the turn is running.
	Events() iter.Seq2[Event, error]

	// Wait waits for terminal completion.
	// Repeated calls return the same terminal result.
	Wait(context.Context) (Result, error)

	// Interrupt stops this turn while preserving the session.
	Interrupt(context.Context) error
}

// Optional capability.
type Steerable interface {
	Steer(context.Context, string) error
}

// Optional capability.
type Compactable interface {
	Compact(context.Context) error
}

type Result struct {
	Text       string
	Structured json.RawMessage
	StopReason StopReason
	Usage      Usage
}
```

That conclusion is unusually well supported by the current coding-agent APIs. Codex App Server explicitly models **thread → turn**, starts a turn with `turn/start`, returns an initial in-progress turn, streams notifications, attaches `turn/steer` and `turn/interrupt` to the active turn, and accepts an `outputSchema` on the turn. citeturn30view3turn30view4turn30view5turn30view6 ACP independently arrived at **session → prompt turn**, with streamed `session/update` notifications, cancellation of the active turn, a terminal stop reason, and another prompt after the turn finishes. citeturn20view1turn21view0

The important design calls are:

| Question | Recommendation |
|---|---|
| `Future`? | **No generic `Future` in the agent API. `Turn` is the future, plus lifecycle controls.** |
| Channel? | **No channel as the principal public result API. Use `iter.Seq2` for event streams and `Wait` for terminal state.** |
| `Session.Wait()`? | **No. Completion belongs to a turn, not a long-lived session.** |
| `Task` object? | **Use `Turn`, not `Task`.** `Task` increasingly means a durable, remotely addressable workflow entity in A2A. citeturn22search1turn22search2 |
| `Steer` location? | **On the active `Turn`.** This prevents ambiguity/races and exactly matches Codex's semantics. citeturn30view5 |
| `Interrupt` location? | **On `Turn`.** ACP/Claude may implement it internally with a session-scoped cancellation mechanism. citeturn21view0turn30view7 |
| `FollowUp`? | **Do not have it.** A follow-up is simply the next `Session.Prompt`. ACP, Codex, and Claude all model continuation that way. citeturn21view0turn30view2turn30view8 |
| `Compact`? | **Optional session capability**, not a core mandatory method. Different systems expose or automate context management differently. Google ADK, for example, performs compaction at the runner/session-management layer rather than making it an intrinsic agent operation. citeturn6view1 |
| JSON method? | **No `PromptJSON`.** Structured output is a prompt/turn option plus a typed package-level convenience helper. Genkit and Codex provide strong precedent. citeturn17view2turn30view3 |
| Higher orchestration? | **Plain Go first.** Loops/functions for coder→reviewer→coder; graph/workflow abstractions only when they provide concrete value; Temporal only when execution must actually be durable. citeturn12view0turn18view5turn23view1 |

The distinction I would preserve aggressively is:

> **Session = conversation/history. Turn = one active unit of agent work. Workflow = orchestration among turns/sessions.**

Google ADK Go makes essentially the same architectural separation: its `Session` is passive conversational state—ID, app/user IDs, state, events, timestamps—while its `Runner.Run` actually drives an invocation and yields events. citeturn6view0turn6view4

## The Go landscape

There are several mature or fast-moving Go projects nearby, but they solve different layers of the problem.

| Project | Public primitive shape | Async / streaming shape | Higher-level orchestration | Relevance to Gimbal |
|---|---|---|---|---|
| **Google ADK Go** | `Agent`, `Runner`, `Session`, `Event`; `Agent.Run(...) iter.Seq2[*session.Event,error]` | Standard Go iterator | Sequential, parallel and loop agents; subagents | Strong evidence for `iter.Seq2`, session/execution separation, and plain compositional agents. citeturn5view0turn6view0turn10view0 |
| **CloudWeGo Eino** | Components, ADK agents, Runner, Graph | `runner.Query(...)`, iterator/event consumption | Compiled graphs/workflows, DeepAgent/subagents, interrupt/resume | Very capable Go-native *agent framework*, but it wants to own more of the inner agent architecture than Gimbal does. citeturn3view0 |
| **Firebase Genkit Go** | `Generate`, model responses, typed flows, chat | Synchronous return plus iterator-based streaming | Generic typed `Flow[In,Out,...]` | Excellent structured-output and typed-workflow precedent. citeturn17view2turn18view5turn18view6 |
| **LangChainGo** | LLMs, agents, chains, tools, output parsers | Conventional synchronous generation APIs are prominent | Chains/agents | Useful prior art, but less specifically shaped around interactive coding-agent harness lifecycle. citeturn13view0 |
| **A2A Go SDK** | `Task`, `Message`, `Artifact`, context IDs, task states | Blocking, immediate/fire-and-forget, streaming/subscription | Remote task continuation and lifecycle | Useful adjacent design, especially for durable/remotely addressable work; too heavy to make its `Task` semantics your local turn primitive. citeturn22search1turn22search2 |
| **Temporal Go** | Workflow, Activity, Child Workflow, Future, Channel, Selector | Durable `Future.Get`, deterministic channels/selectors | Durable workflows | Relevant only when orchestration itself must survive crashes/restarts. Its Future is not evidence that ordinary Go agent calls need one. citeturn23view1turn24view0 |
| **ACP** | Session, prompt turn, updates, stop reason, capabilities | Long-lived JSON-RPC request plus update notifications | Deliberately not a workflow engine | Very good backend protocol and vocabulary source; not broad enough to be the Gimbal abstraction. citeturn20view0turn20view1 |

### Google ADK and Eino are the closest Go-native orchestration prior art

Google ADK's core `Agent` interface is strikingly small:

```go
type Agent interface {
	Name() string
	Description() string
	Run(InvocationContext) iter.Seq2[*session.Event, error]
	SubAgents() []Agent
	FindAgent(name string) Agent
	FindSubAgent(name string) Agent
	// ...
}
```

Its `Runner.Run` similarly returns `iter.Seq2[*session.Event,error]`. The public API does **not** return a channel or a Future for an ordinary run. citeturn5view0turn6view0

ADK also makes orchestration operators explicit but simple: `SequentialAgent`, `ParallelAgent`, and `LoopAgent`. `LoopAgent` repeatedly invokes its children in sequence until a configured iteration limit or termination signal; the package documentation explicitly describes iterative refinement as a use case. citeturn10view0turn12view0

CloudWeGo's Eino has converged on similar concepts. It has Go-native components, an ADK layer for tool use, multi-agent coordination and interrupt/resume, and graph composition in which callers build nodes/edges, compile a graph, then invoke it. Its runner example consumes emitted events from an iterator rather than treating a terminal string as the only observable product. citeturn3view0

These are valuable references, but I would **not use either as Gimbal's core abstraction**. Both frameworks are designed to construct agents and their model/tool loops. Your unit of integration is different: Claude Code, Codex, Antigravity or an ACP agent arrives as an already-formed autonomous harness. Gimbal should orchestrate those harnesses without pretending it owns their internal ReAct/tool/subagent loop. That is a narrower layer.

### Genkit's structured-output design is worth copying

Genkit distinguishes the general model response from typed structured-output conveniences. `genkit.Generate` accepts options and returns a `ModelResponse`; callers can provide `ai.WithOutputType(SomeGoType{})`, after which Genkit derives a JSON Schema and validates/parses the output. It also provides the generic convenience `genkit.GenerateData[T]`, which sets the output type and returns the typed value. citeturn17view2

Its streaming structured-output API similarly uses rangeable iteration:

```go
for val, err := range genkit.GenerateDataStream[[]MenuItem](ctx, g, ...) {
	// ...
}
```

and its workflow API has concrete typed `Flow[In, Out, Stream]` values with synchronous `Run` and iterator-based `Stream`. citeturn17view2turn18view5turn18view6

That suggests a useful pattern for Gimbal: **keep the underlying turn/result non-generic and stable; layer typed helpers over it**.

### A2A shows what a real Task abstraction implies

A2A's Go SDK is useful precisely because it illustrates why I would *not* call Gimbal's per-prompt object `Task`. A2A Tasks have IDs, persistent status, continuation, artifact production, cancellation and subscription. The current CLI can submit work with `--immediate`, get the task ID immediately, query it later, continue the same task, subscribe until terminal state, and enumerate tasks by status. citeturn22search1

The Go implementation has itself moved toward standard iterators: its v1-era API change made an `AgentExecutor`'s `Execute` and `Cancel` return `iter.Seq2[a2a.Event,error]` instead of writing to a queue; the 2026 releases additionally introduced generic pull event queues and substantial task-store machinery. citeturn22search2

So I would reserve **Task** for something whose identity survives the call and potentially the process. A coding-agent prompt inside an existing conversation is better called a **Turn**.

## Coding-agent lifecycle across Codex, Claude, ACP, and Antigravity

The strongest signal is that the actual coding-agent surfaces are already converging around sessions/threads and turns.

### Codex is almost the proposed API already

The current Codex App Server has:

```text
thread/start
thread/resume
thread/fork

turn/start
turn/steer
turn/interrupt

...streamed item / message / tool notifications...
turn/completed
```

`turn/start` returns the initial turn object immediately with `status: "inProgress"` and then emits notifications as work progresses. It accepts turn-specific settings including working directory, sandbox policy, model, effort and `outputSchema`. citeturn30view3

`turn/steer` explicitly appends input to the **currently active turn**. It requires an expected turn ID and fails if no turn is active, which is particularly strong evidence that steering belongs on your `Turn`, not on `Session`: a turn-scoped method naturally carries exactly the identity needed to make stale steering impossible. citeturn30view5

Likewise, `turn/interrupt` takes both the thread and turn IDs and terminally changes the turn to `interrupted`. citeturn30view4

The higher-level Codex SDK deliberately hides most of that lifecycle. It exposes a `Thread`; callers `await thread.run(...)`, call `run()` again for the next conversational turn, or resume an old thread ID. citeturn30view1turn30view2 For Gimbal, the lower-level App Server is the more relevant precedent because you specifically want to preserve steering, events, interruption and other harness features.

Codex App Server also illustrates why an escape hatch matters. In addition to basic threads/turns it exposes review mode, standalone sandboxed command execution, permission profiles, experimental dynamic tools, thread forks and persisted thread goals. Current thread goal APIs even carry an objective, status and token budget. citeturn30view4turn30view6 Trying to make every one of those concepts part of a common cross-agent interface would recreate the lowest-common-denominator problem you are trying to avoid.

### Claude suggests the same session semantics, with a less reified turn

Claude's Python Agent SDK exposes a long-lived `ClaudeSDKClient`. A caller sends `query(...)`, consumes output through `receive_response()`, then calls `query(...)` again for the follow-up; the SDK explicitly describes the latter as continuing the same session context. citeturn30view8

It also has `interrupt()`. Importantly, interruption does not magically discard the active invocation's output: already-produced messages and the terminal `ResultMessage` remain in the stream and must be drained before consuming the result of a subsequent query. citeturn30view7turn30view8 That is another argument for Gimbal owning a **Turn object that continuously drains and associates events with the correct invocation**, rather than exposing a single session-wide raw message channel.

Claude additionally supports session IDs, resumption, forking a resumed session, resuming from an earlier message and programmatically defined subagents. citeturn30view10 These should mostly stay adapter-specific or become optional session creation/resumption capabilities rather than inflating the primitive `Session` interface.

### ACP captures the common denominator, but not the whole one

ACP v1 is remarkably close to the common core you need. Its flow is:

```text
initialize
session/new or session/load

session/prompt
    session/update
    session/update
    ...
    optional session/cancel
session/prompt -> StopReason

session/prompt again for the next turn
```

ACP explicitly defines a “prompt turn” as a complete interaction cycle that may itself contain multiple model requests and tool calls. Progress comes through `session/update`; the original `session/prompt` returns only when the turn terminates, with a `StopReason`. citeturn20view1turn21view0

That maps naturally onto:

```go
turn, err := session.Prompt(ctx, prompt)
result, err := turn.Wait(ctx)
```

An ACP adapter can implement that asynchronous Go surface by running the outstanding JSON-RPC `session/prompt` internally, feeding `session/update` into the turn's event collector, and settling the Turn when the response arrives.

ACP cancellation is named `session/cancel` and identifies only the session, but semantically it cancels the current prompt turn and causes that outstanding prompt to terminate with the `cancelled` stop reason. citeturn21view0 A Gimbal ACP adapter can therefore implement:

```go
turn.Interrupt(ctx)
```

using:

```text
session/cancel
```

without exposing the protocol's transport-level placement in the Go abstraction.

There is, however, **no standardized in-flight `steer` gesture in ACP v1**. The specified flow allows cancellation while a turn is active and another `session/prompt` after it completes; the method inventory contains prompt, cancel, session loading, modes and related operations, but no equivalent to Codex `turn/steer`. citeturn20view1turn21view0 Making ACP the entire Gimbal contract would therefore lose a meaningful Codex capability.

ACP itself anticipates this problem. Every protocol type can carry `_meta`; implementations may add underscore-prefixed extension methods and advertise custom capabilities during initialization. citeturn21view2 In other words, even ACP's designers do not claim that its standardized core should contain every agent-specific operation.

There is also currently no official Go ACP library listed by the ACP project: the official implementations listed are Kotlin, Java, Python, Rust and TypeScript. The protocol repository does publish schemas suitable for generation, so implementing a compact Go client is feasible, but there is no official Go SDK to simply adopt today. citeturn20view2

My conclusion is therefore:

> **Support ACP enthusiastically, but behind `gimbal.Session`. Do not define `gimbal.Session` to be “ACP in Go.”**

### Antigravity reinforces the orchestration/runtime distinction

Google Antigravity 2.0 now explicitly positions itself as a central command center for launching, monitoring and orchestrating multiple coding agents; Google's current material describes standalone Antigravity, an IDE, CLI, SDK, scheduled tasks and multi-agent execution. citeturn27search0turn27search1

The current official SDK material describes `google-antigravity` as a **Python** library exposing the same agent runtime used by the CLI, with policies and lifecycle hooks configured in code. Google also exposes the Antigravity base agent through its managed Agent Platform/Interactions APIs. citeturn27search4turn27search3

That makes Antigravity another good adapter target, but not evidence for importing its higher-level notions into Gimbal. Antigravity's “project,” scheduled-task and multi-agent-manager concepts are a layer above the primitive “run one coding turn in one conversation” API.

## Futures, channels, iterators, and turn handles

This is the part where I think the API decision matters most.

### A Future is too weak

A Future describes:

```text
computation now → value later
```

Your object needs to describe:

```text
computation now
    + streamed semantic events
    + mid-flight steering
    + interruption
    + identity
    + terminal stop reason
    → result later
```

Calling that object a `Future[Result]` leaves most of its reason for existing unexplained. Calling it a `Turn` captures the domain semantics.

Temporal's Future is real and useful, but it exists inside a very different execution model. `workflow.ExecuteActivity` returns immediately with a `Future`; later `Future.Get` blocks for the result, and callers can launch several activities and collect their futures. citeturn23view1turn24view0

Critically, Temporal also replaces normal Go concurrency machinery with deterministic equivalents: workflow goroutines, workflow channels and workflow selectors must be used instead of ordinary goroutines/channels/select because workflow code must replay deterministically. citeturn23view1 So Temporal's Future is primarily evidence for **durable computation handles**, not evidence that a normal Go library should reinvent promises.

A2A presents the other case where a handle matters: a Task may continue remotely after the submitting request, can be addressed by ID, retrieved, subscribed to and canceled later. citeturn22search1 If Gimbal eventually adds a *durable orchestration* package, a `Task` or `Future` concept may make sense there. It does not need to pollute the low-level harness API.

### A result channel is too low-level

Channels work beautifully when the protocol itself is a stream or pipeline. The Go project's canonical pipeline guidance uses receive-only channels for stages, but it also spends considerable attention on what happens when the consumer stops receiving: producers can block indefinitely unless cancellation and draining are carefully designed. citeturn8view0

That is exactly the wrong ownership relationship for a coding agent. The agent **must keep being drained even if the caller stops printing its events**, because:

- the eventual result still has to be collected;
- tool/permission/session state may be encoded in intermediate messages;
- Claude explicitly leaves messages from an interrupted turn buffered until the client consumes them; citeturn30view7turn30view8
- ACP delivers progress separately from the terminal prompt response; citeturn21view0
- Codex's App Server continuously emits notifications after `turn/start`. citeturn30view3

So I would not make this the primary API:

```go
func (s Session) Prompt(...) <-chan Event
```

Nor this:

```go
func (s Session) Prompt(...) <-chan Result
```

The former conflates observation with runtime draining. The latter is just a single-use Future disguised as a channel.

### Standard iterators have become the better event-stream surface

The standard `iter` package defines:

```go
type Seq[V any]     func(yield func(V) bool)
type Seq2[K, V any] func(yield func(K, V) bool)
```

and the producer stops when the sequence ends or the consumer's `yield` returns false. citeturn18view0turn18view1

This has already been adopted by the strongest current Go agent projects: Google ADK returns `iter.Seq2` from agent and runner execution; A2A Go changed its executor API to return `iter.Seq2[a2a.Event,error]`; Genkit uses rangeable streaming sequences for generated data and flows. citeturn5view0turn6view0turn22search2turn18view6

So:

```go
for event, err := range turn.Events() {
	if err != nil {
		// observer/stream error
		break
	}
	handle(event)
}
```

is a better modern-Go surface than a library-defined Stream/Future/Promise abstraction.

Internally, adapters may absolutely use goroutines and channels. Codex and ACP JSON-RPC readers are inherently asynchronous. That is an implementation concern, not a reason to make channels the application-level semantic type.

### Keep `Wait` on the Turn

I would make `Wait` explicit:

```go
result, err := turn.Wait(ctx)
```

rather than requiring callers to discover a special final event.

This gives orchestration code a simple, obvious terminal operation while leaving the event stream optional. The important contract should be:

```text
Prompt submits the turn.
The Turn internally owns transport draining.
Events observes what happens.
Wait observes terminal state.
Interrupt changes the running turn.
```

`Wait` should be idempotent: after completion, every call returns the cached terminal result. Calling `Events` should never be required for the turn to make progress.

Putting `Wait()` on `Session` is substantially worse. A session spans multiple turns; after two prompts, “wait for the session” has no obvious meaning. ACP and Codex both explicitly separate the conversation object from individual prompt/turn lifecycles. citeturn21view0turn30view3

### Use contexts for waits and RPCs, not as the agent's only interrupt mechanism

Go `Context` is designed to carry cancellation/deadlines across API boundaries, and derived cancellations propagate downward. citeturn18view2 It should therefore be present on all blocking Gimbal operations.

But I would preserve `Interrupt` as a first-class semantic operation rather than saying “cancel the Context.” Coding harnesses give interruption additional meaning: Codex marks the turn `interrupted`, ACP returns the semantically meaningful `cancelled` stop reason, and Claude leaves the session usable for another query after interrupting and draining the prior result. citeturn30view4turn21view0turn30view8

A useful contract is:

```go
turn, err := session.Prompt(ctx, text)
```

Here `ctx` bounds submission/startup.

```go
result, err := turn.Wait(ctx)
```

Here `ctx` bounds the caller's willingness to wait. A wait timeout need not destroy the agent turn.

```go
err := turn.Interrupt(ctx)
```

This is the explicit instruction to terminate the agent's current work while retaining its conversation.

That distinction will save you from a lot of accidental “timeout == destroy conversation” coupling later.

## Structured output and extension surface

### Do not make JSON a parallel prompt API

I would avoid:

```go
Prompt(ctx, string) (string, error)
PromptJSON(ctx, string) (map[string]any, error)
```

The difference between text and structured generation is primarily **the requested output contract**, not a fundamentally different interaction.

Codex makes `outputSchema` an option on `turn/start` rather than defining a different turn operation. citeturn30view3 Genkit similarly keeps one `Generate` primitive and adds an output-type option plus the typed `GenerateData[T]` convenience. citeturn17view2

I would make the low-level result able to represent either:

```go
type Result struct {
	Text       string
	Structured json.RawMessage
	StopReason StopReason
	Usage      Usage
}
```

and make output shape an option:

```go
turn, err := session.Prompt(
	ctx,
	"Review this change.",
	gimbal.WithOutputSchema(schema),
)
```

Then provide a package-level typed convenience:

```go
type Review struct {
	Approved bool     `json:"approved"`
	Issues   []string `json:"issues"`
}

review, result, err := gimbal.RunAs[Review](
	ctx,
	reviewer,
	"Review the current working tree.",
)
```

Conceptually, `RunAs[T]` should be just sugar for:

```text
derive JSON schema for T
→ Prompt(... WithOutputSchema(schema))
→ Wait(...)
→ validate/decode Result.Structured into T
```

That buys you typed orchestration without infecting the fundamental `Session` or `Turn` types with generics.

For callers needing steering while waiting on structured output, the non-convenience path remains available:

```go
turn, err := reviewer.Prompt(
	ctx,
	"Review this change.",
	gimbal.Output[Review](),
)
if err != nil {
	return err
}

if steerable, ok := turn.(gimbal.Steerable); ok {
	err = steerable.Steer(ctx, "Pay special attention to transaction isolation.")
	if err != nil {
		return err
	}
}

result, err := turn.Wait(ctx)
if err != nil {
	return err
}

review, err := gimbal.Decode[Review](result)
```

### `Steer`, `Compact`, resume and fork should be capabilities

The shared coding-agent substrate is smaller than the union of vendor features.

Codex currently has explicit active-turn steering, interruption, thread resume/fork, review mode, output schemas and goal management. citeturn30view3turn30view4turn30view5turn30view6 Claude supports interruption, session resumption/forking and subagents. citeturn30view7turn30view10 ACP standardizes cancellation, resume-if-supported, modes, permission requests, plans, usage updates and extension methods, but not Codex-style steering. citeturn20view1turn21view2

Do not create a giant interface whose implementations spend their lives returning `ErrUnsupported`:

```go
type AgentSession interface {
	Prompt(...)
	Steer(...)
	Interrupt(...)
	FollowUp(...)
	Compact(...)
	Fork(...)
	Resume(...)
	SetMode(...)
	SetGoal(...)
	Review(...)
	...
}
```

Small capability interfaces let the Go type system express the distinction:

```go
type Steerable interface {
	Steer(context.Context, string) error
}

type Compactable interface {
	Compact(context.Context) error
}

type Forkable interface {
	Fork(context.Context, ...SessionOption) (Session, error)
}
```

The call site remains explicit:

```go
steerer, ok := turn.(gimbal.Steerable)
if !ok {
	// choose a fallback strategy
}
```

For capabilities that are negotiated dynamically—ACP is explicitly capability-negotiated—the adapter can return a concrete implementation appropriate to the negotiated feature set. ACP's own extensibility system is capability-based, so the model fits naturally. citeturn20view2turn21view2

### Preserve native features through typed adapter packages

I would also avoid a universal:

```go
Native() any
```

as the primary extension story. It turns the abstraction boundary into a type-assertion free-for-all.

Instead, let adapter packages provide typed downcasts or richer views:

```go
import "example.com/gimbal/codex"

cs, ok := codex.AsSession(session)
if ok {
	goal, err := cs.Goal(ctx)
	// Codex-specific feature.
}
```

Similarly:

```go
claude.AsSession(session)
acp.AsSession(session)
antigravity.AsSession(session)
```

This leaves the common package small while deliberately giving sophisticated callers full access to the harness they chose.

ACP can still be used underneath. Its `_meta`, custom methods and custom capabilities explicitly provide room for nonstandard extensions. citeturn21view2

### Normalize events, not every command

The useful common event vocabulary is larger than the useful common command vocabulary.

ACP alone emits message chunks, thoughts, plans, tool-call lifecycle changes, mode changes, command changes and usage/cost updates. citeturn20view1turn21view0 Codex emits started/completed items, agent-message deltas and tool progress. citeturn30view3 ADK's runner similarly treats events as the main unit yielded by execution. citeturn6view0turn6view3

I would therefore expect an eventual Gimbal event family roughly like:

```go
type Event interface {
	Kind() EventKind
}

type MessageEvent struct {
	Role Role
	Text string
}

type ToolEvent struct {
	ID     string
	Name   string
	Status ToolStatus
}

type PlanEvent struct {
	Entries []PlanEntry
}

type UsageEvent struct {
	Usage Usage
}
```

with an adapter-specific event escape hatch for information that genuinely cannot be normalized.

The common **commands**, in contrast, should remain very few:

```text
Prompt
Wait
Interrupt

optional: Steer
optional: Compact
optional: Fork/Resume
```

That asymmetry is healthy. Agent harnesses produce lots of interesting telemetry while sharing only a handful of universally sensible control gestures.

## Higher-level workflows and durability

### The coder → validator → coder loop should initially be ordinary Go

Google ADK publishes an explicit LoopAgent, but its implementation is conceptually simple: repeatedly run child agents sequentially until a maximum iteration count or an exit signal. citeturn12view0 Genkit takes another approach: higher-level workflows are ordinary generic Go functions wrapped as typed flows. citeturn18view5

For Gimbal, I would begin even lower-level:

```go
type Verdict struct {
	Pass     bool   `json:"pass"`
	Feedback string `json:"feedback"`
}

func implement(
	ctx context.Context,
	coder gimbal.Session,
	reviewer gimbal.Session,
	requirement string,
) error {
	prompt := requirement

	for attempt := 0; attempt < 5; attempt++ {
		if _, err := gimbal.Run(ctx, coder, prompt); err != nil {
			return err
		}

		verdict, _, err := gimbal.RunAs[Verdict](
			ctx,
			reviewer,
			"Validate the current working tree. "+
				"Do not modify it. Report whether the implementation is correct.",
		)
		if err != nil {
			return err
		}

		if verdict.Pass {
			return nil
		}

		prompt = "The validator rejected the implementation:\n\n" +
			verdict.Feedback +
			"\n\nFix the problems, then run the relevant tests."
	}

	return ErrValidationLimit
}
```

That is already a perfectly good orchestration DSL: **Go itself**.

You get local variables, `if`, `for`, `defer`, functions, typed data, goroutines when useful, normal error handling and normal testing. A graph abstraction should have to earn its existence.

For parallel candidates:

```go
g, ctx := errgroup.WithContext(ctx)

for _, candidate := range candidates {
	candidate := candidate

	g.Go(func() error {
		_, err := gimbal.Run(ctx, candidate, prompt)
		return err
	})
}

if err := g.Wait(); err != nil {
	return err
}
```

No agent-specific `ParallelGroup` primitive is required until you need graph introspection, serialization, visualization or durability.

### A workflow layer can be added without changing the harness API

If you later want named reusable operators, Eino and ADK give good precedent for keeping them one layer above execution. Eino graphs support nodes/edges, compilation and invocation, while ADK exposes sequential/parallel/loop composition. citeturn3view0turn10view0

A future `gimbal/workflow` package could therefore contain constructs such as:

```go
type Step[I, O any] func(context.Context, I) (O, error)
```

and helpers for:

```text
Sequence
Parallel
Race
Retry
Loop
Map
Validate
```

without changing `Session` or `Turn` at all.

That separation matters. A coding harness API describes **how to drive one agent**. An orchestration API describes **how your program coordinates multiple pieces of work**. Conflating them is how agent frameworks end up owning everything.

### Sprints and milestones should not be primitives

I would not model `Sprint` or `Milestone` in Gimbal.

There is evidence that agent products find higher-level planning useful: ACP has structured plan updates, Codex App Server now exposes persisted thread goals, and Antigravity has explicit projects, scheduled tasks and multi-agent management. citeturn21view0turn30view6turn27search0

But those concepts do not have stable common semantics. A plan may be merely agent telemetry; a Codex goal is persisted thread state; an Antigravity project groups workspace folders/settings. They are not the same abstraction. citeturn21view0turn30view6turn27search0

At Gimbal's layer:

```text
milestone = application data
sprint     = application data
issue      = application data
workflow   = Go control flow
session    = agent conversation
turn       = one agent invocation
```

is a much cleaner boundary.

### Bring in Temporal only when “durable” means durable

If a coder→validator loop should survive a machine/process restart, retry remote steps after outages, remain inspectable for days and resume at the exact logical point, then an in-memory orchestration library should not grow a home-made durability subsystem.

Temporal already implements this execution model: workflow state is reconstructed across worker failures, coordination code must be deterministic, activities return Futures, and child workflows can themselves be asynchronously monitored. citeturn23view0turn23view1

At that point the architecture should be:

```text
Temporal Workflow
    |
    +-- Activity: run Gimbal coding turn
    |
    +-- Activity: run tests
    |
    +-- Activity: run Gimbal validation turn
    |
    +-- branch / loop / retry
```

rather than:

```text
Gimbal grows Futures
      grows persistent task stores
      grows workflow recovery
      grows deterministic replay
      grows schedulers
      ...
```

The `Future` belongs to the durable workflow engine because the workflow engine can actually keep the promise.

## Proposed Gimbal API

Putting the research together, this is approximately where I would start.

```go
package gimbal

import (
	"context"
	"encoding/json"
	"iter"
)

type Session interface {
	ID() string

	// Prompt starts one conversational turn.
	//
	// The returned Turn is the identity and lifecycle handle for that
	// invocation. A Session should normally have at most one active Turn.
	Prompt(
		ctx context.Context,
		prompt string,
		opts ...PromptOption,
	) (Turn, error)
}

type Turn interface {
	ID() string

	// Events yields observable progress for this turn.
	//
	// Consuming Events is not required for the turn to make progress.
	Events() iter.Seq2[Event, error]

	// Wait waits for terminal completion.
	//
	// ctx controls this wait, not the lifetime of the Session.
	// Wait is idempotent after completion.
	Wait(ctx context.Context) (Result, error)

	// Interrupt requests termination of this turn while preserving
	// the containing Session.
	Interrupt(ctx context.Context) error
}
```

This maps cleanly onto Codex's thread/turn distinction and its turn-scoped interrupt; ACP adapters can translate Turn interruption to `session/cancel`; Claude adapters can translate it to `ClaudeSDKClient.interrupt()`. citeturn30view4turn21view0turn30view7

For steering:

```go
// Steerable is implemented by Turns that accept additional user
// direction while still running.
type Steerable interface {
	Steer(ctx context.Context, prompt string) error
}
```

That intentionally means:

```go
turn, err := session.Prompt(ctx, "Fix the failing tests.")
if err != nil {
	return err
}

if steerable, ok := turn.(gimbal.Steerable); ok {
	err := steerable.Steer(
		ctx,
		"Ignore the integration suite for now; isolate the unit failure first.",
	)
	if err != nil {
		return err
	}
}

result, err := turn.Wait(ctx)
```

Codex's current `turn/steer` semantics are essentially exactly this operation, including binding steering to the currently in-flight turn. citeturn30view5

For session-wide context operations:

```go
type Compactable interface {
	Compact(ctx context.Context) error
}

type Forkable interface {
	Fork(ctx context.Context, opts ...SessionOption) (Session, error)
}
```

Do **not** add:

```go
FollowUp(...)
```

because:

```go
_, err := session.Prompt(ctx, "Now implement the second part.")
```

already expresses a follow-up unambiguously. ACP says another `session/prompt` continues the conversation once the prior turn completes; Codex calls `run()` again on the thread; Claude calls `query()` again on the same client session. citeturn21view0turn30view2turn30view8

For results:

```go
type StopReason string

const (
	StopCompleted   StopReason = "completed"
	StopInterrupted StopReason = "interrupted"
	StopRefused     StopReason = "refused"
	StopLimit       StopReason = "limit"
)

type Result struct {
	Text       string
	Structured json.RawMessage
	StopReason StopReason
	Usage      Usage
}

type Usage struct {
	InputTokens  int64
	OutputTokens int64
	Cost         *Cost
}
```

This follows the important ACP idea that semantic terminal states such as cancellation belong in the terminal result rather than being transport failures. ACP specifically requires a cancelled prompt to finish with a meaningful `cancelled` stop reason instead of surfacing the underlying abort as an arbitrary JSON-RPC error. citeturn21view0 Gimbal should use `error` for inability to execute/communicate/interpret the operation and `StopReason` for a successfully observed agent termination.

For structured output:

```go
type PromptOption interface {
	applyPrompt(*promptConfig)
}

func WithOutputSchema(schema json.RawMessage) PromptOption
func Output[T any]() PromptOption

func Decode[T any](result Result) (T, error)

func RunAs[T any](
	ctx context.Context,
	session Session,
	prompt string,
	opts ...PromptOption,
) (T, Result, error)
```

That mirrors the good parts of Genkit's `WithOutputType`/`GenerateData[T]` approach while preserving the underlying Turn abstraction, and it directly maps onto Codex's per-turn `outputSchema`. citeturn17view2turn30view3

For the boring 80% case, add helpers rather than another interface shape:

```go
func Run(
	ctx context.Context,
	session Session,
	prompt string,
	opts ...PromptOption,
) (Result, error) {
	turn, err := session.Prompt(ctx, prompt, opts...)
	if err != nil {
		return Result{}, err
	}
	return turn.Wait(ctx)
}
```

Then straightforward programs look straightforward:

```go
result, err := gimbal.Run(ctx, session, "Fix the failing test.")
if err != nil {
	return err
}

fmt.Println(result.Text)
```

while interactive orchestration gets the full lifecycle:

```go
turn, err := session.Prompt(ctx, "Refactor the storage layer.")
if err != nil {
	return err
}

go func() {
	for event, err := range turn.Events() {
		if err != nil {
			return
		}
		logEvent(event)
	}
}()

if steerable, ok := turn.(gimbal.Steerable); ok {
	_ = steerable.Steer(ctx, "Preserve backward compatibility.")
}

result, err := turn.Wait(ctx)
```

The resulting conceptual API is only a little larger than the original sketch:

```text
Session
    Prompt -> Turn

Turn
    Events
    Wait
    Interrupt

optional Turn
    Steer

optional Session
    Compact
    Fork
    Resume

helpers
    Run
    RunAs[T]
    Decode[T]
```

but it resolves the ownership problems in the two candidate interfaces.

In particular, this original form:

```go
type AgentSession interface {
	RunPrompt(context.Context, string)
	Interrupt(context.Context) error
	Steer(context.Context, string) error
	FollowUp(context.Context, string) error
	Compact(context.Context) error
}
```

puts operations with three different lifetimes on one object: prompt/steer/interrupt concern the current invocation, follow-up concerns the next invocation, and compaction concerns conversation state. The current Codex and ACP protocols both provide concrete evidence that separating conversation from turn is the cleaner model. citeturn30view3turn21view0

The second sketch is substantially closer:

```go
type Task interface {
	Steer(context.Context, string) error
	FollowUp(context.Context, string) error
}

type AgentSession interface {
	RunPrompt(context.Context, string) (Task, error)
	Interrupt(context.Context) error
	Compact(context.Context) error
}
```

but I would make four changes:

```diff
-type Task
+type Turn

 type Turn interface {
     Steer(...)
-    FollowUp(...)
+    Wait(...)
+    Interrupt(...)
+    Events(...)
 }

 type Session interface {
-    RunPrompt(...)
+    Prompt(...)
-    Interrupt(...)
     Compact(...) // optional capability, not base interface
 }
```

The key insight from the current ecosystem is that **the missing abstraction is not a Future. It is the turn**.

A Turn gives you a Future-like terminal value, but it also gives you the exact semantic identity needed for steering, interruption, event attribution and stop state. Codex has already made that object first-class; ACP's specification calls the same lifecycle a prompt turn; Google ADK and A2A Go have converged on iterators for event-producing execution; and Genkit demonstrates how typed structured results can remain a convenience layered over the fundamental operation. citeturn30view3turn21view0turn5view0turn22search2turn17view2

That is a small enough abstraction to remain Go-like, while still being strong enough to wrap the coding agents without sanding away the capabilities that make them different.

## Sources

The highest-weight sources for the Gimbal API recommendation were Codex App Server, ACP Prompt Turn, Google ADK Go, Genkit Go, and Temporal/A2A. Comparing those five led to `Session → Turn`, with `Turn` as the domain-specific future/lifecycle handle instead of a generic `Future[T]`.

### Coding-agent lifecycle / API shape

- [Codex App Server API](https://developers.openai.com/codex/app-server/): `thread/*`, `turn/start`, `turn/steer`, `turn/interrupt`, events, `outputSchema`
- [Codex SDK](https://developers.openai.com/codex/sdk/)
- [Claude Agent SDK reference](https://docs.claude.com/en/docs/agent-sdk/python): sessions, repeated queries, interrupt, resume, fork

### ACP

These led to the conclusion that ACP makes a good backend but should not define Gimbal's whole abstraction.

- [Agent Client Protocol overview](https://agentclientprotocol.com/protocol/overview)
- [ACP Prompt Turn specification](https://agentclientprotocol.com/protocol/v1/prompt-turn)
- [ACP extensibility specification](https://agentclientprotocol.com/protocol/v1/extensibility)
- [ACP protocol repository](https://github.com/agentclientprotocol/agent-client-protocol)

### Go-native agent frameworks

- [Google ADK Go](https://github.com/google/adk-go): `Agent`, `Runner`, `Session`, workflow agents. Influential for its use of standard Go iterators and for separating session state from execution. ADK Go v2 also includes graph, parallel, and loop workflow primitives.
- [CloudWeGo Eino](https://github.com/cloudwego/eino)
- [LangChainGo](https://github.com/tmc/langchaingo)

### Structured output / typed workflows

Source of the recommendation for a generic convenience such as `RunAs[T]` layered over a non-generic core.

- [Genkit Go model APIs](https://genkit.dev/docs/go/models/)
- [Genkit Go flows](https://genkit.dev/docs/go/flows/)

### Tasks and durable orchestration

Main evidence for distinguishing a Gimbal Turn from a durable Task/Future.

- [A2A Go SDK CLI/task API](https://github.com/a2aproject/a2a-go/blob/main/cmd/README.md)
- [A2A Go SDK changelog / API evolution](https://github.com/a2aproject/a2a-go/blob/main/CHANGELOG.md)
- [Temporal Go workflow API](https://pkg.go.dev/go.temporal.io/sdk/workflow)
- [Temporal documentation](https://docs.temporal.io/)

### Go concurrency idioms

Drove the recommendation for `iter.Seq2[Event, error]` plus an explicit `Wait(ctx)` instead of channels as the primary API.

- [Standard library `iter` package](https://pkg.go.dev/iter)
- [Go Concurrency Patterns: Pipelines and cancellation](https://go.dev/blog/pipelines)
- [Go Concurrency Patterns: Context](https://go.dev/blog/context)

### Antigravity

- [Getting started with Google Antigravity (codelab)](https://codelabs.developers.google.com/getting-started-google-antigravity)
- [Antigravity CLI/SDK code-review codelab](https://codelabs.developers.google.com/agy-cli-sdk-code-review)
- [Google Cloud: I/O '26 news for agent developers (Antigravity 2.0)](https://cloud.google.com/blog/topics/developers-practitioners/io26-news-for-agent-developers-on-google-cloud)
