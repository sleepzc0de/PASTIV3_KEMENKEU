package apiakses

import (
	"errors"
	"fmt"
	"strconv"
	"time"
)

// Pengaturan: bawaan global API Akses Data. Disimpan di tabel api_akses_pengaturan (migrasi 060) dan diubah superadmin dari aplikasi; perubahan berlaku dalam
// hitungan detik tanpa deploy ulang. Nilai di bawah hanya cadangan bila tabel kosong atau belum dimigrasi.
type Pengaturan struct {
	LimitPermintaan    int  `json:"limit_permintaan"`        // permintaan per jendela laju
	LimitJendelaDetik  int  `json:"limit_jendela_detik"`     // panjang jendela laju (jendela geser)
	KuotaPermintaan    int  `json:"kuota_permintaan"`        // permintaan per periode kuota
	KuotaPeriodeDetik  int  `json:"kuota_periode_detik"`     // panjang periode kuota (jendela tetap, selaras kelipatan periode sejak epoch UTC)
	PendaftaranDibuka  bool `json:"pendaftaran_dibuka"`      // formulir pendaftaran publik menerima pendaftaran baru
	KunciBerlakuHari   int  `json:"kunci_masa_berlaku_hari"` // masa berlaku kunci baru (0 = tidak kedaluwarsa)
	RotasiTenggangJam  int  `json:"rotasi_tenggang_jam"`     // kunci lama tetap berlaku sekian jam setelah rotasi (0 = langsung dicabut)
	HalamanMaks        int  `json:"halaman_maks"`            // baris maksimal per permintaan daftar
	LogRetensiHari     int  `json:"log_retensi_hari"`        // riwayat permintaan dihapus setelah sekian hari
	AgregatRetensiHari int  `json:"agregat_retensi_hari"`    // hitungan per jam dihapus setelah sekian hari
}

// Batas nilai yang diterima. Batas atas bukan tebakan: LimitMaks menentukan memori pembatas (satu stempel waktu 8 byte per permintaan dalam jendela).
const (
	LimitMin, LimitMaks          = 1, 100_000
	JendelaMin, JendelaMaks      = 1, 3600
	KuotaMin, KuotaMaks          = 1, 100_000_000
	PeriodeMin, PeriodeMaks      = 60, 31 * 24 * 3600
	HalamanMin, HalamanMaksBatas = 10, 1000
	MasaBerlakuMaksHari          = 3650
	TenggangMaksJam              = 24 * 30
	LogRetensiMaksHari           = 365
	AgregatRetensiMaksHari       = 3650
)

// PengaturanBawaan: nilai pabrik (sama dengan yang diisi migrasi).
func PengaturanBawaan() Pengaturan {
	return Pengaturan{
		LimitPermintaan: 1000, LimitJendelaDetik: 60, KuotaPermintaan: 5000, KuotaPeriodeDetik: 3600,
		PendaftaranDibuka: true, KunciBerlakuHari: 0, RotasiTenggangJam: 24, HalamanMaks: 500, LogRetensiHari: 14, AgregatRetensiHari: 400,
	}
}

func dalam(n, min, maks int) bool { return n >= min && n <= maks }

// Validasi memeriksa semua nilai; pesan menyebut kolom yang salah.
func (p Pengaturan) Validasi() error {
	switch {
	case !dalam(p.LimitPermintaan, LimitMin, LimitMaks):
		return fmt.Errorf("Batas permintaan harus antara %d dan %d", LimitMin, LimitMaks)
	case !dalam(p.LimitJendelaDetik, JendelaMin, JendelaMaks):
		return fmt.Errorf("Jendela batas laju harus antara %d dan %d detik", JendelaMin, JendelaMaks)
	case !dalam(p.KuotaPermintaan, KuotaMin, KuotaMaks):
		return fmt.Errorf("Kuota harus antara %d dan %d permintaan", KuotaMin, KuotaMaks)
	case !dalam(p.KuotaPeriodeDetik, PeriodeMin, PeriodeMaks):
		return fmt.Errorf("Periode kuota harus antara %d dan %d detik", PeriodeMin, PeriodeMaks)
	case !dalam(p.HalamanMaks, HalamanMin, HalamanMaksBatas):
		return fmt.Errorf("Baris maksimal per permintaan harus antara %d dan %d", HalamanMin, HalamanMaksBatas)
	case !dalam(p.KunciBerlakuHari, 0, MasaBerlakuMaksHari):
		return fmt.Errorf("Masa berlaku kunci harus antara 0 dan %d hari", MasaBerlakuMaksHari)
	case !dalam(p.RotasiTenggangJam, 0, TenggangMaksJam):
		return fmt.Errorf("Masa tenggang rotasi harus antara 0 dan %d jam", TenggangMaksJam)
	case !dalam(p.LogRetensiHari, 1, LogRetensiMaksHari):
		return fmt.Errorf("Masa simpan riwayat permintaan harus antara 1 dan %d hari", LogRetensiMaksHari)
	case !dalam(p.AgregatRetensiHari, 1, AgregatRetensiMaksHari):
		return fmt.Errorf("Masa simpan hitungan per jam harus antara 1 dan %d hari", AgregatRetensiMaksHari)
	}
	return nil
}

// Daftar kunci baris api_akses_pengaturan.
const (
	kLimit       = "limit_permintaan"
	kJendela     = "limit_jendela_detik"
	kKuota       = "kuota_permintaan"
	kPeriode     = "kuota_periode_detik"
	kDaftar      = "pendaftaran_dibuka"
	kMasaBerlaku = "kunci_masa_berlaku_hari"
	kTenggang    = "rotasi_tenggang_jam"
	kHalaman     = "halaman_maks"
	kLogHari     = "log_retensi_hari"
	kAgregatHari = "agregat_retensi_hari"
)

