## Changes requested

*(Posted as a comment because GitHub doesn't allow requesting changes on your own PR. Treat this as a rejection.)*

To the coding agent on this PR: the core is right, and this is close. The `OrchestratorBackend`/`SecretsBackend` contracts match the accepted plan, every symbol on the retirement list is gone, and `service-check` passes on all three messaging backends. But the gesture isn't finished. Please resolve the two **blocking** issues below in this PR. Then take the two follow-on issues in follow-up PRs after this one merges.

### Blocking: resolve in this PR
- **#480: Finish retiring the old environment/service model.** `gimbal environment` plus the Docker plugin is still a second model of "environment" outside `OrchestratorBackend`. It still carries dead URL-service settings (`PublicHost`, `ServicePorts`, and help text saying "Start means launched, not ready"). `Route.Environment` changed meaning while `internal/host/routes.go` still writes the endpoint URL into it, which breaks `--workflow-endpoint` plus `--task-endpoint`. Execution-route daemons have no secrets backend. Either migrate Docker to an `OrchestratorBackend` or delete it; don't leave both models.
- **#481: Fix the workflow secrets grammar.** `WithYields` carries down the context, so after `ctx = WithYields(...)` an unrelated `RunCommand(ctx, …, "true")` fails the scope. Make requiring look like producing (`WithRequirements(ctx, store, names...)`). Change `Publish` to take a named `StoreWrite{Store, Values []NamedValue}`; Tyler endorsed this shape. Show yields and requirements in the source-derived graph, as the contract requires.

### Follow-on: not blocking this merge, but yours to pick up next
- **#478: Route named-environment work through a backend capability.** `environmentMessaging` depends directly on the concrete `*localOrchestrator` through `WrapMessaging`. It works now; replace it with a narrow routing interface and a generic router.
- **#479: Choose orchestrator and secrets backends independently of messaging plugins.** This is a design question. Open with a short proposal for Tyler before implementing.

When you push, re-run `service-check` on filesystem, HTTP/SSE and River, and update the PR's proof table with the new commit.

