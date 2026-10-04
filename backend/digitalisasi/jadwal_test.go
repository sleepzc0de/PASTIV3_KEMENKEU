package digitalisasi

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

// wib membuat waktu pada zona WIB; tes memakai tanggal tetap supaya hasilnya tidak bergantung pada jam mesin.
func wib(y int, m time.Month, d, h, mnt int) time.Time {
	return time.Date(y, m, d, h, mnt, 0, 0, zonaWIB)
}

func ptr(t time.Time) *time.Time { return &t }

// riwayat membuat peta riwayat terakhir (latest) dan sukses terakhir (lastOK) untuk semua dataset.
func riwayatSemua(sukses time.Time, percobaan time.Time) (latest, lastOK map[string]LogEntry) {
	latest, lastOK = map[string]LogEntry{}, map[string]LogEntry{}
	for i, ds := range Datasets {
		e := LogEntry{ID: int64(i + 1), Dataset: ds.Key, Status: StatusSukses, Dibuat: percobaan, Mulai: ptr(percobaan), Selesai: ptr(sukses)}
		latest[ds.Key] = e
		lastOK[ds.Key] = e
	}
	return latest, lastOK
}

func TestNewJadwalMenggantiNilaiDiLuarRentang(t *testing.T) {
	j := NewJadwal(true, 7, 1, 5)
	if !j.Aktif || j.Interval != 7*24*time.Hour || j.JamMulai != 1 || j.JamAkhir != 5 {
		t.Errorf("jadwal = %+v", j)
	}
	for _, c := range []struct{ hari, mulai, akhir int }{{0, 1, 5}, {-3, 1, 5}, {400, 1, 5}} {
		if got := NewJadwal(true, c.hari, c.mulai, c.akhir).Interval; got != BawaanIntervalHari*24*time.Hour {
			t.Errorf("interval %d hari harus kembali ke bawaan, dapat %s", c.hari, got)
		}
	}
	for _, c := range []struct{ mulai, akhir int }{{-1, 5}, {1, 24}, {25, 3}} {
		j := NewJadwal(true, 7, c.mulai, c.akhir)
		if j.JamMulai != BawaanJamMulai || j.JamAkhir != BawaanJamAkhir {
			t.Errorf("jam %d-%d harus kembali ke bawaan, dapat %d-%d", c.mulai, c.akhir, j.JamMulai, j.JamAkhir)
		}
	}
	if NewJadwal(false, 7, 1, 5).Aktif {
		t.Error("Aktif=false harus dipertahankan")
	}
}

func TestDalamJendelaMemakaiWaktuWIB(t *testing.T) {
	j := NewJadwal(true, 7, 1, 5)
	for _, c := range []struct {
		jam, menit int
		want       bool
	}{{0, 59, false}, {1, 0, true}, {3, 30, true}, {4, 59, true}, {5, 0, false}, {14, 0, false}, {23, 59, false}} {
		// Diberikan sebagai UTC: jam server (UTC di dalam container) tidak boleh memengaruhi jendela WIB.
		utc := wib(2026, 10, 10, c.jam, c.menit).UTC()
		if got := j.DalamJendela(utc); got != c.want {
			t.Errorf("%02d.%02d WIB (%s) dalam jendela = %v, want %v", c.jam, c.menit, utc.Format("15:04 MST"), got, c.want)
		}
	}
	// Jendela yang melewati tengah malam.
	malam := NewJadwal(true, 7, 22, 4)
	for _, c := range []struct {
		jam  int
		want bool
	}{{21, false}, {22, true}, {23, true}, {0, true}, {3, true}, {4, false}, {12, false}} {
		if got := malam.DalamJendela(wib(2026, 10, 10, c.jam, 0)); got != c.want {
			t.Errorf("jendela 22-04, jam %d = %v, want %v", c.jam, got, c.want)
		}
	}
	// Mulai = akhir: sepanjang hari.
	for h := 0; h < 24; h++ {
		if !NewJadwal(true, 7, 6, 6).DalamJendela(wib(2026, 10, 10, h, 0)) {
			t.Errorf("jam %d harus diizinkan bila mulai = akhir", h)
		}
	}
}

