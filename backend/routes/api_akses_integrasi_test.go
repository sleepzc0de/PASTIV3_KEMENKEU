package routes

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"pasti-v3-backend/apiakses"
	"pasti-v3-backend/audit"
	"pasti-v3-backend/handlers"
)

// Tes integrasi API Akses Data dari ujung ke ujung (router asli, SQL Server sungguhan; hanya bila PASTI_UJI_MSSQL_DSN diisi): tidak ada pendaftaran publik, klien didaftarkan
// superadmin (langsung disetujui), penarikan data dengan kunci, scope, batas laju yang diubah tanpa deploy ulang, rotasi dan pencabutan kunci, pemantauan, dan Log Audit.

type ujiAPI struct {
	t *testing.T
	r *gin.Engine
	l *apiakses.Layanan
}

func (u *ujiAPI) panggil(method, path string, hdr map[string]string, body string) (int, map[string]interface{}, http.Header, []byte) {
	u.t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "203.0.113.20:4000"
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	u.r.ServeHTTP(w, req)
	var out map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return w.Code, out, w.Header(), w.Body.Bytes()
}

func bearer(tok string) map[string]string { return map[string]string{"Authorization": "Bearer " + tok} }

func bersihkanApiAksesUji(db *sql.DB) {
	rows, err := db.Query(`SELECT id FROM api_akses_klien WHERE nama_aplikasi LIKE N'UJI-E2E%'`)
	if err == nil {
		var ids []int64
		for rows.Next() {
			var id int64
			rows.Scan(&id)
			ids = append(ids, id)
		}
		rows.Close()
		for _, id := range ids {
			db.Exec(`DELETE FROM api_akses_log WHERE klien_id = @p1`, id)
			db.Exec(`DELETE FROM api_akses_agregat WHERE klien_id = @p1`, id)
			db.Exec(`DELETE FROM api_akses_kuota WHERE klien_id = @p1`, id)
			db.Exec(`DELETE FROM api_akses_kunci WHERE klien_id = @p1`, id)
			db.Exec(`DELETE FROM api_akses_klien_dataset WHERE klien_id = @p1`, id)
			db.Exec(`DELETE FROM api_akses_klien WHERE id = @p1`, id)
		}
	}
	db.Exec(`DELETE FROM api_akses_log WHERE klien_id IS NULL AND ip IN (N'203.0.113.20', N'198.51.100.77')`)
	db.Exec(`DELETE FROM api_akses_agregat WHERE klien_id = 0`)
	db.Exec(`DELETE FROM audit_log WHERE kategori = N'api_akses' OR username LIKE N'uji-peran-%'`)
}

// bodyKlienBaru: isian pendaftaran klien oleh superadmin (POST /api-akses/klien); ubah menyesuaikan isiannya.
func bodyKlienBaru(nama string, ubah func(map[string]interface{})) string {
	m := map[string]interface{}{
		"nama_aplikasi": nama, "instansi": "Direktorat Uji", "unit": "Bagian TI", "pic_nama": "Budi Penguji", "pic_jabatan": "Kepala", "pic_email": "budi.penguji@uji.go.id",
		"pic_telepon": "021-5551234", "tujuan": "Menampilkan sebaran aset tanah pada dasbor pengujian internal.", "dataset_diberikan": []string{"aset.tanah"},
		"ue1_diizinkan": "09971", "limit_permintaan": 100, "catatan": "didaftarkan uji", "label_kunci": "produksi",
	}
	if ubah != nil {
		ubah(m)
	}
	b, _ := json.Marshal(m)
	return string(b)
}

