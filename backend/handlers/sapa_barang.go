package handlers

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"pasti-v3-backend/sapa"
	"pasti-v3-backend/utils"
)

// Template Excel daftar barang (Nota Dinas Usulan Penjualan): unduh template, impor dari berkas yang sudah diisi, dan ekspor
// daftar yang sedang dikerjakan. Ketiganya tidak menyentuh database; hanya pengguna yang boleh memakai SAPA yang dilayani.

const (
	sapaXLSXType      = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	sapaMaksImpor     = sapa.MaksUkuranXLSX + (1 << 20) // berkas + sisa bidang formulir multipart
	sapaMaksEksporReq = 2 << 20                         // badan JSON daftar barang yang diekspor
)

// GET /sapa/barang/template
func UnduhTemplateBarang(c *gin.Context) {
	ctx, cancel := sapaCtx(c)
	defer cancel()
	if _, ok := sapaMasuk(c, ctx); !ok {
		return
	}
	berkas, err := sapa.TemplateBarangXLSX()
	if err != nil {
		sapaGagal(c, "template barang", err)
		return
	}
	sapaKirimUnduhan(c, "Template Daftar Barang SAPA.xlsx", sapaXLSXType, "template.xlsx", berkas)
}

// POST /sapa/barang/impor (multipart: berkas). Baris bermasalah tetap dikembalikan bersama daftar galatnya agar bisa
// diperbaiki di formulir; hanya masalah pada berkas itu sendiri yang dijawab sebagai galat.
func ImporBarang(c *gin.Context) {
	ctx, cancel := sapaCtx(c)
	defer cancel()
	if _, ok := sapaMasuk(c, ctx); !ok {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, sapaMaksImpor)
	fh, err := c.FormFile("berkas")
	if err != nil {
		var mbe *http.MaxBytesError
		if errors.As(err, &mbe) {
			utils.ErrorResponse(c, http.StatusRequestEntityTooLarge, fmt.Sprintf("Berkas terlalu besar (maksimal %d MB)", sapa.MaksUkuranXLSX>>20))
			return
		}
		utils.ErrorResponse(c, http.StatusBadRequest, "Pilih berkas Excel (.xlsx)")
		return
	}
	if !strings.HasSuffix(strings.ToLower(fh.Filename), ".xlsx") {
		utils.ErrorResponse(c, http.StatusBadRequest, "Berkas harus berformat .xlsx (simpan dari Excel sebagai Excel Workbook)")
		return
	}
	if fh.Size > sapa.MaksUkuranXLSX {
		utils.ErrorResponse(c, http.StatusRequestEntityTooLarge, fmt.Sprintf("Berkas terlalu besar (maksimal %d MB)", sapa.MaksUkuranXLSX>>20))
		return
	}
	f, err := fh.Open()
	if err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, "Berkas tidak dapat dibaca")
		return
	}
	defer f.Close()
	berkas, err := io.ReadAll(io.LimitReader(f, sapa.MaksUkuranXLSX+1))
	if err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, "Berkas tidak dapat dibaca")
		return
	}
	hasil, err := sapa.ImporBarangXLSX(berkas)
	if err != nil {
		sapaGagal(c, "impor barang", err)
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "OK", hasil)
}

// POST /sapa/barang/ekspor (JSON: {"barang": [...]})
func EksporBarang(c *gin.Context) {
	ctx, cancel := sapaCtx(c)
	defer cancel()
	if _, ok := sapaMasuk(c, ctx); !ok {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, sapaMaksEksporReq)
	var in struct {
		Barang []sapa.Barang `json:"barang"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, "Permintaan tidak valid")
		return
	}
	berkas, err := sapa.EksporBarangXLSX(in.Barang)
	if err != nil {
		sapaGagal(c, "ekspor barang", err)
		return
	}
	sapaKirimUnduhan(c, "Daftar Barang SAPA.xlsx", sapaXLSXType, "daftar-barang.xlsx", berkas)
}
