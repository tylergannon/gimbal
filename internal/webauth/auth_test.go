package webauth

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tylergannon/gimbal/auth"
)

type testUser string

func (u testUser) ID() string   { return string(u) }
func (u testUser) Name() string { return "Tyler" }

type testUsers struct {
	mu       sync.Mutex
	users    map[string]auth.User
	sessions map[string]auth.Session
	fail     bool
}

func (s *testUsers) CreateUser(_ context.Context, id auth.Identity) (auth.User, error) {
	return nil, errors.New("callback must never create users")
}
func (s *testUsers) UserForIdentity(ctx context.Context, issuer, subject string) (auth.User, error) {
	if issuer != "https://github.com" || subject != "50593" {
		return nil, auth.ErrNotFound
	}
	return s.User(ctx, "tyler")
}
func (s *testUsers) User(_ context.Context, id string) (auth.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.fail {
		return nil, errors.New("storage unavailable")
	}
	u, ok := s.users[id]
	if !ok {
		return nil, auth.ErrNotFound
	}
	return u, nil
}
func (s *testUsers) DeleteUser(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.users, id)
	return nil
}
func (s *testUsers) CreateSession(_ context.Context, v auth.Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.fail {
		return errors.New("storage unavailable")
	}
	s.sessions[v.Hash] = v
	return nil
}
func (s *testUsers) Session(_ context.Context, id string) (auth.Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.fail {
		return auth.Session{}, errors.New("storage unavailable")
	}
	v, ok := s.sessions[id]
	if !ok {
		return v, auth.ErrNotFound
	}
	return v, nil
}
func (s *testUsers) DeleteSession(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.fail {
		return errors.New("storage unavailable")
	}
	delete(s.sessions, id)
	return nil
}

type testProvider struct {
	subject string
	used    bool
	mu      sync.Mutex
}

