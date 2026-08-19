package handlers

import (
	"database/sql"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	mssql "github.com/microsoft/go-mssqldb"

	"pasti-v3-backend/database"
	"pasti-v3-backend/dto"
	"pasti-v3-backend/utils"
)

// ListUsers mengembalikan daftar semua user (admin/superadmin only)
func ListUsers(c *gin.Context) {
	rows, err := database.DB.Query(`
		SELECT u.id, u.username, u.email, u.full_name, u.role, u.is_active,
		       u.auth_provider, u.is_protected, e.nip, e.jabatan, e.satker, u.created_at
		FROM users u
		LEFT JOIN employees e ON e.id = u.employee_id
		ORDER BY u.created_at DESC`)
	if err != nil {
		utils.ErrorResponse(c, http.StatusInternalServerError, "Gagal mengambil daftar user")
		return
	}
	defer rows.Close()

	var users []dto.UserListItem
	for rows.Next() {
		var idRaw mssql.UniqueIdentifier
		var u dto.UserListItem
		var nip, jabatan, satker sql.NullString
		var createdAt sql.NullTime

		if err := rows.Scan(&idRaw, &u.Username, &u.Email, &u.FullName, &u.Role, &u.IsActive,
			&u.AuthProvider, &u.IsProtected, &nip, &jabatan, &satker, &createdAt); err != nil {
			continue
		}

		u.ID = idRaw.String()
		if nip.Valid {
			u.NIP = &nip.String
		}
		if jabatan.Valid {
			u.Jabatan = &jabatan.String
		}
		if satker.Valid {
			u.Satker = &satker.String
		}
		if createdAt.Valid {
			u.CreatedAt = createdAt.Time.Format("2006-01-02 15:04")
		}

		users = append(users, u)
	}

	utils.SuccessResponse(c, http.StatusOK, "Berhasil mengambil daftar user", users)
}

// CreateUser membuat akun baru — bisa dari data HRIS2 (by NIP) atau input manual.
func CreateUser(c *gin.Context) {
	var req dto.CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, "Data tidak valid: "+err.Error())
		return
	}

	// Cegah eskalasi privilege — pembuatan akun lewat fitur ini tidak boleh
	// langsung membuat superadmin.
	if req.Role != "user" && req.Role != "admin" {
		utils.ErrorResponse(c, http.StatusBadRequest, "Role tidak valid")
		return
	}

	fullName := req.FullName
	email := req.Email
	var employeeID *string

	if req.Source == "hris2" {
		if req.NIP == "" {
			utils.ErrorResponse(c, http.StatusBadRequest, "NIP wajib diisi untuk pendaftaran via HRIS2")
			return
		}

		adminUserID := c.GetString("user_id")
		accessToken, err := getValidAccessToken(adminUserID)
		if err != nil {
			utils.ErrorResponse(c, http.StatusUnauthorized, "Sesi SSO Anda telah berakhir, silakan login ulang via SSO untuk memakai fitur ini")
			return
		}

		profile, err := fetchPegawaiByNIP(accessToken, req.NIP)
		if err != nil {
			utils.ErrorResponse(c, http.StatusBadGateway, "Gagal memverifikasi NIP ke HRIS2: "+err.Error())
			return
		}
		if profile == nil {
			utils.ErrorResponse(c, http.StatusNotFound, "NIP tidak ditemukan di HRIS2")
			return
		}

		hrisName := getStringField(profile, "nama")
		hrisEmail := getStringField(profile, "email")
		hrisJabatan := getJabatanAktif(profile)
		hrisSatker := getStringField(profile, "namaSatker")
		hrisKdSatker := getStringField(profile, "kdSatker")

		if fullName == "" {
			fullName = hrisName
		}
		if email == "" {
			email = hrisEmail
		}
		if fullName == "" {
			utils.ErrorResponse(c, http.StatusBadRequest, "Nama pegawai tidak ditemukan di data HRIS2, isi nama lengkap secara manual")
			return
		}
		if email == "" {
			utils.ErrorResponse(c, http.StatusBadRequest, "Email pegawai tidak ditemukan di data HRIS2, isi email secara manual")
			return
		}

		empID, err := upsertEmployeeFromHRIS2(req.NIP, fullName, email, hrisJabatan, hrisSatker, hrisKdSatker)
		if err != nil {
			utils.ErrorResponse(c, http.StatusInternalServerError, "Gagal menyimpan data pegawai: "+err.Error())
			return
		}
		employeeID = &empID
	} else {
		if fullName == "" || email == "" {
			utils.ErrorResponse(c, http.StatusBadRequest, "Nama lengkap dan email wajib diisi untuk pendaftaran manual")
			return
		}
	}

	var exists int
	database.DB.QueryRow(
		`SELECT COUNT(1) FROM users WHERE username = @p1 OR email = @p2`,
		req.Username, email,
	).Scan(&exists)
	if exists > 0 {
		utils.ErrorResponse(c, http.StatusConflict, "Username atau email sudah terdaftar")
		return
	}

	hash, salt, err := utils.HashPassword(req.Password)
	if err != nil {
		utils.ErrorResponse(c, http.StatusInternalServerError, "Gagal memproses password")
		return
	}

	newID := uuid.New().String()

	if employeeID != nil {
		_, err = database.DB.Exec(`
			INSERT INTO users (id, username, email, password_hash, password_salt, full_name, role, is_active, auth_provider, employee_id)
			VALUES (@p1, @p2, @p3, @p4, @p5, @p6, @p7, 1, 'local', @p8)`,
			newID, req.Username, email, hash, salt, fullName, req.Role, *employeeID,
		)
	} else {
		_, err = database.DB.Exec(`
			INSERT INTO users (id, username, email, password_hash, password_salt, full_name, role, is_active, auth_provider)
			VALUES (@p1, @p2, @p3, @p4, @p5, @p6, @p7, 1, 'local')`,
			newID, req.Username, email, hash, salt, fullName, req.Role,
		)
	}
	if err != nil {
		utils.ErrorResponse(c, http.StatusInternalServerError, "Gagal membuat user baru")
		return
	}

	utils.SuccessResponse(c, http.StatusCreated, "User berhasil dibuat", gin.H{
		"id":       newID,
		"username": req.Username,
	})
}

