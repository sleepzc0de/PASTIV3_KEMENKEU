package audit

import (
	"net/url"
	"regexp"
	"sort"
	"strings"
)

// Aturan menerangkan satu rute: aksi dan uraian yang dicatat. Rute yang diubah datanya (POST, PUT, PATCH, DELETE) dicatat walau tidak ada di katalog (dengan aksi
// "METHOD rute" dan kategori dari awalan rute); katalog memberi uraian yang mudah dibaca dan menetapkan GET yang sengaja dicatat (Baca): unduhan, ekspor, dan
// pencarian data pegawai, yang termasuk akses data sensitif.
type Aturan struct {
	Kategori   string
	Aksi       string
	Label      string // boleh memuat {nama_parameter} yang diganti nilai parameter rute
	ObjekTipe  string
	ObjekParam string // nama parameter rute yang menjadi objek_id
	Baca       bool   // GET yang dicatat
	Kueri      bool   // sertakan kueri (sudah dibersihkan dari token/kode/sandi) pada rincian
}

const awalan = "/api/v1"

// katalog: kunci "METHOD pola-rute" (c.FullPath()).
var katalog = map[string]Aturan{
	// ---- akun dan sesi
	"POST " + awalan + "/auth/login":       {KatAuth, AksiLoginBerhasil, "Login dengan kata sandi", "", "", false, false},
	"POST " + awalan + "/auth/register":    {KatAuth, "auth.daftar", "Mendaftarkan akun baru", "pengguna", "", false, false},
	"POST " + awalan + "/auth/persetujuan": {KatAuth, "auth.persetujuan", "Menyetujui pernyataan penggunaan aplikasi", "", "", false, false},
	"POST " + awalan + "/auth/peran/aktif": {KatPeran, "peran.pindah", "Berpindah peran aktif", "peran", "", false, false},

	// ---- pengguna dan peran
	"POST " + awalan + "/users":                      {KatPengguna, "pengguna.buat", "Membuat pengguna", "pengguna", "", false, false},
	"PUT " + awalan + "/users/:id":                   {KatPengguna, "pengguna.ubah", "Mengubah pengguna {id}", "pengguna", "id", false, false},
	"PUT " + awalan + "/users/:id/deactivate":        {KatPengguna, "pengguna.nonaktifkan", "Menonaktifkan atau mengaktifkan pengguna {id}", "pengguna", "id", false, false},
	"DELETE " + awalan + "/users/:id":                {KatPengguna, "pengguna.hapus", "Menghapus pengguna {id}", "pengguna", "id", false, false},
	"POST " + awalan + "/users/:id/peran":            {KatPeran, "peran.beri", "Memberi peran kepada pengguna {id}", "pengguna", "id", false, false},
	"DELETE " + awalan + "/users/:id/peran/:peranId": {KatPeran, "peran.cabut", "Mencabut peran dari pengguna {id}", "pengguna", "id", false, false},

	// ---- Digitalisasi Aset
	"POST " + awalan + "/digitalisasi/sinkronisasi":       {KatDigitalisasi, "digitalisasi.sinkron.mulai", "Memulai sinkronisasi data aset dari SLDK", "", "", false, false},
	"POST " + awalan + "/digitalisasi/sinkronisasi/batal": {KatDigitalisasi, "digitalisasi.sinkron.batal", "Membatalkan sinkronisasi data aset", "", "", false, false},
	"GET " + awalan + "/digitalisasi/ekspor/:dataset":     {KatEkspor, "ekspor.digitalisasi", "Mengekspor data aset ({dataset})", "dataset", "dataset", true, true},

	// ---- SAPA: usulan penjualan
	"POST " + awalan + "/sapa/penjualan":                             {KatSapa, "sapa.usulan.buat", "Membuat usulan penjualan", "usulan", "", false, false},
	"DELETE " + awalan + "/sapa/penjualan/:id":                       {KatSapa, "sapa.usulan.hapus", "Menghapus usulan penjualan {id}", "usulan", "id", false, false},
	"PUT " + awalan + "/sapa/penjualan/:id/tahap/:kunci":             {KatSapa, "sapa.tahap.simpan", "Menyimpan draf tahap {kunci} usulan {id}", "usulan", "id", false, false},
	"POST " + awalan + "/sapa/penjualan/:id/tahap/:kunci/dokumen":    {KatSapa, "sapa.dokumen.buat", "Membuat dokumen tahap {kunci} usulan {id}", "usulan", "id", false, false},
	"POST " + awalan + "/sapa/penjualan/:id/tahap/:kunci/selesai":    {KatSapa, "sapa.tahap.selesai", "Menyelesaikan tahap {kunci} usulan {id}", "usulan", "id", false, false},
	"POST " + awalan + "/sapa/penjualan/:id/tahap/:kunci/lewati":     {KatSapa, "sapa.tahap.lewati", "Melewati tahap {kunci} usulan {id}", "usulan", "id", false, false},
	"PUT " + awalan + "/sapa/penjualan/:id/tahap/:kunci/keterangan":  {KatSapa, "sapa.tahap.keterangan", "Mengubah keterangan tahap {kunci} usulan {id}", "usulan", "id", false, false},
	"POST " + awalan + "/sapa/penjualan/:id/tahap/:kunci/buka-ulang": {KatSapa, "sapa.tahap.buka_ulang", "Membuka ulang tahap {kunci} usulan {id}", "usulan", "id", false, false},
	"GET " + awalan + "/sapa/dokumen/:id/unduh":                      {KatEkspor, "ekspor.sapa.dokumen", "Mengunduh dokumen SAPA {id}", "dokumen", "id", true, false},
	"POST " + awalan + "/sapa/barang/impor":                          {KatSapa, "sapa.barang.impor", "Mengunggah daftar barang dari Excel", "", "", false, false},
	"POST " + awalan + "/sapa/barang/impor-siman":                    {KatSapa, "sapa.barang.impor_siman", "Mengunggah daftar barang dari data SIMAN", "", "", false, false},
	"POST " + awalan + "/sapa/barang/ekspor":                         {KatEkspor, "ekspor.sapa.barang", "Mengunduh daftar barang SAPA ke Excel", "", "", false, false},
	"GET " + awalan + "/sapa/pegawai":                                {KatHRIS2, "hris2.cari", "Mencari pegawai HRIS2 dari SAPA", "", "", true, true},
	"GET " + awalan + "/sapa/pegawai/:nip":                           {KatHRIS2, "hris2.lihat", "Melihat data pegawai HRIS2 dari SAPA", "pegawai", "nip", true, false},

	// ---- SAPA: pengaturan superadmin
	"POST " + awalan + "/sapa/template/:kunci":      {KatSapa, "sapa.template.unggah", "Mengunggah template dokumen {kunci}", "template", "kunci", false, false},
	"GET " + awalan + "/sapa/template/:kunci/unduh": {KatEkspor, "ekspor.sapa.template", "Mengunduh template dokumen {kunci}", "template", "kunci", true, false},
	"PUT " + awalan + "/sapa/bmn/satuan":            {KatSapa, "sapa.bmn.satuan.simpan", "Menyimpan satuan BMN", "", "", false, false},
	"DELETE " + awalan + "/sapa/bmn/satuan":         {KatSapa, "sapa.bmn.satuan.hapus", "Menghapus satuan BMN", "", "", false, false},
	"PUT " + awalan + "/sapa/bmn/jenis":             {KatSapa, "sapa.bmn.jenis.simpan", "Menyimpan jenis BMN", "", "", false, false},
	"DELETE " + awalan + "/sapa/bmn/jenis":          {KatSapa, "sapa.bmn.jenis.hapus", "Menghapus jenis BMN", "", "", false, false},
	"PUT " + awalan + "/sapa/ref-ue1/:kode":         {KatReferensi, "referensi.ue1.sapa.simpan", "Menyimpan sebutan Sekretaris UE1 {kode}", "ue1", "kode", false, false},
	"DELETE " + awalan + "/sapa/ref-ue1/:kode":      {KatReferensi, "referensi.ue1.sapa.hapus", "Menghapus sebutan Sekretaris UE1 {kode}", "ue1", "kode", false, false},

	// ---- referensi
	"PUT " + awalan + "/referensi/ue1/:kode":           {KatReferensi, "referensi.ue1.simpan", "Menyimpan referensi UE1 {kode}", "ue1", "kode", false, false},
	"DELETE " + awalan + "/referensi/ue1/:kode":        {KatReferensi, "referensi.ue1.hapus", "Menghapus referensi UE1 {kode}", "ue1", "kode", false, false},
	"PUT " + awalan + "/referensi/kanwil/:kode":        {KatReferensi, "referensi.kanwil.simpan", "Menyimpan referensi Kanwil {kode}", "kanwil", "kode", false, false},
	"DELETE " + awalan + "/referensi/kanwil/:kode":     {KatReferensi, "referensi.kanwil.hapus", "Menghapus referensi Kanwil {kode}", "kanwil", "kode", false, false},
	"POST " + awalan + "/referensi/kanwil/dari-satker": {KatReferensi, "referensi.kanwil.dari_satker", "Menambah referensi Kanwil dari data satker", "", "", false, false},
	"POST " + awalan + "/referensi/kanwil/tarik-sldk":  {KatReferensi, "referensi.kanwil.tarik_sldk", "Menarik referensi Kanwil dari SLDK", "", "", false, false},

	// ---- Pengadaan Terpadu (Inaproc)
	"GET " + awalan + "/inaproc/ekspor/:awalan/:nama": {KatEkspor, "ekspor.pengadaan", "Mengekspor data pengadaan ({awalan}/{nama})", "dataset", "nama", true, true},
	"POST " + awalan + "/inaproc/penarikan":           {KatPengadaan, "pengadaan.tarik.mulai", "Memulai penarikan data Inaproc", "", "", false, false},
	"POST " + awalan + "/inaproc/penarikan/batal":     {KatPengadaan, "pengadaan.tarik.batal", "Membatalkan penarikan data Inaproc", "", "", false, false},
	"PUT " + awalan + "/inaproc/penarikan/pengaturan": {KatPengadaan, "pengadaan.tarik.pengaturan", "Mengubah pengaturan penarikan data Inaproc", "", "", false, false},

	// ---- HRIS2 (khusus superadmin)
	"GET " + awalan + "/hris2/pegawai/search":      {KatHRIS2, "hris2.cari", "Mencari pegawai di HRIS2", "", "", true, true},
	"GET " + awalan + "/hris2/pegawai/by-nip/:nip": {KatHRIS2, "hris2.lihat", "Melihat data pegawai HRIS2", "pegawai", "nip", true, false},

	// ---- audit itu sendiri
	"GET " + awalan + "/audit/ekspor": {KatAudit, "audit.ekspor", "Mengekspor log audit ke CSV", "", "", true, true},
}

