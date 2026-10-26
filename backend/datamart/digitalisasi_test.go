package datamart

import (
	"flag"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"pasti-v3-backend/digitalisasi"
)

var update = flag.Bool("update", false, "tulis ulang buat_datamart_digitalisasi.sql dari generator")

// Tes statis script datamart Digitalisasi Aset (tanpa database).

func bacaSkripDigitalisasi(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile(BerkasDigitalisasi)
	if err != nil {
		t.Fatalf("%v (buat dengan: go test ./datamart -run TestSkripDigitalisasiSesuaiGenerator -update)", err)
	}
	return strings.ReplaceAll(string(raw), "\r\n", "\n")
}

func TestSkripDigitalisasiSesuaiGenerator(t *testing.T) {
	want, err := SkripDigitalisasi()
	if err != nil {
		t.Fatal(err)
	}
	if *update {
		if err := os.WriteFile(BerkasDigitalisasi, []byte(want), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	if got := bacaSkripDigitalisasi(t); got != want {
		t.Fatalf("%s tidak sejalan dengan aplikasi (datasets.go dan queries/*.sql); jalankan: go test ./datamart -run TestSkripDigitalisasiSesuaiGenerator -update", BerkasDigitalisasi)
	}
}

// Query asli disisipkan APA ADANYA: satu-satunya beda dengan queries/*.sql adalah baris  INTO #raw_<dataset>  pada SELECT akhir.
func TestQueryAsliDisisipkanApaAdanya(t *testing.T) {
	skrip := bacaSkripDigitalisasi(t)
	for _, ds := range digitalisasi.Datasets {
		q, err := ds.Query()
		if err != nil {
			t.Fatal(err)
		}
		q = strings.TrimRight(strings.ReplaceAll(q, "\r\n", "\n"), "\n")
		sisip := "INTO #raw_" + ds.Key + "\n"
		if strings.Count(skrip, sisip) != 1 {
			t.Errorf("%s: harus tepat satu  INTO #raw_%s", ds.Key, ds.Key)
			continue
		}
		// buang satu-satunya baris tambahan lalu cari seluruh query asli sebagai satu potongan utuh
		tanpa := strings.Replace(skrip, sisip, "", 1)
		if !strings.Contains(tanpa, q+"\n") {
			t.Errorf("%s: teks query asli (queries/%s) tidak ada utuh di script", ds.Key, ds.QueryFile)
		}
		// INTO harus langsung sebelum FROM milik SELECT akhir (di kolom pertama)
		if !strings.Contains(skrip, sisip+"FROM ") {
			t.Errorf("%s: INTO #raw_%s tidak berada tepat sebelum FROM", ds.Key, ds.Key)
		}
	}
}

// Setiap kolom tujuan, tiap dataset, dikonversi dengan fungsi yang sesuai jenisnya dan panjang yang sama dengan aplikasi.
func TestKonversiKolomSesuaiDefinisiAplikasi(t *testing.T) {
	skrip := bacaSkripDigitalisasi(t)
	for _, ds := range digitalisasi.Datasets {
		awal := strings.Index(skrip, "CREATE OR ALTER PROCEDURE dbo.datamart_dg_muat_"+ds.Key+" ")
		if awal < 0 {
			t.Fatalf("prosedur muat %s tidak ditemukan", ds.Key)
		}
		akhir := strings.Index(skrip[awal:], "DROP TABLE IF EXISTS #raw_"+ds.Key+";\nEND;\n")
		if akhir < 0 {
			t.Fatalf("akhir prosedur muat %s tidak ditemukan", ds.Key)
		}
		proc := skrip[awal : awal+akhir]
		sisip := strings.Index(proc, "INSERT INTO DIGITALISASI_STG."+ds.Table)
		if sisip < 0 {
			t.Fatalf("%s: INSERT ke staging tidak ditemukan", ds.Key)
		}
		insert := proc[sisip:]
		// variabel jenis kolom mentah: @tp_N dideklarasikan untuk kolom sumber src (dibaca dari katalog tempdb)
		jenis := func(src string) string {
			m := regexp.MustCompile(`@tp_(\d+) TINYINT = CASE WHEN ISNULL\(\(SELECT tipe FROM @jenis WHERE kolom = N'` + regexp.QuoteMeta(src) + `'\), 0\) IN \(62, 59\) THEN 1 ELSE 0 END`).FindStringSubmatch(proc[:sisip])
			if m == nil {
				return ""
			}
			return "@tp_" + m[1]
		}
		for _, c := range ds.Columns {
			src := c.SourceName()
			raw := "r.[" + src + "]"
			var perlu string
			switch c.Kind {
			case digitalisasi.Int, digitalisasi.BigInt, digitalisasi.Decimal:
				tp := jenis(src)
				if tp == "" {
					t.Errorf("%s.%s: jenis kolom mentah %s tidak dibaca dari katalog tempdb", ds.Key, c.Name, src)
					continue
				}
				angka := "CASE WHEN " + tp + " = 1 THEN dbo.dm_float_ke_teks(CAST(" + raw + " AS FLOAT)) ELSE CONVERT(NVARCHAR(4000), " + raw + ", 3) END"
				if c.Kind == digitalisasi.Decimal {
					perlu = "dbo.dm_desimal(" + angka + ", " + strconv.Itoa(c.Scale) + ")"
				} else {
					perlu = "dbo.dm_bilangan(" + angka + ")"
				}
			case digitalisasi.Coord:
				// lintang/bujur lewat CROSS APPLY; kolom FLOAT dipakai langsung, selain itu teks
				tp := jenis(src)
				batas := "180"
				if strings.EqualFold(c.Name, "Latitude") {
					batas = "90"
				}
				if tp == "" {
					t.Errorf("%s.%s: jenis kolom mentah %s tidak dibaca dari katalog tempdb", ds.Key, c.Name, src)
					continue
				}
				perlu = "dbo.dm_koordinat(CASE WHEN " + tp + " = 1 THEN NULL ELSE CONVERT(NVARCHAR(4000), " + raw + ", 3) END, CASE WHEN " + tp + " = 1 THEN CAST(" + raw + " AS FLOAT) END, " + batas + ")"
			default:
				perlu = "dbo.dm_teks(CONVERT(NVARCHAR(MAX), " + raw + "), " + strconv.Itoa(c.Size) + ")"
			}
			if !strings.Contains(insert, perlu) {
				t.Errorf("%s.%s: konversi %q tidak ada di script", ds.Key, c.Name, perlu)
			}
			if !strings.Contains(insert, "["+c.Name+"]") {
				t.Errorf("%s.%s: kolom tujuan tidak ada di daftar INSERT", ds.Key, c.Name)
			}
		}
		// kolom teks dari SLDK dengan tipe yang bentuk tekstualnya berbeda ditolak dengan galat jelas (bukan ditulis dalam bentuk lain)
		if !strings.Contains(proc, "THROW 50050, @salah, 1;") {
			t.Errorf("%s: penjaga tipe kolom teks tidak ada", ds.Key)
		}
	}
}

// Struktur tabel hidup datamart = struktur tabel aplikasi (migrasi 020): tiap CREATE TABLE tabel aplikasi ada di script dengan kolom dan tipe yang sama.
func TestStrukturTabelSamaDenganMigrasiAplikasi(t *testing.T) {
	skrip := bacaSkripDigitalisasi(t)
	migrasi := strings.ReplaceAll(digitalisasi.MigrationSQL(), "\r\n", "\n")
	for _, ds := range digitalisasi.Datasets {
		reMig := regexp.MustCompile(`(?s)CREATE TABLE ` + ds.Table + ` \((.*?)\n    \);`)
		m := reMig.FindStringSubmatch(migrasi)
		if m == nil {
			t.Fatalf("%s tidak ada di migrasi 020", ds.Table)
		}
		reSkrip := regexp.MustCompile(`(?s)CREATE TABLE dbo\.` + ds.Table + ` \((.*?)\n    \);`)
		s := reSkrip.FindStringSubmatch(skrip)
		if s == nil {
			t.Fatalf("%s tidak ada di script", ds.Table)
		}
		norm := func(x string) []string {
			var out []string
			for _, l := range strings.Split(x, "\n") {
				l = strings.TrimSpace(l)
				if l == "" {
					continue
				}
				if i := strings.Index(l, "  --"); i >= 0 {
					l = strings.TrimSpace(l[:i])
				}
				out = append(out, l)
			}
			return out
		}
		a, b := norm(m[1]), norm(s[1])
		if strings.Join(a, "|") != strings.Join(b, "|") {
			t.Errorf("%s: kolom/tipe berbeda dari migrasi 020\n migrasi: %v\n script : %v", ds.Table, a, b)
		}
		// indeks tabel hidup sama dengan migrasi
		for _, col := range digitalisasi.IndexedColumns(ds) {
			idx := "CREATE INDEX idx_" + strings.ToLower(ds.Table) + "_" + strings.ToLower(col) + " ON dbo." + ds.Table + "(" + col + ");"
			if !strings.Contains(skrip, idx) {
				t.Errorf("%s: indeks %q tidak ada di script", ds.Table, idx)
			}
		}
	}
}

// SLDK hanya dibaca: satu-satunya penulisan ke luar objek datamart adalah pembuatan/penghapusan sinonim di skema DJKN. Dan tidak ada yang mengubah server, izin, atau pengguna.
func TestSkripDigitalisasiHanyaMembacaSLDK(t *testing.T) {
	skrip := bacaSkripDigitalisasi(t)
	kode := reKomentarBaris.ReplaceAllString(skrip, "")
	for _, larangan := range []string{"DROP DATABASE", "xp_cmdshell", "sp_addlinkedserver", "sp_configure", "EXECUTE AS", "CREATE LOGIN", "CREATE USER", "GRANT ", "ALTER SERVER"} {
		if strings.Contains(strings.ToUpper(kode), strings.ToUpper(larangan)) {
			t.Errorf("script memuat %q", larangan)
		}
	}
	// tulis ke DJKN hanya berupa sinonim
	for _, m := range regexp.MustCompile(`(?i)\b(INSERT\s+INTO|INSERT|UPDATE|DELETE\s+FROM|TRUNCATE\s+TABLE|ALTER\s+TABLE)\s+DJKN\.`).FindAllString(kode, -1) {
		t.Errorf("script menulis ke objek DJKN (SLDK): %q", m)
	}
	// semua tujuan tulis: tabel sementara #, DIGITALISASI_STG, tabel hidup DIGITALISASI_*, dan tabel jejak
	boleh := regexp.MustCompile(`(?i)^(#|DIGITALISASI_STG\.|dbo\.DIGITALISASI_|dbo\.datamart_digitalisasi_)`)
	for _, m := range regexp.MustCompile(`(?i)\b(?:INSERT(?:\s+INTO)?|UPDATE|TRUNCATE\s+TABLE|DELETE\s+FROM|SELECT\s[^;]*?\bINTO)\s+([#\w\.\[\]]+)`).FindAllStringSubmatch(kode, -1) {
		if !boleh.MatchString(m[1]) {
			t.Errorf("script menulis ke %q di luar daerah kerja datamart", m[1])
		}
	}
	// DJKN.* hanya dipakai lewat sinonim ke SLDK
	if !strings.Contains(skrip, "CREATE SYNONYM DJKN.") || !strings.Contains(skrip, "DROP SYNONYM DJKN.") {
		t.Error("sinonim DJKN tidak dibuat oleh script")
	}
}

func TestObjekSumberSesuaiQuery(t *testing.T) {
	objek, err := ObjekSumber()
	if err != nil {
		t.Fatal(err)
	}
	skrip := bacaSkripDigitalisasi(t)
	if len(objek) < 8 {
		t.Fatalf("hanya %d objek SLDK terbaca: %v", len(objek), objek)
	}
	for _, o := range objek {
		if !strings.Contains(skrip, "(N'"+o+"')") {
			t.Errorf("objek SLDK %s dibaca query tetapi tidak dibuatkan sinonim", o)
		}
	}
	// objek yang tak dikenal query tidak dibuatkan sinonim (hak akses seminimal mungkin)
	m := regexp.MustCompile(`(?s)INSERT @objek \(nama\) VALUES\n(.*?);`).FindStringSubmatch(skrip)
	if m == nil || strings.Count(m[1], "(N'") != len(objek) {
		t.Errorf("daftar sinonim tidak sama dengan objek yang dibaca query: %v", m)
	}
}

// Query yang berakhir dengan ORDER BY harus punya urutan muat yang sama; yang tidak, tidak boleh punya.
func TestUrutanMuatSesuaiOrderByQuery(t *testing.T) {
	for _, ds := range digitalisasi.Datasets {
		q, _ := ds.Query()
		q = strings.ReplaceAll(q, "\r\n", "\n")
		akhir := q[strings.LastIndex(q, "\nFROM "):]
		punyaOrder := regexp.MustCompile(`(?m)^ORDER BY `).MatchString(akhir)
		_, adaUrutan := urutanTambahan[ds.Key]
		if punyaOrder != adaUrutan {
			t.Errorf("%s: query ORDER BY=%v, urutan muat ditetapkan=%v", ds.Key, punyaOrder, adaUrutan)
		}
	}
}

func TestSkripDigitalisasiMemuatPengamanDanJejak(t *testing.T) {
	skrip := bacaSkripDigitalisasi(t)
	for _, perlu := range []string{
		"CREATE OR ALTER PROCEDURE dbo.datamart_digitalisasi_segarkan", "EXEC dbo.datamart_digitalisasi_segarkan;", "sp_getapplock", "SET NOEXEC ON", "SET NOEXEC OFF",
		"digitalisasi_sync_log", "TRUNCATE TABLE dbo.DIGITALISASI_SATKER", "THROW 50041", "BEGIN TRANSACTION", "COMMIT TRANSACTION", "datamart_digitalisasi_run", "datamart_digitalisasi_tabel",
	} {
		if !strings.Contains(skrip, perlu) {
			t.Errorf("script tidak memuat %q", perlu)
		}
	}
	if strings.Count(skrip, "THROW 50041") != len(digitalisasi.Datasets) {
		t.Errorf("pengaman hasil kosong harus ada untuk tiap dataset")
	}
	// @Sumber punya nilai awal yang jelas dan satu-satunya yang perlu diubah
	if !strings.Contains(skrip, "DECLARE @Sumber NVARCHAR(300) = N'"+SumberBawaan+"';") {
		t.Error("baris @Sumber tidak ditemukan")
	}
	// kolom manual satker dipertahankan
	if !strings.Contains(skrip, "COALESCE(dbo.dm_teks(CONVERT(NVARCHAR(MAX), r.[Status_Gedung_Kantor]), 200), (SELECT TOP (1) o.[Status_Gedung_Kantor] FROM dbo.DIGITALISASI_SATKER AS o WHERE o.[Kode_Satker] = ") {
		t.Error("kolom manual Status_Gedung_Kantor tidak dipertahankan")
	}
}
