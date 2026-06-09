package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
)

// masterKey is loaded once from MASTER_KEY env var (32-byte hex string).
var masterKey []byte

func init() {
	raw := os.Getenv("MASTER_KEY")
	if raw == "" {
		return
	}
	b, err := hex.DecodeString(raw)
	if err != nil || len(b) != 32 {
		panic("MASTER_KEY must be a 32-byte hex string (64 hex chars)")
	}
	masterKey = b
}

// EncryptKey encrypts a plaintext API key with AES-256-GCM using the master key.
// Returns the ciphertext (nonce prepended).
func EncryptKey(plaintext string) ([]byte, error) {
	if len(masterKey) == 0 {
		return nil, errors.New("MASTER_KEY not configured")
	}
	block, err := aes.NewCipher(masterKey)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}
	return gcm.Seal(nonce, nonce, []byte(plaintext), nil), nil
}

// DecryptKey decrypts a ciphertext produced by EncryptKey.
func DecryptKey(ciphertext []byte) (string, error) {
	if len(masterKey) == 0 {
		return "", errors.New("MASTER_KEY not configured")
	}
	block, err := aes.NewCipher(masterKey)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(ciphertext) < gcm.NonceSize() {
		return "", errors.New("ciphertext too short")
	}
	nonce := ciphertext[:gcm.NonceSize()]
	ct := ciphertext[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt: %w", err)
	}
	return string(plain), nil
}
