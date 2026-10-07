package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"pasti-v3-backend/peran"
)

// RequireCakupanSemuaPesan menolak peran yang hanya melihat sebagian data (UE1, Kanwil, Satker) dan tamu, dengan pesan yang diberikan pemasangnya. Dipakai untuk status
// dan riwayat sinkronisasi Digitalisasi Aset (tab Sinkronisasi) serta keadaan penarikan Pengadaan.
func TestRequireCakupanSemuaPesan(t *testing.T) {
	const pesan = "Status dan riwayat sinkronisasi hanya tersedia bagi Pengguna Barang dan superadmin."
	gin.SetMode(gin.TestMode)

	for _, c := range []struct {
		nama     string
		role     string
		baris    []peran.Baris
		diterima bool
	}{
		{"superadmin", "superadmin", nil, true},
		{"pengguna barang", "user", []peran.Baris{{ID: 1, Peran: peran.PenggunaBarang}}, true},
		{"UE1", "user", []peran.Baris{{ID: 1, Peran: peran.UE1, Kode: "01501"}}, false},
		{"Kanwil", "user", []peran.Baris{{ID: 1, Peran: peran.Kanwil, Kode: "015010199"}}, false},
		{"Satker", "user", []peran.Baris{{ID: 1, Peran: peran.Satker, Kode: "409294"}}, false},
		{"tamu (tanpa peran)", "user", nil, false},
	} {
		t.Run(c.nama, func(t *testing.T) {
			r := gin.New()
			r.GET("/x",
				func(g *gin.Context) { peran.Pasang(g, peran.Selesaikan(c.role, c.baris)) },
				RequireCakupanSemuaPesan(pesan),
				func(g *gin.Context) { g.JSON(http.StatusOK, gin.H{"ok": true}) })
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/x", nil))

			if c.diterima {
				if w.Code != http.StatusOK {
					t.Errorf("status = %d, want 200; badan: %s", w.Code, w.Body.String())
				}
				return
			}
			if w.Code != http.StatusForbidden {
				t.Fatalf("status = %d, want 403; badan: %s", w.Code, w.Body.String())
			}
			var m map[string]interface{}
			_ = json.Unmarshal(w.Body.Bytes(), &m)
			if m["message"] != pesan || strings.Contains(w.Body.String(), `"ok"`) {
				t.Errorf("badan = %s, want pesan khusus dan penangan tidak berjalan", w.Body.String())
			}
		})
	}
}

// RequireCakupanSemua (penarikan Pengadaan) tetap memakai pesannya sendiri.
func TestRequireCakupanSemuaPesanBawaan(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/x",
		func(g *gin.Context) {
			peran.Pasang(g, peran.Selesaikan("user", []peran.Baris{{ID: 1, Peran: peran.Satker, Kode: "409294"}}))
		},
		RequireCakupanSemua(),
		func(g *gin.Context) { g.JSON(http.StatusOK, gin.H{"ok": true}) })
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/x", nil))
	if w.Code != http.StatusForbidden || !strings.Contains(w.Body.String(), "penarikan data") {
		t.Errorf("status = %d, badan = %s", w.Code, w.Body.String())
	}
}
