package sapa

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// ---------------------------------------------------------------- pembantu validasi

type galat []string

func (g *galat) add(format string, a ...interface{}) { *g = append(*g, fmt.Sprintf(format, a...)) }

func (g *galat) wajib(label, v string, max int) {
	v = strings.TrimSpace(v)
	switch {
	case v == "":
		g.add("%s wajib diisi", label)
	case utf8.RuneCountInString(v) > max:
		g.add("%s terlalu panjang (maksimal %d karakter)", label, max)
	}
}

func (g *galat) opsional(label, v string, max int) {
	if utf8.RuneCountInString(strings.TrimSpace(v)) > max {
		g.add("%s terlalu panjang (maksimal %d karakter)", label, max)
	}
}

func (g *galat) tanggal(label, v string, wajib bool) time.Time {
	if strings.TrimSpace(v) == "" {
		if wajib {
			g.add("%s wajib diisi", label)
		}
		return time.Time{}
	}
	t, err := ParseTanggal(v)
	if err != nil {
		g.add("%s: %s", label, err.Error())
	}
	return t
}

func dalam(v string, daftar []string) bool {
	for _, d := range daftar {
		if v == d {
			return true
		}
	}
	return false
}

func rapikan(s *string) { *s = strings.TrimSpace(*s) }

// ---------------------------------------------------------------- Pembentukan Tim

var JenisTimValid = []string{"Tim Internal Penjualan", "Tim Persiapan Hibah", "Tim Persiapan Tukar Menukar", "Tim Pelaksanaan Tukar Menukar"}

type Anggota struct {
	Nama      string `json:"nama"`
	Jabatan   string `json:"jabatan"`
	Kedudukan string `json:"kedudukan"`
	NIP       string `json:"nip"` // opsional; terisi otomatis saat anggota dipilih dari HRIS2
}

// DataTim: isian formulir Pembentukan Tim.
type DataTim struct {
	JabatanPimpinan string    `json:"jabatan_pimpinan"`
	JenisTim        string    `json:"jenis_tim"`
	MasaAwal        string    `json:"masa_awal"`
	MasaAkhir       string    `json:"masa_akhir"`
	Kota            string    `json:"kota"`
	Anggota         []Anggota `json:"anggota"`
}

func (d *DataTim) Rapikan() {
	for _, s := range []*string{&d.JabatanPimpinan, &d.JenisTim, &d.MasaAwal, &d.MasaAkhir, &d.Kota} {
		rapikan(s)
	}
	for i := range d.Anggota {
		rapikan(&d.Anggota[i].Nama)
		rapikan(&d.Anggota[i].Jabatan)
		rapikan(&d.Anggota[i].Kedudukan)
		d.Anggota[i].NIP = strings.Join(strings.Fields(d.Anggota[i].NIP), "")
	}
}

func (d DataTim) Validasi() []string {
	var g galat
	g.wajib("Jabatan pimpinan", d.JabatanPimpinan, 200)
	if !dalam(d.JenisTim, JenisTimValid) {
		g.add("Jenis tim tidak valid")
	}
	awal := g.tanggal("Masa awal tugas", d.MasaAwal, true)
	akhir := g.tanggal("Masa akhir tugas", d.MasaAkhir, true)
	if !awal.IsZero() && !akhir.IsZero() && akhir.Before(awal) {
		g.add("Masa akhir tugas tidak boleh sebelum masa awal tugas")
	}
	g.wajib("Kota", d.Kota, 100)
	if len(d.Anggota) == 0 {
		g.add("Minimal satu anggota tim")
	}
	if len(d.Anggota) > 60 {
		g.add("Anggota tim terlalu banyak (maksimal 60)")
	}
	for i, a := range d.Anggota {
		g.wajib(fmt.Sprintf("Nama anggota %d", i+1), a.Nama, 150)
		g.wajib(fmt.Sprintf("Jabatan anggota %d", i+1), a.Jabatan, 200)
		g.wajib(fmt.Sprintf("Kedudukan anggota %d", i+1), a.Kedudukan, 100)
		if a.NIP != "" && (len(a.NIP) < 8 || len(a.NIP) > 18 || strings.Trim(a.NIP, "0123456789") != "") {
			g.add("NIP anggota %d harus 8 sampai 18 digit angka", i+1)
		}
	}
	return g
}

// ---------------------------------------------------------------- Berita Acara Penelitian

var BentukValid = []string{"Penjualan", "Tukar Menukar", "Hibah", "Penyertaan Modal Pemerintah Pusat"}

type DataBA struct {
	Bentuk            string `json:"bentuk"`
	TanggalPenelitian string `json:"tanggal_penelitian"`
	NamaTim           string `json:"nama_tim"`
}

