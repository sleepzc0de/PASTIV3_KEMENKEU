package apiakses

import (
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"pasti-v3-backend/audit"
)

// AwalanRute: awalan rute API klien eksternal. Sengaja di luar /api/v1 (API aplikasi untuk pengguna yang login): kedua kelompok tidak berbagi middleware autentikasi,
// jadi token pengguna tidak membuka rute klien dan kunci klien tidak membuka rute pengguna.
const AwalanRute = "/api/data/v1"

// Kode galat yang dibaca mesin pada respons klien (field "code").
const (
	KodeKunciTidakValid = "kunci_tidak_valid"
	KodeAlamatDitolak   = "alamat_ditolak"
	KodeBatasLaju       = "batas_laju_terlampaui"
	KodeKuotaHabis      = "kuota_habis"
	KodeTerlaluBanyak   = "terlalu_banyak_percobaan"
	KodeDatasetDitolak  = "dataset_ditolak"
	KodeLayananGagal    = "layanan_tidak_tersedia"
)

const (
	kunciIdentitas = "apiakses_identitas"
	kunciDataset   = "apiakses_dataset"
	kunciBaris     = "apiakses_baris"
	kunciHasil     = "apiakses_hasil"
	namaPengguna   = "api:" // awalan username pada Log Audit untuk tindakan klien
)

// Identitas: klien yang lolos gerbang untuk permintaan ini.
type Identitas struct {
	KlienID   int64
	KlienKode string
	KlienNama string
	KunciID   int64
	Prefix    string
	Dataset   map[string]bool
	IP        []string
	UE1       []string
	Override  Override
	Batas     BatasEfektif
	Terakhir  Keputusan // keadaan batas laju dan kuota sesudah permintaan ini dihitung
}

// BolehDataset: apakah dataset termasuk scope klien.
func (i *Identitas) BolehDataset(kode string) bool { return i != nil && i.Dataset[kode] }

// DaftarDataset: dataset dalam scope, urut menurut katalog.
func (i *Identitas) DaftarDataset() []string {
	var out []string
	for _, d := range SemuaDataset() {
		if i.Dataset[d.Kode] {
			out = append(out, d.Kode)
		}
	}
	return out
}

// IdentitasDari membaca identitas klien yang dipasang gerbang (nil bila rute ini tidak dilindungi gerbang).
func IdentitasDari(c *gin.Context) *Identitas {
	if v, ok := c.Get(kunciIdentitas); ok {
		i, _ := v.(*Identitas)
		return i
	}
	return nil
}

// CatatDataset dan CatatBaris dipanggil penanganan agar riwayat memuat dataset yang diakses dan jumlah baris yang dikembalikan.
func CatatDataset(c *gin.Context, kode string) { c.Set(kunciDataset, kode) }
func CatatBaris(c *gin.Context, n int)         { c.Set(kunciBaris, n) }

// TolakScope menjawab 403 untuk dataset di luar scope klien dan mencatatnya (riwayat dan Log Audit).
func TolakScope(c *gin.Context, kode string) {
	c.Set(kunciHasil, HasilDitolakScope)
	if i := IdentitasDari(c); i != nil {
		tandaiAudit(c, "apiakses.akses.ditolak_scope", "Klien API meminta dataset di luar scope", i.KlienKode, i.Prefix, "dataset_di_luar_scope", map[string]interface{}{"dataset": potong(kode, 60)})
	}
	c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"success": false, "message": "Kunci Anda tidak memiliki akses ke dataset ini", "code": KodeDatasetDitolak})
}

// tandaiAudit menambahkan keterangan Log Audit pada permintaan klien yang ditolak. Tidak memuat kunci, hash, maupun isi permintaan.
func tandaiAudit(c *gin.Context, aksi, label, klienKode, prefix, alasan string, tambahan map[string]interface{}) {
	gagal := false
	detail := map[string]interface{}{"alasan": alasan}
	if prefix != "" {
		detail["kunci_prefix"] = prefix
	}
	for k, v := range tambahan {
		detail[k] = v
	}
	t := audit.Tanda{Kategori: audit.KatAPIAkses, Aksi: aksi, Label: label, Sukses: &gagal, Detail: detail}
	if klienKode != "" {
		t.Username, t.ObjekTipe, t.ObjekID = namaPengguna+klienKode, "klien_api", klienKode
	}
	audit.Tandai(c, t)
}

// penolakan: alasan sebuah permintaan tidak diteruskan.
type penolakan struct {
	Status  int
	Kode    string
	Pesan   string
	Hasil   string
	Tunggu  int // Retry-After (detik); 0 = tanpa
	Cred    *Kredensial
	Tampung bool // header batas laju/kuota sudah dipasang
}

