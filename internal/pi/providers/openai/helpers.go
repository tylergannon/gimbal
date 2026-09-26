package openai

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"unicode/utf16"

	"github.com/tylergannon/gimbal/internal/pi/model"
)

// Provider and streaming helpers ported alongside openai-completions.ts:
// provider env lookup, surrogate sanitizing, the short hash used for tool-call
// ids, the pi user agent, prompt-cache key clamping and HTTP error formatting.

// getProviderEnvValue resolves a provider env value from scoped overrides, then
// the process environment (pi utils/provider-env.ts).
func getProviderEnvValue(name string, env model.ProviderEnv) string {
	if env != nil {
		if v, ok := env[name]; ok && v != "" {
			return v
		}
	}
	return os.Getenv(name)
}

// sanitizeSurrogates removes unpaired UTF-16 surrogate code points. In Go a
// valid string cannot hold a surrogate rune, but a string built from raw UTF-16
// units can, and some callers do. Valid emoji are single runes above the BMP
// and survive.
func sanitizeSurrogates(text string) string {
	if !strings.ContainsFunc(text, func(r rune) bool { return r >= 0xD800 && r <= 0xDFFF }) {
		return text
	}
	var b strings.Builder
	b.Grow(len(text))
	for _, r := range text {
		if r >= 0xD800 && r <= 0xDFFF {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// shortHash is pi utils/hash.ts shortHash, over UTF-16 code units.
func shortHash(s string) string {
	units := utf16.Encode([]rune(s))
	h1 := uint32(0xdeadbeef)
	h2 := uint32(0x41c6ce57)
	for _, ch := range units {
		h1 = imul(h1^uint32(ch), 2654435761)
		h2 = imul(h2^uint32(ch), 1597334677)
	}
	h1 = imul(h1^(h1>>16), 2246822507) ^ imul(h2^(h2>>13), 3266489909)
	h2 = imul(h2^(h2>>16), 2246822507) ^ imul(h1^(h1>>13), 3266489909)
	return strconv.FormatUint(uint64(h2), 36) + strconv.FormatUint(uint64(h1), 36)
}

func imul(a, b uint32) uint32 { return a * b }

// getPiUserAgent is pi utils/pi-user-agent.ts, using Go's runtime identity in
// place of node's os module.
func getPiUserAgent() string {
	return fmt.Sprintf("pi (%s %s)", runtime.GOOS, runtime.GOARCH)
}

// maxPromptCacheKeyLength is OPENAI_PROMPT_CACHE_KEY_MAX_LENGTH.
const maxPromptCacheKeyLength = 64

// clampPromptCacheKey truncates a prompt cache key by code point.
func clampPromptCacheKey(key string) string {
	runes := []rune(key)
	if len(runes) <= maxPromptCacheKeyLength {
		return key
	}
	return string(runes[:maxPromptCacheKeyLength])
}

// maxProviderErrorBodyChars caps a surfaced HTTP error body.
const maxProviderErrorBodyChars = 4000

func truncateErrorText(text string, maxChars int) string {
	units := utf16.Encode([]rune(text))
	if len(units) <= maxChars {
		return text
	}
	head := string(utf16.Decode(units[:maxChars]))
	return fmt.Sprintf("%s... [truncated %d chars]", head, len(units)-maxChars)
}

// formatHTTPError builds a concise error from a non-2xx provider response,
// preferring the provider's structured error.message when present.
func formatHTTPError(label string, status int, body []byte) error {
	msg := strings.TrimSpace(string(body))
	var parsed struct {
		Error struct {
			Message string `json:"message"`
			Code    string `json:"code"`
		} `json:"error"`
	}
	if json.Unmarshal(body, &parsed) == nil && parsed.Error.Message != "" {
		msg = parsed.Error.Message
		if parsed.Error.Code != "" {
			msg = fmt.Sprintf("%s (%s)", msg, parsed.Error.Code)
		}
	}
	msg = truncateErrorText(msg, maxProviderErrorBodyChars)
	return fmt.Errorf("%s API error %d: %s", label, status, msg)
}

// hasHeader reports whether a provider header carries a non-empty value under
// a case-insensitive name. A nil value is a deletion marker and does not count.
func hasHeader(headers model.ProviderHeaders, name string) bool {
	for key, value := range headers {
		if !strings.EqualFold(key, name) {
			continue
		}
		if value != nil && strings.TrimSpace(*value) != "" {
			return true
		}
	}
	return false
}

// clientAPIKey ports pi's getClientApiKey: an explicit key wins; otherwise a
// header-supplied authorization credential lets the client use an "unused"
// placeholder; otherwise the request fails.
func clientAPIKey(provider model.ProviderId, apiKey string, headers model.ProviderHeaders) (string, error) {
	if apiKey != "" {
		return apiKey, nil
	}
	if hasHeader(headers, "authorization") || hasHeader(headers, "cf-aig-authorization") {
		return "unused", nil
	}
	return "", fmt.Errorf("No API key for provider: %s", provider) //nolint:staticcheck // pi's exact message
}

// resolveClientAPIKey resolves the runtime API key, falling back to the
// Diffusion Router's DIFFUSION_API_KEY environment variable when the caller
// did not supply one.
func resolveClientAPIKey(provider model.ProviderId, apiKey string, headers model.ProviderHeaders, env model.ProviderEnv) (string, error) {
	if apiKey == "" {
		apiKey = getProviderEnvValue("DIFFUSION_API_KEY", env)
	}
	return clientAPIKey(provider, apiKey, headers)
}
