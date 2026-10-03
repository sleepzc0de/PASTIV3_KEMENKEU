package sapa

import (
	"archive/zip"
	"bytes"
	"errors"
	"strings"
	"testing"

	"pasti-v3-backend/sapa/docx"
)

func kasusContoh() Kasus           { return ContohKasus() }
func ndSatkerContoh() DataNDSatker { return ContohNDSatker() }
func ndUE1Contoh() DataNDUE1       { return ContohNDUE1() }
func templateBawaan(t *testing.T, kunci string) []byte {
	t.Helper()
	b, ok := TemplateBawaan(kunci)
	if !ok {
		t.Fatalf("template bawaan %s tidak ada", kunci)
	}
	return b
}

func bukaHasil(t *testing.T, h *Hasil) *docx.Doc {
	t.Helper()
	d, err := docx.Open(h.Berkas)
	if err != nil {
		t.Fatalf("hasil bukan docx yang valid: %v", err)
	}
	return d
}

func TestTotal(t *testing.T) {
	d := ndSatkerContoh()
	jumlah, p, l, err := d.Total()
	if err != nil {
		t.Fatal(err)
	}
	if jumlah != 3 || p != 355000000050 || l != 241000000000 {
		t.Fatalf("jumlah=%d perolehan=%d limit=%d", jumlah, p, l)
	}
	d.Barang[0].NilaiLimit = "abc"
	if _, _, _, err := d.Total(); err == nil {
		t.Error("nilai rusak harus galat")
	}
}

func TestIsiNDSatkerEndToEnd(t *testing.T) {
	h, err := IsiNDSatker(templateBawaan(t, DokNDSatker), kasusContoh(), ndSatkerContoh())
	if err != nil {
		t.Fatal(err)
	}
	if len(h.Peringatan) != 0 {
		t.Errorf("peringatan tak terduga: %v", h.Peringatan)
	}
	if h.NamaFile != "ND Usulan Penjualan Satker PJ-2026-00001.docx" {
		t.Errorf("nama file = %q", h.NamaFile)
	}
	doc := bukaHasil(t, h)
	text := doc.Text()

	for _, want := range []string{
		"berupa 3 (Tiga) bidang dengan harga perolehan sebesar Rp3.550.000.000,50 (Tiga Miliar Lima Ratus Lima Puluh Juta Rupiah Lima Puluh Sen) dengan nilai limit penjualan sebesar Rp2.410.000.000,00 (Dua Miliar Empat Ratus Sepuluh Juta Rupiah)",
		"c.q. Kantor Pelayanan Kekayaan Negara dan Lelang Jakarta I (KPKNL Jakarta I) sesuai tiket pada Aplikasi SIMAN Nomor SIMAN-2026-0099",
		"Yth.\n:\nSekretaris Direktorat Jenderal Contoh",
		"Barang rusak berat dan tidak ekonomis untuk diperbaiki.\nBiaya pemeliharaan tinggi.",
		"Jakarta, [@TanggalND]",
		"Budi Santoso", "198001012005011001", "Kepala Kantor Wilayah DJKN Jakarta",
		"NOMOR [@NomorND]",
	} {
		if !strings.Contains(strings.ReplaceAll(text, "\n:\n", "\n:\n"), want) {
			t.Errorf("hasil tidak memuat %q", want)
		}
	}
	if strings.Contains(text, "<<") || strings.Contains(text, ">>") {
		i := strings.Index(text, "<<")
		t.Errorf("masih ada penanda tersisa: ...%s...", text[max0(i-40):min2(i+60, len(text))])
	}
	if tags := doc.Tags(); len(tags) != 0 {
		t.Errorf("Tags() = %v", tags)
	}

	// Tabel daftar barang: judul, nomor kolom, 3 barang, JUMLAH.
	items := doc.FindTable("Nama Barang")
	rows := items.Rows()
	if len(rows) != 6 {
		t.Fatalf("daftar barang %d baris, want 6", len(rows))
	}
	cell := func(r, c int) string { return rows[r].Cells()[c].Text() }
	if cell(2, 0) != "1" || cell(2, 1) != "Tanah Kantor" || cell(2, 7) != "1.000.000.000,00" || cell(2, 8) != "900.000.000,00" {
		t.Errorf("baris barang 1 salah: %q %q %q %q", cell(2, 0), cell(2, 1), cell(2, 7), cell(2, 8))
	}
	if cell(3, 4) != "Jl. Merdeka 1\nLantai 2" || cell(3, 7) != "2.500.000.000,50" {
		t.Errorf("baris barang 2 salah: %q %q", cell(3, 4), cell(3, 7))
	}
	if cell(4, 0) != "3" {
		t.Errorf("nomor urut baris 3 = %q", cell(4, 0))
	}
	if cell(5, 1) != "3.550.000.000,50" || cell(5, 2) != "2.410.000.000,00" {
		t.Errorf("JUMLAH salah: %q %q", cell(5, 1), cell(5, 2))
	}

	// Checklist: 19 baris data; baris yang dikenali terisi, termasuk baris 15 yang selnya kosong di template.
	ck := doc.FindTable("Jenis Data/Dokumen").Rows()
	want := map[int]string{
		1:  "Barang rusak berat", // alasan lewat penanda biasa
		2:  "Ada",                // daftar barang (otomatis)
		3:  "Tidak ada",          // SK Tim belum dicentang
		4:  "Ada, Nomor BA-12/2026, tanggal 15 September 2026",
		5:  "Ada, Nomor PSP-1, tanggal 2 Januari 2020",
		6:  "Ada",
		7:  "Ada", // SKPP otomatis
		9:  "Ada", // SPTJ formil otomatis
		10: "Tidak ada",
		14: "Ada",
		15: "Ada", // sel kosong di template
		16: "Ada",
		19: "SIMAN-2026-0099",
	}
	for n, w := range want {
		if got := ck[n].Cells()[2].Text(); !strings.HasPrefix(got, w) {
			t.Errorf("checklist baris %d (%q) = %q, want awalan %q", n, strings.TrimSpace(ck[n].Cells()[1].Text())[:20], got, w)
		}
	}
	// Surat-surat pernyataan: penandatangan dan kota terisi.
	if strings.Count(text, "Budi Santoso") < 5 {
		t.Errorf("nama penandatangan harus muncul di setiap surat, hanya %d kali", strings.Count(text, "Budi Santoso"))
	}
}

