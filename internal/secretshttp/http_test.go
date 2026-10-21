package secretshttp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"filippo.io/age"
	"github.com/tylergannon/gimbal/backend"
)

func fixture(t *testing.T) (directory, identityFile, tokenFile, token string) {
	t.Helper()
	dir := t.TempDir()
	identity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatal(err)
	}
	identityFile, tokenFile = filepath.Join(dir, "identity"), filepath.Join(dir, "token")
	token = strings.Repeat("fixture-token-", 4)
	if err := os.WriteFile(identityFile, []byte(identity.String()), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(tokenFile, []byte(token), 0600); err != nil {
		t.Fatal(err)
	}
	return filepath.Join(dir, "data"), identityFile, tokenFile, token
}

func TestEncryptedHTTPStoreSurvivesRestart(t *testing.T) {
	directory, identityFile, tokenFile, token := fixture(t)
	store, err := Open(directory, identityFile)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(Handler(store, token))
	defer server.Close()
	client, err := Connect(Config{server.URL, tokenFile})
	if err != nil {
		t.Fatal(err)
	}
	ctx := t.Context()
	root := backend.Scope{Run: "one", Scope: "root"}
	child := backend.Scope{Run: "one", Scope: "root/child"}
	write := backend.StoreWrite{Store: "db", Values: []backend.NamedValue{{Name: "URL", Value: "sensitive-address-912"}, {Name: "PASSWORD", Value: "sensitive-password-847"}}}
	if err := client.Publish(ctx, root, write); err != nil {
		t.Fatal(err)
	}
	if err := client.Publish(ctx, child, backend.StoreWrite{Store: "db", Values: []backend.NamedValue{{Name: "URL", Value: "child-address"}}}); err != nil {
		t.Fatal(err)
	}
	if err := client.Publish(ctx, backend.Scope{Run: "two", Scope: "root"}, write); err != nil {
		t.Fatal(err)
	}
	// Reopen with the same identity, replacing both server and client.
	server.Close()
	store, err = Open(directory, identityFile)
	if err != nil {
		t.Fatal(err)
	}
	server = httptest.NewServer(Handler(store, token))
	defer server.Close()
	client, err = Connect(Config{server.URL, tokenFile})
	if err != nil {
		t.Fatal(err)
	}
	requirements := backend.Requirements{{Store: "db", Names: []string{"URL"}}}
	values, err := client.Acquire(ctx, child, requirements)
	if err != nil || values["URL"] != "child-address" || values["PASSWORD"] != "sensitive-password-847" {
		t.Fatalf("whole-store acquisition/shadowing: %v %v", values, err)
	}
	if _, err := client.Acquire(ctx, backend.Scope{Run: "other", Scope: "root"}, requirements); err == nil {
		t.Fatal("cross-run values visible")
	}
	if err := client.Release(ctx, child); err != nil {
		t.Fatal(err)
	}
	values, err = client.Acquire(ctx, child, requirements)
	if err != nil || values["URL"] != "sensitive-address-912" {
		t.Fatal("parent lost on child release", err)
	}
	if err := client.Release(ctx, root); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Acquire(ctx, child, requirements); err == nil {
		t.Fatal("released values still visible")
	}
	if _, err := client.Acquire(ctx, backend.Scope{Run: "two", Scope: "root"}, requirements); err != nil {
		t.Fatal("release removed another run", err)
	}
	files, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		raw, err := os.ReadFile(filepath.Join(directory, file.Name()))
		if err != nil {
			t.Fatal(err)
		}
		for _, value := range write.Values {
			if bytes.Contains(raw, []byte(value.Value)) {
				t.Fatalf("plaintext persisted in %s", file.Name())
			}
		}
		info, err := file.Info()
		if err != nil || info.Mode().Perm() != 0600 {
			t.Fatal("private file permissions", err)
		}
	}
	identity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatal(err)
	}
	wrong := filepath.Join(t.TempDir(), "wrong")
	if err := os.WriteFile(wrong, []byte(identity.String()), 0600); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(filepath.Join(directory, "values.age"))
	if _, err := Open(directory, wrong); err == nil {
		t.Fatal("wrong identity accepted")
	}
	after, _ := os.ReadFile(filepath.Join(directory, "values.age"))
	if !bytes.Equal(before, after) {
		t.Fatal("wrong identity replaced the store")
	}
	after[len(after)-1] ^= 1
	if err := os.WriteFile(filepath.Join(directory, "values.age"), after, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(directory, identityFile); err == nil {
		t.Fatal("damaged ciphertext accepted")
	}
}

