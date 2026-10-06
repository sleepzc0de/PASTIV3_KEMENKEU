package handlers

import (
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"pasti-v3-backend/database"
)

// Penyimpanan data SSO Kemenkeu secara lengkap (migrasi 051). Kolom employees (migrasi 002) hanya memuat sebagian klaim; selebihnya ada di
// raw_claims yang ditimpa tiap login. Di sini SETIAP klaim dari userinfo dan id_token juga disimpan satu baris per kunci (employee_claims),
// dan salinan lengkapnya dicatat lagi (employee_claim_history) setiap kali isinya berubah.

// klaimBerubahTiapLogin: klaim id_token yang berganti nilai di setiap login walau pegawainya tidak berubah. Dikeluarkan dari perhitungan
// hash supaya riwayat hanya bertambah ketika data pegawai (jabatan, satker, dst.) benar-benar berbeda.
var klaimBerubahTiapLogin = map[string]bool{
	"iat": true, "exp": true, "nbf": true, "auth_time": true, "nonce": true, "jti": true, "sid": true,
	"at_hash": true, "c_hash": true, "s_hash": true, "amr": true,
}

// klaimTeks membaca satu klaim sebagai teks: string apa adanya, angka (json.Number) sebagai digit aslinya, selain itu kosong.
func klaimTeks(klaim map[string]interface{}, kunci string) string {
	switch v := klaim[kunci].(type) {
	case string:
		return v
	case json.Number:
		return v.String()
	}
	return ""
}

// bacaKlaimIDToken membaca isi (payload) id_token tanpa memeriksa tanda tangannya. Itu sah di sini: id_token diterima langsung dari token
// endpoint SSO lewat TLS (OpenID Connect Core 3.1.3.7), bukan dari pengguna, dan hasilnya hanya disimpan sebagai data, tidak dipakai untuk
// menentukan siapa yang login (itu tetap dari userinfo). Mengembalikan nil bila token kosong atau bukan JWT.
func bacaKlaimIDToken(idToken string) map[string]interface{} {
	bagian := strings.Split(strings.TrimSpace(idToken), ".")
	if len(bagian) != 3 {
		return nil
	}
	mentah, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(bagian[1], "="))
	if err != nil {
		return nil
	}
	var klaim map[string]interface{}
	dec := json.NewDecoder(strings.NewReader(string(mentah)))
	dec.UseNumber() // angka besar (mis. NIP berbentuk angka) tidak berubah menjadi notasi ilmiah
	if err := dec.Decode(&klaim); err != nil {
		return nil
	}
	return klaim
}

// hashKlaim: SHA-256 dari klaim userinfo dan id_token (tanpa klaim yang berganti tiap login). json.Marshal mengurutkan kunci peta, jadi hasilnya
// tetap untuk isi yang sama.
func hashKlaim(userinfo, idToken map[string]interface{}) string {
	bersih := func(m map[string]interface{}) map[string]interface{} {
		out := make(map[string]interface{}, len(m))
		for k, v := range m {
			if !klaimBerubahTiapLogin[k] {
				out[k] = v
			}
		}
		return out
	}
	b, _ := json.Marshal(map[string]interface{}{"userinfo": bersih(userinfo), "id_token": bersih(idToken)})
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

// simpanDataSSO menyimpan data SSO lengkap seorang pegawai dalam satu transaksi: kolom tambahan employees, seluruh klaim (employee_claims),
// dan baris riwayat bila isinya berbeda dari catatan terakhir. idToken boleh nil (id_token tidak terbaca). Dipanggil setelah upsertEmployee.
func simpanDataSSO(employeeID string, userinfo, idToken map[string]interface{}, scope string) error {
	if employeeID == "" {
		return errors.New("employeeID kosong")
	}
	userinfoJSON, _ := json.Marshal(userinfo)
	var idTokenJSON sql.NullString
	if idToken != nil {
		b, _ := json.Marshal(idToken)
		idTokenJSON = sql.NullString{String: string(b), Valid: true}
	}
	hash := hashKlaim(userinfo, idToken)

	tx, err := database.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() // tidak berpengaruh setelah Commit

	if _, err := tx.Exec(`UPDATE employees SET
			raw_id_token_claims = @p2, claims_hash = @p3,
			first_login_at = COALESCE(first_login_at, SYSUTCDATETIME()), last_login_at = SYSUTCDATETIME(), login_count = login_count + 1
		WHERE id = @p1`, employeeID, idTokenJSON, hash); err != nil {
		return fmt.Errorf("perbarui employees: %w", err)
	}

	for _, sumber := range []struct {
		nama  string
		klaim map[string]interface{}
	}{{"userinfo", userinfo}, {"id_token", idToken}} {
		if err := simpanKlaimSumber(tx, employeeID, sumber.nama, sumber.klaim); err != nil {
			return err
		}
	}

	var hashTerakhir sql.NullString
	err = tx.QueryRow(`SELECT TOP (1) claims_hash FROM employee_claim_history WHERE employee_id = @p1 ORDER BY id DESC`, employeeID).Scan(&hashTerakhir)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("baca riwayat klaim: %w", err)
	}
	if errors.Is(err, sql.ErrNoRows) || !hashTerakhir.Valid || hashTerakhir.String != hash {
		var scopeNull sql.NullString
		if s := strings.TrimSpace(scope); s != "" {
			scopeNull = sql.NullString{String: truncateString(s, 500), Valid: true}
		}
		if _, err := tx.Exec(`INSERT INTO employee_claim_history (employee_id, claims_hash, userinfo_claims, id_token_claims, scope)
			VALUES (@p1, @p2, @p3, @p4, @p5)`, employeeID, hash, string(userinfoJSON), idTokenJSON, scopeNull); err != nil {
			return fmt.Errorf("tulis riwayat klaim: %w", err)
		}
	}
	return tx.Commit()
}

// simpanKlaimSumber mengganti seluruh klaim satu sumber dengan isi terbaru: klaim yang ada diperbarui, yang baru ditambah, yang sudah tidak
// dikirim dihapus. Kunci dan nilainya dibaca SQL Server langsung dari JSON (OPENJSON), jadi sama persis dengan penyalinan awal di migrasi 051.
// Sumber nil (mis. id_token tidak terbaca) dibiarkan: data lama tetap, bukan dikosongkan.
func simpanKlaimSumber(tx *sql.Tx, employeeID, sumber string, klaim map[string]interface{}) error {
	if klaim == nil {
		return nil
	}
	j, err := json.Marshal(klaim)
	if err != nil {
		return fmt.Errorf("serialisasi klaim %s: %w", sumber, err)
	}
	if _, err := tx.Exec(`
		DELETE FROM employee_claims WHERE employee_id = @p1 AND sumber = @p2;
		INSERT INTO employee_claims (employee_id, sumber, claim_key, claim_value, tipe)
		SELECT @p1, @p2, j.[key], j.value,
		       CASE j.type WHEN 0 THEN N'null' WHEN 1 THEN N'string' WHEN 2 THEN N'number' WHEN 3 THEN N'bool' ELSE N'json' END
		FROM OPENJSON(@p3) j
		WHERE LEN(j.[key]) <= 150`, employeeID, sumber, string(j)); err != nil {
		return fmt.Errorf("simpan klaim %s: %w", sumber, err)
	}
	return nil
}
