package main

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/auth"
)

func testProvider(t *testing.T, handler http.Handler) *provider {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return &provider{
		clientID: "Iv1.example", clientSecret: "secret-to-not-log", callback: "https://gimbal.example/auth/callback",
		client:           &http.Client{Timeout: time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }},
		authorizationURL: server.URL + "/authorize", tokenURL: server.URL + "/token", userURL: server.URL + "/user",
	}
}

func TestGitHubAppPKCEExchangeAndStableNumericIdentity(t *testing.T) {
	var tokenCalls, userCalls atomic.Int32
	verifier := strings.Repeat("v", 43)
	p := testProvider(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			if r.Method != http.MethodPost || r.Header.Get("Accept") != "application/json" {
				t.Error("token exchange did not use POST JSON response")
			}
			if err := r.ParseForm(); err != nil {
				t.Error(err)
			}
			want := url.Values{"client_id": {"Iv1.example"}, "client_secret": {"secret-to-not-log"}, "code": {"once"}, "redirect_uri": {"https://gimbal.example/auth/callback"}, "code_verifier": {verifier}}
			if r.PostForm.Encode() != want.Encode() || r.URL.RawQuery != "" {
				t.Error("token exchange omitted exact callback/PKCE or put credentials in the URL")
			}
			if tokenCalls.Add(1) != 1 {
				_, _ = w.Write([]byte(`{"error":"bad_verification_code","error_description":"secret-to-not-log"}`))
				return
			}
			_, _ = w.Write([]byte(`{"access_token":"ghu_private_token","token_type":"bearer"}`))
		case "/user":
			userCalls.Add(1)
			if r.Header.Get("Authorization") != "Bearer ghu_private_token" {
				t.Error("identity was not fetched using the exchanged token")
			}
			_, _ = w.Write([]byte(`{"id":9007199254740993,"login":"tylergannon","name":"Tyler"}`))
		default:
			t.Error("unexpected request:", r.URL.Path)
		}
	}))
	challenge := sha256.Sum256([]byte(verifier))
	location, err := p.AuthorizationURL("random-state", base64.RawURLEncoding.EncodeToString(challenge[:]), "ignored-github-nonce")
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(location)
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	if q.Get("client_id") != p.clientID || q.Get("redirect_uri") != p.callback || q.Get("state") != "random-state" || q.Get("code_challenge_method") != "S256" || len(q) != 5 {
		t.Fatal("authorization parameters wrong or extra permissions requested:", q)
	}
	identity, err := p.Exchange(context.Background(), "once", verifier, "ignored-github-nonce")
	if err != nil || identity != (auth.Identity{Issuer: "https://github.com", Subject: "9007199254740993", Name: "Tyler"}) {
		t.Fatalf("verified identity incorrect: %+v, %v", identity, err)
	}
	if _, err := p.Exchange(context.Background(), "once", verifier, ""); err == nil || strings.Contains(err.Error(), "secret-to-not-log") {
		t.Fatal("replayed code was accepted or provider body leaked:", err)
	}
	if tokenCalls.Load() != 2 || userCalls.Load() != 1 {
		t.Fatal("replay reused cached identity instead of exchanging again")
	}
}

func TestConcurrentCodeExchangeReliesOnProviderOneUse(t *testing.T) {
	var tokenCalls atomic.Int32
	p := testProvider(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/token" {
			if tokenCalls.Add(1) == 1 {
				_, _ = w.Write([]byte(`{"access_token":"ghu_valid","token_type":"bearer"}`))
			} else {
				_, _ = w.Write([]byte(`{"error":"bad_verification_code"}`))
			}
			return
		}
		_, _ = w.Write([]byte(`{"id":1,"login":"operator"}`))
	}))
	var successes atomic.Int32
	var workers sync.WaitGroup
	for range 2 {
		workers.Go(func() {
			if identity, err := p.Exchange(context.Background(), "same-code", strings.Repeat("v", 43), ""); err == nil {
				if identity.Name != "operator" {
					t.Error("login display-name fallback missing")
				}
				successes.Add(1)
			}
		})
	}
	workers.Wait()
	if successes.Load() != 1 || tokenCalls.Load() != 2 {
		t.Fatal("concurrent replay bypassed provider one-use exchange")
	}
}

