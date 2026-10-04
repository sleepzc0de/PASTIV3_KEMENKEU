package handlers

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"pasti-v3-backend/config"
	"pasti-v3-backend/utils"
)

// Endpoint halaman Penarikan Data terpadu (/inaproc/penarikan/...): status semua dataset, penarikan manual, pembatalan, riwayat, dan
// pengaturan penarikan otomatis. Membaca terbuka bagi semua pengguna login; memulai, membatalkan, dan mengubah pengaturan khusus admin.

const penarikanTimeout = 30 * time.Second

func penarikReady(c *gin.Context) bool {
	if Penarik == nil {
		utils.ErrorResponse(c, http.StatusServiceUnavailable, "Fitur penarikan data belum siap")
		return false
	}
	return true
}

func penarikGagal(c *gin.Context, apa string, err error) {
	log.Println("[INAPROC PENARIKAN ERROR]", apa+":", err)
	utils.ErrorResponse(c, http.StatusInternalServerError, "Gagal membaca data penarikan")
}

// maskRiwayat menyembunyikan isi galat dan nama pemicu dari non-admin: pesan galat bisa memuat alamat atau potongan respons Inaproc.
func maskRiwayat(r RiwayatPenarikan, admin bool) RiwayatPenarikan {
	if !admin {
		if r.Status == PenarikanGagal {
			pesan := "Penarikan gagal"
			r.Pesan = &pesan
		}
		r.DijalankanOleh = nil
	}
	return r
}

func maskInfoAktif(a *InfoAktif, admin bool) *InfoAktif {
	if a == nil || admin {
		return a
	}
	a.Oleh = ""
	for i := range a.Tugas {
		if a.Tugas[i].Status == PenarikanGagal {
			a.Tugas[i].Pesan = "Penarikan gagal"
		}
	}
	return a
}

type infoDataset struct {
	ID             string            `json:"id"`
	Kelompok       string            `json:"kelompok"`
	Subkelompok    string            `json:"subkelompok"`
	Nama           string            `json:"nama"`
	Deskripsi      string            `json:"deskripsi"`
	Mode           ModeTarik         `json:"mode"`
	Otomatis       bool              `json:"otomatis"`
	PunyaKLPD      bool              `json:"punya_klpd"`
	PunyaTahun     bool              `json:"punya_tahun"`
	PerluStatus    bool              `json:"perlu_status"`
	Baris          int64             `json:"baris"`
	DisinkronAt    *time.Time        `json:"disinkron_at"`
	Terakhir       *RiwayatPenarikan `json:"terakhir"`
	TerakhirSukses *RiwayatPenarikan `json:"terakhir_sukses"`
}

type infoKelompok struct {
	ID   string `json:"id"`
	Nama string `json:"nama"`
}

// GetInaprocPenarikan: seluruh keadaan halaman Penarikan Data.
func GetInaprocPenarikan(c *gin.Context) {
	if !penarikReady(c) {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), penarikanTimeout)
	defer cancel()
	admin := dgIsAdmin(c)
	m := Penarik

	latest, lastOK, err := m.TerakhirPerDataset(ctx)
	if err != nil {
		penarikGagal(c, "riwayat terakhir", err)
		return
	}
	riwayat, err := m.Riwayat(ctx, "", 40)
	if err != nil {
		penarikGagal(c, "riwayat", err)
		return
	}
	for i := range riwayat {
		riwayat[i] = maskRiwayat(riwayat[i], admin)
	}
	pengaturan, err := m.MuatPengaturan(ctx)
	if err != nil {
		penarikGagal(c, "pengaturan", err)
		return
	}
	info, err := m.InfoOtomatis(ctx, pengaturan, time.Now())
	if err != nil {
		penarikGagal(c, "info otomatis", err)
		return
	}
	ringkas := m.RingkasTabel(ctx)

	datasets := make([]infoDataset, 0, len(DaftarDataset))
	for _, d := range DaftarDataset {
		x := infoDataset{ID: d.ID, Kelompok: d.Kelompok, Subkelompok: d.Subkelompok, Nama: d.Nama, Deskripsi: d.Deskripsi, Mode: d.Mode, Otomatis: d.Otomatis,
			PunyaKLPD: d.KolomKLPD != "", PunyaTahun: d.KolomTahun != "", PerluStatus: d.ID == "rup/paket-penyedia" || d.ID == "rup/paket-swakelola" || d.Mode == ModeTransaksi}
		if t, ok := ringkas[d.ID]; ok {
			x.Baris, x.DisinkronAt = t.Baris, t.DisinkronAt
		}
		if r, ok := latest[d.ID]; ok {
			r = maskRiwayat(r, admin)
			x.Terakhir = &r
		}
		if r, ok := lastOK[d.ID]; ok {
			r = maskRiwayat(r, admin)
			x.TerakhirSukses = &r
		}
		datasets = append(datasets, x)
	}
	kelompok := make([]infoKelompok, 0, len(UrutanKelompok))
	for _, k := range UrutanKelompok {
		kelompok = append(kelompok, infoKelompok{ID: k, Nama: NamaKelompok(k)})
	}

	utils.SuccessResponse(c, http.StatusOK, "Berhasil mengambil status penarikan data", gin.H{
		"token_ada":    config.Cfg != nil && config.Cfg.InaprocToken != "",
		"aktif":        maskInfoAktif(m.Aktif(), admin),
		"otomatis":     info,
		"pengaturan":   pengaturan,
		"kelompok":     kelompok,
		"datasets":     datasets,
		"riwayat":      riwayat,
		"kode_klpd":    kemenkeuKLPDCode,
		"tahun_ini":    time.Now().In(zonaWIB).Year(),
		"tahun_bawaan": TahunPenarikan(time.Now(), pengaturan.JumlahTahun),
	})
}

