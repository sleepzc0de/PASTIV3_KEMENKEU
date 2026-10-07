package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"pasti-v3-backend/config"
	"pasti-v3-backend/database"
	"pasti-v3-backend/handlers"
	"pasti-v3-backend/audit"
	"pasti-v3-backend/middleware"
	"pasti-v3-backend/monitor"
	"pasti-v3-backend/routes"
)

func main() {
	config.LoadConfig()
	database.Connect()
	handlers.SinkronkanSuperadmin()
	database.ConnectSLDK()
	handlers.InitAudit()
	handlers.InitMonitor()
	handlers.InitDigitalisasi()
	handlers.InitInaprocPenarikan()

	if config.Cfg.AppEnv == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.Default()
	r.Use(middleware.CORSMiddleware())
	r.Use(middleware.SecurityHeadersMiddleware())
	// Pengukur kinerja dipasang paling luar agar waktu responsnya mencakup semuanya; audit mencatat setelah autentikasi (yang ada di tingkat grup rute) selesai.
	r.Use(monitor.Middleware())
	r.Use(audit.Middleware())

	routes.SetupRoutes(r)

	log.Println("[INFO] PASTI V3 Backend berjalan di port:", config.Cfg.AppPort)
	srv := &http.Server{Addr: ":" + config.Cfg.AppPort, Handler: r}
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal("[FATAL] Server gagal dijalankan:", err)
		}
	}()

	// Dimatikan (docker stop mengirim SIGTERM): selesaikan permintaan yang berjalan lalu tuliskan sisa antrean audit ke database, supaya aktivitas terakhir tidak hilang.
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
	handlers.BerhentiAudit(ctx)
	log.Println("[INFO] Server berhenti")
}
