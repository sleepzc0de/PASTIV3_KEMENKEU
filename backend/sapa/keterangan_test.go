package sapa

import (
	"errors"
	"strings"
	"testing"
)

// Masukan pengguna: nomor SK Tim yang dibuat di luar aplikasi tidak bisa diedit. Keterangan tahap yang dilewati (nomor dan tanggal dokumen) kini boleh diubah oleh pemilik tahap
// walau tahap sesudahnya sudah selesai (hanya catatan, tidak ada dokumen yang bergantung padanya), tetapi tidak setelah usulan selesai (terkunci).
func TestUbahKeteranganTahapDilewati(t *testing.T) {
	e := baru(t)
	k := e.buat(e.satkerA)
	if err := e.l.Lewati(e.ctx, e.satkerA, k.ID, TahapTim, "SK Tim nomor KEP-1/2026 tanggal 2 Januari 2026"); err != nil {
		t.Fatal(err)
	}
	if td := e.tahap(e.satkerA, k.ID, TahapTim); !td.DapatUbahKeterangan || td.Status != StatusDilewati {
		t.Fatalf("tahap dilewati: dapat_ubah_keterangan=%v status=%s", td.DapatUbahKeterangan, td.Status)
	}
	if td := e.tahap(e.satkerA, k.ID, TahapBA); td.DapatUbahKeterangan {
		t.Error("tahap yang belum dikerjakan tidak punya keterangan untuk diubah")
	}

	// diubah: status tetap dilewati dan keterangannya berganti (dirapikan)
	if err := e.l.UbahKeterangan(e.ctx, e.satkerA, k.ID, TahapTim, "  SK Tim nomor KEP-12/2026 tanggal 5 Januari 2026  "); err != nil {
		t.Fatal(err)
	}
	td := e.tahap(e.satkerA, k.ID, TahapTim)
	if td.Status != StatusDilewati || td.Catatan != "SK Tim nomor KEP-12/2026 tanggal 5 Januari 2026" {
		t.Errorf("setelah diubah: status=%s catatan=%q", td.Status, td.Catatan)
	}

	// validasi sama dengan saat melewati
	var ev *ErrValidasi
	adalah(t, e.l.UbahKeterangan(e.ctx, e.satkerA, k.ID, TahapTim, "abc"), &ev)
	adalah(t, e.l.UbahKeterangan(e.ctx, e.satkerA, k.ID, TahapTim, strings.Repeat("x", MaksCatatan+1)), &ev)

	// tetap boleh diubah walau tahap sesudahnya sudah selesai: tahap yang dilewati tidak punya dokumen yang bergantung pada keterangannya
	if err := e.l.Lewati(e.ctx, e.satkerA, k.ID, TahapBA, "dibuat di luar aplikasi"); err != nil {
		t.Fatal(err)
	}
	if _, err := e.l.Hasilkan(e.ctx, e.satkerA, k.ID, TahapNDSatker, jsonDari(t, ContohNDSatker())); err != nil {
		t.Fatal(err)
	}
	if err := e.l.UbahKeterangan(e.ctx, e.satkerA, k.ID, TahapTim, "SK Tim nomor KEP-99/2026 tanggal 9 Januari 2026"); err != nil {
		t.Errorf("tahap sesudahnya sudah selesai: %v", err)
	}
	// ... sedangkan membuat ulang tahap yang dilewati tetap terkunci (aturan lama untuk dokumen)
	var kf *ErrKonflik
	adalah(t, e.l.SimpanDraf(e.ctx, e.satkerA, k.ID, TahapTim, []byte(`{}`)), &kf)

	// bukan tahap yang dilewati: ditolak (tahap selesai di aplikasi dan tahap yang belum dikerjakan)
	adalah(t, e.l.UbahKeterangan(e.ctx, e.satkerA, k.ID, TahapNDSatker, "keterangan apa pun"), &kf)
	adalah(t, e.l.UbahKeterangan(e.ctx, e.satkerA, k.ID, TahapNadineSatker, "keterangan apa pun"), &kf)
	if err := e.l.UbahKeterangan(e.ctx, e.satkerA, k.ID, "aneh", "keterangan apa pun"); !errors.Is(err, ErrTidakDitemukan) {
		t.Errorf("tahap tak dikenal: %v", err)
	}

	// hak: hanya pemilik tahap (Satker) dan superadmin; yang lain ditolak, dan yang tidak melihat usulan mendapat "tidak ditemukan"
	for nama, id := range map[string]Identitas{"kanwil": e.kanwil, "ue1": e.ue1, "pengguna barang": e.barang} {
		if err := e.l.UbahKeterangan(e.ctx, id, k.ID, TahapTim, "keterangan dari bukan pemilik"); !errors.Is(err, ErrTidakBerhak) {
			t.Errorf("%s: %v, want ErrTidakBerhak", nama, err)
		}
	}
	if err := e.l.UbahKeterangan(e.ctx, e.satkerB, k.ID, TahapTim, "keterangan dari satker lain"); !errors.Is(err, ErrTidakDitemukan) {
		t.Errorf("satker lain: %v", err)
	}
	if err := e.l.UbahKeterangan(e.ctx, e.tanpa, k.ID, TahapTim, "keterangan tanpa peran"); !errors.Is(err, ErrTanpaPeran) {
		t.Errorf("tanpa peran: %v", err)
	}
	if err := e.l.UbahKeterangan(e.ctx, e.admin, k.ID, TahapTim, "SK Tim nomor KEP-77/2026 oleh superadmin"); err != nil {
		t.Errorf("superadmin: %v", err)
	}
}

// Usulan yang sudah selesai terkunci total: keterangan tahap yang dilewati pun tidak dapat diubah sampai kuncinya dibuka.
func TestUbahKeteranganDitolakPadaUsulanTerkunci(t *testing.T) {
	e := baru(t)
	k := e.buat(e.satkerA)
	e.selesaikanSemua(k) // tahap Tim dan BA dilewati di dalamnya
	var kf *ErrKonflik
	adalah(t, e.l.UbahKeterangan(e.ctx, e.satkerA, k.ID, TahapTim, "keterangan baru"), &kf)
	if kf == nil || kf.Pesan != PesanTerkunci {
		t.Errorf("pesan = %v, want pesan terkunci", kf)
	}
	for _, td := range mustDetail(t, e, e.admin, k.ID).Tahap {
		if td.DapatUbahKeterangan {
			t.Errorf("tahap %s: dapat_ubah_keterangan harus false pada usulan terkunci", td.Kunci)
		}
	}
}

func mustDetail(t *testing.T, e *env, id Identitas, pid int64) *DetailUsulan {
	t.Helper()
	d, err := e.l.DetailUsulan(e.ctx, id, pid)
	if err != nil {
		t.Fatal(err)
	}
	return d
}
