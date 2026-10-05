package handlers

import (
	"context"
	"database/sql/driver"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"pasti-v3-backend/internal/fakesql"
)

// jamUji: jam palsu; tidur memajukan jam tanpa menunggu sungguhan, jadi tes batas laju berjalan seketika.
type jamUji struct {
	mu  sync.Mutex
	now time.Time
	// tidurTotal: total waktu yang "ditunggu" lewat tidur.
	tidurTotal time.Duration
}

func (j *jamUji) Now() time.Time {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.now
}

func (j *jamUji) Maju(d time.Duration) {
	j.mu.Lock()
	j.now = j.now.Add(d)
	j.mu.Unlock()
}

func (j *jamUji) Tidur(ctx context.Context, d time.Duration) bool {
	if ctx.Err() != nil {
		return false
	}
	j.mu.Lock()
	j.now = j.now.Add(d)
	j.tidurTotal += d
	j.mu.Unlock()
	return true
}

func pasangJamUji(b *BatasInaproc) *jamUji {
	j := &jamUji{now: time.Date(2026, 10, 5, 10, 0, 0, 0, time.UTC)}
	b.sekarang, b.tidurFn = j.Now, j.Tidur
	return j
}

// batasUji memasang pembatas bersama baru dengan jam palsu; dipulihkan saat tes selesai. Semua tes yang memanggil Inaproc memakainya supaya
// jeda 429 dan percobaan ulang tidak membuat tes menunggu sungguhan dan tidak menumpuk pemakaian antar tes.
func batasUji(t *testing.T) (*BatasInaproc, *jamUji) {
	t.Helper()
	b := NewBatasInaproc(0, 0)
	j := pasangJamUji(b)
	aturBatas(b)
	batasUjiAktif, jamUjiAktif = b, j
	t.Cleanup(func() { aturBatas(nil); batasUjiAktif, jamUjiAktif = nil, nil })
	return b, j
}

var (
	batasUjiAktif *BatasInaproc
	jamUjiAktif   *jamUji
)

// pasangBatasUjiBilaPerlu dipanggil pasangKonfigInaproc: tes yang tidak memasang pembatas sendiri tetap mendapat pembatas berjam palsu, supaya
// tidak ada tes yang menunggu jeda 429 sungguhan atau memakai pembatas bersama milik tes lain.
func pasangBatasUjiBilaPerlu(t *testing.T) {
	t.Helper()
	if batasUjiAktif == nil || batas() != batasUjiAktif {
		batasUji(t)
	}
}

// ---- pembatas ----

func TestBatasPerMenitMenahanSampaiJendelaGeserLonggar(t *testing.T) {
	b := NewBatasInaproc(3, 1000)
	j := pasangJamUji(b)
	ctx := context.Background()
	mulai := j.Now()
	for i := 0; i < 3; i++ {
		if err := b.Tunggu(ctx, 0); err != nil {
			t.Fatal(err)
		}
		j.Maju(5 * time.Second) // permintaan pada detik 0, 5, 10
	}
	if j.tidurTotal != 0 {
		t.Fatalf("tiga permintaan pertama tidak boleh menunggu, menunggu %s", j.tidurTotal)
	}
	// Permintaan ke-4 pada detik 15 baru boleh setelah permintaan pertama (detik 0) keluar dari jendela 60 detik.
	if err := b.Tunggu(ctx, 0); err != nil {
		t.Fatal(err)
	}
	if got := j.Now().Sub(mulai); got != 60*time.Second {
		t.Errorf("permintaan ke-4 dikirim pada +%s, want +60s", got)
	}
	s := b.Status()
	if s.TerpakaiMenit != 3 || s.TerpakaiJam != 4 {
		t.Errorf("status = %+v, want 3 dalam menit terakhir dan 4 dalam jam terakhir", s)
	}
}

