package analitik

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Ambang yang dipakai aturan wawasan. Dikumpulkan di satu tempat supaya mudah ditinjau dan diubah.
const (
	ambangSatkerDominan      = 35.0 // % pagu RUP pada satu satker: perhatian
	ambangSatkerBesar        = 20.0 // % pagu RUP pada satu satker: info
	ambangTriwulanIV         = 40.0 // % pagu rencana pemilihan di triwulan IV: perhatian
	ambangBulanPuncak        = 20.0 // % pagu rencana pemilihan pada satu bulan: disebut
	ambangBelumDiumumkan     = 15.0 // % pagu RUP belum terumumkan: perhatian
	ambangCorongPenting      = 50.0 // % pagu RUP belum diproses (setelah Juni): penting
	ambangCorongPerhatian    = 30.0 // idem: perhatian
	ambangCorongBaik         = 20.0 // di bawah ini: baik
	ambangEfisiensiTipis     = 2.0  // % efisiensi: di bawahnya perhatian
	ambangEfisiensiSehat     = 8.0  // % efisiensi: di atasnya baik
	ambangSatuPesertaPenting = 50.0 // % tender hanya satu peserta: penting
	ambangSatuPeserta        = 30.0 // idem: perhatian
	ambangSatuPesertaBaik    = 15.0 // di bawah ini: baik
	ambangRataPeserta        = 3.0  // rata-rata peserta per tender di bawah ini: perhatian
	ambangHHIPenting         = 2500.0
	ambangHHIPerhatian       = 1500.0
	ambangPenyediaTeratas    = 25.0 // % nilai kontrak pada satu penyedia: perhatian
	ambangAddendumPerhatian  = 15.0 // % kontrak beradendum: perhatian
	ambangAddendumPenting    = 30.0 // idem: penting
	ambangWaktuProsesHari    = 60.0 // median hari pengumuman-penetapan pemenang: perhatian
	ambangTenderGagal        = 10.0 // % tender gagal/batal/diulang: perhatian
	ambangEkatalogBesar      = 30.0 // % nilai transaksi lewat e-katalog: info
	ambangKomoditasDominan   = 40.0 // % nilai e-katalog V5 pada satu komoditas: perhatian
	ambangSwasta             = 20.0 // % nilai e-katalog V6 pada paket swasta: info
	ambangPerubahanTahunan   = 10.0 // % perubahan pagu RUP antar tahun yang disebut
	ambangDataLama           = 7 * 24 * time.Hour
	minSampelEfisiensi       = 5
)

var namaBulan = [...]string{"", "Januari", "Februari", "Maret", "April", "Mei", "Juni", "Juli", "Agustus", "September", "Oktober", "November", "Desember"}

// ---- format angka dalam kalimat ----

func desimalID(f float64, maks int) string {
	s := strconv.FormatFloat(f, 'f', maks, 64)
	if strings.Contains(s, ".") {
		s = strings.TrimRight(strings.TrimRight(s, "0"), ".")
	}
	return strings.Replace(s, ".", ",", 1)
}

// Bulat menulis bilangan bulat dengan pemisah ribuan titik.
func Bulat(n int64) string {
	neg := n < 0
	if neg {
		n = -n
	}
	s := strconv.FormatInt(n, 10)
	var b strings.Builder
	for i, r := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte('.')
		}
		b.WriteRune(r)
	}
	if neg {
		return "-" + b.String()
	}
	return b.String()
}

// Rupiah menulis nilai rupiah ringkas: "Rp 1,25 triliun", "Rp 3,4 miliar", "Rp 12,5 juta", atau "Rp 950.000".
func Rupiah(f float64) string {
	neg := f < 0
	if neg {
		f = -f
	}
	var s string
	switch {
	case f >= 1e12:
		s = "Rp " + desimalID(f/1e12, 2) + " triliun"
	case f >= 1e9:
		s = "Rp " + desimalID(f/1e9, 2) + " miliar"
	case f >= 1e6:
		s = "Rp " + desimalID(f/1e6, 1) + " juta"
	default:
		s = "Rp " + Bulat(int64(math.Round(f)))
	}
	if neg {
		return "-" + s
	}
	return s
}

