package apiakses

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// Tes katalog dataset pengadaan: tanpa database. Kebijakan kolom dijaga dua lapis: (1) setiap kolom tabel menurut migrasi harus tercatat sebagai keluar, tertutup, atau teknis
// (kolom baru tidak lolos tanpa dinilai), dan (2) kolom yang keluar tidak boleh bernama seperti data orang, identitas pajak/keuangan, atau isi bebas.

type kolomMigrasi struct{ nama, tipe string }

// kolomTabelMigrasi membaca kolom dan tipe tabel dari berkas migrasi (nil bila tabel tidak ditemukan).
func kolomTabelMigrasi(t *testing.T, tabel string) []kolomMigrasi {
	t.Helper()
	berkas, err := filepath.Glob("../migrations/*.sql")
	if err != nil || len(berkas) == 0 {
		t.Fatalf("migrasi tidak ditemukan: %v", err)
	}
	reBlok := regexp.MustCompile(`(?is)CREATE TABLE ` + regexp.QuoteMeta(tabel) + `\s*\((.*?)\n\s*\);`)
	reKolom := regexp.MustCompile(`(?i)^\s*(\w+)\s+(NVARCHAR|VARCHAR|INT|BIGINT|DECIMAL|DATETIME2|DATE|BIT)\b`)
	for _, f := range berkas {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		m := reBlok.FindStringSubmatch(string(b))
		if m == nil {
			continue
		}
		var out []kolomMigrasi
		for _, baris := range strings.Split(m[1], "\n") {
			if k := reKolom.FindStringSubmatch(baris); k != nil {
				switch strings.ToUpper(k[1]) {
				case "PRIMARY", "CONSTRAINT", "INDEX":
					continue
				}
				out = append(out, kolomMigrasi{k[1], strings.ToUpper(k[2])})
			}
		}
		return out
	}
	return nil
}

func tipeDariSQL(s string) string {
	switch s {
	case "DECIMAL":
		return "desimal"
	case "INT", "BIGINT":
		return "bilangan"
	case "DATETIME2", "DATE":
		return "waktu"
	case "BIT":
		return "logika"
	}
	return "teks"
}

func TestKatalogPengadaanTerbentuk(t *testing.T) {
	kat := KatalogPengadaan()
	if len(kat) == 0 {
		t.Fatal("katalog pengadaan kosong")
	}
	lihat := map[string]bool{}
	tabel := map[string]bool{}
	for _, d := range kat {
		if !strings.HasPrefix(d.Kode, "pengadaan.") || lihat[d.Kode] {
			t.Errorf("kode %q harus berawalan pengadaan. dan unik", d.Kode)
		}
		lihat[d.Kode] = true
		if tabel[d.Tabel()] {
			t.Errorf("tabel %s dipakai dua dataset", d.Tabel())
		}
		tabel[d.Tabel()] = true
		if !regexp.MustCompile(`^pengadaan\.[a-z0-9_]+$`).MatchString(d.Kode) {
			t.Errorf("kode %q hanya boleh huruf kecil, angka, dan garis bawah", d.Kode)
		}
		if d.Label == "" || d.Deskripsi == "" || d.Subkelompok == "" || len(d.Kolom) == 0 {
			t.Errorf("%s: label, deskripsi, subkelompok, dan kolom wajib terisi", d.Kode)
		}
		got, ok := CariDatasetPengadaan(d.Kode)
		if !ok || got.Kode != d.Kode || got.Kunci() != strings.TrimPrefix(d.Kode, "pengadaan.") {
			t.Errorf("CariDatasetPengadaan(%q) = %v, %v", d.Kode, got.Kode, ok)
		}
	}
}

