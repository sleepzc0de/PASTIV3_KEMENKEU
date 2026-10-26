package datamart

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	"pasti-v3-backend/digitalisasi"
)

// Tes integrasi ujung ke ujung (PASTI_UJI_MSSQL_DSN; lihat helper_integrasi_test.go): aplikasi yang SUNGGUHAN (digitalisasi.Manager dan Sync) menarik data dari tabel DIGITALISASI_*
// datamart ke database PASTI, tanpa pernah mengakses SLDK. Pembandingnya adalah tabel acuan yang dibentuk dari SLDK dengan ketujuh query asli (paket acuan).

// sinkronPASTIDari seperti sinkronPASTI tetapi dengan handle datamart tertentu (mis. yang tidak terjangkau).
func (l *lingkParitas) sinkronPASTIDari(t *testing.T, mart *sql.DB) map[string]string {
	t.Helper()
	m := digitalisasi.NewManager(l.pasti, mart)
	if _, err := m.Start(digitalisasi.Keys(), "uji"); err != nil {
		t.Fatalf("Start: %v", err)
	}
	selesai := time.Now().Add(120 * time.Second)
	for m.Active() != nil {
		if time.Now().After(selesai) {
			t.Fatal("sinkronisasi tidak selesai dalam 120 detik")
		}
		time.Sleep(50 * time.Millisecond)
	}
	out := map[string]string{}
	for _, b := range baca(t, l.srv, l.pastiNama, "SELECT TOP (7) dataset, status + N'|' + ISNULL(pesan, N'') FROM digitalisasi_sync_log ORDER BY id DESC") {
		out[b[0]] = b[1]
	}
	return out
}

func semuaStatus(t *testing.T, st map[string]string, awal, memuat string) {
	t.Helper()
	if len(st) != len(digitalisasi.Datasets) {
		t.Fatalf("status %d dataset, seharusnya %d: %v", len(st), len(digitalisasi.Datasets), st)
	}
	for ds, s := range st {
		if !strings.HasPrefix(s, awal) || !strings.Contains(s, memuat) {
			t.Errorf("%s: %q, seharusnya diawali %q dan memuat %q", ds, s, awal, memuat)
		}
	}
}

func (l *lingkParitas) hapusDB(t *testing.T, nama string) {
	t.Helper()
	ctx := context.Background()
	if _, err := l.srv.ExecContext(ctx, "ALTER DATABASE "+kutipDB(nama)+" SET SINGLE_USER WITH ROLLBACK IMMEDIATE"); err != nil {
		t.Fatal(err)
	}
	if _, err := l.srv.ExecContext(ctx, "DROP DATABASE "+kutipDB(nama)); err != nil {
		t.Fatal(err)
	}
}

func (l *lingkParitas) fotoPASTI(t *testing.T) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, ds := range digitalisasi.Datasets {
		r := baca(t, l.srv, l.pastiNama, "SELECT COUNT_BIG(*), ISNULL(CHECKSUM_AGG(CHECKSUM(*)), 0) FROM dbo."+ds.Table)
		out[ds.Table] = strings.Join(r[0], "/")
	}
	return out
}

func TestAplikasiMenarikDariDatamartTanpaSLDK(t *testing.T) {
	l := siapkanParitas(t)
	l.sinkronAcuan(t, 1) // tabel yang seharusnya, dari SLDK
	l.pasangDatamart(t)

	// SLDK dihapus sama sekali: aplikasi tidak boleh membutuhkannya
	l.sldk.Close()
	l.hapusDB(t, l.sldkNama)

	st := l.sinkronPASTI(t)
	semuaStatus(t, st, "sukses|", "dari datamart, data per ")
	l.bandingkanPASTIDenganAcuan(t)

	// riwayat: sumber datamart dan data per = awal penyegaran datamart yang menulis tabelnya
	mart := kutipDB(l.martNama)
	if n := angka(t, l.srv, l.pastiNama, "SELECT COUNT(*) FROM digitalisasi_sync_log WHERE status = N'sukses' AND sumber = N'datamart' AND data_per = (SELECT mulai FROM "+mart+".dbo.datamart_digitalisasi_run WHERE id = 1)"); n != int64(len(digitalisasi.Datasets)) {
		t.Errorf("%d dari %d riwayat memuat sumber datamart dan data_per yang benar", n, len(digitalisasi.Datasets))
	}
	// id_sinkron di PASTI = nomor riwayat sinkronisasi PASTI (bukan nomor proses datamart)
	if n := angka(t, l.srv, l.pastiNama, "SELECT COUNT(*) FROM DIGITALISASI_TANAH t WHERE NOT EXISTS (SELECT 1 FROM digitalisasi_sync_log g WHERE g.id = t.id_sinkron AND g.dataset = N'tanah')"); n != 0 {
		t.Errorf("%d baris tanah dengan id_sinkron yang bukan riwayat PASTI", n)
	}

	// sinkronisasi lagi tanpa penyegaran datamart: sama, dan ditandai sama
	st = l.sinkronPASTI(t)
	semuaStatus(t, st, "sukses|", "sama dengan sinkronisasi sebelumnya")
	l.bandingkanPASTIDenganAcuan(t)
}

