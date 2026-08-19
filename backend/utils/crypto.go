package utils

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"

	"pasti-v3-backend/config"
)

func getEncryptionKey() ([]byte, error) {
	keyB64 := strings.TrimSpace(config.Cfg.TokenEncryptionKey)
	if keyB64 == "" {
		return nil, errors.New("TOKEN_ENCRYPTION_KEY belum dikonfigurasi di .env")
	}

	// Coba decode dengan padding standar dulu, fallback ke raw (tanpa padding)
	// kalau ternyata tanda '=' di akhir sempat terpotong saat copy-paste.
	key, err := base64.StdEncoding.DecodeString(keyB64)
	if err != nil {
		key, err = base64.RawStdEncoding.DecodeString(keyB64)
	}
	if err != nil {
		return nil, fmt.Errorf("TOKEN_ENCRYPTION_KEY bukan base64 valid: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("TOKEN_ENCRYPTION_KEY harus menghasilkan 32 byte setelah decode, saat ini %d byte", len(key))
	}
	return key, nil
}

// EncryptString mengenkripsi token sensitif (access_token/refresh_token SSO)
// sebelum disimpan ke database, memakai AES-256-GCM.
func EncryptString(plaintext string) (string, error) {
	key, err := getEncryptionKey()
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
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

func DecryptString(encoded string) (string, error) {
	key, err := getEncryptionKey()
	if err != nil {
		return "", err
	}
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", errors.New("ciphertext tidak valid")
	}
	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}