func TestBatasPerJamMenahanSampaiMenitTertuaKedaluwarsa(t *testing.T) {
	b := NewBatasInaproc(1000, 5)
	j := pasangJamUji(b)
	ctx := context.Background()
	awal := j.Now()
	// 5 permintaan di menit pertama menghabiskan kuota jam.
	for i := 0; i < 5; i++ {
		if err := b.Tunggu(ctx, 0); err != nil {
			t.Fatal(err)
		}
	}
	if s := b.Status(); s.SisaJam != 0 || s.PulihSekitar == nil {
		t.Fatalf("status = %+v, want kuota habis dan ada perkiraan pulih", s)
	}
	if err := b.Tunggu(ctx, 0); err != nil {
		t.Fatal(err)
	}
	// Menit unix awal berakhir pada awal menit berikutnya + 60 menit; permintaan ke-6 tidak boleh lebih awal dari itu.
	menitAwal := time.Unix((menitUnix(awal)+1)*60, 0).Add(60 * time.Minute)
	if j.Now().Before(menitAwal) {
		t.Errorf("permintaan ke-6 dikirim %s, sebelum kuota menit pertama kedaluwarsa (%s)", j.Now(), menitAwal)
	}
	if j.Now().Sub(awal) > 61*time.Minute+time.Second {
		t.Errorf("menunggu terlalu lama: %s", j.Now().Sub(awal))
	}
}

func TestPastikanMenungguSampaiJatahCukupTanpaMencatat(t *testing.T) {
	b := NewBatasInaproc(1000, 100)
	j := pasangJamUji(b)
	ctx := context.Background()
	for i := 0; i < 80; i++ {
		_ = b.Tunggu(ctx, 0)
	}
	// Butuh 50 tetapi sisa 20: menunggu sampai cukup. Tidak mencatat pemakaian.
	if err := b.Pastikan(ctx, 50); err != nil {
		t.Fatal(err)
	}
	if j.tidurTotal < 50*time.Minute {
		t.Errorf("menunggu %s, want hampir satu jam (jatah baru longgar setelah menit pertama kedaluwarsa)", j.tidurTotal)
	}
	if s := b.Status(); s.TerpakaiJam > 0 {
		t.Errorf("seluruh pemakaian lama sudah kedaluwarsa, status = %+v", s)
	}
	// Permintaan lebih besar dari seluruh jatah dijepit (tidak menunggu selamanya).
	b2 := NewBatasInaproc(1000, 10)
	pasangJamUji(b2)
	if err := b2.Pastikan(ctx, 5000); err != nil {
		t.Fatal(err)
	}
}

func TestTungguInteraktifTidakMenggantung(t *testing.T) {
	b := NewBatasInaproc(1000, 2)
	j := pasangJamUji(b)
	ctx := context.Background()
	_ = b.Tunggu(ctx, 0)
	_ = b.Tunggu(ctx, 0)
	if err := b.Tunggu(ctx, 3*time.Second); !errors.Is(err, ErrKuotaHabis) {
		t.Fatalf("err = %v, want ErrKuotaHabis", err)
	}
	if j.tidurTotal != 0 {
		t.Errorf("permintaan interaktif tidak boleh tidur, tidur %s", j.tidurTotal)
	}
	// Dibatalkan lewat ctx.
	cx, batal := context.WithCancel(ctx)
	batal()
	if err := b.Tunggu(cx, 0); !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
}

