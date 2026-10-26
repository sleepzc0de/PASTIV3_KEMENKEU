package datamart

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"pasti-v3-backend/digitalisasi"
)

// Generator script datamart "Digitalisasi Aset": SATU script T-SQL yang membentuk tabel DIGITALISASI_* di database datamart langsung dari SLDK, dengan isi yang SAMA dengan yang
// disimpan aplikasi lewat sinkronisasi (digitalisasi.Sync). Dibangkitkan dari definisi yang sama dengan aplikasi (digitalisasi.Datasets: kolom, tipe, panjang, kunci) dan dari
// ketujuh query asli (digitalisasi/queries/*.sql, disisipkan apa adanya), sehingga tidak dapat melenceng dari aplikasi.
//
// Cara script bekerja:
//   - query asli dijalankan apa adanya (hanya ditambah  INTO #raw_<dataset>  pada SELECT akhir) lewat sinonim DJKN.* yang menunjuk ke SLDK;
//   - hasil mentahnya dikonversi dengan fungsi dm_* yang meniru digitalisasi.convertValue/fixCoordinates (pangkas spasi, kosong menjadi NULL, potong sesuai panjang kolom,
//     angka dengan koma desimal, koordinat divalidasi dan harus berpasangan, (0,0) dibuang) dan dimuat ke tabel staging;
//   - bila ketujuh dataset berhasil, isi tabel DIGITALISASI_* datamart diganti dalam satu transaksi; bila ada yang gagal, isi lama tidak berubah.

// BerkasDigitalisasi: nama berkas script hasil generator.
const BerkasDigitalisasi = "buat_datamart_digitalisasi.sql"

// SumberBawaan: nilai awal @Sumber pada script (awalan database SLDK).
const SumberBawaan = "[Interchange]"

// urutanTambahan: dataset yang querynya berakhir dengan ORDER BY. Urutan yang sama dipakai saat memuat supaya kolom id (identity) mengikuti urutan keluaran query seperti pada aplikasi.
var urutanTambahan = map[string]string{
	"rusunara":          "r.[Kode_Satker], r.[NUP_Rusun]",
	"mess_rumah_negara": "r.[Kode_Satker], r.[Kode_mess], r.[NUP_mess]",
}

var (
	reKomentarBaris = regexp.MustCompile(`(?m)--.*$`)
	reObjekDJKN     = regexp.MustCompile(`(?i)\bDJKN\.(\w+)`)
)

func normal(s string) string { return strings.ReplaceAll(s, "\r\n", "\n") }

func kut(nama string) string { return "[" + strings.ReplaceAll(nama, "]", "]]") + "]" }

// queryDenganInto menyisipkan  INTO #raw_<key>  sebelum FROM milik SELECT akhir (baris terakhir yang diawali FROM di kolom pertama; subquery dan CTE selalu menjorok). Selain itu
// query tidak diubah satu karakter pun.
func queryDenganInto(q, key string) (string, error) {
	q = normal(q)
	i := strings.LastIndex(q, "\nFROM ")
	if i < 0 {
		return "", fmt.Errorf("query %s: tidak ada baris FROM di kolom pertama untuk SELECT akhir", key)
	}
	return q[:i+1] + "INTO #raw_" + key + "\n" + q[i+1:], nil
}