func (p *testProvider) AuthorizationURL(state, challenge, nonce string) (string, error) {
	return "https://github.com/login/oauth/authorize?state=" + state + "&code_challenge=" + challenge, nil
}
func (p *testProvider) Exchange(ctx context.Context, code, verifier, nonce string) (auth.Identity, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.used || code != "valid" || len(verifier) != 43 {
		return auth.Identity{}, errors.New("invalid or replayed code")
	}
	p.used = true
	return auth.Identity{Issuer: "https://github.com", Subject: p.subject}, nil
}
func fixture(t *testing.T) (*Auth, *testUsers, *testProvider) {
	t.Helper()
	b, _ := aes.NewCipher(make([]byte, 32))
	aead, _ := cipher.NewGCM(b)
	s := &testUsers{users: map[string]auth.User{"tyler": testUser("tyler")}, sessions: map[string]auth.Session{}}
	p := &testProvider{subject: "50593"}
	return &Auth{Origin: "https://gimbal.example", Users: s, provider: p, flow: aead}, s, p
}
func callbackRequest(t *testing.T, a *Auth, stateOverride string) *http.Request {
	t.Helper()
	target, c, err := a.BeginLogin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(target)
	state := u.Query().Get("state")
	if stateOverride != "" {
		state = stateOverride
	}
	r := httptest.NewRequest("GET", "https://gimbal.example/auth/callback?code=valid&state="+state, nil)
	r.AddCookie(c)
	return r
}
func login(t *testing.T, a *Auth) *http.Cookie {
	t.Helper()
	w := httptest.NewRecorder()
	a.callback(w, callbackRequest(t, a, ""))
	if w.Code != 303 {
		t.Fatalf("login: %d %s", w.Code, w.Body.String())
	}
	for _, c := range w.Result().Cookies() {
		if c.Name == SessionCookie {
			return c
		}
	}
	t.Fatal("missing session")
	return nil
}
func TestCallbackAdmitsOnlyExistingIdentityAndChecksState(t *testing.T) {
	for _, tc := range []struct {
		name, subject, state string
		want                 int
	}{{"admitted", "50593", "", 303}, {"other account", "999", "", 403}, {"wrong state", "50593", "wrong", 400}} {
		t.Run(tc.name, func(t *testing.T) {
			a, s, p := fixture(t)
			p.subject = tc.subject
			r := callbackRequest(t, a, tc.state)
			w := httptest.NewRecorder()
			a.callback(w, r)
			if w.Code != tc.want {
				t.Fatalf("got %d: %s", w.Code, w.Body.String())
			}
			if tc.want != 303 && len(s.sessions) != 0 {
				t.Fatal("refused login created session")
			}
			if tc.want == 303 {
				w = httptest.NewRecorder()
				a.callback(w, r)
				if w.Code != 400 || len(s.sessions) != 1 {
					t.Fatal("code replay was accepted")
				}
			}
		})
	}
}
func TestSessionRotationLogoutRevocationAndFailures(t *testing.T) {
	a, s, p := fixture(t)
	first := login(t, a)
	if remaining := time.Until(first.Expires); remaining < 7*24*time.Hour-time.Minute || remaining > 7*24*time.Hour {
		t.Fatalf("session expires in %s, want one week", remaining)
	}
	hash, _ := tokenHash(first.Value)
	if remaining := time.Until(s.sessions[hash].ExpiresAt); remaining < 7*24*time.Hour-time.Minute || remaining > 7*24*time.Hour {
		t.Fatalf("stored session expires in %s, want one week", remaining)
	}
	if !first.Secure || !first.HttpOnly || first.Path != "/" || first.SameSite != http.SameSiteLaxMode {
		t.Fatal("unsafe cookie")
	}
	p.used = false
	r := callbackRequest(t, a, "")
	r.AddCookie(first)
	w := httptest.NewRecorder()
	a.callback(w, r)
	if w.Code != 303 || len(s.sessions) != 1 {
		t.Fatal("old session was not revoked")
	}
	var second *http.Cookie
	for _, c := range w.Result().Cookies() {
		if c.Name == SessionCookie {
			second = c
		}
	}
	r = httptest.NewRequest("POST", "https://gimbal.example/logout", nil)
	r.AddCookie(second)
	s.fail = true
	if a.Logout(r.Context(), r) == nil {
		t.Fatal("failed deletion reported success")
	}
	s.fail = false
	if err := a.Logout(r.Context(), r); err != nil {
		t.Fatal(err)
	}
	if _, _, err := a.session(r.Context(), r); !errors.Is(err, auth.ErrNotFound) {
		t.Fatal("logout retained session")
	}
}
func TestGatePublicBoundaryRedirectsAndOrigin(t *testing.T) {
	a, s, _ := fixture(t)
	c := login(t, a)
	called := 0
	project := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called++
		if auth.UserFrom(r.Context()) == nil {
			t.Fatal("missing user context")
		}
		w.WriteHeader(204)
	})
	anonymous := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if From(r.Context()) != a {
			t.Fatal("missing auth config")
		}
		w.WriteHeader(200)
	})
	gate := a.Gate(project, anonymous, "/_app/remote/hash/beginLogin", map[string]bool{"/_app/immutable/x.js": true})
	for _, tc := range []struct {
		path, method, mode, origin string
		cookie                     bool
		want                       int
	}{{"/auth/login", "GET", "", "", false, 200}, {"/auth/login/__data.json", "GET", "", "", false, 200}, {"/_app/immutable/x.js", "GET", "", "", false, 200}, {"/projects/unknown/files/x", "GET", "navigate", "", false, 303}, {"/projects/unknown/__data.json", "GET", "", "", false, 200}, {"/projects/x/api/runs/y/events", "GET", "", "", false, 401}, {"/_app/remote/hash/other", "POST", "", "", false, 401}, {"/projects/x", "GET", "", "", true, 204}, {"/mutation", "POST", "", "https://attacker.example", true, 403}, {"/mutation", "POST", "", a.Origin, true, 204}} {
		r := httptest.NewRequest(tc.method, "https://gimbal.example"+tc.path, nil)
		r.Header.Set("Sec-Fetch-Mode", tc.mode)
		r.Header.Set("Origin", tc.origin)
		if tc.cookie {
			r.AddCookie(c)
		}
		w := httptest.NewRecorder()
		gate.ServeHTTP(w, r)
		if w.Code != tc.want {
			t.Fatalf("%s %s got %d", tc.method, tc.path, w.Code)
		}
		if strings.HasSuffix(tc.path, "/__data.json") && tc.path != "/auth/login/__data.json" && !strings.Contains(w.Body.String(), `"type":"redirect"`) {
			t.Fatal("missing Kit data redirect")
		}
	}
	if called != 2 {
		t.Fatalf("anonymous requests reached project: %d", called)
	}
	if err := s.DeleteUser(context.Background(), "tyler"); err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("GET", "https://gimbal.example/", nil)
	r.AddCookie(c)
	w := httptest.NewRecorder()
	gate.ServeHTTP(w, r)
	if w.Code != 401 {
		t.Fatal("deleted user retained access")
	}
	s.fail = true
	w = httptest.NewRecorder()
	gate.ServeHTTP(w, r)
	if w.Code != 503 {
		t.Fatal("storage failure did not deny access")
	}
}
func TestLoopbackHTTPUsesLocalCookiesAndRemoteHTTPIsRefused(t *testing.T) {
	for _, tc := range []struct {
		origin string
		want   bool
	}{{"http://localhost:8096", true}, {"http://127.0.0.1:8096", true}, {"http://[::1]:8096", true}, {"http://gimbal.example", false}, {"http://localhost.attacker.example", false}, {"https://gimbal.example", true}} {
		u, err := url.Parse(tc.origin)
		if err != nil {
			t.Fatal(err)
		}
		if validOriginScheme(u) != tc.want {
			t.Fatalf("origin %s", tc.origin)
		}
	}
	a, _, _ := fixture(t)
	a.Origin = "http://localhost:8096"
	_, c, err := a.BeginLogin(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if c.Secure || strings.HasPrefix(c.Name, "__Host-") || !c.HttpOnly || c.Path != "/" {
		t.Fatalf("local flow cookie: %+v", c)
	}
	w := httptest.NewRecorder()
	a.callback(w, callbackRequest(t, a, ""))
	if w.Code != 303 {
		t.Fatalf("local callback got %d %s", w.Code, w.Body.String())
	}
	var session *http.Cookie
	for _, c := range w.Result().Cookies() {
		if c.Name == a.SessionCookieName() {
			session = c
		}
	}
	if session == nil || session.Secure {
		t.Fatal("local session was missing or marked Secure")
	}
	r := httptest.NewRequest("GET", a.Origin+"/", nil)
	r.AddCookie(session)
	if _, _, err := a.session(t.Context(), r); err != nil {
		t.Fatal(err)
	}
}
