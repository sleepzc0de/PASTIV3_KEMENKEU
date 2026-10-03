// Package sldk memuat pengetahuan tentang skema database SLDK (Interchange) yang dipakai bersama oleh
// handler HTTP, perintah sinkronisasi (cmd/sldk-sync), dan pemeriksa skema (probe): daftar kolom,
// tabel anak, tabel referensi, aturan pemantauan, dan SQL agregat. Satu sumber supaya ketiganya tidak
// saling menyimpang.
//
// Tabel utama (DJKN.SIMAN2_M_ASET) berisi ~132 juta baris (~206 GB, 145 kolom). Jangan menambah query
// yang memindainya tanpa memikirkan biayanya.
package sldk

import (
	"fmt"
	"strings"
)

func QuoteIdent(name string) string {
	return "[" + strings.ReplaceAll(name, "]", "]]") + "]"
}

func SplitSchemaTable(ref string) (schema, table string) {
	parts := strings.SplitN(ref, ".", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "dbo", parts[0]
}

func QuoteTableRef(ref string) string {
	schema, table := SplitSchemaTable(ref)
	return QuoteIdent(schema) + "." + QuoteIdent(table)
}

// SiblingTable memberi nama tabel SLDK lain yang berskema sama dengan tabel aset (mis. DJKN.SIMAN2_R_SATKER).
func SiblingTable(assetTableRef, name string) string {
	schema, _ := SplitSchemaTable(assetTableRef)
	return QuoteIdent(schema) + "." + QuoteIdent(name)
}

// AssetColumns: kolom M_ASET yang dikirim ke frontend. Data pribadi (mis. nm_penghuni) sengaja tidak ikut.
var AssetColumns = []string{
	// identitas & klasifikasi
	"id_aset", "kode_register", "kd_satker", "id_satker", "kd_brg", "ur_sskel", "no_aset", "no_kib", "kd_jns_bmn",
	"jns_aset", "merk", "tipe", "serial_number", "no_polisi", "kuantitas", "intra_extra", "status_bmn_yn", "tercatat",
	// nilai
	"rph_aset", "rph_mutasi", "rph_susut", "rph_buku", "umur_sisa", "kd_dsr_hrg", "cara_perlh", "asl_perlh",
	"kd_sumber_dana", "ur_sumber_dana", "no_dok_perolehan",
	// tanggal
	"tgl_perlh", "tgl_rekam", "tgl_rekam_pertama", "tgl_buku_pertama", "tgl_guna", "tgl_renov", "tgl_hapus",
	// status & kondisi
	"kd_kondisi", "kd_status", "status_pengelolaan", "status_bmn_idle", "kd_jns_idle", "brg_hilang_yn", "brg_rusak_yn",
	"dihentikan_yn", "hapus_lainnya_yn", "rencana_hibah_yn", "kemitraan_yn", "properti_investasi_yn", "status_sbsn",
	"kmk_sbsn", "no_dana", "tgl_dana", "tgl_akhir_sbsn", "status_sanksi",
	// lokasi
	"alamat", "alamat_lain", "vc_alamat_lengkap", "komplek", "kd_rtrw", "ur_kel", "ur_kec", "ur_kab", "ur_prov", "kd_pos",
	"gps_latitude", "gps_longitude", "lokasi_ruang", "negara", "bts_utara", "bts_selatan", "bts_barat", "bts_timur",
	// tanah & bangunan
	"luas", "luas_tapak", "luas_tnhl", "luas_tnhk", "luas_tnhb", "luas_pemanfaatan", "jml_lantai", "jml_bdg", "jml_bidang",
	"bentuk", "peruntukan", "peruntukan_tnh", "topografi_kontur", "topografi_elevasi", "aksesibilitas", "panjang", "lebar",
	"optimalisasi", "kapasitas", "sbsk",
	// dokumen & hukum
	"kd_status_hukum", "no_perkara_hukum", "jns_dok_bukti_kepemilikan", "no_dok_bukti_kepemilikan",
	"stat_dok_bukti_kepemilikan", "tgl_dok_bukti_kepemilikan", "jns_sertifikat", "no_psp", "tgl_psp",
	// pengguna
	"jns_pengguna", "kd_unit_pengguna", "nm_unit_pengguna", "ket_pengguna",
	// lain-lain & kualitas data
	"catatan", "jml_photo", "status_data", "sts_his", "sts_ast", "dq_tgl_invalid_cnt", "_ingestion_date", "updated_at",
}

// TextColumns: kolom teks yang boleh dipakai pencarian teks bebas. SLDK_ASSET_SEARCH_COLUMNS di .env
// dipersempit ke himpunan ini supaya nilai di .env tidak bisa menyisipkan kolom sembarang.
var TextColumns = map[string]bool{
	"ur_sskel": true, "merk": true, "tipe": true, "alamat": true, "alamat_lain": true, "vc_alamat_lengkap": true,
	"nm_unit_pengguna": true, "ur_kel": true, "ur_kec": true, "ur_kab": true, "ur_prov": true, "catatan": true,
	"serial_number": true, "no_polisi": true, "kode_register": true, "no_kib": true, "kd_brg": true,
}

var DefaultTextColumns = []string{"ur_sskel", "merk", "tipe", "alamat", "nm_unit_pengguna"}

// ============ Kartu aset: bagian per jenis dari tabel anak yang kecil ============

// DetailSection satu bagian kartu aset. Tabel anak ini kecil (di bawah ~1 GB), jadi aman dipindai langsung per
// id_aset. Tabel besar seperti SIMAN2_T_PENGELOLAAN_DETAIL (~208 GB) dan SIMAN2_M_ASET_PEMAKAI (berisi data
// pribadi) sengaja tidak dipakai.
type DetailSection struct {
	Key     string
	Table   string
	Columns []string
	Where   string // memakai @p1 = id_aset
	OrderBy string
	Top     int
}

func (s DetailSection) Query(tableRef string) string {
	cols := make([]string, len(s.Columns))
	for i, c := range s.Columns {
		cols[i] = QuoteIdent(c)
	}
	q := fmt.Sprintf("SELECT TOP (%d) %s FROM %s WHERE %s", s.Top, strings.Join(cols, ", "), tableRef, s.Where)
	if s.OrderBy != "" {
		q += " ORDER BY " + s.OrderBy
	}
	return q
}

var DetailSections = []DetailSection{
	{Key: "kendaraan", Table: "SIMAN2_M_KANGK", Top: 3, Where: "[id_aset] = @p1",
		Columns: []string{"pabrik", "thn_rakit", "thn_buat", "negara", "muat", "bobot", "daya", "msn_gerak", "jml_msn", "bhn_bakar", "no_mesin", "no_rangka", "no_polisi"}},
	{Key: "tik", Table: "SIMAN2_M_KKTIK", Top: 3, Where: "[id_aset] = @p1",
		Columns: []string{"jns_processor", "processor", "ram", "hdd", "monitor", "spek_lain"}},
	{Key: "riwayat_nopol", Table: "SIMAN2_M_ASET_NOPOL", Top: 20, Where: "[id_aset] = @p1", OrderBy: "[tgl_keluar] DESC",
		Columns: []string{"no_polisi", "jenis_plat_nopol", "tgl_keluar", "tgl_sd_berlaku", "ket", "terakhir_yn"}},
	{Key: "konstruksi_bangunan", Table: "SIMAN2_M_ASET_KONS_BDG", Top: 3, Where: "[id_aset] = @p1", OrderBy: "[tgl_inv] DESC",
		Columns: []string{"tgl_inv", "str_atap", "str_rangka", "material_atap", "material_langit", "lantai", "pelapis_dindin_dlm", "pelapis_dindin_lr", "perkerasan", "pagar", "kd_kondisi", "matrial_dinding"}},
	{Key: "tanah_bangunan", Table: "SIMAN2_M_ASET_TANAH_BANGUNAN", Top: 10, Where: "[id_aset_tanah] = @p1 OR [id_aset_bangunan] = @p1",
		Columns: []string{"id_aset_tanah", "id_aset_bangunan", "nm_pemilik_bangunan", "ur_jenis_kepemilikan", "jml_lantai", "luas_bangunan", "luas_dasar_bangunan", "keterangan"}},
	{Key: "objek_tanah", Table: "SIMAN2_M_ASET_OBJEK_TANAH", Top: 3, Where: "[id_aset] = @p1",
		Columns: []string{"luas", "ukuran", "lebar", "is_rawan_bencana", "is_permasalahan_hukum", "tahun_pajak", "njop", "njop_per_meter", "kode_pos", "lebar_jalan", "nm_jalan_utama", "jarak_jalan_utama", "nm_cbd_terdekat", "jarak_cbd_terdekat", "koordinat"}},
	{Key: "riwayat_hukum", Table: "SIMAN2_M_ASET_HUKUM", Top: 20, Where: "[id_aset] = @p1", OrderBy: "[tgl] DESC",
		Columns: []string{"tgl", "phk_sengketa", "ur_masalah", "no_reg_perkara", "kd_status_hukum", "terakhir_yn"}},
	{Key: "foto", Table: "SIMAN2_M_ASET_PHOTO", Top: 10, Where: "[id_aset] = @p1", OrderBy: "[tanggal] DESC",
		Columns: []string{"nm_photo", "ket_photo", "tanggal", "photo_utama_yn"}},
}

// ============ Tabel referensi (kode -> nama) ============

type RefDef struct {
	Key      string
	Table    string
	CodeSQL  string
	NameSQL  string
	OrderSQL string
	Columns  []string // kolom yang dibaca, untuk pemeriksaan skema
}

func (r RefDef) Query(tableRef string) string {
	return fmt.Sprintf("SELECT %s, %s FROM %s ORDER BY %s", r.CodeSQL, r.NameSQL, tableRef, r.OrderSQL)
}

// Tabelnya kecil (belasan baris), aman dimuat langsung.
var RefDefs = []RefDef{
	{Key: "jenis_bmn", Table: "SIMAN2_R_JNS_BMN", CodeSQL: "CAST([kd_jns_bmn] AS nvarchar(20))", NameSQL: "[nm_jns_bmn]",
		OrderSQL: "[order_no], [kd_jns_bmn]", Columns: []string{"kd_jns_bmn", "nm_jns_bmn", "order_no"}},
	{Key: "kondisi", Table: "SIMAN2_R_KONDISI", CodeSQL: "[kd_kondisi]", NameSQL: "[ur_kondisi]",
		OrderSQL: "[kd_kondisi]", Columns: []string{"kd_kondisi", "ur_kondisi"}},
	{Key: "status_penggunaan", Table: "SIMAN2_R_STATUS", CodeSQL: "[kd_status]", NameSQL: "[ur_status]",
		OrderSQL: "[kd_status]", Columns: []string{"kd_status", "ur_status"}},
	{Key: "status_hukum", Table: "SIMAN2_R_STATUS_HUKUM", CodeSQL: "[kd_status_hukum]",
		NameSQL: "COALESCE(NULLIF([status_hukum], ''), [jns_status_hukum])", OrderSQL: "[kd_status_hukum]",
		Columns: []string{"kd_status_hukum", "status_hukum", "jns_status_hukum"}},
}

// ============ Satker & K/L ============

const (
	TableSatker = "SIMAN2_R_SATKER"
	TableKL     = "SIMAN2_R_KL"
)

var (
	SatkerColumns = []string{"id_satker", "kd_satker", "ur_satker", "id_kl"}
	KLColumns     = []string{"id_kl", "kd_kl", "ur_kl"}
)

// ScopePredicate membatasi baris M_ASET ke satker milik satu K/L menurut kodenya (mis. 015 untuk
// Kementerian Keuangan). param adalah placeholder kode K/L (mis. "@p1"). Bergantung pada asumsi bahwa
// id_satker di M_ASET sama dengan id_satker di R_SATKER; `sldk-sync probe` memeriksanya.
func ScopePredicate(assetTableRef, param string) string {
	return fmt.Sprintf("[id_satker] IN (SELECT s.[id_satker] FROM %s s WHERE s.[id_kl] IN (SELECT k.[id_kl] FROM %s k WHERE k.[kd_kl] = %s))",
		SiblingTable(assetTableRef, TableSatker), SiblingTable(assetTableRef, TableKL), param)
}

// SatkerScopePredicate sama dengan ScopePredicate tetapi untuk tabel R_SATKER (pemilih satker).
func SatkerScopePredicate(assetTableRef, param string) string {
	return fmt.Sprintf("[id_kl] IN (SELECT k.[id_kl] FROM %s k WHERE k.[kd_kl] = %s)", SiblingTable(assetTableRef, TableKL), param)
}

// ExpectedColumns: tabel (tanpa skema) -> kolom yang dibaca kode ini, untuk pemeriksaan skema oleh probe.
func ExpectedColumns(assetTableRef string) map[string][]string {
	_, assetTable := SplitSchemaTable(assetTableRef)
	out := map[string][]string{assetTable: append([]string(nil), AssetColumns...)}
	for _, s := range DetailSections {
		out[s.Table] = s.Columns
	}
	for _, r := range RefDefs {
		out[r.Table] = r.Columns
	}
	out[TableSatker] = SatkerColumns
	out[TableKL] = KLColumns
	return out
}
