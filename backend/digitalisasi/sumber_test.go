package digitalisasi

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"pasti-v3-backend/internal/fakesql"
)

// Tes pemilihan sumber data sinkronisasi (SLDK, dan datamart sebagai cadangan): kapan beralih, kapan TIDAK, dan pengaman anti-mundur. Tanpa database sungguhan.

const rahasiaServer = "server-rahasia.kemenkeu.local"

type sumberEnv struct {
	*managerEnv
	martDB   *sql.DB
	mart     *fakesql.DB
	perilaku map[string]func(ctx context.Context, ds Dataset) (Result, error) // "sldk" | "datamart"

	terakhirPASTI *time.Time // mulai sinkronisasi sukses terakhir (nil = belum pernah)
	martSegar     *time.Time // awal penyegaran datamart terakhir (nil = belum pernah)
	galatMart     error      // galat saat memeriksa kesegaran datamart
	martDiperiksa int
	panggilan     []string // "sumber:dataset"
}

func newSumberEnv(t *testing.T, mode string, adaMart bool) *sumberEnv {
	t.Helper()
	base := newManagerEnv(t, ok(5))
	e := &sumberEnv{managerEnv: base, perilaku: map[string]func(context.Context, Dataset) (Result, error){}}
	var nextID int64
	base.pasti.OnQuery = func(ctx context.Context, q string, args []driver.NamedValue) ([]string, [][]driver.Value, error) {
		e.mu.Lock()
		defer e.mu.Unlock()
		switch {
		case strings.HasPrefix(q, "INSERT INTO digitalisasi_sync_log"):
			nextID++
			return []string{"id"}, [][]driver.Value{{nextID}}, nil
		case strings.HasPrefix(q, "SELECT TOP (1) mulai FROM digitalisasi_sync_log"):
			if e.terakhirPASTI == nil {
				return []string{"mulai"}, nil, nil
			}
			return []string{"mulai"}, [][]driver.Value{{*e.terakhirPASTI}}, nil
		}
		return nil, nil, fmt.Errorf("query tak terduga: %s", q)
	}
	if adaMart {
		e.martDB, e.mart = fakesql.New(t)
		e.mart.OnQuery = func(ctx context.Context, q string, args []driver.NamedValue) ([]string, [][]driver.Value, error) {
			e.mu.Lock()
			defer e.mu.Unlock()
			if !strings.Contains(q, "dbo.datamart_sldk_run") {
				return nil, nil, fmt.Errorf("query tak terduga ke datamart: %s", q)
			}
			e.martDiperiksa++
			if e.galatMart != nil {
				return nil, nil, e.galatMart
			}
			if e.martSegar == nil {
				return []string{"mulai"}, [][]driver.Value{{nil}}, nil
			}
			return []string{"mulai"}, [][]driver.Value{{*e.martSegar}}, nil
		}
		base.m.PasangDatamart(e.martDB, mode)
	} else {
		base.m.PasangDatamart(nil, mode)
	}
	base.m.run = func(ctx context.Context, src, _ *sql.DB, ds Dataset, opt Options) (Result, error) {
		nama := "?"
		switch {
		case src == base.m.sldk:
			nama = "sldk"
		case src != nil && src == e.martDB:
			nama = "datamart"
		}
		e.mu.Lock()
		e.panggilan = append(e.panggilan, nama+":"+ds.Key)
		fn := e.perilaku[nama]
		e.mu.Unlock()
		if fn == nil {
			return Result{Rows: 5, WithCoords: 2}, nil
		}
		return fn(ctx, ds)
	}
	return e
}

func (e *sumberEnv) dipanggil() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return strings.Join(e.panggilan, " ")
}

// sumberDicatat: nilai terakhir yang dicatat ke kolom sumber untuk tiap id riwayat (dari UPDATE ... SET sumber).
func (e *sumberEnv) sumberDicatat() map[int64]string {
	out := map[int64]string{}
	for _, ex := range e.pasti.Execs() {
		if strings.HasPrefix(ex.Query, "UPDATE digitalisasi_sync_log SET sumber = @p1") {
			out[ex.Args[1].Value.(int64)] = ex.Args[0].Value.(string)
		}
	}
	return out
}

