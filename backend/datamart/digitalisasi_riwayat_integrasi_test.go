package datamart

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"pasti-v3-backend/digitalisasi"
)

// Tes integrasi riwayat bulanan (PASTI_UJI_MSSQL_DSN; lihat helper_integrasi_test.go): potret ketujuh tabel DIGITALISASI_* PASTI ke DIGITALISASI_ASET_ALL. Datanya dari alur sungguhan
// (SLDK palsu -> script datamart -> sinkronisasi aplikasi -> tabel PASTI), termasuk kasus tepi fixture (spasi Unicode, teks panjang, koordinat tidak sah, nama penghuni, foto).

func tanggal(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.UTC) }

// simpanRiwayat menjalankan HistorySQL dataset ds di database PASTI dan mengembalikan jumlah baris yang disimpan.
func (l *lingkParitas) simpanRiwayat(t *testing.T, ds digitalisasi.Dataset, periode time.Time, dataPer any) (int64, error) {
	t.Helper()
	var n int64
	err := l.pasti.QueryRowContext(context.Background(), digitalisasi.HistorySQL(ds), sql.Named("periode", periode), sql.Named("data_per", dataPer)).Scan(&n)
	return n, err
}

func (l *lingkParitas) simpanRiwayatSemua(t *testing.T, periode time.Time, dataPer any) map[string]int64 {
	t.Helper()
	out := map[string]int64{}
	for _, ds := range digitalisasi.Datasets {
		n, err := l.simpanRiwayat(t, ds, periode, dataPer)
		if err != nil {
			t.Fatalf("%s: simpan riwayat %s gagal: %v", ds.Key, periode.Format("2006-01-02"), err)
		}
		out[ds.Key] = n
	}
	return out
}

func tipeJSON(c digitalisasi.Column) string {
	switch c.Kind {
	case digitalisasi.Int:
		return "INT"
	case digitalisasi.BigInt:
		return "BIGINT"
	}
	if c.Kind == digitalisasi.Text {
		return "NVARCHAR(MAX)"
	}
	return c.SQLType()
}

// kueriBandingRiwayat: kolom sumber (inti dan JSON) dan padanannya di tabel riwayat untuk satu periode, dalam urutan dan tipe yang sama supaya dapat dibandingkan sebagai himpunan.
// Kolom inti yang tidak dimiliki dataset tidak ikut dibandingkan di sini (diperiksa terpisah: harus NULL).
func kueriBandingRiwayat(ds digitalisasi.Dataset, periode string) (sumber, riwayat string) {
	core := ds.HistoryCoreSources()
	var sc, rc []string
	for _, n := range digitalisasi.HistoryColumnNames() {
		if src, ok := core[n]; ok {
			sc = append(sc, "s."+src)
			rc = append(rc, "a."+n)
		}
	}
	from := "FROM dbo." + digitalisasi.HistoryTable + " AS a"
	if extras := ds.HistoryExtras(); len(extras) > 0 {
		var with []string
		for _, c := range extras {
			sc = append(sc, "s."+c.Name)
			rc = append(rc, "j."+c.Name)
			with = append(with, fmt.Sprintf("%s %s '$.%s'", c.Name, tipeJSON(c), strings.ToLower(c.Name)))
		}
		from += " OUTER APPLY OPENJSON(a.data_tambahan) WITH (" + strings.Join(with, ", ") + ") AS j"
	}
	sumber = "SELECT " + strings.Join(sc, ", ") + " FROM dbo." + ds.Table + " AS s"
	riwayat = "SELECT " + strings.Join(rc, ", ") + " " + from + " WHERE a.periode = '" + periode + "' AND a.jenis_aset = N'" + ds.Key + "'"
	return sumber, riwayat
}

