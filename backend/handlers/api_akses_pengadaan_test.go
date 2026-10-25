package handlers

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"pasti-v3-backend/apiakses"
)

// Tes API data pengadaan: (1) katalog API selaras dengan katalog Inaproc dan aturan pembatasan per satker; (2) alur HTTP lewat gerbang kunci API asli dengan database palsu.

// Dataset Inaproc yang SENGAJA tidak ada di API Akses Data beserta alasannya. Setiap dataset di DaftarDataset harus ada di katalog API atau di sini: dataset baru yang belum
// diputuskan membuat tes gagal, bukan terbuka diam-diam atau terlupa.
var datasetInaprocTidakDiAPI = map[string]string{
	"inaproc_program_master":               "tidak dapat dibatasi per satker (hanya kd_satker internal)",
	"inaproc_non_tender_ekontrak":          "tidak dapat dibatasi per satker (hanya alamat satker)",
	"inaproc_ekatalog6_epurchasing_produk": "tidak dapat dibatasi per satker (kode satker V6 tidak terverifikasi dan tanpa kode RUP)",
	"inaproc_ekatalog_komoditas":           "rujukan katalog nasional, bukan data satker",
	"inaproc_ekatalog_penyedia":            "rujukan katalog nasional berisi kontak penyedia (alamat, email, telepon), bukan data satker",
	"inaproc_ekatalog_distributor":         "rujukan katalog nasional berisi kontak distributor (alamat, email, telepon), bukan data satker",
	"inaproc_ekatalog6_kategori":           "rujukan katalog nasional, bukan data satker",
	"inaproc_ekatalog6_penyedia":           "rujukan katalog nasional berisi kontak penyedia (alamat, email, telepon, NIB), bukan data satker",
	"inaproc_ekatalog6_produk_penyedia":    "rujukan katalog nasional, bukan data satker",
	"inaproc_tender_ekontrak":              "kolom yang terbuka identik dengan Kontrak Tender; pembedanya (riwayat BAP/BAST, SPMK/SPP, penilaian kinerja) berupa JSON bebas yang ditutup",
}

func TestKatalogAPISelarasDenganKatalogInaproc(t *testing.T) {
	dibuka := map[string]apiakses.DatasetPengadaan{}
	for _, d := range apiakses.KatalogPengadaan() {
		dibuka[d.Tabel()] = d
	}
	for _, dp := range DaftarDataset {
		d, ada := dibuka[dp.Tabel]
		alasan, ditutup := datasetInaprocTidakDiAPI[dp.Tabel]
		switch {
		case ada && ditutup:
			t.Errorf("%s: ada di katalog API dan di daftar tidak dibuka (%s) sekaligus", dp.Tabel, alasan)
		case !ada && !ditutup:
			t.Errorf("%s (%s): belum diputuskan untuk API Akses Data; tambahkan ke apiakses/katalog_pengadaan.go atau ke daftar tidak dibuka dengan alasannya", dp.Tabel, dp.ID)
		}
		if !ada {
			continue
		}
		if !DapatDibatasi(dp.Tabel) {
			t.Errorf("%s: dataset API harus dapat dibatasi per satker (pembatasan UE1 klien)", dp.Tabel)
		}
		if d.KolomKunci() != dp.KolomKunci {
			t.Errorf("%s: kolom kunci API %q != kolom kunci katalog Inaproc %q", dp.Tabel, d.KolomKunci(), dp.KolomKunci)
		}
		if d.KolomKLPD() != dp.KolomKLPD || d.KolomTahun() != dp.KolomTahun {
			t.Errorf("%s: penyaring API (%q, %q) != penyaring katalog Inaproc (%q, %q)", dp.Tabel, d.KolomKLPD(), d.KolomTahun(), dp.KolomKLPD, dp.KolomTahun)
		}
	}
	for tabel := range datasetInaprocTidakDiAPI {
		ada := false
		for _, dp := range DaftarDataset {
			ada = ada || dp.Tabel == tabel
		}
		if !ada {
			t.Errorf("daftar tidak dibuka memuat tabel yang tidak ada di katalog Inaproc: %s", tabel)
		}
	}
	if len(apiakses.KatalogPengadaan())+len(datasetInaprocTidakDiAPI) != len(DaftarDataset) {
		t.Errorf("katalog API (%d) + tidak dibuka (%d) != katalog Inaproc (%d)", len(apiakses.KatalogPengadaan()), len(datasetInaprocTidakDiAPI), len(DaftarDataset))
	}
}

// ---------------------------------------------------------------- alur HTTP

type sumberKunciUji struct {
	kred map[string]*apiakses.Kredensial
}

