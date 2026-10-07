package sapa

import (
	"errors"
	"strings"
	"testing"
)

// Tes aturan kunci usulan: usulan yang seluruh tahapnya selesai terkunci total dan hanya superadmin atau Pengguna Barang yang dapat membuka kuncinya; usulan yang
// belum selesai masih dapat diedit (per tahap) dan dihapus.

// selesaikanSemua mengerjakan seluruh 10 tahap usulan sampai selesai, masing-masing oleh perannya.
func (e *env) selesaikanSemua(k *KasusInfo) {
	e.t.Helper()
	e.siapkanNDSatker(k)
	ok := func(err error) {
		e.t.Helper()
		if err != nil {
			e.t.Fatal(err)
		}
	}
	ok(e.l.Selesaikan(e.ctx, e.satkerA, k.ID, TahapNadineSatker, "ND-1/2026", "2026-10-01", ""))
	ok(e.l.Selesaikan(e.ctx, e.satkerA, k.ID, TahapSimanSatker, "", "", ""))
	ok(e.l.Selesaikan(e.ctx, e.kanwil, k.ID, TahapSimanKanwil, "", "", ""))
	ok(e.l.Selesaikan(e.ctx, e.ue1, k.ID, TahapUE1Terima, "", "", ""))
	if _, err := e.l.Hasilkan(e.ctx, e.ue1, k.ID, TahapNDUE1, jsonDari(e.t, ContohNDUE1())); err != nil {
		e.t.Fatal(err)
	}
	ok(e.l.Selesaikan(e.ctx, e.ue1, k.ID, TahapNadineUE1, "", "", ""))
	ok(e.l.Selesaikan(e.ctx, e.ue1, k.ID, TahapSimanUE1, "", "", ""))
	if d, _ := e.l.DetailUsulan(e.ctx, e.admin, k.ID); !d.Selesai {
		e.t.Fatal("usulan harus sudah selesai")
	}
}

func TestUsulanSelesaiTerkunciTotal(t *testing.T) {
	e := baru(t)
	k := e.buat(e.satkerA)
	e.selesaikanSemua(k)

	// Tidak ada tahap yang bisa diubah lagi oleh siapa pun, termasuk tahap terakhir (yang dulu masih bisa dicatat ulang) dan oleh superadmin.
	semua := []Identitas{e.satkerA, e.kanwil, e.ue1, e.admin}
	for _, tahap := range TahapPenjualan {
		for _, id := range semua {
			var errs []error
			errs = append(errs,
				e.l.SimpanDraf(e.ctx, id, k.ID, tahap.Kunci, []byte(`{}`)),
				e.l.Selesaikan(e.ctx, id, k.ID, tahap.Kunci, "X-1", "2026-10-02", "diubah"),
				e.l.Lewati(e.ctx, id, k.ID, tahap.Kunci, "diubah setelah selesai"),
			)
			_, errHasil := e.l.Hasilkan(e.ctx, id, k.ID, tahap.Kunci, []byte(`{}`))
			errs = append(errs, errHasil)
			for i, err := range errs {
				if err == nil {
					t.Errorf("tahap %s oleh %+v (aksi %d): usulan terkunci harus menolak", tahap.Kunci, id.Peran, i)
					continue
				}
				// Peran yang bukan pemilik tahap ditolak karena hak (lebih dulu); pemilik dan superadmin mendapat pesan terkunci.
				var kf *ErrKonflik
				if errors.As(err, &kf) && kf.Pesan != PesanTerkunci {
					t.Errorf("tahap %s: pesan = %q, want pesan terkunci", tahap.Kunci, kf.Pesan)
				}
			}
		}
	}
	// Pemilik tahap terakhir (UE1) mendapat pesan terkunci yang jelas, bukan galat urutan.
	err := e.l.Selesaikan(e.ctx, e.ue1, k.ID, TahapSimanUE1, "", "", "ubah catatan")
	var kf *ErrKonflik
	if !errors.As(err, &kf) || kf.Pesan != PesanTerkunci || !strings.Contains(kf.Pesan, "terkunci") {
		t.Errorf("ubah tahap terakhir setelah selesai: %v", err)
	}

	// Penanda pada detail: tak satu pun tahap dapat diubah, hanya yang berhak membuka kunci yang melihat BolehBukaKunci, dan hanya tahap terakhir yang dapat dibuka ulang.
	for nama, c := range map[string]struct {
		id        Identitas
		bukaKunci bool
	}{"satker": {e.satkerA, false}, "kanwil": {e.kanwil, false}, "ue1": {e.ue1, false}, "pengguna barang": {e.barang, true}, "superadmin": {e.admin, true}} {
		d, err := e.l.DetailUsulan(e.ctx, c.id, k.ID)
		if err != nil {
			t.Fatalf("%s: %v", nama, err)
		}
		if !d.Selesai || d.BolehHapus || d.BolehBukaKunci != c.bukaKunci {
			t.Errorf("%s: selesai=%v boleh_hapus=%v boleh_buka_kunci=%v", nama, d.Selesai, d.BolehHapus, d.BolehBukaKunci)
		}
		for _, td := range d.Tahap {
			if td.DapatDiubah {
				t.Errorf("%s: tahap %s masih dapat diubah pada usulan terkunci", nama, td.Kunci)
			}
			if td.DapatDibukaUlang != (td.Kunci == TahapSimanUE1) {
				t.Errorf("%s: tahap %s dapat_dibuka_ulang = %v", nama, td.Kunci, td.DapatDibukaUlang)
			}
		}
	}

	// Usulan selesai tidak dapat dihapus, siapa pun yang berhak menghapus.
	for _, id := range []Identitas{e.satkerA, e.barang, e.admin} {
		_, err := e.l.HapusUsulan(e.ctx, id, k.ID)
		var kf *ErrKonflik
		if !errors.As(err, &kf) || !strings.Contains(kf.Pesan, "sudah selesai") {
			t.Errorf("hapus usulan selesai oleh %q: %v", id.Peran, err)
		}
	}
	if _, err := e.l.DetailUsulan(e.ctx, e.admin, k.ID); err != nil {
		t.Errorf("usulan selesai tidak boleh hilang: %v", err)
	}
}

