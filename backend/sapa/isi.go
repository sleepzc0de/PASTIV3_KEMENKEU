package sapa

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"pasti-v3-backend/sapa/docx"
)

// Hasil adalah dokumen Word yang sudah diisi.
type Hasil struct {
	Berkas     []byte
	NamaFile   string
	Peringatan []string // hal yang perlu dicek, mis. penanda pada template yang tidak punya nilai
}

// ErrValidasi dikembalikan bila isian formulir belum lengkap atau tidak valid.
type ErrValidasi struct{ Rincian []string }

func (e *ErrValidasi) Error() string { return strings.Join(e.Rincian, "; ") }

func cekValidasi(rincian []string) error {
	if len(rincian) > 0 {
		return &ErrValidasi{Rincian: rincian}
	}
	return nil
}

func namaFile(awal string, noreg string) string {
	bersih := strings.Map(func(r rune) rune {
		if strings.ContainsRune(`\/:*?"<>|`, r) {
			return '-'
		}
		return r
	}, awal+" "+noreg)
	return strings.TrimSpace(bersih) + ".docx"
}

// peringatanDari mengubah daftar penanda tanpa nilai menjadi pesan untuk pengguna.
func peringatanDari(missing []string) []string {
	var out []string
	for _, m := range missing {
		out = append(out, fmt.Sprintf("Penanda <<%s>> pada template tidak punya nilai dan dibiarkan apa adanya", m))
	}
	return out
}

func selesaikan(doc *docx.Doc, nama string, peringatan []string) (*Hasil, error) {
	b, err := doc.Bytes()
	if err != nil {
		return nil, fmt.Errorf("gagal menulis dokumen: %w", err)
	}
	return &Hasil{Berkas: b, NamaFile: nama, Peringatan: peringatan}, nil
}

func bukaTemplate(tpl []byte) (*docx.Doc, error) {
	if len(tpl) == 0 {
		return nil, ErrTemplateTidakAda
	}
	doc, err := docx.Open(tpl)
	if err != nil {
		return nil, fmt.Errorf("template tidak dapat dibuka: %w", err)
	}
	return doc, nil
}

// ---------------------------------------------------------------- tabel Nota Dinas

func isNumberingRow(cells []*docx.Cell) bool {
	if len(cells) < 3 {
		return false
	}
	for i, c := range cells {
		if strings.TrimSpace(c.Text()) != strconv.Itoa(i+1) {
			return false
		}
	}
	return true
}

// isiDaftarBarang mengisi tabel "Daftar Barang" Lampiran Nota Dinas: baris contoh digandakan sebanyak barang, dan baris
// JUMLAH diisi total nilai. Struktur yang diharapkan: baris judul, baris nomor kolom (1..10), satu atau lebih baris
// kosong, baris JUMLAH.
func isiDaftarBarang(doc *docx.Doc, barang []Barang, perolehan, limit int64) error {
	tbl := doc.FindTable("Nama Barang")
	if tbl == nil {
		return errors.New("tabel Daftar Barang (berjudul \"Nama Barang\") tidak ditemukan pada template")
	}
	rows := tbl.Rows()
	jum := -1
	for i, r := range rows {
		if strings.Contains(strings.ToUpper(r.Text()), "JUMLAH") {
			jum = i
		}
	}
	if jum < 0 {
		return errors.New("baris JUMLAH pada tabel Daftar Barang tidak ditemukan")
	}
	first := 1
	for i := 1; i < jum; i++ {
		if isNumberingRow(rows[i].Cells()) {
			first = i + 1
			break
		}
	}
	if first >= jum {
		return errors.New("baris isi pada tabel Daftar Barang tidak ditemukan (harus ada baris kosong sebelum baris JUMLAH)")
	}
	tpl := rows[first]
	if len(tpl.Cells()) < 10 {
		return fmt.Errorf("baris isi tabel Daftar Barang hanya punya %d kolom (harus 10)", len(tpl.Cells()))
	}
	for _, r := range rows[first+1 : jum] { // baris kosong tambahan di template dibuang
		r.Remove()
	}
	cur := tpl
	for i := 1; i < len(barang); i++ {
		cur = cur.CloneAfter()
	}
	all := tbl.Rows()
	for i, b := range barang {
		p, _ := ParseUang(b.NilaiPerolehan)
		l, _ := ParseUang(b.NilaiLimit)
		vals := []string{strconv.Itoa(i + 1), b.Nama, b.Kode, b.NUP, b.Lokasi, b.Kondisi, b.TahunPerolehan, Angka(p), Angka(l), b.Keterangan}
		cells := all[first+i].Cells()
		for j, v := range vals {
			cells[j].SetText(v)
		}
	}
	total := all[len(all)-1].Cells()
	if len(total) >= 3 {
		total[1].SetText(Angka(perolehan))
		total[2].SetText(Angka(limit))
	}
	return nil
}

