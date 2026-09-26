package wire

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
)

// JSON repair and partial parsing, ported from pi
// packages/ai/src/utils/json-parse.ts (upstream d6af72e1). The partial fallback
// reimplements the behavior of the partial-json 0.1.7 package pi depends on.

var validJSONEscapes = map[byte]bool{
	'"': true, '\\': true, '/': true, 'b': true, 'f': true,
	'n': true, 'r': true, 't': true, 'u': true,
}

func isHexDigit(b byte) bool {
	return (b >= '0' && b <= '9') || (b >= 'a' && b <= 'f') || (b >= 'A' && b <= 'F')
}

func escapeControlChar(ch byte) string {
	switch ch {
	case '\b':
		return "\\b"
	case '\f':
		return "\\f"
	case '\n':
		return "\\n"
	case '\r':
		return "\\r"
	case '\t':
		return "\\t"
	default:
		return fmt.Sprintf("\\u%04x", ch)
	}
}

// RepairJSON repairs malformed JSON string literals by escaping raw control
// characters inside strings and doubling backslashes before invalid escape
// characters. It is a port of pi's repairJson.
func RepairJSON(jsonText string) string {
	var b strings.Builder
	b.Grow(len(jsonText))
	inString := false
	for i := 0; i < len(jsonText); i++ {
		ch := jsonText[i]
		if !inString {
			b.WriteByte(ch)
			if ch == '"' {
				inString = true
			}
			continue
		}
		if ch == '"' {
			b.WriteByte(ch)
			inString = false
			continue
		}
		if ch == '\\' {
			if i+1 >= len(jsonText) {
				b.WriteString("\\\\")
				continue
			}
			next := jsonText[i+1]
			if next == 'u' && i+6 <= len(jsonText) {
				hex := jsonText[i+2 : i+6]
				if isHexDigit(hex[0]) && isHexDigit(hex[1]) && isHexDigit(hex[2]) && isHexDigit(hex[3]) {
					b.WriteString("\\u")
					b.WriteString(hex)
					i += 5
					continue
				}
			}
			if validJSONEscapes[next] {
				b.WriteByte('\\')
				b.WriteByte(next)
				i++
				continue
			}
			b.WriteString("\\\\")
			continue
		}
		if ch <= 0x1f {
			b.WriteString(escapeControlChar(ch))
		} else {
			b.WriteByte(ch)
		}
	}
	return b.String()
}

// ParseJSONWithRepair parses JSON, retrying once with RepairJSON on failure.
func ParseJSONWithRepair[T any](jsonText string) (T, error) {
	var out T
	err := json.Unmarshal([]byte(jsonText), &out)
	if err == nil {
		return out, nil
	}
	repaired := RepairJSON(jsonText)
	if repaired != jsonText {
		var repairedOut T
		if err2 := json.Unmarshal([]byte(repaired), &repairedOut); err2 != nil {
			var zero T
			return zero, err2
		}
		return repairedOut, nil
	}
	var zero T
	return zero, err
}

// errPartialJSON stands for whatever the partial-json package throws.
var errPartialJSON = errors.New("wire: unparseable partial json")

func decodeJSONValue(text string) (any, error) {
	var v any
	if err := json.Unmarshal([]byte(text), &v); err != nil {
		return nil, err
	}
	return v, nil
}

// ParseStreamingJSON parses potentially incomplete JSON read from a stream.
// It tries a full parse, then a repaired full parse, then the partial parser on
// both the text and its repair, returning an empty object when all fail. It is a
// port of pi's parseStreamingJson, narrowed to JSON objects because that is what
// tool-call arguments are.
func ParseStreamingJSON(partialJSON string) map[string]any {
	if strings.TrimSpace(partialJSON) == "" {
		return map[string]any{}
	}
	value, err := decodeJSONValue(partialJSON)
	if err != nil {
		if repaired := RepairJSON(partialJSON); repaired != partialJSON {
			value, err = decodeJSONValue(repaired)
		}
	}
	if err != nil {
		value, err = partialJSONParse(partialJSON)
		if err != nil {
			value, err = partialJSONParse(RepairJSON(partialJSON))
		}
		if err != nil {
			return map[string]any{}
		}
	}
	object, ok := value.(map[string]any)
	if !ok {
		return map[string]any{}
	}
	return object
}

// partialJSONParse reads the value that text begins, as much of it as text
// holds. It is the partial-json package's parse(text) with Allow.ALL.
func partialJSONParse(text string) (any, error) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return nil, errPartialJSON
	}
	p := &partialJSONParser{s: trimmed}
	return p.parseAny()
}

type partialJSONParser struct {
	s string
	i int
}

func (p *partialJSONParser) is(i int, c byte) bool {
	return i >= 0 && i < len(p.s) && p.s[i] == c
}

func (p *partialJSONParser) skipBlank() {
	for p.i < len(p.s) {
		switch p.s[p.i] {
		case ' ', '\n', '\r', '\t':
			p.i++
		default:
			return
		}
	}
}