func TestBukaKunciHanyaSuperadminDanPenggunaBarang(t *testing.T) {
	e := baru(t)
	k := e.buat(e.satkerA)
	e.selesaikanSemua(k)

	// Satker pemilik, Kanwil, dan UE1 tidak dapat membuka kunci; usulan tetap selesai.
	for nama, id := range map[string]Identitas{"satker": e.satkerA, "kanwil": e.kanwil, "ue1": e.ue1} {
		if err := e.l.BukaUlang(e.ctx, id, k.ID, TahapSimanUE1); !errors.Is(err, ErrTidakBerhak) {
			t.Errorf("%s membuka kunci: %v, want ErrTidakBerhak", nama, err)
		}
	}
	// Yang tidak boleh melihat usulan mendapat "tidak ditemukan", bukan petunjuk bahwa usulannya ada.
	if err := e.l.BukaUlang(e.ctx, e.satkerB, k.ID, TahapSimanUE1); !errors.Is(err, ErrTidakDitemukan) {
		t.Errorf("satker lain: %v", err)
	}
	if err := e.l.BukaUlang(e.ctx, e.tanpa, k.ID, TahapSimanUE1); !errors.Is(err, ErrTanpaPeran) {
		t.Errorf("tanpa peran: %v", err)
	}
	if d, _ := e.l.DetailUsulan(e.ctx, e.admin, k.ID); !d.Selesai {
		t.Fatal("usulan harus tetap terkunci setelah percobaan yang ditolak")
	}

	// Tahap yang bukan tahap terakhir tidak bisa dibuka ulang langsung (tahap sesudahnya masih selesai).
	var kf *ErrKonflik
	adalah(t, e.l.BukaUlang(e.ctx, e.barang, k.ID, TahapNadineUE1), &kf)

	// Pengguna Barang membuka kunci: tahap terakhir kembali draf, usulan berjalan lagi, dan pemilik tahap dapat mengubah/menyelesaikannya.
	if err := e.l.BukaUlang(e.ctx, e.barang, k.ID, TahapSimanUE1); err != nil {
		t.Fatalf("pengguna barang membuka kunci: %v", err)
	}
	d, _ := e.l.DetailUsulan(e.ctx, e.ue1, k.ID)
	if d.Selesai || d.TahapSaatIni != TahapSimanUE1 {
		t.Fatalf("setelah buka kunci: selesai=%v saat ini=%q", d.Selesai, d.TahapSaatIni)
	}
	if h, _ := e.l.DaftarUsulan(e.ctx, e.admin, FilterDaftar{Status: "berjalan"}); h.Total != 1 {
		t.Errorf("usulan yang dibuka kuncinya harus muncul sebagai berjalan: %+v", h)
	}
	if err := e.l.Selesaikan(e.ctx, e.ue1, k.ID, TahapSimanUE1, "", "", "diselesaikan ulang"); err != nil {
		t.Fatalf("pemilik tahap menyelesaikan ulang: %v", err)
	}
	if d, _ := e.l.DetailUsulan(e.ctx, e.ue1, k.ID); !d.Selesai {
		t.Error("setelah diselesaikan ulang, usulan terkunci lagi")
	}

	// Superadmin juga bisa; setelah tahap terakhir dibuka, tahap sebelumnya menjadi bisa dibuka ulang (satu per satu, dari belakang).
	if err := e.l.BukaUlang(e.ctx, e.admin, k.ID, TahapSimanUE1); err != nil {
		t.Fatalf("superadmin membuka kunci: %v", err)
	}
	if err := e.l.BukaUlang(e.ctx, e.admin, k.ID, TahapNadineUE1); err != nil {
		t.Fatalf("superadmin membuka ulang tahap sebelumnya: %v", err)
	}
}