// ObjekSumber: objek SLDK (skema DJKN) yang dibaca ketujuh query, terurut.
func ObjekSumber() ([]string, error) {
	set := map[string]bool{}
	for _, ds := range digitalisasi.Datasets {
		q, err := ds.Query()
		if err != nil {
			return nil, err
		}
		for _, m := range reObjekDJKN.FindAllStringSubmatch(reKomentarBaris.ReplaceAllString(normal(q), ""), -1) {
			set[strings.ToUpper(m[1])] = true
		}
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out, nil
}

// ddlTabel: CREATE TABLE sebuah dataset di skema tertentu, sama dengan migrasi 020 (id identity, kolom data, id_sinkron, synced_at).
func ddlTabel(ds digitalisasi.Dataset, skema, indent string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%sCREATE TABLE %s.%s (\n", indent, skema, ds.Table)
	fmt.Fprintf(&b, "%s    id BIGINT IDENTITY(1,1) PRIMARY KEY,\n", indent)
	for _, c := range ds.Columns {
		line := fmt.Sprintf("%s    %s %s NULL,", indent, c.Name, c.SQLType())
		if c.Sensitive {
			line += "  -- data pribadi (nama penghuni): batasi akses ke datamart"
		}
		b.WriteString(line + "\n")
	}
	fmt.Fprintf(&b, "%s    id_sinkron BIGINT NULL,\n", indent)
	fmt.Fprintf(&b, "%s    synced_at DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME()\n", indent)
	fmt.Fprintf(&b, "%s);\n", indent)
	return b.String()
}

func ddlIndeks(ds digitalisasi.Dataset, skema, indent string) string {
	var b strings.Builder
	for _, col := range digitalisasi.IndexedColumns(ds) {
		fmt.Fprintf(&b, "%sCREATE INDEX idx_%s_%s ON %s.%s(%s);\n", indent, strings.ToLower(ds.Table), strings.ToLower(col), skema, ds.Table, col)
	}
	return b.String()
}

// kolomData: nama kolom data (tanpa id, id_sinkron, synced_at) dalam kurung siku.
func kolomData(ds digitalisasi.Dataset) []string {
	out := make([]string, len(ds.Columns))
	for i, c := range ds.Columns {
		out[i] = kut(c.Name)
	}
	return out
}

func adaKoordinat(ds digitalisasi.Dataset) (lat, lng bool) {
	_, lat = ds.Column("Latitude")
	_, lng = ds.Column("Longitude")
	return lat, lng
}

// varJenis: nama variabel T-SQL (TINYINT) yang menyimpan jenis kolom mentah ke-i: 1 = FLOAT atau REAL, 0 = lainnya (teks, DECIMAL, bilangan bulat).
func varJenis(i int) string { return fmt.Sprintf("@tp_%d", i) }

// teksAngka: kolom mentah angka sebagai teks untuk dibaca dm_polos. Kolom FLOAT/REAL dibaca aplikasi sebagai bilangan (REAL diperlebar ke FLOAT oleh driver) lalu ditulis sebagai angka
// terpendek yang kembali ke bilangan yang sama, bukan sebagai 17 digit hasil CONVERT; jenis kolom ada di variabel tp.
func teksAngka(raw, tp string) string {
	return fmt.Sprintf("CASE WHEN %[2]s = 1 THEN dbo.dm_float_ke_teks(CAST(%[1]s AS FLOAT)) ELSE CONVERT(NVARCHAR(4000), %[1]s, 3) END", raw, tp)
}

// ekspresiKolom: ekspresi T-SQL yang mengubah kolom mentah #raw (r.[sumber]) menjadi nilai kolom tujuan c, sama dengan digitalisasi.convertValue.
// tp: variabel jenis kolom mentah (lihat varJenis), hanya dipakai kolom angka.
func ekspresiKolom(c digitalisasi.Column, ds digitalisasi.Dataset, tp string) string {
	raw := "r." + kut(c.SourceName())
	var e string
	switch c.Kind {
	case digitalisasi.Int, digitalisasi.BigInt:
		e = fmt.Sprintf("dbo.dm_bilangan(%s)", teksAngka(raw, tp))
	case digitalisasi.Decimal:
		e = fmt.Sprintf("dbo.dm_desimal(%s, %d)", teksAngka(raw, tp), c.Scale)
	case digitalisasi.Coord:
		// lintang/bujur dihitung di CROSS APPLY (k.lat, k.lng) karena harus berpasangan
		if strings.EqualFold(c.Name, "Latitude") {
			return "CASE WHEN k.lat IS NULL OR k.lng IS NULL OR (k.lat = 0 AND k.lng = 0) THEN NULL ELSE CAST(k.lat AS DECIMAL(10, 7)) END"
		}
		return "CASE WHEN k.lat IS NULL OR k.lng IS NULL OR (k.lat = 0 AND k.lng = 0) THEN NULL ELSE CAST(k.lng AS DECIMAL(10, 7)) END"
	default:
		e = fmt.Sprintf("dbo.dm_teks(CONVERT(NVARCHAR(MAX), %s), %d)", raw, c.Size)
	}
	for _, k := range ds.Keep {
		if strings.EqualFold(k, c.Name) {
			// kolom yang diisi manual di tabel tujuan: nilai lama dipertahankan per kunci bila sumber mengirim NULL (digitalisasi.replaceTable)
			kunci, _ := ds.Column(ds.KeyColumn)
			return fmt.Sprintf("COALESCE(%s, (SELECT TOP (1) o.%s FROM dbo.%s AS o WHERE o.%s = %s ORDER BY o.id DESC))",
				e, kut(c.Name), ds.Table, kut(ds.KeyColumn), ekspresiKolom(kunci, digitalisasi.Dataset{}, ""))
		}
	}
	return e
}

// ekspresiBatas: lintang atau bujur (kolom mentah raw, jenis tp) yang sudah dibatasi |nilai| <= batas; kolom FLOAT dipakai langsung sebagai bilangan, bukan lewat teks.
func ekspresiBatas(raw, tp string, batas int) string {
	return fmt.Sprintf("dbo.dm_koordinat(CASE WHEN %[2]s = 1 THEN NULL ELSE CONVERT(NVARCHAR(4000), %[1]s, 3) END, CASE WHEN %[2]s = 1 THEN CAST(%[1]s AS FLOAT) END, %[3]d)",
		raw, tp, batas)
}

// terapkanKoordinat: klausa CROSS APPLY yang menghitung k.lat dan k.lng (lintang dan bujur yang sudah divalidasi) dari kolom mentah #raw; kosong untuk dataset tanpa koordinat.
// tpLat dan tpLng: variabel jenis kolom mentah lintang dan bujur.
func terapkanKoordinat(ds digitalisasi.Dataset, tpLat, tpLng string) string {
	if lat, lng := adaKoordinat(ds); !lat || !lng {
		return ""
	}
	cl, _ := ds.Column("Latitude")
	cg, _ := ds.Column("Longitude")
	return fmt.Sprintf("CROSS APPLY (SELECT lat = %s, lng = %s) AS k\n",
		ekspresiBatas("r."+kut(cl.SourceName()), tpLat, 90), ekspresiBatas("r."+kut(cg.SourceName()), tpLng, 180))
}

// tipeTeksAman: system_type_id kolom mentah yang bentuk tekstualnya sama antara aplikasi (driver Go) dan CONVERT(NVARCHAR) SQL Server: teks, bilangan bulat, bit, DECIMAL/NUMERIC.
// Kolom teks tujuan dari tipe lain (FLOAT, tanggal, uang, ...) ditolak dengan galat yang jelas, bukan diam-diam ditulis dalam bentuk yang berbeda.
var tipeTeksAman = []int{35, 48, 52, 56, 99, 104, 106, 108, 127, 167, 175, 231, 239}

// deklarasiJenis: pernyataan yang membaca tipe kolom mentah dari katalog tempdb, menolak kolom teks bertipe tak aman, dan mengisi variabel jenis (varJenis) tiap kolom angka.
func deklarasiJenis(ds digitalisasi.Dataset) string {
	lit := func(s string) string { return "N'" + strings.ReplaceAll(s, "'", "''") + "'" }
	var b strings.Builder
	fmt.Fprintf(&b, "-- Tipe kolom mentah (system_type_id): kolom FLOAT/REAL (62, 59) dibaca sebagai bilangan, bukan sebagai teks.\n")
	fmt.Fprintf(&b, "DECLARE @jenis TABLE (kolom SYSNAME NOT NULL PRIMARY KEY, tipe INT NOT NULL);\n")
	fmt.Fprintf(&b, "INSERT @jenis (kolom, tipe) SELECT c.name, c.system_type_id FROM tempdb.sys.columns AS c WHERE c.object_id = OBJECT_ID(N'tempdb..#raw_%s');\n", ds.Key)
	var teks, decl []string
	for i, c := range ds.Columns {
		switch c.Kind {
		case digitalisasi.Int, digitalisasi.BigInt, digitalisasi.Decimal, digitalisasi.Coord:
			decl = append(decl, fmt.Sprintf("%s TINYINT = CASE WHEN ISNULL((SELECT tipe FROM @jenis WHERE kolom = %s), 0) IN (62, 59) THEN 1 ELSE 0 END", varJenis(i), lit(c.SourceName())))
		default:
			teks = append(teks, lit(c.SourceName()))
		}
	}
	var aman []string
	for _, t := range tipeTeksAman {
		aman = append(aman, strconv.Itoa(t))
	}
	fmt.Fprintf(&b, "DECLARE %s,\n        @salah NVARCHAR(MAX) = (SELECT STRING_AGG(kolom, N', ') FROM @jenis WHERE kolom IN (%s) AND tipe NOT IN (%s));\n",
		strings.Join(decl, ",\n        "), strings.Join(teks, ", "), strings.Join(aman, ", "))
	fmt.Fprintf(&b, "IF @salah IS NOT NULL\nBEGIN\n    SET @salah = N'%s: kolom teks dari SLDK bertipe FLOAT, tanggal, uang, atau tipe lain yang bentuk tekstualnya berbeda antara aplikasi dan SQL Server (' + @salah + N'). Ubah ke teks pada query sumber. TIDAK ADA yang diubah.';\n    THROW 50050, @salah, 1;\nEND;\n\n", ds.Table)
	return b.String()
}

// prosedurMuat: prosedur yang menjalankan query asli sebuah dataset dan memuat hasilnya (terkonversi) ke tabel staging.
func prosedurMuat(ds digitalisasi.Dataset) (string, error) {
	q, err := ds.Query()
	if err != nil {
		return "", err
	}
	q, err = queryDenganInto(q, ds.Key)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "-- %s: %s\n", ds.Table, ds.Description)
	fmt.Fprintf(&b, "CREATE OR ALTER PROCEDURE dbo.datamart_dg_muat_%s @run BIGINT\nAS\nBEGIN\n    SET NOCOUNT ON;\n\n", ds.Key)
	fmt.Fprintf(&b, "-- ---------------------------------------------------------------------------------------------------\n")
	fmt.Fprintf(&b, "-- QUERY ASLI (queries/%s). Satu-satunya tambahan: INTO #raw_%s pada SELECT akhir.\n", ds.QueryFile, ds.Key)
	fmt.Fprintf(&b, "-- ---------------------------------------------------------------------------------------------------\n")
	b.WriteString(strings.TrimRight(q, "\n") + "\n\n")

	fmt.Fprintf(&b, "-- ---------------------------------------------------------------------------------------------------\n")
	fmt.Fprintf(&b, "-- KONVERSI ke kolom %s (padanan digitalisasi.convertValue dan fixCoordinates)\n", ds.Table)
	fmt.Fprintf(&b, "-- ---------------------------------------------------------------------------------------------------\n")
	b.WriteString(deklarasiJenis(ds))
	kolom := append(kolomData(ds), kut("id_sinkron"))
	fmt.Fprintf(&b, "INSERT INTO DIGITALISASI_STG.%s\n    (%s)\n", ds.Table, strings.Join(kolom, ", "))
	b.WriteString("SELECT\n")
	var ekspr []string
	for i, c := range ds.Columns {
		ekspr = append(ekspr, "    "+ekspresiKolom(c, ds, varJenis(i))+" -- "+c.Name)
	}
	ekspr = append(ekspr, "    @run -- id_sinkron")
	for i := range ekspr {
		if i < len(ekspr)-1 {
			// koma ditaruh sebelum komentar
			j := strings.LastIndex(ekspr[i], " -- ")
			ekspr[i] = ekspr[i][:j] + "," + ekspr[i][j:]
		}
	}
	b.WriteString(strings.Join(ekspr, "\n") + "\n")
	fmt.Fprintf(&b, "FROM #raw_%s AS r\n", ds.Key)
	if lat, lng := adaKoordinat(ds); lat && lng {
		var iLat, iLng int
		for i, c := range ds.Columns {
			switch {
			case strings.EqualFold(c.Name, "Latitude"):
				iLat = i
			case strings.EqualFold(c.Name, "Longitude"):
				iLng = i
			}
		}
		b.WriteString(terapkanKoordinat(ds, varJenis(iLat), varJenis(iLng)))
	}
	if o, ada := urutanTambahan[ds.Key]; ada {
		fmt.Fprintf(&b, "ORDER BY %s\n", o)
	}
	b.WriteString(";\n\n")
	fmt.Fprintf(&b, "DROP TABLE IF EXISTS #raw_%s;\nEND;\n", ds.Key)
	return b.String(), nil
}

// Potongan statis.
const kepala = `-- ============================================================================================================================================
-- DATAMART DIGITALISASI ASET: membentuk tabel DIGITALISASI_* langsung dari SLDK
--
-- FILE INI DIHASILKAN dari backend/digitalisasi (datasets.go dan queries/*.sql) oleh backend/datamart/digitalisasi.go. Jangan disunting manual: ubah sumbernya lalu jalankan
--   go test ./datamart -run TestSkripDigitalisasiSesuaiGenerator -update
-- (tes biasa gagal bila berkas ini tidak sejalan dengan aplikasi).
--
-- APA INI
--   Satu script yang, dijalankan di database DATAMART, (1) membaca SLDK dengan ketujuh query asli Digitalisasi Aset (disisipkan apa adanya di bawah), (2) mengubah hasilnya persis
--   seperti yang dilakukan aplikasi saat sinkronisasi (pangkas spasi, kosong menjadi NULL, potong sesuai panjang kolom, angka berkoma desimal, koordinat divalidasi dan berpasangan),
--   dan (3) mengisi tujuh tabel DIGITALISASI_* yang kolom dan tipenya sama dengan tabel aplikasi (migrasi 020). Hasilnya: data datamart = data yang akan disimpan aplikasi.
--   Aplikasi lalu cukup menarik dari tabel DIGITALISASI_* datamart ini tanpa menyentuh SLDK.
--
-- CARA MEMAKAI (SSMS atau sqlcmd; tidak perlu mode SQLCMD)
--   1. Buka database DATAMART (terpisah dari database PASTI dan dari SLDK) dan pastikan server ini dapat membaca SLDK.
--   2. Ubah satu baris @Sumber di bawah: awalan database SLDK, mis. [Interchange] (satu server) atau [LNK_SLDK].[Interchange] (linked server). Kosongkan bila script ini
--      dijalankan di dalam database SLDK sendiri.
--   3. Jalankan seluruh script. Pertama kali ia membuat tabel; menjalankannya lagi menyegarkan isinya.
--   Sesudah terpasang, penyegaran berikutnya cukup:   EXEC dbo.datamart_digitalisasi_segarkan;   (langkah Transact-SQL biasa pada SQL Agent, di luar jam kerja).
--   Memberi hak baca kepada pengguna aplikasi:        GRANT SELECT ON dbo.DIGITALISASI_SATKER TO <pengguna>; ... (hak tabel tidak hilang saat penyegaran), serta SELECT pada dbo.datamart_digitalisasi_run.
--
-- JAMINAN
--   * SLDK hanya DIBACA (lewat sinonim DJKN.*). Tabel DIGITALISASI_* diisi lewat staging dan diganti dalam SATU transaksi setelah ketujuh dataset berhasil; bila ada yang gagal atau
--     SLDK mengembalikan 0 baris sementara datamart sudah berisi, isi datamart yang lama TIDAK berubah.
--   * Satu proses pada satu waktu. Jejak tiap proses: dbo.datamart_digitalisasi_run (mulai, selesai, status, pesan) dan dbo.datamart_digitalisasi_tabel (jumlah baris dan berkoordinat).
--     Kolom id_sinkron tiap baris = id proses penyegarannya. Waktu data datamart: SELECT MAX(mulai) FROM dbo.datamart_digitalisasi_run WHERE status = N'sukses'.
--   * Script menolak berjalan di database yang berisi digitalisasi_sync_log (database aplikasi PASTI).
--   * Kolom Nama_Penghuni (DIGITALISASI_RUMAH_NEGARA) adalah data pribadi: batasi akses ke datamart seperti ke database PASTI.
--   * Kolom Status_Gedung_Kantor dan Foto pada DIGITALISASI_SATKER diisi manual (query selalu mengirim NULL): nilainya dipertahankan per Kode_Satker, seperti pada aplikasi.
--   * Kesamaan dengan aplikasi dibuktikan tes (bandingan tabel demi tabel, kolom demi kolom). Batasnya: angka dengan lebih dari 15 digit penting (aplikasi membacanya sebagai float64,
--     datamart mempertahankan digitnya), bilangan bulat di atas 2^53, dan bentuk heksadesimal Go (0x1p3) pada kolom angka bertipe teks. Rincian: docs/datamart-digitalisasi.md.
--
-- Syarat: SQL Server 2017 atau lebih baru (STRING_AGG, TRIM), hak membaca tabel SLDK, dan hak CREATE TABLE/SCHEMA/PROCEDURE/FUNCTION/SYNONYM di datamart.
-- ============================================================================================================================================

SET NOCOUNT ON;

DECLARE @Sumber NVARCHAR(300) = N'%[1]s';   -- <<< UBAH DI SINI: awalan database SLDK ([Interchange], [LNK_SLDK].[Interchange], atau kosong bila dijalankan di database SLDK sendiri)
DECLARE @sql NVARCHAR(MAX) = N'';

-- Pengaman: jangan pernah dijalankan di database aplikasi PASTI (tabel DIGITALISASI_* aplikasi akan tertimpa).
IF OBJECT_ID(N'dbo.digitalisasi_sync_log', N'U') IS NOT NULL
BEGIN
    RAISERROR (N'Database ini tampaknya database aplikasi PASTI (ada tabel digitalisasi_sync_log). Jalankan script ini di database datamart yang terpisah. TIDAK ADA yang diubah.', 16, 1);
    SET NOEXEC ON;   -- batch berikutnya hanya dikompilasi, tidak dijalankan (dimatikan lagi di akhir script)
END;

-- Objek SLDK yang dibaca ketujuh query (skema DJKN).
DECLARE @objek TABLE (nama SYSNAME NOT NULL PRIMARY KEY);
INSERT @objek (nama) VALUES
%[2]s;

-- Pengaman @Sumber: diperiksa SEBELUM ada yang dibuat.
IF NULLIF(LTRIM(RTRIM(@Sumber)), N'') IS NULL
BEGIN
    -- Dijalankan DI database SLDK sendiri: objek DJKN.* harus ada sebagai tabel/view sungguhan.
    IF EXISTS (SELECT 1 FROM @objek AS o WHERE OBJECT_ID(N'DJKN.' + o.nama) IS NULL)
    BEGIN
        RAISERROR (N'@Sumber kosong, tetapi objek DJKN.SIMAN2_* tidak ada di database ini. Isi @Sumber dengan awalan database SLDK, mis. [Interchange]. TIDAK ADA yang diubah.', 16, 1);
        SET NOEXEC ON;
    END;
END
ELSE IF EXISTS (SELECT 1 FROM sys.objects AS x WHERE x.schema_id = SCHEMA_ID(N'DJKN') AND x.type IN (N'U', N'V') AND x.name IN (SELECT nama FROM @objek))
BEGIN
    RAISERROR (N'Skema DJKN di database ini sudah berisi tabel/view sungguhan (bukan sinonim), mis. dari script datamart lain. Gunakan database datamart yang bersih. TIDAK ADA yang diubah.', 16, 1);
    SET NOEXEC ON;
END;

IF SCHEMA_ID(N'DJKN') IS NULL EXEC (N'CREATE SCHEMA DJKN');
IF SCHEMA_ID(N'DIGITALISASI_STG') IS NULL EXEC (N'CREATE SCHEMA DIGITALISASI_STG');

-- Jejak proses.
IF OBJECT_ID(N'dbo.datamart_digitalisasi_run', N'U') IS NULL
    CREATE TABLE dbo.datamart_digitalisasi_run (
        id      BIGINT        IDENTITY(1, 1) NOT NULL CONSTRAINT pk_datamart_digitalisasi_run PRIMARY KEY,
        mulai   DATETIME2(3)  NOT NULL CONSTRAINT df_datamart_digitalisasi_run_mulai DEFAULT SYSUTCDATETIME(),   -- UTC
        selesai DATETIME2(3)  NULL,
        status  NVARCHAR(10)  NOT NULL,                                                                          -- berjalan | sukses | gagal
        sumber  NVARCHAR(400) NULL,                                                                              -- SLDK yang dibaca (sinonim DJKN.SIMAN2_M_ASET)
        pesan   NVARCHAR(2000) NULL
    );

IF OBJECT_ID(N'dbo.datamart_digitalisasi_tabel', N'U') IS NULL
    CREATE TABLE dbo.datamart_digitalisasi_tabel (
        run_id    BIGINT        NOT NULL,
        tabel     NVARCHAR(128) NOT NULL,
        baris     BIGINT        NOT NULL,
        koordinat BIGINT        NULL,   -- baris yang punya koordinat valid (tabel bergeo saja)
        CONSTRAINT pk_datamart_digitalisasi_tabel PRIMARY KEY (run_id, tabel),
        CONSTRAINT fk_datamart_digitalisasi_tabel_run FOREIGN KEY (run_id) REFERENCES dbo.datamart_digitalisasi_run (id)
    );

`

const sinonim = `
-- Sinonim DJKN.* ke objek SLDK yang dibaca query. Query asli di bawah memakai nama DJKN.SIMAN2_* apa adanya, sehingga tidak perlu diubah satu karakter pun.
-- (@Sumber kosong = objek DJKN.* sudah ada di database ini: tidak ada sinonim.)
IF NULLIF(LTRIM(RTRIM(@Sumber)), N'') IS NOT NULL
BEGIN
    SET @sql = N'';
    SELECT @sql += N'DROP SYNONYM DJKN.' + QUOTENAME(name) + N';' FROM sys.synonyms WHERE schema_id = SCHEMA_ID(N'DJKN');
    EXEC (@sql);
    SET @sql = N'';
    SELECT @sql += N'CREATE SYNONYM DJKN.' + QUOTENAME(nama) + N' FOR ' + @Sumber + N'.DJKN.' + QUOTENAME(nama) + N';' FROM @objek;
    EXEC (@sql);
END;
GO

`

const fungsi = `-- ============================================================================================================================================
-- FUNGSI KONVERSI: padanan digitalisasi.convertValue / parseNumber / fixCoordinates pada aplikasi (backend/digitalisasi/convert.go)
-- Pola LIKE/PATINDEX dan TRIM memakai COLLATE Latin1_General_BIN2 (urutan kode): collation bawaan database tidak boleh memengaruhi hasil.
-- ============================================================================================================================================

-- strings.TrimSpace (Go): U+0009-000D, U+0020, U+0085, U+00A0, U+1680, U+2000-200A, U+2028, U+2029, U+202F, U+205F, U+3000.
CREATE OR ALTER FUNCTION dbo.dm_pangkas (@v NVARCHAR(MAX))
RETURNS NVARCHAR(MAX)
AS
BEGIN
    RETURN TRIM(NCHAR(9) + NCHAR(10) + NCHAR(11) + NCHAR(12) + NCHAR(13) + NCHAR(32) + NCHAR(133) + NCHAR(160) + NCHAR(5760)
              + NCHAR(8192) + NCHAR(8193) + NCHAR(8194) + NCHAR(8195) + NCHAR(8196) + NCHAR(8197) + NCHAR(8198) + NCHAR(8199) + NCHAR(8200) + NCHAR(8201) + NCHAR(8202)
              + NCHAR(8232) + NCHAR(8233) + NCHAR(8239) + NCHAR(8287) + NCHAR(12288) FROM @v COLLATE Latin1_General_BIN2);
END;
GO

-- Kolom teks: dipangkas, kosong menjadi NULL, dipotong ke @maks unit UTF-16 (0 = tanpa batas) tanpa memotong pasangan pengganti (surrogate) di tengah:
-- bila unit terakhir hasil potongan (posisi @maks) adalah separuh pertama sebuah pasangan, ia dibuang juga.
-- Ditulis sebagai satu ekspresi supaya SQL Server 2019+ dapat menyisipkannya ke query (jauh lebih cepat daripada panggilan fungsi per baris).
CREATE OR ALTER FUNCTION dbo.dm_teks (@v NVARCHAR(MAX), @maks INT)
RETURNS NVARCHAR(MAX)
AS
BEGIN
    RETURN (SELECT CASE
                WHEN x.s IS NULL OR DATALENGTH(x.s) = 0 THEN NULL
                WHEN @maks > 0 AND DATALENGTH(x.s) > @maks * 2
                    THEN CASE WHEN UNICODE(SUBSTRING(x.s, @maks, 1)) BETWEEN 55296 AND 56319 THEN SUBSTRING(x.s, 1, @maks - 1) ELSE SUBSTRING(x.s, 1, @maks) END
                ELSE x.s
            END
            FROM (SELECT dbo.dm_pangkas(@v) AS s) AS x);
END;
GO

-- parseNumber (Go), langkah 1, jalur lengkap: bentuk "polos" sebuah angka dalam teks = tanda minus (bila negatif), digit, dan paling banyak satu titik desimal, tanpa eksponen
-- ("-1.5E+3" -> "-1500", " 0,00005 " -> "0.00005"). NULL bila bukan angka menurut strconv.ParseFloat: tanda +/-, digit, satu titik, eksponen e/E, garis bawah di antara dua digit,
-- dan (aturan aplikasi) satu koma menjadi titik desimal bila tidak ada titik. Tidak dikenali (NULL): bentuk heksadesimal (0x1p3), Inf/NaN, dan angka di atas 10^400.
CREATE OR ALTER FUNCTION dbo.dm_polos_lambat (@v NVARCHAR(MAX))
RETURNS NVARCHAR(MAX)
AS
BEGIN
    DECLARE @t NVARCHAR(MAX) = dbo.dm_pangkas(@v), @tanda NVARCHAR(1) = N'', @m NVARCHAR(MAX), @x NVARCHAR(MAX) = N'', @pe INT, @titik INT, @digit NVARCHAR(MAX), @geser INT = 0, @pos INT, @xs INT = 1;

    IF @t IS NULL OR DATALENGTH(@t) = 0 OR DATALENGTH(@t) > 800 RETURN NULL;
    IF CHARINDEX(N',', @t) > 0 AND CHARINDEX(N'.', @t) = 0 SET @t = STUFF(@t, CHARINDEX(N',', @t), 1, N'.');

    IF CHARINDEX(N'_', @t) > 0
    BEGIN
        -- garis bawah hanya sah tepat di antara dua digit (strconv.underscoreOK)
        IF @t COLLATE Latin1_General_BIN2 LIKE N'%[^0-9]\_%' ESCAPE N'\' OR @t COLLATE Latin1_General_BIN2 LIKE N'\_%' ESCAPE N'\'
           OR @t COLLATE Latin1_General_BIN2 LIKE N'%\_[^0-9]%' ESCAPE N'\' OR @t COLLATE Latin1_General_BIN2 LIKE N'%\_' ESCAPE N'\'
            RETURN NULL;
        SET @t = REPLACE(@t, N'_', N'');
    END;

    IF UNICODE(LEFT(@t, 1)) IN (43, 45)
    BEGIN
        IF UNICODE(LEFT(@t, 1)) = 45 SET @tanda = N'-';
        SET @t = SUBSTRING(@t, 2, 4000);
    END;

    SET @pe = PATINDEX(N'%[eE]%' COLLATE Latin1_General_BIN2, @t);
    IF @pe > 0
    BEGIN
        SET @m = LEFT(@t, @pe - 1);
        SET @x = SUBSTRING(@t, @pe + 1, 4000);
    END
    ELSE SET @m = @t;

    -- mantisa: hanya digit dan paling banyak satu titik, minimal satu digit
    IF PATINDEX(N'%[^0-9.]%' COLLATE Latin1_General_BIN2, @m) > 0 OR PATINDEX(N'%[0-9]%' COLLATE Latin1_General_BIN2, @m) = 0 RETURN NULL;
    SET @titik = CHARINDEX(N'.', @m);
    IF @titik > 0 AND CHARINDEX(N'.', @m, @titik + 1) > 0 RETURN NULL;

    -- eksponen: tanda opsional lalu 1-3 digit
    IF @pe > 0
    BEGIN
        IF UNICODE(LEFT(@x, 1)) IN (43, 45)
        BEGIN
            IF UNICODE(LEFT(@x, 1)) = 45 SET @xs = -1;
            SET @x = SUBSTRING(@x, 2, 4000);
        END;
        IF DATALENGTH(@x) = 0 OR DATALENGTH(@x) > 6 OR PATINDEX(N'%[^0-9]%' COLLATE Latin1_General_BIN2, @x) > 0 RETURN NULL;
        SET @geser = @xs * CAST(@x AS INT);
    END;

    -- geser titik desimal sebanyak eksponen
    SET @digit = REPLACE(@m, N'.', N'');
    SET @pos = CASE WHEN @titik > 0 THEN @titik - 1 ELSE LEN(@digit) END + @geser;
    IF @pos > 400 RETURN NULL;   -- di atas 10^400 (melampaui FLOAT)
    IF @pos < -323 RETURN N'0';   -- di bawah bilangan FLOAT terkecil (sekitar 5E-324): strconv.ParseFloat membulatkannya ke 0
    RETURN @tanda + CASE
        WHEN @pos <= 0 THEN N'0.' + REPLICATE(N'0', -@pos) + @digit
        WHEN @pos >= LEN(@digit) THEN @digit + REPLICATE(N'0', @pos - LEN(@digit))
        ELSE LEFT(@digit, @pos) + N'.' + SUBSTRING(@digit, @pos + 1, 4000)
    END;
END;
GO

-- parseNumber (Go), langkah 1: bentuk "polos" angka (hasil sama dengan dm_polos_lambat). Bentuk yang paling umum, [-]digit[.digit] atau dengan satu koma desimal, langsung dikembalikan
-- dalam satu ekspresi bertipe terbatas (dapat disisipkan ke query oleh SQL Server 2019+, sekitar 2x lebih cepat daripada jalur lengkap); bentuk lain (eksponen, tanda +, garis bawah, dan
-- sebagainya) serta teks panjang lewat dm_polos_lambat.
CREATE OR ALTER FUNCTION dbo.dm_pangkas_pendek (@v NVARCHAR(100))
RETURNS NVARCHAR(100)
AS
BEGIN
    RETURN TRIM(NCHAR(9) + NCHAR(10) + NCHAR(11) + NCHAR(12) + NCHAR(13) + NCHAR(32) + NCHAR(133) + NCHAR(160) + NCHAR(5760)
              + NCHAR(8192) + NCHAR(8193) + NCHAR(8194) + NCHAR(8195) + NCHAR(8196) + NCHAR(8197) + NCHAR(8198) + NCHAR(8199) + NCHAR(8200) + NCHAR(8201) + NCHAR(8202)
              + NCHAR(8232) + NCHAR(8233) + NCHAR(8239) + NCHAR(8287) + NCHAR(12288) FROM @v COLLATE Latin1_General_BIN2);
END;
GO

CREATE OR ALTER FUNCTION dbo.dm_polos (@v NVARCHAR(MAX))
RETURNS NVARCHAR(MAX)
AS
BEGIN
    RETURN (SELECT CASE
                WHEN @v IS NULL THEN NULL
                WHEN DATALENGTH(@v) > 200 THEN dbo.dm_polos_lambat(@v)
                WHEN x.s IS NULL OR DATALENGTH(x.s) = 0 THEN NULL
                WHEN DATALENGTH(x.w) >= 2
                     AND (x.w LIKE N'[0-9]' OR x.w LIKE N'[0-9]%[0-9]')
                     AND x.w NOT LIKE N'%[^0-9.]%'
                     AND LEN(x.w) - LEN(REPLACE(x.w, N'.', N'')) <= 1
                    THEN CAST(x.tanda + x.w AS NVARCHAR(MAX))
                ELSE dbo.dm_polos_lambat(@v)
            END
            FROM (SELECT q2.s, q2.tanda, w = CAST(CASE WHEN CHARINDEX(N'.', q2.u) = 0 THEN REPLACE(q2.u, N',', N'.') ELSE q2.u END AS NVARCHAR(100)) COLLATE Latin1_General_BIN2
                  FROM (SELECT q1.s,
                               tanda = CAST(CASE WHEN LEFT(q1.s, 1) COLLATE Latin1_General_BIN2 = N'-' THEN N'-' ELSE N'' END AS NVARCHAR(1)),
                               u = CAST(CASE WHEN LEFT(q1.s, 1) COLLATE Latin1_General_BIN2 = N'-' THEN SUBSTRING(q1.s, 2, 100) ELSE q1.s END AS NVARCHAR(100))
                        FROM (SELECT dbo.dm_pangkas_pendek(CAST(@v AS NVARCHAR(100))) AS s) AS q1) AS q2) AS x);
END;
GO

-- parseNumber (Go): angka (FLOAT) dari teks; NULL bila bukan angka.
CREATE OR ALTER FUNCTION dbo.dm_angka (@v NVARCHAR(MAX))
RETURNS FLOAT
AS
BEGIN
    RETURN TRY_CAST(dbo.dm_polos(@v) AS FLOAT);
END;
GO

-- Kolom INT/BIGINT: bilangan bulat; bukan angka atau berpecahan -> NULL.
CREATE OR ALTER FUNCTION dbo.dm_bilangan (@v NVARCHAR(MAX))
RETURNS BIGINT
AS
BEGIN
    RETURN (SELECT CASE
                WHEN x.p IS NULL THEN NULL
                WHEN x.t = 0 THEN TRY_CAST(x.p AS BIGINT)
                WHEN SUBSTRING(x.p, x.t + 1, 4000) COLLATE Latin1_General_BIN2 LIKE N'%[^0]%' THEN NULL
                ELSE TRY_CAST(LEFT(x.p, x.t - 1) AS BIGINT)
            END
            FROM (SELECT q.p, t = CHARINDEX(N'.', q.p) FROM (SELECT dbo.dm_polos(@v) AS p) AS q) AS x);
END;
GO

-- Kolom DECIMAL(28, @skala): nilai angka dibulatkan SEKALI ke @skala (0-6) dari digit aslinya, seperti aplikasi yang mengirim teks angka ke kolom DECIMAL.
CREATE OR ALTER FUNCTION dbo.dm_desimal (@v NVARCHAR(MAX), @skala INT)
RETURNS DECIMAL(38, 6)
AS
BEGIN
    RETURN (SELECT CASE
                WHEN x.p IS NULL THEN NULL
                WHEN @skala = 0 THEN TRY_CAST(TRY_CAST(x.p AS DECIMAL(38, 0)) AS DECIMAL(38, 6))
                WHEN @skala = 1 THEN TRY_CAST(TRY_CAST(x.p AS DECIMAL(38, 1)) AS DECIMAL(38, 6))
                WHEN @skala = 2 THEN TRY_CAST(TRY_CAST(x.p AS DECIMAL(38, 2)) AS DECIMAL(38, 6))
                WHEN @skala = 3 THEN TRY_CAST(TRY_CAST(x.p AS DECIMAL(38, 3)) AS DECIMAL(38, 6))
                WHEN @skala = 4 THEN TRY_CAST(TRY_CAST(x.p AS DECIMAL(38, 4)) AS DECIMAL(38, 6))
                WHEN @skala = 5 THEN TRY_CAST(TRY_CAST(x.p AS DECIMAL(38, 5)) AS DECIMAL(38, 6))
                WHEN @skala = 6 THEN TRY_CAST(x.p AS DECIMAL(38, 6))
            END
            FROM (SELECT dbo.dm_polos(@v) AS p) AS x);
END;
GO

-- Kolom sumber bertipe FLOAT/REAL dibaca aplikasi sebagai bilangan (REAL diperlebar ke FLOAT), lalu ditulis sebagai angka terpendek yang kembali ke bilangan yang sama
-- (strconv.FormatFloat 'f', -1). Padanannya: format "R" (budaya tetap en-US), yang mencoba 15 digit lalu langsung 17 digit; Go juga mencoba 16 digit, jadi 16 digit dicoba di sini.
CREATE OR ALTER FUNCTION dbo.dm_float_ke_teks (@f FLOAT)
RETURNS NVARCHAR(60)
AS
BEGIN
    DECLARE @r NVARCHAR(60) = FORMAT(@f, N'R', N'en-US'), @d NVARCHAR(60), @s2 VARCHAR(40);
    IF @r IS NULL RETURN NULL;
    -- banyaknya digit penting pada format "R"
    SET @d = @r COLLATE Latin1_General_BIN2;
    IF CHARINDEX(N'E', @d) > 0 SET @d = LEFT(@d, CHARINDEX(N'E', @d) - 1);
    SET @d = REPLACE(REPLACE(@d, N'-', N''), N'.', N'');
    SET @d = SUBSTRING(@d, PATINDEX(N'%[^0]%' COLLATE Latin1_General_BIN2, @d + N'1'), 60);
    IF LEN(@d) > 15
    BEGIN
        SET @s2 = CONVERT(VARCHAR(40), @f, 2);   -- tepat 16 digit
        IF TRY_CAST(@s2 AS FLOAT) = @f RETURN @s2;
    END;
    RETURN @r;
END;
GO

-- Koordinat: angka dengan |nilai| <= @batas (90 untuk lintang, 180 untuk bujur); selain itu NULL. Kolom FLOAT (@flt) dipakai apa adanya, selain itu teks (@teks) dibaca.
-- Kepasangan dan (0,0) diperiksa pada INSERT.
CREATE OR ALTER FUNCTION dbo.dm_koordinat (@teks NVARCHAR(MAX), @flt FLOAT, @batas FLOAT)
RETURNS FLOAT
AS
BEGIN
    RETURN (SELECT CASE WHEN x.f IS NULL OR ABS(x.f) > @batas THEN NULL ELSE x.f END
            FROM (SELECT CASE WHEN @flt IS NOT NULL THEN @flt ELSE dbo.dm_angka(@teks) END AS f) AS x);
END;
GO

-- ============================================================================================================================================
-- QUERY ASLI PER DATASET + KONVERSI
-- ============================================================================================================================================

`

// SkripDigitalisasi membangkitkan isi buat_datamart_digitalisasi.sql.
func SkripDigitalisasi() (string, error) {
	objek, err := ObjekSumber()
	if err != nil {
		return "", err
	}
	var nilai []string
	for _, o := range objek {
		nilai = append(nilai, fmt.Sprintf("    (N'%s')", o))
	}

	var b strings.Builder
	fmt.Fprintf(&b, kepala, SumberBawaan, strings.Join(nilai, ",\n"))

	// tabel hidup (dbo); dibuat bila belum ada, supaya prosedur dapat membaca nilai manual lama
	b.WriteString("-- Tabel DIGITALISASI_* datamart (sama dengan migrasi 020 aplikasi); dibuat bila belum ada.\n")
	for _, ds := range digitalisasi.Datasets {
		fmt.Fprintf(&b, "IF OBJECT_ID(N'dbo.%s', N'U') IS NULL\nBEGIN\n%s", ds.Table, ddlTabel(ds, "dbo", "    "))
		if idx := ddlIndeks(ds, "dbo", "    "); idx != "" {
			b.WriteString("\n" + idx)
		}
		b.WriteString("END;\n\n")
	}

	b.WriteString(sinonim)
	b.WriteString(fungsi)

	for _, ds := range digitalisasi.Datasets {
		p, err := prosedurMuat(ds)
		if err != nil {
			return "", err
		}
		b.WriteString(p)
		b.WriteString("GO\n\n")
	}

	b.WriteString(orkestrator())
	b.WriteString("-- Jalankan sekarang (pemasangan pertama membuat tabel; pemasangan ulang menyegarkannya).\nEXEC dbo.datamart_digitalisasi_segarkan;\nGO\n\nSET NOEXEC OFF;\nGO\n")
	return b.String(), nil
}

// orkestrator: prosedur induk (kunci, jejak, staging, pemanggilan tujuh prosedur muat, pengaman, penggantian isi, pencatatan).
func orkestrator() string {
	var b strings.Builder
	b.WriteString(`-- ============================================================================================================================================
-- PROSEDUR INDUK: dbo.datamart_digitalisasi_segarkan
-- ============================================================================================================================================
CREATE OR ALTER PROCEDURE dbo.datamart_digitalisasi_segarkan
AS
BEGIN
    SET NOCOUNT ON;
    SET XACT_ABORT ON;

    DECLARE @kunci INT, @run BIGINT, @sql NVARCHAR(MAX), @pesan NVARCHAR(2000), @total BIGINT, @sumber NVARCHAR(400);

    IF OBJECT_ID(N'dbo.digitalisasi_sync_log', N'U') IS NOT NULL
        THROW 50030, N'Database ini tampaknya database aplikasi PASTI (ada tabel digitalisasi_sync_log); datamart harus database terpisah. TIDAK ADA yang diubah.', 1;

    -- Satu proses pada satu waktu (kunci aplikasi bertipe sesi; lepas sendiri bila sambungan putus).
    EXEC @kunci = sp_getapplock @Resource = N'datamart_digitalisasi', @LockMode = N'Exclusive', @LockOwner = N'Session', @LockTimeout = 0;
    IF @kunci < 0
        THROW 50040, N'Proses datamart digitalisasi lain sedang berjalan; proses ini dihentikan.', 1;

    BEGIN TRY
        -- Proses sebelumnya yang masih berstatus berjalan padahal kunci kita dapat = terhenti di tengah jalan.
        UPDATE dbo.datamart_digitalisasi_run SET status = N'gagal', selesai = SYSUTCDATETIME(), pesan = N'Terhenti sebelum selesai (proses berhenti atau sambungan putus).' WHERE status = N'berjalan';

        -- Sisa staging dari proses yang gagal.
        SET @sql = N'';
        SELECT @sql += N'DROP TABLE DIGITALISASI_STG.' + QUOTENAME(name) + N';' FROM sys.tables WHERE schema_id = SCHEMA_ID(N'DIGITALISASI_STG');
        EXEC (@sql);

        SELECT @sumber = LEFT(base_object_name, 400) FROM sys.synonyms WHERE schema_id = SCHEMA_ID(N'DJKN') AND name = N'SIMAN2_M_ASET';
        INSERT dbo.datamart_digitalisasi_run (status, sumber) VALUES (N'berjalan', ISNULL(@sumber, N'langsung (database SLDK sendiri)'));
        SET @run = SCOPE_IDENTITY();

        -- Tabel staging (struktur sama dengan tabel hidup).
`)
	for _, ds := range digitalisasi.Datasets {
		b.WriteString(ddlTabel(ds, "DIGITALISASI_STG", "        "))
		b.WriteString("\n")
	}
	b.WriteString("        -- Muat ketujuh dataset ke staging. Galat dari prosedur muat (kolom atau tabel SLDK hilang, akses ditolak, SLDK tidak terjangkau) tercatat dan menghentikan proses.\n")
	for _, ds := range digitalisasi.Datasets {
		fmt.Fprintf(&b, "        EXEC dbo.datamart_dg_muat_%s @run = @run;\n", ds.Key)
	}
	b.WriteString("\n        -- Pengaman: hasil kosong hampir pasti kegagalan di hulu (akses, filter), bukan kenyataan bahwa semua aset hilang. Datamart yang lama tidak diganti.\n")
	for _, ds := range digitalisasi.Datasets {
		fmt.Fprintf(&b, "        IF NOT EXISTS (SELECT 1 FROM DIGITALISASI_STG.%[1]s) AND EXISTS (SELECT 1 FROM dbo.%[1]s)\n", ds.Table)
		fmt.Fprintf(&b, "            THROW 50041, N'%s: SLDK mengembalikan 0 baris sedangkan datamart sudah berisi data; datamart lama TIDAK diganti. Periksa akses ke SLDK dan filter query.', 1;\n", ds.Table)
	}
	b.WriteString("\n        -- Indeks pada staging (sama dengan tabel hidup).\n")
	for _, ds := range digitalisasi.Datasets {
		b.WriteString(ddlIndeks(ds, "DIGITALISASI_STG", "        "))
	}
	b.WriteString("\n        -- Ganti isi tabel hidup dalam SATU transaksi. Hak akses, indeks, dan definisi tabel hidup tidak berubah.\n        BEGIN TRANSACTION;\n")
	for _, ds := range digitalisasi.Datasets {
		cols := strings.Join(append(kolomData(ds), kut("id_sinkron"), kut("synced_at")), ", ")
		fmt.Fprintf(&b, "        TRUNCATE TABLE dbo.%s;\n", ds.Table)
		fmt.Fprintf(&b, "        INSERT INTO dbo.%[1]s (%[2]s)\n        SELECT %[2]s FROM DIGITALISASI_STG.%[1]s ORDER BY id;\n", ds.Table, cols)
	}
	b.WriteString("        COMMIT TRANSACTION;\n\n        -- Jejak jumlah baris.\n")
	for _, ds := range digitalisasi.Datasets {
		koord := "NULL"
		if lat, lng := adaKoordinat(ds); lat && lng {
			koord = "SUM(CASE WHEN Latitude IS NOT NULL THEN 1 ELSE 0 END)"
		}
		fmt.Fprintf(&b, "        INSERT dbo.datamart_digitalisasi_tabel (run_id, tabel, baris, koordinat) SELECT @run, N'%[1]s', COUNT_BIG(*), %[2]s FROM dbo.%[1]s;\n", ds.Table, koord)
	}
	b.WriteString(`
        SELECT @total = SUM(baris) FROM dbo.datamart_digitalisasi_tabel WHERE run_id = @run;
        UPDATE dbo.datamart_digitalisasi_run SET status = N'sukses', selesai = SYSUTCDATETIME(),
               pesan = CONVERT(NVARCHAR(20), @total) + N' baris di ketujuh tabel DIGITALISASI_*; rincian di dbo.datamart_digitalisasi_tabel.'
        WHERE id = @run;

        SET @sql = N'';
        SELECT @sql += N'DROP TABLE DIGITALISASI_STG.' + QUOTENAME(name) + N';' FROM sys.tables WHERE schema_id = SCHEMA_ID(N'DIGITALISASI_STG');
        EXEC (@sql);

        EXEC sp_releaseapplock @Resource = N'datamart_digitalisasi', @LockOwner = N'Session';
        PRINT N'Datamart Digitalisasi Aset selesai. Proses #' + CONVERT(NVARCHAR(20), @run) + N'; rincian: SELECT * FROM dbo.datamart_digitalisasi_tabel WHERE run_id = ' + CONVERT(NVARCHAR(20), @run) + N'.';
    END TRY
    BEGIN CATCH
        IF XACT_STATE() <> 0 ROLLBACK TRANSACTION;

        SET @pesan = LEFT(ISNULL(ERROR_MESSAGE(), N'Galat tidak diketahui') + N' (' + ISNULL(ERROR_PROCEDURE(), N'-') + N', baris ' + CONVERT(NVARCHAR(10), ERROR_LINE()) + N'). Sumber SLDK: ' + ISNULL(@sumber, N'-') + N'.', 2000);
        IF @run IS NOT NULL
            UPDATE dbo.datamart_digitalisasi_run SET status = N'gagal', selesai = SYSUTCDATETIME(), pesan = @pesan WHERE id = @run;

        -- Buang staging yang setengah jadi; tabel hidup (datamart yang berlaku) tidak disentuh.
        BEGIN TRY
            SET @sql = N'';
            SELECT @sql += N'DROP TABLE DIGITALISASI_STG.' + QUOTENAME(name) + N';' FROM sys.tables WHERE schema_id = SCHEMA_ID(N'DIGITALISASI_STG');
            EXEC (@sql);
        END TRY
        BEGIN CATCH
        END CATCH;
        EXEC sp_releaseapplock @Resource = N'datamart_digitalisasi', @LockOwner = N'Session';

        THROW;
    END CATCH;
END;
GO

`)
	return b.String()
}
