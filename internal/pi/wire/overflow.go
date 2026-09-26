package wire

import (
	"regexp"

	"github.com/tylergannon/gimbal/internal/pi/model"
)

// Context-overflow detection, ported from pi packages/ai/src/utils/overflow.ts
// (upstream d6af72e1).

var overflowPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)prompt (?:is )?too long`),
	regexp.MustCompile(`(?i)request_too_large`),
	regexp.MustCompile(`(?i)input is too long for requested model`),
	regexp.MustCompile(`(?i)exceeds the context window`),
	regexp.MustCompile(`(?i)exceeds (?:the )?(?:model'?s )?maximum context length(?: of [\d,]+ tokens?|\s*\([\d,]+\))`),
	regexp.MustCompile(`(?i)input token count.*exceeds the maximum`),
	regexp.MustCompile(`(?i)maximum prompt length is \d+`),
	regexp.MustCompile(`(?i)reduce the length of the messages`),
	regexp.MustCompile(`(?i)maximum context length is \d+ tokens`),
	regexp.MustCompile(`(?i)exceeds (?:the )?maximum allowed input length of [\d,]+ tokens?`),
	regexp.MustCompile(`(?i)input \(\d+ tokens\) is longer than the model'?s context length \(\d+ tokens\)`),
	regexp.MustCompile(`(?i)exceeds the limit of \d+`),
	regexp.MustCompile(`(?i)exceeds the available context size`),
	regexp.MustCompile(`(?i)greater than the context length`),
	regexp.MustCompile(`(?i)context window exceeds limit`),
	regexp.MustCompile(`(?i)exceeded model token limit`),
	regexp.MustCompile(`(?i)too large for model with \d+ maximum context length`),
	regexp.MustCompile(`(?i)prompt has [\d,]+ tokens?, but the configured context size is [\d,]+ tokens?`),
	regexp.MustCompile(`(?i)model_context_window_exceeded`),
	regexp.MustCompile(`(?i)prompt too long; exceeded (?:max )?context length`),
	regexp.MustCompile(`(?i)range of input length should be`),
	regexp.MustCompile(`(?i)context[_ ]length[_ ]exceeded`),
	regexp.MustCompile(`(?i)too many tokens`),
	regexp.MustCompile(`(?i)token limit exceeded`),
}

var cerebrasBodylessOverflowPattern = regexp.MustCompile(`(?i)^4(?:00|13)\s*(?:status code)?\s*\(no body\)`)

var nonOverflowPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)^(Throttling error|Service unavailable):`),
	regexp.MustCompile(`(?i)rate limit`),
	regexp.MustCompile(`(?i)too many requests`),
}

// IsContextOverflow reports whether an assistant message represents a context
// overflow error, by error message, by silent usage overflow when contextWindow
// is positive, or by a length stop that filled the context.
func IsContextOverflow(message *model.AssistantMessage, contextWindow int) bool {
	if message == nil {
		return false
	}
	if message.StopReason == model.StopError && message.ErrorMessage != "" {
		isNonOverflow := false
		for _, pattern := range nonOverflowPatterns {
			if pattern.MatchString(message.ErrorMessage) {
				isNonOverflow = true
				break
			}
		}
		if !isNonOverflow {
			for _, pattern := range overflowPatterns {
				if pattern.MatchString(message.ErrorMessage) {
					return true
				}
			}
			if string(message.Provider) == "cerebras" && cerebrasBodylessOverflowPattern.MatchString(message.ErrorMessage) {
				return true
			}
		}
	}

	if contextWindow > 0 && message.StopReason == model.StopStop {
		if inputTokens(message) > contextWindow {
			return true
		}
	}

	if contextWindow > 0 && message.StopReason == model.StopLength && message.Usage.Output == 0 {
		if float64(inputTokens(message)) >= float64(contextWindow)*0.99 {
			return true
		}
	}

	return false
}

func inputTokens(message *model.AssistantMessage) int {
	return message.Usage.Input + message.Usage.CacheRead
}

// IsRecoverableLength reports whether a length stop ended below the intended
// output limit, which callers may treat as context pressure. desiredMaxOutput
// must be the original limit before any context-based clamping.
func IsRecoverableLength(message *model.AssistantMessage, desiredMaxOutput int) bool {
	return message != nil &&
		message.StopReason == model.StopLength &&
		desiredMaxOutput > 0 &&
		message.Usage.Output < desiredMaxOutput
}

// OverflowPatterns returns the overflow patterns for testing.
func OverflowPatterns() []*regexp.Regexp {
	return append([]*regexp.Regexp(nil), overflowPatterns...)
}
