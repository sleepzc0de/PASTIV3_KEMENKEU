package routes

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"pasti-v3-backend/config"
	"pasti-v3-backend/database"
	"pasti-v3-backend/digitalisasi"
	"pasti-v3-backend/utils"
)

// Tes integrasi peran data lewat router asli (JWT, middleware, SQL Server sungguhan). Hanya berjalan bila PASTI_UJI_MSSQL_DSN diisi (database KHUSUS TES
// yang sudah dimigrasi). Data uji memakai kode UE1 09971 dan 09972, pengguna berawalan "uji-peran-", dan KLPD "UJIP", lalu dibersihkan sebelum dan sesudah.

const (
	ue1A = "09971"
	ue1B = "09972"
)

// kodeSatkerUji: kode satker lengkap 20 karakter = UE1 (5) + sisa kanwil (4) + satker (6) + anak (3) + "KP".
func kodeSatkerUji(ue1, sisaKanwil, satker string) string { return ue1 + sisaKanwil + satker + "000KP" }

var (
	satkerA1 = kodeSatkerUji(ue1A, "0199", "971001") // kanwil 099710199
	satkerA2 = kodeSatkerUji(ue1A, "0199", "971002") // kanwil 099710199
	satkerA3 = kodeSatkerUji(ue1A, "0299", "971003") // kanwil 099710299
	satkerB1 = kodeSatkerUji(ue1B, "0199", "972001") // kanwil 099720199
)

func bukaDBUji(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("PASTI_UJI_MSSQL_DSN")
	if dsn == "" {
		t.Skip("PASTI_UJI_MSSQL_DSN tidak diisi: tes integrasi SQL Server dilewati")
	}
	db, err := sql.Open("sqlserver", dsn)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Ping(); err != nil {
		t.Fatalf("tidak bisa terhubung ke database uji: %v", err)
	}
	lamaDB, lamaMgr, lamaCfg := database.DB, digitalisasi.Default, config.Cfg
	database.DB = db
	digitalisasi.Default = digitalisasi.NewManager(db, nil)
	config.Cfg = &config.Config{JWTSecret: "rahasia-uji-peran-0123456789abcdef", JWTAccessExpireMin: 60}
	t.Cleanup(func() {
		digitalisasi.Default.Cancel()
		database.DB, digitalisasi.Default, config.Cfg = lamaDB, lamaMgr, lamaCfg
		db.Close()
	})
	return db
}

func bersihkanUjiPeran(db *sql.DB) {
	db.Exec(`DELETE FROM users WHERE username LIKE N'uji-peran-%'`) // user_roles ikut terhapus
	db.Exec(`DELETE FROM employees WHERE sso_sub LIKE N'uji-peran-%'`)
	for _, tabel := range []string{"DIGITALISASI_SATKER", "DIGITALISASI_TANAH", "DIGITALISASI_GEDUNG_LAINNYA"} {
		db.Exec(`DELETE FROM ` + tabel + ` WHERE Kode_UE1 IN (N'09971', N'09972')`)
	}
	db.Exec(`DELETE FROM inaproc_paket_penyedia WHERE kd_klpd = N'UJIP'`)
}

