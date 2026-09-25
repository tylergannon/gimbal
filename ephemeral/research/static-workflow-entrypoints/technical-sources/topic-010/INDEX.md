# Topic 010 Index: Testing Matrix, Behavioral Proof, and Cross-Repository Sequencing

Routes assigned research questions for Topic 010 to primary local evidence, annotated excerpts, and architectural findings.

## Local Sources

- [gimbal-test-matrix-and-e2e.md](./sources/gimbal-test-matrix-and-e2e.md): Existing test runners (`Justfile`), Playwright BDD browser test harness (`e2e/playwright.config.ts`), and comparison of legacy in-process direct calls vs real client protocol tests.
- [behavioral-proof-and-concurrency.md](./sources/behavioral-proof-and-concurrency.md): Standards in `AGENTS.md` and `ephemeral/research/static-workflow-entrypoints/definition-of-done.md` for live model observation, multi-project concurrency, client disconnect survival, and artifact rules.
- [cross-repo-sequencing-and-pinning.md](./sources/cross-repo-sequencing-and-pinning.md): Module dependencies in `go.mod`, SKGO client generation, and delivery rules in `skills/gimbal-release/SKILL.md`.
- [test-matrix-clip.md](./clips/test-matrix-clip.md): Structured test layer matrix spanning SKGO unit fixtures, Gimbal package HTTP/UDS tests, and Playwright E2E.

---

## Question Routing and Local Evidence

### Q1: How should existing tests in Gimbal (just test, just e2e) be updated or extended to exercise both browser and Go client paths against the same remote handler without mocks?

