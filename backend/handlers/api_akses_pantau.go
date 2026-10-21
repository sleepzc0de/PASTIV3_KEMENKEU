package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"pasti-v3-backend/apiakses"
	"pasti-v3-backend/utils"
)

// Pemantauan penarikan data (khusus superadmin, hanya membaca): jumlah permintaan per klien/endpoint/waktu, pemakaian kuota saat ini, galat dan 429, latensi, dan
// riwayat permintaan terakhir. Angka jangka panjang dari hitungan per jam (api_akses_agregat); riwayat per permintaan dari api_akses_log (masa simpan pendek).

// maksRentangPantau: rentang terpanjang yang dapat diminta sekali jalan.
const maksRentangPantau = 400 * 24 * time.Hour

var rentangPantauNama = map[string]time.Duration{
	"1j": time.Hour, "6j": 6 * time.Hour, "24j": 24 * time.Hour, "7h": 7 * 24 * time.Hour, "30h": 30 * 24 * time.Hour, "90h": 90 * 24 * time.Hour, "1t": 365 * 24 * time.Hour,
}

// rentangPantau membaca rentang waktu dari kueri: rentang=1j|6j|24j|7h|30h|90h|1t (bawaan 24j), atau dari dan sampai (RFC3339 atau tanggal WIB).
func rentangPantau(c *gin.Context, maks time.Duration) (time.Time, time.Time, error) {
	now := time.Now().UTC()
	if c.Query("dari") != "" || c.Query("sampai") != "" {
		dari, err := bacaWaktuAudit(c.Query("dari"), false)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("tanggal awal tidak valid")
		}
		sampai, err := bacaWaktuAudit(c.Query("sampai"), true)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("tanggal akhir tidak valid")
		}
		if dari.IsZero() {
			dari = now.Add(-24 * time.Hour)
		}
		if sampai.IsZero() || sampai.After(now) {
			sampai = now
		}
		if !sampai.After(dari) {
			return time.Time{}, time.Time{}, fmt.Errorf("batas akhir waktu harus setelah batas awal")
		}
		if sampai.Sub(dari) > maks {
			return time.Time{}, time.Time{}, fmt.Errorf("rentang waktu terlalu panjang (maksimal %d hari)", int(maks.Hours()/24))
		}
		return dari, sampai, nil
	}
	d, ok := rentangPantauNama[c.DefaultQuery("rentang", "24j")]
	if !ok {
		d = 24 * time.Hour
	}
	if d > maks {
		d = maks
	}
	return now.Add(-d), now, nil
}

func penyaringPantau(c *gin.Context) (apiakses.PenyaringPantau, error) {
	dari, sampai, err := rentangPantau(c, maksRentangPantau)
	if err != nil {
		return apiakses.PenyaringPantau{}, err
	}
	p := apiakses.PenyaringPantau{Dari: dari, Sampai: sampai, Endpoint: strings.TrimSpace(c.Query("endpoint"))}
	if v := strings.TrimSpace(c.Query("klien_id")); v != "" {
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil || id < 0 {
			return p, fmt.Errorf("klien_id tidak valid")
		}
		p.KlienID = id
	}
	return p, nil
}

// pemantauanRes: seluruh isi halaman pemantauan dalam satu jawaban.
type pemantauanRes struct {
	Dari        time.Time                  `json:"dari"`
	Sampai      time.Time                  `json:"sampai"`
	PerHari     bool                       `json:"per_hari"`
	Ringkasan   apiakses.Metrik            `json:"ringkasan"`
	KlienAktif  int64                      `json:"klien_aktif"`
	Deret       []apiakses.TitikDeret      `json:"deret"`
	PerKlien    []apiakses.Kelompok        `json:"per_klien"`
	PerEndpoint []apiakses.Kelompok        `json:"per_endpoint"`
	Pemakaian   []apiakses.PemakaianKlien  `json:"pemakaian"`
	Pencatat    apiakses.StatistikPencatat `json:"pencatat"`
	Pengaturan  apiakses.Pengaturan        `json:"pengaturan"`
}

