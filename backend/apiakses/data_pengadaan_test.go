package apiakses

import (
	"context"
	"database/sql/driver"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"pasti-v3-backend/internal/fakesql"
	"pasti-v3-backend/peran"
)

// Tes pembaca data pengadaan tanpa SQL Server: database palsu mencatat query dan argumennya, dan menjawab dengan baris yang ditetapkan tes.

// batasUji: aturan pembatasan tiruan; menandai kondisi supaya tes dapat memeriksa cakupan mana yang dipasang pada tabel mana.
func batasUji(tabel string, cak peran.Cakupan) (string, bool) {
	return fmt.Sprintf("/*%s|%s|%s*/ 1 = 1", tabel, cak.Tingkat, cak.Kode), true
}

func datasetUji(t *testing.T, kode string) DatasetPengadaan {
	t.Helper()
	d, ok := CariDatasetPengadaan(kode)
	if !ok {
		t.Fatalf("dataset %s tidak ada", kode)
	}
	return d
}

// barisUji menyusun satu baris hasil menurut urutan kolom SELECT dataset: kunci, kolom data (nilai bawaan NULL kecuali ditetapkan), updated_at.
func barisUji(d DatasetPengadaan, kunci string, nilai map[string]driver.Value) []driver.Value {
	b := []driver.Value{kunci}
	for _, k := range d.Kolom {
		b = append(b, nilai[k.Nama])
	}
	return append(b, time.Date(2026, 10, 7, 3, 0, 0, 0, time.FixedZone("WIB", 7*3600)))
}

func kolomHasilUji(d DatasetPengadaan) []string {
	c := []string{d.KolomKunci()}
	for _, k := range d.Kolom {
		c = append(c, k.Nama)
	}
	return append(c, "updated_at")
}

func TestPengadaanDaftarTanpaPenyaringTidakPunyaWhere(t *testing.T) {
	d := datasetUji(t, "pengadaan.tender_pengumuman")
	db, f := fakesql.New(t)
	f.OnQuery = func(context.Context, string, []driver.NamedValue) ([]string, [][]driver.Value, error) {
		return kolomHasilUji(d), nil, nil
	}
	h, err := d.BacaDaftar(context.Background(), db, FilterPengadaan{}, 500)
	if err != nil {
		t.Fatal(err)
	}
	if h.AdaBerikutnya || h.Baris == nil || len(h.Baris) != 0 {
		t.Errorf("hasil kosong harus berupa daftar kosong (bukan nil): %+v", h)
	}
	q := f.Queries()
	if len(q) != 1 {
		t.Fatalf("query = %d, want 1", len(q))
	}
	sqlTeks := strings.Join(strings.Fields(q[0].Query), " ")
	if !strings.HasPrefix(sqlTeks, "SELECT TOP (101) [row_key], [kd_klpd],") || !strings.HasSuffix(sqlTeks, "[updated_at] FROM [inaproc_tender_pengumuman] ORDER BY [row_key]") {
		t.Errorf("sql = %s", sqlTeks)
	}
	if strings.Contains(sqlTeks, "WHERE") || len(q[0].Args) != 0 {
		t.Errorf("tanpa penyaring tidak boleh ada WHERE/argumen: %s %v", sqlTeks, q[0].Args)
	}
	// kolom yang tidak boleh keluar tidak ada di SELECT
	for _, larang := range []string{"nip_ppk", "nama_ppk", "nip_pokja", "nama_pokja", "ket_ditutup", "extra_json", "synced_at", "SELECT *"} {
		if strings.Contains(sqlTeks, larang) {
			t.Errorf("SELECT memuat %q: %s", larang, sqlTeks)
		}
	}
}

