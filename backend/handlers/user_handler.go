package handlers

import (
	"database/sql"
	"net/http"

	"github.com/gin-gonic/gin"

	"pasti-v3-backend/database"
	"pasti-v3-backend/utils"
)

// UpdateUserRole - hanya bisa dipakai oleh admin/superadmin, tidak berlaku untuk akun protected
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

// DeleteUser - tidak berlaku untuk akun protected
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

// DeactivateUser - tidak berlaku untuk akun protected
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
