// Command migrate menerapkan file SQL di folder migrations/ ke database PASTI V3
// secara berurutan (urut nama file) dan mencatat file yang sudah diterapkan di tabel
// schema_migrations, sehingga setiap file hanya dijalankan sekali.
//
// Memakai konfigurasi dan driver yang sama dengan aplikasi (backend/.env), jadi
// tidak butuh sqlcmd atau alat lain. Dipanggil oleh deploy.sh, tapi bisa juga
// dijalankan manual:
//
//	go run ./cmd/migrate -dry-run   # hanya menampilkan migrasi yang belum diterapkan
//	go run ./cmd/migrate -identitas # hanya mencetak jati diri server database yang sebenarnya
//	go run ./cmd/migrate            # menerapkan migrasi yang belum diterapkan
package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"pasti-v3-backend/config"
	"pasti-v3-backend/database"
)

// Baris yang hanya berisi GO (pemisah batch ala SSMS/sqlcmd) bukan T-SQL yang
// valid untuk driver, jadi skrip dipecah di baris tersebut.
var goSeparator = regexp.MustCompile(`(?im)^[ \t]*GO[ \t]*\r?$`)

func splitBatches(script string) []string {
	var batches []string
	for _, b := range goSeparator.Split(script, -1) {
		if strings.TrimSpace(b) != "" {
			batches = append(batches, b)
		}
	}
	return batches
}

// listMigrationFiles mengembalikan nama file *.sql di dir, terurut. File yang
// namanya memuat "verify" dilewati: itu skrip audit (hanya SELECT/PRINT), bukan
// perubahan skema.
func listMigrationFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(name), ".sql") {
			continue
		}
		if strings.Contains(strings.ToLower(name), "verify") {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}

func trackingTableExists(db *sql.DB) (bool, error) {
	var id sql.NullInt64
	err := db.QueryRow(`SELECT OBJECT_ID('dbo.schema_migrations', 'U')`).Scan(&id)
	if err != nil {
		return false, err
	}
	return id.Valid, nil
}

func ensureTrackingTable(db *sql.DB) error {
	_, err := db.Exec(`
		IF OBJECT_ID('dbo.schema_migrations', 'U') IS NULL
		CREATE TABLE dbo.schema_migrations (
			name NVARCHAR(255) NOT NULL PRIMARY KEY,
			applied_at DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME()
		)`)
	return err
}

func appliedMigrations(db *sql.DB) (map[string]bool, error) {
	rows, err := db.Query(`SELECT name FROM dbo.schema_migrations`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	applied := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		applied[name] = true
	}
	return applied, rows.Err()
}

// applyMigration menjalankan satu file dalam satu transaksi: kalau ada batch yang
// gagal, seluruh file dibatalkan dan tidak dicatat sebagai sudah diterapkan.
func applyMigration(db *sql.DB, name, script string) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	for i, batch := range splitBatches(script) {
		if _, err := tx.Exec(batch); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("batch %d gagal: %w", i+1, err)
		}
	}
	if _, err := tx.Exec(`INSERT INTO dbo.schema_migrations (name) VALUES (@p1)`, name); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("gagal mencatat migrasi: %w", err)
	}
	return tx.Commit()
}

func run(db *sql.DB, dir string, dryRun bool) error {
	files, err := listMigrationFiles(dir)
	if err != nil {
		return fmt.Errorf("gagal membaca folder %q: %w", dir, err)
	}
	if len(files) == 0 {
		return errors.New("tidak ada file .sql di folder " + dir)
	}

	exists, err := trackingTableExists(db)
	if err != nil {
		return err
	}
	if !exists && !dryRun {
		if err := ensureTrackingTable(db); err != nil {
			return fmt.Errorf("gagal membuat tabel schema_migrations: %w", err)
		}
		exists = true
	}

	applied := map[string]bool{}
	if exists {
		if applied, err = appliedMigrations(db); err != nil {
			return err
		}
	}

	var pending []string
	for _, name := range files {
		if !applied[name] {
			pending = append(pending, name)
		}
	}

	if len(pending) == 0 {
		log.Printf("[MIGRATE] Tidak ada migrasi baru (%d file sudah diterapkan).", len(files))
		return nil
	}

	if dryRun {
		log.Printf("[MIGRATE] %d migrasi belum diterapkan (dry-run, tidak ada yang dijalankan):", len(pending))
		for _, name := range pending {
			log.Println("  -", name)
		}
		return nil
	}

	for _, name := range pending {
		script, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return fmt.Errorf("gagal membaca %s: %w", name, err)
		}
		log.Printf("[MIGRATE] Menerapkan %s ...", name)
		if err := applyMigration(db, name, string(script)); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
	}

	log.Printf("[MIGRATE] Selesai: %d diterapkan, %d sudah ada sebelumnya.", len(pending), len(files)-len(pending))
	return nil
}

// cetakIdentitas menulis jati diri server database (nama yang dilaporkan SQL Server, bukan alamat yang diketik di DB_HOST) sebagai baris KUNCI=nilai ke w. Satu-satunya
// keluaran di stdout; pesan log lain ke stderr. deploy.sh mencocokkannya dengan EXPECTED_DB_SERVER sebelum migrasi menyentuh database.
func cetakIdentitas(w io.Writer, db *sql.DB) error {
	ctx, batal := context.WithTimeout(context.Background(), 15*time.Second)
	defer batal()
	id, err := database.BacaIdentitas(ctx, db)
	if err != nil {
		return err
	}
	_, err = io.WriteString(w, id.Keluaran())
	return err
}

func main() {
	defaultDir := "migrations"
	if v := os.Getenv("MIGRATIONS_DIR"); v != "" {
		defaultDir = v
	}
	dir := flag.String("dir", defaultDir, "folder berisi file migrasi .sql")
	dryRun := flag.Bool("dry-run", false, "hanya tampilkan migrasi yang belum diterapkan")
	identitas := flag.Bool("identitas", false, "hanya cetak jati diri server database yang sebenarnya (dibaca deploy.sh), tanpa migrasi")
	flag.Parse()

	config.LoadConfig()
	database.Connect()
	defer database.DB.Close()

	if *identitas {
		if err := cetakIdentitas(os.Stdout, database.DB); err != nil {
			log.Fatal("[MIGRATE][FATAL] ", err)
		}
		return
	}

	if err := run(database.DB, *dir, *dryRun); err != nil {
		log.Fatal("[MIGRATE][FATAL] ", err)
	}
}
