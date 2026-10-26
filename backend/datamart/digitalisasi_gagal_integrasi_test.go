package datamart

import (
	"context"
	"strings"
	"testing"

	"pasti-v3-backend/digitalisasi"
)

// Tes integrasi jalur gagal dan keamanan script datamart Digitalisasi Aset (PASTI_UJI_MSSQL_DSN; lihat datamart_integrasi_test.go). Inti yang dibuktikan: apa pun yang salah,
// isi tabel DIGITALISASI_* datamart yang sudah ada TIDAK berubah, kegagalan tercatat jelas, dan script tidak menyentuh database aplikasi PASTI maupun SLDK.

// fotoDatamart: jumlah baris dan checksum isi tiap tabel DIGITALISASI_* hidup di database mart (termasuk id_sinkron dan synced_at), untuk membuktikan "tidak berubah".
func (l *lingkParitas) fotoDatamart(t *testing.T) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, ds := range digitalisasi.Datasets {
		r := baca(t, l.srv, l.martNama, "SELECT COUNT_BIG(*), ISNULL(CHECKSUM_AGG(CHECKSUM(*)), 0) FROM dbo."+ds.Table)
		out[ds.Table] = strings.Join(r[0], "/")
	}
	return out
}

func samaFoto(t *testing.T, sebelum, sesudah map[string]string) {
	t.Helper()
	for tabel, v := range sebelum {
		if sesudah[tabel] != v {
			t.Errorf("%s berubah: %s -> %s", tabel, v, sesudah[tabel])
		}
	}
}

// statusRunTerakhir: status dan pesan proses penyegaran terbaru.
func (l *lingkParitas) statusRunTerakhir(t *testing.T) (status, pesan string) {
	t.Helper()
	r := baca(t, l.srv, l.martNama, "SELECT TOP (1) status, ISNULL(pesan, N'') FROM dbo.datamart_digitalisasi_run ORDER BY id DESC")
	if len(r) != 1 {
		t.Fatal("tidak ada jejak proses")
	}
	return r[0][0], r[0][1]
}

func (l *lingkParitas) stagingKosong(t *testing.T) {
	t.Helper()
	if n := angka(t, l.srv, l.martNama, "SELECT COUNT(*) FROM sys.tables WHERE schema_id = SCHEMA_ID(N'DIGITALISASI_STG')"); n != 0 {
		t.Errorf("tabel staging tersisa: %d", n)
	}
}

// jalankanSemuaBatch menjalankan script seperti SSMS tanpa opsi berhenti saat galat: batch berikutnya tetap dikirim walau batch sebelumnya galat. Mengembalikan semua galat.
func (l *lingkParitas) jalankanSemuaBatch(t *testing.T, db, teks string) []error {
	t.Helper()
	ctx := context.Background()
	conn, err := l.srv.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, "USE "+kutipDB(db)); err != nil {
		t.Fatal(err)
	}
	var galat []error
	for _, batch := range reGO.Split(teks, -1) {
		if strings.TrimSpace(batch) == "" {
			continue
		}
		if _, err := conn.ExecContext(ctx, batch); err != nil {
			galat = append(galat, err)
		}
	}
	return galat
}