// GetInaprocPenarikanAktif: ringan, untuk polling kemajuan saat antrean berjalan.
func GetInaprocPenarikanAktif(c *gin.Context) {
	if !penarikReady(c) {
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "Berhasil mengambil kemajuan penarikan", gin.H{"aktif": maskInfoAktif(Penarik.Aktif(), dgIsAdmin(c))})
}

// GetInaprocPenarikanRiwayat: riwayat penarikan (query: dataset, limit).
func GetInaprocPenarikanRiwayat(c *gin.Context) {
	if !penarikReady(c) {
		return
	}
	dataset := strings.TrimSpace(c.Query("dataset"))
	if dataset != "" {
		if _, ok := DatasetByID(dataset); !ok {
			utils.ErrorResponse(c, http.StatusBadRequest, "Dataset tidak dikenal")
			return
		}
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	ctx, cancel := context.WithTimeout(c.Request.Context(), penarikanTimeout)
	defer cancel()
	riwayat, err := Penarik.Riwayat(ctx, dataset, limit)
	if err != nil {
		penarikGagal(c, "riwayat", err)
		return
	}
	admin := dgIsAdmin(c)
	for i := range riwayat {
		riwayat[i] = maskRiwayat(riwayat[i], admin)
	}
	utils.SuccessResponse(c, http.StatusOK, "Berhasil mengambil riwayat penarikan", riwayat)
}

// permintaanPenarikan: badan POST /inaproc/penarikan. Dua bentuk yang bisa dipadukan:
//   - Tugas: daftar tugas eksplisit (dataset beserta isiannya), satu-satunya cara untuk dataset per kode, kategori bertingkat, dan transaksi
//     dengan penyaring khusus.
//   - Datasets x Tahun: matriks; tiap dataset per KLPD+tahun ditarik untuk tiap tahun, dataset per KLPD sekali. SemuaOtomatis = semua dataset
//     yang bisa ditarik otomatis.
type permintaanPenarikan struct {
	Tugas []struct {
		Dataset string `json:"dataset"`
		PermintaanTarik
	} `json:"tugas"`
	Datasets      []string `json:"datasets"`
	SemuaOtomatis bool     `json:"semua_otomatis"`
	Tahun         []string `json:"tahun"`
	KodeKLPD      string   `json:"kode_klpd"`
}

// susunTugas mengubah permintaan menjadi tugas yang sudah divalidasi; pesan tidak kosong = ditolak (400).
func susunTugas(req permintaanPenarikan) ([]Tugas, string) {
	var tugas []Tugas
	tambah := func(d *DatasetPenarikan, p PermintaanTarik) string {
		p = d.Normalisasi(p)
		if pesan := d.Validasi(p); pesan != "" {
			return fmt.Sprintf("%s: %s", d.Nama, pesan)
		}
		tugas = append(tugas, Tugas{Dataset: d, Perm: p})
		return ""
	}

	for _, t := range req.Tugas {
		d, ok := DatasetByID(t.Dataset)
		if !ok {
			return nil, "Dataset tidak dikenal: " + potong(t.Dataset, 60)
		}
		if pesan := tambah(d, t.PermintaanTarik); pesan != "" {
			return nil, pesan
		}
	}

	ids := req.Datasets
	if req.SemuaOtomatis {
		ids = append(append([]string(nil), ids...), IDDatasetOtomatis()...)
	}
	seen := map[string]bool{}
	for _, id := range ids {
		if seen[id] {
			continue
		}
		seen[id] = true
		d, ok := DatasetByID(id)
		if !ok {
			return nil, "Dataset tidak dikenal: " + potong(id, 60)
		}
		switch d.Mode {
		case ModeKode:
			return nil, d.Nama + ": dataset ini ditarik per kode; isi kodenya lewat daftar tugas"
		case ModeKLPD, ModeKategori:
			if pesan := tambah(d, PermintaanTarik{KodeKLPD: req.KodeKLPD}); pesan != "" {
				return nil, pesan
			}
		default: // klpd_tahun dan transaksi: per tahun
			if len(req.Tahun) == 0 {
				return nil, d.Nama + ": pilih minimal satu tahun"
			}
			for _, th := range req.Tahun {
				if pesan := tambah(d, PermintaanTarik{KodeKLPD: req.KodeKLPD, Tahun: th}); pesan != "" {
					return nil, pesan
				}
			}
		}
	}
	return tugas, ""
}

// StartInaprocPenarikan (admin): memasukkan tugas ke antrean dan langsung kembali; penarikan berjalan di latar belakang.
func StartInaprocPenarikan(c *gin.Context) {
	if !penarikReady(c) {
		return
	}
	var req permintaanPenarikan
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, "Data tidak valid")
		return
	}
	if config.Cfg == nil || config.Cfg.InaprocToken == "" {
		utils.ErrorResponse(c, http.StatusServiceUnavailable, "Integrasi Inaproc belum dikonfigurasi (token kosong)")
		return
	}
	tugas, pesan := susunTugas(req)
	if pesan != "" {
		utils.ErrorResponse(c, http.StatusBadRequest, pesan)
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), penarikanTimeout)
	defer cancel()
	jeda := jedaAntarTugasBawaan
	if p, err := Penarik.MuatPengaturan(ctx); err == nil {
		jeda = time.Duration(p.JedaDetik) * time.Second
	}
	oleh := Oleh{ID: c.GetString("user_id"), Nama: c.GetString("username")}
	info, err := Penarik.Start(tugas, PemicuManual, oleh, jeda)
	switch {
	case err == nil:
		log.Printf("[INAPROC PENARIKAN] %d tugas dimulai oleh %s", info.Total, oleh.Nama)
		c.JSON(http.StatusAccepted, gin.H{"success": true, "message": "Penarikan dimulai", "data": gin.H{"aktif": info}})
	case errors.Is(err, ErrPenarikanSibuk):
		utils.ErrorResponse(c, http.StatusConflict, "Penarikan lain sedang berjalan. Tunggu sampai selesai atau batalkan dulu.")
	case errors.Is(err, ErrTidakAdaTugas):
		utils.ErrorResponse(c, http.StatusBadRequest, "Pilih minimal satu dataset")
	default:
		log.Println("[INAPROC PENARIKAN ERROR] gagal memulai:", err)
		utils.ErrorResponse(c, http.StatusInternalServerError, "Gagal memulai penarikan")
	}
}

