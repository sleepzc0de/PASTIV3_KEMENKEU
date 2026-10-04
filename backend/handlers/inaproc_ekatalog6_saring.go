package handlers

import (
	"errors"
	"io"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// Aturan penyaring dua endpoint E-Katalog V6 yang bukan bentuk bawaan endpointDatar: list-kategori-produk dan
// e-purchasing-by-produk. Nilai penyaring hanya pernah menjadi nilai parameter ke Inaproc atau argumen SQL (tidak pernah
// disambung ke teks SQL), dan panjangnya dibatasi.

func tidakPanjang(nilai ...string) bool {
	for _, n := range nilai {
		if len(n) > panjangKodeMaks {
			return false
		}
	}
	return true
}

func bacaBadanJSON(c *gin.Context, tujuan interface{}) bool {
	// Badan kosong dibolehkan (semua isian opsional); JSON rusak ditolak.
	if err := c.ShouldBindJSON(tujuan); err != nil && !errors.Is(err, io.EOF) {
		return false
	}
	return true
}

// ============================================================
// list-kategori-produk
// ============================================================

// kategoriSaring: kd_kategori_2 hanya boleh bersama kd_kategori_1. Tingkat = 1 (tanpa filter), 2 (kd_kategori_1), atau 3 (keduanya).
type kategoriSaring struct{ Kd1, Kd2 string }

func (k kategoriSaring) validasi() string {
	switch {
	case k.Kd2 != "" && k.Kd1 == "":
		return "kd_kategori_2 harus disertai kd_kategori_1"
	case !tidakPanjang(k.Kd1, k.Kd2):
		return "kd_kategori terlalu panjang"
	}
	return ""
}

func (k kategoriSaring) tingkat() int {
	switch {
	case k.Kd2 != "":
		return 3
	case k.Kd1 != "":
		return 2
	}
	return 1
}

func (k kategoriSaring) params() url.Values {
	p := url.Values{}
	if k.Kd1 != "" {
		p.Set("kd_kategori_1", k.Kd1)
	}
	if k.Kd2 != "" {
		p.Set("kd_kategori_2", k.Kd2)
	}
	return p
}

// kondisi: klausa tingkat + kode induk (argumen @p1, @p2, @p3 berurutan).
func (k kategoriSaring) kondisi() (string, []interface{}) {
	sql := "tingkat = @p1"
	args := []interface{}{k.tingkat()}
	if k.Kd1 != "" {
		args = append(args, k.Kd1)
		sql += " AND kd_kategori_1 = @p" + strconv.Itoa(len(args))
	}
	if k.Kd2 != "" {
		args = append(args, k.Kd2)
		sql += " AND kd_kategori_2 = @p" + strconv.Itoa(len(args))
	}
	return sql, args
}

func kategoriDariQuery(c *gin.Context) kategoriSaring {
	return kategoriSaring{Kd1: strings.TrimSpace(c.Query("kd_kategori_1")), Kd2: strings.TrimSpace(c.Query("kd_kategori_2"))}
}

func ekatalog6KategoriParam(c *gin.Context) (url.Values, string) {
	k := kategoriDariQuery(c)
	if pesan := k.validasi(); pesan != "" {
		return nil, pesan
	}
	return k.params(), ""
}

func ekatalog6KategoriRencana(c *gin.Context) (*rencanaSync, string) {
	var req struct {
		Kd1 string `json:"kd_kategori_1"`
		Kd2 string `json:"kd_kategori_2"`
	}
	if !bacaBadanJSON(c, &req) {
		return nil, "Permintaan tidak valid"
	}
	return ekatalog6KategoriRencanaDari(req.Kd1, req.Kd2)
}

// ekatalog6KategoriRencanaDari sama dengan ekatalog6KategoriRencana, tetapi dari nilai yang sudah terbaca (penarikan terjadwal/antrean).
func ekatalog6KategoriRencanaDari(kd1, kd2 string) (*rencanaSync, string) {
	k := kategoriSaring{Kd1: strings.TrimSpace(kd1), Kd2: strings.TrimSpace(kd2)}
	if pesan := k.validasi(); pesan != "" {
		return nil, pesan
	}

	kondisi, args := k.kondisi()
	jenis := "L1"
	switch k.tingkat() {
	case 2:
		jenis = "L2:" + k.Kd1
	case 3:
		jenis = "L3:" + k.Kd1 + "/" + k.Kd2
	}
	return &rencanaSync{
		Params:    k.params(),
		HapusSQL:  "DELETE FROM " + ekatalog6Kategori.Tabel + " WHERE " + kondisi,
		HapusArgs: args,
		Konteks:   map[string]string{"tingkat": strconv.Itoa(k.tingkat()), "kd_kategori_1": k.Kd1, "kd_kategori_2": k.Kd2},
		LogJenis:  potong(jenis, 50),
	}, ""
}

func ekatalog6KategoriWhere(c *gin.Context) (string, []interface{}) {
	kondisi, args := kategoriDariQuery(c).kondisi()
	return " WHERE " + kondisi, args
}

// ============================================================
// e-purchasing-by-produk
// ============================================================

// Status transaksi yang didukung Inaproc (tidak peka huruf besar/kecil); bawaan COMPLETED.
var statusTransaksiSah = map[string]bool{
	"CANCELLED": true, "CANCELLED_ON_NEGOTIATION": true, "CANCELLED_ON_REVIEW": true, "COMPLETED": true, "ESIGN_IN_PROGRESS": true,
	"ON_ADDENDUM": true, "ON_NEGOTIATION": true, "ON_PROCESS": true, "PAYMENT_OUTSIDE_SYSTEM": true,
	"REQUEST_CANCEL_BY_ADMIN": true, "WAITING_PPK_REVIEW": true, "WAITING_SELLER_CONFIRMATION": true,
}

const statusTransaksiBawaan = "COMPLETED"

var reTahun = regexp.MustCompile(`^[1-9][0-9]{0,3}$`)

// transaksiSaring: tahun wajib; kode_klpd dan kd_kategori_1 minimal salah satu (tanpa keduanya dipakai kode KLPD bawaan K10);
// kd_product dan status opsional. Status selalu dikirim eksplisit (bawaan COMPLETED) supaya yang diminta dan yang disimpan jelas.
type transaksiSaring struct{ Tahun, KodeKLPD, Kd1, KdProduk, Status string }

func (t *transaksiSaring) rapikan() {
	t.Tahun, t.KodeKLPD = strings.TrimSpace(t.Tahun), strings.TrimSpace(t.KodeKLPD)
	t.Kd1, t.KdProduk = strings.TrimSpace(t.Kd1), strings.TrimSpace(t.KdProduk)
	t.Status = strings.ToUpper(strings.TrimSpace(t.Status))
	if t.Status == "" {
		t.Status = statusTransaksiBawaan
	}
	if t.KodeKLPD == "" && t.Kd1 == "" {
		t.KodeKLPD = kemenkeuKLPDCodeTender
	}
}

func (t transaksiSaring) validasi() string {
	switch {
	case t.Tahun == "":
		return "Parameter 'tahun' wajib diisi"
	case !reTahun.MatchString(t.Tahun):
		return "Parameter 'tahun' harus berupa tahun (angka)"
	case !statusTransaksiSah[t.Status]:
		return "Parameter 'status' tidak dikenal"
	case !tidakPanjang(t.KodeKLPD, t.Kd1, t.KdProduk):
		return "Parameter terlalu panjang"
	}
	return ""
}

func (t transaksiSaring) params() url.Values {
	p := url.Values{}
	p.Set("tahun", t.Tahun)
	p.Set("status", t.Status)
	if t.KodeKLPD != "" {
		p.Set("kode_klpd", t.KodeKLPD)
	}
	if t.Kd1 != "" {
		p.Set("kd_kategori_1", t.Kd1)
	}
	if t.KdProduk != "" {
		p.Set("kd_product", t.KdProduk)
	}
	return p
}

// kondisi: klausa penyaring menurut isian yang terisi (argumen @p1... berurutan). Kolom kd_product di API ada pada kolom product_id.
func (t transaksiSaring) kondisi() (string, []interface{}) {
	var bagian []string
	var args []interface{}
	tambah := func(kolom, nilai string) {
		if nilai == "" {
			return
		}
		args = append(args, nilai)
		bagian = append(bagian, kolom+" = @p"+strconv.Itoa(len(args)))
	}
	tambah("tahun", t.Tahun)
	tambah("status", t.Status)
	tambah("kode_klpd", t.KodeKLPD)
	tambah("kd_kategori_1", t.Kd1)
	tambah("product_id", t.KdProduk)
	return strings.Join(bagian, " AND "), args
}

func transaksiDariQuery(c *gin.Context) transaksiSaring {
	return transaksiSaring{
		Tahun: c.Query("tahun"), KodeKLPD: c.Query("kode_klpd"), Kd1: c.Query("kd_kategori_1"),
		KdProduk: c.Query("kd_product"), Status: c.Query("status"),
	}
}

func ekatalog6TransaksiParam(c *gin.Context) (url.Values, string) {
	t := transaksiDariQuery(c)
	t.rapikan()
	if pesan := t.validasi(); pesan != "" {
		return nil, pesan
	}
	return t.params(), ""
}

func ekatalog6TransaksiRencana(c *gin.Context) (*rencanaSync, string) {
	var req struct {
		Tahun    string `json:"tahun"`
		KodeKLPD string `json:"kode_klpd"`
		Kd1      string `json:"kd_kategori_1"`
		KdProduk string `json:"kd_product"`
		Status   string `json:"status"`
	}
	if !bacaBadanJSON(c, &req) {
		return nil, "Permintaan tidak valid"
	}
	return ekatalog6TransaksiRencanaDari(transaksiSaring{Tahun: req.Tahun, KodeKLPD: req.KodeKLPD, Kd1: req.Kd1, KdProduk: req.KdProduk, Status: req.Status})
}

// ekatalog6TransaksiRencanaDari sama dengan ekatalog6TransaksiRencana, tetapi dari nilai yang sudah terbaca (penarikan terjadwal/antrean).
func ekatalog6TransaksiRencanaDari(t transaksiSaring) (*rencanaSync, string) {
	t.rapikan()
	if pesan := t.validasi(); pesan != "" {
		return nil, pesan
	}

	kondisi, args := t.kondisi()
	return &rencanaSync{
		Params:    t.params(),
		HapusSQL:  "DELETE FROM " + ekatalog6Transaksi.Tabel + " WHERE " + kondisi,
		HapusArgs: args,
		Konteks:   map[string]string{"tahun": t.Tahun, "kode_klpd": t.KodeKLPD},
		LogKLPD:   t.KodeKLPD,
		LogTahun:  t.Tahun,
		LogJenis:  potong(t.Status, 50),
	}, ""
}

// Daftar lokal: hanya isian yang diisi yang menyaring (kode KLPD bawaan K10 bila kd_kategori_1 juga kosong, seperti daftar lokal
// endpoint lain); status tidak diberi bawaan.
func ekatalog6TransaksiWhere(c *gin.Context) (string, []interface{}) {
	t := transaksiDariQuery(c)
	t.Tahun = strings.TrimSpace(t.Tahun)
	t.Kd1, t.KdProduk = strings.TrimSpace(t.Kd1), strings.TrimSpace(t.KdProduk)
	t.Status = strings.ToUpper(strings.TrimSpace(t.Status))
	t.KodeKLPD = strings.TrimSpace(t.KodeKLPD)
	if t.KodeKLPD == "" && t.Kd1 == "" {
		t.KodeKLPD = kemenkeuKLPDCodeTender
	}
	kondisi, args := t.kondisi()
	if kondisi == "" {
		return "", nil
	}
	return " WHERE " + kondisi, args
}
