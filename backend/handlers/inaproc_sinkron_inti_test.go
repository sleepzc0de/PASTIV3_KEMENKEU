package handlers

import (
	"context"
	"database/sql/driver"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Tes inti sinkronisasi (endpointDatar.Jalankan): atomik, dapat dibatalkan, dan tanpa konteks HTTP.

func rencanaUji(t *testing.T, e *endpointDatar) *rencanaSync {
	t.Helper()
	r, pesan := e.rencanaDari(syncDatarRequest{Tahun: "2024"})
	if pesan != "" {
		t.Fatal(pesan)
	}
	return r
}

func indeksKejadian(events []string, awalan string, dari int) int {
	for i := dari; i < len(events); i++ {
		if strings.HasPrefix(events[i], awalan) {
			return i
		}
	}
	return -1
}

func berkasSementara() int {
	n, _ := filepath.Glob(filepath.Join(os.TempDir(), "inaproc-sinkron-*.jsonl"))
	return len(n)
}

// Inaproc palsu dua halaman; halaman kedua dijawab `kode` dengan `isi2`.
func inaprocDuaHalaman(t *testing.T, kode2 int, isi2 string) *inaprocPalsu {
	t.Helper()
	p := newInaprocPalsu(t, func(r *http.Request) (int, string) {
		if r.URL.Query().Get("cursor") == "" {
			return 200, `{"success":true,"data":[` + contohNonTenderSelesai + `],"meta":{"limit":1000,"has_more":true,"cursor":"c1"}}`
		}
		return kode2, isi2
	})
	pasangKonfigInaproc(t, p.URL, "token-uji")
	return p
}

// Kegagalan di halaman kedua tidak boleh meninggalkan data setengah jadi: tidak ada penghapusan, tidak ada penyisipan, tidak ada transaksi.
func TestJalankanGagalDiHalamanKeduaTidakMenyentuhData(t *testing.T) {
	inaprocDuaHalaman(t, 429, `{"success":false,"error":{"code":"Too Many Requests","message":"Rate limit exceeded","details":"Please retry later"}}`)
	f := pasangDBPalsu(t)
	sebelum := berkasSementara()

	_, err := nonTenderSelesai.Jalankan(context.Background(), rencanaUji(t, nonTenderSelesai), "")
	var g *GalatSinkron
	if !errors.As(err, &g) || g.Status != 429 || !g.Hulu {
		t.Fatalf("err = %#v, want GalatSinkron 429 dari hulu", err)
	}
	ev := f.Events()
	for _, larangan := range []string{"BEGIN", "EXEC DELETE", "EXEC INSERT INTO inaproc_non_tender_selesai", "COMMIT"} {
		if indeksKejadian(ev, larangan, 0) >= 0 {
			t.Errorf("kejadian %q tidak boleh ada saat pengambilan gagal: %v", larangan, ev)
		}
	}
	// Kegagalan tetap dicatat; penarikan otomatis tidak punya pengguna, jadi synced_by NULL (kolom bertipe UNIQUEIDENTIFIER).
	var dicatat bool
	for _, ex := range f.Execs() {
		if strings.Contains(ex.Query, "INTO inaproc_sync_log") {
			dicatat = true
			if ex.Args[5].Value != "failed" || ex.Args[7].Value != nil {
				t.Errorf("sync_log = %+v, want failed dan synced_by NULL", ex.Args)
			}
		}
	}
	if !dicatat {
		t.Error("kegagalan harus dicatat di inaproc_sync_log")
	}
	if berkasSementara() != sebelum {
		t.Error("berkas sementara tidak dibersihkan")
	}
}

func TestJalankanDibatalkanSebelumMenyentuhData(t *testing.T) {
	inaprocDuaHalaman(t, 200, `{"success":true,"data":[],"meta":{"has_more":false,"cursor":""}}`)
	f := pasangDBPalsu(t)
	ctx, batal := context.WithCancel(context.Background())
	batal()

	_, err := nonTenderSelesai.Jalankan(ctx, rencanaUji(t, nonTenderSelesai), "")
	var g *GalatSinkron
	if !errors.As(err, &g) || g.Status != statusDibatalkan {
		t.Fatalf("err = %#v, want dibatalkan (%d)", err, statusDibatalkan)
	}
	if ev := f.Events(); indeksKejadian(ev, "BEGIN", 0) >= 0 || indeksKejadian(ev, "EXEC DELETE", 0) >= 0 {
		t.Errorf("pembatalan tidak boleh menyentuh data: %v", ev)
	}
}

// Penggantian data lama dan penyisipan yang baru terjadi dalam satu transaksi, setelah semua halaman terambil.
func TestJalankanMenggantiDataDalamSatuTransaksi(t *testing.T) {
	inaprocDuaHalaman(t, 200, `{"success":true,"data":[`+varianBaris(contohNonTenderSelesai, "Halaman2")+`],"meta":{"limit":1000,"has_more":false,"cursor":""}}`)
	f := pasangDBPalsu(t)

	hasil, err := nonTenderSelesai.Jalankan(context.Background(), rencanaUji(t, nonTenderSelesai), "11111111-1111-1111-1111-111111111111")
	if err != nil {
		t.Fatal(err)
	}
	if hasil.TotalSinkron != 2 || hasil.TotalGagal != 0 || hasil.Halaman != 2 {
		t.Errorf("hasil = %+v, want 2 tersimpan, 0 gagal, 2 halaman", hasil)
	}
	ev := f.Events()
	mulai := indeksKejadian(ev, "BEGIN", 0)
	hapus := indeksKejadian(ev, "EXEC DELETE FROM inaproc_non_tender_selesai", 0)
	sisip := indeksKejadian(ev, "EXEC INSERT INTO inaproc_non_tender_selesai", 0)
	commit := indeksKejadian(ev, "COMMIT", 0)
	if !(mulai >= 0 && mulai < hapus && hapus < sisip && sisip < commit) {
		t.Fatalf("urutan kejadian salah (BEGIN=%d DELETE=%d INSERT=%d COMMIT=%d): %v", mulai, hapus, sisip, commit, ev)
	}
	if indeksKejadian(ev, "ROLLBACK", 0) >= 0 {
		t.Errorf("tidak boleh ada ROLLBACK: %v", ev)
	}
	// Pengguna pemicu dicatat bila ada.
	for _, ex := range f.Execs() {
		if strings.Contains(ex.Query, "INTO inaproc_sync_log") && ex.Args[7].Value != "11111111-1111-1111-1111-111111111111" {
			t.Errorf("synced_by = %v", ex.Args[7].Value)
		}
	}
}

// Bila penghapusan data lama gagal, transaksi dibatalkan dan tidak ada yang disisipkan (tidak ada tabrakan dengan data lama).
func TestJalankanHapusGagalMembatalkanTransaksi(t *testing.T) {
	inaprocDuaHalaman(t, 200, `{"success":true,"data":[],"meta":{"has_more":false,"cursor":""}}`)
	f := pasangDBPalsu(t)
	f.OnExec = func(q string, _ []driver.NamedValue) error {
		if strings.HasPrefix(q, "DELETE FROM") {
			return errors.New("deadlock")
		}
		return nil
	}

	_, err := nonTenderSelesai.Jalankan(context.Background(), rencanaUji(t, nonTenderSelesai), "")
	var g *GalatSinkron
	if !errors.As(err, &g) || g.Status != http.StatusInternalServerError || g.Hulu {
		t.Fatalf("err = %#v, want GalatSinkron 500", err)
	}
	ev := f.Events()
	if indeksKejadian(ev, "ROLLBACK", 0) < 0 || indeksKejadian(ev, "EXEC INSERT INTO inaproc_non_tender_selesai", 0) >= 0 || indeksKejadian(ev, "COMMIT", 0) >= 0 {
		t.Errorf("harus ROLLBACK tanpa INSERT/COMMIT: %v", ev)
	}
}
