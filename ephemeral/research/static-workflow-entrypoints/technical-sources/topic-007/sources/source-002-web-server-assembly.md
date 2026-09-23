# Source: web/server.go - Production Handler Assembly and Origin Checks

- **Origin**: `/Users/tyler/.codex/worktrees/d798/gimble/web/server.go`
- **Commit**: `40dc82947eed99202fd9cb1dd377b6a3c2abbccc`
- **Retrieval Date**: 2026-09-23
- **Scope**: Production stack composition (`NewHandler`) and origin refusal middleware (`explainOriginRefusals`).

---

```go
// NewHandler assembles the server. It is here, beside the embedded build and
// not in package gimble, because the page's remote functions import gimble
// and gimble cannot import the page. It exists so that there is exactly one
// production stack: the binary in cmd and any test beside this file both call
// NewHandler, and neither can be green over a composition the other does not
// use.
package web

import (
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/tylergannon/skgo"

	"github.com/tylergannon/gimble/internal/observation"
	generated "github.com/tylergannon/gimble/internal/skgo"
)

// NewHandler builds the server over the frontend build in dist. With a
// non-empty proxy it renders pages from a running `vp dev` server; otherwise it
// serves the embedded build. Loads, remote functions and server routes sit in
// front of pages in both modes, so Kit's generated throwing stubs never answer
// an application request.
func NewHandler(dist fs.FS, proxy, origin string) (http.Handler, string, error) {
	manifest, err := skgo.ReadManifest(dist)
	if err != nil {
		return nil, "", err
	}
	if proxy != "" {
		manifest, err = skgo.ReadDevManifest(dist, proxy)
		if err != nil {
			return nil, "", err
		}
	}

	remoteCfg := manifest.RemoteConfig(origin)
	loadCfg := manifest.LoadConfig(origin)
	endpointCfg := manifest.EndpointConfig(origin)

	mode := "prod"
	var pages http.Handler
	var build func(*skgo.Loads, *skgo.Remotes) (http.Handler, error)
	var endpoints *skgo.Endpoints
	if proxy != "" {
		target, err := url.Parse(proxy)
		if err != nil {
			return nil, "", err
		}
		remoteCfg = manifest.RemoteConfig("")
		remoteCfg.Version = ""
		remoteCfg.Dev = true
		remoteCfg.CookieOrigin = origin
		loadCfg.Version = ""
		loadCfg.Dev = true
		endpointCfg.Dev = true
		mode = "dev"
		build = func(loads *skgo.Loads, remotes *skgo.Remotes) (http.Handler, error) {
			ssr, err := skgo.NewDevSSR(dist, manifest, loads, remotes, proxy, skgo.SSROptions{
				Fetch: observation.Routes(endpoints.Intercept(http.NotFoundHandler())),
			})
			if err != nil {
				return nil, err
			}
			return skgo.NewDevPages(target, manifest, ssr, log.Printf, endpoints), nil
		}
	} else {
		build = func(loads *skgo.Loads, remotes *skgo.Remotes) (http.Handler, error) {
			ssr, err := skgo.NewSSR(dist, manifest, loads, remotes, skgo.SSROptions{
				Fetch: observation.Routes(endpoints.Intercept(http.NotFoundHandler())),
			})
			if err != nil {
				return nil, err
			}
			return skgo.NewStaticHandler(dist, skgo.WithSSR(ssr))
		}
	}

	remoteCfg.Transport = generated.Transport()
	loadCfg.Transport = generated.Transport()

	remotes, err := skgo.NewRemotes(remoteCfg, generated.Remotes()...)
	if err != nil {
		return nil, "", err
	}
	loads, err := skgo.NewLoads(loadCfg, generated.Loads()...)
	if err != nil {
		return nil, "", err
	}
	endpoints, err = skgo.NewEndpoints(endpointCfg, generated.Endpoints()...)
	if err != nil {
		return nil, "", err
	}
	if pages, err = build(loads, remotes); err != nil {
		return nil, "", err
	}
	return observation.Routes(explainOriginRefusals(remoteCfg,
		loads.Intercept(remotes.Intercept(endpoints.Intercept(pages))))), mode, nil
}

// explainOriginRefusals turns the one failure a new app is most likely to hit
// into a message that says what to do about it.
func explainOriginRefusals(cfg skgo.RemoteConfig, next http.Handler) http.Handler {
	if cfg.Origin == "" {
		return next
	}
	appDir := cfg.AppDir
	if appDir == "" {
		appDir = "_app"
	}
	prefix := cfg.Base + "/" + appDir + "/remote/"

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got := r.Header.Get("Origin")
		if r.Method == http.MethodGet || got == cfg.Origin || !strings.HasPrefix(r.URL.Path, prefix) {
			next.ServeHTTP(w, r)
			return
		}
		msg := "This app's origin is " + cfg.Origin + ", but the request came from " +
			quoteOrigin(got) + ", so it was refused.\n\n" +
			"Browse the app at " + cfg.Origin + ", or set GIMBLE_WEB_ORIGIN to the\n" +
			"browser-visible origin when Gimble runs behind a reverse proxy.\n"
		log.Printf("skgo: refused %s %s: Origin %s, want %s. Set GIMBLE_WEB_ORIGIN=%s behind a reverse proxy or browse the app at %s",
			r.Method, r.URL.Path, quoteOrigin(got), cfg.Origin, got, cfg.Origin)
		http.Error(w, msg, http.StatusForbidden)
	})
}
```
