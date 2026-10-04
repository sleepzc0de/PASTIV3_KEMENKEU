package handlers

import (
	"context"
	"database/sql/driver"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"pasti-v3-backend/internal/fakesql"
)

// Tes untuk bagian E-Katalog V6 yang tidak tercakup tes bersama endpointDatar: penanda boolean, kolom yang diisi dari permintaan,
// paket swasta, kategori berjenjang, dan transaksi per produk. Contoh baris disalin dari dokumentasi API Inaproc.

func routerEk6() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	g := r.Group("/inaproc", func(c *gin.Context) { c.Set("user_id", "admin-uji"); c.Next() })
	g.GET("/ekatalog/penyedia-detail", GetEkatalog6Penyedia)
	g.POST("/ekatalog/penyedia-detail/sync", SyncEkatalog6Penyedia)
	g.GET("/ekatalog/list-produk-penyedia", GetEkatalog6ProdukPenyedia)
	g.POST("/ekatalog/list-produk-penyedia/sync", SyncEkatalog6ProdukPenyedia)
	g.GET("/ekatalog/paket-e-purchasing", GetEkatalog6Paket)
	g.POST("/ekatalog/paket-e-purchasing/sync", SyncEkatalog6Paket)
	g.GET("/ekatalog/list-kategori-produk", GetEkatalog6Kategori)
	g.GET("/ekatalog/list-kategori-produk/local", ListLocalEkatalog6Kategori)
	g.POST("/ekatalog/list-kategori-produk/sync", SyncEkatalog6Kategori)
	g.GET("/ekatalog/e-purchasing-by-produk", GetEkatalog6Transaksi)
	g.GET("/ekatalog/e-purchasing-by-produk/local", ListLocalEkatalog6Transaksi)
	g.POST("/ekatalog/e-purchasing-by-produk/sync", SyncEkatalog6Transaksi)
	return r
}

// serverUji memberi Inaproc palsu yang selalu menjawab `data` (satu halaman) dan mencatat query tiap permintaan.
func serverUji(t *testing.T, data ...string) (*inaprocPalsu, *[]string) {
	t.Helper()
	var diminta []string
	p := newInaprocPalsu(t, func(r *http.Request) (int, string) {
		diminta = append(diminta, r.URL.Path+"?"+r.URL.RawQuery)
		return 200, `{"success":true,"data":[` + strings.Join(data, ",") + `],"meta":{"limit":1000,"has_more":false,"cursor":""}}`
	})
	pasangKonfigInaproc(t, p.URL, "token-uji")
	return p, &diminta
}

func queryHulu(t *testing.T, diminta string) url.Values {
	t.Helper()
	i := strings.Index(diminta, "?")
	v, err := url.ParseQuery(diminta[i+1:])
	if err != nil {
		t.Fatal(err)
	}
	return v
}

// argInsert: argumen kolom `nama` pada INSERT pertama ke tabel endpoint ini.
func argInsert(t *testing.T, f *fakesql.DB, ep *endpointDatar, nama string) interface{} {
	t.Helper()
	kolom := append([]string{"row_key"}, ep.SemuaKolom()...)
	for _, ex := range f.Execs() {
		if !strings.HasPrefix(ex.Query, "INSERT INTO "+ep.Tabel+" ") {
			continue
		}
		for i, k := range kolom {
			if k == nama {
				return ex.Args[i].Value
			}
		}
		t.Fatalf("kolom %q tidak ada pada INSERT %s", nama, ep.Tabel)
	}
	t.Fatalf("tidak ada INSERT ke %s", ep.Tabel)
	return nil
}

// hapusTercatat: DELETE pertama ke tabel endpoint ini (teks SQL dan argumennya sebagai teks).
func hapusTercatat(t *testing.T, f *fakesql.DB, ep *endpointDatar) (string, []string) {
	t.Helper()
	for _, ex := range f.Execs() {
		if strings.HasPrefix(ex.Query, "DELETE FROM "+ep.Tabel+" ") {
			var args []string
			for _, a := range ex.Args {
				args = append(args, fmt.Sprint(a.Value))
			}
			return ex.Query, args
		}
	}
	t.Fatalf("tidak ada DELETE ke %s", ep.Tabel)
	return "", nil
}

