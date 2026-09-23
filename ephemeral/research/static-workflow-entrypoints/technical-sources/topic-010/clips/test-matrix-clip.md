# Clip: Gimble Real-Path Testing Matrix (Package, HTTP/UDS, and E2E)

This clip defines the concrete test structure to satisfy the definition of done without mocks:

| Layer | Runner | Target Under Test | Transport & Protocol | Evidence & Gates |
| --- | --- | --- | --- | --- |
| **SKGO Form Client Unit** | `go test ./...` in SKGO | `remote_form.go`, generated Go form client | Pinned SvelteKit golden binary fixtures (`application/x-sveltekit-formdata`) | Verifies header parsing, file offset tables, field issue normalization, and error envelopes |
| **Gimble Package Client/Remote Integration** | `go test ./...` in `web/` | Generated Go client calling concrete `start<Workflow>` remote | HTTP loopback / UDS socket to real `web.NewHandler` or control listener | Verifies admission on first use, run creation, rejection before run creation, client context cancellation |
| **Gimble Lifecycle Beyond Client** | `go test ./...` in `web/` | Host project ownership & active run tracking | Generated client disconnects immediately after HTTP response | Verifies run proceeds under host ownership; follow connects later; panic containment |
| **Browser E2E Human Start** | `just e2e` (`playwright-bdd`) | Visible start form in Chromium | Real browser submitting enhanced form to embedded `bin/gimble --port 0` | Verifies field error display, pending submit states, `aria-invalid` styles, and SvelteKit route navigation |
| **Multi-Project Concurrency Proof** | Real CLI + Browser live run | Serving PID running Project A and Project B | UDS & Browser HTTP | Verifies single PID, distinct storage, project isolation, cheap models (`gpt-5.6-luna`, Claude Haiku, Gemini Flash) |
