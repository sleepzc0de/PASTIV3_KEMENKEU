package sapa

import (
	"context"
	"database/sql/driver"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"pasti-v3-backend/internal/fakesql"
)

// Tes ini memakai driver SQL palsu: tidak memeriksa sintaks T-SQL, tetapi memastikan Store memakai transaksi di tempat
// yang benar, jumlah parameter sama dengan placeholder-nya, masukan pengguna tidak masuk ke teks SQL, dan kolom yang
// dibaca sesuai dengan yang dipilih.

var rePlaceholder = regexp.MustCompile(`@p(\d+)`)

// cekParameter: untuk setiap perintah, placeholder @p1..@pN harus lengkap dan sama banyak dengan argumen.
func cekParameter(t *testing.T, f *fakesql.DB) {
	t.Helper()
	semua := append(f.Queries(), f.Execs()...)
	for _, st := range semua {
		maks := 0
		for _, m := range rePlaceholder.FindAllStringSubmatch(st.Query, -1) {
			var n int
			fmt.Sscanf(m[1], "%d", &n)
			if n > maks {
				maks = n
			}
		}
		if maks != len(st.Args) {
			t.Errorf("perintah memakai @p1..@p%d tetapi %d argumen:\n%s", maks, len(st.Args), st.Query)
		}
		for i := 1; i <= maks; i++ {
			if !strings.Contains(st.Query, fmt.Sprintf("@p%d", i)) {
				t.Errorf("placeholder @p%d hilang:\n%s", i, st.Query)
			}
		}
	}
}

func argSebagai(args []driver.NamedValue, i int) interface{} { return args[i].Value }

var wkt = time.Date(2026, 10, 3, 1, 2, 3, 0, time.UTC)

func TestBuatPenjualanSatuTransaksi(t *testing.T) {
	db, f := fakesql.New(t)
	f.OnQuery = func(_ context.Context, q string, args []driver.NamedValue) ([]string, [][]driver.Value, error) {
		switch {
		case strings.HasPrefix(strings.TrimSpace(q), "MERGE sapa_urutan"):
			return []string{"nilai"}, [][]driver.Value{{int64(7)}}, nil
		case strings.HasPrefix(strings.TrimSpace(q), "INSERT INTO sapa_penjualan"):
			return []string{"id"}, [][]driver.Value{{int64(42)}}, nil
		case strings.Contains(q, "FROM sapa_penjualan p WHERE p.id"):
			return []string{"id", "noreg", "kode_satker", "nama_satker", "kode_ue1", "dibuat_oleh", "dibuat_pada", "diperbarui_pada"},
				[][]driver.Value{{int64(42), "PJ-2026-00007", "015010199409294002", "KPKNL", "01501", "Budi", wkt, wkt}}, nil
		}
		return nil, nil, fmt.Errorf("query tak terduga: %s", q)
	}
	s := NewStore(db)
	k, err := s.BuatPenjualan(context.Background(), BuatInput{KodeSatker: "015010199409294002", NamaSatker: "KPKNL", KodeUE1: "01501", UserID: "6F9619FF-8B86-D011-B42D-00C04FC964FF", Oleh: "Budi"})
	if err != nil {
		t.Fatal(err)
	}
	if k.ID != 42 || k.Noreg != "PJ-2026-00007" || k.DibuatOleh != "Budi" {
		t.Errorf("kasus = %+v", k)
	}
	ev := f.Events()
	if len(ev) != 5 || ev[0] != "BEGIN" || ev[4] != "COMMIT" || !strings.Contains(ev[1], "MERGE") || !strings.Contains(ev[2], "INSERT INTO sapa_penjualan") {
		t.Errorf("urutan kejadian = %v", ev)
	}
	// Nomor urut dan penyimpanan usulan sama-sama di dalam transaksi; kunci urutan "penjualan" dan tahun berjalan.
	q := f.Queries()
	if q[0].Args[0].Value != "penjualan" || q[0].Args[1].Value != int64(time.Now().Year()) && q[0].Args[1].Value != time.Now().Year() {
		t.Errorf("argumen MERGE = %+v", q[0].Args)
	}
	if got := q[1].Args[0].Value; got != fmt.Sprintf("PJ-%d-00007", time.Now().Year()) {
		t.Errorf("noreg yang disimpan = %v", got)
	}
	cekParameter(t, f)
}