func TestDatamartDigitalisasiSLDKKosongTidakMenimpa(t *testing.T) {
	l := siapkanParitas(t)
	l.pasangDatamart(t)
	sebelum := l.fotoDatamart(t)
	jumlahRun := angka(t, l.srv, l.martNama, "SELECT COUNT(*) FROM dbo.datamart_digitalisasi_run")

	// hasil kosong (akses atau filter SLDK bermasalah) hampir pasti kegagalan di hulu, bukan semua aset hilang
	if err := ekse(t, l.srv, l.sldkNama, "UPDATE DJKN.SIMAN2_M_ASET SET status_bmn_yn = N'T'"); err != nil {
		t.Fatal(err)
	}
	err := ekse(t, l.srv, l.martNama, "EXEC dbo.datamart_digitalisasi_segarkan;")
	if err == nil || !strings.Contains(err.Error(), "SLDK mengembalikan 0 baris sedangkan datamart sudah berisi data") {
		t.Fatalf("penyegaran dengan hasil kosong harus ditolak, galat: %v", err)
	}
	samaFoto(t, sebelum, l.fotoDatamart(t))
	if st, pesan := l.statusRunTerakhir(t); st != "gagal" || !strings.Contains(pesan, "0 baris") {
		t.Errorf("jejak proses: %s / %s", st, pesan)
	}
	if n := angka(t, l.srv, l.martNama, "SELECT COUNT(*) FROM dbo.datamart_digitalisasi_run"); n != jumlahRun+1 {
		t.Errorf("jumlah jejak: %d, seharusnya %d", n, jumlahRun+1)
	}
	l.stagingKosong(t)

	// kunci aplikasi dilepas: proses berikutnya (SLDK pulih) berjalan normal
	if err := ekse(t, l.srv, l.sldkNama, "UPDATE DJKN.SIMAN2_M_ASET SET status_bmn_yn = N'Y' WHERE id_aset IS NOT NULL"); err != nil {
		t.Fatal(err)
	}
	l.segarkanDatamart(t)
	if st, _ := l.statusRunTerakhir(t); st != "sukses" {
		t.Errorf("status setelah pulih: %s", st)
	}
}

func TestDatamartDigitalisasiGagalDiTengahTidakMenimpa(t *testing.T) {
	l := siapkanParitas(t)
	l.pasangDatamart(t)
	sebelum := l.fotoDatamart(t)

	// kolom yang hanya dibaca dataset terakhir (gedung_lainnya) hilang: enam dataset pertama sudah termuat ke staging, tetapi tidak satu pun boleh sampai ke tabel hidup
	if err := ekse(t, l.srv, l.sldkNama, "EXEC sp_rename N'DJKN.SIMAN2_M_ASET.nm_unit_pengguna', N'nm_unit_pengguna_lama', N'COLUMN'"); err != nil {
		t.Fatal(err)
	}
	// SLDK diubah supaya data lain ikut berbeda: bila ada yang bocor ke tabel hidup, foto akan berubah
	if err := ekse(t, l.srv, l.sldkNama, "UPDATE DJKN.SIMAN2_R_SATKER SET ur_satker = ur_satker + N' (diubah)'"); err != nil {
		t.Fatal(err)
	}
	err := ekse(t, l.srv, l.martNama, "EXEC dbo.datamart_digitalisasi_segarkan;")
	if err == nil || !strings.Contains(err.Error(), "nm_unit_pengguna") {
		t.Fatalf("kolom yang hilang harus menggagalkan proses dengan galat yang menyebut kolomnya: %v", err)
	}
	samaFoto(t, sebelum, l.fotoDatamart(t))
	if st, pesan := l.statusRunTerakhir(t); st != "gagal" || !strings.Contains(pesan, "nm_unit_pengguna") {
		t.Errorf("jejak proses: %s / %s", st, pesan)
	}
	l.stagingKosong(t)
}