func koneksiGagal(context.Context, Dataset) (Result, error) {
	return Result{}, &SumberError{Err: fmt.Errorf("tidak bisa terhubung ke SLDK: dial tcp %s:1433: i/o timeout", rahasiaServer), Koneksi: true}
}

func queryGagal(context.Context, Dataset) (Result, error) {
	return Result{}, &SumberError{Err: fmt.Errorf("query SLDK gagal: server %s menutup sambungan", rahasiaServer)}
}

func jam(h int) *time.Time {
	t := time.Date(2026, 10, 20, h, 0, 0, 0, time.UTC)
	return &t
}

func TestBeralihKeDatamartBilaSambunganSLDKGagalDanDatamartLebihBaru(t *testing.T) {
	e := newSumberEnv(t, ModeOtomatis, true)
	e.perilaku["sldk"] = koneksiGagal
	e.terakhirPASTI, e.martSegar = jam(1), jam(3)

	if _, err := e.m.Start([]string{"satker", "tanah", "rusunara"}, "admin1"); err != nil {
		t.Fatal(err)
	}
	e.waitIdle(t)

	// sambungan SLDK gagal sekali; dataset berikutnya langsung ke datamart tanpa menunggu SLDK lagi
	if got := e.dipanggil(); got != "sldk:satker datamart:satker datamart:tanah datamart:rusunara" {
		t.Fatalf("urutan = %s", got)
	}
	status, msg := e.finals()
	for id := int64(1); id <= 3; id++ {
		if status[id] != StatusSukses {
			t.Fatalf("status %d = %q (%v)", id, status[id], msg)
		}
		if !strings.Contains(msg[id], "dari datamart, data per 20/10/2026 10.00 WIB") || !strings.Contains(msg[id], "SLDK tidak dapat dihubungi") {
			t.Errorf("pesan %d = %q", id, msg[id])
		}
		if strings.Contains(msg[id], rahasiaServer) || strings.Contains(msg[id], "i/o timeout") {
			t.Errorf("pesan riwayat memuat teks galat mentah: %q", msg[id])
		}
	}
	if s := e.sumberDicatat(); s[1] != SumberDatamart || s[2] != SumberDatamart || s[3] != SumberDatamart {
		t.Errorf("sumber dicatat = %v", s)
	}
}

func TestQueryAjaSLDKGagalTidakMematikanSLDKUntukDatasetBerikutnya(t *testing.T) {
	e := newSumberEnv(t, ModeOtomatis, true)
	e.perilaku["sldk"] = func(ctx context.Context, ds Dataset) (Result, error) {
		if ds.Key == "satker" {
			return queryGagal(ctx, ds)
		}
		return Result{Rows: 5, WithCoords: 2}, nil
	}
	e.terakhirPASTI, e.martSegar = nil, jam(3)

	e.m.Start([]string{"satker", "tanah"}, "admin1")
	e.waitIdle(t)

	if got := e.dipanggil(); got != "sldk:satker datamart:satker sldk:tanah" {
		t.Fatalf("urutan = %s (SLDK tetap dicoba untuk dataset berikutnya bila hanya satu query yang gagal)", got)
	}
	_, msg := e.finals()
	if !strings.Contains(msg[1], "SLDK gagal dibaca") || strings.Contains(msg[1], rahasiaServer) {
		t.Errorf("pesan = %q", msg[1])
	}
	if s := e.sumberDicatat(); s[1] != SumberDatamart || s[2] != SumberSLDK {
		t.Errorf("sumber dicatat = %v", s)
	}
}

func TestDatamartLebihLamaDariDataPASTITidakDipakai(t *testing.T) {
	e := newSumberEnv(t, ModeOtomatis, true)
	e.perilaku["sldk"] = koneksiGagal
	e.terakhirPASTI, e.martSegar = jam(5), jam(3) // PASTI disinkronkan setelah datamart disegarkan

	e.m.Start([]string{"tanah"}, "admin1")
	e.waitIdle(t)

	if got := e.dipanggil(); got != "sldk:tanah" {
		t.Fatalf("datamart yang lebih lama tidak boleh dibaca: %s", got)
	}
	status, msg := e.finals()
	if status[1] != StatusGagal || !strings.Contains(msg[1], "datamart tidak dipakai") || !strings.Contains(msg[1], "lebih lama daripada data PASTI") {
		t.Errorf("status %q pesan %q", status[1], msg[1])
	}
	if e.sumberDicatat()[1] == SumberDatamart {
		t.Error("sumber tidak boleh tercatat datamart bila tidak dibaca")
	}
}

