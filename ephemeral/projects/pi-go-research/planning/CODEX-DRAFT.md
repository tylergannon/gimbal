# Independent draft: owned headless Pi in Gimbal

This is planning input for Tyler and the coordinator, not authorization to implement. Port one coherent headless runtime, qualify useful Go translations against it, and integrate through the existing `HarnessAdapter`. The first milestone is real coding through `pi/diffusion/deepseek-4.1-flash`; it is not full Pi parity. No root-Gimbal API additions, generic orchestration engine, or port implementation is proposed for this planning turn.

## Reference and intended use

The behavioral reference is `/Users/tyler/.local/share/gimbal-pi-research/sources/earendil-works--pi` at `d6af72e1857cfb10b41d8ff8e69f0d72b4cf6d31`. Below, `A`, `L`, and `C` mean its `packages/agent`, `packages/ai`, and `packages/coding-agent`; paths following those prefixes are exact relative paths. Gimbal paths are relative to `/Users/tyler/src/gimbal-pi-research`.

I read `RECOMMENDATION.md` and `REVIEW-STATUS.md`, followed `upstream/corpus/INDEX.md` → `topic-003/INDEX.md`, and checked original loop, session, and test code. `C/src/core/sdk.ts` constructs `Agent` from `A/src/agent.ts` and `AgentSession` from `C/src/core/agent-session.ts`. Use that established stack. `A/src/harness/agent-harness.ts` imports its own runtime and session vocabulary, including Chord types: do not merge that alternative stack into this port. `L/src/compat.ts` is an API facade over actual providers, not another provider implementation. Likewise, `C/src/core/model-registry.ts` explicitly describes itself as an extension compatibility facade over `ModelRuntime`.

Expected uses are a native session doing repository work, successive turns sharing context, a supervisor steering before the next model call, cancellation stopping network and tool work, independent conversation forks, durable reopening, and compaction of long conversations. Model requests run in Gimbal's process; shell tools still create subprocesses. Preserve Codex, Claude, Gemini, and OpenCode behavior.

