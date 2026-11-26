package redact

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var secretOpts = Options{Secrets: true}

func TestMaskAPIKey(t *testing.T) {
	e := New()
	key := "sk-proj-abcdefghij1234567890KLMNOP"
	masked, n := e.Mask("my key is "+key, secretOpts)
	require.Equal(t, 1, n)
	assert.NotContains(t, masked, key)
	assert.Contains(t, masked, "{{API_KEY_")
	// restore round trip
	restored, rn := e.Restore(masked)
	require.Equal(t, 1, rn)
	assert.Equal(t, "my key is "+key, restored)
}

func TestSameValueSamePlaceholder(t *testing.T) {
	e := New()
	key := "sk-ant-api03-abcdefghij1234567890XYZ"
	m1, _ := e.Mask("a "+key+" b", secretOpts)
	m2, _ := e.Mask("c "+key+" d", secretOpts)
	p1 := extractPlaceholder(t, m1)
	p2 := extractPlaceholder(t, m2)
	assert.Equal(t, p1, p2, "same value must yield the same placeholder")
	// different values -> different placeholders
	k2 := "sk-ant-api03-qrstuvwxyz0987654321ABC"
	m3, _ := e.Mask("e "+k2+" f", secretOpts)
	p3 := extractPlaceholder(t, m3)
	assert.NotEqual(t, p1, p3)
}

func TestFieldPasswordRule(t *testing.T) {
	e := New()
	masked, n := e.Mask(`password = "hunter2secret"`, secretOpts)
	require.Equal(t, 1, n)
	assert.NotContains(t, masked, "hunter2secret")
	assert.Contains(t, masked, "{{API_KEY_")
}

func TestCJKLabelRule(t *testing.T) {
	e := New()
	masked, n := e.Mask("密码：abc12345", secretOpts)
	require.Equal(t, 1, n, "CJK label value should be masked, got %q", masked)
	assert.NotContains(t, masked, "abc12345")
}

func TestAuthorizationHeader(t *testing.T) {
	e := New()
	masked, n := e.Mask("Authorization: Bearer abcdef1234567890\nHost: x", secretOpts)
	require.Equal(t, 1, n)
	assert.NotContains(t, masked, "Bearer abcdef1234567890")
	assert.Contains(t, masked, "{{SECRET_")
}

func TestJWTMasked(t *testing.T) {
	e := New()
	jwt := "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJVadQssw5c"
	masked, n := e.Mask("token: "+jwt, secretOpts)
	require.Equal(t, 1, n)
	assert.NotContains(t, masked, jwt)
	assert.Contains(t, masked, "{{JWT_")
}

func TestPEMPrivateKeyMasked(t *testing.T) {
	e := New()
	pem := "-----BEGIN RSA PRIVATE KEY-----\nMIIBOgIBAAJBAK\nabc\ndef\n-----END RSA PRIVATE KEY-----"
	masked, n := e.Mask("cert:\n"+pem+"\nend", secretOpts)
	require.Equal(t, 1, n)
	assert.NotContains(t, masked, "BEGIN RSA PRIVATE KEY")
	assert.Contains(t, masked, "{{PRIVATE_KEY_")
}

func TestURLPasswordMasked(t *testing.T) {
	e := New()
	masked, n := e.Mask("postgres://admin:s3cretpw@db.example.com/x", secretOpts)
	require.Equal(t, 1, n)
	assert.NotContains(t, masked, "s3cretpw")
	assert.Contains(t, masked, "{{PASSWORD_")
}

func TestEmailPersonalOnly(t *testing.T) {
	e := New()
	in := "contact bob@example.com"
	_, n1 := e.Mask(in, Options{Secrets: true})
	assert.Equal(t, 0, n1, "email should not be masked when only Secrets on")
	m2, n2 := e.Mask(in, Options{Personal: true})
	require.Equal(t, 1, n2)
	assert.NotContains(t, m2, "bob@example.com")
}

func TestIDCardChecksum(t *testing.T) {
	e := New()
	valid := "110105194912310022"   // correct checksum (check char '2')
	invalid := "110105194912310021" // wrong checksum digit
	m1, n1 := e.Mask("id "+valid, Options{Personal: true})
	require.Equal(t, 1, n1)
	assert.NotContains(t, m1, valid)
	_, n2 := e.Mask("id "+invalid, Options{Personal: true})
	assert.Equal(t, 0, n2, "invalid checksum ID must not be masked")
}

