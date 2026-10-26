package digitalisasi

import (
	"fmt"
	"strings"
)

// Riwayat bulanan Digitalisasi Aset: tabel DIGITALISASI_ASET_ALL menampung potret (snapshot) isi ketujuh tabel DIGITALISASI_* pada akhir tiap bulan, supaya perubahan data dapat
// dilihat sebagai deret waktu (jumlah, luas, nilai, kondisi, status hukum, dan seterusnya per bulan). Satu baris di tabel ini = satu baris sumber pada satu periode.
//
// Ketujuh tabel sumber kolomnya berbeda-beda, jadi tabel ini memakai kolom inti seragam bertipe asli (dipetakan lewat Roles tiap dataset, sehingga Alamat_tanah, Alamat_bangunan,
// dan seterusnya sama-sama menjadi "alamat") ditambah satu kolom JSON (data_tambahan) berisi kolom khusus jenis aset itu (mis. jumlah kamar, jumlah KDJ/KDO), tanpa puluhan kolom
// kosong. Kolom data pribadi, tautan foto, dan nama pengguna bangunan TIDAK disalin (HistoryExcluded): riwayat tidak boleh menyimpan lebih banyak daripada yang boleh keluar lewat API.
//
// File ini hanya menyiapkan tabel dan SQL penyimpanannya (HistorySQL). Pemicu otomatis tiap akhir bulan belum ada.

const (
	// HistoryTable: nama tabel riwayat bulanan (di database PASTI, bersama tabel DIGITALISASI_* lainnya).
	HistoryTable = "DIGITALISASI_ASET_ALL"
	// HistoryMigrationFile: file migrasi yang membuat tabel itu.
	HistoryMigrationFile = "065_create_digitalisasi_aset_all.sql"
)

// historyColumn: satu kolom inti tabel riwayat dan dari kolom mana pada tiap dataset isinya diambil (kosong bila dataset tidak punya).
type historyColumn struct {
	Name   string
	Kind   Kind
	Size   int // Text: panjang; 0 = MAX
	Scale  int // Decimal
	Remark string
	source func(Dataset) string
}

func role(f func(Roles) string) func(Dataset) string {
	return func(d Dataset) string { return f(d.Roles) }
}

// kolomBernama: nama kolom dataset yang sama dengan name (tanpa membedakan huruf besar/kecil), atau kosong.
func kolomBernama(name string) func(Dataset) string {
	return func(d Dataset) string {
		if c, ok := d.Column(name); ok {
			return c.Name
		}
		return ""
	}
}

// historyColumns: kolom inti, menurut urutan di tabel. Panjang dan skala dipilih sama dengan kolom terbesar di tujuh dataset (dijaga tes: tidak ada nilai yang terpotong).
var historyColumns = []historyColumn{
	{Name: "kode_ue1", Kind: Text, Size: szUE1, source: role(func(r Roles) string { return r.UE1 })},
	{Name: "kode_satker", Kind: Text, Size: szSatker, source: role(func(r Roles) string { return r.Satker })},
	{Name: "nama_satker", Kind: Text, Size: szNama, source: role(func(r Roles) string { return r.NamaSatker })},
	{Name: "kode_register", Kind: Text, Size: szRegister, Remark: "kode register aset (tanah, gedung, rumah negara)", source: role(func(r Roles) string { return r.KodeRegister })},
	{Name: "kode_barang", Kind: Text, Size: szKode, source: role(func(r Roles) string { return r.Kode })},
	{Name: "nup", Kind: Text, Size: szKode, source: role(func(r Roles) string { return r.NUP })},
	{Name: "uraian", Kind: Text, Size: szNama, source: role(func(r Roles) string { return r.Uraian })},
	{Name: "alamat", Kind: Text, source: role(func(r Roles) string { return r.Alamat })},
	{Name: "kelurahan_desa", Kind: Text, Size: szRegion, source: role(func(r Roles) string { return r.Kelurahan })},
	{Name: "kecamatan", Kind: Text, Size: szRegion, source: role(func(r Roles) string { return r.Kecamatan })},
	{Name: "kab_kota", Kind: Text, Size: szRegion, source: role(func(r Roles) string { return r.KabKota })},
	{Name: "provinsi", Kind: Text, Size: szRegion, source: role(func(r Roles) string { return r.Provinsi })},
	{Name: "luas", Kind: Decimal, Scale: 4, Remark: "luas tanah (jenis tanah) atau luas bangunan", source: role(func(r Roles) string { return r.Luas })},
	{Name: "nilai", Kind: Decimal, Scale: 2, source: role(func(r Roles) string { return r.Nilai })},
	{Name: "kondisi", Kind: Text, Size: szKondisi, source: role(func(r Roles) string { return r.Kondisi })},
	{Name: "status_hukum", Kind: Text, source: role(func(r Roles) string { return r.StatusHukum })},
	{Name: "status_asuransi", Kind: Text, Size: 20, source: role(func(r Roles) string { return r.Asuransi })},
	{Name: "status_penghuni", Kind: Text, Size: szStatus, source: role(func(r Roles) string { return r.StatusPenghuni })},
	{Name: "latitude", Kind: Coord, source: kolomBernama("Latitude")},
	{Name: "longitude", Kind: Coord, source: kolomBernama("Longitude")},
}