func TestDatamartDigitalisasiSumberSalahDitolakSebelumMembuatApaPun(t *testing.T) {
	l := siapkanParitas(t)
	// @Sumber menunjuk database yang tidak ada: ditolak SEBELUM ada yang dibuat, dengan daftar tabel yang tidak terbaca beserta alasannya (bukan galat samar saat memuat)
	galat := l.jalankanSemuaBatch(t, l.martNama, l.skripDengan(t, "[uji_dm_tidak_ada_sama_sekali]", false))
	if len(galat) == 0 {
		t.Fatal("sumber yang salah harus menghasilkan galat")
	}
	pesan := galat[0].Error()
	for _, perlu := range []string{"tidak dapat dibaca dari database ini", "[uji_dm_tidak_ada_sama_sekali].DJKN.SIMAN2_M_ASET", "[uji_dm_tidak_ada_sama_sekali].DJKN.SIMAN2_R_SATKER", "TIDAK ADA yang diubah"} {
		if !strings.Contains(pesan, perlu) {
			t.Errorf("pesan tidak memuat %q: %s", perlu, pesan)
		}
	}
	for _, objek := range []string{"SCHEMA_ID(N'DJKN')", "SCHEMA_ID(N'DIGITALISASI_STG')", "OBJECT_ID(N'dbo.DIGITALISASI_TANAH')", "OBJECT_ID(N'dbo.datamart_digitalisasi_run')", "OBJECT_ID(N'dbo.dm_pangkas')"} {
		if l.ada(t, l.martNama, objek) {
			t.Errorf("%s dibuat padahal pemasangan ditolak", objek)
		}
	}

	// pemasangan dengan sumber yang benar berhasil dan hasilnya sama dengan acuan
	l.pasangDatamart(t)
	if st, _ := l.statusRunTerakhir(t); st != "sukses" {
		t.Errorf("status setelah memperbaiki sumber: %s", st)
	}
	if sumber := baca(t, l.srv, l.martNama, "SELECT TOP (1) sumber FROM dbo.datamart_digitalisasi_run ORDER BY id DESC"); !strings.Contains(sumber[0][0], l.sldkNama) {
		t.Errorf("sumber yang tercatat: %v", sumber)
	}
	l.bandingkanKeAcuan(t)
}

// bandingkanKeAcuan: bentuk tabel acuan dari SLDK yang sama lalu bandingkan seluruh tabel dengan datamart.
func (l *lingkParitas) bandingkanKeAcuan(t *testing.T) {
	t.Helper()
	l.sinkronAcuan(t, 1)
	l.bandingkanSemua(t)
}

func TestDatamartDigitalisasiProsesBersamaanDitolak(t *testing.T) {
	l := siapkanParitas(t)
	l.pasangDatamart(t)
	sebelum := l.fotoDatamart(t)

	ctx := context.Background()
	penahan, err := l.srv.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer penahan.Close()
	if _, err := penahan.ExecContext(ctx, "USE "+kutipDB(l.martNama)); err != nil {
		t.Fatal(err)
	}
	// proses lain yang sedang berjalan = memegang kunci aplikasi yang sama
	if _, err := penahan.ExecContext(ctx, "DECLARE @r INT; EXEC @r = sp_getapplock @Resource = N'datamart_digitalisasi', @LockMode = N'Exclusive', @LockOwner = N'Session', @LockTimeout = 0; IF @r < 0 THROW 50000, N'kunci tidak didapat', 1;"); err != nil {
		t.Fatal(err)
	}

	err = ekse(t, l.srv, l.martNama, "EXEC dbo.datamart_digitalisasi_segarkan;")
	if err == nil || !strings.Contains(err.Error(), "Proses datamart digitalisasi lain sedang berjalan") {
		t.Fatalf("proses kedua harus ditolak: %v", err)
	}
	samaFoto(t, sebelum, l.fotoDatamart(t))
	// penolakan tidak boleh menghapus jejak atau melepas kunci milik proses lain: kunci masih dipegang
	if _, err := penahan.ExecContext(ctx, "EXEC sp_releaseapplock @Resource = N'datamart_digitalisasi', @LockOwner = N'Session'"); err != nil {
		t.Fatal(err)
	}
	l.segarkanDatamart(t)
}

