package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"

	"pasti-v3-backend/config"
)

// Katalog dataset Pengadaan, Tender, E-Katalog V5, dan E-Katalog V6 untuk halaman Penarikan Data terpadu, penarikan otomatis, ekspor,
// dan dasbor. Satu entri = satu endpoint Inaproc beserta tabel lokalnya.
//
// Pelaksana penarikan ada dua macam:
//   - generik: endpointDatar (inaproc_tender_datar.go); penggantian datanya atomik dan dapat dibatalkan.
//   - tulisan tangan: 13 handler sinkronisasi lama (RUP dan non-tender) yang dipanggil di dalam proses lewat konteks gin palsu.
//     Handler itu menghapus data lama lebih dulu, jadi sebelum dijalankan dilakukan uji sambungan (satu permintaan kecil ke Inaproc)
//     agar penarikan tidak menghapus data bila Inaproc sedang menolak (token, batas laju). Pembatalan baru berlaku antar tugas.

const (
	KelompokPengadaan  = "pengadaan"
	KelompokTender     = "tender"
	KelompokEkatalogV5 = "ekatalog-v5"
	KelompokEkatalogV6 = "ekatalog-v6"
)

// ModeTarik: apa yang dibutuhkan satu penarikan.
type ModeTarik string

const (
	ModeKLPDTahun ModeTarik = "klpd_tahun" // kode KLPD + tahun
	ModeKLPD      ModeTarik = "klpd"       // kode KLPD saja
	ModeKode      ModeTarik = "kode"       // satu kode rujukan (penyedia, komoditas, ...); tidak ada daftar "semua" di Inaproc
	ModeKategori  ModeTarik = "kategori"   // satu tingkat kategori produk (kosong = tingkat 1)
	ModeTransaksi ModeTarik = "transaksi"  // tahun + status + KLPD/kategori/produk
)

// PermintaanTarik: isian satu penarikan; yang dipakai bergantung pada mode datasetnya.
type PermintaanTarik struct {
	KodeKLPD   string `json:"kode_klpd,omitempty"`
	Tahun      string `json:"tahun,omitempty"`
	Kode       string `json:"kode,omitempty"`
	Status     string `json:"status,omitempty"`      // paket-penyedia/paket-swakelola (RUP) dan transaksi E-Katalog V6
	JenisPaket string `json:"jenis_paket,omitempty"` // history-kaji-ulang
	Kd1        string `json:"kd_kategori_1,omitempty"`
	Kd2        string `json:"kd_kategori_2,omitempty"`
	KdProduk   string `json:"kd_product,omitempty"`
}

var reTahunPenarikan = regexp.MustCompile(`^(19|20)[0-9]{2}$`)

// DatasetPenarikan: satu dataset yang bisa ditarik.
type DatasetPenarikan struct {
	ID          string // jalur API setelah /api/v1/, mis. "tender/pengumuman"; unik
	Kelompok    string
	Subkelompok string
	Nama        string
	Deskripsi   string
	Tabel       string
	Mode        ModeTarik
	// Otomatis: ikut penarikan otomatis secara bawaan. Dataset per kode (rujukan) tidak: tidak ada daftar "semua" di Inaproc.
	Otomatis bool

	// Kolom penyaring tabel untuk ekspor dan tampilan data; kosong = tidak ada kolom itu.
	KolomKLPD, KolomTahun string
	// KolomRingkas: kolom yang masuk tampilan data dan ekspor PDF (Excel dan CSV memuat semua kolom).
	KolomRingkas []string
	// KolomKunci: kolom unik per baris, untuk urutan yang stabil saat membaca halaman demi halaman.
	KolomKunci string

	generik *endpointDatar
	// Dataset tulisan tangan: handler sinkronisasi lama dan penyusun badan JSON-nya.
	tulisan gin.HandlerFunc
}

// NamaLog: nama yang dicatat di inaproc_sync_log oleh penarikan dataset ini.
func (d *DatasetPenarikan) NamaLog() string {
	if d.generik != nil {
		return d.generik.namaLog()
	}
	return d.ID[strings.LastIndex(d.ID, "/")+1:]
}

