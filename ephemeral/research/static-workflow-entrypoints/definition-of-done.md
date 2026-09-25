# Definition of done: shared workflow-start remotes

Applies to [implementation-plan.md](implementation-plan.md). This is the
completion standard for the future implementation, not a report of completed
work. The repository's [definition of done](../../../docs/definition-of-done.md)
continues to govern validation and delivery.

## Scope amendment (2026-09-23)

At the user's direction, visible human start forms and all browser/UI-specific
proof are deferred from the current CLI integration outcome. They are not a
gate for that outcome and must not be implemented as part of it. The scoped
outcome must still prove all five generated CLI clients reach their matching
Form handlers and establish the requested CLI transport and lifecycle behavior.
The separate UI outcome will not run in this workflow. Visible forms remain an
explicit uncompleted item outside this delivery.

The change is done when the architecture is present and the user-visible
behavior below has been observed. Compilation and green tests support those
claims; they do not substitute for the stated running-application evidence.

## Architecture and generation

- Workflow definitions are authored, generated, and compiled before startup.
  Every built-in start remote has a concrete input type and a direct call to
  its named Go workflow. Runtime requests carry no workflow implementation or
  discriminator used to select an executable.
- Workflow packages do not import CLI integration or web assembly. Host
  ownership can be used by route handlers without importing their own assembly.
  The application compiles with the intended dependency direction and retains
  one production handler composition shared by tests and the binary.
- The five built-ins have matching generated CLI clients and SKGO Form handlers.
  Browser and Go clients address the same generated remote identity and execute
  the same handler, even when they reach it through different listeners.
- The old generic submission endpoint, erased Hosted adapters, and executable
  workflow registry are gone. Existing metadata, project-state, and live-run
  control maps do not become alternate workflow dispatchers.
- Generation succeeds from a clean checkout with the pinned released
  dependencies, refreshes stale output, and produces no second-run diff. A
  changed input/result contract updates both client bindings; incompatible
  usages are reported by generation or compilation. No hand-edited generated
  output or local module replacement is required.

Evidence: source/import inspection and focused generator/compilation tests,
including an actual generated client sending an HTTP request to the generated
server handler. A direct call to a shared Go function alone does not prove the
client/remote path.

## Transport and input behavior

- Go and browser submissions preserve omitted Optional values and explicitly
  supplied zero, false, and empty values for the supported input shapes.
- Workflow inputs, role defaults/overrides, project, workdir, and conversation
  association have the same meaning in both clients. Model resolution and
  provider environment remain server-owned.
- Admission errors are returned before any run is created. The browser shows
  the relevant form issues; the CLI reports the error and exits unsuccessfully.
  A remote error envelope is never mistaken for success because HTTP was 200.
- Form validation-only requests cannot start a workflow. Cancelling a client
  call stops its network wait; it does not cancel work already accepted by the
  server. The client does not automatically retry a launch after a lost reply.

Evidence: focused SKGO protocol tests using independent SvelteKit-produced
fixtures, plus real browser and Go-client submissions to the same handler.
Exercise invalid required input, an invalid role choice, and optional presence;
verify the rejected submissions created no run. Tests cover every built-in's
generated binding without requiring five expensive live workflow executions.

## Human and CLI use

- The web application exposes a visible, usable start form for each of
  `review`, `implement`, `research-document`, `pyramid-summary`, and
  `validate-product`. A person can start work without a conversation agent.
- Fields, defaults, optional overrides, pending submission, and validation
  issues work through the visible remote form. Successful submission opens the
  correct project/run page, whose identity is in its route path.
- Each corresponding CLI command reaches the same remote and prints its
  admitted run ID. `--follow` waits for that run and reports terminal failure
  or cancellation with an unsuccessful exit. Without `--follow`, the client
  exits while its accepted run continues.
- A conversation agent's ordinary CLI invocation still associates its run with
  the owning conversation and worktree. The association and terminal status
  survive reopening the conversation.

Evidence: inspect all five rendered forms and CLI help; demonstrate a real human
browser launch and a separate CLI-process launch through different built-ins.
Observe their live and terminal pages. Launch from a real conversation and
reopen it to verify the recorded association rather than trusting agent prose.

## One host, first-use projects, and retained lifecycle

- Starting a workflow for a valid project unknown at startup admits that
  project automatically. No preliminary registration command or server restart
  is necessary. Concurrent first requests for the same canonical project reuse
  one project owner; an ordinary path alias does not create another owner.
- One serving PID owns overlapping runs in different projects. Project A's
  pages and controls cannot address project B's runs. Execution workdirs remain
  distinct from the owning projects' durable storage locations.
- Closing the browser, exiting the launch CLI, or stopping follow leaves the
  accepted run active. Explicit cancellation affects the intended run. Existing
  failure recording, hosted panic containment, coordinated shutdown, project
  ownership release, and history after restart remain working.
- The same generated remote/client path works with `--no-web`. An absent host
  or unavailable remote produces an actionable error; no second runtime owner
  or local execution fallback is started. Explicitly selected independent
  instances remain usable for separate projects and tests.

Evidence: start one built instance with project A, then launch into previously
unknown B through a separate CLI and through a browser start form as applicable.
Observe overlapping work, the single serving PID, correct project pages, and
continued work after the initiating client leaves. Reopen completed histories
after restart. Use existing ownership/lifecycle tests for deterministic alias,
concurrent-admission, failure, and panic cases, extended for the new entry path.
Repeat a representative launch headlessly through the generated Go client.

## Delivery and evidence quality

Use cheap live model bindings: Codex `gpt-5.6-luna`, Claude Haiku, or Gemini
flash. Record which model each observation used. Choose small valid inputs and
isolated projects; do not operate on unrelated active runs to prove lifecycle
behavior.

SKGO's relevant checks and Gimbal's required build, test, vet, formatting, and
browser checks pass. In Gimbal these are `just build`, `just test`, `just vet`,
`just fmt-check`, and `just e2e`. Inspect generated output and the actual
rendered CLI help. These are correctness gates separate from behavioral proof.

An independent validator confirms that the observations establish the claims
above. The implementation report identifies the tested source revision,
observed outcomes, and any unmet requirement. A mocked endpoint or aggregate
green gate does not establish shared remote operation or single-process live
ownership.

Update the affected authoring instructions and web documentation. Record the
release containing the required SKGO support. When the implementation is
merged and installed under the normal Gimbal delivery workflow, report that
state separately from local validation and verify the installed CLI/page path.

Repeatable checks live beside the code they exercise. No committed proof
programs, run logs, screenshots, or transcript dumps are required; describe
what was observed in chat or the PR.