func (d *DataBA) Rapikan() {
	rapikan(&d.Bentuk)
	rapikan(&d.TanggalPenelitian)
	rapikan(&d.NamaTim)
}

func (d DataBA) Validasi() []string {
	var g galat
	if !dalam(d.Bentuk, BentukValid) {
		g.add("Bentuk pemindahtanganan tidak valid")
	}
	g.tanggal("Tanggal penelitian", d.TanggalPenelitian, true)
	g.wajib("Nama tim", d.NamaTim, 200)
	return g
}

// ---------------------------------------------------------------- Nota Dinas Usulan Satker

// ItemDokumen: dokumen pendukung pada checklist kelengkapan. Otomatis = disusun aplikasi dalam berkas Nota Dinas
// sehingga bawaannya "ada".
type ItemDokumen struct {
	Kunci    string `json:"kunci"`
	Label    string `json:"label"`
	Otomatis bool   `json:"otomatis"`
}

const (
	DokDaftarBarang     = "daftar_barang"
	DokSKTimInternal    = "sk_tim"
	DokBeritaAcara      = "ba"
	DokPSP              = "psp"
	DokRP4              = "rp4"
	DokSKPP             = "skpp"
	DokDaftarDihentikan = "daftar_dihentikan"
	DokSPTJFormil       = "sptj_formil"
	DokKIB              = "kib"
	DokLaporanKondisi   = "laporan_kondisi"
	DokBuktiMilik       = "bukti_kepemilikan"
	DokFoto             = "foto"
	DokSPTJLimit        = "sptj_limit"
	DokSPerTusi         = "sper_tusi"
	DokSPerAnggaran     = "sper_anggaran"
	DokLaporanPenilaian = "laporan_penilaian"
	DokDinasTeknis      = "dinas_teknis"
)

// DaftarItemDokumen: dokumen pendukung menurut urutan pada alur kerja (16 dokumen) ditambah keterangan dinas teknis,
// yang ada pada tabel checklist template.
var DaftarItemDokumen = []ItemDokumen{
	{DokDaftarBarang, "Daftar barang objek Penjualan BMN", true},
	{DokSKTimInternal, "Fotokopi keputusan pembentukan Tim Internal Penjualan BMN", false},
	{DokBeritaAcara, "Berita Acara Penelitian BMN yang diusulkan untuk dihapus / Berita Acara Survei Lapangan Tim Internal Penjualan BMN", false},
	{DokPSP, "Fotokopi Keputusan Penetapan Status Penggunaan BMN", false},
	{DokRP4, "Dokumen penetapan RP4 BMN disertai lampiran rencana Pemindahtanganan BMN", false},
	{DokSKPP, "Fotokopi Surat Keterangan Penghentian Penggunaan BMN", true},
	{DokDaftarDihentikan, "Print out Daftar BMN yang dihentikan penggunaannya", false},
	{DokSPTJFormil, "Surat Pernyataan kebenaran formil dan materiil objek dan besaran nilai yang diusulkan", true},
	{DokKIB, "Kartu Identitas Barang (KIB)", false},
	{DokLaporanKondisi, "Laporan Kondisi Barang dan/atau listing history", false},
	{DokBuktiMilik, "Fotokopi dokumen/bukti kepemilikan atau BAST perolehan barang dan/atau dokumen lain yang disetarakan", false},
	{DokFoto, "Foto terkini BMN", false},
	{DokSPTJLimit, "Surat Pernyataan Tanggung Jawab Nilai Limit bermaterai", true},
	{DokSPerTusi, "Surat Pernyataan bahwa penghapusan BMN tidak mengganggu pelaksanaan tugas dan fungsi", true},
	{DokSPerAnggaran, "Surat Pernyataan bahwa penghapusan BMN tidak menjadi dasar pengajuan anggaran", true},
	{DokLaporanPenilaian, "Laporan Penilaian/Kertas Kerja Analisis Penentuan Nilai Taksiran BMN (untuk kendaraan)", false},
	{DokDinasTeknis, "Keterangan penelitian/pemeriksaan teknis dari dinas terkait (Dinas Perhubungan untuk kendaraan bermotor, Dinas Pekerjaan Umum untuk tanah dan/atau bangunan)", false},
}

func ItemDokumenByKunci(kunci string) (ItemDokumen, bool) {
	for _, it := range DaftarItemDokumen {
		if it.Kunci == kunci {
			return it, true
		}
	}
	return ItemDokumen{}, false
}

type Barang struct {
	Nama           string `json:"nama"`
	Kode           string `json:"kode"`
	NUP            string `json:"nup"`
	Lokasi         string `json:"lokasi"` // lokasi/merk/tipe/identitas
	Kondisi        string `json:"kondisi"`
	TahunPerolehan string `json:"tahun_perolehan"`
	NilaiPerolehan string `json:"nilai_perolehan"`
	NilaiLimit     string `json:"nilai_limit"`
	Keterangan     string `json:"keterangan"`
}

