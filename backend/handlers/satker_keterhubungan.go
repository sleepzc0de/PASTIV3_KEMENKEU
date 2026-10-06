package handlers

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"pasti-v3-backend/database"
	"pasti-v3-backend/digitalisasi"
	"pasti-v3-backend/peran"
	"pasti-v3-backend/utils"
)

// Keterhubungan satker antara data aset (Digitalisasi Aset) dan data pengadaan (Inaproc).
//
// Kode satker aset berbentuk 20 karakter, mis. 015040199119091000KP: KL (3) + UE1 (2) + ... + kode satker 6 digit (karakter 10-15) + kode
// anak (16-18, "000" = induk) + "KP". Kode satker 6 digit itulah yang sama dengan kd_satker_str di data Inaproc, jadi keduanya dihubungkan lewat
//
//	SUBSTRING(Kode_Satker, 10, 6)  =  kd_satker_str
//
// Satu kode 6 digit bisa memuat satu induk dan beberapa anak satker; semuanya dihitung sebagai satu satker. Kode Inaproc yang seluruhnya angka
// dan kurang dari 6 digit dilengkapi nol di depan (angka 0 di depan sering hilang bila kode diperlakukan sebagai bilangan).

const (
	// kunciAsetSQL: kode satker 6 digit dari kolom Kode_Satker aset (hanya untuk baris yang cukup panjang, lihat asetPunyaKode).
	kunciAsetSQL = "SUBSTRING(Kode_Satker, 10, 6)"
	// asetPunyaKode: kode satker aset yang bisa dibaca kunci 6 digitnya.
	asetPunyaKode = "Kode_Satker IS NOT NULL AND LEN(Kode_Satker) >= 15"
	// kunciInaprocSQL: kd_satker_str Inaproc yang dirapikan menjadi kunci 6 digit yang sama.
	kunciInaprocSQL = `CASE WHEN LTRIM(RTRIM(kd_satker_str)) NOT LIKE '%[^0-9]%' AND LEN(LTRIM(RTRIM(kd_satker_str))) BETWEEN 1 AND 6
		THEN RIGHT('000000' + LTRIM(RTRIM(kd_satker_str)), 6) ELSE LTRIM(RTRIM(kd_satker_str)) END`
	satkerTimeout = 45 * time.Second

	statusTerhubung      = "terhubung"       // punya data aset dan data pengadaan
	statusHanyaAset      = "hanya_aset"      // ada di data aset, tidak punya pengadaan pada tahun itu
	statusHanyaPengadaan = "hanya_pengadaan" // ada di pengadaan, tidak ditemukan di data aset (kode berbeda atau satker tanpa data BMN)
)

type asetSatker struct {
	Nama, KodeUE1, Jenis string
	KDJ, KDO             int64
	JumlahKode           int // berapa Kode_Satker (induk + anak) memakai kode 6 digit ini
	PerDataset           map[string]int64
	adaInduk             bool
}

type pengadaanSatker struct {
	Nama                         string
	RUPPaket, Tender, NonTender  int64
	RUPPagu, TenderPagu, NonPagu float64
}

type SatkerPengadaan struct {
	RUPPaket      int64   `json:"rup_paket"`
	RUPPagu       float64 `json:"rup_pagu"`
	Tender        int64   `json:"tender"`
	TenderPagu    float64 `json:"tender_pagu"`
	NonTender     int64   `json:"non_tender"`
	NonTenderPagu float64 `json:"non_tender_pagu"`
}

type SatkerTerhubung struct {
	Kode          string           `json:"kode"` // kode satker 6 digit
	Nama          string           `json:"nama"`
	NamaPengadaan string           `json:"nama_pengadaan,omitempty"` // nama menurut Inaproc; hanya bila berbeda atau satkernya tidak ada di data aset
	KodeUE1       string           `json:"kode_ue1"`
	UE1           string           `json:"ue1"` // label dari referensi UE1
	Jenis         string           `json:"jenis"`
	JumlahKode    int              `json:"jumlah_kode_aset"`
	Aset          map[string]int64 `json:"aset"`
	JumlahAset    int64            `json:"jumlah_aset"`
	KDJ           int64            `json:"kdj"`
	KDO           int64            `json:"kdo"`
	Pengadaan     SatkerPengadaan  `json:"pengadaan"`
	Status        string           `json:"status"`
}