func TestPengadaanPenyaringDanPembatasanKlien(t *testing.T) {
	d := datasetUji(t, "pengadaan.tender_pengumuman")
	db, f := fakesql.New(t)
	f.OnQuery = func(_ context.Context, q string, _ []driver.NamedValue) ([]string, [][]driver.Value, error) {
		if strings.Contains(q, "COUNT_BIG") {
			return []string{"n"}, [][]driver.Value{{int64(7)}}, nil
		}
		return kolomHasilUji(d), nil, nil
	}
	fl := FilterPengadaan{
		Q: "100%_[a]", KodeKLPD: "K10", Tahun: "2025", KodeSatker: "119091", KodeUE1: "01504", Kursor: "abc123", PerHalaman: 20, Hitung: true,
		UE1Klien: []string{"01504", "01505"}, Batas: batasUji,
	}
	h, err := d.BacaDaftar(context.Background(), db, fl, 500)
	if err != nil {
		t.Fatal(err)
	}
	if h.Total == nil || *h.Total != 7 {
		t.Errorf("total = %v", h.Total)
	}
	qs := f.Queries()
	if len(qs) != 2 {
		t.Fatalf("query = %d, want 2 (hitung + data)", len(qs))
	}
	hitung, data := strings.Join(strings.Fields(qs[0].Query), " "), strings.Join(strings.Fields(qs[1].Query), " ")
	if strings.Contains(hitung, "[row_key] >") {
		t.Errorf("hitungan tidak boleh memakai kursor: %s", hitung)
	}
	for _, sql := range []string{hitung, data} {
		for _, perlu := range []string{
			"/*inaproc_tender_pengumuman|ue1|01504*/ 1 = 1", "/*inaproc_tender_pengumuman|ue1|01505*/ 1 = 1", " OR ", // pembatasan UE1 klien: salah satu UE1
			"/*inaproc_tender_pengumuman|satker|119091*/ 1 = 1", // penyaring satker
			"[kd_klpd] = @p", "[tahun_anggaran] = @p", "LIKE @p", `ESCAPE '\'`,
		} {
			if !strings.Contains(sql, perlu) {
				t.Errorf("sql tidak memuat %q:\n%s", perlu, sql)
			}
		}
		// masukan pemanggil tidak pernah ada di teks SQL
		for _, masukan := range []string{"K10", "2025", "abc123", "100%", "[a]"} {
			if strings.Contains(sql, masukan) {
				t.Errorf("masukan %q tertulis langsung di SQL:\n%s", masukan, sql)
			}
		}
	}
	// kursor hanya pada query data; semua nilai masukan menjadi argumen (kursor, klpd, tahun, pola pencarian)
	if !strings.Contains(data, "[row_key] > @p1") || len(qs[1].Args) != 4 {
		t.Errorf("data: %s args=%d", data, len(qs[1].Args))
	}
	if len(qs[0].Args) != 3 {
		t.Errorf("hitungan: args=%d, want 3 (klpd, tahun, pola)", len(qs[0].Args))
	}
	var pola string
	for _, a := range qs[1].Args {
		if s, ok := a.Value.(string); ok && strings.Contains(s, "%") {
			pola = s
		}
	}
	if pola != `%100\%\_\[a]%` {
		t.Errorf("pola pencarian = %q, karakter khusus LIKE harus di-escape", pola)
	}
	if !strings.Contains(data, "TOP (21)") {
		t.Errorf("per_halaman 20 harus meminta 21 baris: %s", data)
	}
}

func TestPengadaanTanpaAturanPembatasanMenutupSemuaBaris(t *testing.T) {
	d := datasetUji(t, "pengadaan.tender_pengumuman")
	for nama, fl := range map[string]FilterPengadaan{
		"ue1 klien":   {UE1Klien: []string{"01504"}},
		"kode_ue1":    {KodeUE1: "01504"},
		"kode_satker": {KodeSatker: "119091"},
	} {
		w, _ := d.where(fl, false)
		if !strings.Contains(w, "1 = 0") || strings.Contains(w, "1 = 1") {
			t.Errorf("%s: tanpa aturan pembatasan harus menutup semua baris, got %q", nama, w)
		}
	}
	// tabel yang tidak dapat dibatasi (ok=false) juga menutup semua baris, bukan membuka
	tutup := func(string, peran.Cakupan) (string, bool) { return "1 = 1", false }
	w, _ := d.where(FilterPengadaan{UE1Klien: []string{"01504"}, Batas: tutup}, false)
	if !strings.Contains(w, "1 = 0") || strings.Contains(w, "1 = 1") {
		t.Errorf("tabel tak dapat dibatasi harus menutup semua baris, got %q", w)
	}
	// tanpa pembatasan klien dan tanpa penyaring wilayah: tidak ada kondisi cakupan (dan aturan tidak dipanggil)
	panggil := false
	w, _ = d.where(FilterPengadaan{Batas: func(string, peran.Cakupan) (string, bool) { panggil = true; return "", true }}, false)
	if w != "" || panggil {
		t.Errorf("tanpa pembatasan: where=%q aturanDipanggil=%v", w, panggil)
	}
}

