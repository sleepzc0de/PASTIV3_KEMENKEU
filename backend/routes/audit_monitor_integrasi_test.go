package routes

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"pasti-v3-backend/audit"
	"pasti-v3-backend/handlers"
	"pasti-v3-backend/monitor"
)

// Tes integrasi SQL Server (hanya bila PASTI_UJI_MSSQL_DSN diisi): log audit dan monitor resource hanya untuk superadmin, aktivitas benar-benar tercatat oleh middleware yang
// terpasang pada tabel rute asli, dan ekspor CSV aman dan ikut tercatat.

func panggilAudit(t *testing.T, method, path, token, body string) (int, []byte, http.Header) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(monitor.Middleware())
	r.Use(audit.Middleware())
	SetupRoutes(r)
	req := httptest.NewRequest(method, "/api/v1"+path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "198.51.100.7:5555"
	req.Header.Set("User-Agent", "UjiAudit/1.0")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w.Code, w.Body.Bytes(), w.Header()
}

// pasangPerekam memasang perekam audit baru yang menulis ke store; fungsi yang dikembalikan menyiram antreannya (dan memasang perekam baru agar bisa dipakai lagi).
func pasangPerekam(t *testing.T, store *audit.Store) (siram func()) {
	t.Helper()
	pasang := func() *audit.Perekam {
		p := audit.NewPerekam(store.Simpan, 200)
		p.Mulai()
		audit.Pasang(p)
		return p
	}
	cur := pasang()
	t.Cleanup(func() { cur.Berhenti(context.Background()); audit.Pasang(nil) })
	return func() {
		cur.Berhenti(context.Background())
		cur = pasang()
	}
}

func TestAuditDanMonitorHanyaUntukSuperadminDenganSQLServer(t *testing.T) {
	db := bukaDBUji(t)
	bersih := func() {
		db.Exec(`DELETE FROM audit_log WHERE username LIKE N'uji-peran-%'`)
		bersihkanUjiPeran(db)
	}
	bersih()
	t.Cleanup(bersih)
	store := &audit.Store{DB: db}
	t.Cleanup(handlers.GunakanAuditStore(store))
	pasangPerekam(t, store)
	monitor.Pasang(monitor.NewManager(func() *sql.DB { return db }, nil, &monitor.Store{DB: db}, 30))
	t.Cleanup(func() { monitor.Pasang(nil) })

	admin := buatPenggunaUji(t, db, "audit-admin", "superadmin")
	barang := buatPenggunaUji(t, db, "audit-barang", "user")
	tamu := buatPenggunaUji(t, db, "audit-tamu", "user")
	beriPeran(t, admin, barang, "pengguna_barang", "")

	rute := []string{"/audit", "/audit/ringkasan", "/audit/pengguna", "/audit/opsi", "/audit/ekspor", "/monitor/ringkasan", "/monitor/riwayat?rentang=1j", "/monitor/database"}
	for _, r := range rute {
		if code, _, _ := panggilAudit(t, "GET", r, "", ""); code != http.StatusUnauthorized {
			t.Errorf("GET %s tanpa login: %d, want 401", r, code)
		}
		if code, _, _ := panggilAudit(t, "GET", r, tamu.token, ""); code != http.StatusForbidden {
			t.Errorf("GET %s oleh tamu: %d, want 403", r, code)
		}
		if code, _, _ := panggilAudit(t, "GET", r, barang.token, ""); code != http.StatusForbidden {
			t.Errorf("GET %s oleh Pengguna Barang: %d, want 403 (hanya superadmin)", r, code)
		}
		if code, body, _ := panggilAudit(t, "GET", r, admin.token, ""); code != http.StatusOK {
			t.Errorf("GET %s oleh superadmin: %d, want 200: %s", r, code, string(body))
		}
	}

	// Superadmin yang sedang bertindak sebagai peran data kehilangan hak ini selama peran itu aktif.
	idSatker := beriPeran(t, admin, admin, "satker", "409294")
	pilihPeran(t, admin, idSatker)
	for _, r := range []string{"/audit", "/monitor/ringkasan"} {
		if code, _, _ := panggilAudit(t, "GET", r, admin.token, ""); code != http.StatusForbidden {
			t.Errorf("GET %s oleh superadmin yang bertindak sebagai Satker: %d, want 403", r, code)
		}
	}
	pilihPeran(t, admin, nil) // kembali sebagai superadmin
	if code, _, _ := panggilAudit(t, "GET", "/audit", admin.token, ""); code != http.StatusOK {
		t.Errorf("setelah kembali sebagai superadmin: %d, want 200", code)
	}

	// Tidak ada rute untuk mengubah atau menghapus log audit atau riwayat pemantauan.
	for _, m := range []string{"POST", "PUT", "PATCH", "DELETE"} {
		for _, r := range []string{"/audit", "/audit/1", "/monitor/ringkasan"} {
			if code, _, _ := panggilAudit(t, m, r, admin.token, "{}"); code != http.StatusNotFound && code != http.StatusMethodNotAllowed {
				t.Errorf("%s %s: %d, want 404/405 (log tidak boleh diubah dari API)", m, r, code)
			}
		}
	}

	if code, _, _ := panggilAudit(t, "GET", "/monitor/riwayat?rentang=setahun", admin.token, ""); code != http.StatusBadRequest {
		t.Errorf("rentang tidak dikenal: %d, want 400", code)
	}
	if code, _, _ := panggilAudit(t, "GET", "/audit?dari=bukan-tanggal", admin.token, ""); code != http.StatusBadRequest {
		t.Errorf("tanggal tidak valid: %d, want 400", code)
	}
	if code, _, _ := panggilAudit(t, "GET", "/audit?dari=2026-10-08&sampai=2026-10-01", admin.token, ""); code != http.StatusBadRequest {
		t.Errorf("rentang terbalik: %d, want 400", code)
	}
}