type RingkasanKeterhubungan struct {
	SatkerAset      int `json:"satker_aset"`
	SatkerPengadaan int `json:"satker_pengadaan"`
	Terhubung       int `json:"terhubung"`
	HanyaAset       int `json:"hanya_aset"`
	HanyaPengadaan  int `json:"hanya_pengadaan"`
	// Baris aset yang kode satkernya terlalu pendek untuk dibaca kunci 6 digitnya (tidak ikut dihubungkan).
	AsetTanpaKode int64 `json:"aset_tanpa_kode"`
}

func (p pengadaanSatker) ada() bool { return p.RUPPaket > 0 || p.Tender > 0 || p.NonTender > 0 }

// gabungSatker menyatukan sisi aset dan sisi pengadaan menurut kunci 6 digit. Urutan hasil: menurut kode.
func gabungSatker(aset map[string]*asetSatker, peng map[string]*pengadaanSatker, ref map[string]RefUE1Baris) ([]SatkerTerhubung, RingkasanKeterhubungan) {
	kunci := make(map[string]bool, len(aset)+len(peng))
	for k := range aset {
		kunci[k] = true
	}
	for k, p := range peng {
		if p.ada() {
			kunci[k] = true
		}
	}
	urut := make([]string, 0, len(kunci))
	for k := range kunci {
		urut = append(urut, k)
	}
	sort.Strings(urut)

	out := make([]SatkerTerhubung, 0, len(urut))
	var r RingkasanKeterhubungan
	for _, k := range urut {
		s := SatkerTerhubung{Kode: k, Aset: map[string]int64{}}
		a, ada := aset[k]
		p := peng[k]
		if ada {
			s.Nama, s.KodeUE1, s.Jenis, s.KDJ, s.KDO, s.JumlahKode = a.Nama, a.KodeUE1, a.Jenis, a.KDJ, a.KDO, a.JumlahKode
			for ds, n := range a.PerDataset {
				s.Aset[ds] = n
				s.JumlahAset += n
			}
			if s.KodeUE1 != "" {
				s.UE1 = labelUE1(s.KodeUE1, ref)
			}
			r.SatkerAset++
		}
		if p != nil && p.ada() {
			s.Pengadaan = SatkerPengadaan{RUPPaket: p.RUPPaket, RUPPagu: p.RUPPagu, Tender: p.Tender, TenderPagu: p.TenderPagu, NonTender: p.NonTender, NonTenderPagu: p.NonPagu}
			if !ada {
				s.Nama = p.Nama
			} else if p.Nama != "" && !strings.EqualFold(strings.TrimSpace(p.Nama), strings.TrimSpace(a.Nama)) {
				s.NamaPengadaan = p.Nama
			}
			r.SatkerPengadaan++
		}
		switch {
		case ada && p != nil && p.ada():
			s.Status = statusTerhubung
			r.Terhubung++
		case ada:
			s.Status = statusHanyaAset
			r.HanyaAset++
		default:
			s.Status = statusHanyaPengadaan
			r.HanyaPengadaan++
		}
		out = append(out, s)
	}
	return out, r
}

// ---- pembacaan dari database ----

