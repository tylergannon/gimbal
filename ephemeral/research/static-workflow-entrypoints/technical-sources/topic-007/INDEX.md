# Topic 007: Instance-Scoped Routing and Multi-Listener Mounting

## Index and Question Routing

This topic establishes the local evidence for request routing, middleware dispatch, origin and CSRF policies, and headless multi-listener mounting across the web listener and control socket.

### Question 1: How are requests currently dispatched and filtered by project middleware in `web/` and `web/server.go`, and how must instance-scoped start remotes be mounted to bypass project-scoping filters?

- **Local Evidence**:
  - [`source-001-web-runtime-middleware.md`](sources/source-001-web-runtime-middleware.md): Excerpts of `web/runtime.go:86-141` (listener options), `web/runtime.go:298-351` (`startWeb`), and `web/runtime.go:366-411` (`projectRequest` middleware).
  - [`source-002-web-server-assembly.md`](sources/source-002-web-server-assembly.md): Excerpt of `web/server.go:1-127` (`NewHandler` production stack composition).
  - [`clip-project-middleware-bypass.md`](clips/clip-project-middleware-bypass.md): Structural breakdown of `Referer` sniffing and the required instance-scoped bypass mechanism.
- **Supported Facts**:
  - In `web/runtime.go:366-376`, `projectRequest` wraps the web handler. If `r.URL.Path` begins with `/_app/`, it reads `r.Header.Get("Referer")` and parses its path.
  - In `web/runtime.go:382-386`, the middleware searches `i.projects` for a match where `path` starts with `"/projects/" + candidate.id`.
  - In `web/runtime.go:390-394`, if no project matches (`p == nil`), any request where `strings.Contains(r.URL.Path, "/remote/")` immediately returns `404 Not Found`. Because all SKGO remotes are under `/_app/remote/...`, all remote requests arriving without an admitted project Referer are rejected.
  - When `p != nil`, `p.requestContext(r.Context())` injects project directory, observation registry, runs table, and conversation manager (`web/runtime.go:406-411`).
  - In `web/server.go:125-126`, `NewHandler` returns `observation.Routes(explainOriginRefusals(remoteCfg, loads.Intercept(remotes.Intercept(endpoints.Intercept(pages)))))`.
- **Inference & Implementation Consequence**:
  - Start remotes are *instance-scoped*: the project directory is supplied in the form payload, not the URL path or Referer header.
  - To bypass the project-scoping filter, `projectRequest` must distinguish instance-scoped start remotes from project-scoped remotes (e.g. by checking if the target remote is a start remote, or allowing instance-scoped remote routes through when `p == nil`) so they reach `remotes.Intercept` where the handler performs dynamic project admission via `instance.AdmitProject(arg.ProjectDir)`.
- **Contradictions & Unresolved Questions**:
  - None on the filtering mechanism. The specific route pattern or remote naming convention used to identify instance-scoped start remotes within `projectRequest` must be agreed upon by the code generator and handler assembly.

---

### Question 2: What CSRF, Origin, or Referer policies does SKGO enforce on remote forms, and how can the Go CLI client submit requests without fabricating browser headers?

- **Local Evidence**:
  - [`source-001-web-runtime-middleware.md`](sources/source-001-web-runtime-middleware.md): Excerpts of `web/runtime.go:86-141` (`WithPort`, `WithUDS`, `WithNoWeb`) and `web/runtime.go:230-283` (`startControl`).
  - [`source-003-skgo-remote-csrf-origin.md`](sources/source-003-skgo-remote-csrf-origin.md): Excerpts of `skgo/remote.go:666-675` (origin enforcement) and `skgo/remote_form.go:123-155` (form content-type & CSRF).
  - [`source-002-web-server-assembly.md`](sources/source-002-web-server-assembly.md): Excerpt of `web/server.go:129-161` (`explainOriginRefusals`).
  - [`source-004-web-submit-control-probing.md`](sources/source-004-web-submit-control-probing.md): Excerpt of `web/submit.go:129-141` (`selectedClient.request` over UDS) and `web/control.go:323-333` (`controlMux`).
  - [`clip-control-uds-and-headless-assembly.md`](clips/clip-control-uds-and-headless-assembly.md): Clarification of control UDS vs `cfg.network`, and Origin configuration constraints.
