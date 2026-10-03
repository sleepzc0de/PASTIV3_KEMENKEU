package routes

import (
	"github.com/gin-gonic/gin"

	"pasti-v3-backend/handlers"
	"pasti-v3-backend/middleware"
)

// RegisterSapa memasang rute SAPA (Sistem Administrasi Pengelolaan Aset) pada grup yang sudah memakai autentikasi.
// Hak akses per usulan dan per tahap diperiksa di paket sapa menurut peran SAPA pengguna; pengaturan (template, peran,
// referensi UE1) dan buka-ulang tahap khusus admin/superadmin. Dipisah dari SetupRoutes agar tabel rute ini bisa diuji.
func RegisterSapa(g *gin.RouterGroup) {
	g.GET("/saya", handlers.GetSapaSaya)
	g.GET("/referensi/satker", handlers.GetSapaSatker)
	g.GET("/referensi/bmn", handlers.GetSapaRefBMN) // jenis BMN aktif + satuan yang diizinkan, untuk formulir

	// Template Excel daftar barang: unduh template, impor berkas yang sudah diisi, ekspor daftar yang sedang dikerjakan.
	g.GET("/barang/template", handlers.UnduhTemplateBarang)
	g.POST("/barang/impor", handlers.ImporBarang)
	g.POST("/barang/ekspor", handlers.EksporBarang)

	// Pencarian pegawai HRIS2 untuk mengisi anggota tim (sesi SSO pengguna; hanya NIP, nama, jabatan, satker yang diteruskan).
	g.GET("/pegawai", handlers.SearchSapaPegawai)
	g.GET("/pegawai/:nip", handlers.GetSapaPegawai)

	g.GET("/penjualan", handlers.ListSapaPenjualan)
	g.POST("/penjualan", handlers.CreateSapaPenjualan)
	g.GET("/penjualan/:id", handlers.GetSapaPenjualan)
	g.PUT("/penjualan/:id/tahap/:kunci", handlers.SaveSapaTahap)
	g.POST("/penjualan/:id/tahap/:kunci/dokumen", handlers.GenerateSapaDokumen)
	g.POST("/penjualan/:id/tahap/:kunci/selesai", handlers.CompleteSapaTahap)
	g.POST("/penjualan/:id/tahap/:kunci/lewati", handlers.SkipSapaTahap)
	g.POST("/penjualan/:id/tahap/:kunci/buka-ulang", middleware.RequireRole("admin", "superadmin"), handlers.ReopenSapaTahap)
	g.GET("/dokumen/:id/unduh", handlers.DownloadSapaDokumen)

	admin := g.Group("", middleware.RequireRole("admin", "superadmin"))
	admin.GET("/template", handlers.ListSapaTemplate)
	admin.POST("/template/:kunci", handlers.UploadSapaTemplate)
	admin.GET("/template/:kunci/unduh", handlers.DownloadSapaTemplate)
	admin.GET("/peran", handlers.ListSapaPeran)
	admin.PUT("/peran/:userId", handlers.SetSapaPeran)
	admin.GET("/bmn", handlers.ListSapaBMN)
	admin.PUT("/bmn/satuan", handlers.SaveSapaSatuanBMN)
	admin.DELETE("/bmn/satuan", handlers.DeleteSapaSatuanBMN)
	admin.PUT("/bmn/jenis", handlers.SaveSapaJenisBMN)
	admin.DELETE("/bmn/jenis", handlers.DeleteSapaJenisBMN)
	admin.GET("/ref-ue1", handlers.ListSapaRefUE1)
	admin.PUT("/ref-ue1/:kode", handlers.SaveSapaRefUE1)
	admin.DELETE("/ref-ue1/:kode", handlers.DeleteSapaRefUE1)
}
