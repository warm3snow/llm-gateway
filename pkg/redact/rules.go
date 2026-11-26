package redact

import (
	"regexp"
	"strings"
)

// kindPriority orders overlapping spans: lower rank wins.
var kindPriority = map[string]int{
	KindAPIKey:     0,
	KindPrivateKey: 0,
	KindJWT:        0,
	KindHeader:     1, // SECRET
	KindURLPass:    1, // PASSWORD
	KindEmail:      2,
	KindIDCard:     3,
	KindBankCard:   4,
	KindPhone:      5,
	KindTerm:       6,
	KindCustom:     7,
}

func kindRank(kind string) int {
	if r, ok := kindPriority[kind]; ok {
		return r
	}
	return 8 // custom user rule kinds and anything unknown
}

// compiledRule is a detection rule with its regex precompiled.
type compiledRule struct {
	kind      string
	re        *regexp.Regexp
	capture   bool // value comes from capture group 1
	validator func(input string, value string) bool
	boundary  bool // reject matches embedded inside longer tokens
	minLen    int
}

// compileRule is the declarative form used to declare built-in rules.
type compileRule struct {
	kind      string
	re        *regexp.Regexp
	capture   bool
	validator func(input string, value string) bool
	boundary  bool
	minLen    int
}

var builtRules = mustCompile([]compileRule{
	{
		kind: KindPrivateKey,
		// PEM private key block, header through END line.
		re: regexp.MustCompile(`(?s)-----BEGIN [A-Z0-9 ]*PRIVATE KEY-----.*?-----END [A-Z0-9 ]*PRIVATE KEY-----`),
	},
	{
		kind: KindAPIKey,
		// Known vendor key shapes; boundary-checked so matches inside
		// longer tokens are rejected.
		re: regexp.MustCompile(`(?:sk-(?:ant-)?(?:proj-)?[A-Za-z0-9_-]{20,}|` +
			`[A-Za-z]{2,4}_sk_[A-Za-z0-9]{20,}|` +
			`ark-[A-Za-z0-9]{8,}|` +
			`gh[pousr]_[A-Za-z0-9]{20,}|` +
			`github_pat_[A-Za-z0-9_]{20,}|` +
			`glpat-[A-Za-z0-9_-]{15,}|` +
			`AIza[0-9A-Za-z_-]{35}|` + `xox[baprs]-[A-Za-z0-9-]{10,}|` +
			`(?:sk|rk)_live_[A-Za-z0-9]{10,}|` +
			`hf_[A-Za-z0-9]{20,}|` +
			`gsk_[A-Za-z0-9]{20,}|` +
			`xai-[A-Za-z0-9]{20,}|` +
			`npm_[A-Za-z0-9]{20,}|` +
			`pypi-[A-Za-z0-9_-]{20,}|` +
			`dop_v1_[a-f0-9]{40,}|` +
			`SG\.[A-Za-z0-9_-]{22}\.[A-Za-z0-9_-]{43}|` +
			`(?:AK|AS)IA[A-Z0-9]{16}|` +
			`cf(?:ut|at|k|oat)_[A-Za-z0-9_-]{20,})`),
		boundary: true,
		minLen:   20,
	},
	{
		kind: KindJWT,
		re: regexp.MustCompile(`(?:eyJ[A-Za-z0-9_-]{8,}\.eyJ[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}|` +
			`eyJhIjoi[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+)`),
		minLen: 20,
	},
	{
		kind:    KindURLPass,
		re:      regexp.MustCompile(`[a-zA-Z][a-zA-Z0-9+.-]*://[^\s/:@]+:([^\s/@]+)@[^\s]+`),
		capture: true,
	},
	{
		kind: KindHeader,
		// Authorization / Cookie style header values on their own line.
		re:      regexp.MustCompile(`(?mi)^[ \t]*(?:Authorization|Cookie|X-Api-Key|Proxy-Authorization)[ \t]*:[ \t]*(\S[^\r\n]*)$`),
		capture: true,
	},
	{
		kind: KindEmail,
		re:   regexp.MustCompile(`[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}`),
	},
	{
		kind: KindPhone,
		// Bare Chinese mobile shape; masked only when a digit-boundary
		// check passes so longer numbers (ID card, bank card) are not
		// mistaken for a phone.
		re: regexp.MustCompile(`(?:\+?86)?1[3-9]\d{9}`),
		validator: func(in string, v string) bool {
			i := strings.Index(in, v)
			if i < 0 {
				return false
			}
			return !isDigitByteAt(in, i-1) && !isDigitByteAt(in, i+len(v))
		},
	},
	{
		kind: KindPhone,
		// label-based generic phone rule: adjacent text says phone/电话/tel/mobile.
		re:      regexp.MustCompile(`(?i)(?:phone|电话|tel|mobile)[ \t]*[:：=]?[ \t]*(\+?\d[\d\s-]{8,}\d)`),
		capture: true,
	},
	{
		kind:      KindIDCard,
		re:        regexp.MustCompile(`\d{17}[\dXx]`),
		validator: func(_ string, v string) bool { return idCardValid(v) },
		boundary:  true,
	},
	{
		kind: KindBankCard,
		// 13-19 digits, optionally space/dash-grouped 4-6-4 style.
		re: regexp.MustCompile(`\d{13,19}|\d{4}[ -]\d{4,6}[ -]\d{4}`),
		validator: func(_ string, v string) bool {
			digits := strings.Map(func(r rune) rune {
				if r == ' ' || r == '-' {
					return -1
				}
				return r
			}, v)
			return len(digits) >= 13 && len(digits) <= 19 && luhnValid(digits)
		},
		boundary: true,
	},
	{
		kind: KindAPIKey,
		// field-name rule: key/token/password style names followed by
		// ":" or "=" then a quoted or bare 8+ char value.
		re: regexp.MustCompile(`(?i)(?:api[_-]?key|apikey|access[_-]?key|secret[_-]?key|token|password|passwd|pwd|secret)[ \t]*[:=][ \t]*` +
			`(?:"([^"\n]{8,})"|'([^'\n]{8,})'|([A-Za-z0-9_./+=-]{8,}))`),
		validator: func(_ string, v string) bool {
			// dotted values are handled by the JWT rule; bare host-like
			// values (e.g. "db.example.com") are not secrets
			return looksLikeSecret(v) && !strings.Contains(v, ".")
		},
		minLen: 8,
	},
	{
		kind: KindHeader,
		// CJK-labeled secrets: 密码/密钥/... followed by 为/是/：/:/= then 4+ chars.
		re:        regexp.MustCompile(`(?:密码|密钥|令牌|口令|秘钥|私钥)[ \t]*(?:为|是|：|:|=)[ \t]*([A-Za-z0-9_./+=@#$%^&*!~-]{4,})`),
		validator: func(_ string, v string) bool { return looksLikeSecret(v) || len(v) >= 20 },
	},
})

