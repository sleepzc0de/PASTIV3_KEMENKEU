package middleware

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"pasti-v3-backend/config"
	"pasti-v3-backend/peran"
	"pasti-v3-backend/utils"
)

// Tes gerbang autentikasi tanpa database: keadaan akun dan peran diganti lewat variabel muatStatusAkun dan muatPeran.

type kondisiAkun struct {
	status StatusAkun
	err    error
	baris  []peran.Baris
}

func pasang(t *testing.T, k *kondisiAkun) {
	t.Helper()
	config.Cfg = &config.Config{JWTSecret: "rahasia-uji-middleware-0123456789", JWTAccessExpireMin: 60}
	lamaStatus, lamaPeran := muatStatusAkun, muatPeran
	muatStatusAkun = func(string) (StatusAkun, error) { return k.status, k.err }
	muatPeran = func(_ context.Context, _ string, role string) (peran.Efektif, error) {
		return peran.Selesaikan(role, k.baris), nil
	}
	t.Cleanup(func() { muatStatusAkun, muatPeran = lamaStatus, lamaPeran })
}

// panggilGerbang menjalankan satu permintaan melalui gerbang yang dipilih dan mengembalikan kode, isi, dan nilai role/peran yang dipasang ke konteks.
func panggilGerbang(t *testing.T, gerbang gin.HandlerFunc, tokenRole string, tanpaToken bool) (int, map[string]string, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	var terpasang map[string]string
	r.GET("/x", gerbang, func(c *gin.Context) {
		terpasang = map[string]string{"role": c.GetString("role"), "peran": c.GetString(peran.KunciGinPeran), "user_id": c.GetString("user_id")}
		c.JSON(200, gin.H{"ok": true})
	})
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	if !tanpaToken {
		tok, _, err := utils.GenerateAccessToken("11111111-1111-4111-8111-111111111111", "budi", tokenRole)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w.Code, terpasang, w.Body.String()
}

func TestAuthRequiredMenolakTamuDanYangBelumSetuju(t *testing.T) {
	baris := func(p string) []peran.Baris {
		return []peran.Baris{{ID: 1, Peran: p, Kode: map[string]string{peran.Satker: "119091"}[p]}}
	}
	cases := []struct {
		nama     string
		k        kondisiAkun
		wantAuth int    // AuthRequired
		wantCode string // code galat bila ditolak
		wantTamu int    // AuthTamuBoleh
	}{
		{"tamu yang sudah setuju", kondisiAkun{status: StatusAkun{Aktif: true, Role: "user", Setuju: true}}, 403, "tamu", 200},
		{"tamu belum setuju: persetujuan didahulukan", kondisiAkun{status: StatusAkun{Aktif: true, Role: "user"}}, 403, "persetujuan_diperlukan", 200},
		{"berperan tetapi belum setuju", kondisiAkun{status: StatusAkun{Aktif: true, Role: "user"}, baris: baris(peran.Satker)}, 403, "persetujuan_diperlukan", 200},
		{"berperan dan sudah setuju", kondisiAkun{status: StatusAkun{Aktif: true, Role: "user", Setuju: true}, baris: baris(peran.Satker)}, 200, "", 200},
		{"superadmin tidak wajib setuju", kondisiAkun{status: StatusAkun{Aktif: true, Role: "superadmin"}}, 200, "", 200},
		{"role admin lama tanpa peran jadi tamu", kondisiAkun{status: StatusAkun{Aktif: true, Role: "admin", Setuju: true}}, 403, "tamu", 200},
		{"akun nonaktif", kondisiAkun{status: StatusAkun{Aktif: false, Role: "user", Setuju: true}, baris: baris(peran.Satker)}, 401, "account_inactive", 401},
		{"akun tidak ada", kondisiAkun{err: sql.ErrNoRows}, 401, "", 401},
		{"galat database", kondisiAkun{err: sql.ErrConnDone}, 500, "", 500},
	}
	for _, c := range cases {
		k := c.k
		pasang(t, &k)
		code, _, isi := panggilGerbang(t, AuthRequired(), "user", false)
		if code != c.wantAuth {
			t.Errorf("%s: AuthRequired = %d (%s), want %d", c.nama, code, isi, c.wantAuth)
		}
		if c.wantCode != "" && !contains(isi, `"code":"`+c.wantCode+`"`) {
			t.Errorf("%s: isi %s tidak memuat code %q", c.nama, isi, c.wantCode)
		}
		if code, _, isi := panggilGerbang(t, AuthTamuBoleh(), "user", false); code != c.wantTamu {
			t.Errorf("%s: AuthTamuBoleh = %d (%s), want %d", c.nama, code, isi, c.wantTamu)
		}
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestRoleDiambilDariDatabaseBukanToken(t *testing.T) {
	// Token masih bertuliskan superadmin, tetapi database sudah menurunkannya: hak superadmin langsung hilang (tamu bila tanpa peran).
	pasang(t, &kondisiAkun{status: StatusAkun{Aktif: true, Role: "user", Setuju: true}})
	if code, _, isi := panggilGerbang(t, AuthRequired(), "superadmin", false); code != 403 || !contains(isi, `"code":"tamu"`) {
		t.Errorf("token superadmin, database user = %d %s, want 403 tamu", code, isi)
	}
	// Sebaliknya, token user tidak menurunkan superadmin yang sah menurut database.
	pasang(t, &kondisiAkun{status: StatusAkun{Aktif: true, Role: "superadmin"}})
	code, terpasang, _ := panggilGerbang(t, AuthRequired(), "user", false)
	if code != 200 || terpasang["role"] != "superadmin" || terpasang["peran"] != "superadmin" {
		t.Errorf("token user, database superadmin = %d %v, want 200 superadmin", code, terpasang)
	}
}

func TestTokenTidakAdaAtauRusak(t *testing.T) {
	pasang(t, &kondisiAkun{status: StatusAkun{Aktif: true, Role: "user", Setuju: true}, baris: []peran.Baris{{ID: 1, Peran: peran.PenggunaBarang}}})
	for _, g := range []gin.HandlerFunc{AuthRequired(), AuthTamuBoleh()} {
		if code, _, _ := panggilGerbang(t, g, "user", true); code != 401 {
			t.Errorf("tanpa token = %d, want 401", code)
		}
	}
}

func TestHakKelolaDanLihatPengguna(t *testing.T) {
	cases := []struct {
		nama, role, peranAktif string
		kelola, lihat          bool
	}{
		{"superadmin", "superadmin", "superadmin", true, true},
		{"pengguna barang", "user", peran.PenggunaBarang, true, true},
		{"ue1", "user", peran.UE1, false, true},
		{"kanwil", "user", peran.Kanwil, false, true},
		{"satker", "user", peran.Satker, false, true},
		{"tamu", "user", "", false, false},
		{"superadmin yang bertindak sebagai satker", "user", peran.Satker, false, true},
		{"role admin lama", "admin", "", false, false},
	}
	gin.SetMode(gin.TestMode)
	for _, c := range cases {
		w := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(w)
		ctx.Set("role", c.role)
		ctx.Set(peran.KunciGinPeran, c.peranAktif)
		if got := KelolaPengguna(ctx); got != c.kelola {
			t.Errorf("%s: KelolaPengguna = %v, want %v", c.nama, got, c.kelola)
		}
		if got := LihatPengguna(ctx); got != c.lihat {
			t.Errorf("%s: LihatPengguna = %v, want %v", c.nama, got, c.lihat)
		}
	}
}
