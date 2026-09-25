# Technical Sources Semantic Index: Static Workflow Entrypoints

Authoritative semantic routing tree over the technical research evidence for static workflow entrypoints.
This index directs authors and implementing agents to primary source excerpts, analytical clips, and topic-specific findings without repeating detailed evidence at the root.

---

## 1. Corpus Scope and Source Locations

The evidence cache materializes primary sources and analytical clips gathered across isolated worktrees and pinned upstream references:

- **Gimbal Worktree** (`/Users/tyler/.codex/worktrees/d798/gimbal`, commit `e161721c`): Host runtime (`web/runtime.go`, `web/control.go`), server assembly (`web/server.go`), code generators (`internal/generate/`), built-in workflows (`internal/workflows/`, `cmd/gimbal/defaults.json`), project context (`web/src/project.go`), test harnesses (`web/*_test.go`, `e2e/`), and UI idioms (`ephemeral/research/svelte-idioms/`).
- **SKGO Worktree** (`/Users/tyler/.codex/worktrees/d798/skgo`, commit `fed929b`): Form decoding (`internal/formdata/decode.go`), remote form runtime (`remote_form.go`, `remote.go`), code generation (`internal/gen/emit.go`, `codecs.go`), and git history (`v0.5.0` to `fed929b`).
- **Pinned SvelteKit Reference** (`/Users/tyler/src/skgo/ephemeral/inspiration/reference/kit@3.0.0-next.27/`): Client-side form runtime (`form.svelte.js`, `form-utils.js`, `shared.svelte.js`) and server remote functions (`server/remote.js`).
- **Pinned Module Cache** (`/Users/tyler/go/pkg/mod/github.com/tylergannon/`): Pinned Polytype sources (`polytype@v1.0.3`, `v1.1.0`) covering `polytype.Optional[T]`, `devalue.Uneval`, and scalar validation codecs.

### Cache Directory Layout

The research cache contains 10 topic directories (`topic-001` through `topic-010`). Each topic houses:
- `INDEX.md`: Local question index, supported facts, inferences, and resolved boundaries.
- `clips/`: Synthesized technical clips focusing on key architectural decisions and code excerpts.
- `sources/`: Verbatim primary source extracts with exact line number references.

---

## 2. Citation Conventions and Link Resolution

All route destinations use relative POSIX file links resolving within this research tree:
- **Topic Index**: `[Topic 001 Index](topic-001/INDEX.md#anchor)`
- **Analytical Clip**: `[Host Package Boundary](topic-001/clips/host-package-boundary.md)`
- **Source Excerpt**: `[web-runtime.go:34-70](topic-001/sources/web-runtime.go.txt)`

Citations in leaf files include line spans matching the original repository source files.

---

## 3. Author Question Routes

Follow these routes based on the specific implementation questions encountered when drafting `technical-handoff.md` or executing the architecture.

### Package Architecture & Static Generation Pipeline

