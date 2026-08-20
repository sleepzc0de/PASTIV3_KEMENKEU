package database

import (
	"database/sql"
	"fmt"
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

	connString := fmt.Sprintf(
		"server=%s;user id=%s;password=%s;port=%s;database=%s;encrypt=true;TrustServerCertificate=true",
		cfg.SLDKDBHost, cfg.SLDKDBUser, cfg.SLDKDBPassword, cfg.SLDKDBPort, cfg.SLDKDBName,
	)

	db, err := sql.Open("sqlserver", connString)
	if err != nil {
		log.Println("[ERROR] Gagal membuat koneksi ke SLDK:", err)
		return
	}

	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := db.Ping(); err != nil {
		log.Println("[ERROR] Gagal terhubung ke SLDK (Interchange):", err)
		return
	}

	log.Println("[INFO] Berhasil terhubung ke SLDK:", cfg.SLDKDBName)
	SLDKDB = db
}