func TestAplikasiMengikutiPenyegaranDatamart(t *testing.T) {
	l := siapkanParitas(t)
	l.sinkronAcuan(t, 1)
	l.pasangDatamart(t)
	semuaStatus(t, l.sinkronPASTI(t), "sukses|", "dari datamart")
	dataPerLama := baca(t, l.srv, l.pastiNama, "SELECT MAX(data_per) FROM digitalisasi_sync_log")[0][0]

	// SLDK berubah, datamart disegarkan, acuan dibentuk ulang: aplikasi harus mengikuti dan tetap sama dengan acuan
	time.Sleep(1200 * time.Millisecond) // jejak penyegaran berpresisi milidetik; beri jarak nyata
	for _, q := range []string{
		"UPDATE DJKN.SIMAN2_M_ASET SET ur_sskel = N'Diubah ' + CAST(id_aset AS NVARCHAR(20)) WHERE id_aset IN (110, 121)",
		"UPDATE DJKN.SIMAN2_R_SATKER SET ur_satker = ur_satker + N' (baru)'",
	} {
		if err := ekse(t, l.srv, l.sldkNama, q); err != nil {
			t.Fatal(err)
		}
	}
	l.segarkanDatamart(t)
	l.sinkronAcuan(t, 2)
	st := l.sinkronPASTI(t)
	semuaStatus(t, st, "sukses|", "dari datamart, data per ")
	for ds, s := range st {
		if strings.Contains(s, "sama dengan sinkronisasi sebelumnya") {
			t.Errorf("%s: datamart sudah disegarkan, tidak boleh ditandai sama: %s", ds, s)
		}
	}
	l.bandingkanPASTIDenganAcuan(t)
	if got := baca(t, l.srv, l.pastiNama, "SELECT Uraian_tanah FROM DIGITALISASI_TANAH WHERE kode_register_tanah = N'REG-T-121'"); got[0][0] != "Diubah 121" {
		t.Errorf("perubahan SLDK tidak sampai ke PASTI lewat datamart: %v", got)
	}
	if dataPerBaru := baca(t, l.srv, l.pastiNama, "SELECT MAX(data_per) FROM digitalisasi_sync_log")[0][0]; dataPerBaru <= dataPerLama {
		t.Errorf("data_per tidak maju: %s -> %s", dataPerLama, dataPerBaru)
	}
}

func TestAplikasiMempertahankanKolomManualPASTI(t *testing.T) {
	l := siapkanParitas(t)
	l.sinkronAcuan(t, 1)
	l.pasangDatamart(t)
	semuaStatus(t, l.sinkronPASTI(t), "sukses|", "dari datamart")

	// pengguna mengisi kolom manual di PASTI (dan di acuan, yang meniru aplikasi lama); datamart tidak punya nilai itu
	const ubah = "UPDATE DIGITALISASI_SATKER SET Status_Gedung_Kantor = N'Milik sendiri', Foto = N'foto-manual.jpg' WHERE Kode_Satker IN (SELECT TOP (2) Kode_Satker FROM DIGITALISASI_SATKER ORDER BY Kode_Satker)"
	for _, db := range []string{l.pastiNama, l.acuanNama} {
		if err := ekse(t, l.srv, db, ubah); err != nil {
			t.Fatal(err)
		}
	}
	l.sinkronAcuan(t, 2)
	semuaStatus(t, l.sinkronPASTI(t), "sukses|", "dari datamart")
	if n := angka(t, l.srv, l.pastiNama, "SELECT COUNT(*) FROM DIGITALISASI_SATKER WHERE Status_Gedung_Kantor = N'Milik sendiri' AND Foto = N'foto-manual.jpg'"); n != 2 {
		t.Errorf("kolom manual PASTI hilang saat sinkronisasi dari datamart: %d baris, seharusnya 2", n)
	}
	l.bandingkanPASTIDenganAcuan(t)
}

func TestAplikasiMenolakDatamartYangLebihLama(t *testing.T) {
	l := siapkanParitas(t)
	l.sinkronAcuan(t, 1)
	l.pasangDatamart(t)
	semuaStatus(t, l.sinkronPASTI(t), "sukses|", "dari datamart")
	sebelum := l.fotoPASTI(t)

	// datamart dipulihkan dari cadangan lama: penyegaran yang menulis tabelnya kini tercatat 10 hari lebih awal daripada data PASTI
	if err := ekse(t, l.srv, l.martNama, "UPDATE dbo.datamart_digitalisasi_run SET mulai = DATEADD(DAY, -10, mulai)"); err != nil {
		t.Fatal(err)
	}
	semuaStatus(t, l.sinkronPASTI(t), "gagal|", "lebih lama daripada data PASTI")
	for tabel, v := range sebelum {
		if got := l.fotoPASTI(t)[tabel]; got != v {
			t.Errorf("%s berubah walau datamart ditolak: %s -> %s", tabel, v, got)
		}
	}
	l.bandingkanPASTIDenganAcuan(t)
}

