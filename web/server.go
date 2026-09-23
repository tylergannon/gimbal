// NewHandler assembles the server. It is here, beside the embedded build and
// not in package gimble, because the page's remote functions import gimble
// and gimble cannot import the page. It exists so that there is exactly one
// production stack: the binary in cmd and any test beside this file both call
// NewHandler, and neither can be green over a composition the other does not
// use.
package web

import (
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/tylergannon/skgo"

	"github.com/tylergannon/gimble/internal/builtin"
	"github.com/tylergannon/gimble/internal/observation"
	generated "github.com/tylergannon/gimble/internal/skgo"
)

// stockStartRemotes selects the compiled Form entrypoints by the same
// build-time workflow list used to generate their declarations.
func stockStartRemotes() ([]*skgo.Remote, error) {
	available := make(map[string]*skgo.Remote)
	for _, remote := range generated.Remotes() {
		available[remote.Module()+"#"+remote.Name()] = remote
	}
	starts := make([]*skgo.Remote, 0, len(builtin.Workflows))
	for _, workflow := range builtin.Workflows {
		module := "src/routes/" + strings.ReplaceAll(workflow.Name, "-", "") + "_start.remote.ts"
		key := module + "#start" + workflow.Entry
		remote := available[key]
		if remote == nil || remote.Kind() != skgo.KindForm {
			return nil, fmt.Errorf("gimble: generated start Form %s is missing", key)
		}
		starts = append(starts, remote)
	}
	return starts, nil
}

// NewHandler builds the server over the frontend build in dist. With a
// non-empty proxy it renders pages from a running `vp dev` server; otherwise it
// serves the embedded build. Loads, remote functions and server routes sit in
// front of pages in both modes, so Kit's generated throwing stubs never answer
// an application request.
func NewHandler(dist fs.FS, proxy, origin string) (http.Handler, string, error) {
	// The manifest is read in both modes: it is where appDir and base come
	// from, and those decide the URL prefix remote calls arrive on.
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
	// build makes the page handler once all three registries exist. The
	// renderer needs the loads and remotes directly, and its event.fetch host
	// needs the endpoint registry.
	var build func(*skgo.Loads, *skgo.Remotes) (http.Handler, error)
	var endpoints *skgo.Endpoints
	if proxy != "" {
		target, err := url.Parse(proxy)
		if err != nil {
			return nil, "", err
		}
		// In dev the client is served by vite, not by this build, so its baked
		// version differs from the manifest's — sending x-sveltekit-version
		// would make it reload in a loop. Kit skips the remote origin check in
		// dev too.
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
		// Pages are rendered here, in this process, by the SSR bundle the
		// adapter built. The renderer needs the loads and the remote functions
		// it will answer during a render, so the page handler is built after
		// both of them exist.
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

	// The app's `transport` hook, the Go half of the encode/decode pairs
	// src/hooks.ts declares. Kit puts the same object on both its client and
	// its server, so both registries that serialize a result get it. It is
	// set here, after the dev branch has replaced the configs it rebuilds, so
	// a dev render encodes exactly what a production one does.
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
	// The observation routes sit outside the whole kit stack: they are not
	// pages, loads, remote functions or +server routes, and they read the
	// registry out of the request context the runtime supplies. Everything
	// else falls through to kit exactly as before.
	//
	// They are also the SSR renderer's fetch host above, so a page rendered
	// in this process reaches the same snapshot over the same route the
	// browser uses, without a second composition to keep in step.
	return observation.Routes(explainOriginRefusals(remoteCfg,
		loads.Intercept(remotes.Intercept(endpoints.Intercept(pages))))), mode, nil
}

// explainOriginRefusals turns the one failure a new app is most likely to hit
// into a message that says what to do about it.
//
// A command is a POST, and skgo refuses a non-GET remote call whose `Origin`
// header is not the origin the server was configured with — the same check kit
// makes. That is a bare 403 with no body: in the browser every command fails
// and nothing says why. The runtime normally derives its origin from the
// listener, so this wrapper names both values and the reverse-proxy override.
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

func quoteOrigin(origin string) string {
	if origin == "" {
		return "(no Origin header)"
	}
	return origin
}
