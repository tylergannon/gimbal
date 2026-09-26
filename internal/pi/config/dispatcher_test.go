package config

import (
	"net/http"
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

func TestHTTPProxyIsClientLocal(t *testing.T) {
	for _, name := range []string{"HTTP_PROXY", "HTTPS_PROXY", "http_proxy", "https_proxy", "NO_PROXY", "no_proxy"} {
		t.Setenv(name, "")
	}
	first := NewHTTPClient(HTTPClientOptions{HTTPProxy: "http://first:8080"})
	second := NewHTTPClient(HTTPClientOptions{HTTPProxy: "http://second:8080"})
	request, err := http.NewRequest(http.MethodGet, "https://router.diffusion.io/v1/models", nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		client *http.Client
		want   string
	}{{first, "http://first:8080"}, {second, "http://second:8080"}} {
		got, err := tc.client.Transport.(*http.Transport).Proxy(request)
		if err != nil || got == nil || got.String() != tc.want {
			t.Fatalf("proxy = %v, %v; want %s", got, err, tc.want)
		}
	}
	if os.Getenv("HTTP_PROXY") != "" || os.Getenv("HTTPS_PROXY") != "" {
		t.Fatal("client changed process proxy")
	}
}

func TestNewHTTPClient(t *testing.T) {
	client := NewHTTPClient(HTTPClientOptions{IdleTimeoutMs: 30000})
	if client == nil || client.Transport == nil {
		t.Fatal("client or transport is nil")
	}
}
