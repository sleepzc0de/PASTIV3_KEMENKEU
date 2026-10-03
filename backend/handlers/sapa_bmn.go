package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"pasti-v3-backend/sapa"
	"pasti-v3-backend/utils"
)

// Daftar jenis BMN dan satuan jumlahnya. Formulir memakai daftar aktif; admin mengelola daftar lengkap di Pengaturan SAPA.
// Nama jenis/satuan dikirim di badan permintaan atau query (bukan segmen jalur) karena bisa memuat koma, spasi, dan huruf non-ASCII.

// GET /sapa/referensi/bmn: jenis BMN aktif beserta satuan yang diizinkan untuk tiap jenis.
func GetSapaRefBMN(c *gin.Context) {
	ctx, cancel := sapaCtx(c)
	defer cancel()
	id, ok := sapaMasuk(c, ctx)
	if !ok {
		return
	}
	ref, err := sapaLayanan().RefBMNAktif(ctx, id)
	if err != nil {
		sapaGagal(c, "referensi BMN", err)
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "OK", ref)
}

// GET /sapa/bmn (admin): daftar lengkap termasuk yang nonaktif.
func ListSapaBMN(c *gin.Context) {
	ctx, cancel := sapaCtx(c)
	defer cancel()
	id, ok := sapaIdentitas(c, ctx)
	if !ok {
		return
	}
	ref, err := sapaLayanan().RefBMNSemua(ctx, id)
	if err != nil {
		sapaGagal(c, "daftar BMN", err)
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "OK", ref)
}

func SaveSapaSatuanBMN(c *gin.Context) {
	ctx, cancel := sapaCtx(c)
	defer cancel()
	id, ok := sapaIdentitas(c, ctx)
	if !ok {
		return
	}
	var in sapa.SatuanBMN
	if err := c.ShouldBindJSON(&in); err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, "Permintaan tidak valid")
		return
	}
	if err := sapaLayanan().SimpanSatuanBMN(ctx, id, in); err != nil {
		sapaGagal(c, "simpan satuan BMN", err)
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "Satuan disimpan", nil)
}

func DeleteSapaSatuanBMN(c *gin.Context) {
	ctx, cancel := sapaCtx(c)
	defer cancel()
	id, ok := sapaIdentitas(c, ctx)
	if !ok {
		return
	}
	if err := sapaLayanan().HapusSatuanBMN(ctx, id, c.Query("nama")); err != nil {
		sapaGagal(c, "hapus satuan BMN", err)
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "Satuan dihapus", nil)
}

func SaveSapaJenisBMN(c *gin.Context) {
	ctx, cancel := sapaCtx(c)
	defer cancel()
	id, ok := sapaIdentitas(c, ctx)
	if !ok {
		return
	}
	var in sapa.JenisBMN
	if err := c.ShouldBindJSON(&in); err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, "Permintaan tidak valid")
		return
	}
	if err := sapaLayanan().SimpanJenisBMN(ctx, id, in); err != nil {
		sapaGagal(c, "simpan jenis BMN", err)
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "Jenis BMN disimpan", nil)
}

func DeleteSapaJenisBMN(c *gin.Context) {
	ctx, cancel := sapaCtx(c)
	defer cancel()
	id, ok := sapaIdentitas(c, ctx)
	if !ok {
		return
	}
	if err := sapaLayanan().HapusJenisBMN(ctx, id, c.Query("nama")); err != nil {
		sapaGagal(c, "hapus jenis BMN", err)
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "Jenis BMN dihapus", nil)
}
