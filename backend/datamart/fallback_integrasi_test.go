package datamart

import (
	"context"
	"database/sql"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"pasti-v3-backend/digitalisasi"
)

// Tes integrasi fallback sinkronisasi (PASTI_UJI_MSSQL_DSN; lihat datamart_integrasi_test.go): Manager SUNGGUHAN dan Sync SUNGGUHAN terhadap SQL Server, dengan tiga database
// sementara: PASTI (migrasi 020 dan 063), SLDK palsu, dan datamart yang dibangun script. SLDK "mati" dibuat dengan port yang tidak mendengarkan.

// bukaDB membuka koneksi ke database nama di server DSN uji; host dan port boleh diganti (untuk mensimulasikan SLDK yang tidak terjangkau).
func bukaDB(t *testing.T, nama, hostPort string) *sql.DB {
	t.Helper()
	u, err := url.Parse(os.Getenv("PASTI_UJI_MSSQL_DSN"))
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	q.Set("database", nama)
	u.RawQuery = q.Encode()
	if hostPort != "" {
		u.Host = hostPort
	}
	db, err := sql.Open("sqlserver", u.String())
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(4)
	t.Cleanup(func() { db.Close() })
	return db
}

type lingkunganFallback struct {
	srv                *sql.DB
	pastiNama          string
	sldkNama, martNama string
	pasti, sldk, mart  *sql.DB
	sldkMati           *sql.DB
}

func siapkanFallback(t *testing.T) *lingkunganFallback {
	t.Helper()
	srv := bukaServerUji(t)
	sldkNama, martNama := siapkanDatamart(t, srv)
	pastiNama := buatDBUji(t, srv, "pasti")
	for _, f := range []string{"../migrations/020_create_digitalisasi.sql", "../migrations/063_digitalisasi_sync_log_sumber.sql"} {
		if err := jalankanBerkas(t, srv, pastiNama, bacaBerkas(t, f), nil); err != nil {
			t.Fatalf("migrasi %s gagal: %v", f, err)
		}
	}
	return &lingkunganFallback{
		srv: srv, pastiNama: pastiNama, sldkNama: sldkNama, martNama: martNama,
		pasti: bukaDB(t, pastiNama, ""), sldk: bukaDB(t, sldkNama, ""), mart: bukaDB(t, martNama, ""),
		sldkMati: bukaDB(t, sldkNama, "127.0.0.1:1"), // tidak ada yang mendengarkan: koneksi ditolak seketika
	}
}