func TestDatamartDigitalisasiMenolakDatabaseAplikasi(t *testing.T) {
	l := siapkanParitas(t)
	l.sinkronAcuan(t, 1)
	sebelum := map[string]string{}
	for _, ds := range digitalisasi.Datasets {
		r := baca(t, l.srv, l.acuanNama, "SELECT COUNT_BIG(*), ISNULL(CHECKSUM_AGG(CHECKSUM(*)), 0) FROM dbo."+ds.Table)
		sebelum[ds.Table] = strings.Join(r[0], "/")
	}

	// script dijalankan SALAH di database aplikasi PASTI (mis. USE keliru di SSMS): tidak boleh ada yang berubah, bahkan bila SSMS terus mengirim batch berikutnya
	galat := l.jalankanSemuaBatch(t, l.acuanNama, l.skripDigitalisasi(t))
	if len(galat) == 0 || !strings.Contains(galat[0].Error(), "database aplikasi PASTI") {
		t.Fatalf("galat pertama harus menjelaskan bahwa ini database aplikasi: %v", galat)
	}
	for _, ds := range digitalisasi.Datasets {
		r := baca(t, l.srv, l.acuanNama, "SELECT COUNT_BIG(*), ISNULL(CHECKSUM_AGG(CHECKSUM(*)), 0) FROM dbo."+ds.Table)
		if got := strings.Join(r[0], "/"); got != sebelum[ds.Table] {
			t.Errorf("%s di database aplikasi berubah: %s -> %s", ds.Table, sebelum[ds.Table], got)
		}
	}
	for _, objek := range []string{"SCHEMA_ID(N'DJKN')", "SCHEMA_ID(N'DIGITALISASI_STG')", "OBJECT_ID(N'dbo.datamart_digitalisasi_run')", "OBJECT_ID(N'dbo.dm_pangkas')", "OBJECT_ID(N'dbo.datamart_digitalisasi_segarkan')"} {
		if r := baca(t, l.srv, l.acuanNama, "SELECT CASE WHEN "+objek+" IS NULL THEN N'tidak ada' ELSE N'ADA' END"); r[0][0] != "tidak ada" {
			t.Errorf("script meninggalkan %s di database aplikasi", objek)
		}
	}
	// NOEXEC dimatikan lagi di akhir script: sambungan yang sama tidak tertinggal dalam keadaan "tidak mengeksekusi apa pun"
	if r := baca(t, l.srv, l.acuanNama, "SELECT 1"); r[0][0] != "1" {
		t.Error("sambungan tertinggal dalam keadaan NOEXEC")
	}

	// penjaga kedua di dalam prosedur: bila tabel digitalisasi_sync_log muncul SESUDAH pemasangan, penyegaran menolak
	l2 := siapkanParitas(t)
	l2.pasangDatamart(t)
	sebelum2 := l2.fotoDatamart(t)
	if err := ekse(t, l2.srv, l2.martNama, "CREATE TABLE dbo.digitalisasi_sync_log (id INT);"); err != nil {
		t.Fatal(err)
	}
	err := ekse(t, l2.srv, l2.martNama, "EXEC dbo.datamart_digitalisasi_segarkan;")
	if err == nil || !strings.Contains(err.Error(), "database aplikasi PASTI") {
		t.Fatalf("prosedur harus menolak database aplikasi: %v", err)
	}
	samaFoto(t, sebelum2, l2.fotoDatamart(t))
}

func TestDatamartDigitalisasiMenolakKolomTeksBertipeBerbeda(t *testing.T) {
	// No_aset (teks di tujuan) dari kolom FLOAT: bentuk tekstual FLOAT berbeda antara driver Go dan SQL Server, jadi script menolak, bukan diam-diam menulis bentuk lain
	l := siapkanParitas(t, "ALTER TABLE DJKN.SIMAN2_M_ASET ALTER COLUMN no_aset FLOAT NULL;")
	galat := l.jalankanSemuaBatch(t, l.martNama, l.skripDigitalisasi(t))
	if len(galat) == 0 || !strings.Contains(galat[len(galat)-1].Error(), "No_aset") || !strings.Contains(galat[len(galat)-1].Error(), "TIDAK ADA yang diubah") {
		t.Fatalf("kolom teks bertipe FLOAT harus ditolak dengan galat yang menyebut kolomnya: %v", galat)
	}
	if n := angka(t, l.srv, l.martNama, "SELECT COUNT_BIG(*) FROM dbo.DIGITALISASI_TANAH"); n != 0 {
		t.Errorf("tabel hidup berisi %d baris", n)
	}
	if st, pesan := l.statusRunTerakhir(t); st != "gagal" || !strings.Contains(pesan, "No_aset") {
		t.Errorf("jejak proses: %s / %s", st, pesan)
	}
}