// GET /api-akses/pantau/ringkasan?rentang=&dari=&sampai=&klien_id=&endpoint=
func GetApiAksesPantauRingkasan(c *gin.Context) {
	if !aksesSiap(c) {
		return
	}
	p, err := penyaringPantau(c)
	if err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}
	ctx, cancel := aksesCtx(c)
	defer cancel()
	perHari := p.Sampai.Sub(p.Dari) > 72*time.Hour
	res := pemantauanRes{Dari: p.Dari, Sampai: p.Sampai, PerHari: perHari, Pencatat: apiAkses.Pencatat.Statistik(), Pengaturan: apiAkses.Pengaturan(ctx)}
	if res.Ringkasan, res.KlienAktif, err = apiAkses.Store.Ringkas(ctx, p); err != nil {
		aksesGagal(c, "ringkasan pemantauan", err)
		return
	}
	if res.Deret, err = apiAkses.Store.Deret(ctx, p, perHari); err != nil {
		aksesGagal(c, "deret pemantauan", err)
		return
	}
	if res.PerKlien, err = apiAkses.Store.PerKlien(ctx, p); err != nil {
		aksesGagal(c, "pemantauan per klien", err)
		return
	}
	if res.PerEndpoint, err = apiAkses.Store.PerEndpoint(ctx, p); err != nil {
		aksesGagal(c, "pemantauan per endpoint", err)
		return
	}
	if res.Pemakaian, err = apiAkses.PemakaianSaatIni(ctx); err != nil {
		aksesGagal(c, "pemakaian kuota saat ini", err)
		return
	}
	if p.KlienID > 0 { // saat difilter ke satu klien, pemakaian saat ini pun hanya klien itu
		ada := res.Pemakaian[:0]
		for _, u := range res.Pemakaian {
			if u.KlienID == p.KlienID {
				ada = append(ada, u)
			}
		}
		res.Pemakaian = ada
	}
	utils.SuccessResponse(c, http.StatusOK, "OK", res)
}

// GET /api-akses/pantau/riwayat?...: satu halaman riwayat permintaan terakhir menurut penyaring.
func GetApiAksesPantauRiwayat(c *gin.Context) {
	if !aksesSiap(c) {
		return
	}
	dari, sampai, err := rentangPantau(c, maksRentangPantau)
	if err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}
	f := apiakses.PenyaringRiwayat{
		Dari: dari, Sampai: sampai.Add(time.Minute), Endpoint: strings.TrimSpace(c.Query("endpoint")), Dataset: strings.TrimSpace(c.Query("dataset")),
		Hasil: strings.TrimSpace(c.Query("hasil")), Status: intQuery(c, "status"), IP: strings.TrimSpace(c.Query("ip")), Q: strings.TrimSpace(c.Query("q")),
		Halaman: intQuery(c, "halaman"), PerHalaman: intQuery(c, "per_halaman"),
	}
	if v := strings.TrimSpace(c.Query("klien_id")); v != "" {
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil || id < 0 {
			utils.ErrorResponse(c, http.StatusBadRequest, "klien_id tidak valid")
			return
		}
		f.KlienID = id
	}
	if f.Hasil != "" && !apiakses.HasilSah(f.Hasil) {
		utils.ErrorResponse(c, http.StatusBadRequest, "hasil tidak dikenal")
		return
	}
	ctx, cancel := aksesCtx(c)
	defer cancel()
	daftar, total, err := apiAkses.Store.Riwayat(ctx, f)
	if err != nil {
		aksesGagal(c, "riwayat permintaan", err)
		return
	}
	halaman := f.Halaman
	if halaman < 1 {
		halaman = 1
	}
	per := f.PerHalaman
	if per < 1 {
		per = apiakses.RiwayatPerHalamanBawaan
	}
	if per > apiakses.RiwayatPerHalamanMaks {
		per = apiakses.RiwayatPerHalamanMaks
	}
	utils.SuccessResponse(c, http.StatusOK, "OK", gin.H{"riwayat": daftar, "total": total, "halaman": halaman, "per_halaman": per, "retensi_hari": apiAkses.Pengaturan(ctx).LogRetensiHari})
}

// GET /api-akses/pantau/opsi: pilihan penyaring (klien, endpoint, hasil).
func GetApiAksesPantauOpsi(c *gin.Context) {
	if !aksesSiap(c) {
		return
	}
	ctx, cancel := aksesCtx(c)
	defer cancel()
	ks, err := apiAkses.Store.DaftarOpsiKlien(ctx)
	if err != nil {
		aksesGagal(c, "opsi klien", err)
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "OK", gin.H{"klien": ks, "endpoint": apiakses.DaftarEndpoint(), "hasil": apiakses.DaftarHasil()})
}