// Baris checklist dikenali dari kata kunci pada nama dokumen di barisnya, bukan dari nomor urut, supaya template yang
// urutannya diubah tetap terisi benar. Baris tanpa kunci diisi lewat penanda biasa (<<alasan>>, <<tiket siman>>).
var barisChecklist = []struct{ cocok, kunci string }{
	{"daftar lampiran", DokDaftarBarang},
	{"pembentukan tim", DokSKTimInternal},
	{"berita acara penelitian", DokBeritaAcara},
	{"penetapan status penggunaan", DokPSP},
	{"rp4", DokRP4},
	{"penghentian penggunaan", DokSKPP},
	{"print out", DokDaftarDihentikan},
	{"kebenaran formil", DokSPTJFormil},
	{"kartu identitas barang", DokKIB},
	{"laporan kondisi", DokLaporanKondisi},
	{"dokumen kepemilikan", DokBuktiMilik},
	{"foto terkini", DokFoto},
	{"nilai limit", DokSPTJLimit},
	{"tidak mengganggu", DokSPerTusi},
	{"dasar pengajuan anggaran", DokSPerAnggaran},
	{"dinas terkait", DokDinasTeknis},
	{"laporan penilaian", DokLaporanPenilaian},
}

func hasilChecklist(d DokPendukung) string {
	if !d.Ada {
		return "Tidak ada"
	}
	s := "Ada"
	if d.Nomor != "" {
		s += ", Nomor " + d.Nomor
	}
	if t, _ := ParseTanggal(d.Tanggal); !t.IsZero() {
		s += ", tanggal " + TanggalPanjang(t)
	}
	return s
}

func isiChecklist(doc *docx.Doc, d DataNDSatker) (dikenali int, ada bool) {
	tbl := doc.FindTable("Jenis Data/Dokumen")
	if tbl == nil {
		return 0, false
	}
	for _, row := range tbl.Rows()[1:] {
		cells := row.Cells()
		if len(cells) < 3 {
			continue
		}
		label := strings.ToLower(cells[1].Text())
		for _, def := range barisChecklist {
			if strings.Contains(label, def.cocok) {
				cells[2].SetText(hasilChecklist(d.Dok(def.kunci)))
				dikenali++
				break
			}
		}
	}
	return dikenali, true
}

// ---------------------------------------------------------------- penanda Nota Dinas

func ndTags(k Kasus, d DataNDSatker, jumlah int, perolehan, limit int64) map[string]string {
	ba := d.Dok(DokBeritaAcara)
	nomorBA, tanggalBA := ba.Nomor, ""
	if t, _ := ParseTanggal(ba.Tanggal); !t.IsZero() {
		tanggalBA = TanggalPanjang(t)
	}
	if nomorBA == "" {
		nomorBA = "-"
	}
	if tanggalBA == "" {
		tanggalBA = "-"
	}
	tp, tl := RupiahTerbilang(perolehan), RupiahTerbilang(limit)
	return map[string]string{
		"sekretaris ue1":                  d.TujuanSurat,
		"nama satker":                     k.NamaSatker,
		"nama satker singkat":             d.SingkatanSatker,
		"kota":                            d.Kota,
		"jenis bmn":                       d.JenisBMN,
		"jumlah bmn":                      strconv.Itoa(jumlah),
		"terbilang jumlah bmn":            JumlahTerbilang(int64(jumlah), d.Satuan),
		"total nilai perolehan":           Rupiah(perolehan),
		"terbilang nilai perolehan":       tp,
		"terbilang total nilai perolehan": tp,
		"total nilai limit":               Rupiah(limit),
		"terbilang nilai limit":           tl,
		"terbilang total nilai limit":     tl,
		"alasan":                          d.Alasan,
		"tiket siman":                     d.TiketSiman,
		"nomor tiket":                     d.TiketSiman,
		"nomor ba":                        nomorBA,
		"tanggal ba":                      tanggalBA,
		"kepala kantor wilayah":           d.KepalaKanwil,
		"nama pejabat penandatangan":      d.Penandatangan.Nama,
		"nip pejabat penandatangan":       d.Penandatangan.NIP,
		"jabatan pejabat penandatangan":   d.Penandatangan.Jabatan,
		"noreg aplikasi":                  k.Noreg,
	}
}

