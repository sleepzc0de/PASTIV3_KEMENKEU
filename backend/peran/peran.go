// Paket peran: peran data pengguna (Super Admin, Pengguna Barang, UE1, Kanwil, Satker, atau tanpa peran = tamu) dan cakupan data yang menyertainya.
//
// Kode satker lengkap (mis. 015040199119091000KP) memuat seluruh tingkat organisasi:
//
//	karakter 1-5   : UE1     (015 + 04)           -> peran UE1,   kode 5 digit
//	karakter 1-9   : Kanwil  (UE1 + 4 digit)      -> peran Kanwil, kode 9 digit
//	karakter 10-15 : satker                       -> peran Satker, kode 6 digit
//
// Seorang pengguna boleh memegang banyak peran tetapi bertindak sebagai satu peran dalam satu waktu (peran aktif). Peran aktif menentukan
// cakupan data yang boleh dilihat. Superadmin (ditetapkan di .env) tanpa peran aktif memakai peran bawaan akunnya dan melihat seluruh data. Pengguna tanpa peran apa
// pun adalah tamu: tidak punya cakupan data sama sekali dan tidak dapat membuka fitur apa pun.
package peran

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// Nilai kolom user_roles.role dan nama peran bawaan akun.
const (
	PenggunaBarang = "pengguna_barang"
	UE1            = "ue1"
	Kanwil         = "kanwil"
	Satker         = "satker"

	AkunSuperadmin = "superadmin"
	AkunUser       = "user"
)

// Panjang kode tiap tingkat.
const (
	PanjangUE1    = 5
	PanjangKanwil = 9
	PanjangSatker = 6
)

// PeranDapatDiberikan: peran yang diberikan lewat user_roles (Super Admin berasal dari akun, bukan dari tabel itu).
var PeranDapatDiberikan = []string{PenggunaBarang, UE1, Kanwil, Satker}

var label = map[string]string{
	AkunSuperadmin: "Super Admin", PenggunaBarang: "Pengguna Barang", UE1: "UE1", Kanwil: "Kanwil", Satker: "Satker", AkunUser: "Pengguna",
}

// Label: nama peran untuk ditampilkan.
func Label(p string) string {
	if l, ok := label[p]; ok {
		return l
	}
	return p
}

var reDigit = regexp.MustCompile(`^[0-9]+$`)

// panjangKode: panjang kode yang diwajibkan tiap peran (0 = tanpa kode).
func panjangKode(p string) (int, bool) {
	switch p {
	case PenggunaBarang:
		return 0, true
	case UE1:
		return PanjangUE1, true
	case Kanwil:
		return PanjangKanwil, true
	case Satker:
		return PanjangSatker, true
	}
	return 0, false
}

// ValidasiPeran memeriksa pasangan peran dan kode yang akan diberikan: peran harus dikenal, Pengguna Barang tanpa kode, dan peran lain dengan kode angka
// sepanjang tingkatnya. Mengembalikan kode yang sudah dirapikan.
func ValidasiPeran(p, kode string) (string, error) {
	n, ok := panjangKode(p)
	if !ok {
		return "", fmt.Errorf("peran %q tidak dikenal", p)
	}
	kode = strings.TrimSpace(kode)
	if n == 0 {
		if kode != "" {
			return "", errors.New("peran Pengguna Barang tidak memakai kode")
		}
		return "", nil
	}
	if len(kode) != n || !reDigit.MatchString(kode) {
		return "", fmt.Errorf("peran %s membutuhkan kode %d digit angka", Label(p), n)
	}
	return kode, nil
}

// Tingkat cakupan data.
type Tingkat string

const (
	Semua       Tingkat = "semua"  // seluruh data
	TingkatUE1  Tingkat = "ue1"    // satker yang kode satkernya diawali kode UE1
	TingkatKwl  Tingkat = "kanwil" // satker yang kode satkernya diawali kode Kanwil
	TingkatSatk Tingkat = "satker" // satu satker (kode 6 digit)
	Kosong      Tingkat = "kosong" // tidak ada data (tamu: pengguna yang belum diberi peran)
)

// Cakupan: data yang boleh dilihat peran aktif.
type Cakupan struct {
	Tingkat Tingkat `json:"tingkat"`
	Kode    string  `json:"kode,omitempty"`
}

var CakupanSemua = Cakupan{Tingkat: Semua}

// SemuaData: cakupan tanpa pembatasan.
func (c Cakupan) SemuaData() bool { return c.Tingkat == "" || c.Tingkat == Semua }

