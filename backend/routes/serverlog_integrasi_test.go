package routes

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"pasti-v3-backend/handlers"
	"pasti-v3-backend/serverlog"
)

// Tes integrasi SQL Server (hanya bila PASTI_UJI_MSSQL_DSN diisi): log server hanya untuk superadmin, hanya galat dan peringatan yang tersimpan (info/debug dan permintaan
// normal tidak), kejadian yang berulang digabung, panik tercatat lengkap dengan jejaknya, dan tidak ada rute untuk mengubah atau menghapus log.

func panggilLogServer(t *testing.T, method, path, token string) (int, []byte) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(serverlog.Middleware(time.Hour), serverlog.Pemulih())
	SetupRoutes(r)
	r.GET("/panik-uji", func(c *gin.Context) { panic("uji panik log server") })
	r.GET("/galat-uji", func(c *gin.Context) { c.String(500, "rusak") })
	r.GET("/biasa-uji", func(c *gin.Context) { c.String(200, "ok") })
	req := httptest.NewRequest(method, path, bytes.NewBufferString(""))
	req.RemoteAddr = "198.51.100.8:5555"
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w.Code, w.Body.Bytes()
}

func TestLogServerHanyaSuperadminDanHanyaGalatTersimpan(t *testing.T) {
	db := bukaDBUji(t)
	bersih := func() {
		db.Exec(`DELETE FROM server_log WHERE sumber LIKE N'UJI LOGSERVER%' OR (sumber = N'PANIK' AND pesan LIKE N'%uji panik log server%') OR rute IN (N'/galat-uji')`)
		bersihkanUjiPeran(db)
	}
	bersih()
	t.Cleanup(bersih)

	var konsol bytes.Buffer
	serverlog.Pasang(&konsol, serverlog.Warn, serverlog.SimpanGalatDanPeringatan)
	store := serverlog.Mulai(db)
	t.Cleanup(func() {
		serverlog.Berhenti(context.Background())
		serverlog.Pasang(os.Stderr, serverlog.Warn, serverlog.SimpanMati)
		log.SetOutput(os.Stderr)
	})
	t.Cleanup(handlers.GunakanServerLogStore(store))

	admin := buatPenggunaUji(t, db, "logsrv-admin", "superadmin")
	barang := buatPenggunaUji(t, db, "logsrv-barang", "user")
	tamu := buatPenggunaUji(t, db, "logsrv-tamu", "user")
	beriPeran(t, admin, barang, "pengguna_barang", "")

	// --- akses: hanya superadmin
	rute := []string{"/api/v1/log-server", "/api/v1/log-server/ringkasan", "/api/v1/log-server/opsi"}
	for _, p := range rute {
		if c, _ := panggilLogServer(t, "GET", p, ""); c != http.StatusUnauthorized {
			t.Errorf("GET %s tanpa login: %d, want 401", p, c)
		}
		if c, _ := panggilLogServer(t, "GET", p, tamu.token); c != http.StatusForbidden {
			t.Errorf("GET %s oleh tamu: %d, want 403", p, c)
		}
		if c, _ := panggilLogServer(t, "GET", p, barang.token); c != http.StatusForbidden {
			t.Errorf("GET %s oleh Pengguna Barang: %d, want 403 (hanya superadmin)", p, c)
		}
	}
	if c, _ := panggilLogServer(t, "GET", "/api/v1/log-server/1", barang.token); c != http.StatusForbidden {
		t.Errorf("detail oleh Pengguna Barang: %d, want 403", c)
	}
	// Superadmin yang sedang bertindak sebagai peran data kehilangan hak ini selama peran itu aktif.
	idSatker := beriPeran(t, admin, admin, "satker", "409294")
	pilihPeran(t, admin, idSatker)
	if c, _ := panggilLogServer(t, "GET", "/api/v1/log-server", admin.token); c != http.StatusForbidden {
		t.Errorf("superadmin yang bertindak sebagai Satker: %d, want 403", c)
	}
	pilihPeran(t, admin, nil)

	// --- hanya galat dan peringatan yang tersimpan
	for i := 0; i < 40; i++ { // 40 kali galat yang sama (angka berbeda) -> satu baris
		log.Printf("[UJI LOGSERVER ERROR] gagal menarik halaman %d dari layanan luar", i)
	}
	log.Println("[UJI LOGSERVER WARN] respons layanan luar lambat")
	log.Println("[UJI LOGSERVER DEBUG] rincian debug yang tidak boleh tersimpan")
	log.Println("[UJI LOGSERVER] kemajuan biasa yang tidak boleh tersimpan")
	log.Println("[UJI LOGSERVER INFO] info yang tidak boleh tersimpan")
	for i := 0; i < 30; i++ { // permintaan normal dan 4xx: tidak dicatat
		panggilLogServer(t, "GET", "/biasa-uji", admin.token)
		panggilLogServer(t, "GET", "/api/v1/auth/me", "")
	}
	for i := 0; i < 6; i++ {
		panggilLogServer(t, "GET", "/galat-uji", admin.token) // 500 -> error
	}
	if c, b := panggilLogServer(t, "GET", "/panik-uji", admin.token); c != 500 || !strings.Contains(string(b), "Terjadi kesalahan") {
		t.Fatalf("panik harus dijawab 500 yang rapi: %d %s", c, b)
	}
	serverlog.Berhenti(context.Background()) // siram antrean ke database

	baca := func(path string) map[string]interface{} {
		t.Helper()
		c, b := panggilLogServer(t, "GET", path, admin.token)
		if c != http.StatusOK {
			t.Fatalf("GET %s: %d %s", path, c, b)
		}
		var o struct {
			Data map[string]interface{} `json:"data"`
		}
		if err := json.Unmarshal(b, &o); err != nil {
			t.Fatal(err)
		}
		return o.Data
	}

	d := baca("/api/v1/log-server?sumber=UJI%20LOGSERVER&per_halaman=50")
	baris := d["baris"].([]interface{})
	if len(baris) != 2 {
		t.Fatalf("hanya satu baris error (digabung) dan satu warning yang tersimpan, dapat %d: %v", len(baris), baris)
	}
	var adaError, adaWarn bool
	for _, x := range baris {
		b := x.(map[string]interface{})
		switch b["level"] {
		case "error":
			adaError = true
			if b["jumlah"].(float64) != 40 {
				t.Errorf("40 kejadian yang sama harus digabung menjadi jumlah 40: %v", b["jumlah"])
			}
		case "warn":
			adaWarn = true
		default:
			t.Errorf("level tidak boleh %v", b["level"])
		}
		if strings.Contains(strings.ToLower(b["pesan"].(string)), "debug") || strings.Contains(strings.ToLower(b["pesan"].(string)), "kemajuan") {
			t.Errorf("pesan info/debug tersimpan: %v", b["pesan"])
		}
	}
	if !adaError || !adaWarn {
		t.Fatalf("error dan warning harus ada: %v", baris)
	}
	if strings.Contains(konsol.String(), "rincian debug") || strings.Contains(konsol.String(), "kemajuan biasa") {
		t.Errorf("pada level warn, info/debug tidak boleh dicetak ke konsol:\n%s", konsol.String())
	}

	// Respons 500 tercatat sebagai satu kelompok, dengan rute (pola), metode, dan status.
	g := baca("/api/v1/log-server?q=galat-uji")
	gb := g["baris"].([]interface{})
	if len(gb) != 1 {
		t.Fatalf("6 respons 500 yang sama harus jadi satu baris: %v", gb)
	}
	b0 := gb[0].(map[string]interface{})
	if b0["jumlah"].(float64) != 6 || b0["rute"] != "/galat-uji" || b0["metode"] != "GET" || b0["status"] != float64(500) {
		t.Errorf("rincian respons 500: %v", b0)
	}
	// Permintaan normal tidak tercatat sama sekali.
	if n := baca("/api/v1/log-server?q=biasa-uji")["total"].(float64); n != 0 {
		t.Errorf("permintaan normal tidak boleh tercatat: %v", n)
	}

	// Panik: tercatat dengan jejak tumpukan di detail.
	p := baca("/api/v1/log-server?sumber=PANIK&q=uji%20panik%20log%20server")
	pb := p["baris"].([]interface{})
	if len(pb) != 1 {
		t.Fatalf("panik harus tercatat satu kali: %v", pb)
	}
	id := int64(pb[0].(map[string]interface{})["id"].(float64))
	c, raw := panggilLogServer(t, "GET", "/api/v1/log-server/"+itoa(id), admin.token)
	if c != 200 || !strings.Contains(string(raw), "serverlog_integrasi_test.go") {
		t.Errorf("detail panik harus memuat jejak tumpukan: %d %s", c, string(raw))
	}
	if c, _ := panggilLogServer(t, "GET", "/api/v1/log-server/999999999", admin.token); c != http.StatusNotFound {
		t.Errorf("id tidak ada: %d, want 404", c)
	}
	if c, _ := panggilLogServer(t, "GET", "/api/v1/log-server/abc", admin.token); c != http.StatusBadRequest {
		t.Errorf("id tidak valid: %d, want 400", c)
	}

	// Filter level dan validasi parameter.
	if n := baca("/api/v1/log-server?level=warn&sumber=UJI%20LOGSERVER")["total"].(float64); n != 1 {
		t.Errorf("filter warn: %v", n)
	}
	for _, q := range []string{"?level=info", "?level=debug", "?dari=bukan-tanggal", "?dari=2026-10-08&sampai=2026-10-01"} {
		if c, _ := panggilLogServer(t, "GET", "/api/v1/log-server"+q, admin.token); c != http.StatusBadRequest {
			t.Errorf("%s: %d, want 400 (hanya error/warn dan rentang valid)", q, c)
		}
	}

	// Ringkasan memuat keadaan perekam, pengaturan, dan tekanan beban.
	r := baca("/api/v1/log-server/ringkasan")
	for _, k := range []string{"ringkasan", "perekam", "pengaturan", "ketahanan"} {
		if r[k] == nil {
			t.Errorf("ringkasan tanpa %s", k)
		}
	}
	if o := baca("/api/v1/log-server/opsi"); o["sumber"] == nil || len(o["level"].([]interface{})) != 2 {
		t.Errorf("opsi: %v", o)
	}

	// Tidak ada rute untuk mengubah atau menghapus log dari API.
	for _, m := range []string{"POST", "PUT", "PATCH", "DELETE"} {
		for _, pth := range []string{"/api/v1/log-server", "/api/v1/log-server/1"} {
			if c, _ := panggilLogServer(t, m, pth, admin.token); c != http.StatusNotFound && c != http.StatusMethodNotAllowed {
				t.Errorf("%s %s: %d, want 404/405 (log tidak boleh diubah dari API)", m, pth, c)
			}
		}
	}
}

func itoa(n int64) string {
	b, _ := json.Marshal(n)
	return string(b)
}
