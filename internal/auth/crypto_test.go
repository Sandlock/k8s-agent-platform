package auth

import (
	"os"
	"testing"
)

func withMasterKey(t *testing.T, fn func()) {
	t.Helper()
	// 32-byte test key
	os.Setenv("MASTER_KEY", "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20")
	masterKey = []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16,
		17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32}
	t.Cleanup(func() {
		os.Unsetenv("MASTER_KEY")
		masterKey = nil
	})
	fn()
}

func TestEncryptDecryptKey(t *testing.T) {
	withMasterKey(t, func() {
		plaintext := "sk-ant-test-key-1234567890"
		ct, err := EncryptKey(plaintext)
		if err != nil {
			t.Fatal(err)
		}
		// Ciphertext must not contain the plaintext.
		if string(ct) == plaintext {
			t.Error("ciphertext is plaintext")
		}
		got, err := DecryptKey(ct)
		if err != nil {
			t.Fatal(err)
		}
		if got != plaintext {
			t.Errorf("got %q, want %q", got, plaintext)
		}
	})
}

func TestEncryptProducesUniqueCiphertexts(t *testing.T) {
	withMasterKey(t, func() {
		ct1, _ := EncryptKey("same-key")
		ct2, _ := EncryptKey("same-key")
		if string(ct1) == string(ct2) {
			t.Error("two encryptions of the same key must differ (random nonce)")
		}
	})
}

func TestDecryptRejectsGarbage(t *testing.T) {
	withMasterKey(t, func() {
		_, err := DecryptKey([]byte("this is not valid ciphertext"))
		if err == nil {
			t.Error("expected error decrypting garbage")
		}
	})
}

func TestEncryptWithoutMasterKey(t *testing.T) {
	saved := masterKey
	masterKey = nil
	t.Cleanup(func() { masterKey = saved })

	_, err := EncryptKey("anything")
	if err == nil {
		t.Error("expected error when MASTER_KEY not set")
	}
}