// valid: cakupan terbatas harus berkode angka sepanjang tingkatnya; selain itu dianggap tidak sah (diperlakukan sebagai kosong).
func (c Cakupan) valid() bool {
	switch c.Tingkat {
	case "", Semua, Kosong:
		return true
	case TingkatUE1:
		return len(c.Kode) == PanjangUE1 && reDigit.MatchString(c.Kode)
	case TingkatKwl:
		return len(c.Kode) == PanjangKanwil && reDigit.MatchString(c.Kode)
	case TingkatSatk:
		return len(c.Kode) == PanjangSatker && reDigit.MatchString(c.Kode)
	}
	return false
}

// KondisiSQL: potongan WHERE yang membatasi kolom kode satker lengkap (mis. [Kode_Satker]) ke cakupan ini. Kodenya diperiksa ulang sebagai angka murni,
// sehingga aman ditulis langsung ke SQL (tanpa parameter) dan boleh dipakai di dalam subquery. Kosong/tidak sah menghasilkan "1 = 0"; semua data
// menghasilkan "1 = 1".
func (c Cakupan) KondisiSQL(kolom string) string {
	if !c.valid() {
		return "1 = 0"
	}
	switch c.Tingkat {
	case "", Semua:
		return "1 = 1"
	case TingkatUE1, TingkatKwl:
		return fmt.Sprintf("LEFT(%s, %d) = N'%s'", kolom, len(c.Kode), c.Kode)
	case TingkatSatk:
		return fmt.Sprintf("(LEN(%[1]s) >= 15 AND SUBSTRING(%[1]s, 10, 6) = N'%[2]s')", kolom, c.Kode)
	}
	return "1 = 0"
}

// KondisiSatker6SQL: potongan WHERE yang membatasi kolom kode satker 6 digit (mis. kd_satker_str Inaproc yang sudah dirapikan) ke cakupan ini. Data seperti itu
// tidak memuat UE1 atau Kanwil, jadi tingkat UE1/Kanwil memakai daftar satker data aset: kode 6 digit (karakter ke-10 sampai ke-15) satker yang kode lengkapnya
// berawalan kode cakupan. kolomAset adalah kolom kode satker lengkap pada tabelAset (nama dari kode, bukan masukan pengguna). Satker membandingkan langsung
// dengan kodenya. Kode diperiksa ulang sebagai angka murni (aman ditulis ke SQL tanpa parameter); kosong/tidak sah menghasilkan "1 = 0" dan semua data "1 = 1".
func (c Cakupan) KondisiSatker6SQL(kolom, tabelAset, kolomAset string) string {
	if !c.valid() {
		return "1 = 0"
	}
	switch c.Tingkat {
	case "", Semua:
		return "1 = 1"
	case TingkatSatk:
		return fmt.Sprintf("(%s) = N'%s'", kolom, c.Kode)
	case TingkatUE1, TingkatKwl:
		return fmt.Sprintf("(%[1]s) IN (SELECT SUBSTRING(%[3]s, 10, 6) FROM %[2]s WHERE LEN(%[3]s) >= 15 AND LEFT(%[3]s, %[4]d) = N'%[5]s')", kolom, tabelAset, kolomAset, len(c.Kode), c.Kode)
	}
	return "1 = 0"
}

// BolehKodeSatker: apakah kode satker lengkap (atau awalannya, minimal sepanjang tingkat cakupan) berada dalam cakupan. Padanan KondisiSQL untuk kode
// tunggal di Go.
func (c Cakupan) BolehKodeSatker(kodeSatker string) bool {
	if !c.valid() {
		return false
	}
	switch c.Tingkat {
	case "", Semua:
		return true
	case TingkatUE1, TingkatKwl:
		return strings.HasPrefix(kodeSatker, c.Kode)
	case TingkatSatk:
		return len(kodeSatker) >= 15 && kodeSatker[9:15] == c.Kode
	}
	return false
}

// BolehSatker6: apakah kode satker 6 digit berada dalam cakupan satker (untuk data Inaproc yang hanya punya kode 6 digit, lihat daftarSatker pada
// pemanggil untuk tingkat UE1/Kanwil).
func (c Cakupan) BolehSatker6(kode6 string) bool {
	return c.SemuaData() || (c.Tingkat == TingkatSatk && c.valid() && c.Kode == kode6)
}

// ---------------------------------------------------------------- peran yang dipegang dan peran aktif

// Baris: satu peran yang dipegang pengguna (baris user_roles).
type Baris struct {
	ID    int64  `json:"id"`
	Peran string `json:"role"`
	Kode  string `json:"kode"`
	Aktif bool   `json:"aktif"`
	Label string `json:"label"`
	Oleh  string `json:"dibuat_oleh,omitempty"`
	Pada  string `json:"dibuat_pada,omitempty"`
}

