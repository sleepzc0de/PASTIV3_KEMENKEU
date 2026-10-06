package sapa

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"pasti-v3-backend/sapa/docx"
)

// Kesalahan yang dibedakan oleh lapisan HTTP.
var (
	ErrTidakBerhak    = errors.New("Anda tidak berhak melakukan tindakan ini")
	ErrTidakDitemukan = errors.New("data tidak ditemukan")
)

// ErrKonflik: tindakan bertentangan dengan keadaan usulan (urutan tahap, tahap sudah selesai, dan sebagainya).
type ErrKonflik struct{ Pesan string }

func (e *ErrKonflik) Error() string { return e.Pesan }

func konflik(format string, a ...interface{}) error {
	return &ErrKonflik{Pesan: fmt.Sprintf(format, a...)}
}

func validasi(format string, a ...interface{}) error {
	return &ErrValidasi{Rincian: []string{fmt.Sprintf(format, a...)}}
}

const (
	MaksDataTahap = 1 << 20 // 1 MB JSON isian formulir
	MaksTemplate  = 5 << 20 // 5 MB berkas template
	MaksCatatan   = 1000
	kode18Pola    = `^[0-9]{18}$`
	kodeUE1Pola   = `^[0-9]{5}$`
)

var (
	reKode18 = regexp.MustCompile(kode18Pola)
	reUE1    = regexp.MustCompile(kodeUE1Pola)
	reGUID   = regexp.MustCompile(`^[0-9A-Fa-f]{8}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{12}$`)
)

// Layanan memuat aturan bisnis SAPA di atas penyimpanan Repo.
type Layanan struct{ Repo Repo }

// ---------------------------------------------------------------- identitas

// IdentitasDari menyusun identitas pengguna dari peran data aplikasi yang sedang berlaku (pa; lihat paket peran): Satker (kode 6 digit), Kanwil (9 digit),
// UE1 (5 digit), Pengguna Barang, atau admin/superadmin. SAPA tidak menetapkan peran sendiri. nama dipakai bila data pengguna tidak memuat nama lengkap.
func (l *Layanan) IdentitasDari(ctx context.Context, userID, nama string, pa PeranAplikasi) (Identitas, error) {
	id := Identitas{UserID: userID, Nama: nama, Admin: pa.Admin}
	n, err := l.Repo.NamaPengguna(ctx, userID)
	if err != nil {
		return id, err
	}
	if n != "" {
		id.Nama = n
	}
	kode := strings.TrimSpace(pa.Kode)
	switch strings.TrimSpace(pa.Peran) {
	case PeranSatker:
		id.Peran, id.KodeSatker = PeranSatker, kode
	case PeranKanwil:
		id.Peran, id.KodeKanwil = PeranKanwil, kode
	case PeranUE1:
		id.Peran, id.KodeUE1 = PeranUE1, kode
	case PeranPenggunaBarang:
		id.Peran = PeranPenggunaBarang
	}
	return id, nil
}

// SatkerSaya: satker pada data Digitalisasi Aset yang kode 6 digitnya sama dengan peran Satker pengguna (induk lebih dulu), untuk menyiapkan kode satker
// lengkap saat membuat usulan. Kosong bagi peran lain.
func (l *Layanan) SatkerSaya(ctx context.Context, id Identitas) ([]SatkerInfo, error) {
	if id.Peran != PeranSatker || strings.TrimSpace(id.KodeSatker) == "" {
		return []SatkerInfo{}, nil
	}
	return l.Repo.SatkerDenganKode6(ctx, strings.TrimSpace(id.KodeSatker))
}
// ---------------------------------------------------------------- usulan: membuat dan melihat

// judul: huruf awal setiap kata kapital, sisanya kecil ("KOTA JAKARTA" -> "Kota Jakarta").
func judul(s string) string {
	words := strings.Fields(strings.ToLower(s))
	for i, w := range words {
		r, n := utf8.DecodeRuneInString(w)
		words[i] = string(unicode.ToUpper(r)) + w[n:]
	}
	return strings.Join(words, " ")
}

// awalanWilayah: awalan administratif pada nama kabupaten/kota di data satker. Surat memakai nama tempatnya saja
// ("Jakarta Pusat", bukan "Kota Adm. Jakarta Pusat"), jadi awalan dibuang untuk saran isian kota.
var awalanWilayah = []string{"KOTA ADMINISTRASI ", "KOTA ADM. ", "KOTA ADM ", "KABUPATEN ", "KAB. ", "KAB ", "KOTA "}

