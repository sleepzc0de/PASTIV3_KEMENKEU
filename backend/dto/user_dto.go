package dto

type CreateUserRequest struct {
	Source   string `json:"source" binding:"required,oneof=hris2 manual"`
	NIP      string `json:"nip"` // wajib kalau source=hris2
	Username string `json:"username" binding:"required,min=3,max=50"`
	Password string `json:"password" binding:"required,min=8"`
	Email    string `json:"email"`     // wajib kalau source=manual, opsional (override) kalau hris2
	FullName string `json:"full_name"` // wajib kalau source=manual, opsional (override) kalau hris2
	Role     string `json:"role" binding:"required,oneof=user admin"`
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
}