// Efektif: hasil menentukan peran yang sedang berlaku bagi satu permintaan.
type Efektif struct {
	AkunRole string  // users.role
	Role     string  // peran untuk pemeriksaan hak (RequireRole): peran akun bila bertindak sebagai dirinya sendiri, "user" bila bertindak sebagai peran data
	Peran    string  // peran yang tampil: superadmin, pengguna_barang, ue1, kanwil, satker, atau "" (tamu: pengguna tanpa peran)
	PeranID  int64   // id baris user_roles yang berlaku; 0 bila peran bawaan akun
	Kode     string  // kode peran yang berlaku
	Cakupan  Cakupan // data yang boleh dilihat
}

// SemuaData: peran yang berlaku boleh melihat seluruh data.
func (e Efektif) SemuaData() bool { return e.Cakupan.SemuaData() }

// Tamu: pengguna belum punya peran apa pun (dan bukan superadmin), jadi tidak boleh membuka fitur apa pun.
func (e Efektif) Tamu() bool { return e.Peran == "" }

// Selesaikan menentukan peran yang berlaku.
//
//   - Ada peran aktif yang dipilih: itu yang berlaku (superadmin yang bertindak sebagai peran data kehilangan hak superadmin selama itu).
//   - Tanpa pilihan: superadmin memakai peran akunnya; pengguna biasa yang punya peran memakai peran pertamanya.
//   - Pengguna biasa tanpa peran apa pun: tamu, tanpa data (cakupan kosong). Role akun selain superadmin dan user (mis. "admin" lama) diperlakukan seperti user.
func Selesaikan(akunRole string, daftar []Baris) Efektif {
	var dipilih *Baris
	for i := range daftar {
		if daftar[i].Aktif {
			dipilih = &daftar[i]
			break
		}
	}
	istimewa := akunRole == AkunSuperadmin
	if dipilih == nil && !istimewa && len(daftar) > 0 {
		dipilih = &daftar[0]
	}
	if dipilih == nil {
		if istimewa {
			return Efektif{AkunRole: akunRole, Role: akunRole, Peran: akunRole, Cakupan: CakupanSemua}
		}
		return Efektif{AkunRole: akunRole, Role: AkunUser, Peran: "", Cakupan: Cakupan{Tingkat: Kosong}}
	}
	e := Efektif{AkunRole: akunRole, Role: AkunUser, Peran: dipilih.Peran, PeranID: dipilih.ID, Kode: dipilih.Kode}
	switch dipilih.Peran {
	case PenggunaBarang:
		e.Cakupan = CakupanSemua
	case UE1:
		e.Cakupan = Cakupan{Tingkat: TingkatUE1, Kode: dipilih.Kode}
	case Kanwil:
		e.Cakupan = Cakupan{Tingkat: TingkatKwl, Kode: dipilih.Kode}
	case Satker:
		e.Cakupan = Cakupan{Tingkat: TingkatSatk, Kode: dipilih.Kode}
	default:
		e.Cakupan = Cakupan{Tingkat: Kosong} // peran tak dikenal: jangan membuka data
	}
	if !e.Cakupan.valid() {
		e.Cakupan = Cakupan{Tingkat: Kosong}
	}
	return e
}

// ---------------------------------------------------------------- saran dari kode satker SSO

// Saran: peran yang dapat diturunkan dari kode satker pegawai.
type Saran struct {
	Peran string `json:"role"`
	Kode  string `json:"kode"`
	Label string `json:"label"`
}

// SaranDariKodeSatker menurunkan peran UE1, Kanwil, dan Satker dari kode satker lengkap (minimal 15 karakter, 15 karakter pertama angka). Kode lain
// (mis. hanya 6 digit) hanya menghasilkan peran Satker bila persis 6 digit. Kosong bila kodenya tidak dikenali.
func SaranDariKodeSatker(kode string) []Saran {
	kode = strings.TrimSpace(kode)
	if len(kode) == PanjangSatker && reDigit.MatchString(kode) {
		return []Saran{{Peran: Satker, Kode: kode, Label: Label(Satker)}}
	}
	if len(kode) < 15 || !reDigit.MatchString(kode[:15]) {
		return nil
	}
	return []Saran{
		{Peran: UE1, Kode: kode[:PanjangUE1], Label: Label(UE1)},
		{Peran: Kanwil, Kode: kode[:PanjangKanwil], Label: Label(Kanwil)},
		{Peran: Satker, Kode: kode[9:15], Label: Label(Satker)},
	}
}
