package handlers

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"pasti-v3-backend/apiakses"
	"pasti-v3-backend/database"
	"pasti-v3-backend/utils"
)

// API data untuk klien eksternal (rute /api/data/v1, dilindungi apiakses.Gerbang: kunci API, status klien, allowlist IP, batas laju, dan kuota). Tidak memakai
// autentikasi pengguna dan hanya membaca. Setiap permintaan hanya bisa menjangkau dataset dalam scope kunci itu, dan hanya kolom yang lolos kebijakan kolom
// (apiakses/katalog.go): kolom pribadi dan tautan foto tidak pernah keluar.

const (
	pesanKlienGagal  = "Terjadi kesalahan pada server"
	maksRentangKlien = 30 * 24 * time.Hour
)

func klienTerotentikasi(c *gin.Context) *apiakses.Identitas {
	id := apiakses.IdentitasDari(c)
	if id == nil { // tidak mungkin terjadi bila rute dipasang di belakang gerbang; tertutup bila terjadi
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"success": false, "message": "Kunci API tidak valid atau tidak aktif", "code": apiakses.KodeKunciTidakValid})
	}
	return id
}

type infoKolomRes struct {
	Nama string `json:"nama"`
	Tipe string `json:"tipe"`
}

type katalogItemRes struct {
	Kode        string         `json:"kode"`
	Label       string         `json:"label"`
	Deskripsi   string         `json:"deskripsi"`
	Kelompok    string         `json:"kelompok"`
	Subkelompok string         `json:"subkelompok,omitempty"`
	Alamat      string         `json:"alamat"`
	Geo         bool           `json:"geo"`
	Penyaring   []string       `json:"penyaring"` // parameter penyaring yang berlaku untuk dataset ini (selain per_halaman, kursor, dan hitung)
	Kolom       []infoKolomRes `json:"kolom"`
}

func itemKatalog(d apiakses.DatasetAPI) katalogItemRes {
	r := katalogItemRes{
		Kode: d.Kode, Label: d.Label, Deskripsi: d.Deskripsi, Kelompok: apiakses.KelompokAset, Geo: d.Geo, Alamat: apiakses.AwalanRute + "/aset/" + d.Kunci(),
		Penyaring: []string{"q", "kode_satker", "kode_ue1"}, Kolom: []infoKolomRes{{Nama: "id", Tipe: "bilangan"}},
	}
	if d.KolomProvinsi() != "" {
		r.Penyaring = append(r.Penyaring, "provinsi")
	}
	if d.KolomKabKota() != "" {
		r.Penyaring = append(r.Penyaring, "kab_kota")
	}
	for _, k := range d.Kolom {
		r.Kolom = append(r.Kolom, infoKolomRes{Nama: k.Nama, Tipe: k.Tipe})
	}
	r.Kolom = append(r.Kolom, infoKolomRes{Nama: "disinkronkan_pada", Tipe: "waktu"})
	return r
}

func itemKatalogPengadaan(d apiakses.DatasetPengadaan) katalogItemRes {
	r := katalogItemRes{
		Kode: d.Kode, Label: d.Label, Deskripsi: d.Deskripsi, Kelompok: apiakses.KelompokPengadaan, Subkelompok: d.Subkelompok, Alamat: apiakses.AwalanRute + "/pengadaan/" + d.Kunci(),
		Penyaring: []string{"kode_satker", "kode_ue1"}, Kolom: []infoKolomRes{{Nama: "id", Tipe: "teks"}},
	}
	if len(d.KolomPencarian()) > 0 {
		r.Penyaring = append([]string{"q"}, r.Penyaring...)
	}
	if d.PunyaKLPD() {
		r.Penyaring = append(r.Penyaring, "kode_klpd")
	}
	if d.PunyaTahun() {
		r.Penyaring = append(r.Penyaring, "tahun")
	}
	for _, k := range d.Kolom {
		r.Kolom = append(r.Kolom, infoKolomRes{Nama: k.Nama, Tipe: k.Tipe})
	}
	r.Kolom = append(r.Kolom, infoKolomRes{Nama: "disinkronkan_pada", Tipe: "waktu"})
	return r
}

// GET /katalog: dataset dalam scope kunci ini (aset dan pengadaan) beserta kolomnya dan penyaring yang didukung.
func GetAksesKatalog(c *gin.Context) {
	id := klienTerotentikasi(c)
	if id == nil {
		return
	}
	items := []katalogItemRes{}
	for _, d := range apiakses.Katalog() {
		if id.BolehDataset(d.Kode) {
			items = append(items, itemKatalog(d))
		}
	}
	for _, d := range apiakses.KatalogPengadaan() {
		if id.BolehDataset(d.Kode) {
			items = append(items, itemKatalogPengadaan(d))
		}
	}
	utils.SuccessResponse(c, http.StatusOK, "OK", gin.H{"dataset": items, "parameter": parameterKatalog()})
}