var partialJSONLiterals = []struct {
	word      string
	value     any
	minLength int
}{
	{"null", nil, 0},
	{"true", true, 0},
	{"false", false, 0},
	{"Infinity", math.Inf(1), 0},
	{"-Infinity", math.Inf(-1), 1},
	{"NaN", math.NaN(), 0},
}

func (p *partialJSONParser) parseAny() (any, error) {
	p.skipBlank()
	if p.i >= len(p.s) {
		return nil, errPartialJSON
	}
	switch p.s[p.i] {
	case '"':
		return p.parseStr()
	case '{':
		return p.parseObj()
	case '[':
		return p.parseArr()
	}
	rest := p.s[p.i:]
	for _, literal := range partialJSONLiterals {
		if strings.HasPrefix(rest, literal.word) ||
			(len(rest) < len(literal.word) && len(rest) > literal.minLength && strings.HasPrefix(literal.word, rest)) {
			p.i += len(literal.word)
			return literal.value, nil
		}
	}
	return p.parseNum()
}

// parseStr reads a string from its opening quote — or from wherever an object
// key is expected, quote or not — to the next unescaped quote. An unterminated
// string is read to its end, less a trailing backslash.
func (p *partialJSONParser) parseStr() (any, error) {
	start := p.i
	escape := false
	p.i++
	for p.i < len(p.s) && (p.s[p.i] != '"' || (escape && p.s[p.i-1] == '\\')) {
		escape = p.s[p.i] == '\\' && !escape
		p.i++
	}
	trailing := 0
	if escape {
		trailing = 1
	}
	if p.is(p.i, '"') {
		p.i++
		return decodePartialString(jsSubstring(p.s, start, p.i-trailing))
	}
	if value, err := decodePartialString(jsSubstring(p.s, start, p.i-trailing) + `"`); err == nil {
		return value, nil
	}
	return decodePartialString(jsSubstring(p.s, start, strings.LastIndexByte(p.s, '\\')) + `"`)
}

func decodePartialString(quoted string) (any, error) {
	var s string
	if err := json.Unmarshal([]byte(quoted), &s); err != nil {
		return nil, errPartialJSON
	}
	return s, nil
}

// parseObj reads an object's members until its closing brace. Whatever fails —
// the text ending, a key or value that does not parse — ends the object with
// the members read so far.
func (p *partialJSONParser) parseObj() (any, error) {
	p.i++
	p.skipBlank()
	object := map[string]any{}
	for !p.is(p.i, '}') {
		p.skipBlank()
		if p.i >= len(p.s) {
			return object, nil
		}
		key, err := p.parseStr()
		if err != nil {
			return object, nil
		}
		p.skipBlank()
		p.i++ // colon
		value, err := p.parseAny()
		if err != nil {
			return object, nil
		}
		if name, ok := key.(string); ok {
			object[name] = value
		}
		p.skipBlank()
		if p.is(p.i, ',') {
			p.i++
		}
	}
	p.i++
	return object, nil
}

// parseArr reads an array's elements until its closing bracket, ending it with
// the elements read so far when one fails.
func (p *partialJSONParser) parseArr() (any, error) {
	p.i++
	array := []any{}
	for !p.is(p.i, ']') {
		value, err := p.parseAny()
		if err != nil {
			return array, nil
		}
		array = append(array, value)
		p.skipBlank()
		if p.is(p.i, ',') {
			p.i++
		}
	}
	p.i++
	return array, nil
}

// parseNum reads a number as the text up to the next ',', ']' or '}' — the
// whole text when the number starts it — that JSON accepts, else that text cut
// at the last "e" in the whole text.
func (p *partialJSONParser) parseNum() (any, error) {
	if p.i == 0 {
		if p.s == "-" {
			return nil, errPartialJSON
		}
		if value, err := decodeJSONValue(p.s); err == nil {
			return value, nil
		}
		if cut := strings.LastIndexByte(p.s, 'e'); cut >= 0 {
			if value, err := decodeJSONValue(jsSubstring(p.s, 0, cut)); err == nil {
				return value, nil
			}
		}
		return nil, errPartialJSON
	}
	start := p.i
	if p.s[p.i] == '-' {
		p.i++
	}
	for p.i < len(p.s) && strings.IndexByte(",]}", p.s[p.i]) < 0 {
		p.i++
	}
	text := p.s[start:p.i]
	if value, err := decodeJSONValue(text); err == nil {
		return value, nil
	}
	if text == "-" {
		return nil, errPartialJSON
	}
	if before, _, ok := strings.CutLast(text, "e"); ok {
		if value, err := decodeJSONValue(before); err == nil {
			return value, nil
		}
	}
	return nil, errPartialJSON
}

// jsSubstring is String.prototype.substring: each index clamped to the text,
// and the two swapped when the start is past the end.
func jsSubstring(s string, start, end int) string {
	start, end = min(max(start, 0), len(s)), min(max(end, 0), len(s))
	if start > end {
		start, end = end, start
	}
	return s[start:end]
}