func TestDatamartDipakaiBilaDatasetBelumPernahDisinkronkanWalauLama(t *testing.T) {
	e := newSumberEnv(t, ModeOtomatis, true)
	e.perilaku["sldk"] = koneksiGagal
	e.terakhirPASTI, e.martSegar = nil, jam(1)

	e.m.Start([]string{"tanah"}, "admin1")
	e.waitIdle(t)

	if got := e.dipanggil(); got != "sldk:tanah datamart:tanah" {
		t.Fatalf("urutan = %s", got)
	}
	if status, _ := e.finals(); status[1] != StatusSukses {
		t.Errorf("status = %v", status)
	}
}

func TestKesegaranDatamartTakDiketahuiMenolakPemakaian(t *testing.T) {
	for nama, atur := range map[string]func(e *sumberEnv){
		"jejak tidak terbaca (tabel tidak ada)": func(e *sumberEnv) { e.galatMart = errors.New("Invalid object name 'dbo.datamart_sldk_run'") },
		"belum pernah berhasil disegarkan":      func(e *sumberEnv) { e.martSegar = nil },
	} {
		t.Run(nama, func(t *testing.T) {
			e := newSumberEnv(t, ModeOtomatis, true)
			e.perilaku["sldk"] = koneksiGagal
			atur(e)
			e.m.Start([]string{"tanah"}, "admin1")
			e.waitIdle(t)
			if got := e.dipanggil(); got != "sldk:tanah" {
				t.Fatalf("datamart tak terverifikasi tidak boleh dibaca: %s", got)
			}
			if status, msg := e.finals(); status[1] != StatusGagal || !strings.Contains(msg[1], "datamart tidak dipakai") {
				t.Errorf("status %q pesan %q", status[1], msg[1])
			}
		})
	}
}

func TestGalatBukanBacaSumberTidakMemicuPeralihan(t *testing.T) {
	e := newSumberEnv(t, ModeOtomatis, true)
	e.martSegar = jam(3)
	e.perilaku["sldk"] = func(context.Context, Dataset) (Result, error) {
		// galat data/tulis yang sama di sumber mana pun (bukan SumberError)
		return Result{}, errors.New("hasil query tidak memuat kolom: Kode_Satker")
	}
	e.m.Start([]string{"tanah"}, "admin1")
	e.waitIdle(t)

	if got := e.dipanggil(); got != "sldk:tanah" {
		t.Fatalf("galat non-sumber tidak boleh beralih ke datamart: %s", got)
	}
	if status, msg := e.finals(); status[1] != StatusGagal || !strings.Contains(msg[1], "hasil query tidak memuat kolom") {
		t.Errorf("status %q pesan %q", status[1], msg[1])
	}
	if e.martDiperiksa != 0 {
		t.Errorf("datamart tidak perlu diperiksa: %d", e.martDiperiksa)
	}
}

func TestPembatalanTidakMemicuPeralihan(t *testing.T) {
	e := newSumberEnv(t, ModeOtomatis, true)
	e.martSegar = jam(3)
	mulai := make(chan struct{})
	e.perilaku["sldk"] = func(ctx context.Context, ds Dataset) (Result, error) {
		close(mulai)
		<-ctx.Done()
		return Result{}, &SumberError{Err: fmt.Errorf("query SLDK gagal: %w", ctx.Err())}
	}
	e.m.Start([]string{"tanah"}, "admin1")
	<-mulai
	e.m.Cancel()
	e.waitIdle(t)

	if got := e.dipanggil(); got != "sldk:tanah" {
		t.Fatalf("pembatalan tidak boleh beralih ke datamart: %s", got)
	}
	if status, _ := e.finals(); status[1] != StatusDibatalkan {
		t.Errorf("status = %v", status)
	}
}

