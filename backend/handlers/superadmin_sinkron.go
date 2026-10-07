package handlers

import (
	"database/sql"
	"log"

	"pasti-v3-backend/database"
	"pasti-v3-backend/peran"
	"pasti-v3-backend/utils"
)

// Superadmin ditetapkan HANYA di .env (SUPERADMIN_PROTECTED_EMAIL dan SUPERADMIN_PROTECTED_NIP) dan hanya berlaku untuk akun SSO Kemenkeu, yang email dan NIP-nya
// berasal dari SSO (bukan isian pengguna). SinkronkanSuperadmin dijalankan saat server dimulai: akun yang cocok dengan .env dinaikkan menjadi superadmin, dan akun lain
// yang masih berrole superadmin atau bertanda protected (mis. sisa data lama atau .env yang diubah) diturunkan menjadi pengguna biasa. Saat SSO login, akun yang cocok
// juga dinaikkan (upsertUserFromSSO). Role dibaca dari database pada setiap permintaan, jadi penurunan langsung berlaku.

// keputusanSuperadmin menentukan perubahan untuk satu akun menurut .env. Murni (tanpa database) agar mudah diuji.
func keputusanSuperadmin(authProvider, role string, protected, aktif bool, email, nip string) (naik, turun bool) {
	cocok := authProvider == "sso" && utils.IsProtectedIdentity(email, nip)
	if cocok {
		return role != peran.AkunSuperadmin || !protected || !aktif, false
	}
	return false, role == peran.AkunSuperadmin || protected
}

// SinkronkanSuperadmin menerapkan keputusanSuperadmin pada semua akun yang relevan. Tanpa identitas superadmin di .env tidak ada yang diubah (konfigurasi yang
// terlewat tidak boleh menurunkan semua superadmin).
func SinkronkanSuperadmin() {
	if !utils.SuperadminTerkonfigurasi() {
		log.Println("[SUPERADMIN WARN] SUPERADMIN_PROTECTED_EMAIL/NIP kosong di .env: tidak ada superadmin yang ditetapkan, dan tidak ada akun yang diturunkan")
		return
	}
	rows, err := database.DB.Query(`
		SELECT CONVERT(NVARCHAR(36), u.id), u.role, u.is_protected, u.is_active, u.email, e.nip, u.auth_provider
		FROM users u LEFT JOIN employees e ON e.id = u.employee_id
		WHERE u.auth_provider = N'sso' OR u.role = N'superadmin' OR u.is_protected = 1`)
	if err != nil {
		log.Println("[SUPERADMIN ERROR] gagal membaca akun:", err)
		return
	}
	type akun struct {
		id          string
		naik, turun bool
		email       string
	}
	var ubah []akun
	for rows.Next() {
		var id, role, email, provider string
		var protected, aktif bool
		var nip sql.NullString
		if err := rows.Scan(&id, &role, &protected, &aktif, &email, &nip, &provider); err != nil {
			log.Println("[SUPERADMIN ERROR] gagal membaca baris akun:", err)
			continue
		}
		if naik, turun := keputusanSuperadmin(provider, role, protected, aktif, email, nip.String); naik || turun {
			ubah = append(ubah, akun{id: id, naik: naik, turun: turun, email: email})
		}
	}
	rows.Close()

	for _, a := range ubah {
		var err error
		switch {
		case a.naik:
			_, err = database.DB.Exec(`UPDATE users SET role = N'superadmin', is_protected = 1, is_active = 1, updated_at = SYSUTCDATETIME() WHERE id = @p1`, a.id)
			log.Println("[SUPERADMIN] akun dijadikan superadmin menurut .env:", a.email)
		case a.turun:
			_, err = database.DB.Exec(`UPDATE users SET role = N'user', is_protected = 0, updated_at = SYSUTCDATETIME() WHERE id = @p1`, a.id)
			log.Println("[SUPERADMIN] akun diturunkan dari superadmin (tidak ada di .env atau bukan akun SSO):", a.email)
		}
		if err != nil {
			log.Println("[SUPERADMIN ERROR] gagal memperbarui akun", a.email+":", err)
		}
	}
}
