package sapa

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"pasti-v3-backend/internal/fakesql"
)

func TestDefaultRefBMNMasukAkal(t *testing.T) {
	r := DefaultRefBMN()
	satuan := map[string]bool{}
	for _, s := range r.Satuan {
		satuan[s.Nama] = true
	}
	for _, j := range r.Jenis {
		if len(j.Satuan) == 0 || !j.Mengizinkan(j.SatuanBawaan) {
			t.Errorf("%s: satuan bawaan %q harus termasuk satuan yang diizinkan %v", j.Nama, j.SatuanBawaan, j.Satuan)
		}
		for _, s := range j.Satuan {
			if !satuan[s] {
				t.Errorf("%s: satuan %q tidak ada di daftar satuan", j.Nama, s)
			}
		}
	}
	// Pasangan yang tidak masuk akal tidak boleh ada di daftar awal.
	tanah, _ := r.CariJenis("Tanah")
	if tanah.Mengizinkan("unit") || tanah.Mengizinkan("meter") {
		t.Errorf("Tanah tidak boleh bersatuan unit/meter: %v", tanah.Satuan)
	}
	pm, _ := r.CariJenis("Peralatan dan Mesin")
	if pm.Mengizinkan("meter") || pm.Mengizinkan("bidang") {
		t.Errorf("Peralatan dan Mesin tidak boleh bersatuan meter/bidang: %v", pm.Satuan)
	}
}

// Migrasi 022 menyemai daftar yang sama dengan DefaultRefBMN (yang dipakai tes dan server uji): satu sumber yang dijaga tes.
func TestMigrasi022SamaDenganDefault(t *testing.T) {
	b, err := os.ReadFile("../migrations/022_create_sapa_bmn.sql")
	if err != nil {
		t.Fatal(err)
	}
	sqlText := string(b)
	r := DefaultRefBMN()
	for _, s := range r.Satuan {
		re := regexp.MustCompile(`INSERT INTO sapa_satuan \(nama, urutan\) VALUES \('` + regexp.QuoteMeta(s.Nama) + `', ` + strconv.Itoa(s.Urutan) + `\)`)
		if !re.MatchString(sqlText) {
			t.Errorf("satuan %q (urutan %d) tidak disemai migrasi", s.Nama, s.Urutan)
		}
	}
	for _, j := range r.Jenis {
		re := regexp.MustCompile(`INSERT INTO sapa_jenis_bmn \(nama, urutan, satuan_bawaan\) VALUES \('` + regexp.QuoteMeta(j.Nama) + `', ` + strconv.Itoa(j.Urutan) + `, '` + regexp.QuoteMeta(j.SatuanBawaan) + `'\)`)
		if !re.MatchString(sqlText) {
			t.Errorf("jenis %q (urutan %d, bawaan %q) tidak disemai migrasi", j.Nama, j.Urutan, j.SatuanBawaan)
		}
		for i, s := range j.Satuan {
			if !strings.Contains(sqlText, "('"+j.Nama+"', '"+s+"', "+strconv.Itoa(i+1)+")") {
				t.Errorf("pemetaan %q -> %q (urutan %d) tidak disemai migrasi", j.Nama, s, i+1)
			}
		}
	}
	// Tidak ada pemetaan di migrasi yang tidak ada di default.
	n := 0
	for _, j := range r.Jenis {
		n += len(j.Satuan)
	}
	if got := strings.Count(sqlText, "INSERT INTO sapa_jenis_bmn_satuan (jenis, satuan, urutan) VALUES"); got != len(r.Jenis) {
		t.Errorf("INSERT pemetaan = %d, want %d (satu per jenis)", got, len(r.Jenis))
	}
	if got := len(regexp.MustCompile(`\('[^']+', '[^']+', \d+\)`).FindAllString(sqlText, -1)); got != n {
		t.Errorf("baris pemetaan di migrasi = %d, want %d", got, n)
	}
}

// lewatiTimBA melewati tahap SK Tim dan Berita Acara supaya ND Satker bisa dikerjakan.
func (e *env) lewatiTimBA(k *KasusInfo) {
	e.t.Helper()
	for _, tk := range []string{TahapTim, TahapBA} {
		if err := e.l.Lewati(e.ctx, e.satkerA, k.ID, tk, "dibuat di luar aplikasi"); err != nil {
			e.t.Fatalf("lewati %s: %v", tk, err)
		}
	}
}

