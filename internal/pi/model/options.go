package model

import "net/http"

// ProviderRequestOptions are the authentication, HTTP transport and lifecycle
// options every provider request carries.
type ProviderRequestOptions struct {
	APIKey string
	// OnPayload inspects or replaces a provider payload before it is sent.
	// Returning a nil payload keeps it unchanged.
	OnPayload func(payload any, model *Model) (any, error)
	// OnResponse is invoked after an HTTP response is received.
	OnResponse func(resp ProviderResponse, model *Model) error
	// Headers are custom HTTP headers merged into the provider request. A nil
	// value suppresses a provider/API default header of the same name.
	Headers ProviderHeaders
	// TimeoutMs bounds the wait for a response's headers. Zero means the
	// provider's default.
	TimeoutMs int
	// MaxRetries caps client-side retry attempts.
	MaxRetries int
	// MaxRetryDelayMs caps the delay honored when a server asks for a long
	// wait; nil takes the 60s default, and a zero value disables the cap.
	MaxRetryDelayMs *int
	// HTTPClient overrides the client used for provider HTTP requests. It is
	// the Go stand-in for pi's injectable fetch.
	HTTPClient HTTPDoer
	// Env holds provider-scoped environment overrides.
	Env ProviderEnv
}

// HTTPDoer performs a provider HTTP request. *http.Client satisfies it.
type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// StreamOptions are the options shared by all streaming provider requests.
type StreamOptions struct {
	ProviderRequestOptions
	// OnProviderStreamEvent observes each parsed provider stream event before
	// pi normalizes it.
	OnProviderStreamEvent func(data any, model *Model) error
	Temperature           *float64
	// SamplingParams are arbitrary sampling parameters merged into the request
	// body as-is.
	SamplingParams            map[string]any
	MaxTokens                 *int
	Transport                 Transport
	CacheRetention            CacheRetention
	SessionID                 string
	WebSocketConnectTimeoutMs int
	Metadata                  map[string]any
}

// SimpleStreamOptions extends StreamOptions with unified reasoning controls.
type SimpleStreamOptions struct {
	StreamOptions
	Reasoning ThinkingLevel
	// ToolChoice selects whether the model may call tools.
	ToolChoice ToolChoice
	// Deferred asks a capable provider to return a DeferredHandle.
	Deferred *DeferredRequest
	// ThinkingBudgets are custom token budgets for thinking levels.
	ThinkingBudgets *ThinkingBudgets
}