func (h historyColumn) sqlType() string {
	return Column{Kind: h.Kind, Size: h.Size, Scale: h.Scale}.SQLType()
}

// HistoryExcluded: kolom yang tidak pernah disalin ke riwayat: data pribadi (Sensitive), tautan foto, dan nama pengguna bangunan. Sama dengan kolom yang tidak pernah keluar lewat API
// Akses Data (tes di paket apiakses memastikan keduanya tidak menyimpang).
func (c Column) HistoryExcluded() bool {
	if c.Sensitive {
		return true
	}
	n := strings.ToLower(c.Name)
	return strings.HasPrefix(n, "foto") || n == "nama_pengguna"
}

// HistoryCoreSources: kolom dataset yang dipetakan ke kolom inti riwayat, menurut nama kolom inti (kolom inti yang tidak dimiliki dataset tidak ada di peta).
func (d Dataset) HistoryCoreSources() map[string]string {
	out := map[string]string{}
	for _, h := range historyColumns {
		if src := h.source(d); src != "" {
			out[h.Name] = src
		}
	}
	return out
}

// HistoryExtras: kolom dataset yang tidak punya padanan kolom inti dan tidak dikecualikan; isinya disimpan di kolom JSON data_tambahan (kunci = nama kolom huruf kecil).
func (d Dataset) HistoryExtras() []Column {
	used := map[string]bool{}
	for _, src := range d.HistoryCoreSources() {
		used[strings.ToLower(src)] = true
	}
	var out []Column
	for _, c := range d.Columns {
		if used[strings.ToLower(c.Name)] || c.HistoryExcluded() {
			continue
		}
		out = append(out, c)
	}
	return out
}

// HistoryColumnNames: nama semua kolom inti riwayat menurut urutan di tabel.
func HistoryColumnNames() []string {
	out := make([]string, len(historyColumns))
	for i, h := range historyColumns {
		out[i] = h.Name
	}
	return out
}

// HistorySQL: batch yang menyimpan isi tabel dataset ds ke DIGITALISASI_ASET_ALL sebagai potret periode @periode. Parameter bernama:
//
//	@periode  DATE         tanggal TERAKHIR bulan yang ditutup (mis. 2026-10-31); tabel menolak tanggal lain
//	@data_per DATETIME2(3) "data per" sumber (awal penyegaran datamart), boleh NULL
//
// Aman diulang: potret lama periode dan jenis aset yang sama dibuang lebih dulu, dalam satu transaksi dengan penyisipan (gagal = tidak ada yang berubah). Potret jenis aset lain dan
// periode lain tidak disentuh. Hasilnya satu baris satu kolom (baris): jumlah baris yang disimpan.
func HistorySQL(ds Dataset) string {
	cols := []string{"periode", "jenis_aset"}
	vals := []string{"@periode", "N'" + ds.Key + "'"}
	for _, h := range historyColumns {
		cols = append(cols, h.Name)
		if src := h.source(ds); src != "" {
			vals = append(vals, "s."+src)
		} else {
			vals = append(vals, "NULL")
		}
	}
	cols = append(cols, "data_tambahan", "data_per")
	if extras := ds.HistoryExtras(); len(extras) > 0 {
		var kv []string
		for _, c := range extras {
			kv = append(kv, fmt.Sprintf("s.%s AS %s", c.Name, strings.ToLower(c.Name)))
		}
		vals = append(vals, "(SELECT "+strings.Join(kv, ", ")+" FOR JSON PATH, WITHOUT_ARRAY_WRAPPER)")
	} else {
		vals = append(vals, "NULL")
	}
	vals = append(vals, "@data_per")

	var b strings.Builder
	b.WriteString("SET XACT_ABORT ON;\nBEGIN TRANSACTION;\n")
	fmt.Fprintf(&b, "DELETE FROM %s WHERE periode = @periode AND jenis_aset = N'%s';\n", HistoryTable, ds.Key)
	fmt.Fprintf(&b, "INSERT INTO %s (%s)\nSELECT %s\nFROM %s s\nORDER BY s.id;\n", HistoryTable, strings.Join(cols, ", "), strings.Join(vals, ", "), ds.Table)
	b.WriteString("DECLARE @baris BIGINT = @@ROWCOUNT;\nCOMMIT TRANSACTION;\nSELECT @baris AS baris;\n")
	return b.String()
}