func TestDatamartDigitalisasiPenyegaranUlang(t *testing.T) {
	l := siapkanParitas(t)
	l.pasangDatamart(t)
	l.sinkronAcuan(t, 1)

	// hak akses tabel bertahan lintas penyegaran (isi diganti dengan TRUNCATE + INSERT, bukan DROP + CREATE)
	if err := ekse(t, l.srv, l.martNama, "CREATE ROLE uji_baca_dg; GRANT SELECT ON dbo.DIGITALISASI_TANAH TO uji_baca_dg;"); err != nil {
		t.Fatal(err)
	}
	// proses lama yang tertinggal berstatus berjalan (proses mati di tengah jalan) dan sisa staging
	if err := ekse(t, l.srv, l.martNama, "INSERT dbo.datamart_digitalisasi_run (status) VALUES (N'berjalan'); CREATE TABLE DIGITALISASI_STG.sisa (x INT);"); err != nil {
		t.Fatal(err)
	}
	// data SLDK berubah antara dua penyegaran
	if err := ekse(t, l.srv, l.sldkNama, "UPDATE DJKN.SIMAN2_M_ASET SET ur_sskel = N'Diubah ' + CAST(id_aset AS NVARCHAR(20)) WHERE id_aset = 121"); err != nil {
		t.Fatal(err)
	}
	l.segarkanDatamart(t)
	l.stagingKosong(t)
	if n := angka(t, l.srv, l.martNama, "SELECT COUNT(*) FROM dbo.datamart_digitalisasi_run WHERE status = N'berjalan'"); n != 0 {
		t.Errorf("%d proses masih berstatus berjalan", n)
	}
	if n := angka(t, l.srv, l.martNama, "SELECT COUNT(*) FROM dbo.datamart_digitalisasi_run WHERE status = N'gagal' AND pesan LIKE N'Terhenti%'"); n != 1 {
		t.Errorf("proses terhenti tidak ditandai gagal: %d", n)
	}
	if n := angka(t, l.srv, l.martNama, "SELECT COUNT(*) FROM sys.database_permissions p JOIN sys.database_principals u ON u.principal_id = p.grantee_principal_id WHERE u.name = N'uji_baca_dg' AND p.major_id = OBJECT_ID(N'dbo.DIGITALISASI_TANAH') AND p.permission_name = N'SELECT'"); n != 1 {
		t.Errorf("hak SELECT hilang saat penyegaran: %d", n)
	}
	// perubahan SLDK sampai ke datamart dan tetap identik dengan sinkronisasi ulang aplikasi
	if got := baca(t, l.srv, l.martNama, "SELECT Uraian_tanah FROM dbo.DIGITALISASI_TANAH WHERE kode_register_tanah = N'REG-T-121'"); got[0][0] != "Diubah 121" {
		t.Errorf("perubahan SLDK tidak sampai ke datamart: %v", got)
	}
	l.sinkronAcuan(t, 2)
	l.bandingkanSemua(t)

	// jejak jumlah baris sesuai isi tabel
	for _, ds := range digitalisasi.Datasets {
		jejak := angka(t, l.srv, l.martNama, "SELECT baris FROM dbo.datamart_digitalisasi_tabel WHERE tabel = N'"+ds.Table+"' AND run_id = (SELECT MAX(id) FROM dbo.datamart_digitalisasi_run WHERE status = N'sukses')")
		nyata := angka(t, l.srv, l.martNama, "SELECT COUNT_BIG(*) FROM dbo."+ds.Table)
		if jejak != nyata {
			t.Errorf("%s: jejak %d baris, tabel %d baris", ds.Table, jejak, nyata)
		}
	}
	// SLDK tidak berubah akibat script: tidak ada objek baru dan isi sama
	if n := angka(t, l.srv, l.sldkNama, "SELECT COUNT(*) FROM sys.objects WHERE name LIKE N'datamart%' OR name LIKE N'dm[_]%' OR name LIKE N'DIGITALISASI%'"); n != 0 {
		t.Errorf("SLDK memuat %d objek buatan script datamart", n)
	}
}

