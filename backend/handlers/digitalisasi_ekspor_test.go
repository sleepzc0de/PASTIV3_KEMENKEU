package handlers

import (
	"bytes"
	"context"
	"database/sql/driver"
	"fmt"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"

	"pasti-v3-backend/digitalisasi"
	"pasti-v3-backend/internal/fakesql"
	"pasti-v3-backend/laporan"
)

// Tes unduhan data Digitalisasi Aset dengan database palsu: isi berkas, kolom pribadi, penyaring yang diteruskan sebagai parameter, dan galat.

func eksporFake(p *fakesql.DB, baris int, total int64) {
	p.OnQuery = func(ctx context.Context, q string, args []driver.NamedValue) ([]string, [][]driver.Value, error) {
		switch {
		case strings.HasPrefix(q, "SELECT COUNT_BIG(*)"):
			return []string{"n"}, [][]driver.Value{{total}}, nil
		case strings.HasPrefix(q, "SELECT ["):
			cols := selectedColumns(q)
			tipe := make([]string, len(cols))
			for i, c := range cols {
				if strings.HasPrefix(c, "Luas_") || strings.HasPrefix(c, "Nilai_") || c == "Latitude" || c == "Longitude" {
					tipe[i] = "DECIMAL" // driver SQL Server mengirim DECIMAL sebagai []byte
				}
			}
			p.TipeKolom = tipe
			var out [][]driver.Value
			for i := 0; i < baris; i++ {
				out = append(out, rowFor(cols))
			}
			return cols, out, nil
		}
		return nil, nil, fmt.Errorf("query tak terduga: %s", q)
	}
}

func ambilBerkas(r *gin.Engine, url string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", url, nil))
	return w
}

func TestLabelKolomDigitalisasi(t *testing.T) {
	for kolom, want := range map[string]string{
		"Kode_Satker":            "Kode Satker",
		"Uraian_tanah":           "Uraian tanah",
		"KelurahanDesa_tanah":    "Kelurahan/Desa tanah",
		"KabKota_RN":             "Kab/Kota Rumah Negara",
		"RTRW_bangunan":          "RT/RW bangunan",
		"id_aset_tanah":          "ID aset tanah",
		"kode_register_tanah":    "Kode register tanah",
		"Jumlah_KDJ":             "Jumlah KDJ",
		"Luas_Tanah":             "Luas Tanah (m²)",
		"Nilai_Bangunan":         "Nilai Bangunan (Rp)",
		"Latitude":               "Lintang",
		"Longitude":              "Bujur",
		"synced_at":              "Disinkronkan",
		"Kamar_tipe_E":           "Kamar tipe E",
		"Nama_Penghuni":          "Nama Penghuni",
		"NUP_bangunan":           "NUP bangunan",
		"Status_Gedung_Kantor":   "Status Gedung Kantor",
		"Kecamatan_Satker":       "Kecamatan Satker",
		"Foto":                   "Foto",
		"No_aset_tanah":          "No aset tanah",
		"Provinsi_mess":          "Provinsi mess",
		"Kondisi_KDJ":            "Kondisi KDJ",
		"Kode_UE1":               "Kode UE1",
		"Status_Penghuni":        "Status Penghuni",
		"Kode_Register_bangunan": "Kode Register bangunan",
	} {
		if got := labelKolomDigitalisasi(kolom); got != want {
			t.Errorf("labelKolomDigitalisasi(%q) = %q, want %q", kolom, got, want)
		}
	}
	// Judul kolom satu dataset tidak boleh kembar (satu judul = satu kolom di berkas).
	for _, ds := range digitalisasi.Datasets {
		ada := map[string]string{}
		nama, _ := dgKolomEkspor(ds, true, false)
		for _, n := range nama {
			l := labelKolomDigitalisasi(n)
			if lain, kembar := ada[l]; kembar {
				t.Errorf("%s: kolom %q dan %q berjudul sama %q", ds.Key, lain, n, l)
			}
			ada[l] = n
		}
	}
}

