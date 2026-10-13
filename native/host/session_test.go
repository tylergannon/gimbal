package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

type memorySessionStore struct {
	raw []byte
	err error
}

func (s *memorySessionStore) load() ([]byte, error) { return s.raw, s.err }
func (s *memorySessionStore) save(raw []byte) error {
	if s.err != nil {
		return s.err
	}
	s.raw = append([]byte(nil), raw...)
	return nil
}

func TestDesktopSessionSurvivesProxyRestartAndAuthenticatesNativeCommands(t *testing.T) {
	const origin = "http://127.0.0.1:9876"
	var backendOrigin string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/auth/callback" {
			http.SetCookie(w, &http.Cookie{Name: "gimbal-session", Value: "secret", Path: "/", HttpOnly: true, Expires: time.Now().Add(7 * 24 * time.Hour)})
			w.WriteHeader(303)
			return
		}
		cookie, err := r.Cookie("gimbal-session")
		if err != nil || cookie.Value != "secret" {
			http.Error(w, "unauthenticated", http.StatusUnauthorized)
			return
		}
		if r.Method == "POST" && r.Header.Get("Origin") != backendOrigin {
			t.Error("native command origin was not rewritten")
		}
		if r.URL.Path == "/logout" {
			http.SetCookie(w, &http.Cookie{Name: "gimbal-session", Path: "/", MaxAge: -1})
		}
	}))
	defer backend.Close()
	backendOrigin = backend.URL
	server, _ := url.Parse(backend.URL)
	store := &memorySessionStore{}
	proxy, err := newServerProxy(server, origin, store)
	if err != nil {
		t.Fatal(err)
	}
	call := func(proxy http.Handler, method, path string) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, origin+path, nil)
		r.Header.Set("Origin", origin)
		w := httptest.NewRecorder()
		proxy.ServeHTTP(w, r)
		return w
	}
	w := call(proxy, "GET", "/auth/callback")
	if w.Code != 303 || len(store.raw) == 0 || len(w.Result().Cookies()) != 0 {
		t.Fatalf("login failed or exposed bearer to webview: status %d", w.Code)
	}
	// A second proxy has no webview/URLSession cookies or in-memory login.
	proxy, err = newServerProxy(server, origin, store)
	if err != nil {
		t.Fatal(err)
	}
	for _, method := range []string{"GET", "POST"} {
		if w := call(proxy, method, "/protected"); w.Code != 200 {
			t.Fatalf("restored %s request: %d", method, w.Code)
		}
	}
	if w := call(proxy, "POST", "/logout"); w.Code != 200 || len(store.raw) != 0 {
		t.Fatal("logout retained credential")
	}
	if w := call(proxy, "GET", "/protected"); w.Code != 401 {
		t.Fatal("logged-out request retained login")
	}
}

func TestDesktopProxyProtectsCredentialAndClearsRevocation(t *testing.T) {
	const origin = "http://127.0.0.1:9876"
	calls := 0
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		http.Error(w, "revoked", http.StatusUnauthorized)
	}))
	defer backend.Close()
	server, _ := url.Parse(backend.URL)
	raw, _ := json.Marshal(savedSession{Name: "gimbal-session", Value: "secret", Expires: time.Now().Add(time.Hour)})
	store := &memorySessionStore{raw: raw}
	proxy, err := newServerProxy(server, origin, store)
	if err != nil {
		t.Fatal(err)
	}
	for _, attack := range []string{"origin", "site", "host"} {
		r := httptest.NewRequest("GET", origin+"/protected", nil)
		switch attack {
		case "origin":
			r.Header.Set("Origin", "https://attacker.example")
		case "site":
			r.Header.Set("Sec-Fetch-Site", "cross-site")
		case "host":
			r.Host = "attacker.example:9876"
		}
		w := httptest.NewRecorder()
		proxy.ServeHTTP(w, r)
		if w.Code != 403 || calls != 0 {
			t.Fatalf("%s reached authenticated backend", attack)
		}
	}
	w := httptest.NewRecorder()
	proxy.ServeHTTP(w, httptest.NewRequest("GET", origin+"/protected", nil))
	if w.Code != 401 || len(store.raw) != 0 {
		t.Fatal("revocation retained saved credential")
	}
}

func TestDesktopProxyHTTPSFlowCookiesAndSecureStorageFailure(t *testing.T) {
	const origin = "http://127.0.0.1:9876"
	backend := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/begin" {
			http.SetCookie(w, &http.Cookie{Name: "__Host-gimbal-flow", Value: "flow", Path: "/", Secure: true, HttpOnly: true})
			return
		}
		if flow, err := r.Cookie("__Host-gimbal-flow"); err != nil || flow.Value != "flow" {
			t.Error("flow cookie did not reach registered HTTPS callback")
		}
		http.SetCookie(w, &http.Cookie{Name: "__Host-gimbal-session", Value: "secret", Path: "/", Secure: true, HttpOnly: true, Expires: time.Now().Add(time.Hour)})
	}))
	defer backend.Close()
	server, _ := url.Parse(backend.URL)
	// Use the fixture CA while exercising the production proxy's HTTPS path.
	oldTransport := http.DefaultTransport
	http.DefaultTransport = backend.Client().Transport
	defer func() { http.DefaultTransport = oldTransport }()
	store := &memorySessionStore{}
	proxy, err := newServerProxy(server, origin, store)
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	proxy.ServeHTTP(w, httptest.NewRequest("POST", origin+"/begin", nil))
	cookies := w.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != "gimbal-flow" || cookies[0].Secure || !cookies[0].HttpOnly {
		t.Fatal("HTTPS flow not usable by ephemeral loopback webview")
	}
	r := httptest.NewRequest("GET", origin+"/auth/callback", nil)
	r.AddCookie(cookies[0])
	store.err = errors.New("Keychain locked")
	w = httptest.NewRecorder()
	proxy.ServeHTTP(w, r)
	if w.Code != 503 || strings.Contains(w.Body.String(), "secret") || len(w.Result().Cookies()) != 0 {
		t.Fatal("Keychain failure reported login success or leaked credential")
	}
}

func TestDesktopProxyDiscardsExpiredLogin(t *testing.T) {
	server, _ := url.Parse("http://localhost:8080")
	raw, _ := json.Marshal(savedSession{Name: "gimbal-session", Value: "expired", Expires: time.Now().Add(-time.Minute)})
	store := &memorySessionStore{raw: raw}
	if _, err := newServerProxy(server, "http://127.0.0.1:9876", store); err != nil || len(store.raw) != 0 {
		t.Fatalf("expired credential was not removed: %v", err)
	}
	server, _ = url.Parse("http://remote.example")
	if _, err := newServerProxy(server, "http://127.0.0.1:9876", store); err == nil {
		t.Fatal("accepted remote HTTP for saved credential")
	}
}