func (s *sumberKunciUji) CariKredensial(_ context.Context, prefix string) (*apiakses.Kredensial, error) {
	if k, ok := s.kred[prefix]; ok {
		salin := *k
		return &salin, nil
	}
	return nil, nil
}
func (s *sumberKunciUji) MuatPengaturan(context.Context) (apiakses.Pengaturan, error) {
	return apiakses.PengaturanBawaan(), nil
}

// pasangKlienPengadaan menyiapkan klien dengan kunci sah, scope, dan pembatasan UE1 yang diberikan, serta router klien di belakang gerbang asli.
func pasangKlienPengadaan(t *testing.T, dataset []string, ue1 []string) (*gin.Engine, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	kunci, prefix, err := apiakses.BuatKunci()
	if err != nil {
		t.Fatal(err)
	}
	s := &sumberKunciUji{kred: map[string]*apiakses.Kredensial{}}
	l := apiakses.NewLayanan(nil, s, nil, "pepper-uji-pengadaan")
	s.kred[prefix] = &apiakses.Kredensial{
		KunciID: 1, KlienID: 7, Prefix: prefix, Hash: l.HashKunci(kunci), KlienKode: "KL-UJIPENGA", KlienNama: "Aplikasi Uji",
		KlienStatus: apiakses.StatusDisetujui, Dataset: dataset, UE1: ue1,
	}
	t.Cleanup(GunakanApiAkses(l))

	r := gin.New()
	_ = r.SetTrustedProxies(nil)
	g := r.Group(apiakses.AwalanRute)
	g.GET("/katalog", ApiAksesGerbang(false), GetAksesKatalog)
	g.GET("/pengadaan/:dataset", ApiAksesGerbang(true), GetPengadaanDaftar)
	g.GET("/pengadaan/:dataset/:id", ApiAksesGerbang(true), GetPengadaanDetail)
	return r, kunci
}

func panggilKlien(r *gin.Engine, kunci, url string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, url, nil)
	if kunci != "" {
		req.Header.Set("Authorization", "Bearer "+kunci)
	}
	r.ServeHTTP(w, req)
	return w
}

func isiJSON(t *testing.T, w *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var m map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &m); err != nil {
		t.Fatalf("badan bukan JSON: %v\n%s", err, w.Body.String())
	}
	return m
}

// dbPengadaanUji: database palsu yang menjawab data untuk dataset d (kolom menurut katalog), dan mencatat query.
func dbPengadaanUji(t *testing.T, d apiakses.DatasetPengadaan, baris int) *[]string {
	t.Helper()
	f := pasangDBPalsu(t)
	var sqlTercatat []string
	f.OnQuery = func(_ context.Context, q string, _ []driver.NamedValue) ([]string, [][]driver.Value, error) {
		sqlTercatat = append(sqlTercatat, strings.Join(strings.Fields(q), " "))
		kol := []string{d.KolomKunci()}
		for _, k := range d.Kolom {
			kol = append(kol, k.Nama)
		}
		kol = append(kol, "updated_at")
		switch {
		case strings.Contains(q, "COUNT_BIG"):
			return []string{"n"}, [][]driver.Value{{int64(41)}}, nil
		case strings.Contains(q, "MAX([updated_at])"):
			return []string{"t"}, [][]driver.Value{{time.Date(2026, 10, 7, 3, 0, 0, 0, time.UTC)}}, nil
		}
		var out [][]driver.Value
		for i := 0; i < baris; i++ {
			b := []driver.Value{"kunci" + string(rune('a'+i))}
			for _, k := range d.Kolom {
				switch k.Nama {
				case "kd_tender":
					b = append(b, "T-100")
				case "pagu":
					b = append(b, []byte("2500000.00"))
				default:
					b = append(b, nil)
				}
			}
			out = append(out, append(b, time.Date(2026, 10, 7, 3, 0, 0, 0, time.UTC)))
		}
		return kol, out, nil
	}
	return &sqlTercatat
}