func namaKota(kabKota string) string {
	s := strings.Join(strings.Fields(kabKota), " ")
	up := strings.ToUpper(s)
	for _, a := range awalanWilayah {
		if strings.HasPrefix(up, a) && len(strings.TrimSpace(s[len(a):])) > 0 {
			s = s[len(a):]
			break
		}
	}
	return judul(s)
}

// BuatUsulan membuat usulan penjualan baru dan memberinya nomor registrasi (Noreg). Nama satker diambil dari data
// Digitalisasi Aset bila kodenya ada di sana; bila tidak, namaSatker harus diisi.
func (l *Layanan) BuatUsulan(ctx context.Context, id Identitas, kodeSatker, namaSatker string) (*KasusInfo, error) {
	kode := Kode18(kodeSatker)
	if !reKode18.MatchString(kode) {
		return nil, validasi("Kode satker harus 18 digit angka")
	}
	if err := BolehMembuat(id, kode); err != nil {
		if errors.Is(err, ErrTanpaPeran) {
			return nil, err
		}
		return nil, fmt.Errorf("%w: %s", ErrTidakBerhak, err.Error())
	}
	nama := strings.TrimSpace(namaSatker)
	if s, err := l.Repo.CariSatker(ctx, kode); err != nil {
		return nil, err
	} else if s != nil && s.Nama != "" {
		nama = s.Nama
	}
	if nama == "" {
		return nil, validasi("Kode satker tidak ditemukan di data Digitalisasi Aset; isi nama satker")
	}
	if utf8.RuneCountInString(nama) > 300 {
		return nil, validasi("Nama satker terlalu panjang (maksimal 300 karakter)")
	}
	k, err := l.Repo.BuatPenjualan(ctx, BuatInput{KodeSatker: kode, NamaSatker: nama, KodeUE1: KodeUE1Dari(kode), UserID: id.UserID, Oleh: id.Nama})
	if err != nil {
		return nil, err
	}
	return &k, nil
}

// muat mengambil usulan yang boleh dilihat pengguna. Usulan yang tidak boleh dilihat dianggap tidak ada.
func (l *Layanan) muat(ctx context.Context, id Identitas, pid int64) (*KasusInfo, error) {
	if err := Akses(id); err != nil {
		return nil, err
	}
	k, err := l.Repo.AmbilPenjualan(ctx, pid)
	if err != nil {
		return nil, err
	}
	if k == nil || !Terlihat(id, k.Kasus()) {
		return nil, ErrTidakDitemukan
	}
	return k, nil
}

// IDUsulan mengubah UUID usulan (pengenal publik di alamat halaman dan API) menjadi id internal. UUID yang bentuknya tidak
// sah, usulan yang tidak ada, dan usulan yang tidak boleh dilihat pengguna dijawab sama (ErrTidakDitemukan), sehingga
// keberadaan usulan tidak bisa diketahui dengan menebak. Hak akses tetap diperiksa lagi oleh tiap aksi lewat muat.
func (l *Layanan) IDUsulan(ctx context.Context, id Identitas, uuid string) (int64, error) {
	if err := Akses(id); err != nil {
		return 0, err
	}
	baku, ok := BakukanUUID(uuid)
	if !ok {
		return 0, ErrTidakDitemukan
	}
	k, err := l.Repo.AmbilPenjualanUUID(ctx, baku)
	if err != nil {
		return 0, err
	}
	if k == nil || !Terlihat(id, k.Kasus()) {
		return 0, ErrTidakDitemukan
	}
	return k.ID, nil
}

// RingkasanUsulan: satu baris pada daftar usulan.
type RingkasanUsulan struct {
	KasusInfo
	TahapSaatIni      string `json:"tahap_saat_ini"`
	TahapSaatIniLabel string `json:"tahap_saat_ini_label"`
	PeranSaatIni      string `json:"peran_saat_ini"`
	Selesai           bool   `json:"selesai"`
	TahapSelesai      int    `json:"tahap_selesai"`
	TahapTotal        int    `json:"tahap_total"`
}

type HalamanUsulan struct {
	Usulan  []RingkasanUsulan `json:"usulan"`
	Total   int               `json:"total"`
	Halaman int               `json:"halaman"`
	PerHal  int               `json:"per_halaman"`
}

