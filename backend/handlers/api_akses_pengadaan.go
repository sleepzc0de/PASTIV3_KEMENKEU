package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"pasti-v3-backend/apiakses"
	"pasti-v3-backend/database"
	"pasti-v3-backend/utils"
)

// API data PENGADAAN untuk klien eksternal (rute /api/data/v1/pengadaan, di belakang apiakses.Gerbang yang sama dengan data aset: kunci API, status klien, allowlist IP, batas laju,
// dan kuota). Hanya membaca. Hanya dataset dalam scope kunci itu, dan hanya kolom yang ada di daftar keluar katalog (apiakses/katalog_pengadaan.go): data orang (PPK, Pokja),
// NPWP, rekening, dan isi bebas tidak pernah keluar. Pembatasan UE1 klien memakai aturan per satker yang sama dengan halaman Pengadaan Terpadu (kondisiSatkerInaproc).

// datasetPengadaanKlien: dataset dari rute, dalam scope kunci ini. false bila jawaban sudah ditulis.
func datasetPengadaanKlien(c *gin.Context, id *apiakses.Identitas) (apiakses.DatasetPengadaan, bool) {
	d, ok := apiakses.CariDatasetPengadaan("pengadaan." + c.Param("dataset"))
	if !ok {
		utils.ErrorResponse(c, http.StatusNotFound, "Dataset tidak ditemukan")
		return apiakses.DatasetPengadaan{}, false
	}
	apiakses.CatatDataset(c, d.Kode)
	if !id.BolehDataset(d.Kode) {
		apiakses.TolakScope(c, d.Kode)
		return apiakses.DatasetPengadaan{}, false
	}
	return d, true
}

// GET /pengadaan/:dataset: satu halaman data. Paginasi dengan kursor (id baris terakhir).
func GetPengadaanDaftar(c *gin.Context) {
	id := klienTerotentikasi(c)
	if id == nil {
		return
	}
	d, ok := datasetPengadaanKlien(c, id)
	if !ok {
		return
	}
	// Penyaring wilayah milik dataset aset: diabaikan diam-diam akan mengembalikan seluruh data padahal pemanggil mengira sudah tersaring.
	if strings.TrimSpace(c.Query("provinsi")) != "" || strings.TrimSpace(c.Query("kab_kota")) != "" {
		utils.ErrorResponse(c, http.StatusBadRequest, "provinsi dan kab_kota hanya berlaku untuk dataset aset")
		return
	}
	f := apiakses.FilterPengadaan{
		Q: strings.TrimSpace(c.Query("q")), KodeKLPD: strings.TrimSpace(c.Query("kode_klpd")), Tahun: strings.TrimSpace(c.Query("tahun")),
		KodeSatker: strings.TrimSpace(c.Query("kode_satker")), KodeUE1: strings.TrimSpace(c.Query("kode_ue1")), Kursor: strings.TrimSpace(c.Query("kursor")),
		Hitung: benar(c.Query("hitung")), UE1Klien: id.UE1, Batas: kondisiSatkerInaproc,
	}
	if v := strings.TrimSpace(c.Query("per_halaman")); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			utils.ErrorResponse(c, http.StatusBadRequest, "per_halaman harus bilangan bulat positif")
			return
		}
		f.PerHalaman = n
	}
	if err := d.Periksa(f); err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}
	ctx, cancel := aksesCtx(c)
	defer cancel()
	h, err := d.BacaDaftar(ctx, database.DB, f, apiAkses.Pengaturan(ctx).HalamanMaks)
	if err != nil {
		aksesGagal(c, "baca data "+d.Kode, err)
		return
	}
	apiakses.CatatBaris(c, len(h.Baris))
	data := gin.H{"dataset": d.Kode, "jumlah": len(h.Baris), "baris": h.Baris, "ada_berikutnya": h.AdaBerikutnya, "kursor_berikutnya": nil}
	if h.AdaBerikutnya {
		data["kursor_berikutnya"] = h.KursorBerikutnya
	}
	if h.Total != nil {
		data["total"] = *h.Total
	}
	if t := sinkronTerakhir(c, d.Kode, d.TerakhirSinkron); t != nil {
		data["disinkronkan_terakhir"] = t
	}
	utils.SuccessResponse(c, http.StatusOK, "OK", data)
}

// GET /pengadaan/:dataset/:id: satu baris.
func GetPengadaanDetail(c *gin.Context) {
	id := klienTerotentikasi(c)
	if id == nil {
		return
	}
	d, ok := datasetPengadaanKlien(c, id)
	if !ok {
		return
	}
	ctx, cancel := aksesCtx(c)
	defer cancel()
	b, err := d.BacaDetail(ctx, database.DB, strings.TrimSpace(c.Param("id")), id.UE1, kondisiSatkerInaproc)
	if errors.Is(err, apiakses.ErrTidakAda) {
		utils.ErrorResponse(c, http.StatusNotFound, "Data tidak ditemukan")
		return
	}
	if err != nil {
		aksesGagal(c, "baca detail "+d.Kode, err)
		return
	}
	apiakses.CatatBaris(c, 1)
	utils.SuccessResponse(c, http.StatusOK, "OK", gin.H{"dataset": d.Kode, "baris": b})
}
