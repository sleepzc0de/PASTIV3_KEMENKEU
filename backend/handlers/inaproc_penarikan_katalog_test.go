package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

var (
	reBuatTabel  = regexp.MustCompile(`(?is)CREATE\s+TABLE\s+(\w+)\s*\((.*?)\n\s*\)\s*;`)
	reKolomBaris = regexp.MustCompile(`(?m)^\s*(\w+)\s+(?:NVARCHAR|VARCHAR|INT|BIGINT|DECIMAL|DATETIME2|BIT|UNIQUEIDENTIFIER|DATE)\b`)
)

// kolomMigrasi: tabel → kolomnya, dibaca dari semua berkas migrasi.
func kolomMigrasi(t *testing.T) map[string]map[string]bool {
	t.Helper()
	berkas, err := filepath.Glob("../migrations/*.sql")
	if err != nil || len(berkas) == 0 {
		t.Fatalf("migrasi tidak ditemukan: %v", err)
	}
	out := map[string]map[string]bool{}
	for _, f := range berkas {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range reBuatTabel.FindAllStringSubmatch(string(b), -1) {
			kolom := map[string]bool{}
			for _, k := range reKolomBaris.FindAllStringSubmatch(m[2], -1) {
				kolom[k[1]] = true
			}
			out[m[1]] = kolom
		}
	}
	return out
}

func TestKatalogPenarikanJumlahDanKeunikan(t *testing.T) {
	hitung := map[string]int{}
	ids := map[string]bool{}
	for _, d := range DaftarDataset {
		if ids[d.ID] {
			t.Errorf("ID ganda: %s", d.ID)
		}
		ids[d.ID] = true
		hitung[d.Kelompok]++
		if d.Nama == "" || d.Deskripsi == "" || d.Subkelompok == "" {
			t.Errorf("%s: nama, deskripsi, dan subkelompok wajib diisi", d.ID)
		}
		if (d.generik == nil) == (d.tulisan == nil) {
			t.Errorf("%s: harus punya tepat satu pelaksana (generik atau tulisan tangan)", d.ID)
		}
		if got, ok := DatasetByID(d.ID); !ok || got != d {
			t.Errorf("DatasetByID(%q) tidak mengembalikan dataset itu", d.ID)
		}
	}
	want := map[string]int{KelompokPengadaan: 8, KelompokTender: 16, KelompokEkatalogV5: 5, KelompokEkatalogV6: 5}
	for k, n := range want {
		if hitung[k] != n {
			t.Errorf("kelompok %s: %d dataset, want %d", k, hitung[k], n)
		}
	}
	if len(DaftarDataset) != 34 {
		t.Errorf("jumlah dataset = %d, want 34", len(DaftarDataset))
	}
	for _, k := range UrutanKelompok {
		if NamaKelompok(k) == k {
			t.Errorf("kelompok %s belum punya nama tampil", k)
		}
	}
}

// Tabel, kolom penyaring, dan kolom ringkasan tiap dataset harus ada di migrasinya (kalau tidak, ekspor dan tampilan data pecah).
func TestKatalogPenarikanKolomAdaDiMigrasi(t *testing.T) {
	tabel := kolomMigrasi(t)
	for _, d := range DaftarDataset {
		kolom, ada := tabel[d.Tabel]
		if !ada {
			t.Errorf("%s: tabel %s tidak ditemukan di migrasi", d.ID, d.Tabel)
			continue
		}
		cek := func(nama, k string) {
			if k != "" && !kolom[k] {
				t.Errorf("%s: kolom %s %q tidak ada di tabel %s", d.ID, nama, k, d.Tabel)
			}
		}
		cek("KolomKLPD", d.KolomKLPD)
		cek("KolomTahun", d.KolomTahun)
		if len(d.KolomRingkas) < 3 {
			t.Errorf("%s: kolom ringkasan terlalu sedikit (%v)", d.ID, d.KolomRingkas)
		}
		for _, k := range d.KolomRingkas {
			cek("KolomRingkas", k)
		}
		if !kolom["synced_at"] {
			t.Errorf("%s: tabel %s tidak punya synced_at", d.ID, d.Tabel)
		}
	}
}

func TestKatalogPenarikanModeDanOtomatis(t *testing.T) {
	for _, d := range DaftarDataset {
		switch d.Mode {
		case ModeKode:
			if d.Otomatis {
				t.Errorf("%s: dataset per kode tidak boleh ikut otomatis (tidak ada daftar semua)", d.ID)
			}
		default:
			if !d.Otomatis {
				t.Errorf("%s: seharusnya ikut penarikan otomatis", d.ID)
			}
		}
		if (d.Mode == ModeKLPDTahun) != (d.KolomKLPD != "" && d.KolomTahun != "" && d.Mode != ModeTransaksi) {
			t.Errorf("%s: kolom penyaring tidak cocok dengan mode %s", d.ID, d.Mode)
		}
	}
}