func bacaSatkerAset(ctx context.Context) (map[string]*asetSatker, int64, error) {
	hasil := map[string]*asetSatker{}
	var tanpaKode int64

	// Daftar satker: induk dan anak yang berkode 6 digit sama digabung; nama dan UE1 diambil dari induknya.
	rows, err := database.DB.QueryContext(ctx, `SELECT `+kunciAsetSQL+`, ISNULL(Kode_UE1, N''), ISNULL(Jenis_Satker, N''), ISNULL(Nama_Satker, N''),
		ISNULL(Jumlah_KDJ, 0), ISNULL(Jumlah_KDO, 0) FROM `+dgSumberTabel(ctx, "DIGITALISASI_SATKER", "Kode_Satker")+` WHERE `+asetPunyaKode+` ORDER BY Kode_Satker`)
	if err != nil {
		return nil, 0, fmt.Errorf("baca satker aset: %w", err)
	}
	for rows.Next() {
		var k, ue1, jenis, nama string
		var kdj, kdo int64
		if err := rows.Scan(&k, &ue1, &jenis, &nama, &kdj, &kdo); err != nil {
			rows.Close()
			return nil, 0, err
		}
		a := hasil[k]
		if a == nil {
			a = &asetSatker{PerDataset: map[string]int64{}}
			hasil[k] = a
		}
		a.JumlahKode++
		a.KDJ += kdj
		a.KDO += kdo
		induk := strings.EqualFold(strings.TrimSpace(jenis), "INDUK SATKER")
		if a.Nama == "" || (induk && !a.adaInduk) {
			a.Nama, a.KodeUE1, a.Jenis = nama, ue1, jenis
			a.adaInduk = induk
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, 0, err
	}
	// Baris tanpa kode satker yang terbaca tidak termasuk cakupan UE1/Kanwil/Satker mana pun, jadi hanya dihitung bagi peran yang melihat seluruh data.
	semua := peran.CakupanDari(ctx).SemuaData()
	if semua {
		var satkerTanpa sql.NullInt64
		if err := database.DB.QueryRowContext(ctx, `SELECT COUNT_BIG(*) FROM DIGITALISASI_SATKER WHERE NOT (`+asetPunyaKode+`)`).Scan(&satkerTanpa); err != nil {
			return nil, 0, err
		}
		tanpaKode += satkerTanpa.Int64
	}

	// Jumlah aset per jenis. Satker yang hanya muncul di tabel aset (tidak ada di daftar satker) tetap dicatat dengan nama kosong.
	for _, ds := range digitalisasi.Datasets {
		if ds.Key == "satker" || ds.Roles.Satker != "Kode_Satker" {
			continue
		}
		rows, err := database.DB.QueryContext(ctx, fmt.Sprintf(`SELECT %s, COUNT_BIG(*) FROM %s WHERE %s GROUP BY %s`, kunciAsetSQL, dgSumber(ctx, ds), asetPunyaKode, kunciAsetSQL))
		if err != nil {
			return nil, 0, fmt.Errorf("hitung aset %s: %w", ds.Key, err)
		}
		for rows.Next() {
			var k string
			var n int64
			if err := rows.Scan(&k, &n); err != nil {
				rows.Close()
				return nil, 0, err
			}
			a := hasil[k]
			if a == nil {
				a = &asetSatker{PerDataset: map[string]int64{}}
				hasil[k] = a
			}
			a.PerDataset[ds.Key] += n
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, 0, err
		}
		if semua {
			var tk sql.NullInt64
			if err := database.DB.QueryRowContext(ctx, fmt.Sprintf(`SELECT COUNT_BIG(*) FROM %s WHERE NOT (%s)`, qc(ds.Table), asetPunyaKode)).Scan(&tk); err != nil {
				return nil, 0, err
			}
			tanpaKode += tk.Int64
		}
	}
	return hasil, tanpaKode, nil
}

// bacaSatkerPengadaan membaca jumlah dan nilai pengadaan per satker untuk satu KLPD dan tahun: paket RUP penyedia yang masih berlaku, tender
// (pengumuman versi terbaru), dan non-tender (pengumuman versi terbaru). Definisinya sama dengan dasbor Pengadaan (inaproc_analitik.go).
func bacaSatkerPengadaan(ctx context.Context, klpd, tahun string) (map[string]*pengadaanSatker, error) {
	hasil := map[string]*pengadaanSatker{}
	ambil := func(k string) *pengadaanSatker {
		p := hasil[k]
		if p == nil {
			p = &pengadaanSatker{}
			hasil[k] = p
		}
		return p
	}
	jalankan := func(label, q string, isi func(p *pengadaanSatker, nama string, n int64, nilai float64)) error {
		rows, err := database.DB.QueryContext(ctx, q, klpd, tahun)
		if err != nil {
			return fmt.Errorf("baca %s: %w", label, err)
		}
		defer rows.Close()
		for rows.Next() {
			var k string
			var nama sql.NullString
			var n int64
			var nilai float64
			if err := rows.Scan(&k, &nama, &n, &nilai); err != nil {
				return err
			}
			p := ambil(k)
			if p.Nama == "" {
				p.Nama = nama.String
			}
			isi(p, nama.String, n, nilai)
		}
		return rows.Err()
	}
	kunciTak := " AND kd_satker_str IS NOT NULL AND LTRIM(RTRIM(kd_satker_str)) <> ''"
	if err := jalankan("RUP", `SELECT `+kunciInaprocSQL+`, MAX(nama_satker), COUNT_BIG(*), ISNULL(SUM(`+nilaiPagu+`), 0)
		FROM inaproc_paket_penyedia WHERE kd_klpd=@p1 AND tahun_anggaran=@p2 AND `+aktifRUP+kunciTak+` GROUP BY `+kunciInaprocSQL,
		func(p *pengadaanSatker, _ string, n int64, nilai float64) { p.RUPPaket, p.RUPPagu = n, nilai }); err != nil {
		return nil, err
	}
	if err := jalankan("tender", cteTender+`SELECT `+kunciInaprocSQL+`, MAX(nama_satker), COUNT_BIG(*), ISNULL(SUM(`+nilaiPagu+`), 0)
		FROM tp WHERE 1=1`+kunciTak+` GROUP BY `+kunciInaprocSQL,
		func(p *pengadaanSatker, _ string, n int64, nilai float64) { p.Tender, p.TenderPagu = n, nilai }); err != nil {
		return nil, err
	}
	if err := jalankan("non-tender", cteNonTender+`SELECT `+kunciInaprocSQL+`, MAX(nama_satker), COUNT_BIG(*), ISNULL(SUM(`+nilaiPagu+`), 0)
		FROM np WHERE 1=1`+kunciTak+` GROUP BY `+kunciInaprocSQL,
		func(p *pengadaanSatker, _ string, n int64, nilai float64) { p.NonTender, p.NonPagu = n, nilai }); err != nil {
		return nil, err
	}
	return hasil, nil
}

// batasiPengadaan membatasi data pengadaan ke cakupan peran. Data Inaproc hanya punya kode satker 6 digit (tanpa UE1 atau Kanwil), jadi yang dibolehkan
// bagi peran UE1/Kanwil adalah satker yang dikenal pada data aset dalam cakupannya (aset sudah dibatasi lebih dulu); peran Satker hanya satkernya sendiri,
// walau belum punya data aset. Cakupan kosong tidak mendapat apa pun.
func batasiPengadaan(peng map[string]*pengadaanSatker, aset map[string]*asetSatker, cak peran.Cakupan) map[string]*pengadaanSatker {
	if cak.SemuaData() {
		return peng
	}
	out := map[string]*pengadaanSatker{}
	for k, p := range peng {
		switch {
		case cak.Tingkat == peran.TingkatSatk:
			if cak.BolehSatker6(k) {
				out[k] = p
			}
		case cak.Tingkat == peran.TingkatUE1 || cak.Tingkat == peran.TingkatKwl:
			if _, ada := aset[k]; ada {
				out[k] = p
			}
		}
	}
	return out
}

// GetSatkerKeterhubungan: GET /satker/keterhubungan?tahun=&kode_klpd=. Daftar satker beserta jumlah aset dan pengadaannya, dihubungkan lewat kode
// satker 6 digit. Bawaan: KLPD K10 dan tahun terbaru yang punya data pengadaan. Semua pengguna login.
func GetSatkerKeterhubungan(c *gin.Context) {
	klpd := strings.TrimSpace(c.DefaultQuery("kode_klpd", kemenkeuKLPDCode))
	if klpd == "" || len(klpd) > 20 {
		utils.ErrorResponse(c, http.StatusBadRequest, "Kode KLPD tidak valid")
		return
	}
	tahun := strings.TrimSpace(c.Query("tahun"))
	if tahun != "" && !reTahunAnalitik.MatchString(tahun) {
		utils.ErrorResponse(c, http.StatusBadRequest, "Tahun harus berupa empat digit")
		return
	}
	ctx, cancel := dgKonteks(c, satkerTimeout)
	defer cancel()

	tersedia := tahunTersediaAnalitik(ctx, database.DB, klpd)
	if tahun == "" {
		if len(tersedia) > 0 {
			tahun = tersedia[0]
		} else {
			tahun = fmt.Sprint(time.Now().In(zonaWIB).Year())
		}
	}

	aset, tanpaKode, err := bacaSatkerAset(ctx)
	if err != nil {
		log.Println("[SATKER ERROR]", err)
		utils.ErrorResponse(c, http.StatusInternalServerError, "Gagal membaca data aset satker")
		return
	}
	peng, err := bacaSatkerPengadaan(ctx, klpd, tahun)
	if err != nil {
		log.Println("[SATKER ERROR]", err)
		utils.ErrorResponse(c, http.StatusInternalServerError, "Gagal membaca data pengadaan satker")
		return
	}
	ref, err := muatRefUE1(ctx)
	if err != nil {
		log.Println("[SATKER WARN] gagal membaca referensi UE1:", err)
		ref = map[string]RefUE1Baris{}
	}
	peng = batasiPengadaan(peng, aset, peran.CakupanDari(ctx))
	satker, ringkasan := gabungSatker(aset, peng, ref)
	ringkasan.AsetTanpaKode = tanpaKode
	utils.SuccessResponse(c, http.StatusOK, "Berhasil mengambil keterhubungan satker", gin.H{
		"tahun": tahun, "kode_klpd": klpd, "tahun_tersedia": tersedia, "ringkasan": ringkasan, "satker": satker,
	})
}