// Label kategori untuk tampilan.
var labelKategori = map[string]string{
	KatAuth: "Akun dan login", KatPengguna: "Pengguna", KatPeran: "Peran", KatSapa: "SAPA", KatDigitalisasi: "Digitalisasi Aset", KatPengadaan: "Pengadaan",
	KatReferensi: "Referensi", KatEkspor: "Ekspor dan unduhan", KatHRIS2: "Data pegawai (HRIS2)", KatAudit: "Log audit", KatLainnya: "Lainnya",
}

// urutanKategori: urutan tampil di filter.
var urutanKategori = []string{KatAuth, KatPengguna, KatPeran, KatSapa, KatDigitalisasi, KatPengadaan, KatReferensi, KatEkspor, KatHRIS2, KatAudit, KatLainnya}

// OpsiKategori: pilihan kategori untuk filter.
type OpsiKategori struct {
	Kode  string `json:"kode"`
	Label string `json:"label"`
}

// OpsiAksi: pilihan aksi untuk filter.
type OpsiAksi struct {
	Kode     string `json:"kode"`
	Label    string `json:"label"`
	Kategori string `json:"kategori"`
}

// Opsi: kategori dan aksi yang dikenal (untuk filter di halaman log audit).
type Opsi struct {
	Kategori []OpsiKategori `json:"kategori"`
	Aksi     []OpsiAksi     `json:"aksi"`
}

