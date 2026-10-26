package datamart

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"pasti-v3-backend/digitalisasi"
	"pasti-v3-backend/digitalisasi/acuan"
)

// Tes integrasi paritas Digitalisasi Aset (PASTI_UJI_MSSQL_DSN; lihat helper_integrasi_test.go). Pembuktiannya dua lapis:
//  1. tabel DIGITALISASI_* yang dibentuk script datamart dari SLDK SAMA PERSIS dengan tabel acuan (paket acuan: ketujuh query asli + konversi acuan, ditulis ke tabel berbentuk PASTI):
//     struktur, indeks, dan isi setiap kolom, termasuk kasus tepi (spasi Unicode, teks melebihi panjang kolom, pasangan pengganti di batas potong, koma desimal, koordinat tidak
//     sah, kolom manual yang dipertahankan);
//  2. tabel PASTI hasil sinkronisasi aplikasi yang SUNGGUHAN (digitalisasi.Manager dan Sync membaca datamart, tidak pernah SLDK) sama persis dengan tabel acuan itu.
//
// Lima database sementara: SLDK palsu, datamart (hasil script), acuan (migrasi 020, 063, 064), dan PASTI (migrasi yang sama; diisi aplikasi dari datamart).

type lingkParitas struct {
	srv                                      *sql.DB
	sldkNama, acuanNama, martNama, pastiNama string
	sldk, acuan, mart, pasti                 *sql.DB
}

// migrasiPASTI: migrasi yang membentuk tabel DIGITALISASI_* dan riwayat sinkronisasinya.
var migrasiPASTI = []string{
	"../migrations/020_create_digitalisasi.sql",
	"../migrations/063_digitalisasi_sync_log_sumber.sql",
	"../migrations/064_digitalisasi_sync_log_data_per.sql",
}

// siapkanParitas membuat database-database di atas. Fixture SLDK = sldk_palsu.sql + sldk_palsu_tepi.sql, lalu penyesuaian opsional (SQL) pada SLDK sebelum script dipasang.
func siapkanParitas(t *testing.T, sesuaikanSLDK ...string) *lingkParitas {
	t.Helper()
	srv := bukaServerUji(t)
	l := &lingkParitas{srv: srv}
	l.sldkNama, l.acuanNama, l.martNama, l.pastiNama = buatDBUji(t, srv, "dgsldk"), buatDBUji(t, srv, "dgacuan"), buatDBUji(t, srv, "dgmart"), buatDBUji(t, srv, "dgpasti")
	for _, f := range []string{"testdata/sldk_palsu.sql", "testdata/sldk_palsu_tepi.sql"} {
		if err := jalankanBerkas(t, srv, l.sldkNama, bacaBerkas(t, f), nil); err != nil {
			t.Fatalf("fixture %s gagal dimuat: %v", f, err)
		}
	}
	for _, s := range sesuaikanSLDK {
		if err := jalankanBerkas(t, srv, l.sldkNama, s, nil); err != nil {
			t.Fatalf("penyesuaian SLDK gagal: %v", err)
		}
	}
	for _, db := range []string{l.acuanNama, l.pastiNama} {
		for _, f := range migrasiPASTI {
			if err := jalankanBerkas(t, srv, db, bacaBerkas(t, f), nil); err != nil {
				t.Fatalf("migrasi %s gagal: %v", f, err)
			}
		}
	}
	l.sldk, l.acuan, l.mart, l.pasti = bukaDB(t, l.sldkNama, ""), bukaDB(t, l.acuanNama, ""), bukaDB(t, l.martNama, ""), bukaDB(t, l.pastiNama, "")
	return l
}

// skripDigitalisasi: isi script dengan @Sumber menunjuk SLDK palsu.
func (l *lingkParitas) skripDigitalisasi(t *testing.T) string {
	t.Helper()
	asli := "N'" + SumberBawaan + "'"
	s := bacaBerkas(t, BerkasDigitalisasi)
	if !strings.Contains(s, asli) {
		t.Fatalf("script tidak memuat %s", asli)
	}
	return strings.Replace(s, asli, "N'"+kutipDB(l.sldkNama)+"'", 1)
}

func (l *lingkParitas) pasangDatamart(t *testing.T) {
	t.Helper()
	if err := jalankanBerkas(t, l.srv, l.martNama, l.skripDigitalisasi(t), nil); err != nil {
		t.Fatalf("script datamart gagal dijalankan: %v", err)
	}
}

func (l *lingkParitas) segarkanDatamart(t *testing.T) {
	t.Helper()
	if err := ekse(t, l.srv, l.martNama, "EXEC dbo.datamart_digitalisasi_segarkan;"); err != nil {
		t.Fatalf("penyegaran datamart gagal: %v", err)
	}
}

// sinkronAcuan membentuk tabel acuan: paket acuan membaca SLDK palsu dengan ketujuh query asli dan konversi acuan, lalu menulisnya ke database acuan (berbentuk PASTI).
func (l *lingkParitas) sinkronAcuan(t *testing.T, runID int64) {
	t.Helper()
	for _, ds := range digitalisasi.Datasets {
		if _, err := acuan.Sinkron(context.Background(), l.sldk, l.acuan, ds, runID); err != nil {
			t.Fatalf("acuan %s: %v", ds.Key, err)
		}
	}
}

// sinkronPASTI menjalankan pengelola sinkronisasi aplikasi YANG SUNGGUHAN (digitalisasi.Manager) dengan sumber datamart, ke database PASTI, lalu menunggunya selesai. Mengembalikan
// status tiap dataset (dataset -> "status|pesan").
func (l *lingkParitas) sinkronPASTI(t *testing.T) map[string]string {
	t.Helper()
	return l.sinkronPASTIDari(t, l.mart)
}

// selisihBaris: baris yang hanya ada di a dan yang hanya ada di b (multiset).
func selisihBaris(a, b [][]string) (hanyaA, hanyaB [][]string) {
	hitung := map[string]int{}
	for _, r := range b {
		hitung[strings.Join(r, "\x00")]++
	}
	for _, r := range a {
		k := strings.Join(r, "\x00")
		if hitung[k] > 0 {
			hitung[k]--
		} else {
			hanyaA = append(hanyaA, r)
		}
	}
	sisa := map[string]int{}
	for _, r := range a {
		sisa[strings.Join(r, "\x00")]++
	}
	for _, r := range b {
		k := strings.Join(r, "\x00")
		if sisa[k] > 0 {
			sisa[k]--
		} else {
			hanyaB = append(hanyaB, r)
		}
	}
	return hanyaA, hanyaB
}

