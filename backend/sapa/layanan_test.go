package sapa

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

const (
	kodeA = "015010199409294002" // satker contoh, UE1 01501
	kodeB = "015040199119091000" // satker lain, UE1 01504

	guid1 = "6F9619FF-8B86-D011-B42D-00C04FC964FF"
	guid2 = "A1B2C3D4-0000-4000-8000-000000000002"
)

type env struct {
	t    *testing.T
	repo *MemRepo
	l    *Layanan
	ctx  context.Context

	admin, satkerA, satkerB, kanwil, ue1, ue1Lain, tanpa Identitas
}

func baru(t *testing.T) *env {
	repo := NewMemRepo()
	repo.Satker[kodeA] = SatkerInfo{Kode: kodeA, Nama: "KPKNL Jakarta I", KabKota: "KOTA JAKARTA PUSAT", KodeUE1: "01501"}
	repo.refUE1["01501"] = RefUE1{Kode: "01501", Nama: "Ditjen Contoh", Sekretaris: "Sekretaris Direktorat Jenderal Contoh"}
	return &env{
		t: t, repo: repo, l: &Layanan{Repo: repo}, ctx: context.Background(),
		admin:   Identitas{UserID: "u-admin", Nama: "Admin", Admin: true},
		satkerA: Identitas{UserID: "u-a", Nama: "Petugas A", Peran: PeranSatker, KodeSatker: kodeA},
		satkerB: Identitas{UserID: "u-b", Nama: "Petugas B", Peran: PeranSatker, KodeSatker: kodeB},
		kanwil:  Identitas{UserID: "u-k", Nama: "Petugas Kanwil", Peran: PeranKanwil},
		ue1:     Identitas{UserID: "u-u", Nama: "Petugas UE1", Peran: PeranUE1, KodeUE1: "01501"},
		ue1Lain: Identitas{UserID: "u-u2", Nama: "UE1 Lain", Peran: PeranUE1, KodeUE1: "01504"},
		tanpa:   Identitas{UserID: "u-x", Nama: "Tanpa Peran"},
	}
}

func (e *env) buat(id Identitas) *KasusInfo {
	e.t.Helper()
	k, err := e.l.BuatUsulan(e.ctx, id, kodeA, "")
	if err != nil {
		e.t.Fatalf("BuatUsulan: %v", err)
	}
	return k
}

func jsonDari(t *testing.T, v interface{}) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func (e *env) tahap(id Identitas, pid int64, kunci string) TahapDetail {
	e.t.Helper()
	d, err := e.l.DetailUsulan(e.ctx, id, pid)
	if err != nil {
		e.t.Fatalf("DetailUsulan: %v", err)
	}
	for _, td := range d.Tahap {
		if td.Kunci == kunci {
			return td
		}
	}
	e.t.Fatalf("tahap %s tidak ada", kunci)
	return TahapDetail{}
}

func adalah(t *testing.T, err error, target interface{}) {
	t.Helper()
	switch tg := target.(type) {
	case error:
		if !errors.Is(err, tg) {
			t.Errorf("err = %v, want %v", err, tg)
		}
	case **ErrKonflik:
		if !errors.As(err, tg) {
			t.Errorf("err = %v, want ErrKonflik", err)
		}
	case **ErrValidasi:
		if !errors.As(err, tg) {
			t.Errorf("err = %v, want ErrValidasi", err)
		}
	}
}

func TestBuatUsulan(t *testing.T) {
	e := baru(t)
	k := e.buat(e.satkerA)
	if !strings.HasPrefix(k.Noreg, "PJ-") || k.KodeSatker != kodeA || k.KodeUE1 != "01501" {
		t.Errorf("usulan = %+v", k)
	}
	if k.NamaSatker != "KPKNL Jakarta I" {
		t.Errorf("nama satker harus diambil dari data Digitalisasi Aset: %q", k.NamaSatker)
	}
	if k2 := e.buat(e.admin); k2.Noreg == k.Noreg {
		t.Error("noreg harus unik")
	}

	// Kode ber-KP dirapikan menjadi 18 digit.
	if k3, err := e.l.BuatUsulan(e.ctx, e.satkerA, kodeA+"KP", ""); err != nil || k3.KodeSatker != kodeA {
		t.Errorf("kode ber-KP: %v %+v", err, k3)
	}

	var ev *ErrValidasi
	_, err := e.l.BuatUsulan(e.ctx, e.satkerA, "123", "")
	adalah(t, err, &ev)
	_, err = e.l.BuatUsulan(e.ctx, e.admin, "99999999999999999X", "X")
	adalah(t, err, &ev)

	// Satker tak dikenal: nama wajib diisi manual.
	_, err = e.l.BuatUsulan(e.ctx, e.admin, kodeB, "")
	adalah(t, err, &ev)
	if k4, err := e.l.BuatUsulan(e.ctx, e.admin, kodeB, "  Satker Manual "); err != nil || k4.NamaSatker != "Satker Manual" {
		t.Errorf("nama manual: %v %+v", err, k4)
	}

	// Hak membuat.
	if _, err := e.l.BuatUsulan(e.ctx, e.satkerB, kodeA, ""); !errors.Is(err, ErrTidakBerhak) {
		t.Errorf("satker lain: %v", err)
	}
	if _, err := e.l.BuatUsulan(e.ctx, e.kanwil, kodeA, ""); !errors.Is(err, ErrTidakBerhak) {
		t.Errorf("kanwil: %v", err)
	}
	if _, err := e.l.BuatUsulan(e.ctx, e.tanpa, kodeA, ""); !errors.Is(err, ErrTanpaPeran) {
		t.Errorf("tanpa peran: %v", err)
	}
}