func logTercatat(t *testing.T, f *fakesql.DB) []driver.NamedValue {
	t.Helper()
	for _, ex := range f.Execs() {
		if strings.Contains(ex.Query, "INTO inaproc_sync_log") {
			return ex.Args
		}
	}
	t.Fatal("tidak ada catatan inaproc_sync_log")
	return nil
}

func TestGetInt64FromAnyMenerimaBoolean(t *testing.T) {
	row := map[string]interface{}{"ya": true, "tidak": false}
	if got := getInt64FromAny(row, "ya"); got != int64(1) {
		t.Errorf("true = %v, want 1", got)
	}
	if got := getInt64FromAny(row, "tidak"); got != int64(0) {
		t.Errorf("false = %v, want 0", got)
	}
}

// Respons list-produk-penyedia tidak memuat kode penyedia; kolom kode_penyedia diisi dari kode yang diminta, dan itu pula yang
// dipakai mengganti baris lama.
func TestEkatalog6ProdukPenyediaKodeDariPermintaan(t *testing.T) {
	_, diminta := serverUji(t, contohEk6Produk)
	f := pasangDBPalsu(t)

	if code, body := panggil(routerEk6(), "POST", "/inaproc/ekatalog/list-produk-penyedia/sync", `{"kode":" 01ABCXYZ123 "}`); code != 200 {
		t.Fatalf("%d %v", code, body)
	}
	if h := queryHulu(t, (*diminta)[0]); h.Get("kode_penyedia") != "01ABCXYZ123" {
		t.Errorf("parameter ke Inaproc = %v", h)
	}
	if got := argInsert(t, f, ekatalog6ProdukPenyedia, "kode_penyedia"); got != "01ABCXYZ123" {
		t.Errorf("kode_penyedia = %#v, want 01ABCXYZ123", got)
	}
	if got := argInsert(t, f, ekatalog6ProdukPenyedia, "status_produk_tayang"); got != int64(1) {
		t.Errorf("status_produk_tayang = %#v, want 1 (true di API)", got)
	}
	sql, args := hapusTercatat(t, f, ekatalog6ProdukPenyedia)
	if sql != "DELETE FROM inaproc_ekatalog6_produk_penyedia WHERE kode_penyedia = @p1" || strings.Join(args, ",") != "01ABCXYZ123" {
		t.Errorf("DELETE = %q %v", sql, args)
	}
	if l := logTercatat(t, f); l[0].Value != "ekatalog-list-produk-penyedia" || l[3].Value != "01ABCXYZ123" {
		t.Errorf("sync_log = %+v", l)
	}
}

// kode_klpd "swasta" (nilai literal) diteruskan apa adanya; bila respons mengosongkan kode_klpd, kolom diisi "swasta".
func TestEkatalog6PaketSwasta(t *testing.T) {
	baris := strings.Replace(contohEk6Paket, `"kode_klpd": "K1"`, `"kode_klpd": null`, 1)
	if baris == contohEk6Paket {
		t.Fatal("contoh tidak memuat kode_klpd")
	}
	_, diminta := serverUji(t, baris)
	f := pasangDBPalsu(t)
	r := routerEk6()

	if code, _ := panggil(r, "GET", "/inaproc/ekatalog/paket-e-purchasing?kode_klpd=swasta&tahun=2024", ""); code != 200 {
		t.Fatalf("GET: %d", code)
	}
	if h := queryHulu(t, (*diminta)[0]); h.Get("kode_klpd") != "swasta" || h.Get("tahun") != "2024" {
		t.Errorf("parameter ke Inaproc = %v", h)
	}

	if code, body := panggil(r, "POST", "/inaproc/ekatalog/paket-e-purchasing/sync", `{"kode_klpd":"swasta","tahun":"2024"}`); code != 200 {
		t.Fatalf("sync: %d %v", code, body)
	}
	sql, args := hapusTercatat(t, f, ekatalog6Paket)
	if sql != "DELETE FROM inaproc_ekatalog6_paket_epurchasing WHERE kode_klpd = @p1 AND fiscal_year = @p2" || strings.Join(args, ",") != "swasta,2024" {
		t.Errorf("DELETE = %q %v", sql, args)
	}
	if got := argInsert(t, f, ekatalog6Paket, "kode_klpd"); got != "swasta" {
		t.Errorf("kode_klpd = %#v, want swasta (diisi dari permintaan)", got)
	}
	if got := argInsert(t, f, ekatalog6Paket, "is_swasta"); got != int64(0) {
		t.Errorf("is_swasta = %#v, want 0 (false di API)", got)
	}
}

