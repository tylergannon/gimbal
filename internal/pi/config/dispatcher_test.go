package config

import (
	"os"
	"testing"
)

func TestParseHTTPIdleTimeoutMs(t *testing.T) {
	if got, ok := ParseHTTPIdleTimeoutMs("disabled"); !ok || got != 0 {
		t.Fatalf("disabled = %d, %v", got, ok)
	}
	if got, ok := ParseHTTPIdleTimeoutMs("60000"); !ok || got != 60000 {
		t.Fatalf("numeric string = %d, %v", got, ok)
	}
	if got, ok := ParseHTTPIdleTimeoutMs(float64(300000)); !ok || got != 300000 {
		t.Fatalf("number = %d, %v", got, ok)
	}
	if got, ok := ParseHTTPIdleTimeoutMs(-1); ok {
		t.Fatalf("negative = %d, %v; want invalid", got, ok)
	}
	if got, ok := ParseHTTPIdleTimeoutMs("abc"); ok {
		t.Fatalf("non-numeric = %d, %v; want invalid", got, ok)
	}
	if got, ok := ParseHTTPIdleTimeoutMs(nil); ok {
		t.Fatalf("nil = %d, %v; want invalid", got, ok)
	}
}

func TestFormatHTTPIdleTimeoutMs(t *testing.T) {
	cases := map[int]string{
		30000:  "30 sec",
		60000:  "1 min",
		120000: "2 min",
		300000: "5 min",
		0:      "disabled",
		45000:  "45 sec",
	}
	for input, want := range cases {
		if got := FormatHTTPIdleTimeoutMs(input); got != want {
			t.Fatalf("format(%d) = %q; want %q", input, got, want)
		}
	}
}

func TestApplyHTTPProxySettings(t *testing.T) {
	t.Setenv("HTTP_PROXY", "")
	t.Setenv("HTTPS_PROXY", "")
	ApplyHTTPProxySettings("http://127.0.0.1:7890")
	if got := os.Getenv("HTTP_PROXY"); got != "http://127.0.0.1:7890" {
		t.Fatalf("HTTP_PROXY = %q", got)
	}
	if got := os.Getenv("HTTPS_PROXY"); got != "http://127.0.0.1:7890" {
		t.Fatalf("HTTPS_PROXY = %q", got)
	}

	t.Setenv("HTTP_PROXY", "http://env-http:8080")
	t.Setenv("HTTPS_PROXY", "http://env-https:8080")
	ApplyHTTPProxySettings("http://settings:7890")
	if got := os.Getenv("HTTP_PROXY"); got != "http://env-http:8080" {
		t.Fatalf("HTTP_PROXY overridden: %q", got)
	}
	if got := os.Getenv("HTTPS_PROXY"); got != "http://env-https:8080" {
		t.Fatalf("HTTPS_PROXY overridden: %q", got)
	}
}

func TestNewHTTPClient(t *testing.T) {
	client := NewHTTPClient(HTTPClientOptions{IdleTimeoutMs: 30000})
	if client == nil || client.Transport == nil {
		t.Fatal("client or transport is nil")
	}
}
