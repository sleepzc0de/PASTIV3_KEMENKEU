package dto

import "pasti-v3-backend/peran"

type CreateUserRequest struct {
	Source   string `json:"source" binding:"required,oneof=hris2 manual"`
	NIP      string `json:"nip"`
	Username string `json:"username" binding:"required,min=3,max=50"`
	Password string `json:"password" binding:"required,min=8"`
	Email    string `json:"email"`
	FullName string `json:"full_name"`
	Role     string `json:"role" binding:"omitempty,oneof=user"` // hanya "user" (atau kosong): superadmin ditetapkan di .env, admin lama ditiadakan
}

// UpdateUserRequest untuk edit data user yang sudah ada.
// Password bersifat opsional — kalau dikosongkan, password lama tidak berubah.
type UpdateUserRequest struct {
	FullName string `json:"full_name" binding:"required"`
	Email    string `json:"email" binding:"required,email"`
	Role     string `json:"role" binding:"omitempty,oneof=user"` // diabaikan: role akun tidak dapat diubah lewat API
	IsActive bool   `json:"is_active"`
	Password string `json:"password"` // opsional
}

type UserListItem struct {
	ID           string  `json:"id"`
	Username     string  `json:"username"`
	Email        string  `json:"email"`
	FullName     string  `json:"full_name"`
	Role         string  `json:"role"`
	IsActive     bool    `json:"is_active"`
	AuthProvider string  `json:"auth_provider"`
	IsProtected  bool    `json:"is_protected"`
	NIP          *string `json:"nip"`
	Jabatan      *string `json:"jabatan"`
	Satker       *string `json:"satker"`
	CreatedAt    string  `json:"created_at"`

	KodeSatker *string       `json:"kode_satker"` // kode satker lengkap dari SSO Kemenkeu (kosong untuk akun non-SSO)
	SatkerAset *string       `json:"satker_aset"` // nama satker pada data Digitalisasi Aset yang kode 6 digitnya (karakter ke-10 sampai ke-15) sama dengan kode satker SSO
	PeranData  []peran.Baris `json:"peran_data"`
	Tamu       bool          `json:"tamu"`   // belum punya peran apa pun (dan bukan superadmin)
	Setuju     bool          `json:"setuju"` // sudah menyetujui pernyataan penggunaan aplikasi versi terbaru
}