func isiTabelND(doc *docx.Doc, d DataNDSatker, jumlah int, perolehan, limit int64) ([]string, error) {
	var peringatan []string
	if err := isiDaftarBarang(doc, d.Barang, perolehan, limit); err != nil {
		return nil, err
	}
	n, ada := isiChecklist(doc, d)
	if !ada {
		peringatan = append(peringatan, "Tabel checklist (berjudul \"Jenis Data/Dokumen\") tidak ditemukan pada template")
	} else if n < len(barisChecklist) {
		peringatan = append(peringatan, fmt.Sprintf("Hanya %d dari %d baris checklist yang dikenali pada template", n, len(barisChecklist)))
	}
	return peringatan, nil
}

// IsiNDSatker mengisi template Nota Dinas Usulan Penjualan Satker (satu berkas: ND, lampiran, checklist, surat-surat).
func IsiNDSatker(tpl []byte, k Kasus, d DataNDSatker) (*Hasil, error) {
	d.Rapikan()
	if err := cekValidasi(d.Validasi()); err != nil {
		return nil, err
	}
	jumlah, perolehan, limit, err := d.Total()
	if err != nil {
		return nil, &ErrValidasi{Rincian: []string{err.Error()}}
	}
	doc, err := bukaTemplate(tpl)
	if err != nil {
		return nil, err
	}
	peringatan, err := isiTabelND(doc, d, jumlah, perolehan, limit)
	if err != nil {
		return nil, err
	}
	peringatan = append(peringatan, peringatanDari(doc.Replace(ndTags(k, d, jumlah, perolehan, limit)))...)
	return selesaikan(doc, namaFile("ND Usulan Penjualan Satker", k.Noreg), peringatan)
}

// IsiNDUE1 mengisi template Nota Dinas Usulan Penjualan UE1 dari data usulan Satker dan isian UE1.
func IsiNDUE1(tpl []byte, k Kasus, satker DataNDSatker, ue DataNDUE1) (*Hasil, error) {
	satker.Rapikan()
	ue.Rapikan()
	if err := cekValidasi(satker.Validasi()); err != nil {
		return nil, fmt.Errorf("data usulan Satker belum lengkap: %w", err)
	}
	if err := cekValidasi(ue.Validasi()); err != nil {
		return nil, err
	}
	jumlah, perolehan, limit, err := satker.Total()
	if err != nil {
		return nil, &ErrValidasi{Rincian: []string{err.Error()}}
	}
	doc, err := bukaTemplate(tpl)
	if err != nil {
		return nil, err
	}
	peringatan, err := isiTabelND(doc, satker, jumlah, perolehan, limit)
	if err != nil {
		return nil, err
	}
	tags := ndTags(k, satker, jumlah, perolehan, limit)
	tags["sekretaris ue1"] = ue.SekretarisUE1
	tags["kepala kantor wilayah"] = ue.KepalaKanwil
	tags["pejabat pengelola"] = ue.PejabatPengelola
	tags["nama pejabat ue1 penandatangan"] = ue.Penandatangan.Nama
	tags["jabatan pejabat ue1 penandatangan"] = ue.Penandatangan.Jabatan
	// Pada Nota Dinas UE1 penandatangannya adalah pejabat UE1, bukan kepala satker: penanda umum ikut menunjuk pejabat UE1.
	tags["nama pejabat penandatangan"] = ue.Penandatangan.Nama
	tags["jabatan pejabat penandatangan"] = ue.Penandatangan.Jabatan
	tags["nip pejabat penandatangan"] = ""
	tgl := ""
	if t, _ := ParseTanggal(ue.TanggalND); !t.IsZero() {
		tgl = TanggalPanjang(t)
	}
	tags["nomor nd usulan satker"] = ue.NomorND
	tags["tanggal nd usulan"] = tgl
	tags["tanggal usulan satker"] = tgl
	tags["hal nd usulan"] = ue.HalND
	tags["hal usulan satker"] = ue.HalND
	peringatan = append(peringatan, peringatanDari(doc.Replace(tags))...)
	return selesaikan(doc, namaFile("ND Usulan Penjualan UE1", k.Noreg), peringatan)
}