// Persen menulis persentase dengan satu desimal (koma desimal): "12,3%".
func Persen(f float64) string { return desimalID(f, 1) + "%" }

func porsi(bagian, total float64) float64 {
	if total <= 0 {
		return 0
	}
	return bagian / total * 100
}

func jumlahNilai(p []Pasangan) (n int64, nilai float64) {
	for _, x := range p {
		n += x.Jumlah
		nilai += x.Nilai
	}
	return
}

// Susun menghasilkan wawasan dari hasil dasbor, diurutkan dari yang paling mendesak (urutan penulisan aturan dipertahankan di tiap tingkat).
func Susun(h *Hasil) []Wawasan {
	if h == nil {
		return nil
	}
	if h.Sekarang.IsZero() {
		h.Sekarang = time.Now()
	}
	aturan := []func(*Hasil) []Wawasan{
		wawasanKelengkapan, wawasanRingkasan, wawasanPembanding, wawasanCorong,
		wawasanMetodeRUP, wawasanSatker, wawasanTriwulan, wawasanUmumkan,
		wawasanStatusTender, wawasanEfisiensi, wawasanPersaingan, wawasanPasar, wawasanWaktuProses,
		wawasanAddendum, wawasanKontrakBerakhir, wawasanEkatalog,
	}
	var semua []Wawasan
	for _, f := range aturan {
		semua = append(semua, f(h)...)
	}
	urut := map[string]int{TingkatPenting: 0, TingkatPerhatian: 1, TingkatInfo: 2, TingkatBaik: 3}
	sort.SliceStable(semua, func(i, j int) bool { return urut[semua[i].Tingkat] < urut[semua[j].Tingkat] })
	return semua
}

// ---- aturan ----

func wawasanKelengkapan(h *Hasil) []Wawasan {
	var out []Wawasan
	if len(h.DatasetKosong) > 0 {
		daftar := h.DatasetKosong
		sisa := ""
		if len(daftar) > 6 {
			sisa = fmt.Sprintf(" dan %d dataset lain", len(daftar)-6)
			daftar = daftar[:6]
		}
		out = append(out, Wawasan{BagianData, TingkatPerhatian, "Sebagian data belum ditarik",
			fmt.Sprintf("Belum ada data %s%s untuk %s tahun %s. Angka dan grafik yang bergantung pada dataset itu tampil kosong atau lebih rendah dari kenyataan. Tarik datanya di halaman Penarikan Data.",
				strings.Join(daftar, ", "), sisa, h.KodeKLPD, h.Tahun)})
	}
	if h.TerakhirTarik != nil && !h.Sekarang.IsZero() && h.Sekarang.Sub(*h.TerakhirTarik) > ambangDataLama {
		hari := int(h.Sekarang.Sub(*h.TerakhirTarik).Hours() / 24)
		out = append(out, Wawasan{BagianData, TingkatPerhatian, "Data sudah lama tidak diperbarui",
			fmt.Sprintf("Penarikan data yang berhasil terakhir terjadi %d hari lalu. Periksa jadwal penarikan otomatis atau jalankan penarikan manual agar angka mengikuti kondisi terbaru.", hari)})
	}
	bagian := make([]string, 0, len(h.Galat))
	for b := range h.Galat {
		bagian = append(bagian, b)
	}
	sort.Strings(bagian) // urutan peta acak; hasil harus tetap
	for _, b := range bagian {
		out = append(out, Wawasan{BagianData, TingkatPerhatian, "Satu bagian dasbor gagal dihitung",
			fmt.Sprintf("Bagian %q tidak dapat dibaca. Bagian lain tetap ditampilkan.", b)})
	}
	return out
}