func TestCariDatasetPengadaanHanyaKodeDikenal(t *testing.T) {
	if _, ok := CariDatasetPengadaan("pengadaan.tender_pengumuman"); !ok {
		t.Fatal("pengadaan.tender_pengumuman harus dikenal")
	}
	for _, k := range []string{"", "tender_pengumuman", "pengadaan.", "pengadaan.users", "pengadaan.tender_pengumuman; DROP TABLE x", "PENGADAAN.TENDER_PENGUMUMAN",
		"../pengadaan.tender_pengumuman", "aset.tanah", "pengadaan.inaproc_tender_pengumuman", "pengadaan.program_master", "pengadaan.tender_ekontrak"} {
		if _, ok := CariDatasetPengadaan(k); ok {
			t.Errorf("CariDatasetPengadaan(%q) harus gagal", k)
		}
	}
	// katalog aset tidak mengenal kode pengadaan (dan sebaliknya): dua jalur pembaca terpisah
	if _, ok := CariDataset("pengadaan.tender_pengumuman"); ok {
		t.Error("CariDataset (aset) tidak boleh mengenal kode pengadaan")
	}
	if _, ok := CariDatasetPengadaan("aset.tanah"); ok {
		t.Error("CariDatasetPengadaan tidak boleh mengenal kode aset")
	}
}

// Setiap kolom tabel menurut migrasi harus dinilai (keluar, tertutup, teknis, atau kunci baris), tanpa kolom yatim di daftar kita, dan tipe yang dinyatakan harus sama dengan tipe
// di migrasi. Kolom baru pada tabel Inaproc membuat tes ini gagal sampai dinilai: itulah yang menjaga data orang tidak keluar tanpa disengaja.
func TestSemuaKolomPengadaanDinilaiTerhadapMigrasi(t *testing.T) {
	for _, d := range KatalogPengadaan() {
		kolom := kolomTabelMigrasi(t, d.Tabel())
		if len(kolom) == 0 {
			t.Errorf("%s: tabel %s tidak ditemukan di migrasi", d.Kode, d.Tabel())
			continue
		}
		tipeTabel := map[string]string{}
		for _, k := range kolom {
			tipeTabel[k.nama] = k.tipe
		}
		keluar := map[string]string{}
		for _, k := range d.Kolom {
			keluar[k.Nama] = k.Tipe
			if k.kolom != k.Nama {
				t.Errorf("%s: nama JSON %q harus sama dengan nama kolom %q", d.Kode, k.Nama, k.kolom)
			}
		}
		tertutup := map[string]bool{}
		for _, c := range d.Tertutup {
			tertutup[c] = true
		}
		if _, ada := tipeTabel[d.KolomKunci()]; !ada {
			t.Errorf("%s: kolom kunci %s tidak ada di tabel", d.Kode, d.KolomKunci())
		}
		if _, ada := keluar[d.KolomKunci()]; ada {
			t.Errorf("%s: kolom kunci %s menjadi id, jangan dikeluarkan lagi sebagai kolom", d.Kode, d.KolomKunci())
		}
		for _, k := range kolom {
			n := k.nama
			switch {
			case n == d.KolomKunci(), kolomTeknisPengadaan[n]:
				if _, bocor := keluar[n]; bocor {
					t.Errorf("%s: kolom teknis/kunci %s tidak boleh keluar sebagai kolom data", d.Kode, n)
				}
			case tertutup[n]:
				if _, bocor := keluar[n]; bocor {
					t.Errorf("%s: kolom tertutup %s keluar", d.Kode, n)
				}
			default:
				tipe, ada := keluar[n]
				if !ada {
					t.Errorf("%s: kolom %s pada tabel %s belum dinilai; masukkan ke daftar keluar atau tertutup", d.Kode, n, d.Tabel())
					continue
				}
				if tipe != tipeDariSQL(k.tipe) {
					t.Errorf("%s: kolom %s bertipe %s di migrasi (%s) tetapi dinyatakan %s", d.Kode, n, k.tipe, tipeDariSQL(k.tipe), tipe)
				}
			}
		}
		for n := range keluar {
			if _, ada := tipeTabel[n]; !ada {
				t.Errorf("%s: kolom keluar %s tidak ada di tabel %s", d.Kode, n, d.Tabel())
			}
		}
		for n := range tertutup {
			if _, ada := tipeTabel[n]; !ada {
				t.Errorf("%s: kolom tertutup %s tidak ada di tabel %s", d.Kode, n, d.Tabel())
			}
		}
	}
}

