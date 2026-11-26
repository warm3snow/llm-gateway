package redact

import (
	"sync"
	"unicode"
	"unicode/utf8"
)

// store maps placeholder -> original value with a bounded size. Eviction
// drops the half of entries with the oldest "last used" clock stamp.
type store struct {
	mu       sync.Mutex
	m        map[string]string
	lastUsed map[string]uint64
	clock    uint64
	max      int
}

func newStore(max int) *store {
	if max <= 0 {
		max = maxValues
	}
	return &store{m: make(map[string]string), lastUsed: make(map[string]uint64), max: max}
}

func (s *store) put(tag, value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.m[tag]; !ok && len(s.m) >= s.max {
		s.evictOldestLocked()
	}
	s.clock++
	s.m[tag] = value
	s.lastUsed[tag] = s.clock
}

// evictOldestLocked removes the half of entries with the smallest
// last-used clock values.
func (s *store) evictOldestLocked() {
	type entry struct {
		tag   string
		stamp uint64
	}
	entries := make([]entry, 0, len(s.lastUsed))
	for tag, stamp := range s.lastUsed {
		entries = append(entries, entry{tag, stamp})
	}
	for i := 1; i < len(entries); i++ {
		for j := i; j > 0 && entries[j].stamp < entries[j-1].stamp; j-- {
			entries[j], entries[j-1] = entries[j-1], entries[j]
		}
	}
	half := len(entries) / 2
	if half < 1 {
		half = 1
	}
	for i := 0; i < half; i++ {
		delete(s.m, entries[i].tag)
		delete(s.lastUsed, entries[i].tag)
	}
}

func (s *store) get(tag string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.m[tag]
	if ok {
		s.clock++
		s.lastUsed[tag] = s.clock
	}
	return v, ok
}

func (s *store) len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.m)
}

// ---- validators used by built-in rules --------------------------------------

// luhnValid reports whether the digits pass the Luhn checksum.
func luhnValid(digits string) bool {
	sum, alt := 0, false
	for i := len(digits) - 1; i >= 0; i-- {
		d := digits[i] - '0'
		if alt {
			d *= 2
			if d > 9 {
				d -= 9
			}
		}
		sum += int(d)
		alt = !alt
	}
	return sum%10 == 0
}

var idCardWeights = [17]int{7, 9, 10, 5, 8, 4, 2, 1, 6, 3, 7, 9, 10, 5, 8, 4, 2}
var idCardMapped = [11]byte{'1', '0', 'X', '9', '8', '7', '6', '5', '4', '3', '2'}

// idCardCheckChar computes the GB 11643-1999 checksum character for the
// first 17 digits of a Chinese resident ID number: check = (12 - sum%11)
// mapped through "10X98765432".
func idCardCheckChar(first17 string) byte {
	sum := 0
	for i := 0; i < 17; i++ {
		sum += int(first17[i]-'0') * idCardWeights[i]
	}
	check := (12 - sum%11) % 11
	return idCardMapped[check]
}

// idCardValid validates a Chinese 18-digit resident ID checksum.
func idCardValid(id string) bool {
	if len(id) != 18 {
		return false
	}
	for i := 0; i < 17; i++ {
		if id[i] < '0' || id[i] > '9' {
			return false
		}
	}
	last := id[17]
	if last == 'x' {
		last = 'X'
	}
	if last != 'X' && (last < '0' || last > '9') {
		return false
	}
	return last == idCardCheckChar(id[:17])
}

// looksLikeSecret is a generic heuristic: the value must contain both
// letters and digits, or be at least 20 chars, or look like hex.
func looksLikeSecret(v string) bool {
	if len(v) >= 20 {
		return true
	}
	var hasLetter, hasDigit bool
	for _, r := range v {
		switch {
		case unicode.IsLetter(r):
			hasLetter = true
		case unicode.IsDigit(r):
			hasDigit = true
		}
	}
	if hasLetter && hasDigit {
		return true
	}
	// hex-only string of decent length also counts
	return !hasDigit && hasLetter && isHex(v) && len(v) >= 16
}

func isHex(s string) bool {
	for _, r := range s {
		if !(r >= '0' && r <= '9' || r >= 'a' && r <= 'f' || r >= 'A' && r <= 'F') {
			return false
		}
	}
	return len(s) > 0
}

func isDigit(b byte) bool { return b >= '0' && b <= '9' }

// isTokenChar reports whether r belongs to a longer token; used by the
// boundary check to reject matches embedded inside bigger identifiers.
func isTokenChar(r rune) bool {
	return r == '_' || r == '-' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

// boundaryOK rejects matches embedded inside longer tokens.
func boundaryOK(s string, start, end int) bool {
	if start > 0 {
		if r, _ := utf8.DecodeLastRuneInString(s[:start]); isTokenChar(r) {
			return false
		}
	}
	if end < len(s) {
		if r, _ := utf8.DecodeRuneInString(s[end:]); isTokenChar(r) {
			return false
		}
	}
	return true
}

// isDigitByteAt reports whether the byte at pos is an ASCII digit; out of
// range positions count as non-digits.
func isDigitByteAt(s string, pos int) bool {
	if pos < 0 || pos >= len(s) {
		return false
	}
	return isDigit(s[pos])
}
