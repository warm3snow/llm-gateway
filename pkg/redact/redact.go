// Package redact masks sensitive information (API keys, credentials,
// personal identifiers) in LLM gateway traffic with stable placeholders
// that can be restored before responses are sent back to the client.
package redact

import (
	"regexp"
	"sort"
	"strings"
	"sync"
)

// Rule kinds.
const (
	KindPrivateKey = "PRIVATE_KEY"
	KindAPIKey     = "API_KEY"
	KindJWT        = "JWT"
	KindURLPass    = "PASSWORD"
	KindHeader     = "SECRET"
	KindEmail      = "EMAIL"
	KindPhone      = "PHONE"
	KindIDCard     = "ID_CARD"
	KindBankCard   = "BANK_CARD"
	KindTerm       = "TERM"
	KindCustom     = "CUSTOM"
)

// Actions for Options.Actions: what to do with a found value of a kind.
const (
	ActionMask  = "mask"
	ActionWarn  = "warn"
	ActionBlock = "block"
)

const (
	tagLen     = 8 // characters of base32 kept in a placeholder
	maxValues  = 200000
	maxKindLen = 24

	valueMinLen = 1 // a rule match shorter than this is ignored
)

// decimalRE matches JSON decimal/version-like values (e.g. 3.14, 1.2.3).
var decimalRE = regexp.MustCompile(`^\d+(\.\d+)+$`)

func isDecimalLike(v string) bool { return decimalRE.MatchString(v) }

// span is one found value: byte range in the input plus kind and value.
type span struct {
	start, end int
	kind       string
	value      string
}

// Options configures one Mask/MaskRequest/Preview call.
type Options struct {
	Secrets  bool // mask API_KEY/PRIVATE_KEY/JWT/PASSWORD/SECRET kinds
	Personal bool // mask EMAIL/PHONE/ID_CARD/BANK_CARD

	// Kinds optionally overrides per-kind on/off. Personal kinds are
	// honored only when Personal is on; secrets kinds honor it too.
	Kinds map[string]bool

	Words        []string // custom words always masked (kind TERM); <2 runes skipped
	WordsAnyCase bool     // match words case-insensitively
	WordsWhole   bool     // whole-word match only (boundaries: not [A-Za-z0-9_])

	Rules []Rule // custom rules; at most 32, kind normalized, default CUSTOM

	Actions map[string]string // kind -> "mask"|"warn"|"block" (default mask)

	Allow []string // exact values (case-insensitive) never masked

	Seen func(Hit) // optional callback per hit
}

// Hit is one found sensitive value, reported via Options.Seen.
type Hit struct {
	Kind   string
	Action string // masked|warn|block
	Length int
	Head   string // first 4 chars, for audit logs only
}

// Rule is a user-supplied custom detection rule: either a literal Prefix
// (value = rest of token, >= 8 chars of [A-Za-z0-9_-]) or a Regex (value =
// group 1 when present, else the whole match).
type Rule struct {
	Kind   string `json:"kind"`
	Prefix string `json:"prefix,omitempty"`
	Regex  string `json:"regex,omitempty"`
}

// Engine masks and restores text. Safe for concurrent use.
type Engine struct {
	key   []byte
	store *store

	// compiled custom word rules per (anyCase, whole) combination cache
	wordsMu sync.Mutex
	wordsRe map[string]*regexp.Regexp

	// compiled custom Prefix/Regex rules, rebuilt when Rules change
	customRules []compiledRule
}

// New creates an Engine with a fresh random HMAC key (32 bytes) and an
// empty bounded store.
func New() *Engine {
	return &Engine{
		key:     randomKey(),
		store:   newStore(maxValues),
		wordsRe: make(map[string]*regexp.Regexp),
	}
}

// Mask replaces every found sensitive value in s with a stable placeholder
// and returns the masked text plus the number of replacements.
func (e *Engine) Mask(s string, o Options) (string, int) {
	out, n, _ := e.mask(s, o, false)
	return out, n
}

// MaskRequest masks like Mask and additionally reports whether any found
// value had action "block". "warn" values stay as-is but are reported.
func (e *Engine) MaskRequest(s string, o Options) (masked string, redactions int, blocked bool) {
	return e.mask(s, o, false)
}

// Restore replaces placeholders in s with their original values and returns
// the restored text plus the number of replacements.
func (e *Engine) Restore(s string) (string, int) {
	spans := findPlaceholders(s)
	if len(spans) == 0 {
		return s, 0
	}
	var b strings.Builder
	last := 0
	n := 0
	for _, ps := range spans {
		kind, tag, ok := splitPlaceholder(ps.body)
		if !ok {
			continue
		}
		v, found := e.lookup(kind, tag)
		if !found {
			continue
		}
		b.WriteString(s[last:ps.start])
		b.WriteString(v)
		last = ps.end
		n++
	}
	if n == 0 {
		return s, 0
	}
	b.WriteString(s[last:])
	return b.String(), n
}

// HasPlaceholder reports whether s contains at least one placeholder.
func (e *Engine) HasPlaceholder(s string) bool { return hasPlaceholder(s) }

