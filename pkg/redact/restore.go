package redact

import (
	"regexp"
	"strings"
)

// placeholderRE matches a full placeholder, plus optional stray braces that
// model output sometimes duplicates around an edge ({{{TAG}}}, {{TAG}}}).
// The tag is matched case-insensitively so a mangled tag still restores.
var placeholderRE = regexp.MustCompile(`(?i)\{+\{[A-Z][A-Z0-9_]*_[a-z0-9]{8}\}\}+`)

// placeholderSpan is a matched placeholder occurrence, including any stray
// leading/trailing braces absorbed by the regex.
type placeholderSpan struct {
	start, end int
	body       string // canonical {{KIND_TAG}} form
}

// findPlaceholders scans s for placeholder occurrences.
func findPlaceholders(s string) []placeholderSpan {
	var spans []placeholderSpan
	i := 0
	for _, loc := range placeholderRE.FindAllStringIndex(s, -1) {
		if loc[0] < i {
			continue
		}
		i = loc[1]
		full := s[loc[0]:loc[1]]
		body := strings.TrimSuffix(strings.TrimPrefix(full, "{"), "}")
		spans = append(spans, placeholderSpan{start: loc[0], end: loc[1], body: body})
	}
	return spans
}

// splitPlaceholder splits "{{KIND_TAG}}" into kind and tag. The tag is the
// last 8 chars (base32 lowercase); the kind is everything before it.
func splitPlaceholder(body string) (kind, tag string, ok bool) {
	inner := strings.TrimSuffix(strings.TrimPrefix(body, "{"), "}")
	if inner == body || len(inner) <= tagLen+1 {
		return "", "", false
	}
	tag = inner[len(inner)-tagLen:]
	kind = strings.TrimSuffix(inner[:len(inner)-tagLen], "_")
	kind = strings.TrimPrefix(kind, "{")
	if kind == "" {
		return "", "", false
	}
	return strings.ToLower(kind), strings.ToLower(tag), true
}

// lookup finds the stored value for a placeholder kind+tag, tolerating
// case differences in both the kind and the tag.
func (e *Engine) lookup(kind, tag string) (string, bool) {
	return e.store.get(strings.ToLower(kind + "_" + tag))
}

// hasPlaceholder does a cheap substring precheck for the opening "{{".
func hasPlaceholder(s string) bool { return strings.Contains(s, "{{") }