// Gerbang adalah middleware untuk rute API klien: autentikasi kunci, status klien, allowlist IP, batas laju, dan kuota, lalu mencatat pemakaian. pakaiKuota=false
// untuk rute yang tidak menghabiskan kuota (melihat profil dan pemakaian sendiri); batas lajunya tetap berlaku.
//
// Semua kegagalan autentikasi dijawab sama (401, tanpa membedakan kunci tidak dikenal, salah, dicabut, kedaluwarsa, atau klien tidak aktif) supaya jawaban tidak
// dapat dipakai menebak kunci atau status klien; alasan sebenarnya hanya masuk riwayat dan Log Audit.
func (l *Layanan) Gerbang(pakaiKuota bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		mulai := l.sekarang()
		ip := c.ClientIP()
		u := Penggunaan{
			Waktu: mulai.UTC(), Metode: c.Request.Method, Endpoint: strings.TrimPrefix(c.FullPath(), AwalanRute), IP: ip,
			UserAgent: potong(c.Request.UserAgent(), 200), RequestID: audit.RequestID(c),
		}
		c.Header("Cache-Control", "no-store")

		id, tolak := l.periksa(c, ip, mulai, pakaiKuota)
		if tolak != nil {
			l.jawabTolak(c, tolak, id)
		} else {
			c.Set(kunciIdentitas, id)
			c.Next()
		}
		l.catat(c, u, id, tolak, mulai)
	}
}

// periksa menilai permintaan. Mengembalikan identitas bila lolos; bila ditolak, penolakannya (identitas bisa tetap terisi untuk riwayat).
func (l *Layanan) periksa(c *gin.Context, ip string, now time.Time, pakaiKuota bool) (*Identitas, *penolakan) {
	ctx := c.Request.Context()
	terkunci, tunggu := l.Gagal.Terkunci(ip)

	gagalAuth := func(alasan string, cred *Kredensial) *penolakan {
		l.Gagal.Gagal(ip)
		kode, prefix := "", ""
		if cred != nil {
			kode, prefix = cred.KlienKode, cred.Prefix
		}
		// Alamat yang sudah terkunci tidak dicatat lagi di Log Audit: entri tanpa pengguna berbagi batas laju tulis dengan login gagal, dan banjir percobaan dari satu
		// alamat tidak boleh menyisihkan catatan yang lain. Yang tercatat adalah kegagalan sampai ambang kunci tercapai.
		if !terkunci {
			tandaiAudit(c, "apiakses.akses.tidak_sah", "Percobaan akses API dengan kunci yang tidak sah", kode, prefix, alasan, nil)
		}
		if terkunci {
			return &penolakan{Status: http.StatusTooManyRequests, Kode: KodeTerlaluBanyak, Pesan: "Terlalu banyak percobaan dengan kunci yang tidak valid dari alamat ini. Coba lagi nanti", Hasil: HasilTidakSah, Tunggu: detikNaik(tunggu), Cred: cred}
		}
		return &penolakan{Status: http.StatusUnauthorized, Kode: KodeKunciTidakValid, Pesan: "Kunci API tidak valid atau tidak aktif", Hasil: HasilTidakSah, Cred: cred}
	}

	kunci, ok := KunciDariHeader(c.Request.Header)
	if !ok {
		return nil, gagalAuth("kunci_tidak_ada_atau_salah_bentuk", nil)
	}
	prefix, ok := PrefixKunci(kunci)
	if !ok {
		return nil, gagalAuth("kunci_salah_bentuk", nil)
	}
	hash := l.HashKunci(kunci) // dihitung selalu, juga bila kunci tidak dikenal, agar waktu jawaban tidak membedakan
	cred, err := l.cari(ctx, prefix, !terkunci)
	if err != nil {
		if ctx.Err() == nil {
			serverlogGalat("pencarian kredensial", err)
		}
		return nil, &penolakan{Status: http.StatusServiceUnavailable, Kode: KodeLayananGagal, Pesan: "Layanan sedang tidak tersedia. Coba lagi sebentar lagi", Hasil: HasilGalatServer, Tunggu: 5}
	}
	if cred == nil {
		SamaHash(hash, l.hashDummy)
		return nil, gagalAuth("kunci_tidak_dikenal", nil)
	}
	if !SamaHash(hash, cred.Hash) {
		return nil, gagalAuth("rahasia_salah", cred)
	}
	switch {
	case cred.DicabutAt != nil:
		return nil, gagalAuth("kunci_dicabut", cred)
	case cred.KedaluwarsaAt != nil && !now.Before(*cred.KedaluwarsaAt):
		return nil, gagalAuth("kunci_kedaluwarsa", cred)
	case cred.KlienStatus != StatusDisetujui:
		return nil, gagalAuth("klien_"+cred.KlienStatus, cred)
	}

	batas := l.Pengaturan(ctx).BatasUntuk(cred.Override)
	id := &Identitas{
		KlienID: cred.KlienID, KlienKode: cred.KlienKode, KlienNama: cred.KlienNama, KunciID: cred.KunciID, Prefix: cred.Prefix,
		IP: cred.IP, UE1: cred.UE1, Override: cred.Override, Batas: batas, Dataset: make(map[string]bool, len(cred.Dataset)),
	}
	for _, d := range cred.Dataset {
		id.Dataset[d] = true
	}
	if !IPDiizinkan(cred.IP, ip) {
		// Kunci yang sah dipakai dari alamat di luar allowlist: indikasi kunci bocor. Dihitung sebagai kegagalan dan dicatat.
		l.Gagal.Gagal(ip)
		if !terkunci {
			tandaiAudit(c, "apiakses.akses.ditolak_ip", "Kunci API dipakai dari alamat di luar allowlist", cred.KlienKode, cred.Prefix, "alamat_di_luar_allowlist", nil)
		}
		return id, &penolakan{Status: http.StatusForbidden, Kode: KodeAlamatDitolak, Pesan: "Akses dari alamat ini tidak diizinkan", Hasil: HasilDitolakIP, Cred: cred}
	}

	d := l.Pembatas.Coba(cred.KlienID, batas.KeBatas(), pakaiKuota)
	id.Terakhir = d
	l.catatDipakai(cred.KunciID, ip, now)
	pasangHeaderBatas(c, d)
	if !d.Izin {
		kode, pesan, hasil := KodeBatasLaju, "Batas permintaan terlampaui. Coba lagi setelah waktu yang ditunjukkan Retry-After", HasilBatasLaju
		if d.Alasan == AlasanKuota {
			kode, pesan, hasil = KodeKuotaHabis, "Kuota permintaan habis. Kuota pulih pada waktu yang ditunjukkan X-Quota-Reset", HasilKuotaHabis
		}
		return id, &penolakan{Status: http.StatusTooManyRequests, Kode: kode, Pesan: pesan, Hasil: hasil, Tunggu: d.TungguDetik, Cred: cred, Tampung: true}
	}
	return id, nil
}

