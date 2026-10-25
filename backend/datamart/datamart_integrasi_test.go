package datamart

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	_ "github.com/microsoft/go-mssqldb"

	"pasti-v3-backend/digitalisasi"
)

// Tes integrasi SQL Server: hanya berjalan bila PASTI_UJI_MSSQL_DSN diisi, mis.
//
//	sqlserver://localhost:1433?database=master&authenticator=winsspi&encrypt=disable
//
// Tes membuat DUA database sementara bernama uji_dm_* (SLDK palsu dan datamart) dan menghapusnya lagi; pengguna DSN perlu hak CREATE DATABASE. Database yang ada tidak disentuh.
// Pembuktian utamanya: ketujuh query Digitalisasi Aset, tanpa diubah, memberi hasil yang identik dari SLDK palsu dan dari datamart.

func bukaServerUji(t *testing.T) *sql.DB {
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
	for i := 0; i < 4; i++ { // SQL Server lokal kadang menutup sambungan pertama yang baru dibuka
		if err2 = db.Ping(); err2 == nil {
			break
		}
		time.Sleep(300 * time.Millisecond)
	}
	if err2 != nil {
		t.Fatalf("tidak bisa terhubung ke server uji: %v", err2)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func kutipDB(n string) string { return "[" + strings.ReplaceAll(n, "]", "]]") + "]" }

// buatDBUji membuat database sementara dan menghapusnya saat tes selesai.
func buatDBUji(t *testing.T, srv *sql.DB, nama string) string {
	t.Helper()
	ctx := context.Background()
	full := fmt.Sprintf("uji_dm_%s_%d", nama, time.Now().UnixNano()%1_000_000_000)
	if _, err := srv.ExecContext(ctx, "CREATE DATABASE "+kutipDB(full)); err != nil {
		t.Fatalf("tidak bisa membuat database uji (perlu hak CREATE DATABASE): %v", err)
	}
	t.Cleanup(func() {
		srv.ExecContext(ctx, "ALTER DATABASE "+kutipDB(full)+" SET SINGLE_USER WITH ROLLBACK IMMEDIATE")
		srv.ExecContext(ctx, "DROP DATABASE "+kutipDB(full))
	})
	return full
}

var reGO = regexp.MustCompile(`(?mi)^[ \t]*GO[ \t]*\r?$`)

// jalankanBerkas menjalankan berkas SQL di database nama: variabel sqlcmd $(NAMA) diganti, direktif :on error dibuang, dan batch dipisah di baris GO.
func jalankanBerkas(t *testing.T, srv *sql.DB, nama, teks string, vars map[string]string) error {
	t.Helper()
	for k, v := range vars {
		teks = strings.ReplaceAll(teks, "$("+k+")", v)
	}
	if strings.Contains(tanpaKomentar(teks), "$(") {
		t.Fatal("masih ada variabel sqlcmd yang belum diisi")
	}
	var bersih []string
	for _, l := range strings.Split(teks, "\n") {
		if strings.HasPrefix(strings.TrimSpace(strings.ToLower(l)), ":on error") {
			continue
		}
		bersih = append(bersih, l)
	}
	ctx := context.Background()
	conn, err := srv.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, "USE "+kutipDB(nama)); err != nil {
		t.Fatal(err)
	}
	for _, batch := range reGO.Split(strings.Join(bersih, "\n"), -1) {
		if strings.TrimSpace(batch) == "" {
			continue
		}
		if _, err := conn.ExecContext(ctx, batch); err != nil {
			return err
		}
	}
	return nil
}

func ekse(t *testing.T, srv *sql.DB, nama, perintah string) error {
	t.Helper()
	return jalankanBerkas(t, srv, nama, perintah, nil)
}

// baca menjalankan query di database nama dan mengembalikan seluruh baris (dari semua hasil) sebagai teks, terurut.
func baca(t *testing.T, srv *sql.DB, nama, query string) [][]string {
	t.Helper()
	ctx := context.Background()
	conn, err := srv.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, "USE "+kutipDB(nama)); err != nil {
		t.Fatal(err)
	}
	rows, err := conn.QueryContext(ctx, query)
	if err != nil {
		t.Fatalf("query gagal di %s: %v", nama, err)
	}
	defer rows.Close()
	var out [][]string
	for set := 0; ; set++ {
		cols, err := rows.Columns()
		if err != nil {
			t.Fatal(err)
		}
		for rows.Next() {
			raw := make([]interface{}, len(cols))
			ptr := make([]interface{}, len(cols))
			for i := range raw {
				ptr[i] = &raw[i]
			}
			if err := rows.Scan(ptr...); err != nil {
				t.Fatal(err)
			}
			baris := make([]string, len(cols))
			for i, v := range raw {
				switch x := v.(type) {
				case nil:
					baris[i] = "NULL"
				case []byte:
					baris[i] = string(x)
				case time.Time:
					baris[i] = x.UTC().Format(time.RFC3339Nano)
				default:
					baris[i] = fmt.Sprint(x)
				}
			}
			out = append(out, baris)
		}
		if err := rows.Err(); err != nil {
			t.Fatalf("hasil query terputus di %s: %v", nama, err)
		}
		if !rows.NextResultSet() {
			break
		}
	}
	sort.Slice(out, func(i, j int) bool { return strings.Join(out[i], "\x00") < strings.Join(out[j], "\x00") })
	return out
}