// parameterKatalog: uraian parameter daftar data. Dipakai /katalog (untuk klien) dan dokumentasi di aplikasi (untuk superadmin) supaya keduanya selalu sama.
func parameterKatalog() []gin.H {
	return []gin.H{
		{"nama": "per_halaman", "uraian": "Jumlah baris per permintaan (bawaan " + strconv.Itoa(apiakses.HalamanBawaan) + ", maksimal sesuai pengaturan PASTI)"},
		{"nama": "kursor", "uraian": "Nilai kursor_berikutnya dari respons sebelumnya; kosongkan untuk mulai dari awal. Dataset aset: bilangan; dataset pengadaan: teks buram (sama dengan id baris terakhir)"},
		{"nama": "hitung", "uraian": "1 untuk menyertakan total baris yang cocok (lebih lambat)"},
		{"nama": "q", "uraian": "Pencarian teks (minimal " + strconv.Itoa(apiakses.MinPencarian) + " karakter) pada nama, alamat, dan kode (aset) atau nama paket, satker, dan kode paket (pengadaan)"},
		{"nama": "kode_satker", "uraian": "Dataset aset: kode satker 6 digit, atau awalan kode satker lengkap. Dataset pengadaan: kode satker 6 digit"},
		{"nama": "kode_ue1", "uraian": "Kode UE1 5 digit. Dataset pengadaan memakai daftar satker data aset untuk menentukan satker milik UE1 itu"},
		{"nama": "provinsi", "uraian": "Nama provinsi persis (hanya dataset aset yang punya kolomnya)"},
		{"nama": "kab_kota", "uraian": "Nama kabupaten/kota persis (hanya dataset aset yang punya kolomnya)"},
		{"nama": "kode_klpd", "uraian": "Kode KLPD Inaproc, mis. K10 (hanya dataset pengadaan)"},
		{"nama": "tahun", "uraian": "Tahun anggaran empat digit (hanya dataset pengadaan yang punya kolom tahun)"},
	}
}

// akses dataset: dataset dari rute, dalam scope kunci ini. false bila jawaban sudah ditulis.
func datasetKlien(c *gin.Context, id *apiakses.Identitas) (apiakses.DatasetAPI, bool) {
	d, ok := apiakses.CariDataset("aset." + c.Param("dataset"))
	if !ok {
		utils.ErrorResponse(c, http.StatusNotFound, "Dataset tidak ditemukan")
		return apiakses.DatasetAPI{}, false
	}
	apiakses.CatatDataset(c, d.Kode)
	if !id.BolehDataset(d.Kode) {
		apiakses.TolakScope(c, d.Kode)
		return apiakses.DatasetAPI{}, false
	}
	return d, true
}

// Waktu sinkronisasi terakhir per dataset disimpan sebentar supaya tidak ada query tambahan pada setiap penarikan.
var (
	muSinkron  sync.Mutex
	cacheSinkr = map[string]struct {
		t   *time.Time
		exp time.Time
	}{}
)

// sinkronTerakhir membaca waktu sinkronisasi terakhir dataset berkode kode lewat baca (dataset aset dan pengadaan punya sumber yang berbeda).
func sinkronTerakhir(c *gin.Context, kode string, baca func(context.Context, *sql.DB) (*time.Time, error)) *time.Time {
	now := time.Now()
	muSinkron.Lock()
	if v, ok := cacheSinkr[kode]; ok && now.Before(v.exp) {
		muSinkron.Unlock()
		return v.t
	}
	muSinkron.Unlock()
	ctx, cancel := aksesCtx(c)
	defer cancel()
	t, err := baca(ctx, database.DB)
	if err != nil {
		return nil // tidak fatal: hanya keterangan kesegaran data
	}
	muSinkron.Lock()
	cacheSinkr[kode] = struct {
		t   *time.Time
		exp time.Time
	}{t, now.Add(time.Minute)}
	muSinkron.Unlock()
	return t
}

func benar(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "1", "true", "ya", "y":
		return true
	}
	return false
}