// Preview masks like Mask but does NOT store the values, so a Previewed
// text can never be restored. Useful for logs and audits.
func (e *Engine) Preview(s string, o Options) string {
	out, _, _ := e.mask(s, o, true)
	return out
}

// mask is the core of Mask/MaskRequest/Preview.
func (e *Engine) mask(s string, o Options, preview bool) (string, int, bool) {
	spans := e.collectSpans(s, o)
	if len(spans) == 0 {
		return s, 0, false
	}
	e.sortAndMerge(&spans)
	spans = e.filterAllowed(s, spans, o)
	if len(spans) == 0 {
		return s, 0, false
	}

	var b strings.Builder
	last := 0
	n := 0
	blocked := false
	for _, sp := range spans {
		action := o.Actions[sp.kind]
		if action == "" {
			action = ActionMask
		}
		head := sp.value
		if len(head) > 4 {
			head = head[:4]
		}
		if o.Seen != nil {
			o.Seen(Hit{Kind: sp.kind, Action: action, Length: len(sp.value), Head: head})
		}
		if action == ActionBlock {
			blocked = true
		}
		b.WriteString(s[last:sp.start])
		if action == ActionWarn {
			// value stays as-is but is reported; count it as a hit
			b.WriteString(sp.value)
			n++
		} else {
			ph := e.placeholder(sp.kind, sp.value, preview)
			b.WriteString(ph)
			n++
		}
		last = sp.end
	}
	b.WriteString(s[last:])
	return b.String(), n, blocked
}

// placeholder builds the {{KIND_TAG}} text for a value. With preview it
// computes the tag directly without storing the value.
func (e *Engine) placeholder(kind, value string, preview bool) string {
	tag := placeholderTag(e.key, kind, value)
	if !preview {
		e.store.put(strings.ToLower(kind+"_"+tag), value)
	}
	return "{{" + kind + "_" + tag + "}}"
}

// collectSpans runs all applicable rules and word rules over s.
func (e *Engine) collectSpans(s string, o Options) []span {
	var spans []span
	allowSet := allowSet(o.Allow)

	// custom Prefix/Regex rules first (rank 8 in priority, but collected
	// here so their spans merge with built-in ones)
	for _, cr := range e.customRulesFor(o) {
		spans = append(spans, e.findRuleSpans(s, cr, o, allowSet)...)
	}
	for _, cr := range builtRules {
		if !kindEnabled(cr.kind, o) {
			continue
		}
		spans = append(spans, e.findRuleSpans(s, cr, o, allowSet)...)
	}
	spans = append(spans, e.findWordSpans(s, o, allowSet)...)
	return spans
}

// normalizeCustomKind converts a user-supplied kind to the placeholder
// alphabet ([A-Z0-9_]), defaulting to CUSTOM when empty.
func normalizeCustomKind(kind string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(strings.TrimSpace(kind)) {
		switch {
		case r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_':
			b.WriteRune(r)
		case r == ' ' || r == '-' || r == '.':
			b.WriteByte('_')
		}
	}
	k := strings.Trim(b.String(), "_")
	if k == "" {
		return KindCustom
	}
	if k[0] < 'A' || k[0] > 'Z' {
		k = KindCustom + "_" + k
	}
	if len(k) > maxKindLen {
		k = strings.TrimRight(k[:maxKindLen], "_")
	}
	return k
}

// customRulesFor compiles o.Rules (at most 32; invalid regexes skipped).
func (e *Engine) customRulesFor(o Options) []compiledRule {
	if len(o.Rules) == 0 {
		return nil
	}
	rules := o.Rules
	if len(rules) > 32 {
		rules = rules[:32]
	}
	out := make([]compiledRule, 0, len(rules))
	for _, r := range rules {
		kind := normalizeCustomKind(r.Kind)
		if r.Prefix != "" {
			out = append(out, compiledRule{
				kind: kind,
				re:   prefixRE(r.Prefix),
			})
		} else if r.Regex != "" {
			re, err := regexp.Compile(r.Regex)
			if err != nil {
				continue
			}
			// a rule that matches the empty string would mask everywhere
			if re.MatchString("") {
				continue
			}
			out = append(out, compiledRule{
				kind:    kind,
				re:      re,
				capture: re.NumSubexp() > 0,
			})
		}
	}
	return out
}

// prefixRE builds the rule for a literal prefix marker: value is the rest
// of the token, at least 8 chars of [A-Za-z0-9_-].
func prefixRE(prefix string) *regexp.Regexp {
	return regexp.MustCompile(regexp.QuoteMeta(prefix) + `[A-Za-z0-9_-]{8,}`)
}