func (l *Layanan) DaftarUsulan(ctx context.Context, id Identitas, f FilterDaftar) (*HalamanUsulan, error) {
	if err := Akses(id); err != nil {
		return nil, err
	}
	if f.Limit < 1 || f.Limit > 100 {
		f.Limit = 20
	}
	if f.Offset < 0 {
		f.Offset = 0
	}
	if f.Status != "" && f.Status != "berjalan" && f.Status != "selesai" {
		return nil, validasi("Status tidak dikenal")
	}
	f.Q = strings.TrimSpace(f.Q)
	if utf8.RuneCountInString(f.Q) > 100 {
		return nil, validasi("Kata kunci terlalu panjang")
	}
	list, total, err := l.Repo.DaftarPenjualan(ctx, ScopeDari(id), f)
	if err != nil {
		return nil, err
	}
	ids := make([]int64, len(list))
	for i, k := range list {
		ids[i] = k.ID
	}
	statuses, err := l.Repo.StatusTahapBanyak(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make([]RingkasanUsulan, len(list))
	for i, k := range list {
		s := statuses[k.ID]
		r := RingkasanUsulan{KasusInfo: k, TahapTotal: len(TahapPenjualan)}
		for _, t := range TahapPenjualan {
			if selesaiAtauDilewati(s.get(t.Kunci)) {
				r.TahapSelesai++
			}
		}
		if cur := TahapSaatIni(s); cur == "" {
			r.Selesai = true
		} else {
			t, _, _ := TahapByKunci(cur)
			r.TahapSaatIni, r.TahapSaatIniLabel, r.PeranSaatIni = t.Kunci, t.Label, t.Peran
		}
		out[i] = r
	}
	return &HalamanUsulan{Usulan: out, Total: total, Halaman: f.Offset/f.Limit + 1, PerHal: f.Limit}, nil
}

// TahapDetail: satu tahap beserta keadaannya untuk pengguna yang melihat.
type TahapDetail struct {
	Tahap
	Urutan          int             `json:"urutan"`
	Status          string          `json:"status"`
	DapatDikerjakan bool            `json:"dapat_dikerjakan"` // urutan sudah sampai
	AlasanTerkunci  string          `json:"alasan_terkunci,omitempty"`
	BolehAksi       bool            `json:"boleh_aksi"` // peran pengguna sesuai
	DapatDiubah     bool            `json:"dapat_diubah"`
	Data            json.RawMessage `json:"data,omitempty"`
	Saran           interface{}     `json:"saran,omitempty"`
	Nomor           string          `json:"nomor,omitempty"`
	Tanggal         string          `json:"tanggal,omitempty"`
	Catatan         string          `json:"catatan,omitempty"`
	DiperbaruiOleh  string          `json:"diperbarui_oleh,omitempty"`
	DiperbaruiPada  *time.Time      `json:"diperbarui_pada,omitempty"`
	// Dokumen (hasil) menutupi bidang Dokumen milik Tahap pada JSON, sehingga jenis dokumen yang dihasilkan tahap form
	// disajikan terpisah di JenisDokumen.
	Dokumen          []DokumenInfo   `json:"dokumen"`
	JenisDokumen     string          `json:"jenis_dokumen,omitempty"`
	TemplateTersedia map[string]bool `json:"template_tersedia,omitempty"`
}

type DetailUsulan struct {
	Usulan       KasusInfo     `json:"usulan"`
	Tahap        []TahapDetail `json:"tahap"`
	TahapSaatIni string        `json:"tahap_saat_ini"`
	Selesai      bool          `json:"selesai"`
}

func statusDari(rows map[string]TahapRow) StatusTahap {
	s := StatusTahap{}
	for k, r := range rows {
		s[k] = r.Status
	}
	return s
}

func (l *Layanan) templateTersedia(ctx context.Context, jenis string) bool {
	if _, ok := TemplateBawaan(jenis); ok {
		return true
	}
	b, _, err := l.Repo.TemplateAktif(ctx, jenis)
	return err == nil && len(b) > 0
}

func (l *Layanan) DetailUsulan(ctx context.Context, id Identitas, pid int64) (*DetailUsulan, error) {
	k, err := l.muat(ctx, id, pid)
	if err != nil {
		return nil, err
	}
	rows, err := l.Repo.TahapPenjualan(ctx, pid)
	if err != nil {
		return nil, err
	}
	docs, err := l.Repo.DaftarDokumen(ctx, pid)
	if err != nil {
		return nil, err
	}
	status := statusDari(rows)
	out := &DetailUsulan{Usulan: *k, TahapSaatIni: TahapSaatIni(status), Selesai: Selesai(status)}
	for i, def := range TahapPenjualan {
		row := rows[def.Kunci]
		td := TahapDetail{Tahap: def, Urutan: i + 1, Status: status.get(def.Kunci), Dokumen: []DokumenInfo{}}
		if err := BolehDikerjakan(status, def.Kunci); err != nil {
			td.AlasanTerkunci = err.Error()
		} else {
			td.DapatDikerjakan = true
		}
		td.BolehAksi = BolehBertindak(id, k.Kasus(), def)
		td.DapatDiubah = BolehDiubah(status, def.Kunci) == nil
		if len(row.Data) > 0 {
			td.Data = json.RawMessage(row.Data)
		}
		td.Nomor, td.Tanggal, td.Catatan, td.DiperbaruiOleh = row.Nomor, row.Tanggal, row.Catatan, row.DiperbaruiOleh
		if !row.DiperbaruiPada.IsZero() {
			t := row.DiperbaruiPada
			td.DiperbaruiPada = &t
		}
		for _, d := range docs {
			if d.Tahap == def.Kunci {
				td.Dokumen = append(td.Dokumen, d)
			}
		}
		if def.Jenis == JenisForm {
			td.Saran = l.saran(ctx, def.Kunci, k, rows)
			if len(def.Dokumen) > 0 {
				td.JenisDokumen = def.Dokumen[0]
			}
			td.TemplateTersedia = map[string]bool{}
			for _, dk := range def.Dokumen {
				td.TemplateTersedia[dk] = l.templateTersedia(ctx, dk)
			}
		}
		out.Tahap = append(out.Tahap, td)
	}
	return out, nil
}

// saran memberi nilai awal formulir dari data yang sudah ada (referensi UE1, data satker, tahap sebelumnya).
func (l *Layanan) saran(ctx context.Context, kunci string, k *KasusInfo, rows map[string]TahapRow) interface{} {
	kota := ""
	if s, err := l.Repo.CariSatker(ctx, Kode18(k.KodeSatker)); err == nil && s != nil {
		kota = namaKota(s.KabKota)
	}
	sekretaris := ""
	if r, err := l.Repo.AmbilRefUE1(ctx, k.KodeUE1); err == nil && r != nil {
		sekretaris = r.Sekretaris
	}
	switch kunci {
	case TahapTim:
		return DataTim{JenisTim: JenisTimValid[0], Kota: kota}
	case TahapBA:
		nama := ""
		var tim DataTim
		if r, ok := rows[TahapTim]; ok && len(r.Data) > 0 && json.Unmarshal(r.Data, &tim) == nil {
			nama = tim.JenisTim
		}
		if nama == "" {
			nama = JenisTimValid[0]
		}
		return DataBA{Bentuk: BentukValid[0], NamaTim: nama}
	case TahapNDSatker:
		return DataNDSatker{TujuanSurat: sekretaris, Kota: kota}
	case TahapNDUE1:
		d := DataNDUE1{SekretarisUE1: sekretaris}
		var nd DataNDSatker
		if r, ok := rows[TahapNDSatker]; ok && len(r.Data) > 0 && json.Unmarshal(r.Data, &nd) == nil {
			if nd.TujuanSurat != "" {
				d.SekretarisUE1 = nd.TujuanSurat
			}
			d.KepalaKanwil = nd.KepalaKanwil
		}
		if r, ok := rows[TahapNadineSatker]; ok {
			d.NomorND, d.TanggalND = r.Nomor, r.Tanggal
		}
		d.HalND = "Permohonan Penjualan Barang Milik Negara pada Kementerian Keuangan c.q. " + k.NamaSatker
		return d
	}
	return nil
}

// ---------------------------------------------------------------- tindakan pada tahap

type konteksTahap struct {
	kasus  *KasusInfo
	tahap  Tahap
	rows   map[string]TahapRow
	status StatusTahap
}

// siapKerja memeriksa hak akses dan urutan sebelum sebuah tahap dikerjakan atau diubah.
func (l *Layanan) siapKerja(ctx context.Context, id Identitas, pid int64, kunci string) (*konteksTahap, error) {
	k, err := l.muat(ctx, id, pid)
	if err != nil {
		return nil, err
	}
	def, _, ok := TahapByKunci(kunci)
	if !ok {
		return nil, ErrTidakDitemukan
	}
	if !BolehBertindak(id, k.Kasus(), def) {
		return nil, fmt.Errorf("%w: tahap ini dikerjakan oleh %s", ErrTidakBerhak, PeranLabel(def.Peran))
	}
	rows, err := l.Repo.TahapPenjualan(ctx, pid)
	if err != nil {
		return nil, err
	}
	status := statusDari(rows)
	if err := BolehDikerjakan(status, kunci); err != nil {
		return nil, &ErrKonflik{Pesan: capital(err.Error())}
	}
	if selesaiAtauDilewati(status.get(kunci)) {
		if err := BolehDiubah(status, kunci); err != nil {
			return nil, &ErrKonflik{Pesan: capital(err.Error())}
		}
	}
	return &konteksTahap{kasus: k, tahap: def, rows: rows, status: status}, nil
}

func capital(s string) string {
	if s == "" {
		return s
	}
	r, n := utf8.DecodeRuneInString(s)
	return strings.ToUpper(string(r)) + s[n:]
}

// decodeData membaca JSON isian ke struktur milik tahap dan merapikannya.
func decodeData(kunci string, raw []byte) (interface{}, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, validasi("Data formulir kosong")
	}
	if len(raw) > MaksDataTahap {
		return nil, validasi("Data formulir terlalu besar")
	}
	var v interface{ Rapikan() }
	switch kunci {
	case TahapTim:
		v = &DataTim{}
	case TahapBA:
		v = &DataBA{}
	case TahapNDSatker:
		v = &DataNDSatker{}
	case TahapNDUE1:
		v = &DataNDUE1{}
	default:
		return nil, konflik("Tahap ini tidak memakai formulir")
	}
	if err := json.Unmarshal(raw, v); err != nil {
		return nil, validasi("Data formulir tidak valid: %s", err.Error())
	}
	v.Rapikan()
	return v, nil
}

