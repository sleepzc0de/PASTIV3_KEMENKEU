package handlers

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"pasti-v3-backend/analitik"
	"pasti-v3-backend/database"
)

// Tes integrasi dengan SQL Server sungguhan: memeriksa SQL dasbor, pembaca data/ekspor, pencatatan riwayat penarikan, dan pengaturan
// terhadap skema migrasi yang asli (yang tidak bisa dibuktikan oleh database palsu).
//
// Hanya berjalan bila PASTI_UJI_MSSQL_DSN diisi: DSN ke database KHUSUS TES yang migrasinya sudah diterapkan, mis.
//
//	sqlserver://localhost:1433?database=pasti_uji&authenticator=winsspi&encrypt=disable
//	sqlserver://sa:KATA_SANDI@localhost:1433?database=pasti_uji&encrypt=disable
//
// Data uji memakai kode KLPD "UJI" (dan kode rujukan berawalan "UJI-") dan dibersihkan sebelum serta sesudah tes; pengaturan penarikan
// otomatis dikembalikan ke keadaan semula. Jangan diarahkan ke database produksi.

// klpdUji: kode KLPD data uji. Bawaan "UJI" supaya tidak menyentuh data sungguhan; PASTI_UJI_KLPD=K10 hanya untuk database sementara yang dibuang
// (mis. memeriksa tampilan di browser dengan kode KLPD bawaan aplikasi), dan PASTI_UJI_BIARKAN_DATA=1 membiarkan data uji tersimpan.
var klpdUji = func() string {
	if k := os.Getenv("PASTI_UJI_KLPD"); k != "" {
		return k
	}
	return "UJI"
}()

func biarkanDataUji() bool { return os.Getenv("PASTI_UJI_BIARKAN_DATA") == "1" }

func dbIntegrasi(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("PASTI_UJI_MSSQL_DSN")
	if dsn == "" {
		t.Skip("PASTI_UJI_MSSQL_DSN tidak diisi: tes integrasi SQL Server dilewati")
	}
	db, err := sql.Open("sqlserver", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.Ping(); err != nil {
		t.Fatalf("tidak bisa terhubung ke database uji: %v", err)
	}
	lama := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = lama })
	return db
}

var tabelUji = []struct{ tabel, kolomKLPD string }{
	{"inaproc_paket_penyedia", "kd_klpd"}, {"inaproc_paket_swakelola", "kd_klpd"}, {"inaproc_paket_swakelola_terumumkan", "kd_klpd"},
	{"inaproc_program_master", "kd_klpd"}, {"inaproc_tender_pengumuman", "kd_klpd"}, {"inaproc_non_tender_pengumuman", "kd_klpd"},
	{"inaproc_peserta_tender", "kd_klpd"}, {"inaproc_tender_selesai", "kd_klpd"}, {"inaproc_tender_selesai_nilai", "kd_klpd"},
	{"inaproc_non_tender_selesai", "kd_klpd"}, {"inaproc_tender_ekontrak_kontrak", "kd_klpd"}, {"inaproc_non_tender_ekontrak_kontrak", "kd_klpd"},
	{"inaproc_ekatalog_paket_epurchasing", "kd_klpd"}, {"inaproc_ekatalog6_paket_epurchasing", "kode_klpd"}, {"inaproc_ekatalog6_epurchasing_produk", "kode_klpd"},
}

var rujukanUji = []struct{ tabel, kolom string }{
	{"inaproc_ekatalog_komoditas", "kd_komoditas"}, {"inaproc_ekatalog_penyedia", "kd_penyedia"}, {"inaproc_ekatalog6_penyedia", "kode_penyedia"},
}

func bersihkanUji(t *testing.T, db *sql.DB) {
	t.Helper()
	for _, x := range tabelUji {
		if _, err := db.Exec(fmt.Sprintf("DELETE FROM %s WHERE %s = @p1", x.tabel, x.kolomKLPD), klpdUji); err != nil {
			t.Fatalf("bersihkan %s: %v", x.tabel, err)
		}
	}
	for _, x := range rujukanUji {
		if _, err := db.Exec(fmt.Sprintf("DELETE FROM %s WHERE %s LIKE 'UJI-%%'", x.tabel, x.kolom)); err != nil {
			t.Fatalf("bersihkan %s: %v", x.tabel, err)
		}
	}
	db.Exec("DELETE FROM inaproc_penarikan WHERE parameter LIKE @p1", klpdUji+"/%")
	db.Exec("DELETE FROM inaproc_sync_log WHERE kode_klpd = @p1", klpdUji)
}

var urutBaris int64

