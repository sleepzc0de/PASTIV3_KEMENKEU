package utils

import (
	"strings"

	"pasti-v3-backend/config"
)

// daftarNilai memecah nilai env berisi satu atau beberapa entri (dipisah koma, titik koma, atau spasi) menjadi daftar tanpa entri kosong.
func daftarNilai(s string) []string {
	return strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ';' || r == ' ' || r == '\t' || r == '\n' || r == '\r' })
}

// IsProtectedIdentity memeriksa apakah email/NIP termasuk superadmin permanen. Superadmin hanya ditetapkan di .env (SUPERADMIN_PROTECTED_EMAIL dan
// SUPERADMIN_PROTECTED_NIP; masing-masing boleh berisi beberapa entri dipisah koma) dan hanya berlaku untuk identitas yang berasal dari SSO Kemenkeu.
// Superadmin tidak boleh dihapus, dinonaktifkan, atau diubah rolenya, dan tidak ada jalur lain untuk menjadi superadmin.
func IsProtectedIdentity(email, nip string) bool {
	cfg := config.Cfg
	if cfg == nil {
		return false
	}
	email, nip = strings.TrimSpace(email), strings.TrimSpace(nip)
	if email != "" {
		for _, e := range daftarNilai(cfg.SuperadminProtectedEmail) {
			if strings.EqualFold(email, e) {
				return true
			}
		}
	}
	if nip != "" {
		for _, n := range daftarNilai(cfg.SuperadminProtectedNIP) {
			if nip == n {
				return true
			}
		}
	}
	return false
}

// SuperadminTerkonfigurasi: .env menetapkan minimal satu identitas superadmin. Tanpa itu tidak ada yang boleh diturunkan dari superadmin secara otomatis
// (konfigurasi yang terlewat tidak boleh mengunci semua superadmin).
func SuperadminTerkonfigurasi() bool {
	cfg := config.Cfg
	return cfg != nil && (len(daftarNilai(cfg.SuperadminProtectedEmail)) > 0 || len(daftarNilai(cfg.SuperadminProtectedNIP)) > 0)
}