#### Route 1: How to eliminate cyclic dependencies between workflows, host runtime, and web assembly?
- **When to follow**: When specifying the package layout, defining `internal/host`, or deciding where `Instance`, `Runtime`, and context keys relocate.
- **Destinations**:
  - Routing Analysis: [Topic 001 Index — Boundary Resolution](topic-001/INDEX.md#1-concrete-types-and-lifecycle-functions-moving-to-host-package)
  - Architectural Clip: [Host Package Boundary & Import Decoupling](topic-001/clips/host-package-boundary.md)
  - Key Sources: [web-runtime.go:34-70, 222-283](topic-001/sources/web-runtime.go.txt), [web-server.go:27-127](topic-001/sources/web-server.go.txt), [web-src-project.go:5-35](topic-001/sources/web-src-project.go.txt)

#### Route 2: How to structure code generation so clean checkouts build without missing symbols or cyclic scanning?
- **When to follow**: When redesigning `internal/generate/command.go` and `source.go`, stripping web/CLI imports from workflows, and ordering generation phases.
- **Destinations**:
  - Routing Analysis: [Topic 002 Index — Generation Pipeline](topic-002/INDEX.md#1-generation-changes-in-commandgo-and-sourcego)
  - Architectural Clip: [Codegen Bootstrapping & Multi-Phase Pipeline](topic-002/clips/codegen-bootstrapping.md)
  - Key Sources: [internal-generate-command.go:96-220](topic-002/sources/internal-generate-command.go.txt), [internal-generate-source.go:25-58](topic-002/sources/internal-generate-source.go.txt)

#### Route 3: Where should the static built-in workflow inventory live, and how are standalone examples decoupled?
- **When to follow**: When creating the single source of truth for the 5 stock built-ins (`review`, `implement`, `validate-product`, `research-document`, `pyramid-summary`) and decoupling `cmd/examples/`.
- **Destinations**:
  - Routing Analysis: [Topic 002 Index — Built-in Inventory & Examples](topic-002/INDEX.md#2-defining-the-static-list-of-the-five-stock-built-ins)
  - Architectural Clip: [Built-in Inventory and Example Runner Decoupling](topic-002/clips/codegen-bootstrapping.md#3-static-inventory-definition-and-example-decoupling)
  - Key Sources: [cmd-gimbal-workflows.go:20-52](topic-002/sources/cmd-gimbal-workflows.go.txt), [cmd-examples-interview.go.txt](topic-002/sources/cmd-examples-interview.go.txt)

---

### SKGO Form Wire Protocol and Go Client Contract

#### Route 4: What exact request framing and response envelopes flow over the wire for SKGO forms?
- **When to follow**: When implementing or testing SvelteKit enhanced binary form serialization, 7-byte prologue framing, devalue payload headers, and HTTP 200 response JSON schemas.
- **Destinations**:
  - Routing Analysis: [Topic 003 Index — Wire Protocol](topic-003/INDEX.md#1-request-format-headers-and-binary-encoding)
  - Architectural Clip: [SvelteKit & SKGO Wire Envelopes](topic-003/clips/wire-envelopes.md)
  - Key Sources: [sveltekit-client-form-protocol.txt](topic-003/sources/sveltekit-client-form-protocol.txt), [skgo-remote-form.txt](topic-003/sources/skgo-remote-form.txt), [skgo-formdata.txt](topic-003/sources/skgo-formdata.txt)

#### Route 5: How are validation-only requests detected, and how does the server prevent workflow execution on keystrokes?
- **When to follow**: When verifying that live form validation in the browser UI (`form.validate()`) does not trigger asynchronous workflow runs.
- **Destinations**:
  - Routing Analysis: [Topic 003 Index — Validation-Only Requests](topic-003/INDEX.md#3-validation-only-requests-and-prevention-of-workflow-execution)
  - Architectural Clip: [Validation-Only Request Guard](topic-003/clips/wire-envelopes.md#3-validation-only-requests-and-side-effect-suppression)
  - Key Sources: [skgo-remote-form.txt:176-179](topic-003/sources/skgo-remote-form.txt), [sveltekit-client-form-protocol.txt](topic-003/sources/sveltekit-client-form-protocol.txt)

#### Route 6: How should the typed Go client be generated, transported over UDS/HTTP, and handle cancellation?
- **When to follow**: When implementing SKGO Go client generation in `internal/gen`, configuring Unix domain socket dialers, and ensuring network wait cancellation does not kill server runs.
- **Destinations**:
  - Routing Analysis: [Topic 004 Index — Go Client Architecture](topic-004/INDEX.md#1-client-api-signatures-type-declarations-and-package-structure)
  - Architectural Clip: [Client Transport & Cancellation Contract](topic-004/clips/client-transport-and-cancellation.md)
  - Key Sources: [gimbal-web-submit-and-control.txt](topic-004/sources/gimbal-web-submit-and-control.txt), [gimbal-implementation-plan-contract.txt](topic-004/sources/gimbal-implementation-plan-contract.txt)

#### Route 7: What differences exist between SKGO v0.5.0 and fed929b, and what release baseline is required?
- **When to follow**: When checking what SKGO PR #152 changed (`internal/devalue` deletion) and determining the release tag needed before Gimbal can pin it.
- **Destinations**:
  - Routing Analysis: [Topic 004 Index — SKGO Baseline Delta](topic-004/INDEX.md#4-skgo-baseline-comparison-v050-vs-fed929b)
  - Architectural Clip: [SKGO Version Delta & Release Gate](topic-004/clips/client-transport-and-cancellation.md#4-skgo-baseline-evolution-and-minimum-release-baseline)
  - Key Sources: [skgo-git-diff-v0.5.0-fed929b.txt](topic-004/sources/skgo-git-diff-v0.5.0-fed929b.txt), [skgo-gen-emit-and-codecs.txt](topic-004/sources/skgo-gen-emit-and-codecs.txt)

---

### Polytype Codecs and Form Data Binding

#### Route 8: Where does form decoding bypass Polytype codecs, and how must Optional[T] and scalars be supported?
- **When to follow**: When patching SKGO's `internal/formdata/decode.go` to support `polytype.Optional[T]` and distinguishing omitted fields from explicit zero/false/empty values.
- **Destinations**:
  - Routing Analysis: [Topic 005 Index — Codec Bypass & Optional[T]](topic-005/INDEX.md#1-where-does-skgo-form-decoding-bypass-polytype-codecs-and-how-must-it-be-modified-to-delegate-scalar-values-to-polytype)
  - Architectural Clip: [Form Scalar & Optional[T] Delegation](topic-005/clips/form-scalar-optional-delegation.md)
  - Key Sources: [skgo-form-decoding-source.txt:84-145, 173-228](topic-005/sources/skgo-form-decoding-source.txt), [polytype-optional-and-codecs-source.txt:14-22, 174-213](topic-005/sources/polytype-optional-and-codecs-source.txt)

#### Route 9: How do nested struct paths and parameter shapes map to Gimbal workflow inputs?
- **When to follow**: When verifying parameter shapes across the five built-in workflows and determining how SvelteKit's dotted field paths (`params.goal`) bind to Go structs.
- **Destinations**:
  - Routing Analysis: [Topic 005 Index — Nested Parameter Shapes](topic-005/INDEX.md#3-how-does-polytype-handle-nested-struct-field-paths-in-standard-form-data-submissions-and-what-conventions-match-gimbals-workflow-parameter-shapes)
  - Architectural Clip: [Nested Structs and Parameter Shapes](topic-005/clips/nested-structs-parameter-shapes.md)
  - Key Sources: [sveltekit-form-utils-reference.txt:16-18, 83-113](topic-005/sources/sveltekit-form-utils-reference.txt), [skgo-form-decoding-source.txt:280-314](topic-005/sources/skgo-form-decoding-source.txt)

#### Route 10: What are the 15 built-in roles, and how are overrides defined, defaulted, and validated on admission?
- **When to follow**: When designing generated request structs with `Role<Name> polytype.Optional[string]` fields, auditing `cmd/gimbal/defaults.json`, and resolving models via `binding.Roles`.
- **Destinations**:
  - Routing Analysis: [Topic 006 Index — Role Model Schema & Validation](topic-006/INDEX.md#1-what-model-roles-are-declared-across-the-five-built-in-workflows-and-how-are-their-default-values-structured-in-cmdgimbaldefaultsjson)
  - Architectural Clip: [Built-in Role Inventory & Defaults Schema](topic-006/clips/built-in-role-inventory-and-defaults.md)
  - Architectural Clip: [Server-Side Model Resolution & Fast Failure](topic-006/clips/server-side-model-resolution.md)
  - Key Sources: [gimbal-defaults-and-workflow-roles.txt](topic-006/sources/gimbal-defaults-and-workflow-roles.txt), [gimbal-model-resolution-and-binding.txt](topic-006/sources/gimbal-model-resolution-and-binding.txt)

---

### Instance-Scoped Routing and Multi-Listener Mounting

#### Route 11: How do instance-scoped start remotes bypass project middleware and avoid browser Referer requirements?
- **When to follow**: When updating `projectRequest` middleware in `web/runtime.go` to admit unknown projects without failing on missing `Referer` headers.
- **Destinations**:
  - Routing Analysis: [Topic 007 Index — Middleware Dispatch](topic-007/INDEX.md#question-1-how-are-requests-currently-dispatched-and-filtered-by-project-middleware-in-web-and-webservergo-and-how-must-instance-scoped-start-remotes-be-mounted-to-bypass-project-scoping-filters)
  - Architectural Clip: [Project Middleware Bypass for Instance Start Remotes](topic-007/clips/clip-project-middleware-bypass.md)
  - Key Sources: [source-001-web-runtime-middleware.md:366-411](topic-007/sources/source-001-web-runtime-middleware.md), [source-002-web-server-assembly.md:125-126](topic-007/sources/source-002-web-server-assembly.md)

#### Route 12: How are CSRF, Origin, and control socket permissions configured across web and UDS listeners?
- **When to follow**: When ensuring Go CLI clients connecting over the control UDS succeed with empty `Origin`, while browser traffic retains origin verification.
- **Destinations**:
  - Routing Analysis: [Topic 007 Index — Origin and CSRF Policies](topic-007/INDEX.md#question-2-what-csrf-origin-or-referer-policies-does-skgo-enforce-on-remote-forms-and-how-can-the-go-cli-client-submit-requests-without-fabricating-browser-headers)
  - Architectural Clip: [Control UDS and Headless Assembly](topic-007/clips/clip-control-uds-and-headless-assembly.md)
  - Key Sources: [source-003-skgo-remote-csrf-origin.md:666-675](topic-007/sources/source-003-skgo-remote-csrf-origin.md), [source-004-web-submit-control-probing.md](topic-007/sources/source-004-web-submit-control-probing.md)

#### Route 13: How does headless mode (`--no-web`) mount the shared remote handler assembly without frontend assets?
- **When to follow**: When implementing headless server assembly and confirming that `skgo.NewRemotes` has zero runtime dependency on `dist fs.FS` or Vite manifests.
- **Destinations**:
  - Routing Analysis: [Topic 007 Index — Headless Multi-Listener Assembly](topic-007/INDEX.md#question-3-how-are-start-remote-handlers-mounted-on-the-control-listeneruds-in-headless-mode---no-web-using-the-shared-handler-assembly)
  - Architectural Clip: [Pure Go Remotes on Control Listener](topic-007/clips/clip-control-uds-and-headless-assembly.md#3-pure-go-remotes-on-control-listener-under---no-web)
  - Key Sources: [source-001-web-runtime-middleware.md:132-189](topic-007/sources/source-001-web-runtime-middleware.md), [source-002-web-server-assembly.md:102-126](topic-007/sources/source-002-web-server-assembly.md)

---

### Project Admission Concurrency, Locking, and Run Lifetime

#### Route 14: How does canonical path resolution and `owner.lock` guarantee single ownership per repository?
- **When to follow**: When handling relative paths, trailing slashes, symlink aliases, and verifying `unix.Flock` cross-process mutual exclusion.
- **Destinations**:
  - Routing Analysis: [Topic 008 Index — Canonical Path & Locking](topic-008/INDEX.md#question-2-how-does-canonical-project-path-resolution-handling-symlinks-relative-paths-and-trailing-slashes-operate-under-the-admission-lock-to-guarantee-a-single-project-owner-per-repository)
  - Key Sources: [source-001-runtime-admission-lifecycle.md:218-296](topic-008/sources/source-001-runtime-admission-lifecycle.md), [source-004-project-ownership-isolation-tests.md:83-127](topic-008/sources/source-004-project-ownership-isolation-tests.md)

#### Route 15: How are asynchronous workflow run contexts isolated from transient HTTP request contexts?
- **When to follow**: When structuring the goroutine boundary so client disconnection does not abort accepted runs.
- **Destinations**:
  - Routing Analysis: [Topic 008 Index — Context Hierarchy & Run Lifetime](topic-008/INDEX.md#question-1-how-must-the-context-hierarchy-in-webruntimego-and-websrcprojectgo-be-structured-so-that-completing-the-http-request-does-not-terminate-the-asynchronous-workflow-run-context)
  - Architectural Clip: [Decoupled Run Context Hierarchy](topic-008/clips/clip-context-hierarchy.md)
  - Key Sources: [source-001-runtime-admission-lifecycle.md:417-471](topic-008/sources/source-001-runtime-admission-lifecycle.md), [source-002-control-submission-async-run.md:119-158](topic-008/sources/source-002-control-submission-async-run.md)

#### Route 16: How does a single PID track concurrent runs across projects while maintaining endpoint isolation?
- **When to follow**: When auditing `Instance.activeRuns sync.WaitGroup` vs project-isolated `.gimbal` directories, live run tables, and control routes.
- **Destinations**:
  - Routing Analysis: [Topic 008 Index — Concurrency Tracking & Isolation](topic-008/INDEX.md#question-3-how-does-the-host-track-concurrent-active-runs-across-multiple-admitted-projects-under-a-single-pid-while-ensuring-isolation-of-project-scoped-observation-and-control-endpoints)
  - Key Sources: [source-001-runtime-admission-lifecycle.md:57-70](topic-008/sources/source-001-runtime-admission-lifecycle.md), [source-002-control-submission-async-run.md:335-361](topic-008/sources/source-002-control-submission-async-run.md), [source-004-project-ownership-isolation-tests.md:46-60](topic-008/sources/source-004-project-ownership-isolation-tests.md)

---

### SvelteKit Start Forms and UI Conventions

#### Route 17: Where in the SvelteKit route hierarchy should start forms be mounted?
- **When to follow**: When setting up the route paths (`/start/[workflow]` vs `/projects/[project]/start/[workflow]`) conforming to path-based identity.
- **Destinations**:
  - Routing Analysis: [Topic 009 Index — Route Hierarchy](topic-009/INDEX.md#q1-where-in-the-sveltekit-route-hierarchy-websrcroutes-should-the-start-forms-for-the-five-built-ins-be-mounted-to-align-with-existing-page-and-path-conventions)
  - Key Sources: [gimbal-route-hierarchy-and-middleware.md](topic-009/sources/gimbal-route-hierarchy-and-middleware.md), [gimbal-ui-conventions-and-dos-donts.md](topic-009/sources/gimbal-ui-conventions-and-dos-donts.md)

#### Route 18: How do start forms bind to skgo.Form, shadcn inputs, pending states, and field issues?
- **When to follow**: When implementing SvelteKit 5 forms without hidden proxy forms (Rule #10), using `form.fields.<name>.as('text')` and `aria-invalid`.
- **Destinations**:
  - Routing Analysis: [Topic 009 Index — Form Bindings & shadcn](topic-009/INDEX.md#q2-how-do-existing-forms-in-websrc-integrate-with-skgoform-bindings-to-display-pending-states-field-specific-validation-errors-and-server-failure-envelopes)
  - Architectural Clip: [Idiomatic SvelteKit Start Form Binding](topic-009/clips/skgo-form-binding-clip.md)
  - Key Sources: [sveltekit-remote-form-runtime.md](topic-009/sources/sveltekit-remote-form-runtime.md), [gimbal-ui-conventions-and-dos-donts.md](topic-009/sources/gimbal-ui-conventions-and-dos-donts.md)

#### Route 19: How should the UI present project directory selection and navigate upon run admission?
- **When to follow**: When building the hybrid project selector (admitted dropdown vs custom input path) and wiring `goto()` to `/projects/[project]/runs/[runID]`.
- **Destinations**:
  - Routing Analysis: [Topic 009 Index — Directory Selection & Navigation](topic-009/INDEX.md#q3-how-should-the-ui-structure-the-project-directory-input-choosing-existing-projects-vs-entering-new-paths-alongside-workflow-parameters-and-model-overrides-navigating-to-the-admitted-run-page-upon-completion)
  - Key Sources: [gimbal-route-hierarchy-and-middleware.md:4](topic-009/sources/gimbal-route-hierarchy-and-middleware.md), [gimbal-ui-conventions-and-dos-donts.md](topic-009/sources/gimbal-ui-conventions-and-dos-donts.md)

---

### Verification, Proof, and Cross-Repository Sequencing

#### Route 20: How should package tests and Playwright E2E test real HTTP/UDS round trips without mocks?
- **When to follow**: When upgrading `just test` (package tests using generated client over HTTP/UDS) and adding `just e2e` scenarios for human start forms.
- **Destinations**:
  - Routing Analysis: [Topic 010 Index — Test Matrix](topic-010/INDEX.md#q1-how-should-existing-tests-in-gimbal-just-test-just-e2e-be-updated-or-extended-to-exercise-both-browser-and-go-client-paths-against-the-same-remote-handler-without-mocks)
  - Architectural Clip: [Comprehensive Non-Mock Test Matrix](topic-010/clips/test-matrix-clip.md)
  - Key Sources: [gimbal-test-matrix-and-e2e.md](topic-010/sources/gimbal-test-matrix-and-e2e.md)

#### Route 21: How to perform live behavioral proof with cheap models and satisfy proof reporting rules?
- **When to follow**: When running manual multi-project concurrency, disconnect survival, and invalid input checks with cheap models (`gpt-5.6-luna`, Haiku, Flash).
- **Destinations**:
  - Routing Analysis: [Topic 010 Index — Behavioral Proof](topic-010/INDEX.md#q2-how-can-behavioral-proof-be-structured-to-verify-multi-project-concurrency-client-disconnect-survival-and-invalid-input-rejection-using-cheap-live-models-codex-gpt-56-luna-claude-haiku-gemini-flash)
  - Key Sources: [behavioral-proof-and-concurrency.md](topic-010/sources/behavioral-proof-and-concurrency.md)

#### Route 22: What cross-repository git and Go module steps deliver the changes cleanly?
- **When to follow**: When sequencing SKGO development, local branch integration with temporary `replace`, SKGO tagging, and Gimbal module pinning.
- **Destinations**:
  - Routing Analysis: [Topic 010 Index — Cross-Repository Sequencing](topic-010/INDEX.md#q3-what-exact-cross-repository-workflow-local-branch-testing-skgo-release-tagging-gimbal-module-pinning-and-post-install-validation-ensures-delivery-without-breaking-ci-or-leaving-local-replacements)
  - Key Sources: [cross-repo-sequencing-and-pinning.md](topic-010/sources/cross-repo-sequencing-and-pinning.md)

---

## 4. Cross-Cutting Thematic Routes

These themes span multiple research areas and link together intersecting design decisions.

### Theme A: Decoupled Lifecycle — Transient Request vs Long-Lived Run
Workflow runs are long-lived tasks that must survive client disconnects, network timeouts, and browser navigation.
- **Host runtime context separation**: [Topic 001 Index](topic-001/INDEX.md), [Topic 008 Clip](topic-008/clips/clip-context-hierarchy.md)
- **Client cancellation non-propagation**: [Topic 004 Clip](topic-004/clips/client-transport-and-cancellation.md#3-caller-context-cancellation-and-server-run-lifetime)
- **Asynchronous start channel synchronization**: [Topic 008 Source 002](topic-008/sources/source-002-control-submission-async-run.md)
- **Disconnect survival behavioral proof**: [Topic 010 Source](topic-010/sources/behavioral-proof-and-concurrency.md#2-multi-project-concurrency-and-host-lifecycle-requirements)

### Theme B: Wire Protocol Framing & Error Envelopes
Both browser forms and Go CLI clients share the exact same binary wire protocol and HTTP 200 JSON envelope conventions.
- **SvelteKit binary form serialization & response schemas**: [Topic 003 Clip](topic-003/clips/wire-envelopes.md)
- **Typed client response mapping (`*skgo.Invalid` vs `*skgo.HTTPError`)**: [Topic 004 Clip](topic-004/clips/client-transport-and-cancellation.md#1-client-api-signatures-and-error-unmarshaling)
- **Reflection decoding of `polytype.Optional[T]` and omission preservation**: [Topic 005 Clip](topic-005/clips/form-scalar-optional-delegation.md)
- **Client-side reactive issues and shadcn `aria-invalid`**: [Topic 009 Clip](topic-009/clips/skgo-form-binding-clip.md)

### Theme C: Instance-Level Admission vs Project-Level Scoping
Gimbal operates as an instance daemon admitting arbitrary repositories on first use.
- **Separation of instance vs project runtime state**: [Topic 001 Clip](topic-001/clips/host-package-boundary.md)
- **Instance-scoped start routing & middleware bypass**: [Topic 007 Clip](topic-007/clips/clip-project-middleware-bypass.md)
- **Canonical project path deduplication & cross-process locking**: [Topic 008 Source 001](topic-008/sources/source-001-runtime-admission-lifecycle.md)
- **UI hybrid project directory selector**: [Topic 009 Index](topic-009/INDEX.md#q3-how-should-the-ui-structure-the-project-directory-input-choosing-existing-projects-vs-entering-new-paths-alongside-workflow-parameters-and-model-overrides-navigating-to-the-admitted-run-page-upon-completion)

### Theme D: Static Build-Time Generation vs Dynamic Reflection
Gimbal eliminates runtime registries and generic submission maps in favor of statically checked code generation.
- **Removal of `web` imports from workflow packages**: [Topic 002 Clip](topic-002/clips/codegen-bootstrapping.md#1-workflow-package-decoupling)
- **Static built-in inventory vs dynamic discovery**: [Topic 002 Clip](topic-002/clips/codegen-bootstrapping.md#3-static-inventory-definition-and-example-decoupling)
- **Generated typed Go client functions**: [Topic 004 Clip](topic-004/clips/client-transport-and-cancellation.md#1-client-api-signatures-and-error-unmarshaling)
- **Statically generated role override fields**: [Topic 006 Clip](topic-006/clips/built-in-role-inventory-and-defaults.md#2-generated-request-struct-representation)

### Theme E: Admission-Time Validation & Fast Failure
Invalid requests must fail before any disk state, run ID, or goroutine is created.
- **Validation-only request side-effect suppression**: [Topic 003 Clip](topic-003/clips/wire-envelopes.md#3-validation-only-requests-and-side-effect-suppression)
- **Scalar bounds & enum checking in form decoder**: [Topic 005 Clip](topic-005/clips/form-scalar-optional-delegation.md#2-handling-optionalt-and-omission-distinction)
- **Provider harness & model alias pre-validation**: [Topic 006 Clip](topic-006/clips/server-side-model-resolution.md#2-validation-prior-to-run-creation)
- **Negative testing of rejected inputs**: [Topic 010 Clip](topic-010/clips/test-matrix-clip.md)

---

## 5. Known Gaps, Conflicts, and Implementation Guardrails

The following historical contradictions and implementation gaps must be guarded against:

1. **Superseded REST Architecture**: `assessment.md` originally recommended a dedicated JSON REST launch endpoint on the control socket. This was **superseded** by `implementation-plan.md`. The agreed architecture mounts the exact same generated SKGO Form start remotes on both the browser web listener and the control UDS.
2. **SvelteKit Hidden Proxy Forms Anti-Pattern**: Existing codebase code in `+page.svelte` used `<form aria-hidden="true">` proxy forms with manual `.fields.set()` calls. This directly violates Tyler's Rule #10 (`ephemeral/research/svelte-idioms/dos-and-donts.md`). All new start forms must use real `<form {...formRemote}>` bindings.
3. **SKGO Form Decoding Gap**: Pinned SKGO `internal/formdata/decode.go` does not decode `polytype.Optional[T]`. This must be patched in SKGO to inspect struct types and check parsed key presence.
4. **Missing SKGO Go Client Generator**: SKGO at `fed929b` deleted `internal/devalue` but does not yet generate typed Go Form client callers. This capability must be added in SKGO before tagging.
5. **Decoupling Standalone Examples**: Standalone examples in `cmd/examples/` previously invoked generated commands that depended on stock host admission, causing runtime submission errors. They must be decoupled to invoke `gimbal.Run` directly.
6. **No Local Module Replacements in Delivery**: The Definition of Done strictly forbids leaving `replace github.com/tylergannon/skgo => ...` in `go.mod`. The required SKGO changes must be landed and tagged upstream first.

---

## 6. Representative Route Walks & Verification

These representative walks verify that an author can trace from an implementation question to concrete source evidence in ≤ 3 hops:

- **Walk 1: Go CLI Start Invocation via UDS to Detached Server Execution**
  1. Entrypoint: [Route 6: Go Client Architecture](#route-6-how-should-the-typed-go-client-be-generated-transported-over-udshttp-and-handle-cancellation)
  2. Intermediate Node: [Topic 004 Index](topic-004/INDEX.md#2-http-and-unix-domain-socket-uds-transport-configuration) & [Client Transport Clip](topic-004/clips/client-transport-and-cancellation.md)
  3. Evidence Passage: [gimbal-web-submit-and-control.txt](topic-004/sources/gimbal-web-submit-and-control.txt) (lines 129-141: UDS `http.Transport` dialing).
  4. Cross-cutting jump: [Theme A: Decoupled Lifecycle](#theme-a-decoupled-lifecycle--transient-request-vs-long-lived-run) -> [Topic 008 Source 002](topic-008/sources/source-002-control-submission-async-run.md) (lines 119-158: `started` channel wait and background `p.Run(ctx)`).

- **Walk 2: Browser Form Submission with Invalid Model Override**
  1. Entrypoint: [Route 10: Model Role Schema & Validation](#route-10-what-are-the-15-built-in-roles-and-how-are-overrides-defined-defaulted-and-validated-on-admission)
  2. Intermediate Node: [Topic 006 Index](topic-006/INDEX.md#3-how-does-the-server-resolve-and-validate-model-overrides-against-provider-configurations-in-the-startup-environment-prior-to-run-creation) & [Server-Side Resolution Clip](topic-006/clips/server-side-model-resolution.md)
  3. Evidence Passage: [gimbal-model-resolution-and-binding.txt](topic-006/sources/gimbal-model-resolution-and-binding.txt) (lines 58-89: `binding.Roles` alias resolution and adapter validation).
  4. Cross-cutting jump: [Theme B: Wire Protocol Framing](#theme-b-wire-protocol-framing--error-envelopes) -> [Topic 003 Clip](topic-003/clips/wire-envelopes.md#2-response-serialization-under-devaluejson-wire-protocol) (HTTP 200 response with `{"type":"error", "error":{...}}` or field issues, creating no run on disk).

- **Walk 3: Clean-Slate Generation Pipeline Sequencing**
  1. Entrypoint: [Route 2: Static Generation Pipeline](#route-2-how-to-structure-code-generation-so-clean-checkouts-build-without-missing-symbols-or-cyclic-scanning)
  2. Intermediate Node: [Topic 002 Index](topic-002/INDEX.md#3-clean-checkout-generation-without-circular-dependencies) & [Codegen Bootstrapping Clip](topic-002/clips/codegen-bootstrapping.md)
  3. Evidence Passage: [internal-generate-source.go:25-58](topic-002/sources/internal-generate-source.go.txt) (overlay extraction) and [internal-generate-command.go:96-220](topic-002/sources/internal-generate-command.go.txt).
  4. Cross-cutting jump: [Route 22: Cross-Repo Sequencing](#route-22-what-cross-repository-git-and-go-module-steps-deliver-the-changes-cleanly) -> [Topic 010 Source](topic-010/sources/cross-repo-sequencing-and-pinning.md).

---

## 7. Topic Index Master Directory

Quick reference connecting all 10 topics to their assigned scope and local files:

| Topic | Directory | Questions Addressed | Key Leaf Files |
| --- | --- | --- | --- |
| **001: Workflow, Host, and Remote Handler Boundaries** | [`topic-001/INDEX.md`](topic-001/INDEX.md) | Concrete types to move, context key relocation, acyclic remote handler references | [host-package-boundary.md](topic-001/clips/host-package-boundary.md), [web-runtime.go.txt](topic-001/sources/web-runtime.go.txt), [web-server.go.txt](topic-001/sources/web-server.go.txt) |
| **002: Code Generation and Built-in Bootstrapping** | [`topic-002/INDEX.md`](topic-002/INDEX.md) | Generator template changes, static built-in inventory, clean generation ordering, example decoupling | [codegen-bootstrapping.md](topic-002/clips/codegen-bootstrapping.md), [internal-generate-command.go.txt](topic-002/sources/internal-generate-command.go.txt), [cmd-gimbal-workflows.go.txt](topic-002/sources/cmd-gimbal-workflows.go.txt) |
| **003: SvelteKit Enhanced Form Protocol and Wire Format** | [`topic-003/INDEX.md`](topic-003/INDEX.md) | Binary form encoding, HTTP 200 response serialization, validation-only side-effect suppression | [wire-envelopes.md](topic-003/clips/wire-envelopes.md), [sveltekit-client-form-protocol.txt](topic-003/sources/sveltekit-client-form-protocol.txt), [skgo-remote-form.txt](topic-003/sources/skgo-remote-form.txt) |
| **004: Typed Go Client Generation and Transport** | [`topic-004/INDEX.md`](topic-004/INDEX.md) | Client API signatures, UDS/HTTP transport, cancellation vs run lifetime, SKGO diffs v0.5.0 vs fed929b | [client-transport-and-cancellation.md](topic-004/clips/client-transport-and-cancellation.md), [gimbal-web-submit-and-control.txt](topic-004/sources/gimbal-web-submit-and-control.txt), [skgo-git-diff-v0.5.0-fed929b.txt](topic-004/sources/skgo-git-diff-v0.5.0-fed929b.txt) |
| **005: Scalar and Optional Binding in SKGO Form Decoding** | [`topic-005/INDEX.md`](topic-005/INDEX.md) | Codec bypass, reflection delegation, Optional[T] presence vs omission, nested parameter paths | [form-scalar-optional-delegation.md](topic-005/clips/form-scalar-optional-delegation.md), [nested-structs-parameter-shapes.md](topic-005/clips/nested-structs-parameter-shapes.md), [skgo-form-decoding-source.txt](topic-005/sources/skgo-form-decoding-source.txt) |
| **006: Workflow Role Model Schema and Server Resolution** | [`topic-006/INDEX.md`](topic-006/INDEX.md) | 15 built-in roles, defaults.json structure, override struct fields, admission model validation | [built-in-role-inventory-and-defaults.md](topic-006/clips/built-in-role-inventory-and-defaults.md), [server-side-model-resolution.md](topic-006/clips/server-side-model-resolution.md), [gimbal-defaults-and-workflow-roles.txt](topic-006/sources/gimbal-defaults-and-workflow-roles.txt) |
| **007: Instance-Scoped Routing and Multi-Listener Mounting** | [`topic-007/INDEX.md`](topic-007/INDEX.md) | Project middleware bypass, Origin/CSRF policy on UDS, headless `--no-web` pure Go remotes | [clip-project-middleware-bypass.md](topic-007/clips/clip-project-middleware-bypass.md), [clip-control-uds-and-headless-assembly.md](topic-007/clips/clip-control-uds-and-headless-assembly.md), [source-001-web-runtime-middleware.md](topic-007/sources/source-001-web-runtime-middleware.md) |
| **008: Project Admission Concurrency and Run Lifetime** | [`topic-008/INDEX.md`](topic-008/INDEX.md) | Context hierarchy, canonical path resolution, `owner.lock` flock, multi-project PID isolation | [clip-context-hierarchy.md](topic-008/clips/clip-context-hierarchy.md), [source-001-runtime-admission-lifecycle.md](topic-008/sources/source-001-runtime-admission-lifecycle.md), [source-004-project-ownership-isolation-tests.md](topic-008/sources/source-004-project-ownership-isolation-tests.md) |
| **009: SvelteKit Start Form UI Conventions** | [`topic-009/INDEX.md`](topic-009/INDEX.md) | Route mounting, Rule #10 no proxy forms, shadcn `aria-invalid`, hybrid directory selector, `goto` | [skgo-form-binding-clip.md](topic-009/clips/skgo-form-binding-clip.md), [sveltekit-remote-form-runtime.md](topic-009/sources/sveltekit-remote-form-runtime.md), [gimbal-ui-conventions-and-dos-donts.md](topic-009/sources/gimbal-ui-conventions-and-dos-donts.md) |
| **010: Testing Matrix, Behavioral Proof, and Sequencing** | [`topic-010/INDEX.md`](topic-010/INDEX.md) | Package HTTP/UDS tests, Playwright BDD, cheap live model proof rules, SKGO release and pinning | [test-matrix-clip.md](topic-010/clips/test-matrix-clip.md), [gimbal-test-matrix-and-e2e.md](topic-010/sources/gimbal-test-matrix-and-e2e.md), [cross-repo-sequencing-and-pinning.md](topic-010/sources/cross-repo-sequencing-and-pinning.md) |