func (l *lingkunganFallback) jalankan(t *testing.T, sldk *sql.DB, mode string) {
	t.Helper()
	m := digitalisasi.NewManager(l.pasti, sldk)
	m.PasangDatamart(l.mart, mode)
	if _, err := m.Start(digitalisasi.Keys(), "uji"); err != nil {
		t.Fatalf("Start: %v", err)
	}
	selesai := time.Now().Add(60 * time.Second)
	for m.Active() != nil {
		if time.Now().After(selesai) {
			t.Fatal("sinkronisasi tidak selesai dalam 60 detik")
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// riwayat: dataset -> "status|sumber|pesan" untuk n baris riwayat terbaru yang diurutkan menurut id.
func (l *lingkunganFallback) riwayat(t *testing.T, n int) map[string]string {
	t.Helper()
	r := baca(t, l.srv, l.pastiNama, "SELECT TOP ("+strconv.Itoa(n)+") dataset, status + N'|' + ISNULL(sumber, N'-') + N'|' + ISNULL(pesan, N'') FROM digitalisasi_sync_log ORDER BY id DESC")
	out := map[string]string{}
	for _, b := range r {
		out[b[0]] = b[1]
	}
	return out
}

func (l *lingkunganFallback) jumlah(t *testing.T, tabel string) int64 {
	t.Helper()
	return angka(t, l.srv, l.pastiNama, "SELECT COUNT_BIG(*) FROM "+tabel)
}

func TestFallbackKeDatamartBilaSLDKTidakTerjangkau(t *testing.T) {
	l := siapkanFallback(t)

	l.jalankan(t, l.sldkMati, digitalisasi.ModeOtomatis)

	// ketujuh dataset sukses dari datamart, dan isi tabel PASTI sama dengan hasil query asli dari SLDK palsu
	for dataset, status := range l.riwayat(t, 7) {
		if !strings.HasPrefix(status, "sukses|datamart|") || !strings.Contains(status, "dari datamart, data per") || !strings.Contains(status, "SLDK tidak dapat dihubungi") {
			t.Errorf("%s: %s", dataset, status)
		}
		if strings.Contains(status, "127.0.0.1") || strings.Contains(status, "refused") || strings.Contains(status, "dial") {
			t.Errorf("%s: pesan riwayat memuat galat mentah: %s", dataset, status)
		}
	}
	for _, ds := range digitalisasi.Datasets {
		q, err := ds.Query()
		if err != nil {
			t.Fatal(err)
		}
		if want, got := int64(len(baca(t, l.srv, l.sldkNama, q))), l.jumlah(t, ds.Table); got != want || want == 0 {
			t.Errorf("%s: tabel PASTI %d baris, SLDK palsu menghasilkan %d", ds.Key, got, want)
		}
	}
}

func TestFallbackTidakMemundurkanDataPASTIYangLebihBaru(t *testing.T) {
	l := siapkanFallback(t)
	// Jejak datamart berpresisi detik (DATETIME2(0) membulatkan), jadi beri jarak nyata antara pemasangan datamart dan sinkronisasi pertama agar urutannya tegas.
	time.Sleep(2200 * time.Millisecond)
	l.jalankan(t, l.sldkMati, digitalisasi.ModeOtomatis) // pertama kali: datamart dipakai (belum ada data PASTI)
	sebelum := l.jumlah(t, "DIGITALISASI_TANAH")

	// SLDK masih mati dan datamart belum disegarkan sejak itu: datamart tidak lebih baru daripada data PASTI, jadi tidak boleh dipakai lagi
	time.Sleep(1200 * time.Millisecond)
	l.jalankan(t, l.sldkMati, digitalisasi.ModeOtomatis)
	r := l.riwayat(t, 7)
	for dataset, status := range r {
		if !strings.HasPrefix(status, "gagal|") || !strings.Contains(status, "datamart tidak dipakai") || !strings.Contains(status, "lebih lama daripada data PASTI") {
			t.Errorf("%s: %s", dataset, status)
		}
	}
	if got := l.jumlah(t, "DIGITALISASI_TANAH"); got != sebelum {
		t.Errorf("data PASTI berubah walau datamart ditolak: %d -> %d", sebelum, got)
	}

	// datamart disegarkan dari SLDK (mis. oleh job terjadwal), lalu sinkronisasi cadangan diizinkan lagi
	time.Sleep(1200 * time.Millisecond)
	if err := ekse(t, l.srv, l.martNama, "EXEC dbo.datamart_sldk_segarkan;"); err != nil {
		t.Fatalf("penyegaran datamart: %v", err)
	}
	l.jalankan(t, l.sldkMati, digitalisasi.ModeOtomatis)
	for dataset, status := range l.riwayat(t, 7) {
		if !strings.HasPrefix(status, "sukses|datamart|") {
			t.Errorf("setelah datamart disegarkan, %s: %s", dataset, status)
		}
	}
}

func TestSLDKSehatDipakaiLebihDulu(t *testing.T) {
	l := siapkanFallback(t)
	l.jalankan(t, l.sldk, digitalisasi.ModeOtomatis)
	for dataset, status := range l.riwayat(t, 7) {
		if !strings.HasPrefix(status, "sukses|sldk|") || strings.Contains(status, "datamart") {
			t.Errorf("%s: %s", dataset, status)
		}
	}
}

func TestModeSLDKTidakBeralihWalauDatamartAda(t *testing.T) {
	l := siapkanFallback(t)
	l.jalankan(t, l.sldkMati, digitalisasi.ModeSLDK)
	for dataset, status := range l.riwayat(t, 7) {
		if !strings.HasPrefix(status, "gagal|sldk|") {
			t.Errorf("%s: %s", dataset, status)
		}
	}
	if n := l.jumlah(t, "DIGITALISASI_TANAH"); n != 0 {
		t.Errorf("tidak boleh ada data bila SLDK gagal dan datamart tidak diizinkan: %d baris", n)
	}
}

func TestModeDatamartMembacaDatamartWalauSLDKSehat(t *testing.T) {
	l := siapkanFallback(t)
	l.jalankan(t, l.sldk, digitalisasi.ModeDatamart)
	for dataset, status := range l.riwayat(t, 7) {
		if !strings.HasPrefix(status, "sukses|datamart|") {
			t.Errorf("%s: %s", dataset, status)
		}
	}
}

func TestInfoSumberMembacaJejakDatamartSungguhan(t *testing.T) {
	l := siapkanFallback(t)
	m := digitalisasi.NewManager(l.pasti, l.sldkMati)
	m.PasangDatamart(l.mart, digitalisasi.ModeOtomatis)
	i := m.InfoSumber(context.Background())
	if !i.Tersedia || !i.Cadangan || !i.DatamartTerjangkau || i.DatamartDisegarkan == nil || time.Since(*i.DatamartDisegarkan) > time.Minute {
		t.Errorf("info = %+v", i)
	}
}
