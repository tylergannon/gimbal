// Package webauth owns Gimbal's OAuth flow and browser sessions. Provider and
// persistence implementations are selected from native plugin registrations.
package webauth

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/auth"
)

const FlowCookie = "__Host-gimbal-flow"
const SessionCookie = "__Host-gimbal-session"

// Auth is one instance's selected provider, users and trusted public origin.
type Auth struct {
	Origin   string
	Users    auth.Users
	provider auth.Provider
	flow     cipher.AEAD
}
type selection struct {
	Name          string          `json:"name"`
	Configuration json.RawMessage `json:"configuration"`
}
type configuration struct {
	Provider selection `json:"provider"`
	Users    selection `json:"users"`
	Origin   string    `json:"origin"`
}
type contextKey struct{}

func From(ctx context.Context) *Auth { a, _ := ctx.Value(contextKey{}).(*Auth); return a }
func (a *Auth) context(ctx context.Context) context.Context {
	return context.WithValue(ctx, contextKey{}, a)
}

// Open resolves an explicit absolute auth file before any listener starts.
func Open(ctx context.Context, path string) (*Auth, error) {
	if !filepath.IsAbs(path) {
		return nil, errors.New("auth file must be an absolute path")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("auth file: %w", err)
	}
	var cfg configuration
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("auth file: %w", err)
	}
	if err := dec.Decode(new(any)); err != io.EOF {
		return nil, errors.New("auth file must contain one JSON object")
	}
	origin, err := url.Parse(cfg.Origin)
	if err != nil || !validOriginScheme(origin) || origin.Hostname() == "" || origin.User != nil || origin.Path != "" || origin.RawQuery != "" || origin.Fragment != "" {
		return nil, errors.New("auth origin must be HTTPS, or HTTP on localhost/loopback, without a path")
	}
	if other := os.Getenv("GIMBAL_WEB_ORIGIN"); other != "" && other != cfg.Origin {
		return nil, errors.New("auth origin conflicts with GIMBAL_WEB_ORIGIN")
	}
	pd, ok := gimbal.DefaultRegistry.Provider(cfg.Provider.Name)
	if !ok {
		return nil, fmt.Errorf("unknown auth provider %q", cfg.Provider.Name)
	}
	ud, ok := gimbal.DefaultRegistry.Users(cfg.Users.Name)
	if !ok {
		return nil, fmt.Errorf("unknown auth users plugin %q", cfg.Users.Name)
	}
	if err := pd.ValidateConfiguration(cfg.Provider.Configuration); err != nil {
		return nil, err
	}
	if err := ud.ValidateConfiguration(cfg.Users.Configuration); err != nil {
		return nil, err
	}
	p, err := pd.Connect(ctx, cfg.Provider.Configuration, cfg.Origin+"/auth/callback")
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, errors.New("auth provider returned nil")
	}
	a := &Auth{Origin: cfg.Origin, provider: p}
	users, err := ud.Connect(ctx, cfg.Users.Configuration)
	if err != nil {
		return nil, errors.Join(err, a.Close())
	}
	a.Users = users
	if users == nil {
		return nil, errors.Join(errors.New("auth users plugin returned nil"), a.Close())
	}
	key := make([]byte, 32)
	if _, err = rand.Read(key); err != nil {
		return nil, errors.Join(err, a.Close())
	}
	block, err := aes.NewCipher(key)
	if err == nil {
		a.flow, err = cipher.NewGCM(block)
	}
	if err != nil {
		return nil, errors.Join(err, a.Close())
	}
	return a, nil
}
func validOriginScheme(u *url.URL) bool {
	if u == nil {
		return false
	}
	if u.Scheme == "https" {
		return true
	}
	ip := net.ParseIP(u.Hostname())
	return u.Scheme == "http" && (u.Hostname() == "localhost" || ip != nil && ip.IsLoopback())
}
func (a *Auth) cookieName(name string) string {
	if strings.HasPrefix(a.Origin, "https://") {
		return name
	}
	return strings.TrimPrefix(name, "__Host-")
}

// SessionCookieName is the host-only session cookie used by this origin.
func (a *Auth) SessionCookieName() string { return a.cookieName(SessionCookie) }

