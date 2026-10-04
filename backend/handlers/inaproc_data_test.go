package handlers

import (
	"bytes"
	"context"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"

	"pasti-v3-backend/internal/fakesql"
)

func routerData() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	g := r.Group("/inaproc", func(c *gin.Context) { c.Set("username", "uji"); c.Next() })
	g.GET("/data/:awalan/:nama", GetInaprocData)
	g.GET("/ekspor/:awalan/:nama", EksporInaprocData)
	return r
}

func ambil(r *gin.Engine, url string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, url, nil))
	return w
}

// dbData: database palsu yang menjawab COUNT, daftar kolom, tahun, dan data dengan isi yang ditetapkan tes.
type dbData struct {
	f         *fakesql.DB
	total     int64
	galatBaca error
}

func pasangDBData(t *testing.T, kolomData []string, baris [][]driver.Value) *dbData {
	t.Helper()
	f := pasangDBPalsu(t)
	d := &dbData{f: f, total: int64(len(baris))}
	f.OnQuery = func(_ context.Context, q string, _ []driver.NamedValue) ([]string, [][]driver.Value, error) {
		switch {
		case strings.Contains(q, "COUNT_BIG"):
			return []string{"n"}, [][]driver.Value{{d.total}}, nil
		case strings.Contains(q, "SELECT TOP 0 *"):
			return []string{"row_key", "kd_klpd", "nama_paket", "pagu", "tahun_anggaran", "extra_json", "synced_at"}, nil, nil
		case strings.Contains(q, "SELECT DISTINCT"):
			return []string{"tahun"}, [][]driver.Value{{"2025"}, {"2024"}}, nil
		}
		if d.galatBaca != nil {
			return nil, nil, d.galatBaca
		}
		// Jumlah kolom hasil mengikuti daftar kolom di SELECT, supaya tes tidak bergantung pada kolom ringkasan tiap dataset.
		n := strings.Count(q[:strings.Index(q, " FROM ")], ",") + 1
		cols := make([]string, n)
		for i := range cols {
			cols[i] = "c" + strconv.Itoa(i)
		}
		out := make([][]driver.Value, len(baris))
		for i, b := range baris {
			out[i] = make([]driver.Value, n)
			copy(out[i], b)
		}
		return cols, out, nil
	}
	return d
}

func kueriData(f *fakesql.DB, mengandung string) (fakesql.Stmt, bool) {
	for _, q := range f.Queries() {
		if strings.HasPrefix(q.Query, "SELECT [") && strings.Contains(q.Query, mengandung) {
			return q, true
		}
	}
	return fakesql.Stmt{}, false
}

func TestWhereMenyusunPenyaringDenganParameter(t *testing.T) {
	d, _ := DatasetByID("tender/pengumuman")
	w, args := d.where(PenyaringData{KodeKLPD: "K10", Tahun: "2025"})
	if w != "[kd_klpd] = @p1 AND [tahun_anggaran] = @p2" || len(args) != 2 || args[0] != "K10" || args[1] != "2025" {
		t.Errorf("where = %q %v", w, args)
	}
	if w, args := d.where(PenyaringData{}); w != "" || args != nil {
		t.Errorf("tanpa penyaring: %q %v", w, args)
	}

	// Pencarian: LIKE di semua kolom ringkasan dengan satu parameter; karakter khusus LIKE jadi literal.
	w, args = d.where(PenyaringData{Tahun: "2025", Cari: "50%_[x]"})
	if !strings.HasPrefix(w, "[tahun_anggaran] = @p1 AND (CAST([") || !strings.Contains(w, "AS NVARCHAR(4000)) LIKE @p2") || strings.Count(w, "@p2") != len(d.KolomRingkas) {
		t.Errorf("where cari = %q", w)
	}
	if args[1] != "%50[%][_][[]x]%" {
		t.Errorf("pola LIKE = %q", args[1])
	}
	// Nilai pengguna tidak pernah masuk ke teks SQL.
	if strings.Contains(w, "50") {
		t.Errorf("nilai pengguna ikut ke teks SQL: %q", w)
	}

	// Dataset tanpa kolom KLPD/tahun mengabaikan penyaring itu.
	penyedia, _ := DatasetByID("ekatalog/penyedia-detail")
	if w, args := penyedia.where(PenyaringData{KodeKLPD: "K10", Tahun: "2025"}); w != "" || args != nil {
		t.Errorf("dataset per kode: %q %v", w, args)
	}
	// V6 memakai nama kolom lain.
	v6, _ := DatasetByID("ekatalog/paket-e-purchasing")
	if w, _ := v6.where(PenyaringData{KodeKLPD: "K10", Tahun: "2025"}); w != "[kode_klpd] = @p1 AND [fiscal_year] = @p2" {
		t.Errorf("v6: %q", w)
	}
}

