# Client Transport Architecture and Caller Context Cancellation

### 1. HTTP and Unix Domain Socket (UDS) Transport Architecture

Gimbal's control socket uses a local Unix Domain Socket (UDS) located at `<instance-dir>/control/<pid>.sock`.
To connect over UDS without external libraries, the client configures a standard `http.Transport` with a custom `DialContext`:

```go
func NewControlTransport(socketPath string) http.RoundTripper {
	return &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "unix", socketPath)
		},
	}
}
```

- When dialing over UDS, the host portion of the request URL is arbitrary (e.g. `http://gimbal/_app/remote/...` or `http://localhost/...`).
- Project admission requests supply the canonical project directory, which for instance-scoped start remotes is passed in the request body (or via `X-Gimbal-Project` header on control endpoints).

---

### 2. Caller Context Cancellation and Server Run Lifetime

#### A. Network Cancellation vs. Server Run Lifetime
When a CLI client calls a workflow-start remote:
1. The client issues an HTTP POST with `http.NewRequestWithContext(callerCtx, http.MethodPost, url, body)`.
2. If `callerCtx` is canceled (e.g. user hits `Ctrl+C` or a timeout expires while waiting for the response):
   - `client.Do(req)` unblocks immediately and returns `callerCtx.Err()`.
   - The underlying network transport closes the TCP/UDS connection.
   - **Crucial Rule**: The client DOES NOT emit an asynchronous cancellation RPC to the server.
3. On the server side:
   - The start remote handler executes project admission, creates the run in the host registry, and starts the workflow in an independent goroutine:
     ```go
     go func() {
         defer project.finishRun(run.ID)
         env := run.Env(project.dir, workDir, roleModels)
         workflow.Run(run.Context, env, params)
     }()
     ```
   - Notice `run.Context` is bound to the project/server lifecycle, NOT to the HTTP request context `r.Context()`.
   - Even if `r.Context()` terminates when the client disconnects or after the response is sent, `run.Context` remains active.

#### B. No Automatic Mutation Retry
- An HTTP mutation (such as starting a workflow) must never be automatically retried upon connection drop or timeout.
- Retrying a start request when a response was lost in flight would risk admitting and starting duplicate workflow runs.
- If the CLI encounters a network error, it must report the failure to the user, who can inspect existing runs via `gimbal runs` or the web UI.