// Kolom PDF harus kolom yang benar-benar ada di tabelnya, bukan kolom pribadi, dan ringkas (muat satu halaman).
func TestKolomPDFAdaDiTabelDanBukanPribadi(t *testing.T) {
	for _, ds := range digitalisasi.Datasets {
		kolom := dgKolomPDF(ds)
		if len(kolom) < 5 || len(kolom) > 8 {
			t.Errorf("%s: %d kolom PDF %v, want 5-8", ds.Key, len(kolom), kolom)
		}
		for _, n := range kolom {
			c, ada := ds.Column(n)
			if !ada || c.Sensitive {
				t.Errorf("%s: kolom PDF %q tidak ada di tabel atau kolom pribadi", ds.Key, n)
			}
		}
	}
}

func TestNamaBerkasDigitalisasi(t *testing.T) {
	now := time.Date(2026, 10, 6, 4, 30, 0, 0, time.UTC) // 11:30 WIB
	if got := namaBerkasDigitalisasi("gedung_kantor_utama", "xlsx", false, now); got != "digitalisasi-gedung-kantor-utama_20261006-1130.xlsx" {
		t.Errorf("nama = %q", got)
	}
	if got := namaBerkasDigitalisasi("tanah", "csv", true, now); got != "digitalisasi-tanah-disaring_20261006-1130.csv" {
		t.Errorf("nama disaring = %q", got)
	}
}

// Excel dan CSV memuat semua kolom (bukan hanya kolom tabel daftar) ditambah waktu sinkronisasi, dan kolom pribadi hanya untuk admin.
func TestEksporCSVSemuaKolomDanKolomPribadi(t *testing.T) {
	p, _ := setupDG(t)
	eksporFake(p, 2, 2)

	for _, tc := range []struct {
		role  string
		admin bool
	}{{"user", false}, {"admin", true}} {
		w := ambilBerkas(dgRouter(tc.role), "/dg/ekspor/rumah_negara?format=csv")
		if w.Code != 200 {
			t.Fatalf("%s: status %d: %s", tc.role, w.Code, w.Body)
		}
		if ct := w.Header().Get("Content-Type"); ct != "text/csv; charset=utf-8" {
			t.Errorf("Content-Type = %q", ct)
		}
		if cd := w.Header().Get("Content-Disposition"); !regexp.MustCompile(`^attachment; filename="digitalisasi-rumah-negara_\d{8}-\d{4}\.csv"$`).MatchString(cd) {
			t.Errorf("Content-Disposition = %q", cd)
		}
		sel := lastQuery(t, p, "SELECT [").Query
		if has := strings.Contains(sel, "Nama_Penghuni"); has != tc.admin {
			t.Errorf("%s: Nama_Penghuni di SELECT = %v, want %v", tc.role, has, tc.admin)
		}
		for _, wajib := range []string{"[Kode_Register_RN]", "[RTRW_RN]", "[Latitude]", "[synced_at]"} { // kolom yang tidak tampil di daftar
			if !strings.Contains(sel, wajib) {
				t.Errorf("%s: SELECT tidak memuat %s: %s", tc.role, wajib, sel)
			}
		}
		if strings.Contains(sel, "[id]") || !strings.Contains(sel, "ORDER BY [Kode_Satker], id") {
			t.Errorf("%s: kolom id tidak diekspor dan urutannya per satker: %s", tc.role, sel)
		}

		isi := strings.TrimPrefix(w.Body.String(), "\xEF\xBB\xBF") // BOM supaya Excel membaca UTF-8
		if !strings.HasPrefix(w.Body.String(), "\xEF\xBB\xBF") {
			t.Errorf("%s: CSV tanpa BOM", tc.role)
		}
		baris := strings.Split(strings.TrimRight(isi, "\r\n"), "\r\n")
		if len(baris) != 3 { // judul + 2 baris data
			t.Fatalf("%s: %d baris CSV, want 3: %q", tc.role, len(baris), isi)
		}
		judul := baris[0]
		if has := strings.Contains(judul, "Nama Penghuni"); has != tc.admin {
			t.Errorf("%s: judul memuat Nama Penghuni = %v, want %v (%s)", tc.role, has, tc.admin, judul)
		}
		// Singkatan dan uraian UE1 dari referensi (subquery ke ref_ue1) tepat setelah kode UE1, bukan kolom tabel sehingga tidak ikut di PDF ringkas.
		if !strings.Contains(judul, "Kode UE1;Singkatan UE1;Uraian UE1;") {
			t.Errorf("%s: judul tidak memuat Kode UE1;Singkatan UE1;Uraian UE1 berurutan: %s", tc.role, judul)
		}
		if !strings.Contains(sel, "FROM ref_ue1 r WHERE r.kode = [Kode_UE1]) AS [Singkatan_UE1]") || !strings.Contains(sel, "SELECT TOP (1) r.nama FROM ref_ue1 r") {
			t.Errorf("%s: SELECT tidak membaca singkatan dan uraian UE1 dari ref_ue1: %s", tc.role, sel)
		}
		for _, mau := range []string{"Luas Rumah Negara (m²)", "Lintang", "Bujur", "Disinkronkan (WIB)"} {
			if !strings.Contains(judul, mau) {
				t.Errorf("%s: judul tidak memuat %q: %s", tc.role, mau, judul)
			}
		}
		// Pemisah titik koma; desimal ditulis apa adanya (bukan notasi ilmiah), waktu sebagai tanggal-jam.
		if !strings.Contains(baris[1], "123.5") || !strings.Contains(baris[1], "-6.2") || !strings.Contains(baris[1], "2026-10-03 08:02:03") || strings.Count(judul, ";") != strings.Count(baris[1], ";") {
			t.Errorf("%s: baris data = %q", tc.role, baris[1])
		}
	}
}

