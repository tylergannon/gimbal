// Package secretshttp connects trusted Gimbal executors to an encrypted secrets
// store. Bearer credentials authorize the executor, not individual workflows.
package secretshttp

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/tylergannon/gimbal/backend"
)

type Config struct {
	Endpoint  string
	TokenFile string
}

func ParseConfig(raw json.RawMessage) (Config, error) {
	var config Config
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(&config); err != nil {
		return config, fmt.Errorf("invalid HTTP secrets configuration")
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return config, fmt.Errorf("invalid HTTP secrets configuration")
	}
	u, err := url.Parse(config.Endpoint)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return config, fmt.Errorf("secrets endpoint must be an http(s) origin without credentials, query or path")
	}
	if !filepath.IsAbs(config.TokenFile) {
		return config, fmt.Errorf("secrets token file must be absolute")
	}
	config.Endpoint = strings.TrimRight(config.Endpoint, "/")
	return config, nil
}

// ReadToken reads an externally provisioned credential. Configuration retains
// only this path, so backend selections can safely travel with run metadata.
func ReadToken(filename string) (string, error) {
	if !filepath.IsAbs(filename) {
		return "", fmt.Errorf("secrets token file must be absolute")
	}
	raw, err := os.ReadFile(filename)
	if err != nil {
		return "", fmt.Errorf("read secrets token file: %w", err)
	}
	token := strings.TrimSpace(string(raw))
	if len(token) < 32 {
		return "", fmt.Errorf("secrets token must contain at least 32 printable ASCII characters without whitespace")
	}
	for _, c := range token {
		if c < 33 || c > 126 {
			return "", fmt.Errorf("secrets token must contain at least 32 printable ASCII characters without whitespace")
		}
	}
	return token, nil
}

type request struct {
	Scope        backend.Scope
	Write        backend.StoreWrite
	Requirements backend.Requirements
}

// Handler admits trusted executors using one deployment credential. Executors
// enforce workflow dependencies; this API does not provide per-run isolation
// against a malicious authenticated executor.
func Handler(store backend.SecretsBackend, token string) http.Handler {
	expected := sha256.Sum256([]byte("Bearer " + token))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		actual := sha256.Sum256([]byte(r.Header.Get("Authorization")))
		if token == "" || subtle.ConstantTimeCompare(actual[:], expected[:]) != 1 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if r.Header.Get("Origin") != "" {
			http.Error(w, "browser origins are not accepted", http.StatusForbidden)
			return
		}
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var input request
		d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<20))
		d.DisallowUnknownFields()
		if err := d.Decode(&input); err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		if err := d.Decode(new(any)); err != io.EOF {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		var values map[string]string
		var err error
		switch r.URL.Path {
		case "/publish":
			err = store.Publish(r.Context(), input.Scope, input.Write)
		case "/acquire":
			values, err = store.Acquire(r.Context(), input.Scope, input.Requirements)
		case "/release":
			err = store.Release(r.Context(), input.Scope)
		default:
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if err != nil {
			// Never echo request data, decrypted data or backend parser errors.
			http.Error(w, "secrets operation failed", http.StatusUnprocessableEntity)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(values)
	})
}

type Client struct {
	endpoint, token string
	http            *http.Client
}

func Connect(config Config) (*Client, error) {
	// Validate even when constructed directly, rather than through the registry.
	raw, err := json.Marshal(config)
	if err != nil {
		return nil, err
	}
	config, err = ParseConfig(raw)
	if err != nil {
		return nil, err
	}
	token, err := ReadToken(config.TokenFile)
	if err != nil {
		return nil, err
	}
	return &Client{config.Endpoint, token, &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}}, nil
}

func (c *Client) call(ctx context.Context, operation string, input request) (map[string]string, error) {
	raw, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint+"/"+operation, bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("create secrets request")
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	response, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("secrets %s: %w", operation, err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("secrets %s: HTTP %d", operation, response.StatusCode)
	}
	var values map[string]string
	if err := json.NewDecoder(io.LimitReader(response.Body, 64<<20)).Decode(&values); err != nil {
		return nil, fmt.Errorf("invalid secrets response")
	}
	return values, nil
}

func (c *Client) Publish(ctx context.Context, scope backend.Scope, write backend.StoreWrite) error {
	_, err := c.call(ctx, "publish", request{Scope: scope, Write: write})
	return err
}
func (c *Client) Acquire(ctx context.Context, scope backend.Scope, requirements backend.Requirements) (map[string]string, error) {
	return c.call(ctx, "acquire", request{Scope: scope, Requirements: requirements})
}
func (c *Client) Release(ctx context.Context, scope backend.Scope) error {
	_, err := c.call(ctx, "release", request{Scope: scope})
	return err
}