func max0(i int) int {
	if i < 0 {
		return 0
	}
	return i
}

func min2(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func TestEveryChecklistRowOfProvidedTemplatesIsRecognised(t *testing.T) {
	for _, kunci := range []string{DokNDSatker, DokNDUE1} {
		doc, err := docx.Open(templateBawaan(t, kunci))
		if err != nil {
			t.Fatal(err)
		}
		rows := doc.FindTable("Jenis Data/Dokumen").Rows()[1:]
		if len(rows) != 19 {
			t.Fatalf("%s: %d baris checklist", kunci, len(rows))
		}
		kunciTerpakai := map[string]int{}
		for n, r := range rows {
			label := strings.ToLower(r.Cells()[1].Text())
			var cocok []string
			for _, def := range barisChecklist {
				if strings.Contains(label, def.cocok) {
					cocok = append(cocok, def.kunci)
				}
			}
			isAlasan := n == 0 || n == 18 // "Penjelasan..." dan "Tiket SIMAN" diisi lewat penanda biasa
			switch {
			case isAlasan && len(cocok) != 0:
				t.Errorf("%s baris %d (%q) seharusnya tidak cocok dengan definisi checklist: %v", kunci, n+1, label, cocok)
			case !isAlasan && len(cocok) == 0:
				t.Errorf("%s baris %d (%q) tidak dikenali", kunci, n+1, label)
			case len(cocok) > 1:
				t.Errorf("%s baris %d (%q) ambigu: %v", kunci, n+1, label, cocok)
			}
			for _, k := range cocok {
				kunciTerpakai[k]++
			}
		}
		for _, def := range barisChecklist {
			if kunciTerpakai[def.kunci] != 1 {
				t.Errorf("%s: dokumen %q cocok dengan %d baris, want 1", kunci, def.kunci, kunciTerpakai[def.kunci])
			}
		}
	}
}

func TestIsiNDUE1EndToEnd(t *testing.T) {
	h, err := IsiNDUE1(templateBawaan(t, DokNDUE1), kasusContoh(), ndSatkerContoh(), ndUE1Contoh())
	if err != nil {
		t.Fatal(err)
	}
	if len(h.Peringatan) != 0 {
		t.Errorf("peringatan tak terduga: %v", h.Peringatan)
	}
	doc := bukaHasil(t, h)
	text := doc.Text()
	for _, want := range []string{
		"Berita Acara Penelitian dan Pemeriksaan Barang Milik Negara yang Diusulkan untuk Dilakukan Penjualan pada Kantor Pelayanan Kekayaan Negara dan Lelang Jakarta I Nomor BA-12/2026 tanggal 15 September 2026, adalah karena Barang rusak berat",
		"Nota Dinas Kepala Kantor Pelayanan Kekayaan Negara dan Lelang Jakarta I Nomor ND-77/KPKNL.JKT1/2026 tanggal 1 Oktober 2026 hal Permohonan Penjualan BMN pada KPKNL Jakarta I;",
		"Dewi Lestari", "Direktur Barang Milik Negara", "Kepala Kantor Wilayah DJKN Jakarta",
		"Rp3.550.000.000,50",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("hasil tidak memuat %q", want)
		}
	}
	if strings.Contains(text, "<<") {
		i := strings.Index(text, "<<")
		t.Errorf("penanda tersisa: %s", text[i:min2(i+60, len(text))])
	}
	// Penandatangan Nota Dinas UE1 adalah pejabat UE1; kepala satker tidak boleh muncul sebagai penandatangan.
	if strings.Contains(text, "Budi Santoso") {
		t.Error("nama kepala satker tidak boleh muncul pada Nota Dinas UE1")
	}
	if strings.Count(text, "Dewi Lestari") < 2 {
		t.Errorf("nama pejabat UE1 harus muncul di blok tanda tangan Nota Dinas dan Lampiran, muncul %d kali", strings.Count(text, "Dewi Lestari"))
	}
}