func TestKeterlihatanUsulan(t *testing.T) {
	e := baru(t)
	k := e.buat(e.satkerA)

	for nama, id := range map[string]Identitas{"satker sendiri": e.satkerA, "admin": e.admin, "kanwil": e.kanwil, "ue1 sendiri": e.ue1} {
		if _, err := e.l.DetailUsulan(e.ctx, id, k.ID); err != nil {
			t.Errorf("%s harus melihat: %v", nama, err)
		}
	}
	// Yang tidak berhak melihat mendapat "tidak ditemukan" (tidak membocorkan keberadaan usulan).
	for nama, id := range map[string]Identitas{"satker lain": e.satkerB, "ue1 lain": e.ue1Lain} {
		if _, err := e.l.DetailUsulan(e.ctx, id, k.ID); !errors.Is(err, ErrTidakDitemukan) {
			t.Errorf("%s: %v", nama, err)
		}
	}
	if _, err := e.l.DetailUsulan(e.ctx, e.tanpa, k.ID); !errors.Is(err, ErrTanpaPeran) {
		t.Errorf("tanpa peran: %v", err)
	}
	if _, err := e.l.DetailUsulan(e.ctx, e.admin, 9999); !errors.Is(err, ErrTidakDitemukan) {
		t.Errorf("usulan tak ada: %v", err)
	}

	// Daftar mengikuti cakupan.
	e.buat(e.admin)
	h, err := e.l.DaftarUsulan(e.ctx, e.satkerB, FilterDaftar{})
	if err != nil || h.Total != 0 {
		t.Errorf("satker lain: %v %+v", err, h)
	}
	h, err = e.l.DaftarUsulan(e.ctx, e.satkerA, FilterDaftar{})
	if err != nil || h.Total != 2 {
		t.Errorf("satker sendiri: %v %+v", err, h)
	}
	if h.Usulan[0].TahapSaatIni != TahapTim || h.Usulan[0].PeranSaatIni != PeranSatker || h.Usulan[0].TahapTotal != 10 || h.Usulan[0].Selesai {
		t.Errorf("ringkasan = %+v", h.Usulan[0])
	}
	if _, err := e.l.DaftarUsulan(e.ctx, e.tanpa, FilterDaftar{}); !errors.Is(err, ErrTanpaPeran) {
		t.Errorf("daftar tanpa peran: %v", err)
	}
	var ev *ErrValidasi
	_, err = e.l.DaftarUsulan(e.ctx, e.admin, FilterDaftar{Status: "aneh"})
	adalah(t, err, &ev)
}

func TestUrutanDanHakTahap(t *testing.T) {
	e := baru(t)
	k := e.buat(e.satkerA)
	nd := jsonDari(t, ContohNDSatker())

	// Tidak boleh melompati tahap.
	var kf *ErrKonflik
	_, err := e.l.Hasilkan(e.ctx, e.satkerA, k.ID, TahapNDSatker, nd)
	adalah(t, err, &kf)
	if kf != nil && !strings.Contains(kf.Pesan, "Pembentukan Tim") {
		t.Errorf("pesan harus menyebut tahap yang harus selesai: %q", kf.Pesan)
	}
	err = e.l.Selesaikan(e.ctx, e.satkerA, k.ID, TahapNadineSatker, "ND-1", "2026-10-01", "")
	adalah(t, err, &kf)

	// Peran yang salah.
	if err := e.l.Lewati(e.ctx, e.ue1, k.ID, TahapTim, "dibuat di luar aplikasi"); !errors.Is(err, ErrTidakDitemukan) && !errors.Is(err, ErrTidakBerhak) {
		t.Errorf("ue1: %v", err)
	}
	if err := e.l.Lewati(e.ctx, e.kanwil, k.ID, TahapTim, "dibuat di luar aplikasi"); !errors.Is(err, ErrTidakBerhak) {
		t.Errorf("kanwil pada tahap satker: %v", err)
	}
	if err := e.l.Lewati(e.ctx, e.satkerB, k.ID, TahapTim, "dibuat di luar aplikasi"); !errors.Is(err, ErrTidakDitemukan) {
		t.Errorf("satker lain: %v", err)
	}

	// Tahap tak dikenal.
	if err := e.l.Lewati(e.ctx, e.satkerA, k.ID, "aneh", "dibuat di luar aplikasi"); !errors.Is(err, ErrTidakDitemukan) {
		t.Errorf("tahap tak dikenal: %v", err)
	}
}

