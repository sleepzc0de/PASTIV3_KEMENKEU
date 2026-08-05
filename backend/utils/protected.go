package utils

import (
	"strings"

	"pasti-v3-backend/config"
)

// IsProtectedIdentity memeriksa apakah email/NIP termasuk superadmin permanen
// yang tidak boleh dihapus, dinonaktifkan, atau diubah rolenya.
func IsProtectedIdentity(email, nip string) bool {
	cfg := config.Cfg

	if cfg.SuperadminProtectedEmail != "" &&
		strings.EqualFold(strings.TrimSpace(email), strings.TrimSpace(cfg.SuperadminProtectedEmail)) {
		return true
	}
	if cfg.SuperadminProtectedNIP != "" && nip != "" &&
		strings.TrimSpace(nip) == strings.TrimSpace(cfg.SuperadminProtectedNIP) {
		return true
	}
	return false
}
