# Static workflow entrypoints and the multi-project host

Assessment of `cde79c24`, including PR #362 (`3ee80979`), on 2026-09-23.
This is a recommendation for review, not an implemented or accepted replacement
contract. Three independent code passes covered dispatch, host ownership, and
SKGO/Polytype transport. No application code was changed.

Subsequent decision: Tyler chose automatic project admission and the same SKGO
remote endpoint for browser and generated Go clients, with workflow definitions
remaining entirely at design/build time. The
[implementation plan](implementation-plan.md) and
[definition of done](definition-of-done.md) supersede this assessment's separate
JSON endpoint recommendation. The findings below remain the historical audit.

## Recommendation

Keep the multi-project, single-process host. Replace its generic workflow
submission dispatcher with generated, typed entrypoints for each compiled
workflow. Generate each CLI's call to its specific endpoint from the same
workflow declaration. Each handler must call its concrete Go workflow directly.

Share a workflow's typed admission function between its CLI HTTP adapter and
its browser remote function. For the CLI, prefer a small generated JSON endpoint
on the existing control socket. For an actual browser start form, use SKGO Form
over that same typed operation. Using the identical SvelteKit wire protocol for
both clients is possible, but adds framework coupling without improving the
workflow's static call chain. Do not introduce a second copy of validation,
model binding, or run ownership in the adapters.

This is a good architecture for Gimble's local, trusted, compiled Go workflows.
It keeps execution visible to the compiler and to readers, fits the existing
generator, and retains one authoritative owner of live state. Its advantage is
clarity and typed contracts, not dispatch performance: an HTTP router still
selects a handler at runtime. The useful invariant is that after routing to a
workflow's independently defined handler, no request-supplied workflow name
selects an erased executable from another registry.

## What exists

The current hosted call chain is:

```text
generated review command
  -> web.Submit(Submission{Name: "review", Params: raw JSON, ...})
  -> POST /control/submit
  -> instance.workflows[request.Name]
  -> generated Hosted() decoder
  -> review.Review(ctx, env, params)
```

The relevant code is `internal/generate/command.go:121,207`,
`web/submit.go:32`, `web/control.go:94`, and `web/runtime.go:67-78`.
`cmd/gimble/workflows.go:44-51` manually assembles the executable registry.
Separate Cobra entrypoints therefore do not currently imply static server
invocation. Generating several URLs that still call this dispatcher would not
correct the architecture.

The host itself is useful and substantially aligned:

- An Instance owns one web listener and one control socket; each admitted
  project owns its registry, live controls, conversations, and durable files
  (`web/runtime.go:54-70,261-282`). Submitted bodies execute in server
  goroutines (`web/control.go:119-126`).
- Canonical project paths and a project lock prevent two Instances from owning
  the same project (`web/runtime.go:222-254,285-291`). Multiple explicitly
  configured instances may still host different projects.
- Owning project and execution directory are separate. A conversation's
  worktree does not become the storage owner (`web/control.go:99-112,124`).
- Accepted work derives from the project's lifetime, survives client exit,
  appears in that project's live registry, and participates in coordinated
  shutdown (`web/control.go:119-124,283-300`; `web/runtime.go:437-466`).
- Run publication is coordinated with registry visibility; hosted root and
  Group panics are contained. Those fixes should survive the admission rewrite
  (`internal/observation/store.go:127-145`; `web/runtime.go:427-434`).