func TestBuatPenjualanGagalMembatalkanTransaksi(t *testing.T) {
	db, f := fakesql.New(t)
	f.OnQuery = func(_ context.Context, q string, _ []driver.NamedValue) ([]string, [][]driver.Value, error) {
		if strings.Contains(q, "MERGE") {
			return []string{"nilai"}, [][]driver.Value{{int64(1)}}, nil
		}
		return nil, nil, errors.New("disk penuh")
	}
	if _, err := NewStore(db).BuatPenjualan(context.Background(), BuatInput{KodeSatker: kodeA}); err == nil {
		t.Fatal("harus galat")
	}
	if ev := f.Events(); ev[len(ev)-1] != "ROLLBACK" || f.Count("COMMIT") != 0 {
		t.Errorf("kejadian = %v", ev)
	}
}

func TestSimpanTahapUpsertDalamTransaksi(t *testing.T) {
	db, f := fakesql.New(t)
	s := NewStore(db)
	err := s.SimpanTahap(context.Background(), 42, TahapRow{Kunci: TahapNadineSatker, Status: StatusSelesai, Nomor: "ND-1", Tanggal: "2026-10-01", DiperbaruiOleh: "Budi", DiperbaruiPada: wkt})
	if err != nil {
		t.Fatal(err)
	}
	if ev := f.Events(); ev[0] != "BEGIN" || ev[len(ev)-1] != "COMMIT" || f.Count("EXEC") != 2 {
		t.Errorf("kejadian = %v", ev)
	}
	ex := f.Execs()
	if !strings.Contains(ex[0].Query, "UPDATE sapa_penjualan_tahap") || !strings.Contains(ex[0].Query, "INSERT INTO sapa_penjualan_tahap") {
		t.Errorf("upsert tidak lengkap:\n%s", ex[0].Query)
	}
	// Data kosong dan catatan kosong dikirim sebagai NULL.
	if argSebagai(ex[0].Args, 3) != nil || argSebagai(ex[0].Args, 6) != nil {
		t.Errorf("data/catatan kosong harus NULL: %+v", ex[0].Args)
	}
	if argSebagai(ex[0].Args, 4) != "ND-1" || argSebagai(ex[0].Args, 5) != "2026-10-01" {
		t.Errorf("nomor/tanggal = %+v", ex[0].Args)
	}

	db2, f2 := fakesql.New(t)
	err = NewStore(db2).SimpanTahap(context.Background(), 1, TahapRow{Kunci: TahapTim, Status: StatusDraft, Data: []byte(`{"a":1}`)})
	if err != nil {
		t.Fatal(err)
	}
	if got := f2.Execs()[0].Args[3].Value; got != `{"a":1}` {
		t.Errorf("data = %#v", got)
	}
	cekParameter(t, f)
	cekParameter(t, f2)
}

