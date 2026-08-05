package utils

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
)

// GenerateRandomURLSafeString menghasilkan string acak URL-safe (dipakai untuk state & code_verifier)
func GenerateRandomURLSafeString(byteLength int) (string, error) {
	b := make([]byte, byteLength)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// GenerateCodeChallengeS256 menghasilkan code_challenge dari code_verifier sesuai spesifikasi PKCE (RFC 7636)
func GenerateCodeChallengeS256(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}
