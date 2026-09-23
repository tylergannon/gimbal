# Clip: HTTP Request vs Asynchronous Run Context Hierarchy

### Context Lifetime Problem

When an HTTP request hits an SKGO remote handler:
1. `net/http` provides `r.Context()`, which is cancelled as soon as the HTTP response finishes writing, or if the client disconnects prematurely.
2. In `skgo/remote_form.go:184-186`, `serveForm` passes `ctx := withEvent(r.Context(), ev)` to the handler.
3. If a workflow runs under `ctx`, returning from the remote handler completes the HTTP response, which cancels `ctx` and terminates the workflow immediately.

### Required Architecture

1. The asynchronous workflow execution must run in a separate goroutine under the long-lived project context `p.ctx` (which descends from instance context `i.ctx`), completely decoupled from `r.Context()`.
2. The remote handler coordinates with the spawned run using a channel (`started := make(chan string, 1)` via `conversationRunStartedKey`).
3. The remote handler blocks on `started` until the run is admitted and registered (or returns an admission error).
4. Upon receiving the run ID, the handler returns the typed admission result `AdmissionResult{ID: id}`.
5. If the client disconnects or cancels its HTTP request *during* network wait (`select` on `r.Context().Done()`), the HTTP request terminates, but the already admitted workflow continues to run under `p.ctx`.
6. Graceful shutdown: `Instance.activeRuns sync.WaitGroup` is incremented on run start and decremented on completion; `Instance.Wait()` blocks until all active runs finish unwinding.
