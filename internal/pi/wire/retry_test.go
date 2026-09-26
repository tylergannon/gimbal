package wire

import (
	"context"
	"testing"
	"time"

	"github.com/tylergannon/gimbal/internal/pi/model"
)

func assistantError(errorMessage string) *model.AssistantMessage {
	return &model.AssistantMessage{StopReason: model.StopError, ErrorMessage: errorMessage}
}

func assistantText(text string) *model.AssistantMessage {
	return &model.AssistantMessage{
		StopReason: model.StopStop,
		Content:    model.ContentList{model.TextContent{Text: text}},
	}
}

func TestIsRetryableAssistantError(t *testing.T) {
	retryable := []string{
		"An error occurred while processing your request. You can retry your request, or contact us through our help center at help.openai.com if the error persists. Please include the request ID req_******** in your message.",
		`{"message":"The system encountered an unexpected error during processing. Try your request again."}`,
		"ResourceExhausted: Worker local total request limit reached (288/48)",
		"The socket connection was closed unexpectedly. For more information, pass `verbose: true` in the second argument to fetch()",
		"Error: exceeded request buffer limit while retrying upstream",
		"The pending stream has been canceled (caused by: getaddrinfo ENOTFOUND bedrock-runtime.us-east-1.amazonaws.com)",
		"connect ENOTFOUND api.example.com",
		"EAI_AGAIN api.example.com",
		"getaddrinfo failed for api.example.com",
		"OpenAI Responses stream ended before a terminal response event",
		"The system is currently experiencing high demand and cannot process your request. Your request exceeds the maximum usage size allowed during peak load.",
		"overloaded_error",
		"520 status code (no body)",
		"524 status code (no body)",
	}
	for _, message := range retryable {
		if !IsRetryableAssistantError(assistantError(message)) {
			t.Errorf("IsRetryableAssistantError(%q) = false, want true", message)
		}
	}

	nonRetryable := []string{
		"429 quota exceeded",
		"insufficient_quota",
	}
	for _, message := range nonRetryable {
		if IsRetryableAssistantError(assistantError(message)) {
			t.Errorf("IsRetryableAssistantError(%q) = true, want false", message)
		}
	}

	if IsRetryableAssistantError(assistantText("not an error")) {
		t.Error("a successful message must not be retryable")
	}
}

func TestRetryDelayMs(t *testing.T) {
	if got := RetryDelayMs(RetryPolicy{BaseDelayMs: 2000}, 6); got != 60000 {
		t.Errorf("RetryDelayMs(base 2000, attempt 6) = %d, want 60000", got)
	}
	capped := 5000
	if got := RetryDelayMs(RetryPolicy{BaseDelayMs: 2000, MaxAgentDelayMs: &capped}, 5); got != 5000 {
		t.Errorf("RetryDelayMs(capped 5000, attempt 5) = %d, want 5000", got)
	}
	zero := 0
	if got := RetryDelayMs(RetryPolicy{BaseDelayMs: 2000, MaxAgentDelayMs: &zero}, 5); got != 0 {
		t.Errorf("RetryDelayMs(capped 0, attempt 5) = %d, want 0", got)
	}
}

func TestRetryAssistantCallReturnsSuccessWithoutRetrying(t *testing.T) {
	calls := 0
	produce := func() *model.AssistantMessage {
		calls++
		return assistantText("ok")
	}
	result := RetryAssistantCall(context.Background(), produce, &RetryPolicy{Enabled: true, MaxRetries: 3}, nil)
	if result.Content[0].(model.TextContent).Text != "ok" {
		t.Fatalf("result content = %#v", result.Content)
	}
	if calls != 1 {
		t.Fatalf("produce called %d times, want 1", calls)
	}
}

func TestRetryAssistantCallDoesNotRetryAborted(t *testing.T) {
	calls := 0
	produce := func() *model.AssistantMessage {
		calls++
		return &model.AssistantMessage{StopReason: model.StopAborted}
	}
	scheduled := 0
	result := RetryAssistantCall(context.Background(), produce, &RetryPolicy{Enabled: true, MaxRetries: 3}, &RetryCallbacks{
		OnRetryScheduled: func(int, int, int, string) { scheduled++ },
	})
	if result.StopReason != model.StopAborted {
		t.Fatalf("stop reason = %q, want aborted", result.StopReason)
	}
	if calls != 1 || scheduled != 0 {
		t.Fatalf("calls=%d scheduled=%d, want 1 and 0", calls, scheduled)
	}
}