func TestJatuhTempoMenurutInterval(t *testing.T) {
	j := NewJadwal(true, 7, 1, 5)
	now := wib(2026, 10, 10, 2, 0)

	// Belum pernah disinkronkan: semua jatuh tempo, urut menurut Datasets.
	if got := j.JatuhTempo(now, nil, nil); len(got) != len(Datasets) || got[0] != Datasets[0].Key {
		t.Errorf("belum pernah: %v", got)
	}

	// Sukses kemarin (manual maupun otomatis): tidak jatuh tempo.
	latest, lastOK := riwayatSemua(now.Add(-24*time.Hour), now.Add(-25*time.Hour))
	if got := j.JatuhTempo(now, latest, lastOK); len(got) != 0 {
		t.Errorf("sukses kemarin: %v", got)
	}

	// Sukses tepat 7 hari lalu: jatuh tempo; sedikit kurang dari 7 hari: belum.
	latest, lastOK = riwayatSemua(now.Add(-7*24*time.Hour), now.Add(-7*24*time.Hour-time.Hour))
	if got := j.JatuhTempo(now, latest, lastOK); len(got) != len(Datasets) {
		t.Errorf("tepat 7 hari: %v", got)
	}
	latest, lastOK = riwayatSemua(now.Add(-7*24*time.Hour+time.Minute), now.Add(-7*24*time.Hour))
	if got := j.JatuhTempo(now, latest, lastOK); len(got) != 0 {
		t.Errorf("kurang dari 7 hari: %v", got)
	}

	// Hanya sebagian yang lama: yang lama saja yang jatuh tempo.
	latest, lastOK = riwayatSemua(now.Add(-24*time.Hour), now.Add(-25*time.Hour))
	tua := Datasets[len(Datasets)-1].Key
	e := lastOK[tua]
	e.Selesai = ptr(now.Add(-9 * 24 * time.Hour))
	lastOK[tua] = e
	l := latest[tua]
	l.Dibuat = now.Add(-9*24*time.Hour - time.Hour)
	latest[tua] = l
	if got := j.JatuhTempo(now, latest, lastOK); len(got) != 1 || got[0] != tua {
		t.Errorf("hanya %s yang lama: %v", tua, got)
	}
}

func TestJatuhTempoMenahanPercobaanUlangSetelahGagal(t *testing.T) {
	j := NewJadwal(true, 7, 1, 5)
	now := wib(2026, 10, 10, 2, 0)
	key := Datasets[0].Key
	lama := map[string]LogEntry{key: {Dataset: key, Status: StatusSukses, Dibuat: now.Add(-10 * 24 * time.Hour), Selesai: ptr(now.Add(-10 * 24 * time.Hour))}}

	gagal := func(sejak time.Duration) map[string]LogEntry {
		return map[string]LogEntry{key: {ID: 99, Dataset: key, Status: StatusGagal, Dibuat: now.Add(-sejak), Selesai: ptr(now.Add(-sejak + time.Hour))}}
	}
	// Gagal 1 jam lalu (mis. server dimulai ulang di tengah sinkronisasi): jangan langsung dicoba lagi.
	for _, d := range j.JatuhTempo(now, gagal(time.Hour), lama) {
		if d == key {
			t.Error("percobaan yang gagal 1 jam lalu tidak boleh langsung diulang")
		}
	}
	// Gagal 21 jam lalu: dicoba lagi (malam berikutnya).
	found := false
	for _, d := range j.JatuhTempo(now, gagal(21*time.Hour), lama) {
		found = found || d == key
	}
	if !found {
		t.Error("percobaan yang gagal 21 jam lalu harus diulang")
	}
	// Percobaan yang masih antri/berjalan juga dihitung sebagai percobaan.
	sedang := map[string]LogEntry{key: {ID: 100, Dataset: key, Status: StatusBerjalan, Dibuat: now.Add(-30 * time.Minute)}}
	for _, d := range j.JatuhTempo(now, sedang, lama) {
		if d == key {
			t.Error("dataset yang sedang berjalan tidak boleh jatuh tempo")
		}
	}
}