func TestEksporXLSXAngkaDanFormatKoordinat(t *testing.T) {
	p, _ := setupDG(t)
	eksporFake(p, 3, 3)

	w := ambilBerkas(dgRouter("user"), "/dg/ekspor/tanah")
	if w.Code != 200 {
		t.Fatalf("status %d: %s", w.Code, w.Body)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" {
		t.Errorf("Content-Type = %q (format bawaan harus xlsx)", ct)
	}
	f, err := excelize.OpenReader(bytes.NewReader(w.Body.Bytes()))
	if err != nil {
		t.Fatalf("berkas bukan xlsx yang sah: %v", err)
	}
	defer f.Close()
	if f.GetSheetName(0) != "Tanah" {
		t.Errorf("nama sheet = %q", f.GetSheetName(0))
	}
	rows, err := f.GetRows("Tanah")
	if err != nil || len(rows) != 4 {
		t.Fatalf("baris = %d, err %v; want 4 (judul + 3 data)", len(rows), err)
	}
	kolom := map[string]int{}
	for i, j := range rows[0] {
		kolom[j] = i
	}
	for _, mau := range []string{"Luas Tanah (m²)", "Nilai Tanah (Rp)", "Lintang", "Bujur", "Disinkronkan (WIB)", "Kode register tanah", "ID aset tanah"} {
		if _, ada := kolom[mau]; !ada {
			t.Errorf("judul %q tidak ada: %v", mau, rows[0])
		}
	}
	sel := lastQuery(t, p, "SELECT [").Query
	if strings.Contains(sel, "Nama_Penghuni") {
		t.Errorf("tanah tidak punya kolom pribadi: %s", sel)
	}
	cell := func(judul string) string {
		n, _ := excelize.CoordinatesToCellName(kolom[judul]+1, 2)
		return n
	}
	// Koordinat tersimpan sebagai bilangan dengan 7 desimal; luas dengan format ribuan dua desimal.
	lat := cell("Lintang")
	if v, _ := f.GetCellValue("Tanah", lat, excelize.Options{RawCellValue: true}); v != "-6.2" {
		t.Errorf("Lintang mentah = %q, want -6.2 (bilangan, bukan teks)", v)
	}
	if v, _ := f.GetCellValue("Tanah", lat); v != "-6.2000000" {
		t.Errorf("Lintang tampil = %q, want -6.2000000 (tujuh desimal)", v)
	}
	if v, _ := f.GetCellValue("Tanah", cell("Luas Tanah (m²)")); v != "123.50" {
		t.Errorf("Luas tampil = %q, want 123.50", v)
	}
	if v, _ := f.GetCellValue("Tanah", cell("Disinkronkan (WIB)")); v != "2026-10-03 08:02:03" {
		t.Errorf("Disinkronkan = %q", v)
	}
}

