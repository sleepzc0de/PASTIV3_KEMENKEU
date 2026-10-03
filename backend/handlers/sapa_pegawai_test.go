package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/gin-gonic/gin"

	"pasti-v3-backend/config"
	"pasti-v3-backend/sapa"
)

// Tes pencarian pegawai HRIS2 untuk SAPA dengan HRIS2 palsu (httptest): memeriksa bidang yang diteruskan (tanpa data
// pribadi), pembersihan masukan pencarian, hak akses, dan pemetaan galat.

type hrisPalsu struct {
	t         *testing.T
	srv       *httptest.Server
	cari      func(w http.ResponseWriter, r *http.Request)
	detail    func(w http.ResponseWriter, r *http.Request)
	panggilan int32
	filters   []string
	auth      []string
}

func newHRISPalsu(t *testing.T) *hrisPalsu {
	h := &hrisPalsu{t: t}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/Profile/GetAllPegawai", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&h.panggilan, 1)
		h.filters = append(h.filters, r.URL.Query().Get("Filters"))
		h.auth = append(h.auth, r.Header.Get("Authorization"))
		if r.URL.Query().Get("PageSize") != "15" {
			t.Errorf("PageSize = %q", r.URL.Query().Get("PageSize"))
		}
		h.cari(w, r)
	})
	mux.HandleFunc("/api/Profile/GetPegawai", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&h.panggilan, 1)
		h.auth = append(h.auth, r.Header.Get("Authorization"))
		h.detail(w, r)
	})
	h.srv = httptest.NewServer(mux)
	t.Cleanup(h.srv.Close)
	return h
}

func tulis(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	fmt.Fprint(w, body)
}

type lingkunganPegawai struct {
	t        *testing.T
	r        *gin.Engine
	hris     *hrisPalsu
	repo     *sapa.MemRepo
	prov     string // jenis akun pengguna uji
	tokenErr error
}

const guidPeg = "00000000-0000-4000-8000-0000000000aa"