func TestIsiNDTanpaBeritaAcaraMemakaiStrip(t *testing.T) {
	d := ndSatkerContoh()
	d.Dokumen[DokBeritaAcara] = DokPendukung{Ada: false}
	h, err := IsiNDUE1(templateBawaan(t, DokNDUE1), kasusContoh(), d, ndUE1Contoh())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(bukaHasil(t, h).Text(), "Nomor - tanggal -, adalah karena") {
		t.Error("tanpa BA, nomor dan tanggal BA harus diisi tanda hubung, bukan dibiarkan kosong")
	}
}

func TestValidasiND(t *testing.T) {
	d := ndSatkerContoh()
	if v := d.Validasi(); len(v) != 0 {
		t.Fatalf("data contoh harus valid: %v", v)
	}
	cases := []struct {
		nama  string
		ubah  func(*DataNDSatker)
		harus string
	}{
		{"belum RP4", func(d *DataNDSatker) { d.SudahRP4 = false }, "RP4"},
		{"tanpa barang", func(d *DataNDSatker) { d.Barang = nil }, "minimal berisi satu barang"},
		{"NIP pendek", func(d *DataNDSatker) { d.Penandatangan.NIP = "123" }, "NIP"},
		{"NIP huruf", func(d *DataNDSatker) { d.Penandatangan.NIP = "19800101200501100x" }, "NIP"},
		{"limit nol", func(d *DataNDSatker) { d.Barang[0].NilaiLimit = "0" }, "lebih dari nol"},
		{"nilai bukan angka", func(d *DataNDSatker) { d.Barang[1].NilaiPerolehan = "banyak" }, "Nilai perolehan barang 2"},
		{"tahun aneh", func(d *DataNDSatker) { d.Barang[0].TahunPerolehan = "20" }, "Tahun perolehan barang 1"},
		{"alasan kosong", func(d *DataNDSatker) { d.Alasan = "  " }, "Alasan"},
		{"BA tanpa nomor", func(d *DataNDSatker) { d.Dokumen[DokBeritaAcara] = DokPendukung{Ada: true} }, "nomor dan tanggal Berita Acara"},
		{"dokumen tak dikenal", func(d *DataNDSatker) { d.Dokumen["rahasia"] = DokPendukung{Ada: true} }, "tidak dikenal"},
		{"tanggal dokumen salah", func(d *DataNDSatker) { d.Dokumen[DokPSP] = DokPendukung{Ada: true, Tanggal: "kemarin"} }, "TTTT-BB-HH"},
		{"terlalu banyak barang", func(d *DataNDSatker) { d.Barang = make([]Barang, 501) }, "terlalu banyak"},
	}
	for _, c := range cases {
		d := ndSatkerContoh()
		d.Dokumen = map[string]DokPendukung{DokBeritaAcara: {Ada: true, Nomor: "BA-1", Tanggal: "2026-09-15"}, DokPSP: {Ada: true}}
		c.ubah(&d)
		d.Rapikan()
		got := strings.Join(d.Validasi(), "; ")
		if !strings.Contains(got, c.harus) {
			t.Errorf("%s: galat %q tidak memuat %q", c.nama, got, c.harus)
		}
	}
}