Current authority is `harness.go`, `session.go`, `events.go` (the intent's singular filename does not exist here), `usage.go`, and Godoc. `internal/binding/binding.go` currently has no Pi case. That observation is local, not a claim about #386's remote progress.

## Scope recommendation

Include the established loop and session lifecycle; text/thinking/tool-call streams; built-in read, edit, write, bash, grep, find, and ls semantics; image reads and tool-result image handling; session tree persistence and projection; clone-style independent forks; automatic and internally callable manual compaction; project instructions, skills, prompt templates, and scoped configuration. Keep default tools distinct from available tools, as `sdk.ts` does. No reflection-based schema derivation.

Recommend a staged provider scope: qualify OpenAI-compatible Chat Completions for Router first. Native Responses, Anthropic, Google/Vertex, Bedrock, Azure, Mistral, Pi Messages, OAuth flows, classifier/image-generation APIs, and deferred inference remain explicit later work. Existing Gimbal harnesses continue providing their current coverage. This is a scoped headless coding port, not a claim to implement every Pi provider. Approving broader coverage requires additional provider assignments and their own qualification; it must not silently enlarge this first batch.

Exclude terminal/web presentation, HTML export, RPC transport, package installation, TypeScript extension execution, extension UI hooks, remote-worker machinery, and the newer harness runtime. Preserve opaque extension/custom entries when reading history where possible, but reject unsupported replay semantics explicitly. Do not pretend excluded extension hooks have run.

Recommend native JSONL compatibility for the supported entry types at this pin, including tested historical migrations, using Gimbal-owned storage. Import operates on a copy; never mutate a user's original Pi session. Unsupported versions or required semantics fail clearly. This is more work than private Go-only persistence, but prevents a second history design and permits meaningful differential tests. Exact import breadth needs approval before contracts freeze.

## Assignments and exclusive ownership

Use `internal/pi/` as the implementation root. Package names below are internal responsibilities, not additions to root Gimbal. Each owner gets all production files, colocated tests, and `testdata/` in its directory unless explicitly narrowed. “Port/adapt” permits donor reuse only after comparison with the pinned TypeScript behavior.

| # | Exclusive destination and responsibility | Primary source and test anchors | Treatment |
|---|---|---|---|
| 1 | `internal/pi/model/`: shared values, schemas, copies, provider/tool contracts, entry records | `L/src/types.ts`, `L/src/utils/transcript.ts`, `A/src/types.ts`, `C/src/core/messages.ts`; `A/test/agent-loop.test.ts`, `C/test/session-manager/build-context.test.ts` | Port; frontier-owned contracts |
| 2 | `internal/pi/provider/`: Chat Completions request/stream/retry implementation | `L/src/api/openai-completions.ts`, `transform-messages.ts`, `simple-options.ts`, `L/src/utils/{json-parse,provider-retry,overflow}.ts`; `L/test/openai-completions-{tool-choice,reasoning-details,retry}.test.ts` | Port/adapt; Router first |
| 3 | `internal/pi/config/`: model metadata, endpoint/auth resolution, settings and trust | `C/src/core/{model-runtime,model-config,models-store,provider-composer,auth-storage,settings-manager,resolve-config-value}.ts`, `L/src/auth/resolve.ts`; `C/test/{model-runtime-auth-options,settings-manager,runtime-credentials}.test.ts` | Port/adapt supported configuration |
| 4 | `internal/pi/files/`: path resolution, truncation, shared file-mutation queue, image normalization | `C/src/core/tools/{path-utils,truncate,file-mutation-queue}.ts`, `C/src/utils/{image-process,tool-result-images}.ts`; `C/test/{file-mutation-queue,path-utils,image-processing}.test.ts` | Port/adapt |
| 5 | `internal/pi/readtools/`: read, grep, find, ls | `C/src/core/tools/{read,grep,find,ls}.ts`; `C/test/tools.test.ts`, `C/test/suite/regressions/3303-find-nested-gitignore.test.ts` | Port/adapt; no renderers |
| 6 | `internal/pi/edittools/`: edit and write | `C/src/core/tools/{edit,edit-diff,write}.ts`; `C/test/{tools,edit-tool-legacy-input}.test.ts` | Port/adapt; shared queue from #4 |
| 7 | `internal/pi/shell/`: bash, output capture, process cleanup | `C/src/core/{bash-executor,tools/bash,tools/output-accumulator}.ts`, `C/src/utils/{shell,child-process}.ts`; regressions `5208-late-bash-output` and `5303-bash-output-truncation` under `C/test/suite/regressions/` | Port/adapt; OS behavior explicit |
| 8 | `internal/pi/resources/`: instructions, skills, templates, system-prompt assembly | `C/src/core/{resource-loader,skills,prompt-templates,system-prompt}.ts`; `C/test/{skills,resource-loader,system-prompt}.test.ts` | Port/adapt headless subset |
| 9 | `internal/pi/history/`: JSONL IO, migrations, branches, projection | `C/src/core/session-manager.ts`, `messages.ts`; `C/test/session-manager/{migration,load-entries,save-entry,build-context,tree-traversal}.test.ts` | Port/adapt; shared entry types owned by #1 |
| 10 | `internal/pi/compact/`: cut points, summaries, branch summaries, token estimation | `C/src/core/compaction/{compaction,branch-summarization,utils}.ts`; `C/test/{compaction,compaction-serialization,branch-summarization}.test.ts` | Port/adapt; history stays non-destructive |
| 11 | `internal/pi/agent/`: loop, scheduling, steering queues, continuation | `A/src/{agent,agent-loop,types}.ts`; `A/test/{agent,agent-loop}.test.ts` | Port/adapt established stack only |
| 12 | `internal/pi/session/`: assembly, lifecycle, recovery, settlement, reopen and clone | `C/src/core/{sdk,agent-session,agent-session-runtime}.ts`; `C/test/suite/agent-session-{queue,compaction,retry-events,boundaries}.test.ts` | Port selected session semantics |
| 13 | `internal/pi/adapter/`: existing HarnessAdapter implementation, event projection, output and usage | Gimbal `harness.go`, `session.go`, `events.go`, `usage.go`; upstream `C/test/rpc-prompt-response-semantics.test.ts` for lifecycle reference only | Gimbal bridge; no RPC port |
| 14 | `internal/workflows/piport/` only: dedicated delivery workflow and its tests/help | `internal/workflows/review/review.go`, `group.go`, `iterate.go`, `command.go`, workflow-authoring skill | Existing Gimbal orchestration |
| 15 | Integration owner: `go.mod`, `go.sum`, dependency notices, `internal/binding/`, `internal/modelalias/`, builtin registration, generation, actual #386 cutover files | Existing binding and modelalias tests; actual rebased #386 implementation | Gimbal integration; serial ownership |
| 16 | `internal/pi/qualification/`: integrated black-box package tests and independent live qualification | `C/test/suite/regressions/{8724-in-memory-fork-active-tool,6363-agent-settled-event,9340-9777-auto-compaction-cancellation}.test.ts`; Gimbal session tests | Independent acceptance |

These are sixteen assignments, not sixteen simultaneous workers. #15 alone runs generation and modifies generated files; #14 requests registration changes from #15. Package tests stay with their owners; #16 adds independent integration tests without rewriting module tests. Shared schema changes return to #1, then rebase dependent work. No worker edits another package or the module files to make its branch compile. Shared file utilities remain below tool packages; resources imports config/files/model, and agent imports only model. Session composes all modules. Adapter and qualification sit above session. This direction prevents history/compaction and tool/session cycles.

## Contracts to land before fan-out

#1 and #15 land a compiling baseline containing real value definitions, interface signatures, serialization/copy tests, and approved dependencies. Concrete implementations can follow; workers must not invent incompatible local placeholders.

The baseline should define these internal signatures, with the referenced values and their codecs owned by #1 (illustrative Go, not a request for public API):

```go
type Provider interface {
    Stream(context.Context, Request, func(Delta)) (AssistantMessage, error)
}
type Tool interface {
    Definition() ToolDefinition
    Execute(context.Context, ToolCall, func(ToolUpdate)) (ToolResult, error)
}
type Store interface {
    Snapshot(context.Context) (HistorySnapshot, error)
    Append(context.Context, Revision, []Entry) (Revision, error)
}
```

Snapshots carry active leaf and revision; appends reject a stale revision. Concrete clone/open operations belong to history, while controller operations belong to session. Delta values identify the message/content block and distinguish start/update/end; complete messages carry stop reason, usage, and provider replay metadata. Tests fix JSON representations and nested-copy behavior before workers consume these types.

**Messages and providers.** Typed tagged content includes text, images, thinking with replay metadata, tool calls with stable IDs and raw JSON arguments, and results with matching call IDs. Include system/tool declarations and supported coding history variants. Own all slices, maps, and raw byte buffers at snapshot boundaries. Provider `Stream(ctx, request, emit)` returns an authoritative completed assistant message or failure; deltas cannot substitute for completion. Request carries resolved provider/model, capabilities, endpoint, headers, credentials, effort, and transcript explicitly. Preserve absent metadata versus actual reported zero where it affects accounting. Provider normalization is separate from Gimbal event projection.

**Tools and loop.** Tool definitions contain explicit JSON schemas, execution mode, cancellable execution, update callback, result content/details/error/termination. Tool argument validation is independent of final-output validation. Tool failures normally become model-visible results; context cancellation terminates execution. In `A/src/agent-loop.ts`, preparation is ordered; default execution can be parallel; one sequential tool serializes the entire batch. Completion events may interleave while persisted results retain source order. Length-truncated calls never execute. Corresponding tests are at `A/test/agent-loop.test.ts:408,620,714,819,901`. Preserve steering polls around request preparation rather than importing an older “interrupt remaining tools” algorithm.

**History and compaction.** `history` owns durable append and active-leaf projection, importing only shared model values. `compact` consumes an immutable history snapshot and provider capability, returning a proposed summary/cut; `session` validates the source revision and commits it. History does not import compact, agent, or adapter. Cancellation or summary failure cannot install a partial compaction. Keep tool-call/result boundaries and system/tool-state changes in projection. Session storage has one writer per path; incompatible concurrent opens fail rather than silently share mutable state.

Configuration carries explicit workdir, home/config directories, environment snapshot, trust choice, and storage root. Never use process-wide `Chdir` or `Setenv`. Scope dynamic command caches to that configuration; honor stored-credential ownership in `L/src/auth/resolve.ts`, with no ambient fallback after a stored credential fails. Default to explicit project trust supplied by the host.

**Lifecycle.** One session controller owns active turn, queues, append ordering, and settlement. Steer acknowledgments refer to consumption before the next model call, not enqueue success. Waiting callers receive false if the turn settles first; cancellation clears that turn's pending inputs. HTTP, tools, retry waits, and compaction receive the active context. Close cancels, joins, releases resources, and is idempotent. Do not hold a session mutex through network calls or callbacks.

Fork follows Gimbal's independent-copy meaning, not Pi's rewind-to-user-message command. Recommend capturing a consistent committed prefix under the controller, excluding an unfinished assistant/tool batch; parent execution continues. Copy nested content, configuration, and credentials by value where mutable, allocate new queues/cancellation state, and never copy live tool processes. Freeze this active-turn snapshot rule before dispatch and test it explicitly: upstream #8724 is a regression reference, not an identical Gimbal API contract.

**Adapter and ownership.** Only adapter imports root Gimbal. Serialize `onEvent` delivery with stable native message/tool references; map into the existing OpenCode-shaped event vocabulary. Report observation gaps without converting successful execution into failure. Provider failure, cancellation, missing terminal result, and invalid required output remain failures. Return after session settlement, including recovery/compaction, not at `agent_end`. Schema prompts request one JSON value; return raw candidate bytes to existing `Generate` validation/re-ask. Text returns a JSON string. Account for all model calls, including recovery and compaction, without double-counting step events and turn totals.

Resume needs an explicit construction path because `HarnessAdapter` has no Resume method. Recommend an internal adapter-construction option carrying a validated native session reference, consumed by `CreateSession`; ordinary construction creates new sessions. #15 must wire this through the actual #386 caller/configuration surface after rebase, without changing root APIs. Do not claim arbitrary Gimbal-run restart merely because native session reopening works.

## Donors and scope limits

Inspect sky-valley at `e6b56e7223cfab707b21a0a19e0a9a2417ac960a`, not as an oracle. Its `agent/loop.go`, `coding/editmatch.go`, `coding/session_store.go`, and `ai/providers/` are useful translation candidates. `coding/editmatch.go` already handles NFKC and JavaScript-like trimming; compare byte/Unicode offsets and untouched text against current `edit-diff.ts`. Its `agent/transcript_test.go` and `agent/testdata/transcript/capture.mts` identify an older `95fbc0499`/0.87.0 reference, so recapture against our pin. `coding/remotesession.go` and `coding/transcript.go` import client/protocol; `ai/types.go` imports telemetry. Extract only needed code and dependencies.

The reported race-suite passes and shell/ambient-home failures are earlier observations, not tests rerun for this draft. Distinguish #5303 output still arriving through an open pipe from #5208 callbacks arriving after an operation resolved: upstream requires capturing the former and ignoring the latter.

Secondary donors remain eligible: pi-llm-go `providers/openai/openai.go:55–86` supports custom URLs/headers, and Pi Agent Go `tool.go:148` supplies reflection-free `Raw`. Its `agent.go` Snapshot copies outer messages while message contents contain slices; do not adopt that as deep-fork proof. Preserve upstream MIT notices and Sky Valley notices for translated/copied portions, including fixtures. #15 owns consolidated license placement.

## Waves and dedicated workflow

First settle #1 contracts and #15 dependency baseline; #14 can design against existing Gimbal independently. Then run #2, #3, #4, #9, #11 in bounded batches. After those land, #5, #6, #7, #8, #10 can proceed; #8 needs configuration/files, #10 needs provider/history. #12 follows those implementations, #13 follows session, and #15 performs route cutover. #16 can design tests early but accepts only the integrated build. Cap concurrent implementation branches at three initially; five ready packages therefore take at least two batches. Reviews also consume slots. Integration remains serial between batches.

Write one ordinary workflow, not a reusable DAG scheduler. Inputs are an accepted local plan, source root/pins, baseline commit, and explicit remaining assignments. Materialize local handoffs with absolute source/test paths, allowed files, dependencies, acceptance checks, and exclusions. Prompts say “Read and implement the assignment in /absolute/path.” Run-owned handoffs/check output stay outside tracked source. All worker worktrees start from the exact integrated batch baseline.

Illustrative control flow, inside `gimbal.Run` (worktree creation and integration are inline `RunCommand` calls, omitted here for space):

```go
for batchCtx, batch := range gimbal.Iterate(ctx, "batch", batches) {
    group := gimbal.Group(batchCtx, "implementations")
    for _, assignment := range batch.Assignments {
        group.Go("module", func(child context.Context) error {
            coder := gimbal.NewSession(child, implementRole, assignment.Worktree)
            coach := gimbal.NewSession(child, coachRole, assignment.Worktree)
            accepted := false
            for attemptCtx, _ := range gimbal.Iterate(child, "attempt", []int{1, 2, 3}) {
                _, err := coder.Generate[gimbal.Text](attemptCtx,
                    "Read and implement the assignment in " + assignment.Handoff,
                    gimbal.WithSupervisor(coach, "Keep to the assigned behavior and files."))
                if err != nil { return err }
                code, out, stderr, err := gimbal.RunCommand(attemptCtx,
                    "module-tests", assignment.Worktree, "go", assignment.TestArgs...)
                if err != nil { return err }
                gimbal.Set(attemptCtx, "test-output", out + "\n" + stderr)
                judge := gimbal.NewSession(attemptCtx, reviewRole, assignment.Worktree)
                verdict, err := judge.Generate[Verdict](attemptCtx,
                    "Read the local handoff and actual diff. Independently check acceptance; report concrete defects.")
                if err != nil { return err }
                if code == 0 && verdict.Accepted { accepted = true; break }
                // Save findings and failed exit status into this assignment's
                // local handoff for the next attempt. IO failure returns error.
            }
            if !accepted { return fmt.Errorf("assignment %s not accepted", assignment.ID) }
            return nil
        })
    }
    if err := group.Wait(); err != nil { return err }
    if err := batchCtx.Err(); err != nil { return err }
    // Inspect allowed-path diff; commit verified worker changes, then cherry-pick
    // each serially into integration via RunCommand. Check every exit code.
    // Run integrated tests/build before starting the next batch.
}
return ctx.Err()
```

`Verdict` is a workflow-local generated output type; no new root API. Integration also checks the expected assignment count so stopped iteration cannot mean completion. `Check` may add diagnostic context, but its nil error does not mean exit zero. Enforce ownership from actual Git diffs before acceptance. Group's first ordinary error cancels siblings; always call Wait to join. An operator-killed child does not cancel siblings, but Wait still returns its error. Cleanup follows joining with a bounded independent cleanup context; retain failed worktrees for diagnosis.

Use existing Codex/Claude harnesses to build the port. Cheap implementations can use Luna/Haiku, escalating uncertain contracts and final acceptance to frontier judgment. Workflow smoke runs use `gpt-5.6-luna` or Haiku and report the model. Scope coaches are advisory; invalid evidence and unmet requirements govern acceptance. Three attempts bound implementation rejection; infrastructure failure propagates immediately. No automatic merge-conflict invention or weakening tests.

Restart from inspected integration commits plus retained worker branches. Supply remaining assignments explicitly; verify accepted commits are ancestors and contracts still match, then recheck affected integration. Do not infer success from an old green agent summary or promise transparent workflow resumption. #15 rebases onto #386's actual result and alone replaces its Pi implementation/registration while preserving the single `pi/...` namespace; no duplicate route or compatibility shim.

## Qualification and acceptance

Translate relevant upstream cases beside each package. Direct differential tests feed identical scripted provider events, tool outcomes, clocks, and filesystem fixtures to the pinned TypeScript stack and Go; compare requests, history projection, results, and semantic event order. Normalize only incidental IDs/timestamps, preserving identity relationships. Execute the reference from a disposable copy; durable repeatable comparison drivers are package tests, never committed proof programs or captured live logs.

Qualification must demonstrate fragmented tool JSON, reasoning replay, finish reasons, failed/truncated streams, argument errors, ordering, Unicode edits, symlink mutation serialization, output truncation, cancellation, resource precedence, migration, compacted reopening, and fork deep isolation. Use isolated homes and configuration, race tests, and targeted fault injection. Donor green tests do not establish current parity.

Then run the built application through normal binding and `run-prompt`, using Router `deepseek-4.1-flash`: read a seeded file, edit it, execute a shell assertion, return schema-valid output, and continue a second turn. Inspect the file and command result yourself. Observe streamed tools/text, consumed steering, cancellation of a process tree, independent parent/child conversations, reopening in a fresh process, and compaction followed by retained-context use. Exercise two concurrent sessions with different directories/configuration and another existing harness. Verify no Pi agent subprocess exists; shell children are expected. Confirm UI activity through the existing projection.

The local #386 issue snapshot gives Router's compatible endpoint configuration, not live acceptance of `store`, token-limit, reasoning, or usage fields. Validate actual outgoing requests and Router responses. Missing credentials or unsupported fields block the Router milestone, not permission to substitute a mock claim. Report observed model, outcomes, usage availability, and remaining limitations in chat/PR. Commit neither run logs nor screenshots.

Decisions before execution: approve scoped provider breadth, native import versions, active-turn fork boundary, resume caller surface after #386, and first supported OS (recommend macOS/Linux; defer PowerShell/Windows qualification explicitly). Main risks are upstream version drift, hidden extension/presentation coupling, process-global caches in an embedded runtime, and premature settlement. Resolve those through the named contracts and tests, not size estimates. Research bug #389 and unrelated sprint work remain outside this port.