// GET /aset/:dataset: satu halaman data. Paginasi dengan kursor.
func GetAksesDaftar(c *gin.Context) {
	id := klienTerotentikasi(c)
	if id == nil {
		return
	}
	d, ok := datasetKlien(c, id)
	if !ok {
		return
	}
	f := apiakses.FilterData{
		Q: strings.TrimSpace(c.Query("q")), KodeSatker: strings.TrimSpace(c.Query("kode_satker")), KodeUE1: strings.TrimSpace(c.Query("kode_ue1")),
		Provinsi: strings.TrimSpace(c.Query("provinsi")), KabKota: strings.TrimSpace(c.Query("kab_kota")), Hitung: benar(c.Query("hitung")), UE1Klien: id.UE1,
	}
	if v := strings.TrimSpace(c.Query("kursor")); v != "" {
		k, err := strconv.ParseInt(v, 10, 64)
		if err != nil || k < 0 {
			utils.ErrorResponse(c, http.StatusBadRequest, "kursor tidak valid")
			return
		}
		f.Kursor = k
	}
	if v := strings.TrimSpace(c.Query("per_halaman")); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			utils.ErrorResponse(c, http.StatusBadRequest, "per_halaman harus bilangan bulat positif")
			return
		}
		f.PerHalaman = n
	}
	if err := f.Validasi(); err != nil {
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

// GET /aset/:dataset/:id: satu baris.
func GetAksesDetail(c *gin.Context) {
	id := klienTerotentikasi(c)
	if id == nil {
		return
	}
	d, ok := datasetKlien(c, id)
	if !ok {
		return
	}
	rid, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || rid <= 0 {
		utils.ErrorResponse(c, http.StatusBadRequest, "ID tidak valid")
		return
	}
	ctx, cancel := aksesCtx(c)
	defer cancel()
	b, err := d.BacaDetail(ctx, database.DB, rid, id.UE1)
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

// pemakaianRes: keadaan batas laju dan kuota klien sekarang.
type pemakaianRes struct {
	LimitSisa     int       `json:"limit_sisa"`
	KuotaTerpakai int       `json:"kuota_terpakai"`
	KuotaSisa     int       `json:"kuota_sisa"`
	KuotaReset    time.Time `json:"kuota_reset"`
}

// GET /saya: profil singkat klien, scope, dan batas yang berlaku. Tidak memakai kuota.
func GetAksesSaya(c *gin.Context) {
	id := klienTerotentikasi(c)
	if id == nil {
		return
	}
	d := id.Terakhir
	utils.SuccessResponse(c, http.StatusOK, "OK", gin.H{
		"klien":        gin.H{"kode": id.KlienKode, "nama_aplikasi": id.KlienNama},
		"kunci":        gin.H{"prefix": id.Prefix},
		"dataset":      id.DaftarDataset(),
		"ue1":          nilaiAtauKosong(id.UE1),
		"ip_diizinkan": nilaiAtauKosong(id.IP),
		"batas":        id.Batas,
		"pemakaian":    pemakaianRes{LimitSisa: d.LimitSisa, KuotaTerpakai: d.KuotaTerpakai, KuotaSisa: d.KuotaSisa, KuotaReset: d.KuotaReset},
	})
}

func nilaiAtauKosong(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// GET /saya/penggunaan?rentang=1j|6j|24j|7h|30h: ringkasan pemakaian klien sendiri (jumlah, galat, 429, latensi, per endpoint, deret waktu, dan permintaan terakhir).
// Hanya data milik klien yang memanggil. Tidak memakai kuota.
func GetAksesSayaPenggunaan(c *gin.Context) {
	id := klienTerotentikasi(c)
	if id == nil {
		return
	}
	dari, sampai, err := rentangPantau(c, maksRentangKlien)
	if err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}
	p := apiakses.PenyaringPantau{Dari: dari, Sampai: sampai, KlienID: id.KlienID}
	ctx, cancel := aksesCtx(c)
	defer cancel()
	perHari := sampai.Sub(dari) > 72*time.Hour
	ring, _, err := apiAkses.Store.Ringkas(ctx, p)
	if err != nil {
		aksesGagal(c, "ringkasan pemakaian klien", err)
		return
	}
	deret, err := apiAkses.Store.Deret(ctx, p, perHari)
	if err != nil {
		aksesGagal(c, "deret pemakaian klien", err)
		return
	}
	ep, err := apiAkses.Store.PerEndpoint(ctx, p)
	if err != nil {
		aksesGagal(c, "pemakaian klien per endpoint", err)
		return
	}
	riw, _, err := apiAkses.Store.Riwayat(ctx, apiakses.PenyaringRiwayat{Dari: dari, Sampai: sampai.Add(time.Minute), KlienID: id.KlienID, PerHalaman: 20})
	if err != nil {
		aksesGagal(c, "riwayat pemakaian klien", err)
		return
	}
	for i := range riw { // klien tidak perlu melihat data internal PASTI
		riw[i].KlienID, riw[i].KlienKode, riw[i].KlienNama, riw[i].KunciPrefix = 0, "", "", ""
	}
	d := id.Terakhir
	utils.SuccessResponse(c, http.StatusOK, "OK", gin.H{
		"dari": dari, "sampai": sampai, "per_hari": perHari, "ringkasan": ring, "deret": deret, "per_endpoint": ep, "riwayat_terakhir": riw,
		"pemakaian": pemakaianRes{LimitSisa: d.LimitSisa, KuotaTerpakai: d.KuotaTerpakai, KuotaSisa: d.KuotaSisa, KuotaReset: d.KuotaReset}, "batas": id.Batas,
	})
}
