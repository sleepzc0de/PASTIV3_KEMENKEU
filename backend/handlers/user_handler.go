package handlers

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	mssql "github.com/microsoft/go-mssqldb"

	"pasti-v3-backend/audit"
	"pasti-v3-backend/database"
	"pasti-v3-backend/dto"
	"pasti-v3-backend/peran"
	"pasti-v3-backend/persetujuan"
	"pasti-v3-backend/utils"
)

// isSelf memeriksa apakah target adalah akun yang sedang login. GUID dibandingkan tanpa
// membedakan huruf besar/kecil karena token dan database bisa memformatnya berbeda.
func isSelf(c *gin.Context, targetID string) bool {
	me := c.GetString("user_id")
	return me != "" && strings.EqualFold(me, targetID)
}

// kuerPengguna membaca pengguna yang boleh dilihat pemanggil (kondisiPengguna), dengan profil pegawai, satker pada data aset, peran data, dan status persetujuan. id
// kosong = semua; terisi = hanya pengguna itu.
func kuerPengguna(c *gin.Context, ctx context.Context, id string) ([]dto.UserListItem, error) {
	q := `
		SELECT u.id, u.username, u.email, u.full_name, u.role, u.is_active, u.auth_provider, u.is_protected,
		       COALESCE(u.nip, e.nip), e.jabatan, e.satker, e.kode_satker, sa.nama, u.created_at,
		       CASE WHEN u.persetujuan_at IS NOT NULL AND u.persetujuan_versi = @p1 THEN 1 ELSE 0 END
		FROM users u
		LEFT JOIN employees e ON e.id = u.employee_id
		LEFT JOIN (SELECT SUBSTRING(Kode_Satker, 10, 6) AS k6, COALESCE(MAX(CASE WHEN Jenis_Satker = N'INDUK SATKER' THEN Nama_Satker END), MAX(Nama_Satker)) AS nama
		           FROM DIGITALISASI_SATKER WHERE LEN(Kode_Satker) >= 15 GROUP BY SUBSTRING(Kode_Satker, 10, 6)) sa
		       ON LEN(e.kode_satker) >= 15 AND SUBSTRING(e.kode_satker, 10, 6) = sa.k6
		WHERE (` + kondisiPengguna(c) + `)`
	args := []interface{}{persetujuan.Versi}
	if id != "" {
		q += " AND u.id = @p2"
		args = append(args, id)
	}
	q += " ORDER BY u.created_at DESC"
	rows, err := database.DB.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := []dto.UserListItem{}
	for rows.Next() {
		var idRaw mssql.UniqueIdentifier
		var u dto.UserListItem
		var nip, jabatan, satker, kodeSatker, satkerAset sql.NullString
		var createdAt sql.NullTime
		if err := rows.Scan(&idRaw, &u.Username, &u.Email, &u.FullName, &u.Role, &u.IsActive, &u.AuthProvider, &u.IsProtected,
			&nip, &jabatan, &satker, &kodeSatker, &satkerAset, &createdAt, &u.Setuju); err != nil {
			return nil, err
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
		if kodeSatker.Valid {
			u.KodeSatker = &kodeSatker.String
		}
		if satkerAset.Valid {
			u.SatkerAset = &satkerAset.String
		}
		if createdAt.Valid {
			u.CreatedAt = createdAt.Time.Format("2006-01-02 15:04")
		}
		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Peran data tiap pengguna; gagal membacanya tidak menggagalkan daftar (peran dikosongkan).
	semua, err := peran.DaftarSemua(ctx)
	if err != nil {
		log.Println("[PENGGUNA WARN] gagal membaca peran semua pengguna:", err)
		semua = map[string][]peran.Baris{}
	}
	for i := range users {
		users[i].PeranData = semua[strings.ToUpper(users[i].ID)]
		if users[i].PeranData == nil {
			users[i].PeranData = []peran.Baris{}
		}
		users[i].Tamu = len(users[i].PeranData) == 0 && users[i].Role != peran.AkunSuperadmin
	}
	return users, nil
}

// ListUsers: GET /users. Daftar pengguna yang boleh dilihat peran pemanggil: semua (superadmin), semua kecuali superadmin (Pengguna Barang), atau pengguna dalam
// cakupan kode satker SSO-nya (UE1, Kanwil, Satker; hanya lihat). Lihat akses_pengguna.go.
func ListUsers(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 20*time.Second)
	defer cancel()
	users, err := kuerPengguna(c, ctx, "")
	if err != nil {
		log.Println("[PENGGUNA ERROR] daftar pengguna:", err)
		utils.ErrorResponse(c, http.StatusInternalServerError, "Gagal mengambil daftar user")
		return
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

	// Akun yang dibuat lewat fitur ini selalu berrole "user" dan berstatus tamu sampai diberi peran data. Superadmin hanya ditetapkan di .env (tidak ada jalur lain),
	// dan role admin lama sudah ditiadakan.
	if req.Role != "" && req.Role != "user" {
		utils.ErrorResponse(c, http.StatusBadRequest, "Role tidak valid")
		return
	}
	req.Role = "user"

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
			utils.ErrorResponseWithCode(c, http.StatusUnauthorized, "Sesi SSO Anda telah berakhir, silakan login ulang via SSO untuk memakai fitur ini", utils.CodeSSOSessionExpired)
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

	audit.Tandai(c, audit.Tanda{ObjekTipe: "pengguna", ObjekID: newID, Detail: map[string]interface{}{"username": req.Username, "role": req.Role}})
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

func DeleteUser(c *gin.Context) {
	targetID := c.Param("id")
	if !penggunaTerlihatID(c, targetID) {
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
		utils.ErrorResponse(c, http.StatusForbidden, "Akun superadmin permanen ini tidak dapat dihapus")
		return
	}

	audit.Tandai(c, audit.Tanda{Detail: map[string]interface{}{"username": usernameDariID(targetID)}}) // dicatat sebelum akunnya hilang
	_, err = database.DB.Exec(`DELETE FROM users WHERE id = @p1`, targetID)
	if err != nil {
		utils.ErrorResponse(c, http.StatusInternalServerError, "Gagal menghapus user")
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "User berhasil dihapus", nil)
}

func DeactivateUser(c *gin.Context) {
	targetID := c.Param("id")

	// Penonaktifan langsung mengakhiri sesi, jadi menonaktifkan akun sendiri berarti mengunci diri.
	if isSelf(c, targetID) {
		utils.ErrorResponse(c, http.StatusForbidden, "Anda tidak dapat menonaktifkan akun Anda sendiri")
		return
	}
	if !penggunaTerlihatID(c, targetID) {
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

// UpdateUser mengubah data dasar user (nama, email, status aktif), dan password secara opsional. Role akun tidak dapat diubah lewat API. Tidak berlaku untuk akun
// protected, dan Pengguna Barang tidak dapat menyentuh superadmin (dijawab 404).
func UpdateUser(c *gin.Context) {
	targetID := c.Param("id")

	var req dto.UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, "Data tidak valid: "+err.Error())
		return
	}

	if !req.IsActive && isSelf(c, targetID) {
		utils.ErrorResponse(c, http.StatusForbidden, "Anda tidak dapat menonaktifkan akun Anda sendiri")
		return
	}
	if !penggunaTerlihatID(c, targetID) {
		return
	}

	var isProtected bool
	var authProvider string
	err := database.DB.QueryRow(`SELECT is_protected, auth_provider FROM users WHERE id = @p1`, targetID).
		Scan(&isProtected, &authProvider)
	if err == sql.ErrNoRows {
		utils.ErrorResponse(c, http.StatusNotFound, "User tidak ditemukan")
		return
	} else if err != nil {
		utils.ErrorResponse(c, http.StatusInternalServerError, "Terjadi kesalahan server")
		return
	}

	if isProtected {
		utils.ErrorResponse(c, http.StatusForbidden, "Akun superadmin permanen ini tidak dapat diubah")
		return
	}

	// Cegah bentrok email dengan user lain
	var emailOwner string
	err = database.DB.QueryRow(`SELECT id FROM users WHERE email = @p1 AND id != @p2`, req.Email, targetID).Scan(&emailOwner)
	if err == nil {
		utils.ErrorResponse(c, http.StatusConflict, "Email sudah digunakan oleh user lain")
		return
	} else if err != sql.ErrNoRows {
		utils.ErrorResponse(c, http.StatusInternalServerError, "Gagal memeriksa email")
		return
	}

	if req.Password != "" {
		if len(req.Password) < 8 {
			utils.ErrorResponse(c, http.StatusBadRequest, "Password baru minimal 8 karakter")
			return
		}
		if authProvider == "sso" {
			utils.ErrorResponse(c, http.StatusBadRequest, "Akun SSO Kemenkeu tidak memiliki password lokal, tidak bisa diatur di sini")
			return
		}

		hash, salt, err := utils.HashPassword(req.Password)
		if err != nil {
			utils.ErrorResponse(c, http.StatusInternalServerError, "Gagal memproses password")
			return
		}

		_, err = database.DB.Exec(`
			UPDATE users SET full_name=@p1, email=@p2, is_active=@p3,
			       password_hash=@p4, password_salt=@p5, updated_at=SYSUTCDATETIME()
			WHERE id=@p6`,
			req.FullName, req.Email, req.IsActive, hash, salt, targetID,
		)
		if err != nil {
			utils.ErrorResponse(c, http.StatusInternalServerError, "Gagal memperbarui user")
			return
		}
	} else {
		_, err = database.DB.Exec(`
			UPDATE users SET full_name=@p1, email=@p2, is_active=@p3, updated_at=SYSUTCDATETIME()
			WHERE id=@p4`,
			req.FullName, req.Email, req.IsActive, targetID,
		)
		if err != nil {
			utils.ErrorResponse(c, http.StatusInternalServerError, "Gagal memperbarui user")
			return
		}
	}

	audit.Tandai(c, audit.Tanda{Detail: map[string]interface{}{"aktif": req.IsActive, "kata_sandi_diganti": req.Password != ""}})
	utils.SuccessResponse(c, http.StatusOK, "User berhasil diperbarui", nil)
}

// GetUserDetail mengambil data 1 user untuk keperluan mengisi form edit. Pengguna di luar yang boleh dilihat pemanggil dijawab 404.
func GetUserDetail(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 20*time.Second)
	defer cancel()
	users, err := kuerPengguna(c, ctx, c.Param("id"))
	if err != nil {
		log.Println("[PENGGUNA ERROR] detail pengguna:", err)
		utils.ErrorResponse(c, http.StatusInternalServerError, "Gagal mengambil data user")
		return
	}
	if len(users) == 0 {
		utils.ErrorResponse(c, http.StatusNotFound, "User tidak ditemukan")
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "Berhasil mengambil data user", users[0])
}
