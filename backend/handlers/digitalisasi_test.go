package handlers

import (
	"bytes"
	"context"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"pasti-v3-backend/database"
	"pasti-v3-backend/digitalisasi"
	"pasti-v3-backend/internal/fakesql"
)

// Tes handler Digitalisasi Aset dengan database palsu: memeriksa perilaku (privasi, parameterisasi, bentuk
// respons, kecocokan jumlah kolom SELECT dengan Scan), bukan keabsahan sintaks T-SQL di SQL Server.

func setupDG(t *testing.T) (pasti, sldk *fakesql.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	pDB, p := fakesql.New(t)
	sDB, s := fakesql.New(t)
	oldDB, oldSLDK, oldMgr := database.DB, database.SLDKDB, digitalisasi.Default
	database.DB, database.SLDKDB = pDB, sDB
	digitalisasi.Default = digitalisasi.NewManager(pDB, sDB)
	t.Cleanup(func() {
		digitalisasi.Default.Cancel()
		deadline := time.Now().Add(2 * time.Second)
		for digitalisasi.Default.Active() != nil && time.Now().Before(deadline) {
			time.Sleep(5 * time.Millisecond)
		}
		database.DB, database.SLDKDB, digitalisasi.Default = oldDB, oldSLDK, oldMgr
	})
	return p, s
}

func dgRouter(role string) *gin.Engine {
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("role", role)
		c.Set("username", "tester")
		c.Next()
	})
	g := r.Group("/dg")
	g.GET("/ringkasan", GetDigitalisasiRingkasan)
	g.GET("/peta", GetDigitalisasiPeta)
	g.GET("/ekspor/:dataset", EksporDigitalisasiData)
	g.GET("/data/:dataset", ListDigitalisasiData)
	g.GET("/data/:dataset/:id", GetDigitalisasiDetail)
	g.GET("/sinkronisasi", GetDigitalisasiSinkronisasi)
	g.POST("/sinkronisasi", StartDigitalisasiSync)
	g.POST("/sinkronisasi/batal", CancelDigitalisasiSync)
	return r
}

