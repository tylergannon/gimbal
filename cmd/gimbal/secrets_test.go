package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveSecretsAddsDopplerValuesWithoutReplacingLocalOnes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v3/configs/config/secrets/download" || r.URL.Query().Get("format") != "json" {
			t.Errorf("Doppler request URL = %s", r.URL)
		}
		if r.Header.Get("Authorization") != "Bearer local-token" {
			t.Error("Doppler request did not use the configured token")
		}
		_, _ = w.Write([]byte(`{"DIFFUSION_API_KEY":"from-doppler","EXPLICIT":"from-doppler"}`))
	}))
	defer server.Close()

	filename := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(filename, []byte(`{"uploadArtifact":{"provider":"r2"},"secrets":{"DOPPLER_TOKEN":"local-token","EXPLICIT":"from-file"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	lookup := func(name string) (string, bool) {
		if name == "DIFFUSION_API_KEY" {
			return "from-environment", true
		}
		return "", false
	}
	secrets, err := resolveSecrets(t.Context(), filename, lookup, server.Client(), server.URL+"/v3/configs/config/secrets/download?format=json")
	if err != nil {
		t.Fatal(err)
	}
	if secrets["DOPPLER_TOKEN"] != "local-token" || secrets["EXPLICIT"] != "from-file" || secrets["DIFFUSION_API_KEY"] != "from-doppler" {
		t.Fatalf("resolved secret precedence or loading is wrong: names = %v", secretNames(secrets))
	}
	if value, exists := lookup("DIFFUSION_API_KEY"); !exists || value != "from-environment" {
		t.Fatal("ambient environment override changed")
	}
}

func TestResolveSecretsReportsDopplerFailureWithoutCredential(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "private response secret", http.StatusUnauthorized)
	}))
	defer server.Close()
	filename := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(filename, []byte(`{"secrets":{"DOPPLER_TOKEN":"private-token"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := resolveSecrets(context.Background(), filename, func(string) (string, bool) { return "", false }, server.Client(), server.URL)
	if err == nil || !strings.Contains(err.Error(), "HTTP 401") || strings.Contains(err.Error(), "private-token") || strings.Contains(err.Error(), "private response secret") {
		t.Fatalf("Doppler error = %v", err)
	}
}

func TestLoadCommandSecretsMakesConfigValuesAvailableToAdapters(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("DOPPLER_TOKEN", "")
	t.Setenv("GIMBAL_TEST_SECRET", "ambient")
	if err := os.MkdirAll(filepath.Join(home, ".gimbal"), 0o700); err != nil {
		t.Fatal(err)
	}
	filename := filepath.Join(home, ".gimbal", "config.json")
	if err := os.WriteFile(filename, []byte(`{"secrets":{"GIMBAL_TEST_SECRET":"from-file","GIMBAL_TEST_NEW_SECRET":"from-file"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIMBAL_TEST_NEW_SECRET", "")
	if err := os.Unsetenv("GIMBAL_TEST_NEW_SECRET"); err != nil {
		t.Fatal(err)
	}
	if err := loadCommandSecrets(t.Context()); err != nil {
		t.Fatal(err)
	}
	if got := os.Getenv("GIMBAL_TEST_SECRET"); got != "ambient" {
		t.Fatalf("ambient secret replaced: %q", got)
	}
	if got := os.Getenv("GIMBAL_TEST_NEW_SECRET"); got != "from-file" {
		t.Fatalf("config secret not made available: %q", got)
	}
}

func TestNeedsSecrets(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want bool
	}{
		{nil, true},
		{[]string{"run", "implementation"}, true},
		{[]string{"run-prompt", "--model", "pi/diffusion/deepseek-4.1-flash", "hello"}, true},
		{[]string{"upload-artifact", "proof.png"}, true},
		{[]string{"runs"}, false},
		{[]string{"opencode", "start"}, true},
		{[]string{"opencode", "stop"}, false},
		{[]string{"--help"}, false},
		{[]string{"run", "--help"}, false},
	} {
		if got := needsSecrets(tc.args); got != tc.want {
			t.Errorf("needsSecrets(%v) = %t, want %t", tc.args, got, tc.want)
		}
	}
}

func secretNames(secrets map[string]string) []string {
	names := make([]string, 0, len(secrets))
	for name := range secrets {
		names = append(names, name)
	}
	return names
}