// ada: apakah objek (ekspresi seperti OBJECT_ID(...) atau SCHEMA_ID(...)) ada di database db.
func (l *lingkParitas) ada(t *testing.T, db, ekspresi string) bool {
	t.Helper()
	return baca(t, l.srv, db, "SELECT CASE WHEN "+ekspresi+" IS NULL THEN 0 ELSE 1 END")[0][0] == "1"
}

func TestDatamartDigitalisasiSumberKosongDiDalamSLDK(t *testing.T) {
	// @Sumber kosong = script dijalankan di dalam database yang sudah berisi DJKN.SIMAN2_*; di sini database SLDK palsu itu sendiri (di data warehouse, itulah keadaan sebenarnya)
	l := siapkanParitas(t)
	l.sinkronAcuan(t, 1)
	if err := jalankanBerkas(t, l.srv, l.sldkNama, l.skripDengan(t, "", false), nil); err != nil {
		t.Fatalf("pemasangan di dalam database yang berisi tabel SLDK gagal: %v", err)
	}
	if st := baca(t, l.srv, l.sldkNama, "SELECT TOP (1) status FROM dbo.datamart_digitalisasi_run ORDER BY id DESC")[0][0]; st != "sukses" {
		t.Errorf("status: %s", st)
	}
	if n := angka(t, l.srv, l.sldkNama, "SELECT COUNT(*) FROM sys.synonyms"); n != 0 {
		t.Errorf("@Sumber kosong tidak boleh membuat sinonim, ada %d", n)
	}
	l.martNama, l.mart = l.sldkNama, l.sldk // bandingkan tabel yang terbentuk di database itu dengan hasil acuan
	l.bandingkanSemua(t)
}

func TestDatamartDigitalisasiSumberKosongTanpaObjekTidakMengubahApaPun(t *testing.T) {
	// database tempat script dijalankan tidak berisi tabel SLDK (DJKN.SIMAN2_*) dan @Sumber kosong: ditolak dengan daftar tabel yang tidak terbaca, tanpa membuat apa pun
	l := siapkanParitas(t)
	galat := l.jalankanSemuaBatch(t, l.martNama, l.skripDengan(t, "", false))
	if len(galat) == 0 || !strings.Contains(galat[0].Error(), "tidak dapat dibaca dari database ini") || !strings.Contains(galat[0].Error(), "DJKN.SIMAN2_M_ASET") {
		t.Fatalf("harus ditolak dengan penjelasan: %v", galat)
	}
	for _, objek := range []string{"SCHEMA_ID(N'DJKN')", "SCHEMA_ID(N'DIGITALISASI_STG')", "OBJECT_ID(N'dbo.DIGITALISASI_TANAH')", "OBJECT_ID(N'dbo.datamart_digitalisasi_run')", "OBJECT_ID(N'dbo.dm_pangkas')"} {
		if l.ada(t, l.martNama, objek) {
			t.Errorf("%s dibuat padahal pemasangan ditolak (pesan menjanjikan TIDAK ADA yang diubah)", objek)
		}
	}
}

func TestDatamartDigitalisasiSkemaDJKNSungguhanDitolak(t *testing.T) {
	// database yang sudah berisi tabel DJKN.SIMAN2_* sungguhan (mis. datamart mentah dari script lain): sinonim tidak boleh menimpanya
	l := siapkanParitas(t)
	if err := ekse(t, l.srv, l.martNama, "EXEC (N'CREATE SCHEMA DJKN'); CREATE TABLE DJKN.SIMAN2_M_ASET (x INT); INSERT DJKN.SIMAN2_M_ASET VALUES (42);"); err != nil {
		t.Fatal(err)
	}
	galat := l.jalankanSemuaBatch(t, l.martNama, l.skripDigitalisasi(t))
	if len(galat) == 0 || !strings.Contains(galat[0].Error(), "sudah berisi tabel/view sungguhan") {
		t.Fatalf("harus ditolak dengan penjelasan: %v", galat)
	}
	if got := baca(t, l.srv, l.martNama, "SELECT x FROM DJKN.SIMAN2_M_ASET"); len(got) != 1 || got[0][0] != "42" {
		t.Errorf("tabel yang sudah ada berubah: %v", got)
	}
	for _, objek := range []string{"OBJECT_ID(N'dbo.DIGITALISASI_TANAH')", "OBJECT_ID(N'dbo.datamart_digitalisasi_run')", "OBJECT_ID(N'dbo.dm_pangkas')", "SCHEMA_ID(N'DIGITALISASI_STG')"} {
		if l.ada(t, l.martNama, objek) {
			t.Errorf("%s dibuat padahal pemasangan ditolak", objek)
		}
	}
}