func TestValidasiBMNKesesuaianJenisDanSatuan(t *testing.T) {
	ref := DefaultRefBMN()
	cases := []struct {
		jenis, satuan string
		galat         string // potongan pesan; kosong = lolos
		jk, sk        string // nama kanonik yang diharapkan
	}{
		{"Tanah", "bidang", "", "Tanah", "bidang"},
		{"tanah", "BIDANG", "", "Tanah", "bidang"}, // huruf besar/kecil disamakan dengan daftar
		{"  Peralatan dan Mesin ", " Unit ", "", "Peralatan dan Mesin", "unit"},
		{"Tanah", "unit", "tidak sesuai untuk jenis BMN \"Tanah\"", "", ""},
		{"Peralatan dan Mesin", "meter", "tidak sesuai", "", ""},
		{"Peralatan dan Mesin", "bidang", "tidak sesuai", "", ""},
		{"Kendaraan Bermotor", "paket", "tidak sesuai", "", ""},
		{"Mobil mewah", "unit", "tidak ada di daftar", "", ""},
		{"", "unit", "", "", "unit"}, // jenis kosong: kewajibannya diperiksa di tempat lain
		{"Tanah", "", "", "Tanah", ""},
	}
	for _, c := range cases {
		jk, sk, g := ValidasiBMN(ref, c.jenis, c.satuan)
		if c.galat == "" {
			if len(g) != 0 || (c.jk != "" && jk != c.jk) || (c.sk != "" && sk != c.sk) {
				t.Errorf("%q/%q: jk=%q sk=%q galat=%v", c.jenis, c.satuan, jk, sk, g)
			}
			continue
		}
		if len(g) != 1 || !strings.Contains(g[0], c.galat) {
			t.Errorf("%q/%q: galat = %v, want memuat %q", c.jenis, c.satuan, g, c.galat)
		}
	}
	// Pesan menyebut satuan yang boleh.
	_, _, g := ValidasiBMN(ref, "Tanah", "unit")
	if !strings.Contains(g[0], "bidang") {
		t.Errorf("pesan harus menyebut satuan yang boleh: %v", g)
	}
}

func TestRefBMNAktifMenyaringYangNonaktif(t *testing.T) {
	ref := DefaultRefBMN()
	for i := range ref.Satuan {
		if ref.Satuan[i].Nama == "bidang" {
			ref.Satuan[i].Aktif = false // Tanah kehilangan satu-satunya satuan aktif -> tidak bisa dipakai
		}
	}
	for i := range ref.Jenis {
		if ref.Jenis[i].Nama == "Kendaraan Bermotor" {
			ref.Jenis[i].Aktif = false
		}
		if ref.Jenis[i].Nama == "Tanah dan Bangunan" {
			ref.Jenis[i].SatuanBawaan = "bidang" // bawaan nonaktif -> dialihkan ke satuan aktif pertama
		}
	}
	a := ref.Aktif()
	if _, ok := a.CariJenis("Tanah"); ok {
		t.Error("jenis tanpa satuan aktif tidak boleh ditawarkan")
	}
	if _, ok := a.CariJenis("Kendaraan Bermotor"); ok {
		t.Error("jenis nonaktif tidak boleh ditawarkan")
	}
	if _, ok := a.CariSatuan("bidang"); ok {
		t.Error("satuan nonaktif tidak boleh ditawarkan")
	}
	tb, _ := a.CariJenis("Tanah dan Bangunan")
	if tb.Mengizinkan("bidang") || tb.SatuanBawaan != "unit" {
		t.Errorf("Tanah dan Bangunan = %+v", tb)
	}
	// Validasi menjelaskan jenis yang nonaktif berbeda dari yang tidak dikenal.
	_, _, g := ValidasiBMN(ref, "Kendaraan Bermotor", "unit")
	if len(g) != 1 || !strings.Contains(g[0], "sudah tidak tersedia") {
		t.Errorf("jenis nonaktif: %v", g)
	}
}

