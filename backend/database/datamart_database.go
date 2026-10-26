package database

import (
	"context"
	"database/sql"
	"log"
	"time"

	"pasti-v3-backend/config"

	_ "github.com/microsoft/go-mssqldb"
)

// DatamartDB: handle ke database datamart, satu-satunya sumber data sinkronisasi Digitalisasi Aset (tabel DIGITALISASI_* hasil script backend/datamart/buat_datamart_digitalisasi.sql;
// lihat docs/datamart-digitalisasi.md). nil bila DATAMART_DB_HOST kosong. Tetap ada walau ping saat server dimulai gagal (sql.Open tidak menyambung; sambungan dibuat saat dipakai),
// jadi datamart yang sedang mati saat server dimulai dapat dicoba lagi nanti tanpa memulai ulang server.
var DatamartDB *sql.DB

// ConnectDatamart membuka koneksi ke datamart bila DATAMART_DB_HOST terisi. Tidak fatal: tanpa datamart hanya sinkronisasi Digitalisasi Aset yang tidak berjalan. Pengguna datamart
// cukup berhak SELECT pada tujuh tabel dbo.DIGITALISASI_* dan dbo.datamart_digitalisasi_run.
func ConnectDatamart() {
	cfg := config.Cfg

	if cfg.DatamartDBHost == "" {
		log.Println("[WARN] DATAMART_DB_HOST kosong, sinkronisasi Digitalisasi Aset dinonaktifkan")
		return
	}

	db, err := sql.Open("sqlserver", stringKoneksi(cfg.DatamartDBHost, cfg.DatamartDBUser, cfg.DatamartDBPassword, cfg.DatamartDBPort, cfg.DatamartDBName))
	if err != nil {
		log.Println("[ERROR] Gagal membuat koneksi ke datamart:", err)
		return
	}
	// Kolam kecil: datamart hanya dibaca saat sinkronisasi.
	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(10 * time.Minute)
	db.SetConnMaxIdleTime(3 * time.Minute)
	DatamartDB = db

	ctx, batal := context.WithTimeout(context.Background(), 20*time.Second)
	defer batal()
	if err := db.PingContext(ctx); err != nil {
		log.Println("[WARN] Datamart belum dapat dihubungi saat server dimulai (akan dicoba lagi saat dipakai):", err)
		return
	}
	log.Println("[INFO] Berhasil terhubung ke datamart:", cfg.DatamartDBName)
}