func siapkanPegawai(t *testing.T) *lingkunganPegawai {
	t.Helper()
	gin.SetMode(gin.TestMode)
	oldCfg := config.Cfg
	config.Cfg = &config.Config{HTTPClientTimeoutSeconds: 5}
	t.Cleanup(func() { config.Cfg = oldCfg })

	e := &lingkunganPegawai{t: t, hris: newHRISPalsu(t), repo: sapa.NewMemRepo(), prov: "sso"}
	e.repo.Pengguna = []sapa.PeranRow{{UserID: guidPeg, Username: "budi", Nama: "Budi"}}
	if err := e.repo.SimpanPeran(context.Background(), guidPeg, sapa.PeranSatker, "015010199409294002", "", "seed"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(GunakanSapaLayanan(&sapa.Layanan{Repo: e.repo}))
	t.Cleanup(GunakanHRIS2(
		func() string { return e.hris.srv.URL },
		func(string) (string, error) {
			if e.tokenErr != nil {
				return "", e.tokenErr
			}
			return "TOKEN-SSO", nil
		},
		func(string) (string, error) { return e.prov, nil },
	))

	e.r = gin.New()
	e.r.Use(func(c *gin.Context) {
		c.Set("user_id", guidPeg)
		c.Set("username", "budi")
		c.Set("role", "user")
		c.Next()
	})
	e.r.GET("/sapa/pegawai", SearchSapaPegawai)
	e.r.GET("/sapa/pegawai/:nip", GetSapaPegawai)
	return e
}

func (e *lingkunganPegawai) get(path string) (int, map[string]interface{}, string) {
	e.t.Helper()
	w := httptest.NewRecorder()
	e.r.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
	_, m := decodeJSON(w.Body.Bytes())
	return w.Code, m, w.Body.String()
}

func TestCariPegawaiHanyaMeneruskanBidangYangDibutuhkan(t *testing.T) {
	e := siapkanPegawai(t)
	e.hris.cari = func(w http.ResponseWriter, r *http.Request) {
		tulis(w, 200, `{"statusCode":200,"isError":false,"data":{"items":[
			{"nama":"Budi Santoso","nip18":"198001012005011001","gelarDepan":"Dr.","gelarBelakang":"S.Kom., M.M.","namaSatker":"KPKNL Jakarta I","namaJabatan":"Kepala Seksi A",
			 "tanggalLahir":"1980-01-01","noHp":"081234567890","email":"rahasia@x.go.id","tempatLahir":"Bogor","gravatar":"http://foto"},
			{"nama":"Siti","nip18":"199001012015022002","namaSatker":"KPKNL Jakarta I","jabatan":[{"namaJabatan":"Pelaksana"}]},
			{"nip18":"1","namaSatker":"tanpa nama dilewati"}
		]}}`)
	}
	kode, m, body := e.get("/sapa/pegawai?q=Budi+Santoso")
	if kode != 200 {
		t.Fatalf("status %d: %s", kode, body)
	}
	for _, rahasia := range []string{"1980-01-01", "081234567890", "rahasia@x.go.id", "Bogor", "foto", "tanggalLahir"} {
		if strings.Contains(body, rahasia) {
			t.Errorf("data pribadi %q ikut diteruskan: %s", rahasia, body)
		}
	}
	list := m["data"].(map[string]interface{})["pegawai"].([]interface{})
	if len(list) != 2 {
		t.Fatalf("hasil = %v", list)
	}
	p := list[0].(map[string]interface{})
	if p["nip"] != "198001012005011001" || p["nama"] != "Budi Santoso" || p["nama_lengkap"] != "Dr. Budi Santoso, S.Kom., M.M." || p["jabatan"] != "Kepala Seksi A" || p["satker"] != "KPKNL Jakarta I" {
		t.Errorf("pegawai 1 = %v", p)
	}
	if list[1].(map[string]interface{})["jabatan"] != "Pelaksana" {
		t.Errorf("jabatan dari larik jabatan: %v", list[1])
	}
	if e.hris.filters[0] != "(Nama|Nip18)@=*Budi Santoso" || e.hris.auth[0] != "Bearer TOKEN-SSO" {
		t.Errorf("filter = %q auth = %q", e.hris.filters[0], e.hris.auth[0])
	}
}

func TestCariPegawaiMembersihkanMasukan(t *testing.T) {
	e := siapkanPegawai(t)
	e.hris.cari = func(w http.ResponseWriter, r *http.Request) { tulis(w, 200, `{"data":[]}`) }
	// Karakter khusus filter (koma, |, kurung, \, @, =, *) tidak boleh lolos sehingga filter tidak bisa diubah.
	_, _, _ = e.get("/sapa/pegawai?q=" + url.QueryEscape(`budi,Nama@=*x|(y)\z`))
	if len(e.hris.filters) != 1 {
		t.Fatalf("filter = %v", e.hris.filters)
	}
	isi := strings.TrimPrefix(e.hris.filters[0], "(Nama|Nip18)@=*")
	if strings.ContainsAny(isi, ",|()\\@=*") {
		t.Errorf("isi filter masih memuat karakter khusus: %q", e.hris.filters[0])
	}
	if !strings.Contains(isi, "budi") || !strings.Contains(isi, "Nama") {
		t.Errorf("kata yang sah harus tetap ada: %q", isi)
	}

	for _, q := range []string{"", "ab", "  a  ", "a b", "!!!", ",,,|||"} {
		before := atomic.LoadInt32(&e.hris.panggilan)
		kode, _, _ := e.get("/sapa/pegawai?q=" + url.QueryEscape(q))
		if kode != 400 || atomic.LoadInt32(&e.hris.panggilan) != before {
			t.Errorf("q=%q: status %d, HRIS dipanggil=%v", q, kode, atomic.LoadInt32(&e.hris.panggilan) != before)
		}
	}
	// NIP (angka) boleh dicari.
	if kode, _, _ := e.get("/sapa/pegawai?q=1980010120"); kode != 200 {
		t.Errorf("pencarian NIP: %d", kode)
	}
}

func TestCariPegawaiBentukResponsLonggar(t *testing.T) {
	cases := map[string]string{
		"larik langsung": `[{"nama":"A","nip18":"1"}]`,
		"data larik":     `{"data":[{"nama":"A","nip18":"1"}]}`,
		"data items":     `{"data":{"items":[{"nama":"A","nip18":"1"}]}}`,
		"data result":    `{"data":{"result":[{"nama":"A","nip18":"1"}]}}`,
		"objek tunggal":  `{"data":{"nama":"A","nip18":"1"}}`,
		"kosong":         `{"data":{"totalCount":0}}`,
		"data null":      `{"data":null}`,
	}
	for nama, body := range cases {
		e := siapkanPegawai(t)
		e.hris.cari = func(w http.ResponseWriter, r *http.Request) { tulis(w, 200, body) }
		kode, m, raw := e.get("/sapa/pegawai?q=abc")
		if kode != 200 {
			t.Errorf("%s: status %d %s", nama, kode, raw)
			continue
		}
		n := len(m["data"].(map[string]interface{})["pegawai"].([]interface{}))
		want := 1
		if nama == "kosong" || nama == "data null" {
			want = 0
		}
		if n != want {
			t.Errorf("%s: %d hasil, want %d", nama, n, want)
		}
	}
}

func TestCariPegawaiMembatasiJumlah(t *testing.T) {
	e := siapkanPegawai(t)
	e.hris.cari = func(w http.ResponseWriter, r *http.Request) {
		var items []string
		for i := 0; i < 40; i++ {
			items = append(items, fmt.Sprintf(`{"nama":"P%d","nip18":"%018d"}`, i, i))
		}
		tulis(w, 200, `{"data":[`+strings.Join(items, ",")+`]}`)
	}
	_, m, _ := e.get("/sapa/pegawai?q=abc")
	if n := len(m["data"].(map[string]interface{})["pegawai"].([]interface{})); n != 15 {
		t.Errorf("hasil = %d, want 15", n)
	}
}

func TestPegawaiHakAksesDanGalat(t *testing.T) {
	// Tanpa peran SAPA: ditolak sebelum HRIS2 dipanggil.
	e := siapkanPegawai(t)
	_ = e.repo.SimpanPeran(context.Background(), guidPeg, "", "", "", "seed")
	e.hris.cari = func(w http.ResponseWriter, r *http.Request) { tulis(w, 200, `{"data":[]}`) }
	e.hris.detail = e.hris.cari
	for _, p := range []string{"/sapa/pegawai?q=budi", "/sapa/pegawai/198001012005011001"} {
		kode, m, _ := e.get(p)
		if kode != 403 || m["code"] != "sapa_tanpa_akses" || atomic.LoadInt32(&e.hris.panggilan) != 0 {
			t.Errorf("%s tanpa peran: %d %v dipanggil=%d", p, kode, m["code"], e.hris.panggilan)
		}
	}

	// Akun lokal (bukan SSO).
	e = siapkanPegawai(t)
	e.prov = "local"
	e.hris.cari = func(w http.ResponseWriter, r *http.Request) { t.Error("HRIS2 tidak boleh dipanggil untuk akun lokal") }
	if kode, m, _ := e.get("/sapa/pegawai?q=budi"); kode != 403 || !strings.Contains(m["message"].(string), "SSO") {
		t.Errorf("akun lokal: %d %v", kode, m)
	}

	// Token SSO habis.
	e = siapkanPegawai(t)
	e.tokenErr = errors.New("token SSO sudah kedaluwarsa")
	if kode, m, _ := e.get("/sapa/pegawai?q=budi"); kode != 401 || m["code"] != "sso_session_expired" {
		t.Errorf("token habis: %d %v", kode, m)
	}

	// HRIS2 menolak token, galat server, galat di amplop, bukan JSON, tidak terjangkau.
	for nama, c := range map[string]struct {
		handler func(w http.ResponseWriter, r *http.Request)
		kode    int
		code    interface{}
	}{
		"ditolak":    {func(w http.ResponseWriter, r *http.Request) { tulis(w, 401, `{}`) }, 401, "sso_session_expired"},
		"galat 500":  {func(w http.ResponseWriter, r *http.Request) { tulis(w, 500, `rahasia internal`) }, 502, nil},
		"isError":    {func(w http.ResponseWriter, r *http.Request) { tulis(w, 200, `{"isError":true,"message":"rahasia"}`) }, 502, nil},
		"bukan JSON": {func(w http.ResponseWriter, r *http.Request) { tulis(w, 200, `<html>`) }, 502, nil},
	} {
		e := siapkanPegawai(t)
		e.hris.cari = c.handler
		kode, m, raw := e.get("/sapa/pegawai?q=budi")
		if kode != c.kode || (c.code != nil && m["code"] != c.code) {
			t.Errorf("%s: %d %v", nama, kode, m)
		}
		if strings.Contains(raw, "rahasia") {
			t.Errorf("%s: isi respons HRIS2 bocor ke klien: %s", nama, raw)
		}
	}
}

func TestDetailPegawai(t *testing.T) {
	e := siapkanPegawai(t)
	e.hris.detail = func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("nip") != "198001012005011001" {
			t.Errorf("nip = %q", r.URL.Query().Get("nip"))
		}
		tulis(w, 200, `{"statusCode":200,"isError":false,"data":{"nama":"Budi Santoso","nip18":"198001012005011001","gelarDepan":"","gelarBelakang":"S.E.",
			"namaSatker":"KPKNL Jakarta I","jabatan":[{"namaJabatan":"Kepala Seksi Pelayanan Lelang","statusJabatan":"Definitif"}],
			"tanggalLahir":"1980-01-01","noHp":"0812","email":"x@y"}}`)
	}
	kode, m, body := e.get("/sapa/pegawai/198001012005011001")
	if kode != 200 {
		t.Fatalf("status %d: %s", kode, body)
	}
	d := m["data"].(map[string]interface{})
	if d["jabatan"] != "Kepala Seksi Pelayanan Lelang" || d["nama_lengkap"] != "Budi Santoso, S.E." || d["nip"] != "198001012005011001" || d["satker"] != "KPKNL Jakarta I" {
		t.Errorf("detail = %v", d)
	}
	for _, rahasia := range []string{"1980-01-01", "0812", "x@y", "Definitif"} {
		if strings.Contains(body, rahasia) {
			t.Errorf("bidang %q ikut diteruskan: %s", rahasia, body)
		}
	}

	// NIP tidak sah tidak sampai ke HRIS2.
	sebelum := atomic.LoadInt32(&e.hris.panggilan)
	for _, nip := range []string{"abc", "123", "1980010120050110011234567", "198001012005011001;DROP", "../x"} {
		if kode, _, _ := e.get("/sapa/pegawai/" + url.PathEscape(nip)); kode != 400 && kode != 404 {
			t.Errorf("nip %q: status %d", nip, kode)
		}
	}
	if atomic.LoadInt32(&e.hris.panggilan) != sebelum {
		t.Error("NIP tidak sah tidak boleh diteruskan ke HRIS2")
	}

	// Tidak ditemukan: 404 dari HRIS2, atau data kosong.
	e.hris.detail = func(w http.ResponseWriter, r *http.Request) { tulis(w, 404, `{}`) }
	if kode, _, _ := e.get("/sapa/pegawai/198001012005011001"); kode != 404 {
		t.Errorf("404 HRIS2 -> %d", kode)
	}
	e.hris.detail = func(w http.ResponseWriter, r *http.Request) { tulis(w, 200, `{"isError":false,"data":{}}`) }
	if kode, _, _ := e.get("/sapa/pegawai/198001012005011001"); kode != 404 {
		t.Errorf("data kosong -> %d", kode)
	}
}

func TestBersihQueryPegawai(t *testing.T) {
	cases := map[string]string{
		"  Budi   Santoso ": "Budi Santoso",
		"Dr. Budi-Santoso":  "Dr. Budi-Santoso",
		"O'Brien":           "O'Brien",
		"a,b|c(d)e\\f":      "a b c d e f",
		"(Nama|Nip18)@=*x":  "Nama Nip18 x",
		"Évreux Ünal":       "Évreux Ünal",
		"1980 0101":         "1980 0101",
		"<script>alert(1)":  "script alert 1",
	}
	for in, want := range cases {
		if got := bersihQueryPegawai(in); got != want {
			t.Errorf("bersihQueryPegawai(%q) = %q, want %q", in, got, want)
		}
	}
}

func decodeJSON(b []byte) (interface{}, map[string]interface{}) {
	var raw interface{}
	_ = json.Unmarshal(b, &raw)
	m, _ := raw.(map[string]interface{})
	return raw, m
}