func contohBaris(r [][]string) string {
	var out []string
	for i, b := range r {
		if i == 5 {
			out = append(out, fmt.Sprintf("... (+%d)", len(r)-5))
			break
		}
		out = append(out, fmt.Sprintf("%q", b))
	}
	return strings.Join(out, "\n    ")
}

// bandingkanTabel: struktur (kolom, tipe, indeks) dan isi satu tabel DIGITALISASI_* di datamart harus sama dengan di tabel acuan.
// urut: bandingkan juga urutan baris menurut id (dataset yang querynya ber-ORDER BY).
func (l *lingkParitas) bandingkanTabel(t *testing.T, ds digitalisasi.Dataset, urut bool) {
	t.Helper()
	l.bandingkanDB(t, ds, urut, "acuan", l.acuanNama, "datamart", l.martNama)
}

// bandingkanDB membandingkan satu tabel DIGITALISASI_* antara dua database: namaA (diberi label labelA) dan namaB.
func (l *lingkParitas) bandingkanDB(t *testing.T, ds digitalisasi.Dataset, urut bool, labelA, namaA, labelB, namaB string) {
	t.Helper()
	const kolomSQL = `SELECT c.column_id, c.name, ty.name, c.max_length, c.[precision], c.scale, c.is_nullable, c.is_identity, ISNULL(OBJECT_DEFINITION(c.default_object_id), N'')
FROM sys.columns AS c JOIN sys.types AS ty ON ty.user_type_id = c.user_type_id WHERE c.object_id = OBJECT_ID(N'dbo.%s') ORDER BY c.column_id`
	const indeksSQL = `SELECT i.type_desc, i.is_unique, i.is_primary_key,
  (SELECT STRING_AGG(c.name + CASE WHEN ic.is_descending_key = 1 THEN N' DESC' ELSE N'' END, N',') WITHIN GROUP (ORDER BY ic.key_ordinal)
   FROM sys.index_columns AS ic JOIN sys.columns AS c ON c.object_id = ic.object_id AND c.column_id = ic.column_id
   WHERE ic.object_id = i.object_id AND ic.index_id = i.index_id AND ic.is_included_column = 0)
FROM sys.indexes AS i WHERE i.object_id = OBJECT_ID(N'dbo.%s') AND i.type > 0`
	for _, q := range []string{kolomSQL, indeksSQL} {
		a, b := baca(t, l.srv, namaA, fmt.Sprintf(q, ds.Table)), baca(t, l.srv, namaB, fmt.Sprintf(q, ds.Table))
		if hA, hB := selisihBaris(a, b); len(hA) > 0 || len(hB) > 0 {
			t.Errorf("%s: struktur tabel %s dan %s berbeda\n  hanya di %s:\n    %s\n  hanya di %s:\n    %s", ds.Table, labelA, labelB, labelA, contohBaris(hA), labelB, contohBaris(hB))
		}
	}

	q := "SELECT " + strings.Join(kolomData(ds), ", ") + " FROM dbo." + ds.Table
	var a, b [][]string
	if urut {
		a, b = bacaUrut(t, l.srv, namaA, q+" ORDER BY id"), bacaUrut(t, l.srv, namaB, q+" ORDER BY id")
	} else {
		a, b = baca(t, l.srv, namaA, q), baca(t, l.srv, namaB, q)
	}
	if len(a) == 0 {
		t.Errorf("%s: %s tidak menghasilkan baris (fixture tidak bermakna)", ds.Table, labelA)
	}
	if len(a) != len(b) {
		t.Errorf("%s: %s %d baris, %s %d baris", ds.Table, labelA, len(a), labelB, len(b))
	}
	if hA, hB := selisihBaris(a, b); len(hA) > 0 || len(hB) > 0 {
		t.Errorf("%s: isi tabel berbeda\n  hanya di %s:\n    %s\n  hanya di %s:\n    %s", ds.Table, labelA, contohBaris(hA), labelB, contohBaris(hB))
	} else if urut {
		for i := range a {
			if strings.Join(a[i], "\x00") != strings.Join(b[i], "\x00") {
				t.Errorf("%s: urutan baris (menurut id) berbeda mulai baris ke-%d\n  %s: %q\n  %s: %q", ds.Table, i+1, labelA, a[i], labelB, b[i])
				break
			}
		}
	}
}

func (l *lingkParitas) bandingkanSemua(t *testing.T) {
	t.Helper()
	for _, ds := range digitalisasi.Datasets {
		_, berurut := urutanTambahan[ds.Key]
		l.bandingkanTabel(t, ds, berurut)
	}
}

// bandingkanPASTIDenganAcuan: tabel PASTI hasil sinkronisasi aplikasi dari datamart harus sama persis dengan tabel acuan (yang dibentuk dari SLDK).
func (l *lingkParitas) bandingkanPASTIDenganAcuan(t *testing.T) {
	t.Helper()
	for _, ds := range digitalisasi.Datasets {
		_, berurut := urutanTambahan[ds.Key]
		l.bandingkanDB(t, ds, berurut, "acuan", l.acuanNama, "PASTI", l.pastiNama)
	}
}

