package wire

import (
	"context"
	"fmt"
	"math"
	"math/bits"
	"regexp"
	"strings"
	"time"

	"github.com/tylergannon/gimbal/internal/pi/model"
)

// Assistant-call retry and provider-error classification, ported from pi
// packages/ai/src/utils/retry.ts (upstream d6af72e1).

// providerErrorPattern is a JavaScript RegExp without the "u" flag as pi's
// buildProviderErrorPattern builds it: patterns joined with "|" and matched
// case-insensitively. A JS RegExp's "i" folds ASCII letters only, so the
// alternatives are compiled lowercased and the input is ASCII-lowercased before
// matching; "." becomes a class that excludes the same characters a JS "."
// excludes.
type providerErrorPattern struct {
	re *regexp.Regexp
}

// jsDot is "." of a JavaScript RegExp without the "u" flag, restricted to the
// text between ASCII literals these patterns use.
const jsDot = `[^\n\r\x{2028}\x{2029}\x{10000}-\x{10FFFF}]`

func buildProviderErrorPattern(patterns []string) providerErrorPattern {
	alternatives := make([]string, len(patterns))
	for i, pattern := range patterns {
		if strings.ContainsAny(pattern, `\^$|*+()[]{}`) {
			panic(fmt.Sprintf("wire: provider error pattern %q uses RegExp syntax beyond literals, \".\" and \"?\"; "+
				"teach buildProviderErrorPattern its JavaScript semantics before adding it", pattern))
		}
		alternatives[i] = strings.ReplaceAll(asciiLower(pattern), ".", jsDot)
	}
	return providerErrorPattern{re: regexp.MustCompile(strings.Join(alternatives, "|"))}
}

// MatchString is RegExp.prototype.test on s.
func (p providerErrorPattern) MatchString(s string) bool {
	return p.re.MatchString(asciiLower(s))
}

// asciiLower lowercases A-Z and leaves every other byte as it is.
func asciiLower(s string) string {
	for i := 0; i < len(s); i++ {
		if 'A' <= s[i] && s[i] <= 'Z' {
			b := []byte(s)
			for j := i; j < len(b); j++ {
				if 'A' <= b[j] && b[j] <= 'Z' {
					b[j] += 'a' - 'A'
				}
			}
			return string(b)
		}
	}
	return s
}

// nonRetryableProviderLimitErrorPattern matches provider error text that
// indicates a subscription/account/billing limit rather than a transient
// failure. Matches here suppress retries.
var nonRetryableProviderLimitErrorPattern = buildProviderErrorPattern([]string{
	"GoUsageLimitError",
	"FreeUsageLimitError",
	"Monthly usage limit reached",
	"available balance",
	"insufficient_quota",
	"out of budget",
	"quota exceeded",
	"billing",
})

// retryableProviderErrorPattern matches provider/transport error text that
// looks like a transient failure worth retrying.
var retryableProviderErrorPattern = buildProviderErrorPattern([]string{
	"overloaded",
	"currently experiencing high demand",
	"rate.?limit",
	"too many requests",
	"429",
	"500",
	"502",
	"503",
	"504",
	"520",
	"524",
	"service.?unavailable",
	"server.?error",
	"internal.?error",
	"provider.?returned.?error",
	"exceeded request buffer limit while retrying upstream",
	"network.?error",
	"connection.?error",
	"connection.?refused",
	"connection.?lost",
	"other side closed",
	"fetch failed",
	"getaddrinfo",
	"ENOTFOUND",
	"EAI_AGAIN",
	"upstream.?connect",
	"reset before headers",
	"socket hang up",
	"socket connection was closed",
	"timed? out",
	"timeout",
	"terminated",
	"websocket.?closed",
	"websocket.?error",
	"ended without",
	"stream ended before message_stop",
	"stream ended before a terminal response event",
	"http2 request did not get a response",
	"retry delay",
	"you can retry your request",
	"try your request again",
	"please retry your request",
	"ResourceExhausted",
})

// IsRetryableAssistantError classifies whether a failed assistant message looks
// like a transient provider or transport error, so callers can decide if the
// last assistant turn should be restarted. It does not implement retry policy.
func IsRetryableAssistantError(message *model.AssistantMessage) bool {
	if message == nil || message.StopReason != model.StopError || message.ErrorMessage == "" {
		return false
	}
	if nonRetryableProviderLimitErrorPattern.MatchString(message.ErrorMessage) {
		return false
	}
	return retryableProviderErrorPattern.MatchString(message.ErrorMessage)
}

// RetryPolicy is bounded retry with exponential backoff
// (BaseDelayMs * 2^(attempt-1)), capped at MaxAgentDelayMs. It mirrors pi's
// RetryPolicy / settings.retry.
type RetryPolicy struct {
	Enabled bool
	// MaxRetries is the max retry attempts (0 = no retries). The initial call
	// never counts as a retry.
	MaxRetries int
	// BaseDelayMs is the base backoff, before MaxAgentDelayMs caps it.
	BaseDelayMs int
	// MaxAgentDelayMs caps each computed backoff. Nil takes
	// DefaultMaxAgentRetryDelayMs; zero is a real cap of zero.
	MaxAgentDelayMs *int
}