func TestLaporkan429MenahanSemuaDanMengeskalasi(t *testing.T) {
	b := NewBatasInaproc(1000, 1000)
	j := pasangJamUji(b)
	ctx := context.Background()

	if d := b.Laporkan429(0); d != 65*time.Second {
		t.Errorf("429 pertama = %s, want 65s", d)
	}
	mulai := j.Now()
	_ = b.Tunggu(ctx, 0)
	if got := j.Now().Sub(mulai); got != 65*time.Second {
		t.Errorf("menunggu %s, want 65s", got)
	}
	if d := b.Laporkan429(0); d != 5*time.Minute {
		t.Errorf("429 kedua = %s, want 5m", d)
	}
	if d := b.Laporkan429(0); d != 15*time.Minute {
		t.Errorf("429 ketiga = %s, want 15m", d)
	}
	if d := b.Laporkan429(0); d != 60*time.Minute {
		t.Errorf("429 keempat = %s, want 60m", d)
	}
	// Retry-After dari Inaproc didahulukan, dan dibatasi 70 menit.
	if d := b.Laporkan429(90 * time.Second); d != 90*time.Second {
		t.Errorf("dengan Retry-After = %s, want 90s", d)
	}
	if d := b.Laporkan429(5 * time.Hour); d != 70*time.Minute {
		t.Errorf("Retry-After raksasa = %s, want dijepit 70m", d)
	}
	if s := b.Status(); s.TahanSampai == nil || !s.TahanSampai.After(j.Now().Add(60*time.Minute)) {
		t.Errorf("status = %+v, want ditahan > 60 menit", s)
	}
	// Berhasil mengulang eskalasi dari awal; 429 yang berjarak > 30 menit juga.
	b.LaporkanBerhasil()
	j.Maju(3 * time.Hour)
	if d := b.Laporkan429(0); d != 65*time.Second {
		t.Errorf("setelah berhasil = %s, want 65s", d)
	}
}

func TestBatasMenyimpanSelisihDanMemuatUlangSetelahRestart(t *testing.T) {
	db, f := fakesql.New(t)
	tersimpan := map[int64]int{} // isi tabel inaproc_kuota_menit palsu
	f.OnExec = func(q string, args []driver.NamedValue) error {
		if strings.Contains(q, "UPDATE inaproc_kuota_menit") {
			tersimpan[args[0].Value.(int64)] += int(args[1].Value.(int64))
		}
		return nil
	}
	f.OnQuery = func(_ context.Context, q string, _ []driver.NamedValue) ([]string, [][]driver.Value, error) {
		var baris [][]driver.Value
		for m, n := range tersimpan {
			baris = append(baris, []driver.Value{m, int64(n)})
		}
		return []string{"menit", "jumlah"}, baris, nil
	}

	b := NewBatasInaproc(1000, 1000)
	pasangJamUji(b)
	for i := 0; i < 7; i++ {
		_ = b.Tunggu(context.Background(), 0)
	}
	if err := b.Simpan(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	total := 0
	for _, n := range tersimpan {
		total += n
	}
	if total != 7 {
		t.Fatalf("tersimpan %d, want 7", total)
	}
	// Disimpan lagi tanpa permintaan baru: tidak ada selisih, tidak menulis ganda.
	execSebelum := len(f.Execs())
	if err := b.Simpan(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	for _, ex := range f.Execs()[execSebelum:] {
		if strings.Contains(ex.Query, "UPDATE inaproc_kuota_menit") {
			t.Error("menulis ulang hitungan yang sudah tersimpan")
		}
	}
	_ = b.Tunggu(context.Background(), 0)
	_ = b.Simpan(context.Background(), db)
	total = 0
	for _, n := range tersimpan {
		total += n
	}
	if total != 8 {
		t.Errorf("setelah satu permintaan lagi tersimpan %d, want 8 (selisih ditambahkan, bukan ditimpa)", total)
	}

	// "Restart": pembatas baru memuat hitungan dari database, jadi kuota yang sudah terpakai tidak terlupa.
	baru := NewBatasInaproc(1000, 1000)
	pasangJamUji(baru)
	if err := baru.Muat(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	if s := baru.Status(); s.TerpakaiJam != 8 {
		t.Errorf("setelah dimuat terpakai %d, want 8", s.TerpakaiJam)
	}
	// Server lain ikut memakai kuota yang sama: hitungan yang lebih besar di database menang.
	for m := range tersimpan {
		tersimpan[m] += 100
	}
	_ = baru.Simpan(context.Background(), db)
	if s := baru.Status(); s.TerpakaiJam != 108 {
		t.Errorf("setelah digabung terpakai %d, want 108", s.TerpakaiJam)
	}
}

// ---- klien ----

func TestBacaRetryAfter(t *testing.T) {
	sekarang := time.Date(2026, 10, 5, 10, 0, 0, 0, time.UTC)
	h := func(v string) http.Header {
		x := http.Header{}
		if v != "" {
			x.Set("Retry-After", v)
		}
		return x
	}
	for in, want := range map[string]time.Duration{
		"": 0, "30": 30 * time.Second, "0": 0, "-5": 0, "abc": 0,
		sekarang.Add(90 * time.Second).UTC().Format(http.TimeFormat): 90 * time.Second,
		sekarang.Add(-time.Minute).UTC().Format(http.TimeFormat):     0,
	} {
		if got := bacaRetryAfter(h(in), sekarang); got != want {
			t.Errorf("Retry-After %q = %s, want %s", in, got, want)
		}
	}
}

func inaprocUrut(t *testing.T, balasan ...func(w http.ResponseWriter)) (*httptest.Server, *int32) {
	t.Helper()
	var n int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		i := int(atomic.AddInt32(&n, 1)) - 1
		if i >= len(balasan) {
			i = len(balasan) - 1
		}
		balasan[i](w)
	}))
	t.Cleanup(srv.Close)
	return srv, &n
}