// sisip memasukkan satu baris ke tabel dengan kolom menurut peta (row_key dibuat otomatis).
func sisip(t *testing.T, db *sql.DB, tabel string, kolom map[string]interface{}) {
	t.Helper()
	nama := []string{"row_key"}
	args := []interface{}{fmt.Sprintf("%s-uji-%d", klpdUji, atomic.AddInt64(&urutBaris, 1))} // memuat kode KLPD: data uji berbeda KLPD tidak bentrok kunci
	var kunci []string
	for k := range kolom {
		kunci = append(kunci, k)
	}
	sort.Strings(kunci)
	ph := []string{"@p1"}
	for i, k := range kunci {
		nama = append(nama, k)
		args = append(args, kolom[k])
		ph = append(ph, fmt.Sprintf("@p%d", i+2))
	}
	if _, err := db.Exec(fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", tabel, strings.Join(nama, ", "), strings.Join(ph, ", ")), args...); err != nil {
		t.Fatalf("sisip %s: %v", tabel, err)
	}
}

func hariUji(th, bln, hari int) time.Time {
	return time.Date(th, time.Month(bln), hari, 0, 0, 0, 0, time.UTC)
}

type m = map[string]interface{}

// seedAnalitik mengisi data uji; angka harapan di TestAnalitikDenganSQLServer dihitung dari data ini.
func seedAnalitik(t *testing.T, db *sql.DB) {
	t.Helper()
	k, th := klpdUji, "2025"
	rup := func(kd string, satker string, pagu int64, metode, jenis, umumkan string, ukm, pdn interface{}, awal interface{}, aktif, hapus int) {
		sisip(t, db, "inaproc_paket_penyedia", m{"kd_klpd": k, "tahun_anggaran": th, "kd_rup": kd, "nama_satker": satker, "pagu": pagu, "metode_pengadaan": metode,
			"jenis_pengadaan": jenis, "status_umumkan_rup": umumkan, "status_ukm": ukm, "status_pdn": pdn, "tgl_awal_pemilihan": awal, "status_aktif_rup": aktif, "status_delete_rup": hapus})
	}
	rup("RUP1", "Satker A", 600_000_000, "Tender", "Barang", "Terumumkan", "UKM", "PDN", hariUji(2025, 2, 10), 1, 0)
	rup("RUP2", "Satker A", 300_000_000, "Tender", "Jasa Konsultansi", "Terumumkan", "Non UKM", "PDN", hariUji(2025, 11, 5), 1, 0)
	rup("RUP3", "Satker B", 100_000_000, "Pengadaan Langsung", "Barang", "Belum Terumumkan", "UKM", "Non PDN", hariUji(2025, 11, 20), 1, 0)
	rup("RUP4", "Satker B", 200_000_000, "E-Purchasing", "Barang", "Terumumkan", "UKM", "PDN", hariUji(2025, 12, 1), 1, 0)
	rup("RUP5", "Satker B", 999_000_000, "Tender", "Barang", "Terumumkan", "UKM", "PDN", hariUji(2025, 1, 1), 1, 1) // dihapus
	rup("RUP6", "Satker B", 888_000_000, "Tender", "Barang", "Terumumkan", "UKM", "PDN", hariUji(2025, 1, 1), 0, 0) // tidak aktif
	rup("RUP7", "Satker A", 100_000_000, "Seleksi", "Jasa Lainnya", "Terumumkan", nil, nil, nil, 1, 0)
	sisip(t, db, "inaproc_paket_penyedia", m{"kd_klpd": k, "tahun_anggaran": "2024", "kd_rup": "RUP2024", "pagu": int64(1_000_000_000), "status_aktif_rup": 1, "status_delete_rup": 0})

	sisip(t, db, "inaproc_paket_swakelola", m{"kd_klpd": k, "tahun_anggaran": th, "kd_rup": "SW1"})
	sisip(t, db, "inaproc_paket_swakelola", m{"kd_klpd": k, "tahun_anggaran": th, "kd_rup": "SW2"})
	sisip(t, db, "inaproc_paket_swakelola_terumumkan", m{"kd_klpd": k, "tahun_anggaran": th, "kd_rup": "SW1", "pagu": int64(50_000_000), "status_aktif_rup": 1, "status_delete_rup": 0})
	sisip(t, db, "inaproc_paket_swakelola_terumumkan", m{"kd_klpd": k, "tahun_anggaran": th, "kd_rup": "SW2", "pagu": int64(25_000_000), "status_aktif_rup": 1, "status_delete_rup": 0})
	sisip(t, db, "inaproc_paket_swakelola_terumumkan", m{"kd_klpd": k, "tahun_anggaran": th, "kd_rup": "SW3", "pagu": int64(1_000_000_000), "status_aktif_rup": 1, "status_delete_rup": 1})
	sisip(t, db, "inaproc_program_master", m{"kd_klpd": k, "tahun_anggaran": th, "kd_program": "P1", "pagu_program": int64(2_000_000_000), "is_deleted": 0})
	sisip(t, db, "inaproc_program_master", m{"kd_klpd": k, "tahun_anggaran": th, "kd_program": "P2", "pagu_program": int64(500_000_000), "is_deleted": 0})
	sisip(t, db, "inaproc_program_master", m{"kd_klpd": k, "tahun_anggaran": th, "kd_program": "P3", "pagu_program": int64(9_000_000_000), "is_deleted": 1})

	tender := func(kd string, versi int, rup, status, metode, jenis string, pagu, hps float64, tgl time.Time) {
		sisip(t, db, "inaproc_tender_pengumuman", m{"kd_klpd": k, "tahun_anggaran": th, "kd_tender": kd, "versi_tender": versi, "kd_rup": rup, "status_tender": status,
			"mtd_pemilihan": metode, "jenis_pengadaan": jenis, "pagu": pagu, "hps": hps, "tgl_pengumuman_tender": tgl})
	}
	tender("T1", 1, "RUP1", "Selesai", "Tender", "Barang", 600e6, 590e6, hariUji(2025, 3, 1))
	tender("T1", 2, "RUP1", "Selesai", "Tender", "Barang", 600e6, 590e6, hariUji(2025, 3, 1)) // versi lama: tidak boleh dihitung dua kali
	tender("T2", 1, "RUP2", "Gagal", "Seleksi", "Jasa Konsultansi", 300e6, 290e6, hariUji(2025, 4, 1))
	tender("T3", 1, "RUP9", "Berjalan", "Tender Cepat", "Barang", 50e6, 48e6, hariUji(2025, 5, 1))
	nontender := func(kd, rup, status string, pagu, hps float64, tgl time.Time) {
		sisip(t, db, "inaproc_non_tender_pengumuman", m{"kd_klpd": k, "tahun_anggaran": th, "kd_nontender": kd, "versi_nontender": 1, "kd_rup": rup, "status_nontender": status,
			"mtd_pemilihan": "Pengadaan Langsung", "pagu": pagu, "hps": hps, "tgl_pengumuman_nontender": tgl})
	}
	nontender("N1", "RUP3", "Selesai", 100e6, 99e6, hariUji(2025, 6, 1))
	nontender("N2", "RUP10", "Selesai", 20e6, 20e6, hariUji(2025, 6, 15))

	for _, p := range []struct {
		tender string
		n      int
	}{{"T1", 3}, {"T2", 1}, {"T3", 5}} {
		for i := 1; i <= p.n; i++ {
			sisip(t, db, "inaproc_peserta_tender", m{"kd_klpd": k, "tahun_anggaran": th, "kd_tender": p.tender, "kd_peserta": fmt.Sprintf("%s-%d", p.tender, i)})
		}
	}
	sisip(t, db, "inaproc_tender_selesai", m{"kd_klpd": k, "tahun_anggaran": th, "kd_tender": "T1", "tgl_pengumuman_tender": hariUji(2025, 3, 1), "tgl_penetapan_pemenang": hariUji(2025, 3, 31)})
	sisip(t, db, "inaproc_tender_selesai", m{"kd_klpd": k, "tahun_anggaran": th, "kd_tender": "T2", "tgl_pengumuman_tender": hariUji(2025, 4, 1), "tgl_penetapan_pemenang": hariUji(2025, 5, 16)})
	sisip(t, db, "inaproc_tender_selesai", m{"kd_klpd": k, "tahun_anggaran": th, "kd_tender": "T9", "tgl_pengumuman_tender": hariUji(2025, 4, 1), "tgl_penetapan_pemenang": hariUji(2025, 3, 1)}) // data salah: dibuang dari waktu proses

	nilai := func(kd, nama string, hps, kontrak interface{}) {
		sisip(t, db, "inaproc_tender_selesai_nilai", m{"kd_klpd": k, "tahun_anggaran": th, "kd_tender": kd, "nama_penyedia": nama, "hps": hps, "nilai_kontrak": kontrak})
	}
	nilai("T1", "PT Maju Jaya", 590e6, 560e6)
	nilai("T2", "CV Sukses Abadi", 290e6, 285e6)
	nilai("T3", "PT MAJU JAYA ", 48e6, 50e6) // nama sama dengan beda huruf/spasi: satu penyedia
	nilai("T4", "PT Lain", nil, 10e6)        // tanpa HPS: dihitung di nilai kontrak, tidak di efisiensi
	sisip(t, db, "inaproc_non_tender_selesai", m{"kd_klpd": k, "tahun_anggaran": th, "kd_nontender": "N1", "nama_penyedia": "pt maju jaya", "hps": 99e6, "nilai_kontrak": 90e6})

	kontrakT := func(kd, no string, versi int, nilai float64, status string, tgl, akhir time.Time) {
		sisip(t, db, "inaproc_tender_ekontrak_kontrak", m{"kd_klpd": k, "tahun_anggaran": th, "kd_tender": kd, "no_kontrak": no, "versi_addendum": versi, "nilai_kontrak": nilai,
			"status_kontrak": status, "tgl_kontrak": tgl, "tgl_kontrak_akhir": akhir, "nama_paket": "Paket " + kd, "nama_penyedia": "PT Maju Jaya"})
	}
	kontrakT("T1", "K-001", 0, 560e6, "Berjalan", hariUji(2025, 4, 10), hariUji(2025, 10, 20))
	kontrakT("T1", "K-001", 1, 570e6, "Berjalan", hariUji(2025, 4, 10), hariUji(2025, 10, 20)) // addendum: versi terbaru yang dihitung
	kontrakT("T2", "K-002", 0, 285e6, "Selesai", hariUji(2025, 5, 20), hariUji(2025, 10, 10))
	kontrakN := func(kd, no string, add string, nilai float64, tgl, akhir time.Time) {
		sisip(t, db, "inaproc_non_tender_ekontrak_kontrak", m{"kd_klpd": k, "tahun_anggaran": th, "kd_nontender": kd, "no_kontrak": no, "apakah_addendum": add, "nilai_kontrak": nilai,
			"status_kontrak": "Berjalan", "tgl_kontrak": tgl, "tgl_kontrak_akhir": akhir})
	}
	kontrakN("N1", "NK-001", "Ya", 90e6, hariUji(2025, 6, 20), hariUji(2025, 12, 31))
	kontrakN("N2", "NK-002", "Tidak", 20e6, hariUji(2025, 6, 25), hariUji(2025, 11, 15))

	sisip(t, db, "inaproc_ekatalog_komoditas", m{"kd_komoditas": "UJI-K1", "nama_komoditas": "Laptop"})
	sisip(t, db, "inaproc_ekatalog_penyedia", m{"kd_penyedia": "UJI-V1", "nama_penyedia": "PT Penyedia Satu"})
	v5 := func(paket, kom, pen string, total float64, tgl time.Time, status string) {
		sisip(t, db, "inaproc_ekatalog_paket_epurchasing", m{"kd_klpd": k, "tahun_anggaran": th, "kd_paket": paket, "kd_komoditas": kom, "kd_penyedia": pen, "total_harga": total,
			"tanggal_buat_paket": tgl, "paket_status_str": status})
	}
	v5("P1", "UJI-K1", "UJI-V1", 100e6, hariUji(2025, 3, 5), "Selesai")
	v5("P1", "UJI-K1", "UJI-V1", 50e6, hariUji(2025, 3, 5), "Selesai")
	v5("P2", "UJI-K2", "UJI-V2", 70e6, hariUji(2025, 7, 7), "Batal")

	sisip(t, db, "inaproc_ekatalog6_penyedia", m{"kode_penyedia": "UJI-S1", "nama_penyedia": "PT Penyedia V6"})
	v6 := func(order, penyedia, status string, swasta int, rup string, total float64, tgl time.Time) {
		sisip(t, db, "inaproc_ekatalog6_paket_epurchasing", m{"kode_klpd": k, "fiscal_year": th, "order_id": order, "kode_penyedia": penyedia, "status": status, "is_swasta": swasta,
			"rup_code": rup, "total": total, "order_date": tgl})
	}
	v6("O1", "UJI-S1", "COMPLETED", 0, "RUP4", 200e6, hariUji(2025, 8, 8))
	v6("O2", "UJI-S2", "COMPLETED", 1, "RUPX", 30e6, hariUji(2025, 9, 9))
	v6("O3", "UJI-S1", "ON_PROCESS", 0, "", 20e6, hariUji(2025, 9, 10))
	for i, kat := range []struct {
		nama  string
		nilai float64
	}{{"Komputer", 100e6}, {"Komputer", 50e6}, {"Alat Tulis", 10e6}} {
		sisip(t, db, "inaproc_ekatalog6_epurchasing_produk", m{"kode_klpd": k, "tahun": th, "order_id": fmt.Sprintf("TR%d", i), "kategori_1": kat.nama, "nilai_transaksi": kat.nilai})
	}

	// Tahun sebelumnya (pembanding).
	sisip(t, db, "inaproc_tender_pengumuman", m{"kd_klpd": k, "tahun_anggaran": "2024", "kd_tender": "T2024a", "versi_tender": 1})
	sisip(t, db, "inaproc_tender_pengumuman", m{"kd_klpd": k, "tahun_anggaran": "2024", "kd_tender": "T2024b", "versi_tender": 1})
	sisip(t, db, "inaproc_tender_selesai_nilai", m{"kd_klpd": k, "tahun_anggaran": "2024", "kd_tender": "T2024a", "nama_penyedia": "PT Lama", "nilai_kontrak": 400e6})
}