// ---------------------------------------------------------------- SK Tim dan Berita Acara

// IsiSKTim mengisi template SK Tim. Anggota dicetak dengan menggandakan baris tabel yang memuat <<nama anggota>>.
func IsiSKTim(tpl []byte, k Kasus, d DataTim) (*Hasil, error) {
	d.Rapikan()
	if err := cekValidasi(d.Validasi()); err != nil {
		return nil, err
	}
	doc, err := bukaTemplate(tpl)
	if err != nil {
		return nil, err
	}
	var peringatan []string
	items := make([]map[string]string, len(d.Anggota))
	var daftar []string
	for i, a := range d.Anggota {
		items[i] = map[string]string{
			"no": strconv.Itoa(i + 1), "nama anggota": a.Nama, "jabatan anggota": a.Jabatan, "kedudukan": a.Kedudukan, "nip anggota": a.NIP,
		}
		daftar = append(daftar, fmt.Sprintf("%d. %s – %s – %s", i+1, a.Nama, a.Jabatan, a.Kedudukan))
	}
	if !doc.RepeatRow("nama anggota", items) {
		// Tanpa tabel anggota, penanda <<daftar anggota>> (bila ada) memuat semua anggota sebagai teks.
		if !contains(doc.Tags(), "daftar anggota") {
			peringatan = append(peringatan, "Template tidak memuat baris tabel dengan <<nama anggota>> maupun <<daftar anggota>>; daftar anggota tidak tercetak")
		}
	}
	awal, _ := ParseTanggal(d.MasaAwal)
	akhir, _ := ParseTanggal(d.MasaAkhir)
	peringatan = append(peringatan, peringatanDari(doc.Replace(map[string]string{
		"nama satker":      k.NamaSatker,
		"nomor tiket":      k.Noreg,
		"noreg aplikasi":   k.Noreg,
		"kota":             d.Kota,
		"jabatan pimpinan": d.JabatanPimpinan,
		"jenis tim":        d.JenisTim,
		"masa awal tugas":  TanggalPanjang(awal),
		"masa akhir tugas": TanggalPanjang(akhir),
		"jumlah anggota":   strconv.Itoa(len(d.Anggota)),
		"daftar anggota":   strings.Join(daftar, "\n"),
	}))...)
	return selesaikan(doc, namaFile("SK Tim", k.Noreg), peringatan)
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// IsiBA mengisi template Berita Acara Penelitian.
func IsiBA(tpl []byte, k Kasus, d DataBA) (*Hasil, error) {
	d.Rapikan()
	if err := cekValidasi(d.Validasi()); err != nil {
		return nil, err
	}
	doc, err := bukaTemplate(tpl)
	if err != nil {
		return nil, err
	}
	t, _ := ParseTanggal(d.TanggalPenelitian)
	peringatan := peringatanDari(doc.Replace(map[string]string{
		"nama satker":                k.NamaSatker,
		"nomor tiket":                k.Noreg,
		"noreg aplikasi":             k.Noreg,
		"bentuk pemindahtanganan":    d.Bentuk,
		"hari penelitian":            NamaHari(t),
		"tanggal penelitian":         strconv.Itoa(t.Day()),
		"bulan penelitian":           NamaBulan(t),
		"tahun penelitian":           strconv.Itoa(t.Year()),
		"tanggal penelitian lengkap": TanggalPanjang(t),
		"nama tim":                   d.NamaTim,
	}))
	return selesaikan(doc, namaFile("Berita Acara Penelitian", k.Noreg), peringatan)
}
