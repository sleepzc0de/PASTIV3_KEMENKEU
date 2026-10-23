package handlers

import (
	"errors"
	"strings"

	"github.com/google/uuid"
	mssql "github.com/microsoft/go-mssqldb"

	"pasti-v3-backend/database"
)

// Penautan akun pada login SSO.
//
// Pegawai dikenali lewat sso_sub (ID unik dari penyedia SSO), pengguna lewat employee_id. Penyedia SSO yang berbeda (mis. SSO demo di development dan SSO
// Kemenkeu di production) menerbitkan sub yang berbeda untuk orang yang sama, sehingga pegawai baru dibuat dan tidak ada pengguna yang tertaut ke sana. Email
// (dan username) pengguna itu unik, jadi menambah pengguna baru gagal dengan galat basis data. Karena itu, sebelum menambah, dicari akun yang sudah memakai
// email itu: tepat satu akun SSO ditautkan ke pegawai baru; selain itu (akun lokal, atau lebih dari satu akun) login ditolak dengan pesan yang jelas, tanpa
// mengambil alih akun yang bukan akun SSO.

// errAkunBentrok: email atau username hasil login SSO sudah dipakai akun yang tidak boleh ditautkan otomatis (akun lokal, atau lebih dari satu akun sekaligus).
var errAkunBentrok = errors.New("email atau username sudah dipakai akun lain yang tidak bisa ditautkan otomatis")

// akunSama: satu baris users yang email atau username-nya sama dengan identitas SSO yang sedang login.
type akunSama struct {
	ID           string
	Role         string
	IsActive     bool
	AuthProvider string
}

type aksiTautan int

const (
	tautanBuatBaru aksiTautan = iota // tidak ada akun dengan email/username itu: tambah pengguna baru
	tautanTautkan                    // tepat satu akun SSO: tautkan ke pegawai ini
	tautanBentrok                    // akun lokal, atau lebih dari satu akun: tolak
)

// putusanTautan memutuskan apa yang dilakukan saat login SSO tidak menemukan pengguna yang tertaut ke pegawainya.
func putusanTautan(sama []akunSama) aksiTautan {
	switch len(sama) {
	case 0:
		return tautanBuatBaru
	case 1:
		if strings.EqualFold(sama[0].AuthProvider, "sso") {
			return tautanTautkan
		}
	}
	return tautanBentrok
}

// cariAkunSama mengambil paling banyak dua akun yang username-nya sama dengan username, atau emailnya sama dengan email (bila email tidak kosong). Dua sudah cukup:
// lebih dari satu berarti bentrok.
func cariAkunSama(username, email string) ([]akunSama, error) {
	rows, err := database.DB.Query(
		`SELECT TOP 2 id, role, is_active, auth_provider FROM users WHERE username = @p1 OR (@p2 <> N'' AND email = @p2)`,
		username, email,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var hasil []akunSama
	for rows.Next() {
		var id mssql.UniqueIdentifier
		var a akunSama
		if err := rows.Scan(&id, &a.Role, &a.IsActive, &a.AuthProvider); err != nil {
			return nil, err
		}
		a.ID = id.String()
		hasil = append(hasil, a)
	}
	return hasil, rows.Err()
}

// pesanGalatSesiAkun: pesan untuk layar login bila sesi akun gagal dibuat. Rincian teknis (nama tabel, constraint, data) hanya dicatat di log server; pengguna
// hanya melihat pesan umum dan kode rujukan.
func pesanGalatSesiAkun(err error) string {
	if errors.Is(err, errAkunBentrok) {
		return "Email Anda sudah terdaftar sebagai akun lain di PASTI dan tidak dapat ditautkan otomatis. Hubungi administrator."
	}
	return "Gagal membuat sesi akun. Silakan coba lagi; bila berulang, hubungi administrator."
}

// kodeRujukan: kode pendek yang sama tampil di layar dan tercatat bersama rincian galat di log server, supaya administrator dapat mencocokkannya.
func kodeRujukan() string { return strings.ToUpper(uuid.New().String()[:8]) }
