package handlers

import (
	"database/sql"
	"encoding/json"
	"os"
	"regexp"
	"strings"
	"testing"
)

// Tes integrasi dengan SQL Server sungguhan untuk penyimpanan data SSO lengkap (migrasi 051). Dilewati bila PASTI_UJI_MSSQL_DSN kosong.
// Pegawai uji memakai sso_sub berawalan "uji-sso-" dan dibersihkan sebelum serta sesudah tes.

func bersihkanPegawaiUji(t *testing.T, db *sql.DB) {
	t.Helper()
	// employee_claims dan employee_claim_history ikut terhapus (ON DELETE CASCADE).
	if _, err := db.Exec(`DELETE FROM employees WHERE sso_sub LIKE N'uji-sso-%'`); err != nil {
		t.Fatalf("bersihkan pegawai uji: %v", err)
	}
}

func pegawaiUji(t *testing.T, db *sql.DB, sub string, claims map[string]interface{}) string {
	t.Helper()
	id, err := upsertEmployee(claims, sub, klaimTeks(claims, "nip"), "", klaimTeks(claims, "email"))
	if err != nil {
		t.Fatalf("upsertEmployee: %v", err)
	}
	return id
}

func hitungSSO(t *testing.T, db *sql.DB, q string, args ...interface{}) int {
	t.Helper()
	var n int
	if err := db.QueryRow(q, args...).Scan(&n); err != nil {
		t.Fatalf("%s: %v", q, err)
	}
	return n
}

