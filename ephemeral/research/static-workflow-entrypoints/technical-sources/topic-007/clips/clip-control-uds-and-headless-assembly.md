# Clip: Control UDS vs Browser Listener, Origin Constraints, and Headless Assembly

### 1. Control UDS vs Optional Browser Listener (`cfg.network`)

In `web/runtime.go`:
- **Control UDS**: Started unconditionally by `Instance.startControl()` (`web/runtime.go:174, 230-304`). It always listens on a dedicated Unix Domain Socket (`<instanceDir>/<identity>.sock`), irrespective of web configuration.
- **Browser/Web Listener**: Configured by `cfg config` and started conditionally by `Instance.startWeb(cfg)` (`web/runtime.go:298-351`):
  - `WithPort(port)` sets `cfg.network = "tcp"` (serving HTTP to browsers).
  - `WithUDS(path)` sets `cfg.network = "unix"` (serving the web application/SSR over a UDS path).
  - `WithNoWeb()` sets `cfg.network = "none"` (disabling `startWeb` entirely).
`cfg.network` governs *only* the browser web listener. It must not be conflated with the instance's internal control UDS.

### 2. Origin Configuration on Control Remote Assembly

In existing code:
- The control socket (`controlMux` in `web/control.go:323-333`) currently mounts only observation routes and internal JSON control endpoints (`/control/runs`, `/control/submit`, etc.). It mounts **no** SKGO remote assembly.
- SKGO's Origin check (`skgo/remote.go:669`) rejects any non-GET remote call when `rs.cfg.Origin != ""` and `r.Header.Get("Origin") != rs.cfg.Origin`.
- When mounting start remotes on the control socket/UDS, the control remote assembly does **not** automatically receive an empty Origin from existing control code.
- Configuring the control remote assembly is an **implementation choice**:
  - The control socket handler assembly must be configured (e.g. with `RemoteConfig{Origin: ""}`) so local Go CLI clients over UDS are not blocked by browser Origin checks.
  - Concurrently, the browser-facing listener (`startWeb`) must continue enforcing its configured `Origin` (and `explainOriginRefusals`) to preserve CSRF protections against browser traffic.

### 3. Shared Remote Assembly in Headless Mode Requires No Fake Assets

In `skgo`:
- `skgo.NewRemotes(cfg RemoteConfig, fns ...*Remote)` (`skgo/remote.go:541`) requires only a `RemoteConfig` and the slice of `*Remote` descriptors (from `generated.Remotes()`).
- `skgo.NewRemotes` contains pure Go devalue argument parsing, function invocation, and response encoding. It has **no dependency** on `dist fs.FS`, Vite manifests, SSR bundles, or static HTML/JS assets.
- Frontend build assets (`dist fs.FS`) are required only by `skgo.ReadManifest(dist)`, `skgo.NewSSR(dist, ...)`, and `skgo.NewStaticHandler(dist, ...)` in `web/server.go:NewHandler` for rendering HTML pages.
- Therefore, in headless mode (`--no-web`) or on the control listener, mounting the shared remote handlers does **not** require fabricating fake or stubbed frontend assets. The remote handler registry operates directly on pure Go bindings.
