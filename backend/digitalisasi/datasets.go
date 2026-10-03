// Package digitalisasi menyalin hasil tujuh query "digitalisasi aset" dari SLDK (Interchange) ke tabel
// DIGITALISASI_* di database PASTI, supaya dashboard (ringkasan, peta, daftar) tidak perlu menyentuh tabel aset
// SLDK yang berukuran ratusan GB.
//
// Satu sumber kebenaran: daftar kolom di bawah dipakai untuk (1) membuat migrasi tabel, (2) memetakan hasil
// query ke kolom tujuan, (3) membentuk query analitik. Migrasi 020 dicek terhadap daftar ini oleh tes.
package digitalisasi

import (
	"embed"
	"fmt"
	"strings"
)

//go:embed queries/*.sql
var queryFS embed.FS

// Kind adalah jenis nilai kolom tujuan.
type Kind int

const (
	Text    Kind = iota // NVARCHAR(Size); Size 0 = NVARCHAR(MAX)
	Int                 // INT
	BigInt              // BIGINT (id aset)
	Decimal             // DECIMAL(28, Scale); dikirim sebagai string agar tidak kehilangan presisi
	Coord               // lintang/bujur: DECIMAL(10, 7); nilai di luar rentang bumi dibuang (NULL)
)

// Column memetakan satu kolom hasil query (Source) ke kolom tabel tujuan (Name).
type Column struct {
	Name      string
	Source    string // nama kolom pada hasil query; kosong = sama dengan Name
	Kind      Kind
	Size      int  // Text: panjang; 0 = MAX
	Scale     int  // Decimal: jumlah desimal
	Sensitive bool // data pribadi: hanya untuk admin, tidak ikut pencarian
	Display   bool // tampil di tabel daftar (urutan = urutan kolom ini)
}

func (c Column) SourceName() string {
	if c.Source != "" {
		return c.Source
	}
	return c.Name
}

// SQLType: tipe kolom tujuan untuk migrasi.
func (c Column) SQLType() string {
	switch c.Kind {
	case Int:
		return "INT"
	case BigInt:
		return "BIGINT"
	case Decimal:
		return fmt.Sprintf("DECIMAL(28, %d)", c.Scale)
	case Coord:
		return "DECIMAL(10, 7)"
	default:
		if c.Size > 0 {
			return fmt.Sprintf("NVARCHAR(%d)", c.Size)
		}
		return "NVARCHAR(MAX)"
	}
}

// Roles menamai kolom yang berperan sama di tiap dataset (nama kolomnya berbeda-beda), untuk analitik generik.
type Roles struct {
	UE1, Satker, NamaSatker               string
	Uraian, Kode, KodeRegister, NUP       string
	Alamat, Kelurahan, Kecamatan, KabKota string
	Provinsi                              string
	Luas, Nilai                           string
	Kondisi, StatusHukum, Foto, Asuransi  string
	StatusPenghuni                        string
}

// Dataset: satu query SLDK dan satu tabel tujuan.
type Dataset struct {
	Key         string
	Table       string
	Label       string
	Description string
	QueryFile   string
	Columns     []Column
	Roles       Roles
	Geo         bool // punya kolom Latitude/Longitude (muncul di peta)
	// Keep: kolom yang diisi manual di tabel tujuan (query selalu mengirim NULL). Nilainya dipertahankan per
	// KeyColumn saat sinkronisasi bila sumber mengirim NULL.
	Keep      []string
	KeyColumn string
	// SearchColumns: kolom teks yang dicari lewat kotak pencarian daftar (tanpa kolom pribadi).
	SearchColumns []string
}