func TestNDSatkerMenolakPasanganTidakMasukAkal(t *testing.T) {
	e := baru(t)
	k := e.buat(e.satkerA)
	e.lewatiTimBA(k)

	buat := func(jenis, satuan string) (*HasilDokumen, error) {
		d := ContohNDSatker()
		d.JenisBMN, d.Satuan = jenis, satuan
		return e.l.Hasilkan(e.ctx, e.satkerA, k.ID, TahapNDSatker, jsonDari(e.t, d))
	}
	var ev *ErrValidasi
	for _, c := range [][2]string{{"Tanah", "unit"}, {"Peralatan dan Mesin", "meter"}, {"Kendaraan Bermotor", "bidang"}, {"Pesawat", "unit"}} {
		_, err := buat(c[0], c[1])
		adalah(t, err, &ev)
		if ev == nil || len(ev.Rincian) != 1 {
			t.Errorf("%v: rincian = %v", c, ev)
		}
	}
	// Satuan kosong wajib diisi.
	_, err := buat("Tanah", "")
	adalah(t, err, &ev)
	if ev == nil || !strings.Contains(strings.Join(ev.Rincian, ";"), "Satuan jumlah BMN wajib diisi") {
		t.Errorf("satuan kosong: %v", ev)
	}
	// Galat BMN dan galat isian lain dilaporkan bersama (satu putaran perbaikan).
	d := ContohNDSatker()
	d.JenisBMN, d.Satuan, d.Alasan = "Tanah", "unit", ""
	_, err = e.l.Hasilkan(e.ctx, e.satkerA, k.ID, TahapNDSatker, jsonDari(t, d))
	adalah(t, err, &ev)
	if ev == nil || len(ev.Rincian) != 2 {
		t.Errorf("galat gabungan: %v", ev)
	}

	// Pasangan yang sah lolos dan penulisannya disamakan dengan daftar.
	h, err := buat("tanah", "BIDANG")
	if err != nil || h == nil {
		t.Fatalf("pasangan sah: %v", err)
	}
	var tersimpan DataNDSatker
	if err := json.Unmarshal(e.tahap(e.satkerA, k.ID, TahapNDSatker).Data, &tersimpan); err != nil || tersimpan.JenisBMN != "Tanah" || tersimpan.Satuan != "bidang" {
		t.Errorf("data tersimpan = %q/%q (%v)", tersimpan.JenisBMN, tersimpan.Satuan, err)
	}
}