func TestAPIPengadaanDaftarLewatGerbang(t *testing.T) {
	d, _ := apiakses.CariDatasetPengadaan("pengadaan.tender_pengumuman")
	r, kunci := pasangKlienPengadaan(t, []string{"pengadaan.tender_pengumuman"}, []string{"01504"})
	sqlTercatat := dbPengadaanUji(t, d, 3)

	w := panggilKlien(r, kunci, apiakses.AwalanRute+"/pengadaan/tender_pengumuman?per_halaman=2&tahun=2025&kode_klpd=K10&hitung=1&q=laptop")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", w.Code, w.Body.String())
	}
	data := isiJSON(t, w)["data"].(map[string]interface{})
	if data["dataset"] != "pengadaan.tender_pengumuman" || data["jumlah"] != float64(2) || data["ada_berikutnya"] != true || data["total"] != float64(41) {
		t.Errorf("data = %v", data)
	}
	baris := data["baris"].([]interface{})
	b := baris[0].(map[string]interface{})
	if b["id"] != "kuncia" || b["kd_tender"] != "T-100" || b["pagu"] != float64(2500000) {
		t.Errorf("baris = %v", b)
	}
	if data["kursor_berikutnya"] != "kuncib" {
		t.Errorf("kursor_berikutnya = %v, want id baris terakhir pada halaman", data["kursor_berikutnya"])
	}
	if data["disinkronkan_terakhir"] == nil {
		t.Error("disinkronkan_terakhir harus terisi")
	}
	for _, larang := range []string{"nip_ppk", "nama_ppk", "nip_pokja", "nama_pokja", "extra_json", "row_key"} {
		if _, ada := b[larang]; ada || strings.Contains(w.Body.String(), `"`+larang+`"`) {
			t.Errorf("respons memuat %q", larang)
		}
	}
	// pembatasan UE1 klien ikut aturan per satker yang sama dengan halaman Pengadaan Terpadu (satker data aset yang berawalan UE1)
	var data0 string
	for _, q := range *sqlTercatat {
		if strings.HasPrefix(q, "SELECT TOP (3)") {
			data0 = q
		}
	}
	for _, perlu := range []string{"DIGITALISASI_SATKER", "N'01504'", "[kd_klpd] = @p", "[tahun_anggaran] = @p", "FROM [inaproc_tender_pengumuman]"} {
		if !strings.Contains(data0, perlu) {
			t.Errorf("sql data tidak memuat %q: %s", perlu, data0)
		}
	}
	if strings.Contains(data0, "K10") || strings.Contains(data0, "laptop") {
		t.Errorf("masukan klien tertulis di SQL: %s", data0)
	}
}

func TestAPIPengadaanDetailLewatGerbang(t *testing.T) {
	d, _ := apiakses.CariDatasetPengadaan("pengadaan.tender_pengumuman")
	r, kunci := pasangKlienPengadaan(t, []string{"pengadaan.tender_pengumuman"}, nil)

	dbPengadaanUji(t, d, 1)
	w := panggilKlien(r, kunci, apiakses.AwalanRute+"/pengadaan/tender_pengumuman/abc123")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", w.Code, w.Body.String())
	}
	b := isiJSON(t, w)["data"].(map[string]interface{})["baris"].(map[string]interface{})
	if b["kd_tender"] != "T-100" {
		t.Errorf("baris = %v", b)
	}

	dbPengadaanUji(t, d, 0)
	if w := panggilKlien(r, kunci, apiakses.AwalanRute+"/pengadaan/tender_pengumuman/abc123"); w.Code != http.StatusNotFound {
		t.Errorf("baris tidak ada: status = %d", w.Code)
	}
	if w := panggilKlien(r, kunci, apiakses.AwalanRute+"/pengadaan/tender_pengumuman/a%27%20OR%201=1"); w.Code != http.StatusNotFound {
		t.Errorf("id bentuk salah: status = %d, want 404 tanpa menyentuh database", w.Code)
	}
}

