package sapa

import (
	"context"
	"database/sql/driver"
	"encoding/json"
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
			// SQL Server mengembalikan CONVERT(NVARCHAR(36), uniqueidentifier) dalam huruf besar.
			return []string{"id", "uuid", "noreg", "kode_satker", "nama_satker", "kode_ue1", "dibuat_oleh", "dibuat_pada", "diperbarui_pada"},
				[][]driver.Value{{int64(42), "6F9619FF-8B86-D011-B42D-00C04FC964FF", "PJ-2026-00007", "015010199409294002", "KPKNL", "01501", "Budi", wkt, wkt}}, nil
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
	if k.UUID != "6f9619ff-8b86-d011-b42d-00c04fc964ff" {
		t.Errorf("UUID harus dibakukan ke huruf kecil, dapat %q", k.UUID)
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
	// UUID dibuat aplikasi (acak, bukan berurutan) dan disimpan sebagai argumen terakhir INSERT.
	disimpan, _ := q[1].Args[6].Value.(string)
	if baku, ok := BakukanUUID(disimpan); !ok || baku != disimpan {
		t.Errorf("UUID yang disimpan = %q, harus UUID baku huruf kecil", disimpan)
	}
	if !strings.Contains(q[1].Query, "uuid") {
		t.Errorf("INSERT tidak menyimpan kolom uuid:\n%s", q[1].Query)
	}
	cekParameter(t, f)
}

func TestAmbilPenjualanMenurutUUID(t *testing.T) {
	db, f := fakesql.New(t)
	f.OnQuery = func(_ context.Context, q string, args []driver.NamedValue) ([]string, [][]driver.Value, error) {
		if !strings.Contains(q, "WHERE p.uuid = @p1") {
			return nil, nil, fmt.Errorf("query tak terduga: %s", q)
		}
		if args[0].Value == "6f9619ff-8b86-d011-b42d-00c04fc964ff" {
			return []string{"id", "uuid", "noreg", "kode_satker", "nama_satker", "kode_ue1", "dibuat_oleh", "dibuat_pada", "diperbarui_pada"},
				[][]driver.Value{{int64(42), "6F9619FF-8B86-D011-B42D-00C04FC964FF", "PJ-2026-00007", kodeA, "KPKNL", "01501", "", wkt, wkt}}, nil
		}
		return []string{"id", "uuid", "noreg", "kode_satker", "nama_satker", "kode_ue1", "dibuat_oleh", "dibuat_pada", "diperbarui_pada"}, nil, nil
	}
	s := NewStore(db)
	k, err := s.AmbilPenjualanUUID(context.Background(), "6f9619ff-8b86-d011-b42d-00c04fc964ff")
	if err != nil || k == nil || k.ID != 42 || k.UUID != "6f9619ff-8b86-d011-b42d-00c04fc964ff" {
		t.Fatalf("hasil = %+v, err = %v", k, err)
	}
	if k, err := s.AmbilPenjualanUUID(context.Background(), "00000000-0000-4000-8000-000000000000"); err != nil || k != nil {
		t.Errorf("UUID yang tidak ada harus menghasilkan nil tanpa galat: %+v, %v", k, err)
	}
	cekParameter(t, f)
}

func TestKasusDanDokumenTidakMengirimNomorIDInternal(t *testing.T) {
	k, _ := json.Marshal(KasusInfo{ID: 12345, UUID: "6f9619ff-8b86-d011-b42d-00c04fc964ff", Noreg: "PJ-2026-00001"})
	if !strings.Contains(string(k), `"id":"6f9619ff-8b86-d011-b42d-00c04fc964ff"`) || strings.Contains(string(k), "12345") {
		t.Errorf("KasusInfo = %s; id harus UUID dan nomor internal tidak boleh ikut", k)
	}
	d, _ := json.Marshal(DokumenInfo{ID: 7, PenjualanID: 12345})
	if strings.Contains(string(d), "12345") || strings.Contains(string(d), "penjualan_id") {
		t.Errorf("DokumenInfo = %s; id penjualan internal tidak boleh ikut", d)
	}
}

func TestBakukanUUID(t *testing.T) {
	for _, c := range []struct {
		masuk, want string
		ok          bool
	}{
		{"6F9619FF-8B86-D011-B42D-00C04FC964FF", "6f9619ff-8b86-d011-b42d-00c04fc964ff", true},
		{" 6f9619ff-8b86-d011-b42d-00c04fc964ff ", "6f9619ff-8b86-d011-b42d-00c04fc964ff", true},
		{"6f9619ff8b86d011b42d00c04fc964ff", "", false},                       // tanpa tanda hubung
		{"{6f9619ff-8b86-d011-b42d-00c04fc964ff}", "", false},                 // berkurung
		{"urn:uuid:6f9619ff-8b86-d011-b42d-00c04fc964ff", "", false},          // berawalan
		{"6f9619ff-8b86-d011-b42d-00c04fc964fg", "", false},                   // bukan heksadesimal
		{"6f9619ff-8b86-d011-b42d-00c04fc964ff'; DROP TABLE x;--", "", false}, // sisipan SQL
		{"1", "", false},
		{"", "", false},
	} {
		if got, ok := BakukanUUID(c.masuk); got != c.want || ok != c.ok {
			t.Errorf("BakukanUUID(%q) = %q, %v; want %q, %v", c.masuk, got, ok, c.want, c.ok)
		}
	}
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
		return []string{"id", "uuid", "noreg", "kode_satker", "nama_satker", "kode_ue1", "dibuat_oleh", "dibuat_pada", "diperbarui_pada"},
			[][]driver.Value{{int64(9), "6F9619FF-8B86-D011-B42D-00C04FC964FF", "PJ-2026-00009", kodeA, "KPKNL", "01501", "", wkt, wkt}}, nil
	}
	s := NewStore(db)
	// Kata kunci berniat jahat dan berisi karakter khusus LIKE.
	kunci := "x'; DROP TABLE users;-- 100%_[a]"
	list, total, err := s.DaftarPenjualan(context.Background(), Scope{Kode6: Kode6Dari(kodeA)}, FilterDaftar{Q: kunci, Status: "selesai", Offset: 20, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if total != 3 || len(list) != 1 || list[0].ID != 9 {
		t.Errorf("hasil = %d %+v", total, list)
	}
	for _, st := range f.Queries() {
		if strings.Contains(st.Query, "DROP") || strings.Contains(st.Query, kodeA) || strings.Contains(st.Query, Kode6Dari(kodeA)) {
			t.Errorf("masukan pengguna masuk ke teks SQL:\n%s", st.Query)
		}
		if !strings.Contains(st.Query, "SUBSTRING(p.kode_satker, 10, 6) = @p1") {
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
	if _, _, err := s.DaftarPenjualan(context.Background(), Scope{Kanwil9: "015010199"}, FilterDaftar{}); err != nil {
		t.Fatal(err)
	}
	if q := f.Queries()[2].Query; !strings.Contains(q, "LEFT(p.kode_satker, 9) = @p1") || strings.Contains(q, "015010199") {
		t.Errorf("cakupan Kanwil harus memakai 9 karakter pertama kode satker sebagai parameter:\n%s", q)
	}
	if got := f.Queries()[2].Args[0].Value; got != "015010199" {
		t.Errorf("argumen Kanwil = %v", got)
	}
	if _, _, err := s.DaftarPenjualan(context.Background(), Scope{Semua: true}, FilterDaftar{}); err != nil {
		t.Fatal(err)
	}
	if q := f.Queries()[4].Query; strings.Contains(q, "WHERE") {
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

func TestNamaPenggunaTidakDikenalDanDikenal(t *testing.T) {
	db, f := fakesql.New(t)
	f.OnQuery = func(_ context.Context, q string, args []driver.NamedValue) ([]string, [][]driver.Value, error) {
		if strings.Contains(q, "sapa_peran") {
			return nil, nil, fmt.Errorf("peran SAPA tidak dibaca lagi: %s", q)
		}
		cols := []string{"full_name"}
		switch args[0].Value {
		case guid1:
			return cols, [][]driver.Value{{"Budi Santoso"}}, nil
		case guid2:
			return cols, [][]driver.Value{{nil}}, nil
		}
		return cols, nil, nil
	}
	s := NewStore(db)
	if n, err := s.NamaPengguna(context.Background(), "tidak-ada"); err != nil || n != "" {
		t.Errorf("tak dikenal: %q %v", n, err)
	}
	if n, err := s.NamaPengguna(context.Background(), guid1); err != nil || n != "Budi Santoso" {
		t.Errorf("nama = %q %v", n, err)
	}
	if n, err := s.NamaPengguna(context.Background(), guid2); err != nil || n != "" {
		t.Errorf("nama kosong = %q %v", n, err)
	}
	cekParameter(t, f)
}

// Satker berkode 6 digit itu (induk lebih dulu) dicari dengan parameter, bukan dengan menyisipkan kode ke teks SQL.
func TestSatkerDenganKode6(t *testing.T) {
	db, f := fakesql.New(t)
	f.OnQuery = func(_ context.Context, q string, args []driver.NamedValue) ([]string, [][]driver.Value, error) {
		return []string{"Kode_Satker", "Nama_Satker", "KabKota_Satker", "Provinsi_Satker", "Kode_UE1"},
			[][]driver.Value{
				{"015010199409294000KP", "INDUK", "KOTA JAKARTA PUSAT", "DKI JAKARTA", "01501"},
				{"015010199409294001KP", "ANAK", nil, nil, nil},
			}, nil
	}
	hasil, err := NewStore(db).SatkerDenganKode6(context.Background(), "409294")
	if err != nil || len(hasil) != 2 || hasil[0].Nama != "INDUK" || hasil[0].KodeUE1 != "01501" || hasil[1].KabKota != "" {
		t.Fatalf("hasil = %+v (%v)", hasil, err)
	}
	q := f.Queries()[0]
	if !strings.Contains(q.Query, "SUBSTRING(Kode_Satker, 10, 6) = @p1") || strings.Contains(q.Query, "409294") || q.Args[0].Value != "409294" {
		t.Errorf("kode 6 digit harus parameter:\n%s %+v", q.Query, q.Args)
	}
	if !strings.Contains(q.Query, "ORDER BY CASE WHEN SUBSTRING(Kode_Satker, 16, 3) = '000'") {
		t.Errorf("induk harus lebih dulu:\n%s", q.Query)
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

// Nama UE1 dibaca dari referensi umum (ref_ue1); daftar memuat juga UE1 aktif yang belum punya sebutan Sekretaris.
func TestRefUE1MemakaiReferensiUmum(t *testing.T) {
	db, f := fakesql.New(t)
	f.OnQuery = func(_ context.Context, q string, args []driver.NamedValue) ([]string, [][]driver.Value, error) {
		switch {
		case strings.Contains(q, "WHERE s.kode = @p1"):
			return []string{"kode", "nama", "sebutan"}, [][]driver.Value{{"01509", "DIREKTORAT JENDERAL KEKAYAAN NEGARA", "Sekretaris Direktorat Jenderal Kekayaan Negara"}}, nil
		case strings.Contains(q, "UNION ALL"):
			return []string{"kode", "nama", "sebutan"}, [][]driver.Value{
				{"01501", "SEKRETARIAT JENDERAL", ""},
				{"01509", "DIREKTORAT JENDERAL KEKAYAAN NEGARA", "Sekretaris Direktorat Jenderal Kekayaan Negara"},
			}, nil
		}
		return nil, nil, fmt.Errorf("query tak terduga: %s", q)
	}
	s := &Store{DB: db}
	ctx := context.Background()

	r, err := s.AmbilRefUE1(ctx, "01509")
	if err != nil || r == nil || r.Nama != "DIREKTORAT JENDERAL KEKAYAAN NEGARA" || r.Sekretaris == "" {
		t.Fatalf("AmbilRefUE1 = %+v, %v", r, err)
	}
	if q := lastQueryText(f); !strings.Contains(q, "LEFT JOIN ref_ue1 m ON m.kode = s.kode") || !strings.Contains(q, "COALESCE(m.nama, s.nama)") {
		t.Errorf("AmbilRefUE1 harus mengutamakan nama dari ref_ue1:\n%s", q)
	}

	daftar, err := s.DaftarRefUE1(ctx)
	if err != nil || len(daftar) != 2 || daftar[0].Kode != "01501" || daftar[0].Sekretaris != "" || daftar[1].Sekretaris == "" {
		t.Fatalf("DaftarRefUE1 = %+v, %v", daftar, err)
	}
	if q := lastQueryText(f); !strings.Contains(q, "m.aktif = 1") || !strings.Contains(q, "NOT EXISTS (SELECT 1 FROM sapa_ref_ue1 s WHERE s.kode = m.kode)") {
		t.Errorf("DaftarRefUE1 harus menambah UE1 aktif yang belum punya sebutan:\n%s", q)
	}
	cekParameter(t, f)
}

func lastQueryText(f *fakesql.DB) string {
	qs := f.Queries()
	if len(qs) == 0 {
		return ""
	}
	return qs[len(qs)-1].Query
}
