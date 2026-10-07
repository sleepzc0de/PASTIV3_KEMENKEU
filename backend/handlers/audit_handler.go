package handlers

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"pasti-v3-backend/audit"
	"pasti-v3-backend/config"
	"pasti-v3-backend/database"
	"pasti-v3-backend/utils"
)

// Log audit aktivitas pengguna: khusus superadmin (dipasang di routes). Hanya membaca; tidak ada rute untuk mengubah atau menghapus entri.

var (
	auditStore       *audit.Store
	auditBatalBersih context.CancelFunc
)

// Tanggal tanpa jam pada filter dibaca sebagai tanggal WIB (zonaWIB, UTC+7, didefinisikan di inaproc_penarikan_jadwal.go).

// InitAudit memasang perekam audit (menulis berkelompok ke database) dan pembersih retensi. Dipanggil dari main setelah database tersambung.
func InitAudit() {
	if database.DB == nil {
		return
	}
	auditStore = audit.Mulai(database.DB)
	ctx, batal := context.WithCancel(context.Background())
	auditBatalBersih = batal
	hari := 365
	if config.Cfg != nil {
		hari = config.Cfg.AuditRetensiHari
	}
	audit.MulaiPembersihan(ctx, auditStore, hari)
	log.Printf("[AUDIT] Pencatatan aktivitas pengguna aktif (retensi %d hari)", hari)
}

// BerhentiAudit menyiram sisa antrean audit ke database; dipanggil saat server dimatikan.
func BerhentiAudit(ctx context.Context) {
	if auditBatalBersih != nil {
		auditBatalBersih()
	}
	audit.Berhenti(ctx)
}

// GunakanAuditStore mengganti penyimpanan audit yang dibaca handler (untuk tes). Mengembalikan fungsi pemulih.
func GunakanAuditStore(s *audit.Store) func() {
	lama := auditStore
	auditStore = s
	return func() { auditStore = lama }
}

func auditCtx(c *gin.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(c.Request.Context(), 25*time.Second)
}

func auditSiap(c *gin.Context) bool {
	if auditStore == nil {
		utils.ErrorResponse(c, http.StatusServiceUnavailable, "Log audit belum aktif")
		return false
	}
	return true
}

func auditGagal(c *gin.Context, konteks string, err error) {
	log.Printf("[AUDIT ERROR] %s: %v", konteks, err)
	utils.ErrorResponse(c, http.StatusInternalServerError, "Gagal membaca log audit")
}

// bacaWaktuAudit membaca waktu dari RFC3339 ("2026-10-07T08:00:00Z") atau tanggal ("2026-10-07", dibaca sebagai awal hari WIB). akhirHari: tanggal tanpa jam pada batas
// "sampai" berarti sampai akhir hari itu (awal hari berikutnya).
func bacaWaktuAudit(s string, akhirHari bool) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.UTC(), nil
	}
	if t, err := time.ParseInLocation("2006-01-02", s, zonaWIB); err == nil {
		if akhirHari {
			t = t.AddDate(0, 0, 1)
		}
		return t.UTC(), nil
	}
	return time.Time{}, fmt.Errorf("waktu %q tidak valid", s)
}

func intQuery(c *gin.Context, nama string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(c.Query(nama)))
	return n
}

// penyaringAudit menyusun penyaring dari parameter kueri: dari, sampai, user_id, username, kategori, aksi, hasil (semua|berhasil|gagal), ip, q, halaman, per_halaman.
func penyaringAudit(c *gin.Context) (audit.Penyaring, error) {
	var p audit.Penyaring
	var err error
	if p.Dari, err = bacaWaktuAudit(c.Query("dari"), false); err != nil {
		return p, err
	}
	if p.Sampai, err = bacaWaktuAudit(c.Query("sampai"), true); err != nil {
		return p, err
	}
	if !p.Dari.IsZero() && !p.Sampai.IsZero() && !p.Sampai.After(p.Dari) {
		return p, errors.New("batas akhir waktu harus setelah batas awal")
	}
	p.UserID = c.Query("user_id")
	p.Username = c.Query("username")
	p.Kategori = c.Query("kategori")
	p.Aksi = c.Query("aksi")
	p.IP = c.Query("ip")
	p.Q = c.Query("q")
	switch c.Query("hasil") {
	case "berhasil":
		v := true
		p.Sukses = &v
	case "gagal":
		v := false
		p.Sukses = &v
	}
	p.Halaman, p.PerHalaman = intQuery(c, "halaman"), intQuery(c, "per_halaman")
	return p, nil
}

// GET /audit: satu halaman entri log audit menurut filter.
func GetAuditLog(c *gin.Context) {
	if !auditSiap(c) {
		return
	}
	p, err := penyaringAudit(c)
	if err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}
	ctx, cancel := auditCtx(c)
	defer cancel()
	d, err := auditStore.Cari(ctx, p)
	if err != nil {
		auditGagal(c, "cari", err)
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "OK", d)
}

