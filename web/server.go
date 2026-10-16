// NewHandler assembles the server. It is here, beside the embedded build and
// not in package gimbal, because the page's remote functions import gimbal
// and gimbal cannot import the page. It exists so that there is exactly one
// production stack: the binary in cmd and any test beside this file both call
// NewHandler, and neither can be green over a composition the other does not
// use.
package web

import (
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"strings"

	"github.com/tylergannon/gimbal/internal/webauth"

	"github.com/tylergannon/skgo"

	"github.com/tylergannon/gimbal/internal/observation"
	generated "github.com/tylergannon/gimbal/internal/skgo"
)

type handlerSettings struct {
	project func(http.Handler) http.Handler
	auth    *webauth.Auth
}

// NewHandler builds the production server over the frontend build in dist.
func NewHandler(dist fs.FS, origin string, settings ...handlerSettings) (http.Handler, string, error) {
	return newHandler(dist, origin, nil, settings...)
}

func newHandler(dist fs.FS, origin string, development *skgo.NativeDevelopment, settings ...handlerSettings) (http.Handler, string, error) {
	manifest, err := skgo.ReadManifest(dist)
	if err != nil {
		return nil, "", err
	}

	cfg := handlerSettings{}
	if len(settings) > 0 {
		cfg = settings[0]
	}
	if cfg.auth != nil && development != nil {
		return nil, "", fmt.Errorf("auth requires a production web build")
	}
	remoteCfg := manifest.RemoteConfig(origin)
	loadCfg := manifest.LoadConfig(origin)
	endpointCfg := manifest.EndpointConfig(origin)
	loadCfg.Matchers = generated.Matchers()
	endpointCfg.Matchers = generated.Matchers()
	handleCfg := manifest.HandleConfig()
	handleCfg.Origin = origin
	handleCfg.Matchers = generated.Matchers()
	var bound http.Handler
	internal := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { bound.ServeHTTP(w, r) })

	mode := "prod"
	var pages http.Handler
	// build makes the page handler once all three registries exist. The
	// renderer needs the loads and remotes directly, and its event.fetch host
	// needs the endpoint registry.
	var build func(*skgo.Loads, *skgo.Remotes) (http.Handler, error)
	var endpoints *skgo.Endpoints
	if development != nil {
		mode = "dev"
	}
	build = func(loads *skgo.Loads, remotes *skgo.Remotes) (http.Handler, error) {
		ssr, err := development.NewSSR(dist, manifest, loads, remotes, skgo.SSROptions{
			Fetch: internal,
		})
		if err != nil {
			return nil, err
		}
		endpoints.SetErrorTemplate(ssr.ErrorTemplate())
		handleCfg.ErrorTemplate = ssr.ErrorTemplate()
		return development.NewStaticHandler(dist, skgo.WithSSR(ssr))
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
	handleCfg.Loads = loads
	handleCfg.Static = skgo.ServedAsFile(pages)
	bound = observation.Routes(explainOriginRefusals(remoteCfg,
		generated.RequestBoundary(handleCfg, loads.Intercept(remotes.Intercept(endpoints.Intercept(pages))))))
	anonymous := bound
	if cfg.project != nil {
		bound = cfg.project(bound)
	}
	if cfg.auth != nil {
		begin := ""
		for _, r := range generated.Remotes() {
			if r.Module() == "src/routes/auth.remote.ts" && r.Name() == "beginLogin" {
				begin = manifest.Base + "/" + manifest.AppDir + "/remote/" + r.ID()
			}
		}
		if begin == "" {
			return nil, "", fmt.Errorf("auth beginLogin remote is not generated")
		}
		static := map[string]bool{}
		err := fs.WalkDir(dist, "client", func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() {
				static[manifest.Base+"/"+strings.TrimPrefix(path, "client/")] = true
			}
			return nil
		})
		if err != nil {
			return nil, "", err
		}
		bound = cfg.auth.Gate(bound, anonymous, begin, static)
	}
	return skgo.FetchConfig{Origin: origin, Base: manifest.Base, Handler: internal, Prerendered: manifest.Prerendered}.Intercept(bound), mode, nil
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
			"Browse the app at " + cfg.Origin + ", or set GIMBAL_WEB_ORIGIN to the\n" +
			"browser-visible origin when Gimbal runs behind a reverse proxy.\n"
		log.Printf("skgo: refused %s %s: Origin %s, want %s. Set GIMBAL_WEB_ORIGIN=%s behind a reverse proxy or browse the app at %s",
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