func TestParitasDatamartDenganAplikasi(t *testing.T) {
	l := siapkanParitas(t)
	l.sinkronAcuan(t, 1)
	l.pasangDatamart(t)

	t.Run("semua tabel identik", func(t *testing.T) { l.bandingkanSemua(t) })

	// Kasus tepi benar-benar sampai ke hasil dan nilainya sesuai harapan manusia (bukan hanya "sama-sama salah").
	t.Run("kasus tepi bermakna", func(t *testing.T) {
		for _, c := range []struct{ nama, db, query, want string }{
			{"spasi Unicode dipangkas", l.martNama, "SELECT Alamat_tanah FROM dbo.DIGITALISASI_TANAH WHERE kode_register_tanah = N'REG-T-110'", "Jl. Tepi"},
			{"RTRW hanya spasi -> NULL", l.martNama, "SELECT RTRW_tanah FROM dbo.DIGITALISASI_TANAH WHERE kode_register_tanah = N'REG-T-110'", "NULL"},
			{"uraian dipotong di batas pasangan pengganti", l.martNama, "SELECT DATALENGTH(Uraian_tanah) FROM dbo.DIGITALISASI_TANAH WHERE kode_register_tanah = N'REG-T-110'", "998"},
			{"kondisi 150 huruf dipotong ke 100", l.martNama, "SELECT DATALENGTH(Kondisi_Tanah) FROM dbo.DIGITALISASI_TANAH WHERE kode_register_tanah = N'REG-T-121'", "200"},
			{"nilai besar utuh", l.martNama, "SELECT Nilai_Tanah FROM dbo.DIGITALISASI_TANAH WHERE kode_register_tanah = N'REG-T-121'", "9999999999999.99"},
			{"alamat NVARCHAR(MAX) tidak dipotong", l.martNama, "SELECT LEN(Alamat_tanah) FROM dbo.DIGITALISASI_TANAH WHERE kode_register_tanah = N'REG-T-122'", "700"},
			{"U+200B bukan spasi (Go)", l.martNama, "SELECT UNICODE(LEFT(Uraian_tanah, 1)) FROM dbo.DIGITALISASI_TANAH WHERE kode_register_tanah = N'REG-T-122'", "8203"},
			{"satker bernama spasi di tepi dipangkas", l.martNama, "SELECT Nama_Satker FROM dbo.DIGITALISASI_TANAH WHERE kode_register_tanah = N'REG-T-122'", "Satker Spasi"},
			{"koma desimal", l.martNama, "SELECT CONCAT(Latitude, N'|', Longitude) FROM dbo.DIGITALISASI_TANAH WHERE kode_register_tanah = N'REG-T-110'", "-6.2000000|106.8000000"},
			{"lintang di luar rentang -> keduanya NULL", l.martNama, "SELECT CONCAT(ISNULL(CAST(Latitude AS NVARCHAR(20)), N'-'), N'|', ISNULL(CAST(Longitude AS NVARCHAR(20)), N'-')) FROM dbo.DIGITALISASI_TANAH WHERE kode_register_tanah = N'REG-T-111'", "-|-"},
			{"(0,0) -> NULL", l.martNama, "SELECT CONCAT(ISNULL(CAST(Latitude AS NVARCHAR(20)), N'-'), N'|', ISNULL(CAST(Longitude AS NVARCHAR(20)), N'-')) FROM dbo.DIGITALISASI_TANAH WHERE kode_register_tanah = N'REG-T-112'", "-|-"},
			{"tak berpasangan -> NULL", l.martNama, "SELECT CONCAT(ISNULL(CAST(Latitude AS NVARCHAR(20)), N'-'), N'|', ISNULL(CAST(Longitude AS NVARCHAR(20)), N'-')) FROM dbo.DIGITALISASI_TANAH WHERE kode_register_tanah = N'REG-T-113'", "-|-"},
			{"eksponen", l.martNama, "SELECT CONCAT(Latitude, N'|', Longitude) FROM dbo.DIGITALISASI_TANAH WHERE kode_register_tanah = N'REG-T-115'", "10.0000000|20.0000000"},
			{"dibulatkan ke 7 desimal", l.martNama, "SELECT CONCAT(Latitude, N'|', Longitude) FROM dbo.DIGITALISASI_TANAH WHERE kode_register_tanah = N'REG-T-116'", "-6.1234568|106.1234568"},
			{"tepat di batas rentang", l.martNama, "SELECT CONCAT(Latitude, N'|', Longitude) FROM dbo.DIGITALISASI_TANAH WHERE kode_register_tanah = N'REG-T-118'", "90.0000000|180.0000000"},
			{"eksponen d ditolak (Go)", l.martNama, "SELECT CONCAT(ISNULL(CAST(Latitude AS NVARCHAR(20)), N'-'), N'|', ISNULL(CAST(Longitude AS NVARCHAR(20)), N'-')) FROM dbo.DIGITALISASI_TANAH WHERE kode_register_tanah = N'REG-T-123'", "-|-"},
			{"tanda plus dan koma", l.martNama, "SELECT CONCAT(Latitude, N'|', Longitude) FROM dbo.DIGITALISASI_TANAH WHERE kode_register_tanah = N'REG-T-125'", "7.5000000|110.2500000"},
			{"urutan id rusunara mengikuti ORDER BY (satker, NUP numerik)", l.martNama, "SELECT STRING_AGG(NUP_Rusun, N'|') WITHIN GROUP (ORDER BY id) FROM dbo.DIGITALISASI_RUSUNARA", "1|3|7|2|11"},
			{"urutan id mess mengikuti ORDER BY (satker, kode, NUP numerik)", l.martNama, "SELECT STRING_AGG(NUP_mess, N'|') WITHIN GROUP (ORDER BY id) FROM dbo.DIGITALISASI_MESS_RUMAH_NEGARA", "1|2|9|4"},
			{"gedung kantor utama tepi ada", l.martNama, "SELECT COUNT(*) FROM dbo.DIGITALISASI_GEDUNG_KANTOR_UTAMA WHERE Alamat_bangunan = N'Jl. Tepi'", "1"},
			{"nama penghuni dipangkas", l.martNama, "SELECT Nama_Penghuni FROM dbo.DIGITALISASI_RUMAH_NEGARA WHERE Kode_Register_RN = N'REG-R-310'", "Budi Tepi"},
		} {
			r := baca(t, l.srv, c.db, c.query)
			if len(r) != 1 || r[0][0] != c.want {
				t.Errorf("%s: %v, seharusnya %q", c.nama, r, c.want)
			}
		}
	})

	// Penyegaran ulang (hidup berjalan): kolom manual dipertahankan per Kode_Satker dan hasilnya tetap identik dengan sinkronisasi ulang aplikasi.
	t.Run("kolom manual dipertahankan dan penyegaran ulang tetap identik", func(t *testing.T) {
		const ubah = "UPDATE dbo.DIGITALISASI_SATKER SET Status_Gedung_Kantor = N'Milik sendiri', Foto = N'foto-manual.jpg' WHERE Kode_Satker IN (SELECT TOP (2) Kode_Satker FROM dbo.DIGITALISASI_SATKER ORDER BY Kode_Satker)"
		for _, db := range []string{l.acuanNama, l.martNama} {
			if err := ekse(t, l.srv, db, ubah); err != nil {
				t.Fatal(err)
			}
		}
		if n := angka(t, l.srv, l.martNama, "SELECT COUNT(*) FROM dbo.DIGITALISASI_SATKER WHERE Status_Gedung_Kantor = N'Milik sendiri'"); n != 2 {
			t.Fatalf("pengubahan manual: %d baris, seharusnya 2", n)
		}
		l.sinkronAcuan(t, 2)
		l.segarkanDatamart(t)
		if n := angka(t, l.srv, l.martNama, "SELECT COUNT(*) FROM dbo.DIGITALISASI_SATKER WHERE Status_Gedung_Kantor = N'Milik sendiri' AND Foto = N'foto-manual.jpg'"); n != 2 {
			t.Errorf("kolom manual datamart hilang saat penyegaran: %d baris, seharusnya 2", n)
		}
		l.bandingkanSemua(t)
		// id_sinkron mengikuti nomor proses masing-masing
		if n := angka(t, l.srv, l.martNama, "SELECT COUNT(*) FROM dbo.DIGITALISASI_TANAH WHERE id_sinkron <> (SELECT MAX(id) FROM dbo.datamart_digitalisasi_run WHERE status = N'sukses')"); n != 0 {
			t.Errorf("%d baris tanah tidak memuat id_sinkron proses terakhir", n)
		}
	})
}