func wawasanRingkasan(h *Hasil) []Wawasan {
	r := h.RUP
	if r == nil || r.TotalPaket == 0 && r.PaketSwakelola == 0 {
		return nil
	}
	isi := fmt.Sprintf("%s paket penyedia direncanakan dengan total pagu %s", Bulat(r.TotalPaket), Rupiah(r.TotalPagu))
	if r.PaketSwakelola > 0 {
		isi += fmt.Sprintf("; ditambah %s paket swakelola", Bulat(r.PaketSwakelola))
		if r.PaguSwakelola > 0 {
			isi += fmt.Sprintf(" (pagu terumumkan %s)", Rupiah(r.PaguSwakelola))
		}
	}
	isi += "."
	if r.PaguProgram > 0 && r.TotalPagu > 0 {
		isi += fmt.Sprintf(" Pagu paket penyedia setara %s dari pagu program (%s).", Persen(porsi(r.TotalPagu, r.PaguProgram)), Rupiah(r.PaguProgram))
	}
	return []Wawasan{{BagianRingkasan, TingkatInfo, fmt.Sprintf("Perencanaan pengadaan %s", h.Tahun), isi}}
}

func wawasanPembanding(h *Hasil) []Wawasan {
	p, r := h.Pembanding, h.RUP
	if p == nil || r == nil || p.RUPPagu <= 0 || r.TotalPagu <= 0 {
		return nil
	}
	var out []Wawasan
	ubah := (r.TotalPagu - p.RUPPagu) / p.RUPPagu * 100
	if math.Abs(ubah) >= ambangPerubahanTahunan {
		arah := "naik"
		if ubah < 0 {
			arah = "turun"
		}
		out = append(out, Wawasan{BagianRingkasan, TingkatInfo, fmt.Sprintf("Pagu RUP %s dibanding %s", arah, p.Tahun),
			fmt.Sprintf("Pagu paket penyedia %s %s: dari %s (%s paket) menjadi %s (%s paket).",
				arah, Persen(math.Abs(ubah)), Rupiah(p.RUPPagu), Bulat(p.RUPPaket), Rupiah(r.TotalPagu), Bulat(r.TotalPaket))})
	}
	if pm := h.Pemilihan; pm != nil && p.TenderJumlah > 0 && pm.TenderJumlah > 0 {
		sel := float64(pm.TenderJumlah-p.TenderJumlah) / float64(p.TenderJumlah) * 100
		if math.Abs(sel) >= ambangPerubahanTahunan {
			arah := "lebih banyak"
			if sel < 0 {
				arah = "lebih sedikit"
			}
			out = append(out, Wawasan{BagianPemilihan, TingkatInfo, "Jumlah tender berubah dari tahun lalu",
				fmt.Sprintf("Ada %s tender tahun %s, %s (%s) dibanding %s tender pada %s.", Bulat(pm.TenderJumlah), h.Tahun, arah, Persen(math.Abs(sel)), Bulat(p.TenderJumlah), p.Tahun)})
		}
	}
	return out
}

func wawasanCorong(h *Hasil) []Wawasan {
	c := h.Corong
	if c == nil || c.TotalPaket == 0 {
		return nil
	}
	var belum, diproses Pasangan
	for _, t := range c.Tahap {
		if t.Label == "Belum diproses" {
			belum = t
		} else {
			diproses.Jumlah += t.Jumlah
			diproses.Nilai += t.Nilai
		}
	}
	pb := porsi(belum.Nilai, c.TotalPagu)
	isi := fmt.Sprintf("%s dari %s paket RUP (pagu %s dari %s, %s) belum ditemukan di tender, non-tender, maupun e-purchasing berdasarkan kode RUP; yang sudah diproses %s paket (%s).",
		Bulat(belum.Jumlah), Bulat(c.TotalPaket), Rupiah(belum.Nilai), Rupiah(c.TotalPagu), Persen(pb), Bulat(diproses.Jumlah), Rupiah(diproses.Nilai))
	bln := int(h.Sekarang.Month())
	tahunIni := strconv.Itoa(h.Sekarang.Year()) == h.Tahun
	switch {
	case pb >= ambangCorongPenting && tahunIni && bln > 6:
		return []Wawasan{{BagianCorong, TingkatPenting, "Sebagian besar pagu perencanaan belum diproses", isi + " Sudah melewati pertengahan tahun: risiko penyerapan menumpuk di akhir tahun."}}
	case pb >= ambangCorongPerhatian:
		return []Wawasan{{BagianCorong, TingkatPerhatian, "Pagu perencanaan yang belum diproses masih besar", isi + " Pastikan paket yang akan dikerjakan segera masuk proses pemilihan, dan paket yang batal dihapus dari RUP."}}
	case pb < ambangCorongBaik:
		return []Wawasan{{BagianCorong, TingkatBaik, "Perencanaan sudah banyak masuk proses pengadaan", isi}}
	}
	return []Wawasan{{BagianCorong, TingkatInfo, "Keterhubungan perencanaan dengan proses pengadaan", isi}}
}