// GET /audit/ringkasan: angka ringkas aktivitas pada rentang waktu (bawaan 24 jam terakhir).
func GetAuditRingkasan(c *gin.Context) {
	if !auditSiap(c) {
		return
	}
	p, err := penyaringAudit(c)
	if err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}
	ctx, cancel := auditCtx(c)
	defer cancel()
	r, err := auditStore.Ringkas(ctx, p.Dari, p.Sampai)
	if err != nil {
		auditGagal(c, "ringkasan", err)
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "OK", gin.H{"ringkasan": r, "perekam": audit.StatistikDefault(), "retensi_hari": retensiAuditHari()})
}

func retensiAuditHari() int {
	if config.Cfg != nil {
		return config.Cfg.AuditRetensiHari
	}
	return 365
}

// GET /audit/pengguna: ringkasan aktivitas per pengguna (yang terakhir aktif lebih dulu); username menyaring menurut nama.
func GetAuditPengguna(c *gin.Context) {
	if !auditSiap(c) {
		return
	}
	p, err := penyaringAudit(c)
	if err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}
	ctx, cancel := auditCtx(c)
	defer cancel()
	d, err := auditStore.PerPengguna(ctx, p.Dari, p.Sampai, p.Username, p.Halaman, p.PerHalaman)
	if err != nil {
		auditGagal(c, "per pengguna", err)
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "OK", d)
}

// GET /audit/opsi: kategori dan aksi yang dikenal, untuk filter.
func GetAuditOpsi(c *gin.Context) {
	utils.SuccessResponse(c, http.StatusOK, "OK", audit.DaftarOpsi())
}

// amankanSel mencegah injeksi rumus saat CSV dibuka di Excel: sel yang diawali = + - @ atau tab/CR diberi awalan tanda kutip.
func amankanSel(s string) string {
	if s == "" {
		return s
	}
	switch s[0] {
	case '=', '+', '-', '@', '\t', '\r':
		return "'" + s
	}
	return s
}

var kolomEksporAudit = []string{"Waktu (WIB)", "Waktu (UTC)", "Pengguna", "Nama lengkap", "Peran", "Kode peran", "Kategori", "Aksi", "Uraian", "Objek", "Metode", "Rute", "Status HTTP", "Hasil",
	"Durasi (ms)", "IP", "User agent", "ID permintaan", "Rincian (JSON)"}

// GET /audit/ekspor: log audit menurut filter sebagai CSV (UTF-8 dengan BOM agar terbaca benar di Excel), paling banyak audit.MaksEkspor baris. Pengeksporan itu sendiri
// tercatat di log audit (aksi audit.ekspor).
func EksporAuditLog(c *gin.Context) {
	if !auditSiap(c) {
		return
	}
	p, err := penyaringAudit(c)
	if err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Minute)
	defer cancel()

	nama := "log-audit-" + time.Now().In(zonaWIB).Format("20060102-150405") + ".csv"
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", `attachment; filename="`+nama+`"`)
	c.Header("Cache-Control", "no-store")
	c.Status(http.StatusOK)
	c.Writer.Write([]byte("\xEF\xBB\xBF"))
	w := csv.NewWriter(c.Writer)
	w.Write(kolomEksporAudit)
	n := 0
	err = auditStore.Alir(ctx, p, audit.MaksEkspor, func(e audit.Entri) error {
		hasil := "Berhasil"
		if !e.Sukses {
			hasil = "Gagal"
		}
		objek := e.ObjekTipe
		if e.ObjekID != "" {
			objek += ":" + e.ObjekID
		}
		rincian := ""
		if len(e.Detail) > 0 {
			if b, err := json.Marshal(e.Detail); err == nil {
				rincian = string(b)
			}
		}
		row := []string{e.Waktu.In(zonaWIB).Format("2006-01-02 15:04:05"), e.Waktu.UTC().Format("2006-01-02 15:04:05"), e.Username, e.NamaLengkap, e.Peran, e.KodePeran, e.Kategori, e.Aksi,
			e.Label, objek, e.Metode, e.Rute, strconv.Itoa(e.Status), hasil, strconv.Itoa(e.DurasiMS), e.IP, e.UserAgent, e.RequestID, rincian}
		for i := range row {
			row[i] = amankanSel(row[i])
		}
		n++
		if n%500 == 0 {
			w.Flush()
		}
		return w.Write(row)
	})
	w.Flush()
	if err != nil {
		// Header sudah terkirim; hanya bisa dicatat.
		log.Printf("[AUDIT ERROR] ekspor terhenti setelah %d baris: %v", n, err)
	}
}