// SimpanDraf menyimpan isian formulir tanpa memvalidasi kelengkapannya.
func (l *Layanan) SimpanDraf(ctx context.Context, id Identitas, pid int64, kunci string, raw []byte) error {
	kt, err := l.siapKerja(ctx, id, pid, kunci)
	if err != nil {
		return err
	}
	if kt.tahap.Jenis != JenisForm {
		return konflik("Tahap ini tidak memakai formulir")
	}
	v, err := decodeData(kunci, raw)
	if err != nil {
		return err
	}
	data, _ := json.Marshal(v)
	return l.Repo.SimpanTahap(ctx, pid, TahapRow{Kunci: kunci, Status: StatusDraft, Data: data, DiperbaruiOleh: id.Nama, DiperbaruiPada: time.Now().UTC()})
}

// HasilDokumen: dokumen yang baru dibuat.
type HasilDokumen struct {
	Dokumen    DokumenInfo `json:"dokumen"`
	Peringatan []string    `json:"peringatan"`
}

func (l *Layanan) template(ctx context.Context, jenis string) ([]byte, error) {
	b, _, err := l.Repo.TemplateAktif(ctx, jenis)
	if err != nil {
		return nil, err
	}
	if len(b) > 0 {
		return b, nil
	}
	if b, ok := TemplateBawaan(jenis); ok {
		return b, nil
	}
	return nil, ErrTemplateTidakAda
}