func wawasanMetodeRUP(h *Hasil) []Wawasan {
	r := h.RUP
	if r == nil || len(r.PerMetode) == 0 || r.TotalPagu <= 0 {
		return nil
	}
	atas := r.PerMetode[0]
	p := porsi(atas.Nilai, r.TotalPagu)
	if p < 40 {
		return nil
	}
	return []Wawasan{{BagianRUP, TingkatInfo, fmt.Sprintf("Metode %s mendominasi perencanaan", atas.Label),
		fmt.Sprintf("%s pagu paket penyedia (%s dari %s paket) direncanakan lewat metode %s.", Persen(p), Rupiah(atas.Nilai), Bulat(atas.Jumlah), atas.Label)}}
}

func wawasanSatker(h *Hasil) []Wawasan {
	r := h.RUP
	if r == nil || len(r.TopSatker) == 0 || r.TotalPagu <= 0 {
		return nil
	}
	atas := r.TopSatker[0]
	p := porsi(atas.Nilai, r.TotalPagu)
	isi := fmt.Sprintf("%s memegang %s pagu paket penyedia (%s dari %s) lewat %s paket.", atas.Label, Persen(p), Rupiah(atas.Nilai), Rupiah(r.TotalPagu), Bulat(atas.Jumlah))
	switch {
	case p >= ambangSatkerDominan:
		return []Wawasan{{BagianRUP, TingkatPerhatian, "Pagu terkonsentrasi pada satu satuan kerja", isi + " Kinerja penyerapan satker ini sangat menentukan capaian keseluruhan."}}
	case p >= ambangSatkerBesar:
		return []Wawasan{{BagianRUP, TingkatInfo, "Satuan kerja dengan pagu terbesar", isi}}
	}
	return nil
}

func wawasanTriwulan(h *Hasil) []Wawasan {
	r := h.RUP
	if r == nil || len(r.PerBulanPemilihan) == 0 {
		return nil
	}
	var total, q4, puncak float64
	puncakBulan := 0
	for _, t := range r.PerBulanPemilihan {
		total += t.Nilai
		if t.Bulan >= 10 && t.Bulan <= 12 {
			q4 += t.Nilai
		}
		if t.Nilai > puncak {
			puncak, puncakBulan = t.Nilai, t.Bulan
		}
	}
	if total <= 0 || puncakBulan < 1 || puncakBulan > 12 {
		return nil
	}
	pq4, pp := porsi(q4, total), porsi(puncak, total)
	switch {
	case pq4 >= ambangTriwulanIV:
		return []Wawasan{{BagianRUP, TingkatPerhatian, "Rencana pemilihan menumpuk di triwulan IV",
			fmt.Sprintf("%s pagu yang punya jadwal pemilihan (%s) dijadwalkan mulai pada Oktober-Desember; bulan puncaknya %s (%s). Jadwal sepadat ini berisiko membuat realisasi kontrak dan pembayaran menumpuk di akhir tahun.",
				Persen(pq4), Rupiah(q4), namaBulan[puncakBulan], Persen(pp))}}
	case pp >= ambangBulanPuncak:
		return []Wawasan{{BagianRUP, TingkatInfo, "Bulan puncak rencana pemilihan",
			fmt.Sprintf("%s mencatat %s dari pagu yang punya jadwal pemilihan (%s). Triwulan IV memuat %s.", namaBulan[puncakBulan], Persen(pp), Rupiah(puncak), Persen(pq4))}}
	}
	return nil
}

func sudahDiumumkan(label string) bool {
	l := strings.ToLower(label)
	return strings.Contains(l, "umumkan") && !strings.Contains(l, "belum") && !strings.Contains(l, "tidak")
}