func TestKutipMelindungiNamaDanKolomKunciAdaDiMigrasi(t *testing.T) {
	if got := kutip("a]b"); got != "[a]]b]" {
		t.Errorf("kutip = %q", got)
	}
	tabel := kolomMigrasi(t)
	for _, d := range DaftarDataset {
		if d.KolomKunci == "" || !tabel[d.Tabel][d.KolomKunci] {
			t.Errorf("%s: kolom kunci %q tidak ada di tabel %s", d.ID, d.KolomKunci, d.Tabel)
		}
	}
}

func TestGetDataMengembalikanHalamanDanTotal(t *testing.T) {
	d, _ := DatasetByID("tender/pengumuman")
	db := pasangDBData(t, d.KolomRingkas, [][]driver.Value{
		append([]driver.Value{"K10-1", "Pengadaan", []byte("1500000.50"), time.Date(2025, 3, 4, 0, 0, 0, 0, time.UTC)}, make([]driver.Value, len(d.KolomRingkas)-4)...),
	})
	db.total = 123
	db.f.TipeKolom = []string{"NVARCHAR", "NVARCHAR", "DECIMAL", "DATETIME2"}

	w := ambil(routerData(), "/inaproc/data/tender/pengumuman?kode_klpd=K10&tahun=2025&halaman=3&per_halaman=20&cari=laptop")
	if w.Code != 200 {
		t.Fatalf("status %d: %s", w.Code, w.Body)
	}
	var resp struct {
		Data struct {
			Dataset string `json:"dataset"`
			Kolom   []struct {
				Nama, Label, Jenis string
			} `json:"kolom"`
			Baris   [][]interface{} `json:"baris"`
			Total   int64           `json:"total"`
			Halaman int             `json:"halaman"`
			Per     int             `json:"per_halaman"`
			Tahun   []string        `json:"tahun_tersedia"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	r := resp.Data
	if r.Dataset != "tender/pengumuman" || r.Total != 123 || r.Halaman != 3 || r.Per != 20 || len(r.Baris) != 1 || len(r.Kolom) != len(d.KolomRingkas) {
		t.Errorf("respons = %+v", r)
	}
	if r.Kolom[0].Label == "" || r.Kolom[2].Jenis != "angka" || r.Kolom[3].Jenis != "tanggal" || r.Kolom[0].Jenis != "teks" {
		t.Errorf("kolom = %+v", r.Kolom[:4])
	}
	// DECIMAL dikirim sebagai bilangan, waktu sebagai teks ISO.
	if r.Baris[0][2] != 1500000.5 || r.Baris[0][3] != "2025-03-04" {
		t.Errorf("baris = %v", r.Baris[0][:4])
	}
	if strings.Join(r.Tahun, ",") != "2025,2024" {
		t.Errorf("tahun = %v", r.Tahun)
	}
	// Halaman 3 x 20 baris = offset 40, dan urutannya stabil menurut kolom kunci.
	q, ada := kueriData(db.f, "OFFSET")
	if !ada || !strings.Contains(q.Query, "ORDER BY [row_key] OFFSET 40 ROWS FETCH NEXT 20 ROWS ONLY") || !strings.Contains(q.Query, "FROM [inaproc_tender_pengumuman]") {
		t.Errorf("query = %q", q.Query)
	}
	if len(q.Args) != 3 || q.Args[0].Value != "K10" || q.Args[1].Value != "2025" || q.Args[2].Value != "%laptop%" {
		t.Errorf("args = %+v", q.Args)
	}
}

func TestGetDataMenolakDatasetAtauPenyaringNgawur(t *testing.T) {
	pasangDBData(t, nil, nil)
	r := routerData()
	if w := ambil(r, "/inaproc/data/tender/tidak-ada"); w.Code != 404 {
		t.Errorf("dataset tak dikenal: %d", w.Code)
	}
	if w := ambil(r, "/inaproc/data/tender/pengumuman?tahun="+strings.Repeat("9", 101)); w.Code != 400 {
		t.Errorf("penyaring terlalu panjang: %d", w.Code)
	}
	// per_halaman di luar rentang kembali ke bawaan, bukan menarik seluruh tabel.
	db := pasangDBData(t, []string{"a"}, nil)
	if w := ambil(r, "/inaproc/data/tender/pengumuman?per_halaman=999999"); w.Code != 200 {
		t.Fatalf("status %d", w.Code)
	}
	if q, _ := kueriData(db.f, "OFFSET"); !strings.Contains(q.Query, "FETCH NEXT 50 ROWS ONLY") {
		t.Errorf("per_halaman ngawur tidak dibatasi: %q", q.Query)
	}
}

func TestEksporCSVMemuatSemuaKolomTanpaKolomTeknis(t *testing.T) {
	db := pasangDBData(t, []string{"kd_klpd", "nama_paket", "pagu", "tahun_anggaran", "synced_at"}, [][]driver.Value{
		{"K10", "=cmd|' /C calc'!A0", []byte("2500000.75"), "2025", time.Date(2026, 10, 4, 1, 2, 3, 0, time.UTC)},
	})
	db.f.TipeKolom = []string{"NVARCHAR", "NVARCHAR", "DECIMAL", "NVARCHAR", "DATETIME2"}

	w := ambil(routerData(), "/inaproc/ekspor/tender/pengumuman?format=csv&kode_klpd=K10&tahun=2025&pemisah=titik-koma")
	if w.Code != 200 {
		t.Fatalf("status %d: %s", w.Code, w.Body)
	}
	if ct := w.Header().Get("Content-Type"); ct != "text/csv; charset=utf-8" {
		t.Errorf("Content-Type = %q", ct)
	}
	cd := w.Header().Get("Content-Disposition")
	if !strings.HasPrefix(cd, `attachment; filename="tender-pengumuman_K10_2025_`) || !strings.HasSuffix(cd, `.csv"`) {
		t.Errorf("Content-Disposition = %q", cd)
	}
	isi := strings.TrimPrefix(w.Body.String(), "\xEF\xBB\xBF")
	baris := strings.Split(strings.TrimSpace(isi), "\r\n")
	if baris[0] != "kd_klpd;nama_paket;pagu;tahun_anggaran;synced_at" {
		t.Errorf("judul = %q", baris[0])
	}
	if baris[1] != "K10;'=cmd|' /C calc'!A0;2500000.75;2025;2026-10-04 01:02:03" {
		t.Errorf("baris = %q", baris[1])
	}
	// Kolom dipilih eksplisit dari metadata tabel; row_key dan extra_json tidak diminta.
	q, ada := kueriData(db.f, "ORDER BY")
	if !ada || !strings.HasPrefix(q.Query, "SELECT [kd_klpd], [nama_paket], [pagu], [tahun_anggaran], [synced_at] FROM [inaproc_tender_pengumuman] WHERE") ||
		strings.Contains(q.Query, "extra_json") || strings.Contains(q.Query, "OFFSET") {
		t.Errorf("query ekspor = %q", q.Query)
	}
}

func TestEksporXLSXDanPDF(t *testing.T) {
	d, _ := DatasetByID("tender/pengumuman")
	baris := [][]driver.Value{make([]driver.Value, len(d.KolomRingkas))}
	baris[0][0] = "K10-1"
	pasangDBData(t, d.KolomRingkas, baris)
	r := routerData()

	w := ambil(r, "/inaproc/ekspor/tender/pengumuman?format=xlsx")
	if w.Code != 200 || !strings.HasPrefix(w.Header().Get("Content-Type"), "application/vnd.openxmlformats") {
		t.Fatalf("xlsx: %d %q", w.Code, w.Header().Get("Content-Type"))
	}
	f, err := excelize.OpenReader(bytes.NewReader(w.Body.Bytes()))
	if err != nil {
		t.Fatalf("xlsx tidak sah: %v", err)
	}
	defer f.Close()
	if rows, _ := f.GetRows(f.GetSheetList()[0]); len(rows) != 2 {
		t.Errorf("baris xlsx = %d, want judul + 1", len(rows))
	}

	w = ambil(r, "/inaproc/ekspor/tender/pengumuman?format=pdf&tahun=2025")
	if w.Code != 200 || w.Header().Get("Content-Type") != "application/pdf" || !bytes.HasPrefix(w.Body.Bytes(), []byte("%PDF-")) {
		t.Errorf("pdf: %d %q", w.Code, w.Header().Get("Content-Type"))
	}
}

func TestEksporMenolakFormatBatasDanDatasetNgawur(t *testing.T) {
	db := pasangDBData(t, []string{"a"}, nil)
	r := routerData()
	if w := ambil(r, "/inaproc/ekspor/tender/pengumuman?format=docx"); w.Code != 400 {
		t.Errorf("format ngawur: %d", w.Code)
	}
	if w := ambil(r, "/inaproc/ekspor/tender/pengumuman?format=csv&pemisah=spasi"); w.Code != 400 {
		t.Errorf("pemisah ngawur: %d", w.Code)
	}
	if w := ambil(r, "/inaproc/ekspor/x/y?format=csv"); w.Code != 404 {
		t.Errorf("dataset tak dikenal: %d", w.Code)
	}
	// Melebihi batas satu sheet Excel: ditolak sebelum membaca data, CSV tetap diterima.
	db.total = 2_000_000
	w := ambil(r, "/inaproc/ekspor/tender/pengumuman?format=xlsx")
	if w.Code != 422 || !strings.Contains(w.Body.String(), "CSV") {
		t.Errorf("xlsx melebihi batas: %d %s", w.Code, w.Body)
	}
	if _, ada := kueriData(db.f, "ORDER BY"); ada {
		t.Error("data tidak boleh dibaca bila batas Excel terlewati")
	}
}

// Kegagalan sebelum byte pertama dijawab sebagai galat JSON biasa (bukan berkas rusak).
func TestEksporGagalSebelumMulaiMenjawabGalatJSON(t *testing.T) {
	db := pasangDBData(t, []string{"a"}, nil)
	db.galatBaca = errors.New("timeout")
	w := ambil(routerData(), "/inaproc/ekspor/tender/pengumuman?format=csv")
	if w.Code != 500 || strings.Contains(w.Body.String(), "timeout") {
		t.Errorf("status %d, isi %s (galat internal tidak boleh bocor)", w.Code, w.Body)
	}
	if w.Header().Get("Content-Disposition") != "" {
		t.Error("tidak boleh ada header unduhan saat gagal")
	}
}

func TestNamaBerkasEksporAmanDanBermakna(t *testing.T) {
	d, _ := DatasetByID("ekatalog-archive/paket-e-purchasing")
	now := time.Date(2026, 10, 4, 2, 30, 0, 0, time.UTC) // 09.30 WIB
	got := namaBerkasEkspor(d, PenyaringData{KodeKLPD: "K/10 ../x", Tahun: "2025"}, "xlsx", now)
	if got != "ekatalog-archive-paket-e-purchasing_K-10----x_2025_20261004-0930.xlsx" {
		t.Errorf("nama = %q", got)
	}
	if strings.ContainsAny(got, `/\"`+"\r\n") {
		t.Errorf("nama mengandung karakter berbahaya: %q", got)
	}
}