func TestEksporPDFMemakaiKolomTabelDaftar(t *testing.T) {
	p, _ := setupDG(t)
	eksporFake(p, 2, 2)

	w := ambilBerkas(dgRouter("user"), "/dg/ekspor/tanah?format=pdf&provinsi=Jawa+Barat")
	if w.Code != 200 {
		t.Fatalf("status %d: %s", w.Code, w.Body)
	}
	if !strings.HasPrefix(w.Body.String(), "%PDF-") || w.Header().Get("Content-Type") != "application/pdf" {
		t.Errorf("bukan PDF: %q %q", w.Header().Get("Content-Type"), w.Body.String()[:8])
	}
	sel := lastQuery(t, p, "SELECT [").Query
	want := "SELECT [Nama_Satker], [Uraian_tanah], [KabKota_tanah], [Provinsi_tanah], [Luas_Tanah], [Kondisi_Tanah], [Nilai_Tanah] FROM [DIGITALISASI_TANAH]"
	if !strings.HasPrefix(sel, want) {
		t.Errorf("PDF hanya memuat kolom ringkasan seperti tabel di layar:\n got  %s\n want %s...", sel, want)
	}
	if cd := w.Header().Get("Content-Disposition"); !strings.Contains(cd, "digitalisasi-tanah-disaring_") || !strings.HasSuffix(cd, `.pdf"`) {
		t.Errorf("Content-Disposition = %q", cd)
	}
}

// Penyaring daftar diteruskan ke COUNT dan SELECT berkas sebagai parameter yang sama (masukan pengguna tidak masuk teks SQL).
func TestEksporMeneruskanPenyaringSebagaiParameter(t *testing.T) {
	p, _ := setupDG(t)
	eksporFake(p, 1, 1)

	w := ambilBerkas(dgRouter("user"), "/dg/ekspor/tanah?format=csv&q=100%25_%5Bx&ue1=01504&provinsi=Jawa+Barat&kondisi=Baik&tanpa_koordinat=1")
	if w.Code != 200 {
		t.Fatalf("status %d: %s", w.Code, w.Body)
	}
	for _, awal := range []string{"SELECT COUNT_BIG(*)", "SELECT ["} {
		st := lastQuery(t, p, awal)
		if strings.Contains(st.Query, "100%") || strings.Contains(st.Query, "01504") || strings.Contains(st.Query, "Jawa Barat") {
			t.Fatalf("masukan pengguna tidak boleh ada di teks SQL: %s", st.Query)
		}
		if !strings.Contains(st.Query, "ESCAPE '\\'") || !strings.Contains(st.Query, "[Latitude] IS NULL") {
			t.Fatalf("bentuk SQL tak terduga: %s", st.Query)
		}
		want := []interface{}{`%100\%\_\[x%`, "01504", "Jawa Barat", "Baik"}
		if len(st.Args) != len(want) {
			t.Fatalf("args = %v", st.Args)
		}
		for i, w := range want {
			if st.Args[i].Value != w {
				t.Errorf("%s: arg %d = %#v, want %#v", awal, i+1, st.Args[i].Value, w)
			}
		}
	}
	if cd := w.Header().Get("Content-Disposition"); !strings.Contains(cd, "-disaring_") {
		t.Errorf("berkas hasil penyaringan harus bertanda disaring: %q", cd)
	}
	// Penyaring yang tidak berlaku untuk dataset ini (jenis_satker hanya untuk satker) diabaikan, sama seperti daftar.
	if sel := lastQuery(t, p, "SELECT [").Query; strings.Contains(sel, "Jenis_Satker") {
		t.Errorf("jenis_satker tidak berlaku untuk tanah: %s", sel)
	}
}