func TestBukaUlangTahapBelumSelesaiDitolakKonflik(t *testing.T) {
	// Pengguna Barang melihat semua usulan, jadi dapat membuka kunci usulan satker mana pun.
	e := baru(t)
	k, err := e.l.BuatUsulan(e.ctx, e.admin, kodeB, "Satker B")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.l.DetailUsulan(e.ctx, e.barang, k.ID); err != nil {
		t.Fatalf("pengguna barang melihat usulan satker lain: %v", err)
	}
	// Belum ada tahap yang selesai: tidak ada yang bisa dibuka ulang (bukan galat hak).
	var kf *ErrKonflik
	adalah(t, e.l.BukaUlang(e.ctx, e.barang, k.ID, TahapTim), &kf)
}

func TestHapusUsulanBelumSelesai(t *testing.T) {
	e := baru(t)
	k := e.buat(e.satkerA)
	e.siapkanNDSatker(k) // ada isian, dokumen hasil, dan tahap yang dilewati
	if d, _ := e.l.DetailUsulan(e.ctx, e.satkerA, k.ID); len(d.Tahap[2].Dokumen) != 1 {
		t.Fatal("prasyarat: ND Satker harus punya dokumen hasil")
	}

	// Detail memberi tahu siapa yang boleh menghapus.
	for nama, c := range map[string]struct {
		id    Identitas
		boleh bool
	}{"satker pemilik": {e.satkerA, true}, "pengguna barang": {e.barang, true}, "superadmin": {e.admin, true}, "kanwil": {e.kanwil, false}, "ue1": {e.ue1, false}} {
		d, err := e.l.DetailUsulan(e.ctx, c.id, k.ID)
		if err != nil || d.BolehHapus != c.boleh {
			t.Errorf("%s: boleh_hapus = %v (%v), want %v", nama, d != nil && d.BolehHapus, err, c.boleh)
		}
	}

	// Yang tidak berhak ditolak, dan yang tidak boleh melihat mendapat "tidak ditemukan".
	for nama, id := range map[string]Identitas{"kanwil": e.kanwil, "ue1": e.ue1} {
		if _, err := e.l.HapusUsulan(e.ctx, id, k.ID); !errors.Is(err, ErrTidakBerhak) {
			t.Errorf("%s menghapus: %v, want ErrTidakBerhak", nama, err)
		}
	}
	for nama, id := range map[string]Identitas{"satker lain": e.satkerB, "kanwil lain": e.kanwilLain, "ue1 lain": e.ue1Lain} {
		if _, err := e.l.HapusUsulan(e.ctx, id, k.ID); !errors.Is(err, ErrTidakDitemukan) {
			t.Errorf("%s menghapus: %v, want ErrTidakDitemukan", nama, err)
		}
	}
	if _, err := e.l.HapusUsulan(e.ctx, e.tanpa, k.ID); !errors.Is(err, ErrTanpaPeran) {
		t.Errorf("tanpa peran: %v", err)
	}
	if _, err := e.l.DetailUsulan(e.ctx, e.satkerA, k.ID); err != nil {
		t.Fatalf("usulan tidak boleh hilang setelah percobaan yang ditolak: %v", err)
	}

	// Satker pemilik menghapus usulan yang masih berjalan: usulan, tahap, dan dokumennya hilang.
	dihapus, err := e.l.HapusUsulan(e.ctx, e.satkerA, k.ID)
	if err != nil || dihapus == nil || dihapus.Noreg != k.Noreg {
		t.Fatalf("hapus oleh satker pemilik: %v %+v", err, dihapus)
	}
	if _, err := e.l.DetailUsulan(e.ctx, e.admin, k.ID); !errors.Is(err, ErrTidakDitemukan) {
		t.Errorf("usulan yang dihapus: %v", err)
	}
	if docs, _ := e.repo.DaftarDokumen(e.ctx, k.ID); len(docs) != 0 {
		t.Errorf("dokumen usulan yang dihapus masih ada: %v", docs)
	}
	if rows, _ := e.repo.TahapPenjualan(e.ctx, k.ID); len(rows) != 0 {
		t.Errorf("tahap usulan yang dihapus masih ada: %v", rows)
	}
	if h, _ := e.l.DaftarUsulan(e.ctx, e.admin, FilterDaftar{}); h.Total != 0 {
		t.Errorf("daftar setelah hapus = %+v", h)
	}
	// Menghapus dua kali: tidak ada lagi.
	if _, err := e.l.HapusUsulan(e.ctx, e.admin, k.ID); !errors.Is(err, ErrTidakDitemukan) {
		t.Errorf("hapus dua kali: %v", err)
	}
	// Noreg tidak dipakai ulang.
	if baru := e.buat(e.satkerA); baru.Noreg == k.Noreg {
		t.Errorf("noreg %s dipakai ulang setelah usulan dihapus", baru.Noreg)
	}

	// Pengguna Barang dan superadmin juga dapat menghapus usulan yang belum selesai, termasuk milik satker lain.
	for nama, id := range map[string]Identitas{"pengguna barang": e.barang, "superadmin": e.admin} {
		k2 := e.buat(e.satkerA)
		if _, err := e.l.HapusUsulan(e.ctx, id, k2.ID); err != nil {
			t.Errorf("%s menghapus: %v", nama, err)
		}
	}
}