func jsonKlaim(t *testing.T, s string) map[string]interface{} {
	t.Helper()
	var m map[string]interface{}
	dec := json.NewDecoder(strings.NewReader(s))
	dec.UseNumber()
	if err := dec.Decode(&m); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestSimpanDataSSODenganSQLServer(t *testing.T) {
	db := dbIntegrasi(t)
	bersihkanPegawaiUji(t, db)
	if !biarkanDataUji() {
		t.Cleanup(func() { bersihkanPegawaiUji(t, db) })
	}

	ui1 := jsonKlaim(t, `{"sub":"uji-sso-1","name":"PEGAWAI UJI","email":"uji1@example.test","nip":198505152010011001,"jabatan":"Pranata Komputer",
		"kode_satker":"015040199119091000KP","satker":"KPP Uji","roles":["a","b"],"aktif":true,"atasan":null,"detail":{"unit":"X","eselon":3}}`)
	id1 := jsonKlaim(t, `{"sub":"uji-sso-1","iss":"https://sso.example.test","iat":1700000000,"exp":1700003600,"nonce":"n1","unit_kerja":"Unit A"}`)
	id := pegawaiUji(t, db, "uji-sso-1", ui1)

	// ---- login pertama
	if err := simpanDataSSO(id, ui1, id1, "openid profile"); err != nil {
		t.Fatal(err)
	}
	if got := hitungSSO(t, db, `SELECT COUNT(*) FROM employee_claims WHERE employee_id = @p1 AND sumber = N'userinfo'`, id); got != len(ui1) {
		t.Errorf("klaim userinfo tersimpan %d, want %d (satu baris per kunci)", got, len(ui1))
	}
	if got := hitungSSO(t, db, `SELECT COUNT(*) FROM employee_claims WHERE employee_id = @p1 AND sumber = N'id_token'`, id); got != len(id1) {
		t.Errorf("klaim id_token tersimpan %d, want %d", got, len(id1))
	}
	type klaim struct {
		nilai sql.NullString
		tipe  string
	}
	baca := func(sumber, kunci string) klaim {
		var k klaim
		if err := db.QueryRow(`SELECT claim_value, tipe FROM employee_claims WHERE employee_id = @p1 AND sumber = @p2 AND claim_key = @p3`, id, sumber, kunci).Scan(&k.nilai, &k.tipe); err != nil {
			t.Fatalf("baca klaim %s/%s: %v", sumber, kunci, err)
		}
		return k
	}
	for _, c := range []struct {
		kunci, nilai, tipe string
		valid              bool
	}{
		{"jabatan", "Pranata Komputer", "string", true},
		{"nip", "198505152010011001", "number", true}, // digit asli tidak berubah
		{"aktif", "true", "bool", true},
		{"atasan", "", "null", false},
		{"roles", `["a","b"]`, "json", true},
		{"detail", `{"eselon":3,"unit":"X"}`, "json", true},
	} {
		got := baca("userinfo", c.kunci)
		if got.tipe != c.tipe || got.nilai.Valid != c.valid || (c.valid && !jsonSama(got.nilai.String, c.nilai)) {
			t.Errorf("klaim %s = (%q valid=%v tipe=%s), want (%q valid=%v tipe=%s)", c.kunci, got.nilai.String, got.nilai.Valid, got.tipe, c.nilai, c.valid, c.tipe)
		}
	}
	if got := baca("id_token", "unit_kerja"); got.nilai.String != "Unit A" {
		t.Errorf("id_token.unit_kerja = %q", got.nilai.String)
	}
	var idTokenMentah sql.NullString
	var loginCount int
	var pertama, terakhir sql.NullTime
	if err := db.QueryRow(`SELECT raw_id_token_claims, login_count, first_login_at, last_login_at FROM employees WHERE id = @p1`, id).Scan(&idTokenMentah, &loginCount, &pertama, &terakhir); err != nil {
		t.Fatal(err)
	}
	if !idTokenMentah.Valid || !strings.Contains(idTokenMentah.String, "Unit A") {
		t.Errorf("raw_id_token_claims = %q", idTokenMentah.String)
	}
	if loginCount != 1 || !pertama.Valid || !terakhir.Valid {
		t.Errorf("login_count=%d pertama=%v terakhir=%v, want 1 dan keduanya terisi", loginCount, pertama.Valid, terakhir.Valid)
	}
	if got := hitungSSO(t, db, `SELECT COUNT(*) FROM employee_claim_history WHERE employee_id = @p1`, id); got != 1 {
		t.Errorf("riwayat setelah login pertama = %d, want 1", got)
	}

	// ---- login kedua: data pegawai sama, hanya klaim id_token yang berganti tiap login (iat/exp/nonce) -> tidak ada riwayat baru
	id2 := jsonKlaim(t, `{"sub":"uji-sso-1","iss":"https://sso.example.test","iat":1700009999,"exp":1700013599,"nonce":"n2","unit_kerja":"Unit A"}`)
	if err := simpanDataSSO(id, ui1, id2, "openid profile"); err != nil {
		t.Fatal(err)
	}
	if got := hitungSSO(t, db, `SELECT COUNT(*) FROM employee_claim_history WHERE employee_id = @p1`, id); got != 1 {
		t.Errorf("riwayat setelah login kedua tanpa perubahan data = %d, want tetap 1", got)
	}
	if got := hitungSSO(t, db, `SELECT login_count FROM employees WHERE id = @p1`, id); got != 2 {
		t.Errorf("login_count = %d, want 2", got)
	}
	if got := baca("id_token", "nonce"); got.nilai.String != "n2" {
		t.Errorf("klaim id_token harus diperbarui ke login terbaru, nonce = %q", got.nilai.String)
	}

	// ---- login ketiga: pegawai dimutasi (jabatan dan satker berubah, satu klaim hilang, satu klaim baru)
	ui3 := jsonKlaim(t, `{"sub":"uji-sso-1","name":"PEGAWAI UJI","email":"uji1@example.test","nip":198505152010011001,"jabatan":"Kepala Seksi",
		"kode_satker":"015090199200200000KP","satker":"KPKNL Uji","roles":["a"],"aktif":true,"atasan":null,"golongan":"III/d"}`)
	if err := simpanDataSSO(id, ui3, id2, "openid profile"); err != nil {
		t.Fatal(err)
	}
	if got := hitungSSO(t, db, `SELECT COUNT(*) FROM employee_claim_history WHERE employee_id = @p1`, id); got != 2 {
		t.Errorf("riwayat setelah mutasi = %d, want 2", got)
	}
	if got := baca("userinfo", "jabatan"); got.nilai.String != "Kepala Seksi" {
		t.Errorf("jabatan = %q, want Kepala Seksi", got.nilai.String)
	}
	if got := baca("userinfo", "golongan"); got.nilai.String != "III/d" {
		t.Errorf("klaim baru golongan = %q", got.nilai.String)
	}
	if got := hitungSSO(t, db, `SELECT COUNT(*) FROM employee_claims WHERE employee_id = @p1 AND sumber = N'userinfo' AND claim_key = N'detail'`, id); got != 0 {
		t.Error("klaim yang tidak dikirim lagi (detail) harus hilang dari employee_claims")
	}
	// riwayat menyimpan keadaan LAMA dan BARU utuh (tidak saling menimpa)
	var lama, baru string
	rows, err := db.Query(`SELECT userinfo_claims FROM employee_claim_history WHERE employee_id = @p1 ORDER BY id`, id)
	if err != nil {
		t.Fatal(err)
	}
	var semua []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			t.Fatal(err)
		}
		semua = append(semua, s)
	}
	rows.Close()
	if len(semua) == 2 {
		lama, baru = semua[0], semua[1]
	}
	if !strings.Contains(lama, "Pranata Komputer") || !strings.Contains(baru, "Kepala Seksi") || !strings.Contains(lama, "119091") {
		t.Errorf("riwayat harus memuat keadaan lama dan baru:\nlama=%s\nbaru=%s", lama, baru)
	}

	// ---- id_token tidak terbaca (nil): klaim id_token lama dibiarkan, bukan dikosongkan
	if err := simpanDataSSO(id, ui3, nil, ""); err != nil {
		t.Fatal(err)
	}
	if got := hitungSSO(t, db, `SELECT COUNT(*) FROM employee_claims WHERE employee_id = @p1 AND sumber = N'id_token'`, id); got != len(id2) {
		t.Errorf("klaim id_token setelah login tanpa id_token = %d, want tetap %d", got, len(id2))
	}
}