func TestNormalisasiDanRingkas(t *testing.T) {
	pengumuman, _ := DatasetByID("tender/pengumuman")
	p := pengumuman.Normalisasi(PermintaanTarik{Tahun: " 2025 ", Status: "x", Kode: "y", Kd1: "z"})
	if p.KodeKLPD != "K10" || p.Tahun != "2025" || p.Status != "" || p.Kode != "" || p.Kd1 != "" {
		t.Errorf("normalisasi = %+v", p)
	}
	if got := pengumuman.Ringkas(p); got != "K10/2025" {
		t.Errorf("ringkas = %q", got)
	}

	// Status hanya berlaku di dua dataset RUP yang memang menerimanya.
	paket, _ := DatasetByID("rup/paket-penyedia")
	if p := paket.Normalisasi(PermintaanTarik{Tahun: "2025", Status: "Terumumkan"}); p.Status != "Terumumkan" || paket.Ringkas(p) != "K10/2025/Terumumkan" {
		t.Errorf("paket-penyedia: %+v %q", p, paket.Ringkas(p))
	}
	kaji, _ := DatasetByID("rup/history-kaji-ulang")
	if p := kaji.Normalisasi(PermintaanTarik{Tahun: "2025", JenisPaket: "Penyedia", Status: "abaikan"}); p.JenisPaket != "Penyedia" || p.Status != "" {
		t.Errorf("history-kaji-ulang: %+v", p)
	}

	satker, _ := DatasetByID("ekatalog-archive/instansi-satker")
	if p := satker.Normalisasi(PermintaanTarik{Tahun: "2025"}); p.Tahun != "" || satker.Ringkas(p) != "K10" {
		t.Errorf("instansi-satker: %+v", p)
	}

	penyedia, _ := DatasetByID("ekatalog/penyedia-detail")
	if p := penyedia.Normalisasi(PermintaanTarik{Kode: " 01ABC ", Tahun: "2025"}); p.Kode != "01ABC" || p.Tahun != "" || penyedia.Ringkas(p) != "01ABC" {
		t.Errorf("penyedia: %+v", p)
	}

	kategori, _ := DatasetByID("ekatalog/list-kategori-produk")
	if got := kategori.Ringkas(kategori.Normalisasi(PermintaanTarik{})); got != "L1" {
		t.Errorf("kategori L1 = %q", got)
	}
	if got := kategori.Ringkas(kategori.Normalisasi(PermintaanTarik{Kd1: "A", Kd2: "B"})); got != "L3:A/B" {
		t.Errorf("kategori L3 = %q", got)
	}
	// kd_kategori_2 tanpa kd_kategori_1 dibuang (bukan kombinasi sah).
	if p := kategori.Normalisasi(PermintaanTarik{Kd2: "B"}); p.Kd2 != "" {
		t.Errorf("kd2 yatim tidak dibuang: %+v", p)
	}

	transaksi, _ := DatasetByID("ekatalog/e-purchasing-by-produk")
	p = transaksi.Normalisasi(PermintaanTarik{Tahun: "2025"})
	if p.Status != "COMPLETED" || p.KodeKLPD != "K10" || transaksi.Ringkas(p) != "2025/COMPLETED/K10" {
		t.Errorf("transaksi: %+v %q", p, transaksi.Ringkas(p))
	}
}

func TestValidasiPermintaan(t *testing.T) {
	uji := func(id string, p PermintaanTarik, wantKosong bool) {
		t.Helper()
		d, _ := DatasetByID(id)
		if pesan := d.Validasi(d.Normalisasi(p)); (pesan == "") != wantKosong {
			t.Errorf("%s %+v: pesan = %q", id, p, pesan)
		}
	}
	uji("tender/pengumuman", PermintaanTarik{Tahun: "2025"}, true)
	uji("tender/pengumuman", PermintaanTarik{}, false)
	uji("tender/pengumuman", PermintaanTarik{Tahun: "25"}, false)
	uji("rup/paket-penyedia", PermintaanTarik{Tahun: "abcd"}, false)
	uji("ekatalog-archive/instansi-satker", PermintaanTarik{}, true)
	uji("ekatalog/penyedia-detail", PermintaanTarik{}, false)
	uji("ekatalog/penyedia-detail", PermintaanTarik{Kode: "01ABC"}, true)
	uji("ekatalog/penyedia-detail", PermintaanTarik{Kode: strings.Repeat("x", 101)}, false)
	uji("ekatalog/list-kategori-produk", PermintaanTarik{}, true)
	uji("ekatalog/list-kategori-produk", PermintaanTarik{Kd1: "A"}, true)
	uji("ekatalog/e-purchasing-by-produk", PermintaanTarik{Tahun: "2025"}, true)
	uji("ekatalog/e-purchasing-by-produk", PermintaanTarik{}, false)
	uji("ekatalog/e-purchasing-by-produk", PermintaanTarik{Tahun: "2025", Status: "NGAWUR"}, false)
}

