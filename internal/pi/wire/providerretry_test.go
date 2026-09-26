package wire

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"
)

func providerError(status int, headers http.Header) error {
	return &ProviderHTTPError{Err: errors.New("provider error"), Status: status, Headers: headers}
}

func headerRetryAfterMs(value string) http.Header {
	return http.Header{"Retry-After-Ms": []string{value}}
}

func TestRetryProviderRequestRetriesRetryableErrors(t *testing.T) {
	calls := 0
	request := func() (string, error) {
		calls++
		if calls == 1 {
			return "", providerError(http.StatusTooManyRequests, headerRetryAfterMs("1"))
		}
		return "ok", nil
	}
	result, err := RetryProviderRequest(context.Background(), request, ProviderRetryOptions{MaxRetries: 1})
	if err != nil {
		t.Fatalf("RetryProviderRequest error = %v", err)
	}
	if result != "ok" {
		t.Fatalf("result = %q, want ok", result)
	}
	if calls != 2 {
		t.Fatalf("calls = %d, want 2", calls)
	}
}

func TestRetryProviderRequestHonorsXShouldRetryFalse(t *testing.T) {
	requestErr := providerError(http.StatusTooManyRequests, http.Header{"X-Should-Retry": []string{"false"}})
	calls := 0
	request := func() (string, error) {
		calls++
		return "", requestErr
	}
	_, err := RetryProviderRequest(context.Background(), request, ProviderRetryOptions{MaxRetries: 2})
	if !errors.Is(err, requestErr) {
		t.Fatalf("error = %v, want the original provider error", err)
	}
	if calls != 1 {
		t.Fatalf("calls = %d, want 1", calls)
	}
}

func TestRetryProviderRequestRejectsExcessiveRetryDelay(t *testing.T) {
	calls := 0
	request := func() (string, error) {
		calls++
		return "", providerError(http.StatusTooManyRequests, http.Header{"Retry-After": []string{"277403"}})
	}
	maxDelay := 1000
	_, err := RetryProviderRequest(context.Background(), request, ProviderRetryOptions{MaxRetries: 1, MaxRetryDelayMs: &maxDelay})
	if err == nil {
		t.Fatal("expected an error for an excessive retry delay")
	}
	if !strings.Contains(err.Error(), "Server requested 277403s retry delay (max: 1s)") {
		t.Fatalf("error = %q, want the fail-fast message", err)
	}
	if calls != 1 {
		t.Fatalf("calls = %d, want 1", calls)
	}
}

func TestRetryProviderRequestCanDisableDelayCap(t *testing.T) {
	calls := 0
	request := func() (string, error) {
		calls++
		if calls == 1 {
			return "", providerError(http.StatusTooManyRequests, http.Header{"Retry-After": []string{"0.001"}})
		}
		return "ok", nil
	}
	maxDelay := 0
	result, err := RetryProviderRequest(context.Background(), request, ProviderRetryOptions{MaxRetries: 1, MaxRetryDelayMs: &maxDelay})
	if err != nil {
		t.Fatalf("RetryProviderRequest error = %v", err)
	}
	if result != "ok" || calls != 2 {
		t.Fatalf("result=%q calls=%d, want ok and 2", result, calls)
	}
}

func TestRetryProviderRequestAbortsRetryDelay(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	requestCalled := make(chan struct{}, 1)
	calls := 0
	request := func() (string, error) {
		calls++
		select {
		case requestCalled <- struct{}{}:
		default:
		}
		return "", providerError(http.StatusTooManyRequests, headerRetryAfterMs("5000"))
	}
	maxDelay := 0
	done := make(chan error, 1)
	go func() {
		_, err := RetryProviderRequest(ctx, request, ProviderRetryOptions{MaxRetries: 2, MaxRetryDelayMs: &maxDelay})
		done <- err
	}()
	select {
	case <-requestCalled:
	case <-time.After(time.Second):
		t.Fatal("request was not called")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, ErrProviderAborted) {
			t.Fatalf("error = %v, want ErrProviderAborted", err)
		}
	case <-time.After(time.Second):
		t.Fatal("RetryProviderRequest did not return after cancel")
	}
	if calls != 1 {
		t.Fatalf("calls = %d, want 1", calls)
	}
}