func (a *Auth) Close() error {
	var errs []error
	if c, ok := a.Users.(io.Closer); ok {
		errs = append(errs, c.Close())
	}
	if c, ok := a.provider.(io.Closer); ok {
		errs = append(errs, c.Close())
	}
	return errors.Join(errs...)
}

type pendingFlow struct {
	State, Verifier, Nonce string
	Expires                int64
}

func randomToken() string {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}
func (a *Auth) cookie(name, value string, expiry time.Time) *http.Cookie {
	return &http.Cookie{Name: a.cookieName(name), Value: value, Path: "/", HttpOnly: true, Secure: strings.HasPrefix(a.Origin, "https://"), SameSite: http.SameSiteLaxMode, Expires: expiry}
}
func (a *Auth) clearCookie(w http.ResponseWriter, name string) {
	c := a.cookie(name, "", time.Unix(1, 0))
	c.MaxAge = -1
	http.SetCookie(w, c)
}
func (a *Auth) BeginLogin(ctx context.Context) (string, *http.Cookie, error) {
	if err := ctx.Err(); err != nil {
		return "", nil, err
	}
	flow := pendingFlow{State: randomToken(), Verifier: randomToken(), Nonce: randomToken(), Expires: time.Now().Add(5 * time.Minute).Unix()}
	challenge := sha256.Sum256([]byte(flow.Verifier))
	target, err := a.provider.AuthorizationURL(flow.State, base64.RawURLEncoding.EncodeToString(challenge[:]), flow.Nonce)
	if err != nil {
		return "", nil, err
	}
	data, err := json.Marshal(flow)
	if err != nil {
		return "", nil, err
	}
	nonce := make([]byte, a.flow.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return "", nil, err
	}
	sealed := a.flow.Seal(nonce, nonce, data, []byte(FlowCookie))
	return target, a.cookie(FlowCookie, base64.RawURLEncoding.EncodeToString(sealed), time.Unix(flow.Expires, 0)), nil
}
func tokenHash(token string) (string, error) {
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(raw) != 32 {
		return "", auth.ErrNotFound
	}
	h := sha256.Sum256(raw)
	return hex.EncodeToString(h[:]), nil
}
func (a *Auth) session(ctx context.Context, r *http.Request) (auth.Session, auth.User, error) {
	c, err := r.Cookie(a.SessionCookieName())
	if err != nil {
		return auth.Session{}, nil, auth.ErrNotFound
	}
	hash, err := tokenHash(c.Value)
	if err != nil {
		return auth.Session{}, nil, err
	}
	s, err := a.Users.Session(ctx, hash)
	if err != nil {
		return s, nil, err
	}
	if s.Hash != hash || s.UserID == "" || !time.Now().Before(s.ExpiresAt) {
		return s, nil, auth.ErrNotFound
	}
	u, err := a.Users.User(ctx, s.UserID)
	if err != nil {
		return s, nil, err
	}
	if u == nil || u.ID() == "" || u.ID() != s.UserID {
		return s, nil, auth.ErrNotFound
	}
	return s, u, nil
}
func (a *Auth) Logout(ctx context.Context, r *http.Request) error {
	s, _, err := a.session(ctx, r)
	if err != nil {
		return err
	}
	return a.Users.DeleteSession(ctx, s.Hash)
}
func (a *Auth) callback(w http.ResponseWriter, r *http.Request) {
	a.clearCookie(w, FlowCookie)
	refuse := func(status int, msg string) { http.Error(w, msg, status) }
	c, err := r.Cookie(a.cookieName(FlowCookie))
	if err != nil || len(c.Value) > 2048 {
		refuse(400, "Login expired; please sign in again.")
		return
	}
	raw, err := base64.RawURLEncoding.DecodeString(c.Value)
	if err != nil || len(raw) < a.flow.NonceSize() {
		refuse(400, "Invalid login flow.")
		return
	}
	data, err := a.flow.Open(nil, raw[:a.flow.NonceSize()], raw[a.flow.NonceSize():], []byte(FlowCookie))
	var flow pendingFlow
	if err != nil || json.Unmarshal(data, &flow) != nil || time.Now().Unix() >= flow.Expires || flow.State == "" || subtle.ConstantTimeCompare([]byte(flow.State), []byte(r.URL.Query().Get("state"))) != 1 {
		refuse(400, "Login expired or invalid; please sign in again.")
		return
	}
	if r.URL.Query().Get("error") != "" || r.URL.Query().Get("code") == "" {
		refuse(400, "GitHub login was declined; please sign in again.")
		return
	}
	identity, err := a.provider.Exchange(r.Context(), r.URL.Query().Get("code"), flow.Verifier, flow.Nonce)
	if err != nil || identity.Issuer == "" || identity.Subject == "" {
		refuse(400, "GitHub login could not be verified; please sign in again.")
		return
	}
	u, err := a.Users.UserForIdentity(r.Context(), identity.Issuer, identity.Subject)
	if errors.Is(err, auth.ErrNotFound) {
		refuse(403, "This GitHub account is not admitted to this Gimbal instance.")
		return
	}
	if err != nil || u == nil || u.ID() == "" {
		refuse(503, "User storage is unavailable.")
		return
	}
	token := randomToken()
	hash, _ := tokenHash(token)
	session := auth.Session{Hash: hash, UserID: u.ID(), ExpiresAt: time.Now().Add(7 * 24 * time.Hour)}
	if err := a.Users.CreateSession(r.Context(), session); err != nil {
		refuse(503, "Session storage is unavailable.")
		return
	}
	if old, err := r.Cookie(a.SessionCookieName()); err == nil {
		if oldHash, err := tokenHash(old.Value); err == nil {
			if err := a.Users.DeleteSession(r.Context(), oldHash); err != nil {
				_ = a.Users.DeleteSession(r.Context(), hash)
				refuse(503, "Previous session could not be revoked.")
				return
			}
		}
	}
	http.SetCookie(w, a.cookie(SessionCookie, token, session.ExpiresAt))
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// Gate wraps the whole application. Public dispatch bypasses the project boundary.
// public contains exact generated beginLogin and static client paths.
func (a *Auth) Gate(protected, anonymous http.Handler, beginLogin string, static map[string]bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r = r.WithContext(a.context(r.Context()))
		path := r.URL.Path
		isRead := r.Method == http.MethodGet || r.Method == http.MethodHead
		if isRead && static[path] {
			anonymous.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		if r.Method == http.MethodGet && path == "/auth/callback" {
			a.callback(w, r)
			return
		}
		if r.Method == http.MethodGet && (path == "/auth/login" || path == "/auth/login/__data.json") || r.Method == http.MethodPost && path == beginLogin {
			anonymous.ServeHTTP(w, r)
			return
		}
		_, u, err := a.session(r.Context(), r)
		if err != nil {
			if !errors.Is(err, auth.ErrNotFound) {
				http.Error(w, "Authentication storage is unavailable.", http.StatusServiceUnavailable)
				return
			}
			if r.Method == http.MethodGet && strings.HasSuffix(path, "/__data.json") {
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(struct {
					Type     string `json:"type"`
					Status   int    `json:"status"`
					Location string `json:"location"`
				}{"redirect", 303, "/auth/login"})
				return
			}
			if isRead && (r.Header.Get("Sec-Fetch-Mode") == "navigate" || r.Header.Get("Sec-Fetch-Dest") == "document") {
				http.Redirect(w, r, "/auth/login", http.StatusSeeOther)
				return
			}
			http.Error(w, "Sign in to Gimbal.", http.StatusUnauthorized)
			return
		}
		if !isRead && r.Header.Get("Origin") != a.Origin {
			http.Error(w, "Request origin is not allowed.", http.StatusForbidden)
			return
		}
		ctx, cancel := context.WithCancel(auth.WithUser(r.Context(), u))
		defer cancel()
		done := make(chan struct{})
		defer close(done)
		go func() {
			tick := time.NewTicker(30 * time.Second)
			defer tick.Stop()
			for {
				select {
				case <-done:
					return
				case <-ctx.Done():
					return
				case <-tick.C:
					if _, _, err := a.session(ctx, r); err != nil {
						cancel()
						return
					}
				}
			}
		}()
		protected.ServeHTTP(w, r.WithContext(ctx))
	})
}
