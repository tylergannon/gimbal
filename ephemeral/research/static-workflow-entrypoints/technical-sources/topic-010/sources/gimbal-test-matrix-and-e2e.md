# Gimbal Test Matrix, E2E Harness, and Package Test Suites

- **Origin**: `/Users/tyler/.codex/worktrees/d798/gimbal`
- **Files**:
  - `Justfile`
  - `e2e/playwright.config.ts`
  - `web/interview_test.go`
  - `web/submit_test.go`
  - `ephemeral/research/static-workflow-entrypoints/definition-of-done.md`
- **Commit/Baseline**: `main` / `e161721c`
- **Retrieval Date**: 2026-09-23

---

## 1. Test Targets in `Justfile`

From `Justfile` (lines 19–32):

```make
e2e run="run":
    cd e2e && pnpm install
    cd e2e && pnpm exec playwright install chromium
    cd e2e && SKGO_E2E_RUN={{run}} pnpm test

vet:
    go vet ./...
    go build -o bin/gimbal ./cmd/gimbal
    ./bin/gimbal lint ./...

test:
    go test -count=1 ./...
    cd web && pnpm test
```

---

## 2. Real Binary Web Server in Playwright E2E

From `e2e/playwright.config.ts` (lines 23–35):

```typescript
webServer: process.env.BASE_URL
    ? undefined
    : {
            command: '../../../bin/gimbal --port 0',
            cwd: fileURLToPath(new URL('fixtures/project', import.meta.url)),
            env: { GIMBAL_WEB_PROXY: '', GIMBAL_WEB_ORIGIN: '' },
            wait: {
                stderr:
                    /gimbal: web application listening on 127\.0\.0\.1:(?<gimbal_e2e_port>\d+) \(prod\)/
            },
            gracefulShutdown: { signal: 'SIGINT', timeout: 5_000 }
        },
```

---

## 3. Direct Function Calls vs Client Protocol Proof

From `ephemeral/research/static-workflow-entrypoints/definition-of-done.md` (lines 34–37, 53–58):

```markdown
Evidence: source/import inspection and focused generator/compilation tests,
including an actual generated client sending an HTTP request to the generated
server handler. A direct call to a shared Go function alone does not prove the
client/remote path.
...
Evidence: focused SKGO protocol tests using independent SvelteKit-produced
fixtures, plus real browser and Go-client submissions to the same handler.
Exercise invalid required input, an invalid role choice, and optional presence;
verify the rejected submissions created no run. Tests cover every built-in's
generated binding without requiring five expensive live workflow executions.
```

Compare with historical in-process direct call in `web/interview_test.go` (lines 107–109):

```go
answered, err := routes.Skgo_answerInterview(runtime.ctx, routes.InterviewAnswer{
    Run: id, QuestionID: first.QuestionID, Answer: "Blue",
})
```
*Note: Directly invoking `routes.Skgo_answerInterview` in-process skips HTTP transport, wire encoding, header verification, and error envelopes. The definition of done expressly requires testing the real client over HTTP/UDS.*

---

## 4. Retaining Run Ownership Beyond Client Lifecycle

From `web/submit_test.go` (lines 58–67):

```go
clientCtx, stopClient := context.WithCancel(t.Context())
admitted, err := Submit(clientCtx, instanceDir, projectB, Submission{Name: "fixture", Params: json.RawMessage(`{"Choice":"kept"}`), WorkDir: workdir})
if err != nil {
    t.Fatal(err)
}
stopClient()
<-started
if _, err := os.Stat(filepath.Join(projectB, ".gimbal", "runs", admitted.ID)); err != nil {
    t.Fatal(err)
}
```
*Note: In `submit_test.go`, the client context `clientCtx` is cancelled immediately after submission (`stopClient()`), and the test verifies the run continues executing on the host. In the new architecture, the same behavior must be verified using the generated Go client calling the concrete start remote.*