func wawasanUmumkan(h *Hasil) []Wawasan {
	r := h.RUP
	if r == nil || len(r.StatusUmumkan) == 0 || r.TotalPagu <= 0 {
		return nil
	}
	var belumJml int64
	var belumNilai float64
	for _, s := range r.StatusUmumkan {
		if !sudahDiumumkan(s.Label) {
			belumJml += s.Jumlah
			belumNilai += s.Nilai
		}
	}
	p := porsi(belumNilai, r.TotalPagu)
	if p < ambangBelumDiumumkan {
		return nil
	}
	return []Wawasan{{BagianRUP, TingkatPerhatian, "Banyak paket RUP belum terumumkan",
		fmt.Sprintf("%s paket (%s pagu, %s) belum berstatus terumumkan di SiRUP. Paket baru boleh diproses setelah diumumkan, jadi keterlambatan pengumuman menunda seluruh siklus pengadaan.",
			Bulat(belumJml), Persen(p), Rupiah(belumNilai))}}
}

func wawasanStatusTender(h *Hasil) []Wawasan {
	p := h.Pemilihan
	if p == nil || len(p.StatusTender) == 0 {
		return nil
	}
	total, _ := jumlahNilai(p.StatusTender)
	var gagal int64
	for _, s := range p.StatusTender {
		l := strings.ToLower(s.Label)
		if strings.Contains(l, "gagal") || strings.Contains(l, "batal") || strings.Contains(l, "ulang") || strings.Contains(l, "ditutup") {
			gagal += s.Jumlah
		}
	}
	if total == 0 || gagal == 0 {
		return nil
	}
	pg := porsi(float64(gagal), float64(total))
	if pg < ambangTenderGagal {
		return nil
	}
	return []Wawasan{{BagianPemilihan, TingkatPerhatian, "Cukup banyak tender gagal atau diulang",
		fmt.Sprintf("%s dari %s tender (%s) berstatus gagal, dibatalkan, ditutup, atau diulang. Telaah penyebab umumnya (dokumen, HPS, kualifikasi) untuk menekan pengulangan yang memperlambat pengadaan.",
			Bulat(gagal), Bulat(total), Persen(pg))}}
}

func wawasanEfisiensi(h *Hasil) []Wawasan {
	p := h.Pemilihan
	if p == nil || p.Efisiensi.Sampel == 0 || p.Efisiensi.TotalHPS <= 0 {
		return nil
	}
	e := p.Efisiensi
	isi := fmt.Sprintf("Dari %s paket selesai, total HPS %s menjadi nilai kontrak %s: selisih %s (%s). Median per paket %s.",
		Bulat(e.Sampel), Rupiah(e.TotalHPS), Rupiah(e.TotalKontrak), Rupiah(e.TotalHPS-e.TotalKontrak), Persen(e.Persen), Persen(e.Median))
	if e.Sampel < minSampelEfisiensi {
		isi += " Sampel masih sedikit, jadi angka ini belum bisa dianggap representatif."
		return []Wawasan{{BagianPemilihan, TingkatInfo, "Efisiensi harga baru dihitung dari sedikit paket", isi}}
	}
	switch {
	case e.Persen < 0:
		return []Wawasan{{BagianPemilihan, TingkatPenting, "Nilai kontrak melampaui HPS",
			isi + " Nilai kontrak seharusnya tidak melebihi HPS; periksa paket dengan selisih negatif (kemungkinan addendum, data salah, atau HPS terlalu rendah)."}}
	case e.Persen < ambangEfisiensiTipis:
		return []Wawasan{{BagianPemilihan, TingkatPerhatian, "Efisiensi harga tipis",
			isi + " Hasil pemilihan nyaris sama dengan HPS: bisa berarti persaingan lemah atau HPS yang terlalu dekat dengan harga penawar."}}
	case e.Persen >= ambangEfisiensiSehat:
		return []Wawasan{{BagianPemilihan, TingkatBaik, "Efisiensi harga sehat", isi}}
	}
	return []Wawasan{{BagianPemilihan, TingkatInfo, "Efisiensi harga dari pemilihan", isi}}
}

