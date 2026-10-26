package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"pasti-v3-backend/apiakses"
	"pasti-v3-backend/utils"
)

// Dokumentasi API Akses Data di aplikasi (tab Dokumentasi pada menu API Akses Data, khusus superadmin). Rujukan teknisnya dibaca dari kode yang sama dengan yang melayani klien:
// katalog dataset dan kolom, penyaring per dataset, uraian parameter, nama header, kode galat gerbang, dan bawaan batas. Dengan begitu dokumentasi tidak dapat menyimpang dari
// perilaku sebenarnya. Uraian umum (alur, contoh kode) ada di frontend.

type headerDokumentasi struct {
	Nama string `json:"nama"`
	Arah string `json:"arah"` // "permintaan" (dikirim klien) atau "respons" (dikirim PASTI)
	Arti string `json:"arti"`
}

type endpointDokumentasi struct {
	Metode string `json:"metode"`
	Path   string `json:"path"` // relatif terhadap alamat_dasar; bagian rute ditulis {nama}
	Isi    string `json:"isi"`
	Kuota  bool   `json:"kuota"` // true bila permintaan menghabiskan kuota
}

type galatDokumentasi struct {
	HTTP int    `json:"http"`
	Kode string `json:"kode,omitempty"` // nilai "code" pada isi respons; kosong bila tidak ada
	Arti string `json:"arti"`
}

type datasetDokumentasi struct {
	katalogItemRes
	Tertutup []string `json:"tertutup,omitempty"` // kolom tabel pengadaan yang sengaja tidak keluar
}

type bawaanDokumentasi struct {
	LimitPermintaan   int `json:"limit_permintaan"`
	LimitJendelaDetik int `json:"limit_jendela_detik"`
	KuotaPermintaan   int `json:"kuota_permintaan"`
	KuotaPeriodeDetik int `json:"kuota_periode_detik"`
	HalamanBawaan     int `json:"halaman_bawaan"`
	HalamanMaks       int `json:"halaman_maks"`
	MinPencarian      int `json:"min_pencarian"`
}

// endpointGerbang: rute klien di bawah apiakses.AwalanRute. Tes memastikan daftar ini sama dengan rute yang dipasang di routes/routes.go.
func endpointGerbang() []endpointDokumentasi {
	return []endpointDokumentasi{
		{"GET", "/katalog", "Dataset dalam scope kunci (aset dan pengadaan) beserta kolom, tipe, dan penyaring yang didukung.", false},
		{"GET", "/aset/{dataset}", "Satu halaman data aset.", true},
		{"GET", "/aset/{dataset}/{id}", "Satu baris data aset.", true},
		{"GET", "/pengadaan/{dataset}", "Satu halaman data pengadaan (Inaproc).", true},
		{"GET", "/pengadaan/{dataset}/{id}", "Satu baris data pengadaan.", true},
		{"GET", "/saya", "Profil klien, scope, batas yang berlaku, dan sisa kuota.", false},
		{"GET", "/saya/penggunaan", "Ringkasan pemakaian sendiri: jumlah, galat, 429, latensi, dan permintaan terakhir. Parameter rentang: 1j, 6j, 24j, 7h, atau 30h.", false},
	}
}

func headerGerbang() []headerDokumentasi {
	return []headerDokumentasi{
		{"Authorization", "permintaan", "Bearer <kunci API>. Pilih ini atau X-API-Key."},
		{"X-API-Key", "permintaan", "Kunci API langsung, bila klien tidak dapat mengirim Authorization."},
		{"X-RateLimit-Limit", "respons", "Jumlah permintaan yang diizinkan per jendela batas laju."},
		{"X-RateLimit-Remaining", "respons", "Sisa permintaan pada jendela batas laju."},
		{"X-RateLimit-Reset", "respons", "Waktu batas laju pulih penuh (detik Unix, UTC)."},
		{"X-Quota-Limit", "respons", "Jumlah permintaan yang diizinkan per periode kuota."},
		{"X-Quota-Remaining", "respons", "Sisa permintaan pada periode kuota."},
		{"X-Quota-Reset", "respons", "Waktu kuota pulih (detik Unix, UTC)."},
		{"Retry-After", "respons", "Pada 429 dan 503: detik yang harus ditunggu sebelum mengulang."},
		{"X-Request-ID", "respons", "Pengenal permintaan; sertakan saat melapor ke administrator agar permintaan dapat dilacak."},
		{"Cache-Control", "respons", "Selalu no-store: jangan menyimpan respons di cache perantara."},
		{"WWW-Authenticate", "respons", "Pada 401: skema Bearer dengan realm pasti-api."},
	}
}