// Penyalinan awal di migrasi 051 (klaim dari raw_claims yang sudah ada sebelum tabel baru): dijalankan ulang dari berkas migrasinya sendiri.
func TestMigrasi051MenyalinRawClaimsYangAda(t *testing.T) {
	db := dbIntegrasi(t)
	bersihkanPegawaiUji(t, db)
	if !biarkanDataUji() {
		t.Cleanup(func() { bersihkanPegawaiUji(t, db) })
	}
	raw := `{"sub":"uji-sso-lama","name":"PEGAWAI LAMA","nip":"198001012005011001","aktif":false,"roles":["x"],"catatan":null,"golongan":4}`
	if _, err := db.Exec(`INSERT INTO employees (id, sso_sub, name, raw_claims) VALUES (NEWID(), N'uji-sso-lama', N'PEGAWAI LAMA', @p1)`, raw); err != nil {
		t.Fatal(err)
	}
	var id string
	if err := db.QueryRow(`SELECT CONVERT(NVARCHAR(36), id) FROM employees WHERE sso_sub = N'uji-sso-lama'`).Scan(&id); err != nil {
		t.Fatal(err)
	}

	skrip, err := os.ReadFile("../migrations/051_sso_data_lengkap.sql")
	if err != nil {
		t.Fatal(err)
	}
	batches := regexp.MustCompile(`(?im)^[ \t]*GO[ \t]*\r?$`).Split(string(skrip), -1)
	salin := batches[len(batches)-1] // batch terakhir = penyalinan awal
	if !strings.Contains(salin, "INSERT INTO employee_claims") {
		t.Fatalf("batch terakhir migrasi bukan penyalinan awal:\n%s", salin)
	}
	if _, err := db.Exec(salin); err != nil {
		t.Fatalf("jalankan penyalinan awal: %v", err)
	}

	got := map[string]string{}
	rows, err := db.Query(`SELECT claim_key, ISNULL(claim_value, N'<NULL>') + N'|' + tipe FROM employee_claims WHERE employee_id = @p1 AND sumber = N'userinfo'`, id)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			t.Fatal(err)
		}
		got[k] = v
	}
	rows.Close()
	want := map[string]string{"sub": "uji-sso-lama|string", "name": "PEGAWAI LAMA|string", "nip": "198001012005011001|string", "aktif": "false|bool",
		"roles": `["x"]|json`, "catatan": "<NULL>|null", "golongan": "4|number"}
	for k, w := range want {
		if got[k] != w {
			t.Errorf("klaim %s = %q, want %q", k, got[k], w)
		}
	}
	if len(got) != len(want) {
		t.Errorf("jumlah klaim = %d, want %d", len(got), len(want))
	}
	if n := hitungSSO(t, db, `SELECT COUNT(*) FROM employee_claim_history WHERE employee_id = @p1 AND claims_hash IS NULL AND userinfo_claims = @p2`, id, raw); n != 1 {
		t.Errorf("salinan awal riwayat = %d, want 1", n)
	}
	// dijalankan ulang tidak menggandakan
	if _, err := db.Exec(salin); err != nil {
		t.Fatal(err)
	}
	if n := hitungSSO(t, db, `SELECT COUNT(*) FROM employee_claims WHERE employee_id = @p1`, id); n != len(want) {
		t.Errorf("setelah dijalankan ulang: %d klaim, want %d", n, len(want))
	}
}

// jsonSama membandingkan dua teks JSON tanpa memperhatikan urutan kunci (OPENJSON mengembalikan teks asli yang kita serialisasi lebih dulu).
func jsonSama(a, b string) bool {
	var x, y interface{}
	if json.Unmarshal([]byte(a), &x) != nil || json.Unmarshal([]byte(b), &y) != nil {
		return a == b
	}
	ja, _ := json.Marshal(x)
	jb, _ := json.Marshal(y)
	return string(ja) == string(jb)
}