func (d Dataset) Query() (string, error) {
	b, err := queryFS.ReadFile("queries/" + d.QueryFile)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// Column mencari kolom tujuan menurut nama (tanpa membedakan huruf besar/kecil).
func (d Dataset) Column(name string) (Column, bool) {
	for _, c := range d.Columns {
		if strings.EqualFold(c.Name, name) {
			return c, true
		}
	}
	return Column{}, false
}

// DisplayColumns: kolom untuk tabel daftar. Kolom pribadi hanya ikut bila includeSensitive.
func (d Dataset) DisplayColumns(includeSensitive bool) []Column {
	var out []Column
	for _, c := range d.Columns {
		if c.Display && (includeSensitive || !c.Sensitive) {
			out = append(out, c)
		}
	}
	return out
}

func txt(name string, size int) Column {
	return Column{Name: name, Kind: Text, Size: size, Display: true}
}
func hid(name string, size int) Column { return Column{Name: name, Kind: Text, Size: size} } // tidak tampil di daftar
func cnt(name string) Column           { return Column{Name: name, Kind: Int, Display: true} }
func dec(name string, scale int) Column {
	return Column{Name: name, Kind: Decimal, Scale: scale, Display: true}
}
func coord(name, source string) Column { return Column{Name: name, Source: source, Kind: Coord} }

// Panjang kolom: kode dan wilayah diberi ruang lega; teks bebas (alamat, uraian panjang) NVARCHAR(MAX).
const (
	szUE1      = 10
	szSatker   = 40
	szNama     = 500
	szKode     = 30
	szRegister = 60
	szRegion   = 200
	szRTRW     = 50
	szKondisi  = 100
	szStatus   = 100
)

// Sisipan kolom yang sama di tiap dataset aset.
func head() []Column {
	return []Column{hid("Kode_UE1", szUE1), txt("Kode_Satker", szSatker), txt("Nama_Satker", szNama)}
}

func gps() []Column {
	return []Column{coord("Latitude", "GPS_Latitude"), coord("Longitude", "GPS_Longitude")}
}

func join(parts ...[]Column) []Column {
	var out []Column
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

// Datasets: urutan = urutan tampil di UI dan urutan "Sinkronkan semua".
var Datasets = []Dataset{
	{
		Key: "satker", Table: "DIGITALISASI_SATKER", Label: "Satuan Kerja", QueryFile: "satker.sql",
		Description: "Satker aktif KL 015 (anak satker pada UE1 tertentu digabung ke induk) beserta jumlah kendaraan dinas",
		Columns: []Column{
			hid("Kode_UE1", szUE1), txt("Kode_Satker", szSatker), txt("Jenis_Satker", 20), txt("Nama_Satker", szNama),
			txt("Alamat_Satker", 0), txt("KelurahanDesa_Satker", szRegion), txt("Kecamatan_Satker", szRegion),
			txt("KabKota_Satker", szRegion), txt("Provinsi_Satker", szRegion),
			txt("Status_Gedung_Kantor", 200), txt("Foto", 900),
			cnt("Jumlah_KDJ"), txt("Kondisi_KDJ", 0), cnt("Jumlah_KDO"), txt("Kondisi_KDO", 0),
		},
		Roles: Roles{UE1: "Kode_UE1", Satker: "Kode_Satker", NamaSatker: "Nama_Satker", Alamat: "Alamat_Satker",
			Kelurahan: "KelurahanDesa_Satker", Kecamatan: "Kecamatan_Satker", KabKota: "KabKota_Satker", Provinsi: "Provinsi_Satker", Foto: "Foto"},
		Keep: []string{"Status_Gedung_Kantor", "Foto"}, KeyColumn: "Kode_Satker",
		SearchColumns: []string{"Kode_Satker", "Nama_Satker", "Alamat_Satker", "KabKota_Satker", "Provinsi_Satker"},
	},
	{
		Key: "tanah", Table: "DIGITALISASI_TANAH", Label: "Tanah", QueryFile: "tanah.sql", Geo: true,
		Description: "Aset tanah (kd_brg 2%) KL 015 beserta status hukum, jumlah bangunan di atasnya, dan koordinat",
		Columns: join([]Column{
			{Name: "id_aset_tanah", Kind: BigInt}, hid("kode_register_tanah", szRegister), hid("Kode_UE1", szUE1),
			txt("Kode_Satker", szSatker), txt("Nama_Satker", szNama),
			txt("Kode_tanah", szKode), txt("Uraian_tanah", szNama), txt("No_aset", szKode),
			txt("Alamat_tanah", 0), hid("RTRW_tanah", szRTRW), txt("KelurahanDesa_tanah", szRegion), txt("Kecamatan_tanah", szRegion),
			txt("KabKota_tanah", szRegion), txt("Provinsi_tanah", szRegion),
			dec("Luas_Tanah", 4), dec("Luas_Bangunan", 4), cnt("Jumlah_Bangunan"),
			txt("Status_hukum", 0), txt("Foto", 0), txt("Kondisi_Tanah", szKondisi), dec("Nilai_Tanah", 2),
		}, gps()),
		Roles: Roles{UE1: "Kode_UE1", Satker: "Kode_Satker", NamaSatker: "Nama_Satker", Kode: "Kode_tanah", NUP: "No_aset", KodeRegister: "kode_register_tanah",
			Uraian: "Uraian_tanah", Alamat: "Alamat_tanah", Kelurahan: "KelurahanDesa_tanah", Kecamatan: "Kecamatan_tanah", KabKota: "KabKota_tanah",
			Provinsi: "Provinsi_tanah", Luas: "Luas_Tanah", Nilai: "Nilai_Tanah", Kondisi: "Kondisi_Tanah", StatusHukum: "Status_hukum", Foto: "Foto"},
		SearchColumns: []string{"Kode_Satker", "Nama_Satker", "Uraian_tanah", "Alamat_tanah", "KabKota_tanah", "Provinsi_tanah", "kode_register_tanah"},
	},
	{
		Key: "gedung_kantor_utama", Table: "DIGITALISASI_GEDUNG_KANTOR_UTAMA", Label: "Gedung Kantor Utama", QueryFile: "gedung_kantor_utama.sql", Geo: true,
		Description: "Gedung kantor utama (kd_brg 4010101001) KL 015 beserta tanah di bawahnya, status asuransi, dan koordinat",
		Columns: join(head(), []Column{
			hid("Kode_Register_bangunan", szRegister), txt("Kode_bangunan", szKode), txt("Uraian_bangunan", szNama), txt("NUP_bangunan", szKode),
			txt("Alamat_bangunan", 0), hid("RTRW_bangunan", szRTRW), txt("KelurahanDesa_bangunan", szRegion), txt("Kecamatan_bangunan", szRegion),
			txt("KabKota_bangunan", szRegion), txt("Provinsi_bangunan", szRegion),
			dec("Luas_Bangunan", 4), txt("Foto_bangunan", 0), txt("Kondisi_Bangunan", szKondisi), dec("Nilai_Bangunan", 2),
			txt("Status_Asuransi", 20), txt("Kode_tanah", szKode), txt("Uraian_tanah", szNama), txt("No_aset_tanah", szKode),
		}, gps()),
		Roles: Roles{UE1: "Kode_UE1", Satker: "Kode_Satker", NamaSatker: "Nama_Satker", Kode: "Kode_bangunan", NUP: "NUP_bangunan", KodeRegister: "Kode_Register_bangunan",
			Uraian: "Uraian_bangunan", Alamat: "Alamat_bangunan", Kelurahan: "KelurahanDesa_bangunan", Kecamatan: "Kecamatan_bangunan", KabKota: "KabKota_bangunan",
			Provinsi: "Provinsi_bangunan", Luas: "Luas_Bangunan", Nilai: "Nilai_Bangunan", Kondisi: "Kondisi_Bangunan", Foto: "Foto_bangunan", Asuransi: "Status_Asuransi"},
		SearchColumns: []string{"Kode_Satker", "Nama_Satker", "Uraian_bangunan", "Alamat_bangunan", "KabKota_bangunan", "Provinsi_bangunan", "Kode_Register_bangunan"},
	},
	{
		Key: "gedung_lainnya", Table: "DIGITALISASI_GEDUNG_LAINNYA", Label: "Gedung Lainnya", QueryFile: "gedung_lainnya.sql", Geo: true,
		Description: "Gedung dan bangunan lain (kd_brg 40101%, selain gedung kantor utama) KL 015 beserta fungsi, status hukum, dan asuransi",
		Columns: join(head(), []Column{
			hid("Kode_Register_bangunan", szRegister), txt("Kode_bangunan", szKode), txt("Uraian_bangunan", szNama), txt("NUP_bangunan", szKode),
			txt("Alamat_bangunan", 0), hid("RTRW_bangunan", szRTRW), txt("KelurahanDesa_bangunan", szRegion), txt("Kecamatan_bangunan", szRegion),
			txt("KabKota_bangunan", szRegion), txt("Provinsi_bangunan", szRegion),
			dec("Luas_Bangunan", 4), txt("Foto_bangunan", 0), txt("Fungsi", szNama), txt("Status_Hukum", 0), txt("Nama_Pengguna", szNama),
			txt("Kondisi_Bangunan", szKondisi), dec("Nilai_Bangunan", 2), txt("Status_Asuransi", 20),
		}, gps()),
		Roles: Roles{UE1: "Kode_UE1", Satker: "Kode_Satker", NamaSatker: "Nama_Satker", Kode: "Kode_bangunan", NUP: "NUP_bangunan", KodeRegister: "Kode_Register_bangunan",
			Uraian: "Uraian_bangunan", Alamat: "Alamat_bangunan", Kelurahan: "KelurahanDesa_bangunan", Kecamatan: "Kecamatan_bangunan", KabKota: "KabKota_bangunan",
			Provinsi: "Provinsi_bangunan", Luas: "Luas_Bangunan", Nilai: "Nilai_Bangunan", Kondisi: "Kondisi_Bangunan", StatusHukum: "Status_Hukum", Foto: "Foto_bangunan", Asuransi: "Status_Asuransi"},
		SearchColumns: []string{"Kode_Satker", "Nama_Satker", "Uraian_bangunan", "Fungsi", "Alamat_bangunan", "KabKota_bangunan", "Provinsi_bangunan", "Kode_Register_bangunan"},
	},
	{
		Key: "rusunara", Table: "DIGITALISASI_RUSUNARA", Label: "Rusunara", QueryFile: "rusunara.sql", Geo: true,
		Description: "Rumah susun negara (kd_brg 4010208%) KL 015 beserta jumlah kamar tidur per tipe (E, D, C)",
		Columns: join(head(), []Column{
			txt("Kode_Rusun", szKode), txt("Uraian_Rusun", szNama), txt("NUP_Rusun", szKode),
			txt("Alamat_Rusun", 0), hid("RTRW_Rusun", szRTRW), txt("KelurahanDesa_Rusun", szRegion), txt("Kecamatan_Rusun", szRegion),
			txt("KabKota_Rusun", szRegion), txt("Provinsi_Rusun", szRegion),
			dec("Luas_Rusun", 4), txt("Foto_Rusun", 0), txt("Kondisi_Rusun", szKondisi), dec("Nilai_Rusun", 2),
			cnt("Kamar_tipe_E"), cnt("Kamar_tipe_D"), cnt("Kamar_tipe_C"),
		}, gps()),
		Roles: Roles{UE1: "Kode_UE1", Satker: "Kode_Satker", NamaSatker: "Nama_Satker", Kode: "Kode_Rusun", NUP: "NUP_Rusun",
			Uraian: "Uraian_Rusun", Alamat: "Alamat_Rusun", Kelurahan: "KelurahanDesa_Rusun", Kecamatan: "Kecamatan_Rusun", KabKota: "KabKota_Rusun",
			Provinsi: "Provinsi_Rusun", Luas: "Luas_Rusun", Nilai: "Nilai_Rusun", Kondisi: "Kondisi_Rusun", Foto: "Foto_Rusun"},
		SearchColumns: []string{"Kode_Satker", "Nama_Satker", "Uraian_Rusun", "Alamat_Rusun", "KabKota_Rusun", "Provinsi_Rusun"},
	},
	{
		Key: "rumah_negara", Table: "DIGITALISASI_RUMAH_NEGARA", Label: "Rumah Negara", QueryFile: "rumah_negara.sql", Geo: true,
		Description: "Rumah negara (kd_brg 4010201%, 4010202%, 4010209%; tanpa mess) KL 015 beserta status penghuni dan status hukum",
		Columns: join(head(), []Column{
			hid("Kode_Register_RN", szRegister), txt("Kode_RN", szKode), txt("Uraian_RN", szNama), txt("NUP_RN", szKode),
			txt("Alamat_RN", 0), hid("RTRW_RN", szRTRW), txt("KelurahanDesa_RN", szRegion), txt("Kecamatan_RN", szRegion),
			txt("KabKota_RN", szRegion), txt("Provinsi_RN", szRegion),
			dec("Luas_RN", 4), txt("Foto_RN", 0), txt("Status_Penghuni", szStatus),
			{Name: "Nama_Penghuni", Kind: Text, Size: szNama, Sensitive: true, Display: true},
			txt("Kondisi_RN", szKondisi), txt("Status_Hukum", 0),
		}, gps()),
		Roles: Roles{UE1: "Kode_UE1", Satker: "Kode_Satker", NamaSatker: "Nama_Satker", Kode: "Kode_RN", NUP: "NUP_RN", KodeRegister: "Kode_Register_RN",
			Uraian: "Uraian_RN", Alamat: "Alamat_RN", Kelurahan: "KelurahanDesa_RN", Kecamatan: "Kecamatan_RN", KabKota: "KabKota_RN",
			Provinsi: "Provinsi_RN", Luas: "Luas_RN", Kondisi: "Kondisi_RN", StatusHukum: "Status_Hukum", Foto: "Foto_RN", StatusPenghuni: "Status_Penghuni"},
		SearchColumns: []string{"Kode_Satker", "Nama_Satker", "Uraian_RN", "Alamat_RN", "KabKota_RN", "Provinsi_RN", "Kode_Register_RN"},
	},
	{
		Key: "mess_rumah_negara", Table: "DIGITALISASI_MESS_RUMAH_NEGARA", Label: "Mess Rumah Negara", QueryFile: "mess_rumah_negara.sql", Geo: true,
		Description: "Mess rumah negara (kd_brg 4010202016) KL 015 beserta status hukum dan jumlah kamar tidur tipe E dan D",
		Columns: join(head(), []Column{
			txt("Kode_mess", szKode), txt("Uraian_mess", szNama), txt("NUP_mess", szKode),
			txt("Alamat_mess", 0), hid("RTRW_mess", szRTRW), txt("KelurahanDesa_mess", szRegion), txt("Kecamatan_mess", szRegion),
			txt("KabKota_mess", szRegion), txt("Provinsi_mess", szRegion),
			dec("Luas_mess", 4), txt("Foto_mess", 0), txt("Kondisi_mess", szKondisi), dec("Nilai_mess", 2), txt("Status_Hukum", 0),
			cnt("Kamar_tipe_E"), cnt("Kamar_tipe_D"),
		}, gps()),
		Roles: Roles{UE1: "Kode_UE1", Satker: "Kode_Satker", NamaSatker: "Nama_Satker", Kode: "Kode_mess", NUP: "NUP_mess",
			Uraian: "Uraian_mess", Alamat: "Alamat_mess", Kelurahan: "KelurahanDesa_mess", Kecamatan: "Kecamatan_mess", KabKota: "KabKota_mess",
			Provinsi: "Provinsi_mess", Luas: "Luas_mess", Nilai: "Nilai_mess", Kondisi: "Kondisi_mess", StatusHukum: "Status_Hukum", Foto: "Foto_mess"},
		SearchColumns: []string{"Kode_Satker", "Nama_Satker", "Uraian_mess", "Alamat_mess", "KabKota_mess", "Provinsi_mess"},
	},
}

func ByKey(key string) (Dataset, bool) {
	for _, d := range Datasets {
		if d.Key == key {
			return d, true
		}
	}
	return Dataset{}, false
}

// Keys: semua kunci dataset menurut urutan tampil.
func Keys() []string {
	keys := make([]string, len(Datasets))
	for i, d := range Datasets {
		keys[i] = d.Key
	}
	return keys
}