func hampir(t *testing.T, nama string, got, want, toleransi float64) {
	t.Helper()
	if got < want-toleransi || got > want+toleransi {
		t.Errorf("%s = %v, want %v (+-%v)", nama, got, want, toleransi)
	}
}

func TestAnalitikDenganSQLServer(t *testing.T) {
	db := dbIntegrasi(t)
	bersihkanUji(t, db)
	if !biarkanDataUji() {
		t.Cleanup(func() { bersihkanUji(t, db) })
	}
	seedAnalitik(t, db)

	sekarang := time.Date(2025, 10, 4, 3, 0, 0, 0, zonaWIB)
	h := HitungAnalitik(context.Background(), db, klpdUji, "2025", sekarang)
	if len(h.Galat) != 0 {
		t.Fatalf("bagian gagal dibaca: %v (lihat log untuk SQL-nya)", h.Galat)
	}

	// ---- RUP ----
	r := h.RUP
	if r.TotalPaket != 5 || r.TotalPagu != 1_300_000_000 {
		t.Errorf("RUP total = %d paket / %v (paket dihapus dan tidak aktif tidak boleh dihitung)", r.TotalPaket, r.TotalPagu)
	}
	if r.PaketSwakelola != 2 || r.PaguSwakelola != 75_000_000 || r.PaguProgram != 2_500_000_000 {
		t.Errorf("swakelola/program = %d / %v / %v", r.PaketSwakelola, r.PaguSwakelola, r.PaguProgram)
	}
	if len(r.TopSatker) != 2 || r.TopSatker[0].Label != "Satker A" || r.TopSatker[0].Nilai != 1_000_000_000 || r.TopSatker[0].Jumlah != 3 {
		t.Errorf("top satker = %+v", r.TopSatker)
	}
	if len(r.PerMetode) < 1 || r.PerMetode[0].Label != "Tender" || r.PerMetode[0].Nilai != 900_000_000 {
		t.Errorf("per metode = %+v", r.PerMetode)
	}
	var belum float64
	for _, s := range r.StatusUmumkan {
		if s.Label == "Belum Terumumkan" {
			belum = s.Nilai
		}
	}
	if belum != 100_000_000 {
		t.Errorf("status umumkan = %+v", r.StatusUmumkan)
	}
	if len(r.PerBulanPemilihan) != 12 || r.PerBulanPemilihan[1].Nilai != 600e6 || r.PerBulanPemilihan[10].Nilai != 400e6 || r.PerBulanPemilihan[11].Nilai != 200e6 || r.PerBulanPemilihan[0].Nilai != 0 {
		t.Errorf("per bulan pemilihan = %+v", r.PerBulanPemilihan)
	}
	var tanpaUKM int64
	for _, s := range r.StatusUKM {
		if s.Label == "(tidak diisi)" {
			tanpaUKM = s.Jumlah
		}
	}
	if tanpaUKM != 1 {
		t.Errorf("status UKM = %+v", r.StatusUKM)
	}

	// ---- Corong ----
	c := h.Corong
	peta := map[string]analitik.Pasangan{}
	for _, x := range c.Tahap {
		peta[x.Label] = x
	}
	if c.TotalPaket != 5 || peta["Tender"].Jumlah != 2 || peta["Tender"].Nilai != 900e6 || peta["Non-tender"].Jumlah != 1 || peta["E-Purchasing"].Jumlah != 1 ||
		peta["E-Purchasing"].Nilai != 200e6 || peta["Belum diproses"].Jumlah != 1 || peta["Belum diproses"].Nilai != 100e6 {
		t.Errorf("corong = %+v", c)
	}

	// ---- Pemilihan ----
	p := h.Pemilihan
	if p.TenderJumlah != 3 || p.TenderPagu != 950e6 || p.TenderHPS != 928e6 {
		t.Errorf("tender = %d / %v / %v (versi lama tidak boleh dihitung dua kali)", p.TenderJumlah, p.TenderPagu, p.TenderHPS)
	}
	if p.NonTenderJumlah != 2 || p.NonTenderPagu != 120e6 {
		t.Errorf("non-tender = %d / %v", p.NonTenderJumlah, p.NonTenderPagu)
	}
	if p.TenderSelesai != 3 || p.NilaiKontrak != 995e6 {
		t.Errorf("selesai = %d, nilai kontrak = %v", p.TenderSelesai, p.NilaiKontrak)
	}
	e := p.Efisiensi
	if e.Sampel != 4 || e.TotalHPS != 1027e6 || e.TotalKontrak != 985e6 {
		t.Errorf("efisiensi = %+v", e)
	}
	hampir(t, "efisiensi %", e.Persen, 4.0896, 0.001)
	hampir(t, "median efisiensi", e.Median, 3.4044, 0.001)
	if len(e.Sebaran) != 5 || e.Sebaran[0].Jumlah != 1 || e.Sebaran[1].Jumlah != 1 || e.Sebaran[2].Jumlah != 2 || e.Sebaran[3].Jumlah != 0 {
		t.Errorf("sebaran efisiensi = %+v", e.Sebaran)
	}
	pr := p.Persaingan
	if pr.TenderBerpeserta != 3 || pr.SatuPeserta != 1 || pr.Sebaran[0].Jumlah != 1 || pr.Sebaran[2].Jumlah != 1 || pr.Sebaran[3].Jumlah != 1 {
		t.Errorf("persaingan = %+v", pr)
	}
	hampir(t, "rata peserta", pr.RataPeserta, 3.0, 0.001)
	if p.WaktuProses.Sampel != 2 {
		t.Errorf("waktu proses sampel = %d (data dengan tanggal terbalik harus dibuang)", p.WaktuProses.Sampel)
	}
	hampir(t, "median hari proses", p.WaktuProses.Median, 37.5, 0.001)
	hampir(t, "rata hari proses", p.WaktuProses.Rata, 37.5, 0.001)
	if p.Pasar.JumlahPenyedia != 3 || p.Pasar.TotalNilai != 995e6 || len(p.Pasar.Top) == 0 || strings.ToLower(strings.TrimSpace(p.Pasar.Top[0].Label)) != "pt maju jaya" || p.Pasar.Top[0].Nilai != 700e6 || p.Pasar.Top[0].Jumlah != 3 {
		t.Errorf("pasar = %+v", p.Pasar)
	}
	hampir(t, "HHI", p.Pasar.HHI, 5770.8, 1.0)

	// ---- Kontrak ----
	k := h.Kontrak
	if k.TenderJumlah != 2 || k.TenderNilai != 855e6 || k.NonTenderJumlah != 2 || k.NonTenderNilai != 110e6 || k.Addendum != 2 {
		t.Errorf("kontrak = %+v (versi addendum terbaru yang dihitung)", k)
	}
	if k.BerakhirDalam != 2 || k.NilaiBerakhir != 590e6 || len(k.AkanBerakhir) != 2 || k.AkanBerakhir[0].NoKontrak != "K-001" || k.AkanBerakhir[0].SisaHari != 16 || k.AkanBerakhir[1].NoKontrak != "NK-002" {
		t.Errorf("kontrak berakhir = %d / %v / %+v", k.BerakhirDalam, k.NilaiBerakhir, k.AkanBerakhir)
	}
	if k.PerBulan[3].Nilai != 570e6 || k.PerBulan[4].Nilai != 285e6 || k.PerBulan[5].Jumlah != 2 || k.PerBulan[5].Nilai != 110e6 {
		t.Errorf("kontrak per bulan = %+v", k.PerBulan)
	}

	// ---- E-Katalog ----
	ek := h.Ekatalog
	if ek.V5.Paket != 2 || ek.V5.Nilai != 220e6 || ek.V5.PerBulan[2].Nilai != 150e6 || ek.V5.PerBulan[6].Nilai != 70e6 {
		t.Errorf("v5 = %+v", ek.V5)
	}
	if len(ek.V5.TopKomoditas) != 2 || ek.V5.TopKomoditas[0].Label != "Laptop" || ek.V5.TopKomoditas[0].Jumlah != 1 || ek.V5.TopKomoditas[0].Nilai != 150e6 || ek.V5.TopKomoditas[1].Label != "Komoditas UJI-K2" {
		t.Errorf("v5 komoditas = %+v", ek.V5.TopKomoditas)
	}
	if len(ek.V5.TopPenyedia) != 2 || ek.V5.TopPenyedia[0].Label != "PT Penyedia Satu" || ek.V5.TopPenyedia[1].Label != "Penyedia UJI-V2" {
		t.Errorf("v5 penyedia = %+v", ek.V5.TopPenyedia)
	}
	if ek.V6.Order != 3 || ek.V6.Nilai != 250e6 || ek.V6.OrderSwasta != 1 || ek.V6.NilaiSwasta != 30e6 {
		t.Errorf("v6 = %+v", ek.V6)
	}
	if len(ek.V6.TopPenyedia) != 2 || ek.V6.TopPenyedia[0].Label != "PT Penyedia V6" || ek.V6.TopPenyedia[0].Jumlah != 2 || ek.V6.TopPenyedia[0].Nilai != 220e6 || ek.V6.TopPenyedia[1].Label != "Penyedia UJI-S2" {
		t.Errorf("v6 penyedia = %+v", ek.V6.TopPenyedia)
	}
	if ek.V6.TransaksiBaris != 3 || ek.V6.TransaksiNilai != 160e6 || len(ek.V6.TopKategori) != 2 || ek.V6.TopKategori[0].Label != "Komputer" || ek.V6.TopKategori[0].Nilai != 150e6 {
		t.Errorf("v6 transaksi = %d / %v / %+v", ek.V6.TransaksiBaris, ek.V6.TransaksiNilai, ek.V6.TopKategori)
	}

	// ---- Pembanding, kelengkapan, dan wawasan ----
	if h.Pembanding == nil || h.Pembanding.Tahun != "2024" || h.Pembanding.RUPPaket != 1 || h.Pembanding.RUPPagu != 1e9 || h.Pembanding.TenderJumlah != 2 || h.Pembanding.NilaiKontrak != 400e6 {
		t.Errorf("pembanding = %+v", h.Pembanding)
	}
	if len(h.DatasetKosong) != 0 {
		t.Errorf("dataset kosong = %v, want tidak ada", h.DatasetKosong)
	}
	w := analitik.Susun(h)
	if len(w) < 5 {
		t.Errorf("wawasan terlalu sedikit untuk data selengkap ini: %d", len(w))
	}
	for _, x := range w {
		if x.Judul == "" || x.Isi == "" {
			t.Errorf("wawasan kosong: %+v", x)
		}
	}

	// ---- Tahun tanpa data: semua bagian tetap berhasil (median/pembagian dari himpunan kosong) ----
	kosong := HitungAnalitik(context.Background(), db, klpdUji, "2031", sekarang)
	if len(kosong.Galat) != 0 {
		t.Fatalf("bagian gagal pada tahun kosong: %v", kosong.Galat)
	}
	if kosong.RUP.TotalPaket != 0 || kosong.Pemilihan.Efisiensi.Sampel != 0 || kosong.Pemilihan.Persaingan.TenderBerpeserta != 0 || kosong.Corong.TotalPaket != 0 || kosong.Kontrak.BerakhirDalam != 0 {
		t.Errorf("tahun kosong tidak nol: %+v", kosong.RUP)
	}
	if len(kosong.DatasetKosong) != len(datasetDasbor) {
		t.Errorf("dataset kosong = %d, want %d", len(kosong.DatasetKosong), len(datasetDasbor))
	}
	if tersedia := tahunTersediaAnalitik(context.Background(), db, klpdUji); len(tersedia) < 2 || tersedia[0] != "2025" || tersedia[1] != "2024" {
		t.Errorf("tahun tersedia = %v", tersedia)
	}
}

