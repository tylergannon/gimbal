package wire

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"net/http"
	"strconv"
	"time"
)

// Provider request retry, ported from pi packages/ai/src/utils/provider-retry.ts
// (upstream d6af72e1). It reproduces the retry behavior of the pinned OpenAI
// and Anthropic SDKs while making their backoff sleep interruptible.

// DefaultMaxRetryDelayMs is the ceiling on a server-requested Retry-After
// delay. Set ProviderRetryOptions.MaxRetryDelayMs to zero to disable it.
const DefaultMaxRetryDelayMs = 60_000

// ErrProviderAborted is returned when the context is done before or during a
// retry wait (pi's createAbortError). Its message is pi's "Request aborted".
var ErrProviderAborted error = providerAbortedError{}

type providerAbortedError struct{}

func (providerAbortedError) Error() string { return "Request aborted" }

// serverRetryDelayError carries pi's fail-fast message for a server-requested
// delay above the configured cap.
type serverRetryDelayError struct{ message string }

func (e *serverRetryDelayError) Error() string { return e.message }

// ProviderStatusError is implemented by provider request errors that carry the
// HTTP status and response headers the retry loop classifies. A provider error
// with no status is treated as retryable, matching the SDKs.
type ProviderStatusError interface {
	error
	// ProviderStatus returns the HTTP status code; ok is false when the error
	// carries no status.
	ProviderStatus() (status int, ok bool)
	// ProviderHeaders returns the response headers, or nil.
	ProviderHeaders() http.Header
}

// ProviderHTTPError is a ProviderStatusError wrapping an underlying request
// error with its HTTP status and response headers.
type ProviderHTTPError struct {
	Err     error
	Status  int
	Headers http.Header
}

// Error implements error.
func (e *ProviderHTTPError) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}
	return fmt.Sprintf("provider request failed with status %d", e.Status)
}

// Unwrap exposes the underlying error.
func (e *ProviderHTTPError) Unwrap() error { return e.Err }

// ProviderStatus implements ProviderStatusError.
func (e *ProviderHTTPError) ProviderStatus() (int, bool) { return e.Status, e.Status != 0 }

// ProviderHeaders implements ProviderStatusError.
func (e *ProviderHTTPError) ProviderHeaders() http.Header { return e.Headers }

// ProviderRetryOptions configures RetryProviderRequest.
type ProviderRetryOptions struct {
	// MaxRetries caps retry attempts; zero means a single attempt.
	MaxRetries int
	// MaxRetryDelayMs caps a server-requested retry delay. Nil takes
	// DefaultMaxRetryDelayMs; zero disables the cap.
	MaxRetryDelayMs *int
}

func providerStatusError(err error) (ProviderStatusError, bool) {
	if pe, ok := errors.AsType[ProviderStatusError](err); ok {
		return pe, true
	}
	return nil, false
}

// isRetryableProviderError implements the pinned OpenAI/Anthropic SDK retry
// matrix on a provider error.
func isRetryableProviderError(err ProviderStatusError) bool {
	headers := err.ProviderHeaders()
	if headers != nil {
		switch headers.Get("x-should-retry") {
		case "true":
			return true
		case "false":
			return false
		}
	}
	status, ok := err.ProviderStatus()
	if !ok {
		return true
	}
	return status == http.StatusRequestTimeout ||
		status == http.StatusConflict ||
		status == http.StatusTooManyRequests ||
		status >= 500
}

func validateServerRetryDelayMs(delayMs float64, maxRetryDelayMs *int, providerErrorMessage string) (float64, error) {
	maxDelayMs := float64(DefaultMaxRetryDelayMs)
	if maxRetryDelayMs != nil {
		maxDelayMs = float64(*maxRetryDelayMs)
	}
	if maxDelayMs > 0 && delayMs > maxDelayMs {
		return 0, &serverRetryDelayError{message: fmt.Sprintf("Server requested %ss retry delay (max: %ss). %s",
			ceilSeconds(delayMs), ceilSeconds(maxDelayMs), providerErrorMessage)}
	}
	return delayMs, nil
}

func ceilSeconds(ms float64) string {
	seconds := math.Ceil(ms / 1000)
	if math.IsInf(seconds, 1) {
		return "Infinity"
	}
	return strconv.FormatFloat(seconds, 'f', -1, 64)
}

func getRetryDelayMs(err ProviderStatusError, retryIndex int, maxRetryDelayMs *int) (float64, error) {
	headers := err.ProviderHeaders()
	if headers != nil {
		if value := headers.Get("retry-after-ms"); value != "" {
			if parsed, parseErr := strconv.ParseFloat(value, 64); parseErr == nil {
				return validateServerRetryDelayMs(parsed, maxRetryDelayMs, err.Error())
			}
		}
		if value := headers.Get("retry-after"); value != "" {
			if seconds, parseErr := strconv.ParseFloat(value, 64); parseErr == nil {
				return validateServerRetryDelayMs(seconds*1000, maxRetryDelayMs, err.Error())
			}
			if when, parseErr := http.ParseTime(value); parseErr == nil {
				return validateServerRetryDelayMs(float64(time.Until(when).Milliseconds()), maxRetryDelayMs, err.Error())
			}
			// A present but unparseable Retry-After is still server-dictated: pi's
			// Date.parse yields NaN, which its sleep clamps to an immediate retry.
			return validateServerRetryDelayMs(math.NaN(), maxRetryDelayMs, err.Error())
		}
	}
	exponentialDelay := math.Min(0.5*math.Pow(2, float64(retryIndex)), 8) * 1000
	return exponentialDelay * (1 - rand.Float64()*0.25), nil
}

func abortableSleep(ctx context.Context, ms float64) error {
	if ms < 0 || math.IsNaN(ms) {
		ms = 0
	}
	timer := time.NewTimer(time.Duration(ms) * time.Millisecond)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ErrProviderAborted
	case <-timer.C:
		return nil
	}
}

// RetryProviderRequest runs request with bounded retries on transient provider
// errors, sleeping between attempts. Each retry is a fresh request; when the
// context is done the request fails with ErrProviderAborted.
//
// When options.MaxRetries is zero exactly one attempt is made. A server-requested
// retry delay above options.MaxRetryDelayMs fails immediately.
func RetryProviderRequest[T any](ctx context.Context, request func() (T, error), options ProviderRetryOptions) (T, error) {
	maxRetries := options.MaxRetries
	retriesRemaining := maxRetries

	for {
		value, err := request()
		if err == nil {
			return value, nil
		}
		if ctx.Err() != nil {
			var zero T
			return zero, ErrProviderAborted
		}
		providerErr, ok := providerStatusError(err)
		if retriesRemaining <= 0 || !ok || !isRetryableProviderError(providerErr) {
			var zero T
			return zero, err
		}

		retryIndex := maxRetries - retriesRemaining
		retriesRemaining--
		delayMs, delayErr := getRetryDelayMs(providerErr, retryIndex, options.MaxRetryDelayMs)
		if delayErr != nil {
			var zero T
			return zero, delayErr
		}
		if err := abortableSleep(ctx, delayMs); err != nil {
			var zero T
			return zero, err
		}
	}
}