// Normalisasi mengisi bawaan menurut mode dan memangkas spasi; hasilnya dipakai sebagai kunci jatuh tempo dan ringkasan riwayat.
func (d *DatasetPenarikan) Normalisasi(p PermintaanTarik) PermintaanTarik {
	p.KodeKLPD, p.Tahun, p.Kode = strings.TrimSpace(p.KodeKLPD), strings.TrimSpace(p.Tahun), strings.TrimSpace(p.Kode)
	p.Status, p.JenisPaket = strings.TrimSpace(p.Status), strings.TrimSpace(p.JenisPaket)
	p.Kd1, p.Kd2, p.KdProduk = strings.TrimSpace(p.Kd1), strings.TrimSpace(p.Kd2), strings.TrimSpace(p.KdProduk)

	switch d.Mode {
	case ModeKLPDTahun, ModeKLPD:
		if p.KodeKLPD == "" {
			p.KodeKLPD = kemenkeuKLPDCode
		}
		if d.Mode == ModeKLPD {
			p.Tahun = ""
		}
		p.Kode, p.Kd1, p.Kd2, p.KdProduk = "", "", "", ""
		if d.tulisan == nil || (d.ID != "rup/paket-penyedia" && d.ID != "rup/paket-swakelola") {
			p.Status = ""
		}
		if d.ID != "rup/history-kaji-ulang" {
			p.JenisPaket = ""
		}
	case ModeKode:
		p.KodeKLPD, p.Tahun, p.Status, p.JenisPaket, p.Kd1, p.Kd2, p.KdProduk = "", "", "", "", "", "", ""
	case ModeKategori:
		p.KodeKLPD, p.Tahun, p.Kode, p.Status, p.JenisPaket, p.KdProduk = "", "", "", "", "", ""
		if p.Kd1 == "" {
			p.Kd2 = ""
		}
	case ModeTransaksi:
		t := transaksiSaring{Tahun: p.Tahun, KodeKLPD: p.KodeKLPD, Kd1: p.Kd1, KdProduk: p.KdProduk, Status: p.Status}
		t.rapikan()
		p = PermintaanTarik{Tahun: t.Tahun, KodeKLPD: t.KodeKLPD, Kd1: t.Kd1, KdProduk: t.KdProduk, Status: t.Status}
	}
	return p
}

// Validasi: pesan tidak kosong = permintaan ditolak. Dipanggil pada p yang sudah dinormalisasi.
func (d *DatasetPenarikan) Validasi(p PermintaanTarik) string {
	if !tidakPanjang(p.KodeKLPD, p.Kode, p.Status, p.JenisPaket, p.Kd1, p.Kd2, p.KdProduk) {
		return "Isian terlalu panjang"
	}
	switch d.Mode {
	case ModeKLPDTahun:
		if !reTahunPenarikan.MatchString(p.Tahun) {
			return "Tahun harus berupa empat digit (mis. 2025)"
		}
	case ModeKode:
		if p.Kode == "" {
			return "Kode wajib diisi"
		}
	case ModeKategori, ModeTransaksi:
		if _, pesan := d.rencanaGenerik(p); pesan != "" {
			return pesan
		}
	}
	return ""
}

// Ringkas: ringkasan isian untuk riwayat dan kunci jatuh tempo (p dinormalisasi).
func (d *DatasetPenarikan) Ringkas(p PermintaanTarik) string {
	var bagian []string
	tambah := func(s string) {
		if s != "" {
			bagian = append(bagian, s)
		}
	}
	switch d.Mode {
	case ModeKLPDTahun:
		tambah(p.KodeKLPD)
		tambah(p.Tahun)
		tambah(p.Status)
		tambah(p.JenisPaket)
	case ModeKLPD:
		tambah(p.KodeKLPD)
	case ModeKode:
		tambah(p.Kode)
	case ModeKategori:
		switch {
		case p.Kd2 != "":
			tambah("L3:" + p.Kd1 + "/" + p.Kd2)
		case p.Kd1 != "":
			tambah("L2:" + p.Kd1)
		default:
			tambah("L1")
		}
	case ModeTransaksi:
		tambah(p.Tahun)
		tambah(p.Status)
		tambah(p.KodeKLPD)
		tambah(p.Kd1)
		tambah(p.KdProduk)
	}
	return strings.Join(bagian, "/")
}