func TestPengadaanValidasiMasukan(t *testing.T) {
	d := datasetUji(t, "pengadaan.tender_pengumuman")
	buruk := map[string]FilterPengadaan{
		"q pendek":           {Q: "ab"},
		"q panjang":          {Q: strings.Repeat("a", MaksPencarian+1)},
		"klpd injeksi":       {KodeKLPD: "K10' OR 1=1--"},
		"klpd panjang":       {KodeKLPD: strings.Repeat("K", 21)},
		"tahun huruf":        {Tahun: "20x5"},
		"tahun lima digit":   {Tahun: "20255"},
		"tahun injeksi":      {Tahun: "2025'; DROP TABLE x;--"},
		"satker 5 digit":     {KodeSatker: "11909"},
		"satker prefix lama": {KodeSatker: "119091A"},
		"ue1 6 digit":        {KodeUE1: "015041"},
		"kursor injeksi":     {Kursor: "x' OR '1'='1"},
		"kursor spasi":       {Kursor: "a b"},
		"kursor panjang":     {Kursor: strings.Repeat("a", 256)},
	}
	for nama, fl := range buruk {
		if err := d.Periksa(fl); err == nil {
			t.Errorf("%s: harus ditolak", nama)
		}
	}
	sah := FilterPengadaan{Q: "laptop", KodeKLPD: "K10", Tahun: "2025", KodeSatker: "119091", KodeUE1: "01504", Kursor: strings.Repeat("a", 64)}
	if err := d.Periksa(sah); err != nil {
		t.Errorf("penyaring sah ditolak: %v", err)
	}
	// pesan galat tidak memantulkan isian pemanggil
	err := d.Periksa(FilterPengadaan{KodeKLPD: "<script>alert(1)</script>"})
	if err == nil || strings.Contains(err.Error(), "script") {
		t.Errorf("galat = %v", err)
	}
	// penyaring yang tidak dikenal dataset ditolak, bukan diabaikan
	inst := datasetUji(t, "pengadaan.ekatalog_instansi_satker")
	if err := inst.Periksa(FilterPengadaan{Tahun: "2025"}); err == nil {
		t.Error("dataset tanpa kolom tahun harus menolak penyaring tahun")
	}
	if err := inst.Periksa(FilterPengadaan{KodeKLPD: "K10"}); err != nil {
		t.Errorf("instansi_satker punya kd_klpd: %v", err)
	}
}