func TestLewatiTahap(t *testing.T) {
	e := baru(t)
	k := e.buat(e.satkerA)

	var ev *ErrValidasi
	adalah(t, e.l.Lewati(e.ctx, e.satkerA, k.ID, TahapTim, "  "), &ev)
	adalah(t, e.l.Lewati(e.ctx, e.satkerA, k.ID, TahapTim, strings.Repeat("x", MaksCatatan+1)), &ev)

	if err := e.l.Lewati(e.ctx, e.satkerA, k.ID, TahapTim, "SK Tim No. 5/2026 dibuat manual"); err != nil {
		t.Fatal(err)
	}
	if td := e.tahap(e.satkerA, k.ID, TahapTim); td.Status != StatusDilewati || td.Catatan == "" {
		t.Errorf("tahap = %+v", td)
	}
	if err := e.l.Lewati(e.ctx, e.satkerA, k.ID, TahapBA, "BA dibuat manual"); err != nil {
		t.Fatal(err)
	}

	// ND Satker tidak boleh dilewati.
	var kf *ErrKonflik
	adalah(t, e.l.Lewati(e.ctx, e.satkerA, k.ID, TahapNDSatker, "alasan apa pun"), &kf)
}

// siapkanNDSatker melewati Tim dan BA lalu menghasilkan ND Satker.
func (e *env) siapkanNDSatker(k *KasusInfo) *HasilDokumen {
	e.t.Helper()
	for _, tk := range []string{TahapTim, TahapBA} {
		if err := e.l.Lewati(e.ctx, e.satkerA, k.ID, tk, "dibuat di luar aplikasi"); err != nil {
			e.t.Fatalf("lewati %s: %v", tk, err)
		}
	}
	h, err := e.l.Hasilkan(e.ctx, e.satkerA, k.ID, TahapNDSatker, jsonDari(e.t, ContohNDSatker()))
	if err != nil {
		e.t.Fatalf("Hasilkan ND Satker: %v", err)
	}
	return h
}

func TestHasilkanNDSatkerDenganTemplateBawaan(t *testing.T) {
	e := baru(t)
	k := e.buat(e.satkerA)
	h := e.siapkanNDSatker(k)

	if h.Dokumen.NamaFile == "" || h.Dokumen.Ukuran < 1000 || h.Dokumen.JenisLabel == "" || h.Dokumen.Tahap != TahapNDSatker {
		t.Errorf("dokumen = %+v", h.Dokumen)
	}
	td := e.tahap(e.satkerA, k.ID, TahapNDSatker)
	if td.Status != StatusSelesai || len(td.Dokumen) != 1 || td.Data == nil {
		t.Errorf("tahap = %+v", td)
	}

	// Berkas bisa diunduh pemilik, tetapi bukan satker lain.
	info, berkas, err := e.l.UnduhDokumen(e.ctx, e.satkerA, h.Dokumen.ID)
	if err != nil || info.ID != h.Dokumen.ID || len(berkas) != h.Dokumen.Ukuran {
		t.Errorf("unduh: %v %+v", err, info)
	}
	if _, _, err := e.l.UnduhDokumen(e.ctx, e.satkerB, h.Dokumen.ID); !errors.Is(err, ErrTidakDitemukan) {
		t.Errorf("satker lain mengunduh: %v", err)
	}
	if _, _, err := e.l.UnduhDokumen(e.ctx, e.ue1, h.Dokumen.ID); err != nil {
		t.Errorf("UE1 harus bisa mengunduh: %v", err)
	}
	if _, _, err := e.l.UnduhDokumen(e.ctx, e.tanpa, h.Dokumen.ID); !errors.Is(err, ErrTanpaPeran) {
		t.Errorf("tanpa peran: %v", err)
	}
	if _, _, err := e.l.UnduhDokumen(e.ctx, e.admin, 12345); !errors.Is(err, ErrTidakDitemukan) {
		t.Errorf("dokumen tak ada: %v", err)
	}

	// Membuat ulang boleh selama tahap sesudahnya belum selesai: dokumen lama tetap ada sebagai riwayat.
	if _, err := e.l.Hasilkan(e.ctx, e.satkerA, k.ID, TahapNDSatker, jsonDari(t, ContohNDSatker())); err != nil {
		t.Fatal(err)
	}
	if td := e.tahap(e.satkerA, k.ID, TahapNDSatker); len(td.Dokumen) != 2 {
		t.Errorf("riwayat dokumen = %d", len(td.Dokumen))
	}
}

func TestHasilkanMenolakDataTidakSah(t *testing.T) {
	e := baru(t)
	k := e.buat(e.satkerA)
	for _, tk := range []string{TahapTim, TahapBA} {
		_ = e.l.Lewati(e.ctx, e.satkerA, k.ID, tk, "dibuat di luar aplikasi")
	}
	var ev *ErrValidasi
	d := ContohNDSatker()
	d.Barang = nil
	_, err := e.l.Hasilkan(e.ctx, e.satkerA, k.ID, TahapNDSatker, jsonDari(t, d))
	adalah(t, err, &ev)
	if ev != nil && len(ev.Rincian) == 0 {
		t.Error("rincian validasi kosong")
	}
	_, err = e.l.Hasilkan(e.ctx, e.satkerA, k.ID, TahapNDSatker, []byte(`{"barang": "bukan daftar"}`))
	adalah(t, err, &ev)
	_, err = e.l.Hasilkan(e.ctx, e.satkerA, k.ID, TahapNDSatker, []byte(`null`))
	adalah(t, err, &ev)
	_, err = e.l.Hasilkan(e.ctx, e.satkerA, k.ID, TahapNDSatker, nil)
	adalah(t, err, &ev)
	_, err = e.l.Hasilkan(e.ctx, e.satkerA, k.ID, TahapNDSatker, make([]byte, MaksDataTahap+1))
	adalah(t, err, &ev)

	// Gagal validasi tidak boleh menandai tahap selesai atau menyimpan dokumen.
	if td := e.tahap(e.satkerA, k.ID, TahapNDSatker); td.Status != StatusBelum || len(td.Dokumen) != 0 {
		t.Errorf("tahap = %+v", td)
	}
}

