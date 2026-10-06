package handlers

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"pasti-v3-backend/peran"
)

// Dataset yang SENGAJA tidak dapat dibatasi per satker (tertutup bagi UE1, Kanwil, dan Satker). Setiap dataset di katalog harus masuk salah satu golongan:
// dataset baru yang tidak punya kode satker dan tidak ditambahkan ke inaproc_cakupan.go gagal di tes ini, bukan diam-diam terbuka.
var datasetTidakDapatDibatasi = map[string]string{
	"rup/program-master":                           "hanya kd_satker internal, tanpa kd_satker_str dan tanpa kode paket",
	"tender/non-tender-ekontrak":                   "hanya alamat satker, tanpa kode satker",
	"ekatalog/e-purchasing-by-produk":              "kode satker V6 berformat tidak terverifikasi dan tanpa kode RUP",
	"ekatalog-archive/komoditas-detail":            "rujukan katalog, bukan data satker",
	"ekatalog-archive/penyedia-detail":             "rujukan katalog, bukan data satker",
	"ekatalog-archive/penyedia-distributor-detail": "rujukan katalog, bukan data satker",
	"ekatalog/list-kategori-produk":                "rujukan katalog, bukan data satker",
	"ekatalog/penyedia-detail":                     "rujukan katalog, bukan data satker",
	"ekatalog/list-produk-penyedia":                "rujukan katalog, bukan data satker",
}

func TestSemuaDatasetTerklasifikasiUntukCakupan(t *testing.T) {
	if len(DaftarDataset) == 0 {
		t.Fatal("katalog dataset kosong")
	}
	dikenal := map[string]bool{}
	for _, d := range DaftarDataset {
		dikenal[d.ID] = true
		_, tertutup := datasetTidakDapatDibatasi[d.ID]
		if dapat := DapatDibatasi(d.Tabel); dapat == tertutup {
			t.Errorf("dataset %s (tabel %s): DapatDibatasi=%v tetapi tertutup=%v; golongkan di inaproc_cakupan.go atau di daftar tes ini", d.ID, d.Tabel, dapat, tertutup)
		}
	}
	for id := range datasetTidakDapatDibatasi {
		if !dikenal[id] {
			t.Errorf("daftar dataset tertutup memuat ID yang tidak ada di katalog: %s", id)
		}
	}
	// peran terbatas: BolehDilihat sejalan dengan DapatDibatasi; peran semua data boleh semuanya
	terbatas := peran.Cakupan{Tingkat: peran.TingkatSatk, Kode: "119091"}
	for _, d := range DaftarDataset {
		if !d.BolehDilihat(peran.CakupanSemua) || !d.BolehDilihat(peran.Cakupan{}) {
			t.Errorf("%s: peran semua data harus boleh melihat", d.ID)
		}
		if d.BolehDilihat(terbatas) != DapatDibatasi(d.Tabel) {
			t.Errorf("%s: BolehDilihat bagi peran terbatas harus sama dengan DapatDibatasi", d.ID)
		}
	}
}

// blokTabel mengembalikan teks CREATE TABLE sebuah tabel dari berkas migrasi ("" bila tidak ditemukan).
func blokTabel(t *testing.T, tabel string) string {
	t.Helper()
	berkas, err := filepath.Glob("../migrations/*.sql")
	if err != nil || len(berkas) == 0 {
		t.Fatalf("migrasi tidak ditemukan: %v", err)
	}
	re := regexp.MustCompile(`(?is)CREATE TABLE ` + regexp.QuoteMeta(tabel) + `\s*\((.*?)\n\s*\);`)
	for _, f := range berkas {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if m := re.FindStringSubmatch(string(b)); m != nil {
			return m[1]
		}
	}
	return ""
}