func TestAdminMengaturJenisDanSatuanBMN(t *testing.T) {
	e := baru(t)
	var ev *ErrValidasi
	var kf *ErrKonflik

	// Hanya admin.
	if err := e.l.SimpanSatuanBMN(e.ctx, e.satkerA, SatuanBMN{Nama: "meter", Aktif: true}); !errors.Is(err, ErrTidakBerhak) {
		t.Errorf("non-admin simpan satuan: %v", err)
	}
	if _, err := e.l.RefBMNSemua(e.ctx, e.satkerA); !errors.Is(err, ErrTidakBerhak) {
		t.Errorf("non-admin daftar lengkap: %v", err)
	}
	// Pengguna SAPA melihat daftar aktif; tanpa peran tidak.
	if r, err := e.l.RefBMNAktif(e.ctx, e.satkerA); err != nil || len(r.Jenis) != 7 {
		t.Errorf("daftar aktif: %v %d", err, len(r.Jenis))
	}
	if _, err := e.l.RefBMNAktif(e.ctx, e.tanpa); !errors.Is(err, ErrTanpaPeran) {
		t.Errorf("tanpa peran: %v", err)
	}

	// Validasi masukan.
	adalah(t, e.l.SimpanSatuanBMN(e.ctx, e.admin, SatuanBMN{Nama: "  "}), &ev)
	adalah(t, e.l.SimpanSatuanBMN(e.ctx, e.admin, SatuanBMN{Nama: "a/b"}), &ev)
	adalah(t, e.l.SimpanSatuanBMN(e.ctx, e.admin, SatuanBMN{Nama: strings.Repeat("x", MaksSatuanBMN+1)}), &ev)
	adalah(t, e.l.SimpanSatuanBMN(e.ctx, e.admin, SatuanBMN{Nama: "meter", Urutan: -1}), &ev)
	adalah(t, e.l.SimpanJenisBMN(e.ctx, e.admin, JenisBMN{Nama: "Pagar", Aktif: true}), &ev)                                                  // tanpa satuan
	adalah(t, e.l.SimpanJenisBMN(e.ctx, e.admin, JenisBMN{Nama: "Pagar", Aktif: true, Satuan: []string{"meter"}}), &ev)                       // satuan belum ada
	adalah(t, e.l.SimpanJenisBMN(e.ctx, e.admin, JenisBMN{Nama: "Pagar", Aktif: true, Satuan: []string{"unit"}, SatuanBawaan: "paket"}), &ev) // bawaan di luar pilihan
	adalah(t, e.l.SimpanJenisBMN(e.ctx, e.admin, JenisBMN{Nama: "", Satuan: []string{"unit"}}), &ev)

	// Menambah satuan dan jenis baru: Jalan memakai meter, tetapi Peralatan dan Mesin tetap tidak boleh.
	if err := e.l.SimpanSatuanBMN(e.ctx, e.admin, SatuanBMN{Nama: " meter ", Aktif: true, Urutan: 7}); err != nil {
		t.Fatal(err)
	}
	if err := e.l.SimpanJenisBMN(e.ctx, e.admin, JenisBMN{Nama: "Pagar dan Jalan Lingkungan", Aktif: true, Urutan: 8, Satuan: []string{"METER", "meter", "unit"}}); err != nil {
		t.Fatal(err)
	}
	ref, _ := e.l.RefBMNSemua(e.ctx, e.admin)
	pj, ok := ref.CariJenis("Pagar dan Jalan Lingkungan")
	if !ok || len(pj.Satuan) != 2 || pj.Satuan[0] != "meter" || pj.SatuanBawaan != "meter" {
		t.Errorf("jenis baru = %+v", pj)
	}
	if _, _, g := ValidasiBMN(ref, "Pagar dan Jalan Lingkungan", "meter"); len(g) != 0 {
		t.Errorf("meter untuk jenis baru: %v", g)
	}
	if _, _, g := ValidasiBMN(ref, "Peralatan dan Mesin", "meter"); len(g) != 1 {
		t.Errorf("meter untuk Peralatan dan Mesin tetap ditolak: %v", g)
	}

	// Mengubah jenis yang ada mempertahankan penulisan namanya.
	if err := e.l.SimpanJenisBMN(e.ctx, e.admin, JenisBMN{Nama: "tanah", Aktif: true, Urutan: 1, Satuan: []string{"bidang", "unit"}}); err != nil {
		t.Fatal(err)
	}
	ref, _ = e.l.RefBMNSemua(e.ctx, e.admin)
	if tj, _ := ref.CariJenis("Tanah"); tj.Nama != "Tanah" || !tj.Mengizinkan("unit") {
		t.Errorf("Tanah = %+v", tj)
	}
	if n := len(ref.Jenis); n != 8 {
		t.Errorf("jenis = %d (tidak boleh menggandakan Tanah)", n)
	}

	// Hapus satuan yang menjadi satu-satunya satuan suatu jenis ditolak dan menyebut jenisnya.
	err := e.l.HapusSatuanBMN(e.ctx, e.admin, "eksemplar") // bukan satu-satunya di mana pun: boleh
	if err != nil {
		t.Errorf("hapus satuan yang bukan tunggal: %v", err)
	}
	ref, _ = e.l.RefBMNSemua(e.ctx, e.admin)
	if atl, _ := ref.CariJenis("Aset Tetap Lainnya"); atl.Mengizinkan("eksemplar") || len(atl.Satuan) != 3 {
		t.Errorf("pemetaan harus ikut terhapus: %+v", atl)
	}
	// Kendaraan Bermotor hanya punya "unit": menghapus unit ditolak.
	adalah(t, e.l.HapusSatuanBMN(e.ctx, e.admin, "unit"), &kf)
	if kf == nil || !strings.Contains(kf.Pesan, "Kendaraan Bermotor") {
		t.Errorf("pesan konflik: %v", kf)
	}
	// Satuan bawaan dialihkan saat satuan bawaan dihapus (jenis masih punya satuan lain).
	if err := e.l.SimpanJenisBMN(e.ctx, e.admin, JenisBMN{Nama: "Pagar dan Jalan Lingkungan", Aktif: true, Urutan: 8, Satuan: []string{"meter", "paket"}, SatuanBawaan: "paket"}); err != nil {
		t.Fatal(err)
	}
	if err := e.l.HapusSatuanBMN(e.ctx, e.admin, "paket"); err != nil {
		t.Fatalf("hapus paket: %v", err)
	}
	ref, _ = e.l.RefBMNSemua(e.ctx, e.admin)
	if pj, _ := ref.CariJenis("Pagar dan Jalan Lingkungan"); pj.SatuanBawaan != "meter" || len(pj.Satuan) != 1 {
		t.Errorf("bawaan dialihkan: %+v", pj)
	}
	if err := e.l.HapusSatuanBMN(e.ctx, e.admin, "tidak ada"); !errors.Is(err, ErrTidakDitemukan) {
		t.Errorf("satuan tak ada: %v", err)
	}

	// Hapus jenis.
	if err := e.l.HapusJenisBMN(e.ctx, e.admin, "pagar dan jalan lingkungan"); err != nil {
		t.Fatal(err)
	}
	if err := e.l.HapusJenisBMN(e.ctx, e.admin, "pagar dan jalan lingkungan"); !errors.Is(err, ErrTidakDitemukan) {
		t.Errorf("hapus dua kali: %v", err)
	}
	if err := e.l.HapusJenisBMN(e.ctx, e.satkerA, "Tanah"); !errors.Is(err, ErrTidakBerhak) {
		t.Errorf("non-admin hapus jenis: %v", err)
	}
}