func mustCompile(rules []compileRule) []compiledRule {
	out := make([]compiledRule, 0, len(rules))
	for _, r := range rules {
		v := r.validator
		if v == nil {
			v = func(string, string) bool { return true }
		}
		minLen := r.minLen
		if minLen < valueMinLen {
			minLen = valueMinLen
		}
		out = append(out, compiledRule{
			kind: r.kind, re: r.re, capture: r.capture,
			validator: v, boundary: r.boundary, minLen: minLen,
		})
	}
	return out
}

// wordRE builds a regexp for a custom word, escaping regexp metacharacters.
func wordRE(word string, anyCase, whole bool) *regexp.Regexp {
	var b strings.Builder
	if whole {
		b.WriteString(`(?:^|[^A-Za-z0-9_])`)
	}
	for _, r := range word {
		if strings.ContainsRune(`\.+*?()|[]{}^$`, r) {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	if whole {
		b.WriteString(`(?:$|[^A-Za-z0-9_])`)
	}
	if anyCase {
		return regexp.MustCompile(`(?i)` + b.String())
	}
	return regexp.MustCompile(b.String())
}

// normalizeKind upper-cases a kind for Options.Kinds lookups.
func normalizeKind(k string) string { return strings.ToUpper(strings.TrimSpace(k)) }