func TestIsiMenolakDataTidakValid(t *testing.T) {
	d := ndSatkerContoh()
	d.SudahRP4 = false
	_, err := IsiNDSatker(templateBawaan(t, DokNDSatker), kasusContoh(), d)
	var ev *ErrValidasi
	if !errors.As(err, &ev) || len(ev.Rincian) == 0 {
		t.Fatalf("err = %v; harus *ErrValidasi", err)
	}
	if _, err := IsiNDSatker(nil, kasusContoh(), ndSatkerContoh()); !errors.Is(err, ErrTemplateTidakAda) {
		t.Errorf("tanpa template: %v", err)
	}
	if _, err := IsiNDSatker([]byte("bukan docx"), kasusContoh(), ndSatkerContoh()); err == nil || errors.Is(err, ErrTemplateTidakAda) {
		t.Errorf("template rusak: %v", err)
	}
}

func TestRapikanMenormalkanNIPDanSpasi(t *testing.T) {
	d := ndSatkerContoh()
	d.Penandatangan.NIP = " 19800101 200501 1001 "
	d.Kota = "  Jakarta  "
	d.Rapikan()
	if d.Penandatangan.NIP != "198001012005011001" || d.Kota != "Jakarta" {
		t.Errorf("rapikan: %q %q", d.Penandatangan.NIP, d.Kota)
	}
}

// ---------------------------------------------------------------- SK Tim dan BA dengan template buatan

func buatDocx(t *testing.T, body string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	add := func(name, content string) {
		w, _ := zw.Create(name)
		w.Write([]byte(content))
	}
	add("[Content_Types].xml", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="xml" ContentType="application/xml"/></Types>`)
	add("word/document.xml", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body>`+body+`</w:body></w:document>`)
	zw.Close()
	return buf.Bytes()
}

func para(s string) string { return `<w:p><w:r><w:t xml:space="preserve">` + s + `</w:t></w:r></w:p>` }

func timContoh() DataTim { return ContohTim() }

func TestIsiSKTimDenganTabelAnggota(t *testing.T) {
	body := para("Menetapkan &lt;&lt;jenis tim&gt;&gt; pada &lt;&lt;nama satker&gt;&gt; di &lt;&lt;kota&gt;&gt; ({{x}}) Noreg &lt;&lt;nomor tiket&gt;&gt;; masa tugas &lt;&lt;masa awal tugas&gt;&gt; s.d. &lt;&lt;masa akhir tugas&gt;&gt;; oleh &lt;&lt;jabatan pimpinan&gt;&gt;; &lt;&lt;jumlah anggota&gt;&gt; anggota.") +
		`<w:tbl><w:tr><w:tc>` + para("No") + `</w:tc><w:tc>` + para("Nama") + `</w:tc></w:tr><w:tr><w:tc>` + para("&lt;&lt;no&gt;&gt;") + `</w:tc><w:tc>` + para("&lt;&lt;nama anggota&gt;&gt; (&lt;&lt;jabatan anggota&gt;&gt;) - &lt;&lt;kedudukan&gt;&gt;") + `</w:tc></w:tr></w:tbl>`
	h, err := IsiSKTim(buatDocx(t, body), kasusContoh(), timContoh())
	if err != nil {
		t.Fatal(err)
	}
	if len(h.Peringatan) != 0 {
		t.Errorf("peringatan: %v", h.Peringatan)
	}
	doc := bukaHasil(t, h)
	text := doc.Text()
	if !strings.Contains(text, "Menetapkan Tim Internal Penjualan pada Kantor Pelayanan Kekayaan Negara dan Lelang Jakarta I di Jakarta ({{x}}) Noreg PJ-2026-00001; masa tugas 2 Januari 2026 s.d. 31 Desember 2026; oleh Kepala Kantor; 3 anggota.") {
		t.Errorf("kalimat SK salah:\n%s", text)
	}
	rows := doc.Tables()[0].Rows()
	if len(rows) != 4 {
		t.Fatalf("%d baris, want 4", len(rows))
	}
	if got := rows[3].Cells()[0].Text() + "|" + rows[3].Cells()[1].Text(); got != "3|Andi (Pelaksana) - Anggota" {
		t.Errorf("baris anggota terakhir = %q", got)
	}
}