// pasangHeaderBatas menulis header batas laju dan kuota. Reset dalam detik Unix (UTC).
func pasangHeaderBatas(c *gin.Context, d Keputusan) {
	h := c.Writer.Header()
	h.Set("X-RateLimit-Limit", strconv.Itoa(d.Batas.LimitPermintaan))
	h.Set("X-RateLimit-Remaining", strconv.Itoa(d.LimitSisa))
	h.Set("X-RateLimit-Reset", strconv.FormatInt(d.LimitReset.Unix(), 10))
	h.Set("X-Quota-Limit", strconv.Itoa(d.Batas.Kuota))
	h.Set("X-Quota-Remaining", strconv.Itoa(d.KuotaSisa))
	h.Set("X-Quota-Reset", strconv.FormatInt(d.KuotaReset.Unix(), 10))
}

func (l *Layanan) jawabTolak(c *gin.Context, t *penolakan, id *Identitas) {
	if t.Tunggu > 0 {
		c.Header("Retry-After", strconv.Itoa(t.Tunggu))
	}
	if t.Status == http.StatusUnauthorized {
		c.Header("WWW-Authenticate", `Bearer realm="pasti-api"`)
	}
	body := gin.H{"success": false, "message": t.Pesan, "code": t.Kode}
	if t.Tunggu > 0 {
		body["retry_after_detik"] = t.Tunggu
	}
	c.AbortWithStatusJSON(t.Status, body)
}

// catat mencatat pemakaian setelah permintaan selesai (ditolak maupun dilayani).
func (l *Layanan) catat(c *gin.Context, u Penggunaan, id *Identitas, t *penolakan, mulai time.Time) {
	if l.Pencatat == nil {
		return
	}
	u.Status = c.Writer.Status()
	u.DurasiMS = int(l.sekarang().Sub(mulai) / time.Millisecond)
	switch {
	case t != nil:
		u.Hasil = t.Hasil
	default:
		u.Hasil = HasilDariStatus(u.Status)
		if v, ok := c.Get(kunciHasil); ok {
			if s, ok := v.(string); ok && s != "" {
				u.Hasil = s
			}
		}
	}
	if id != nil {
		u.KlienID, u.KunciPrefix = id.KlienID, id.Prefix
	} else if t != nil && t.Cred != nil {
		u.KlienID, u.KunciPrefix = t.Cred.KlienID, t.Cred.Prefix
	}
	if v, ok := c.Get(kunciDataset); ok {
		u.Dataset, _ = v.(string)
	}
	if v, ok := c.Get(kunciBaris); ok {
		if n, ok := v.(int); ok {
			u.Baris = &n
		}
	}
	l.Pencatat.Catat(u)
}

// serverlogGalat mencatat galat internal gerbang ke log standar (masuk Log Server), tanpa detail kunci.
func serverlogGalat(konteks string, err error) {
	log.Println("[API-AKSES ERROR] "+konteks+":", err)
}