func galatGerbang() []galatDokumentasi {
	return []galatDokumentasi{
		{http.StatusBadRequest, "", "Parameter tidak valid, atau penyaring yang tidak didukung dataset itu (pesan menyebut parameternya)."},
		{http.StatusUnauthorized, apiakses.KodeKunciTidakValid, "Kunci tidak ada, salah, kedaluwarsa, dicabut, atau klien belum atau tidak lagi disetujui (sengaja tidak dibedakan)."},
		{http.StatusForbidden, apiakses.KodeAlamatDitolak, "Alamat IP tidak ada dalam allowlist kunci."},
		{http.StatusForbidden, apiakses.KodeDatasetDitolak, "Dataset di luar scope kunci."},
		{http.StatusNotFound, "", "Dataset atau baris tidak ada, termasuk dataset pengadaan yang dibuka lewat alamat /aset/... atau sebaliknya."},
		{http.StatusTooManyRequests, apiakses.KodeBatasLaju, "Batas laju terlampaui; tunggu sesuai Retry-After."},
		{http.StatusTooManyRequests, apiakses.KodeKuotaHabis, "Kuota habis; tunggu sampai X-Quota-Reset."},
		{http.StatusTooManyRequests, apiakses.KodeTerlaluBanyak, "Terlalu banyak percobaan dengan kunci tidak valid dari alamat ini."},
		{http.StatusServiceUnavailable, apiakses.KodeLayananGagal, "Layanan sedang bermasalah; coba lagi setelah Retry-After."},
	}
}

// susunDokumentasi menyusun isi jawaban dari kode (bukan dari database) dan pengaturan global yang berlaku.
func susunDokumentasi(p apiakses.Pengaturan) gin.H {
	dataset := []datasetDokumentasi{}
	for _, d := range apiakses.Katalog() {
		dataset = append(dataset, datasetDokumentasi{katalogItemRes: itemKatalog(d)})
	}
	for _, d := range apiakses.KatalogPengadaan() {
		dataset = append(dataset, datasetDokumentasi{katalogItemRes: itemKatalogPengadaan(d), Tertutup: d.Tertutup})
	}
	return gin.H{
		"alamat_dasar": apiakses.AwalanRute,
		"bawaan": bawaanDokumentasi{
			LimitPermintaan: p.LimitPermintaan, LimitJendelaDetik: p.LimitJendelaDetik, KuotaPermintaan: p.KuotaPermintaan, KuotaPeriodeDetik: p.KuotaPeriodeDetik,
			HalamanBawaan: apiakses.HalamanBawaan, HalamanMaks: p.HalamanMaks, MinPencarian: apiakses.MinPencarian,
		},
		"endpoint":  endpointGerbang(),
		"parameter": parameterKatalog(),
		"header":    headerGerbang(),
		"galat":     galatGerbang(),
		"dataset":   dataset,
	}
}

// GET /api-akses/dokumentasi: bahan dokumentasi API untuk superadmin. Berbeda dengan /katalog (milik klien, hanya dataset dalam scope kuncinya), isinya SEMUA dataset.
func GetApiAksesDokumentasi(c *gin.Context) {
	if !aksesSiap(c) {
		return
	}
	ctx, cancel := aksesCtx(c)
	defer cancel()
	utils.SuccessResponse(c, http.StatusOK, "OK", susunDokumentasi(apiAkses.Pengaturan(ctx)))
}