// Kebijakan dari sisi nama: kolom yang keluar tidak boleh terlihat seperti data orang, identitas pajak/keuangan, atau isi bebas, apa pun isi daftar di atas. Bila tes ini gagal untuk
// kolom yang memang aman, tambahkan ke pengecualian dengan alasannya; jangan melonggarkan polanya.
func TestKolomPribadiPengadaanTidakPernahKeluar(t *testing.T) {
	larangan := regexp.MustCompile(`(?i)(^|_)(nip|npwp|npwp16|username|email|telp|telepon|ppk|pp|pokja|jabatan|user|wakil|kso|pelaksana|alasan|ket|json|bukti|dok|catatan|rek|rekening|bank)(_|$)|informasi_lainnya|pemilik`)
	// Pengecualian: nama mirip tetapi isinya label status yang terbatas (bukan isian bebas petugas).
	pengecualian := map[string]string{
		"status_nontender_pct_ket": "label status pencatatan non-tender",
		"status_swakelola_pct_ket": "label status pencatatan swakelola",
	}
	for _, d := range KatalogPengadaan() {
		for _, k := range d.Kolom {
			if larangan.MatchString(k.Nama) {
				if _, ok := pengecualian[k.Nama]; !ok {
					t.Errorf("%s: kolom %s tampak seperti data orang/pajak/keuangan/isi bebas dan tidak boleh keluar", d.Kode, k.Nama)
				}
			}
		}
	}
	// Kolom-kolom yang HARUS tertutup di tabel yang memilikinya (bukti dua arah: tertutup, bukan sekadar tidak terdaftar).
	wajibTutup := map[string][]string{
		"pengadaan.tender_pengumuman":              {"nama_ppk", "nip_ppk", "nama_pokja", "nip_pokja", "ket_ditutup", "ket_diulang"},
		"pengadaan.rup_paket_penyedia":             {"nama_ppk", "nip_ppk", "username_ppk", "alasan_non_ukm", "alasan_dikecualikan"},
		"pengadaan.tender_kontrak":                 {"nama_ppk", "nip_ppk", "jabatan_ppk", "no_sk_ppk", "npwp_penyedia", "wakil_sah_penyedia", "no_rek_bank", "nama_rek_bank", "nama_pemilik_rek_bank", "anggota_kso"},
		"pengadaan.nontender_pengumuman":           {"nip_nama_ppk", "nip_nama_pp", "nip_nama_pokja"},
		"pengadaan.tender_peserta":                 {"npwp_penyedia", "npwp_penyedia_16", "alasan"},
		"pengadaan.ekatalog_v5_paket":              {"ppk_nip", "kd_user_ppk", "email_user_pokja", "no_telp_user_pokja", "npwp_satker"},
		"pengadaan.pencatatan_swakelola_realisasi": {"nama_pelaksana", "npwp_pelaksana", "dok_realisasi"},
	}
	for kode, cols := range wajibTutup {
		d, ok := CariDatasetPengadaan(kode)
		if !ok {
			t.Errorf("%s tidak ada di katalog", kode)
			continue
		}
		tutup := map[string]bool{}
		for _, c := range d.Tertutup {
			tutup[c] = true
		}
		for _, c := range cols {
			if !tutup[c] {
				t.Errorf("%s: %s wajib tertutup", kode, c)
			}
		}
	}
}