func angka(t *testing.T, srv *sql.DB, nama, query string) int64 {
	t.Helper()
	r := baca(t, srv, nama, query)
	if len(r) != 1 || len(r[0]) != 1 {
		t.Fatalf("query angka mengembalikan %v", r)
	}
	var n int64
	if _, err := fmt.Sscan(r[0][0], &n); err != nil {
		t.Fatalf("%q bukan angka: %v", r[0][0], err)
	}
	return n
}

func bacaBerkas(t *testing.T, nama string) string {
	t.Helper()
	b, err := os.ReadFile(nama)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// siapkanDatamart membuat SLDK palsu dan datamart, memasang script (yang sekaligus menjalankan penyegaran pertama), lalu mengembalikan nama kedua database.
func siapkanDatamart(t *testing.T, srv *sql.DB) (sldk, mart string) {
	t.Helper()
	sldk, mart = buatDBUji(t, srv, "sldk"), buatDBUji(t, srv, "mart")
	if err := jalankanBerkas(t, srv, sldk, bacaBerkas(t, "testdata/sldk_palsu.sql"), nil); err != nil {
		t.Fatalf("fixture SLDK palsu gagal dimuat: %v", err)
	}
	if err := jalankanBerkas(t, srv, mart, bacaBerkas(t, berkasSkrip), map[string]string{"SUMBER": kutipDB(sldk)}); err != nil {
		t.Fatalf("script datamart gagal dipasang: %v", err)
	}
	return sldk, mart
}

func TestDatamartMemberiHasilQueryIdentikDenganSLDK(t *testing.T) {
	srv := bukaServerUji(t)
	sldk, mart := siapkanDatamart(t, srv)

	if len(digitalisasi.Datasets) != 7 {
		t.Fatalf("tes ini mengira 7 dataset, ada %d; tinjau fixture dan tes", len(digitalisasi.Datasets))
	}
	for _, ds := range digitalisasi.Datasets {
		q, err := ds.Query()
		if err != nil {
			t.Fatal(err)
		}
		dariSLDK := baca(t, srv, sldk, q)
		dariMart := baca(t, srv, mart, q)
		if len(dariSLDK) == 0 {
			t.Errorf("%s: fixture tidak menghasilkan baris dari SLDK palsu, sehingga perbandingan tidak bermakna", ds.Key)
		}
		if fmt.Sprint(dariSLDK) != fmt.Sprint(dariMart) {
			t.Errorf("%s: hasil datamart berbeda dari SLDK\n SLDK    (%d baris): %v\n datamart (%d baris): %v", ds.Key, len(dariSLDK), dariSLDK, len(dariMart), dariMart)
		}
	}
}

func TestDatamartIsiDanPengecualian(t *testing.T) {
	srv := bukaServerUji(t)
	sldk, mart := siapkanDatamart(t, srv)

	ids := func(q string) string {
		var s []string
		for _, r := range baca(t, srv, mart, q) {
			s = append(s, r[0])
		}
		return strings.Join(s, ",")
	}
	// aset inti + tautan; yang berstatus N, non-015, jalan, kendaraan non-015, dan bangunan berstatus N tidak ikut
	if got := ids("SELECT DISTINCT id_aset FROM DJKN.SIMAN2_M_ASET ORDER BY 1"); got != "100,101,102,200,201,202,300,301,400,500,501,502" {
		t.Errorf("id_aset di datamart = %s", got)
	}
	// nama penghuni hanya rumah negara (bukan mess 301, bukan tanah 100)
	if got := ids("SELECT DISTINCT id_aset FROM DJKN.SIMAN2_M_ASET_PEMAKAI ORDER BY 1"); got != "300" {
		t.Errorf("pemakai hanya untuk rumah negara 300, got %s", got)
	}
	// kolom yang tidak dipakai query tidak tersalin, dan hanya skema DJKN yang berisi tabel hasil
	if n := angka(t, srv, mart, "SELECT COUNT(*) FROM sys.columns WHERE object_id = OBJECT_ID('DJKN.SIMAN2_M_ASET') AND name IN ('nilai_buku', 'keterangan', 'dibuat_oleh')"); n != 0 {
		t.Errorf("kolom tak terpakai tersalin: %d", n)
	}
	if n := angka(t, srv, mart, "SELECT COUNT(*) FROM sys.columns WHERE object_id = OBJECT_ID('DJKN.SIMAN2_M_ASET')"); n != 25 {
		t.Errorf("kolom SIMAN2_M_ASET = %d, want 25", n)
	}
	// hanya snapshot status hukum terbaru
	if got := ids("SELECT DISTINCT TGL_TARIK FROM DJKN.SIMAN2_R_STATUS_HUKUM_NEW"); got != "20250101" {
		t.Errorf("snapshot status hukum = %s", got)
	}
	// view SLDK menjadi tabel di datamart, tipe kolom mengikuti sumber
	if n := angka(t, srv, mart, "SELECT COUNT(*) FROM sys.tables WHERE name = 'VW_SIMAN_RUANG_GEDUNG_BANGUNAN_015' AND schema_id = SCHEMA_ID('DJKN')"); n != 1 {
		t.Errorf("tabel pengganti view tidak ada")
	}
	tipe := "SELECT TYPE_NAME(system_type_id) + '(' + CAST(precision AS varchar) + ',' + CAST(scale AS varchar) + ')' FROM sys.columns WHERE object_id = OBJECT_ID('DJKN.SIMAN2_M_ASET') AND name = 'rph_aset'"
	if a, b := baca(t, srv, sldk, tipe), baca(t, srv, mart, tipe); fmt.Sprint(a) != fmt.Sprint(b) || fmt.Sprint(a) != "[[decimal(20,2)]]" {
		t.Errorf("tipe rph_aset harus mengikuti SLDK: SLDK %v, datamart %v", a, b)
	}
	// jejak
	if n := angka(t, srv, mart, "SELECT COUNT(*) FROM dbo.datamart_sldk_run WHERE status = N'sukses'"); n != 1 {
		t.Errorf("proses sukses = %d", n)
	}
	if n := angka(t, srv, mart, "SELECT COUNT(*) FROM dbo.datamart_sldk_tabel"); n != 11 {
		t.Errorf("baris jejak tabel = %d, want 11", n)
	}
	if n := angka(t, srv, mart, "SELECT COUNT(*) FROM sys.tables WHERE schema_id IN (SCHEMA_ID('DJKN_STG'), SCHEMA_ID('DJKN_OLD'))"); n != 0 {
		t.Errorf("tabel sisa di DJKN_STG/DJKN_OLD: %d", n)
	}
}

func TestDatamartPenyegaranUlangDanKegagalanTidakMerusakDatamart(t *testing.T) {
	srv := bukaServerUji(t)
	sldk, mart := siapkanDatamart(t, srv)
	hitung := "SELECT (SELECT COUNT_BIG(*) FROM DJKN.SIMAN2_M_ASET) * 1000000 + (SELECT COUNT_BIG(*) FROM DJKN.SIMAN2_M_ASET_PHOTO) * 1000 + (SELECT COUNT_BIG(*) FROM DJKN.SIMAN2_R_SATKER)"
	awal := angka(t, srv, mart, hitung)

	// hak baca di tingkat skema (cara yang dianjurkan di dokumen) harus bertahan melewati penukaran tabel
	if err := ekse(t, srv, mart, "CREATE USER uji_pembaca WITHOUT LOGIN; GRANT SELECT ON SCHEMA::DJKN TO uji_pembaca;"); err != nil {
		t.Fatal(err)
	}

	// penyegaran ulang (jalur penukaran tabel yang sudah ada) lewat prosedur terpasang, tanpa variabel
	if err := ekse(t, srv, mart, "EXEC dbo.datamart_sldk_segarkan;"); err != nil {
		t.Fatalf("penyegaran ulang gagal: %v", err)
	}
	if got := angka(t, srv, mart, hitung); got != awal {
		t.Errorf("isi berubah setelah penyegaran tanpa perubahan di SLDK: %d -> %d", awal, got)
	}
	if n := angka(t, srv, mart, "EXECUTE AS USER = 'uji_pembaca'; SELECT COUNT(*) FROM DJKN.SIMAN2_M_ASET; REVERT;"); n == 0 {
		t.Error("pengguna dengan hak baca tingkat skema tidak dapat membaca setelah penyegaran")
	}

	// perubahan di SLDK ikut terbawa pada penyegaran berikutnya
	if err := ekse(t, srv, sldk, "UPDATE DJKN.SIMAN2_M_ASET SET status_bmn_yn = 'N' WHERE id_aset = 201"); err != nil {
		t.Fatal(err)
	}
	if err := ekse(t, srv, mart, "EXEC dbo.datamart_sldk_segarkan;"); err != nil {
		t.Fatalf("penyegaran gagal: %v", err)
	}
	if n := angka(t, srv, mart, "SELECT COUNT(*) FROM DJKN.SIMAN2_M_ASET WHERE id_aset = 201"); n != 0 {
		t.Errorf("aset 201 (kini berstatus N di SLDK) masih ada di datamart")
	}
	awal = angka(t, srv, mart, hitung)

	periksaUtuh := func(skenario string) {
		t.Helper()
		if got := angka(t, srv, mart, hitung); got != awal {
			t.Errorf("%s: datamart lama berubah (%d -> %d)", skenario, awal, got)
		}
		if n := angka(t, srv, mart, "SELECT COUNT(*) FROM sys.tables WHERE schema_id IN (SCHEMA_ID('DJKN_STG'), SCHEMA_ID('DJKN_OLD'))"); n != 0 {
			t.Errorf("%s: tabel sisa di DJKN_STG/DJKN_OLD: %d", skenario, n)
		}
		if n := angka(t, srv, mart, "SELECT COUNT(*) FROM dbo.datamart_sldk_run WHERE status = N'berjalan'"); n != 0 {
			t.Errorf("%s: proses tertinggal berstatus berjalan: %d", skenario, n)
		}
	}
	pesanTerakhir := func() string {
		r := baca(t, srv, mart, "SELECT TOP 1 status + N'|' + ISNULL(pesan, N'') FROM dbo.datamart_sldk_run ORDER BY id DESC")
		return r[0][0]
	}

	// skema SLDK berubah SETELAH terpasang (kolom hilang): gagal, tercatat, datamart utuh
	if err := ekse(t, srv, sldk, "ALTER TABLE DJKN.SIMAN2_M_ASET DROP COLUMN merk"); err != nil {
		t.Fatal(err)
	}
	err := ekse(t, srv, mart, "EXEC dbo.datamart_sldk_segarkan;")
	if err == nil || !strings.Contains(err.Error(), "merk") {
		t.Fatalf("kolom hilang harus menggagalkan penyegaran dengan menyebut kolomnya, got %v", err)
	}
	if p := pesanTerakhir(); !strings.HasPrefix(p, "gagal|") || !strings.Contains(p, "merk") {
		t.Errorf("riwayat = %q", p)
	}
	periksaUtuh("kolom SLDK hilang")

	// SLDK mengembalikan 0 aset: ditolak, datamart lama tetap
	if err := ekse(t, srv, sldk, "ALTER TABLE DJKN.SIMAN2_M_ASET ADD merk VARCHAR(100) NULL; DELETE FROM DJKN.SIMAN2_M_ASET"); err != nil {
		t.Fatal(err)
	}
	err = ekse(t, srv, mart, "EXEC dbo.datamart_sldk_segarkan;")
	if err == nil || !strings.Contains(err.Error(), "TIDAK diganti") {
		t.Fatalf("SLDK kosong harus ditolak, got %v", err)
	}
	if p := pesanTerakhir(); !strings.HasPrefix(p, "gagal|") || !strings.Contains(p, "TIDAK diganti") {
		t.Errorf("riwayat = %q", p)
	}
	periksaUtuh("SLDK kosong")
}

func TestDatamartMenolakProsesGanda(t *testing.T) {
	srv := bukaServerUji(t)
	_, mart := siapkanDatamart(t, srv)

	// sesi lain memegang kunci aplikasi yang sama
	ctx := context.Background()
	penahan, err := srv.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := penahan.ExecContext(ctx, "USE "+kutipDB(mart)); err != nil {
		t.Fatal(err)
	}
	var hasil int
	if err := penahan.QueryRowContext(ctx, "DECLARE @r INT; EXEC @r = sp_getapplock N'datamart_sldk', N'Exclusive', N'Session', 0; SELECT @r").Scan(&hasil); err != nil || hasil < 0 {
		t.Fatalf("penahan tidak mendapat kunci: %d %v", hasil, err)
	}
	err = ekse(t, srv, mart, "EXEC dbo.datamart_sldk_segarkan;")
	if err == nil || !strings.Contains(err.Error(), "lain sedang berjalan") {
		t.Errorf("proses ganda harus ditolak, got %v", err)
	}
	penahan.ExecContext(ctx, "EXEC sp_releaseapplock N'datamart_sldk', N'Session'")
	penahan.Close()
	if err := ekse(t, srv, mart, "EXEC dbo.datamart_sldk_segarkan;"); err != nil {
		t.Errorf("setelah kunci dilepas penyegaran harus berhasil: %v", err)
	}
}