// KeBaris mengubah pengaturan menjadi pasangan kunci -> nilai untuk disimpan.
func (p Pengaturan) KeBaris() map[string]string {
	b := "0"
	if p.PendaftaranDibuka {
		b = "1"
	}
	return map[string]string{
		kLimit: strconv.Itoa(p.LimitPermintaan), kJendela: strconv.Itoa(p.LimitJendelaDetik), kKuota: strconv.Itoa(p.KuotaPermintaan), kPeriode: strconv.Itoa(p.KuotaPeriodeDetik),
		kDaftar: b, kMasaBerlaku: strconv.Itoa(p.KunciBerlakuHari), kTenggang: strconv.Itoa(p.RotasiTenggangJam), kHalaman: strconv.Itoa(p.HalamanMaks),
		kLogHari: strconv.Itoa(p.LogRetensiHari), kAgregatHari: strconv.Itoa(p.AgregatRetensiHari),
	}
}

// DariBaris membaca pengaturan dari pasangan kunci -> nilai. Kunci yang hilang atau nilai yang tidak sah memakai nilai pabrik, sehingga satu baris rusak tidak
// mematikan API; hasil akhirnya tetap harus lolos Validasi (bila tidak, seluruhnya kembali ke nilai pabrik).
func DariBaris(baris map[string]string) Pengaturan {
	p := PengaturanBawaan()
	bil := func(k string, tujuan *int) {
		if v, ok := baris[k]; ok {
			if n, err := strconv.Atoi(v); err == nil {
				*tujuan = n
			}
		}
	}
	bil(kLimit, &p.LimitPermintaan)
	bil(kJendela, &p.LimitJendelaDetik)
	bil(kKuota, &p.KuotaPermintaan)
	bil(kPeriode, &p.KuotaPeriodeDetik)
	bil(kMasaBerlaku, &p.KunciBerlakuHari)
	bil(kTenggang, &p.RotasiTenggangJam)
	bil(kHalaman, &p.HalamanMaks)
	bil(kLogHari, &p.LogRetensiHari)
	bil(kAgregatHari, &p.AgregatRetensiHari)
	if v, ok := baris[kDaftar]; ok {
		p.PendaftaranDibuka = v == "1" || v == "true"
	}
	if p.Validasi() != nil {
		return PengaturanBawaan()
	}
	return p
}

// BatasEfektif: batas yang berlaku bagi satu klien (override klien atau bawaan global).
type BatasEfektif struct {
	LimitPermintaan   int `json:"limit_permintaan"`
	LimitJendelaDetik int `json:"limit_jendela_detik"`
	KuotaPermintaan   int `json:"kuota_permintaan"`
	KuotaPeriodeDetik int `json:"kuota_periode_detik"`
}

// Override: nilai khusus klien; nil = ikut bawaan global.
type Override struct {
	LimitPermintaan   *int `json:"limit_permintaan"`
	LimitJendelaDetik *int `json:"limit_jendela_detik"`
	KuotaPermintaan   *int `json:"kuota_permintaan"`
	KuotaPeriodeDetik *int `json:"kuota_periode_detik"`
}

// Validasi memeriksa override yang terisi dengan batas yang sama seperti bawaan global.
func (o Override) Validasi() error {
	cek := func(v *int, min, maks int, nama string) error {
		if v != nil && !dalam(*v, min, maks) {
			return fmt.Errorf("%s harus antara %d dan %d", nama, min, maks)
		}
		return nil
	}
	for _, e := range []error{
		cek(o.LimitPermintaan, LimitMin, LimitMaks, "Batas permintaan"),
		cek(o.LimitJendelaDetik, JendelaMin, JendelaMaks, "Jendela batas laju (detik)"),
		cek(o.KuotaPermintaan, KuotaMin, KuotaMaks, "Kuota"),
		cek(o.KuotaPeriodeDetik, PeriodeMin, PeriodeMaks, "Periode kuota (detik)"),
	} {
		if e != nil {
			return e
		}
	}
	return nil
}

// BatasUntuk menggabungkan bawaan global dengan override klien.
func (p Pengaturan) BatasUntuk(o Override) BatasEfektif {
	b := BatasEfektif{p.LimitPermintaan, p.LimitJendelaDetik, p.KuotaPermintaan, p.KuotaPeriodeDetik}
	if o.LimitPermintaan != nil {
		b.LimitPermintaan = *o.LimitPermintaan
	}
	if o.LimitJendelaDetik != nil {
		b.LimitJendelaDetik = *o.LimitJendelaDetik
	}
	if o.KuotaPermintaan != nil {
		b.KuotaPermintaan = *o.KuotaPermintaan
	}
	if o.KuotaPeriodeDetik != nil {
		b.KuotaPeriodeDetik = *o.KuotaPeriodeDetik
	}
	return b
}

// KeBatas mengubah batas efektif menjadi bentuk yang dipakai pembatas.
func (b BatasEfektif) KeBatas() Batas {
	return Batas{
		LimitPermintaan: b.LimitPermintaan, LimitJendela: time.Duration(b.LimitJendelaDetik) * time.Second,
		Kuota: b.KuotaPermintaan, KuotaPeriode: time.Duration(b.KuotaPeriodeDetik) * time.Second,
	}
}

var errPengaturan = errors.New("pengaturan API akses tidak sah")
