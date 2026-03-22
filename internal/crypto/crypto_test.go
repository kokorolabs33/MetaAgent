package crypto

import (
	"encoding/hex"
	"testing"
)

func testKey() string {
	// 32 bytes = 64 hex chars
	return "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef" // pragma: allowlist secret
}

func TestEncryptDecrypt(t *testing.T) {
	plaintext := []byte(`{"token":"sk-secret-123"}`)
	encrypted, err := Encrypt(plaintext, testKey())
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if encrypted == "" {
		t.Fatal("encrypted is empty")
	}
	// Encrypted should be hex
	if _, err := hex.DecodeString(encrypted); err != nil {
		t.Fatalf("encrypted is not valid hex: %v", err)
	}

	decrypted, err := Decrypt(encrypted, testKey())
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if string(decrypted) != string(plaintext) {
		t.Errorf("got %q, want %q", string(decrypted), string(plaintext))
	}
}

func TestDecryptWrongKey(t *testing.T) {
	plaintext := []byte("secret data")
	encrypted, err := Encrypt(plaintext, testKey())
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	wrongKey := "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789" // pragma: allowlist secret
	_, err = Decrypt(encrypted, wrongKey)
	if err == nil {
		t.Fatal("expected error decrypting with wrong key")
	}
}

func TestEncryptInvalidKey(t *testing.T) {
	_, err := Encrypt([]byte("data"), "tooshort")
	if err == nil {
		t.Fatal("expected error for short key")
	}
}

func TestDecryptInvalidCiphertext(t *testing.T) {
	_, err := Decrypt("not-hex", testKey())
	if err == nil {
		t.Fatal("expected error for invalid hex")
	}
}