// Kolom yang dipakai pembatasan harus benar-benar ada pada tabelnya menurut migrasi (salah ketik akan jadi galat SQL atau, lebih buruk, pembatasan yang keliru).
func TestKolomPembatasanAdaDiMigrasi(t *testing.T) {
	for tabel := range tabelSatkerLangsung {
		b := blokTabel(t, tabel)
		if b == "" {
			t.Errorf("tabel %s tidak ditemukan di migrasi", tabel)
			continue
		}
		if !regexp.MustCompile(`(?m)^\s*kd_satker_str\s`).MatchString(b) {
			t.Errorf("tabel %s tidak punya kolom kd_satker_str", tabel)
		}
	}
	punyaKolom := func(tabel, kolom string) bool {
		return regexp.MustCompile(`(?m)^\s*` + regexp.QuoteMeta(kolom) + `\s`).MatchString(blokTabel(t, tabel))
	}
	for tabel, tt := range tabelSatkerTautan {
		if tabelSatkerLangsung[tabel] {
			t.Errorf("%s ada di golongan langsung dan tautan sekaligus", tabel)
		}
		if !punyaKolom(tabel, tt.kolom) {
			t.Errorf("tabel %s tidak punya kolom tautan %s", tabel, tt.kolom)
		}
		if len(tt.sumber) == 0 {
			t.Errorf("tabel %s tanpa sumber tautan", tabel)
		}
		for _, s := range tt.sumber {
			if !tabelSatkerLangsung[s.tabel] {
				t.Errorf("sumber tautan %s untuk %s harus tabel yang punya kd_satker_str", s.tabel, tabel)
			}
			if !punyaKolom(s.tabel, s.kolom) {
				t.Errorf("sumber tautan %s tidak punya kolom %s", s.tabel, s.kolom)
			}
		}
	}
	// tabel yang menjadi sasaran pembatasan harus ada pada katalog dataset (tidak ada entri yatim di registri)
	ada := map[string]bool{}
	for _, d := range DaftarDataset {
		ada[d.Tabel] = true
	}
	for tabel := range tabelSatkerLangsung {
		if !ada[tabel] {
			t.Errorf("registri memuat tabel yang tidak ada di katalog dataset: %s", tabel)
		}
	}
	for tabel := range tabelSatkerTautan {
		if !ada[tabel] {
			t.Errorf("registri tautan memuat tabel yang tidak ada di katalog dataset: %s", tabel)
		}
	}
}