func TestPengadaanPaginasiKursorDanPemetaanNilai(t *testing.T) {
	d := datasetUji(t, "pengadaan.tender_pengumuman")
	db, f := fakesql.New(t)
	nilai := func(id string) []driver.Value {
		return barisUji(d, id, map[string]driver.Value{
			"kd_tender":      "T-1",
			"nama_paket":     []byte("Pengadaan laptop"), // NVARCHAR(MAX) dapat tiba sebagai byte
			"pagu":           []byte("1234567.50"),       // DECIMAL tiba sebagai teks
			"versi_tender":   int64(2),
			"tgl_buat_paket": time.Date(2026, 3, 1, 8, 30, 0, 0, time.FixedZone("WIB", 7*3600)),
		})
	}
	f.OnQuery = func(context.Context, string, []driver.NamedValue) ([]string, [][]driver.Value, error) {
		return kolomHasilUji(d), [][]driver.Value{nilai("k1"), nilai("k2"), nilai("k3")}, nil // 3 baris untuk halaman berukuran 2: ada berikutnya
	}
	h, err := d.BacaDaftar(context.Background(), db, FilterPengadaan{PerHalaman: 2}, 500)
	if err != nil {
		t.Fatal(err)
	}
	if len(h.Baris) != 2 || !h.AdaBerikutnya || h.KursorBerikutnya != "k2" {
		t.Fatalf("halaman: %d baris, ada=%v, kursor=%q", len(h.Baris), h.AdaBerikutnya, h.KursorBerikutnya)
	}
	b := h.Baris[0]
	if b["id"] != "k1" || b["kd_tender"] != "T-1" || b["nama_paket"] != "Pengadaan laptop" {
		t.Errorf("teks: %v", b)
	}
	if v, ok := b["pagu"].(float64); !ok || v != 1234567.5 {
		t.Errorf("desimal harus menjadi angka: %#v", b["pagu"])
	}
	if b["versi_tender"] != int64(2) {
		t.Errorf("bilangan: %#v", b["versi_tender"])
	}
	if b["tgl_buat_paket"] != "2026-03-01T01:30:00Z" {
		t.Errorf("waktu harus ISO UTC: %#v", b["tgl_buat_paket"])
	}
	if b["disinkronkan_pada"] != "2026-10-06T20:00:00Z" {
		t.Errorf("disinkronkan_pada = %#v", b["disinkronkan_pada"])
	}
	if v, ada := b["hps"]; !ada || v != nil {
		t.Errorf("NULL harus tetap null: %#v ada=%v", v, ada)
	}
	for _, larang := range []string{"nip_ppk", "nama_ppk", "extra_json", "row_key", "updated_at", "synced_at"} {
		if _, ada := b[larang]; ada {
			t.Errorf("baris memuat %q", larang)
		}
	}

	// halaman terakhir: tidak ada berikutnya, kursor kosong
	f.OnQuery = func(context.Context, string, []driver.NamedValue) ([]string, [][]driver.Value, error) {
		return kolomHasilUji(d), [][]driver.Value{nilai("k9")}, nil
	}
	h, err = d.BacaDaftar(context.Background(), db, FilterPengadaan{PerHalaman: 2}, 500)
	if err != nil || len(h.Baris) != 1 || h.AdaBerikutnya || h.KursorBerikutnya != "" {
		t.Errorf("halaman terakhir: %+v %v", h, err)
	}
}

func TestPengadaanPerHalamanDibatasiPengaturan(t *testing.T) {
	d := datasetUji(t, "pengadaan.rup_paket_penyedia")
	db, f := fakesql.New(t)
	f.OnQuery = func(context.Context, string, []driver.NamedValue) ([]string, [][]driver.Value, error) {
		return kolomHasilUji(d), nil, nil
	}
	cases := []struct{ per, maks, topn int }{{0, 500, 101}, {100000, 500, 501}, {50, 10, 11}, {5, 0, 6}}
	for _, c := range cases {
		if _, err := d.BacaDaftar(context.Background(), db, FilterPengadaan{PerHalaman: c.per}, c.maks); err != nil {
			t.Fatal(err)
		}
		qs := f.Queries()
		if got := qs[len(qs)-1].Query; !strings.Contains(got, fmt.Sprintf("TOP (%d)", c.topn)) {
			t.Errorf("per=%d maks=%d: want TOP (%d), got %s", c.per, c.maks, c.topn, strings.Join(strings.Fields(got), " "))
		}
	}
}