func TestGitHubRejectsInvalidProviderResults(t *testing.T) {
	for _, tc := range []struct {
		name, token, user string
		status            int
	}{
		{"denied", `{"error":"access_denied"}`, "", 200},
		{"missing token", `{}`, "", 200},
		{"bad token type", `{"access_token":"token","token_type":"other"}`, "", 200},
		{"invalid token JSON", "bad JSON secret-to-not-log", "", 200},
		{"failed token endpoint", "secret-to-not-log", "", 503},
		{"zero user ID", `{"access_token":"token","token_type":"bearer"}`, `{"id":0,"login":"operator"}`, 200},
		{"fractional user ID", `{"access_token":"token","token_type":"bearer"}`, `{"id":1.2,"login":"operator"}`, 200},
		{"missing login", `{"access_token":"token","token_type":"bearer"}`, `{"id":1}`, 200},
		{"oversized user body", `{"access_token":"token","token_type":"bearer"}`, strings.Repeat("x", 65537), 200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := testProvider(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/token" {
					w.WriteHeader(tc.status)
					_, _ = w.Write([]byte(tc.token))
					return
				}
				_, _ = w.Write([]byte(tc.user))
			}))
			identity, err := p.Exchange(context.Background(), "code", strings.Repeat("v", 43), "")
			if err == nil || identity != (auth.Identity{}) || strings.Contains(err.Error(), "secret-to-not-log") {
				t.Fatalf("unverified identity admitted or response leaked: %+v, %v", identity, err)
			}
		})
	}
}

func TestGitHubContextCancellation(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	p := testProvider(t, http.HandlerFunc(func(http.ResponseWriter, *http.Request) { close(started); <-release }))
	defer close(release)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := p.Exchange(ctx, "code", strings.Repeat("v", 43), "")
		done <- err
	}()
	<-started
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatal("in-flight provider request ignored cancellation:", err)
	}
}

func TestGitHubRegistrationAndConfiguration(t *testing.T) {
	var registry gimbal.Registry
	if err := Register(&registry); err != nil {
		t.Fatal(err)
	}
	d, ok := registry.Provider("github")
	if !ok {
		t.Fatal("github plugin not registered")
	}
	for _, raw := range []string{`{}`, `null`, `{"clientID":"x"}`, `{"clientID":"x","clientSecretEnv":"1_INVALID"}`, `{"clientID":"x","clientSecretEnv":"SECRET","secret":"plaintext"}`, `{} {}`} {
		if d.ValidateConfiguration(json.RawMessage(raw)) == nil {
			t.Fatal("invalid configuration accepted:", raw)
		}
	}
	t.Setenv("GIMBAL_TEST_GITHUB_SECRET", "test-secret")
	raw := json.RawMessage(`{"clientID":"Iv1.example","clientSecretEnv":"GIMBAL_TEST_GITHUB_SECRET"}`)
	for _, callback := range []string{"https://gimbal.example/auth/callback", "http://localhost:8080/auth/callback", "http://127.0.0.1:8080/auth/callback", "http://[::1]:8080/auth/callback"} {
		if _, err := d.Connect(context.Background(), raw, callback); err != nil {
			t.Fatal("valid callback refused:", callback, err)
		}
	}
	for _, callback := range []string{"http://gimbal.example/auth/callback", "http://localhost.evil.example/auth/callback", "http://192.168.1.1/auth/callback", "https://user:pass@gimbal.example/auth/callback", "https://gimbal.example/auth/callback?x=1", "https://gimbal.example/auth/callback#fragment", "not a URL"} {
		if _, err := d.Connect(context.Background(), raw, callback); err == nil {
			t.Fatal("invalid callback accepted:", callback)
		}
	}
	t.Setenv("GIMBAL_TEST_GITHUB_SECRET", "")
	if _, err := d.Connect(context.Background(), raw, "https://gimbal.example/auth/callback"); err == nil {
		t.Fatal("missing secret accepted")
	}
}