func TestModeSLDKTidakPernahMembacaDatamart(t *testing.T) {
	e := newSumberEnv(t, ModeSLDK, true)
	e.perilaku["sldk"] = koneksiGagal
	e.terakhirPASTI, e.martSegar = nil, jam(3)
	e.m.Start([]string{"tanah"}, "admin1")
	e.waitIdle(t)
	if got := e.dipanggil(); got != "sldk:tanah" {
		t.Fatalf("mode sldk membaca %s", got)
	}
	if status, _ := e.finals(); status[1] != StatusGagal {
		t.Errorf("status = %v", status)
	}
}

func TestModeDatamartHanyaMembacaDatamartTanpaPemeriksaanLebihBaru(t *testing.T) {
	e := newSumberEnv(t, ModeDatamart, true)
	e.perilaku["sldk"] = func(context.Context, Dataset) (Result, error) {
		t.Error("SLDK tidak boleh dibaca pada mode datamart")
		return Result{}, nil
	}
	e.terakhirPASTI, e.martSegar = jam(5), jam(3) // lebih lama dari data PASTI, tetapi pilihan sengaja operator

	e.m.Start([]string{"tanah"}, "admin1")
	e.waitIdle(t)

	if got := e.dipanggil(); got != "datamart:tanah" {
		t.Fatalf("urutan = %s", got)
	}
	status, msg := e.finals()
	if status[1] != StatusSukses || !strings.Contains(msg[1], "dari datamart, data per") || strings.Contains(msg[1], "SLDK tidak dapat dihubungi") {
		t.Errorf("status %q pesan %q", status[1], msg[1])
	}
}

func TestTanpaSLDKDatamartMenjadiSatuSatunyaSumber(t *testing.T) {
	e := newSumberEnv(t, ModeOtomatis, true)
	e.m.sldk = nil
	e.martSegar = jam(3)
	if !e.m.AdaSumber() {
		t.Fatal("datamart saja sudah cukup sebagai sumber")
	}
	if _, err := e.m.Start([]string{"tanah"}, "admin1"); err != nil {
		t.Fatal(err)
	}
	e.waitIdle(t)
	if got := e.dipanggil(); got != "datamart:tanah" {
		t.Errorf("urutan = %s", got)
	}
}

func TestStartMenolakBilaTidakAdaSumber(t *testing.T) {
	for nama, atur := range map[string]func(e *sumberEnv){
		"mode datamart tanpa datamart":  func(e *sumberEnv) {},
		"tanpa SLDK dan tanpa datamart": func(e *sumberEnv) { e.m.sldk = nil },
	} {
		t.Run(nama, func(t *testing.T) {
			mode := ModeOtomatis
			if strings.HasPrefix(nama, "mode datamart") {
				mode = ModeDatamart
			}
			e := newSumberEnv(t, mode, false)
			atur(e)
			if _, err := e.m.Start([]string{"tanah"}, "admin1"); !errors.Is(err, ErrNoSLDK) {
				t.Errorf("err = %v, want ErrNoSLDK", err)
			}
		})
	}
	// mode sldk dengan hanya datamart terkonfigurasi: tidak ada sumber yang sesuai mode
	e := newSumberEnv(t, ModeSLDK, true)
	e.m.sldk = nil
	if _, err := e.m.Start([]string{"tanah"}, "admin1"); !errors.Is(err, ErrNoSLDK) {
		t.Errorf("mode sldk tanpa SLDK: err = %v", err)
	}
}

func TestKeduaSumberGagalMenyebutKeduanya(t *testing.T) {
	e := newSumberEnv(t, ModeOtomatis, true)
	e.perilaku["sldk"] = koneksiGagal
	e.perilaku["datamart"] = func(context.Context, Dataset) (Result, error) {
		return Result{}, &SumberError{Err: errors.New("query datamart gagal: Invalid object name 'DJKN.SIMAN2_M_ASET'")}
	}
	e.martSegar = jam(3)
	e.m.Start([]string{"tanah"}, "admin1")
	e.waitIdle(t)

	status, msg := e.finals()
	if status[1] != StatusGagal || !strings.Contains(msg[1], "SLDK: ") || !strings.Contains(msg[1], "datamart: query datamart gagal") {
		t.Errorf("status %q pesan %q", status[1], msg[1])
	}
}

