package docx

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func loadTemplate(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "templates", name))
	if err != nil {
		t.Fatalf("template %s: %v", name, err)
	}
	return b
}

// miniDocx membentuk .docx minimal dari isi <w:body>.
func miniDocx(t *testing.T, body string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	add := func(name, content string) {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		w.Write([]byte(content))
	}
	add("[Content_Types].xml", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="xml" ContentType="application/xml"/></Types>`)
	add("word/document.xml", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" xmlns:w14="http://schemas.microsoft.com/office/word/2010/wordml"><w:body>`+body+`</w:body></w:document>`)
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func open(t *testing.T, b []byte) *Doc {
	t.Helper()
	d, err := Open(b)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func roundTrip(t *testing.T, d *Doc) *Doc {
	t.Helper()
	b, err := d.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	return open(t, b)
}

func TestNormalizeTag(t *testing.T) {
	for in, want := range map[string]string{
		"nama satker":                      "nama satker",
		" jabatan  pejabat penandatangan ": "jabatan pejabat penandatangan",
		"Terbilang Nilai Limit":            "terbilang nilai limit",
	} {
		if got := NormalizeTag(in); got != want {
			t.Errorf("NormalizeTag(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestTagsOfProvidedTemplates(t *testing.T) {
	cases := map[string][]string{
		"nd_satker.docx": {"sekretaris ue1", "nama satker", "jumlah bmn", "terbilang jumlah bmn", "total nilai perolehan", "terbilang nilai perolehan", "total nilai limit",
			"terbilang nilai limit", "nama satker singkat", "nomor tiket", "kepala kantor wilayah", "jenis bmn", "alasan", "hasil checklist", "tiket siman",
			"nama pejabat penandatangan", "nip pejabat penandatangan", "jabatan pejabat penandatangan", "kota"},
		"nd_ue1.docx": {"sekretaris ue1", "nama satker", "nomor ba", "tanggal ba", "nomor nd usulan satker", "tanggal nd usulan", "hal nd usulan",
			"nama pejabat ue1 penandatangan", "jabatan pejabat ue1 penandatangan", "kepala kantor wilayah", "pejabat pengelola", "hasil checklist", "tiket siman"},
	}
	for name, want := range cases {
		got := open(t, loadTemplate(t, name)).Tags()
		set := map[string]bool{}
		for _, g := range got {
			set[g] = true
		}
		for _, w := range want {
			if !set[w] {
				t.Errorf("%s: penanda %q tidak terdeteksi (terdeteksi: %v)", name, w, got)
			}
		}
	}
}

func TestReplaceAcrossRunsInProvidedTemplate(t *testing.T) {
	d := open(t, loadTemplate(t, "nd_satker.docx"))
	values := map[string]string{
		"jumlah bmn": "3", "terbilang jumlah bmn": "(Tiga)",
		"total nilai perolehan": "Rp1.500.000,00", "terbilang nilai perolehan": "(Satu Juta Lima Ratus Ribu Rupiah)",
		"total nilai limit": "Rp900.000,00", "terbilang nilai limit": "(Sembilan Ratus Ribu Rupiah)",
		"nama satker": "KPKNL Jakarta I", "nama satker singkat": "", "nomor tiket": "TKT-1234",
	}
	missing := d.Replace(values)

	text := d.Text()
	want := "berupa 3 (Tiga) dengan harga perolehan sebesar Rp1.500.000,00 (Satu Juta Lima Ratus Ribu Rupiah) dengan nilai limit penjualan sebesar Rp900.000,00 (Sembilan Ratus Ribu Rupiah)  pada Kementerian Keuangan c.q. KPKNL Jakarta I sesuai tiket pada Aplikasi SIMAN Nomor TKT-1234, dengan rincian"
	if !strings.Contains(text, want) {
		i := strings.Index(text, "bersama ini")
		t.Fatalf("kalimat inti tidak sesuai.\nada : %s\nwant: %s", text[i:i+450], want)
	}
	if strings.Contains(text, "()") || strings.Contains(text, "( )") || strings.Contains(text, "KPKNL Jakarta I ()") {
		t.Error("penanda kosong yang diapit kurung harus ikut menghapus kurungnya")
	}
	for _, still := range []string{"[@NomorND]", "[@TanggalND]"} {
		if !strings.Contains(text, still) {
			t.Errorf("penanda aplikasi persuratan %s tidak boleh disentuh", still)
		}
	}
	if !strings.Contains(text, "<<alasan>>") {
		t.Error("penanda tanpa nilai harus dibiarkan")
	}
	hasMissing := map[string]bool{}
	for _, m := range missing {
		hasMissing[m] = true
	}
	if !hasMissing["alasan"] || !hasMissing["kota"] || hasMissing["nama satker"] {
		t.Errorf("daftar penanda yang belum terisi salah: %v", missing)
	}
	// Tidak ada sisa penanda yang sebenarnya punya nilai.
	for _, tag := range d.Tags() {
		if _, ok := values[tag]; ok {
			t.Errorf("penanda %q masih tersisa padahal punya nilai", tag)
		}
	}
}

func TestOutputIsValidDocxAndKeepsOtherParts(t *testing.T) {
	orig := loadTemplate(t, "nd_ue1.docx")
	d := open(t, orig)
	d.Replace(map[string]string{"nama satker": "Satker Contoh"})
	out, err := d.Bytes()
	if err != nil {
		t.Fatal(err)
	}

	readAll := func(b []byte) map[string][]byte {
		zr, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
		if err != nil {
			t.Fatal(err)
		}
		m := map[string][]byte{}
		for _, f := range zr.File {
			rc, _ := f.Open()
			data, _ := io.ReadAll(rc)
			rc.Close()
			m[f.Name] = data
		}
		return m
	}
	a, b := readAll(orig), readAll(out)
	var an, bn []string
	for k := range a {
		an = append(an, k)
	}
	for k := range b {
		bn = append(bn, k)
	}
	sort.Strings(an)
	sort.Strings(bn)
	if strings.Join(an, ",") != strings.Join(bn, ",") {
		t.Fatalf("daftar entri zip berubah:\n%v\n%v", an, bn)
	}
	for name, data := range a {
		if partPattern.MatchString(name) {
			// Bagian yang diolah harus tetap XML yang valid.
			dec := xml.NewDecoder(bytes.NewReader(b[name]))
			for {
				if _, err := dec.Token(); err != nil {
					if err != io.EOF {
						t.Fatalf("%s bukan XML valid: %v", name, err)
					}
					break
				}
			}
			continue
		}
		if !bytes.Equal(data, b[name]) {
			t.Errorf("bagian %s berubah padahal tidak diolah", name)
		}
	}
	if !strings.Contains(roundTrip(t, d).Text(), "Satker Contoh") {
		t.Error("nilai hilang setelah ditulis dan dibaca ulang")
	}
}

func TestPlaceholderSplitAcrossFormattedRunsKeepsFirstRunStyle(t *testing.T) {
	body := `<w:p><w:r><w:rPr><w:b/></w:rPr><w:t xml:space="preserve">Halo &lt;&lt;na</w:t></w:r>` +
		`<w:r><w:rPr><w:i/></w:rPr><w:t>ma </w:t></w:r><w:r><w:t>satker&gt;&gt;, selamat</w:t></w:r></w:p>`
	d := open(t, miniDocx(t, body))
	d.Replace(map[string]string{"nama satker": "BPK"})
	got := roundTrip(t, d)
	if got.Text() != "Halo BPK, selamat" {
		t.Fatalf("teks = %q", got.Text())
	}
	xmlOut, _ := got.Bytes()
	zr, _ := zip.NewReader(bytes.NewReader(xmlOut), int64(len(xmlOut)))
	for _, f := range zr.File {
		if f.Name == documentPart {
			rc, _ := f.Open()
			b, _ := io.ReadAll(rc)
			rc.Close()
			s := string(b)
			// "BPK" ada di run pertama (yang tebal), bukan di run miring.
			iBold, iBPK, iItalic := strings.Index(s, "<w:b/>"), strings.Index(s, "BPK"), strings.Index(s, "<w:i/>")
			if !(iBold < iBPK && iBPK < iItalic) {
				t.Errorf("nilai harus berada di run pertama (gaya tebal): %s", s)
			}
		}
	}
}

func TestMultilineValueBecomesLineBreaks(t *testing.T) {
	d := open(t, miniDocx(t, `<w:p><w:r><w:t>Alasan: &lt;&lt;alasan&gt;&gt;.</w:t></w:r></w:p>`))
	d.Replace(map[string]string{"alasan": "rusak berat\nbiaya perbaikan tinggi"})
	got := roundTrip(t, d)
	if got.Text() != "Alasan: rusak berat\nbiaya perbaikan tinggi." {
		t.Fatalf("teks = %q", got.Text())
	}
	b, _ := got.Bytes()
	if !bytes.Contains(unzip(t, b, documentPart), []byte("<w:br")) {
		t.Error("baris baru harus menjadi <w:br/>")
	}
}

func unzip(t *testing.T, b []byte, name string) []byte {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range zr.File {
		if f.Name == name {
			rc, _ := f.Open()
			defer rc.Close()
			data, _ := io.ReadAll(rc)
			return data
		}
	}
	t.Fatalf("entri %s tidak ada", name)
	return nil
}

func TestValueIsEscapedAndControlCharsDropped(t *testing.T) {
	d := open(t, miniDocx(t, `<w:p><w:r><w:t>&lt;&lt;x&gt;&gt;</w:t></w:r></w:p>`))
	d.Replace(map[string]string{"x": "A & B <tag> \x00\x07 \"q\""})
	got := roundTrip(t, d)
	if got.Text() != `A & B <tag>  "q"` {
		t.Fatalf("teks = %q", got.Text())
	}
}

func TestRepeatedTagsAndAdjacentTags(t *testing.T) {
	d := open(t, miniDocx(t, `<w:p><w:r><w:t>&lt;&lt;a&gt;&gt;&lt;&lt;b&gt;&gt; dan &lt;&lt;a&gt;&gt;</w:t></w:r></w:p>`))
	d.Replace(map[string]string{"a": "1", "b": "2"})
	if got := roundTrip(t, d).Text(); got != "12 dan 1" {
		t.Fatalf("teks = %q", got)
	}
}

func TestUnicodeAroundTags(t *testing.T) {
	d := open(t, miniDocx(t, `<w:p><w:r><w:t>Rp — &lt;&lt;n&gt;&gt; — ✓ é</w:t></w:r></w:p>`))
	d.Replace(map[string]string{"n": "≥ 5"})
	if got := roundTrip(t, d).Text(); got != "Rp — ≥ 5 — ✓ é" {
		t.Fatalf("teks = %q", got)
	}
}

func TestProvidedTemplateTables(t *testing.T) {
	d := open(t, loadTemplate(t, "nd_satker.docx"))

	items := d.FindTable("Nama Barang")
	if items == nil {
		t.Fatal("tabel daftar barang tidak ditemukan")
	}
	rows := items.Rows()
	if len(rows) != 4 {
		t.Fatalf("daftar barang: %d baris, want 4 (judul, nomor kolom, contoh isi, jumlah)", len(rows))
	}
	data := rows[2]
	if len(data.Cells()) != 10 {
		t.Fatalf("baris isi punya %d sel", len(data.Cells()))
	}
	// Gandakan baris isi untuk tiga barang, lalu isi sel menurut urutan kolom.
	vals := [][]string{
		{"1", "Tanah A", "2010101001", "1", "Jl. A", "Baik", "2001", "1.000.000,00", "900.000,00", "-"},
		{"2", "Gedung B", "4010101001", "2", "Jl. B\nlantai 2", "Rusak Berat", "1999", "2.000.000,00", "1.500.000,00", ""},
	}
	prev := data
	for i := 1; i < len(vals); i++ {
		prev = prev.CloneAfter()
	}
	all := items.Rows()
	for i, v := range vals {
		for j, c := range all[2+i].Cells() {
			c.SetText(v[j])
		}
	}
	total := all[len(all)-1]
	cells := total.Cells()
	if !strings.Contains(total.Text(), "JUMLAH") || len(cells) != 4 {
		t.Fatalf("baris jumlah tidak dikenali: %q (%d sel)", total.Text(), len(cells))
	}
	cells[1].SetText("3.000.000,00")
	cells[2].SetText("2.400.000,00")

	re := open(t, mustBytes(t, d))
	tbl := re.FindTable("Nama Barang")
	got := tbl.Rows()
	if len(got) != 5 {
		t.Fatalf("setelah penggandaan: %d baris, want 5", len(got))
	}
	if txt := got[3].Cells()[1].Text(); txt != "Gedung B" {
		t.Errorf("sel (baris 3, kolom 1) = %q", txt)
	}
	if txt := got[3].Cells()[4].Text(); txt != "Jl. B\nlantai 2" {
		t.Errorf("sel multibaris = %q", txt)
	}
	if txt := got[4].Cells()[1].Text(); txt != "3.000.000,00" {
		t.Errorf("total perolehan = %q", txt)
	}

	// Tabel checklist: baris 15 punya sel kosong tanpa penanda; baris lain memuat <<hasil checklist>>.
	ck := re.FindTable("Jenis Data/Dokumen")
	if ck == nil || len(ck.Rows()) != 20 {
		t.Fatalf("tabel checklist tidak sesuai harapan")
	}
	cr := ck.Rows()
	if got := cr[2].Cells()[2].Text(); got != "<<hasil checklist>>" {
		t.Fatalf("baris 2 = %q", got)
	}
	cr[2].Cells()[2].SetText("Ada")
	empty := cr[15].Cells()[2]
	if empty.Text() != "" {
		t.Fatalf("baris 15 seharusnya kosong, isi = %q", empty.Text())
	}
	empty.SetText("Tidak ada")
	re2 := open(t, mustBytes(t, re))
	cr2 := re2.FindTable("Jenis Data/Dokumen").Rows()
	if cr2[2].Cells()[2].Text() != "Ada" || cr2[15].Cells()[2].Text() != "Tidak ada" {
		t.Errorf("isi sel checklist: %q / %q", cr2[2].Cells()[2].Text(), cr2[15].Cells()[2].Text())
	}
}

func mustBytes(t *testing.T, d *Doc) []byte {
	t.Helper()
	b, err := d.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// Template buatan Word sendiri sudah memuat beberapa paraId ganda (Word tidak mempermasalahkannya); yang dijaga
// di sini adalah penggandaan baris tidak menambah id ganda.
func TestClonedRowsDoNotAddDuplicateParagraphIDs(t *testing.T) {
	countDuplicates := func(b []byte) int {
		ids := map[string]int{}
		for _, part := range strings.Split(string(unzip(t, b, documentPart)), `w14:paraId="`)[1:] {
			ids[part[:8]]++
		}
		n := 0
		for _, c := range ids {
			if c > 1 {
				n += c - 1
			}
		}
		return n
	}
	orig := loadTemplate(t, "nd_satker.docx")
	d := open(t, orig)
	row := d.FindTable("Nama Barang").Rows()[2]
	for i := 0; i < 5; i++ {
		row = row.CloneAfter()
	}
	before, after := countDuplicates(orig), countDuplicates(mustBytes(t, d))
	if after > before {
		t.Fatalf("id paragraf ganda bertambah dari %d menjadi %d", before, after)
	}
}

func TestRepeatRow(t *testing.T) {
	row := func(a, b string) string {
		return `<w:tr><w:tc><w:p><w:r><w:t>` + a + `</w:t></w:r></w:p></w:tc><w:tc><w:p><w:r><w:t>` + b + `</w:t></w:r></w:p></w:tc></w:tr>`
	}
	body := `<w:tbl>` + row("No", "Nama") + row("&lt;&lt;no&gt;&gt;", "&lt;&lt;nama anggota&gt;&gt;") + `</w:tbl><w:p><w:r><w:t>Selesai</w:t></w:r></w:p>`
	d := open(t, miniDocx(t, body))
	ok := d.RepeatRow("Nama Anggota", []map[string]string{
		{"no": "1", "nama anggota": "Budi"},
		{"no": "2", "nama anggota": "Siti"},
		{"no": "3", "nama anggota": "Andi"},
	})
	if !ok {
		t.Fatal("baris berpenanda tidak ditemukan")
	}
	got := roundTrip(t, d)
	rows := got.Tables()[0].Rows()
	if len(rows) != 4 {
		t.Fatalf("%d baris, want 4 (judul + 3 anggota)", len(rows))
	}
	for i, want := range []string{"1\nBudi", "2\nSiti", "3\nAndi"} {
		cells := rows[i+1].Cells()
		if got := cells[0].Text() + "\n" + cells[1].Text(); got != want {
			t.Errorf("baris %d = %q, want %q", i+1, got, want)
		}
	}
	if d.RepeatRow("tidak ada", nil) {
		t.Error("penanda yang tidak ada harus mengembalikan false")
	}
	// Tanpa item: baris asli dibuang.
	d2 := open(t, miniDocx(t, body))
	d2.RepeatRow("nama anggota", nil)
	if n := len(roundTrip(t, d2).Tables()[0].Rows()); n != 1 {
		t.Errorf("tanpa item harus tersisa 1 baris, dapat %d", n)
	}
}

func TestOpenRejectsGarbage(t *testing.T) {
	if _, err := Open([]byte("bukan zip")); err == nil {
		t.Error("harus menolak berkas yang bukan zip")
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create("readme.txt")
	w.Write([]byte("x"))
	zw.Close()
	if _, err := Open(buf.Bytes()); err == nil || !strings.Contains(err.Error(), "word/document.xml") {
		t.Errorf("zip tanpa document.xml: %v", err)
	}
	if _, err := Open(miniDocx(t, `<w:p><w:r><w:t>x`)); err == nil {
		t.Error("XML rusak harus ditolak")
	}
}

// RemoveTagParagraphs: butir tembusan yang hanya berisi penanda dibuang utuh (bukan dibiarkan kosong); penanda di tengah kalimat dan satu-satunya paragraf sel tidak dibuang.
func TestRemoveTagParagraphs(t *testing.T) {
	p := func(s string) string { return `<w:p><w:r><w:t>` + s + `</w:t></w:r></w:p>` }
	// penanda terpecah di beberapa run (seperti pada template asli) tetap dikenali
	terpecah := `<w:p><w:r><w:t>&lt;&lt;</w:t></w:r><w:r><w:t>Kepala Kantor Wilayah</w:t></w:r><w:r><w:t>&gt;&gt;</w:t></w:r></w:p>`
	body := p("Tembusan:") + p("Kepala Biro") + terpecah + p("Sesudah <<kepala kantor wilayah>> disampaikan") + p("Penutup")
	body = strings.ReplaceAll(body, "<<", "&lt;&lt;")
	body = strings.ReplaceAll(body, ">>", "&gt;&gt;")
	d := open(t, miniDocx(t, body))
	if n := d.RemoveTagParagraphs(" kepala  KANTOR wilayah "); n != 1 {
		t.Fatalf("dibuang = %d, want 1 (hanya paragraf yang isinya penanda saja)", n)
	}
	teks := roundTrip(t, d).Text()
	if strings.Contains(teks, "<<Kepala Kantor Wilayah>>") {
		t.Errorf("butir tembusan tidak dibuang: %q", teks)
	}
	if !strings.Contains(teks, "Sesudah <<kepala kantor wilayah>> disampaikan") {
		t.Errorf("penanda di tengah kalimat tidak boleh ikut dibuang: %q", teks)
	}
	if got := strings.Split(teks, "\n"); len(got) != 4 || got[1] != "Kepala Biro" || got[2] != "Sesudah <<kepala kantor wilayah>> disampaikan" {
		t.Errorf("susunan sesudah dibuang = %q", got)
	}
	// penanda lain tidak terpengaruh dan penanda yang tidak ada mengembalikan 0
	if n := open(t, miniDocx(t, body)).RemoveTagParagraphs("tidak ada"); n != 0 {
		t.Errorf("penanda yang tidak ada: %d", n)
	}

	// satu-satunya paragraf dalam sel dibiarkan; sel berparagraf banyak boleh dibuang salah satunya
	tabel := `<w:tbl><w:tr><w:tc>` + strings.ReplaceAll(strings.ReplaceAll(p("<<kanwil>>"), "<<", "&lt;&lt;"), ">>", "&gt;&gt;") + `</w:tc><w:tc>` +
		strings.ReplaceAll(strings.ReplaceAll(p("<<kanwil>>")+p("Lain"), "<<", "&lt;&lt;"), ">>", "&gt;&gt;") + `</w:tc></w:tr></w:tbl>`
	d2 := open(t, miniDocx(t, tabel))
	if n := d2.RemoveTagParagraphs("kanwil"); n != 1 {
		t.Errorf("sel: dibuang = %d, want 1 (paragraf tunggal dalam sel dibiarkan)", n)
	}
	sel := roundTrip(t, d2).Tables()[0].Rows()[0].Cells()
	if sel[0].Text() != "<<kanwil>>" || sel[1].Text() != "Lain" {
		t.Errorf("isi sel = %q dan %q", sel[0].Text(), sel[1].Text())
	}
}