func TestSemuaDatasetGabunganAsetDanPengadaan(t *testing.T) {
	semua := SemuaDataset()
	if len(semua) != len(Katalog())+len(KatalogPengadaan()) {
		t.Fatalf("SemuaDataset = %d, want %d", len(semua), len(Katalog())+len(KatalogPengadaan()))
	}
	lihat := map[string]bool{}
	lihatPengadaan := false
	var gabung []string
	for _, d := range semua {
		if lihat[d.Kode] {
			t.Errorf("kode ganda %s", d.Kode)
		}
		lihat[d.Kode] = true
		gabung = append(gabung, d.Kode)
		switch d.Kelompok {
		case KelompokAset:
			if lihatPengadaan {
				t.Errorf("dataset aset %s muncul setelah dataset pengadaan; urutan harus aset lalu pengadaan", d.Kode)
			}
			if d.Subkelompok != "" {
				t.Errorf("%s: dataset aset tidak punya subkelompok", d.Kode)
			}
		case KelompokPengadaan:
			lihatPengadaan = true
			if d.Subkelompok == "" {
				t.Errorf("%s: dataset pengadaan harus punya subkelompok", d.Kode)
			}
		default:
			t.Errorf("%s: kelompok %q tidak dikenal", d.Kode, d.Kelompok)
		}
		if !DatasetDikenal(d.Kode) {
			t.Errorf("DatasetDikenal(%q) harus benar", d.Kode)
		}
		// batas lebar kolom di database: api_akses_klien_dataset.dataset dan api_akses_log.dataset (NVARCHAR(60))
		if len(d.Kode) > 60 {
			t.Errorf("kode %s lebih dari 60 karakter (lebar kolom dataset di database)", d.Kode)
		}
	}
	// api_akses_klien.dataset_diminta menyimpan semua kode dipisah koma; migrasi 062 melebarkannya ke 4.000 karakter.
	if n := len(strings.Join(gabung, ",")); n > 4000 {
		t.Errorf("daftar penuh kode dataset (%d karakter) melebihi kolom dataset_diminta (4000)", n)
	}
	if DatasetDikenal("pengadaan.tidak_ada") || DatasetDikenal("") || DatasetDikenal("pengadaan.") {
		t.Error("kode yang tidak ada tidak boleh dikenal")
	}
	if got := DaftarKode(); len(got) != len(semua) {
		t.Errorf("DaftarKode = %d, want %d", len(got), len(semua))
	}
}

func TestNormalisasiDatasetCampuranAsetDanPengadaan(t *testing.T) {
	ds, err := NormalisasiDataset([]string{" pengadaan.tender_pengumuman", "aset.tanah", "pengadaan.tender_pengumuman", "aset.satker", "pengadaan.rup_paket_penyedia"}, 10)
	if err != nil {
		t.Fatal(err)
	}
	// aset lebih dulu (urutan katalog Digitalisasi Aset), lalu pengadaan menurut urutan katalog pengadaan; tanpa duplikat
	urutan := map[string]int{}
	for i, d := range SemuaDataset() {
		urutan[d.Kode] = i
	}
	if len(ds) != 4 || !sort.SliceIsSorted(ds, func(i, j int) bool { return urutan[ds[i]] < urutan[ds[j]] }) {
		t.Errorf("hasil = %v", ds)
	}
	for _, buruk := range []string{"pengadaan.xyz", "pengadaan.", "pengadaan.tender_pengumuman,aset.tanah", "pengadaan.tender_pengumuman'--"} {
		if _, err := NormalisasiDataset([]string{"aset.tanah", buruk}, 10); err == nil {
			t.Errorf("kode %q harus ditolak", buruk)
		}
	}
	// daftar penuh (semua aset dan semua pengadaan) harus muat dalam batas jumlah yang dipakai pendaftaran
	var semua []string
	for _, d := range SemuaDataset() {
		semua = append(semua, d.Kode)
	}
	if _, err := NormalisasiDataset(semua, maksDatasetMinta); err != nil {
		t.Errorf("semua dataset harus dapat diberikan sekaligus: %v", err)
	}
}

func TestKodeDatasetPengadaanDiTabelKlienDataset(t *testing.T) {
	// Kode ditulis ke api_akses_klien_dataset(dataset NVARCHAR(60)) dan dibaca kembali apa adanya oleh gerbang: huruf kecil, angka, titik, garis bawah saja.
	re := regexp.MustCompile(`^[a-z0-9_.]+$`)
	for _, d := range SemuaDataset() {
		if !re.MatchString(d.Kode) {
			t.Errorf("kode %q memuat karakter di luar [a-z0-9_.]", d.Kode)
		}
	}
}