func TestInfoSumberMelaporkanKonfigurasiDanKesegaranDenganCache(t *testing.T) {
	e := newSumberEnv(t, ModeOtomatis, true)
	e.martSegar = jam(3)
	ctx := context.Background()

	i := e.m.InfoSumber(ctx)
	if i.Mode != ModeOtomatis || !i.SLDK || !i.Datamart || !i.Tersedia || !i.Cadangan || !i.DatamartTerjangkau || i.DatamartDisegarkan == nil || !i.DatamartDisegarkan.Equal(*jam(3)) {
		t.Fatalf("info = %+v", i)
	}
	e.m.InfoSumber(ctx)
	e.m.InfoSumber(ctx)
	if e.martDiperiksa != 1 {
		t.Errorf("polling halaman tidak boleh membebani datamart: %d pemeriksaan", e.martDiperiksa)
	}

	// datamart tidak terjangkau
	e2 := newSumberEnv(t, ModeOtomatis, true)
	e2.galatMart = errors.New("koneksi ditolak")
	if i := e2.m.InfoSumber(ctx); !i.Datamart || i.DatamartTerjangkau || i.DatamartDisegarkan != nil || !i.Tersedia {
		t.Errorf("info datamart mati = %+v", i)
	}

	// tanpa datamart
	e3 := newSumberEnv(t, ModeOtomatis, false)
	if i := e3.m.InfoSumber(ctx); i.Datamart || i.Cadangan || !i.Tersedia || i.DatamartDisegarkan != nil {
		t.Errorf("info tanpa datamart = %+v", i)
	}
	// mode sldk: datamart ada tetapi bukan cadangan
	e4 := newSumberEnv(t, ModeSLDK, true)
	e4.martSegar = jam(3)
	if i := e4.m.InfoSumber(ctx); i.Cadangan || !i.Tersedia {
		t.Errorf("info mode sldk = %+v", i)
	}
}