func call(r *gin.Engine, method, url, body string) (int, map[string]interface{}) {
	req := httptest.NewRequest(method, url, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var out map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return w.Code, out
}

func dataOf(t *testing.T, m map[string]interface{}) map[string]interface{} {
	t.Helper()
	d, ok := m["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("respons tanpa data: %v", m)
	}
	return d
}

// lastQuery: perintah Query terakhir yang diawali prefix.
func lastQuery(t *testing.T, p *fakesql.DB, prefix string) fakesql.Stmt {
	t.Helper()
	qs := p.Queries()
	for i := len(qs) - 1; i >= 0; i-- {
		if strings.HasPrefix(qs[i].Query, prefix) {
			return qs[i]
		}
	}
	t.Fatalf("tidak ada query berawalan %q; yang terkirim: %d", prefix, len(qs))
	return fakesql.Stmt{}
}

// selectedColumns membaca daftar kolom dari "SELECT a, [b], c FROM ..." supaya fake bisa membalas sesuai kolom.
func selectedColumns(q string) []string {
	body := q[len("SELECT "):strings.Index(q, " FROM ")]
	var cols []string
	for _, c := range strings.Split(body, ",") {
		cols = append(cols, strings.Trim(strings.TrimSpace(c), "[]"))
	}
	return cols
}

func rowFor(cols []string) []driver.Value {
	row := make([]driver.Value, len(cols))
	for i, c := range cols {
		switch c {
		case "id":
			row[i] = int64(7)
		case "Luas_RN", "Luas_Tanah":
			row[i] = []byte("123.5000") // decimal dari driver SQL Server datang sebagai []byte
		case "Latitude":
			row[i] = []byte("-6.2000000")
		case "Longitude":
			row[i] = []byte("106.8000000")
		case "synced_at":
			row[i] = time.Date(2026, 10, 3, 1, 2, 3, 0, time.UTC)
		default:
			row[i] = "x"
		}
	}
	return row
}

func listFake(p *fakesql.DB, rows int) {
	p.OnQuery = func(ctx context.Context, q string, args []driver.NamedValue) ([]string, [][]driver.Value, error) {
		switch {
		case strings.HasPrefix(q, "SELECT COUNT(*)"):
			return []string{"n"}, [][]driver.Value{{int64(2)}}, nil
		case strings.HasPrefix(q, "SELECT DISTINCT"):
			return []string{"v"}, [][]driver.Value{{"A"}, {"B"}}, nil
		case strings.HasPrefix(q, "SELECT id"):
			cols := selectedColumns(q)
			var out [][]driver.Value
			for i := 0; i < rows; i++ {
				out = append(out, rowFor(cols))
			}
			return cols, out, nil
		}
		return nil, nil, fmt.Errorf("query tak terduga: %s", q)
	}
}

func TestListHidesPersonalColumnFromNonAdmin(t *testing.T) {
	p, _ := setupDG(t)
	listFake(p, 1)

	for _, tc := range []struct {
		role  string
		admin bool
	}{{"user", false}, {"admin", true}, {"superadmin", true}} {
		code, body := call(dgRouter(tc.role), "GET", "/dg/data/rumah_negara", "")
		if code != 200 {
			t.Fatalf("%s: status %d: %v", tc.role, code, body)
		}
		if has := strings.Contains(lastQuery(t, p, "SELECT id").Query, "Nama_Penghuni"); has != tc.admin {
			t.Errorf("%s: Nama_Penghuni di SELECT = %v, want %v", tc.role, has, tc.admin)
		}
		data := dataOf(t, body)
		hasCol := false
		for _, c := range data["kolom"].([]interface{}) {
			if c.(map[string]interface{})["nama"] == "Nama_Penghuni" {
				hasCol = true
			}
		}
		row := data["rows"].([]interface{})[0].(map[string]interface{})
		_, leaked := row["Nama_Penghuni"]
		if hasCol != tc.admin || leaked != tc.admin {
			t.Errorf("%s: kolom di metadata=%v, di baris=%v, want %v", tc.role, hasCol, leaked, tc.admin)
		}
	}
}

func TestListParametersAndNumericNormalization(t *testing.T) {
	p, _ := setupDG(t)
	listFake(p, 1)

	// q (didekode) = 100%_[x : karakter khusus LIKE harus di-escape, dan masuk sebagai parameter.
	code, body := call(dgRouter("user"), "GET", "/dg/data/rumah_negara?per_page=500&page=3&ue1=01504&q=100%25_%5Bx", "")
	if code != 200 {
		t.Fatalf("status %d: %v", code, body)
	}
	st := lastQuery(t, p, "SELECT id")
	if strings.Contains(st.Query, "100%") || strings.Contains(st.Query, "01504") {
		t.Fatalf("masukan pengguna tidak boleh ada di teks SQL: %s", st.Query)
	}
	if !strings.Contains(st.Query, "ESCAPE '\\'") || !strings.Contains(st.Query, "OFFSET @p3 ROWS FETCH NEXT @p4 ROWS ONLY") {
		t.Fatalf("bentuk SQL tak terduga: %s", st.Query)
	}
	want := []interface{}{`%100\%\_\[x%`, "01504", int64(200), int64(100)} // per_page dibatasi 100; halaman 3 -> offset 200
	if len(st.Args) != len(want) {
		t.Fatalf("args = %v", st.Args)
	}
	for i, w := range want {
		if st.Args[i].Value != w {
			t.Errorf("arg %d = %#v, want %#v", i+1, st.Args[i].Value, w)
		}
	}

	data := dataOf(t, body)
	if data["total"] != float64(2) || data["page"] != float64(3) || data["per_page"] != float64(100) {
		t.Errorf("paginasi = %v %v %v", data["total"], data["page"], data["per_page"])
	}
	row := data["rows"].([]interface{})[0].(map[string]interface{})
	if row["Luas_RN"] != 123.5 {
		t.Errorf("Luas_RN = %#v; desimal harus jadi angka JSON", row["Luas_RN"])
	}
	if row["Latitude"] != -6.2 {
		t.Errorf("Latitude = %#v", row["Latitude"])
	}
	filters := data["filter"].(map[string]interface{})
	for _, k := range []string{"ue1", "provinsi", "kondisi"} {
		if len(filters[k].([]interface{})) != 2 {
			t.Errorf("opsi filter %s = %v", k, filters[k])
		}
	}
}

func TestListRejectsBadInput(t *testing.T) {
	p, _ := setupDG(t)
	listFake(p, 0)
	r := dgRouter("user")
	if code, _ := call(r, "GET", "/dg/data/tidak_ada", ""); code != 404 {
		t.Errorf("dataset tak dikenal: %d", code)
	}
	if code, _ := call(r, "GET", "/dg/data/tanah?q="+strings.Repeat("a", 101), ""); code != 400 {
		t.Errorf("q terlalu panjang: %d", code)
	}
	if code, _ := call(r, "GET", "/dg/data/tanah?page=-4&per_page=abc", ""); code != 200 {
		t.Errorf("halaman/per_page tak valid harus dikoreksi, bukan galat: %d", code)
	}
	st := lastQuery(t, p, "SELECT id")
	if st.Args[0].Value != int64(0) || st.Args[1].Value != int64(25) {
		t.Errorf("koreksi halaman: %v", st.Args)
	}
}

func TestDetail(t *testing.T) {
	p, _ := setupDG(t)
	found := true
	p.OnQuery = func(ctx context.Context, q string, args []driver.NamedValue) ([]string, [][]driver.Value, error) {
		cols := selectedColumns(q)
		if !found {
			return cols, nil, nil
		}
		return cols, [][]driver.Value{rowFor(cols)}, nil
	}

	code, body := call(dgRouter("user"), "GET", "/dg/data/rumah_negara/7", "")
	if code != 200 {
		t.Fatalf("status %d: %v", code, body)
	}
	st := lastQuery(t, p, "SELECT id")
	if strings.Contains(st.Query, "Nama_Penghuni") || !strings.Contains(st.Query, "[synced_at]") || st.Args[0].Value != int64(7) {
		t.Fatalf("query detail non-admin: %s %v", st.Query, st.Args)
	}
	row := dataOf(t, body)["row"].(map[string]interface{})
	if row["Luas_RN"] != 123.5 || row["Latitude"] != -6.2 {
		t.Errorf("angka tidak dinormalkan: %v", row)
	}

	code, _ = call(dgRouter("admin"), "GET", "/dg/data/rumah_negara/7", "")
	if code != 200 || !strings.Contains(lastQuery(t, p, "SELECT id").Query, "Nama_Penghuni") {
		t.Error("admin harus mendapat kolom pribadi")
	}

	for _, id := range []string{"abc", "0", "-1", "1.5"} {
		if code, _ := call(dgRouter("user"), "GET", "/dg/data/tanah/"+id, ""); code != 400 {
			t.Errorf("id %q: %d, want 400", id, code)
		}
	}
	found = false
	if code, _ := call(dgRouter("user"), "GET", "/dg/data/tanah/999", ""); code != 404 {
		t.Errorf("tidak ada: %d, want 404", code)
	}
}

func TestMapUsesIndonesiaBoundsAndParameters(t *testing.T) {
	p, _ := setupDG(t)
	p.OnQuery = func(ctx context.Context, q string, args []driver.NamedValue) ([]string, [][]driver.Value, error) {
		switch {
		case strings.HasPrefix(q, "SELECT COUNT(*)"):
			return []string{"a", "b", "c"}, [][]driver.Value{{int64(10), int64(6), int64(3)}}, nil
		case strings.HasPrefix(q, "SELECT TOP"):
			return []string{"id", "Latitude", "Longitude"}, [][]driver.Value{
				{int64(1), []byte("-6.2088001234"), []byte("106.8456007")},
				{int64(2), []byte("-7.25"), []byte("112.75")},
			}, nil
		}
		return nil, nil, fmt.Errorf("query tak terduga: %s", q)
	}
	code, body := call(dgRouter("user"), "GET", "/dg/peta?dataset=tanah,gedung_kantor_utama,satker,../x&ue1=01504", "")
	if code != 200 {
		t.Fatalf("status %d: %v", code, body)
	}
	sets := dataOf(t, body)["datasets"].([]interface{})
	if len(sets) != 2 {
		t.Fatalf("hanya dataset beralamat koordinat yang diminta yang boleh keluar (satker tidak punya koordinat): %d", len(sets))
	}
	s := sets[0].(map[string]interface{})
	if s["key"] != "tanah" || s["total"] != float64(10) || s["bertitik"] != float64(6) || s["tanpa_koordinat"] != float64(3) || s["di_luar_indonesia"] != float64(1) {
		t.Errorf("hitungan = %v", s)
	}
	pts := s["titik"].([]interface{})
	first := pts[0].([]interface{})
	if first[0] != float64(1) || first[1] != -6.208800 || first[2] != 106.845601 {
		t.Errorf("titik pertama = %v (harus dibulatkan 6 desimal)", first)
	}

	st := lastQuery(t, p, "SELECT TOP")
	if !strings.Contains(st.Query, "BETWEEN -11.5 AND 6.5") || !strings.Contains(st.Query, "BETWEEN 94.5 AND 141.5") {
		t.Errorf("kotak batas Indonesia tidak dipakai: %s", st.Query)
	}
	if strings.Contains(st.Query, "01504") || st.Args[0].Value != "01504" {
		t.Errorf("ue1 harus parameter: %s %v", st.Query, st.Args)
	}
}

func TestRingkasanNotAvailableWhenNothingSynced(t *testing.T) {
	p, _ := setupDG(t)
	p.OnQuery = func(ctx context.Context, q string, args []driver.NamedValue) ([]string, [][]driver.Value, error) {
		var rows [][]driver.Value
		for _, ds := range digitalisasi.Datasets {
			rows = append(rows, []driver.Value{ds.Key, int64(0)})
		}
		return []string{"k", "n"}, rows, nil
	}
	code, body := call(dgRouter("user"), "GET", "/dg/ringkasan", "")
	if code != 200 || dataOf(t, body)["tersedia"] != false {
		t.Fatalf("status %d, body %v", code, body)
	}
}

// ringkasanFake menjawab setiap query ringkasan dengan jumlah kolom yang sama seperti yang di-Scan handler,
// sehingga ketidakcocokan (kolom SELECT vs Scan) muncul sebagai galat di tes ini.
func ringkasanFake(p *fakesql.DB) {
	logRow := []driver.Value{int64(3), "tanah", "sukses", time.Date(2026, 10, 3, 1, 0, 0, 0, time.UTC), nil,
		time.Date(2026, 10, 3, 1, 5, 0, 0, time.UTC), int64(10), int64(6), "10 baris", "admin1"}
	p.OnQuery = func(ctx context.Context, q string, args []driver.NamedValue) ([]string, [][]driver.Value, error) {
		switch {
		case strings.HasPrefix(q, "SELECT 'satker', COUNT(*) FROM"):
			var rows [][]driver.Value
			for _, ds := range digitalisasi.Datasets {
				rows = append(rows, []driver.Value{ds.Key, int64(10)})
			}
			return []string{"k", "n"}, rows, nil
		case strings.HasPrefix(q, "SELECT COUNT(*), CAST("): // statistik per dataset aset
			return make([]string, 7), [][]driver.Value{{int64(10), []byte("100.5000"), []byte("2000.00"), int64(6), int64(2), int64(1), int64(0)}}, nil
		case strings.Contains(q, "SUM(CASE WHEN Jenis_Satker"):
			return make([]string, 4), [][]driver.Value{{int64(3), int64(2), int64(12), int64(5)}}, nil
		case strings.Contains(q, "AS ds"): // rincian UE1 / provinsi
			return make([]string, 5), [][]driver.Value{
				{"tanah", "01504", int64(3), []byte("10.0000"), []byte("5.00")},
				{"rusunara", "(kosong)", int64(1), []byte("0"), []byte("0")},
			}, nil
		case strings.Contains(q, "FROM DIGITALISASI_SATKER GROUP BY"):
			return make([]string, 4), [][]driver.Value{{"01504", int64(4), int64(8), int64(2)}, {"01599", int64(1), int64(0), int64(0)}}, nil
		case strings.Contains(q, "GROUP BY CAST("): // kondisi, status hukum, asuransi, penghuni
			return make([]string, 3), [][]driver.Value{{"Baik", int64(5), []byte("10.00")}, {nil, int64(1), []byte("0")}}, nil
		case strings.HasPrefix(q, "SELECT ISNULL(SUM("), strings.HasPrefix(q, "SELECT COUNT(*) FROM DIGITALISASI_SATKER"):
			return []string{"n"}, [][]driver.Value{{int64(4)}}, nil
		case strings.Contains(q, "PARTITION BY dataset"):
			return make([]string, 10), [][]driver.Value{logRow}, nil
		}
		return nil, nil, fmt.Errorf("query tak terduga: %s", q)
	}
}

func TestRingkasanShape(t *testing.T) {
	p, _ := setupDG(t)
	ringkasanFake(p)
	code, body := call(dgRouter("user"), "GET", "/dg/ringkasan", "")
	if code != 200 {
		t.Fatalf("status %d: %v", code, body)
	}
	d := dataOf(t, body)
	if d["tersedia"] != true {
		t.Fatal("tersedia harus true")
	}
	aset := d["aset"].([]interface{})
	if len(aset) != 6 {
		t.Fatalf("aset = %d, want 6 dataset aset (tanpa satker)", len(aset))
	}
	tanah := aset[0].(map[string]interface{})
	if tanah["key"] != "tanah" || tanah["jumlah"] != float64(10) || tanah["luas"] != 100.5 || tanah["nilai"] != 2000.0 ||
		tanah["bertitik"] != float64(6) || tanah["tanpa_koordinat"] != float64(2) || tanah["di_luar_indonesia"] != float64(2) {
		t.Errorf("tanah = %v", tanah)
	}
	sat := d["satker"].(map[string]interface{})
	if sat["total"] != float64(5) || sat["induk"] != float64(3) || sat["kdj"] != float64(12) {
		t.Errorf("satker = %v", sat)
	}
	ue1 := d["ue1"].([]interface{})
	if len(ue1) < 2 || ue1[0].(map[string]interface{})["kode"] != "01504" || ue1[0].(map[string]interface{})["label"] != "01504 · DJP" {
		t.Errorf("ue1 = %v", ue1)
	}
	if len(d["provinsi"].([]interface{})) == 0 {
		t.Error("provinsi kosong")
	}
	for _, k := range []string{"kondisi", "status_hukum", "asuransi", "hunian", "kelengkapan", "sinkron", "status_penghuni"} {
		if d[k] == nil {
			t.Errorf("bagian %s hilang", k)
		}
	}
	// Peran kolom: rumah negara tidak punya nilai, sedangkan tanah punya.
	for _, a := range aset {
		m := a.(map[string]interface{})
		if m["key"] == "rumah_negara" && m["punya_nilai"] != false {
			t.Error("rumah_negara tidak punya kolom nilai")
		}
	}
}

func TestStatusMasksFailureDetailsFromNonAdmin(t *testing.T) {
	p, _ := setupDG(t)
	ctx, stop := context.WithCancel(context.Background())
	t.Cleanup(stop) // menghentikan penjadwal yang dinyalakan di bawah
	digitalisasi.Default.MulaiPenjadwal(ctx, digitalisasi.NewJadwal(true, 7, 1, 5))
	secret := "mssql: Login failed for user 'sa' (host 10.1.2.3)"
	failed := []driver.Value{int64(5), "tanah", "gagal", time.Now(), nil, time.Now(), nil, nil, secret, "admin1"}
	p.OnQuery = func(ctx context.Context, q string, args []driver.NamedValue) ([]string, [][]driver.Value, error) {
		switch {
		case strings.HasPrefix(q, "SELECT 'satker', COUNT(*) FROM"):
			return []string{"k", "n"}, [][]driver.Value{{"tanah", int64(4)}}, nil
		case strings.Contains(q, "PARTITION BY dataset"), strings.HasPrefix(q, "SELECT TOP"):
			return make([]string, 10), [][]driver.Value{failed}, nil
		}
		return nil, nil, fmt.Errorf("query tak terduga: %s", q)
	}

	_, body := call(dgRouter("user"), "GET", "/dg/sinkronisasi", "")
	raw, _ := json.Marshal(body)
	if strings.Contains(string(raw), "Login failed") || strings.Contains(string(raw), "10.1.2.3") || strings.Contains(string(raw), "admin1") {
		t.Fatalf("detail galat/pengguna bocor ke non-admin: %s", raw)
	}
	if !strings.Contains(string(raw), "Sinkronisasi gagal") {
		t.Errorf("non-admin tetap harus tahu bahwa sinkronisasi gagal: %s", raw)
	}

	_, body = call(dgRouter("admin"), "GET", "/dg/sinkronisasi", "")
	raw, _ = json.Marshal(body)
	if !strings.Contains(string(raw), "Login failed") {
		t.Errorf("admin harus melihat pesan lengkap: %s", raw)
	}
	d := dataOf(t, body)
	if d["sldk_tersedia"] != true || len(d["datasets"].([]interface{})) != len(digitalisasi.Datasets) {
		t.Errorf("status = %v", d)
	}

	// Jadwal sinkronisasi otomatis ikut dikirim (terlihat oleh semua pengguna) beserta perkiraan berikutnya.
	for _, peran := range []string{"user", "admin"} {
		_, b := call(dgRouter(peran), "GET", "/dg/sinkronisasi", "")
		oto, _ := dataOf(t, b)["otomatis"].(map[string]interface{})
		if oto["aktif"] != true || oto["interval_hari"] != float64(7) || oto["jam_mulai"] != float64(1) || oto["jam_akhir"] != float64(5) || oto["zona"] != "WIB" || oto["berikutnya"] == nil {
			t.Errorf("%s: otomatis = %v", peran, oto)
		}
	}
}

func TestStatusOtomatisNonaktif(t *testing.T) {
	p, _ := setupDG(t)
	p.OnQuery = func(ctx context.Context, q string, args []driver.NamedValue) ([]string, [][]driver.Value, error) {
		switch {
		case strings.HasPrefix(q, "SELECT 'satker', COUNT(*) FROM"):
			return []string{"k", "n"}, nil, nil
		case strings.Contains(q, "PARTITION BY dataset"), strings.HasPrefix(q, "SELECT TOP"):
			return make([]string, 10), nil, nil
		}
		return nil, nil, fmt.Errorf("query tak terduga: %s", q)
	}
	digitalisasi.Default.MulaiPenjadwal(context.Background(), digitalisasi.NewJadwal(false, 7, 1, 5))
	_, body := call(dgRouter("admin"), "GET", "/dg/sinkronisasi", "")
	oto, _ := dataOf(t, body)["otomatis"].(map[string]interface{})
	if oto["aktif"] != false || oto["berikutnya"] != nil {
		t.Errorf("dimatikan: otomatis = %v", oto)
	}
}

func TestSyncEndpoints(t *testing.T) {
	p, s := setupDG(t)
	var ids int64
	p.OnQuery = func(ctx context.Context, q string, args []driver.NamedValue) ([]string, [][]driver.Value, error) {
		if strings.HasPrefix(q, "INSERT INTO digitalisasi_sync_log") {
			return []string{"id"}, [][]driver.Value{{atomic.AddInt64(&ids, 1)}}, nil
		}
		return nil, nil, fmt.Errorf("query tak terduga: %s", q)
	}
	// Query SLDK "berjalan lama": baru kembali saat dibatalkan.
	s.OnQuery = func(ctx context.Context, q string, args []driver.NamedValue) ([]string, [][]driver.Value, error) {
		<-ctx.Done()
		return nil, nil, ctx.Err()
	}
	r := dgRouter("admin")

	for name, body := range map[string]string{"kosong": `{}`, "tak dikenal": `{"datasets":["../x"]}`, "bukan JSON": `xx`} {
		if code, _ := call(r, "POST", "/dg/sinkronisasi", body); code != 400 {
			t.Errorf("%s: %d, want 400", name, code)
		}
	}
	if p.Count("QUERY") != 0 {
		t.Fatal("permintaan tidak valid tidak boleh menulis riwayat")
	}

	code, body := call(r, "POST", "/dg/sinkronisasi", `{"datasets":["tanah"]}`)
	if code != 202 {
		t.Fatalf("mulai: %d %v", code, body)
	}
	aktif := dataOf(t, body)["aktif"].(map[string]interface{})
	if aktif["oleh"] != "tester" || len(aktif["datasets"].([]interface{})) != 1 {
		t.Errorf("aktif = %v", aktif)
	}
	if code, _ := call(r, "POST", "/dg/sinkronisasi", `{"datasets":["satker"]}`); code != 409 {
		t.Errorf("sinkronisasi kedua: %d, want 409", code)
	}

	code, body = call(r, "POST", "/dg/sinkronisasi/batal", "")
	if code != 200 || dataOf(t, body)["dibatalkan"] != true {
		t.Fatalf("batal: %d %v", code, body)
	}
	deadline := time.Now().Add(2 * time.Second)
	for digitalisasi.Default.Active() != nil {
		if time.Now().After(deadline) {
			t.Fatal("antrean tidak berhenti setelah dibatalkan")
		}
		time.Sleep(5 * time.Millisecond)
	}
	_, body = call(r, "POST", "/dg/sinkronisasi/batal", "")
	if dataOf(t, body)["dibatalkan"] != false {
		t.Error("batal tanpa antrean harus dibatalkan=false")
	}

	// "semua" memasukkan seluruh dataset.
	code, body = call(r, "POST", "/dg/sinkronisasi", `{"semua":true}`)
	if code != 202 || len(dataOf(t, body)["aktif"].(map[string]interface{})["datasets"].([]interface{})) != len(digitalisasi.Datasets) {
		t.Fatalf("semua: %d %v", code, body)
	}
	call(r, "POST", "/dg/sinkronisasi/batal", "")

	// Tanpa koneksi SLDK.
	for digitalisasi.Default.Active() != nil {
		time.Sleep(5 * time.Millisecond)
	}
	digitalisasi.Default = digitalisasi.NewManager(database.DB, nil)
	if code, _ := call(r, "POST", "/dg/sinkronisasi", `{"datasets":["tanah"]}`); code != 503 {
		t.Errorf("tanpa SLDK: %d, want 503", code)
	}
}

func TestHandlersWhenManagerNotInitialised(t *testing.T) {
	gin.SetMode(gin.TestMode)
	old := digitalisasi.Default
	digitalisasi.Default = nil
	defer func() { digitalisasi.Default = old }()
	r := dgRouter("admin")
	for _, tc := range [][2]string{{"GET", "/dg/ringkasan"}, {"GET", "/dg/sinkronisasi"}, {"POST", "/dg/sinkronisasi"}, {"POST", "/dg/sinkronisasi/batal"}} {
		if code, _ := call(r, tc[0], tc[1], `{"datasets":["tanah"]}`); code != 503 {
			t.Errorf("%s %s: %d, want 503", tc[0], tc[1], code)
		}
	}
}
