package routes

import (
	"net/http"
	"sort"
	"testing"

	"github.com/gin-gonic/gin"

	"pasti-v3-backend/audit"
)

// Katalog audit harus tetap sejalan dengan tabel rute: aturan untuk rute yang sudah dihapus atau diganti nama menandakan katalog usang, dan rute baru yang mengubah data
// tanpa aturan akan tercatat dengan uraian generik ("Menjalankan POST ...") sehingga sulit dibaca di log. Tes ini memaksa pengembang menambah aturannya.
func TestKatalogAuditSesuaiTabelRute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	SetupRoutes(r)
	ada := map[string]bool{}
	var ubah []string
	for _, rt := range r.Routes() {
		kunci := rt.Method + " " + rt.Path
		ada[kunci] = true
		switch rt.Method {
		case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
			ubah = append(ubah, kunci)
		}
	}

	known := map[string]bool{}
	for _, k := range audit.KunciKatalog() {
		known[k] = true
		if !ada[k] {
			t.Errorf("katalog audit memuat rute yang tidak ada di tabel rute: %s", k)
		}
	}

	// Rute SAPA dipasang oleh RegisterSapa pada grup /api/v1/sapa (SetupRoutes sudah memanggilnya), jadi ikut terdaftar di atas.
	sort.Strings(ubah)
	for _, k := range ubah {
		if !known[k] {
			t.Errorf("rute pengubah data %s belum punya aturan di katalog audit (audit/katalog.go)", k)
		}
	}
}

// Setiap GET yang sengaja dicatat (ekspor, unduhan, pencarian pegawai) harus ada di katalog, dan GET biasa tidak boleh ikut tercatat tanpa sengaja.
func TestGETYangDicatatAdalahBacaanSensitif(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	SetupRoutes(r)
	sensitif := map[string]bool{}
	for _, k := range audit.KunciKatalog() {
		if len(k) > 4 && k[:4] == "GET " {
			sensitif[k] = true
		}
	}
	for k := range sensitif {
		bukanBaca := true
		for _, rt := range r.Routes() {
			if rt.Method+" "+rt.Path == k {
				bukanBaca = false
			}
		}
		if bukanBaca {
			t.Errorf("GET %s ada di katalog tetapi tidak ada rutenya", k)
		}
	}
	// Daftar GET yang dicatat sengaja dibatasi pada ekspor/unduhan/pencarian pegawai; menambahnya harus keputusan sadar (ubah daftar ini bersama katalog).
	diharapkan := []string{
		"GET /api/v1/audit/ekspor", "GET /api/v1/digitalisasi/ekspor/:dataset", "GET /api/v1/hris2/pegawai/by-nip/:nip", "GET /api/v1/hris2/pegawai/search",
		"GET /api/v1/inaproc/ekspor/:awalan/:nama", "GET /api/v1/sapa/dokumen/:id/unduh", "GET /api/v1/sapa/pegawai", "GET /api/v1/sapa/pegawai/:nip", "GET /api/v1/sapa/template/:kunci/unduh",
	}
	var got []string
	for k := range sensitif {
		got = append(got, k)
	}
	sort.Strings(got)
	if len(got) != len(diharapkan) {
		t.Fatalf("GET yang dicatat = %v, want %v", got, diharapkan)
	}
	for i := range got {
		if got[i] != diharapkan[i] {
			t.Errorf("GET yang dicatat[%d] = %s, want %s", i, got[i], diharapkan[i])
		}
	}
}