func TestRetryAssistantCallDoesNotRetryNonRetryable(t *testing.T) {
	calls := 0
	produce := func() *model.AssistantMessage {
		calls++
		return assistantError("insufficient_quota")
	}
	finished := 0
	result := RetryAssistantCall(context.Background(), produce, &RetryPolicy{Enabled: true, MaxRetries: 3}, &RetryCallbacks{
		OnRetryFinished: func(bool, int, string) { finished++ },
	})
	if result.StopReason != model.StopError {
		t.Fatalf("stop reason = %q, want error", result.StopReason)
	}
	if calls != 1 || finished != 0 {
		t.Fatalf("calls=%d finished=%d, want 1 and 0", calls, finished)
	}
}

func TestRetryAssistantCallRetriesThenReturnsFinalError(t *testing.T) {
	calls := 0
	produce := func() *model.AssistantMessage {
		calls++
		return assistantError("terminated")
	}
	var scheduled [][2]int
	var finished []any
	result := RetryAssistantCall(context.Background(), produce, &RetryPolicy{Enabled: true, MaxRetries: 3}, &RetryCallbacks{
		OnRetryScheduled: func(attempt, maxAttempts, delayMs int, _ string) {
			scheduled = append(scheduled, [2]int{attempt, delayMs})
		},
		OnRetryFinished: func(success bool, attempt int, finalError string) {
			finished = []any{success, attempt, finalError}
		},
	})
	if result.StopReason != model.StopError {
		t.Fatalf("stop reason = %q, want error", result.StopReason)
	}
	if calls != 4 {
		t.Fatalf("produce called %d times, want 4", calls)
	}
	if len(scheduled) != 3 {
		t.Fatalf("scheduled %d retries, want 3", len(scheduled))
	}
	if len(finished) != 3 || finished[0] != false || finished[1] != 3 || finished[2] != "terminated" {
		t.Fatalf("finished = %#v, want false, 3, terminated", finished)
	}
}

func TestRetryAssistantCallReportsCappedDelays(t *testing.T) {
	calls := 0
	produce := func() *model.AssistantMessage {
		calls++
		if calls < 5 {
			return assistantError("terminated")
		}
		return assistantText("recovered")
	}
	capped := 15
	var delays []int
	RetryAssistantCall(context.Background(), produce, &RetryPolicy{Enabled: true, MaxRetries: 4, BaseDelayMs: 10, MaxAgentDelayMs: &capped}, &RetryCallbacks{
		OnRetryScheduled: func(_, _, delayMs int, _ string) { delays = append(delays, delayMs) },
	})
	want := []int{10, 15, 15, 15}
	if len(delays) != len(want) {
		t.Fatalf("delays = %v, want %v", delays, want)
	}
	for i := range want {
		if delays[i] != want[i] {
			t.Fatalf("delays = %v, want %v", delays, want)
		}
	}
}

func TestRetryAssistantCallStopsOnceSuccessful(t *testing.T) {
	calls := 0
	produce := func() *model.AssistantMessage {
		calls++
		if calls < 3 {
			return assistantError("terminated")
		}
		return assistantText("recovered")
	}
	var finished []any
	result := RetryAssistantCall(context.Background(), produce, &RetryPolicy{Enabled: true, MaxRetries: 3}, &RetryCallbacks{
		OnRetryFinished: func(success bool, attempt int, finalError string) {
			finished = []any{success, attempt, finalError}
		},
	})
	if result.Content[0].(model.TextContent).Text != "recovered" {
		t.Fatalf("result content = %#v", result.Content)
	}
	if calls != 3 {
		t.Fatalf("calls = %d, want 3", calls)
	}
	if len(finished) != 3 || finished[0] != true || finished[1] != 2 || finished[2] != "" {
		t.Fatalf("finished = %#v, want true, 2, empty", finished)
	}
}

