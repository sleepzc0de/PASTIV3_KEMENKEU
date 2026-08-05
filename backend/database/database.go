package database

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	"pasti-v3-backend/config"

	_ "github.com/microsoft/go-mssqldb"
)

var DB *sql.DB

func Connect() {
	cfg := config.Cfg

	connString := fmt.Sprintf(
		"server=%s;user id=%s;password=%s;port=%s;database=%s;encrypt=true;TrustServerCertificate=true",
		cfg.DBHost, cfg.DBUser, cfg.DBPassword, cfg.DBPort, cfg.DBName,
	)

	db, err := sql.Open("sqlserver", connString)
	if err != nil {
		log.Fatal("[FATAL] Gagal membuat koneksi database:", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := db.Ping(); err != nil {
		log.Fatal("[FATAL] Gagal terhubung ke SQL Server:", err)
	}

	log.Println("[INFO] Berhasil terhubung ke SQL Server 2022:", cfg.DBName)
	DB = db
}
