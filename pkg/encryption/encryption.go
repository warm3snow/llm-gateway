package encryption

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"io"
)

// AES-256-GCM encryption for API keys and sensitive data

var encryptionKey []byte

// InitEncryptionKey initializes the encryption key from a 64-char hex string (32 bytes)
func InitEncryptionKey(hexKey string) error {
	if hexKey == "" {
		// Generate a random key
		key := make([]byte, 32)
		if _, err := io.ReadFull(rand.Reader, key); err != nil {
			return err
		}
		encryptionKey = key
		return nil
	}

	key, err := hex.DecodeString(hexKey)
	if err != nil {
		return errors.New("invalid encryption key: must be 64-character hex string (32 bytes)")
	}
	if len(key) != 32 {
		return errors.New("invalid encryption key length: must be 32 bytes (64 hex chars)")
	}
	encryptionKey = key
	return nil
}

// GetEncryptionKey returns the current encryption key, generating one if not set
func GetEncryptionKey() []byte {
	if encryptionKey == nil {
		_ = InitEncryptionKey("")
	}
	return encryptionKey
}

// Encrypt encrypts plaintext using AES-256-GCM
// Returns base64-encoded ciphertext (nonce prepended)
func Encrypt(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}

	block, err := aes.NewCipher(GetEncryptionKey())
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts a base64-encoded ciphertext (from Encrypt)
func Decrypt(encoded string) (string, error) {
	if encoded == "" {
		return "", nil
	}

	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(GetEncryptionKey())
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", errors.New("ciphertext too short")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}

// HashVirtualKey creates a SHA-256 hash of the virtual key for storage
func HashVirtualKey(key, salt string) string {
	h := sha256.Sum256([]byte(key + salt))
	return hex.EncodeToString(h[:])
}

// GenerateSalt creates a random 16-byte salt, hex-encoded
func GenerateSalt() string {
	b := make([]byte, 16)
	io.ReadFull(rand.Reader, b)
	return hex.EncodeToString(b)
}

// MaskAPIKey returns a masked version of an API key for display
func MaskAPIKey(key string) string {
	if len(key) <= 8 {
		return "****"
	}
	return key[:4] + "..." + key[len(key)-4:]
}
