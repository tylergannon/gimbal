# Typed Go Client Generation and Transport Architecture

## Scope

This topic defines the Go client contract for SKGO remote forms, covering typed API signatures, package structure, HTTP/UDS transport configuration, caller context cancellation propagation without server run aborts, and the release delta between pinned SKGO v0.5.0 and worktree fed929b.

## Synthesis

### 1. Client API Signatures, Type Declarations, and Package Structure

To eliminate duplicated SvelteKit wire protocol logic in Gimble, SKGO must generate typed Go client callers from the same remote form declarations used to generate TypeScript stubs and server handlers. ([skgo-gen-emit-and-codecs.txt](sources/skgo-gen-emit-and-codecs.txt), [gimble-implementation-plan-contract.txt](sources/gimble-implementation-plan-contract.txt))

- **Package Structure**:
  - The client generation logic belongs in SKGO's `internal/gen` (e.g. extending `emit.go` to write a client package or exported client methods).
  - The shared wire encoding and response decoding belong in SKGO's runtime (e.g., `skgo` or a subpackage like `skgo/client`).
  - Gimble's CLI commands import the generated client package (e.g. `github.com/tylergannon/gimble/internal/skgo/client` or direct package bindings) and call typed functions.
- **Type Declarations and Signatures**:
  - Generated client method:
    ```go
    func (c *Client) <RemoteName>(ctx context.Context, in <InputType>) (<OutputType>, error)
    ```
    or package-level invoker:
    ```go
    func <RemoteName>(ctx context.Context, client *http.Client, baseURL string, in <InputType>) (<OutputType>, error)
    ```
  - **Inputs and Outputs**: Reuses the exact Go types defined for the workflow/form, preventing type duplication.
  - **Error Types**: Returns typed errors matching the wire contract:
    - `*skgo.Invalid`: Populated with `[]skgo.Issue{Field, Message}` when the server returns validation issues.
    - `*skgo.HTTPError`: Populated with `Status` and `Message` when the server returns a failure envelope (`{"type":"error", "error": {...}}`).
    - Standard Go errors (`context.Canceled`, network errors) for transport failures.

### 2. HTTP and Unix Domain Socket (UDS) Transport Configuration

Gimble instances listen on a web port and a dedicated Unix Domain Socket at `<instance-dir>/control/<pid>.sock`.
The generated client must accept a configured `http.RoundTripper` or `*http.Client` to communicate over either transport. ([gimble-web-submit-and-control.txt](sources/gimble-web-submit-and-control.txt), [client-transport-and-cancellation.md](clips/client-transport-and-cancellation.md))

- **UDS Configuration**:
  ```go
  transport := &http.Transport{
      DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
          return (&net.Dialer{}).DialContext(ctx, "unix", socketPath)
      },
  }
  client := &http.Client{Transport: transport}
  ```
- **URL Convention**: When dialing via UDS, the host in the URL is synthetic (e.g., `http://gimble/_app/remote/...` or `http://localhost/...`), while the request path targets the remote function endpoint.
- **Headers**: Client requests can include necessary headers (e.g. `X-Gimble-Project` where applicable) without fabricating browser-specific Origin or Referer headers.

### 3. Caller Context Cancellation and Server Run Lifetime

The client must honor caller context cancellation during network wait without terminating work accepted by the server. ([gimble-web-submit-and-control.txt](sources/gimble-web-submit-and-control.txt), [gimble-implementation-plan-contract.txt](sources/gimble-implementation-plan-contract.txt), [client-transport-and-cancellation.md](clips/client-transport-and-cancellation.md))

- **Client Cancellation Behavior**:
  - The client makes the HTTP call using `http.NewRequestWithContext(ctx, http.MethodPost, url, body)`.
  - When `ctx` is canceled (e.g. via `Ctrl+C` or deadline), `client.Do(req)` unblocks immediately and returns `ctx.Err()`.
  - **No Cancellation RPC**: The client must **not** issue a cancellation request (such as `POST /api/runs/{id}/cancel`) upon network wait termination.
