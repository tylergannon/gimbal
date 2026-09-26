package files

import "strings"

// isJSWhitespace reports whether r is removed by String.prototype.trim. The set
// is ECMAScript WhiteSpace ∪ LineTerminator, which is not Go's unicode.IsSpace:
// JavaScript strips U+FEFF and keeps U+0085, Go does the reverse.
func isJSWhitespace(r rune) bool {
	switch r {
	case '\t', '\n', '\v', '\f', '\r', ' ', '\u00a0', '\u1680',
		'\u2000', '\u2001', '\u2002', '\u2003', '\u2004', '\u2005', '\u2006',
		'\u2007', '\u2008', '\u2009', '\u200a', '\u2028', '\u2029', '\u202f',
		'\u205f', '\u3000', '\ufeff':
		return true
	}
	return false
}

// jsTrim is String.prototype.trim.
func jsTrim(s string) string { return strings.TrimFunc(s, isJSWhitespace) }

// baseMimeType is pi's baseMimeType: the type before any parameters, trimmed as
// JavaScript trims and lower-cased.
func baseMimeType(mimeType string) string {
	base := mimeType
	if i := strings.IndexByte(base, ';'); i >= 0 {
		base = base[:i]
	}
	return strings.ToLower(jsTrim(base))
}