func TestAktivitasTercatatDanEksporCSVDenganSQLServer(t *testing.T) {
	db := bukaDBUji(t)
	bersih := func() {
		db.Exec(`DELETE FROM audit_log WHERE username LIKE N'uji-peran-%'`)
		bersihkanUjiPeran(db)
	}
	bersih()
	t.Cleanup(bersih)
	store := &audit.Store{DB: db}
	t.Cleanup(handlers.GunakanAuditStore(store))
	siram := pasangPerekam(t, store)

	admin := buatPenggunaUji(t, db, "audit-admin2", "superadmin")
	barang := buatPenggunaUji(t, db, "audit-barang2", "user")
	sasaran := buatPenggunaUji(t, db, "audit-sasaran", "user")
	beriPeran(t, admin, barang, "pengguna_barang", "")

	// 1) superadmin mengubah pengguna, memberi peran; 2) Pengguna Barang mencoba membuka log audit (ditolak)
	badan, _ := json.Marshal(map[string]interface{}{"full_name": "Sasaran Uji", "email": "sasaran-uji@example.test", "is_active": true, "password": "rahasia-sekali-123"})
	if code, b, _ := panggilAudit(t, "PUT", "/users/"+sasaran.id, admin.token, string(badan)); code != 200 {
		t.Fatalf("ubah pengguna: %d %s", code, b)
	}
	// memberi peran lewat router yang memasang middleware audit (beriPeran memakai router tanpa audit, jadi pemberian peran untuk menyiapkan tes tidak tercatat)
	if code, b, _ := panggilAudit(t, "POST", "/users/"+sasaran.id+"/peran", admin.token, `{"role":"kanwil","kode":"015010199"}`); code != http.StatusCreated {
		t.Fatalf("beri peran: %d %s", code, b)
	}
	panggilAudit(t, "GET", "/audit", barang.token, "")
	// GET biasa tidak dicatat
	panggilAudit(t, "GET", "/users", admin.token, "")
	siram()

	ctx := context.Background()
	ambil := func(aksi string) []audit.Entri {
		t.Helper()
		d, err := store.Cari(ctx, audit.Penyaring{Username: "uji-peran-", Aksi: aksi})
		if err != nil {
			t.Fatal(err)
		}
		return d.Entri
	}

	ubah := ambil("pengguna.ubah")
	if len(ubah) != 1 {
		t.Fatalf("entri pengguna.ubah = %d, want 1", len(ubah))
	}
	e := ubah[0]
	if e.Username != admin.username || e.Kategori != audit.KatPengguna || !strings.EqualFold(e.ObjekID, sasaran.id) || e.ObjekTipe != "pengguna" || !e.Sukses || e.Status != 200 ||
		e.Peran != "superadmin" || e.IP != "198.51.100.7" || e.UserAgent != "UjiAudit/1.0" || e.Label == "" || e.RequestID == "" || !strings.EqualFold(e.UserID, admin.id) {
		t.Errorf("entri ubah pengguna = %+v", e)
	}
	if e.Detail["aktif"] != true || e.Detail["kata_sandi_diganti"] != true {
		t.Errorf("rincian = %+v", e.Detail)
	}
	// kata sandi dan isi permintaan tidak boleh ada di mana pun pada entri
	if semua, _ := json.Marshal(e); strings.Contains(string(semua), "rahasia-sekali-123") || strings.Contains(string(semua), "sasaran-uji@example.test") {
		t.Errorf("isi permintaan bocor ke log audit: %s", semua)
	}

	peranBeri := ambil("peran.beri")
	if len(peranBeri) != 1 {
		t.Fatalf("entri peran.beri = %d, want 1", len(peranBeri))
	}
	if k := peranBeri[0]; k.Detail["peran"] != "kanwil" || k.Detail["kode"] != "015010199" || k.Detail["baru"] != true || k.Kategori != audit.KatPeran || !strings.EqualFold(k.ObjekID, sasaran.id) || k.Status != http.StatusCreated {
		t.Errorf("entri peran kanwil = %+v", k)
	}

	ditolak := ambil(audit.AksiDitolak)
	if len(ditolak) != 1 || ditolak[0].Username != barang.username || ditolak[0].Sukses || ditolak[0].Status != 403 || ditolak[0].Peran != "pengguna_barang" || ditolak[0].Rute != "/api/v1/audit" {
		t.Errorf("entri akses ditolak = %+v", ditolak)
	}
	if got := ambil("GET /api/v1/users"); len(got) != 0 {
		t.Errorf("GET biasa tercatat: %+v", got)
	}

	// Ringkasan dan aktivitas per pengguna melalui API.
	code, body, _ := panggilAudit(t, "GET", "/audit/ringkasan?dari=2020-01-01", admin.token, "")
	var rg struct {
		Data struct {
			Ringkasan audit.Ringkasan `json:"ringkasan"`
		} `json:"data"`
	}
	// tercatat: ubah pengguna, beri peran, dan akses ditolak (menyiapkan peran Pengguna Barang lewat helper tes tanpa audit tidak dihitung)
	if code != 200 || json.Unmarshal(body, &rg) != nil || rg.Data.Ringkasan.Total < 3 || rg.Data.Ringkasan.Ditolak < 1 || rg.Data.Ringkasan.PenggunaAktif < 2 {
		t.Errorf("ringkasan API: %d %s", code, body)
	}
	code, body, _ = panggilAudit(t, "GET", "/audit/pengguna?username=uji-peran-&dari=2020-01-01", admin.token, "")
	var pg struct {
		Data audit.DaftarPengguna `json:"data"`
	}
	if code != 200 || json.Unmarshal(body, &pg) != nil || pg.Data.Total < 2 {
		t.Errorf("per pengguna API: %d %s", code, body)
	}

	// Sisipkan entri yang sengaja berisi rumus untuk memeriksa pengamanan CSV.
	if err := store.Simpan(ctx, []audit.Entri{{Waktu: time.Now(), Username: "uji-peran-rumus", Kategori: audit.KatLainnya, Aksi: "uji.rumus", Label: "=HYPERLINK(\"http://jahat\")", Metode: "POST", Rute: "/x", Status: 200, Sukses: true}}); err != nil {
		t.Fatal(err)
	}

	// Ekspor CSV
	code, body, hdr := panggilAudit(t, "GET", "/audit/ekspor?username=uji-peran-&dari=2020-01-01", admin.token, "")
	if code != 200 || !strings.HasPrefix(hdr.Get("Content-Type"), "text/csv") || !strings.Contains(hdr.Get("Content-Disposition"), "log-audit-") || hdr.Get("Cache-Control") != "no-store" {
		t.Fatalf("ekspor: %d %v", code, hdr)
	}
	if !bytes.HasPrefix(body, []byte("\xEF\xBB\xBF")) {
		t.Error("CSV harus diawali BOM UTF-8 agar terbaca benar di Excel")
	}
	baris, err := csv.NewReader(bytes.NewReader(bytes.TrimPrefix(body, []byte("\xEF\xBB\xBF")))).ReadAll()
	if err != nil {
		t.Fatalf("CSV tidak sah: %v", err)
	}
	if len(baris) < 5 || baris[0][0] != "Waktu (WIB)" || baris[0][7] != "Aksi" || len(baris[0]) != 19 {
		t.Fatalf("CSV: %d baris, header %v", len(baris), baris[0])
	}
	var adaUbah, adaRumus bool
	for _, b := range baris[1:] {
		if len(b) != 19 {
			t.Fatalf("baris tidak 19 kolom: %v", b)
		}
		adaUbah = adaUbah || b[7] == "pengguna.ubah" && b[13] == "Berhasil"
		if b[7] == "uji.rumus" {
			adaRumus = true
			if b[8] != "'=HYPERLINK(\"http://jahat\")" {
				t.Errorf("sel rumus tidak diamankan: %q", b[8])
			}
		}
	}
	if !adaUbah || !adaRumus {
		t.Errorf("isi CSV kurang: ubah=%v rumus=%v", adaUbah, adaRumus)
	}

	// Pengeksporan itu sendiri tercatat.
	siram()
	ek := ambil("audit.ekspor")
	if len(ek) != 1 || ek[0].Username != admin.username || ek[0].Kategori != audit.KatAudit || ek[0].Detail["kueri"] == nil || !strings.Contains(ek[0].Detail["kueri"].(string), "username=uji-peran-") {
		t.Errorf("entri ekspor = %+v", ek)
	}
}
