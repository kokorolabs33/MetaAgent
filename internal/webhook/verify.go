package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// VerifyHMAC checks that the given signature matches the HMAC-SHA256 of payload
// using either the primary secret or the previousSecret (for dual-secret rotation
// during grace periods). The signature may be in raw hex or "sha256=<hex>" format
// (GitHub convention). Returns true if either secret produces a matching signature.
func VerifyHMAC(payload []byte, signature, secret, previousSecret string) bool { // pragma: allowlist secret
	if signature == "" {
		return false
	}

	// Strip optional "sha256=" prefix (GitHub format)
	sig := strings.TrimPrefix(signature, "sha256=")

	sigBytes, err := hex.DecodeString(sig)
	if err != nil {
		return false
	}

	// Try primary secret
	if checkMAC(payload, sigBytes, secret) {
		return true
	}

	// Try previous secret for dual-secret rotation (D-03)
	if previousSecret != "" && checkMAC(payload, sigBytes, previousSecret) { // pragma: allowlist secret
		return true
	}

	return false
}

// checkMAC computes HMAC-SHA256 of payload with the given key and compares
// using constant-time comparison to prevent timing attacks.
func checkMAC(payload, expectedMAC []byte, key string) bool {
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write(payload)
	computed := mac.Sum(nil)
	return hmac.Equal(computed, expectedMAC)
}
