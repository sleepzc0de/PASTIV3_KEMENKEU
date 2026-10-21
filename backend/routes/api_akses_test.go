package routes

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"pasti-v3-backend/apiakses"
	"pasti-v3-backend/handlers"
)

// Tes tabel rute API Akses Data tanpa database: TIDAK ADA rute publik (pendaftaran klien hanya oleh superadmin); setiap rute pengelolaan menolak permintaan tanpa login, dan
// setiap rute klien menolak permintaan tanpa kunci API.

func TestRuteAPIAksesTerlindungi(t *testing.T) {
	pasangCfgUji(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	_ = r.SetTrustedProxies(nil)
	SetupRoutes(r)
	l := apiakses.NewLayanan(nil, nil, nil, "pepper-uji")
	t.Cleanup(handlers.GunakanApiAkses(l))

	var pengelolaan, klien int
	var adaBuatKlien bool
	for _, rt := range r.Routes() {
		kunci := rt.Method + " " + rt.Path
		switch {
		case strings.HasPrefix(rt.Path, apiakses.AwalanRute):
			klien++
			if rt.Method != http.MethodGet {
				t.Errorf("%s: API klien hanya boleh membaca (GET)", kunci)
			}
			// Tanpa kunci apa pun: 401, tidak pernah 200/404/500 (gerbang terpasang di setiap rute).
			w := httptest.NewRecorder()
			req := httptest.NewRequest(rt.Method, strings.NewReplacer(":dataset", "tanah", ":id", "1").Replace(rt.Path), nil)
			r.ServeHTTP(w, req)
			if w.Code != http.StatusUnauthorized {
				t.Errorf("%s tanpa kunci: %d, want 401", kunci, w.Code)
			}
			// Token pengguna (JWT) bukan kunci API.
			w = httptest.NewRecorder()
			req = httptest.NewRequest(rt.Method, strings.NewReplacer(":dataset", "tanah", ":id", "1").Replace(rt.Path), nil)
			req.Header.Set("Authorization", "Bearer eyJhbGciOiJIUzI1NiJ9.eyJyb2xlIjoic3VwZXJhZG1pbiJ9.x")
			r.ServeHTTP(w, req)
			if w.Code != http.StatusUnauthorized {
				t.Errorf("%s dengan JWT: %d, want 401", kunci, w.Code)
			}
		case strings.HasPrefix(rt.Path, "/api/v1/api-akses/"):
			pengelolaan++
			if kunci == "POST /api/v1/api-akses/klien" {
				adaBuatKlien = true
			}
			p := strings.NewReplacer(":id", "1", ":kunciId", "1").Replace(rt.Path)
			// Kunci API (atau apa pun selain JWT pengguna) tidak membuka rute pengelolaan.
			for _, auth := range []string{"", "Bearer pasti_aaaaaaaaaa_" + strings.Repeat("A", 43), "Bearer x"} {
				w := httptest.NewRecorder()
				req := httptest.NewRequest(rt.Method, p, strings.NewReader("{}"))
				if auth != "" {
					req.Header.Set("Authorization", auth)
				}
				r.ServeHTTP(w, req)
				if w.Code != http.StatusUnauthorized {
					t.Errorf("%s dengan %q: %d, want 401", kunci, auth, w.Code)
				}
			}
		}
	}
	if klien < 5 || pengelolaan < 13 || !adaBuatKlien {
		t.Errorf("rute tidak lengkap: klien %d, pengelolaan %d, POST /klien %v", klien, pengelolaan, adaBuatKlien)
	}
}

func TestKunciAPITidakMembukaRutePengguna(t *testing.T) {
	pasangCfgUji(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	SetupRoutes(r)
	kunci := "pasti_" + strings.Repeat("a", 10) + "_" + strings.Repeat("B", 43)
	for _, p := range []string{"/api/v1/users", "/api/v1/audit", "/api/v1/monitor/ringkasan", "/api/v1/digitalisasi/data/tanah", "/api/v1/inaproc/dataset", "/api/v1/auth/me", "/api/v1/sapa/penjualan"} {
		for _, hdr := range []string{"Authorization", "X-API-Key"} {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, p, nil)
			if hdr == "Authorization" {
				req.Header.Set(hdr, "Bearer "+kunci)
			} else {
				req.Header.Set(hdr, kunci)
			}
			r.ServeHTTP(w, req)
			if w.Code != http.StatusUnauthorized {
				t.Errorf("GET %s dengan kunci API lewat %s: %d, want 401", p, hdr, w.Code)
			}
		}
	}
}

// Formulir dan rute pendaftaran publik sudah ditiadakan: permukaan publiknya tidak boleh ada lagi (bukan hanya tersembunyi).
func TestRutePendaftaranPublikTidakAda(t *testing.T) {
	pasangCfgUji(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	SetupRoutes(r)
	l := apiakses.NewLayanan(nil, nil, nil, "pepper-uji")
	t.Cleanup(handlers.GunakanApiAkses(l))

	// Tidak ada rute dengan jalur pendaftaran/info publik pada tabel rute.
	for _, rt := range r.Routes() {
		if rt.Path == "/api/v1/api-akses/info" || rt.Path == "/api/v1/api-akses/pendaftaran" || strings.Contains(rt.Path, "pendaftaran") {
			t.Errorf("rute publik lama masih ada: %s %s", rt.Method, rt.Path)
		}
	}
	// Dan memanggilnya (dengan badan yang dulu sah) menghasilkan 404, untuk semua metode.
	badan := `{"nama_aplikasi":"Aplikasi Uji","instansi":"Instansi Uji","pic_nama":"Budi","pic_email":"b@uji.go.id","tujuan":"Menarik data aset untuk pengujian internal sistem.","dataset_diminta":["aset.tanah"],"captcha_id":"x","captcha_answer":"1"}`
	for _, m := range []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch} {
		for _, p := range []string{"/api/v1/api-akses/info", "/api/v1/api-akses/pendaftaran"} {
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(m, p, strings.NewReader(badan)))
			if w.Code != http.StatusNotFound {
				t.Errorf("%s %s: %d, want 404", m, p, w.Code)
			}
		}
	}
	// Pembuatan klien tanpa login ditolak (401): hanya superadmin yang dapat mendaftarkan.
	for _, auth := range []string{"", "Bearer x", "Bearer pasti_aaaaaaaaaa_" + strings.Repeat("A", 43)} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/api-akses/klien", strings.NewReader(badan))
		req.Header.Set("Content-Type", "application/json")
		if auth != "" {
			req.Header.Set("Authorization", auth)
		}
		r.ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("POST /api-akses/klien dengan %q: %d, want 401", auth, w.Code)
		}
	}
}