// Sumber bertipe FLOAT: pembacaan angka oleh aplikasi (float64) dan oleh datamart (CONVERT ... 3) harus tetap menghasilkan nilai yang sama.
func TestParitasSumberBertipeFloat(t *testing.T) {
	l := siapkanParitas(t, `
UPDATE DJKN.SIMAN2_M_ASET SET gps_latitude = NULL, gps_longitude = NULL WHERE TRY_CAST(gps_latitude AS FLOAT) IS NULL OR TRY_CAST(gps_longitude AS FLOAT) IS NULL;
ALTER TABLE DJKN.SIMAN2_M_ASET ALTER COLUMN luas FLOAT NULL;
ALTER TABLE DJKN.SIMAN2_M_ASET ALTER COLUMN luas_tnhb REAL NULL;
ALTER TABLE DJKN.SIMAN2_M_ASET ALTER COLUMN rph_aset FLOAT NULL;
ALTER TABLE DJKN.SIMAN2_M_ASET ALTER COLUMN gps_latitude FLOAT NULL;
ALTER TABLE DJKN.SIMAN2_M_ASET ALTER COLUMN gps_longitude REAL NULL;
GO
UPDATE DJKN.SIMAN2_M_ASET SET luas = 2.675, luas_tnhb = 0.1, rph_aset = 1234567.005, gps_latitude = -6.123456789012345, gps_longitude = 106.12346 WHERE id_aset = 110;
UPDATE DJKN.SIMAN2_M_ASET SET luas = 1.00005, luas_tnhb = 1E-7, rph_aset = 99999999999.995, gps_latitude = 5E-8, gps_longitude = 1E-8 WHERE id_aset = 111;
UPDATE DJKN.SIMAN2_M_ASET SET luas = 0.30000000000000004, luas_tnhb = 3.4E+12, rph_aset = 1E+20, gps_latitude = 89.99999999999999, gps_longitude = 179.5 WHERE id_aset = 112;
`)
	l.sinkronAcuan(t, 1)
	l.pasangDatamart(t)
	l.bandingkanSemua(t)
}

// ---------------------------------------------------------------------------------------------------------------------------------------------------------------------
// Baterai konversi nilai: fungsi dan ekspresi datamart (yang dibangkitkan generator, bukan salinan) dibandingkan dengan acuan.KonversiNilai pada ribuan masukan.

var angkaTetap = []string{
	"", " ", "0", "-0", "+0", "1", "-1", "+1", "1.5", "1,5", "-6,2", "  -6,2  ", "1,234.5", "1.234,5", "1,2,3", "1e1", "1E1", "1e+1", "1e-1", "1d5", "1D5", ".5", "5.", ".", "-", "+", "-.", "+.",
	"e", "1e", "e1", ".e1", "1.e1", "1e1.5", "1e+", "0x10", "0x1p3", "1_000", "1 000", "abc", "NaN", "nan", "Inf", "-Inf", "Infinity", "inf", "1e400", "-1e400", "1e-400", "00012", "012.50",
	"123456789012345", "1234567890123456", "12345678901234567", "123456789012345678", "0.1", "0.30000000000000004", "2.675", "1.005", "123456789012.34", "9999999999999.99", "12345678901234.56",
	"0.00005", "0.00004999999999999999", "-0.00005", "2147483647", "2147483648", "-2147483648", "\u0663", "\uff11\uff12\uff13", "\uff15", "\uff11.\uff15", "$5", "5$", "1\u00a05", "\u00a05\u2003", "\t5\r\n", "5\x00", "- 5", "5-", "--5",
	"++5", "+-5", "1..5", "1.5.5", "1,5,5", ",", ",5", "5,", "1,,5", "\u221e", "1e5e5", "\u0661\u0662\u0663", "90", "-90", "90.0000001", "180", "-180", "180.0000001", "91", "-91", "181", "0.0000001", "0.00000005", "0.00000004999",
	"1e-7", "5e-8", "-5e-8", "4.4e-8", "106,8", "106.8", "-6.2", "  106.123456789  ", "1\u2003", "\u3000-6,2\u2028",
}

// acakAngka: n masukan angka acak dengan beragam bentuk (tanda, titik/koma desimal, eksponen, spasi, panjang digit sampai maksDigit).
func acakAngka(r *rand.Rand, n, maksDigit int) []string {
	out := make([]string, 0, n)
	for len(out) < n {
		d := 1 + r.Intn(maksDigit)
		var digit strings.Builder
		for i := 0; i < d; i++ {
			digit.WriteByte(byte('0' + r.Intn(10)))
		}
		s := digit.String()
		if r.Intn(3) > 0 && d > 1 {
			p := 1 + r.Intn(d-1)
			sep := "."
			if r.Intn(4) == 0 {
				sep = ","
			}
			s = s[:p] + sep + s[p:]
		}
		switch r.Intn(4) {
		case 0:
			s = "-" + s
		case 1:
			s = "+" + s
		}
		if r.Intn(6) == 0 {
			e := []string{"e", "E"}[r.Intn(2)]
			s += e + []string{"", "+", "-"}[r.Intn(3)] + strconv.Itoa(r.Intn(12))
		}
		if r.Intn(8) == 0 {
			s = " \t" + s + "\u00a0 "
		}
		out = append(out, s)
	}
	return out
}