func wawasanPersaingan(h *Hasil) []Wawasan {
	p := h.Pemilihan
	if p == nil || p.Persaingan.TenderBerpeserta == 0 {
		return nil
	}
	c := p.Persaingan
	ps := porsi(float64(c.SatuPeserta), float64(c.TenderBerpeserta))
	isi := fmt.Sprintf("%s dari %s tender (%s) hanya diikuti satu peserta; rata-rata %s peserta per tender.",
		Bulat(c.SatuPeserta), Bulat(c.TenderBerpeserta), Persen(ps), desimalID(c.RataPeserta, 1))
	switch {
	case ps >= ambangSatuPesertaPenting:
		return []Wawasan{{BagianPemilihan, TingkatPenting, "Persaingan tender sangat lemah", isi + " Tender dengan peserta tunggal rawan harga tidak kompetitif; tinjau syarat kualifikasi, waktu pengumuman, dan sosialisasi."}}
	case ps >= ambangSatuPeserta || c.RataPeserta < ambangRataPeserta:
		return []Wawasan{{BagianPemilihan, TingkatPerhatian, "Persaingan tender perlu diperkuat", isi}}
	case ps < ambangSatuPesertaBaik:
		return []Wawasan{{BagianPemilihan, TingkatBaik, "Persaingan tender cukup sehat", isi}}
	}
	return []Wawasan{{BagianPemilihan, TingkatInfo, "Tingkat persaingan tender", isi}}
}

func wawasanPasar(h *Hasil) []Wawasan {
	p := h.Pemilihan
	if p == nil || p.Pasar.TotalNilai <= 0 || len(p.Pasar.Top) == 0 {
		return nil
	}
	m := p.Pasar
	atas := m.Top[0]
	pa := porsi(atas.Nilai, m.TotalNilai)
	isi := fmt.Sprintf("%s penyedia memperoleh total nilai kontrak %s. Penyedia terbesar, %s, menguasai %s (%s dari %s kontrak). Indeks HHI %s.",
		Bulat(m.JumlahPenyedia), Rupiah(m.TotalNilai), atas.Label, Persen(pa), Rupiah(atas.Nilai), Bulat(atas.Jumlah), Bulat(int64(math.Round(m.HHI))))
	switch {
	case m.HHI >= ambangHHIPenting:
		return []Wawasan{{BagianPemilihan, TingkatPenting, "Pasar penyedia sangat terkonsentrasi", isi + " HHI di atas 2.500 menandakan nilai kontrak terkumpul pada segelintir penyedia; ketergantungan dan risiko kolusi perlu dipantau."}}
	case m.HHI >= ambangHHIPerhatian || pa >= ambangPenyediaTeratas:
		return []Wawasan{{BagianPemilihan, TingkatPerhatian, "Konsentrasi penyedia cukup tinggi", isi}}
	}
	return []Wawasan{{BagianPemilihan, TingkatBaik, "Pasar penyedia relatif terdiversifikasi", isi}}
}

func wawasanWaktuProses(h *Hasil) []Wawasan {
	p := h.Pemilihan
	if p == nil || p.WaktuProses.Sampel == 0 {
		return nil
	}
	w := p.WaktuProses
	isi := fmt.Sprintf("Median %s hari dari pengumuman tender sampai penetapan pemenang (rata-rata %s hari, %s tender).",
		desimalID(w.Median, 0), desimalID(w.Rata, 0), Bulat(w.Sampel))
	if w.Median > ambangWaktuProsesHari {
		return []Wawasan{{BagianPemilihan, TingkatPerhatian, "Proses pemilihan tender relatif lama", isi + " Proses di atas dua bulan menggeser mulai kontrak ke belakang dan memperbesar risiko penumpukan di akhir tahun."}}
	}
	return []Wawasan{{BagianPemilihan, TingkatInfo, "Lama proses pemilihan tender", isi}}
}

func wawasanAddendum(h *Hasil) []Wawasan {
	k := h.Kontrak
	if k == nil {
		return nil
	}
	total := k.TenderJumlah + k.NonTenderJumlah
	if total == 0 || k.Addendum == 0 {
		return nil
	}
	p := porsi(float64(k.Addendum), float64(total))
	isi := fmt.Sprintf("%s dari %s kontrak (%s) pernah diubah lewat addendum.", Bulat(k.Addendum), Bulat(total), Persen(p))
	switch {
	case p >= ambangAddendumPenting:
		return []Wawasan{{BagianKontrak, TingkatPenting, "Addendum sangat sering terjadi", isi + " Frekuensi setinggi ini menandakan perencanaan atau spesifikasi awal yang kurang matang."}}
	case p >= ambangAddendumPerhatian:
		return []Wawasan{{BagianKontrak, TingkatPerhatian, "Banyak kontrak beradendum", isi}}
	}
	return []Wawasan{{BagianKontrak, TingkatInfo, "Kontrak dengan addendum", isi}}
}