func TestBerikutnyaDiselarasKeJendela(t *testing.T) {
	j := NewJadwal(true, 7, 1, 5)

	// Sukses terakhir Sabtu 3 Okt 03.00 WIB: 7 hari kemudian Sabtu 10 Okt 03.00 WIB, sudah di dalam jendela.
	sukses := wib(2026, 10, 3, 3, 0)
	latest, lastOK := riwayatSemua(sukses, sukses.Add(-time.Hour))
	if got := j.Berikutnya(wib(2026, 10, 5, 12, 0), latest, lastOK); got == nil || !got.Equal(wib(2026, 10, 10, 3, 0)) {
		t.Errorf("berikutnya = %v, want 10 Okt 03.00 WIB", got)
	}

	// Sukses terakhir pukul 10.00: 7 hari kemudian jam 10.00 (di luar jendela) -> pukul 01.00 hari berikutnya.
	sukses = wib(2026, 10, 3, 10, 0)
	latest, lastOK = riwayatSemua(sukses, sukses.Add(-time.Hour))
	if got := j.Berikutnya(wib(2026, 10, 5, 12, 0), latest, lastOK); got == nil || !got.Equal(wib(2026, 10, 11, 1, 0)) {
		t.Errorf("berikutnya = %v, want 11 Okt 01.00 WIB", got)
	}

	// Sudah lewat jatuh tempo, sekarang siang: malam ini pukul 01.00 (hari berikutnya).
	if got := j.Berikutnya(wib(2026, 10, 20, 14, 0), latest, lastOK); got == nil || !got.Equal(wib(2026, 10, 21, 1, 0)) {
		t.Errorf("lewat tempo siang: %v, want 21 Okt 01.00 WIB", got)
	}
	// Sudah lewat jatuh tempo dan sekarang di dalam jendela: sekarang juga.
	now := wib(2026, 10, 20, 2, 30)
	if got := j.Berikutnya(now, latest, lastOK); got == nil || !got.Equal(now) {
		t.Errorf("lewat tempo dalam jendela: %v, want %v", got, now)
	}
	// Lewat tempo setelah tengah malam sebelum jendela dibuka: pukul 01.00 hari yang sama.
	if got := j.Berikutnya(wib(2026, 10, 20, 0, 10), latest, lastOK); got == nil || !got.Equal(wib(2026, 10, 20, 1, 0)) {
		t.Errorf("lewat tempo 00.10: %v, want 20 Okt 01.00 WIB", got)
	}
	// Belum pernah disinkronkan: dihitung dari sekarang.
	if got := j.Berikutnya(wib(2026, 10, 20, 14, 0), nil, nil); got == nil || !got.Equal(wib(2026, 10, 21, 1, 0)) {
		t.Errorf("belum pernah: %v", got)
	}
	// Tidak aktif: tidak ada jadwal.
	if got := NewJadwal(false, 7, 1, 5).Berikutnya(time.Now(), nil, nil); got != nil {
		t.Errorf("tidak aktif harus nil: %v", got)
	}
}

// ---- penjadwal di atas Manager ----

type envJadwal struct {
	*managerEnv
	mu       sync.Mutex
	latest   map[string]LogEntry
	lastOK   map[string]LogEntry
	galat    error
	inserted []string // nilai dijalankan_oleh tiap baris antrean yang dibuat
}