// Tabel SLDK di data warehouse bernama lain (skema dan nama berbeda): pemetaan per tabel, tanpa menyunting query.
func TestDatamartDigitalisasiPemetaanSumberUntukNamaBerbeda(t *testing.T) {
	l := siapkanParitas(t)
	l.sinkronAcuan(t, 1) // acuan dibentuk lebih dulu, selagi tabel masih bernama asli
	pindah := "EXEC (N'CREATE SCHEMA DWH'); ALTER SCHEMA DWH TRANSFER DJKN.SIMAN2_R_KONDISI; EXEC sp_rename N'DWH.SIMAN2_R_KONDISI', N'REF_KONDISI';"
	if err := ekse(t, l.srv, l.sldkNama, pindah); err != nil {
		t.Fatal(err)
	}

	// tanpa pemetaan: ditolak dengan menyebut tabel yang hilang
	galat := l.jalankanSemuaBatch(t, l.martNama, l.skripDengan(t, kutipDB(l.sldkNama), false))
	if len(galat) == 0 || !strings.Contains(galat[0].Error(), "SIMAN2_R_KONDISI") {
		t.Fatalf("tabel yang berpindah harus disebut: %v", galat)
	}
	if l.ada(t, l.martNama, "OBJECT_ID(N'dbo.DIGITALISASI_TANAH')") {
		t.Error("tabel dibuat padahal pemasangan ditolak")
	}

	// dengan pemetaan: berhasil, dan hasilnya sama dengan acuan
	peta := "UPDATE @peta SET sumber = N'" + kutipDB(l.sldkNama) + ".DWH.REF_KONDISI' WHERE nama = N'SIMAN2_R_KONDISI';"
	if err := jalankanBerkas(t, l.srv, l.martNama, l.skripDengan(t, kutipDB(l.sldkNama), false, peta), nil); err != nil {
		t.Fatalf("pemasangan dengan pemetaan gagal: %v", err)
	}
	if n := angka(t, l.srv, l.martNama, "SELECT COUNT(*) FROM sys.synonyms WHERE name = N'SIMAN2_R_KONDISI' AND base_object_name LIKE N'%DWH%REF_KONDISI%'"); n != 1 {
		t.Errorf("sinonim untuk tabel yang dipetakan: %d", n)
	}
	l.bandingkanSemua(t)
}

