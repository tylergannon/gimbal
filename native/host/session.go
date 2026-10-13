package main

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"time"
)

// Only Gimbal's session is durable. OAuth flow cookies stay in the ephemeral
// webview, and neither the page nor the native commands receive the bearer.
type savedSession struct {
	Name    string    `json:"name"`
	Value   string    `json:"value"`
	Expires time.Time `json:"expires"`
}

type sessionStore interface {
	load() ([]byte, error)
	save([]byte) error // nil deletes the credential
}

func newServerProxy(server *url.URL, origin string, store sessionStore) (http.Handler, error) {
	local, err := url.Parse(origin)
	if err != nil || local.Host == "" {
		return nil, fmt.Errorf("invalid desktop proxy origin")
	}
	ip := net.ParseIP(server.Hostname())
	loopback := server.Hostname() == "localhost" || ip != nil && ip.IsLoopback()
	if server.Scheme != "https" && (server.Scheme != "http" || !loopback) {
		return nil, fmt.Errorf("desktop login requires HTTPS or loopback HTTP")
	}
	var mu sync.Mutex
	var session savedSession
	raw, err := store.load()
	if err != nil {
		return nil, err
	}
	if len(raw) != 0 {
		if err := json.Unmarshal(raw, &session); err != nil {
			return nil, fmt.Errorf("read desktop session: %w", err)
		}
		if !time.Now().Before(session.Expires) {
			if err := store.save(nil); err != nil {
				return nil, err
			}
			session = savedSession{}
		}
	}
	sessionName, flowName := "gimbal-session", "gimbal-flow"
	if server.Scheme == "https" {
		sessionName, flowName = "__Host-"+sessionName, "__Host-"+flowName
	}
	if session.Value != "" && session.Name != sessionName {
		return nil, fmt.Errorf("desktop session does not belong to this server")
	}
	proxy := &httputil.ReverseProxy{
		Rewrite: func(r *httputil.ProxyRequest) {
			r.SetURL(server)
			r.Out.Header.Set("X-Gimbal-Desktop", "macos")
			if r.In.Header.Get("Origin") != "" {
				r.Out.Header.Set("Origin", server.String())
			}
			if ref, err := url.Parse(r.In.Header.Get("Referer")); err == nil && ref.Host != "" {
				ref.Scheme, ref.Host = server.Scheme, server.Host
				r.Out.Header.Set("Referer", ref.String())
			}
			r.Out.Header.Del("Cookie")
			for _, cookie := range r.In.Cookies() {
				switch cookie.Name {
				case "gimbal-session", "__Host-gimbal-session":
					continue
				case "gimbal-flow":
					cookie.Name = flowName
				}
				r.Out.AddCookie(cookie)
			}
			mu.Lock()
			defer mu.Unlock()
			if session.Value != "" && time.Now().Before(session.Expires) {
				r.Out.AddCookie(&http.Cookie{Name: session.Name, Value: session.Value})
			}
		},
		ModifyResponse: func(r *http.Response) error {
			mu.Lock()
			defer mu.Unlock()
			cookies := r.Cookies()
			r.Header.Del("Set-Cookie")
			for _, cookie := range cookies {
				if cookie.Name == sessionName {
					next := savedSession{Name: cookie.Name, Value: cookie.Value, Expires: cookie.Expires}
					var raw []byte
					if cookie.MaxAge >= 0 && cookie.Value != "" && time.Now().Before(cookie.Expires) {
						raw, _ = json.Marshal(next)
					} else {
						next = savedSession{}
					}
					if err := store.save(raw); err != nil {
						return err
					}
					session = next
					continue
				}
				if cookie.Name == flowName {
					// The shell is HTTP loopback even when its server uses HTTPS.
					cookie.Name, cookie.Secure = "gimbal-flow", false
				}
				r.Header.Add("Set-Cookie", cookie.String())
			}
			requestCookie, _ := r.Request.Cookie(sessionName)
			if r.StatusCode == http.StatusUnauthorized && session.Value != "" && requestCookie != nil && requestCookie.Value == session.Value {
				if err := store.save(nil); err != nil {
					return err
				}
				session = savedSession{}
			}
			return nil
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			http.Error(w, "Gimbal server or secure login storage is unavailable. Quit and reopen the app to retry; pending recording text is not saved.", http.StatusServiceUnavailable)
		},
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check before rewriting Origin or adding credentials. The loopback
		// proxy must not lend its saved login to another website.
		if r.Host != local.Host {
			http.Error(w, "Request host is not allowed.", http.StatusForbidden)
			return
		}
		if supplied := r.Header.Get("Origin"); supplied != "" && supplied != origin {
			http.Error(w, "Request origin is not allowed.", http.StatusForbidden)
			return
		}
		if site := r.Header.Get("Sec-Fetch-Site"); site == "cross-site" || site == "same-site" {
			http.Error(w, "Request site is not allowed.", http.StatusForbidden)
			return
		}
		proxy.ServeHTTP(w, r)
	}), nil
}
