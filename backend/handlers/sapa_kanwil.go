package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"pasti-v3-backend/utils"
)

// GET /sapa/referensi/kanwil: Referensi Kanwil yang aktif untuk pemilih tembusan Kepala Kantor Wilayah pada formulir Nota Dinas (dicari menurut kode atau uraian),
// beserta saran teks tembusannya. Isinya dikelola superadmin di Administrasi > Referensi Kanwil; semua pengguna SAPA boleh membacanya.
func GetSapaRefKanwil(c *gin.Context) {
	ctx, cancel := sapaCtx(c)
	defer cancel()
	id, ok := sapaMasuk(c, ctx)
	if !ok {
		return
	}
	daftar, err := sapaLayanan().RefKanwilAktif(ctx, id)
	if err != nil {
		sapaGagal(c, "referensi Kanwil", err)
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "OK", gin.H{"daftar": daftar})
}