- **Supported Facts**:
  - **Listener Distinction**: `cfg.network` in `web/runtime.go:86-141` configures *only* the optional browser/web listener (`WithPort` -> tcp, `WithUDS` -> unix, `WithNoWeb` -> none). The control UDS is separate and always started unconditionally by `Instance.startControl()` (`web/runtime.go:174, 230-304`).
  - **SKGO Origin Enforcement**: `skgo/remote.go:669` checks:
    `if rs.cfg.Origin != "" && r.Method != http.MethodGet && r.Header.Get("Origin") != rs.cfg.Origin`.
    If `rs.cfg.Origin` is non-empty and `Origin` does not match, SKGO returns HTTP 403 Forbidden (`{"message": "Cross-site remote requests are forbidden"}`). `web/server.go:explainOriginRefusals` wraps this check on the web listener with an advisory error message.
  - **Existing Control Code Has No Remote Assembly**: In `web/control.go:323-333`, `controlMux` mounts only internal observation and control routes; existing control code contains no SKGO remote assembly and does not set or pass `Origin`.
  - **SKGO Form Media Type & CSRF**: `skgo/remote_form.go:135-154` requires `POST` and checks `media := mediaType(r.Header.Get("Content-Type"))`. If `!isFormContentType(media)` or `media != formdata.ContentType`, it responds with HTTP 415 Unsupported Media Type.
  - **SKGO Referer Policy**: SKGO itself enforces **no** Referer policy. The requirement for a Referer header exists solely in Gimbal's `web/runtime.go:372` to resolve project identity for project-scoped browser interactions.
- **Inference & Implementation Consequence**:
  - The new control remote assembly does **not** automatically receive an empty Origin from existing control code; configuring its Origin is an **implementation choice**:
    - The control listener assembly must be configured (e.g. with `RemoteConfig{Origin: ""}`) so local Go CLI clients connecting over the control UDS are not rejected by Origin validation and do not need to fabricate browser `Origin` headers.
    - Concurrently, the browser-facing web listener must retain its configured `Origin` (and `explainOriginRefusals`) to preserve CSRF protections against browser traffic.
  - Because start remotes are instance-scoped and supply `ProjectDir` directly in the form payload, the Go CLI client does not need to fabricate a browser `Referer` header.
- **Contradictions & Unresolved Questions**:
  - If the Go CLI client connects over loopback TCP to the browser listener (where `cfg.Origin` is set) rather than the control UDS, standard SKGO Origin enforcement would fail without an Origin header. The accepted plan explicitly designates the control UDS as the primary transport for the Go client ("The client accepts the configured HTTP transport, so Gimbal can use its Unix socket").

---

### Question 3: How are start remote handlers mounted on the control listener/UDS in headless mode (`--no-web`) using the shared handler assembly?

- **Local Evidence**:
  - [`source-001-web-runtime-middleware.md`](sources/source-001-web-runtime-middleware.md): Excerpt of `web/runtime.go:132-134` (`WithNoWeb`), `web/runtime.go:186-189` (`NewInstance` in headless mode), and `web/runtime.go:230-283` (`startControl`).
  - [`source-002-web-server-assembly.md`](sources/source-002-web-server-assembly.md): Excerpt of `web/server.go:1-6, 102-126` (`NewHandler` and `skgo.NewRemotes`).
  - [`source-004-web-submit-control-probing.md`](sources/source-004-web-submit-control-probing.md): Excerpt of `web/control.go:323-333` (`controlMux`) and `web/submit.go:86-127` (`selectedInstance`).
  - [`clip-control-uds-and-headless-assembly.md`](clips/clip-control-uds-and-headless-assembly.md): Separation of shared remote handler assembly from frontend asset requirements.
- **Supported Facts**:
  - In `web/runtime.go:186-189`, when `WithNoWeb()` is configured, `cfg.network == "none"`. `startWeb` is skipped entirely; only `startControl()` runs.
  - In `web/control.go:270-275, 323-333`, `startControl` attaches `controlMux(i)` to the UDS HTTP server. `controlMux` unconditionally calls `h.project(r)`, returning 404 for unadmitted projects, and does not mount `skgo.Remotes`.
  - In `web/server.go:102`, `skgo.NewRemotes(remoteCfg, generated.Remotes()...)` instantiates the remote handlers.
  - In `skgo/remote.go:541-561`, `skgo.NewRemotes` requires only `RemoteConfig` and `fns ...*Remote`. It has **no dependency** on `dist fs.FS`, Vite manifests, SSR bundles, or static frontend assets.
  - Frontend assets (`dist fs.FS`) are required only by `skgo.ReadManifest(dist)`, `skgo.NewSSR(dist, ...)`, and `skgo.NewStaticHandler(dist, ...)` in `web/server.go:NewHandler` for HTML page rendering.
- **Inference & Implementation Consequence**:
  - The shared remote assembly (`skgo.NewRemotes` with `generated.Remotes()`) can be constructed and mounted directly on the control listener in headless mode (`--no-web`) without constructing fake or stubbed frontend build assets.
  - To preserve the single production stack invariant (`web/server.go:1-6`) across both listeners, the shared handler composition must expose the shared remotes interceptor (and observation routes) to `startControl()`, or mount a shared handler on the control socket.
  - `controlMux` on the control UDS must be updated to route `/_app/remote/...` to the shared remotes interceptor and allow instance-scoped start requests without requiring pre-existing project admission.
- **Contradictions & Unresolved Questions**:
  - Clarified: Initial thoughts that `NewHandler` would require fake frontend assets in headless mode were disproven by inspecting `skgo.NewRemotes`: remote handler execution is pure Go and has zero dependency on frontend static files.
