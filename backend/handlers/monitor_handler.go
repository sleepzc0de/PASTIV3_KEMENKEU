package handlers

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"pasti-v3-backend/config"
	"pasti-v3-backend/database"
	"pasti-v3-backend/digitalisasi"
	"pasti-v3-backend/monitor"
	"pasti-v3-backend/utils"
)

// Pemantauan resource: khusus superadmin (dipasang di routes). Hanya membaca; tidak ada yang bisa diubah dari sini.

var monitorBatal context.CancelFunc

// InitMonitor membuat pengelola pemantauan dan menjalankan pengukur berkala (bila MONITOR_AKTIF). Dipanggil dari main setelah database tersambung dan sebelum
// penjadwal lain dijalankan, supaya tugas latar belakang yang berat ikut terukur.
func InitMonitor() {
	if database.DB == nil {
		return
	}
	retensi := 30
	aktif := true
	if config.Cfg != nil {
		retensi, aktif = config.Cfg.MonitorRetensiHari, config.Cfg.MonitorAktif
	}
	m := monitor.NewManager(
		func() *sql.DB { return database.DB },
		func() *sql.DB { return database.SLDKDB },
		&monitor.Store{DB: database.DB},
		retensi,
	)
	m.TugasLatar = tugasLatarBelakang
	monitor.Pasang(m)
	if !aktif {
		log.Println("[MONITOR] Pengukur resource berkala dimatikan (MONITOR_AKTIF=false); halaman hanya menampilkan keadaan sesaat")
		return
	}
	ctx, batal := context.WithCancel(context.Background())
	monitorBatal = batal
	m.Mulai(ctx)
	log.Printf("[MONITOR] Pengukur resource aktif (riwayat disimpan tiap 5 menit, retensi %d hari)", retensi)
}

// BerhentiMonitor menghentikan pengukur berkala; dipanggil saat server dimatikan.
func BerhentiMonitor() {
	if monitorBatal != nil {
		monitorBatal()
	}
}

// tugasLatarBelakang menyebut tugas besar yang sedang berjalan: keduanya membebani CPU, memori, dan koneksi database, sehingga menjelaskan lonjakan pada grafik.
func tugasLatarBelakang() []monitor.TugasLatar {
	out := []monitor.TugasLatar{}
	t := monitor.TugasLatar{Nama: "Sinkronisasi Digitalisasi Aset (SLDK)"}
	if digitalisasi.Default != nil {
		if a := digitalisasi.Default.Active(); a != nil {
			t.Berjalan = true
			t.Info = fmt.Sprintf("%d dataset, dimulai %s oleh %s", len(a.Datasets), a.Mulai.In(zonaWIB).Format("15:04"), a.Oleh)
		}
	}
	out = append(out, t)
	p := monitor.TugasLatar{Nama: "Penarikan data Pengadaan (Inaproc)"}
	if Penarik != nil {
		if a := Penarik.Aktif(); a != nil {
			p.Berjalan = true
			p.Info = fmt.Sprintf("%d dari %d tugas selesai, dimulai %s (%s)", a.Selesai, a.Total, a.Mulai.In(zonaWIB).Format("15:04"), a.Pemicu)
		}
	}
	return append(out, p)
}

func monitorSiap(c *gin.Context) *monitor.Manager {
	m := monitor.Default()
	if m == nil {
		utils.ErrorResponse(c, http.StatusServiceUnavailable, "Pemantauan resource belum aktif")
		return nil
	}
	return m
}

// GET /monitor/ringkasan: keadaan server, aplikasi, dan database saat ini beserta penilaian resource mana yang cukup atau perlu ditambah.
func GetMonitorRingkasan(c *gin.Context) {
	m := monitorSiap(c)
	if m == nil {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 25*time.Second)
	defer cancel()
	utils.SuccessResponse(c, http.StatusOK, "OK", m.Ringkasan(ctx))
}

// GET /monitor/riwayat?rentang=1j|24j|7h|30h: deret waktu untuk grafik.
func GetMonitorRiwayat(c *gin.Context) {
	m := monitorSiap(c)
	if m == nil {
		return
	}
	rentang := c.DefaultQuery("rentang", "24j")
	ctx, cancel := context.WithTimeout(c.Request.Context(), 25*time.Second)
	defer cancel()
	r, ok := m.Riwayat(ctx, rentang)
	if !ok {
		utils.ErrorResponse(c, http.StatusBadRequest, "Rentang tidak dikenal; pilih 1j, 24j, 7h, atau 30h")
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "OK", r)
}

// GET /monitor/database: tabel terbesar pada database aplikasi (lebih berat daripada ringkasan, jadi dipisah dan disimpan sementara).
func GetMonitorDatabase(c *gin.Context) {
	m := monitorSiap(c)
	if m == nil {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 25*time.Second)
	defer cancel()
	tabel, galat := m.TabelTeratas(ctx)
	if tabel == nil {
		tabel = []monitor.TabelDB{}
	}
	utils.SuccessResponse(c, http.StatusOK, "OK", gin.H{"tabel": tabel, "catatan": galat})
}
