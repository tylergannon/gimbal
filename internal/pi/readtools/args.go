package readtools

import (
	"encoding/json"

	"github.com/tylergannon/gimbal/internal/pi/model"
)

// desc attaches a JSON-Schema description to a schema node.
func desc(s *model.Schema, description string) *model.Schema {
	s.Description = description
	return s
}

// argStr reads a string argument, returning "" when absent or not a string.
func argStr(params map[string]any, key string) string {
	v, _ := params[key].(string)
	return v
}

// argInt reads an integer-valued argument. JSON numbers decode to float64, so
// that is the common case; whole-number strings and json.Number are accepted
// too.
func argInt(params map[string]any, key string) (int, bool) {
	switch v := params[key].(type) {
	case float64:
		return int(v), true
	case int:
		return v, true
	case int64:
		return int(v), true
	case json.Number:
		i, err := v.Int64()
		if err != nil {
			return 0, false
		}
		return int(i), true
	default:
		return 0, false
	}
}

// argBool reads a boolean argument, returning false when absent.
func argBool(params map[string]any, key string) bool {
	v, _ := params[key].(bool)
	return v
}

// textResult wraps a text body as a tool result.
func textResult(text string) model.AgentToolResult {
	return model.AgentToolResult{Content: model.ContentList{model.TextContent{Text: text}}}
}

// maxInt disables the line limit for the byte-capped tools, matching pi's
// Number.MAX_SAFE_INTEGER.
const maxInt = int(^uint(0) >> 1)

// Per-tool default result caps, matching pi's DEFAULT_LIMIT constants.
const (
	grepDefaultLimit = 100
	findDefaultLimit = 1000
	lsDefaultLimit   = 500
)
