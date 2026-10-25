package database

import (
	"context"
	"database/sql"
	"log"
	"time"

	"pasti-v3-backend/config"

	_ "github.com/microsoft/go-mssqldb"
)

// SLDKDB: koneksi ke SLDK yang berhasil dibuktikan (ping) saat server dimulai; nil bila SLDK tidak dikonfigurasi atau tidak terjangkau saat itu. Dipakai fitur yang menganggap
// "ada" sebagai "tersedia" (monitor, referensi Kanwil).
var SLDKDB *sql.DB

// SLDKDBTerkonfigurasi: handle ke SLDK selama SLDK_DB_HOST terisi, walau ping saat server dimulai gagal (sql.Open tidak menyambung; sambungan dibuat saat dipakai). Dipakai
// sinkronisasi Digitalisasi Aset: SLDK yang mati saat server dimulai harus bisa dicoba lagi nanti tanpa memulai ulang server, dan sinkronisasi dapat beralih ke datamart.
var SLDKDBTerkonfigurasi *sql.DB

// DatamartDB: handle ke datamart SLDK (cadangan sinkronisasi Digitalisasi Aset); nil bila DATAMART_DB_HOST kosong. Tetap ada walau ping saat server dimulai gagal.
var DatamartDB *sql.DB

// bukaKolamBaca membuka kolam koneksi kecil untuk sumber data eksternal yang hanya dibaca saat sinkronisasi. Ping gagal tidak membatalkan handle: dikembalikan bersama galatnya.
func bukaKolamBaca(host, user, password, port, name string, maksKoneksi int) (*sql.DB, error) {
	db, err := sql.Open("sqlserver", stringKoneksi(host, user, password, port, name))
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(maksKoneksi)
	db.SetMaxIdleConns(maksKoneksi / 2)
	db.SetConnMaxLifetime(10 * time.Minute)
	db.SetConnMaxIdleTime(3 * time.Minute)

	ctx, batal := context.WithTimeout(context.Background(), 20*time.Second)
	defer batal()
	return db, db.PingContext(ctx)
}

// ConnectSLDK membuat koneksi TERPISAH ke database Interchange (SLDK).
// Sengaja tidak fatal kalau gagal, supaya downtime SLDK tidak
// mematikan seluruh aplikasi PASTI V3.
func ConnectSLDK() {
	cfg := config.Cfg

	if cfg.SLDKDBHost == "" {
		log.Println("[WARN] SLDK_DB_HOST kosong, integrasi SLDK dinonaktifkan")
		return
	}

	// Kolam SLDK sengaja kecil: SLDK adalah sumber data besar milik pihak lain dan hanya dibaca saat sinkronisasi.
	db, err := bukaKolamBaca(cfg.SLDKDBHost, cfg.SLDKDBUser, cfg.SLDKDBPassword, cfg.SLDKDBPort, cfg.SLDKDBName, 10)
	if db == nil {
		log.Println("[ERROR] Gagal membuat koneksi ke SLDK:", err)
		return
	}
	SLDKDBTerkonfigurasi = db
	if err != nil {
		log.Println("[ERROR] Gagal terhubung ke SLDK (Interchange):", err)
		return
	}

	log.Println("[INFO] Berhasil terhubung ke SLDK:", cfg.SLDKDBName)
	SLDKDB = db
}

// ConnectDatamart membuka koneksi ke datamart SLDK bila DATAMART_DB_HOST terisi. Tidak fatal: datamart hanya cadangan sinkronisasi Digitalisasi Aset. Pengguna
// datamart cukup berhak SELECT pada skema DJKN dan dbo.datamart_sldk_run (lihat docs/datamart-sldk.md).
func ConnectDatamart() {
	cfg := config.Cfg

	if cfg.DatamartDBHost == "" {
		return
	}
	db, err := bukaKolamBaca(cfg.DatamartDBHost, cfg.DatamartDBUser, cfg.DatamartDBPassword, cfg.DatamartDBPort, cfg.DatamartDBName, 5)
	if db == nil {
		log.Println("[ERROR] Gagal membuat koneksi ke datamart SLDK:", err)
		return
	}
	DatamartDB = db
	if err != nil {
		log.Println("[WARN] Datamart SLDK belum dapat dihubungi saat server dimulai (akan dicoba lagi saat dipakai):", err)
		return
	}
	log.Println("[INFO] Berhasil terhubung ke datamart SLDK:", cfg.DatamartDBName)
}