func TestRetryAssistantCallReportsAbortedRetriedCall(t *testing.T) {
	calls := 0
	produce := func() *model.AssistantMessage {
		calls++
		if calls == 1 {
			return assistantError("terminated")
		}
		return &model.AssistantMessage{StopReason: model.StopAborted}
	}
	var finished []any
	result := RetryAssistantCall(context.Background(), produce, &RetryPolicy{Enabled: true, MaxRetries: 3}, &RetryCallbacks{
		OnRetryFinished: func(success bool, attempt int, finalError string) {
			finished = []any{success, attempt, finalError}
		},
	})
	if result.StopReason != model.StopAborted {
		t.Fatalf("stop reason = %q, want aborted", result.StopReason)
	}
	if calls != 2 {
		t.Fatalf("calls = %d, want 2", calls)
	}
	if len(finished) != 3 || finished[0] != false || finished[1] != 1 {
		t.Fatalf("finished = %#v, want false, 1", finished)
	}
}

func TestRetryAssistantCallDisabledPolicy(t *testing.T) {
	calls := 0
	produce := func() *model.AssistantMessage {
		calls++
		return assistantError("terminated")
	}
	scheduled, finished := 0, 0
	result := RetryAssistantCall(context.Background(), produce, &RetryPolicy{Enabled: false, MaxRetries: 3}, &RetryCallbacks{
		OnRetryScheduled: func(int, int, int, string) { scheduled++ },
		OnRetryFinished:  func(bool, int, string) { finished++ },
	})
	if result.StopReason != model.StopError {
		t.Fatalf("stop reason = %q, want error", result.StopReason)
	}
	if calls != 1 || scheduled != 0 || finished != 0 {
		t.Fatalf("calls=%d scheduled=%d finished=%d, want 1, 0, 0", calls, scheduled, finished)
	}
}

func TestRetryAssistantCallAttemptStartOrder(t *testing.T) {
	calls := 0
	produce := func() *model.AssistantMessage {
		calls++
		if calls < 3 {
			return assistantError("terminated")
		}
		return assistantText("recovered")
	}
	var events []string
	result := RetryAssistantCall(context.Background(), produce, &RetryPolicy{Enabled: true, MaxRetries: 3}, &RetryCallbacks{
		OnRetryScheduled:    func(attempt int, _ int, _ int, _ string) { events = append(events, "retry") },
		OnRetryAttemptStart: func() { events = append(events, "attempt-start") },
	})
	if result.Content[0].(model.TextContent).Text != "recovered" {
		t.Fatalf("result content = %#v", result.Content)
	}
	want := []string{"retry", "attempt-start", "retry", "attempt-start"}
	if len(events) != len(want) {
		t.Fatalf("events = %v, want %v", events, want)
	}
	for i := range want {
		if events[i] != want[i] {
			t.Fatalf("events = %v, want %v", events, want)
		}
	}
}

func TestRetryAssistantCallAbortsBackoff(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	produceCalled := make(chan struct{}, 1)
	produce := func() *model.AssistantMessage {
		select {
		case produceCalled <- struct{}{}:
		default:
		}
		return assistantError("terminated")
	}
	var finished []any
	done := make(chan *model.AssistantMessage, 1)
	go func() {
		done <- RetryAssistantCall(ctx, produce, &RetryPolicy{Enabled: true, MaxRetries: 5, BaseDelayMs: 10_000}, &RetryCallbacks{
			OnRetryFinished: func(success bool, attempt int, finalError string) {
				finished = []any{success, attempt, finalError}
			},
		})
	}()
	select {
	case <-produceCalled:
	case <-time.After(time.Second):
		t.Fatal("produce was not called")
	}
	cancel()
	var result *model.AssistantMessage
	select {
	case result = <-done:
	case <-time.After(time.Second):
		t.Fatal("RetryAssistantCall did not return after cancel")
	}
	if result.StopReason != model.StopAborted || result.ErrorMessage != "" {
		t.Fatalf("result = %+v, want aborted with empty error", result)
	}
	if len(finished) != 3 || finished[0] != false || finished[1] != 1 || finished[2] != "terminated" {
		t.Fatalf("finished = %#v, want false, 1, terminated", finished)
	}
}
