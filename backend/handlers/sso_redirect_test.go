package handlers

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"pasti-v3-backend/config"
)

// Kegagalan SSO dikembalikan ke /kembali-masuk, bukan /login: alamat halaman login frontend disembunyikan, dan frontend yang
// meneruskan peramban ke sana. Jalur galat ini tidak menyentuh database.
func TestSSOCallbackGalatKembaliKeJalurKembaliMasuk(t *testing.T) {
	gin.SetMode(gin.TestMode)
	lama := config.Cfg
	config.Cfg = &config.Config{FrontendURL: "https://pasti.example.test"}
	t.Cleanup(func() { config.Cfg = lama })

	r := gin.New()
	r.GET("/sso/callback/login", SSOCallback)

	for _, c := range []struct{ nama, query, alasan string }{
		{"ditolak SSO", "?error=access_denied", "Kemenkeu menolak: access_denied"},
		{"parameter kurang", "?code=abc", "Parameter tidak lengkap dari SSO"},
	} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/sso/callback/login"+c.query, nil))
		if w.Code != http.StatusFound {
			t.Fatalf("%s: status = %d, want 302", c.nama, w.Code)
		}
		loc := w.Header().Get("Location")
		u, err := url.Parse(loc)
		if err != nil {
			t.Fatalf("%s: Location tidak sah: %q", c.nama, loc)
		}
		if u.Scheme+"://"+u.Host != "https://pasti.example.test" || u.Path != "/kembali-masuk" {
			t.Errorf("%s: Location = %q, want https://pasti.example.test/kembali-masuk?...", c.nama, loc)
		}
		if strings.Contains(loc, "/login") {
			t.Errorf("%s: Location memuat /login: %q", c.nama, loc)
		}
		if u.Query().Get("error") != "sso_failed" || u.Query().Get("reason") != c.alasan {
			t.Errorf("%s: query = %v", c.nama, u.Query())
		}
	}
}
