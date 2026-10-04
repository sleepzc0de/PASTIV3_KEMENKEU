package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestSusunTugasMatriksDanEksplisit(t *testing.T) {
	// Dua dataset per KLPD+tahun x dua tahun = 4, ditambah dataset per KLPD (1) dan kategori tingkat 1 (1).
	tugas, pesan := susunTugas(permintaanPenarikan{
		Datasets: []string{"tender/pengumuman", "rup/paket-penyedia", "ekatalog-archive/instansi-satker", "ekatalog/list-kategori-produk", "tender/pengumuman"},
		Tahun:    []string{"2025", "2024"},
	})
	if pesan != "" || len(tugas) != 6 {
		t.Fatalf("tugas=%d pesan=%q, want 6 tugas", len(tugas), pesan)
	}
	var kunci []string
	for _, tg := range tugas {
		kunci = append(kunci, tg.Kunci())
	}
	for _, mau := range []string{"tender/pengumuman|K10/2025", "tender/pengumuman|K10/2024", "rup/paket-penyedia|K10/2025", "ekatalog-archive/instansi-satker|K10", "ekatalog/list-kategori-produk|L1"} {
		if !strings.Contains(strings.Join(kunci, ","), mau) {
			t.Errorf("tugas %q tidak ada di %v", mau, kunci)
		}
	}

	// Semua dataset otomatis x 2 tahun = jumlah tugas rencana otomatis dengan tahun yang sama.
	tugas, pesan = susunTugas(permintaanPenarikan{SemuaOtomatis: true, Tahun: []string{"2025", "2024"}, KodeKLPD: "K10"})
	if pesan != "" || len(tugas) != 56 {
		t.Errorf("semua otomatis: %d tugas, pesan %q, want 56", len(tugas), pesan)
	}

	// Daftar eksplisit: dataset per kode dan status RUP.
	var req permintaanPenarikan
	if err := json.Unmarshal([]byte(`{"tugas":[
		{"dataset":"ekatalog/penyedia-detail","kode":" 01ABC "},
		{"dataset":"rup/paket-penyedia","tahun":"2025","status":"Terumumkan"},
		{"dataset":"ekatalog/e-purchasing-by-produk","tahun":"2025","status":"completed","kd_kategori_1":"K1"}]}`), &req); err != nil {
		t.Fatal(err)
	}
	tugas, pesan = susunTugas(req)
	if pesan != "" || len(tugas) != 3 {
		t.Fatalf("eksplisit: %d tugas, pesan %q", len(tugas), pesan)
	}
	if tugas[0].Kunci() != "ekatalog/penyedia-detail|01ABC" || tugas[1].Kunci() != "rup/paket-penyedia|K10/2025/Terumumkan" || tugas[2].Kunci() != "ekatalog/e-purchasing-by-produk|2025/COMPLETED/K1" {
		t.Errorf("kunci = %s | %s | %s", tugas[0].Kunci(), tugas[1].Kunci(), tugas[2].Kunci())
	}
}

func TestSusunTugasMenolakPermintaanSalah(t *testing.T) {
	salah := map[string]permintaanPenarikan{
		"dataset tak dikenal":      {Datasets: []string{"tidak/ada"}, Tahun: []string{"2025"}},
		"tanpa tahun":              {Datasets: []string{"tender/pengumuman"}},
		"tahun ngawur":             {Datasets: []string{"tender/pengumuman"}, Tahun: []string{"25"}},
		"dataset per kode matriks": {Datasets: []string{"ekatalog/penyedia-detail"}, Tahun: []string{"2025"}},
		"transaksi status ngawur": {Tugas: []struct {
			Dataset string `json:"dataset"`
			PermintaanTarik
		}{{Dataset: "ekatalog/e-purchasing-by-produk", PermintaanTarik: PermintaanTarik{Tahun: "2025", Status: "NGAWUR"}}}},
		"tugas tanpa kode": {Tugas: []struct {
			Dataset string `json:"dataset"`
			PermintaanTarik
		}{{Dataset: "ekatalog/penyedia-detail"}}},
	}
	for nama, req := range salah {
		if tugas, pesan := susunTugas(req); pesan == "" || tugas != nil {
			t.Errorf("%s: tugas=%d pesan=%q, want ditolak", nama, len(tugas), pesan)
		}
	}
	// Kosong sama sekali bukan galat di sini (Start yang menolaknya).
	if tugas, pesan := susunTugas(permintaanPenarikan{}); pesan != "" || len(tugas) != 0 {
		t.Errorf("kosong: %d %q", len(tugas), pesan)
	}
}