func seedAsetUji(t *testing.T, db *sql.DB) {
	t.Helper()
	exec := func(q string, args ...interface{}) {
		t.Helper()
		if _, err := db.Exec(q, args...); err != nil {
			t.Fatalf("%v\n%s", err, q)
		}
	}
	for _, s := range []struct{ ue1, kode, nama string }{{ue1A, satkerA1, "SATKER A1"}, {ue1A, satkerA2, "SATKER A2"}, {ue1A, satkerA3, "SATKER A3"}, {ue1B, satkerB1, "SATKER B1"}} {
		exec(`INSERT INTO DIGITALISASI_SATKER (Kode_UE1, Kode_Satker, Jenis_Satker, Nama_Satker, Jumlah_KDJ, Jumlah_KDO) VALUES (@p1, @p2, N'INDUK SATKER', @p3, 1, 0)`, s.ue1, s.kode, s.nama)
	}
	// tanah: A1 x2, A2 x1, A3 x1, B1 x2 (semua bertitik koordinat di Indonesia)
	for _, s := range []struct{ ue1, kode string }{{ue1A, satkerA1}, {ue1A, satkerA1}, {ue1A, satkerA2}, {ue1A, satkerA3}, {ue1B, satkerB1}, {ue1B, satkerB1}} {
		exec(`INSERT INTO DIGITALISASI_TANAH (Kode_UE1, Kode_Satker, Nama_Satker, Uraian_tanah, Provinsi_tanah, Latitude, Longitude) VALUES (@p1, @p2, N'x', N'Tanah uji', N'DKI JAKARTA', -6.2, 106.8)`, s.ue1, s.kode)
	}
	exec(`INSERT INTO DIGITALISASI_GEDUNG_LAINNYA (Kode_UE1, Kode_Satker, Nama_Satker) VALUES (@p1, @p2, N'x')`, ue1A, satkerA1)
	// pengadaan KLPD UJIP 2025: 971001 x2, 971003 x1, 972001 x1, 999999 x1 (tidak ada di data aset)
	for i, k := range []string{"971001", "971001", "971003", "972001", "999999"} {
		exec(`INSERT INTO inaproc_paket_penyedia (row_key, kd_klpd, tahun_anggaran, kd_satker_str, nama_satker, pagu, status_delete_rup, status_aktif_rup) VALUES (@p1, N'UJIP', N'2025', @p2, @p3, 1000, 0, 1)`,
			fmt.Sprintf("UJIP-peran-%d", i), k, "SATKER "+k)
	}
}

type penggunaUji struct{ id, username, role, token string }

func buatPenggunaUji(t *testing.T, db *sql.DB, nama, role string) penggunaUji {
	t.Helper()
	p := penggunaUji{id: strings.ToUpper(uuid.New().String()), username: "uji-peran-" + nama, role: role}
	if _, err := db.Exec(`INSERT INTO users (id, username, email, full_name, role, is_active) VALUES (@p1, @p2, @p3, @p4, @p5, 1)`, p.id, p.username, p.username+"@example.test", "PENGGUNA "+nama, role); err != nil {
		t.Fatalf("buat pengguna %s: %v", nama, err)
	}
	tok, _, err := utils.GenerateAccessToken(p.id, p.username, role)
	if err != nil {
		t.Fatal(err)
	}
	p.token = tok
	return p
}

func panggil(t *testing.T, method, path, token, body string) (int, map[string]interface{}, []byte) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	SetupRoutes(r)
	req := httptest.NewRequest(method, "/api/v1"+path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var out map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return w.Code, out, w.Body.Bytes()
}

func data(t *testing.T, m map[string]interface{}) map[string]interface{} {
	t.Helper()
	d, ok := m["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("respons tanpa data: %v", m)
	}
	return d
}

func beriPeran(t *testing.T, admin, pengguna penggunaUji, role, kode string) int64 {
	t.Helper()
	badan, _ := json.Marshal(map[string]string{"role": role, "kode": kode})
	code, body, _ := panggil(t, "POST", "/users/"+pengguna.id+"/peran", admin.token, string(badan))
	if code != http.StatusCreated && code != http.StatusOK {
		t.Fatalf("beri peran %s %s: %d %v", role, kode, code, body)
	}
	return int64(data(t, body)["id"].(float64))
}

func pilihPeran(t *testing.T, p penggunaUji, id interface{}) {
	t.Helper()
	badan, _ := json.Marshal(map[string]interface{}{"id": id})
	if code, body, _ := panggil(t, "POST", "/auth/peran/aktif", p.token, string(badan)); code != 200 {
		t.Fatalf("pilih peran %v: %d %v", id, code, body)
	}
}

