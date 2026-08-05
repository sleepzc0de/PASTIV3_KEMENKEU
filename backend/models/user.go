package models

import "time"

type User struct {
	ID                  string     `json:"id"`
	Username            string     `json:"username"`
	Email               string     `json:"email"`
	PasswordHash        *string    `json:"-"`
	PasswordSalt        *string    `json:"-"`
	FullName            string     `json:"full_name"`
	Role                string     `json:"role"`
	IsActive            bool       `json:"is_active"`
	AuthProvider        string     `json:"auth_provider"` // 'local' | 'sso'
	IsProtected         bool       `json:"is_protected"`  // superadmin permanen
	EmployeeID          *string    `json:"employee_id,omitempty"`
	FailedLoginAttempts int        `json:"-"`
	LockedUntil         *time.Time `json:"-"`
	LastLogin           *time.Time `json:"last_login,omitempty"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}