// CancelInaprocPenarikan (admin): membatalkan antrean yang sedang berjalan.
func CancelInaprocPenarikan(c *gin.Context) {
	if !penarikReady(c) {
		return
	}
	dibatalkan := Penarik.Batalkan()
	if dibatalkan {
		log.Printf("[INAPROC PENARIKAN] penarikan dibatalkan oleh %s", c.GetString("username"))
	}
	utils.SuccessResponse(c, http.StatusOK, "Permintaan pembatalan diterima", gin.H{"dibatalkan": dibatalkan})
}

// PutInaprocPenarikanPengaturan (admin): menyimpan pengaturan penarikan otomatis; berlaku pada putaran penjadwal berikutnya.
func PutInaprocPenarikanPengaturan(c *gin.Context) {
	if !penarikReady(c) {
		return
	}
	var p Pengaturan
	if err := c.ShouldBindJSON(&p); err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, "Data tidak valid")
		return
	}
	if pesan := p.Rapikan(); pesan != "" {
		utils.ErrorResponse(c, http.StatusBadRequest, pesan)
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), penarikanTimeout)
	defer cancel()
	if err := Penarik.SimpanPengaturan(ctx, p, c.GetString("username")); err != nil {
		penarikGagal(c, "simpan pengaturan", err)
		return
	}
	log.Printf("[INAPROC PENARIKAN] pengaturan otomatis diubah oleh %s (aktif=%v, tiap %d hari, %02d.00-%02d.00 WIB, %d dataset)",
		c.GetString("username"), p.Aktif, p.IntervalHari, p.JamMulai, p.JamAkhir, len(p.Dataset))
	tersimpan, err := Penarik.MuatPengaturan(ctx)
	if err != nil {
		penarikGagal(c, "muat pengaturan", err)
		return
	}
	info, err := Penarik.InfoOtomatis(ctx, tersimpan, time.Now())
	if err != nil {
		penarikGagal(c, "info otomatis", err)
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "Pengaturan penarikan otomatis disimpan", gin.H{"pengaturan": tersimpan, "otomatis": info})
}