// DaftarOpsi menyusun pilihan filter dari katalog (tanpa membaca database).
func DaftarOpsi() Opsi {
	o := Opsi{}
	for _, k := range urutanKategori {
		o.Kategori = append(o.Kategori, OpsiKategori{Kode: k, Label: labelKategori[k]})
	}
	lihat := map[string]bool{}
	tambah := func(kode, label, kat string) {
		if lihat[kode] {
			return
		}
		lihat[kode] = true
		o.Aksi = append(o.Aksi, OpsiAksi{Kode: kode, Label: label, Kategori: kat})
	}
	tambah(AksiLoginBerhasil, "Login berhasil", KatAuth)
	tambah(AksiLoginGagal, "Login gagal", KatAuth)
	tambah(AksiDitolak, "Akses ditolak", KatLainnya)
	for _, a := range katalog {
		tambah(a.Aksi, labelUmum(a.Label), a.Kategori)
	}
	sort.Slice(o.Aksi, func(i, j int) bool { return o.Aksi[i].Kode < o.Aksi[j].Kode })
	return o
}

// KunciKatalog: semua kunci "METHOD rute" pada katalog (untuk tes yang memeriksa katalog tetap sesuai tabel rute).
func KunciKatalog() []string {
	out := make([]string, 0, len(katalog))
	for k := range katalog {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// KategoriDariRute menentukan kategori dari awalan rute untuk rute yang tidak ada di katalog.
func KategoriDariRute(rute string) string {
	p := strings.TrimPrefix(rute, awalan)
	p = strings.TrimPrefix(p, "/")
	seg := p
	if i := strings.IndexByte(p, '/'); i >= 0 {
		seg = p[:i]
	}
	switch {
	case strings.HasPrefix(rute, "/sso/"), seg == "auth":
		return KatAuth
	case seg == "users":
		return KatPengguna
	case seg == "digitalisasi":
		return KatDigitalisasi
	case seg == "sapa":
		return KatSapa
	case seg == "hris2":
		return KatHRIS2
	case seg == "referensi":
		return KatReferensi
	case seg == "inaproc":
		return KatPengadaan
	case seg == "audit":
		return KatAudit
	}
	return KatLainnya
}

var reParameter = regexp.MustCompile(`\{([a-zA-Z]+)\}`)

var reKurungKosong = regexp.MustCompile(`\s*\([\s/]*\)`)

// labelUmum: label aturan tanpa nilai parameter ("Mengubah pengguna {id}" -> "Mengubah pengguna"), untuk daftar pilihan aksi dan ringkasan.
func labelUmum(label string) string {
	s := reParameter.ReplaceAllString(label, "")
	s = reKurungKosong.ReplaceAllString(s, "")
	return strings.Join(strings.Fields(s), " ")
}

// isiLabel mengganti {nama} pada label dengan nilai parameter rute (dipotong supaya label tetap pendek).
func isiLabel(label string, param map[string]string) string {
	return reParameter.ReplaceAllStringFunc(label, func(m string) string {
		v := param[m[1:len(m)-1]]
		if len(v) > 60 {
			v = v[:60] + "…"
		}
		return v
	})
}

var reKueriRahasia = regexp.MustCompile(`(?i)^(access_token|id_token|refresh_token|token|code|secret|client_secret|password|passwd|sandi|kata_sandi|api_key|apikey|key|authorization|state)$`)

// bersihkanKueri membuang pasangan kueri yang berpotensi rahasia (token, kode OAuth, sandi, kunci) dan memotong hasilnya.
func bersihkanKueri(raw string) string {
	if raw == "" {
		return ""
	}
	v, err := url.ParseQuery(raw)
	if err != nil {
		return ""
	}
	for k := range v {
		if reKueriRahasia.MatchString(k) {
			v.Del(k)
		}
	}
	s := v.Encode() // diurutkan menurut kunci
	if len(s) > 300 {
		s = s[:300] + "…"
	}
	return s
}
