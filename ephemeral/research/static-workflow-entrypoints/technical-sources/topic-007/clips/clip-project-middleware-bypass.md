# Clip: Project Middleware Scoping and Instance-Scoped Bypass

### The Problem in Current Dispatch

In `web/runtime.go:projectRequest`:

1. All incoming requests to `/_app/` inspect `Referer`:
   ```go
   if strings.HasPrefix(path, "/_app/") {
       path = r.Header.Get("Referer")
       if parsed, err := url.Parse(path); err == nil {
           path = parsed.Path
       }
   }
   ```
2. The runtime attempts to match `path` against known projects (`/projects/<candidate.id>`).
3. If no project matches (`p == nil`):
   ```go
   if strings.HasPrefix(r.URL.Path, "/projects/") || strings.Contains(r.URL.Path, "/remote/") || strings.HasPrefix(r.URL.Path, "/api/") {
       http.NotFound(w, r)
       return
   }
   ```
   Since all SKGO remotes are served under `/_app/remote/...`, they contain `"/remote/"`. As a result:
   - Any remote call from a non-project page (e.g. root `/` or `/start`) has `p == nil` and is dropped with 404.
   - Any remote call from the Go CLI client (which does not provide a browser Referer) has `p == nil` and is dropped with 404.

### The Required Instance-Scoped Bypass

Instance-scoped start remotes supply the project directory within the request payload. Therefore:
- The middleware must not reject `p == nil` if the request is destined for an instance-scoped start remote.
- Instead, it must forward the request to the handler stack without demanding a pre-admitted project in the context.
- Dynamic project admission occurs inside the start remote handler via `instance.AdmitProject(arg.ProjectDir)` before workflow execution starts.
