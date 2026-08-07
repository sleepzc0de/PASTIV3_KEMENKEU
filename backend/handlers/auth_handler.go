package handlers

import (
	"database/sql"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	mssql "github.com/microsoft/go-mssqldb"

	"pasti-v3-backend/database"
	"pasti-v3-backend/dto"
	"pasti-v3-backend/models"
	"pasti-v3-backend/utils"
)

const maxFailedAttempts = 5
const lockDuration = 15 * time.Minute

// Register - membuat user baru (untuk kebutuhan setup awal/testing)
func Register(c *gin.Context) {
	var req dto.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, "Data tidak valid: "+err.Error())
		return
	}

	var exists int
	err := database.DB.QueryRow(
		`SELECT COUNT(1) FROM users WHERE username = @p1 OR email = @p2`,
		req.Username, req.Email,
	).Scan(&exists)
	if err != nil {
		utils.ErrorResponse(c, http.StatusInternalServerError, "Gagal memeriksa data user")
		return
	}
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

	_, err = database.DB.Exec(
		`INSERT INTO users (id, username, email, password_hash, password_salt, full_name, role, is_active)
		 VALUES (@p1, @p2, @p3, @p4, @p5, @p6, 'user', 1)`,
		newID, req.Username, req.Email, hash, salt, req.FullName,
	)
	if err != nil {
		utils.ErrorResponse(c, http.StatusInternalServerError, "Gagal membuat user baru")
		return
	}

	utils.SuccessResponse(c, http.StatusCreated, "Registrasi berhasil", gin.H{
		"id":       newID,
		"username": req.Username,
	})
}

// Login - autentikasi user dengan JWT
func Login(c *gin.Context) {
	var req dto.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, "Username dan password wajib diisi")
		return
	}

	var user models.User
	var idRaw mssql.UniqueIdentifier
	var lockedUntil sql.NullTime
	var passwordHash sql.NullString
	var passwordSalt sql.NullString

	row := database.DB.QueryRow(
		`SELECT id, username, email, password_hash, password_salt, full_name, role,
		        is_active, failed_login_attempts, locked_until
		 FROM users WHERE username = @p1 OR email = @p1`,
		req.Username,
	)

	err := row.Scan(&idRaw, &user.Username, &user.Email, &passwordHash, &passwordSalt,
		&user.FullName, &user.Role, &user.IsActive, &user.FailedLoginAttempts, &lockedUntil)

	if err == sql.ErrNoRows {
		utils.ErrorResponse(c, http.StatusUnauthorized, "Username atau password salah")
		return
	} else if err != nil {
		utils.ErrorResponse(c, http.StatusInternalServerError, "Terjadi kesalahan pada server")
		return
	}

	user.ID = idRaw.String()

	if !user.IsActive {
		utils.ErrorResponse(c, http.StatusForbidden, "Akun tidak aktif, hubungi administrator")
		return
	}

	if lockedUntil.Valid && lockedUntil.Time.After(time.Now()) {
		utils.ErrorResponse(c, http.StatusForbidden, "Akun terkunci sementara akibat percobaan login gagal berulang. Coba lagi nanti")
		return
	}

	if !passwordHash.Valid || !passwordSalt.Valid || passwordHash.String == "" || passwordSalt.String == "" {
		utils.ErrorResponse(c, http.StatusUnauthorized, "Akun ini terdaftar melalui SSO Kemenkeu, silakan login menggunakan tombol SSO")
		return
	}

	if err := utils.VerifyPassword(req.Password, passwordSalt.String, passwordHash.String); err != nil {
		newAttempts := user.FailedLoginAttempts + 1
		if newAttempts >= maxFailedAttempts {
			lockUntil := time.Now().Add(lockDuration)
			database.DB.Exec(
				`UPDATE users SET failed_login_attempts = @p1, locked_until = @p2 WHERE id = @p3`,
				newAttempts, lockUntil, user.ID,
			)
			utils.ErrorResponse(c, http.StatusForbidden, "Terlalu banyak percobaan gagal. Akun dikunci 15 menit")
			return
		}
		database.DB.Exec(`UPDATE users SET failed_login_attempts = @p1 WHERE id = @p2`, newAttempts, user.ID)
		utils.ErrorResponse(c, http.StatusUnauthorized, "Username atau password salah")
		return
	}

	database.DB.Exec(
		`UPDATE users SET failed_login_attempts = 0, locked_until = NULL, last_login = SYSUTCDATETIME() WHERE id = @p1`,
		user.ID,
	)

	accessToken, expiresIn, err := utils.GenerateAccessToken(user.ID, user.Username, user.Role)
	if err != nil {
		utils.ErrorResponse(c, http.StatusInternalServerError, "Gagal membuat token")
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Login berhasil", dto.LoginResponse{
		AccessToken: accessToken,
		ExpiresIn:   expiresIn,
		User: dto.UserProfile{
			ID:       user.ID,
			Username: user.Username,
			Email:    user.Email,
			FullName: user.FullName,
			Role:     user.Role,
		},
	})
}

// Me - ambil data user + profil pegawai (kalau login via SSO)
func Me(c *gin.Context) {
	userID := c.GetString("user_id")

	var (
		idRaw        mssql.UniqueIdentifier
		user         models.User
		employeeJab  sql.NullString
		employeeSat  sql.NullString
		employeeOrg  sql.NullString
		employeeNIP  sql.NullString
		employeePict sql.NullString
	)

	err := database.DB.QueryRow(`
		SELECT u.id, u.username, u.email, u.full_name, u.role, u.auth_provider, u.is_protected,
		       e.jabatan, e.satker, e.organisasi, e.nip, e.picture
		FROM users u
		LEFT JOIN employees e ON e.id = u.employee_id
		WHERE u.id = @p1`, userID,
	).Scan(&idRaw, &user.Username, &user.Email, &user.FullName, &user.Role, &user.AuthProvider, &user.IsProtected,
		&employeeJab, &employeeSat, &employeeOrg, &employeeNIP, &employeePict)

	if err != nil {
		utils.ErrorResponse(c, http.StatusNotFound, "User tidak ditemukan")
		return
	}

	user.ID = idRaw.String()

	utils.SuccessResponse(c, http.StatusOK, "Berhasil mengambil data user", gin.H{
		"id":            user.ID,
		"username":      user.Username,
		"email":         user.Email,
		"full_name":     user.FullName,
		"role":          user.Role,
		"auth_provider": user.AuthProvider,
		"is_protected":  user.IsProtected,
		"jabatan":       employeeJab.String,
		"satker":        employeeSat.String,
		"organisasi":    employeeOrg.String,
		"nip":           employeeNIP.String,
		"picture":       employeePict.String,
	})
}