func TestMaskRiwayatMenyembunyikanGalatDariNonAdmin(t *testing.T) {
	pesan, oleh := "dial tcp 10.1.2.3:443: connection refused", "admin1"
	r := RiwayatPenarikan{Status: PenarikanGagal, Pesan: &pesan, DijalankanOleh: &oleh}
	if got := maskRiwayat(r, false); *got.Pesan != "Penarikan gagal" || got.DijalankanOleh != nil {
		t.Errorf("non-admin melihat %v / %v", *got.Pesan, got.DijalankanOleh)
	}
	if got := maskRiwayat(r, true); *got.Pesan != pesan || got.DijalankanOleh == nil {
		t.Errorf("admin harus melihat apa adanya: %+v", got)
	}
	ok := RiwayatPenarikan{Status: PenarikanSukses, Pesan: &pesan}
	if got := maskRiwayat(ok, false); *got.Pesan != pesan {
		t.Error("pesan penarikan sukses tidak perlu disembunyikan")
	}

	a := &InfoAktif{Oleh: "admin1", Tugas: []TugasInfo{{Status: PenarikanGagal, Pesan: "rahasia"}, {Status: PenarikanBerjalan, Pesan: "halaman 2"}}}
	a = maskInfoAktif(a, false)
	if a.Oleh != "" || a.Tugas[0].Pesan != "Penarikan gagal" || a.Tugas[1].Pesan != "halaman 2" {
		t.Errorf("info aktif = %+v", a)
	}
}

func routerPenarikan() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	g := r.Group("/inaproc", func(c *gin.Context) { c.Set("user_id", "u-1"); c.Set("username", "admin1"); c.Next() })
	g.POST("/penarikan", StartInaprocPenarikan)
	g.POST("/penarikan/batal", CancelInaprocPenarikan)
	g.GET("/penarikan/aktif", GetInaprocPenarikanAktif)
	g.PUT("/penarikan/pengaturan", PutInaprocPenarikanPengaturan)
	return r
}

func kirim(r *gin.Engine, metode, url, badan string) (int, map[string]interface{}) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(metode, url, strings.NewReader(badan))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	var out map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return w.Code, out
}

func TestStartPenarikanHTTP(t *testing.T) {
	pasangKonfigInaproc(t, "http://tidak-dipakai.invalid", "token-uji")
	var dijalankan int32
	lepas := make(chan struct{})
	m, _ := penarikUji(t, func(ctx context.Context, tg Tugas, oleh string, kabar func(string)) (HasilSinkron, error) {
		atomic.AddInt32(&dijalankan, 1)
		<-lepas
		return HasilSinkron{}, nil
	})
	lama := Penarik
	Penarik = m
	t.Cleanup(func() { Penarik = lama })
	r := routerPenarikan()

	if kode, _ := kirim(r, "POST", "/inaproc/penarikan", `{bukan json`); kode != 400 {
		t.Errorf("JSON rusak: %d", kode)
	}
	if kode, res := kirim(r, "POST", "/inaproc/penarikan", `{"datasets":["tender/pengumuman"]}`); kode != 400 || !strings.Contains(res["message"].(string), "tahun") {
		t.Errorf("tanpa tahun: %d %v", kode, res)
	}
	if kode, _ := kirim(r, "POST", "/inaproc/penarikan", `{}`); kode != 400 {
		t.Errorf("tanpa tugas: %d", kode)
	}

	kode, res := kirim(r, "POST", "/inaproc/penarikan", `{"datasets":["tender/pengumuman","tender/peserta-tender"],"tahun":["2025"]}`)
	if kode != http.StatusAccepted {
		t.Fatalf("status %d: %v", kode, res)
	}
	aktif := res["data"].(map[string]interface{})["aktif"].(map[string]interface{})
	if aktif["total"].(float64) != 2 || aktif["pemicu"] != PemicuManual || aktif["oleh"] != "admin1" {
		t.Errorf("aktif = %v", aktif)
	}
	// Antrean kedua selama yang pertama berjalan: 409.
	if kode, _ := kirim(r, "POST", "/inaproc/penarikan", `{"datasets":["tender/tender-selesai"],"tahun":["2025"]}`); kode != http.StatusConflict {
		t.Errorf("antrean kedua: %d, want 409", kode)
	}
	// Polling kemajuan.
	kode, res = kirim(r, "GET", "/inaproc/penarikan/aktif", "")
	if kode != 200 || res["data"].(map[string]interface{})["aktif"] == nil {
		t.Errorf("aktif: %d %v", kode, res)
	}
	// Batalkan.
	kode, res = kirim(r, "POST", "/inaproc/penarikan/batal", "")
	if kode != 200 || res["data"].(map[string]interface{})["dibatalkan"] != true {
		t.Errorf("batal: %d %v", kode, res)
	}
	close(lepas)
	tungguSelesai(t, m)
	kode, res = kirim(r, "GET", "/inaproc/penarikan/aktif", "")
	if kode != 200 || res["data"].(map[string]interface{})["aktif"] != nil {
		t.Errorf("setelah selesai: %d %v", kode, res)
	}
	if kode, res := kirim(r, "POST", "/inaproc/penarikan/batal", ""); kode != 200 || res["data"].(map[string]interface{})["dibatalkan"] != false {
		t.Errorf("batal tanpa antrean: %d %v", kode, res)
	}
}