func TestTahapTanpaTemplate(t *testing.T) {
	e := baru(t)
	k := e.buat(e.satkerA)
	// SK Tim tidak punya template bawaan: harus ada unggahan admin.
	_, err := e.l.Hasilkan(e.ctx, e.satkerA, k.ID, TahapTim, jsonDari(t, ContohTim()))
	if !errors.Is(err, ErrTemplateTidakAda) {
		t.Errorf("err = %v", err)
	}
	if td := e.tahap(e.satkerA, k.ID, TahapTim); td.TemplateTersedia[DokSKTim] {
		t.Error("template SK Tim seharusnya belum tersedia")
	}

	tpl := buatDocx(t, para("SK &lt;&lt;jenis tim&gt;&gt; &lt;&lt;nama satker&gt;&gt;"))
	if _, err := e.l.UnggahTemplate(e.ctx, e.admin, DokSKTim, "SK Tim.docx", tpl, ""); err != nil {
		t.Fatalf("unggah: %v", err)
	}
	if td := e.tahap(e.satkerA, k.ID, TahapTim); !td.TemplateTersedia[DokSKTim] {
		t.Error("template SK Tim seharusnya tersedia setelah diunggah")
	}
	if _, err := e.l.Hasilkan(e.ctx, e.satkerA, k.ID, TahapTim, jsonDari(t, ContohTim())); err != nil {
		t.Fatalf("hasilkan SK Tim: %v", err)
	}
}

func TestEksternalDanAlurPenuh(t *testing.T) {
	e := baru(t)
	k := e.buat(e.satkerA)
	e.siapkanNDSatker(k)

	var ev *ErrValidasi
	var kf *ErrKonflik
	// ND Satker yang diselesaikan lewat Selesaikan (bukan tahap eksternal) ditolak.
	adalah(t, e.l.Selesaikan(e.ctx, e.satkerA, k.ID, TahapNDSatker, "", "", ""), &kf)

	// Nadine: nomor dan tanggal wajib.
	adalah(t, e.l.Selesaikan(e.ctx, e.satkerA, k.ID, TahapNadineSatker, "", "", ""), &ev)
	adalah(t, e.l.Selesaikan(e.ctx, e.satkerA, k.ID, TahapNadineSatker, "ND-1/2026", "31-12-2026", ""), &ev)
	adalah(t, e.l.Selesaikan(e.ctx, e.satkerA, k.ID, TahapNadineSatker, strings.Repeat("N", 151), "2026-10-01", ""), &ev)
	if err := e.l.Selesaikan(e.ctx, e.satkerA, k.ID, TahapNadineSatker, "ND-1/2026", "2026-10-01", "ok"); err != nil {
		t.Fatal(err)
	}
	if td := e.tahap(e.satkerA, k.ID, TahapNadineSatker); td.Status != StatusSelesai || td.Nomor != "ND-1/2026" || td.Tanggal != "2026-10-01" {
		t.Errorf("tahap = %+v", td)
	}

	// ND Satker tidak boleh dibuat ulang setelah Nadine selesai.
	_, err := e.l.Hasilkan(e.ctx, e.satkerA, k.ID, TahapNDSatker, jsonDari(t, ContohNDSatker()))
	adalah(t, err, &kf)

	// Tahap Satker berikutnya (SIMAN): nomor/tanggal opsional.
	if err := e.l.Selesaikan(e.ctx, e.satkerA, k.ID, TahapSimanSatker, "", "", ""); err != nil {
		t.Fatal(err)
	}
	// Kanwil, bukan satker, yang meneliti tiket.
	if err := e.l.Selesaikan(e.ctx, e.satkerA, k.ID, TahapSimanKanwil, "", "", ""); !errors.Is(err, ErrTidakBerhak) {
		t.Errorf("satker pada tahap kanwil: %v", err)
	}
	if err := e.l.Selesaikan(e.ctx, e.kanwil, k.ID, TahapSimanKanwil, "", "", "diteliti"); err != nil {
		t.Fatal(err)
	}
	// UE1 menerima.
	if err := e.l.Selesaikan(e.ctx, e.ue1Lain, k.ID, TahapUE1Terima, "", "", ""); !errors.Is(err, ErrTidakDitemukan) {
		t.Errorf("UE1 lain: %v", err)
	}
	if err := e.l.Selesaikan(e.ctx, e.ue1, k.ID, TahapUE1Terima, "", "", ""); err != nil {
		t.Fatal(err)
	}

	// Saran ND UE1 memuat nomor/tanggal Nadine dan data satker.
	sar, ok := e.tahap(e.ue1, k.ID, TahapNDUE1).Saran.(DataNDUE1)
	if !ok || sar.NomorND != "ND-1/2026" || sar.TanggalND != "2026-10-01" || sar.SekretarisUE1 == "" || !strings.Contains(sar.HalND, "KPKNL Jakarta I") {
		t.Errorf("saran ND UE1 = %+v", sar)
	}

	// ND UE1: hanya UE1 (atau admin) yang boleh; satker tidak.
	nd1 := jsonDari(t, ContohNDUE1())
	if _, err := e.l.Hasilkan(e.ctx, e.satkerA, k.ID, TahapNDUE1, nd1); !errors.Is(err, ErrTidakBerhak) {
		t.Errorf("satker pada tahap UE1: %v", err)
	}
	if _, err := e.l.Hasilkan(e.ctx, e.ue1, k.ID, TahapNDUE1, nd1); err != nil {
		t.Fatalf("ND UE1: %v", err)
	}
	if err := e.l.Selesaikan(e.ctx, e.ue1, k.ID, TahapNadineUE1, "", "", ""); err != nil {
		t.Fatal(err)
	}

	// Belum selesai sampai tahap terakhir.
	d, _ := e.l.DetailUsulan(e.ctx, e.ue1, k.ID)
	if d.Selesai || d.TahapSaatIni != TahapSimanUE1 {
		t.Errorf("detail = selesai %v, saat ini %s", d.Selesai, d.TahapSaatIni)
	}
	if err := e.l.Selesaikan(e.ctx, e.ue1, k.ID, TahapSimanUE1, "", "", ""); err != nil {
		t.Fatal(err)
	}
	d, _ = e.l.DetailUsulan(e.ctx, e.ue1, k.ID)
	if !d.Selesai || d.TahapSaatIni != "" {
		t.Errorf("detail akhir = selesai %v, saat ini %q", d.Selesai, d.TahapSaatIni)
	}
	if h, _ := e.l.DaftarUsulan(e.ctx, e.admin, FilterDaftar{Status: "selesai"}); h.Total != 1 || !h.Usulan[0].Selesai || h.Usulan[0].TahapSelesai != 10 {
		t.Errorf("daftar selesai = %+v", h)
	}
	if h, _ := e.l.DaftarUsulan(e.ctx, e.admin, FilterDaftar{Status: "berjalan"}); h.Total != 0 {
		t.Errorf("daftar berjalan = %+v", h)
	}
}