func TestIsiSKTimTanpaTabelMemakaiDaftarAnggotaAtauMemberiPeringatan(t *testing.T) {
	h, err := IsiSKTim(buatDocx(t, para("Anggota:\n&lt;&lt;daftar anggota&gt;&gt;")), kasusContoh(), timContoh())
	if err != nil {
		t.Fatal(err)
	}
	if len(h.Peringatan) != 0 {
		t.Errorf("peringatan: %v", h.Peringatan)
	}
	if !strings.Contains(bukaHasil(t, h).Text(), "1. Budi – Kepala Seksi A – Ketua\n2. Siti – Kepala Seksi B – Sekretaris\n3. Andi – Pelaksana – Anggota") {
		t.Errorf("daftar anggota salah: %s", bukaHasil(t, h).Text())
	}

	h, err = IsiSKTim(buatDocx(t, para("Tidak ada penanda anggota")), kasusContoh(), timContoh())
	if err != nil {
		t.Fatal(err)
	}
	if len(h.Peringatan) != 1 || !strings.Contains(h.Peringatan[0], "daftar anggota tidak tercetak") {
		t.Errorf("peringatan = %v", h.Peringatan)
	}
}

func TestValidasiTim(t *testing.T) {
	d := timContoh()
	if v := d.Validasi(); len(v) != 0 {
		t.Fatalf("data contoh harus valid: %v", v)
	}
	d.MasaAkhir = "2025-12-31"
	d.JenisTim = "Tim Rahasia"
	d.Anggota = append(d.Anggota, Anggota{})
	got := strings.Join(d.Validasi(), "; ")
	for _, want := range []string{"Masa akhir tugas tidak boleh sebelum", "Jenis tim tidak valid", "Nama anggota 4 wajib"} {
		if !strings.Contains(got, want) {
			t.Errorf("galat %q tidak memuat %q", got, want)
		}
	}
	if (DataTim{}).Validasi() == nil {
		t.Error("data kosong harus tidak valid")
	}
}

func TestIsiBAHariDanTanggal(t *testing.T) {
	body := para("Pada hari &lt;&lt;hari penelitian&gt;&gt; tanggal &lt;&lt;tanggal penelitian&gt;&gt; bulan &lt;&lt;bulan penelitian&gt;&gt; tahun &lt;&lt;tahun penelitian&gt;&gt; (&lt;&lt;tanggal penelitian lengkap&gt;&gt;), &lt;&lt;nama tim&gt;&gt; meneliti &lt;&lt;bentuk pemindahtanganan&gt;&gt; &lt;&lt;nama satker&gt;&gt; / &lt;&lt;nomor tiket&gt;&gt;.")
	h, err := IsiBA(buatDocx(t, body), kasusContoh(), DataBA{Bentuk: "Penjualan", TanggalPenelitian: "2026-10-03", NamaTim: "Tim Internal Penjualan"})
	if err != nil {
		t.Fatal(err)
	}
	want := "Pada hari Sabtu tanggal 3 bulan Oktober tahun 2026 (3 Oktober 2026), Tim Internal Penjualan meneliti Penjualan Kantor Pelayanan Kekayaan Negara dan Lelang Jakarta I / PJ-2026-00001."
	if got := bukaHasil(t, h).Text(); got != want {
		t.Errorf("teks = %q", got)
	}
	if _, err := IsiBA(buatDocx(t, body), kasusContoh(), DataBA{Bentuk: "Lainnya"}); err == nil {
		t.Error("data BA tidak valid harus ditolak")
	}
}

func TestPenandaTakDikenalDiberiPeringatan(t *testing.T) {
	h, err := IsiBA(buatDocx(t, para("&lt;&lt;nama satker&gt;&gt; dan &lt;&lt;lokasi gudang&gt;&gt;")), kasusContoh(), DataBA{Bentuk: "Penjualan", TanggalPenelitian: "2026-10-03", NamaTim: "T"})
	if err != nil {
		t.Fatal(err)
	}
	if len(h.Peringatan) != 1 || !strings.Contains(h.Peringatan[0], "lokasi gudang") {
		t.Errorf("peringatan = %v", h.Peringatan)
	}
	if !strings.Contains(bukaHasil(t, h).Text(), "<<lokasi gudang>>") {
		t.Error("penanda tak dikenal harus dibiarkan terlihat")
	}
}