- **Server Lifetime Decoupling**:
  - The server creates the workflow run under an independent, server-owned context (`run.Context` derived from the project/instance lifecycle), NOT the transient HTTP request context `r.Context()`.
  - Closing the HTTP connection or aborting the request leaves the accepted run active under server ownership.
- **No Automatic Retry on Mutation**:
  - If a network error or context cancellation occurs while waiting for a response, the client must **never** automatically retry the mutation, as doing so could spawn duplicate runs.

### 4. SKGO Baseline Comparison: v0.5.0 vs fed929b

An inspection of SKGO commit history and diffs reveals significant evolution between the pinned baseline and current worktree. ([skgo-git-diff-v0.5.0-fed929b.txt](sources/skgo-git-diff-v0.5.0-fed929b.txt))

- **Changes between v0.5.0 and fed929b**:
  - `v0.5.0` (`a14ceed`): Delegated project generation to VitePlus/sv. Had internal devalue implementation (`internal/devalue`).
  - `v0.6.0` (`6c8a95a`): Exposed Svelte installer choices in `skgo new`.
  - `fed929b` (`HEAD` of `codex/workflow-form-clients`): PR #152 deleted `internal/devalue` entirely (2,574 LOC removed) and refactored SKGO to use `github.com/tylergannon/polytype/devalue.Uneval` directly.
- **Missing Features in fed929b**:
  - Does **not** yet generate Go clients for remote Forms in `internal/gen`.
  - Does **not** yet fix form scalar/Optional binding in `internal/formdata/decode.go` (it still bypasses Polytype codecs and fails to bind scalars to `polytype.Optional[T]`).
- **Minimum Release Baseline for Gimble**:
  - Gimble cannot pin `v0.5.0` or `fed929b` directly.
  - SKGO must implement typed Go Form client generation, wire transport encoding/decoding, and Polytype-delegated form decoding on top of `fed929b`.
  - A new release tag (e.g., `v0.7.0`) must be cut and published. Gimble can then update `go.mod` to pin this release, satisfying the Definition of Done requirement: "leave no local module replacement in the delivered build".

## Actionable Constraints for Implementation

1. **Configurable Transport**: SKGO client functions must accept an `http.RoundTripper` or `*http.Client` so Gimble can route over UDS or standard HTTP interchangeably.
2. **Context Independence**: Aborting client network wait must unblock the CLI without canceling the server run.
3. **No Retries**: The client must treat POST mutations as non-idempotent and refrain from automatic retries.
4. **Cross-Repo Delivery Order**: SKGO must be updated and tagged first; Gimble updates its pin only after SKGO releases the required client generator and form decoder fixes.

## Questions Addressed

- **API signature and package structure**: Documented typed client signatures, input/output reuse, error unmarshaling (`*skgo.Invalid` vs `*skgo.HTTPError`), and SKGO generator integration points.
- **UDS transport configuration**: Provided exact Go `http.Transport` `DialContext` configuration for Gimble's control socket.
- **Context cancellation propagation**: Clarified the boundary between network wait abortion and server run context independence.
- **Differences and release baseline**: Cataloged git commits from `v0.5.0` through `v0.6.0` to `fed929b`, identified missing client generation and form binding capabilities, and defined the release baseline required for Gimble.

## Distinctions: Supported Facts vs Inference

- **Supported Fact**: SKGO commit `fed929b` deleted `internal/devalue` in favor of `polytype/devalue.Uneval` and does not contain Go client generation.
- **Supported Fact**: Gimble connects to its control socket via `http.Transport` with custom `DialContext` using `net.Dialer.DialContext(ctx, "unix", socketPath)`.
- **Supported Fact**: Server workflow execution runs in a background goroutine using `run.Context`, decoupled from `r.Context()`.
- **Inference**: A new minor release (e.g. `v0.7.0`) will be needed once client generation and form decoding fixes land in SKGO.

## Contradictions and Unresolved Questions

- None. The client contract and transport constraints align directly with the accepted architecture in `implementation-plan.md` and `definition-of-done.md`.