func acakTeks(r *rand.Rand, n int) []string {
	fragmen := []string{"a", "B", "1", " ", " ", "\t", "\r", "\n", "\v", "\f", "\u00a0", "\u0085", "\u1680", "\u2000", "\u2003", "\u200a", "\u2028", "\u2029", "\u202f", "\u205f", "\u3000", "\u200b", "\ufeff", "\u180e", "\u2060", "\u00e9", "\u65e5\u672c", "\U0001f600", "\U0001f600\U0001f601", ",", ".", "-", "x"}
	out := make([]string, 0, n)
	for len(out) < n {
		var b strings.Builder
		for i, k := 0, r.Intn(40); i < k; i++ {
			b.WriteString(fragmen[r.Intn(len(fragmen))])
		}
		out = append(out, b.String())
	}
	return out
}

// kolomBaterai: tabel baterai di database datamart: id, a, b bertipe tipe (b hanya dipakai baterai koordinat).
func (l *lingkParitas) kolomBaterai(t *testing.T, tipe string, kolom ...string) {
	t.Helper()
	var defs []string
	for _, k := range kolom {
		defs = append(defs, kutipDB(k)+" "+tipe+" NULL")
	}
	if err := ekse(t, l.srv, l.martNama, "DROP TABLE IF EXISTS dbo.bat_uji; CREATE TABLE dbo.bat_uji (id INT NOT NULL PRIMARY KEY, "+strings.Join(defs, ", ")+");"); err != nil {
		t.Fatal(err)
	}
}

// isiBaterai memasukkan baris (id, nilai...) secara bertahap; nilai boleh string atau float64.
func (l *lingkParitas) isiBaterai(t *testing.T, kolom []string, baris [][]interface{}) {
	t.Helper()
	lebar := 1 + len(kolom)
	per := 1800 / lebar
	kut := make([]string, len(kolom))
	for i, k := range kolom {
		kut[i] = kutipDB(k)
	}
	for dari := 0; dari < len(baris); dari += per {
		sampai := dari + per
		if sampai > len(baris) {
			sampai = len(baris)
		}
		var vals []string
		var args []interface{}
		for i := dari; i < sampai; i++ {
			ph := make([]string, lebar)
			for j := 0; j < lebar; j++ {
				ph[j] = "@p" + strconv.Itoa(len(args)+1)
				args = append(args, baris[i][j])
			}
			vals = append(vals, "("+strings.Join(ph, ", ")+")")
		}
		if _, err := l.mart.Exec("INSERT INTO dbo.bat_uji (id, "+strings.Join(kut, ", ")+") VALUES "+strings.Join(vals, ", "), args...); err != nil {
			t.Fatalf("mengisi baterai: %v", err)
		}
	}
}

