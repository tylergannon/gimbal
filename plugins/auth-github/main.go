// Build with go build -buildmode=plugin -o auth-github.so ./plugins/auth-github.
package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/auth"
)

type configuration struct {
	ClientID        string `json:"clientID"`
	ClientSecretEnv string `json:"clientSecretEnv"`
}

type provider struct {
	clientID, clientSecret, callback    string
	client                              *http.Client
	authorizationURL, tokenURL, userURL string
}

func parse(raw json.RawMessage) (configuration, error) {
	var c configuration
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(&c); err != nil {
		return c, errors.New("github auth configuration must contain clientID and clientSecretEnv")
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return c, errors.New("github auth configuration has trailing content")
	}
	if strings.TrimSpace(c.ClientID) == "" || strings.TrimSpace(c.ClientSecretEnv) == "" {
		return c, errors.New("github auth requires clientID and clientSecretEnv")
	}
	for i, ch := range c.ClientSecretEnv {
		if ch != '_' && (ch < 'A' || ch > 'Z') && (ch < 'a' || ch > 'z') && (i == 0 || ch < '0' || ch > '9') {
			return c, errors.New("github auth clientSecretEnv must be an environment variable name")
		}
	}
	return c, nil
}

func connect(ctx context.Context, raw json.RawMessage, callback string) (auth.Provider, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	c, err := parse(raw)
	if err != nil {
		return nil, err
	}
	u, err := url.Parse(callback)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Opaque != "" {
		return nil, errors.New("github auth callback must be an exact URL without query or fragment")
	}
	loopback := u.Hostname() == "localhost" || net.ParseIP(u.Hostname()).IsLoopback()
	if u.Scheme != "https" && (u.Scheme != "http" || !loopback) {
		return nil, errors.New("github auth callback requires HTTPS except on localhost or a loopback IP")
	}
	secret := os.Getenv(c.ClientSecretEnv)
	if strings.TrimSpace(secret) == "" {
		return nil, fmt.Errorf("github auth client secret environment variable %s is missing or blank", c.ClientSecretEnv)
	}
	return &provider{
		clientID: c.ClientID, clientSecret: secret, callback: callback,
		client:           &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }},
		authorizationURL: "https://github.com/login/oauth/authorize",
		tokenURL:         "https://github.com/login/oauth/access_token",
		userURL:          "https://api.github.com/user",
	}, nil
}

func (p *provider) AuthorizationURL(state, codeChallenge, _ string) (string, error) {
	challenge, err := base64.RawURLEncoding.DecodeString(codeChallenge)
	if state == "" || err != nil || len(challenge) != 32 || len(codeChallenge) != 43 {
		return "", errors.New("github authorization requires state and an S256 PKCE challenge")
	}
	u, err := url.Parse(p.authorizationURL)
	if err != nil {
		return "", err
	}
	q := url.Values{
		"client_id": {p.clientID}, "redirect_uri": {p.callback}, "state": {state},
		"code_challenge": {codeChallenge}, "code_challenge_method": {"S256"},
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func (p *provider) Exchange(ctx context.Context, code, codeVerifier, _ string) (auth.Identity, error) {
	if err := ctx.Err(); err != nil {
		return auth.Identity{}, err
	}
	if code == "" || len(codeVerifier) < 43 || len(codeVerifier) > 128 || strings.IndexFunc(codeVerifier, func(ch rune) bool {
		return (ch < 'a' || ch > 'z') && (ch < 'A' || ch > 'Z') && (ch < '0' || ch > '9') && !strings.ContainsRune("-._~", ch)
	}) >= 0 {
		return auth.Identity{}, errors.New("github exchange requires a code and PKCE verifier")
	}
	values := url.Values{
		"client_id": {p.clientID}, "client_secret": {p.clientSecret}, "code": {code},
		"redirect_uri": {p.callback}, "code_verifier": {codeVerifier},
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, p.tokenURL, strings.NewReader(values.Encode()))
	if err != nil {
		return auth.Identity{}, errors.New("github token request could not be created")
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Accept", "application/json")
	var token struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		Error       string `json:"error"`
	}
	if err := p.read(ctx, request, &token); err != nil {
		return auth.Identity{}, fmt.Errorf("github token exchange: %w", err)
	}
	if token.Error != "" || token.AccessToken == "" || !strings.EqualFold(token.TokenType, "bearer") {
		return auth.Identity{}, errors.New("github refused the authorization code")
	}
	request, err = http.NewRequestWithContext(ctx, http.MethodGet, p.userURL, nil)
	if err != nil {
		return auth.Identity{}, errors.New("github identity request could not be created")
	}
	request.Header.Set("Authorization", "Bearer "+token.AccessToken)
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	var user struct {
		ID    int64  `json:"id"`
		Login string `json:"login"`
		Name  string `json:"name"`
	}
	if err := p.read(ctx, request, &user); err != nil {
		return auth.Identity{}, fmt.Errorf("github identity verification: %w", err)
	}
	if user.ID <= 0 || strings.TrimSpace(user.Login) == "" {
		return auth.Identity{}, errors.New("github returned an invalid user identity")
	}
	if user.Name == "" {
		user.Name = user.Login
	}
	return auth.Identity{Issuer: "https://github.com", Subject: strconv.FormatInt(user.ID, 10), Name: user.Name}, nil
}

// Provider responses may contain credentials. Never include their bodies, URL,
// or HTTP-client errors in messages returned to the application.
func (p *provider) read(ctx context.Context, request *http.Request, result any) error {
	response, err := p.client.Do(request)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return errors.New("provider request failed")
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("provider returned HTTP %d", response.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(response.Body, 64*1024+1))
	if err != nil && ctx.Err() != nil {
		return ctx.Err()
	}
	if err != nil || len(raw) > 64*1024 {
		return errors.New("provider response unreadable or too large")
	}
	if err := json.Unmarshal(raw, result); err != nil {
		return errors.New("provider response was invalid JSON")
	}
	return nil
}

func Register(r *gimbal.Registry) error {
	return r.RegisterProvider(gimbal.ProviderDefinition{
		Name: "github", ValidateConfiguration: func(raw json.RawMessage) error { _, err := parse(raw); return err }, Connect: connect,
	})
}