// rencanaGenerik menyusun rencana sinkronisasi dataset generik.
func (d *DatasetPenarikan) rencanaGenerik(p PermintaanTarik) (*rencanaSync, string) {
	switch d.Mode {
	case ModeKategori:
		return ekatalog6KategoriRencanaDari(p.Kd1, p.Kd2)
	case ModeTransaksi:
		return ekatalog6TransaksiRencanaDari(transaksiSaring{Tahun: p.Tahun, KodeKLPD: p.KodeKLPD, Kd1: p.Kd1, KdProduk: p.KdProduk, Status: p.Status})
	}
	return d.generik.rencanaDari(syncDatarRequest{KodeKLPD: p.KodeKLPD, Tahun: p.Tahun, Kode: p.Kode})
}

// Jalankan menarik dataset ini dari Inaproc ke tabel lokal. p harus sudah dinormalisasi dan lolos Validasi. kabar (opsional) menerima
// pesan kemajuan. Galat selalu bertipe *GalatSinkron.
func (d *DatasetPenarikan) Jalankan(ctx context.Context, p PermintaanTarik, oleh string, kabar func(string)) (HasilSinkron, error) {
	if config.Cfg.InaprocToken == "" {
		return HasilSinkron{}, &GalatSinkron{Status: http.StatusServiceUnavailable, Pesan: "Integrasi Inaproc belum dikonfigurasi (token kosong)"}
	}
	if d.generik != nil {
		rencana, pesan := d.rencanaGenerik(p)
		if pesan != "" {
			return HasilSinkron{}, &GalatSinkron{Status: http.StatusBadRequest, Pesan: pesan}
		}
		rencana.Kemajuan = kabar
		return d.generik.Jalankan(ctx, rencana, oleh)
	}
	return d.jalankanTulisan(ctx, p, oleh, kabar)
}

// jalankanTulisan menjalankan handler sinkronisasi lama di dalam proses (konteks gin palsu), didahului uji sambungan.
func (d *DatasetPenarikan) jalankanTulisan(ctx context.Context, p PermintaanTarik, oleh string, kabar func(string)) (HasilSinkron, error) {
	if kabar != nil {
		kabar("Memeriksa sambungan ke Inaproc")
	}
	uji := url.Values{}
	uji.Set("kode_klpd", p.KodeKLPD)
	uji.Set("tahun", p.Tahun)
	uji.Set("limit", "1")
	if p.Status != "" {
		uji.Set("status", p.Status)
	}
	if p.JenisPaket != "" {
		uji.Set("jenis_paket", p.JenisPaket)
	}
	body, status, err := callInaprocEndpointCtx(ctx, "/api/v1/"+d.ID, uji)
	if err != nil {
		if ctx.Err() != nil {
			return HasilSinkron{}, &GalatSinkron{Status: statusDibatalkan, Pesan: "Sinkronisasi dibatalkan"}
		}
		return HasilSinkron{}, &GalatSinkron{Status: http.StatusBadGateway, Pesan: "Gagal menghubungi API Inaproc (timeout/jaringan)"}
	}
	if status != http.StatusOK {
		return HasilSinkron{}, &GalatSinkron{Status: status, Pesan: "Sinkronisasi dibatalkan sebelum data lama disentuh: " + extractInaprocErrorMessage(body, status), Hulu: true}
	}
	if ctx.Err() != nil {
		return HasilSinkron{}, &GalatSinkron{Status: statusDibatalkan, Pesan: "Sinkronisasi dibatalkan"}
	}

	if kabar != nil {
		kabar("Menarik dari Inaproc dan menyimpan (pembatalan baru berlaku setelah dataset ini selesai)")
	}
	badan := map[string]string{"kode_klpd": p.KodeKLPD, "tahun": p.Tahun}
	if p.Status != "" {
		badan["status"] = p.Status
	}
	if p.JenisPaket != "" {
		badan["jenis_paket"] = p.JenisPaket
	}
	kode, resp := panggilHandlerGin(d.tulisan, badan, oleh)

	var amplop struct {
		Message string `json:"message"`
		Data    struct {
			TotalSynced int `json:"total_synced"`
			TotalFailed int `json:"total_failed"`
			Halaman     int `json:"pages_fetched"`
		} `json:"data"`
	}
	_ = json.Unmarshal(resp, &amplop)
	if kode < 200 || kode > 299 {
		pesan := amplop.Message
		if pesan == "" {
			pesan = "Sinkronisasi gagal"
		}
		return HasilSinkron{}, &GalatSinkron{Status: kode, Pesan: pesan, Hulu: kode == http.StatusTooManyRequests}
	}
	return HasilSinkron{TotalSinkron: amplop.Data.TotalSynced, TotalGagal: amplop.Data.TotalFailed, Halaman: amplop.Data.Halaman}, nil
}