// Hasilkan memvalidasi isian formulir, mengisi template Word, menyimpan dokumennya, dan menandai tahap selesai.
func (l *Layanan) Hasilkan(ctx context.Context, id Identitas, pid int64, kunci string, raw []byte) (*HasilDokumen, error) {
	kt, err := l.siapKerja(ctx, id, pid, kunci)
	if err != nil {
		return nil, err
	}
	if kt.tahap.Jenis != JenisForm || len(kt.tahap.Dokumen) != 1 {
		return nil, konflik("Tahap ini tidak menghasilkan dokumen")
	}
	jenis := kt.tahap.Dokumen[0]
	v, err := decodeData(kunci, raw)
	if err != nil {
		return nil, err
	}
	// Jenis dan satuan BMN harus sesuai daftar admin (mis. Tanah tidak boleh bersatuan "unit").
	if d, ok := v.(*DataNDSatker); ok {
		if err := l.cekBMN(ctx, d); err != nil {
			return nil, err
		}
	}
	tpl, err := l.template(ctx, jenis)
	if err != nil {
		return nil, err
	}
	kasus := kt.kasus.Kasus()

	var hasil *Hasil
	switch d := v.(type) {
	case *DataTim:
		hasil, err = IsiSKTim(tpl, kasus, *d)
	case *DataBA:
		hasil, err = IsiBA(tpl, kasus, *d)
	case *DataNDSatker:
		hasil, err = IsiNDSatker(tpl, kasus, *d)
	case *DataNDUE1:
		var satker DataNDSatker
		r, ok := kt.rows[TahapNDSatker]
		if !ok || len(r.Data) == 0 || json.Unmarshal(r.Data, &satker) != nil {
			return nil, konflik("Data Nota Dinas usulan Satker tidak ditemukan")
		}
		hasil, err = IsiNDUE1(tpl, kasus, satker, *d)
	}
	if err != nil {
		return nil, err
	}

	data, _ := json.Marshal(v)
	info, err := l.Repo.SimpanDokumen(ctx, DokumenBaru{PenjualanID: pid, Tahap: kunci, Jenis: jenis, NamaFile: hasil.NamaFile, Berkas: hasil.Berkas, Peringatan: hasil.Peringatan, Oleh: id.Nama})
	if err != nil {
		return nil, err
	}
	if err := l.Repo.SimpanTahap(ctx, pid, TahapRow{Kunci: kunci, Status: StatusSelesai, Data: data, DiperbaruiOleh: id.Nama, DiperbaruiPada: time.Now().UTC()}); err != nil {
		return nil, err
	}
	if j, ok := JenisByKunci(jenis); ok {
		info.JenisLabel = j.Label
	}
	return &HasilDokumen{Dokumen: info, Peringatan: hasil.Peringatan}, nil
}