func TestHTTPAuthenticationAndRequestValidation(t *testing.T) {
	_, _, tokenFile, token := fixture(t)
	server := httptest.NewServer(Handler(backend.NewMemorySecrets(), token))
	defer server.Close()
	for _, test := range []struct {
		auth, origin, body string
		code               int
	}{
		{"", "", `{}`, 401},
		{"Bearer wrong", "", `{}`, 401},
		{"Bearer " + token, "https://example.com", `{}`, 403},
		{"Bearer " + token, "", `{"unexpected":"private-payload"}`, 400},
		{"Bearer " + token, "", `{} {}`, 400},
	} {
		req, err := http.NewRequest(http.MethodPost, server.URL+"/publish", strings.NewReader(test.body))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Authorization", test.auth)
		req.Header.Set("Origin", test.origin)
		response, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(response.Body)
		_ = response.Body.Close()
		if err != nil || response.StatusCode != test.code || strings.Contains(string(body), "private-payload") {
			t.Fatalf("status=%d body=%q error=%v", response.StatusCode, body, err)
		}
		if response.Header.Get("Cache-Control") != "no-store" {
			t.Fatal("response may be cached")
		}
	}
	client, err := Connect(Config{server.URL, tokenFile})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := client.Publish(ctx, backend.Scope{}, backend.StoreWrite{}); err == nil {
		t.Fatal("canceled request succeeded")
	}
	redirected := false
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { redirected = true }))
	defer target.Close()
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
	}))
	defer redirect.Close()
	client, err = Connect(Config{redirect.URL, tokenFile})
	if err != nil {
		t.Fatal(err)
	}
	if err := client.Publish(t.Context(), backend.Scope{}, backend.StoreWrite{}); err == nil || redirected {
		t.Fatal("followed redirect or accepted it")
	}
}

func TestConcurrentPublicationsAndFailedBatch(t *testing.T) {
	directory, identityFile, tokenFile, token := fixture(t)
	store, err := Open(directory, identityFile)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(Handler(store, token))
	defer server.Close()
	client, err := Connect(Config{server.URL, tokenFile})
	if err != nil {
		t.Fatal(err)
	}
	scope := backend.Scope{Run: "run"}
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Go(func() {
			err := client.Publish(t.Context(), scope, backend.StoreWrite{Store: "db", Values: []backend.NamedValue{{Name: fmt.Sprintf("N%d", i), Value: "present"}}})
			if err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	if err := client.Publish(t.Context(), scope, backend.StoreWrite{Store: "db", Values: []backend.NamedValue{{Name: "NEW", Value: "bad-batch"}, {Name: "N0", Value: "duplicate"}}}); err == nil {
		t.Fatal("duplicate accepted")
	}
	values, err := client.Acquire(t.Context(), scope, backend.Requirements{{Store: "db"}})
	if err != nil || len(values) != 12 {
		t.Fatalf("lost publication or partial batch: %v %v", values, err)
	}
}

func TestConfigurationKeepsCredentialsOutOfSelection(t *testing.T) {
	for _, raw := range []string{
		`{"Endpoint":"http://user:password@localhost","TokenFile":"/token"}`,
		`{"Endpoint":"http://localhost?token=secret","TokenFile":"/token"}`,
		`{"Endpoint":"http://localhost","TokenFile":"relative"}`,
		`{"Endpoint":"http://localhost","TokenFile":"/token","Token":"secret"}`,
		`{"Endpoint":"http://localhost","TokenFile":"/token"} {}`,
	} {
		if _, err := ParseConfig(json.RawMessage(raw)); err == nil {
			t.Fatal("accepted invalid config", raw)
		}
	}
}
