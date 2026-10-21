package apiakses

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/microsoft/go-mssqldb"
)

// Tes integrasi SQL Server: hanya berjalan bila PASTI_UJI_MSSQL_DSN diisi (database KHUSUS TES yang sudah dimigrasi sampai 060), mis.
//
//	sqlserver://localhost:1433?database=pasti_uji&authenticator=winsspi&encrypt=disable
//
// Data uji memakai tanggal tahun 2001 untuk log/agregat dan kode satker berawalan "UJIAPI"/"01504 0199" yang dibersihkan sebelum dan sesudah tes.

func bukaDBUji(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("PASTI_UJI_MSSQL_DSN")
	if dsn == "" {
		t.Skip("PASTI_UJI_MSSQL_DSN tidak diisi: tes integrasi SQL Server dilewati")
	}
	db, err := sql.Open("sqlserver", dsn)
	if err != nil {
		t.Fatal(err)
	}
	var err2 error
	for i := 0; i < 4; i++ { // SQL Server lokal kadang menutup sambungan pertama yang baru dibuka; coba ulang sebentar
		if err2 = db.Ping(); err2 == nil {
			break
		}
		time.Sleep(300 * time.Millisecond)
	}
	if err2 != nil {
		t.Fatalf("tidak bisa terhubung ke database uji: %v", err2)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func hapusKlienUji(db *sql.DB, ids ...int64) {
	for _, id := range ids {
		db.Exec(`DELETE FROM api_akses_log WHERE klien_id = @p1`, id)
		db.Exec(`DELETE FROM api_akses_agregat WHERE klien_id = @p1`, id)
		db.Exec(`DELETE FROM api_akses_kuota WHERE klien_id = @p1`, id)
		db.Exec(`DELETE FROM api_akses_kunci WHERE klien_id = @p1`, id)
		db.Exec(`DELETE FROM api_akses_klien_dataset WHERE klien_id = @p1`, id)
		db.Exec(`DELETE FROM api_akses_klien WHERE id = @p1`, id)
	}
}

func klienUji(t *testing.T, st *Store, nama string) (int64, string) {
	t.Helper()
	p := pendaftaranSah()
	p.NamaAplikasi = nama
	if g := p.Bersihkan(); len(g) != 0 {
		t.Fatal(g)
	}
	id, kode, err := st.TambahKlien(context.Background(), p, "203.0.113.1")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { hapusKlienUji(st.DB, id) })
	return id, kode
}

func intp(n int) *int { return &n }

func TestIntegrasiSiklusHidupKlienDanKunci(t *testing.T) {
	db := bukaDBUji(t)
	st := &Store{DB: db}
	ctx := context.Background()
	id, kode := klienUji(t, st, "UJI Siklus")

	k, err := st.AmbilKlien(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if k.Kode != kode || k.Status != StatusMenunggu || k.DidaftarkanIP != "203.0.113.1" || strings.Join(k.DatasetDiminta, ",") != "aset.satker,aset.tanah" || len(k.DatasetDiberikan) != 0 {
		t.Fatalf("klien baru = %+v", k)
	}
	if k.PICEmail != "budi.santoso@contoh.go.id" || k.Tujuan == "" {
		t.Errorf("isian tidak tersimpan: %+v", k)
	}

	// Kunci tidak bisa diterbitkan sebelum disetujui.
	if _, err := st.TerbitkanKunci(ctx, BuatKunciBaru{KlienID: id, Prefix: "aaaaaaaaaa", Hash: strings.Repeat("0", 64)}); !errors.Is(err, ErrStatusTidakSesuai) {
		t.Fatalf("sebelum disetujui: %v, want ErrStatusTidakSesuai", err)
	}
	if _, err := st.TerbitkanKunci(ctx, BuatKunciBaru{KlienID: 999999999, Prefix: "aaaaaaaaab", Hash: strings.Repeat("0", 64)}); !errors.Is(err, ErrTidakAda) {
		t.Fatalf("klien tidak ada: %v", err)
	}

	cak := UbahCakupan{DatasetDiberikan: []string{"aset.tanah"}, IPDiizinkan: []string{"198.51.100.0/24"}, UE1Diizinkan: []string{"01504"}, Override: Override{LimitPermintaan: intp(77), KuotaPeriodeDetik: intp(1800)}}
	if err := st.UbahStatus(ctx, id, []string{StatusMenunggu}, StatusDisetujui, "superadmin-uji", "disetujui untuk uji", &cak); err != nil {
		t.Fatal(err)
	}
	if err := st.UbahStatus(ctx, id, []string{StatusMenunggu}, StatusDisetujui, "x", "", nil); !errors.Is(err, ErrStatusTidakSesuai) {
		t.Errorf("menyetujui dua kali: %v", err)
	}
	if err := st.UbahStatus(ctx, 999999999, []string{StatusMenunggu}, StatusDisetujui, "x", "", nil); !errors.Is(err, ErrTidakAda) {
		t.Errorf("klien tidak ada: %v", err)
	}
	k, _ = st.AmbilKlien(ctx, id)
	if k.Status != StatusDisetujui || k.DiputuskanOleh != "superadmin-uji" || k.DiputuskanAt == nil || k.Catatan != "disetujui untuk uji" {
		t.Errorf("setelah disetujui: %+v", k)
	}
	if strings.Join(k.DatasetDiberikan, ",") != "aset.tanah" || strings.Join(k.IPDiizinkan, ",") != "198.51.100.0/24" || strings.Join(k.UE1Diizinkan, ",") != "01504" {
		t.Errorf("scope: %+v", k)
	}
	if k.LimitPermintaan == nil || *k.LimitPermintaan != 77 || k.LimitJendelaDetik != nil || k.KuotaPeriodeDetik == nil || *k.KuotaPeriodeDetik != 1800 {
		t.Errorf("override: %+v %+v %+v", k.LimitPermintaan, k.LimitJendelaDetik, k.KuotaPeriodeDetik)
	}

	// Penerbitan kunci pertama.
	kunci, prefix, _ := BuatKunci()
	hash := HashKunci(KunciHash("pepper"), kunci)
	k1, err := st.TerbitkanKunci(ctx, BuatKunciBaru{KlienID: id, Prefix: prefix, Hash: hash, Label: "produksi", Oleh: "superadmin-uji"})
	if err != nil {
		t.Fatal(err)
	}
	cred, err := st.CariKredensial(ctx, prefix)
	if err != nil || cred == nil {
		t.Fatalf("CariKredensial: %v %v", cred, err)
	}
	if cred.Hash != hash || cred.KlienID != id || cred.KlienStatus != StatusDisetujui || cred.KlienKode != kode || cred.DicabutAt != nil || cred.KedaluwarsaAt != nil {
		t.Errorf("kredensial = %+v", cred)
	}
	if strings.Join(cred.Dataset, ",") != "aset.tanah" || strings.Join(cred.IP, ",") != "198.51.100.0/24" || strings.Join(cred.UE1, ",") != "01504" || cred.Override.LimitPermintaan == nil || *cred.Override.LimitPermintaan != 77 {
		t.Errorf("scope pada kredensial: %+v", cred)
	}
	if c, err := st.CariKredensial(ctx, "zzzzzzzzzz"); err != nil || c != nil {
		t.Errorf("prefix tidak ada: %v %v", c, err)
	}
	// Prefix harus unik.
	if _, err := st.TerbitkanKunci(ctx, BuatKunciBaru{KlienID: id, Prefix: prefix, Hash: hash}); err == nil || !adaGalatUnik(err) {
		t.Errorf("prefix kembar harus ditolak sebagai galat unik: %v", err)
	}
	// Batas jumlah kunci aktif.
	for i := 1; i < MaksKunciAktif; i++ {
		_, p, _ := BuatKunci()
		if _, err := st.TerbitkanKunci(ctx, BuatKunciBaru{KlienID: id, Prefix: p, Hash: hash}); err != nil {
			t.Fatalf("kunci ke-%d: %v", i+1, err)
		}
	}
	_, pLebih, _ := BuatKunci()
	if _, err := st.TerbitkanKunci(ctx, BuatKunciBaru{KlienID: id, Prefix: pLebih, Hash: hash}); !errors.Is(err, ErrTerlaluBanyakKunci) {
		t.Errorf("kunci berlebih: %v", err)
	}

	// Rotasi dengan masa tenggang: kunci lama kedaluwarsa dalam waktu dekat, tidak langsung dicabut.
	_, pRot, _ := BuatKunci()
	kRot, err := st.TerbitkanKunci(ctx, BuatKunciBaru{KlienID: id, Prefix: pRot, Hash: hash, CabutLama: true, Tenggang: 2 * time.Hour})
	if err != nil {
		t.Fatalf("rotasi: %v", err)
	}
	lama, _ := st.CariKredensial(ctx, prefix)
	if lama.DicabutAt != nil || lama.KedaluwarsaAt == nil || lama.KedaluwarsaAt.Before(time.Now().Add(100*time.Minute)) || lama.KedaluwarsaAt.After(time.Now().Add(130*time.Minute)) {
		t.Errorf("kunci lama setelah rotasi bertenggang: dicabut=%v kedaluwarsa=%v", lama.DicabutAt, lama.KedaluwarsaAt)
	}
	baru, _ := st.CariKredensial(ctx, pRot)
	if baru.KedaluwarsaAt != nil || baru.DicabutAt != nil {
		t.Errorf("kunci baru harus bebas: %+v", baru)
	}
	// Rotasi tanpa tenggang: semua kunci lain dicabut seketika.
	_, pRot2, _ := BuatKunci()
	if _, err := st.TerbitkanKunci(ctx, BuatKunciBaru{KlienID: id, Prefix: pRot2, Hash: hash, CabutLama: true, Oleh: "superadmin-uji"}); err != nil {
		t.Fatal(err)
	}
	aktif := 0
	ks, _ := st.DaftarKunci(ctx, id)
	for _, x := range ks {
		if x.Aktif {
			aktif++
			if x.Prefix != pRot2 {
				t.Errorf("hanya kunci terakhir yang boleh aktif, got %s", x.Prefix)
			}
		} else if x.DicabutAt == nil && x.ID != k1 && x.ID != kRot {
			t.Errorf("kunci tidak aktif tanpa tanda dicabut: %+v", x)
		}
	}
	if aktif != 1 {
		t.Errorf("kunci aktif = %d, want 1", aktif)
	}
	// Cabut satu kunci.
	if _, err := st.CabutKunci(ctx, id, 123456789, "x", ""); !errors.Is(err, ErrTidakAda) {
		t.Errorf("cabut kunci tak ada: %v", err)
	}
	if p, err := st.CabutKunci(ctx, id, ks[0].ID, "superadmin-uji", "uji"); err != nil || p != ks[0].Prefix {
		t.Errorf("cabut kunci: %q %v", p, err)
	}
	if _, err := st.CabutKunci(ctx, id, ks[0].ID, "x", ""); !errors.Is(err, ErrTidakAda) {
		t.Errorf("mencabut dua kali: %v", err)
	}
	lain, _ := klienUji(t, st, "UJI Lain")
	if _, err := st.CabutKunci(ctx, lain, ks[1].ID, "x", ""); !errors.Is(err, ErrTidakAda) {
		t.Errorf("kunci klien lain tidak boleh dicabut lewat klien ini: %v", err)
	}

	// Mencabut klien menghentikan semua kuncinya; mengaktifkan kembali tidak menghidupkan kunci lama.
	if err := st.UbahStatus(ctx, id, []string{StatusDisetujui}, StatusDicabut, "superadmin-uji", "uji pencabutan", nil); err != nil {
		t.Fatal(err)
	}
	c, _ := st.CariKredensial(ctx, pRot2)
	if c.DicabutAt == nil || c.KlienStatus != StatusDicabut {
		t.Errorf("setelah klien dicabut: %+v", c)
	}
	if _, err := st.TerbitkanKunci(ctx, BuatKunciBaru{KlienID: id, Prefix: "bbbbbbbbbb", Hash: hash}); !errors.Is(err, ErrStatusTidakSesuai) {
		t.Errorf("klien dicabut tidak bisa diberi kunci: %v", err)
	}
	if err := st.UbahStatus(ctx, id, []string{StatusDicabut}, StatusDisetujui, "superadmin-uji", "", nil); err != nil {
		t.Fatal(err)
	}
	c, _ = st.CariKredensial(ctx, pRot2)
	if c.DicabutAt == nil {
		t.Error("kunci lama tidak boleh hidup kembali setelah klien diaktifkan lagi")
	}
	if k, _ := st.AmbilKlien(ctx, id); k.KunciAktif != 0 {
		t.Errorf("kunci aktif setelah diaktifkan kembali = %d, want 0 (kunci baru harus diterbitkan)", k.KunciAktif)
	}

	// Ditolak bersifat final, dan tidak bisa diubah scope-nya.
	tolak, _ := klienUji(t, st, "UJI Ditolak")
	if err := st.UbahStatus(ctx, tolak, []string{StatusMenunggu}, StatusDitolak, "superadmin-uji", "tidak memenuhi syarat", nil); err != nil {
		t.Fatal(err)
	}
	if err := st.UbahStatus(ctx, tolak, []string{StatusDicabut, StatusMenunggu}, StatusDisetujui, "x", "", nil); !errors.Is(err, ErrStatusTidakSesuai) {
		t.Errorf("ditolak -> disetujui: %v", err)
	}
	if err := st.UbahCakupan(ctx, tolak, UbahCakupan{DatasetDiberikan: []string{"aset.tanah"}}); !errors.Is(err, ErrStatusTidakSesuai) {
		t.Errorf("mengubah klien ditolak: %v", err)
	}
	if err := st.UbahCakupan(ctx, 999999999, UbahCakupan{}); !errors.Is(err, ErrTidakAda) {
		t.Errorf("klien tidak ada: %v", err)
	}
	if err := st.UbahCakupan(ctx, id, UbahCakupan{DatasetDiberikan: []string{"aset.satker", "aset.tanah"}, Override: Override{}, Catatan: "diubah"}); err != nil {
		t.Fatal(err)
	}
	k, _ = st.AmbilKlien(ctx, id)
	if len(k.DatasetDiberikan) != 2 || k.LimitPermintaan != nil || len(k.IPDiizinkan) != 0 || k.Catatan != "diubah" {
		t.Errorf("setelah UbahCakupan: %+v", k)
	}

	// Daftar dan hitungan.
	daftar, total, err := st.DaftarKlien(ctx, PenyaringKlien{Q: "UJI Ditolak"})
	if err != nil || total != 1 || len(daftar) != 1 || daftar[0].ID != tolak {
		t.Errorf("pencarian nama: %v %d %v", daftar, total, err)
	}
	if d, tot, _ := st.DaftarKlien(ctx, PenyaringKlien{Q: "UJI", Status: StatusDitolak}); tot < 1 || len(d) < 1 || d[0].Status != StatusDitolak {
		t.Errorf("saringan status: %v %d", d, tot)
	}
	if _, tot, err := st.DaftarKlien(ctx, PenyaringKlien{Q: "%' OR 1=1 --"}); err != nil || tot != 0 {
		t.Errorf("karakter khusus LIKE/SQL harus literal: %d %v", tot, err)
	}
	h, err := st.HitungStatus(ctx)
	if err != nil || h[StatusDitolak] < 1 || h[StatusDisetujui] < 1 {
		t.Errorf("HitungStatus = %v %v", h, err)
	}
	if _, err := st.AmbilKlien(ctx, 999999999); !errors.Is(err, ErrTidakAda) {
		t.Errorf("AmbilKlien tak ada: %v", err)
	}
	if dipakai, err := st.DaftarAktif(ctx); err != nil || len(dipakai) < 1 {
		t.Errorf("DaftarAktif: %v %v", dipakai, err)
	}
	if o, err := st.DaftarOpsiKlien(ctx); err != nil || len(o) < 1 {
		t.Errorf("DaftarOpsiKlien: %v %v", o, err)
	}
	if err := st.TandaiDipakai(ctx, []Pakai{{KunciID: k1, Waktu: time.Now(), IP: "198.51.100.5"}}); err != nil {
		t.Fatal(err)
	}
	ks, _ = st.DaftarKunci(ctx, id)
	var dipakaiOK bool
	for _, x := range ks {
		if x.ID == k1 && x.TerakhirDipakaiAt != nil && x.TerakhirIP == "198.51.100.5" {
			dipakaiOK = true
		}
	}
	if !dipakaiOK {
		t.Error("terakhir dipakai tidak tersimpan")
	}
}

func TestIntegrasiPengaturan(t *testing.T) {
	db := bukaDBUji(t)
	st := &Store{DB: db}
	ctx := context.Background()
	t.Cleanup(func() { st.SimpanPengaturan(ctx, PengaturanBawaan(), "pulih") })

	p, err := st.MuatPengaturan(ctx)
	if err != nil || p != PengaturanBawaan() {
		t.Fatalf("bawaan dari migrasi: %+v %v (harus 1000/60 dan 5000/3600)", p, err)
	}
	p.LimitPermintaan, p.KuotaPermintaan, p.PendaftaranDibuka, p.KunciBerlakuHari = 321, 4321, false, 45
	if err := st.SimpanPengaturan(ctx, p, "superadmin-uji"); err != nil {
		t.Fatal(err)
	}
	got, err := st.MuatPengaturan(ctx)
	if err != nil || got != p {
		t.Errorf("setelah disimpan: %+v %v", got, err)
	}
	var oleh string
	db.QueryRow(`SELECT diubah_oleh FROM api_akses_pengaturan WHERE kunci = N'limit_permintaan'`).Scan(&oleh)
	if oleh != "superadmin-uji" {
		t.Errorf("diubah_oleh = %q", oleh)
	}
	bad := p
	bad.LimitPermintaan = 0
	if err := st.SimpanPengaturan(ctx, bad, "x"); err == nil {
		t.Error("pengaturan tidak sah tidak boleh disimpan")
	}
	// Pengaturan hilang sebagian di tabel -> sisanya tetap terbaca (nilai pabrik untuk yang hilang).
	db.Exec(`DELETE FROM api_akses_pengaturan WHERE kunci = N'halaman_maks'`)
	if got, _ := st.MuatPengaturan(ctx); got.HalamanMaks != 500 || got.LimitPermintaan != 321 {
		t.Errorf("baris hilang: %+v", got)
	}
}

func TestIntegrasiKuotaLogDanAgregat(t *testing.T) {
	db := bukaDBUji(t)
	st := &Store{DB: db}
	ctx := context.Background()
	id, _ := klienUji(t, st, "UJI Pantau")
	id2, _ := klienUji(t, st, "UJI Pantau Dua")

	// Kuota: menyimpan dua kali memperbarui baris yang sama.
	mulai := time.Date(2001, 2, 3, 4, 0, 0, 0, time.UTC)
	if err := st.SimpanKuota(ctx, []KuotaSnapshot{{KlienID: id, Mulai: mulai, Jumlah: 10}}); err != nil {
		t.Fatal(err)
	}
	if err := st.SimpanKuota(ctx, []KuotaSnapshot{{KlienID: id, Mulai: mulai, Jumlah: 12}, {KlienID: id2, Mulai: mulai, Jumlah: 3}}); err != nil {
		t.Fatal(err)
	}
	ks, err := st.MuatKuota(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var j1, j2 int
	for _, k := range ks {
		if k.KlienID == id && k.Mulai.Equal(mulai) {
			j1 = k.Jumlah
		}
		if k.KlienID == id2 {
			j2 = k.Jumlah
		}
	}
	if j1 != 12 || j2 != 3 {
		t.Errorf("kuota tersimpan = %d dan %d, want 12 dan 3", j1, j2)
	}

	// Riwayat dan agregat dari pencatat sungguhan.
	pen := NewPencatat(st, 100)
	pen.Mulai()
	t0 := time.Date(2001, 2, 3, 4, 30, 0, 0, time.UTC)
	n := 25
	cat := []Penggunaan{
		{Waktu: t0, KlienID: id, KunciPrefix: "abcdefghij", RequestID: "req-1", Metode: "GET", Endpoint: "/aset/:dataset", Dataset: "aset.tanah", Status: 200, Hasil: HasilOK, DurasiMS: 20, Baris: &n, IP: "203.0.113.5", UserAgent: "uji/1"},
		{Waktu: t0.Add(time.Minute), KlienID: id, Metode: "GET", Endpoint: "/aset/:dataset", Dataset: "aset.tanah", Status: 200, Hasil: HasilOK, DurasiMS: 400, Baris: &n, IP: "203.0.113.5"},
		{Waktu: t0.Add(2 * time.Minute), KlienID: id, Metode: "GET", Endpoint: "/aset/:dataset", Status: 429, Hasil: HasilBatasLaju, DurasiMS: 1, IP: "203.0.113.5"},
		{Waktu: t0.Add(3 * time.Minute), KlienID: id, Metode: "GET", Endpoint: "/aset/:dataset/:id", Dataset: "aset.tanah", Status: 500, Hasil: HasilGalatServer, DurasiMS: 3000, IP: "203.0.113.6"},
		{Waktu: t0.Add(4 * time.Minute), KlienID: 0, Metode: "GET", Endpoint: "/aset/:dataset", Status: 401, Hasil: HasilTidakSah, DurasiMS: 2, IP: "198.51.100.1"},
		{Waktu: t0.Add(2 * time.Hour), KlienID: id2, Metode: "GET", Endpoint: "/katalog", Status: 200, Hasil: HasilOK, DurasiMS: 10, IP: "203.0.113.9"},
	}
	for _, u := range cat {
		pen.Catat(u)
	}
	pen.Berhenti(ctx)
	if s := pen.Statistik(); s.Gagal != 0 || s.Ditulis != 6 {
		t.Fatalf("statistik pencatat = %+v", s)
	}
	// Penulisan agregat kedua pada kunci yang sama menjumlahkan, bukan menimpa.
	pen2 := NewPencatat(st, 100)
	pen2.Mulai()
	pen2.Catat(Penggunaan{Waktu: t0, KlienID: id, Metode: "GET", Endpoint: "/aset/:dataset", Dataset: "aset.tanah", Status: 200, Hasil: HasilOK, DurasiMS: 4000, Baris: &n, IP: "203.0.113.5"})
	pen2.Berhenti(ctx)

	dari, sampai := time.Date(2001, 2, 3, 0, 0, 0, 0, time.UTC), time.Date(2001, 2, 4, 0, 0, 0, 0, time.UTC)
	m, aktif, err := st.Ringkas(ctx, PenyaringPantau{Dari: dari, Sampai: sampai, KlienID: id})
	if err != nil {
		t.Fatal(err)
	}
	if m.Jumlah != 5 || m.Sukses != 3 || m.BatasLaju != 1 || m.GalatServer != 1 || m.Baris != 75 || m.LatMaksMS != 4000 || aktif != 1 {
		t.Errorf("ringkasan klien 1 = %+v aktif=%d", m, aktif)
	}
	semua, aktif, _ := st.Ringkas(ctx, PenyaringPantau{Dari: dari, Sampai: sampai})
	if semua.Jumlah != 7 || semua.Ditolak != 1 || aktif != 2 {
		t.Errorf("ringkasan semua = %+v aktif=%d (klien tidak dikenal tidak dihitung sebagai klien aktif)", semua, aktif)
	}
	if semua.LatP95MS < 250 || semua.LatP95MS > 4000 {
		t.Errorf("p95 = %d", semua.LatP95MS)
	}
	if kosong, _, err := st.Ringkas(ctx, PenyaringPantau{Dari: dari.AddDate(0, 0, 10), Sampai: sampai.AddDate(0, 0, 10)}); err != nil || kosong.Jumlah != 0 {
		t.Errorf("rentang kosong: %+v %v", kosong, err)
	}

	jamJam, err := st.Deret(ctx, PenyaringPantau{Dari: dari.Add(4 * time.Hour), Sampai: dari.Add(8 * time.Hour)}, false)
	if err != nil || len(jamJam) != 4 || jamJam[0].Jumlah != 6 || jamJam[2].Jumlah != 1 || jamJam[1].Jumlah != 0 {
		t.Errorf("deret per jam: %+v %v", jamJam, err)
	}
	if jamJam[0].Gagal != 3 || jamJam[0].Dibatasi != 1 {
		t.Errorf("gagal/dibatasi: %+v", jamJam[0])
	}
	hari, err := st.Deret(ctx, PenyaringPantau{Dari: dari, Sampai: sampai.AddDate(0, 0, 2)}, true)
	var jumlahHari int64
	for _, h := range hari {
		jumlahHari += h.Jumlah
	}
	if err != nil || jumlahHari != 7 {
		t.Errorf("deret per hari: %+v %v", hari, err)
	}

	pk, err := st.PerKlien(ctx, PenyaringPantau{Dari: dari, Sampai: sampai})
	if err != nil || len(pk) != 3 {
		t.Fatalf("per klien: %+v %v", pk, err)
	}
	if pk[0].KlienID != id || pk[0].Jumlah != 5 || pk[0].Nama != "UJI Pantau" || pk[0].Kode == "" {
		t.Errorf("per klien[0]: %+v", pk[0])
	}
	pe, err := st.PerEndpoint(ctx, PenyaringPantau{Dari: dari, Sampai: sampai})
	if err != nil || len(pe) != 3 || pe[0].Kunci != "/aset/:dataset" || pe[0].Jumlah != 5 {
		t.Errorf("per endpoint: %+v %v", pe, err)
	}
	if pf, _ := st.PerEndpoint(ctx, PenyaringPantau{Dari: dari, Sampai: sampai, Endpoint: "/katalog"}); len(pf) != 1 || pf[0].Jumlah != 1 {
		t.Errorf("filter endpoint: %+v", pf)
	}

	// Riwayat dan penyaringnya.
	r, total, err := st.Riwayat(ctx, PenyaringRiwayat{Dari: dari, Sampai: sampai, KlienID: id})
	if err != nil || total != 5 || len(r) != 5 {
		t.Fatalf("riwayat klien: %d %v", total, err)
	}
	if r[0].Waktu.Before(r[len(r)-1].Waktu) {
		t.Error("riwayat harus terbaru dulu")
	}
	for _, u := range r {
		if u.KlienNama != "UJI Pantau" || u.KlienKode == "" {
			t.Errorf("nama klien tidak ikut: %+v", u)
		}
	}
	if _, total, _ := st.Riwayat(ctx, PenyaringRiwayat{Dari: dari, Sampai: sampai, Hasil: HasilBatasLaju}); total != 1 {
		t.Errorf("filter hasil: %d", total)
	}
	if rr, total, _ := st.Riwayat(ctx, PenyaringRiwayat{Dari: dari, Sampai: sampai, KlienID: id, PerHalaman: 2, Halaman: 3}); total != 5 || len(rr) != 1 {
		t.Errorf("halaman 3 dari 5 baris per 2: %d baris dari total %d", len(rr), total)
	}
	if _, total, _ := st.Riwayat(ctx, PenyaringRiwayat{Dari: dari, Sampai: sampai, Status: 500}); total != 1 {
		t.Errorf("filter status: %d", total)
	}
	if _, total, _ := st.Riwayat(ctx, PenyaringRiwayat{Dari: dari, Sampai: sampai, IP: "198.51"}); total != 1 {
		t.Errorf("filter ip: %d", total)
	}
	if _, total, _ := st.Riwayat(ctx, PenyaringRiwayat{Dari: dari, Sampai: sampai, Q: "req-1"}); total != 1 {
		t.Errorf("cari request id: %d", total)
	}
	if _, total, err := st.Riwayat(ctx, PenyaringRiwayat{Dari: dari, Sampai: sampai, Q: "'; DROP TABLE api_akses_log; --"}); err != nil || total != 0 {
		t.Errorf("masukan jahat harus literal: %d %v", total, err)
	}
	if _, total, _ := st.Riwayat(ctx, PenyaringRiwayat{Dari: dari, Sampai: sampai, Dataset: "aset.tanah"}); total != 4 {
		t.Errorf("filter dataset: %d", total)
	}

	// Pembersihan retensi.
	batas := time.Date(2001, 2, 3, 4, 45, 0, 0, time.UTC)
	n1, err := st.HapusLogSebelum(ctx, batas)
	if err != nil || n1 < 5 {
		t.Errorf("HapusLogSebelum = %d %v", n1, err)
	}
	if _, total, _ := st.Riwayat(ctx, PenyaringRiwayat{Dari: dari, Sampai: sampai}); total != 1 {
		t.Errorf("sisa setelah pembersihan = %d, want 1 (yang jam 06:30)", total)
	}
	if _, err := st.HapusAgregatSebelum(ctx, sampai); err != nil {
		t.Fatal(err)
	}
	if m, _, _ := st.Ringkas(ctx, PenyaringPantau{Dari: dari, Sampai: sampai}); m.Jumlah != 0 {
		t.Errorf("agregat setelah pembersihan: %+v", m)
	}
}

// ---------------------------------------------------------------- pembacaan data aset

const (
	satkerUjiA = "015040199119091000KP" // UE1 01504, satker 119091
	satkerUjiB = "015150199691273000KP" // UE1 01515, satker 691273
)

func bersihkanDataUji(db *sql.DB) {
	for _, tb := range []string{"DIGITALISASI_TANAH", "DIGITALISASI_RUMAH_NEGARA", "DIGITALISASI_GEDUNG_LAINNYA"} {
		db.Exec(`DELETE FROM ` + tb + ` WHERE Nama_Satker LIKE N'UJIAPI%'`)
	}
	db.Exec(`DELETE FROM digitalisasi_sync_log WHERE dibuat_oleh = N'ujiapi'`)
}

func siapkanDataUji(t *testing.T, db *sql.DB) {
	t.Helper()
	bersihkanDataUji(db)
	t.Cleanup(func() { bersihkanDataUji(db) })
	ins := func(q string, args ...interface{}) {
		if _, err := db.Exec(q, args...); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	for i := 1; i <= 5; i++ {
		kode, prov := satkerUjiA, "JAWA BARAT"
		if i > 3 {
			kode, prov = satkerUjiB, "BALI"
		}
		ins(`INSERT INTO DIGITALISASI_TANAH (Kode_UE1, Kode_Satker, Nama_Satker, Uraian_tanah, Alamat_tanah, KabKota_tanah, Provinsi_tanah, Luas_Tanah, Latitude, Longitude, Foto, Nilai_Tanah)
			VALUES (N'x', @p1, @p2, @p3, N'Jl. Uji 100%', N'KOTA UJI', @p4, 1234.5000, -6.2000000, 106.8000000, N'https://internal.example/foto-rahasia.jpg', 5000000.25)`,
			kode, fmt.Sprintf("UJIAPI satker %d", i), fmt.Sprintf("Tanah uji %d", i), prov)
	}
	ins(`INSERT INTO DIGITALISASI_RUMAH_NEGARA (Kode_UE1, Kode_Satker, Nama_Satker, Uraian_RN, Status_Penghuni, Nama_Penghuni, Foto_RN, Kondisi_RN)
		VALUES (N'x', @p1, N'UJIAPI rumah', N'Rumah dinas uji', N'Dihuni', N'PENGHUNI RAHASIA UJI', N'foto-rn-rahasia.jpg', N'Baik')`, satkerUjiA)
	ins(`INSERT INTO DIGITALISASI_GEDUNG_LAINNYA (Kode_UE1, Kode_Satker, Nama_Satker, Uraian_bangunan, Nama_Pengguna, Foto_bangunan)
		VALUES (N'x', @p1, N'UJIAPI gedung', N'Gedung uji', N'PENGGUNA GEDUNG RAHASIA', N'foto-gedung-rahasia.jpg')`, satkerUjiA)
}

func TestIntegrasiBacaDataAset(t *testing.T) {
	db := bukaDBUji(t)
	siapkanDataUji(t, db)
	ctx := context.Background()
	tanah, _ := CariDataset("aset.tanah")

	// Halaman pertama: kolom, tipe nilai, dan kolom yang tidak boleh keluar.
	h, err := tanah.BacaDaftar(ctx, db, FilterData{Q: "Tanah uji", PerHalaman: 2, Hitung: true}, 500)
	if err != nil {
		t.Fatal(err)
	}
	if len(h.Baris) != 2 || !h.AdaBerikutnya || h.KursorBerikutnya == 0 || h.Total == nil || *h.Total != 5 {
		t.Fatalf("halaman 1: %d baris, ada=%v kursor=%d total=%v", len(h.Baris), h.AdaBerikutnya, h.KursorBerikutnya, h.Total)
	}
	b := h.Baris[0]
	if b["uraian_tanah"] != "Tanah uji 1" || b["luas_tanah"] != 1234.5 || b["latitude"] != -6.2 || b["nilai_tanah"] != 5000000.25 {
		t.Errorf("baris: %+v", b)
	}
	if _, ada := b["foto"]; ada {
		t.Error("kolom Foto tidak boleh keluar")
	}
	if _, ada := b["disinkronkan_pada"]; !ada {
		t.Error("waktu sinkronisasi harus ikut")
	}
	if b["id"] == nil || b["id"].(int64) <= 0 {
		t.Errorf("id = %v", b["id"])
	}
	raw, _ := json.Marshal(h.Baris)
	if strings.Contains(string(raw), "foto-rahasia") || strings.Contains(string(raw), "internal.example") {
		t.Errorf("tautan foto bocor: %s", raw)
	}

	// Paginasi kursor melewati semua baris tanpa duplikat atau lompatan.
	var ids []int64
	f := FilterData{Q: "Tanah uji", PerHalaman: 2}
	for hal := 0; hal < 10; hal++ {
		r, err := tanah.BacaDaftar(ctx, db, f, 500)
		if err != nil {
			t.Fatal(err)
		}
		for _, x := range r.Baris {
			ids = append(ids, x["id"].(int64))
		}
		if !r.AdaBerikutnya {
			if r.KursorBerikutnya != 0 {
				t.Error("halaman terakhir tidak punya kursor")
			}
			break
		}
		f.Kursor = r.KursorBerikutnya
	}
	if len(ids) != 5 {
		t.Fatalf("seluruh halaman = %d baris, want 5", len(ids))
	}
	for i := 1; i < len(ids); i++ {
		if ids[i] <= ids[i-1] {
			t.Errorf("urutan id tidak naik: %v", ids)
		}
	}
	// per_halaman dibatasi pengaturan.
	if r, _ := tanah.BacaDaftar(ctx, db, FilterData{Q: "Tanah uji", PerHalaman: 1000}, 3); len(r.Baris) != 3 || !r.AdaBerikutnya {
		t.Errorf("halaman maks 3: %d baris", len(r.Baris))
	}

	// Pembatasan UE1 klien.
	r, _ := tanah.BacaDaftar(ctx, db, FilterData{Q: "Tanah uji", UE1Klien: []string{"01504"}, Hitung: true}, 500)
	if len(r.Baris) != 3 || *r.Total != 3 {
		t.Errorf("UE1 01504: %d baris", len(r.Baris))
	}
	r, _ = tanah.BacaDaftar(ctx, db, FilterData{Q: "Tanah uji", UE1Klien: []string{"01504", "01515"}}, 500)
	if len(r.Baris) != 5 {
		t.Errorf("dua UE1: %d", len(r.Baris))
	}
	r, _ = tanah.BacaDaftar(ctx, db, FilterData{Q: "Tanah uji", UE1Klien: []string{"99999"}}, 500)
	if len(r.Baris) != 0 {
		t.Errorf("UE1 lain: %d", len(r.Baris))
	}
	r, _ = tanah.BacaDaftar(ctx, db, FilterData{Q: "Tanah uji", UE1Klien: []string{"bukan-ue1"}}, 500)
	if len(r.Baris) != 0 {
		t.Errorf("UE1 rusak harus menutup semua baris: %d", len(r.Baris))
	}
	// kode_ue1 dari klien tidak bisa melebarkan pembatasan miliknya.
	r, _ = tanah.BacaDaftar(ctx, db, FilterData{Q: "Tanah uji", UE1Klien: []string{"01504"}, KodeUE1: "01515"}, 500)
	if len(r.Baris) != 0 {
		t.Errorf("kode_ue1 di luar pembatasan klien: %d", len(r.Baris))
	}

	// Penyaring.
	cek := func(nama string, f FilterData, want int) {
		t.Helper()
		f.Q = firstNonEmpty(f.Q, "Tanah uji")
		r, err := tanah.BacaDaftar(ctx, db, f, 500)
		if err != nil {
			t.Fatalf("%s: %v", nama, err)
		}
		if len(r.Baris) != want {
			t.Errorf("%s: %d baris, want %d", nama, len(r.Baris), want)
		}
	}
	cek("satker 6 digit", FilterData{KodeSatker: "691273"}, 2)
	cek("awalan kode satker", FilterData{KodeSatker: "01504"}, 3)
	cek("kode_ue1", FilterData{KodeUE1: "01515"}, 2)
	cek("provinsi", FilterData{Provinsi: "bali"}, 2)
	cek("kab_kota", FilterData{KabKota: "KOTA UJI"}, 5)
	cek("provinsi tak ada", FilterData{Provinsi: "ATLANTIS"}, 0)
	cek("persen bukan wildcard", FilterData{Q: "Tanah%uji"}, 0)
	cek("garis bawah bukan wildcard", FilterData{Q: "Tanah_uji"}, 0)
	cek("pencarian persen literal", FilterData{Q: "100%"}, 5)
	cek("pencarian alamat", FilterData{Q: "Jl. Uji 100%"}, 5)
	// Masukan jahat menjadi nilai literal, tidak pernah SQL.
	for _, jahat := range []string{"'; DROP TABLE DIGITALISASI_TANAH; --", "' OR '1'='1", "x' UNION SELECT name FROM sys.tables --"} {
		cek("Q jahat "+jahat, FilterData{Q: jahat}, 0)
		cek("provinsi jahat", FilterData{Provinsi: jahat}, 0)
		if _, err := tanah.BacaDaftar(ctx, db, FilterData{KodeSatker: jahat}, 500); err == nil {
			t.Errorf("kode_satker jahat harus ditolak validasi: %q", jahat)
		}
	}
	var ada int
	if err := db.QueryRow(`SELECT COUNT(*) FROM DIGITALISASI_TANAH WHERE Nama_Satker LIKE N'UJIAPI%'`).Scan(&ada); err != nil || ada != 5 {
		t.Fatalf("tabel masih utuh? %d %v", ada, err)
	}

	// Kolom pribadi pada dataset lain tidak pernah keluar, dan pencarian tidak menjadi jalan mengintipnya.
	rn, _ := CariDataset("aset.rumah_negara")
	rr, err := rn.BacaDaftar(ctx, db, FilterData{Q: "Rumah dinas uji"}, 500)
	if err != nil || len(rr.Baris) != 1 {
		t.Fatalf("rumah negara: %v %d", err, len(rr.Baris))
	}
	raw, _ = json.Marshal(rr.Baris)
	for _, s := range []string{"PENGHUNI RAHASIA", "nama_penghuni", "foto-rn", "foto_rn"} {
		if strings.Contains(strings.ToLower(string(raw)), strings.ToLower(s)) {
			t.Errorf("rumah negara membocorkan %q: %s", s, raw)
		}
	}
	if rr.Baris[0]["status_penghuni"] != "Dihuni" {
		t.Errorf("kolom biasa harus tetap ada: %v", rr.Baris[0])
	}
	if x, _ := rn.BacaDaftar(ctx, db, FilterData{Q: "PENGHUNI RAHASIA"}, 500); len(x.Baris) != 0 {
		t.Error("pencarian tidak boleh mencocokkan kolom yang tidak keluar (nama penghuni)")
	}
	gl, _ := CariDataset("aset.gedung_lainnya")
	gr, _ := gl.BacaDaftar(ctx, db, FilterData{Q: "Gedung uji"}, 500)
	raw, _ = json.Marshal(gr.Baris)
	if len(gr.Baris) != 1 || strings.Contains(string(raw), "PENGGUNA GEDUNG") || strings.Contains(string(raw), "foto-gedung") {
		t.Errorf("gedung lainnya: %s", raw)
	}

	// Detail.
	id := ids[0]
	d, err := tanah.BacaDetail(ctx, db, id, nil)
	if err != nil || d["uraian_tanah"] != "Tanah uji 1" {
		t.Fatalf("detail: %v %v", d, err)
	}
	if _, err := tanah.BacaDetail(ctx, db, id, []string{"01515"}); !errors.Is(err, ErrTidakAda) {
		t.Errorf("detail di luar UE1 klien harus dianggap tidak ada: %v", err)
	}
	if _, err := tanah.BacaDetail(ctx, db, id, []string{"01504"}); err != nil {
		t.Errorf("detail dalam UE1: %v", err)
	}
	if _, err := tanah.BacaDetail(ctx, db, 999999999, nil); !errors.Is(err, ErrTidakAda) {
		t.Errorf("detail tak ada: %v", err)
	}

	// Kesegaran data.
	if t0, err := tanah.TerakhirSinkron(ctx, db); err != nil {
		t.Errorf("TerakhirSinkron: %v", err)
	} else if t0 != nil {
		t.Logf("sinkronisasi terakhir: %v", t0)
	}
	db.Exec(`INSERT INTO digitalisasi_sync_log (dataset, status, selesai, dijalankan_oleh) VALUES (N'tanah', N'sukses', SYSUTCDATETIME(), N'ujiapi')`)
	if t0, err := tanah.TerakhirSinkron(ctx, db); err != nil || t0 == nil || time.Since(*t0) > time.Minute {
		t.Errorf("TerakhirSinkron setelah sinkron: %v %v", t0, err)
	}
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