type Penandatangan struct {
	Nama    string `json:"nama"`
	NIP     string `json:"nip"`
	Jabatan string `json:"jabatan"`
}

// DokPendukung: hasil checklist satu dokumen pendukung.
type DokPendukung struct {
	Ada     bool   `json:"ada"`
	Nomor   string `json:"nomor"`
	Tanggal string `json:"tanggal"`
}

// DataNDSatker: isian formulir Nota Dinas Usulan Penjualan Satker. Jumlah dan total nilai tidak diisi: dihitung dari
// daftar barang supaya tidak pernah berbeda dengan tabel lampiran.
type DataNDSatker struct {
	SudahRP4        bool                    `json:"sudah_rp4"`
	TujuanSurat     string                  `json:"tujuan_surat"` // Sekretaris UE1
	Kota            string                  `json:"kota"`
	SingkatanSatker string                  `json:"singkatan_satker"`
	JenisBMN        string                  `json:"jenis_bmn"`
	Satuan          string                  `json:"satuan"`
	Alasan          string                  `json:"alasan"`
	TiketSiman      string                  `json:"tiket_siman"`
	KepalaKanwil    string                  `json:"kepala_kanwil"`
	Penandatangan   Penandatangan           `json:"penandatangan"`
	Barang          []Barang                `json:"barang"`
	Dokumen         map[string]DokPendukung `json:"dokumen"`
}

func (d *DataNDSatker) Rapikan() {
	for _, s := range []*string{&d.TujuanSurat, &d.Kota, &d.SingkatanSatker, &d.JenisBMN, &d.Satuan, &d.Alasan, &d.TiketSiman, &d.KepalaKanwil,
		&d.Penandatangan.Nama, &d.Penandatangan.NIP, &d.Penandatangan.Jabatan} {
		rapikan(s)
	}
	d.Penandatangan.NIP = strings.Join(strings.Fields(d.Penandatangan.NIP), "")
	for i := range d.Barang {
		b := &d.Barang[i]
		for _, s := range []*string{&b.Nama, &b.Kode, &b.NUP, &b.Lokasi, &b.Kondisi, &b.TahunPerolehan, &b.NilaiPerolehan, &b.NilaiLimit, &b.Keterangan} {
			rapikan(s)
		}
	}
	for k, v := range d.Dokumen {
		v.Nomor, v.Tanggal = strings.TrimSpace(v.Nomor), strings.TrimSpace(v.Tanggal)
		d.Dokumen[k] = v
	}
}

// Dok mengembalikan hasil checklist satu dokumen; yang belum diisi mengikuti bawaannya (dokumen otomatis = ada).
func (d DataNDSatker) Dok(kunci string) DokPendukung {
	if v, ok := d.Dokumen[kunci]; ok {
		return v
	}
	it, _ := ItemDokumenByKunci(kunci)
	return DokPendukung{Ada: it.Otomatis}
}

// Total menjumlahkan daftar barang: banyak barang, total nilai perolehan, dan total nilai limit (dalam sen).
func (d DataNDSatker) Total() (jumlah int, perolehan, limit int64, err error) {
	for i, b := range d.Barang {
		p, e := ParseUang(b.NilaiPerolehan)
		if e != nil {
			return 0, 0, 0, fmt.Errorf("nilai perolehan barang %d: %w", i+1, e)
		}
		l, e := ParseUang(b.NilaiLimit)
		if e != nil {
			return 0, 0, 0, fmt.Errorf("nilai limit barang %d: %w", i+1, e)
		}
		perolehan += p
		limit += l
	}
	if perolehan > MaxSen || limit > MaxSen {
		return 0, 0, 0, fmt.Errorf("total nilai terlalu besar")
	}
	return len(d.Barang), perolehan, limit, nil
}