// Selesaikan menandai tahap eksternal (Nadine/SIMAN) selesai, dengan nomor, tanggal, dan catatan dari aplikasi tersebut.
func (l *Layanan) Selesaikan(ctx context.Context, id Identitas, pid int64, kunci, nomor, tanggal, catatan string) error {
	kt, err := l.siapKerja(ctx, id, pid, kunci)
	if err != nil {
		return err
	}
	if kt.tahap.Jenis != JenisEksternal {
		return konflik("Tahap ini diselesaikan dengan membuat dokumen")
	}
	nomor, tanggal, catatan = strings.TrimSpace(nomor), strings.TrimSpace(tanggal), strings.TrimSpace(catatan)
	var g galat
	g.opsional("Nomor", nomor, 150)
	g.opsional("Catatan", catatan, MaksCatatan)
	if kt.tahap.WajibNomorTanggal {
		g.wajib("Nomor", nomor, 150)
	}
	g.tanggal("Tanggal", tanggal, kt.tahap.WajibNomorTanggal)
	if len(g) > 0 {
		return &ErrValidasi{Rincian: g}
	}
	return l.Repo.SimpanTahap(ctx, pid, TahapRow{Kunci: kunci, Status: StatusSelesai, Nomor: nomor, Tanggal: tanggal, Catatan: catatan, DiperbaruiOleh: id.Nama, DiperbaruiPada: time.Now().UTC()})
}

// Lewati menandai tahap form yang dikerjakan di luar aplikasi (SK Tim, Berita Acara) sebagai dilewati.
func (l *Layanan) Lewati(ctx context.Context, id Identitas, pid int64, kunci, catatan string) error {
	kt, err := l.siapKerja(ctx, id, pid, kunci)
	if err != nil {
		return err
	}
	if !kt.tahap.BolehDilewati {
		return konflik("Tahap ini tidak boleh dilewati")
	}
	catatan = strings.TrimSpace(catatan)
	if utf8.RuneCountInString(catatan) < 5 {
		return validasi("Tuliskan alasan melewati tahap ini (minimal 5 karakter), mis. nomor dan tanggal dokumen yang dibuat di luar aplikasi")
	}
	if utf8.RuneCountInString(catatan) > MaksCatatan {
		return validasi("Catatan terlalu panjang (maksimal %d karakter)", MaksCatatan)
	}
	prev := kt.rows[kunci]
	return l.Repo.SimpanTahap(ctx, pid, TahapRow{Kunci: kunci, Status: StatusDilewati, Data: prev.Data, Catatan: catatan, DiperbaruiOleh: id.Nama, DiperbaruiPada: time.Now().UTC()})
}

// BukaUlang (khusus admin) mengembalikan tahap yang sudah selesai atau dilewati menjadi draf, selama belum ada tahap
// sesudahnya yang selesai. Dokumen yang pernah dibuat tetap tersimpan sebagai riwayat.
func (l *Layanan) BukaUlang(ctx context.Context, id Identitas, pid int64, kunci string) error {
	if err := Akses(id); err != nil {
		return err
	}
	if !id.Admin {
		return ErrTidakBerhak
	}
	if _, err := l.muat(ctx, id, pid); err != nil {
		return err
	}
	if _, _, ok := TahapByKunci(kunci); !ok {
		return ErrTidakDitemukan
	}
	rows, err := l.Repo.TahapPenjualan(ctx, pid)
	if err != nil {
		return err
	}
	status := statusDari(rows)
	if !selesaiAtauDilewati(status.get(kunci)) {
		return konflik("Tahap ini belum selesai")
	}
	if err := BolehDiubah(status, kunci); err != nil {
		return &ErrKonflik{Pesan: capital(err.Error())}
	}
	prev := rows[kunci]
	return l.Repo.SimpanTahap(ctx, pid, TahapRow{Kunci: kunci, Status: StatusDraft, Data: prev.Data, Nomor: prev.Nomor, Tanggal: prev.Tanggal, Catatan: prev.Catatan, DiperbaruiOleh: id.Nama, DiperbaruiPada: time.Now().UTC()})
}

