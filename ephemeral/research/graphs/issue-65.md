# Make validation fluent: isolation, artifacts, a validation node, and declared arguments

URL: https://github.com/tylergannon/gimble/issues/65
State: closed
Milestone: None
Updated: 2026-09-13T16:47:09Z

## Purpose

Validation is the thing Tractor exists to get right, and it is currently assembled by hand out of parts that do not know about each other. A checklist item's `command` and `infer` are the right shape — the engine owns `done`, a judge reads evidence, a failing item reopens — but authoring that well is hard enough that pipelines reach for a test command instead, and several structural holes let a validator reach a verdict it did not earn.

This issue collects the whole topic: what landed in v0.9.0, what is still missing, one open design question, and the rule that keeps it from turning into a process orchestrator.

**The dichotomy this is answering, in Tyler's words:** unit tests, linters and tripwires are wanted everywhere and are good for code quality, algorithms and edge cases — *and they are not proof of work*. A check in the loop is fine. What is missing is a validator that has no stake in completion, cannot see the source, understands that pass/fail is whether the application satisfies its promises rather than a hunt for hypothetical bugs, and **cannot report success except insofar as it actually saw the software working**.

The strongest form of that last requirement which an engine can actually enforce: *no pass unless the engine started the application, observed it become ready, and the verdict was adjudicated only from artifacts produced inside that window.* Everything beyond that is prompt discipline and should be recognised as such.

## Landed in v0.9.0 (#60)

`.tractor/run` — the engine allocates a port, exports `PORT` and `TRACTOR_URL`, starts the repository's own command in a process group, waits for the port to accept, validates the whole set against that one instance, then signals the group away. An application that never accepts fails every item with no judge consulted.

That closes the worst false pass: a validator that truthfully observed the wrong thing — a stale server from the last lap, a developer's own instance, a staging URL. Nothing downstream can catch that, because the evidence is authentic. It is closed by the port being unguessable, not by instruction.

## Open work

### 1. Turn isolation and declared artifacts

Every LLM turn opens with `WithCwd(workdir)` and `PermissionModeBypassAll` (`harness/claude/native.go:25`), and `AgentNode` has no workdir field. That applies to the `infer` judge and to the goal evaluator, whose prompt tells it to "open and inspect every file" (`engine/loop.go`).

So a validator has a shell in the repository. It can read the implementer's commits, and — worse — repair the environment it was sent to judge and then approve its own repair. Source-blindness alone does not close this; taking the working directory away closes all three at once.

- `workdir` or `isolated: true` on agent nodes and on the judge, so the turn's cwd is a scratch directory holding the task and an empty evidence directory.
- Declared `artifacts`, collected the way `fan_out` branch artifacts already are (`engine/artifacts.go`), with zero artifacts a hard failure. `infer` already fails on zero matched evidence files (`engine/loop.go:330`); this generalises that.
- The judge must not receive the validator's prose. If it can read "I signed in successfully" it is the same claim laundered through a second model; isolation is only real when the judge's input differs in kind from the actor's output.

### 2. A validation node — open design question

**Tyler's proposal (not yet ruled on):** a node dedicated to validation, possibly a new kind of supervisor. It patrols on its own clock, gets application rebuilds on node transitions, and when it detects that the application validates, marks the thing done so the development loop can exit — "like an async synchronization primitive."

**The counter-argument (Claude and a Fable reviewer, not ratified):** Tractor's node types each carry routing semantics — loop owns `done`, supervisor cannot route, command routes on exit code — and a `validation` type may add none that an `agent` node with an enforced contract does not already have. And a type hardcoded to a browser ages badly: CLIs, HTTP services and daemons are the same problem with a different client.

Both readings survive the discussion and this is Tyler's call. Note the async design needs item 3 before it is sound, or it is a race: it can pass against a commit that no longer exists and release the loop on a stale verdict.

### 3. Verdicts that record what they were taken against

`done: true` says an item passed, not what it passed against. Re-attestation runs on every lap inside a run and stops when the run does, so a ledger committed with every box ticked is a claim about a moment that has passed.