// ---- list-kategori-produk ----

func TestEkatalog6KategoriGetBerjenjang(t *testing.T) {
	_, diminta := serverUji(t, `{"datamart_id":1,"kd_kategori_1":"10000001","nama_kategori_1":"Elektronik"}`)
	r := routerEk6()
	jalur := "/inaproc/ekatalog/list-kategori-produk"

	for _, tc := range []struct {
		q        string
		kd1, kd2 string
	}{
		{"", "", ""},
		{"?kd_kategori_1=10000001", "10000001", ""},
		{"?kd_kategori_1=10000001&kd_kategori_2=KAT-002", "10000001", "KAT-002"},
		{"?kd_kategori_1=%20A%20", "A", ""},
	} {
		*diminta = nil
		if code, body := panggil(r, "GET", jalur+tc.q, ""); code != 200 {
			t.Fatalf("GET %q: %d %v", tc.q, code, body)
		}
		if !strings.HasPrefix((*diminta)[0], "/api/v1/ekatalog/list-kategori-produk?") {
			t.Errorf("jalur hulu = %q", (*diminta)[0])
		}
		h := queryHulu(t, (*diminta)[0])
		if h.Get("kd_kategori_1") != tc.kd1 || h.Get("kd_kategori_2") != tc.kd2 || h.Has("kode_klpd") || h.Has("tahun") || h.Get("limit") == "" {
			t.Errorf("GET %q: parameter ke Inaproc = %v", tc.q, h)
		}
	}

	// kd_kategori_2 hanya boleh bersama kd_kategori_1; terlalu panjang ditolak; semuanya tanpa menghubungi Inaproc.
	*diminta = nil
	for _, q := range []string{"?kd_kategori_2=KAT-002", "?kd_kategori_1=" + strings.Repeat("9", panjangKodeMaks+1)} {
		if code, _ := panggil(r, "GET", jalur+q, ""); code != 400 {
			t.Errorf("GET %q: %d, want 400", q, code)
		}
	}
	if len(*diminta) != 0 {
		t.Fatalf("masukan tidak sah tidak boleh menghubungi Inaproc: %v", *diminta)
	}

	// Nilai berisi karakter parameter tetap satu nilai.
	if code, _ := panggil(r, "GET", jalur+"?kd_kategori_1="+url.QueryEscape("a&kd_kategori_2=b"), ""); code != 200 {
		t.Fatalf("GET karakter khusus: %d", code)
	}
	if h := queryHulu(t, (*diminta)[0]); h.Get("kd_kategori_1") != "a&kd_kategori_2=b" || h.Has("kd_kategori_2") {
		t.Errorf("parameter ke Inaproc = %v", h)
	}
}