func TestKondisiSatkerInaproc(t *testing.T) {
	satker := peran.Cakupan{Tingkat: peran.TingkatSatk, Kode: "119091"}
	ue1 := peran.Cakupan{Tingkat: peran.TingkatUE1, Kode: "01504"}
	kanwil := peran.Cakupan{Tingkat: peran.TingkatKwl, Kode: "015040199"}

	// semua data: tanpa pembatasan, nama tabel apa adanya (query peran semua data tidak berubah)
	for _, cak := range []peran.Cakupan{peran.CakupanSemua, {}} {
		if k, ok := kondisiSatkerInaproc("inaproc_paket_penyedia", cak); k != "1 = 1" || !ok {
			t.Errorf("semua data: %q %v", k, ok)
		}
		if s := sumberInaproc(cak, "inaproc_paket_penyedia"); s != "inaproc_paket_penyedia" {
			t.Errorf("sumber semua data = %q", s)
		}
		if s := sumberInaprocAlias(cak, "inaproc_paket_penyedia", "x"); s != "inaproc_paket_penyedia x" {
			t.Errorf("sumber beralias semua data = %q", s)
		}
	}

	// langsung: satker membandingkan kode, UE1/Kanwil memakai daftar satker data aset
	k, ok := kondisiSatkerInaproc("inaproc_paket_penyedia", satker)
	if !ok || !strings.Contains(k, "N'119091'") || strings.Contains(k, "DIGITALISASI_SATKER") || !strings.Contains(k, "kd_satker_str") {
		t.Errorf("satker langsung: %q %v", k, ok)
	}
	for nama, cak := range map[string]peran.Cakupan{"ue1": ue1, "kanwil": kanwil} {
		k, ok := kondisiSatkerInaproc("inaproc_tender_pengumuman", cak)
		if !ok || !strings.Contains(k, "DIGITALISASI_SATKER") || !strings.Contains(k, "N'"+cak.Kode+"'") {
			t.Errorf("%s langsung: %q %v", nama, k, ok)
		}
	}

	// tautan: tabel pembanding dan kolom kodenya tampil; kode banyak memakai STRING_SPLIT
	k, ok = kondisiSatkerInaproc("inaproc_tender_selesai_nilai", satker)
	if !ok || !strings.Contains(k, "[inaproc_tender_selesai_nilai].[kd_tender] IN (") || !strings.Contains(k, "[inaproc_tender_pengumuman]") || !strings.Contains(k, "[inaproc_tender_selesai]") || strings.Contains(k, "STRING_SPLIT") {
		t.Errorf("tautan tender: %q %v", k, ok)
	}
	k, ok = kondisiSatkerInaproc("inaproc_ekatalog6_paket_epurchasing", ue1)
	if !ok || !strings.Contains(k, "STRING_SPLIT([inaproc_ekatalog6_paket_epurchasing].[rup_code], ';')") || !strings.Contains(k, "[inaproc_paket_penyedia]") {
		t.Errorf("tautan e-katalog V6: %q %v", k, ok)
	}

	// yang tidak dapat dibatasi atau cakupan tidak sah ditutup, bukan dibuka
	if k, ok := kondisiSatkerInaproc("inaproc_program_master", satker); ok || k != "1 = 0" {
		t.Errorf("program master: %q %v, want 1 = 0 dan tidak ok", k, ok)
	}
	if k, ok := kondisiSatkerInaproc("tabel_tak_dikenal", ue1); ok || k != "1 = 0" {
		t.Errorf("tabel tak dikenal: %q %v", k, ok)
	}
	for nama, cak := range map[string]peran.Cakupan{
		"kosong":       {Tingkat: peran.Kosong},
		"kode rusak":   {Tingkat: peran.TingkatSatk, Kode: "1190'; DROP TABLE x;--"},
		"ue1 pendek":   {Tingkat: peran.TingkatUE1, Kode: "0150"},
		"tingkat lain": {Tingkat: "lain", Kode: "01504"},
	} {
		k, _ := kondisiSatkerInaproc("inaproc_paket_penyedia", cak)
		if !strings.Contains(k, "1 = 0") || strings.Contains(k, "DROP") {
			t.Errorf("%s: %q, want tertutup", nama, k)
		}
		k, _ = kondisiSatkerInaproc("inaproc_tender_selesai_nilai", cak)
		if strings.Contains(k, "DROP") || !strings.Contains(k, "1 = 0") {
			t.Errorf("%s (tautan): %q, want tertutup", nama, k)
		}
		if s := sumberInaproc(cak, "inaproc_paket_penyedia"); !strings.Contains(s, "1 = 0") || !strings.HasSuffix(s, "AS [inaproc_paket_penyedia]") {
			t.Errorf("%s: sumber = %q", nama, s)
		}
	}

	// tabel turunan: bernama sama dengan tabelnya agar kolom pada query pemanggil tetap berlaku; alias sendiri dihormati
	if s := sumberInaproc(satker, "inaproc_paket_penyedia"); !strings.HasPrefix(s, "(SELECT * FROM [inaproc_paket_penyedia] WHERE ") || !strings.HasSuffix(s, ") AS [inaproc_paket_penyedia]") {
		t.Errorf("sumber terbatas = %q", s)
	}
	if s := sumberInaprocAlias(satker, "inaproc_tender_pengumuman", "x"); !strings.HasPrefix(s, "(SELECT * FROM [inaproc_tender_pengumuman] WHERE ") || !strings.HasSuffix(s, ") AS x") {
		t.Errorf("sumber beralias terbatas = %q", s)
	}
}

// Kunci satker 6 digit: kode pendek dilengkapi nol; sama dengan yang dipakai keterhubungan satker (satu sumber).
func TestKunciSatkerInaprocKolomSatuSumber(t *testing.T) {
	if kunciInaprocSQL != kunciSatkerInaprocKolom("kd_satker_str") {
		t.Error("keterhubungan satker harus memakai ekspresi kunci yang sama")
	}
	k := kunciSatkerInaprocKolom("[kd_satker_str]")
	if !strings.Contains(k, "RIGHT('000000' + LTRIM(RTRIM([kd_satker_str])), 6)") || !strings.Contains(k, "'%[^0-9]%'") {
		t.Errorf("kunci = %q", k)
	}
}
