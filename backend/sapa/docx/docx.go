// Package docx mengisi template Word (.docx) berpenanda <<nama penanda>>.
//
// Template ditulis orang di Word, jadi sebuah penanda sering terpecah di beberapa "run" teks (mis. "<<jumlah ",
// "bmn", ">>" pada tiga run berbeda). Paket ini membaca teks per paragraf, mencocokkan penanda pada teks utuh,
// lalu menulis nilainya kembali ke run tempat penanda dimulai (gaya huruf run itu yang dipakai). Selain
// penanda, tersedia operasi tabel: menggandakan baris dan mengisi sel.
//
// Penanda gaya lain (mis. [@NomorND] milik aplikasi persuratan) sengaja tidak disentuh.
package docx

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"

	"github.com/beevik/etree"
)

const (
	maxEntries      = 2000
	maxEntrySize    = 40 << 20
	maxTotalSize    = 120 << 20
	documentPart    = "word/document.xml"
	wordNS          = "w"
	xmlSpaceAttrKey = "space"
)

// Penanda: <<teks>> (tanpa < atau > di dalamnya).
var tagPattern = regexp.MustCompile(`<<([^<>]{1,160})>>`)

var partPattern = regexp.MustCompile(`^word/(document|header\d*|footer\d*)\.xml$`)

// NormalizeTag: huruf kecil, spasi berlebih dibuang. "<< Jabatan  Pejabat >>" -> "jabatan pejabat".
func NormalizeTag(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(s), " "))
}

type entry struct {
	header zip.FileHeader
	data   []byte
}

// Doc adalah satu berkas .docx yang sedang diolah.
type Doc struct {
	names   []string
	entries map[string]*entry
	parts   map[string]*etree.Document // hanya bagian yang bisa memuat penanda
}

// Open membaca berkas .docx. Batas ukuran menjaga dari berkas zip yang dibuat untuk menghabiskan memori.
func Open(data []byte) (*Doc, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("bukan berkas .docx yang valid: %w", err)
	}
	if len(zr.File) == 0 || len(zr.File) > maxEntries {
		return nil, errors.New("isi berkas .docx tidak wajar")
	}
	d := &Doc{entries: map[string]*entry{}, parts: map[string]*etree.Document{}}
	var total uint64
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		total += f.UncompressedSize64
		if f.UncompressedSize64 > maxEntrySize || total > maxTotalSize {
			return nil, errors.New("berkas .docx terlalu besar setelah diekstrak")
		}
		rc, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("gagal membaca %s: %w", f.Name, err)
		}
		b, err := io.ReadAll(io.LimitReader(rc, maxEntrySize+1))
		rc.Close()
		if err != nil {
			return nil, fmt.Errorf("gagal membaca %s: %w", f.Name, err)
		}
		if len(b) > maxEntrySize {
			return nil, errors.New("berkas .docx terlalu besar setelah diekstrak")
		}
		d.names = append(d.names, f.Name)
		d.entries[f.Name] = &entry{header: f.FileHeader, data: b}
	}
	if _, ok := d.entries[documentPart]; !ok {
		return nil, errors.New("berkas .docx tidak memuat word/document.xml")
	}
	for _, name := range d.names {
		if !partPattern.MatchString(name) {
			continue
		}
		doc := etree.NewDocument()
		doc.ReadSettings.PreserveCData = true
		if err := doc.ReadFromBytes(d.entries[name].data); err != nil {
			return nil, fmt.Errorf("%s bukan XML yang valid: %w", name, err)
		}
		d.parts[name] = doc
	}
	return d, nil
}

