package main

import (
	"fmt"
	"log"

	"github.com/google/uuid"

	"pasti-v3-backend/config"
	"pasti-v3-backend/database"
	"pasti-v3-backend/persetujuan"
	"pasti-v3-backend/utils"
)

type seedUser struct {
	Username string
	Password string
	Email    string
	FullName string
	Role     string // role akun: hanya "user" (superadmin ditetapkan di .env dan hanya lewat SSO)
	NIP      string
	Peran    string // peran data awal ("pengguna_barang" atau kosong = tamu)
}

func main() {
	config.LoadConfig()
	database.Connect()

	users := []seedUser{
		{
			Username: "ROMADAN_015",
			Password: "U2FsdGVkX18BbKc/pqz9aCL+lnR/LMyuy5zomSGffLaAs8/MHYD5hFXyyY+j3Bt+",
			Email:    "romadan.015@pasti.kemenkeu.go.id",
			FullName: "Romadan (Pengguna Barang Dummy)",
			Role:     "user",
			NIP:      "199001012015011001",
			Peran:    "pengguna_barang",
		},
		{
			Username: "USER_015",
			Password: "U2FsdGVkX18Clm6NLd+yDVH7Tv70ro7HuFR7b6LTawtqfVqOnaLtZPvThYBYksSd",
			Email:    "user.015@pasti.kemenkeu.go.id",
			FullName: "User Dummy 015",
			Role:     "user",
			NIP:      "199001012015011002",
		},
	}

	for _, u := range users {
		if err := upsertSeedUser(u); err != nil {
			log.Printf("[SEED ERROR] Gagal membuat user %s: %v\n", u.Username, err)
			continue
		}
		fmt.Printf("[SEED SUCCESS] User %s (%s) berhasil dibuat/diperbarui\n", u.Username, u.Role)
	}
}

func upsertSeedUser(u seedUser) error {
	hash, salt, err := utils.HashPassword(u.Password)
	if err != nil {
		return fmt.Errorf("gagal hash password: %w", err)
	}

	var existingID string
	err = database.DB.QueryRow(
		`SELECT id FROM users WHERE username = @p1`, u.Username,
	).Scan(&existingID)

	if err != nil {
		// User belum ada -> insert baru
		newID := uuid.New().String()
		_, err = database.DB.Exec(
			`INSERT INTO users (id, username, email, password_hash, password_salt, full_name, role, is_active, auth_provider)
			 VALUES (@p1, @p2, @p3, @p4, @p5, @p6, @p7, 1, 'local')`,
			newID, u.Username, u.Email, hash, salt, u.FullName, u.Role,
		)
		if err != nil {
			return err
		}
	} else {
		// User sudah ada -> update password & data
		_, err = database.DB.Exec(
			`UPDATE users SET password_hash=@p1, password_salt=@p2, email=@p3, full_name=@p4, role=@p5, is_active=1
			 WHERE username=@p6`,
			hash, salt, u.Email, u.FullName, u.Role, u.Username,
		)
		if err != nil {
			return err
		}
	}

	// Akun dummy untuk pengembangan: dianggap sudah menyetujui pernyataan (kalau tidak, hanya modal persetujuan yang tampil), dan diberi peran data bila ada.
	// Pengguna tanpa peran adalah tamu.
	if _, err = database.DB.Exec(
		`UPDATE users SET nip=@p1, persetujuan_at=COALESCE(persetujuan_at, SYSUTCDATETIME()), persetujuan_versi=@p2 WHERE username=@p3`,
		u.NIP, persetujuan.Versi, u.Username,
	); err != nil {
		return err
	}
	if u.Peran != "" {
		_, err = database.DB.Exec(
			`INSERT INTO user_roles (user_id, role, kode, dibuat_oleh)
			 SELECT id, @p1, N'', N'seed' FROM users WHERE username=@p2
			   AND NOT EXISTS (SELECT 1 FROM user_roles r WHERE r.user_id = users.id AND r.role = @p1 AND r.kode = N'')`,
			u.Peran, u.Username,
		)
	}
	return err
}