func TestNDUE1MembutuhkanNDSatker(t *testing.T) {
	e := baru(t)
	k := e.buat(e.satkerA)
	// Memaksa status tahap sebelumnya selesai tanpa data ND Satker (mis. data rusak).
	for _, tp := range TahapPenjualan[:7] {
		_ = e.repo.SimpanTahap(e.ctx, k.ID, TahapRow{Kunci: tp.Kunci, Status: StatusSelesai})
	}
	var kf *ErrKonflik
	_, err := e.l.Hasilkan(e.ctx, e.ue1, k.ID, TahapNDUE1, jsonDari(t, ContohNDUE1()))
	adalah(t, err, &kf)
}

func TestSimpanDraf(t *testing.T) {
	e := baru(t)
	k := e.buat(e.satkerA)

	// Draf tidak divalidasi kelengkapannya, tetapi harus JSON yang bisa dibaca.
	if err := e.l.SimpanDraf(e.ctx, e.satkerA, k.ID, TahapTim, []byte(`{"jabatan_pimpinan": "  Kepala Kantor  "}`)); err != nil {
		t.Fatal(err)
	}
	td := e.tahap(e.satkerA, k.ID, TahapTim)
	if td.Status != StatusDraft {
		t.Errorf("status = %s", td.Status)
	}
	var tim DataTim
	if err := json.Unmarshal(td.Data, &tim); err != nil || tim.JabatanPimpinan != "Kepala Kantor" {
		t.Errorf("data draf = %s (%v)", td.Data, err)
	}

	var ev *ErrValidasi
	adalah(t, e.l.SimpanDraf(e.ctx, e.satkerA, k.ID, TahapTim, []byte(`{"anggota": 5}`)), &ev)
	adalah(t, e.l.SimpanDraf(e.ctx, e.satkerA, k.ID, TahapTim, nil), &ev)

	var kf *ErrKonflik
	adalah(t, e.l.SimpanDraf(e.ctx, e.satkerA, k.ID, TahapNDSatker, []byte(`{}`)), &kf) // urutan
	if err := e.l.SimpanDraf(e.ctx, e.satkerB, k.ID, TahapTim, []byte(`{}`)); !errors.Is(err, ErrTidakDitemukan) {
		t.Errorf("satker lain: %v", err)
	}
	// Tahap eksternal tidak memakai formulir.
	for _, tk := range []string{TahapTim, TahapBA} {
		_ = e.l.Lewati(e.ctx, e.satkerA, k.ID, tk, "dibuat di luar aplikasi")
	}
	e.siapkanNDSatkerSisa(k)
	adalah(t, e.l.SimpanDraf(e.ctx, e.satkerA, k.ID, TahapNadineSatker, []byte(`{}`)), &kf)
}

func (e *env) siapkanNDSatkerSisa(k *KasusInfo) {
	if _, err := e.l.Hasilkan(e.ctx, e.satkerA, k.ID, TahapNDSatker, jsonDari(e.t, ContohNDSatker())); err != nil {
		e.t.Fatal(err)
	}
}