func TestEksporPenyaringSatker(t *testing.T) {
	p, _ := setupDG(t)
	eksporFake(p, 1, 1)
	w := ambilBerkas(dgRouter("user"), "/dg/ekspor/satker?format=csv&jenis_satker=INDUK+SATKER")
	if w.Code != 200 {
		t.Fatalf("status %d: %s", w.Code, w.Body)
	}
	st := lastQuery(t, p, "SELECT COUNT_BIG(*)")
	if !strings.Contains(st.Query, "[Jenis_Satker] = @p1") || len(st.Args) != 1 || st.Args[0].Value != "INDUK SATKER" {
		t.Errorf("COUNT satker = %s %v", st.Query, st.Args)
	}
}

func TestEksporMenolakPermintaanSalah(t *testing.T) {
	p, _ := setupDG(t)
	eksporFake(p, 1, 1)
	r := dgRouter("user")
	for nama, tc := range map[string]struct {
		url  string
		code int
	}{
		"format ngawur":      {"/dg/ekspor/tanah?format=docx", 400},
		"pemisah ngawur":     {"/dg/ekspor/tanah?format=csv&pemisah=spasi", 400},
		"dataset tak ada":    {"/dg/ekspor/tidak-ada", 404},
		"kata kunci panjang": {"/dg/ekspor/tanah?q=" + strings.Repeat("a", 101), 400},
	} {
		if w := ambilBerkas(r, tc.url); w.Code != tc.code {
			t.Errorf("%s: status %d, want %d: %s", nama, w.Code, tc.code, w.Body)
		}
	}
	// Tidak ada query yang dijalankan untuk permintaan yang ditolak.
	if n := len(p.Queries()); n != 0 {
		t.Errorf("permintaan yang ditolak menjalankan %d query", n)
	}
}

// Excel dibatasi satu sheet; ditolak sebelum data dibaca, dan CSV tetap boleh.
func TestEksporXLSXMelebihiBatasSheetDitolak(t *testing.T) {
	p, _ := setupDG(t)
	eksporFake(p, 1, int64(laporan.MaksBarisExcel)+1)
	w := ambilBerkas(dgRouter("user"), "/dg/ekspor/tanah?format=xlsx")
	if w.Code != 422 || !strings.Contains(w.Body.String(), "CSV") {
		t.Errorf("status %d: %s", w.Code, w.Body)
	}
	for _, q := range p.Queries() {
		if strings.HasPrefix(q.Query, "SELECT [") {
			t.Errorf("data tidak boleh dibaca bila ditolak: %s", q.Query)
		}
	}
	if w := ambilBerkas(dgRouter("user"), "/dg/ekspor/tanah?format=csv"); w.Code != 200 {
		t.Errorf("CSV melebihi batas sheet tetap boleh, status %d", w.Code)
	}
}

func TestEksporGalatDatabaseTidakBocor(t *testing.T) {
	p, _ := setupDG(t)
	p.OnQuery = func(context.Context, string, []driver.NamedValue) ([]string, [][]driver.Value, error) {
		return nil, nil, fmt.Errorf("dial tcp 10.1.2.3:1433: connection refused")
	}
	w := ambilBerkas(dgRouter("user"), "/dg/ekspor/tanah?format=csv")
	if w.Code != 500 || strings.Contains(w.Body.String(), "10.1.2.3") {
		t.Errorf("status %d: %s (rincian galat internal tidak boleh bocor)", w.Code, w.Body)
	}
}