func balas(kode int, isi string) func(w http.ResponseWriter) {
	return func(w http.ResponseWriter) { w.WriteHeader(kode); _, _ = w.Write([]byte(isi)) }
}

func TestKlien429DitungguLaluDilanjutkanTanpaMenggugurkanPenarikan(t *testing.T) {
	_, jam := batasUji(t)
	srv, n := inaprocUrut(t,
		func(w http.ResponseWriter) {
			w.Header().Set("Retry-After", "120")
			w.WriteHeader(429)
			_, _ = w.Write([]byte(`{"error":{"message":"Rate limit"}}`))
		},
		balas(200, `{"success":true,"data":[]}`))
	pasangKonfigInaproc(t, srv.URL, "t")
	jam.tidurTotal = 0
	mulai := jam.Now()

	body, status, err := callInaprocEndpointCtx(context.Background(), "/api/v1/x", url.Values{})
	if err != nil || status != 200 || !strings.Contains(string(body), "success") {
		t.Fatalf("status=%d err=%v body=%s", status, err, body)
	}
	if *n != 2 {
		t.Errorf("permintaan = %d, want 2 (429 lalu berhasil)", *n)
	}
	if got := jam.Now().Sub(mulai); got != 120*time.Second {
		t.Errorf("menunggu %s, want 120s sesuai Retry-After", got)
	}
}

func TestKlien429TerusMenerusBerakhirSetelahEmpatPercobaan(t *testing.T) {
	_, jam := batasUji(t)
	srv, n := inaprocUrut(t, balas(429, `{"error":{"message":"Rate limit exceeded"}}`))
	pasangKonfigInaproc(t, srv.URL, "t")
	mulai := jam.Now()
	_, status, err := callInaprocEndpointCtx(context.Background(), "/api/v1/x", url.Values{})
	if err != nil || status != 429 {
		t.Fatalf("status=%d err=%v", status, err)
	}
	if *n != maksCoba429 {
		t.Errorf("permintaan = %d, want %d", *n, maksCoba429)
	}
	// Jeda bertahap bersama: 65 detik, 5 menit, 15 menit antar keempat percobaan.
	if got := jam.Now().Sub(mulai); got != 65*time.Second+5*time.Minute+15*time.Minute {
		t.Errorf("total menunggu %s, want 21m5s", got)
	}
}