func TestEkatalog6KategoriSyncPerTingkat(t *testing.T) {
	// Level 2 dan 3 tidak memuat kode induk di responsnya.
	l1 := `{"datamart_id":1,"kd_kategori_1":"X","nama_kategori_1":"Satu"}`
	l2 := `{"datamart_id":5,"kd_kategori_2":"Y2","nama_kategori_2":"Dua"}` // kode level 2 itu sendiri, bukan kode induk
	l3 := `{"datamart_id":9,"kd_kategori_3":"Z","nama_kategori_3":"Tiga"}`
	tabel := "inaproc_ekatalog6_kategori"

	for _, tc := range []struct {
		nama, badan, baris string
		sqlHapus           string
		argsHapus          string
		tingkat, kd1, kd2  string
		jenis              string
	}{
		{"L1", `{}`, l1, "DELETE FROM " + tabel + " WHERE tingkat = @p1", "1", "1", "X", "", "L1"},
		{"L2", `{"kd_kategori_1":" X "}`, l2, "DELETE FROM " + tabel + " WHERE tingkat = @p1 AND kd_kategori_1 = @p2", "2,X", "2", "X", "", "L2:X"},
		{"L3", `{"kd_kategori_1":"X","kd_kategori_2":"Y"}`, l3, "DELETE FROM " + tabel + " WHERE tingkat = @p1 AND kd_kategori_1 = @p2 AND kd_kategori_2 = @p3", "3,X,Y", "3", "X", "Y", "L3:X/Y"},
	} {
		t.Run(tc.nama, func(t *testing.T) {
			_, diminta := serverUji(t, tc.baris)
			f := pasangDBPalsu(t)
			if code, body := panggil(routerEk6(), "POST", "/inaproc/ekatalog/list-kategori-produk/sync", tc.badan); code != 200 {
				t.Fatalf("%d %v", code, body)
			}
			h := queryHulu(t, (*diminta)[0])
			if (h.Get("kd_kategori_1") != "") != (tc.tingkat != "1") || (h.Get("kd_kategori_2") != "") != (tc.tingkat == "3") {
				t.Errorf("parameter ke Inaproc = %v", h)
			}
			sql, args := hapusTercatat(t, f, ekatalog6Kategori)
			if sql != tc.sqlHapus || strings.Join(args, ",") != tc.argsHapus {
				t.Errorf("DELETE = %q %v, want %q %s", sql, args, tc.sqlHapus, tc.argsHapus)
			}
			// tingkat dan kode induk (yang tidak ada di respons) diisi dari permintaan.
			if got := fmt.Sprint(argInsert(t, f, ekatalog6Kategori, "tingkat")); got != tc.tingkat {
				t.Errorf("tingkat = %q, want %q", got, tc.tingkat)
			}
			// L1: kd_kategori_1 dari baris, kd_kategori_2 kosong. L2: kd_kategori_1 dari permintaan, kd_kategori_2 dari baris ("Y2").
			// L3: keduanya dari permintaan (baris hanya memuat kd_kategori_3).
			kd1, kd2 := argInsert(t, f, ekatalog6Kategori, "kd_kategori_1"), argInsert(t, f, ekatalog6Kategori, "kd_kategori_2")
			var want2 interface{}
			switch tc.tingkat {
			case "2":
				want2 = "Y2"
			case "3":
				want2 = "Y"
			}
			if kd1 != "X" || kd2 != want2 {
				t.Errorf("tingkat %s: kd_kategori_1 = %#v, kd_kategori_2 = %#v, want X dan %#v", tc.tingkat, kd1, kd2, want2)
			}
			if l := logTercatat(t, f); l[0].Value != "ekatalog-list-kategori-produk" || l[3].Value != tc.jenis {
				t.Errorf("sync_log = %+v, want jenis %q", l, tc.jenis)
			}
		})
	}

	// kd_kategori_2 tanpa kd_kategori_1 dan JSON rusak: ditolak sebelum menyentuh Inaproc atau database.
	_, diminta := serverUji(t, l1)
	f := pasangDBPalsu(t)
	for _, b := range []string{`{"kd_kategori_2":"Y"}`, `{rusak`} {
		if code, _ := panggil(routerEk6(), "POST", "/inaproc/ekatalog/list-kategori-produk/sync", b); code != 400 {
			t.Errorf("sync %q: %d, want 400", b, code)
		}
	}
	if len(*diminta) != 0 || len(f.Execs()) != 0 {
		t.Errorf("tidak boleh menyentuh Inaproc/database: %v %v", *diminta, f.Execs())
	}
}

func TestEkatalog6KategoriDaftarLokal(t *testing.T) {
	for _, tc := range []struct{ q, where, args string }{
		{"", "WHERE tingkat = @p1", "1"},
		{"?kd_kategori_1=X", "WHERE tingkat = @p1 AND kd_kategori_1 = @p2", "2,X"},
		{"?kd_kategori_1=X&kd_kategori_2=Y", "WHERE tingkat = @p1 AND kd_kategori_1 = @p2 AND kd_kategori_2 = @p3", "3,X,Y"},
	} {
		f := pasangDBPalsu(t)
		var query string
		var args []driver.NamedValue
		f.OnQuery = func(ctx context.Context, q string, a []driver.NamedValue) ([]string, [][]driver.Value, error) {
			query, args = q, a
			return []string{"row_key"}, [][]driver.Value{{"k1"}}, nil
		}
		if code, _ := panggil(routerEk6(), "GET", "/inaproc/ekatalog/list-kategori-produk/local"+tc.q, ""); code != 200 {
			t.Fatalf("%q: %d", tc.q, code)
		}
		var got []string
		for _, a := range args {
			got = append(got, fmt.Sprint(a.Value))
		}
		if !strings.Contains(query, tc.where) || strings.Join(got, ",") != tc.args {
			t.Errorf("%q: query = %s args = %v", tc.q, query, got)
		}
	}
}