- **Supported Facts**:
  - The definition of done establishes: *"A direct call to a shared Go function alone does not prove the client/remote path"* ([`gimbal-test-matrix-and-e2e.md:3`](./sources/gimbal-test-matrix-and-e2e.md#3-direct-function-calls-vs-client-protocol-proof)).
  - Existing tests like `web/interview_test.go` directly invoked `routes.Skgo_answerInterview` in-process, bypassing the HTTP wire protocol, header validation, and error envelopes ([`gimbal-test-matrix-and-e2e.md:3`](./sources/gimbal-test-matrix-and-e2e.md#3-direct-function-calls-vs-client-protocol-proof)).
  - Existing tests like `web/submit_test.go` exercised `/control/submit` with generic `Submission{}` payloads, which is slated for deletion ([`gimbal-test-matrix-and-e2e.md:4`](./sources/gimbal-test-matrix-and-e2e.md#4-retaining-run-ownership-beyond-client-lifecycle)).
  - Playwright E2E (`e2e/playwright.config.ts`) runs against the real embedded production binary `bin/gimbal --port 0` without mocks ([`gimbal-test-matrix-and-e2e.md:2`](./sources/gimbal-test-matrix-and-e2e.md#2-real-binary-web-server-in-playwright-e2e)).
- **Inference & Recommendation**:
  - **`just test` (Package tests in `web/` and `cmd/gimbal`)**:
    - Replace `submit_test.go` with package-level tests that instantiate the real handler via `web.NewHandler` and exercise the generated Go client over HTTP loopback / UDS against concrete start remotes (e.g. `startReview`).
    - Test invalid required inputs and invalid role models to verify rejected requests return error envelopes before any run is created on disk.
    - Test client context cancellation during network wait to prove the server retains ownership of the accepted run.
  - **`just e2e` (Playwright BDD in `e2e/`)**:
    - Add a BDD feature scenario (e.g. `start-workflow.feature`) to test the human visible start form: navigate to `/start/review`, fill in target project and parameters, observe pending state, submit, and verify automated navigation to the admitted run page `/projects/[project]/runs/[runID]`.
- **Contradictions Identified**:
  - Direct in-memory function calls (like `routes.Skgo_...`) do not satisfy DoD for client/remote verification; real HTTP/UDS round trips through the generated client are required.

---

### Q2: How can behavioral proof be structured to verify multi-project concurrency, client disconnect survival, and invalid input rejection using cheap live models (Codex gpt-5.6-luna, Claude Haiku, Gemini Flash)?

- **Supported Facts**:
  - `AGENTS.md` and DoD strictly forbid committing proof programs, run logs, or transcript dumps: *"Proof is running the real thing yourself and saying what you saw, in the chat or the PR description"* ([`behavioral-proof-and-concurrency.md:1`](./sources/behavioral-proof-and-concurrency.md#1-owners-rules-on-proof-and-artifacts-in-agentsmd)).
  - Live proof must use the cheapest model tier: Codex `gpt-5.6-luna`, Claude Haiku, or Gemini Flash, explicitly naming the model used ([`behavioral-proof-and-concurrency.md:1`](./sources/behavioral-proof-and-concurrency.md#1-owners-rules-on-proof-and-artifacts-in-agentsmd)).
  - DoD specifies multi-project concurrency: a single serving PID must admit an unknown project directory on first use and handle concurrent runs across project A and project B without cross-talk ([`behavioral-proof-and-concurrency.md:2`](./sources/behavioral-proof-and-concurrency.md#2-multi-project-concurrency-and-host-lifecycle-requirements)).
  - DoD specifies client disconnect survival: closing the browser tab or killing the CLI process without `--follow` leaves the admitted run executing under host ownership ([`behavioral-proof-and-concurrency.md:2`](./sources/behavioral-proof-and-concurrency.md#2-multi-project-concurrency-and-host-lifecycle-requirements)).
- **Inference & Recommendation**:
  - **Verification Scenario 1: Multi-Project Concurrency**:
    1. Start a single server instance `bin/gimbal --port 8080` (PID $PID).
    2. Admitting Project A via CLI `gimbal review --target HEAD~1` (binding `code-review` to `gpt-5.6-luna`).
    3. Concurrently admit previously unknown Project B via visible browser start form at `http://127.0.0.1:8080/start/implement`.
    4. Confirm single PID $PID serves both, active runs appear in their respective `.gimbal/runs/` directories, and Project A's interface cannot access Project B's runs.
  - **Verification Scenario 2: Client Disconnect Survival**:
    1. Start a workflow via CLI without `--follow`, or start via browser form and immediately close the tab.
    2. Inspect process table and run registry to verify the run continues executing.
    3. Reopen browser or attach with `gimbal follow <runID>` to observe terminal completion.
  - **Verification Scenario 3: Input Rejection**:
    1. Submit a start request with missing required fields or invalid model name.
    2. Observe immediate HTTP 400 / field issues returned; confirm no run directory or ID was created under `.gimbal/runs/`.
- **Unresolved / Decision Points**:
  - Exact model name alias configuration for test runs (verifying `cmd/gimbal/defaults.json` maps each role to the cheap tier during test invocations).

---

### Q3: What exact cross-repository workflow (local branch testing, SKGO release tagging, Gimbal module pinning, and post-install validation) ensures delivery without breaking CI or leaving local replacements?

- **Supported Facts**:
  - Gimbal uses Go modules with a strict pinning policy: `github.com/tylergannon/skgo v0.5.0` in `go.mod` ([`cross-repo-sequencing-and-pinning.md:1`](./sources/cross-repo-sequencing-and-pinning.md#1-pinned-dependency-structure-in-gomod)).
  - SKGO is also used as a Go tool dependency: `tool github.com/tylergannon/skgo/cmd/skgo`, run via `go generate ./...` ([`cross-repo-sequencing-and-pinning.md:1`](./sources/cross-repo-sequencing-and-pinning.md#1-pinned-dependency-structure-in-gomod)).
  - Plan and DoD rule: *"Land and publish the needed SKGO change, then pin that version in Gimbal; leave no local module replacement in the delivered build"* ([`cross-repo-sequencing-and-pinning.md:2`](./sources/cross-repo-sequencing-and-pinning.md#2-cross-repository-sequencing-rules)).
  - `skills/gimbal-release/SKILL.md` requires clean checkout build with `just build` and `go install ./cmd/gimbal` without local replacements ([`cross-repo-sequencing-and-pinning.md:3`](./sources/cross-repo-sequencing-and-pinning.md#3-merge-installation-and-clean-build-standards)).
- **Inference & Recommendation**:
  - **Phase 1 (SKGO Development & Verification)**:
    - In SKGO worktree (`/Users/tyler/.codex/worktrees/d798/skgo`), implement Go client generation in `internal/gen/` and fix form data decoding in `remote_form.go` / `internal/formdata/`.
    - Run SKGO test suite (`go test -count=1 ./...`) against pinned SvelteKit binary fixtures.
  - **Phase 2 (Local Integration in Gimbal Worktree)**:
    - Temporarily add `replace github.com/tylergannon/skgo => ../../skgo` in Gimbal's `go.mod` for local integration and generator debugging (`go generate ./...`, `just test`, `just e2e`).
  - **Phase 3 (SKGO Release Tagging)**:
    - Land SKGO changes on its main branch, tag semantic release (e.g. `v0.6.0`), and push to GitHub.
  - **Phase 4 (Gimbal Module Pinning & Verification)**:
    - Remove the `replace` directive from Gimbal's `go.mod`.
    - Update module dependency: `go get github.com/tylergannon/skgo@v0.6.0` and update `tool github.com/tylergannon/skgo/cmd/skgo`.
    - Run `go mod tidy` and verify clean build: `just build`, `just test`, `just vet`, `just fmt-check`, `just e2e`.
    - Verify generation produces no diff on a clean second run.
  - **Phase 5 (Post-Install Validation)**:
    - Install CLI locally: `go install ./cmd/gimbal`.
    - Reinstall skills: `skills add ...` per `skills/gimbal-release/SKILL.md`.
    - Run `gimbal --help` and verify installed binary serves web app and executes workflow remotes.
- **Unresolved / Decision Points**:
  - Coordination of exact SKGO release version number with upstream maintainer (Tyler) prior to tagging.