func logRow(e LogEntry) []driver.Value {
	var mulai, selesai, baris, koord, pesan, oleh driver.Value
	if e.Mulai != nil {
		mulai = *e.Mulai
	}
	if e.Selesai != nil {
		selesai = *e.Selesai
	}
	return []driver.Value{e.ID, e.Dataset, e.Status, e.Dibuat, mulai, selesai, baris, koord, pesan, oleh}
}

func newEnvJadwal(t *testing.T, j Jadwal) *envJadwal {
	t.Helper()
	base := newManagerEnv(t, ok(5))
	e := &envJadwal{managerEnv: base, latest: map[string]LogEntry{}, lastOK: map[string]LogEntry{}}
	var nextID int64
	base.pasti.OnQuery = func(ctx context.Context, q string, args []driver.NamedValue) ([]string, [][]driver.Value, error) {
		e.mu.Lock()
		defer e.mu.Unlock()
		switch {
		case strings.HasPrefix(q, "INSERT INTO digitalisasi_sync_log"):
			nextID++
			e.inserted = append(e.inserted, args[2].Value.(string))
			return []string{"id"}, [][]driver.Value{{nextID}}, nil
		case strings.Contains(q, "ROW_NUMBER"):
			if e.galat != nil {
				return nil, nil, e.galat
			}
			src := e.latest
			if strings.Contains(q, "WHERE status = 'sukses'") {
				src = e.lastOK
			}
			cols := []string{"id", "dataset", "status", "dibuat", "mulai", "selesai", "jumlah_baris", "jumlah_koordinat", "pesan", "dijalankan_oleh"}
			var rows [][]driver.Value
			for _, ds := range Datasets {
				if en, has := src[ds.Key]; has {
					rows = append(rows, logRow(en))
				}
			}
			return cols, rows, nil
		}
		return nil, nil, errors.New("query tak terduga: " + q)
	}
	base.m.mu.Lock()
	base.m.jadwal = j
	base.m.mu.Unlock()
	return e
}

func TestPenjadwalMemulaiSemuaDatasetYangBelumPernahDisinkronkan(t *testing.T) {
	e := newEnvJadwal(t, NewJadwal(true, 7, 1, 5))
	e.m.cekOtomatis(context.Background(), wib(2026, 10, 10, 2, 0))
	e.waitIdle(t)

	if got := len(e.ran()); got != len(Datasets) {
		t.Fatalf("dataset yang dijalankan = %v, want semua (%d)", e.ran(), len(Datasets))
	}
	for _, oleh := range e.inserted {
		if oleh != PengirimOtomatis {
			t.Errorf("dijalankan_oleh = %q, want %q", oleh, PengirimOtomatis)
		}
	}
}

func TestPenjadwalHanyaMenyinkronkanYangJatuhTempo(t *testing.T) {
	e := newEnvJadwal(t, NewJadwal(true, 7, 1, 5))
	now := wib(2026, 10, 10, 2, 0)
	e.latest, e.lastOK = riwayatSemua(now.Add(-24*time.Hour), now.Add(-25*time.Hour))
	tua := Datasets[1].Key
	x := e.lastOK[tua]
	x.Selesai = ptr(now.Add(-8 * 24 * time.Hour))
	e.lastOK[tua] = x
	y := e.latest[tua]
	y.Dibuat = now.Add(-8*24*time.Hour - time.Hour)
	e.latest[tua] = y

	e.m.cekOtomatis(context.Background(), now)
	e.waitIdle(t)
	if got := strings.Join(e.ran(), ","); got != tua {
		t.Errorf("dijalankan = %q, want hanya %q", got, tua)
	}
}