func TestDrafTidakMenimpaTahapSelesaiYangTerkunci(t *testing.T) {
	e := baru(t)
	k := e.buat(e.satkerA)
	e.siapkanNDSatker(k)
	if err := e.l.Selesaikan(e.ctx, e.satkerA, k.ID, TahapNadineSatker, "ND-1", "2026-10-01", ""); err != nil {
		t.Fatal(err)
	}
	var kf *ErrKonflik
	adalah(t, e.l.SimpanDraf(e.ctx, e.satkerA, k.ID, TahapNDSatker, jsonDari(t, ContohNDSatker())), &kf)
	adalah(t, e.l.Lewati(e.ctx, e.satkerA, k.ID, TahapBA, "alasan lain lagi"), &kf)
}

func TestBukaUlang(t *testing.T) {
	e := baru(t)
	k := e.buat(e.satkerA)
	e.siapkanNDSatker(k)
	_ = e.l.Selesaikan(e.ctx, e.satkerA, k.ID, TahapNadineSatker, "ND-1", "2026-10-01", "")

	if err := e.l.BukaUlang(e.ctx, e.satkerA, k.ID, TahapNadineSatker); !errors.Is(err, ErrTidakBerhak) {
		t.Errorf("non-admin: %v", err)
	}
	var kf *ErrKonflik
	// ND Satker terkunci oleh Nadine yang sudah selesai; buka Nadine dulu.
	adalah(t, e.l.BukaUlang(e.ctx, e.admin, k.ID, TahapNDSatker), &kf)
	adalah(t, e.l.BukaUlang(e.ctx, e.admin, k.ID, TahapSimanSatker), &kf) // belum selesai

	if err := e.l.BukaUlang(e.ctx, e.admin, k.ID, TahapNadineSatker); err != nil {
		t.Fatal(err)
	}
	td := e.tahap(e.admin, k.ID, TahapNadineSatker)
	if td.Status != StatusDraft || td.Nomor != "ND-1" {
		t.Errorf("tahap = %+v", td)
	}
	if err := e.l.BukaUlang(e.ctx, e.admin, k.ID, TahapNDSatker); err != nil {
		t.Fatalf("setelah Nadine dibuka, ND Satker boleh dibuka: %v", err)
	}
	if err := e.l.BukaUlang(e.ctx, e.admin, k.ID, "aneh"); !errors.Is(err, ErrTidakDitemukan) {
		t.Errorf("tahap tak dikenal: %v", err)
	}
	// Dokumen lama tetap ada sebagai riwayat.
	if td := e.tahap(e.admin, k.ID, TahapNDSatker); len(td.Dokumen) != 1 || td.Status != StatusDraft {
		t.Errorf("tahap ND = %+v", td)
	}
}

func TestAturPeran(t *testing.T) {
	e := baru(t)
	e.repo.Pengguna = []PeranRow{{UserID: guid1, Username: "budi", Nama: "Budi", Email: "budi@x"}, {UserID: guid2, Username: "siti", Nama: "Siti"}}

	var ev *ErrValidasi
	if err := e.l.AturPeran(e.ctx, e.satkerA, guid1, PeranKanwil, "", ""); !errors.Is(err, ErrTidakBerhak) {
		t.Errorf("non-admin: %v", err)
	}
	adalah(t, e.l.AturPeran(e.ctx, e.admin, guid1, PeranSatker, "123", ""), &ev)
	adalah(t, e.l.AturPeran(e.ctx, e.admin, guid1, PeranUE1, "", "12"), &ev)
	adalah(t, e.l.AturPeran(e.ctx, e.admin, guid1, "tamu", "", ""), &ev)
	adalah(t, e.l.AturPeran(e.ctx, e.admin, "", PeranKanwil, "", ""), &ev)

	if err := e.l.AturPeran(e.ctx, e.admin, guid1, PeranSatker, kodeA+"KP", "99999"); err != nil {
		t.Fatal(err)
	}
	id, err := e.l.IdentitasDari(e.ctx, guid1, "Budi", false)
	if err != nil || id.Peran != PeranSatker || id.KodeSatker != kodeA || id.KodeUE1 != "" {
		t.Errorf("identitas = %+v (%v)", id, err)
	}
	if err := e.l.AturPeran(e.ctx, e.admin, guid1, PeranUE1, kodeA, "01501"); err != nil {
		t.Fatal(err)
	}
	if id, _ := e.l.IdentitasDari(e.ctx, guid1, "Budi", false); id.Peran != PeranUE1 || id.KodeUE1 != "01501" || id.KodeSatker != "" {
		t.Errorf("identitas = %+v", id)
	}
	list, err := e.l.DaftarPeran(e.ctx, e.admin, "bud")
	if err != nil || len(list) != 1 || list[0].Peran != PeranUE1 {
		t.Errorf("daftar = %+v (%v)", list, err)
	}
	if _, err := e.l.DaftarPeran(e.ctx, e.satkerA, ""); !errors.Is(err, ErrTidakBerhak) {
		t.Errorf("non-admin daftar: %v", err)
	}
	// Peran kosong mencabut.
	if err := e.l.AturPeran(e.ctx, e.admin, guid1, "", "", ""); err != nil {
		t.Fatal(err)
	}
	if id, _ := e.l.IdentitasDari(e.ctx, guid1, "Budi", false); id.Peran != "" {
		t.Errorf("peran harus dicabut: %+v", id)
	}
}