func TestRiwayatBulananMenyimpanPotretTabelPASTI(t *testing.T) {
	l := siapkanParitas(t)
	l.pasangDatamart(t)
	st := l.sinkronPASTI(t)
	semuaStatus(t, st, "sukses|", "dari datamart")

	dataPer := time.Date(2026, 10, 31, 16, 30, 15, 123_000_000, time.UTC)
	okt := tanggal(2026, time.October, 31)
	n := l.simpanRiwayatSemua(t, okt, dataPer)

	// jumlah baris = isi tabel sumber, untuk setiap dataset (dan fixture bermakna: tidak ada dataset kosong)
	for _, ds := range digitalisasi.Datasets {
		sumber := angka(t, l.srv, l.pastiNama, "SELECT COUNT_BIG(*) FROM dbo."+ds.Table)
		if sumber == 0 {
			t.Errorf("%s: tabel sumber kosong, fixture tidak bermakna", ds.Key)
		}
		riwayat := angka(t, l.srv, l.pastiNama, fmt.Sprintf("SELECT COUNT_BIG(*) FROM dbo.%s WHERE periode = '2026-10-31' AND jenis_aset = N'%s'", digitalisasi.HistoryTable, ds.Key))
		if n[ds.Key] != sumber || riwayat != sumber {
			t.Errorf("%s: sumber %d baris, dilaporkan %d, tersimpan %d", ds.Key, sumber, n[ds.Key], riwayat)
		}
	}

	t.Run("isi sama persis dengan sumber (kolom inti dan JSON)", func(t *testing.T) {
		for _, ds := range digitalisasi.Datasets {
			qs, qr := kueriBandingRiwayat(ds, "2026-10-31")
			a, b := baca(t, l.srv, l.pastiNama, qs), baca(t, l.srv, l.pastiNama, qr)
			if len(a) == 0 || len(b) == 0 {
				t.Errorf("%s: perbandingan kosong (sumber %d baris, riwayat %d baris)", ds.Key, len(a), len(b))
			}
			if hA, hB := selisihBaris(a, b); len(hA) > 0 || len(hB) > 0 {
				t.Errorf("%s: riwayat berbeda dari sumber\n  hanya di sumber:\n    %s\n  hanya di riwayat:\n    %s", ds.Key, contohBaris(hA), contohBaris(hB))
			}
		}
	})

	t.Run("kolom inti yang tidak dimiliki dataset NULL, meta terisi", func(t *testing.T) {
		for _, ds := range digitalisasi.Datasets {
			core := ds.HistoryCoreSources()
			for _, kol := range digitalisasi.HistoryColumnNames() {
				if _, ada := core[kol]; ada {
					continue
				}
				if n := angka(t, l.srv, l.pastiNama, fmt.Sprintf("SELECT COUNT_BIG(*) FROM dbo.%s WHERE jenis_aset = N'%s' AND %s IS NOT NULL", digitalisasi.HistoryTable, ds.Key, kol)); n != 0 {
					t.Errorf("%s: kolom %s tidak dimiliki dataset tetapi berisi %d baris", ds.Key, kol, n)
				}
			}
		}
		if n := angka(t, l.srv, l.pastiNama, "SELECT COUNT_BIG(*) FROM dbo."+digitalisasi.HistoryTable+" WHERE diambil_pada IS NULL OR data_per <> '2026-10-31T16:30:15.123'"); n != 0 {
			t.Errorf("%d baris dengan data_per atau diambil_pada salah", n)
		}
		if n := angka(t, l.srv, l.pastiNama, "SELECT COUNT_BIG(*) FROM dbo."+digitalisasi.HistoryTable+" WHERE data_tambahan IS NOT NULL AND ISJSON(data_tambahan) <> 1"); n != 0 {
			t.Errorf("%d baris dengan JSON tidak sah", n)
		}
	})

	t.Run("data pribadi dan foto tidak ikut", func(t *testing.T) {
		const rn = "dbo.DIGITALISASI_RUMAH_NEGARA"
		if n := angka(t, l.srv, l.pastiNama, "SELECT COUNT_BIG(*) FROM "+rn+" WHERE Nama_Penghuni IS NOT NULL AND LEN(Nama_Penghuni) > 0"); n == 0 {
			t.Fatal("fixture tidak punya nama penghuni: tes tidak bermakna")
		}
		// kontrol positif: pola pencarian yang sama memang menemukan nama itu bila ada
		if n := angka(t, l.srv, l.pastiNama, "SELECT COUNT_BIG(*) FROM "+rn+" AS r WHERE r.Nama_Penghuni IS NOT NULL AND LEN(r.Nama_Penghuni) > 0 AND CONCAT(N'x|', r.Nama_Penghuni, N'|y') LIKE N'%' + r.Nama_Penghuni + N'%'"); n == 0 {
			t.Fatal("kontrol positif gagal: pola LIKE tidak menemukan nama penghuni")
		}
		bocor := angka(t, l.srv, l.pastiNama, fmt.Sprintf(`SELECT COUNT_BIG(*) FROM dbo.%s AS a
WHERE EXISTS (SELECT 1 FROM %s AS r WHERE r.Nama_Penghuni IS NOT NULL AND LEN(r.Nama_Penghuni) > 0
  AND CONCAT(a.nama_satker, N'|', a.uraian, N'|', a.alamat, N'|', a.status_hukum, N'|', a.status_penghuni, N'|', a.kondisi, N'|', a.data_tambahan) LIKE N'%%' + r.Nama_Penghuni + N'%%')`, digitalisasi.HistoryTable, rn))
		if bocor != 0 {
			t.Errorf("%d baris riwayat memuat nama penghuni", bocor)
		}
		// tautan foto: nilai kolom Foto* di tabel sumber tidak boleh muncul di riwayat
		var fotoAda int64
		for _, ds := range digitalisasi.Datasets {
			for _, c := range ds.Columns {
				if !strings.HasPrefix(strings.ToLower(c.Name), "foto") {
					continue
				}
				fotoAda += angka(t, l.srv, l.pastiNama, fmt.Sprintf("SELECT COUNT_BIG(*) FROM dbo.%s WHERE %s IS NOT NULL AND LEN(%s) > 0", ds.Table, c.Name, c.Name))
				bocor := angka(t, l.srv, l.pastiNama, fmt.Sprintf(`SELECT COUNT_BIG(*) FROM dbo.%s AS a WHERE EXISTS (SELECT 1 FROM dbo.%s AS s
WHERE s.%s IS NOT NULL AND LEN(s.%s) > 0 AND a.jenis_aset = N'%s' AND a.data_tambahan LIKE N'%%' + s.%s + N'%%')`, digitalisasi.HistoryTable, ds.Table, c.Name, c.Name, ds.Key, c.Name))
				if bocor != 0 {
					t.Errorf("%s.%s: %d baris riwayat memuat tautan foto", ds.Key, c.Name, bocor)
				}
			}
		}
		if fotoAda == 0 {
			t.Error("fixture tidak punya foto: tes tidak bermakna")
		}
		if n := angka(t, l.srv, l.pastiNama, "SELECT COUNT_BIG(*) FROM dbo."+digitalisasi.HistoryTable+" WHERE data_tambahan LIKE N'%foto%' OR data_tambahan LIKE N'%nama_penghuni%' OR data_tambahan LIKE N'%nama_pengguna%'"); n != 0 {
			t.Errorf("%d baris dengan kunci terlarang di JSON", n)
		}
	})

	t.Run("aman diulang dan periode lain tidak tersentuh", func(t *testing.T) {
		sebelum := l.hitungRiwayat(t)
		ulang := l.simpanRiwayatSemua(t, okt, dataPer)
		for k, v := range n {
			if ulang[k] != v {
				t.Errorf("%s: ulangan menyimpan %d baris, semula %d", k, ulang[k], v)
			}
		}
		if sesudah := l.hitungRiwayat(t); sesudah != sebelum {
			t.Errorf("mengulang potret periode yang sama mengubah jumlah baris: %v -> %v", sebelum, sesudah)
		}
	})

	t.Run("deret waktu: potret lama tetap, potret baru memuat perubahan", func(t *testing.T) {
		luasOkt := l.totalLuasTanah(t, "2026-10-31")
		// data berubah di bulan berikutnya: setiap luas tanah bertambah 10 m2
		if err := ekse(t, l.srv, l.pastiNama, "UPDATE dbo.DIGITALISASI_TANAH SET Luas_Tanah = Luas_Tanah + 10 WHERE Luas_Tanah IS NOT NULL"); err != nil {
			t.Fatal(err)
		}
		berubah := angka(t, l.srv, l.pastiNama, "SELECT COUNT_BIG(*) FROM dbo.DIGITALISASI_TANAH WHERE Luas_Tanah IS NOT NULL")
		l.simpanRiwayatSemua(t, tanggal(2026, time.November, 30), time.Date(2026, 11, 30, 16, 0, 0, 0, time.UTC))

		if got := l.totalLuasTanah(t, "2026-10-31"); got != luasOkt {
			t.Errorf("potret Oktober berubah: %s -> %s", luasOkt, got)
		}
		selisih := angka(t, l.srv, l.pastiNama, fmt.Sprintf("SELECT CAST(%s - %s AS BIGINT)", l.totalLuasTanah(t, "2026-11-30"), luasOkt))
		if selisih != berubah*10 {
			t.Errorf("selisih total luas Nov-Okt = %d, seharusnya %d (%d tanah x 10)", selisih, berubah*10, berubah)
		}
		// dua periode hadir berdampingan untuk tiap jenis aset
		for _, ds := range digitalisasi.Datasets {
			if n := angka(t, l.srv, l.pastiNama, fmt.Sprintf("SELECT COUNT(DISTINCT periode) FROM dbo.%s WHERE jenis_aset = N'%s'", digitalisasi.HistoryTable, ds.Key)); n != 2 {
				t.Errorf("%s: %d periode, seharusnya 2", ds.Key, n)
			}
		}
	})

	// Contoh query pada docs/digitalisasi-aset.md (bagian Riwayat bulanan) dijalankan sungguhan: sintaks dan nama kolomnya harus benar.
	t.Run("contoh query di dokumentasi berjalan", func(t *testing.T) {
		doc := strings.ReplaceAll(bacaBerkas(t, "../../docs/digitalisasi-aset.md"), "\r\n", "\n")
		i := strings.Index(doc, "## Riwayat bulanan")
		if i < 0 {
			t.Fatal("bagian Riwayat bulanan tidak ada di dokumen")
		}
		bagian := doc[i+len("## Riwayat bulanan"):]
		if j := strings.Index(bagian, "\n## "); j >= 0 {
			bagian = bagian[:j]
		}
		blok := regexp.MustCompile("(?s)```sql\n(.*?)```").FindAllStringSubmatch(bagian, -1)
		if len(blok) < 4 {
			t.Fatalf("hanya %d contoh query di dokumen", len(blok))
		}
		for n, b := range blok {
			q := strings.TrimSpace(b[1])
			if !regexp.MustCompile(`(?i)^(SELECT|WITH)`).MatchString(q) {
				t.Errorf("contoh %d bukan query baca: %.40q", n+1, q)
				continue
			}
			if r := baca(t, l.srv, l.pastiNama, strings.TrimSuffix(q, ";")); n == 0 && len(r) == 0 {
				t.Errorf("contoh 1 tidak menghasilkan baris")
			}
		}
	})

	t.Run("tabel menolak periode bukan akhir bulan dan JSON tidak sah", func(t *testing.T) {
		before := l.hitungRiwayat(t)
		tanah, _ := digitalisasi.ByKey("tanah")
		if _, err := l.simpanRiwayat(t, tanah, tanggal(2026, time.October, 15), nil); err == nil || !strings.Contains(err.Error(), "ck_digitalisasi_aset_all_periode") {
			t.Errorf("periode 2026-10-15 harus ditolak oleh CHECK periode, err = %v", err)
		}
		if err := ekse(t, l.srv, l.pastiNama, "INSERT INTO dbo."+digitalisasi.HistoryTable+" (periode, jenis_aset, data_tambahan) VALUES ('2026-12-31', N'tanah', N'bukan json')"); err == nil || !strings.Contains(err.Error(), "ck_digitalisasi_aset_all_json") {
			t.Errorf("JSON tidak sah harus ditolak oleh CHECK, err = %v", err)
		}
		if after := l.hitungRiwayat(t); after != before {
			t.Errorf("penolakan mengubah isi tabel: %v -> %v", before, after)
		}
		// koneksi tidak tertinggal dalam transaksi yang terbuka: potret berikutnya tetap berhasil
		if _, err := l.simpanRiwayat(t, tanah, tanggal(2026, time.November, 30), nil); err != nil {
			t.Errorf("potret sesudah penolakan gagal: %v", err)
		}
	})
}

// hitungRiwayat: jumlah baris riwayat per "periode|jenis_aset".
func (l *lingkParitas) hitungRiwayat(t *testing.T) string {
	t.Helper()
	var out []string
	for _, r := range baca(t, l.srv, l.pastiNama, "SELECT CONVERT(CHAR(10), periode, 23) + N'|' + jenis_aset, COUNT_BIG(*) FROM dbo."+digitalisasi.HistoryTable+" GROUP BY periode, jenis_aset") {
		out = append(out, r[0]+"="+r[1])
	}
	return strings.Join(out, ",")
}

func (l *lingkParitas) totalLuasTanah(t *testing.T, periode string) string {
	t.Helper()
	r := baca(t, l.srv, l.pastiNama, fmt.Sprintf("SELECT CAST(SUM(luas) AS NVARCHAR(40)) FROM dbo.%s WHERE jenis_aset = N'tanah' AND periode = '%s'", digitalisasi.HistoryTable, periode))
	if len(r) != 1 || r[0][0] == "NULL" {
		t.Fatalf("total luas tanah periode %s kosong: %v", periode, r)
	}
	return r[0][0]
}