// Pelaksana tulisan tangan: uji sambungan dulu; bila Inaproc menolak, handler sinkronisasi (yang menghapus data lama lebih dulu)
// tidak boleh dijalankan.
func TestJalankanTulisanGagalUjiSambunganTidakMenyentuhData(t *testing.T) {
	p := newInaprocPalsu(t, func(r *http.Request) (int, string) {
		return 429, `{"success":false,"error":{"code":"Too Many Requests","message":"Rate limit exceeded","details":"x"}}`
	})
	pasangKonfigInaproc(t, p.URL, "token-uji")
	f := pasangDBPalsu(t)

	d, _ := DatasetByID("rup/paket-penyedia")
	_, err := d.Jalankan(context.Background(), d.Normalisasi(PermintaanTarik{Tahun: "2025"}), "", nil)
	var g *GalatSinkron
	if !errors.As(err, &g) || g.Status != 429 || !g.Hulu {
		t.Fatalf("err = %#v, want GalatSinkron 429 dari hulu", err)
	}
	for _, ex := range f.Execs() {
		if strings.HasPrefix(ex.Query, "DELETE") {
			t.Errorf("data lama tidak boleh dihapus saat uji sambungan gagal: %s", ex.Query)
		}
	}
	// 429 dicoba ulang per halaman di klien (menunggu jeda bersama), tetapi hanya permintaan uji yang pernah dikirim.
	if p.Permintaan() != maksCoba429 {
		t.Errorf("permintaan uji = %d, want %d (satu permintaan uji dengan pengulangan 429)", p.Permintaan(), maksCoba429)
	}
}

func TestJalankanTulisanBerhasilMelaluiHandlerLama(t *testing.T) {
	p := newInaprocPalsu(t, func(r *http.Request) (int, string) {
		return 200, `{"success":true,"data":[],"meta":{"limit":1000,"has_more":false,"cursor":""}}`
	})
	pasangKonfigInaproc(t, p.URL, "token-uji")
	f := pasangDBPalsu(t)

	d, _ := DatasetByID("rup/paket-penyedia")
	hasil, err := d.Jalankan(context.Background(), d.Normalisasi(PermintaanTarik{Tahun: "2025"}), "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if hasil.TotalSinkron != 0 || hasil.Halaman != 1 {
		t.Errorf("hasil = %+v", hasil)
	}
	var hapus, log int
	for _, ex := range f.Execs() {
		switch {
		case strings.HasPrefix(ex.Query, "DELETE FROM inaproc_paket_penyedia"):
			hapus++
			if ex.Args[0].Value != "K10" || ex.Args[1].Value != "2025" {
				t.Errorf("argumen hapus = %+v", ex.Args)
			}
		case strings.Contains(ex.Query, "INTO inaproc_sync_log"):
			log++
			if ex.Args[7].Value != nil {
				t.Errorf("synced_by = %v, want NULL untuk penarikan otomatis", ex.Args[7].Value)
			}
		}
	}
	if hapus != 1 || log != 1 {
		t.Errorf("hapus=%d log=%d, want 1 dan 1", hapus, log)
	}
	// Uji sambungan (limit=1) lalu penarikan sungguhan (limit=1000).
	if p.Permintaan() != 2 {
		t.Errorf("permintaan ke Inaproc = %d, want 2", p.Permintaan())
	}
}

func TestJalankanTanpaTokenDitolak(t *testing.T) {
	pasangKonfigInaproc(t, "http://tidak-dipakai.invalid", "")
	d, _ := DatasetByID("tender/pengumuman")
	_, err := d.Jalankan(context.Background(), d.Normalisasi(PermintaanTarik{Tahun: "2025"}), "", nil)
	var g *GalatSinkron
	if !errors.As(err, &g) || g.Status != http.StatusServiceUnavailable {
		t.Fatalf("err = %#v, want 503", err)
	}
}

func TestPanggilHandlerGinMeneruskanBadanDanPengguna(t *testing.T) {
	var badan map[string]string
	var pengguna string
	kode, resp := panggilHandlerGin(func(c *gin.Context) {
		_ = c.ShouldBindJSON(&badan)
		pengguna = c.GetString("user_id")
		c.JSON(201, gin.H{"ok": true})
	}, map[string]string{"tahun": "2025"}, "u-1")
	if kode != 201 || badan["tahun"] != "2025" || pengguna != "u-1" || !json.Valid(resp) {
		t.Errorf("kode=%d badan=%v pengguna=%q resp=%s", kode, badan, pengguna, resp)
	}
}

// Permintaan: jumlah permintaan yang sudah diterima Inaproc palsu.
func (p *inaprocPalsu) Permintaan() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.diminta)
}