// Usulan yang sudah selesai dapat dihapus setelah kuncinya dibuka (tahap terakhir dibuka ulang membuatnya berjalan lagi).
func TestHapusSetelahBukaKunci(t *testing.T) {
	e := baru(t)
	k := e.buat(e.satkerA)
	e.selesaikanSemua(k)
	if _, err := e.l.HapusUsulan(e.ctx, e.satkerA, k.ID); err == nil {
		t.Fatal("usulan terkunci tidak boleh dihapus")
	}
	if err := e.l.BukaUlang(e.ctx, e.barang, k.ID, TahapSimanUE1); err != nil {
		t.Fatal(err)
	}
	if _, err := e.l.HapusUsulan(e.ctx, e.satkerA, k.ID); err != nil {
		t.Fatalf("setelah dibuka kuncinya, satker pemilik boleh menghapus: %v", err)
	}
}

// Penghapusan tidak boleh menghapus usulan yang selesai di antara pemeriksaan dan penghapusan: syarat berada di Repo (satu pernyataan).
func TestRepoHapusPenjualanMenolakUsulanSelesai(t *testing.T) {
	e := baru(t)
	k := e.buat(e.satkerA)
	e.selesaikanSemua(k)
	if ok, err := e.repo.HapusPenjualan(e.ctx, k.ID); err != nil || ok {
		t.Errorf("repo menghapus usulan selesai: ok=%v err=%v", ok, err)
	}
	if ok, err := e.repo.HapusPenjualan(e.ctx, 9999); err != nil || ok {
		t.Errorf("repo menghapus usulan tak ada: ok=%v err=%v", ok, err)
	}
}