// mesinHandlerLama: satu mesin gin bersama untuk konteks buatan. gin.CreateTestContext membuat mesin baru tiap dipanggil, dan di mode debug tiap
// mesin baru mencetak peringatan "Running in debug mode" ke log, sehingga log penuh pengulangan di tiap tugas dataset lama.
var mesinHandlerLama = sync.OnceValue(func() *gin.Engine { return gin.New() })

// panggilHandlerGin memanggil handler gin di dalam proses dengan badan JSON dan pengguna pemicu (kosong = penarikan otomatis).
func panggilHandlerGin(h gin.HandlerFunc, badan interface{}, oleh string) (int, []byte) {
	rec := httptest.NewRecorder()
	c := gin.CreateTestContextOnly(rec, mesinHandlerLama())
	b, _ := json.Marshal(badan)
	c.Request = httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(b))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("user_id", oleh)
	h(c)
	return rec.Code, rec.Body.Bytes()
}

// ============================================================
// Katalog
// ============================================================

// DaftarDataset: semua dataset menurut urutan tampil (kelompok, lalu urutan di sini).
var DaftarDataset []*DatasetPenarikan

var indeksDataset = map[string]*DatasetPenarikan{}

// DatasetByID mengembalikan dataset menurut ID-nya.
func DatasetByID(id string) (*DatasetPenarikan, bool) {
	d, ok := indeksDataset[strings.TrimSpace(id)]
	return d, ok
}

func daftarkan(d *DatasetPenarikan) {
	if _, ada := indeksDataset[d.ID]; ada {
		panic("dataset ganda di katalog penarikan: " + d.ID)
	}
	DaftarDataset = append(DaftarDataset, d)
	indeksDataset[d.ID] = d
}

// kolomRingkasDari memecah daftar kolom SELECT (teks beberapa baris) menjadi kolom ringkasan PDF: tanpa kunci baris dan penanda waktu.
func kolomRingkasDari(daftar string) []string {
	var out []string
	for _, k := range strings.Split(daftar, ",") {
		k = strings.TrimSpace(k)
		switch k {
		case "", "row_key", "datamart_id", "synced_at":
			continue
		}
		out = append(out, k)
	}
	return out
}

// generikDataset: dataset dari endpointDatar. Kolom penyaring ekspor menurut bentuk saringannya.
func generikDataset(e *endpointDatar, kel, sub, nama, deskripsi string, mode ModeTarik) *DatasetPenarikan {
	d := &DatasetPenarikan{
		ID: e.awalan() + "/" + e.Nama, Kelompok: kel, Subkelompok: sub, Nama: nama, Deskripsi: deskripsi, Tabel: e.Tabel, Mode: mode,
		Otomatis: mode != ModeKode, generik: e, KolomRingkas: kolomRingkasDari(e.KolomDaftar), KolomKunci: "row_key",
	}
	switch {
	case mode == ModeKLPDTahun:
		d.KolomKLPD, d.KolomTahun = e.kolomKLPD(), e.kolomTahun()
	case mode == ModeKLPD:
		d.KolomKLPD = e.kolomKLPD()
	case mode == ModeTransaksi:
		d.KolomKLPD, d.KolomTahun = "kode_klpd", "tahun"
	}
	return d
}