func TestNamaFileAmanDariKarakterTerlarang(t *testing.T) {
	if got := namaFile("ND: Satker/UE1", `PJ-2026\1*`); strings.ContainsAny(got, `\/:*?"<>|`) {
		t.Errorf("nama file %q memuat karakter terlarang", got)
	}
}

func TestDaftarJenisLengkap(t *testing.T) {
	for _, j := range DaftarJenis {
		if _, _, ok := TahapByKunci(j.Tahap); !ok {
			t.Errorf("jenis %s menunjuk tahap %q yang tidak ada", j.Kunci, j.Tahap)
		}
		if _, ok := TemplateBawaan(j.Kunci); ok != j.Bawaan {
			t.Errorf("jenis %s: Bawaan=%v tetapi berkas bawaan ada=%v", j.Kunci, j.Bawaan, ok)
		}
		seen := map[string]bool{}
		for _, p := range j.Penanda {
			if seen[p.Nama] {
				t.Errorf("jenis %s: penanda %q ganda", j.Kunci, p.Nama)
			}
			seen[p.Nama] = true
		}
	}
	for _, tp := range TahapPenjualan {
		for _, k := range tp.Dokumen {
			if j, ok := JenisByKunci(k); !ok || j.Tahap != tp.Kunci {
				t.Errorf("tahap %s menghasilkan %q yang tidak cocok dengan daftar jenis", tp.Kunci, k)
			}
		}
	}
}

// Semua penanda pada template bawaan harus dikenal (tercantum di daftar penanda atau punya nilai pada pembangun).
func TestPenandaTemplateBawaanTercakupDiDaftar(t *testing.T) {
	for _, kunci := range []string{DokNDSatker, DokNDUE1} {
		jenis, _ := JenisByKunci(kunci)
		known := map[string]bool{}
		for _, p := range jenis.Penanda {
			known[p.Nama] = true
		}
		doc, _ := docx.Open(templateBawaan(t, kunci))
		for _, tag := range doc.Tags() {
			if !known[tag] {
				t.Errorf("%s: penanda <<%s>> ada di template tetapi tidak tercantum di DaftarJenis", kunci, tag)
			}
		}
	}
}

func TestNIPAnggotaOpsionalDanDivalidasi(t *testing.T) {
	d := timContoh()
	d.Anggota[0].NIP = " 1980 0101 2005 0110 01 "
	d.Rapikan()
	if d.Anggota[0].NIP != "198001012005011001" {
		t.Errorf("spasi pada NIP harus dibuang: %q", d.Anggota[0].NIP)
	}
	if v := d.Validasi(); len(v) != 0 {
		t.Errorf("NIP sah + anggota tanpa NIP harus lolos: %v", v)
	}
	for _, bad := range []string{"123", "19800101200501100112345", "19800101A005011001", "1980-01-01"} {
		d := timContoh()
		d.Anggota[1].NIP = bad
		v := d.Validasi()
		if len(v) != 1 || !strings.Contains(v[0], "NIP anggota 2") {
			t.Errorf("NIP %q harus ditolak: %v", bad, v)
		}
	}
}

func TestIsiSKTimMengisiNIPAnggota(t *testing.T) {
	body := `<w:tbl><w:tr><w:tc>` + para("Nama") + `</w:tc><w:tc>` + para("NIP") + `</w:tc></w:tr><w:tr><w:tc>` + para("&lt;&lt;nama anggota&gt;&gt;") + `</w:tc><w:tc>` + para("&lt;&lt;nip anggota&gt;&gt;") + `</w:tc></w:tr></w:tbl>`
	h, err := IsiSKTim(buatDocx(t, body), kasusContoh(), timContoh())
	if err != nil {
		t.Fatal(err)
	}
	if len(h.Peringatan) != 0 {
		t.Errorf("peringatan: %v", h.Peringatan)
	}
	rows := bukaHasil(t, h).Tables()[0].Rows()
	if len(rows) != 4 {
		t.Fatalf("%d baris, want 4", len(rows))
	}
	// Anggota pertama punya NIP; yang lain kosong (sel dikosongkan, penanda tidak tersisa).
	if got := rows[1].Cells()[1].Text(); got != "198001012005011001" {
		t.Errorf("NIP anggota 1 = %q", got)
	}
	if got := rows[2].Cells()[1].Text(); got != "" {
		t.Errorf("anggota tanpa NIP harus kosong, bukan penanda: %q", got)
	}
}