func TestDaftarBMNKosongMenolakNDSatker(t *testing.T) {
	e := baru(t)
	k := e.buat(e.satkerA)
	e.lewatiTimBA(k)
	ref, _ := e.repo.AmbilRefBMN(e.ctx)
	for _, j := range ref.Jenis {
		_ = e.repo.HapusJenisBMN(e.ctx, j.Nama)
	}
	var ev *ErrValidasi
	_, err := e.l.Hasilkan(e.ctx, e.satkerA, k.ID, TahapNDSatker, jsonDari(t, ContohNDSatker()))
	adalah(t, err, &ev)
	if ev == nil || !strings.Contains(ev.Rincian[0], "belum diatur") {
		t.Errorf("daftar kosong: %v", ev)
	}
}

// ---------------------------------------------------------------- SQL (driver palsu)

func TestStoreRefBMNDanSimpanJenisDalamTransaksi(t *testing.T) {
	ctx := context.Background()
	db, f := fakesql.New(t)
	f.OnQuery = func(_ context.Context, q string, _ []driver.NamedValue) ([]string, [][]driver.Value, error) {
		switch {
		case strings.Contains(q, "FROM sapa_satuan"):
			return []string{"nama", "aktif", "urutan"}, [][]driver.Value{{"bidang", true, int64(1)}, {"unit", true, int64(2)}, {"meter", false, int64(9)}}, nil
		case strings.Contains(q, "FROM sapa_jenis_bmn_satuan"):
			return []string{"jenis", "satuan"}, [][]driver.Value{{"Tanah", "bidang"}, {"tanah dan bangunan", "bidang"}, {"Tanah dan Bangunan", "unit"}, {"Yatim", "unit"}}, nil
		case strings.Contains(q, "FROM sapa_jenis_bmn"):
			return []string{"nama", "aktif", "urutan", "satuan_bawaan"}, [][]driver.Value{{"Tanah", true, int64(1), "bidang"}, {"Tanah dan Bangunan", true, int64(2), ""}}, nil
		}
		return nil, nil, errors.New("query tak terduga: " + q)
	}
	s := NewStore(db)
	ref, err := s.AmbilRefBMN(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(ref.Satuan) != 3 || ref.Satuan[2].Aktif || len(ref.Jenis) != 2 {
		t.Fatalf("ref = %+v", ref)
	}
	// Pemetaan dicocokkan tanpa membedakan huruf besar/kecil; pemetaan yatim (jenis tak dikenal) diabaikan.
	if got := ref.Jenis[1].Satuan; len(got) != 2 || got[0] != "bidang" || got[1] != "unit" || ref.Jenis[1].SatuanBawaan != "" {
		t.Errorf("Tanah dan Bangunan = %+v", ref.Jenis[1])
	}

	db2, f2 := fakesql.New(t)
	if err := NewStore(db2).SimpanJenisBMN(ctx, JenisBMN{Nama: "Pagar", Aktif: true, Urutan: 8, Satuan: []string{"meter", "unit"}, SatuanBawaan: "meter"}, "Admin"); err != nil {
		t.Fatal(err)
	}
	ev := f2.Events()
	// BEGIN, upsert jenis, hapus pemetaan lama, dua pemetaan baru, COMMIT: semuanya dalam satu transaksi.
	if len(ev) != 6 || ev[0] != "BEGIN" || ev[5] != "COMMIT" || !strings.Contains(ev[1], "UPDATE sapa_jenis_bmn") || !strings.Contains(ev[2], "DELETE FROM sapa_jenis_bmn_satuan") {
		t.Errorf("kejadian = %v", ev)
	}
	if ex := f2.Execs(); ex[2].Args[1].Value != "meter" || ex[2].Args[2].Value != int64(1) || ex[3].Args[1].Value != "unit" || ex[3].Args[2].Value != int64(2) {
		t.Errorf("urutan pemetaan = %+v / %+v", ex[2].Args, ex[3].Args)
	}
	cekParameter(t, f)
	cekParameter(t, f2)

	db3, f3 := fakesql.New(t)
	s3 := NewStore(db3)
	_ = s3.SimpanSatuanBMN(ctx, SatuanBMN{Nama: "meter", Aktif: true, Urutan: 7}, "Admin")
	_ = s3.HapusSatuanBMN(ctx, "meter")
	_ = s3.HapusJenisBMN(ctx, "Pagar")
	if f3.Count("EXEC DELETE FROM sapa_satuan") != 1 || f3.Count("EXEC DELETE FROM sapa_jenis_bmn") != 1 {
		t.Errorf("kejadian = %v", f3.Events())
	}
	cekParameter(t, f3)
}