// bacaBaterai: nilai mentah kolom (seperti yang dilihat aplikasi dari driver) per id, dan hasil ekspresi datamart.
func (l *lingkParitas) bacaMentah(t *testing.T, kolom string) map[int]interface{} {
	t.Helper()
	rows, err := l.mart.Query("SELECT id, " + kutipDB(kolom) + " FROM dbo.bat_uji ORDER BY id")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := map[int]interface{}{}
	for rows.Next() {
		var id int
		var v interface{}
		if err := rows.Scan(&id, &v); err != nil {
			t.Fatal(err)
		}
		out[id] = v
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func (l *lingkParitas) hasilEkspresi(t *testing.T, jenis int, ekspresi string, tambahan string) map[int]interface{} {
	t.Helper()
	rows, err := l.mart.Query("DECLARE @tp TINYINT = " + strconv.Itoa(jenis) + "; SELECT r.id, " + ekspresi + " FROM dbo.bat_uji AS r " + tambahan)
	if err != nil {
		t.Fatalf("ekspresi datamart gagal: %v\n%s", err, ekspresi)
	}
	defer rows.Close()
	out := map[int]interface{}{}
	for rows.Next() {
		var id int
		var v interface{}
		if err := rows.Scan(&id, &v); err != nil {
			t.Fatal(err)
		}
		out[id] = v
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func teksNilai(v interface{}) string {
	switch x := v.(type) {
	case nil:
		return "NULL"
	case []byte:
		return string(x)
	case float64:
		return strconv.FormatFloat(x, 'g', -1, 64)
	}
	return fmt.Sprint(v)
}

func kolomDataset(t *testing.T, key, nama string) (digitalisasi.Dataset, digitalisasi.Column) {
	t.Helper()
	for _, ds := range digitalisasi.Datasets {
		if ds.Key == key {
			c, ok := ds.Column(nama)
			if !ok {
				t.Fatalf("kolom %s.%s tidak ada", key, nama)
			}
			return ds, c
		}
	}
	t.Fatalf("dataset %s tidak ada", key)
	return digitalisasi.Dataset{}, digitalisasi.Column{}
}

// desimalServer: nilai teks diubah SQL Server ke DECIMAL(28, skala), persis seperti saat aplikasi menyimpannya (parameter teks -> kolom DECIMAL).
func (l *lingkParitas) desimalServer(t *testing.T, skala int, teks map[int]interface{}) map[int]string {
	t.Helper()
	ids := make([]int, 0, len(teks))
	for id := range teks {
		ids = append(ids, id)
	}
	out := map[int]string{}
	for dari := 0; dari < len(ids); dari += 800 {
		sampai := dari + 800
		if sampai > len(ids) {
			sampai = len(ids)
		}
		var vals []string
		var args []interface{}
		for _, id := range ids[dari:sampai] {
			args = append(args, id, teks[id])
			vals = append(vals, fmt.Sprintf("(@p%d, @p%d)", len(args)-1, len(args)))
		}
		q := fmt.Sprintf("SELECT v.id, TRY_CAST(CAST(v.t AS NVARCHAR(100)) AS DECIMAL(28, %d)) FROM (VALUES %s) AS v(id, t)", skala, strings.Join(vals, ", "))
		rows, err := l.mart.Query(q, args...)
		if err != nil {
			t.Fatal(err)
		}
		for rows.Next() {
			var id int
			var v interface{}
			if err := rows.Scan(&id, &v); err != nil {
				rows.Close()
				t.Fatal(err)
			}
			out[id] = teksNilai(v)
		}
		rows.Close()
	}
	return out
}

func laporkanBeda(t *testing.T, nama string, beda []string, total int) {
	t.Helper()
	if len(beda) > 0 {
		if len(beda) > 25 {
			beda = append(beda[:25], fmt.Sprintf("... (+%d)", len(beda)-25))
		}
		t.Errorf("%s: %d dari %d masukan berbeda dengan aplikasi:\n  %s", nama, len(beda), total, strings.Join(beda, "\n  "))
	}
}

func TestParitasBateraiKonversi(t *testing.T) {
	l := siapkanParitas(t)
	l.pasangDatamart(t) // fungsi dm_* dipasang script
	r := rand.New(rand.NewSource(20260101))

	// Teks: tiga panjang kolom yang dipakai (10 contoh kecil, 500 NVARCHAR(500), 0 = MAX).
	t.Run("teks", func(t *testing.T) {
		masuk := append(acakTeks(r, 1500), "", " ", strings.Repeat("a", 499)+"\U0001f600"+"bb", strings.Repeat("a", 500)+"\U0001f600", strings.Repeat("a", 498)+"\U0001f600", "\U0001f600\U0001f600\U0001f600\U0001f600\U0001f600\U0001f600", strings.Repeat("\U0001f600", 251), strings.Repeat(" ", 20)+"x"+strings.Repeat("\u3000", 20))
		for _, ukuran := range []int{10, 500, 0} {
			l.kolomBaterai(t, "NVARCHAR(MAX)", "v")
			baris := make([][]interface{}, len(masuk))
			for i, s := range masuk {
				baris[i] = []interface{}{i, s}
			}
			l.isiBaterai(t, []string{"v"}, baris)
			c := digitalisasi.Column{Name: "x", Source: "v", Kind: digitalisasi.Text, Size: ukuran}
			got := l.hasilEkspresi(t, 0, ekspresiKolom(c, digitalisasi.Dataset{}, "@tp"), "")
			var beda []string
			for i, s := range masuk {
				want := "NULL"
				if w := acuan.KonversiNilai(c, s); w != nil {
					want = w.(string)
				}
				if g := teksNilai(got[i]); g != want {
					beda = append(beda, fmt.Sprintf("ukuran %d: %q -> datamart %q, aplikasi %q", ukuran, s, g, want))
				}
			}
			laporkanBeda(t, "teks", beda, len(masuk))
		}
	})

	// Bilangan bulat (INT dan BIGINT): masukan teks dan float.
	t.Run("bilangan bulat", func(t *testing.T) {
		masuk := append(append([]string{}, angkaTetap...), acakAngka(r, 2500, 18)...)
		l.kolomBaterai(t, "NVARCHAR(MAX)", "v")
		baris := make([][]interface{}, len(masuk))
		for i, s := range masuk {
			baris[i] = []interface{}{i, s}
		}
		l.isiBaterai(t, []string{"v"}, baris)
		c := digitalisasi.Column{Name: "x", Source: "v", Kind: digitalisasi.BigInt}
		got := l.hasilEkspresi(t, 0, ekspresiKolom(c, digitalisasi.Dataset{}, "@tp"), "")
		var beda []string
		dilewati := 0
		defer func() {
			t.Logf("%d dari %d masukan di luar jaminan presisi (lihat deviasiDikenal) tidak dibandingkan", dilewati, len(masuk))
		}()
		for i, s := range masuk {
			if deviasiDikenal(s, c.Kind) {
				dilewati++
				continue
			}
			want := "NULL"
			if w := acuan.KonversiNilai(c, s); w != nil {
				want = strconv.FormatInt(w.(int64), 10)
			}
			if g := teksNilai(got[i]); g != want {
				beda = append(beda, fmt.Sprintf("%q -> datamart %s, aplikasi %s", s, g, want))
			}
		}
		laporkanBeda(t, "bilangan bulat", beda, len(masuk))
	})

	// Desimal: sumber teks, sumber DECIMAL(18,2), sumber FLOAT; skala 2 dan 4 (skala kolom aplikasi).
	t.Run("desimal", func(t *testing.T) {
		jenis := []struct {
			nama, tipe string
			masuk      func() []interface{}
		}{
			{"teks", "NVARCHAR(MAX)", func() []interface{} {
				var o []interface{}
				for _, s := range append(append([]string{}, angkaTetap...), acakAngka(r, 2500, 17)...) {
					o = append(o, s)
				}
				return o
			}},
			{"DECIMAL(18,2)", "DECIMAL(18, 2)", func() []interface{} {
				var o []interface{}
				for _, s := range []string{"0", "0.50", "-0.01", "1234567890123456.78", "9999999999999.99", "123456789012.34", "2.67", "2.68", "1000000.00", "0.05", "99999999999999.99"} {
					o = append(o, s)
				}
				for _, s := range acakAngka(r, 1500, 16) {
					if strings.ContainsAny(s, "eE, \t\u00a0+") {
						continue
					}
					if i := strings.Index(s, "."); i >= 0 && len(s)-i-1 > 2 {
						s = s[:i+3]
					}
					o = append(o, s)
				}
				return o
			}},
			{"FLOAT", "FLOAT", func() []interface{} {
				o := []interface{}{0.0, 0.1, 0.2, 0.3, 0.1 + 0.2, 2.675, 1.005, 1.15, 1e-7, 5e-8, 4.4e-8, 123456789.125, 1e15, 1e16, 12345678901234.56, 99999999999.995, -0.005, -2.675}
				for i := 0; i < 1500; i++ {
					v := r.Float64() * math.Pow(10, float64(r.Intn(15)-4))
					if r.Intn(2) == 0 {
						v = math.Round(v*1000) / 1000
					}
					if r.Intn(4) == 0 {
						v = -v
					}
					o = append(o, v)
				}
				return o
			}},
		}
		for _, j := range jenis {
			kode := kodeJenisSumber(j.tipe)
			masuk := j.masuk()
			l.kolomBaterai(t, j.tipe, "v")
			baris := make([][]interface{}, len(masuk))
			for i, v := range masuk {
				baris[i] = []interface{}{i, v}
			}
			l.isiBaterai(t, []string{"v"}, baris)
			mentah := l.bacaMentah(t, "v")
			for _, skala := range []int{2, 4} {
				c := digitalisasi.Column{Name: "x", Source: "v", Kind: digitalisasi.Decimal, Scale: skala}
				got := l.hasilEkspresi(t, kode, fmt.Sprintf("TRY_CAST(%s AS DECIMAL(28, %d))", ekspresiKolom(c, digitalisasi.Dataset{}, "@tp"), skala), "")
				app := map[int]interface{}{}
				for i := range masuk {
					app[i] = acuan.KonversiNilai(c, mentah[i])
				}
				want := l.desimalServer(t, skala, app)
				var beda []string
				dilewati := 0
				for i := range masuk {
					if kode == 0 && deviasiDikenal(teksNilai(mentah[i]), digitalisasi.Decimal) {
						dilewati++
						continue
					}
					if g := teksNilai(got[i]); g != want[i] {
						beda = append(beda, fmt.Sprintf("%v (%s, skala %d) -> datamart %s, aplikasi %s", teksNilai(mentah[i]), j.nama, skala, g, want[i]))
					}
				}
				t.Logf("desimal %s skala %d: %d dari %d masukan di luar jaminan presisi tidak dibandingkan", j.nama, skala, dilewati, len(masuk))
				laporkanBeda(t, "desimal "+j.nama, beda, len(masuk))
			}
		}
	})

	// Koordinat: pasangan (lintang, bujur) sebagai teks, DECIMAL, dan FLOAT; aturan berpasangan dan (0,0).
	t.Run("koordinat", func(t *testing.T) {
		ds, cl := kolomDataset(t, "tanah", "Latitude")
		_, cg := kolomDataset(t, "tanah", "Longitude")
		sl, sg := cl.SourceName(), cg.SourceName()
		jenis := []struct {
			nama, tipe string
			masuk      func() [][2]interface{}
		}{
			{"teks", "NVARCHAR(MAX)", func() [][2]interface{} {
				semua := append(append([]string{}, angkaTetap...), acakAngka(r, 1500, 17)...)
				var o [][2]interface{}
				for i, s := range semua {
					o = append(o, [2]interface{}{s, semua[(i*7+3)%len(semua)]})
					o = append(o, [2]interface{}{s, "106.5"}, [2]interface{}{"-6.5", s})
				}
				o = append(o, [2]interface{}{"0", "0"}, [2]interface{}{"0,0", "0.0"}, [2]interface{}{"-0", "+0"}, [2]interface{}{"", ""}, [2]interface{}{"5", ""}, [2]interface{}{"", "5"})
				return o
			}},
			{"DECIMAL(9,6)", "DECIMAL(9, 6)", func() [][2]interface{} {
				var o [][2]interface{}
				for i := 0; i < 1500; i++ {
					o = append(o, [2]interface{}{strconv.FormatFloat((r.Float64()*2-1)*90, 'f', 6, 64), strconv.FormatFloat((r.Float64()*2-1)*180, 'f', 6, 64)})
				}
				o = append(o, [2]interface{}{"0", "0"}, [2]interface{}{"90", "180"}, [2]interface{}{"-90", "-180"}, [2]interface{}{"0.000001", "0"})
				return o
			}},
			{"FLOAT", "FLOAT", func() [][2]interface{} {
				var o [][2]interface{}
				for i := 0; i < 1500; i++ {
					o = append(o, [2]interface{}{(r.Float64()*2 - 1) * 95, (r.Float64()*2 - 1) * 185})
				}
				o = append(o, [2]interface{}{-6.123456789, 106.123456789}, [2]interface{}{0.0, 0.0}, [2]interface{}{5e-8, 5e-8}, [2]interface{}{90.0, 180.0}, [2]interface{}{-6.2, 106.8}, [2]interface{}{1.15, 2.675})
				return o
			}},
		}
		for _, j := range jenis {
			kode := kodeJenisSumber(j.tipe)
			masuk := j.masuk()
			l.kolomBaterai(t, j.tipe, sl, sg)
			baris := make([][]interface{}, len(masuk))
			for i, p := range masuk {
				baris[i] = []interface{}{i, p[0], p[1]}
			}
			l.isiBaterai(t, []string{sl, sg}, baris)
			ml, mg := l.bacaMentah(t, sl), l.bacaMentah(t, sg)
			gotLat := l.hasilEkspresi(t, kode, "CONCAT(ISNULL(CAST("+ekspresiKolom(cl, ds, "@tp")+" AS NVARCHAR(40)), N'NULL'), N'|', ISNULL(CAST("+ekspresiKolom(cg, ds, "@tp")+" AS NVARCHAR(40)), N'NULL'))", terapkanKoordinat(ds, "@tp", "@tp"))

			// padanan aplikasi: convertValue per kolom lalu fixCoordinates (pasangan dan nol)
			app := map[int]interface{}{}
			for i := range masuk {
				a, b := acuan.KonversiNilai(cl, ml[i]), acuan.KonversiNilai(cg, mg[i])
				if a == nil || b == nil || (a.(float64) == 0 && b.(float64) == 0) {
					a, b = nil, nil
				}
				app[i] = [2]interface{}{a, b}
			}
			// nilai float64 aplikasi dibulatkan ke DECIMAL(10,7) oleh SQL Server, persis seperti saat disimpan
			var beda []string
			for i := 0; i < len(masuk); i += 400 {
				sampai := i + 400
				if sampai > len(masuk) {
					sampai = len(masuk)
				}
				var vals []string
				var args []interface{}
				for k := i; k < sampai; k++ {
					p := app[k].([2]interface{})
					args = append(args, k, p[0], p[1])
					vals = append(vals, fmt.Sprintf("(@p%d, CAST(@p%d AS FLOAT), CAST(@p%d AS FLOAT))", len(args)-2, len(args)-1, len(args)))
				}
				rows, err := l.mart.Query("SELECT v.id, CONCAT(ISNULL(CAST(CAST(v.a AS DECIMAL(10, 7)) AS NVARCHAR(40)), N'NULL'), N'|', ISNULL(CAST(CAST(v.b AS DECIMAL(10, 7)) AS NVARCHAR(40)), N'NULL')) FROM (VALUES "+strings.Join(vals, ", ")+") AS v(id, a, b)", args...)
				if err != nil {
					t.Fatal(err)
				}
				for rows.Next() {
					var id int
					var w string
					if err := rows.Scan(&id, &w); err != nil {
						rows.Close()
						t.Fatal(err)
					}
					if kode == 0 && (deviasiDikenal(teksNilai(ml[id]), digitalisasi.Coord) || deviasiDikenal(teksNilai(mg[id]), digitalisasi.Coord)) {
						continue
					}
					if g := teksNilai(gotLat[id]); g != w {
						beda = append(beda, fmt.Sprintf("(%s, %s) [%s] -> datamart %s, aplikasi %s", teksNilai(ml[id]), teksNilai(mg[id]), j.nama, g, w))
					}
				}
				rows.Close()
			}
			laporkanBeda(t, "koordinat "+j.nama, beda, len(masuk))
		}
	})
}

// kodeJenisSumber: nilai variabel jenis (varJenis) untuk kolom mentah bertipe tipe: 1 = FLOAT atau REAL (dibaca sebagai bilangan), 0 = lainnya.
func kodeJenisSumber(tipe string) int {
	switch strings.ToUpper(tipe) {
	case "FLOAT", "REAL":
		return 1
	}
	return 0
}

// deviasiDikenal: masukan teks yang di luar jaminan kesamaan persis dengan aplikasi (didokumentasikan di docs/datamart-sldk.md):
//   - bentuk heksadesimal Go (0x1p3), yang tidak dikenali datamart;
//   - angka dengan lebih dari 15 digit penting: aplikasi membacanya sebagai float64 (kehilangan digit), datamart mempertahankan digit aslinya;
//   - bilangan bulat di atas 2^53: aplikasi memakai nilai float64-nya.
func deviasiDikenal(s string, kind digitalisasi.Kind) bool {
	t := strings.TrimSpace(s)
	if strings.ContainsAny(t, "xXpP") {
		return true
	}
	m := t
	if i := strings.IndexAny(m, "eE"); i >= 0 {
		m = m[:i]
	}
	var d []byte
	for i := 0; i < len(m); i++ {
		if m[i] >= '0' && m[i] <= '9' {
			d = append(d, m[i])
		}
	}
	d = []byte(strings.Trim(string(d), "0"))
	if len(d) > 15 {
		return true
	}
	if kind == digitalisasi.Int || kind == digitalisasi.BigInt {
		u := strings.ReplaceAll(t, "_", "")
		if !strings.Contains(u, ".") {
			u = strings.Replace(u, ",", ".", 1)
		}
		if f, err := strconv.ParseFloat(u, 64); err == nil && math.Abs(f) >= 1<<53 {
			return true
		}
	}
	return false
}

// Uji skala (opsional): PASTI_UJI_BARIS=200000 menambahkan sebanyak itu tanah ke SLDK palsu, mengukur lama penyegaran datamart dibanding sinkronisasi aplikasi, dan membandingkan
// jumlah baris serta checksum isi tiap tabel. Tidak ikut berjalan bila PASTI_UJI_BARIS kosong.
func TestParitasSkalaBesar(t *testing.T) {
	n, _ := strconv.Atoi(os.Getenv("PASTI_UJI_BARIS"))
	if n <= 0 {
		t.Skip("PASTI_UJI_BARIS tidak diisi: uji skala dilewati")
	}
	l := siapkanParitas(t)
	tambah := fmt.Sprintf(`
;WITH n AS (SELECT TOP (%d) ROW_NUMBER() OVER (ORDER BY (SELECT NULL)) AS i FROM sys.all_objects AS a CROSS JOIN sys.all_objects AS b CROSS JOIN sys.all_objects AS c)
INSERT DJKN.SIMAN2_M_ASET (id_aset, kode_register, kd_satker, kd_brg, ur_sskel, no_aset, alamat, kd_rtrw, ur_kel, ur_kec, ur_kab, ur_prov, luas, luas_tnhb, kd_kondisi, rph_aset, gps_latitude, gps_longitude, status_bmn_yn, kd_status_hukum, TGL_TARIK)
SELECT 1000000 + i, N'REG-BIG-' + CAST(i AS NVARCHAR(12)), CASE WHEN i %% 2 = 0 THEN '015100199100001000KP' ELSE '015040199119091000KP' END, '2010101001',
       N'Tanah besar nomor ' + CAST(i AS NVARCHAR(12)), i, N' Jl. Besar ' + CAST(i AS NVARCHAR(12)) + N' ', CASE WHEN i %% 7 = 0 THEN NULL ELSE '01/02' END, N'Kelurahan ' + CAST(i %% 500 AS NVARCHAR(6)),
       N'Kecamatan ' + CAST(i %% 50 AS NVARCHAR(6)), N'Kota ' + CAST(i %% 20 AS NVARCHAR(6)), N'Provinsi ' + CAST(i %% 5 AS NVARCHAR(6)), (i %% 100000) / 100.0, (i %% 5000) / 100.0, CAST(1 + i %% 3 AS VARCHAR(2)),
       i * 1234.56, CASE WHEN i %% 11 = 0 THEN NULL ELSE CAST(-11 + (i %% 17000) / 1000.0 AS VARCHAR(30)) END, CASE WHEN i %% 11 = 0 THEN NULL ELSE REPLACE(CAST(95 + (i %% 45000) / 1000.0 AS VARCHAR(30)), '.', ',') END,
       'Y', CASE WHEN i %% 3 = 0 THEN '01' ELSE '02' END, 20250101
FROM n;
`, n)
	if err := ekse(t, l.srv, l.sldkNama, tambah); err != nil {
		t.Fatal(err)
	}
	mulai := time.Now()
	l.sinkronAcuan(t, 1)
	lamaApp := time.Since(mulai)
	l.pasangDatamart(t)
	var lamaMart time.Duration
	r := baca(t, l.srv, l.martNama, "SELECT DATEDIFF_BIG(MILLISECOND, mulai, selesai) FROM dbo.datamart_digitalisasi_run WHERE id = (SELECT MAX(id) FROM dbo.datamart_digitalisasi_run WHERE status = N'sukses')")
	if ms, err := strconv.Atoi(r[0][0]); err == nil {
		lamaMart = time.Duration(ms) * time.Millisecond
	}
	t.Logf("%d baris tanah tambahan: sinkronisasi aplikasi %s, penyegaran datamart %s", n, lamaApp.Round(time.Millisecond), lamaMart.Round(time.Millisecond))

	for _, ds := range digitalisasi.Datasets {
		q := "SELECT COUNT_BIG(*), ISNULL(CHECKSUM_AGG(CHECKSUM(" + strings.Join(kolomData(ds), ", ") + ")), 0) FROM dbo." + ds.Table
		a, b := baca(t, l.srv, l.acuanNama, q), baca(t, l.srv, l.martNama, q)
		if strings.Join(a[0], "/") != strings.Join(b[0], "/") {
			t.Errorf("%s: PASTI %v, datamart %v", ds.Table, a[0], b[0])
		}
		t.Logf("%s: %s baris", ds.Table, a[0][0])
	}
}
