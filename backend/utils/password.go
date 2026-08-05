package utils

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"

	"pasti-v3-backend/config"

	"golang.org/x/crypto/bcrypt"
)

// GenerateSalt membuat random salt unik per user (256-bit)
func GenerateSalt() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(b), nil
}

// combineWithPepper menggabungkan password + salt (per-user) + pepper (rahasia server, dari env)
// lalu di-hash SHA-256 dulu supaya panjang input konsisten sebelum masuk bcrypt
// (bcrypt punya batas maksimal 72 byte input).
func combineWithPepper(password, salt string) string {
	pepper := config.Cfg.PasswordPepper
	combined := password + salt + pepper
	sum := sha256.Sum256([]byte(combined))
	return hex.EncodeToString(sum[:])
}

// HashPassword: password mentah -> salt unik -> gabung pepper -> bcrypt hash
func HashPassword(password string) (hash string, salt string, err error) {
	salt, err = GenerateSalt()
	if err != nil {
		return "", "", err
	}

	preHashed := combineWithPepper(password, salt)

	hashBytes, err := bcrypt.GenerateFromPassword([]byte(preHashed), bcrypt.DefaultCost)
	if err != nil {
		return "", "", err
	}

	return string(hashBytes), salt, nil
}

// VerifyPassword membandingkan password input dengan hash tersimpan
func VerifyPassword(password, salt, hash string) error {
	preHashed := combineWithPepper(password, salt)
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(preHashed))
	if err != nil {
		return errors.New("password tidak cocok")
	}
	return nil
}
