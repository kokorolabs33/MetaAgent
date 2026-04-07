package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func computeHMAC(payload []byte, secret string) string { // pragma: allowlist secret
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}

func TestVerifyHMAC_CorrectSecret(t *testing.T) {
	payload := []byte(`{"event":"push"}`)
	secret := "my-test-key" // pragma: allowlist secret
	sig := computeHMAC(payload, secret)

	if !VerifyHMAC(payload, sig, secret, "") {
		t.Fatal("expected VerifyHMAC to return true for correct secret")
	}
}

func TestVerifyHMAC_WrongSecret(t *testing.T) {
	payload := []byte(`{"event":"push"}`)
	secret := "my-test-key"                  // pragma: allowlist secret
	sig := computeHMAC(payload, "wrong-key") // pragma: allowlist secret

	if VerifyHMAC(payload, sig, secret, "") {
		t.Fatal("expected VerifyHMAC to return false for wrong secret")
	}
}

func TestVerifyHMAC_EmptySignature(t *testing.T) {
	payload := []byte(`{"event":"push"}`)

	if VerifyHMAC(payload, "", "my-test-key", "") { // pragma: allowlist secret
		t.Fatal("expected VerifyHMAC to return false for empty signature")
	}
}

func TestVerifyHMAC_DualSecretRotation(t *testing.T) {
	payload := []byte(`{"event":"push"}`)
	oldSecret := "old-key" // pragma: allowlist secret
	newSecret := "new-key" // pragma: allowlist secret
	// Signature was made with old secret (before rotation)
	sig := computeHMAC(payload, oldSecret)

	// Primary is newSecret, previous is oldSecret -- should still pass
	if !VerifyHMAC(payload, sig, newSecret, oldSecret) {
		t.Fatal("expected VerifyHMAC to return true when previous_secret matches")
	}
}

func TestVerifyHMAC_DualSecretPrimaryStillWorks(t *testing.T) {
	payload := []byte(`{"event":"push"}`)
	newSecret := "new-key" // pragma: allowlist secret
	sig := computeHMAC(payload, newSecret)

	// Primary is newSecret, previous is oldSecret -- primary should still work
	if !VerifyHMAC(payload, sig, newSecret, "old-key") { // pragma: allowlist secret
		t.Fatal("expected VerifyHMAC to return true for primary secret with previous set")
	}
}

func TestVerifyHMAC_Sha256PrefixFormat(t *testing.T) {
	payload := []byte(`{"event":"push"}`)
	secret := "my-test-key" // pragma: allowlist secret
	rawSig := computeHMAC(payload, secret)
	prefixedSig := "sha256=" + rawSig

	if !VerifyHMAC(payload, prefixedSig, secret, "") {
		t.Fatal("expected VerifyHMAC to accept sha256= prefixed signature (GitHub format)")
	}
}

func TestVerifyHMAC_RawHexFormat(t *testing.T) {
	payload := []byte(`{"event":"push"}`)
	secret := "my-test-key" // pragma: allowlist secret
	rawSig := computeHMAC(payload, secret)

	if !VerifyHMAC(payload, rawSig, secret, "") {
		t.Fatal("expected VerifyHMAC to accept raw hex signature")
	}
}

func TestVerifyHMAC_NeitherSecretMatches(t *testing.T) {
	payload := []byte(`{"event":"push"}`)
	sig := computeHMAC(payload, "totally-different") // pragma: allowlist secret

	if VerifyHMAC(payload, sig, "key-a", "key-b") { // pragma: allowlist secret
		t.Fatal("expected VerifyHMAC to return false when neither secret matches")
	}
}

func TestVerifyHMAC_EmptyPreviousSecretNotTried(t *testing.T) {
	payload := []byte(`{"event":"push"}`)
	sig := computeHMAC(payload, "wrong-key") // pragma: allowlist secret

	// previousSecret is empty, should not match anything
	if VerifyHMAC(payload, sig, "correct-key", "") { // pragma: allowlist secret
		t.Fatal("expected VerifyHMAC to return false; empty previousSecret should not help")
	}
}