// ---- e-purchasing-by-produk ----

func TestEkatalog6TransaksiGetAturanPenyaring(t *testing.T) {
	_, diminta := serverUji(t, contohEk6Transaksi)
	r := routerEk6()
	jalur := "/inaproc/ekatalog/e-purchasing-by-produk"

	for _, tc := range []struct {
		q    string
		want map[string]string // parameter yang harus ada ("" = tidak boleh ada)
	}{
		// Tanpa kode_klpd/kd_kategori_1: KLPD bawaan K10; status selalu eksplisit (bawaan COMPLETED).
		{"?tahun=2024", map[string]string{"tahun": "2024", "status": "COMPLETED", "kode_klpd": "K10", "kd_kategori_1": "", "kd_product": ""}},
		// Hanya kategori: kode_klpd tidak ditambahkan sendiri (akan mempersempit hasil).
		{"?tahun=2024&kd_kategori_1=C1", map[string]string{"kd_kategori_1": "C1", "kode_klpd": "", "status": "COMPLETED"}},
		// Semua isian; status tidak peka huruf besar/kecil dan dikirim huruf besar.
		{"?tahun=2024&kode_klpd=K5&kd_kategori_1=C1&kd_product=P9&status=on_process", map[string]string{"kode_klpd": "K5", "kd_kategori_1": "C1", "kd_product": "P9", "status": "ON_PROCESS"}},
		{"?tahun=2024&status=%20cancelled_on_review%20", map[string]string{"status": "CANCELLED_ON_REVIEW"}},
	} {
		*diminta = nil
		if code, body := panggil(r, "GET", jalur+tc.q, ""); code != 200 {
			t.Fatalf("GET %q: %d %v", tc.q, code, body)
		}
		if !strings.HasPrefix((*diminta)[0], "/api/v1/ekatalog/e-purchasing-by-produk?") {
			t.Errorf("jalur hulu = %q", (*diminta)[0])
		}
		h := queryHulu(t, (*diminta)[0])
		for k, v := range tc.want {
			if h.Get(k) != v || (v == "" && h.Has(k)) {
				t.Errorf("GET %q: %s = %q (ada=%v), want %q", tc.q, k, h.Get(k), h.Has(k), v)
			}
		}
	}

	// Ditolak sebelum menghubungi Inaproc.
	*diminta = nil
	for _, q := range []string{
		"", "?kode_klpd=K10", "?tahun=abc", "?tahun=0", "?tahun=20245", "?tahun=-1", "?tahun=2024&status=BOGUS",
		"?tahun=2024&status=COMPLETED%3BDROP", "?tahun=2024&kd_product=" + strings.Repeat("9", panjangKodeMaks+1),
	} {
		if code, _ := panggil(r, "GET", jalur+q, ""); code != 400 {
			t.Errorf("GET %q: %d, want 400", q, code)
		}
	}
	if len(*diminta) != 0 {
		t.Fatalf("masukan tidak sah tidak boleh menghubungi Inaproc: %v", *diminta)
	}

	// Seluruh 12 status yang didokumentasikan diterima.
	for s := range statusTransaksiSah {
		if code, _ := panggil(r, "GET", jalur+"?tahun=2024&status="+s, ""); code != 200 {
			t.Errorf("status %s: %d, want 200", s, code)
		}
	}
	if len(statusTransaksiSah) != 12 {
		t.Errorf("daftar status = %d, dokumentasi menyebut 12", len(statusTransaksiSah))
	}
}

