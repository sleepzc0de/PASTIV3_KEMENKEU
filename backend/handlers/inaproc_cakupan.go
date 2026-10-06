package handlers

import (
	"fmt"
	"strings"

	"pasti-v3-backend/peran"
)

// Pembatasan data Pengadaan (tabel inaproc_*) menurut cakupan peran: UE1, Kanwil, atau Satker hanya melihat pengadaan satker yang menjadi cakupannya.
//
// Kuncinya kd_satker_str Inaproc, kode satker 6 digit yang sama dengan karakter ke-10 sampai ke-15 kode satker data aset (lihat satker_keterhubungan.go).
// Peran Satker membandingkannya langsung dengan kodenya; UE1 dan Kanwil memakai daftar satker data aset yang kode lengkapnya berawalan kode UE1/Kanwil
// (peran.Cakupan.KondisiSatker6SQL). Konsekuensinya satker yang punya pengadaan tetapi tidak ada di data aset tidak terlihat oleh UE1/Kanwil-nya.
//
// Tiap tabel masuk salah satu dari tiga golongan; tabel yang tidak terdaftar (mis. dataset baru) otomatis tidak dapat dibatasi, jadi tertutup bagi peran terbatas:
//   - langsung: punya kolom kd_satker_str;
//   - tautan: tidak punya kolom satker, tetapi memuat kode paket (RUP, tender, atau pencatatan) yang juga ada pada tabel yang punya kd_satker_str, jadi baris
//     dibatasi lewat paket yang satkernya dalam cakupan. Baris yang paketnya tidak ditemukan di tabel pembanding tidak ikut (lebih baik kurang daripada bocor);
//   - tidak dapat dibatasi: tidak punya kode satker yang dapat dipercaya dan tidak ada tautan paket.

// Tabel yang punya kolom kd_satker_str.
var tabelSatkerLangsung = map[string]bool{
	"inaproc_paket_penyedia":                  true,
	"inaproc_paket_penyedia_terumumkan":       true,
	"inaproc_paket_anggaran_penyedia":         true,
	"inaproc_paket_swakelola_terumumkan":      true,
	"inaproc_paket_anggaran_swakelola":        true,
	"inaproc_history_kaji_ulang":              true,
	"inaproc_jadwal_tahapan_tender":           true,
	"inaproc_jadwal_tahapan_non_tender":       true,
	"inaproc_tender_pengumuman":               true,
	"inaproc_peserta_tender":                  true,
	"inaproc_tender_ekontrak":                 true,
	"inaproc_tender_ekontrak_kontrak":         true,
	"inaproc_tender_selesai":                  true,
	"inaproc_non_tender_pengumuman":           true,
	"inaproc_non_tender_ekontrak_kontrak":     true,
	"inaproc_non_tender_selesai":              true,
	"inaproc_pencatatan_non_tender":           true,
	"inaproc_pencatatan_non_tender_realisasi": true,
	"inaproc_pencatatan_swakelola":            true,
	"inaproc_ekatalog_instansi_satker":        true,
}

// tautanSatker: cara membatasi tabel yang tidak punya kolom satker.
type tautanSatker struct {
	kolom  string         // kolom kode paket pada tabel ini
	banyak bool           // kolom dapat memuat beberapa kode dipisah ";" (satu paket e-purchasing bisa memuat beberapa kode RUP)
	sumber []tautanSumber // tabel pembanding (harus golongan langsung); baris lolos bila kodenya ada pada salah satunya
}

type tautanSumber struct {
	tabel string
	kolom string // kolom kode paket pada tabel pembanding
}

var tabelSatkerTautan = map[string]tautanSatker{
	// Paket swakelola: lewat kode RUP swakelola terumumkan atau anggarannya.
	"inaproc_paket_swakelola": {kolom: "kd_rup", sumber: []tautanSumber{
		{"inaproc_paket_swakelola_terumumkan", "kd_rup"}, {"inaproc_paket_anggaran_swakelola", "kd_rup"},
	}},
	// Realisasi pencatatan swakelola tidak memuat satker; satu pencatatan dapat punya beberapa realisasi.
	"inaproc_pencatatan_swakelola_realisasi": {kolom: "kd_swakelola_pct", sumber: []tautanSumber{
		{"inaproc_pencatatan_swakelola", "kd_swakelola_pct"},
	}},
	// Nilai tender selesai: satkernya berbentuk kode bertitik yang berbeda dari kd_satker_str, jadi lewat kode tender.
	"inaproc_tender_selesai_nilai": {kolom: "kd_tender", sumber: []tautanSumber{
		{"inaproc_tender_pengumuman", "kd_tender"}, {"inaproc_tender_selesai", "kd_tender"},
	}},
	// Paket e-purchasing: lewat kode RUP paket penyedia (paket tanpa kode RUP, mis. paket swasta, tidak ikut).
	"inaproc_ekatalog_paket_epurchasing": {kolom: "kd_rup", banyak: true, sumber: []tautanSumber{
		{"inaproc_paket_penyedia", "kd_rup"},
	}},
	"inaproc_ekatalog6_paket_epurchasing": {kolom: "rup_code", banyak: true, sumber: []tautanSumber{
		{"inaproc_paket_penyedia", "kd_rup"},
	}},
}

