package digitalisasi

import (
	"fmt"
	"strings"
)

// MigrationFile adalah nama file migrasi yang dibentuk dari daftar Datasets.
const MigrationFile = "020_create_digitalisasi.sql"

const migrationHeader = `-- Tabel digitalisasi aset (KL 015) yang disalin dari SLDK, plus riwayat sinkronisasinya.
--
-- FILE INI DIHASILKAN dari backend/digitalisasi/datasets.go. Jangan disunting manual: ubah datasets.go lalu
-- jalankan  go test ./digitalisasi -run TestMigrationFile -update  (tes biasa gagal bila file ini tidak sejalan).
--
-- Tabel DIGITALISASI_* diisi oleh fitur Sinkronisasi di halaman Digitalisasi Aset (isinya diganti penuh tiap
-- sinkronisasi). Kolom id_sinkron menunjuk ke digitalisasi_sync_log.id.
`

// indexedRoles: peran kolom yang diberi index (semuanya pendek, bukan NVARCHAR(MAX)).
func indexedColumns(ds Dataset) []string {
	var cols []string
	for _, name := range []string{ds.Roles.UE1, ds.Roles.Satker, ds.Roles.Provinsi} {
		if name == "" {
			continue
		}
		if c, ok := ds.Column(name); ok && !(c.Kind == Text && c.Size == 0) {
			cols = append(cols, c.Name)
		}
	}
	return cols
}

// MigrationSQL membentuk isi file migrasi yang membuat semua tabel DIGITALISASI_* dan digitalisasi_sync_log.
func MigrationSQL() string {
	var b strings.Builder
	b.WriteString(migrationHeader)

	b.WriteString(`
-- Riwayat sinkronisasi per dataset: antri -> berjalan -> sukses | gagal | dibatalkan.
IF NOT EXISTS (SELECT * FROM sys.tables WHERE name = 'digitalisasi_sync_log')
BEGIN
    CREATE TABLE digitalisasi_sync_log (
        id BIGINT IDENTITY(1,1) PRIMARY KEY,
        dataset NVARCHAR(40) NOT NULL,       -- kunci dataset (lihat datasets.go)
        status NVARCHAR(20) NOT NULL,        -- antri | berjalan | sukses | gagal | dibatalkan
        dibuat DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME(),
        mulai DATETIME2 NULL,
        selesai DATETIME2 NULL,
        jumlah_baris BIGINT NULL,
        jumlah_koordinat BIGINT NULL,        -- baris yang punya koordinat valid
        pesan NVARCHAR(1000) NULL,           -- kemajuan saat berjalan; ringkasan atau galat setelah selesai
        dijalankan_oleh NVARCHAR(100) NULL
    );

    CREATE INDEX idx_digitalisasi_sync_log_dataset ON digitalisasi_sync_log(dataset, id);
END;
`)

	for _, ds := range Datasets {
		fmt.Fprintf(&b, "\n-- %s: %s\n", ds.Table, ds.Description)
		fmt.Fprintf(&b, "IF NOT EXISTS (SELECT * FROM sys.tables WHERE name = '%s')\nBEGIN\n", ds.Table)
		fmt.Fprintf(&b, "    CREATE TABLE %s (\n", ds.Table)
		b.WriteString("        id BIGINT IDENTITY(1,1) PRIMARY KEY,\n")
		for _, c := range ds.Columns {
			line := fmt.Sprintf("        %s %s NULL,", c.Name, c.SQLType())
			if c.Sensitive {
				line += "  -- data pribadi: hanya tampil untuk admin"
			}
			b.WriteString(line + "\n")
		}
		b.WriteString("        id_sinkron BIGINT NULL,\n")
		b.WriteString("        synced_at DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME()\n")
		b.WriteString("    );\n")
		if cols := indexedColumns(ds); len(cols) > 0 {
			b.WriteString("\n")
			for _, col := range cols {
				fmt.Fprintf(&b, "    CREATE INDEX idx_%s_%s ON %s(%s);\n", strings.ToLower(ds.Table), strings.ToLower(col), ds.Table, col)
			}
		}
		b.WriteString("END;\n")
	}
	return b.String()
}
