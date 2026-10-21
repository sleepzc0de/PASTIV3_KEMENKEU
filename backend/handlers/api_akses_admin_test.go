package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"pasti-v3-backend/apiakses"
	"pasti-v3-backend/internal/fakesql"
)

// Tes validasi pendaftaran klien oleh superadmin (POST /api-akses/klien) tanpa SQL Server: isian yang tidak sah ditolak SEBELUM menyentuh database. Pembuatan sungguhan,
// hak akses superadmin, dan Log Audit diuji di routes/api_akses_integrasi_test.go.

func routerKlienBaru(t *testing.T) (*gin.Engine, *fakesql.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, f := fakesql.New(t)
	t.Cleanup(GunakanApiAkses(apiakses.NewLayanan(&apiakses.Store{DB: db}, nil, nil, "pepper-uji")))
	r := gin.New()
	r.POST("/klien", PostApiAksesKlien)
	return r, f
}

func bodyBaru(ubah func(map[string]interface{})) string {
	m := map[string]interface{}{
		"nama_aplikasi": "Aplikasi Uji", "instansi": "Direktorat Uji", "unit": "Bagian TI", "pic_nama": "Budi Penguji", "pic_jabatan": "Kepala", "pic_email": "budi@uji.go.id",
		"pic_telepon": "021-5551234", "tujuan": "Menampilkan sebaran aset tanah pada dasbor pengujian internal.", "dataset_diberikan": []string{"aset.tanah"},
	}
	if ubah != nil {
		ubah(m)
	}
	b, _ := json.Marshal(m)
	return string(b)
}

func kirimBaru(r *gin.Engine, badan string) (int, map[string]interface{}) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/klien", strings.NewReader(badan))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	var out map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return w.Code, out
}

func TestKlienBaruIsianTidakSahDitolakSebelumDatabase(t *testing.T) {
	r, f := routerKlienBaru(t)
	n := func(v int) int { return v }
	cases := []struct {
		nama  string
		ubah  func(map[string]interface{})
		kolom []string
	}{
		{"nama kosong", func(m map[string]interface{}) { m["nama_aplikasi"] = "   " }, []string{"nama_aplikasi"}},
		{"nama berkarakter kendali", func(m map[string]interface{}) { m["nama_aplikasi"] = "Apl\u0000ikasi" }, []string{"nama_aplikasi"}},
		{"instansi kosong", func(m map[string]interface{}) { m["instansi"] = "" }, []string{"instansi"}},
		{"email salah", func(m map[string]interface{}) { m["pic_email"] = "bukan-email" }, []string{"pic_email"}},
		{"email dengan baris baru", func(m map[string]interface{}) { m["pic_email"] = "a@b.go.id\nBcc: x@y.com" }, []string{"pic_email"}},
		{"telepon huruf", func(m map[string]interface{}) { m["pic_telepon"] = "abc" }, []string{"pic_telepon"}},
		{"tujuan pendek", func(m map[string]interface{}) { m["tujuan"] = "uji" }, []string{"tujuan"}},
		{"tujuan terlalu panjang", func(m map[string]interface{}) { m["tujuan"] = strings.Repeat("x", 2001) }, []string{"tujuan"}},
		{"dataset tidak dikenal", func(m map[string]interface{}) { m["dataset_diberikan"] = []string{"aset.tanah", "users"} }, []string{"dataset"}},
		{"tanpa dataset", func(m map[string]interface{}) { m["dataset_diberikan"] = []string{} }, []string{"dataset"}},
		{"dataset null", func(m map[string]interface{}) { m["dataset_diberikan"] = nil }, []string{"dataset"}},
		{"IP bukan alamat", func(m map[string]interface{}) { m["ip_diizinkan"] = "bukan-ip" }, []string{"ip"}},
		{"IP membuka semuanya", func(m map[string]interface{}) { m["ip_diizinkan"] = "0.0.0.0/0" }, []string{"ip"}},
		{"UE1 bukan 5 digit", func(m map[string]interface{}) { m["ue1_diizinkan"] = "1504" }, []string{"ue1"}},
		{"UE1 berisi SQL", func(m map[string]interface{}) { m["ue1_diizinkan"] = "01504' OR '1'='1" }, []string{"ue1"}},
		{"batas nol", func(m map[string]interface{}) { m["limit_permintaan"] = n(0) }, []string{"limit"}},
		{"batas terlalu besar", func(m map[string]interface{}) { m["limit_permintaan"] = 100001 }, []string{"limit"}},
		{"jendela nol", func(m map[string]interface{}) { m["limit_jendela_detik"] = 0 }, []string{"jendela"}},
		{"kuota negatif", func(m map[string]interface{}) { m["kuota_permintaan"] = -1 }, []string{"kuota"}},
		{"periode 10 detik", func(m map[string]interface{}) { m["kuota_periode_detik"] = 10 }, []string{"periode"}},
		{"catatan terlalu panjang", func(m map[string]interface{}) { m["catatan"] = strings.Repeat("c", 1001) }, []string{"catatan"}},
		{"semua salah sekaligus", func(m map[string]interface{}) {
			m["nama_aplikasi"], m["pic_email"], m["dataset_diberikan"], m["ip_diizinkan"], m["limit_permintaan"] = "", "x", []string{}, "x", 0
		}, []string{"nama_aplikasi", "pic_email", "dataset", "ip", "limit"}},
	}
	for _, c := range cases {
		code, out := kirimBaru(r, bodyBaru(c.ubah))
		if code != http.StatusBadRequest || out["code"] != "validasi" {
			t.Errorf("%s: %d %v, want 400 validasi", c.nama, code, out)
			continue
		}
		galat, _ := out["errors"].(map[string]interface{})
		for _, k := range c.kolom {
			if galat[k] == nil || galat[k] == "" {
				t.Errorf("%s: kolom %s harus bergalat: %v", c.nama, k, galat)
			}
		}
		// Pesan tidak memantulkan isian pemanggil.
		if s := string(mustJSONUji(out)); strings.Contains(s, "OR '1'='1") || strings.Contains(s, "Bcc:") {
			t.Errorf("%s: pesan galat memantulkan isian: %s", c.nama, s)
		}
	}
	if ev := f.Events(); len(ev) != 0 {
		t.Errorf("isian yang tidak sah tidak boleh menyentuh database: %v", ev)
	}
	for _, rusak := range []string{"", "{bukan json", "[]", "null"} {
		if code, _ := kirimBaru(r, rusak); code != http.StatusBadRequest {
			t.Errorf("badan %q: %d, want 400", rusak, code)
		}
	}
	if ev := f.Events(); len(ev) != 0 {
		t.Errorf("badan rusak tidak boleh menyentuh database: %v", ev)
	}
}