// Bawaan script (tanpa diubah sedikit pun) di data warehouse: yang tersisa hanya tabel, dan menjalankannya lagi menyegarkan isinya.
func TestDatamartDigitalisasiBawaanMenyisakanHanyaTabel(t *testing.T) {
	l := siapkanParitas(t)
	l.sinkronAcuan(t, 1)
	utuh := bacaBerkas(t, BerkasDigitalisasi) // tanpa penyuntingan apa pun: @Sumber kosong, @Bersihkan = 1
	if err := jalankanBerkas(t, l.srv, l.sldkNama, utuh, nil); err != nil {
		t.Fatalf("pemasangan bawaan gagal: %v", err)
	}
	l.martNama, l.mart = l.sldkNama, l.sldk

	l.bandingkanSemua(t)
	for nama, query := range map[string]string{
		"rutin sementara":     "SELECT COUNT(*) FROM sys.objects WHERE type IN (N'P', N'FN', N'IF', N'TF') AND (name LIKE N'dm[_]%' OR name LIKE N'datamart[_]%')",
		"sinonim":             "SELECT COUNT(*) FROM sys.synonyms",
		"tabel staging/skema": "SELECT COUNT(*) FROM sys.schemas WHERE name = N'DIGITALISASI_STG'",
	} {
		if n := angka(t, l.srv, l.sldkNama, query); n != 0 {
			t.Errorf("%s tersisa: %d", nama, n)
		}
	}
	if n := angka(t, l.srv, l.sldkNama, "SELECT COUNT(*) FROM sys.tables WHERE name LIKE N'DIGITALISASI[_]%' OR name LIKE N'datamart[_]digitalisasi[_]%'"); n != 9 {
		t.Errorf("tabel hasil: %d, seharusnya 7 DIGITALISASI_* dan 2 tabel jejak", n)
	}
	// tabel SLDK asli tidak tersentuh
	if n := angka(t, l.srv, l.sldkNama, "SELECT COUNT(*) FROM DJKN.SIMAN2_M_ASET"); n == 0 {
		t.Error("tabel SLDK kosong")
	}

	// jalankan lagi: menyegarkan (dua proses sukses), isi tetap sama dengan acuan
	if err := jalankanBerkas(t, l.srv, l.sldkNama, utuh, nil); err != nil {
		t.Fatalf("pemasangan ulang gagal: %v", err)
	}
	if n := angka(t, l.srv, l.sldkNama, "SELECT COUNT(*) FROM dbo.datamart_digitalisasi_run WHERE status = N'sukses'"); n != 2 {
		t.Errorf("proses sukses: %d, seharusnya 2", n)
	}
	l.bandingkanSemua(t)
}

// Penyegaran yang gagal tidak membuang apa pun (rutin dipertahankan untuk diagnosis) dan tabel lama tetap.
func TestDatamartDigitalisasiBawaanMempertahankanRutinBilaGagal(t *testing.T) {
	l := siapkanParitas(t)
	utuh := bacaBerkas(t, BerkasDigitalisasi)
	if err := jalankanBerkas(t, l.srv, l.sldkNama, utuh, nil); err != nil {
		t.Fatalf("pemasangan awal gagal: %v", err)
	}
	l.martNama, l.mart = l.sldkNama, l.sldk
	sebelum := l.fotoDatamart(t)

	if err := ekse(t, l.srv, l.sldkNama, "EXEC sp_rename N'DJKN.SIMAN2_M_ASET.nm_unit_pengguna', N'nm_unit_pengguna_lama', N'COLUMN'"); err != nil {
		t.Fatal(err)
	}
	// SSMS tetap mengirim batch berikutnya setelah galat; pembersihan harus melihat bahwa penyegaran terakhir gagal
	galat := l.jalankanSemuaBatch(t, l.sldkNama, utuh)
	if len(galat) == 0 {
		t.Fatal("penyegaran dengan kolom hilang seharusnya gagal")
	}
	// penyebab sebenarnya tampil pada keluaran script (galat pembuatan prosedur), dan jejak menunjuk ke sana
	semua := ""
	for _, e := range galat {
		semua += e.Error() + " | "
	}
	if !strings.Contains(semua, "nm_unit_pengguna") {
		t.Errorf("galat kolom yang hilang tidak tampil pada keluaran script: %s", semua)
	}
	samaFoto(t, sebelum, l.fotoDatamart(t))
	if n := angka(t, l.srv, l.sldkNama, "SELECT COUNT(*) FROM sys.objects WHERE type = N'P' AND name = N'datamart_digitalisasi_segarkan'"); n != 1 {
		t.Error("prosedur harus dipertahankan bila penyegaran gagal")
	}
	if st, pesan := l.statusRunTerakhir(t); st != "gagal" || !strings.Contains(pesan, "lihat galat SEBELUMNYA") {
		t.Errorf("jejak: %s / %s", st, pesan)
	}
}