// DefaultMaxAgentRetryDelayMs is pi's DEFAULT_MAX_AGENT_RETRY_DELAY_MS.
const DefaultMaxAgentRetryDelayMs = 60_000

// RetryDelayMs is pi's retryDelayMs: the backoff for one attempt, capped.
func RetryDelayMs(policy RetryPolicy, attempt int) int {
	maxDelayMs := DefaultMaxAgentRetryDelayMs
	if policy.MaxAgentDelayMs != nil {
		maxDelayMs = *policy.MaxAgentDelayMs
	}
	delayMs := shiftSaturating(policy.BaseDelayMs, attempt-1)
	if delayMs > maxDelayMs {
		return maxDelayMs
	}
	return delayMs
}

// shiftSaturating returns base * 2^shift, saturating to math.MaxInt on overflow
// rather than wrapping. A negative shift is pi's Math.max(0, attempt-1).
func shiftSaturating(base, shift int) int {
	if shift <= 0 || base == 0 {
		return base
	}
	if shift >= bits.UintSize {
		return math.MaxInt
	}
	shifted := base << uint(shift)
	if shifted>>uint(shift) != base {
		return math.MaxInt
	}
	return shifted
}

// RetryCallbacks are the optional hooks RetryAssistantCall emits around each
// retry. The whole value may be nil.
type RetryCallbacks struct {
	// OnRetryScheduled fires before the backoff sleep of each retry attempt
	// (1-indexed).
	OnRetryScheduled func(attempt, maxAttempts, delayMs int, errorMessage string)
	// OnRetryAttemptStart fires after the backoff sleep, immediately before the
	// retried call starts.
	OnRetryAttemptStart func()
	// OnRetryFinished fires once when the loop ends; success is true if a later
	// call completed normally.
	OnRetryFinished func(success bool, attempt int, finalError string)
}

func (c *RetryCallbacks) scheduled(attempt, maxAttempts, delayMs int, errorMessage string) {
	if c != nil && c.OnRetryScheduled != nil {
		c.OnRetryScheduled(attempt, maxAttempts, delayMs, errorMessage)
	}
}

func (c *RetryCallbacks) attemptStart() {
	if c != nil && c.OnRetryAttemptStart != nil {
		c.OnRetryAttemptStart()
	}
}

func (c *RetryCallbacks) finished(success bool, attempt int, finalError string) {
	if c != nil && c.OnRetryFinished != nil {
		c.OnRetryFinished(success, attempt, finalError)
	}
}

// RetryAssistantCall runs a single assistant-producing call with bounded retry
// on transient errors (pi retryAssistantCall).
//
// A successful response returns immediately. Aborts are terminal and never
// retried, but reported as unsuccessful if they happen after a retry was
// scheduled. An abort during the backoff sleep is normalized to an aborted
// message. A non-retryable error returns immediately. Otherwise it retries up
// to policy.MaxRetries times; when policy is nil or disabled the first response
// is returned unchanged. produce must return a non-nil message.
func RetryAssistantCall(ctx context.Context, produce func() *model.AssistantMessage, policy *RetryPolicy, callbacks *RetryCallbacks) *model.AssistantMessage {
	maxAttempts := 0
	if policy != nil && policy.Enabled {
		maxAttempts = policy.MaxRetries
	}

	attempt := 0
	lastRetryScheduled := false
	lastRetryAttempt := 0
	for {
		response := produce()

		if response.StopReason == model.StopAborted {
			if lastRetryScheduled {
				callbacks.finished(false, lastRetryAttempt, "")
			}
			return response
		}

		if response.StopReason != model.StopError {
			if lastRetryScheduled {
				callbacks.finished(true, lastRetryAttempt, "")
			}
			return response
		}

		if attempt >= maxAttempts || !IsRetryableAssistantError(response) {
			if lastRetryScheduled {
				callbacks.finished(false, lastRetryAttempt, response.ErrorMessage)
			}
			return response
		}

		attempt++
		lastRetryScheduled = true
		lastRetryAttempt = attempt
		errorMessage := response.ErrorMessage
		if errorMessage == "" {
			errorMessage = "Unknown error"
		}
		delayMs := RetryDelayMs(*policy, attempt)
		callbacks.scheduled(attempt, maxAttempts, delayMs, errorMessage)

		timer := time.NewTimer(time.Duration(delayMs) * time.Millisecond)
		select {
		case <-timer.C:
		case <-ctx.Done():
			timer.Stop()
			callbacks.finished(false, attempt, errorMessage)
			aborted := *response
			aborted.StopReason = model.StopAborted
			aborted.ErrorMessage = ""
			return &aborted
		}
		callbacks.attemptStart()
	}
}