// UnduhDokumen mengembalikan berkas dokumen hasil bila pengguna boleh melihat usulannya.
func (l *Layanan) UnduhDokumen(ctx context.Context, id Identitas, dokID int64) (*DokumenInfo, []byte, error) {
	if err := Akses(id); err != nil {
		return nil, nil, err
	}
	info, data, err := l.Repo.AmbilDokumen(ctx, dokID)
	if err != nil {
		return nil, nil, err
	}
	if info == nil {
		return nil, nil, ErrTidakDitemukan
	}
	if _, err := l.muat(ctx, id, info.PenjualanID); err != nil {
		return nil, nil, err
	}
	return info, data, nil
}

// ---------------------------------------------------------------- referensi untuk formulir

// CariSatker mencari satker menurut kode 18 digit di data Digitalisasi Aset, untuk mengisi formulir usulan baru.
func (l *Layanan) CariSatker(ctx context.Context, id Identitas, kode string) (*SatkerInfo, *RefUE1, error) {
	if err := Akses(id); err != nil {
		return nil, nil, err
	}
	kode = Kode18(kode)
	if !reKode18.MatchString(kode) {
		return nil, nil, validasi("Kode satker harus 18 digit angka")
	}
	s, err := l.Repo.CariSatker(ctx, kode)
	if err != nil {
		return nil, nil, err
	}
	ref, err := l.Repo.AmbilRefUE1(ctx, KodeUE1Dari(kode))
	if err != nil {
		return nil, nil, err
	}
	return s, ref, nil
}

// ---------------------------------------------------------------- administrasi (khusus admin)

func (l *Layanan) khususAdmin(id Identitas) error {
	if !id.Admin {
		return ErrTidakBerhak
	}
	return nil
}

func (l *Layanan) DaftarRefUE1(ctx context.Context, id Identitas) ([]RefUE1, error) {
	if err := l.khususAdmin(id); err != nil {
		return nil, err
	}
	return l.Repo.DaftarRefUE1(ctx)
}

func (l *Layanan) SimpanRefUE1(ctx context.Context, id Identitas, r RefUE1) error {
	if err := l.khususAdmin(id); err != nil {
		return err
	}
	r.Kode, r.Nama, r.Sekretaris = strings.TrimSpace(r.Kode), strings.TrimSpace(r.Nama), strings.TrimSpace(r.Sekretaris)
	var g galat
	if !reUE1.MatchString(r.Kode) {
		g.add("Kode UE1 harus 5 digit angka")
	}
	g.wajib("Nama UE1", r.Nama, 200)
	g.wajib("Sebutan sekretaris", r.Sekretaris, 300)
	if len(g) > 0 {
		return &ErrValidasi{Rincian: g}
	}
	return l.Repo.SimpanRefUE1(ctx, r, id.Nama)
}

func (l *Layanan) HapusRefUE1(ctx context.Context, id Identitas, kode string) error {
	if err := l.khususAdmin(id); err != nil {
		return err
	}
	if !reUE1.MatchString(strings.TrimSpace(kode)) {
		return validasi("Kode UE1 harus 5 digit angka")
	}
	return l.Repo.HapusRefUE1(ctx, strings.TrimSpace(kode))
}

// StatusTemplate: keadaan template satu jenis dokumen.
type StatusTemplate struct {
	Jenis   JenisDokumen   `json:"jenis"`
	Sumber  string         `json:"sumber"` // "unggahan", "bawaan", atau "belum"
	Aktif   *TemplateInfo  `json:"aktif,omitempty"`
	Riwayat []TemplateInfo `json:"riwayat"`
}

func (l *Layanan) DaftarTemplate(ctx context.Context, id Identitas) ([]StatusTemplate, error) {
	if err := l.khususAdmin(id); err != nil {
		return nil, err
	}
	out := make([]StatusTemplate, 0, len(DaftarJenis))
	for _, j := range DaftarJenis {
		st := StatusTemplate{Jenis: j, Sumber: "belum", Riwayat: []TemplateInfo{}}
		if _, ok := TemplateBawaan(j.Kunci); ok {
			st.Sumber = "bawaan"
		}
		riwayat, err := l.Repo.RiwayatTemplate(ctx, j.Kunci)
		if err != nil {
			return nil, err
		}
		for i := range riwayat {
			if riwayat[i].Aktif {
				a := riwayat[i]
				st.Aktif = &a
				st.Sumber = "unggahan"
			}
		}
		if riwayat != nil {
			st.Riwayat = riwayat
		}
		out = append(out, st)
	}
	return out, nil
}

// HasilUnggah: hasil memeriksa template yang diunggah.
type HasilUnggah struct {
	Template     TemplateInfo `json:"template"`
	Penanda      []string     `json:"penanda"`       // penanda yang ditemukan pada template
	TidakDikenal []string     `json:"tidak_dikenal"` // penanda yang tidak tercantum di daftar penanda jenis ini
	TidakDipakai []string     `json:"tidak_dipakai"` // penanda yang dikenal tetapi tidak ada di template
	Peringatan   []string     `json:"peringatan"`
}

