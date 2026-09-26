package config

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/http/httpproxy"
)

// DefaultHTTPIdleTimeoutMs is pi's default HTTP header/body idle timeout.
const DefaultHTTPIdleTimeoutMs = 300_000

// HTTPIdleTimeoutChoice is one selectable idle-timeout option.
type HTTPIdleTimeoutChoice struct {
	Label     string
	TimeoutMs int
}

// HTTPIdleTimeoutChoices are the offered idle-timeout options, in display order.
var HTTPIdleTimeoutChoices = []HTTPIdleTimeoutChoice{
	{Label: "30 sec", TimeoutMs: 30_000},
	{Label: "1 min", TimeoutMs: 60_000},
	{Label: "2 min", TimeoutMs: 120_000},
	{Label: "5 min", TimeoutMs: 300_000},
	{Label: "disabled", TimeoutMs: 0},
}

// ParseHTTPIdleTimeoutMs normalizes a settings value into a timeout in
// milliseconds. It accepts a number, a numeric string, or the literal
// "disabled" (0). It returns (0, false) for absent, empty or invalid values.
func ParseHTTPIdleTimeoutMs(value any) (int, bool) {
	switch typed := value.(type) {
	case nil:
		return 0, false
	case string:
		trimmed := strings.TrimSpace(typed)
		if strings.EqualFold(trimmed, "disabled") {
			return 0, true
		}
		if trimmed == "" {
			return 0, false
		}
		number, err := strconv.ParseFloat(trimmed, 64)
		if err != nil {
			return 0, false
		}
		return parseTimeoutNumber(number)
	case bool:
		return 0, false
	default:
		number, ok := toFloat64(typed)
		if !ok {
			return 0, false
		}
		return parseTimeoutNumber(number)
	}
}

func parseTimeoutNumber(number float64) (int, bool) {
	if number != number || number < 0 || number > float64(int(^uint(0)>>1)) {
		return 0, false
	}
	return int(number), true
}

func toFloat64(value any) (float64, bool) {
	switch typed := value.(type) {
	case int:
		return float64(typed), true
	case int32:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case float32:
		return float64(typed), true
	case float64:
		return typed, true
	case json.Number:
		number, err := typed.Float64()
		if err != nil {
			return 0, false
		}
		return number, true
	}
	return 0, false
}

// FormatHTTPIdleTimeoutMs renders a timeout for display.
func FormatHTTPIdleTimeoutMs(timeoutMs int) string {
	for _, choice := range HTTPIdleTimeoutChoices {
		if choice.TimeoutMs == timeoutMs {
			return choice.Label
		}
	}
	return fmt.Sprintf("%d sec", timeoutMs/1000)
}

// HTTPClientOptions configures a Pi HTTP client.
type HTTPClientOptions struct {
	// IdleTimeoutMs bounds the wait for response headers and idle keep-alive
	// connections. Zero disables the timeout.
	IdleTimeoutMs int
	// HTTPProxy supplies a client-local default when the environment has no proxy.
	HTTPProxy string
	// Transport overrides the transport. When nil a proxied transport is used.
	Transport http.RoundTripper
}

// NewHTTPClient returns an HTTP client configured with pi's idle-timeout and
// proxy behavior.
func NewHTTPClient(options HTTPClientOptions) *http.Client {
	client := &http.Client{}
	if options.Transport != nil {
		client.Transport = options.Transport
		return client
	}
	proxyConfig := httpproxy.FromEnvironment()
	if proxyConfig.HTTPProxy == "" {
		proxyConfig.HTTPProxy = strings.TrimSpace(options.HTTPProxy)
	}
	if proxyConfig.HTTPSProxy == "" {
		proxyConfig.HTTPSProxy = strings.TrimSpace(options.HTTPProxy)
	}
	proxy := proxyConfig.ProxyFunc()
	timeout := time.Duration(options.IdleTimeoutMs) * time.Millisecond
	transport := &http.Transport{
		Proxy:                 func(req *http.Request) (*url.URL, error) { return proxy(req.URL) },
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: time.Second,
	}
	if options.IdleTimeoutMs > 0 {
		transport.ResponseHeaderTimeout = timeout
	}
	client.Transport = transport
	return client
}