// tulisanDataset: dataset dari handler sinkronisasi lama (RUP dan non-tender); semuanya per KLPD + tahun.
func tulisanDataset(id, kel, sub, nama, deskripsi, tabel string, h gin.HandlerFunc, ringkas string) *DatasetPenarikan {
	kunci := "row_key"
	if id == "rup/history-kaji-ulang" {
		kunci = "datamart_id" // satu-satunya tabel lama dengan kunci utama datamart_id
	}
	return &DatasetPenarikan{
		KolomKunci: kunci, ID: id, Kelompok: kel, Subkelompok: sub, Nama: nama, Deskripsi: deskripsi, Tabel: tabel, Mode: ModeKLPDTahun, Otomatis: true,
		KolomKLPD: "kd_klpd", KolomTahun: "tahun_anggaran", KolomRingkas: kolomRingkasDari(ringkas), tulisan: h,
	}
}

func init() {
	// ---- Pengadaan (RUP) ----
	for _, d := range []*DatasetPenarikan{
		tulisanDataset("rup/paket-penyedia", KelompokPengadaan, "Paket penyedia", "Paket Penyedia",
			"Rencana paket pengadaan melalui penyedia: pagu, metode, jenis, jadwal pemilihan, dan PPK.",
			"inaproc_paket_penyedia", SyncPaketPenyedia,
			`kd_rup, nama_paket, nama_satker, pagu, metode_pengadaan, jenis_pengadaan, status_umumkan_rup, nama_ppk, tgl_awal_pemilihan, tgl_akhir_pemilihan, tahun_anggaran`),
		tulisanDataset("rup/paket-penyedia-terumumkan", KelompokPengadaan, "Paket penyedia", "Paket Penyedia Terumumkan",
			"Paket penyedia yang sudah diumumkan di SiRUP.",
			"inaproc_paket_penyedia_terumumkan", SyncPaketPenyediaTerumumkan,
			`kd_rup, nama_paket, nama_satker, pagu, metode_pengadaan, status_umumkan_rup, nama_ppk, tgl_awal_pemilihan, tgl_akhir_pemilihan, tahun_anggaran`),
		tulisanDataset("rup/paket-anggaran-penyedia", KelompokPengadaan, "Paket penyedia", "Anggaran Paket Penyedia",
			"Rincian sumber dana dan anggaran (MAK) per paket penyedia.",
			"inaproc_paket_anggaran_penyedia", SyncPaketAnggaranPenyedia,
			`kd_rup, nama_satker, mak, pagu, sumber_dana, asal_dana, status_umumkan_rup, tahun_anggaran`),
		tulisanDataset("rup/paket-swakelola", KelompokPengadaan, "Paket swakelola", "Paket Swakelola",
			"Rencana paket yang dikerjakan secara swakelola.",
			"inaproc_paket_swakelola", SyncPaketSwakelola,
			`kd_rup, nama_paket, nama_satker, status, tahun_anggaran`),
		tulisanDataset("rup/paket-swakelola-terumumkan", KelompokPengadaan, "Paket swakelola", "Paket Swakelola Terumumkan",
			"Paket swakelola yang sudah diumumkan di SiRUP.",
			"inaproc_paket_swakelola_terumumkan", SyncPaketSwakelolaTerumumkan,
			`kd_rup, nama_paket, nama_satker, pagu, nama_ppk, status_umumkan_rup, tgl_awal_pelaksanaan_kontrak, tgl_akhir_pelaksanaan_kontrak, tahun_anggaran`),
		tulisanDataset("rup/paket-anggaran-swakelola", KelompokPengadaan, "Paket swakelola", "Anggaran Paket Swakelola",
			"Rincian sumber dana dan anggaran (MAK) per paket swakelola.",
			"inaproc_paket_anggaran_swakelola", SyncPaketAnggaranSwakelola,
			`kd_rup, nama_satker, mak, pagu, sumber_dana, asal_dana, status_umumkan_rup, tahun_anggaran`),
		tulisanDataset("rup/program-master", KelompokPengadaan, "Master dan riwayat", "Program Master",
			"Daftar program beserta pagunya.",
			"inaproc_program_master", SyncProgramMaster,
			`kd_program, nama_program, kd_satker, pagu_program, is_deleted, tahun_anggaran`),
		tulisanDataset("rup/history-kaji-ulang", KelompokPengadaan, "Master dan riwayat", "Riwayat Kaji Ulang",
			"Perubahan (kaji ulang) paket RUP: jenis revisi, alasan, dan tanggal.",
			"inaproc_history_kaji_ulang", SyncHistoryKajiUlang,
			`nama_satker, kd_rup_lama, kd_rup_baru, jenis_paket, jenis_revisi, alasan_kajiulang, tgl_kaji_ulang, tahun_anggaran`),
	} {
		daftarkan(d)
	}

	// ---- Tender ----
	for _, d := range []*DatasetPenarikan{
		generikDataset(tenderPengumuman, KelompokTender, "Tender", "Pengumuman Tender", "Pengumuman tender: pagu, HPS, metode, kualifikasi, dan status.", ModeKLPDTahun),
		generikDataset(tenderPeserta, KelompokTender, "Tender", "Peserta Tender", "Peserta tender beserta penawaran dan statusnya.", ModeKLPDTahun),
		tulisanDataset("tender/jadwal-tahapan-tender", KelompokTender, "Tender", "Jadwal Tahapan Tender", "Jadwal tiap tahapan tender.",
			"inaproc_jadwal_tahapan_tender", SyncJadwalTahapanTender,
			`kd_tender, kd_satker, nama_akt, nama_tahapan, tahun_anggaran, tgl_awal, tgl_akhir`),
		generikDataset(tenderEkontrak, KelompokTender, "Tender", "E-Kontrak Tender", "Riwayat BAP/BAST, SPMK/SPP, dan penilaian kinerja kontrak tender.", ModeKLPDTahun),
		generikDataset(tenderEkontrakKontrak, KelompokTender, "Tender", "Kontrak Tender", "Kontrak hasil tender: nilai, penyedia, dan masa kontrak.", ModeKLPDTahun),
		generikDataset(tenderSelesai, KelompokTender, "Tender", "Tender Selesai", "Tender yang sudah selesai: pemenang dan nilai.", ModeKLPDTahun),
		generikDataset(tenderSelesaiNilai, KelompokTender, "Tender", "Nilai Tender Selesai", "Nilai penawaran, koreksi, dan negosiasi tender selesai.", ModeKLPDTahun),
		tulisanDataset("tender/non-tender-pengumuman", KelompokTender, "Non-tender", "Pengumuman Non-Tender", "Pengumuman pengadaan non-tender: pagu, HPS, metode.",
			"inaproc_non_tender_pengumuman", SyncNonTenderPengumuman,
			`kd_nontender, kd_rup, nama_paket, nama_satker, mtd_pemilihan, pagu, hps, status_nontender, tgl_pengumuman_nontender, tahun_anggaran`),
		generikDataset(nonTenderSelesai, KelompokTender, "Non-tender", "Non-Tender Selesai", "Pengadaan non-tender yang sudah selesai.", ModeKLPDTahun),
		tulisanDataset("tender/non-tender-ekontrak", KelompokTender, "Non-tender", "E-Kontrak Non-Tender", "Riwayat BAP/BAST, SPMK/SPP, dan penilaian kinerja non-tender.",
			"inaproc_non_tender_ekontrak", SyncNonTenderEkontrak,
			`kd_tender, nama_paket, alamat_satker, tahun_anggaran`),
		tulisanDataset("tender/non-tender-ekontrak-kontrak", KelompokTender, "Non-tender", "Kontrak Non-Tender", "Kontrak pengadaan non-tender: nilai, penyedia, status.",
			"inaproc_non_tender_ekontrak_kontrak", SyncNonTenderEkontrakKontrak,
			`kd_nontender, no_kontrak, nama_paket, nama_penyedia, nilai_kontrak, status_kontrak, tgl_kontrak, tahun_anggaran`),
		tulisanDataset("tender/jadwal-tahapan-non-tender", KelompokTender, "Non-tender", "Jadwal Tahapan Non-Tender", "Jadwal tiap tahapan pengadaan non-tender.",
			"inaproc_jadwal_tahapan_non_tender", SyncJadwalTahapanNonTender,
			`kd_nontender, kd_satker, nama_akt, nama_tahapan, tahun_anggaran, tgl_awal, tgl_akhir`),
		generikDataset(pencatatanNonTender, KelompokTender, "Pencatatan", "Pencatatan Non-Tender", "Pencatatan pengadaan non-tender di luar e-tendering.", ModeKLPDTahun),
		generikDataset(pencatatanNonTenderRealisasi, KelompokTender, "Pencatatan", "Realisasi Pencatatan Non-Tender", "Realisasi pencatatan non-tender.", ModeKLPDTahun),
		generikDataset(pencatatanSwakelola, KelompokTender, "Pencatatan", "Pencatatan Swakelola", "Pencatatan paket swakelola.", ModeKLPDTahun),
		generikDataset(pencatatanSwakelolaRealisasi, KelompokTender, "Pencatatan", "Realisasi Pencatatan Swakelola", "Realisasi pencatatan swakelola.", ModeKLPDTahun),
	} {
		daftarkan(d)
	}

	// ---- E-Katalog V5 (archive) ----
	for _, d := range []*DatasetPenarikan{
		generikDataset(ekatalogPaket, KelompokEkatalogV5, "Transaksi", "Paket E-Purchasing V5", "Paket e-purchasing arsip E-Katalog V5: komoditas, penyedia, kuantitas, dan nilai.", ModeKLPDTahun),
		generikDataset(ekatalogInstansiSatker, KelompokEkatalogV5, "Rujukan", "Instansi dan Satker", "Daftar instansi dan satuan kerja pengguna E-Katalog.", ModeKLPD),
		generikDataset(ekatalogKomoditas, KelompokEkatalogV5, "Rujukan", "Komoditas", "Rujukan komoditas (dicari per kode komoditas).", ModeKode),
		generikDataset(ekatalogPenyedia, KelompokEkatalogV5, "Rujukan", "Penyedia", "Rujukan penyedia (dicari per kode penyedia).", ModeKode),
		generikDataset(ekatalogDistributor, KelompokEkatalogV5, "Rujukan", "Distributor", "Rujukan penyedia distributor (dicari per kode distributor).", ModeKode),
	} {
		daftarkan(d)
	}

	// ---- E-Katalog V6 ----
	for _, d := range []*DatasetPenarikan{
		generikDataset(ekatalog6Paket, KelompokEkatalogV6, "Transaksi", "Paket E-Purchasing V6", "Paket e-purchasing E-Katalog V6, termasuk paket swasta.", ModeKLPDTahun),
		generikDataset(ekatalog6Transaksi, KelompokEkatalogV6, "Transaksi", "Transaksi per Produk", "Nilai transaksi e-purchasing per produk, kategori, dan KLPD.", ModeTransaksi),
		generikDataset(ekatalog6Kategori, KelompokEkatalogV6, "Rujukan", "Kategori Produk", "Kategori produk tiga tingkat (tingkat 1 untuk penarikan otomatis).", ModeKategori),
		generikDataset(ekatalog6Penyedia, KelompokEkatalogV6, "Rujukan", "Penyedia V6", "Rujukan penyedia E-Katalog V6 (dicari per kode penyedia).", ModeKode),
		generikDataset(ekatalog6ProdukPenyedia, KelompokEkatalogV6, "Rujukan", "Produk Penyedia", "Daftar produk milik penyedia (dicari per kode penyedia).", ModeKode),
	} {
		daftarkan(d)
	}
}

// NamaKelompok: nama tampil kelompok.
func NamaKelompok(k string) string {
	switch k {
	case KelompokPengadaan:
		return "Pengadaan (RUP)"
	case KelompokTender:
		return "Tender dan Non-Tender"
	case KelompokEkatalogV5:
		return "E-Katalog V5"
	case KelompokEkatalogV6:
		return "E-Katalog V6"
	}
	return k
}

// UrutanKelompok: urutan tampil kelompok.
var UrutanKelompok = []string{KelompokPengadaan, KelompokTender, KelompokEkatalogV5, KelompokEkatalogV6}