// Isian sah tanpa captcha atau kolom pendaftaran publik lainnya lolos validasi: yang menggagalkannya di sini hanya database palsu (500), bukan validasi (400). Ini juga
// membuktikan tidak ada lagi captcha atau pemeriksaan "pendaftaran dibuka" pada jalur pembuatan klien.
func TestKlienBaruIsianSahTanpaCaptchaLolosValidasi(t *testing.T) {
	r, f := routerKlienBaru(t)
	code, out := kirimBaru(r, bodyBaru(func(m map[string]interface{}) {
		m["ip_diizinkan"] = "203.0.113.0/24, 10.1.2.3"
		m["ue1_diizinkan"] = "01504, 01505"
		m["limit_permintaan"], m["kuota_permintaan"] = 500, 9000
	}))
	if code != http.StatusInternalServerError {
		t.Fatalf("isian sah: %d %v, want 500 (database palsu)", code, out)
	}
	if out["code"] == "validasi" || out["errors"] != nil {
		t.Errorf("isian sah ditolak validasi: %v", out)
	}
	if f.Count("BEGIN") != 1 {
		t.Errorf("pembuatan harus dicoba dalam satu transaksi: %v", f.Events())
	}
	if msg, _ := out["message"].(string); strings.Contains(strings.ToLower(msg), "fakesql") || strings.Contains(msg, "OnQuery") {
		t.Errorf("galat internal bocor: %q", msg)
	}
}

func TestKlienBaruTanpaLayananMenjawab503(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Cleanup(GunakanApiAkses(nil))
	r := gin.New()
	r.POST("/klien", PostApiAksesKlien)
	if code, _ := kirimBaru(r, bodyBaru(nil)); code != http.StatusServiceUnavailable {
		t.Errorf("tanpa layanan: %d, want 503", code)
	}
}

func TestUbahGalatMelaporkanKolomMemetakanPesanPertama(t *testing.T) {
	n := func(v int) *int { return &v }
	_, g := (cakupanReq{DatasetDiberikan: []string{"aset.tanah"}, IPDiizinkan: "10.0.0.1", UE1Diizinkan: "01504"}).ubahGalat()
	if len(g) != 0 {
		t.Fatalf("cakupan sah: %v", g)
	}
	u, g := (cakupanReq{DatasetDiberikan: []string{"aset.satker", "aset.tanah", "aset.tanah"}, IPDiizinkan: "10.0.0.1/24", LimitPermintaan: n(7), Catatan: "  catatan  "}).ubahGalat()
	if len(g) != 0 || strings.Join(u.DatasetDiberikan, ",") != "aset.satker,aset.tanah" || u.IPDiizinkan[0] != "10.0.0.0/24" || *u.Override.LimitPermintaan != 7 || u.Catatan != "catatan" {
		t.Errorf("normalisasi: %+v %v", u, g)
	}
	_, pesan := (cakupanReq{DatasetDiberikan: []string{"aset.tanah"}, IPDiizinkan: "x", LimitPermintaan: n(0)}).ubah()
	if pesan == "" {
		t.Error("ubah harus memberi satu pesan bila ada galat")
	}
}

func mustJSONUji(v interface{}) []byte { b, _ := json.Marshal(v); return b }