func TestPenjadwalTidakBerjalanDiLuarJendelaAtauBilaTidakAktif(t *testing.T) {
	// Di luar jendela jam (siang hari WIB) walau semua jatuh tempo.
	e := newEnvJadwal(t, NewJadwal(true, 7, 1, 5))
	e.m.cekOtomatis(context.Background(), wib(2026, 10, 10, 14, 0))
	if e.m.Active() != nil || len(e.ran()) != 0 || e.pasti.Count("QUERY") != 0 {
		t.Errorf("di luar jendela tidak boleh melakukan apa pun: %v, %v", e.ran(), e.pasti.Events())
	}
	// Tidak aktif.
	e = newEnvJadwal(t, NewJadwal(false, 7, 1, 5))
	e.m.cekOtomatis(context.Background(), wib(2026, 10, 10, 2, 0))
	if len(e.ran()) != 0 || e.pasti.Count("QUERY") != 0 {
		t.Errorf("tidak aktif tidak boleh melakukan apa pun: %v", e.pasti.Events())
	}
	// Tanpa koneksi SLDK.
	e = newEnvJadwal(t, NewJadwal(true, 7, 1, 5))
	e.m.sldk = nil
	e.m.cekOtomatis(context.Background(), wib(2026, 10, 10, 2, 0))
	if len(e.ran()) != 0 || e.pasti.Count("QUERY") != 0 {
		t.Errorf("tanpa SLDK tidak boleh melakukan apa pun: %v", e.pasti.Events())
	}
}

func TestPenjadwalMengalahPadaSinkronisasiYangSedangBerjalan(t *testing.T) {
	release := make(chan struct{})
	e := newEnvJadwal(t, NewJadwal(true, 7, 1, 5))
	e.m.run = func(ctx context.Context, _, _ *sql.DB, ds Dataset, opt Options) (Result, error) {
		e.managerEnv.mu.Lock()
		e.managerEnv.runs = append(e.managerEnv.runs, ds.Key)
		e.managerEnv.mu.Unlock()
		<-release
		return Result{Rows: 1}, nil
	}
	if _, err := e.m.Start([]string{"satker"}, "admin"); err != nil {
		t.Fatal(err)
	}
	sebelum := e.pasti.Count("QUERY")
	e.m.cekOtomatis(context.Background(), wib(2026, 10, 10, 2, 0))
	if e.pasti.Count("QUERY") != sebelum {
		t.Error("penjadwal tidak boleh membaca riwayat atau memulai apa pun saat sinkronisasi manual berjalan")
	}
	close(release)
	e.waitIdle(t)
	if got := strings.Join(e.ran(), ","); got != "satker" {
		t.Errorf("hanya sinkronisasi manual yang boleh jalan: %s", got)
	}
}

func TestPenjadwalBertahanSaatRiwayatGagalDibaca(t *testing.T) {
	e := newEnvJadwal(t, NewJadwal(true, 7, 1, 5))
	e.galat = errors.New("database mati")
	e.m.cekOtomatis(context.Background(), wib(2026, 10, 10, 2, 0)) // tidak boleh panic
	if e.m.Active() != nil || len(e.ran()) != 0 {
		t.Errorf("galat baca riwayat tidak boleh memulai sinkronisasi: %v", e.ran())
	}
	e.galat = nil
	e.m.cekOtomatis(context.Background(), wib(2026, 10, 10, 2, 15)) // putaran berikutnya pulih
	e.waitIdle(t)
	if len(e.ran()) != len(Datasets) {
		t.Errorf("setelah pulih: %v", e.ran())
	}
}

func TestInfoOtomatis(t *testing.T) {
	e := newEnvJadwal(t, NewJadwal(true, 7, 1, 5))
	now := wib(2026, 10, 20, 14, 0)
	info := e.m.InfoOtomatis(now, nil, nil)
	if !info.Aktif || info.IntervalHari != 7 || info.JamMulai != 1 || info.JamAkhir != 5 || info.Zona != "WIB" || info.Berikutnya == nil || !info.Berikutnya.Equal(wib(2026, 10, 21, 1, 0)) {
		t.Errorf("info = %+v", info)
	}
	// Tanpa SLDK atau dimatikan: tidak ada jadwal yang dijanjikan.
	e.m.sldk = nil
	if info := e.m.InfoOtomatis(now, nil, nil); info.Aktif || info.Berikutnya != nil {
		t.Errorf("tanpa SLDK: %+v", info)
	}
	e2 := newEnvJadwal(t, NewJadwal(false, 7, 1, 5))
	if info := e2.m.InfoOtomatis(now, nil, nil); info.Aktif || info.Berikutnya != nil {
		t.Errorf("dimatikan: %+v", info)
	}
}