// sisipMenunggu menyisipkan baris berstatus menunggu seperti sisa formulir pendaftaran publik yang lama (tidak ada jalur kode yang membuatnya lagi).
func sisipMenunggu(t *testing.T, db *sql.DB, nama string) int64 {
	t.Helper()
	var id int64
	err := db.QueryRow(`INSERT INTO api_akses_klien (kode, nama_aplikasi, instansi, pic_nama, pic_email, tujuan, dataset_diminta, didaftarkan_ip)
		OUTPUT inserted.id VALUES (@p1, @p2, N'Instansi Lama', N'Pemohon Lama', N'lama@uji.go.id', N'Pendaftaran dari formulir lama yang sudah ditiadakan.', N'aset.tanah', N'198.51.100.77')`,
		"KL-"+strings.ToUpper(strings.ReplaceAll(nama[len(nama)-5:], " ", "X")), nama).Scan(&id)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestAPIAksesAlurLengkapDenganSQLServer(t *testing.T) {
	mulaiTes := time.Now()
	db := bukaDBUji(t)
	// Satu sambungan saja (permintaan di tes ini berurutan): login Windows (SSPI) paralel ke SQL Server lokal kadang diputus ("connection forcibly closed"), yang
	// bukan galat aplikasi. Tidak ada kode yang memegang satu sambungan sambil meminta yang lain, jadi aman.
	db.SetMaxOpenConns(1)
	db.SetConnMaxIdleTime(2 * time.Second)
	bersih := func() { bersihkanApiAksesUji(db); bersihkanUjiPeran(db) }
	bersih()
	t.Cleanup(bersih)
	seedAsetUji(t, db)

	st := &apiakses.Store{DB: db}
	t.Cleanup(func() { st.SimpanPengaturan(context.Background(), apiakses.PengaturanBawaan(), "pulih") })
	st.SimpanPengaturan(context.Background(), apiakses.PengaturanBawaan(), "awal")
	l := apiakses.NewLayanan(st, st, apiakses.NewPencatat(st, 1000), "pepper-uji-e2e")
	l.Mulai()
	t.Cleanup(func() { l.Berhenti(context.Background()) })
	t.Cleanup(handlers.GunakanApiAkses(l))
	auditStore := &audit.Store{DB: db}
	t.Cleanup(handlers.GunakanAuditStore(auditStore))
	siramAudit := pasangPerekam(t, auditStore)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	_ = r.SetTrustedProxies(nil)
	r.Use(audit.Middleware())
	SetupRoutes(r)
	u := &ujiAPI{t: t, r: r, l: l}

	admin := buatPenggunaUji(t, db, "api-admin", "superadmin")
	barang := buatPenggunaUji(t, db, "api-barang", "user")
	beriPeran(t, admin, barang, "pengguna_barang", "")
	tamu := buatPenggunaUji(t, db, "api-tamu", "user")

	// ---- Tidak ada pendaftaran publik: rute lama tidak ada (404), bahkan untuk superadmin
	for _, tok := range []string{"", tamu.token, barang.token, admin.token} {
		for _, p := range []string{"/api/v1/api-akses/info", "/api/v1/api-akses/pendaftaran"} {
			for _, mtd := range []string{"GET", "POST"} {
				if code, _, _, _ := u.panggil(mtd, p, bearer(tok), bodyKlienBaru("UJI-E2E Lama", nil)); code != 404 {
					t.Errorf("%s %s: %d, want 404 (formulir publik sudah ditiadakan)", mtd, p, code)
				}
			}
		}
	}

	// ---- Hanya superadmin yang mengelola dan mendaftarkan klien
	for _, tok := range []string{"", tamu.token, barang.token} {
		want := 403
		if tok == "" {
			want = 401
		}
		for _, p := range []string{"/api/v1/api-akses/klien", "/api/v1/api-akses/pengaturan", "/api/v1/api-akses/pantau/ringkasan", "/api/v1/api-akses/pantau/riwayat", "/api/v1/api-akses/pantau/opsi"} {
			if code, _, _, _ := u.panggil("GET", p, bearer(tok), ""); code != want {
				t.Errorf("GET %s token=%q: %d, want %d", p, tok[:min(len(tok), 8)], code, want)
			}
		}
		if code, _, _, _ := u.panggil("PUT", "/api/v1/api-akses/pengaturan", bearer(tok), "{}"); code != want {
			t.Errorf("PUT pengaturan: %d, want %d", code, want)
		}
		if code, _, _, _ := u.panggil("POST", "/api/v1/api-akses/klien", bearer(tok), bodyKlienBaru("UJI-E2E Tanpa Hak", nil)); code != want {
			t.Errorf("POST /klien token=%q: %d, want %d (hanya superadmin yang dapat mendaftarkan klien)", tok[:min(len(tok), 8)], code, want)
		}
	}
	var tersimpan int
	db.QueryRow(`SELECT COUNT(*) FROM api_akses_klien WHERE nama_aplikasi LIKE N'UJI-E2E%'`).Scan(&tersimpan)
	if tersimpan != 0 {
		t.Fatalf("permintaan tanpa hak superadmin tidak boleh menyimpan klien: %d", tersimpan)
	}

	// ---- Validasi isian pendaftaran oleh superadmin (semua kolom yang salah dilaporkan sekaligus; tidak ada yang tersimpan)
	buruk := []struct {
		nama  string
		ubah  func(map[string]interface{})
		kolom []string
	}{
		{"nama kosong", func(m map[string]interface{}) { m["nama_aplikasi"] = "  " }, []string{"nama_aplikasi"}},
		{"email salah", func(m map[string]interface{}) { m["pic_email"] = "bukan-email" }, []string{"pic_email"}},
		{"tujuan terlalu pendek", func(m map[string]interface{}) { m["tujuan"] = "singkat" }, []string{"tujuan"}},
		{"dataset di luar katalog", func(m map[string]interface{}) { m["dataset_diberikan"] = []string{"aset.tanah", "users"} }, []string{"dataset"}},
		{"tanpa dataset", func(m map[string]interface{}) { m["dataset_diberikan"] = []string{} }, []string{"dataset"}},
		{"IP membuka semuanya", func(m map[string]interface{}) { m["ip_diizinkan"] = "0.0.0.0/0" }, []string{"ip"}},
		{"UE1 salah", func(m map[string]interface{}) { m["ue1_diizinkan"] = "abc" }, []string{"ue1"}},
		{"batas nol", func(m map[string]interface{}) { m["limit_permintaan"] = 0; m["kuota_periode_detik"] = 5 }, []string{"limit", "periode"}},
		{"banyak sekaligus", func(m map[string]interface{}) {
			m["pic_nama"] = ""
			m["instansi"] = ""
			m["ip_diizinkan"] = "xx"
			m["kuota_permintaan"] = -1
		}, []string{"pic_nama", "instansi", "ip", "kuota"}},
	}
	for _, b := range buruk {
		code, m, _, _ := u.panggil("POST", "/api/v1/api-akses/klien", bearer(admin.token), bodyKlienBaru("UJI-E2E Buruk", b.ubah))
		if code != 400 || m["code"] != "validasi" {
			t.Errorf("%s: %d %v, want 400 validasi", b.nama, code, m)
			continue
		}
		galat, _ := m["errors"].(map[string]interface{})
		for _, k := range b.kolom {
			if galat[k] == nil {
				t.Errorf("%s: kolom %s harus bergalat: %v", b.nama, k, galat)
			}
		}
	}
	if code, _, _, _ := u.panggil("POST", "/api/v1/api-akses/klien", bearer(admin.token), "{bukan json"); code != 400 {
		t.Errorf("JSON rusak: %d, want 400", code)
	}
	db.QueryRow(`SELECT COUNT(*) FROM api_akses_klien WHERE nama_aplikasi LIKE N'UJI-E2E%'`).Scan(&tersimpan)
	if tersimpan != 0 {
		t.Fatalf("isian yang tidak sah tidak boleh menyimpan klien: %d", tersimpan)
	}

	// ---- Superadmin mendaftarkan klien: langsung disetujui, dataset tanah, UE1 09971, batas laju 100; kunci pertama terbit sekaligus.
	// Isian yang bukan milik formulir (status, id, diputuskan_oleh) tidak boleh berpengaruh.
	buat := bodyKlienBaru("UJI-E2E Satu", func(m map[string]interface{}) {
		m["status"] = "menunggu"
		m["id"] = 1
		m["diputuskan_oleh"] = "penyusup"
		m["kunci_aktif"] = 99
	})
	code, m, hdr, body := u.panggil("POST", "/api/v1/api-akses/klien", bearer(admin.token), buat)
	if code != 201 {
		t.Fatalf("daftarkan klien: %d %v", code, m)
	}
	if hdr.Get("Cache-Control") != "no-store" {
		t.Errorf("respons berisi kunci harus no-store: %q", hdr.Get("Cache-Control"))
	}
	dk := data(t, m)
	kl := dk["klien"].(map[string]interface{})
	kode := kl["kode"].(string)
	idKlien := int64(kl["id"].(float64))
	if !strings.HasPrefix(kode, "KL-") || kl["status"] != "disetujui" || kl["diputuskan_oleh"] != admin.username || kl["nama_aplikasi"] != "UJI-E2E Satu" || idKlien == 1 {
		t.Errorf("klien = %v", kl)
	}
	if ds := kl["dataset_diberikan"].([]interface{}); len(ds) != 1 || ds[0] != "aset.tanah" || kl["kunci_aktif"].(float64) != 1 {
		t.Errorf("dataset/kunci aktif: %v", kl)
	}
	if ue := kl["ue1_diizinkan"].([]interface{}); len(ue) != 1 || ue[0] != "09971" || kl["batas_berlaku"].(map[string]interface{})["limit_permintaan"].(float64) != 100 {
		t.Errorf("UE1/batas: %v", kl)
	}
	kunciBaru := dk["kunci_baru"].(map[string]interface{})
	kunci := kunciBaru["kunci"].(string)
	prefix := kunciBaru["info"].(map[string]interface{})["prefix"].(string)
	if _, ok := apiakses.PrefixKunci(kunci); !ok {
		t.Fatalf("bentuk kunci: %q", kunci)
	}
	if !strings.Contains(string(body), kunci) {
		t.Fatal("kunci harus ada pada respons penerbitan")
	}
	basis := fmt.Sprintf("/api/v1/api-akses/klien/%d", idKlien)
	code, m, _, _ = u.panggil("GET", "/api/v1/api-akses/klien?q=UJI-E2E", bearer(admin.token), "")
	if code != 200 || len(data(t, m)["klien"].([]interface{})) != 1 {
		t.Fatalf("daftar klien: %d %v", code, m)
	}
	if data(t, m)["per_status"].(map[string]interface{})["menunggu"].(float64) != 0 {
		t.Errorf("klien yang didaftarkan superadmin tidak boleh menambah yang menunggu: %v", data(t, m)["per_status"])
	}

	// Kunci tidak boleh terbaca lagi dari mana pun, dan database hanya menyimpan hash.
	for _, p := range []string{basis, "/api/v1/api-akses/klien", "/api/v1/api-akses/pantau/ringkasan", "/api/v1/api-akses/pantau/riwayat"} {
		_, _, _, b := u.panggil("GET", p, bearer(admin.token), "")
		if bytes.Contains(b, []byte(kunci)) || bytes.Contains(b, []byte(l.HashKunci(kunci))) {
			t.Errorf("GET %s membocorkan kunci atau hashnya", p)
		}
	}
	var hashDB string
	db.QueryRow(`SELECT hash FROM api_akses_kunci WHERE prefix = @p1`, prefix).Scan(&hashDB)
	if hashDB != l.HashKunci(kunci) || strings.Contains(hashDB, kunci[len(kunci)-20:]) || len(hashDB) != 64 {
		t.Errorf("hash di database = %q", hashDB)
	}
	var kolomMemuatKunci int
	db.QueryRow(`SELECT COUNT(*) FROM api_akses_kunci WHERE hash LIKE N'%' + @p1 + N'%' OR label LIKE N'%' + @p1 + N'%'`, kunci[len(kunci)-20:]).Scan(&kolomMemuatKunci)
	if kolomMemuatKunci != 0 {
		t.Error("kunci asli tersimpan di database")
	}

	// ---- Penarikan data dengan kunci
	api := apiakses.AwalanRute
	pakai := map[string]string{"Authorization": "Bearer " + kunci}
	code, m, hdr, _ = u.panggil("GET", api+"/aset/tanah?q=Tanah%20uji&per_halaman=2&hitung=1", pakai, "")
	if code != 200 {
		t.Fatalf("tarik data: %d %v", code, m)
	}
	dd := data(t, m)
	if int(dd["total"].(float64)) != 4 { // UE1 09971: A1 x2, A2, A3; B1 (09972) tidak boleh ikut
		t.Errorf("total = %v, want 4 (hanya UE1 09971)", dd["total"])
	}
	if len(dd["baris"].([]interface{})) != 2 || dd["ada_berikutnya"] != true || dd["kursor_berikutnya"] == nil {
		t.Errorf("halaman: %v", dd)
	}
	for _, b := range dd["baris"].([]interface{}) {
		row := b.(map[string]interface{})
		if row["provinsi_tanah"] != "DKI JAKARTA" || row["uraian_tanah"] != "Tanah uji" {
			t.Errorf("baris: %v", row)
		}
		if _, ada := row["foto"]; ada {
			t.Error("foto tidak boleh keluar")
		}
	}
	if hdr.Get("X-RateLimit-Limit") != "100" || hdr.Get("X-Quota-Limit") != "5000" {
		t.Errorf("header batas: limit %q kuota %q", hdr.Get("X-RateLimit-Limit"), hdr.Get("X-Quota-Limit"))
	}
	for k := range dd["baris"].([]interface{})[0].(map[string]interface{}) {
		if strings.Contains(strings.ToLower(k), "foto") {
			t.Errorf("kolom %s", k)
		}
	}
	// Halaman berikutnya via kursor.
	kursor := fmt.Sprint(int64(dd["kursor_berikutnya"].(float64)))
	code, m, _, _ = u.panggil("GET", api+"/aset/tanah?q=Tanah%20uji&per_halaman=2&kursor="+kursor, pakai, "")
	if code != 200 || len(data(t, m)["baris"].([]interface{})) != 2 || data(t, m)["ada_berikutnya"] != false {
		t.Errorf("halaman 2: %d %v", code, m)
	}

	// Scope dan penyaringan oleh pemilik.
	if code, m, _, _ := u.panggil("GET", api+"/aset/satker", pakai, ""); code != 403 || m["code"] != apiakses.KodeDatasetDitolak {
		t.Errorf("dataset di luar scope: %d %v", code, m)
	}
	if code, _, _, _ := u.panggil("GET", api+"/aset/tidak_ada", pakai, ""); code != 404 {
		t.Errorf("dataset tidak ada: %d", code)
	}
	if code, _, _, _ := u.panggil("GET", api+"/aset/tanah?kode_ue1=09972&q=Tanah%20uji", pakai, ""); code != 200 {
		t.Errorf("kode_ue1: %d", code)
	} else if _, m, _, _ := u.panggil("GET", api+"/aset/tanah?kode_ue1=09972&q=Tanah%20uji", pakai, ""); len(data(t, m)["baris"].([]interface{})) != 0 {
		t.Error("kode_ue1 milik UE1 lain tidak boleh melebarkan pembatasan klien")
	}
	for _, p := range []string{"?per_halaman=abc", "?kursor=-5", "?kode_satker=%27%3B%20DROP", "?q=ab", "?kode_ue1=1"} {
		if code, _, _, _ := u.panggil("GET", api+"/aset/tanah"+p, pakai, ""); code != 400 {
			t.Errorf("parameter %s: %d, want 400", p, code)
		}
	}
	var idLain int64 // baris tanah milik UE1 09972: tidak terjangkau klien ini
	db.QueryRow(`SELECT TOP 1 id FROM DIGITALISASI_TANAH WHERE Kode_UE1 = N'09972'`).Scan(&idLain)
	if code, _, _, _ := u.panggil("GET", fmt.Sprintf("%s/aset/tanah/%d", api, idLain), pakai, ""); code != 404 {
		t.Errorf("detail di luar UE1 klien: %d, want 404", code)
	}
	var idSendiri int64
	db.QueryRow(`SELECT TOP 1 id FROM DIGITALISASI_TANAH WHERE Kode_UE1 = N'09971'`).Scan(&idSendiri)
	if code, m, _, _ := u.panggil("GET", fmt.Sprintf("%s/aset/tanah/%d", api, idSendiri), pakai, ""); code != 200 || data(t, m)["baris"].(map[string]interface{})["uraian_tanah"] != "Tanah uji" {
		t.Errorf("detail: %d %v", code, m)
	}
	if code, m, _, _ := u.panggil("GET", api+"/katalog", pakai, ""); code != 200 || len(data(t, m)["dataset"].([]interface{})) != 1 {
		t.Errorf("katalog hanya memuat dataset dalam scope: %d %v", code, m)
	}
	code, m, _, b := u.panggil("GET", api+"/saya", pakai, "")
	if code != 200 || data(t, m)["klien"].(map[string]interface{})["kode"] != kode || bytes.Contains(b, []byte(kunci)) {
		t.Errorf("/saya: %d %s", code, b)
	}

	// ---- Pemisahan dengan autentikasi pengguna
	if code, _, _, _ := u.panggil("GET", api+"/katalog", bearer(admin.token), ""); code != 401 {
		t.Errorf("JWT superadmin pada API klien: %d, want 401", code)
	}
	for _, p := range []string{"/api/v1/users", "/api/v1/audit", "/api/v1/api-akses/klien", "/api/v1/digitalisasi/data/tanah", "/api/v1/auth/me"} {
		if code, _, _, _ := u.panggil("GET", p, pakai, ""); code != 401 {
			t.Errorf("kunci API pada %s: %d, want 401", p, code)
		}
	}

	// ---- Superadmin menurunkan batas klien ke 6 per 60 detik: lebih dari 6 permintaan sudah terjadi dalam 60 detik terakhir, jadi langsung 429 (jendela geser)
	turun := `{"dataset_diberikan":["aset.tanah"],"ue1_diizinkan":"09971","limit_permintaan":6,"catatan":"uji batas"}`
	if code, m, _, _ := u.panggil("PUT", basis, bearer(admin.token), turun); code != 200 {
		t.Fatalf("turunkan batas: %d %v", code, m)
	}
	code, m, hdr, _ = u.panggil("GET", api+"/aset/tanah?per_halaman=1", pakai, "")
	if code != 429 || m["code"] != apiakses.KodeBatasLaju || m["success"] != false {
		t.Fatalf("harus 429: %d %v", code, m)
	}
	if ra := hdr.Get("Retry-After"); ra == "" || ra == "0" || hdr.Get("X-RateLimit-Remaining") != "0" || hdr.Get("X-RateLimit-Limit") != "6" {
		t.Errorf("header 429: %v", hdr)
	}
	if m["retry_after_detik"].(float64) < 1 || m["retry_after_detik"].(float64) > 60 {
		t.Errorf("retry_after_detik = %v", m["retry_after_detik"])
	}
	// /saya tidak menghabiskan kuota tetapi tetap dikenai batas laju.
	if code, _, _, _ := u.panggil("GET", api+"/saya", pakai, ""); code != 429 {
		t.Errorf("/saya saat batas laju habis: %d, want 429", code)
	}

	// ---- Superadmin menaikkan batas klien: berlaku seketika tanpa deploy ulang
	ubah := `{"dataset_diberikan":["aset.tanah"],"ue1_diizinkan":"09971","limit_permintaan":500,"kuota_permintaan":9000,"catatan":"dinaikkan"}`
	if code, m, _, _ := u.panggil("PUT", basis, bearer(admin.token), ubah); code != 200 {
		t.Fatalf("ubah klien: %d %v", code, m)
	}
	code, _, hdr, _ = u.panggil("GET", api+"/aset/tanah?per_halaman=1", pakai, "")
	if code != 200 || hdr.Get("X-RateLimit-Limit") != "500" || hdr.Get("X-Quota-Limit") != "9000" {
		t.Errorf("batas baru: %d limit %q kuota %q", code, hdr.Get("X-RateLimit-Limit"), hdr.Get("X-Quota-Limit"))
	}
	// Input batas tidak sah ditolak.
	for _, bad := range []string{`{"dataset_diberikan":["aset.tanah"],"limit_permintaan":0}`, `{"dataset_diberikan":["aset.tanah"],"limit_permintaan":999999999}`, `{"dataset_diberikan":["aset.tanah"],"ip_diizinkan":"0.0.0.0/0"}`,
		`{"dataset_diberikan":["tabel_lain"]}`, `{"dataset_diberikan":["aset.tanah"],"ue1_diizinkan":"abc"}`} {
		if code, _, _, _ := u.panggil("PUT", basis, bearer(admin.token), bad); code != 400 {
			t.Errorf("PUT %s: %d, want 400", bad, code)
		}
	}
	// Allowlist IP: alamat penguji (203.0.113.20) tidak ada dalam daftar -> 403; setelah ditambahkan -> 200.
	pasangIP := func(ip string) {
		b, _ := json.Marshal(map[string]interface{}{"dataset_diberikan": []string{"aset.tanah"}, "ue1_diizinkan": "09971", "ip_diizinkan": ip, "limit_permintaan": 500})
		if code, m, _, _ := u.panggil("PUT", basis, bearer(admin.token), string(b)); code != 200 {
			t.Fatalf("pasang allowlist: %d %v", code, m)
		}
	}
	pasangIP("198.51.100.0/24")
	if code, _, _, _ := u.panggil("GET", api+"/katalog", pakai, ""); code != 403 {
		t.Errorf("IP di luar allowlist: %d, want 403", code)
	}
	pasangIP("203.0.113.0/24")
	if code, _, _, _ := u.panggil("GET", api+"/katalog", pakai, ""); code != 200 {
		t.Errorf("IP dalam allowlist: %d, want 200", code)
	}

	// ---- Pengaturan global: berlaku tanpa deploy ulang untuk klien tanpa override
	code, m, _, _ = u.panggil("GET", "/api/v1/api-akses/pengaturan", bearer(admin.token), "")
	p := data(t, m)["pengaturan"].(map[string]interface{})
	if code != 200 || p["limit_permintaan"].(float64) != 1000 || p["limit_jendela_detik"].(float64) != 60 || p["kuota_permintaan"].(float64) != 5000 || p["kuota_periode_detik"].(float64) != 3600 {
		t.Fatalf("bawaan harus 1000/60 dan 5000/3600: %d %v", code, p)
	}
	p["limit_permintaan"], p["kuota_permintaan"] = 1500, 7000
	pb, _ := json.Marshal(p)
	if code, m, _, _ := u.panggil("PUT", "/api/v1/api-akses/pengaturan", bearer(admin.token), string(pb)); code != 200 {
		t.Fatalf("simpan pengaturan: %d %v", code, m)
	}
	p["limit_permintaan"] = 0
	pb, _ = json.Marshal(p)
	if code, _, _, _ := u.panggil("PUT", "/api/v1/api-akses/pengaturan", bearer(admin.token), string(pb)); code != 400 {
		t.Errorf("pengaturan tidak sah: %d, want 400", code)
	}
	tanpaOverride := fmt.Sprintf(`{"dataset_diberikan":["aset.tanah"],"ue1_diizinkan":"09971","ip_diizinkan":""}`)
	u.panggil("PUT", basis, bearer(admin.token), tanpaOverride)
	_, _, hdr, _ = u.panggil("GET", api+"/katalog", pakai, "")
	if hdr.Get("X-RateLimit-Limit") != "1500" || hdr.Get("X-Quota-Limit") != "7000" {
		t.Errorf("klien tanpa override harus mengikuti bawaan baru: limit %q kuota %q", hdr.Get("X-RateLimit-Limit"), hdr.Get("X-Quota-Limit"))
	}

	// ---- Rotasi: kunci lama tetap berlaku selama masa tenggang, lalu dicabut dan langsung ditolak
	code, m, _, _ = u.panggil("POST", basis+"/kunci", bearer(admin.token), `{"label":"rotasi","rotasi":true}`)
	if code != 201 {
		t.Fatalf("rotasi: %d %v", code, m)
	}
	kunci2 := data(t, m)["kunci"].(string)
	if kunci2 == kunci {
		t.Fatal("kunci baru harus berbeda")
	}
	pakai2 := map[string]string{"X-API-Key": kunci2}
	if code, _, _, _ := u.panggil("GET", api+"/katalog", pakai2, ""); code != 200 {
		t.Errorf("kunci baru: %d", code)
	}
	if code, _, _, _ := u.panggil("GET", api+"/katalog", pakai, ""); code != 200 {
		t.Errorf("kunci lama harus tetap berlaku selama masa tenggang: %d", code)
	}
	var idKunciLama int64
	db.QueryRow(`SELECT id FROM api_akses_kunci WHERE prefix = @p1`, prefix).Scan(&idKunciLama)
	if code, _, _, _ := u.panggil("DELETE", fmt.Sprintf("%s/kunci/%d?alasan=uji", basis, idKunciLama), bearer(admin.token), ""); code != 200 {
		t.Fatalf("cabut kunci lama: %d", code)
	}
	if code, _, _, _ := u.panggil("GET", api+"/katalog", pakai, ""); code != 401 {
		t.Errorf("kunci yang dicabut harus langsung ditolak: %d, want 401", code)
	}
	if code, _, _, _ := u.panggil("DELETE", fmt.Sprintf("%s/kunci/%d", basis, idKunciLama), bearer(admin.token), ""); code != 404 {
		t.Errorf("mencabut dua kali: %d, want 404", code)
	}
	if code, _, _, _ := u.panggil("GET", api+"/katalog", pakai2, ""); code != 200 {
		t.Errorf("kunci baru tetap berlaku: %d", code)
	}

	// ---- Pemantauan (menunggu pencatat menulis)
	var ring map[string]interface{}
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		code, m, _, _ = u.panggil("GET", fmt.Sprintf("/api/v1/api-akses/pantau/ringkasan?rentang=1j&klien_id=%d", idKlien), bearer(admin.token), "")
		if code != 200 {
			t.Fatalf("ringkasan: %d %v", code, m)
		}
		ring = data(t, m)
		if r := ring["ringkasan"].(map[string]interface{}); r["jumlah"].(float64) >= 20 {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	rg := ring["ringkasan"].(map[string]interface{})
	if rg["jumlah"].(float64) < 20 || rg["sukses"].(float64) < 10 || rg["batas_laju"].(float64) < 1 || rg["ditolak"].(float64) < 1 {
		t.Errorf("ringkasan = %v", rg)
	}
	if rg["lat_rata_ms"] == nil || rg["lat_p95_ms"] == nil {
		t.Errorf("latensi: %v", rg)
	}
	if len(ring["per_klien"].([]interface{})) != 1 || ring["per_klien"].([]interface{})[0].(map[string]interface{})["nama"] != "UJI-E2E Satu" {
		t.Errorf("per_klien: %v", ring["per_klien"])
	}
	if len(ring["per_endpoint"].([]interface{})) < 2 || len(ring["deret"].([]interface{})) < 1 || len(ring["pemakaian"].([]interface{})) != 1 {
		t.Errorf("per_endpoint/deret/pemakaian: %v", ring)
	}
	pk := ring["pemakaian"].([]interface{})[0].(map[string]interface{})
	if pk["batas"].(map[string]interface{})["limit_permintaan"].(float64) != 1500 {
		t.Errorf("pemakaian kuota saat ini: %v", pk)
	}
	// Kuota berganti periode tiap jam penuh UTC: bila tes ini kebetulan melewati pergantian jam, hitungannya memang kembali nol (itu perilaku yang benar), jadi hanya
	// diperiksa bila periode berjalan sudah dimulai sebelum tes mulai.
	if reset, err := time.Parse(time.RFC3339, pk["kuota_reset"].(string)); err == nil && !reset.Add(-time.Hour).After(mulaiTes) && pk["kuota_terpakai"].(float64) < 5 {
		t.Errorf("kuota terpakai harus mencerminkan permintaan yang sudah terjadi: %v", pk)
	}
	code, m, _, _ = u.panggil("GET", fmt.Sprintf("/api/v1/api-akses/pantau/riwayat?rentang=1j&klien_id=%d&hasil=batas_laju", idKlien), bearer(admin.token), "")
	rw := data(t, m)["riwayat"].([]interface{})
	if code != 200 || len(rw) < 1 || rw[0].(map[string]interface{})["status_http"].(float64) != 429 || rw[0].(map[string]interface{})["klien_nama"] != "UJI-E2E Satu" {
		t.Errorf("riwayat 429: %d %v", code, m)
	}
	if code, _, _, _ := u.panggil("GET", "/api/v1/api-akses/pantau/riwayat?hasil=sembarang", bearer(admin.token), ""); code != 400 {
		t.Errorf("hasil tidak dikenal: %d", code)
	}
	if code, _, _, _ := u.panggil("GET", "/api/v1/api-akses/pantau/ringkasan?dari=2026-10-08&sampai=2026-10-01", bearer(admin.token), ""); code != 400 {
		t.Errorf("rentang terbalik: %d", code)
	}
	code, m, _, _ = u.panggil("GET", api+"/saya/penggunaan?rentang=1j", map[string]string{"Authorization": "Bearer " + kunci2}, "")
	if code != 200 || data(t, m)["ringkasan"].(map[string]interface{})["jumlah"].(float64) < 1 {
		t.Errorf("pemilik klien melihat pemakaiannya sendiri: %d %v", code, m)
	}
	if rl := data(t, m)["riwayat_terakhir"].([]interface{}); len(rl) > 0 && (rl[0].(map[string]interface{})["klien_kode"] != nil || rl[0].(map[string]interface{})["kunci_prefix"] != nil) {
		t.Errorf("riwayat untuk klien tidak boleh memuat data internal: %v", rl[0])
	}
	if code, _, _, _ := u.panggil("GET", api+"/saya/penggunaan?rentang=1t", pakai2, ""); code != 200 {
		t.Log("rentang 1t dipotong ke 30 hari") // dipotong menjadi 30 hari, bukan ditolak
	}

	// ---- Pencabutan klien: semua kunci langsung mati; diaktifkan kembali memberi kunci baru, kunci lama tetap mati
	if code, _, _, _ := u.panggil("POST", basis+"/cabut", bearer(admin.token), `{}`); code != 400 {
		t.Errorf("mencabut tanpa alasan: %d, want 400", code)
	}
	if code, _, _, _ := u.panggil("POST", basis+"/cabut", bearer(admin.token), `{"catatan":"uji pencabutan"}`); code != 200 {
		t.Fatalf("cabut klien: %d", code)
	}
	if code, _, _, _ := u.panggil("GET", api+"/katalog", pakai2, ""); code != 401 {
		t.Errorf("klien dicabut: %d, want 401", code)
	}
	if code, _, _, _ := u.panggil("POST", basis+"/cabut", bearer(admin.token), `{"catatan":"lagi"}`); code != 409 {
		t.Errorf("mencabut klien yang sudah dicabut: %d, want 409", code)
	}
	code, m, _, _ = u.panggil("POST", basis+"/aktifkan", bearer(admin.token), `{}`)
	if code != 200 {
		t.Fatalf("aktifkan: %d %v", code, m)
	}
	kunci3 := data(t, m)["kunci_baru"].(map[string]interface{})["kunci"].(string)
	if code, _, _, _ := u.panggil("GET", api+"/katalog", pakai2, ""); code != 401 {
		t.Errorf("kunci lama tidak boleh hidup lagi: %d, want 401", code)
	}
	if code, _, _, _ := u.panggil("GET", api+"/katalog", map[string]string{"Authorization": "Bearer " + kunci3}, ""); code != 200 {
		t.Errorf("kunci baru setelah diaktifkan kembali: %d, want 200", code)
	}

	// ---- Sisa pendaftaran lama berstatus menunggu (dari formulir publik yang sudah ditiadakan) tetap dapat diputuskan superadmin: ditolak bersifat final, atau disetujui
	idTolak := sisipMenunggu(t, db, "UJI-E2E Lama Ditolak")
	bt := fmt.Sprintf("/api/v1/api-akses/klien/%d", idTolak)
	if code, _, _, _ := u.panggil("POST", bt+"/tolak", bearer(admin.token), `{"catatan":"tidak memenuhi syarat"}`); code != 200 {
		t.Errorf("tolak: %d", code)
	}
	if code, _, _, _ := u.panggil("POST", bt+"/setujui", bearer(admin.token), `{}`); code != 409 {
		t.Errorf("setujui yang sudah ditolak: %d, want 409", code)
	}
	if code, _, _, _ := u.panggil("PUT", bt, bearer(admin.token), `{"dataset_diberikan":["aset.tanah"]}`); code != 409 {
		t.Errorf("ubah klien ditolak: %d, want 409", code)
	}
	idSetuju := sisipMenunggu(t, db, "UJI-E2E Lama Disetujui")
	code, m, _, _ = u.panggil("POST", fmt.Sprintf("/api/v1/api-akses/klien/%d/setujui", idSetuju), bearer(admin.token), `{"ue1_diizinkan":"09971"}`)
	if code != 200 || data(t, m)["kunci_baru"] == nil || data(t, m)["klien"].(map[string]interface{})["status"] != "disetujui" {
		t.Errorf("setujui pendaftaran lama: %d %v", code, m)
	}
	// Pengaturan tidak lagi memuat pendaftaran publik.
	if _, m, _, _ := u.panggil("GET", "/api/v1/api-akses/pengaturan", bearer(admin.token), ""); strings.Contains(string(mustJSON(m)), "pendaftaran") {
		t.Errorf("pengaturan masih memuat pendaftaran publik: %v", m)
	}

	// ---- Log Audit: pengelolaan dan kejadian keamanan tercatat, tanpa kunci apa pun
	siramAudit()
	rows, err := db.Query(`SELECT aksi, ISNULL(detail, N''), label, ISNULL(username, N''), status_http FROM audit_log WHERE kategori = N'api_akses'`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	aksi := map[string]int{}
	for rows.Next() {
		var a, det, label, user string
		var status int
		rows.Scan(&a, &det, &label, &user, &status)
		aksi[a]++
		for _, rahasia := range []string{kunci, kunci2, kunci3, l.HashKunci(kunci), l.HashKunci(kunci2), "budi.penguji@uji.go.id"} {
			if strings.Contains(det+label+user, rahasia) {
				t.Errorf("Log Audit memuat data rahasia/pribadi pada aksi %s: %s", a, det)
			}
		}
	}
	for _, w := range []string{"apiakses.klien.buat", "apiakses.klien.setujui", "apiakses.klien.ubah", "apiakses.kunci.rotasi", "apiakses.kunci.cabut", "apiakses.klien.cabut", "apiakses.klien.aktifkan",
		"apiakses.klien.tolak", "apiakses.pengaturan.ubah", "akses.ditolak"} {
		if aksi[w] == 0 {
			t.Errorf("aksi %s tidak tercatat di Log Audit (tercatat: %v)", w, aksi)
		}
	}
}

func mustJSON(v interface{}) []byte { b, _ := json.Marshal(v); return b }