// Setiap dataset di katalog: kolom ringkasan, penyaring, dan kunci urutan harus ada di tabel sungguhan, dan query halaman/ekspor valid.
func TestKatalogDataDenganSQLServer(t *testing.T) {
	db := dbIntegrasi(t)
	ctx := context.Background()
	for _, d := range DaftarDataset {
		p := PenyaringData{KodeKLPD: klpdUji, Tahun: "2025", Cari: "x"}
		if _, err := d.hitung(ctx, db, p); err != nil {
			t.Errorf("%s: hitung: %v", d.ID, err)
			continue
		}
		src, err := d.bacaBaris(ctx, db, d.KolomRingkas, p, 0, 10)
		if err != nil {
			t.Errorf("%s: baca halaman: %v", d.ID, err)
			continue
		}
		for src.Next() {
		}
		if src.Err() != nil {
			t.Errorf("%s: pindai: %v", d.ID, src.Err())
		}
		src.Close()

		semua, err := d.semuaKolom(ctx, db)
		if err != nil || len(semua) < len(d.KolomRingkas) {
			t.Errorf("%s: semua kolom: %v (%d kolom)", d.ID, err, len(semua))
			continue
		}
		src, err = d.bacaBaris(ctx, db, semua, PenyaringData{}, 0, 0)
		if err != nil {
			t.Errorf("%s: baca semua kolom: %v", d.ID, err)
			continue
		}
		for src.Next() {
			break
		}
		src.Close()
		d.tahunTersedia(ctx, db)
	}
}