func TestPengadaanDetail(t *testing.T) {
	d := datasetUji(t, "pengadaan.tender_pengumuman")
	db, f := fakesql.New(t)
	var ada bool
	f.OnQuery = func(context.Context, string, []driver.NamedValue) ([]string, [][]driver.Value, error) {
		if !ada {
			return kolomHasilUji(d), nil, nil
		}
		return kolomHasilUji(d), [][]driver.Value{barisUji(d, "abc", map[string]driver.Value{"kd_tender": "T-9"})}, nil
	}
	// id yang bukan kunci baris sah tidak sampai ke database
	for _, buruk := range []string{"", "a b", "x' OR '1'='1", "a;b", strings.Repeat("a", 256)} {
		if _, err := d.BacaDetail(context.Background(), db, buruk, nil, batasUji); !errors.Is(err, ErrTidakAda) {
			t.Errorf("id %q: err = %v, want ErrTidakAda", buruk, err)
		}
	}
	if n := len(f.Queries()); n != 0 {
		t.Errorf("id tak sah tidak boleh menjalankan query, got %d", n)
	}
	if _, err := d.BacaDetail(context.Background(), db, "abc", []string{"01504"}, batasUji); !errors.Is(err, ErrTidakAda) {
		t.Errorf("baris tidak ada: err = %v", err)
	}
	ada = true
	b, err := d.BacaDetail(context.Background(), db, "abc", []string{"01504"}, batasUji)
	if err != nil || b["id"] != "abc" || b["kd_tender"] != "T-9" {
		t.Fatalf("detail = %v %v", b, err)
	}
	q := f.Queries()
	last := strings.Join(strings.Fields(q[len(q)-1].Query), " ")
	if !strings.Contains(last, "/*inaproc_tender_pengumuman|ue1|01504*/ 1 = 1") || !strings.Contains(last, "[row_key] = @p1") || len(q[len(q)-1].Args) != 1 || q[len(q)-1].Args[0].Value != "abc" {
		t.Errorf("detail harus membatasi UE1 klien dan memakai id sebagai argumen: %s %v", last, q[len(q)-1].Args)
	}
	// klien dibatasi UE1 tetapi tanpa aturan pembatasan: tertutup
	_, _ = d.BacaDetail(context.Background(), db, "abc", []string{"01504"}, nil)
	q = f.Queries()
	if last := q[len(q)-1].Query; !strings.Contains(last, "1 = 0") {
		t.Errorf("tanpa aturan pembatasan detail harus tertutup: %s", last)
	}
}

func TestPengadaanKunciBarisKhususRiwayatKajiUlang(t *testing.T) {
	d := datasetUji(t, "pengadaan.rup_riwayat_kaji_ulang")
	if d.KolomKunci() != "datamart_id" {
		t.Fatalf("kunci = %s", d.KolomKunci())
	}
	db, f := fakesql.New(t)
	f.OnQuery = func(context.Context, string, []driver.NamedValue) ([]string, [][]driver.Value, error) {
		return kolomHasilUji(d), nil, nil
	}
	if _, err := d.BacaDaftar(context.Background(), db, FilterPengadaan{Kursor: "100"}, 500); err != nil {
		t.Fatal(err)
	}
	q := strings.Join(strings.Fields(f.Queries()[0].Query), " ")
	if !strings.HasPrefix(q, "SELECT TOP (101) [datamart_id],") || !strings.Contains(q, "[datamart_id] > @p1") || !strings.HasSuffix(q, "ORDER BY [datamart_id]") {
		t.Errorf("sql = %s", q)
	}
	if strings.Contains(q, "row_key") || strings.Contains(q, "alasan_kajiulang") {
		t.Errorf("sql memuat kolom yang tidak ada/ditutup: %s", q)
	}
}

func TestPengadaanTerakhirSinkron(t *testing.T) {
	d := datasetUji(t, "pengadaan.tender_pengumuman")
	db, f := fakesql.New(t)
	waktu := time.Date(2026, 10, 7, 3, 0, 0, 0, time.UTC)
	f.OnQuery = func(context.Context, string, []driver.NamedValue) ([]string, [][]driver.Value, error) {
		return []string{"t"}, [][]driver.Value{{waktu}}, nil
	}
	got, err := d.TerakhirSinkron(context.Background(), db)
	if err != nil || got == nil || !got.Equal(waktu) {
		t.Errorf("terakhir = %v %v", got, err)
	}
	if q := f.Queries()[0].Query; q != "SELECT MAX([updated_at]) FROM [inaproc_tender_pengumuman]" {
		t.Errorf("sql = %s", q)
	}
	f.OnQuery = func(context.Context, string, []driver.NamedValue) ([]string, [][]driver.Value, error) {
		return []string{"t"}, [][]driver.Value{{nil}}, nil
	}
	if got, err := d.TerakhirSinkron(context.Background(), db); err != nil || got != nil {
		t.Errorf("tabel kosong: %v %v", got, err)
	}
}
