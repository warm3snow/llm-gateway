package redact

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"strings"
)

var (
	base32NoPad = base32.StdEncoding.WithPadding(base32.NoPadding)
	cryptoRand  = rand.Reader
)

// placeholderTag derives the deterministic 8-char lowercase base32 tag for
// kind+value via HMAC-SHA256. Same kind+value always yields the same tag.
func placeholderTag(key []byte, kind, value string) string {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(kind))
	mac.Write([]byte{0})
	mac.Write([]byte(value))
	return strings.ToLower(base32NoPad.EncodeToString(mac.Sum(nil))[:tagLen])
}

// randomKey generates 32 random bytes for HMAC tagging. In-memory only,
// never persisted or logged.
func randomKey() []byte {
	key := make([]byte, 32)
	if _, err := cryptoRand.Read(key); err != nil {
		// crypto/rand failure is unrecoverable; fall back to a fixed,
		// process-local key rather than panicking.
		for i := range key {
			key[i] = byte(i*7 + 1)
		}
	}
	return key
}