func wawasanKontrakBerakhir(h *Hasil) []Wawasan {
	k := h.Kontrak
	if k == nil || k.BerakhirDalam == 0 {
		return nil
	}
	hari := k.HariPeringatan
	if hari <= 0 {
		hari = 60
	}
	isi := fmt.Sprintf("%s kontrak bernilai total %s akan berakhir dalam %d hari.", Bulat(k.BerakhirDalam), Rupiah(k.NilaiBerakhir), hari)
	if len(k.AkanBerakhir) > 0 {
		pertama := k.AkanBerakhir[0]
		isi += fmt.Sprintf(" Yang paling dekat: %s (%s), berakhir %s, sisa %d hari.", ringkasTeks(pertama.NamaPaket, 80), Rupiah(pertama.Nilai), pertama.Berakhir.Format("02-01-2006"), pertama.SisaHari)
	}
	return []Wawasan{{BagianKontrak, TingkatPerhatian, "Kontrak segera berakhir", isi + " Pastikan serah terima dan pembayaran terjadwal."}}
}

func ringkasTeks(s string, maks int) string {
	r := []rune(strings.Join(strings.Fields(s), " "))
	if len(r) <= maks {
		return string(r)
	}
	return string(r[:maks-1]) + "…"
}

func wawasanEkatalog(h *Hasil) []Wawasan {
	e := h.Ekatalog
	if e == nil {
		return nil
	}
	var out []Wawasan
	ekat := e.V5.Nilai + e.V6.Nilai
	var kontrak float64
	if h.Pemilihan != nil {
		kontrak = h.Pemilihan.NilaiKontrak
	}
	if ekat > 0 {
		p := porsi(ekat, ekat+kontrak)
		isi := fmt.Sprintf("Transaksi e-purchasing mencapai %s (V5 %s, V6 %s), dibanding nilai kontrak tender dan non-tender %s.", Rupiah(ekat), Rupiah(e.V5.Nilai), Rupiah(e.V6.Nilai), Rupiah(kontrak))
		if kontrak > 0 && p >= ambangEkatalogBesar {
			isi = fmt.Sprintf("E-Katalog menyumbang %s dari seluruh nilai pengadaan yang terealisasi. ", Persen(p)) + isi
			out = append(out, Wawasan{BagianEkatalog, TingkatInfo, "E-Katalog menjadi saluran pengadaan utama", isi})
		} else {
			out = append(out, Wawasan{BagianEkatalog, TingkatInfo, "Realisasi lewat e-purchasing", isi})
		}
	}
	if e.V5.Nilai > 0 && len(e.V5.TopKomoditas) > 0 {
		atas := e.V5.TopKomoditas[0]
		if p := porsi(atas.Nilai, e.V5.Nilai); p >= ambangKomoditasDominan {
			out = append(out, Wawasan{BagianEkatalog, TingkatPerhatian, "Belanja e-katalog bergantung pada satu komoditas",
				fmt.Sprintf("Komoditas %s menyerap %s nilai e-purchasing V5 (%s dari %s).", atas.Label, Persen(p), Rupiah(atas.Nilai), Rupiah(e.V5.Nilai))})
		}
	}
	if e.V6.Nilai > 0 && e.V6.NilaiSwasta > 0 {
		if p := porsi(e.V6.NilaiSwasta, e.V6.Nilai); p >= ambangSwasta {
			out = append(out, Wawasan{BagianEkatalog, TingkatInfo, "Porsi paket swasta di E-Katalog V6 cukup besar",
				fmt.Sprintf("%s nilai e-purchasing V6 (%s dari %s) berasal dari paket swasta (%s order).", Persen(p), Rupiah(e.V6.NilaiSwasta), Rupiah(e.V6.Nilai), Bulat(e.V6.OrderSwasta))})
		}
	}
	return out
}
