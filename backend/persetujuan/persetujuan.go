// Package persetujuan memuat pernyataan penggunaan aplikasi yang harus disetujui setiap pengguna (kecuali superadmin) sebelum memakai aplikasi, dan
// pemeriksaan isian profil akun non-SSO yang diminta bersamaan dengan persetujuan. Fungsinya murni (tanpa database) agar mudah diuji.
package persetujuan

import (
	"net/mail"
	"regexp"
	"strings"
	"unicode"
)

// Versi pernyataan. Naikkan setiap kali isi pernyataan berubah: pengguna yang menyetujui versi lain diminta menyetujui ulang.
const Versi = "1"

// Frasa yang harus diketik pengguna (selain mencentang persetujuan). Dibandingkan tanpa membedakan huruf besar/kecil dan spasi berlebih.
const Frasa = "SAYA SETUJU"

const Judul = "Pernyataan Penggunaan Aplikasi PASTI V3"

// Paragraf pernyataan, ditampilkan berurutan di modal persetujuan.
var Paragraf = []string{
	"Aplikasi PASTI V3 digunakan untuk keperluan kedinasan di lingkungan Kementerian Keuangan. Data aset, pengadaan, dan administrasi pengelolaan BMN di dalamnya bersifat terbatas dan hanya boleh dilihat sesuai peran dan cakupan data yang ditetapkan kepada Anda.",
	"Akun bersifat pribadi. Anda bertanggung jawab atas kerahasiaan akun dan seluruh aktivitas yang dilakukan dengan akun Anda, serta tidak boleh membagikan akun atau kata sandi kepada orang lain.",
	"Data yang dilihat atau diunduh dari aplikasi hanya dipakai untuk pelaksanaan tugas kedinasan dan tidak disebarluaskan kepada pihak yang tidak berwenang.",
	"Data identitas Anda (nama, NIP, dan email) dipakai untuk mengenali Anda dan menetapkan hak akses. Aktivitas penggunaan aplikasi dicatat dan dapat diperiksa untuk keperluan pengamanan dan audit.",
	"Hak akses ditetapkan oleh administrator. Sebelum peran ditetapkan, akun Anda berstatus tamu dan belum dapat membuka fitur apa pun. Pelanggaran terhadap pernyataan ini dapat berakibat pencabutan akses dan tindakan sesuai ketentuan yang berlaku.",
}

// FrasaSah: apakah isian pengguna sama dengan Frasa (tanpa membedakan huruf besar/kecil dan spasi berlebih).
func FrasaSah(s string) bool {
	return strings.EqualFold(strings.Join(strings.Fields(s), " "), Frasa)
}

// Profil: isian akun non-SSO.
type Profil struct {
	Nama  string `json:"nama"`
	NIP   string `json:"nip"`
	Email string `json:"email"`
}

var (
	reNIP18 = regexp.MustCompile(`^[0-9]{18}$`)
	reNIP9  = regexp.MustCompile(`^[0-9]{9}$`)
)

// Rapikan: nama tanpa spasi berlebih, NIP tanpa spasi, email huruf kecil tanpa spasi tepi.
func (p Profil) Rapikan() Profil {
	return Profil{
		Nama:  strings.Join(strings.Fields(p.Nama), " "),
		NIP:   strings.Join(strings.Fields(p.NIP), ""),
		Email: strings.ToLower(strings.TrimSpace(p.Email)),
	}
}

// Validasi memeriksa profil yang sudah dirapikan; hasilnya pesan galat per isian (kosong = sah).
func (p Profil) Validasi() map[string]string {
	g := map[string]string{}
	switch n := len([]rune(p.Nama)); {
	case n < 3:
		g["nama"] = "Nama lengkap wajib diisi (minimal 3 karakter)"
	case n > 100:
		g["nama"] = "Nama lengkap maksimal 100 karakter"
	case strings.IndexFunc(p.Nama, unicode.IsControl) >= 0:
		g["nama"] = "Nama lengkap memuat karakter yang tidak diizinkan"
	}
	if !reNIP18.MatchString(p.NIP) && !reNIP9.MatchString(p.NIP) {
		g["nip"] = "NIP wajib diisi dengan 18 digit angka (atau 9 digit untuk NIP lama)"
	}
	if !EmailSah(p.Email) {
		g["email"] = "Email wajib diisi dengan alamat email yang sah (email kedinasan atau pribadi)"
	}
	return g
}

// EmailSah: alamat email biasa (tanpa nama tampilan atau tanda kurung), maksimal 100 karakter (lebar kolom users.email), dengan domain bertitik.
func EmailSah(email string) bool {
	if email == "" || len(email) > 100 {
		return false
	}
	a, err := mail.ParseAddress(email)
	if err != nil || a.Address != email || a.Name != "" {
		return false
	}
	at := strings.LastIndex(email, "@")
	domain := email[at+1:]
	return at > 0 && strings.Contains(domain, ".") && !strings.HasPrefix(domain, ".") && !strings.HasSuffix(domain, ".")
}