func TestReferensiUE1(t *testing.T) {
	e := baru(t)
	var ev *ErrValidasi
	if err := e.l.SimpanRefUE1(e.ctx, e.satkerA, RefUE1{Kode: "01502", Nama: "X", Sekretaris: "Y"}); !errors.Is(err, ErrTidakBerhak) {
		t.Errorf("non-admin: %v", err)
	}
	adalah(t, e.l.SimpanRefUE1(e.ctx, e.admin, RefUE1{Kode: "1", Nama: "X", Sekretaris: "Y"}), &ev)
	adalah(t, e.l.SimpanRefUE1(e.ctx, e.admin, RefUE1{Kode: "01502", Nama: "", Sekretaris: "Y"}), &ev)
	if err := e.l.SimpanRefUE1(e.ctx, e.admin, RefUE1{Kode: " 01502 ", Nama: " Ditjen Lain ", Sekretaris: " Sekretaris Ditjen Lain "}); err != nil {
		t.Fatal(err)
	}
	list, _ := e.l.DaftarRefUE1(e.ctx, e.admin)
	if len(list) != 2 || list[1].Nama != "Ditjen Lain" || list[1].Sekretaris != "Sekretaris Ditjen Lain" {
		t.Errorf("daftar = %+v", list)
	}
	adalah(t, e.l.HapusRefUE1(e.ctx, e.admin, "x"), &ev)
	if err := e.l.HapusRefUE1(e.ctx, e.admin, "01502"); err != nil {
		t.Fatal(err)
	}
	if list, _ := e.l.DaftarRefUE1(e.ctx, e.admin); len(list) != 1 {
		t.Errorf("daftar = %+v", list)
	}
}

func TestCariSatker(t *testing.T) {
	e := baru(t)
	s, ref, err := e.l.CariSatker(e.ctx, e.satkerB, kodeA+"KP")
	if err != nil || s == nil || s.Nama != "KPKNL Jakarta I" || ref == nil || ref.Sekretaris == "" {
		t.Errorf("hasil = %+v %+v %v", s, ref, err)
	}
	if s, _, err := e.l.CariSatker(e.ctx, e.satkerB, kodeB); err != nil || s != nil {
		t.Errorf("satker tak dikenal: %+v %v", s, err)
	}
	var ev *ErrValidasi
	_, _, err = e.l.CariSatker(e.ctx, e.satkerB, "12")
	adalah(t, err, &ev)
	if _, _, err := e.l.CariSatker(e.ctx, e.tanpa, kodeA); !errors.Is(err, ErrTanpaPeran) {
		t.Errorf("tanpa peran: %v", err)
	}
}

func TestSaranFormulir(t *testing.T) {
	e := baru(t)
	k := e.buat(e.satkerA)
	nd, ok := e.tahap(e.satkerA, k.ID, TahapNDSatker).Saran.(DataNDSatker)
	if !ok || nd.TujuanSurat != "Sekretaris Direktorat Jenderal Contoh" || nd.Kota != "Jakarta Pusat" {
		t.Errorf("saran ND Satker = %+v", nd)
	}
	tim, ok := e.tahap(e.satkerA, k.ID, TahapTim).Saran.(DataTim)
	if !ok || tim.JenisTim == "" || tim.Kota != "Jakarta Pusat" {
		t.Errorf("saran Tim = %+v", tim)
	}
	if td := e.tahap(e.satkerA, k.ID, TahapSimanSatker); td.Saran != nil {
		t.Errorf("tahap eksternal tidak punya saran: %+v", td.Saran)
	}
}