// upsertEmployeeFromHRIS2 menyimpan data pegawai hasil lookup NIP ke tabel
// employees, dengan sso_sub placeholder ("hris2-manual:<nip>") karena
// belum tentu pegawai ini sudah pernah login via SSO.
// Catatan: kalau pegawai ini nanti login SSO sendiri, sistem akan membuat
// baris employee terpisah (sso_sub asli beda dari placeholder ini) —
// keterbatasan yang bisa disatukan nanti lewat fitur merge manual bila diperlukan.
func upsertEmployeeFromHRIS2(nip, name, email, jabatan, satker, kdSatker string) (string, error) {
	placeholderSub := "hris2-manual:" + nip

	var existingIDRaw mssql.UniqueIdentifier
	err := database.DB.QueryRow(`SELECT id FROM employees WHERE sso_sub = @p1 OR nip = @p2`, placeholderSub, nip).Scan(&existingIDRaw)

	if err == sql.ErrNoRows {
		newID := uuid.New().String()
		_, err = database.DB.Exec(`
			INSERT INTO employees (id, sso_sub, nip, name, email, jabatan, satker, kode_satker)
			VALUES (@p1, @p2, @p3, @p4, @p5, @p6, @p7, @p8)`,
			newID, placeholderSub, nip, name, email, jabatan, satker, kdSatker,
		)
		if err != nil {
			return "", fmt.Errorf("insert employee gagal: %w", err)
		}
		return newID, nil
	} else if err != nil {
		return "", fmt.Errorf("query cek employee gagal: %w", err)
	}

	existingID := existingIDRaw.String()
	database.DB.Exec(`
		UPDATE employees SET name=@p1, email=@p2, jabatan=@p3, satker=@p4, kode_satker=@p5, updated_at=SYSUTCDATETIME()
		WHERE id=@p6`,
		name, email, jabatan, satker, kdSatker, existingID,
	)
	return existingID, nil
}

// ============ Handler proteksi superadmin (sudah ada sebelumnya) ============

func UpdateUserRole(c *gin.Context) {
	targetID := c.Param("id")
	var body struct {
		Role string `json:"role" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, "Data tidak valid")
		return
	}

	var isProtected bool
	err := database.DB.QueryRow(`SELECT is_protected FROM users WHERE id = @p1`, targetID).Scan(&isProtected)
	if err == sql.ErrNoRows {
		utils.ErrorResponse(c, http.StatusNotFound, "User tidak ditemukan")
		return
	} else if err != nil {
		utils.ErrorResponse(c, http.StatusInternalServerError, "Terjadi kesalahan server")
		return
	}

	if isProtected {
		utils.ErrorResponse(c, http.StatusForbidden, "Akun superadmin permanen ini tidak dapat diubah rolenya")
		return
	}

	_, err = database.DB.Exec(`UPDATE users SET role=@p1, updated_at=SYSUTCDATETIME() WHERE id=@p2`, body.Role, targetID)
	if err != nil {
		utils.ErrorResponse(c, http.StatusInternalServerError, "Gagal memperbarui role")
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Role berhasil diperbarui", nil)
}

func DeleteUser(c *gin.Context) {
	targetID := c.Param("id")

	var isProtected bool
	err := database.DB.QueryRow(`SELECT is_protected FROM users WHERE id = @p1`, targetID).Scan(&isProtected)
	if err == sql.ErrNoRows {
		utils.ErrorResponse(c, http.StatusNotFound, "User tidak ditemukan")
		return
	} else if err != nil {
		utils.ErrorResponse(c, http.StatusInternalServerError, "Terjadi kesalahan server")
		return
	}

	if isProtected {
		utils.ErrorResponse(c, http.StatusForbidden, "Akun superadmin permanen ini tidak dapat dihapus")
		return
	}

	_, err = database.DB.Exec(`DELETE FROM users WHERE id = @p1`, targetID)
	if err != nil {
		utils.ErrorResponse(c, http.StatusInternalServerError, "Gagal menghapus user")
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "User berhasil dihapus", nil)
}

func DeactivateUser(c *gin.Context) {
	targetID := c.Param("id")

	var isProtected bool
	err := database.DB.QueryRow(`SELECT is_protected FROM users WHERE id = @p1`, targetID).Scan(&isProtected)
	if err == sql.ErrNoRows {
		utils.ErrorResponse(c, http.StatusNotFound, "User tidak ditemukan")
		return
	} else if err != nil {
		utils.ErrorResponse(c, http.StatusInternalServerError, "Terjadi kesalahan server")
		return
	}

	if isProtected {
		utils.ErrorResponse(c, http.StatusForbidden, "Akun superadmin permanen ini tidak dapat dinonaktifkan")
		return
	}

	_, err = database.DB.Exec(`UPDATE users SET is_active=0, updated_at=SYSUTCDATETIME() WHERE id=@p1`, targetID)
	if err != nil {
		utils.ErrorResponse(c, http.StatusInternalServerError, "Gagal menonaktifkan user")
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "User berhasil dinonaktifkan", nil)
}