func TestEkatalog6TransaksiSync(t *testing.T) {
	tabel := "inaproc_ekatalog6_epurchasing_produk"
	for _, tc := range []struct {
		nama, badan, hapus, args string
		klpdLog, tahunLog, jenis string
		kodeKlpdKolom            interface{} // nilai kolom kode_klpd bila respons mengosongkannya
	}{
		{
			"bawaan", `{"tahun":"2024"}`,
			"DELETE FROM " + tabel + " WHERE tahun = @p1 AND status = @p2 AND kode_klpd = @p3", "2024,COMPLETED,K10",
			"K10", "2024", "COMPLETED", "K10",
		},
		{
			"kategori dan produk", `{"tahun":" 2024 ","kd_kategori_1":"C1","kd_product":"P9","status":"on_process"}`,
			"DELETE FROM " + tabel + " WHERE tahun = @p1 AND status = @p2 AND kd_kategori_1 = @p3 AND product_id = @p4", "2024,ON_PROCESS,C1,P9",
			"", "2024", "ON_PROCESS", nil,
		},
	} {
		t.Run(tc.nama, func(t *testing.T) {
			// Baris dari Inaproc tanpa kode_klpd, untuk melihat pengisian dari permintaan.
			baris := strings.Replace(contohEk6Transaksi, `"kode_klpd": "K1"`, `"kode_klpd": null`, 1)
			_, diminta := serverUji(t, baris)
			f := pasangDBPalsu(t)
			if code, body := panggil(routerEk6(), "POST", "/inaproc/ekatalog/e-purchasing-by-produk/sync", tc.badan); code != 200 {
				t.Fatalf("%d %v", code, body)
			}
			if h := queryHulu(t, (*diminta)[0]); h.Get("tahun") != "2024" || h.Get("status") == "" {
				t.Errorf("parameter ke Inaproc = %v", h)
			}
			sql, args := hapusTercatat(t, f, ekatalog6Transaksi)
			if sql != tc.hapus || strings.Join(args, ",") != tc.args {
				t.Errorf("DELETE = %q %v, want %q %s", sql, args, tc.hapus, tc.args)
			}
			// Respons tidak memuat tahun: kolom tahun diisi dari permintaan.
			if got := argInsert(t, f, ekatalog6Transaksi, "tahun"); got != "2024" {
				t.Errorf("tahun = %#v, want 2024", got)
			}
			if got := argInsert(t, f, ekatalog6Transaksi, "kode_klpd"); got != tc.kodeKlpdKolom {
				t.Errorf("kode_klpd = %#v, want %#v", got, tc.kodeKlpdKolom)
			}
			l := logTercatat(t, f)
			if l[0].Value != "ekatalog-e-purchasing-by-produk" || l[1].Value != tc.klpdLog || l[2].Value != tc.tahunLog || l[3].Value != tc.jenis {
				t.Errorf("sync_log = %+v", l)
			}
		})
	}

	_, diminta := serverUji(t, contohEk6Transaksi)
	f := pasangDBPalsu(t)
	for _, b := range []string{`{}`, `{"tahun":"abc"}`, `{"tahun":"2024","status":"BOGUS"}`, `{"kode_klpd":"K10"}`, `{rusak`} {
		if code, _ := panggil(routerEk6(), "POST", "/inaproc/ekatalog/e-purchasing-by-produk/sync", b); code != 400 {
			t.Errorf("sync %q: %d, want 400", b, code)
		}
	}
	if len(*diminta) != 0 || len(f.Execs()) != 0 {
		t.Errorf("masukan tidak sah tidak boleh menyentuh Inaproc/database: %v %v", *diminta, f.Execs())
	}
}

func TestEkatalog6TransaksiDaftarLokal(t *testing.T) {
	for _, tc := range []struct{ q, where, args string }{
		{"", "WHERE kode_klpd = @p1", "K10"},
		{"?tahun=2024&status=completed", "WHERE tahun = @p1 AND status = @p2 AND kode_klpd = @p3", "2024,COMPLETED,K10"},
		{"?tahun=2024&kd_kategori_1=C1", "WHERE tahun = @p1 AND kd_kategori_1 = @p2", "2024,C1"},
	} {
		f := pasangDBPalsu(t)
		var query string
		var args []driver.NamedValue
		f.OnQuery = func(ctx context.Context, q string, a []driver.NamedValue) ([]string, [][]driver.Value, error) {
			query, args = q, a
			return []string{"row_key"}, [][]driver.Value{{"k1"}}, nil
		}
		if code, _ := panggil(routerEk6(), "GET", "/inaproc/ekatalog/e-purchasing-by-produk/local"+tc.q, ""); code != 200 {
			t.Fatalf("%q: %d", tc.q, code)
		}
		var got []string
		for _, a := range args {
			got = append(got, fmt.Sprint(a.Value))
		}
		if !strings.Contains(query, tc.where) || strings.Join(got, ",") != tc.args {
			t.Errorf("%q: query = %s args = %v", tc.q, query, got)
		}
	}
}