// Bytes menulis kembali berkas .docx. Bagian yang tidak diubah disalin apa adanya.
func (d *Doc) Bytes() ([]byte, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, name := range d.names {
		e := d.entries[name]
		data := e.data
		if doc, ok := d.parts[name]; ok {
			doc.WriteSettings.CanonicalEndTags = false
			b, err := doc.WriteToBytes()
			if err != nil {
				return nil, fmt.Errorf("gagal menulis %s: %w", name, err)
			}
			data = b
		}
		h := &zip.FileHeader{Name: name, Method: zip.Deflate, Modified: e.header.Modified}
		if e.header.Method == zip.Store {
			h.Method = zip.Store
		}
		w, err := zw.CreateHeader(h)
		if err != nil {
			return nil, err
		}
		if _, err := w.Write(data); err != nil {
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// ---------------------------------------------------------------- paragraf dan segmen teks

// segment: satu potong teks yang bisa diubah (isi sebuah <w:t>), atau karakter tetap (tab, pindah baris).
type segment struct {
	el    *etree.Element // <w:t>; nil untuk karakter tetap
	run   *etree.Element
	text  string
	fixed bool
}

func isW(e *etree.Element, tag string) bool { return e.Space == wordNS && e.Tag == tag }

// runsOf mengumpulkan run milik paragraf ini saja (tidak masuk ke paragraf bersarang, mis. kotak teks).
func runsOf(p *etree.Element) []*etree.Element {
	var runs []*etree.Element
	var walk func(e *etree.Element)
	walk = func(e *etree.Element) {
		for _, c := range e.ChildElements() {
			switch {
			case isW(c, "p"):
				// paragraf bersarang diolah sendiri
			case isW(c, "r"):
				runs = append(runs, c)
			default:
				walk(c)
			}
		}
	}
	walk(p)
	return runs
}

func segmentsOf(p *etree.Element) []*segment {
	var segs []*segment
	for _, r := range runsOf(p) {
		for _, c := range r.ChildElements() {
			switch {
			case isW(c, "t"):
				segs = append(segs, &segment{el: c, run: r, text: c.Text()})
			case isW(c, "tab"):
				segs = append(segs, &segment{run: r, text: "\t", fixed: true})
			case isW(c, "br"), isW(c, "cr"):
				segs = append(segs, &segment{run: r, text: "\n", fixed: true})
			}
		}
	}
	return segs
}

func paragraphsIn(root *etree.Element) []*etree.Element {
	var out []*etree.Element
	var walk func(e *etree.Element)
	walk = func(e *etree.Element) {
		for _, c := range e.ChildElements() {
			if isW(c, "p") {
				out = append(out, c)
			}
			walk(c)
		}
	}
	walk(root)
	return out
}

func sanitize(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '\n' {
			return r
		}
		if r == '\r' {
			return -1
		}
		if r < 0x20 && r != '\t' {
			return -1
		}
		return r
	}, s)
}

// setSegText menulis teks ke <w:t>; baris baru ("\n") menjadi <w:br/> di antara beberapa <w:t>.
func setSegText(t *etree.Element, text string) {
	text = sanitize(text)
	// Hapus <w:br/> dan <w:t> tambahan hasil penulisan sebelumnya tidak diperlukan: elemen asli hanya diganti isinya.
	lines := strings.Split(text, "\n")
	t.SetText(lines[0])
	t.CreateAttr("xml:space", "preserve")
	parent := t.Parent()
	at := t.Index() + 1
	for _, line := range lines[1:] {
		br := etree.NewElement("w:br")
		nt := etree.NewElement("w:t")
		nt.SetText(line)
		nt.CreateAttr("xml:space", "preserve")
		parent.InsertChildAt(at, br)
		parent.InsertChildAt(at+1, nt)
		at += 2
	}
}

// ---------------------------------------------------------------- penanda

// Tags mengembalikan penanda yang ada di dokumen (ternormalisasi, unik, urut kemunculan).
func (d *Doc) Tags() []string {
	seen := map[string]bool{}
	var out []string
	for _, name := range d.partNames() {
		for _, p := range paragraphsIn(d.parts[name].Root()) {
			var sb strings.Builder
			for _, s := range segmentsOf(p) {
				sb.WriteString(s.text)
			}
			for _, m := range tagPattern.FindAllStringSubmatch(sb.String(), -1) {
				k := NormalizeTag(m[1])
				if !seen[k] {
					seen[k] = true
					out = append(out, k)
				}
			}
		}
	}
	return out
}

// Text mengembalikan teks isi dokumen (satu paragraf per baris), untuk pratinjau dan pengujian.
func (d *Doc) Text() string {
	var lines []string
	for _, p := range paragraphsIn(d.parts[documentPart].Root()) {
		var sb strings.Builder
		for _, s := range segmentsOf(p) {
			sb.WriteString(s.text)
		}
		lines = append(lines, sb.String())
	}
	return strings.Join(lines, "\n")
}

func (d *Doc) partNames() []string {
	var names []string
	for n := range d.parts {
		names = append(names, n)
	}
	sort.Slice(names, func(i, j int) bool {
		if names[i] == documentPart {
			return true
		}
		if names[j] == documentPart {
			return false
		}
		return names[i] < names[j]
	})
	return names
}

// Replace mengganti semua penanda yang punya nilai. Kunci values harus ternormalisasi (NormalizeTag).
// Penanda tanpa nilai dibiarkan apa adanya dan dikembalikan sebagai daftar (ternormalisasi, unik).
// Penanda bernilai kosong yang diapit "(" dan ")" menghapus tanda kurungnya juga.
func (d *Doc) Replace(values map[string]string) (missing []string) {
	seen := map[string]bool{}
	for _, name := range d.partNames() {
		for _, p := range paragraphsIn(d.parts[name].Root()) {
			for _, k := range replaceInParagraph(p, values) {
				if !seen[k] {
					seen[k] = true
					missing = append(missing, k)
				}
			}
		}
	}
	return missing
}

// RemoveTagParagraphs membuang paragraf yang isinya HANYA penanda tag (mis. satu butir daftar tembusan bernomor otomatis), sehingga butir itu tidak tersisa sebagai
// baris kosong dan penomoran otomatis menyesuaikan. Paragraf yang memuat penanda di tengah kalimat tidak disentuh (penandanya tetap diisi lewat Replace), dan satu-satunya
// paragraf dalam sel tabel dibiarkan (dokumen Word mewajibkan sel memuat sedikitnya satu paragraf). Mengembalikan jumlah paragraf yang dibuang.
func (d *Doc) RemoveTagParagraphs(tag string) int {
	tag = NormalizeTag(tag)
	n := 0
	for _, name := range d.partNames() {
		for _, p := range paragraphsIn(d.parts[name].Root()) {
			var sb strings.Builder
			for _, s := range segmentsOf(p) {
				sb.WriteString(s.text)
			}
			m := tagPattern.FindAllStringSubmatchIndex(sb.String(), -1)
			if len(m) != 1 || NormalizeTag(sb.String()[m[0][2]:m[0][3]]) != tag || strings.TrimSpace(tagPattern.ReplaceAllString(sb.String(), "")) != "" {
				continue
			}
			parent := p.Parent()
			if parent == nil {
				continue
			}
			if isW(parent, "tc") {
				paragraf := 0
				for _, c := range parent.ChildElements() {
					if isW(c, "p") {
						paragraf++
					}
				}
				if paragraf <= 1 {
					continue
				}
			}
			parent.RemoveChild(p)
			n++
		}
	}
	return n
}

func replaceInParagraph(p *etree.Element, values map[string]string) (missing []string) {
	segs := segmentsOf(p)
	if len(segs) == 0 {
		return nil
	}
	// Teks utuh (dalam rune) dan pemilik tiap rune.
	var runes []rune
	var owner []int
	for i, s := range segs {
		for _, r := range s.text {
			runes = append(runes, r)
			owner = append(owner, i)
		}
	}
	full := string(runes)
	locs := tagPattern.FindAllStringSubmatchIndex(full, -1)
	if len(locs) == 0 {
		return nil
	}
	// indeks byte -> indeks rune
	byteToRune := make([]int, len(full)+1)
	ri := 0
	for bi := range full {
		byteToRune[bi] = ri
		ri++
	}
	byteToRune[len(full)] = ri

	type repl struct {
		s, e int
		text string
	}
	var repls []repl
	skip := make([]bool, len(runes))
	for _, loc := range locs {
		key := NormalizeTag(full[loc[2]:loc[3]])
		val, ok := values[key]
		if !ok {
			missing = append(missing, key)
			continue
		}
		s, e := byteToRune[loc[0]], byteToRune[loc[1]]
		if strings.TrimSpace(val) == "" {
			val = ""
			if s > 0 && e < len(runes) && runes[s-1] == '(' && runes[e] == ')' {
				skip[s-1], skip[e] = true, true
				if s > 1 && runes[s-2] == ' ' {
					skip[s-2] = true
				}
			}
		}
		repls = append(repls, repl{s, e, val})
	}
	if len(repls) == 0 {
		return missing
	}

	out := make([]strings.Builder, len(segs))
	for i := 0; i < len(runes); {
		if len(repls) > 0 && repls[0].s == i {
			out[owner[i]].WriteString(repls[0].text)
			i = repls[0].e
			repls = repls[1:]
			continue
		}
		if !skip[i] {
			out[owner[i]].WriteRune(runes[i])
		}
		i++
	}
	for i, s := range segs {
		if s.fixed {
			continue
		}
		if got := out[i].String(); got != s.text {
			setSegText(s.el, got)
		}
	}
	return missing
}

// ---------------------------------------------------------------- tabel

// Table adalah satu tabel Word (<w:tbl>).
type Table struct{ el *etree.Element }

// Row adalah satu baris (<w:tr>).
type Row struct{ el *etree.Element }

// Cell adalah satu sel (<w:tc>).
type Cell struct{ el *etree.Element }

func textOfElement(e *etree.Element) string {
	var sb strings.Builder
	var walk func(x *etree.Element)
	walk = func(x *etree.Element) {
		for _, c := range x.ChildElements() {
			switch {
			case isW(c, "t"):
				sb.WriteString(c.Text())
			case isW(c, "tab"):
				sb.WriteString("\t")
			case isW(c, "br"), isW(c, "cr"):
				sb.WriteString("\n")
			case isW(c, "p") && sb.Len() > 0:
				sb.WriteString("\n")
				walk(c)
			default:
				walk(c)
			}
		}
	}
	walk(e)
	return sb.String()
}

// Tables mengembalikan seluruh tabel tingkat atas pada isi dokumen.
func (d *Doc) Tables() []*Table {
	var out []*Table
	body := d.parts[documentPart].Root().SelectElement("w:body")
	if body == nil {
		return nil
	}
	for _, c := range body.ChildElements() {
		if isW(c, "tbl") {
			out = append(out, &Table{c})
		}
	}
	return out
}

// FindTable mencari tabel pertama yang teksnya memuat contains (tanpa membedakan huruf besar/kecil).
func (d *Doc) FindTable(contains string) *Table {
	want := strings.ToLower(contains)
	for _, t := range d.Tables() {
		if strings.Contains(strings.ToLower(t.Text()), want) {
			return t
		}
	}
	return nil
}

func (t *Table) Text() string { return textOfElement(t.el) }

func (t *Table) Rows() []*Row {
	var out []*Row
	for _, c := range t.el.ChildElements() {
		if isW(c, "tr") {
			out = append(out, &Row{c})
		}
	}
	return out
}

func (r *Row) Text() string { return textOfElement(r.el) }

func (r *Row) Cells() []*Cell {
	var out []*Cell
	for _, c := range r.el.ChildElements() {
		if isW(c, "tc") {
			out = append(out, &Cell{c})
		}
	}
	return out
}

// Remove membuang baris dari tabel.
func (r *Row) Remove() { r.el.Parent().RemoveChild(r.el) }

// stripIDs membuang atribut id paragraf (w14:paraId/textId) pada salinan, supaya tidak ada id ganda.
func stripIDs(e *etree.Element) {
	var attrs []string
	for _, a := range e.Attr {
		if a.Key == "paraId" || a.Key == "textId" {
			attrs = append(attrs, a.Space+":"+a.Key)
		}
	}
	for _, a := range attrs {
		e.RemoveAttr(a)
	}
	for _, c := range e.ChildElements() {
		stripIDs(c)
	}
}

// CloneAfter menyalin baris (beserta format selnya) dan menyisipkannya tepat setelah baris ini.
func (r *Row) CloneAfter() *Row {
	cp := r.el.Copy()
	stripIDs(cp)
	parent := r.el.Parent()
	parent.InsertChildAt(r.el.Index()+1, cp)
	return &Row{cp}
}

// Replace mengganti penanda pada baris ini saja (kunci ternormalisasi); penanda lain dibiarkan.
func (r *Row) Replace(values map[string]string) {
	for _, p := range paragraphsIn(r.el) {
		replaceInParagraph(p, values)
	}
}

func (r *Row) hasTag(tag string) bool {
	for _, p := range paragraphsIn(r.el) {
		var sb strings.Builder
		for _, s := range segmentsOf(p) {
			sb.WriteString(s.text)
		}
		for _, m := range tagPattern.FindAllStringSubmatch(sb.String(), -1) {
			if NormalizeTag(m[1]) == tag {
				return true
			}
		}
	}
	return false
}

// RepeatRow mencari baris tabel yang memuat penanda tag, menggandakannya untuk setiap item (nilai penanda pada baris
// diambil dari item), lalu membuang baris asli. Mengembalikan false bila tidak ada baris yang memuat penanda itu.
// Bila items kosong, baris asli dibuang saja.
func (d *Doc) RepeatRow(tag string, items []map[string]string) bool {
	tag = NormalizeTag(tag)
	body := d.parts[documentPart].Root().SelectElement("w:body")
	if body == nil {
		return false
	}
	var found *Row
	var walk func(e *etree.Element)
	walk = func(e *etree.Element) {
		for _, c := range e.ChildElements() {
			if found != nil {
				return
			}
			if isW(c, "tr") {
				if r := (&Row{c}); r.hasTag(tag) {
					found = r
					return
				}
			}
			walk(c)
		}
	}
	walk(body)
	if found == nil {
		return false
	}
	// Setiap salinan dibuat dari baris asli (yang penandanya masih utuh), disisipkan setelah salinan sebelumnya.
	parent := found.el.Parent()
	last := found.el
	for _, it := range items {
		cp := found.el.Copy()
		stripIDs(cp)
		parent.InsertChildAt(last.Index()+1, cp)
		(&Row{cp}).Replace(it)
		last = cp
	}
	found.Remove()
	return true
}

func (c *Cell) Text() string { return textOfElement(c.el) }

// SetText mengganti isi sel dengan teks (baris baru menjadi pindah baris). Gaya huruf diambil dari teks
// pertama di sel; bila sel kosong, dari properti run paragrafnya.
func (c *Cell) SetText(text string) {
	var p *etree.Element
	for _, ch := range c.el.ChildElements() {
		if isW(ch, "p") {
			p = ch
			break
		}
	}
	if p == nil {
		p = etree.NewElement("w:p")
		c.el.AddChild(p)
	}
	segs := segmentsOf(p)
	var first *segment
	for _, s := range segs {
		if !s.fixed {
			first = s
			break
		}
	}
	if first != nil {
		for _, s := range segs {
			if !s.fixed && s != first {
				setSegText(s.el, "")
			}
		}
		setSegText(first.el, text)
		return
	}
	run := etree.NewElement("w:r")
	if ppr := p.SelectElement("w:pPr"); ppr != nil {
		if rpr := ppr.SelectElement("w:rPr"); rpr != nil {
			run.AddChild(rpr.Copy())
		}
	}
	t := etree.NewElement("w:t")
	run.AddChild(t)
	p.AddChild(run)
	setSegText(t, text)
}