func TestBankCardLuhn(t *testing.T) {
	e := New()
	card := "4111111111111111" // passes Luhn
	m, n := e.Mask("card "+card, Options{Personal: true})
	require.Equal(t, 1, n)
	assert.NotContains(t, m, card)
	// non-Luhn 16-digit number not masked
	_, n2 := e.Mask("card 4111111111111112", Options{Personal: true})
	assert.Equal(t, 0, n2)
}

func TestCustomWords(t *testing.T) {
	e := New()
	// case-insensitive
	m, n := e.Mask("the ProjectX thing", Options{Words: []string{"projectx"}, WordsAnyCase: true})
	require.Equal(t, 1, n)
	assert.NotContains(t, m, "ProjectX")
	assert.Contains(t, m, "{{TERM_")

	// whole-word on: "projectxy" not matched
	_, n2 := e.Mask("projectxy stays", Options{Words: []string{"projectx"}, WordsWhole: true})
	assert.Equal(t, 0, n2)

	// whole-word off: substring matched
	m3, n3 := e.Mask("projectxy masked", Options{Words: []string{"projectx"}})
	require.Equal(t, 1, n3)
	assert.NotContains(t, m3, "projectx")
}

func TestCustomPrefixRule(t *testing.T) {
	e := New()
	rules := []Rule{{Kind: "MYTOKEN", Prefix: "ztk_"}}
	m, n := e.Mask("key ztk_abcdefgh12", Options{Secrets: true, Rules: rules})
	require.Equal(t, 1, n)
	assert.NotContains(t, m, "ztk_abcdefgh12")
	assert.Contains(t, m, "{{MYTOKEN_")
}

func TestCustomRegexRule(t *testing.T) {
	e := New()
	rules := []Rule{{Kind: "HEXID", Regex: `id:([a-f0-9]{12})`}}
	in := "record id:deadbeef1234"
	m, n := e.Mask(in, Options{Secrets: true, Rules: rules})
	require.Equal(t, 1, n)
	assert.NotContains(t, m, "deadbeef1234")
	assert.Contains(t, m, "record id:{{HEXID_")
}

func TestAllowList(t *testing.T) {
	e := New()
	key := "sk-proj-abcdefghij1234567890KLMNOP"
	m, n := e.Mask("key "+key, Options{Secrets: true, Allow: []string{key}})
	assert.Equal(t, 0, n)
	assert.Contains(t, m, key)
}

func TestActionWarn(t *testing.T) {
	e := New()
	key := "sk-proj-abcdefghij1234567890KLMNOP"
	var hits []Hit
	m, n := e.Mask("key "+key, Options{
		Secrets: true,
		Actions: map[string]string{"API_KEY": ActionWarn},
		Seen:    func(h Hit) { hits = append(hits, h) },
	})
	// warn keeps the original value but counts as a reported hit
	assert.Equal(t, 1, n)
	assert.Contains(t, m, key)
	require.Len(t, hits, 1)
	assert.Equal(t, ActionWarn, hits[0].Action)
	assert.Equal(t, KindAPIKey, hits[0].Kind)
}

func TestActionBlock(t *testing.T) {
	e := New()
	key := "sk-proj-abcdefghij1234567890KLMNOP"
	masked, n, blocked := e.MaskRequest("key "+key, Options{
		Secrets: true,
		Actions: map[string]string{"API_KEY": ActionBlock},
	})
	require.True(t, blocked)
	assert.NotContains(t, masked, key)
	assert.Equal(t, 1, n)
}

func TestRestoreRoundTrip(t *testing.T) {
	e := New()
	key := "sk-proj-abcdefghij1234567890KLMNOP"
	email := "alice@example.com"
	in := fmt.Sprintf(`{"key": "%s", "email": "%s"}`, key, email)
	masked, n := e.Mask(in, Options{Secrets: true, Personal: true})
	require.Equal(t, 2, n)
	restored, rn := e.Restore(masked)
	require.Equal(t, 2, rn)
	assert.Equal(t, in, restored)
}

