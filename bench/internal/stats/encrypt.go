package stats

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/rthomazel/mcp/bench/internal"
)

// LoadKey reads the encryption key from the Docker Secret file.
// Returns nil, nil if the file does not exist or is empty — stats run without encryption.
func LoadKey() ([]byte, error) {
	raw, err := os.ReadFile(internal.StatsEncryptionKeyPath)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read stats key: %w", err)
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(raw)))
	if err != nil {
		return nil, fmt.Errorf("decode stats key: %w", err)
	}
	if len(decoded) != 32 {
		return nil, fmt.Errorf("stats key must be 32 bytes, got %d", len(decoded))
	}
	return decoded, nil
}

// Encrypt encrypts plaintext with AES-256-GCM using a fresh random nonce.
// Returns the envelope: "v1:<base64 nonce>:<base64 ciphertext+tag>".
func Encrypt(key []byte, plaintext string) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("new cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("new gcm: %w", err)
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}
	ciphertext := aead.Seal(nil, nonce, []byte(plaintext), nil)
	return "v1:" +
		base64.StdEncoding.EncodeToString(nonce) + ":" +
		base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts a "v1:<b64nonce>:<b64ct+tag>" envelope.
func Decrypt(key []byte, envelope string) (string, error) {
	parts := strings.SplitN(envelope, ":", 3)
	if len(parts) != 3 || parts[0] != "v1" {
		return "", fmt.Errorf("unrecognised envelope format")
	}
	nonce, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return "", fmt.Errorf("decode nonce: %w", err)
	}
	ciphertext, err := base64.StdEncoding.DecodeString(parts[2])
	if err != nil {
		return "", fmt.Errorf("decode ciphertext: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("new cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("new gcm: %w", err)
	}
	if len(nonce) != aead.NonceSize() {
		return "", fmt.Errorf("invalid nonce size: got %d, want %d", len(nonce), aead.NonceSize())
	}
	plaintext, err := aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt: %w", err)
	}
	return string(plaintext), nil
}