func TestMulaiPenjadwalMenetapkanJadwalDanTidakMenjalankanBilaMati(t *testing.T) {
	e := newEnvJadwal(t, Jadwal{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	e.m.MulaiPenjadwal(ctx, NewJadwal(false, 3, 2, 4))
	if j := e.m.Jadwal(); j.Aktif || j.Interval != 3*24*time.Hour || j.JamMulai != 2 || j.JamAkhir != 4 {
		t.Errorf("jadwal = %+v", j)
	}
	time.Sleep(20 * time.Millisecond)
	if e.pasti.Count("QUERY") != 0 {
		t.Error("jadwal nonaktif tidak boleh menjalankan penjadwal")
	}
}

func TestPenjadwalBerjalanSendiriSetelahTundaAwalLaluBerhentiBilaKonteksBerakhir(t *testing.T) {
	lamaTunda, lamaPeriode := tundaAwal, periodeCek
	tundaAwal, periodeCek = 10*time.Millisecond, 10*time.Millisecond
	t.Cleanup(func() { tundaAwal, periodeCek = lamaTunda, lamaPeriode })

	e := newEnvJadwal(t, Jadwal{})
	ctx, stop := context.WithCancel(context.Background())
	// Sepanjang hari (mulai = akhir) supaya tes tidak bergantung pada jam mesin.
	e.m.MulaiPenjadwal(ctx, NewJadwal(true, 7, 6, 6))

	deadline := time.Now().Add(3 * time.Second)
	for len(e.ran()) < len(Datasets) {
		if time.Now().After(deadline) {
			t.Fatalf("penjadwal tidak memulai sinkronisasi sendiri; jalan: %v", e.ran())
		}
		time.Sleep(5 * time.Millisecond)
	}
	stop()
	e.waitIdle(t)
	// Setelah konteks berakhir tidak ada antrean baru (beri satu periode untuk memastikan).
	time.Sleep(40 * time.Millisecond)
	e.waitIdle(t)
	sebelum := len(e.ran())
	time.Sleep(60 * time.Millisecond)
	if sesudah := len(e.ran()); sesudah != sebelum {
		t.Errorf("penjadwal masih berjalan setelah konteks berakhir: %d -> %d", sebelum, sesudah)
	}
}

func TestSinkronisasiTanpaBatasWaktuBawaan(t *testing.T) {
	var punyaBatas bool
	var mu sync.Mutex
	env := newManagerEnv(t, func(ctx context.Context, ds Dataset) (Result, error) {
		_, has := ctx.Deadline()
		mu.Lock()
		punyaBatas = has
		mu.Unlock()
		time.Sleep(60 * time.Millisecond) // lebih lama dari batas mana pun yang dipasang tes lain
		return Result{Rows: 1}, ctx.Err()
	})
	if env.m.Timeout != 0 {
		t.Fatalf("Timeout bawaan = %s, want 0 (tanpa batas)", env.m.Timeout)
	}
	if _, err := env.m.Start([]string{"satker"}, "admin"); err != nil {
		t.Fatal(err)
	}
	env.waitIdle(t)
	mu.Lock()
	defer mu.Unlock()
	if punyaBatas {
		t.Error("konteks sinkronisasi tidak boleh punya batas waktu secara bawaan")
	}
	status, _ := env.finals()
	if status[1] != StatusSukses {
		t.Errorf("status = %q, want sukses", status[1])
	}
}