func TestDaftarPenjualanMemakaiParameterDanMeloloskanLIKE(t *testing.T) {
	db, f := fakesql.New(t)
	f.OnQuery = func(_ context.Context, q string, _ []driver.NamedValue) ([]string, [][]driver.Value, error) {
		if strings.Contains(q, "COUNT(1) FROM sapa_penjualan p") {
			return []string{"n"}, [][]driver.Value{{int64(3)}}, nil
		}
		return []string{"id", "noreg", "kode_satker", "nama_satker", "kode_ue1", "dibuat_oleh", "dibuat_pada", "diperbarui_pada"},
			[][]driver.Value{{int64(9), "PJ-2026-00009", kodeA, "KPKNL", "01501", "", wkt, wkt}}, nil
	}
	s := NewStore(db)
	// Kata kunci berniat jahat dan berisi karakter khusus LIKE.
	kunci := "x'; DROP TABLE users;-- 100%_[a]"
	list, total, err := s.DaftarPenjualan(context.Background(), Scope{Kode18: kodeA}, FilterDaftar{Q: kunci, Status: "selesai", Offset: 20, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if total != 3 || len(list) != 1 || list[0].ID != 9 {
		t.Errorf("hasil = %d %+v", total, list)
	}
	for _, st := range f.Queries() {
		if strings.Contains(st.Query, "DROP") || strings.Contains(st.Query, kodeA) {
			t.Errorf("masukan pengguna masuk ke teks SQL:\n%s", st.Query)
		}
		if !strings.Contains(st.Query, "p.kode_satker = @p1") {
			t.Errorf("cakupan satker hilang:\n%s", st.Query)
		}
		if !strings.Contains(st.Query, ">= 10") {
			t.Errorf("filter selesai harus memakai jumlah tahap (10):\n%s", st.Query)
		}
	}
	qs := f.Queries()
	if got := qs[0].Args[1].Value; got != "%x'; DROP TABLE users;-- 100[%][_][[]a]%" {
		t.Errorf("pola LIKE = %q", got)
	}
	// Halaman: OFFSET dan FETCH sebagai parameter terakhir.
	if a := qs[1].Args; len(a) != 4 || a[2].Value != int64(20) && a[2].Value != 20 {
		t.Errorf("argumen halaman = %+v", a)
	}
	cekParameter(t, f)

	// Tanpa hak: tidak ada query sama sekali.
	db2, f2 := fakesql.New(t)
	if l, n, err := NewStore(db2).DaftarPenjualan(context.Background(), Scope{TidakAda: true}, FilterDaftar{}); err != nil || n != 0 || len(l) != 0 || len(f2.Events()) != 0 {
		t.Errorf("TidakAda: %v %d %v %v", err, n, l, f2.Events())
	}
	if l, n, err := NewStore(db2).DaftarPenjualan(context.Background(), Scope{}, FilterDaftar{}); err != nil || n != 0 || len(l) != 0 || len(f2.Events()) != 0 {
		t.Errorf("scope kosong harus dianggap tanpa hak: %v %d %v %v", err, n, l, f2.Events())
	}
}

func TestDaftarPenjualanCakupanUE1DanSemua(t *testing.T) {
	db, f := fakesql.New(t)
	f.OnQuery = func(_ context.Context, q string, _ []driver.NamedValue) ([]string, [][]driver.Value, error) {
		if strings.HasPrefix(strings.TrimSpace(q), "SELECT COUNT(1)") { // bukan subquery penghitung tahap
			return []string{"n"}, [][]driver.Value{{int64(0)}}, nil
		}
		return []string{"id", "noreg", "kode_satker", "nama_satker", "kode_ue1", "dibuat_oleh", "dibuat_pada", "diperbarui_pada"}, nil, nil
	}
	s := NewStore(db)
	if _, _, err := s.DaftarPenjualan(context.Background(), Scope{KodeUE1: "01501"}, FilterDaftar{Status: "berjalan"}); err != nil {
		t.Fatal(err)
	}
	if q := f.Queries()[0].Query; !strings.Contains(q, "p.kode_ue1 = @p1") || !strings.Contains(q, "< 10") {
		t.Errorf("query UE1:\n%s", q)
	}
	if _, _, err := s.DaftarPenjualan(context.Background(), Scope{Semua: true}, FilterDaftar{}); err != nil {
		t.Fatal(err)
	}
	if q := f.Queries()[2].Query; strings.Contains(q, "WHERE") {
		t.Errorf("cakupan semua tanpa filter tidak boleh punya WHERE:\n%s", q)
	}
	cekParameter(t, f)
}

func TestStatusTahapBanyakDanTahapPenjualan(t *testing.T) {
	db, f := fakesql.New(t)
	f.OnQuery = func(_ context.Context, q string, _ []driver.NamedValue) ([]string, [][]driver.Value, error) {
		if strings.Contains(q, "SELECT penjualan_id, kunci, status") {
			return []string{"a", "b", "c"}, [][]driver.Value{{int64(1), "tim", "selesai"}, {int64(1), "ba", "draft"}, {int64(3), "tim", "dilewati"}}, nil
		}
		return []string{"kunci", "status", "data", "nomor", "tanggal", "catatan", "oleh", "pada"}, [][]driver.Value{
			{"nadine_satker", "selesai", nil, "ND-1", time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), nil, "Budi", wkt},
			{"tim", "draft", []byte(`{"a":1}`), nil, nil, "catatan", nil, wkt},
		}, nil
	}
	s := NewStore(db)
	m, err := s.StatusTahapBanyak(context.Background(), []int64{1, 2, 3})
	if err != nil {
		t.Fatal(err)
	}
	if m[1]["tim"] != "selesai" || m[1]["ba"] != "draft" || m[3]["tim"] != "dilewati" || m[2] == nil || len(m[2]) != 0 {
		t.Errorf("status = %+v", m)
	}
	if got := f.Queries()[0].Query; !strings.Contains(got, "IN (@p1,@p2,@p3)") {
		t.Errorf("IN list: %s", got)
	}
	if m, err := s.StatusTahapBanyak(context.Background(), nil); err != nil || len(m) != 0 || f.Count("QUERY") != 1 {
		t.Errorf("tanpa id: %v %v", m, err)
	}

	rows, err := s.TahapPenjualan(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	n := rows["nadine_satker"]
	if n.Nomor != "ND-1" || n.Tanggal != "2026-10-01" || n.DiperbaruiOleh != "Budi" || n.Data != nil {
		t.Errorf("nadine = %+v", n)
	}
	if tm := rows["tim"]; string(tm.Data) != `{"a":1}` || tm.Catatan != "catatan" || tm.Nomor != "" || tm.Tanggal != "" {
		t.Errorf("tim = %+v", tm)
	}
	cekParameter(t, f)
}

func TestSimpanPeranMemeriksaPenggunaDanMenghapus(t *testing.T) {
	ctx := context.Background()
	db, f := fakesql.New(t)
	f.OnQuery = func(_ context.Context, q string, _ []driver.NamedValue) ([]string, [][]driver.Value, error) {
		return []string{"n"}, [][]driver.Value{{int64(0)}}, nil // pengguna tidak ada
	}
	if err := NewStore(db).SimpanPeran(ctx, guid1, PeranKanwil, "", "", "Admin"); !errors.Is(err, ErrTidakDitemukan) {
		t.Errorf("pengguna tak ada: %v", err)
	}
	if ev := f.Events(); ev[len(ev)-1] != "ROLLBACK" || f.Count("EXEC") != 0 {
		t.Errorf("kejadian = %v", ev)
	}

	db2, f2 := fakesql.New(t)
	f2.OnQuery = func(_ context.Context, q string, _ []driver.NamedValue) ([]string, [][]driver.Value, error) {
		return []string{"n"}, [][]driver.Value{{int64(1)}}, nil
	}
	s := NewStore(db2)
	if err := s.SimpanPeran(ctx, guid1, "", "", "", "Admin"); err != nil {
		t.Fatal(err)
	}
	if ex := f2.Execs(); len(ex) != 1 || !strings.HasPrefix(strings.TrimSpace(ex[0].Query), "DELETE FROM sapa_peran") {
		t.Errorf("peran kosong harus menghapus: %+v", ex)
	}
	if err := s.SimpanPeran(ctx, guid1, PeranSatker, kodeA, "", "Admin"); err != nil {
		t.Fatal(err)
	}
	ex := f2.Execs()[1]
	if ex.Args[2].Value != kodeA || ex.Args[3].Value != nil {
		t.Errorf("argumen = %+v", ex.Args)
	}
	cekParameter(t, f)
	cekParameter(t, f2)
}

func TestPeranPenggunaTidakDikenalDanDikenal(t *testing.T) {
	db, f := fakesql.New(t)
	f.OnQuery = func(_ context.Context, q string, args []driver.NamedValue) ([]string, [][]driver.Value, error) {
		cols := []string{"full_name", "peran", "kode_satker", "kode_ue1"}
		switch args[0].Value {
		case guid1:
			return cols, [][]driver.Value{{"Budi Santoso", "satker", kodeA, nil}}, nil
		case guid2:
			return cols, [][]driver.Value{{"Siti", nil, nil, nil}}, nil
		}
		return cols, nil, nil
	}
	s := NewStore(db)
	if p, err := s.PeranPengguna(context.Background(), "tidak-ada"); err != nil || p != nil {
		t.Errorf("tak dikenal: %+v %v", p, err)
	}
	p, err := s.PeranPengguna(context.Background(), guid1)
	if err != nil || p == nil || p.Nama != "Budi Santoso" || p.Peran != PeranSatker || p.KodeSatker != kodeA || p.KodeUE1 != "" {
		t.Errorf("peran = %+v %v", p, err)
	}
	p, err = s.PeranPengguna(context.Background(), guid2)
	if err != nil || p == nil || p.Nama != "Siti" || p.Peran != "" {
		t.Errorf("tanpa peran = %+v %v", p, err)
	}
	cekParameter(t, f)
}

func TestSimpanTemplateVersiBaruDanDokumen(t *testing.T) {
	ctx := context.Background()
	db, f := fakesql.New(t)
	f.OnQuery = func(_ context.Context, q string, args []driver.NamedValue) ([]string, [][]driver.Value, error) {
		switch {
		case strings.Contains(q, "MAX(versi)"):
			return []string{"v"}, [][]driver.Value{{int64(3)}}, nil
		case strings.Contains(q, "INSERT INTO sapa_template"):
			return []string{"id", "kunci", "versi", "nama_file", "ukuran", "aktif", "catatan", "oleh", "pada"},
				[][]driver.Value{{int64(11), "sk_tim", int64(3), "SK.docx", int64(4), true, nil, "Admin", wkt}}, nil
		case strings.Contains(q, "INSERT INTO sapa_dokumen"):
			return []string{"id", "pada"}, [][]driver.Value{{int64(77), wkt}}, nil
		}
		return nil, nil, fmt.Errorf("tak terduga: %s", q)
	}
	s := NewStore(db)
	info, err := s.SimpanTemplate(ctx, TemplateBaru{Kunci: "sk_tim", NamaFile: "SK.docx", Berkas: []byte("abcd"), Oleh: "Admin"})
	if err != nil {
		t.Fatal(err)
	}
	if info.ID != 11 || info.Versi != 3 || !info.Aktif || info.Catatan != "" || info.NamaFile != "SK.docx" {
		t.Errorf("info = %+v", info)
	}
	ev := f.Events()
	// BEGIN, versi, nonaktifkan versi lama, simpan baru, COMMIT
	if len(ev) != 5 || ev[0] != "BEGIN" || !strings.Contains(ev[2], "UPDATE sapa_template SET aktif = 0") || ev[4] != "COMMIT" {
		t.Errorf("kejadian = %v", ev)
	}

	d, err := s.SimpanDokumen(ctx, DokumenBaru{PenjualanID: 5, Tahap: TahapNDSatker, Jenis: DokNDSatker, NamaFile: "ND.docx", Berkas: []byte("xyz"), Peringatan: []string{"a", "b"}, Oleh: "Budi"})
	if err != nil {
		t.Fatal(err)
	}
	if d.ID != 77 || d.Ukuran != 3 || d.JenisLabel == "" || len(d.Peringatan) != 2 {
		t.Errorf("dokumen = %+v", d)
	}
	qs := f.Queries()
	last := qs[len(qs)-1]
	if last.Args[5].Value == nil || last.Args[6].Value != `["a","b"]` {
		t.Errorf("argumen dokumen = %+v", last.Args)
	}
	cekParameter(t, f)
}

func TestCariSatkerMencocokkanAwalan(t *testing.T) {
	db, f := fakesql.New(t)
	f.OnQuery = func(_ context.Context, q string, args []driver.NamedValue) ([]string, [][]driver.Value, error) {
		cols := []string{"kode", "nama", "kab", "prov", "ue1"}
		if args[0].Value == kodeA {
			return cols, [][]driver.Value{{kodeA + "KP", "KPKNL Jakarta I", "KOTA JAKARTA PUSAT", nil, "01501"}}, nil
		}
		return cols, nil, nil
	}
	s := NewStore(db)
	si, err := s.CariSatker(context.Background(), kodeA)
	if err != nil || si == nil || si.Kode != kodeA+"KP" || si.Nama != "KPKNL Jakarta I" || si.KabKota != "KOTA JAKARTA PUSAT" || si.Provinsi != "" || si.KodeUE1 != "01501" {
		t.Errorf("satker = %+v %v", si, err)
	}
	if si, err := s.CariSatker(context.Background(), kodeB); err != nil || si != nil {
		t.Errorf("tak ada: %+v %v", si, err)
	}
	if q := f.Queries()[0].Query; !strings.Contains(q, "DIGITALISASI_SATKER") || !strings.Contains(q, "LIKE @p1 + '%'") {
		t.Errorf("query:\n%s", q)
	}
	cekParameter(t, f)
}