Stamping a verdict with the commit it was taken at, and only releasing on a verdict whose commit is HEAD, is both the fix for the async validator above and the answer to what a checked box means.

**#51 trials exactly this at workflow level, and Tyler accepted that framing explicitly as not a core feature.** Anything here should wait on what that trial shows rather than pre-empt it.

### 4. Declared run arguments

`--goal` is a literal `strings.ReplaceAll` of `$goal` and is the entire input interface. Nothing can carry a fact a run needs.

- `args:` on the pipeline as a flat JSON Schema, validated at startup and refusing to launch on violation, the way the graph is already linted before a token burns.
- `$args.k` substituted in prompts. `$goal` becomes `args.goal`, required by default, `--goal` kept as UX; `NeedsGoal` in the workflow catalogue becomes schema `required`, which removes a special case rather than adding a system.
- Hard limits, or this becomes a template language: flat scalars only, non-scalars refused at lint, no conditionals or expressions.
- An argument substituted into a `command` string is shell — quote it or refuse it.
- Secrets do not go here. A seeded password would land in prompts, run logs and the timeline. Environment, not arguments.
- **Facts the engine consumes are not arguments.** How the application starts and what port it listens on belong in `.tractor/run`. A base URL in prompt text is how a validator picks its own target again.

### 5. Loops cannot say they need a service

The presence of `.tractor/run` is the entire opt-in, so `tractor validate` cannot tell you a workflow that needs an application has no way to start one — you find out on the first lap. A field on the loop node would let lint catch it, at the cost of a schema change. Deliberately deferred in #60.

### 6. Where the service is available

The service is up for validation only. A reviewing agent that wants to use the application does not get one, which is a real gap for the "run the software yourself rather than reading the diff" review nodes in the built-in workflows.

### 7. Re-attestation cost

The engine re-validates every done item on every arrival. With browser validators that is every scenario, every lap, up to the loop's budget. Starting the application once per arrival (already done) is the main mitigation, but the cost is still linear in closed items and will be felt first here.

### 8. Criteria that cannot drift toward what got built

`df-easy-loop-e2e` upstream fixes acceptance criteria during the requirements interview, before code exists, and fails validation if any is unmet regardless of what else passed.

**Tyler's ruling:** a holdout set is overrated as redundancy. Forcing an independent agent to actually use fully-built software beats it, and plotting quasi-random paths through an application yields more than a fixed list ever will. A holdout earns its place specifically where there is a table of data and the application must behave predictably across all the tuples — that is a designer's call about the shape of the promise.

What survives is not the *holdout* but the *fixed*: criteria should be shown to the implementer, and must not be editable downstream to match what got built. Related: in `delivery-loop` the `plan` node writes both the plan and its checks, so the checks inherit the planner's stake — they likely belong to `plan_critique`, which already asks where an item's check would pass while the item is false.

## The rule that keeps this bounded

**Tractor holds one handle and never looks inside it.** It knows a PID, a port and an exit status — not how many processes there are, what they are, or what order they start in. `exec go run ./cmd/server`, `OVERMIND_PORT=$PORT exec overmind start` and `exec docker compose up` are indistinguishable from the engine's side.

The test for any field proposed here: *does it require the engine to know something about the inside of the handle?* If so it does not belong. That is why there is no `ready:`, `env:`, `depends_on:`, `services:`, `build:` or `port:` — each is a step toward reimplementing Compose badly, and there are better implementations of that than we will ever write.

## Known residual, not planned

Allocating a port means bind-close-reuse, which is a TOCTOU race. It is closed in practice by watching the child: an application that cannot bind almost always exits, and an exit before readiness draws a fresh port and retries. What remains needs an application to fail to bind, survive anyway, *and* an unrelated HTTP server to occupy that exact ephemeral port. Closing it would mean asking the OS which PID holds the listener — `/proc/net/tcp` on Linux, `lsof` on macOS, no Go stdlib. Recorded rather than pretended away; not worth the platform code until it bites.

## Related

#56 per-node skills, #58 model role presets, #59 supervisor nudge content, #51 stale-revision review trial, #48 bounded validation log reads.

🤖 Generated with [Claude Code](https://claude.com/claude-code)