func TestNormalisasiModeSumber(t *testing.T) {
	for in, want := range map[string]string{"": ModeOtomatis, "otomatis": ModeOtomatis, "sldk": ModeSLDK, "datamart": ModeDatamart, "DATAMART": ModeOtomatis, "lain": ModeOtomatis} {
		if got := NormalisasiMode(in); got != want {
			t.Errorf("NormalisasiMode(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestPenjadwalMengulangLebihCepatSinkronisasiDariDatamart(t *testing.T) {
	sukses := time.Date(2026, 10, 20, 3, 0, 0, 0, time.UTC)
	sumber := func(s string) *string { return &s }
	entri := func(sumberData *string) (latest, lastOK map[string]LogEntry) {
		e := LogEntry{ID: 1, Dataset: "tanah", Status: StatusSukses, Dibuat: sukses, Mulai: &sukses, Selesai: &sukses, Sumber: sumberData}
		return map[string]LogEntry{"tanah": e}, map[string]LogEntry{"tanah": e}
	}
	j := NewJadwal(true, 7, 1, 5)
	j.UlangBilaDatamart = true

	latest, lastOK := entri(sumber(SumberDatamart))
	if got, want := j.saatJatuhTempo(sukses, "tanah", latest, lastOK), sukses.Add(jedaCoba); !got.Equal(want) {
		t.Errorf("dari datamart: jatuh tempo %v, want %v (jeda coba, bukan 7 hari)", got, want)
	}
	for nama, s := range map[string]*string{"dari SLDK": sumber(SumberSLDK), "riwayat lama tanpa sumber": nil} {
		latest, lastOK = entri(s)
		if got, want := j.saatJatuhTempo(sukses, "tanah", latest, lastOK), sukses.Add(7*24*time.Hour); !got.Equal(want) {
			t.Errorf("%s: jatuh tempo %v, want %v", nama, got, want)
		}
	}
	// SLDK memang tidak dipakai (mode datamart): sinkronisasi dari datamart mengikuti interval biasa
	j.UlangBilaDatamart = false
	latest, lastOK = entri(sumber(SumberDatamart))
	if got, want := j.saatJatuhTempo(sukses, "tanah", latest, lastOK), sukses.Add(7*24*time.Hour); !got.Equal(want) {
		t.Errorf("tanpa pengulangan: jatuh tempo %v, want %v", got, want)
	}
}

func TestMulaiPenjadwalMengisiUlangBilaDatamartHanyaBilaSLDKDipakai(t *testing.T) {
	cases := []struct {
		nama string
		mode string
		sldk bool
		want bool
	}{
		{"otomatis dengan SLDK", ModeOtomatis, true, true},
		{"mode datamart", ModeDatamart, true, false},
		{"mode sldk", ModeSLDK, true, false},
		{"otomatis tanpa SLDK", ModeOtomatis, false, false},
	}
	for _, c := range cases {
		e := newSumberEnv(t, c.mode, true)
		if !c.sldk {
			e.m.sldk = nil
		}
		e.m.MulaiPenjadwal(context.Background(), NewJadwal(false, 7, 1, 5))
		if got := e.m.Jadwal().UlangBilaDatamart; got != c.want {
			t.Errorf("%s: UlangBilaDatamart = %v, want %v", c.nama, got, c.want)
		}
	}
}

func TestPenjadwalBerjalanBilaHanyaDatamartAda(t *testing.T) {
	e := newSumberEnv(t, ModeOtomatis, true)
	e.m.sldk = nil
	e.m.MulaiPenjadwal(context.Background(), NewJadwal(false, 7, 1, 5))
	if info := e.m.InfoOtomatis(time.Now(), nil, nil); info.Aktif {
		t.Error("jadwal dimatikan, harus tidak aktif")
	}
	e.m.mu.Lock()
	e.m.jadwal = NewJadwal(true, 7, 1, 5)
	e.m.mu.Unlock()
	if info := e.m.InfoOtomatis(time.Now(), nil, nil); !info.Aktif {
		t.Error("sinkronisasi otomatis harus aktif walau hanya datamart yang tersedia")
	}
}

// ---------------------------------------------------------------- Sync: galat sumber ditandai, nama sumber ikut pesan

func TestSyncMenandaiGalatBacaSumberDanMemakaiNamaSumber(t *testing.T) {
	ds, _ := ByKey("satker")
	pastiDB, _ := fakesql.New(t)

	// sambungan tidak dapat dibuka (handle sudah ditutup): SumberError dengan Koneksi
	tutup, _ := fakesql.New(t)
	tutup.Close()
	_, err := Sync(context.Background(), tutup, pastiDB, ds, Options{NamaSumber: "datamart"})
	var se *SumberError
	if !errors.As(err, &se) || !se.Koneksi {
		t.Fatalf("err = %v (%T), want SumberError Koneksi", err, err)
	}
	if !strings.Contains(err.Error(), "tidak bisa terhubung ke datamart") {
		t.Errorf("pesan harus memakai nama sumber: %v", err)
	}
	_, err = Sync(context.Background(), tutup, pastiDB, ds, Options{})
	if err == nil || !strings.Contains(err.Error(), "tidak bisa terhubung ke SLDK") {
		t.Errorf("nama bawaan harus SLDK: %v", err)
	}

	// query ditolak: SumberError tanpa Koneksi
	sldk, s := fakesql.New(t)
	s.OnQuery = func(context.Context, string, []driver.NamedValue) ([]string, [][]driver.Value, error) {
		return nil, nil, errors.New("permission denied")
	}
	_, err = Sync(context.Background(), sldk, pastiDB, ds, Options{})
	if !errors.As(err, &se) || se.Koneksi || !strings.Contains(err.Error(), "query SLDK gagal") {
		t.Errorf("err = %v", err)
	}

	// galat yang sama di sumber mana pun (kolom hasil hilang) BUKAN SumberError
	s.OnQuery = func(context.Context, string, []driver.NamedValue) ([]string, [][]driver.Value, error) {
		return []string{"kolom_lain"}, [][]driver.Value{{"x"}}, nil
	}
	_, err = Sync(context.Background(), sldk, pastiDB, ds, Options{})
	if err == nil || errors.As(err, &se) {
		t.Errorf("kolom hasil hilang bukan galat baca sumber: %v", err)
	}
}