func TestRestoreStrayBraces(t *testing.T) {
	e := New()
	key := "sk-proj-abcdefghij1234567890KLMNOP"
	masked, _ := e.Mask("v "+key, secretOpts)
	ph := extractPlaceholder(t, masked)
	// simulate model output duplicating an edge brace
	wrapped := strings.Replace(masked, ph, "{"+ph, 1)
	restored, n := e.Restore(wrapped)
	require.Equal(t, 1, n)
	assert.Contains(t, restored, key)
}

func TestPreviewCannotRestore(t *testing.T) {
	e := New()
	key := "sk-proj-abcdefghij1234567890KLMNOP"
	p := e.Preview("key "+key, secretOpts)
	assert.NotContains(t, p, key)
	assert.Contains(t, p, "{{API_KEY_")
	restored, n := e.Restore(p)
	assert.Equal(t, 0, n, "preview values are not stored")
	assert.Equal(t, p, restored)
}

func TestStabilityAcrossCalls(t *testing.T) {
	e := New()
	key := "sk-proj-abcdefghij1234567890KLMNOP"
	m1, _ := e.Mask("x "+key, secretOpts)
	m2, _ := e.Mask("y "+key, secretOpts)
	assert.Equal(t, extractPlaceholder(t, m1), extractPlaceholder(t, m2))
}

func TestHasPlaceholder(t *testing.T) {
	e := New()
	assert.False(t, e.HasPlaceholder("nothing here"))
	assert.False(t, e.HasPlaceholder("{ single brace"))
	m, _ := e.Mask("key sk-proj-abcdefghij1234567890KLMNOP", secretOpts)
	assert.True(t, e.HasPlaceholder(m))
}

func TestKindOverrides(t *testing.T) {
	e := New()
	in := "email bob@example.com card 4111111111111111"
	// disable EMAIL explicitly
	m, n := e.Mask(in, Options{Personal: true, Kinds: map[string]bool{"EMAIL": false}})
	assert.Equal(t, 1, n, "only the bank card should be masked")
	assert.Contains(t, m, "bob@example.com")
	assert.NotContains(t, m, "4111111111111111")
}

func TestNoEmptyPlaceholder(t *testing.T) {
	e := New()
	m, n := e.Mask("", secretOpts)
	assert.Equal(t, 0, n)
	assert.Equal(t, "", m)
}

func TestConcurrentMaskRestore(t *testing.T) {
	e := New()
	keys := make([]string, 8)
	for i := range keys {
		keys[i] = fmt.Sprintf("sk-proj-key%08d1234567890ABC", i)
	}
	done := make(chan struct{})
	for i := 0; i < 8; i++ {
		go func(i int) {
			defer func() { done <- struct{}{} }()
			for j := 0; j < 50; j++ {
				m, n := e.Mask("k "+keys[i], secretOpts)
				if n != 1 {
					t.Errorf("expected 1 redaction, got %d", n)
				}
				r, rn := e.Restore(m)
				if rn != 1 || !strings.Contains(r, keys[i]) {
					t.Errorf("restore failed: n=%d text=%q", rn, r)
				}
				_ = e.HasPlaceholder(m)
				_ = e.Preview("p "+keys[i], secretOpts)
			}
		}(i)
	}
	for i := 0; i < 8; i++ {
		<-done
	}
}

func TestVendorKeyShapes(t *testing.T) {
	e := New()
	cases := []string{
		"github_pat_11ABCDEFG0123456789_abcdefghij",
		"AIzaSyA1234567890abcdefghijklmnopqrsABC",
		"xoxb-123456789012-1234567890123",
		"glpat-abcdefg123456789",
		"hf_aBcDeFgHiJkLmNoPqRsT",
		"cfat_abcdefghijklmnopqrsu",
		"pypi-AgEIcHlwaS5vcmcA67gBCAA",
		"dop_v1_a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0",
		// Built at runtime: a literal SendGrid-shaped key trips GitHub's
		// secret scanner even though it is fake.
		"S" + "G." + "ECcNQf2suOoOOsgAwpIYiI" + "." + "P9K2zzSxxnF1eKUpTXYgwMzqHxqcAsblDvliin8NvY4",
		"AKIAIOSFODNN7EXAMPLE",
		"ASIAIOSFODNN7EXAMPLE",
		"sk_live_abcdefghijklmnop",
	}
	for _, key := range cases {
		m, n := e.Mask("k "+key, secretOpts)
		require.Equal(t, 1, n, "expected %q to be masked", key)
		assert.NotContains(t, m, key)
	}
}