func TestTemplateAdmin(t *testing.T) {
	e := baru(t)
	var ev *ErrValidasi

	if _, err := e.l.DaftarTemplate(e.ctx, e.satkerA); !errors.Is(err, ErrTidakBerhak) {
		t.Errorf("non-admin: %v", err)
	}
	st, err := e.l.DaftarTemplate(e.ctx, e.admin)
	if err != nil || len(st) != len(DaftarJenis) {
		t.Fatalf("daftar = %d (%v)", len(st), err)
	}
	for _, s := range st {
		want := "belum"
		if s.Jenis.Bawaan {
			want = "bawaan"
		}
		if s.Sumber != want || s.Aktif != nil {
			t.Errorf("%s: sumber %q aktif %+v", s.Jenis.Kunci, s.Sumber, s.Aktif)
		}
	}

	good := buatDocx(t, para("SK &lt;&lt;jenis tim&gt;&gt; &lt;&lt;penanda ngawur&gt;&gt;"))
	for nama, c := range map[string]struct {
		file string
		isi  []byte
	}{
		"bukan docx":    {"template.pdf", good},
		"kosong":        {"template.docx", nil},
		"zip rusak":     {"template.docx", []byte("bukan zip")},
		"terlalu besar": {"template.docx", make([]byte, MaksTemplate+1)},
	} {
		if _, err := e.l.UnggahTemplate(e.ctx, e.admin, DokSKTim, c.file, c.isi, ""); err == nil {
			t.Errorf("%s: harus ditolak", nama)
		} else {
			adalah(t, err, &ev)
		}
	}
	if _, err := e.l.UnggahTemplate(e.ctx, e.admin, "aneh", "t.docx", good, ""); !errors.Is(err, ErrTidakDitemukan) {
		t.Errorf("jenis tak dikenal: %v", err)
	}
	if _, err := e.l.UnggahTemplate(e.ctx, e.satkerA, DokSKTim, "t.docx", good, ""); !errors.Is(err, ErrTidakBerhak) {
		t.Errorf("non-admin unggah: %v", err)
	}
	if st, _ := e.l.DaftarTemplate(e.ctx, e.admin); st[0].Aktif != nil {
		t.Error("template yang ditolak tidak boleh tersimpan")
	}

	res, err := e.l.UnggahTemplate(e.ctx, e.admin, DokSKTim, "SK Tim v1.docx", good, "versi awal")
	if err != nil {
		t.Fatal(err)
	}
	if res.Template.Versi != 1 || !res.Template.Aktif || len(res.TidakDikenal) != 1 || res.TidakDikenal[0] != "penanda ngawur" || len(res.Peringatan) == 0 {
		t.Errorf("hasil = %+v", res)
	}
	if len(res.TidakDipakai) == 0 {
		t.Error("penanda yang dikenal tetapi tidak dipakai harus dilaporkan")
	}
	res2, err := e.l.UnggahTemplate(e.ctx, e.admin, DokSKTim, "SK Tim v2.docx", good, "")
	if err != nil || res2.Template.Versi != 2 {
		t.Fatalf("versi 2: %v %+v", err, res2)
	}
	for _, s := range mustDaftar(t, e) {
		if s.Jenis.Kunci == DokSKTim && (s.Sumber != "unggahan" || s.Aktif.Versi != 2 || len(s.Riwayat) != 2) {
			t.Errorf("status SK Tim = %+v", s)
		}
	}
	nama, berkas, err := e.l.UnduhTemplate(e.ctx, e.admin, DokSKTim)
	if err != nil || nama != "SK Tim v2.docx" || len(berkas) == 0 {
		t.Errorf("unduh = %q %d %v", nama, len(berkas), err)
	}

	// Template bawaan bisa diunduh; jenis tanpa template tidak.
	if _, b, err := e.l.UnduhTemplate(e.ctx, e.admin, DokNDSatker); err != nil || len(b) == 0 {
		t.Errorf("unduh bawaan: %v", err)
	}
	if _, _, err := e.l.UnduhTemplate(e.ctx, e.admin, DokBA); !errors.Is(err, ErrTemplateTidakAda) {
		t.Errorf("unduh tanpa template: %v", err)
	}
	if _, _, err := e.l.UnduhTemplate(e.ctx, e.satkerA, DokBA); !errors.Is(err, ErrTidakBerhak) {
		t.Errorf("non-admin unduh: %v", err)
	}
}

func mustDaftar(t *testing.T, e *env) []StatusTemplate {
	t.Helper()
	st, err := e.l.DaftarTemplate(e.ctx, e.admin)
	if err != nil {
		t.Fatal(err)
	}
	return st
}

// Template ND Satker unggahan menggantikan yang bawaan, dan struktur yang rusak (tabel Daftar Barang hilang) ditolak.
func TestUnggahTemplateNDMengganti(t *testing.T) {
	e := baru(t)
	var ev *ErrValidasi
	// Tanpa tabel Daftar Barang dan Checklist: pengisian uji coba harus gagal sehingga template ditolak.
	_, err := e.l.UnggahTemplate(e.ctx, e.admin, DokNDSatker, "nd.docx", buatDocx(t, para("ND &lt;&lt;nama satker&gt;&gt;")), "")
	adalah(t, err, &ev)

	bawaan, _ := TemplateBawaan(DokNDSatker)
	if _, err := e.l.UnggahTemplate(e.ctx, e.admin, DokNDSatker, "nd baru.docx", bawaan, "salinan"); err != nil {
		t.Fatalf("template bawaan sendiri harus lolos uji coba: %v", err)
	}
	if st := mustDaftar(t, e); st[2].Jenis.Kunci != DokNDSatker || st[2].Sumber != "unggahan" {
		t.Errorf("status = %+v", st)
	}
}

func TestNamaKotaMembuangAwalanAdministratif(t *testing.T) {
	cases := map[string]string{
		"KOTA ADM. JAKARTA PUSAT":   "Jakarta Pusat",
		"KOTA ADMINISTRASI JAKARTA": "Jakarta",
		"KAB. BOGOR":                "Bogor",
		"KABUPATEN TOLI-TOLI":       "Toli-toli",
		"kota   bandung":            "Bandung",
		"KOTA":                      "Kota", // tanpa nama di belakangnya: dibiarkan
		"  Surabaya ":               "Surabaya",
		"":                          "",
		"ÉVREUX":                    "Évreux",
		"KOTAMOBAGU":                "Kotamobagu", // bukan awalan "KOTA "
	}
	for in, want := range cases {
		if got := namaKota(in); got != want {
			t.Errorf("namaKota(%q) = %q, want %q", in, got, want)
		}
	}
}