func totalTanah(t *testing.T, p penggunaUji) (int, []string) {
	t.Helper()
	code, body, _ := panggil(t, "GET", "/digitalisasi/data/tanah?per_page=100", p.token, "")
	if code != 200 {
		t.Fatalf("daftar tanah: %d %v", code, body)
	}
	d := data(t, body)
	var ue1 []string
	for _, x := range d["filter"].(map[string]interface{})["ue1"].([]interface{}) {
		if s := x.(string); strings.HasPrefix(s, "0997") {
			ue1 = append(ue1, s)
		}
	}
	sort.Strings(ue1)
	return int(d["total"].(float64)), ue1
}

// jumlah baris uji pada daftar tanah (pengguna tanpa pembatasan bisa melihat data lain di database uji, jadi dihitung dari baris milik tes ini saja).
func barisUji(t *testing.T, p penggunaUji) int {
	t.Helper()
	code, body, _ := panggil(t, "GET", "/digitalisasi/data/tanah?per_page=100&q=Tanah%20uji", p.token, "")
	if code != 200 {
		t.Fatalf("daftar tanah: %d %v", code, body)
	}
	return int(data(t, body)["total"].(float64))
}

func TestPeranDataDenganSQLServer(t *testing.T) {
	db := bukaDBUji(t)
	bersihkanUjiPeran(db)
	if os.Getenv("PASTI_UJI_BIARKAN_DATA") != "1" {
		t.Cleanup(func() { bersihkanUjiPeran(db) })
	}
	seedAsetUji(t, db)

	admin := buatPenggunaUji(t, db, "admin", "admin")
	budi := buatPenggunaUji(t, db, "budi", "user")
	sinta := buatPenggunaUji(t, db, "sinta", "user")

	t.Run("tanpa peran melihat semua data seperti sebelumnya", func(t *testing.T) {
		if n := barisUji(t, budi); n != 6 {
			t.Errorf("pengguna tanpa peran melihat %d baris uji, want 6", n)
		}
		code, body, _ := panggil(t, "GET", "/auth/me", budi.token, "")
		d := data(t, body)
		p := d["peran"].(map[string]interface{})
		if code != 200 || d["role"] != "user" || d["akun_role"] != "user" || p["peran"] != "" || p["cakupan"].(map[string]interface{})["tingkat"] != "semua" {
			t.Errorf("me tanpa peran: %d %v", code, d)
		}
	})

	t.Run("hanya admin yang boleh mengelola peran", func(t *testing.T) {
		if code, _, _ := panggil(t, "POST", "/users/"+sinta.id+"/peran", budi.token, `{"role":"ue1","kode":"09971"}`); code != 403 {
			t.Errorf("pengguna biasa memberi peran = %d, want 403", code)
		}
		if code, _, _ := panggil(t, "GET", "/users/"+sinta.id+"/peran", budi.token, ""); code != 403 {
			t.Errorf("pengguna biasa melihat peran orang lain = %d, want 403", code)
		}
		if code, _, _ := panggil(t, "POST", "/users/"+sinta.id+"/peran", "", `{"role":"ue1","kode":"09971"}`); code != 401 {
			t.Errorf("tanpa token = %d, want 401", code)
		}
	})

	t.Run("pemberian peran divalidasi dan tidak digandakan", func(t *testing.T) {
		for _, badan := range []string{`{"role":"ue1","kode":"0997"}`, `{"role":"kanwil","kode":"09971"}`, `{"role":"satker","kode":"97100x"}`, `{"role":"pengguna_barang","kode":"09971"}`,
			`{"role":"superadmin"}`, `{"role":"admin"}`, `{"role":"peretas","kode":"09971"}`, `{"role":""}`, `bukan json`} {
			if code, _, _ := panggil(t, "POST", "/users/"+sinta.id+"/peran", admin.token, badan); code != 400 {
				t.Errorf("badan %s = %d, want 400", badan, code)
			}
		}
		if code, _, _ := panggil(t, "POST", "/users/"+uuid.New().String()+"/peran", admin.token, `{"role":"ue1","kode":"09971"}`); code != 404 {
			t.Errorf("pengguna tidak ada = %d, want 404", code)
		}
		if code, _, _ := panggil(t, "POST", "/users/bukan-uuid/peran", admin.token, `{"role":"ue1","kode":"09971"}`); code != 400 {
			t.Errorf("id bukan uuid = %d, want 400", code)
		}
		id1 := beriPeran(t, admin, sinta, "pengguna_barang", "")
		id2 := beriPeran(t, admin, sinta, "pengguna_barang", "")
		if id1 != id2 {
			t.Errorf("peran yang sama persis digandakan: %d dan %d", id1, id2)
		}
		if code, _, _ := panggil(t, "DELETE", fmt.Sprintf("/users/%s/peran/%d", sinta.id, id1), admin.token, ""); code != 200 {
			t.Errorf("cabut = %d", code)
		}
		if code, _, _ := panggil(t, "DELETE", fmt.Sprintf("/users/%s/peran/%d", sinta.id, id1), admin.token, ""); code != 404 {
			t.Errorf("cabut ulang = %d, want 404", code)
		}
		if code, _, _ := panggil(t, "DELETE", fmt.Sprintf("/users/%s/peran/abc", sinta.id), admin.token, ""); code != 400 {
			t.Errorf("id peran bukan angka = %d, want 400", code)
		}
	})

	t.Run("saran peran dari kode satker pada data SSO", func(t *testing.T) {
		empID := strings.ToUpper(uuid.New().String())
		if _, err := db.Exec(`INSERT INTO employees (id, sso_sub, kode_satker) VALUES (@p1, N'uji-peran-emp', @p2)`, empID, satkerA1); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`UPDATE users SET employee_id = @p1 WHERE id = @p2`, empID, sinta.id); err != nil {
			t.Fatal(err)
		}
		code, body, _ := panggil(t, "GET", "/users/"+sinta.id+"/peran", admin.token, "")
		d := data(t, body)
		saran := d["saran"].([]interface{})
		if code != 200 || d["kode_satker_sso"] != satkerA1 || len(saran) != 3 {
			t.Fatalf("saran: %d %v", code, d)
		}
		kode := map[string]string{}
		for _, x := range saran {
			m := x.(map[string]interface{})
			kode[m["role"].(string)] = m["kode"].(string)
		}
		if kode["ue1"] != "09971" || kode["kanwil"] != "099710199" || kode["satker"] != "971001" {
			t.Errorf("saran = %v, want ue1 09971, kanwil 099710199, satker 971001", kode)
		}
	})

	t.Run("peran UE1 membatasi semua pembaca data", func(t *testing.T) {
		idUE1 := beriPeran(t, admin, budi, "ue1", ue1A)
		pilihPeran(t, budi, idUE1)

		code, body, _ := panggil(t, "GET", "/auth/me", budi.token, "")
		d := data(t, body)
		c := d["peran"].(map[string]interface{})["cakupan"].(map[string]interface{})
		if d["role"] != "user" || d["peran"].(map[string]interface{})["peran"] != "ue1" || c["tingkat"] != "ue1" || c["kode"] != ue1A {
			t.Errorf("me: %v", d)
		}

		// daftar + pilihan filter
		total, ue1 := totalTanah(t, budi)
		if total != 4 || len(ue1) != 1 || ue1[0] != ue1A {
			t.Errorf("daftar tanah UE1 A: total=%d filter ue1=%v, want 4 dan [09971] saja", total, ue1)
		}
		// detail baris UE1 lain tidak boleh terbaca walau id-nya ditebak
		var idB int64
		db.QueryRow(`SELECT TOP (1) id FROM DIGITALISASI_TANAH WHERE Kode_UE1 = @p1`, ue1B).Scan(&idB)
		var idA int64
		db.QueryRow(`SELECT TOP (1) id FROM DIGITALISASI_TANAH WHERE Kode_UE1 = @p1`, ue1A).Scan(&idA)
		if code, _, _ := panggil(t, "GET", fmt.Sprintf("/digitalisasi/data/tanah/%d", idB), budi.token, ""); code != 404 {
			t.Errorf("detail baris UE1 lain = %d, want 404", code)
		}
		if code, _, _ := panggil(t, "GET", fmt.Sprintf("/digitalisasi/data/tanah/%d", idA), budi.token, ""); code != 200 {
			t.Errorf("detail baris sendiri = %d, want 200", code)
		}
		// filter ue1 ke UE1 lain tidak membuka apa pun
		if code, body, _ := panggil(t, "GET", "/digitalisasi/data/tanah?ue1="+ue1B, budi.token, ""); code != 200 || data(t, body)["total"] != float64(0) {
			t.Errorf("filter UE1 lain: %d total=%v, want 0", code, data(t, body)["total"])
		}
		// peta
		_, body, _ = panggil(t, "GET", "/digitalisasi/peta?dataset=tanah", budi.token, "")
		if set := data(t, body)["datasets"].([]interface{})[0].(map[string]interface{}); set["total"] != float64(4) {
			t.Errorf("peta total = %v, want 4", set["total"])
		}
		// ekspor CSV hanya baris dalam cakupan
		code, _, mentah := panggil(t, "GET", "/digitalisasi/ekspor/tanah?format=csv", budi.token, "")
		baris := strings.Split(strings.TrimRight(strings.TrimPrefix(string(mentah), "\xEF\xBB\xBF"), "\r\n"), "\r\n")
		if code != 200 || len(baris) != 5 || strings.Contains(string(mentah), ue1B) {
			t.Errorf("ekspor: %d, %d baris (want 5 = judul + 4), memuat UE1 lain: %v", code, len(baris), strings.Contains(string(mentah), ue1B))
		}
		// ringkasan
		_, body, _ = panggil(t, "GET", "/digitalisasi/ringkasan", budi.token, "")
		rg := data(t, body)
		var tanah map[string]interface{}
		for _, a := range rg["aset"].([]interface{}) {
			if m := a.(map[string]interface{}); m["key"] == "tanah" {
				tanah = m
			}
		}
		if tanah["jumlah"] != float64(4) || rg["satker"].(map[string]interface{})["total"] != float64(3) {
			t.Errorf("ringkasan: tanah=%v satker=%v, want 4 tanah dan 3 satker", tanah["jumlah"], rg["satker"])
		}
		ue1List := rg["ue1"].([]interface{})
		if len(ue1List) != 1 || ue1List[0].(map[string]interface{})["kode"] != ue1A {
			t.Errorf("ringkasan per UE1 = %v, want hanya %s", ue1List, ue1A)
		}
		// data Pengadaan lengkap tidak tersedia bagi peran terbatas
		for _, path := range []string{"/inaproc/analitik", "/inaproc/penarikan", "/inaproc/data/tender/pengumuman", "/inaproc/ekspor/tender/pengumuman"} {
			if code, _, _ := panggil(t, "GET", path, budi.token, ""); code != 403 {
				t.Errorf("GET %s oleh peran UE1 = %d, want 403", path, code)
			}
		}
		// keterhubungan: aset dalam cakupan, pengadaan hanya untuk satker yang dikenal di sana
		_, body, _ = panggil(t, "GET", "/satker/keterhubungan?kode_klpd=UJIP&tahun=2025", budi.token, "")
		kodeSatker := []string{}
		for _, x := range data(t, body)["satker"].([]interface{}) {
			kodeSatker = append(kodeSatker, x.(map[string]interface{})["kode"].(string))
		}
		sort.Strings(kodeSatker)
		if strings.Join(kodeSatker, ",") != "971001,971002,971003" {
			t.Errorf("keterhubungan UE1 A = %v, want 971001,971002,971003 (tanpa 972001 dan tanpa 999999)", kodeSatker)
		}
	})

	t.Run("peran Kanwil dan Satker lebih sempit lagi, dan berpindah peran berlaku langsung", func(t *testing.T) {
		idKanwil := beriPeran(t, admin, budi, "kanwil", ue1A+"0199")
		idSatker := beriPeran(t, admin, budi, "satker", "971001")

		pilihPeran(t, budi, idKanwil)
		if total, _ := totalTanah(t, budi); total != 3 { // A1 x2 + A2 x1; A3 ada di kanwil lain
			t.Errorf("tanah kanwil 099710199 = %d, want 3", total)
		}
		pilihPeran(t, budi, idSatker)
		if total, _ := totalTanah(t, budi); total != 2 {
			t.Errorf("tanah satker 971001 = %d, want 2", total)
		}
		_, body, _ := panggil(t, "GET", "/digitalisasi/ringkasan", budi.token, "")
		if s := data(t, body)["satker"].(map[string]interface{}); s["total"] != float64(1) {
			t.Errorf("ringkasan satker = %v, want 1 satker", s)
		}
		_, body, _ = panggil(t, "GET", "/satker/keterhubungan?kode_klpd=UJIP&tahun=2025", budi.token, "")
		daftar := data(t, body)["satker"].([]interface{})
		if len(daftar) != 1 || daftar[0].(map[string]interface{})["kode"] != "971001" {
			t.Fatalf("keterhubungan satker = %v, want hanya 971001", daftar)
		}
		if rup := daftar[0].(map[string]interface{})["pengadaan"].(map[string]interface{})["rup_paket"]; rup != float64(2) {
			t.Errorf("RUP satker 971001 = %v, want 2", rup)
		}

		// peran yang bukan milik sendiri ditolak dan tidak mengubah peran aktif
		var idLain int64
		db.QueryRow(`SELECT TOP (1) id FROM user_roles WHERE user_id <> @p1`, budi.id).Scan(&idLain)
		if idLain != 0 {
			if code, _, _ := panggil(t, "POST", "/auth/peran/aktif", budi.token, fmt.Sprintf(`{"id":%d}`, idLain)); code != 404 {
				t.Errorf("memilih peran milik orang lain = %d, want 404", code)
			}
		}
		if total, _ := totalTanah(t, budi); total != 2 {
			t.Errorf("peran aktif berubah setelah pilihan ditolak: tanah = %d, want 2", total)
		}

		// daftar peran sendiri
		code, body, _ := panggil(t, "GET", "/auth/peran", budi.token, "")
		d := data(t, body)
		if code != 200 || len(d["tersedia"].([]interface{})) != 3 || d["peran"] != "satker" || d["peran_id"] != float64(idSatker) || d["bawaan"] != false {
			t.Errorf("/auth/peran: %d %v", code, d)
		}

		// mencabut peran aktif: kembali ke peran pertama yang dipegang (UE1) karena pengguna biasa tidak punya peran bawaan
		if code, _, _ := panggil(t, "DELETE", fmt.Sprintf("/users/%s/peran/%d", budi.id, idSatker), admin.token, ""); code != 200 {
			t.Fatalf("cabut peran aktif = %d", code)
		}
		if total, _ := totalTanah(t, budi); total != 4 {
			t.Errorf("setelah peran aktif dicabut, tanah = %d, want 4 (peran pertama: UE1)", total)
		}
	})

	t.Run("admin yang bertindak sebagai peran data kehilangan hak administrasi sementara", func(t *testing.T) {
		if code, _, _ := panggil(t, "GET", "/users", admin.token, ""); code != 200 {
			t.Fatalf("admin melihat pengguna = %d", code)
		}
		idSatker := beriPeran(t, admin, admin, "satker", "972001")
		pilihPeran(t, admin, idSatker)

		if code, _, _ := panggil(t, "GET", "/users", admin.token, ""); code != 403 {
			t.Errorf("admin yang bertindak sebagai Satker mengelola pengguna = %d, want 403", code)
		}
		if total, _ := totalTanah(t, admin); total != 2 { // B1 x2
			t.Errorf("admin sebagai Satker 972001 melihat %d tanah, want 2", total)
		}
		code, body, _ := panggil(t, "GET", "/auth/me", admin.token, "")
		if d := data(t, body); code != 200 || d["role"] != "user" || d["akun_role"] != "admin" {
			t.Errorf("me admin sebagai Satker: %v", d)
		}
		// kembali ke peran bawaan (id null)
		pilihPeran(t, admin, nil)
		if code, _, _ := panggil(t, "GET", "/users", admin.token, ""); code != 200 {
			t.Errorf("admin setelah kembali ke peran bawaan = %d, want 200", code)
		}
		if n := barisUji(t, admin); n != 6 {
			t.Errorf("admin peran bawaan melihat %d baris uji, want 6", n)
		}
		if code, _, _ := panggil(t, "GET", "/inaproc/analitik", admin.token, ""); code != 200 {
			t.Errorf("admin peran bawaan membuka analitik Pengadaan = %d, want 200", code)
		}
	})

	t.Run("pengguna barang melihat semua data tetapi bukan admin", func(t *testing.T) {
		pb := buatPenggunaUji(t, db, "barang", "user")
		beriPeran(t, admin, pb, "pengguna_barang", "")
		if n := barisUji(t, pb); n != 6 {
			t.Errorf("pengguna barang melihat %d baris uji, want 6", n)
		}
		if code, _, _ := panggil(t, "GET", "/inaproc/analitik", pb.token, ""); code != 200 {
			t.Errorf("pengguna barang membuka analitik Pengadaan = %d, want 200", code)
		}
		if code, _, _ := panggil(t, "GET", "/users", pb.token, ""); code != 403 {
			t.Errorf("pengguna barang mengelola pengguna = %d, want 403", code)
		}
	})

	t.Run("pembatasan wajib: pengguna tanpa peran tidak melihat data apa pun", func(t *testing.T) {
		tanpa := buatPenggunaUji(t, db, "tanpa", "user")
		config.Cfg.PeranDataWajib = true
		defer func() { config.Cfg.PeranDataWajib = false }()

		if n := barisUji(t, tanpa); n != 0 {
			t.Errorf("tanpa peran + wajib melihat %d baris, want 0", n)
		}
		if code, _, _ := panggil(t, "GET", "/inaproc/analitik", tanpa.token, ""); code != 403 {
			t.Errorf("tanpa peran + wajib membuka analitik = %d, want 403", code)
		}
		code, body, _ := panggil(t, "GET", "/digitalisasi/ringkasan", tanpa.token, "")
		if d := data(t, body); code != 200 || d["tersedia"] != false {
			t.Errorf("ringkasan tanpa peran + wajib: %d %v", code, d)
		}
		if _, body, _ := panggil(t, "GET", "/auth/me", tanpa.token, ""); data(t, body)["peran"].(map[string]interface{})["wajib"] != true {
			t.Error("me harus memberi tahu bahwa pembatasan diwajibkan")
		}
		// admin tidak terpengaruh, begitu pula pengguna yang sudah punya peran
		if n := barisUji(t, admin); n != 6 {
			t.Errorf("admin saat wajib melihat %d baris, want 6", n)
		}
		// pembatasan hanya pada data: profil sendiri tetap bisa dibuka
		if code, _, _ := panggil(t, "GET", "/auth/me", tanpa.token, ""); code != 200 {
			t.Errorf("me = %d", code)
		}
	})

	t.Run("akun dinonaktifkan tetap ditolak walau punya peran", func(t *testing.T) {
		db.Exec(`UPDATE users SET is_active = 0 WHERE id = @p1`, budi.id)
		if code, _, _ := panggil(t, "GET", "/auth/peran", budi.token, ""); code != 401 {
			t.Errorf("akun nonaktif = %d, want 401", code)
		}
	})
}
