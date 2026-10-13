package database

import (
	"context"
	"database/sql"
	"log"
	"time"

	"pasti-v3-backend/config"

	_ "github.com/microsoft/go-mssqldb"
)

var SLDKDB *sql.DB

// ConnectSLDK membuat koneksi TERPISAH ke database Interchange (SLDK).
// Sengaja tidak fatal kalau gagal, supaya downtime SLDK tidak
// mematikan seluruh aplikasi PASTI V3.
func ConnectSLDK() {
	cfg := config.Cfg

	if cfg.SLDKDBHost == "" {
		log.Println("[WARN] SLDK_DB_HOST kosong, integrasi SLDK dinonaktifkan")
		return
	}

	connString := stringKoneksi(cfg.SLDKDBHost, cfg.SLDKDBUser, cfg.SLDKDBPassword, cfg.SLDKDBPort, cfg.SLDKDBName)

	db, err := sql.Open("sqlserver", connString)
	if err != nil {
		log.Println("[ERROR] Gagal membuat koneksi ke SLDK:", err)
		return
	}

	// Kolam SLDK sengaja kecil: SLDK adalah sumber data besar milik pihak lain dan hanya dibaca saat sinkronisasi.
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(10 * time.Minute)
	db.SetConnMaxIdleTime(3 * time.Minute)

	ctx, batal := context.WithTimeout(context.Background(), 20*time.Second)
	defer batal()
	if err := db.PingContext(ctx); err != nil {
		log.Println("[ERROR] Gagal terhubung ke SLDK (Interchange):", err)
		return
	}

	log.Println("[INFO] Berhasil terhubung ke SLDK:", cfg.SLDKDBName)
	SLDKDB = db
}