func TestAPIPengadaanScopeDanMasukanBuruk(t *testing.T) {
	r, kunci := pasangKlienPengadaan(t, []string{"pengadaan.tender_pengumuman", "aset.tanah"}, nil)
	f := pasangDBPalsu(t)
	f.OnQuery = func(context.Context, string, []driver.NamedValue) ([]string, [][]driver.Value, error) {
		t.Error("masukan buruk tidak boleh sampai ke database")
		return nil, nil, nil
	}
	base := apiakses.AwalanRute + "/pengadaan/"
	cases := []struct {
		nama, url string
		want      int
		code      string
	}{
		{"tanpa kunci", base + "tender_pengumuman", http.StatusUnauthorized, apiakses.KodeKunciTidakValid},
		{"dataset di luar scope", base + "rup_paket_penyedia", http.StatusForbidden, apiakses.KodeDatasetDitolak},
		{"dataset di luar scope (detail)", base + "rup_paket_penyedia/x1", http.StatusForbidden, apiakses.KodeDatasetDitolak},
		{"dataset tidak ada", base + "tidak_ada", http.StatusNotFound, ""},
		{"nama tabel langsung", base + "inaproc_tender_pengumuman", http.StatusNotFound, ""},
		{"dataset yang tidak dibuka", base + "program_master", http.StatusNotFound, ""},
		{"dataset aset lewat rute pengadaan", base + "tanah", http.StatusNotFound, ""},
		{"provinsi", base + "tender_pengumuman?provinsi=Jawa", http.StatusBadRequest, ""},
		{"kab_kota", base + "tender_pengumuman?kab_kota=Bogor", http.StatusBadRequest, ""},
		{"tahun salah", base + "tender_pengumuman?tahun=abcd", http.StatusBadRequest, ""},
		{"klpd injeksi", base + "tender_pengumuman?kode_klpd=K10%27%20OR%201%3D1", http.StatusBadRequest, ""},
		{"satker 15 digit", base + "tender_pengumuman?kode_satker=015040199119091", http.StatusBadRequest, ""},
		{"q pendek", base + "tender_pengumuman?q=ab", http.StatusBadRequest, ""},
		{"per_halaman nol", base + "tender_pengumuman?per_halaman=0", http.StatusBadRequest, ""},
		{"per_halaman huruf", base + "tender_pengumuman?per_halaman=x", http.StatusBadRequest, ""},
		{"kursor injeksi", base + "tender_pengumuman?kursor=a%27--", http.StatusBadRequest, ""},
	}
	for _, c := range cases {
		k := kunci
		if c.nama == "tanpa kunci" {
			k = ""
		}
		w := panggilKlien(r, k, c.url)
		if w.Code != c.want {
			t.Errorf("%s: status = %d, want %d (%s)", c.nama, w.Code, c.want, w.Body.String())
			continue
		}
		if c.code != "" && isiJSON(t, w)["code"] != c.code {
			t.Errorf("%s: code = %v, want %s", c.nama, isiJSON(t, w)["code"], c.code)
		}
	}
	// dataset tanpa kolom tahun menolak penyaring tahun (bukan mengabaikannya)
	r2, k2 := pasangKlienPengadaan(t, []string{"pengadaan.ekatalog_instansi_satker"}, nil)
	if w := panggilKlien(r2, k2, base+"ekatalog_instansi_satker?tahun=2025"); w.Code != http.StatusBadRequest {
		t.Errorf("tahun pada dataset tanpa tahun: status = %d, want 400", w.Code)
	}
}

func TestAPIKatalogMemuatPengadaanSesuaiScope(t *testing.T) {
	r, kunci := pasangKlienPengadaan(t, []string{"pengadaan.tender_kontrak", "pengadaan.ekatalog_instansi_satker", "aset.tanah"}, nil)
	w := panggilKlien(r, kunci, apiakses.AwalanRute+"/katalog")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", w.Code, w.Body.String())
	}
	data := isiJSON(t, w)["data"].(map[string]interface{})
	items := data["dataset"].([]interface{})
	kode := map[string]map[string]interface{}{}
	for _, it := range items {
		m := it.(map[string]interface{})
		kode[m["kode"].(string)] = m
	}
	if len(kode) != 3 || kode["aset.tanah"] == nil || kode["pengadaan.tender_kontrak"] == nil || kode["pengadaan.ekatalog_instansi_satker"] == nil {
		t.Fatalf("katalog = %v", kode)
	}
	if kode["pengadaan.tender_pengumuman"] != nil {
		t.Error("dataset di luar scope tidak boleh tampil di katalog")
	}
	k := kode["pengadaan.tender_kontrak"]
	if k["kelompok"] != "pengadaan" || k["subkelompok"] != "Tender" || k["alamat"] != apiakses.AwalanRute+"/pengadaan/tender_kontrak" || kode["aset.tanah"]["kelompok"] != "aset" {
		t.Errorf("item = %v", k)
	}
	namaKolom := map[string]string{}
	for _, c := range k["kolom"].([]interface{}) {
		cm := c.(map[string]interface{})
		namaKolom[cm["nama"].(string)] = cm["tipe"].(string)
	}
	if namaKolom["id"] != "teks" || namaKolom["nilai_kontrak"] != "desimal" || namaKolom["tgl_kontrak"] != "waktu" || namaKolom["disinkronkan_pada"] != "waktu" {
		t.Errorf("kolom = %v", namaKolom)
	}
	for _, larang := range []string{"nip_ppk", "nama_ppk", "npwp_penyedia", "no_rek_bank", "wakil_sah_penyedia", "extra_json"} {
		if _, ada := namaKolom[larang]; ada {
			t.Errorf("katalog menawarkan kolom terlarang %s", larang)
		}
	}
	penyaring := func(m map[string]interface{}) string {
		var s []string
		for _, p := range m["penyaring"].([]interface{}) {
			s = append(s, p.(string))
		}
		return strings.Join(s, ",")
	}
	if got := penyaring(k); got != "q,kode_satker,kode_ue1,kode_klpd,tahun" {
		t.Errorf("penyaring kontrak tender = %s", got)
	}
	if got := penyaring(kode["pengadaan.ekatalog_instansi_satker"]); got != "q,kode_satker,kode_ue1,kode_klpd" {
		t.Errorf("penyaring instansi satker = %s (tanpa tahun)", got)
	}
}