func (e *Engine) findRuleSpans(s string, cr compiledRule, o Options, allow map[string]bool) []span {
	var out []span
	for _, loc := range cr.re.FindAllStringIndex(s, -1) {
		valStart, valEnd := loc[0], loc[1]
		if cr.capture {
			// capture group 1: find first non-empty submatch
			m := cr.re.FindStringSubmatch(s[loc[0]:loc[1]])
			if m == nil {
				continue
			}
			g := -1
			for i := 1; i < len(m); i++ {
				if m[i] != "" {
					g = i
					break
				}
			}
			if g < 0 {
				continue
			}
			// compute group position within match
			sub := cr.re.FindStringSubmatchIndex(s[loc[0]:loc[1]])
			valStart = loc[0] + sub[2*g]
			valEnd = loc[0] + sub[2*g+1]
		}
		value := s[valStart:valEnd]
		if len(value) < valueMinLen || len([]rune(value)) < valueMinLen {
			continue
		}
		if cr.minLen > 0 && len(value) < cr.minLen {
			continue
		}
		if cr.validator != nil && !cr.validator(s, value) {
			continue
		}
		if cr.boundary && !boundaryOK(s, valStart, valEnd) {
			continue
		}
		if allow[strings.ToLower(value)] {
			continue
		}
		if isDecimalLike(value) && cr.kind != KindPhone && cr.kind != KindIDCard && cr.kind != KindBankCard {
			continue
		}
		out = append(out, span{start: valStart, end: valEnd, kind: cr.kind, value: value})
	}
	return out
}

// findWordSpans finds custom word matches (kind TERM).
func (e *Engine) findWordSpans(s string, o Options, allow map[string]bool) []span {
	var out []span
	for _, w := range o.Words {
		if len([]rune(w)) < 2 {
			continue
		}
		re := e.wordRegexp(w, o.WordsAnyCase, o.WordsWhole)
		for _, loc := range re.FindAllStringIndex(s, -1) {
			start, end := loc[0], loc[1]
			if o.WordsWhole {
				// the regex consumed one boundary char on each side when
				// present; trim it back off so only the word is replaced
				v := s[start:end]
				if strings.HasPrefix(strings.ToLower(v), strings.ToLower(w)) {
					// boundary was at start of string (no char consumed)
				} else {
					start++
				}
				if strings.HasSuffix(strings.ToLower(v), strings.ToLower(w)) {
					// boundary was at end of string (no char consumed)
				} else {
					end--
				}
			}
			if end <= start {
				continue
			}
			value := s[start:end]
			if allow[strings.ToLower(value)] {
				continue
			}
			out = append(out, span{start: start, end: end, kind: KindTerm, value: value})
		}
	}
	return out
}

// wordRegexp compiles (and caches) the regexp for one custom word.
func (e *Engine) wordRegexp(word string, anyCase, whole bool) *regexp.Regexp {
	ck := word + "\x00" + boolKey(anyCase) + boolKey(whole)
	e.wordsMu.Lock()
	defer e.wordsMu.Unlock()
	if re, ok := e.wordsRe[ck]; ok {
		return re
	}
	re := wordRE(word, anyCase, whole)
	e.wordsRe[ck] = re
	return re
}

func boolKey(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

// kindEnabled says whether a kind should be collected for this call.
func kindEnabled(kind string, o Options) bool {
	if o.Kinds != nil {
		if on, ok := o.Kinds[normalizeKind(kind)]; ok {
			if !on {
				return false
			}
			// explicitly enabled
			return true
		}
	}
	switch kind {
	case KindAPIKey, KindPrivateKey, KindJWT, KindURLPass, KindHeader:
		return o.Secrets
	case KindEmail, KindPhone, KindIDCard, KindBankCard:
		return o.Personal
	default:
		return true // TERM, CUSTOM, user-defined kinds
	}
}

// allowSet lowercases the Allow list for case-insensitive comparison.
func allowSet(allow []string) map[string]bool {
	if len(allow) == 0 {
		return nil
	}
	m := make(map[string]bool, len(allow))
	for _, a := range allow {
		m[strings.ToLower(a)] = true
	}
	return m
}

// sortAndMerge sorts spans by start and resolves overlaps preferring the
// earlier (lower rank) kind; longer span wins at equal rank.
func (e *Engine) sortAndMerge(spans *[]span) {
	all := *spans
	sort.SliceStable(all, func(i, j int) bool {
		ri, rj := kindRank(all[i].kind), kindRank(all[j].kind)
		if ri != rj {
			return ri < rj
		}
		if all[i].start != all[j].start {
			return all[i].start < all[j].start
		}
		// longer span first at equal rank+start
		return (all[i].end - all[i].start) > (all[j].end - all[j].start)
	})
	var kept []span
	for _, sp := range all {
		overlap := false
		for _, k := range kept {
			if sp.start < k.end && k.start < sp.end {
				overlap = true
				break
			}
			if k.start >= sp.end {
				break
			}
		}
		if !overlap {
			kept = append(kept, sp)
		}
	}
	sort.SliceStable(kept, func(i, j int) bool { return kept[i].start < kept[j].start })
	*spans = kept
}

// filterAllowed drops spans whose value is in the Allow list (already done
// in findRuleSpans) — kept as a hook for future post-filters.
func (e *Engine) filterAllowed(_ string, spans []span, _ Options) []span {
	return spans
}