func TestPhoneLabelRule(t *testing.T) {
	e := New()
	m, n := e.Mask("phone: 13812345678", Options{Personal: true})
	require.Equal(t, 1, n)
	assert.NotContains(t, m, "13812345678")

	m2, n2 := e.Mask("他的电话是 13812345678", Options{Personal: true})
	require.Equal(t, 1, n2)
	assert.NotContains(t, m2, "13812345678")

	// a bare mobile passes the shape rule (digit boundaries) and is masked
	m3, n3 := e.Mask("order 13812345678", Options{Personal: true})
	require.Equal(t, 1, n3)
	assert.NotContains(t, m3, "13812345678")

	// a long number that is not a mobile shape is not masked
	_, n4 := e.Mask("order 861234567890123", Options{Personal: true})
	assert.Equal(t, 0, n4, "non-mobile long number should not be masked")
}

func TestDecimalNotMasked(t *testing.T) {
	e := New()
	m, n := e.Mask("pi is 3.14159 and version 1.2.3", Options{Personal: true, Secrets: true})
	assert.Equal(t, 0, n)
	assert.Equal(t, "pi is 3.14159 and version 1.2.3", m)
}

func TestAllowCaseInsensitive(t *testing.T) {
	e := New()
	m, n := e.Mask("word SpecialToken here", Options{Words: []string{"specialtoken"}, Allow: []string{"specialtoken"}})
	assert.Equal(t, 0, n)
	assert.Contains(t, m, "SpecialToken")
}

func TestCustomKindNormalized(t *testing.T) {
	e := New()
	rules := []Rule{{Kind: "my-token", Prefix: "ztk_"}}
	m, n := e.Mask("key ztk_abcdefgh12", Options{Secrets: true, Rules: rules})
	require.Equal(t, 1, n)
	assert.Contains(t, m, "{{MY_TOKEN_")
}

func TestRestoreCaseTolerant(t *testing.T) {
	e := New()
	key := "sk-proj-abcdefghij1234567890KLMNOP"
	m, _ := e.Mask("key "+key, secretOpts)
	ph := extractPlaceholder(t, m)
	// model may alter case of the tag; restore must tolerate it
	mangled := strings.ToUpper(ph)
	restored, n := e.Restore(mangled)
	require.Equal(t, 1, n)
	assert.Contains(t, restored, key)
}

func TestPlaceholderKindTagLayout(t *testing.T) {
	e := New()
	key := "sk-proj-abcdefghij1234567890KLMNOP"
	m, _ := e.Mask("key "+key, secretOpts)
	ph := extractPlaceholder(t, m)
	// {{KIND_ + 8 lowercase base32 chars + }}
	assert.Regexp(t, `^\{\{[A-Z][A-Z0-9_]*_[a-z2-7]{8}\}\}$`, ph)
}

func TestMaskRequestWarnNotBlocked(t *testing.T) {
	e := New()
	key := "sk-proj-abcdefghij1234567890KLMNOP"
	_, _, blocked := e.MaskRequest("key "+key, Options{
		Secrets: true,
		Actions: map[string]string{"API_KEY": ActionWarn},
	})
	assert.False(t, blocked)
}

func TestRuleNeverEmptyMatch(t *testing.T) {
	e := New()
	rules := []Rule{{Regex: `x*`}}
	_, n := e.Mask("aaa", Options{Secrets: true, Rules: rules})
	assert.Equal(t, 0, n)
}

// extractPlaceholder pulls the first {{KIND_...}} out of masked text.
func extractPlaceholder(t *testing.T, s string) string {
	t.Helper()
	i := strings.Index(s, "{{")
	require.GreaterOrEqual(t, i, 0, "no placeholder in %q", s)
	j := strings.Index(s[i:], "}}")
	require.GreaterOrEqual(t, j, 0)
	return s[i : i+j+2]
}