// DapatDibatasi: apakah data tabel dapat dibatasi per satker. Hanya yang terdaftar.
func DapatDibatasi(tabel string) bool {
	if tabelSatkerLangsung[tabel] {
		return true
	}
	_, ada := tabelSatkerTautan[tabel]
	return ada
}

// kunciSatkerInaprocKolom: kode satker 6 digit dari kolom kode satker Inaproc. Kode yang seluruhnya angka dan kurang dari 6 digit dilengkapi nol di depan
// (nol di depan sering hilang bila kode diperlakukan sebagai bilangan); selain itu dipakai apa adanya (tanpa spasi tepi).
func kunciSatkerInaprocKolom(kolom string) string {
	return fmt.Sprintf(`CASE WHEN LTRIM(RTRIM(%[1]s)) NOT LIKE '%%[^0-9]%%' AND LEN(LTRIM(RTRIM(%[1]s))) BETWEEN 1 AND 6
		THEN RIGHT('000000' + LTRIM(RTRIM(%[1]s)), 6) ELSE LTRIM(RTRIM(%[1]s)) END`, kolom)
}

// kondisiSatkerInaproc: potongan WHERE yang membatasi baris tabel ke cakupan; kolom pada tabel itu ditulis tanpa awalan atau dengan nama tabel sebagai
// awalan. ok=false bila tabelnya tidak dapat dibatasi (pemanggil menolak atau mengosongkan, bukan membuka semuanya). Semua data menghasilkan "1 = 1".
func kondisiSatkerInaproc(tabel string, cak peran.Cakupan) (string, bool) {
	if cak.SemuaData() {
		return "1 = 1", true
	}
	if tabelSatkerLangsung[tabel] {
		return cak.KondisiSatker6SQL(kunciSatkerInaprocKolom("kd_satker_str"), "DIGITALISASI_SATKER", "Kode_Satker"), true
	}
	t, ok := tabelSatkerTautan[tabel]
	if !ok {
		return "1 = 0", false
	}
	var atau []string
	for _, s := range t.sumber {
		kond, _ := kondisiSatkerInaproc(s.tabel, cak)
		pembanding := fmt.Sprintf("SELECT %[2]s FROM %[1]s WHERE %[2]s IS NOT NULL AND (%[3]s)", kutip(s.tabel), kutip(s.kolom), kond)
		kolom := kutip(tabel) + "." + kutip(t.kolom)
		if t.banyak {
			atau = append(atau, fmt.Sprintf("EXISTS (SELECT 1 FROM STRING_SPLIT(%s, ';') k WHERE LTRIM(RTRIM(k.value)) <> N'' AND LTRIM(RTRIM(k.value)) IN (%s))", kolom, pembanding))
		} else {
			atau = append(atau, fmt.Sprintf("%s IN (%s)", kolom, pembanding))
		}
	}
	return "(" + strings.Join(atau, " OR ") + ")", true
}

// sumberInaproc: tabel yang dibatasi ke cakupan, untuk dipakai di FROM. Semua data mengembalikan nama tabel apa adanya (query tidak berubah); selain itu
// tabel turunan bernama sama sehingga kolom dan awalan nama tabel pada query pemanggil tetap berlaku. Tabel yang tidak dapat dibatasi menjadi kosong.
func sumberInaproc(cak peran.Cakupan, tabel string) string {
	if cak.SemuaData() {
		return tabel
	}
	kond, _ := kondisiSatkerInaproc(tabel, cak)
	return fmt.Sprintf("(SELECT * FROM %s WHERE %s) AS %s", kutip(tabel), kond, kutip(tabel))
}

// sumberInaprocAlias: seperti sumberInaproc untuk query yang memberi alias sendiri pada tabelnya (FROM tabel alias).
func sumberInaprocAlias(cak peran.Cakupan, tabel, alias string) string {
	if cak.SemuaData() {
		return tabel + " " + alias
	}
	kond, _ := kondisiSatkerInaproc(tabel, cak)
	return fmt.Sprintf("(SELECT * FROM %s WHERE %s) AS %s", kutip(tabel), kond, alias)
}