const historyMigrationHeader = `-- Riwayat bulanan Digitalisasi Aset: potret ketujuh tabel DIGITALISASI_* pada akhir tiap bulan, untuk melihat perubahan data sebagai deret waktu.
--
-- FILE INI DIHASILKAN dari backend/digitalisasi/riwayat.go. Jangan disunting manual: ubah riwayat.go lalu
-- jalankan  go test ./digitalisasi -run TestHistoryMigrationFile -update  (tes biasa gagal bila file ini tidak sejalan).
--
-- Satu baris = satu baris tabel sumber pada satu periode. periode = TANGGAL TERAKHIR bulan yang ditutup (posisi akhir Oktober 2026 = 2026-10-31). jenis_aset = kunci dataset
-- (satker, tanah, gedung_kantor_utama, gedung_lainnya, rusunara, rumah_negara, mess_rumah_negara). Kolom inti seragam dipetakan dari kolom tiap tabel sumber; kolom khusus jenis
-- aset ada di data_tambahan (JSON, kunci = nama kolom huruf kecil). Data pribadi, tautan foto, dan nama pengguna bangunan tidak disalin.
-- Aman dijalankan berulang kali (idempotent).
`

// HistoryMigrationSQL membentuk isi file migrasi yang membuat tabel DIGITALISASI_ASET_ALL.
func HistoryMigrationSQL() string {
	var b strings.Builder
	b.WriteString(historyMigrationHeader)
	fmt.Fprintf(&b, "\nIF NOT EXISTS (SELECT * FROM sys.tables WHERE name = '%s')\nBEGIN\n", HistoryTable)
	fmt.Fprintf(&b, "    CREATE TABLE %s (\n", HistoryTable)
	b.WriteString("        id BIGINT IDENTITY(1,1) NOT NULL,\n")
	b.WriteString("        periode DATE NOT NULL,                -- tanggal terakhir bulan yang ditutup\n")
	b.WriteString("        jenis_aset NVARCHAR(40) NOT NULL,     -- kunci dataset\n")
	for _, h := range historyColumns {
		line := fmt.Sprintf("        %s %s NULL,", h.Name, h.sqlType())
		if h.Remark != "" {
			line += "  -- " + h.Remark
		}
		b.WriteString(line + "\n")
	}
	b.WriteString("        data_tambahan NVARCHAR(MAX) NULL,     -- JSON: kolom khusus jenis aset\n")
	b.WriteString("        data_per DATETIME2(3) NULL,           -- data sumber per kapan (awal penyegaran datamart, UTC)\n")
	b.WriteString("        diambil_pada DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME(),\n")
	fmt.Fprintf(&b, "        CONSTRAINT pk_%s PRIMARY KEY NONCLUSTERED (id),\n", strings.ToLower(HistoryTable))
	fmt.Fprintf(&b, "        CONSTRAINT ck_%s_periode CHECK (periode = EOMONTH(periode)),\n", strings.ToLower(HistoryTable))
	fmt.Fprintf(&b, "        CONSTRAINT ck_%s_json CHECK (data_tambahan IS NULL OR ISJSON(data_tambahan) = 1)\n", strings.ToLower(HistoryTable))
	b.WriteString("    );\n\n")
	// Penambahan tiap akhir bulan berurutan menurut periode, jadi indeks berkelompok pada periode membuat penyisipan menambah di ujung dan pembacaan satu periode berurutan.
	fmt.Fprintf(&b, "    CREATE CLUSTERED INDEX cx_%s ON %s(periode, jenis_aset, id);\n", strings.ToLower(HistoryTable), HistoryTable)
	fmt.Fprintf(&b, "    CREATE INDEX idx_%s_satker ON %s(jenis_aset, kode_satker, periode);\n", strings.ToLower(HistoryTable), HistoryTable)
	fmt.Fprintf(&b, "    CREATE INDEX idx_%s_ue1 ON %s(kode_ue1, jenis_aset, periode);\n", strings.ToLower(HistoryTable), HistoryTable)
	fmt.Fprintf(&b, "    CREATE INDEX idx_%s_register ON %s(kode_register, periode);\n", strings.ToLower(HistoryTable), HistoryTable)
	b.WriteString("END;\n")
	return b.String()
}