func TestAplikasiTidakDipengaruhiPenyegaranDatamartYangGagal(t *testing.T) {
	l := siapkanParitas(t)
	l.sinkronAcuan(t, 1)
	l.pasangDatamart(t)
	semuaStatus(t, l.sinkronPASTI(t), "sukses|", "dari datamart")
	l.hapusDB(t, l.sldkNama) // SLDK hilang: penyegaran datamart berikutnya gagal, tabelnya tetap yang lama
	l.sldk.Close()

	if err := ekse(t, l.srv, l.martNama, "EXEC dbo.datamart_digitalisasi_segarkan;"); err == nil {
		t.Fatal("penyegaran tanpa SLDK seharusnya gagal")
	}
	semuaStatus(t, l.sinkronPASTI(t), "sukses|", "sama dengan sinkronisasi sebelumnya")
	l.bandingkanPASTIDenganAcuan(t)

	// halaman Sinkronisasi diberi tahu: penyegaran terakhir gagal, dan data per tetap penyegaran sukses sebelumnya
	info := digitalisasi.NewManager(l.pasti, l.mart).InfoSumber(context.Background())
	if !info.Tersedia || !info.Terjangkau || info.PenyegaranTerakhir != "gagal" || info.DataPer == nil {
		t.Errorf("info = %+v", info)
	}
}

func TestAplikasiTidakMenimpaDenganDatamartKosong(t *testing.T) {
	l := siapkanParitas(t)
	l.sinkronAcuan(t, 1)
	l.pasangDatamart(t)
	semuaStatus(t, l.sinkronPASTI(t), "sukses|", "dari datamart")
	sebelum := l.fotoPASTI(t)

	if err := ekse(t, l.srv, l.martNama, "TRUNCATE TABLE dbo.DIGITALISASI_TANAH"); err != nil {
		t.Fatal(err)
	}
	st := l.sinkronPASTIDari(t, l.mart)
	if !strings.HasPrefix(st["tanah"], "gagal|") || !strings.Contains(st["tanah"], "0 baris sedangkan tabel sudah berisi data") {
		t.Errorf("tanah: %s", st["tanah"])
	}
	if got := l.fotoPASTI(t)["DIGITALISASI_TANAH"]; got != sebelum["DIGITALISASI_TANAH"] {
		t.Errorf("tanah PASTI berubah walau datamart kosong: %s -> %s", sebelum["DIGITALISASI_TANAH"], got)
	}
	// dataset lain tetap berjalan
	if !strings.HasPrefix(st["satker"], "sukses|") {
		t.Errorf("satker: %s", st["satker"])
	}
}

func TestAplikasiDatamartTidakTerjangkau(t *testing.T) {
	l := siapkanParitas(t)
	l.sinkronAcuan(t, 1)
	l.pasangDatamart(t)
	semuaStatus(t, l.sinkronPASTI(t), "sukses|", "dari datamart")
	sebelum := l.fotoPASTI(t)

	mati := bukaDB(t, l.martNama, "127.0.0.1:1") // tidak ada yang mendengarkan: koneksi ditolak seketika
	st := l.sinkronPASTIDari(t, mati)
	semuaStatus(t, st, "gagal|", "")
	for ds, s := range st {
		if !strings.Contains(s, "datamart") {
			t.Errorf("%s: pesan harus menyebut datamart: %s", ds, s)
		}
	}
	for tabel, v := range sebelum {
		if got := l.fotoPASTI(t)[tabel]; got != v {
			t.Errorf("%s berubah walau datamart tidak terjangkau", tabel)
		}
	}
	if info := digitalisasi.NewManager(l.pasti, mati).InfoSumber(context.Background()); !info.Tersedia || info.Terjangkau || info.DataPer != nil {
		t.Errorf("info = %+v", info)
	}
}

func TestAplikasiMenolakTabelDatamartYangBukanBuatanScript(t *testing.T) {
	l := siapkanParitas(t)
	l.sinkronAcuan(t, 1)
	l.pasangDatamart(t)
	semuaStatus(t, l.sinkronPASTI(t), "sukses|", "dari datamart")
	sebelum := l.fotoPASTI(t)

	// Luas_Tanah diubah menjadi FLOAT: aplikasi tidak boleh menyalin nilai yang kehilangan presisi diam-diam
	if err := ekse(t, l.srv, l.martNama, "ALTER TABLE dbo.DIGITALISASI_TANAH ALTER COLUMN Luas_Tanah FLOAT NULL"); err != nil {
		t.Fatal(err)
	}
	st := l.sinkronPASTIDari(t, l.mart)
	if !strings.HasPrefix(st["tanah"], "gagal|") || !strings.Contains(st["tanah"], "Luas_Tanah") || !strings.Contains(st["tanah"], "float64") {
		t.Errorf("tanah: %s", st["tanah"])
	}
	if got := l.fotoPASTI(t)["DIGITALISASI_TANAH"]; got != sebelum["DIGITALISASI_TANAH"] {
		t.Errorf("tanah PASTI berubah: %s -> %s", sebelum["DIGITALISASI_TANAH"], got)
	}
}
