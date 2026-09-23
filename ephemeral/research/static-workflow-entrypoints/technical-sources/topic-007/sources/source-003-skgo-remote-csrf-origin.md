# Source: skgo/remote.go and skgo/remote_form.go - Origin and CSRF Policies

- **Origin**: `/Users/tyler/.codex/worktrees/d798/skgo/remote.go` and `/Users/tyler/.codex/worktrees/d798/skgo/remote_form.go`
- **Commit**: `fed929b0f9894dc12c72b75da8387f572b5aacd6`
- **Retrieval Date**: 2026-09-23
- **Scope**: Origin verification on non-GET remotes and Content-Type enforcement on remote forms.

---

### Origin Checking in `remote.go:666-675`

```go
func (rs *Remotes) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Kit refuses cross-site remote calls before it even looks the id up, and
	// the refusal is a plain 403 rather than a remote-function envelope.
	if rs.cfg.Origin != "" && r.Method != http.MethodGet && r.Header.Get("Origin") != rs.cfg.Origin {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "private, no-store")
		w.WriteHeader(http.StatusForbidden)
		writeJSON(w, map[string]string{"message": "Cross-site remote requests are forbidden"})
		return
	}

	rest := strings.TrimPrefix(r.URL.Path, rs.prefix)
	parts := strings.Split(rest, "/")
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		rs.writeError(w, notFound())
		return
	}

	fn, ok := rs.fns[parts[0]+"/"+parts[1]]
	if !ok {
		rs.writeError(w, notFound())
		return
	}
...
```

### Form Content-Type and CSRF Policy in `remote_form.go:123-155`

```go
func (rs *Remotes) serveForm(w http.ResponseWriter, r *http.Request, fn *Remote) {
	if r.Method != http.MethodPost {
		rs.writeErrorStatus(w, &HTTPError{
			Status:  405,
			Message: "`form` functions must be invoked via POST request, not " + r.Method,
		}, http.StatusMethodNotAllowed)
		return
	}

	// Kit refuses a non-form content type with 415 before it reads a byte,
	// because the form content types are the ones a cross-origin <form> can
	// send and so the ones its CSRF protection is written around.
	media := mediaType(r.Header.Get("Content-Type"))
	if !isFormContentType(media) {
		rs.writeErrorStatus(w, &HTTPError{
			Status:  415,
			Message: "`form` functions expect form-encoded data — received " + r.Header.Get("Content-Type"),
		}, http.StatusUnsupportedMediaType)
		return
	}

	// Of those, only kit's binary envelope can be answered here. The rest
	// belong to a submission kit did not enhance, which posts to the page's
	// own URL with `?/remote=<id>` and is answered by rendering the page —
	// something a CSR-only app has no server-side renderer to do.
	if media != formdata.ContentType {
		rs.writeErrorStatus(w, &HTTPError{
			Status:  415,
			Message: "`form` functions need an enhanced submission; this app renders on the client only",
		}, http.StatusUnsupportedMediaType)
		return
	}

	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxFormBytes))
	if err != nil {
		rs.writeErrorStatus(w, &HTTPError{Status: 413, Message: "Payload Too Large"}, http.StatusRequestEntityTooLarge)
		return
	}
...
```