// Pengelola dan pengaturan terhadap tabel inaproc_penarikan* yang asli (OUTPUT INSERTED, UPDATE ... IF @@ROWCOUNT, penjadwal due).
func TestPenarikanDenganSQLServer(t *testing.T) {
	db := dbIntegrasi(t)
	bersihkanUji(t, db)
	t.Cleanup(func() { bersihkanUji(t, db) })

	m := NewPenarikInaproc(db)
	jedaLama := jedaUlang429
	jedaUlang429 = []time.Duration{time.Millisecond}
	t.Cleanup(func() { jedaUlang429 = jedaLama })

	var panggilan int32
	m.jalankan = func(ctx context.Context, tg Tugas, oleh string, kabar func(string)) (HasilSinkron, error) {
		kabar("sedang berjalan")
		switch atomic.AddInt32(&panggilan, 1) {
		case 1:
			return HasilSinkron{TotalSinkron: 12, TotalGagal: 2, Halaman: 3}, nil
		default:
			return HasilSinkron{}, &GalatSinkron{Status: http.StatusBadGateway, Pesan: "Gagal menghubungi API Inaproc"}
		}
	}
	tugas := []Tugas{
		tugasUji(t, "tender/pengumuman", PermintaanTarik{KodeKLPD: klpdUji, Tahun: "2025"}),
		tugasUji(t, "tender/peserta-tender", PermintaanTarik{KodeKLPD: klpdUji, Tahun: "2025"}),
	}
	if _, err := m.Start(tugas, PemicuOtomatis, Oleh{Nama: "otomatis"}, 0); err != nil {
		t.Fatal(err)
	}
	tungguSelesai(t, m)

	riwayat, err := m.Riwayat(context.Background(), "", 100)
	if err != nil {
		t.Fatal(err)
	}
	ditemukan := map[string]RiwayatPenarikan{}
	for _, r := range riwayat {
		if strings.HasPrefix(r.Parameter, klpdUji+"/") {
			ditemukan[r.Dataset] = r
		}
	}
	a, b := ditemukan["tender/pengumuman"], ditemukan["tender/peserta-tender"]
	if a.Status != PenarikanSukses || a.JumlahBaris == nil || *a.JumlahBaris != 12 || *a.BarisGagal != 2 || *a.Halaman != 3 || a.Pemicu != PemicuOtomatis || a.BatchID == "" || len(a.BatchID) != 36 {
		t.Errorf("riwayat sukses = %+v", a)
	}
	if a.Pesan == nil || *a.Pesan != "2 baris gagal disimpan" || a.Selesai == nil || a.Mulai == nil {
		t.Errorf("pesan/waktu riwayat sukses = %+v", a)
	}
	if b.Status != PenarikanGagal || b.Pesan == nil || *b.Pesan != "Gagal menghubungi API Inaproc" || b.JumlahBaris != nil {
		t.Errorf("riwayat gagal = %+v", b)
	}

	// Dasar jatuh tempo penjadwal: sukses terakhir dan percobaan terakhir per (dataset, parameter) dari penarikan otomatis.
	oto, err := m.TerakhirOtomatis(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	ok, gagal := oto["tender/pengumuman|"+klpdUji+"/2025"], oto["tender/peserta-tender|"+klpdUji+"/2025"]
	if ok.SuksesTerakhir == nil || gagal.SuksesTerakhir != nil || gagal.CobaTerakhir.IsZero() {
		t.Errorf("riwayat otomatis = %+v / %+v", ok, gagal)
	}
	if latest, lastOK, err := m.TerakhirPerDataset(context.Background()); err != nil || latest["tender/pengumuman"].Dataset == "" || lastOK["tender/pengumuman"].Status != PenarikanSukses {
		t.Errorf("terakhir per dataset: %v", err)
	}
	if _, err := m.RecoverOrphans(context.Background()); err != nil {
		t.Errorf("RecoverOrphans: %v", err)
	}
	if ringkas := m.RingkasTabel(context.Background()); len(ringkas) != len(DaftarDataset) {
		t.Errorf("ringkasan tabel = %d dataset, want %d", len(ringkas), len(DaftarDataset))
	}

	// Pengaturan: simpan, baca ulang, simpan lagi (UPDATE), lalu pulihkan keadaan semula.
	asli, err := m.MuatPengaturan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if asli.BawaanServer {
			db.Exec("DELETE FROM inaproc_penarikan_pengaturan WHERE id = 1")
		} else {
			m.SimpanPengaturan(context.Background(), asli, asli.DiubahOleh)
		}
	})
	baru := Pengaturan{Aktif: false, IntervalHari: 3, JamMulai: 22, JamAkhir: 4, KodeKLPD: "K10", JumlahTahun: 3, JedaDetik: 5, Dataset: []string{"tender/pengumuman", "rup/paket-penyedia"}}
	if pesan := baru.Rapikan(); pesan != "" {
		t.Fatal(pesan)
	}
	for i := 0; i < 2; i++ { // pertama = INSERT, kedua = UPDATE
		baru.IntervalHari = 3 + i
		if err := m.SimpanPengaturan(context.Background(), baru, "uji"); err != nil {
			t.Fatal(err)
		}
		got, err := m.MuatPengaturan(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if got.BawaanServer || got.Aktif || got.IntervalHari != 3+i || got.JamMulai != 22 || got.JamAkhir != 4 || got.JumlahTahun != 3 || got.JedaDetik != 5 ||
			strings.Join(got.Dataset, ",") != "rup/paket-penyedia,tender/pengumuman" || got.DiubahOleh != "uji" || got.Diubah == nil {
			t.Errorf("pengaturan putaran %d = %+v", i, got)
		}
	}
	var baris int
	db.QueryRow("SELECT COUNT(*) FROM inaproc_penarikan_pengaturan").Scan(&baris)
	if baris != 1 {
		t.Errorf("baris pengaturan = %d, want tepat 1", baris)
	}
}
