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

// Tes tabel rute API Akses Data tanpa database: setiap rute yang bukan formulir pendaftaran publik harus menolak permintaan tanpa kredensial yang sesuai.

func TestRuteAPIAksesTerlindungi(t *testing.T) {
	pasangCfgUji(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	_ = r.SetTrustedProxies(nil)
	SetupRoutes(r)
	l := apiakses.NewLayanan(nil, nil, nil, "pepper-uji")
	t.Cleanup(handlers.GunakanApiAkses(l))

	publik := map[string]bool{"GET /api/v1/api-akses/info": true, "POST /api/v1/api-akses/pendaftaran": true}
	var pengelolaan, klien int
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
		case strings.HasPrefix(rt.Path, "/api/v1/api-akses/") && !publik[kunci]:
			pengelolaan++
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
	if klien < 5 || pengelolaan < 12 {
		t.Errorf("rute tidak lengkap: klien %d, pengelolaan %d", klien, pengelolaan)
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

func TestPendaftaranPublikTidakMemberiKredensial(t *testing.T) {
	pasangCfgUji(t)
	// Formulir publik hanya menerima data pendaftaran; rute pengelolaan yang menerbitkan kunci tidak ada tanpa login superadmin (diuji di atas). Di sini: rute publik
	// tidak menerima metode lain.
	gin.SetMode(gin.TestMode)
	r := gin.New()
	SetupRoutes(r)
	for _, m := range []string{http.MethodPut, http.MethodDelete, http.MethodPatch} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(m, "/api/v1/api-akses/pendaftaran", strings.NewReader("{}")))
		if w.Code != http.StatusNotFound && w.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s /api-akses/pendaftaran: %d, want 404/405", m, w.Code)
		}
	}
}
