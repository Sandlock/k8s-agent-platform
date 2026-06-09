// Package auth handles password hashing and session token management.
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"golang.org/x/crypto/argon2"
)

const (
	argonTime    = 1
	argonMemory  = 64 * 1024
	argonThreads = 4
	argonKeyLen  = 32
	saltLen      = 16
	tokenBytes   = 32
)

// HashPassword hashes a plaintext password with argon2id and a random salt.
// Returns a storable string: hex(salt) + "$" + hex(hash).
func HashPassword(password string) (string, error) {
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	hash := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	return hex.EncodeToString(salt) + "$" + hex.EncodeToString(hash), nil
}

// CheckPassword verifies a plaintext password against a stored hash string.
func CheckPassword(password, stored string) bool {
	if len(stored) < saltLen*2+1 {
		return false
	}
	saltHex := stored[:saltLen*2]
	hashHex := stored[saltLen*2+1:]
	salt, err := hex.DecodeString(saltHex)
	if err != nil {
		return false
	}
	expected, err := hex.DecodeString(hashHex)
	if err != nil {
		return false
	}
	actual := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	if len(actual) != len(expected) {
		return false
	}
	var diff byte
	for i := range actual {
		diff |= actual[i] ^ expected[i]
	}
	return diff == 0
}

// NewToken generates a cryptographically random session token.
// Returns the raw token (sent to client) and its SHA-256 hash (stored in DB).
func NewToken() (token, tokenHash string, err error) {
	b := make([]byte, tokenBytes)
	if _, err = rand.Read(b); err != nil {
		return "", "", fmt.Errorf("generate token: %w", err)
	}
	token = hex.EncodeToString(b)
	tokenHash = HashToken(token)
	return token, tokenHash, nil
}

// HashToken returns the SHA-256 hex hash of a raw token.
func HashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}