// validasiBarang memeriksa satu baris daftar barang. sufiks menempel pada nama bidang pada pesan: " 3" di formulir
// ("Nama barang 3 wajib diisi"), atau " (baris 5)" pada impor Excel.
func validasiBarang(sufiks string, b Barang) []string {
	var g galat
	g.wajib("Nama barang"+sufiks, b.Nama, 300)
	g.opsional("Kode barang"+sufiks, b.Kode, 30)
	g.opsional("NUP barang"+sufiks, b.NUP, 30)
	g.opsional("Lokasi/merk/tipe barang"+sufiks, b.Lokasi, 500)
	g.opsional("Kondisi barang"+sufiks, b.Kondisi, 50)
	g.opsional("Keterangan barang"+sufiks, b.Keterangan, 300)
	if b.TahunPerolehan != "" {
		y := 0
		if len(b.TahunPerolehan) == 4 {
			fmt.Sscanf(b.TahunPerolehan, "%d", &y)
		}
		if y < 1900 || y > time.Now().Year()+1 {
			g.add("Tahun perolehan barang%s tidak valid", sufiks)
		}
	}
	if _, e := ParseUang(b.NilaiPerolehan); e != nil {
		g.add("Nilai perolehan barang%s: %s", sufiks, e.Error())
	}
	if l, e := ParseUang(b.NilaiLimit); e != nil {
		g.add("Nilai limit barang%s: %s", sufiks, e.Error())
	} else if l == 0 {
		g.add("Nilai limit barang%s harus lebih dari nol", sufiks)
	}
	return g
}
func (d DataNDSatker) Validasi() []string {
	var g galat
	if !d.SudahRP4 {
		g.add("BMN harus sudah diusulkan di RP4 sebelum Nota Dinas dibuat")
	}
	g.wajib("Tujuan surat (Sekretaris UE1)", d.TujuanSurat, 300)
	g.wajib("Kota/kabupaten lokasi satker", d.Kota, 100)
	g.opsional("Singkatan satker", d.SingkatanSatker, 100)
	g.wajib("Jenis BMN", d.JenisBMN, 200)
	g.wajib("Satuan jumlah BMN", d.Satuan, 30)
	g.wajib("Alasan/pertimbangan penjualan", d.Alasan, 2000)
	g.wajib("Nomor tiket SIMAN", d.TiketSiman, 100)
	g.wajib("Tembusan Kepala Kantor Wilayah", d.KepalaKanwil, 300)
	g.wajib("Nama pejabat penandatangan", d.Penandatangan.Nama, 150)
	g.wajib("Jabatan pejabat penandatangan", d.Penandatangan.Jabatan, 300)
	if nip := d.Penandatangan.NIP; len(nip) != 18 || strings.Trim(nip, "0123456789") != "" {
		g.add("NIP pejabat penandatangan harus 18 digit angka")
	}

	if len(d.Barang) == 0 {
		g.add("Daftar barang minimal berisi satu barang")
	}
	if len(d.Barang) > 500 {
		g.add("Daftar barang terlalu banyak (maksimal 500)")
	}
	for i, b := range d.Barang {
		g = append(g, validasiBarang(fmt.Sprintf(" %d", i+1), b)...)
	}
	for k, v := range d.Dokumen {
		it, ok := ItemDokumenByKunci(k)
		if !ok {
			g.add("Dokumen pendukung \"%s\" tidak dikenal", k)
			continue
		}
		g.opsional("Nomor "+it.Kunci, v.Nomor, 150)
		g.tanggal("Tanggal "+it.Kunci, v.Tanggal, false)
	}
	if ba := d.Dok(DokBeritaAcara); ba.Ada && (ba.Nomor == "" || ba.Tanggal == "") {
		g.add("Isi nomor dan tanggal Berita Acara Penelitian")
	}
	return g
}

// ---------------------------------------------------------------- Nota Dinas Usulan UE1

type DataNDUE1 struct {
	NomorND          string        `json:"nomor_nd"`   // Nota Dinas usulan Satker
	TanggalND        string        `json:"tanggal_nd"` // tanggal Nota Dinas usulan Satker
	HalND            string        `json:"hal_nd"`
	SekretarisUE1    string        `json:"sekretaris_ue1"`
	KepalaKanwil     string        `json:"kepala_kanwil"`
	PejabatPengelola string        `json:"pejabat_pengelola"`
	Penandatangan    Penandatangan `json:"penandatangan"` // NIP tidak dipakai pada template UE1
}

func (d *DataNDUE1) Rapikan() {
	for _, s := range []*string{&d.NomorND, &d.TanggalND, &d.HalND, &d.SekretarisUE1, &d.KepalaKanwil, &d.PejabatPengelola, &d.Penandatangan.Nama, &d.Penandatangan.Jabatan} {
		rapikan(s)
	}
}

func (d DataNDUE1) Validasi() []string {
	var g galat
	g.wajib("Nomor Nota Dinas usulan Satker", d.NomorND, 150)
	g.tanggal("Tanggal Nota Dinas usulan Satker", d.TanggalND, true)
	g.wajib("Hal Nota Dinas usulan Satker", d.HalND, 500)
	g.wajib("Sekretaris UE1", d.SekretarisUE1, 300)
	g.wajib("Tembusan Kepala Kantor Wilayah", d.KepalaKanwil, 300)
	g.wajib("Tembusan pejabat pengelola", d.PejabatPengelola, 300)
	g.wajib("Nama pejabat UE1 penandatangan", d.Penandatangan.Nama, 150)
	g.wajib("Jabatan pejabat UE1 penandatangan", d.Penandatangan.Jabatan, 300)
	return g
}