PR #362's description reports overlapping built-binary reviews in two projects
using `gpt-5.6-luna:low`, one host PID, project-scoped pages, restart history,
and active-run SIGINT cleanup. This assessment inspected that report and ran
focused tests; it did not repeat those live provider runs. See
[PR #362](https://github.com/tylergannon/gimble/pull/362).

## Dynamic-dispatch inventory

There is one active workflow execution registry: the hosted submission path
above. The audit found no additional rerun dispatcher, child-workflow registry,
workflow interpreter, or server subprocess fallback in active application code.
Frozen legacy code was excluded.

Conversations now instruct their agent to invoke the ordinary CLI with explicit
instance, project, workdir, and conversation flags
(`internal/conversation/manager.go:587-591`). They converge on the same
submission path. Their former structured workflow response and direct launch
callbacks are gone.

These other selections are appropriate and should remain:

| Selection | Purpose |
| --- | --- |
| HTTP and Cobra routing | Select an independently defined application entrypoint. |
| Project map | Select state and ownership for a requested project. |
| Run-ID control table | Steer or stop an existing run, rather than select a new workflow. |
| Graph registry | Look up display metadata for a recorded workflow name. |
| Provider/model binding | Configure the harness used by a known workflow role. |
| PromiseLoop task selection | Select data for the explicit body already written in Go. |
| Run/Group callbacks at concrete call sites | Establish lifecycle and concurrency for ordinary Go bodies. |

`run-prompt` is a deliberate separate-process/headless path over a fresh log
directory (`cmd/gimble/run_prompt.go:45,72-101`). Direct library `gimble.Run`
also remains standalone (`run.go:145-150`). Neither dynamically selects a
workflow or silently takes over an admitted project's hosted execution.
Retain these as explicitly separate library/tool modes unless the desired
scope is broadened to eliminate all standalone execution.

Two shipped examples expose a registration mismatch:
`cmd/examples/interview/main.go:13` and
`cmd/examples/implement-interview/main.go:13` construct generated commands,
but neither workflow is registered by the stock server. Against that server,
their submissions reach the unknown-workflow refusal. This is a source-proven
call-chain mismatch; no provider run was started to reproduce it. Prefer
keeping these as explicitly standalone library examples, or supply a matching
example host. Adding them to the stock product is a separate scope decision.

## Reusing SKGO remote functions

There are no current built-in workflow-start remotes to reuse. The complete
`web/skgo.remotes.json` lists interview answering, conversation creation and
messages, run cancellation, turn stopping, steering, and loop steering.
`docs/web-app.md:90` still describes the start-workflow form as designed, not
built. The API design record's Form example is a design sketch; it is not the
active admission implementation.

Reusing the same typed Go operation is straightforward in principle. Reusing
the exact remote HTTP protocol is a separate choice:

| Choice | Assessment |
| --- | --- |
| Generated JSON endpoint plus SKGO remote, both calling one typed admission function | Recommended. Small CLI protocol, explicit project selection, headless support, shared behavior. Browser stays on SKGO remotes. |
| Generated Go client for the SKGO remote protocol | Feasible, but requires framework-owned client/endpoint metadata support and project context over the control socket. Prefer an SKGO feature if this is chosen, not copied protocol code in Gimble. |
| Hand-maintained CLI imitation of SvelteKit browser requests | Avoid. It duplicates framework framing, endpoint IDs, error handling, and origin/referrer assumptions. |

Pinned dependencies are SKGO v0.5.0 and Polytype v1.0.3 (`go.mod`). Relevant
constraints from those versions:

- Commands use module-derived remote IDs, base64url devalue arguments, and
  remote result/error envelopes. HTTP success alone is not workflow admission
  success. Forms additionally have their own form-data encoding. See pinned
  `skgo@v0.5.0/remote.go:262,754,927`,
  `internal/remotearg/remotearg.go:89`, and `remote_form.go:132`.
- Browser project selection currently uses the referring page for remote URLs
  (`web/runtime.go:366-402`). The CLI should select its project explicitly on
  the local control transport, as it does today, rather than invent a browser
  Referer. Browser origin checks must retain their existing meaning.
- A no-web instance mounts control/observation routes, not the complete SKGO
  web stack (`web/control.go:323-332`). Calling browser remotes from the CLI
  would require deliberately mounting the relevant handlers headlessly.
- There is no public Go remote client in the pinned SKGO surface. Form encoding
  is internal. Generated request/result codecs alone do not implement the
  complete remote protocol.
- Transport error mapping also belongs in the adapters: SKGO Form field
  issues, JSON admission failures, and run terminal failures are different
  results. In pinned SKGO, returning `skgo.Invalid` from a Command does not
  produce a Form-style field issue (`remote.go:968`). Keep shared validation
  independent of that browser-only error representation.
- Polytype's devalue encoder already omits absent Optional values and preserves
  present zero, false, and empty values; its decoder reconstructs presence.
  See `polytype@v1.0.3/devalue/codegen/encode.go:135-144` and
  `devalue/codegen/decode.go:273-289` in the module cache. A devalue upgrade is
  not demonstrated as necessary for the current scalar workflow inputs.
- Existing workflow input structs need correct optional JSON tags before
  passing through Polytype's typed grammar. For example,
  `researchdocument.Params` has untagged Optional fields
  (`internal/workflows/researchdocument/researchdocument.go:73-75`), while
  Polytype requires `json:",omitzero"`
  (`internal/builder/typegrammar.go:726-727`). The current CLI manually omits
  absent values in a temporary map, so it bypasses this issue today.
- The current `Models map[WorkflowRole]string` is outside Polytype's closed
  grammar. Generate fields for the workflow's known roles and construct the
  binding map within the server. Expanding Polytype with general maps is not
  necessary to express a statically known role set.
- There is a real SKGO Form gap. Form arguments bypass generated Polytype
  codecs (`skgo@v0.5.0/internal/gen/codecs.go:80-94`). SKGO's form decoder
  assigns strings, numbers, and booleans directly to corresponding Go kinds;
  it does not unwrap `polytype.Optional[T]`
  (`internal/formdata/decode.go:68-82,102-122`). A present scalar optional
  cannot bind directly to the Optional struct. Browser start forms reusing
  these exact types therefore need focused SKGO form-binding support for
  presence/coercion, or generated form-specific conversion. Prefer the focused
  SKGO improvement when those forms are implemented. Using Command to avoid
  the problem would violate the app's real-form convention.

Official SvelteKit documentation describes remotes as generated HTTP
entrypoints, uses devalue for applicable arguments/results, and still labels
the feature experimental. That supports sharing the typed operation while
keeping framework wire details inside SKGO:
[Remote functions](https://svelte.dev/docs/kit/remote-functions).

## Package and generation consequences

A route cannot simply import a workflow package today. Generated workflow files
import `web`; `web/server.go` imports `internal/skgo`; generated SKGO code imports
the routes. A route importing the workflow would complete a Go import cycle.

Separate generated CLI/client integration from the packages containing workflow
bodies and graphs. Application assembly can then import concrete workflows and
wire their typed handlers. Shared run ownership must sit below the adapters,
without depending on route assembly. Do not break the cycle by inserting
another name-to-callback registry: that recreates the rejected architecture.
This is a packaging change, not a reason to wrap workflow tactics in a new
framework.

SKGO markers require a locally declared function
(`skgo@v0.5.0/internal/gen/scan.go:392`). A generated local remote forwarding
to its typed admission operation is supported; marking an imported function
directly is not. That small transport adapter does not select workflows.

Generation must keep the built-in inclusion set aligned across CLI commands,
HTTP handlers, remote declarations where needed, graph metadata, and help.
Use one build-time input with explicit generated wiring. Generating code and
registering literal HTTP routes is compatible with the architecture; runtime
workflow discovery or an interpreter is unnecessary.

Only workflow bodies compiled into the chosen server can execute there. A
new authored workflow requires rebuilding and restarting that server. A
separate CLI cannot transmit a Go closure. Keep the current in-checkout
authoring boundary explicit; do not imply external plugin loading or an
external-module authoring contract that the current tooling cannot support.

## Other ramifications

1. **On-demand projects are not exposed yet.** The stock CLI can only target
   an already admitted project. Startup accepts repeated `--project` flags;
   subsequent admission exists only through the Go `AdmitProject` method.
   The client's discovery probe and every control route reject an unadmitted
   project (`web/submit.go:113-125`; `web/control.go:323-332`). For ordinary
   use against arbitrary directories, let the selected server admit the
   supplied project on first submission. Instance discovery must first prove
   the instance is alive independently of that project's admission.
2. **Instance selection is separate from project selection.** The CLI uses
   `GIMBLE_INSTANCE_DIR`, otherwise cwd-relative `.gimble`. An admitted
   project's directory works because the host writes project-local discovery
   (`web/control.go:306-319`), but `--project /B` from an unrelated cwd does
   not itself select the host. A consistent configured instance directory
   provides the desired one-host experience. Preserve explicit overrides and
   explicit startup; automatic bootstrap is not required for this correction.
3. **Validate before acceptance.** Generated typed handlers should reject
   malformed inputs, missing required inputs/roles, and inappropriate role
   names before creating a run. Today only the outer envelope and supplied
   role entries are checked first; concrete parameters decode inside the
   already started run (`web/control.go:89-125`; generated `Hosted`). Required
   Cobra flags do not protect direct HTTP callers. Structural validation and
   existing workflow-specific domain validation should have one owner each.
4. **Preserve detached ownership.** Admission returns a run ID after live
   publication, not a completed workflow result. The workflow context comes
   from the server/project, not the HTTP request. Client exit or stopping
   follow does not stop the work. Explicit run cancellation remains separate.
5. **Preserve conversation bookkeeping.** The typed admission path must retain
   project/worktree checks and durable conversation association/terminal status
   updates (`web/control.go:107-112,145-154`). Different transport adapters must
   not quietly have different launch behavior.
6. **Keep execution context explicit.** Project, workdir, inputs, and role
   choices cross the boundary. Provider credentials, PATH, executable lookup,
   and inherited process environment belong to the server. Avoid process-wide
   chdir/environment changes. Owning a run's observations does not isolate two
   concurrent workflows writing the same execution directory.
7. **Client and running server can differ.** Installing a new executable does
   not update the already-running process. A small version/identity check and
   actionable rebuild/restart errors are preferable to silently accepting
   incompatible request shapes. No compatibility framework is needed.
8. **Admission is a side effect.** Do not automatically retry after a lost
   response: the run may already exist. Current submission has no deduplication
   identity. Keep that behavior explicit; exactly-once admission is not a
   prerequisite invented for this change.
9. **One host shares resources and failure fate.** Existing recovery covers
   workflow/Group panics, not arbitrary unmanaged goroutines or process-fatal
   failures. Cancellation is cooperative. This is an appropriate tradeoff for
   trusted local Go code, not an isolation boundary for untrusted plugins.
10. **Persistent hosting changes operational assumptions.** Server shutdown
    currently handles SIGINT but not SIGTERM (`cmd/gimble/main.go:231`), unlike
    run-prompt. Also, completed observation stores never evict
    (`internal/observation/registry.go:15-18`). Address normal service shutdown
    directly; assess retention with measured long-lived use. Do not turn memory
    scaling into a speculative prerequisite for static entrypoints.

## Course of action

First, settle the static invocation boundary in this assessment: compiled
workflow-specific handlers and direct typed calls, with runtime selection only
of project/run state and ordinary transport routing. Keep the multi-project
Instance and existing ownership behavior.

Then deliver the admission replacement as one bounded change: split generated
CLI integration away from workflow body packages, generate per-workflow typed
requests and literal endpoints, route generated commands to them, and delete
Submission's workflow-name selection, WorkflowEntry, WithWorkflows, Hosted, and
the executable registry. Share project admission and run lifecycle; keep the
concrete workflow call visible in each handler. Correct the examples and
update affected authoring instructions alongside the change.

Include on-demand project admission if "arbitrary project directories" means
first use from the CLI, as this assessment recommends. Keep instance discovery
independent from the selected project's prior admission. Preserve detached
runs, role choices, live controls, conversation linking, and terminal follow.

Browser start forms are currently unbuilt. When adding them, generate or author
per-workflow SKGO Forms over the same typed admission operations. Fix SKGO's
Optional form binding at that point. No evidence requires a Polytype devalue
upgrade or a general SKGO Go client to replace dynamic workflow dispatch.

If identical remote endpoints for CLI and browser are a firm product goal,
choose that deliberately and implement supported Go remote clients/endpoint
metadata in SKGO, plus headless mounting and explicit project context. That
is a valid alternative with more framework work; it should not emerge as an
accidental imitation of browser traffic in Gimble.

## Verification and completion evidence

Existing focused tests passed in this assessment for Instance/project
ownership, browser project controls, detached submissions, conversation
association, run publication, hosted panic containment, generated CLI optional
presence, generated source, and command help/required inputs. The separate
dispatch audit also ran the focused analyzer/generator tests successfully.
No provider/model workflow was run for this assessment. These tests support
the current hosting behavior, not conformance to the proposed replacement.

Current generator tests actually require `Hosted() web.WorkflowEntry` and
generic `web.Submit` (`internal/generate/graph_test.go:323-325`). Change those
expectations with the generator. There is also a stale lint callback index:
`internal/gimblelint/analyzer.go:694-697` treats Run's callback as argument 2;
the current signature places it at 3 (`run.go:151`), and the test stub still
models the old signature. Correct that narrow gap if lint is relied on, without
building a whole-program architecture analyzer.

The implementation should demonstrate:

- The built server's concrete endpoint receives the exact workflow inputs and
  calls that workflow; generation/compilation fails on mismatched signatures.
- CLI and browser admission adapters, when both exist, preserve omitted values
  and explicit zero/false/empty values and report the same admission errors.
- Invalid inputs are rejected before a run is created. Project identity,
  workdir, roles, and conversation linking reach the intended run.
- Separate CLI processes start overlapping cheap-model workflows in projects
  A and B under one host PID, including a project unknown at startup if
  on-demand admission is included. Each page/control targets only its project.
- Client exit and follow interruption leave accepted work running; explicit
  cancellation, failure, panic, shutdown, and restart history retain their
  established behavior.
- Headless invocation works. A missing/stale/incompatible host produces an
  actionable error and never silently starts a second runtime owner.

Use current source plus observed runs for the final report. No committed proof
programs, run logs, screenshots, or transcript dumps are needed.
