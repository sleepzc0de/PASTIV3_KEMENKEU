package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime/debug"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"pasti-v3-backend/audit"
	"pasti-v3-backend/config"
	"pasti-v3-backend/database"
	"pasti-v3-backend/handlers"
	"pasti-v3-backend/middleware"
	"pasti-v3-backend/monitor"
	"pasti-v3-backend/routes"
	"pasti-v3-backend/serverlog"
)

func main() {
	config.LoadConfig()
	// Log standar dialihkan ke pencatat: hanya error dan warning yang dicetak (di production) dan disimpan; tidak ada log akses per permintaan.
	serverlog.Pasang(os.Stderr, handlers.LevelKonsolLog(), serverlog.BacaModeSimpan(config.Cfg.LogSimpan))
	terapkanBatasMemori()

	database.Connect()
	handlers.SinkronkanSuperadmin()
	database.ConnectSLDK()
	database.ConnectDatamart()
	handlers.InitServerLog()
	handlers.InitAudit()
	handlers.InitMonitor()
	handlers.InitApiAkses()
	handlers.InitDigitalisasi()
	handlers.InitInaprocPenarikan()

	if config.Cfg.AppEnv == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	// gin.New (bukan gin.Default): Default mencetak satu baris log untuk SETIAP permintaan, yang pada pemakaian ramai membengkakkan log berukuran ratusan MB per hari.
	// Gantinya, serverlog.Middleware hanya mencatat respons 5xx dan permintaan lambat, dan serverlog.Pemulih menangkap panik.
	r := gin.New()
	// IP klien hanya diambil dari header proksi (X-Real-IP yang ditimpa Nginx) bila sambungan datang dari proksi tepercaya; selain itu header diabaikan sehingga
	// pembatas laju dan log audit tidak bisa dikelabui dengan header palsu.
	if err := r.SetTrustedProxies(daftarProksi(config.Cfg.TrustedProxies)); err != nil {
		log.Println("[WARN] TRUSTED_PROXIES tidak valid, tidak ada proksi yang dipercaya:", err)
		r.SetTrustedProxies(nil)
	}
	r.RemoteIPHeaders = []string{"X-Real-IP", "X-Forwarded-For"}

	r.Use(serverlog.Middleware(time.Duration(config.Cfg.LogLambatMS) * time.Millisecond))
	// Pengukur kinerja dipasang paling luar agar waktu responsnya mencakup semuanya (termasuk penolakan 429/503); audit mencatat setelah autentikasi (yang ada di tingkat grup rute) selesai.
	r.Use(monitor.Middleware())
	// CORS sebelum pembatas supaya penolakan 429/503 pun terbaca oleh frontend (tanpa header CORS, browser menyembunyikan statusnya).
	r.Use(middleware.CORSMiddleware())
	r.Use(middleware.SecurityHeadersMiddleware())
	r.Use(middleware.SerentakBawaan())
	r.Use(middleware.BatasLajuUmum())
	r.Use(middleware.BatasBadanBawaan())
	r.Use(middleware.InvalidasiAkunMiddleware())
	r.Use(audit.Middleware())
	r.Use(serverlog.Pemulih())

	routes.SetupRoutes(r)

	log.Println("[INFO] PASTI V3 Backend berjalan di port:", config.Cfg.AppPort)
	// Batas waktu server: tanpa ini sambungan yang sengaja dibuat lambat (slowloris) atau tidak pernah ditutup menumpuk tanpa batas. Batas tulis 15 menit sejalan dengan
	// proxy_read_timeout Nginx (ekspor dan sinkronisasi yang lama masih dilayani).
	srv := &http.Server{
		Addr:              ":" + config.Cfg.AppPort,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       2 * time.Minute,
		WriteTimeout:      15 * time.Minute,
		IdleTimeout:       2 * time.Minute,
		MaxHeaderBytes:    64 << 10,
	}
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal("[FATAL] Server gagal dijalankan:", err)
		}
	}()

	// Dimatikan (docker stop mengirim SIGTERM): selesaikan permintaan yang berjalan lalu tuliskan sisa antrean audit dan log ke database, supaya aktivitas dan galat terakhir tidak hilang.
	henti := make(chan os.Signal, 1)
	signal.Notify(henti, syscall.SIGINT, syscall.SIGTERM)
	<-henti
	log.Println("[INFO] Menerima sinyal berhenti, menutup server…")
	ctx, batal := context.WithTimeout(context.Background(), 15*time.Second)
	defer batal()
	if err := srv.Shutdown(ctx); err != nil {
		log.Println("[WARN] Penutupan server tidak tuntas:", err)
	}
	handlers.BerhentiMonitor()
	handlers.BerhentiApiAkses(ctx)
	handlers.BerhentiAudit(ctx)
	handlers.BerhentiServerLog(ctx)
	log.Println("[INFO] Server berhenti")
}

// daftarProksi memecah TRUSTED_PROXIES (dipisah koma) menjadi daftar IP/CIDR.
func daftarProksi(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// terapkanBatasMemori memasang batas lunak memori Go sehingga pemulung memori bekerja lebih agresif sebelum proses menyentuh batas memori container dan dimatikan OS.
// Urutan: GOMEMLIMIT (dihormati runtime sendiri) > BATAS_MEMORI_MB > 85% dari batas memori container bila container dibatasi.
func terapkanBatasMemori() {
	if os.Getenv("GOMEMLIMIT") != "" {
		return
	}
	var batas int64
	if mb := config.Cfg.BatasMemoriMB; mb > 0 {
		batas = int64(mb) << 20
	} else if b, ok := monitor.BatasMemoriKontainer(); ok {
		batas = int64(b) / 100 * 85
	}
	if batas > 0 {
		debug.SetMemoryLimit(batas)
		log.Printf("[INFO] Batas lunak memori Go: %d MB", batas>>20)
	}
}
