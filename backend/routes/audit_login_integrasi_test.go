package routes

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/mojocn/base64Captcha"

	"pasti-v3-backend/audit"
	"pasti-v3-backend/utils"
)

// Tes integrasi SQL Server: setiap jalur login (berhasil, gagal dengan berbagai alasan, akun dikunci) tercatat dengan alasan yang benar, dan nama yang diketik untuk akun yang
// tidak ada TIDAK disimpan (orang kerap salah mengetik kata sandi di kolom nama pengguna).

func TestLoginTercatatDenganAlasanDenganSQLServer(t *testing.T) {
	db := bukaDBUji(t)
	bersih := func() {
		db.Exec(`DELETE FROM audit_log WHERE username LIKE N'uji-peran-%' OR ip = N'198.51.100.7'`)
		bersihkanUjiPeran(db)
	}
	bersih()
	t.Cleanup(bersih)
	store := &audit.Store{DB: db}
	siram := pasangPerekam(t, store)

	pengguna := buatPenggunaUji(t, db, "audit-login", "user")
	nonaktif := buatPenggunaUji(t, db, "audit-nonaktif", "user")
	hash, salt, err := utils.HashPassword("Rahasia-123")
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []penggunaUji{pengguna, nonaktif} {
		if _, err := db.Exec(`UPDATE users SET password_hash = @p1, password_salt = @p2 WHERE id = @p3`, hash, salt, p.id); err != nil {
			t.Fatal(err)
		}
	}
	db.Exec(`UPDATE users SET is_active = 0 WHERE id = @p1`, nonaktif.id)

	nomor := 0
	login := func(username, sandi string, captchaBenar bool) int {
		t.Helper()
		nomor++
		id := "uji-captcha-" + string(rune('a'+nomor))
		base64Captcha.DefaultMemStore.Set(id, "12345")
		jawab := "12345"
		if !captchaBenar {
			jawab = "99999"
		}
		b, _ := json.Marshal(map[string]string{"username": username, "password": sandi, "captcha_id": id, "captcha_answer": jawab})
		code, _, _ := panggilAudit(t, "POST", "/auth/login", "", string(b))
		return code
	}

	const salahKetik = "sandi-salah-diketik-di-kolom-nama"
	if c := login("uji-peran-audit-login", "Rahasia-123", true); c != 200 {
		t.Fatalf("login benar: %d", c)
	}
	login("uji-peran-audit-login", "SalahTotal-1", true)                     // kata sandi salah
	login(salahKetik, "apa-saja-123", true)                                  // akun tidak ada
	login("uji-peran-audit-login", "Rahasia-123", false)                     // captcha salah
	login("uji-peran-audit-nonaktif", "Rahasia-123", true)                   // akun nonaktif
	panggilAudit(t, "POST", "/auth/login", "", `{"bukan":"json yang benar"`) // permintaan rusak
	siram()

	d, err := store.Cari(context.Background(), audit.Penyaring{IP: "198.51.100.7", Kategori: audit.KatAuth})
	if err != nil {
		t.Fatal(err)
	}
	alasan := map[string]audit.Entri{}
	var berhasil []audit.Entri
	for _, e := range d.Entri {
		if e.Aksi == audit.AksiLoginBerhasil {
			berhasil = append(berhasil, e)
			continue
		}
		if e.Aksi != audit.AksiLoginGagal {
			t.Errorf("aksi tak terduga %q pada jalur login", e.Aksi)
		}
		a, _ := e.Detail["alasan"].(string)
		alasan[a] = e
	}
	if len(berhasil) != 1 || berhasil[0].Username != "uji-peran-audit-login" || !berhasil[0].Sukses || berhasil[0].Detail["metode"] != "password" || !strings.EqualFold(berhasil[0].UserID, pengguna.id) {
		t.Errorf("login berhasil = %+v", berhasil)
	}
	for _, a := range []string{"kata_sandi_salah", "pengguna_tidak_ditemukan", "captcha_salah", "akun_tidak_aktif", "permintaan_tidak_valid"} {
		e, ok := alasan[a]
		if !ok {
			t.Errorf("alasan %q tidak tercatat (ada: %v)", a, keys(alasan))
			continue
		}
		if e.Sukses || e.Status < 400 || !strings.HasPrefix(e.Label, "Login gagal") {
			t.Errorf("%s: %+v", a, e)
		}
	}
	if e := alasan["kata_sandi_salah"]; e.Username != "uji-peran-audit-login" || e.Detail["percobaan"] != float64(1) {
		t.Errorf("kata sandi salah = %+v", e)
	}
	if e := alasan["akun_tidak_aktif"]; e.Username != "uji-peran-audit-nonaktif" {
		t.Errorf("akun nonaktif = %+v", e)
	}
	// akun yang tidak ada: tanpa pengguna, dan teks yang diketik tidak boleh tersimpan di mana pun
	if e := alasan["pengguna_tidak_ditemukan"]; e.Username != "" || e.UserID != "" {
		t.Errorf("akun tidak ada = %+v", e)
	}
	semua, _ := json.Marshal(d.Entri)
	for _, rahasia := range []string{salahKetik, "apa-saja-123", "SalahTotal-1", "Rahasia-123"} {
		if strings.Contains(string(semua), rahasia) {
			t.Errorf("teks %q bocor ke log audit", rahasia)
		}
	}

	// lima kali salah berturut-turut: akun dikunci dan itu tercatat
	for i := 0; i < 4; i++ {
		login("uji-peran-audit-login", "SalahTotal-1", true)
	}
	login("uji-peran-audit-login", "Rahasia-123", true) // sekarang terkunci walau kata sandi benar
	siram()
	d, _ = store.Cari(context.Background(), audit.Penyaring{IP: "198.51.100.7", Aksi: audit.AksiLoginGagal})
	var dikunci, terkunci bool
	for _, e := range d.Entri {
		switch e.Detail["alasan"] {
		case "akun_dikunci":
			dikunci = true
		case "akun_terkunci":
			terkunci = true
		}
	}
	if !dikunci || !terkunci {
		t.Errorf("penguncian akun tercatat: dikunci=%v terkunci=%v", dikunci, terkunci)
	}
}

func keys(m map[string]audit.Entri) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	return out
}
