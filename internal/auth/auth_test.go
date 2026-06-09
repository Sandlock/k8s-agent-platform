package auth

import (
	"strings"
	"testing"
)

func TestHashAndCheckPassword(t *testing.T) {
	hash, err := HashPassword("correct-horse-battery-staple")
	if err != nil {
		t.Fatal(err)
	}
	if !CheckPassword("correct-horse-battery-staple", hash) {
		t.Error("correct password not accepted")
	}
	if CheckPassword("wrong-password", hash) {
		t.Error("wrong password accepted")
	}
}

func TestHashPasswordUnique(t *testing.T) {
	h1, _ := HashPassword("same")
	h2, _ := HashPassword("same")
	if h1 == h2 {
		t.Error("two hashes of the same password must differ (random salt)")
	}
}

func TestNewToken(t *testing.T) {
	tok, hash, err := NewToken()
	if err != nil {
		t.Fatal(err)
	}
	if len(tok) == 0 || len(hash) == 0 {
		t.Error("empty token or hash")
	}
	if HashToken(tok) != hash {
		t.Error("HashToken(token) != tokenHash from NewToken")
	}
}

func TestHashTokenDeterministic(t *testing.T) {
	h1 := HashToken("mytoken")
	h2 := HashToken("mytoken")
	if h1 != h2 {
		t.Error("HashToken must be deterministic")
	}
	if strings.Contains(h1, "mytoken") {
		t.Error("HashToken must not contain plaintext")
	}
}