func TestKlien5xxDanGalatJaringanDicobaUlangPerHalaman(t *testing.T) {
	_, jam := batasUji(t)
	srv, n := inaprocUrut(t, balas(502, `bad gateway`), balas(500, `oops`), balas(200, `{"success":true}`))
	pasangKonfigInaproc(t, srv.URL, "t")
	_, status, err := callInaprocEndpointCtx(context.Background(), "/api/v1/x", url.Values{})
	if err != nil || status != 200 || *n != 3 {
		t.Fatalf("status=%d err=%v permintaan=%d, want 200 setelah 3 permintaan", status, err, *n)
	}
	if jam.tidurTotal != 13*time.Second {
		t.Errorf("jeda antar percobaan = %s, want 3s + 10s", jam.tidurTotal)
	}

	// Tetap 5xx: berakhir setelah tiga percobaan dan status aslinya diteruskan.
	srv2, n2 := inaprocUrut(t, balas(503, `maintenance`))
	pasangKonfigInaproc(t, srv2.URL, "t")
	_, status, err = callInaprocEndpointCtx(context.Background(), "/api/v1/x", url.Values{})
	if err != nil || status != 503 || *n2 != maksCoba5xx {
		t.Errorf("status=%d err=%v permintaan=%d, want 503 setelah %d permintaan", status, err, *n2, maksCoba5xx)
	}

	// 4xx selain 429 tidak dicoba ulang (kesalahan permintaan, bukan gangguan sementara).
	srv3, n3 := inaprocUrut(t, balas(404, `{"error":{"message":"tidak ada"}}`))
	pasangKonfigInaproc(t, srv3.URL, "t")
	_, status, _ = callInaprocEndpointCtx(context.Background(), "/api/v1/x", url.Values{})
	if status != 404 || *n3 != 1 {
		t.Errorf("status=%d permintaan=%d, want 404 tanpa pengulangan", status, *n3)
	}
}

func TestKlienInteraktifSatuKaliCobaDanKuotaHabisDijawab429(t *testing.T) {
	b, _ := batasUji(t)
	srv, n := inaprocUrut(t, balas(200, `{"success":true}`))
	pasangKonfigInaproc(t, srv.URL, "t")

	// Normal: satu permintaan, tanpa pengulangan walau galat.
	if _, status, err := callInaprocEndpointInteraktif("/api/v1/x", url.Values{}); err != nil || status != 200 {
		t.Fatalf("status=%d err=%v", status, err)
	}
	srvGalat, nGalat := inaprocUrut(t, balas(502, `x`))
	pasangKonfigInaproc(t, srvGalat.URL, "t")
	if _, status, _ := callInaprocEndpointInteraktif("/api/v1/x", url.Values{}); status != 502 || *nGalat != 1 {
		t.Errorf("status=%d permintaan=%d, want 502 tanpa pengulangan", status, *nGalat)
	}

	// Kuota habis: dijawab 429 sintetis seketika, tanpa menghubungi Inaproc dan tanpa menunggu lama.
	b.perJam = 1
	pasangKonfigInaproc(t, srv.URL, "t")
	sebelum := atomic.LoadInt32(n)
	body, status, err := callInaprocEndpointInteraktif("/api/v1/x", url.Values{})
	if err != nil || status != 429 || !strings.Contains(string(body), "Kuota permintaan Inaproc sedang habis") {
		t.Fatalf("status=%d err=%v body=%s", status, err, body)
	}
	if atomic.LoadInt32(n) != sebelum {
		t.Error("kuota habis tidak boleh menghubungi Inaproc")
	}
}

func TestKlienMemakaiUlangKoneksi(t *testing.T) {
	batasUji(t)
	srv, _ := inaprocUrut(t, balas(200, `{}`))
	pasangKonfigInaproc(t, srv.URL, "t")
	if klienInaproc() != klienInaproc() {
		t.Error("http.Client harus dipakai ulang untuk timeout yang sama")
	}
}

// kosongkanBatasUji menghapus pemakaian dan jeda 429 pada pembatas bersama (tanpa mengganti jamnya).
func kosongkanBatasUji() {
	b := batas()
	b.mu.Lock()
	b.burst, b.menit, b.tersimpan = nil, map[int64]int{}, map[int64]int{}
	b.tahanSampai, b.beruntun429, b.terakhir429 = time.Time{}, 0, time.Time{}
	b.mu.Unlock()
}