func TestStartPenarikanTanpaTokenDitolak(t *testing.T) {
	pasangKonfigInaproc(t, "http://x", "")
	m, _ := penarikUji(t, sukses(0))
	lama := Penarik
	Penarik = m
	t.Cleanup(func() { Penarik = lama })
	if kode, _ := kirim(routerPenarikan(), "POST", "/inaproc/penarikan", `{"datasets":["tender/pengumuman"],"tahun":["2025"]}`); kode != http.StatusServiceUnavailable {
		t.Errorf("status %d, want 503", kode)
	}
}

func TestPenarikanBelumSiapMenjawab503(t *testing.T) {
	lama := Penarik
	Penarik = nil
	t.Cleanup(func() { Penarik = lama })
	r := routerPenarikan()
	for _, rute := range [][2]string{{"POST", "/inaproc/penarikan"}, {"POST", "/inaproc/penarikan/batal"}, {"GET", "/inaproc/penarikan/aktif"}, {"PUT", "/inaproc/penarikan/pengaturan"}} {
		if kode, _ := kirim(r, rute[0], rute[1], `{}`); kode != http.StatusServiceUnavailable {
			t.Errorf("%s %s: %d, want 503", rute[0], rute[1], kode)
		}
	}
}

func TestPutPengaturanMenolakDanMenyimpan(t *testing.T) {
	pasangKonfigInaproc(t, "http://x", "token-uji")
	m, f := penarikUji(t, sukses(0))
	lama := Penarik
	Penarik = m
	t.Cleanup(func() { Penarik = lama })
	r := routerPenarikan()

	for nama, badan := range map[string]string{
		"interval nol":     `{"aktif":true,"interval_hari":0,"jam_mulai":1,"jam_akhir":5,"kode_klpd":"K10","jumlah_tahun":2,"jeda_detik":2,"dataset":["tender/pengumuman"]}`,
		"dataset per kode": `{"aktif":true,"interval_hari":2,"jam_mulai":1,"jam_akhir":5,"kode_klpd":"K10","jumlah_tahun":2,"jeda_detik":2,"dataset":["ekatalog/penyedia-detail"]}`,
		"json rusak":       `{`,
	} {
		if kode, _ := kirim(r, "PUT", "/inaproc/penarikan/pengaturan", badan); kode != 400 {
			t.Errorf("%s: %d, want 400", nama, kode)
		}
	}
	for _, ex := range f.Execs() {
		if strings.Contains(ex.Query, "inaproc_penarikan_pengaturan") {
			t.Fatalf("pengaturan ngawur tidak boleh disimpan: %s", ex.Query)
		}
	}
	kode, res := kirim(r, "PUT", "/inaproc/penarikan/pengaturan",
		`{"aktif":true,"interval_hari":2,"jam_mulai":1,"jam_akhir":5,"kode_klpd":" K10 ","jumlah_tahun":2,"jeda_detik":2,"dataset":["tender/pengumuman"]}`)
	if kode != 200 {
		t.Fatalf("simpan: %d %v", kode, res)
	}
	var disimpan bool
	for _, ex := range f.Execs() {
		if strings.Contains(ex.Query, "UPDATE inaproc_penarikan_pengaturan") && ex.Args[4].Value == "K10" && ex.Args[7].Value == `["tender/pengumuman"]` && ex.Args[8].Value == "admin1" {
			disimpan = true
		}
	}
	if !disimpan {
		t.Error("pengaturan tidak tersimpan dengan benar")
	}
}