// dryRun mengisi template dengan data contoh untuk memastikan strukturnya bisa diisi sebelum dipakai pengguna.
func dryRun(jenis string, tpl []byte) (*Hasil, error) {
	switch jenis {
	case DokSKTim:
		return IsiSKTim(tpl, ContohKasus(), ContohTim())
	case DokBA:
		return IsiBA(tpl, ContohKasus(), ContohBA())
	case DokNDSatker:
		return IsiNDSatker(tpl, ContohKasus(), ContohNDSatker())
	case DokNDUE1:
		return IsiNDUE1(tpl, ContohKasus(), ContohNDSatker(), ContohNDUE1())
	}
	return nil, validasi("Jenis dokumen tidak dikenal")
}

// UnggahTemplate memeriksa lalu menyimpan template baru (menjadi versi aktif). Template yang tidak bisa dibuka atau
// strukturnya tidak sesuai (mis. tabel Daftar Barang tidak ada) ditolak.
func (l *Layanan) UnggahTemplate(ctx context.Context, id Identitas, jenis, namaFile string, berkas []byte, catatan string) (*HasilUnggah, error) {
	if err := l.khususAdmin(id); err != nil {
		return nil, err
	}
	j, ok := JenisByKunci(jenis)
	if !ok {
		return nil, ErrTidakDitemukan
	}
	namaFile = strings.TrimSpace(namaFile)
	if !strings.HasSuffix(strings.ToLower(namaFile), ".docx") {
		return nil, validasi("Berkas template harus berformat .docx")
	}
	if len(berkas) == 0 {
		return nil, validasi("Berkas template kosong")
	}
	if len(berkas) > MaksTemplate {
		return nil, validasi("Berkas template terlalu besar (maksimal %d MB)", MaksTemplate>>20)
	}
	catatan = strings.TrimSpace(catatan)
	if utf8.RuneCountInString(catatan) > 500 {
		return nil, validasi("Catatan terlalu panjang (maksimal 500 karakter)")
	}
	doc, err := docx.Open(berkas)
	if err != nil {
		return nil, validasi("Template tidak dapat dibaca: %s", err.Error())
	}
	found := doc.Tags()
	dikenal := map[string]bool{}
	for _, p := range j.Penanda {
		dikenal[p.Nama] = true
	}
	res := &HasilUnggah{Penanda: found, TidakDikenal: []string{}, TidakDipakai: []string{}, Peringatan: []string{}}
	ada := map[string]bool{}
	for _, t := range found {
		ada[t] = true
		if !dikenal[t] {
			res.TidakDikenal = append(res.TidakDikenal, t)
		}
	}
	for _, p := range j.Penanda {
		if !ada[p.Nama] {
			res.TidakDipakai = append(res.TidakDipakai, p.Nama)
		}
	}
	if _, err := dryRun(jenis, berkas); err != nil {
		var ev *ErrValidasi
		if errors.As(err, &ev) {
			return nil, err
		}
		return nil, validasi("Struktur template tidak sesuai: %s", err.Error())
	}
	if len(res.TidakDikenal) > 0 {
		res.Peringatan = append(res.Peringatan, fmt.Sprintf("%d penanda tidak dikenal akan dibiarkan apa adanya pada dokumen hasil", len(res.TidakDikenal)))
	}
	info, err := l.Repo.SimpanTemplate(ctx, TemplateBaru{Kunci: jenis, NamaFile: namaFile, Berkas: berkas, Catatan: catatan, Oleh: id.Nama})
	if err != nil {
		return nil, err
	}
	res.Template = info
	return res, nil
}

// UnduhTemplate mengembalikan template yang sedang dipakai (unggahan terbaru, atau bawaan aplikasi).
func (l *Layanan) UnduhTemplate(ctx context.Context, id Identitas, jenis string) (nama string, berkas []byte, err error) {
	if err := l.khususAdmin(id); err != nil {
		return "", nil, err
	}
	if _, ok := JenisByKunci(jenis); !ok {
		return "", nil, ErrTidakDitemukan
	}
	b, info, err := l.Repo.TemplateAktif(ctx, jenis)
	if err != nil {
		return "", nil, err
	}
	if len(b) > 0 && info != nil {
		return info.NamaFile, b, nil
	}
	if b, ok := TemplateBawaan(jenis); ok {
		return "Template " + jenis + " (bawaan).docx", b, nil
	}
	return "", nil, ErrTemplateTidakAda
}
