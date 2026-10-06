package routes

import (
	"github.com/gin-gonic/gin"

	"pasti-v3-backend/handlers"
	"pasti-v3-backend/middleware"
)

func SetupRoutes(r *gin.Engine) {
	r.GET("/sso/login", handlers.SSOLogin)
	r.GET("/sso/callback/login", handlers.SSOCallback)

	api := r.Group("/api/v1")
	{
		auth := api.Group("/auth")
		{
			auth.GET("/captcha", handlers.GenerateCaptcha)
			auth.POST("/register", handlers.Register)
			auth.POST("/login", handlers.Login)
			auth.GET("/me", middleware.AuthRequired(), handlers.Me)
			// Peran data: daftar peran sendiri dan berpindah peran aktif.
			auth.GET("/peran", middleware.AuthRequired(), handlers.GetPeranSaya)
			auth.POST("/peran/aktif", middleware.AuthRequired(), handlers.PostPeranAktif)
		}

		users := api.Group("/users", middleware.AuthRequired(), middleware.RequireRole("admin", "superadmin"))
		{
			users.GET("", handlers.ListUsers)
			users.POST("", handlers.CreateUser)
			users.GET("/:id", handlers.GetUserDetail)
			users.PUT("/:id", handlers.UpdateUser)
			users.PUT("/:id/role", handlers.UpdateUserRole)
			users.PUT("/:id/deactivate", handlers.DeactivateUser)
			users.DELETE("/:id", handlers.DeleteUser)
			// Peran data pengguna (Pengguna Barang, UE1, Kanwil, Satker) dan cakupan datanya.
			users.GET("/:id/peran", handlers.GetPeranPengguna)
			users.POST("/:id/peran", handlers.PostPeranPengguna)
			users.DELETE("/:id/peran/:peranId", handlers.DeletePeranPengguna)
		}

		// Digitalisasi Aset: data hasil sinkronisasi dari SLDK (dibaca semua pengguna login; sinkronisasi khusus admin).
		digitalisasi := api.Group("/digitalisasi", middleware.AuthRequired())
		{
			digitalisasi.GET("/ringkasan", handlers.GetDigitalisasiRingkasan)
			digitalisasi.GET("/peta", handlers.GetDigitalisasiPeta)
			digitalisasi.GET("/ekspor/:dataset", handlers.EksporDigitalisasiData)
			digitalisasi.GET("/data/:dataset", handlers.ListDigitalisasiData)
			digitalisasi.GET("/data/:dataset/:id", handlers.GetDigitalisasiDetail)
			digitalisasi.GET("/sinkronisasi", handlers.GetDigitalisasiSinkronisasi)
			digitalisasi.POST("/sinkronisasi", middleware.RequireRole("admin", "superadmin"), handlers.StartDigitalisasiSync)
			digitalisasi.POST("/sinkronisasi/batal", middleware.RequireRole("admin", "superadmin"), handlers.CancelDigitalisasiSync)
		}

		// SAPA (Sistem Administrasi Pengelolaan Aset), modul Penjualan. Hak akses per usulan dan per tahap diperiksa di
		// paket sapa menurut peran data aplikasi pengguna (SAPA tidak punya peran sendiri); pengaturan (template, jenis/satuan BMN, referensi UE1) khusus admin/superadmin.
		RegisterSapa(api.Group("/sapa", middleware.AuthRequired()))

		// Seluruh fitur HRIS2 (pencarian & detail pegawai) sekarang khusus admin/superadmin.
		hris2 := api.Group("/hris2", middleware.AuthRequired(), middleware.RequireRole("admin", "superadmin"))
		{
			hris2.GET("/pegawai/search", handlers.SearchPegawai)
			hris2.GET("/pegawai/by-nip/:nip", handlers.SearchPegawaiByNIP)
		}

		// Keterhubungan satker: aset (Digitalisasi Aset) dan pengadaan (Inaproc) dihubungkan lewat kode satker 6 digit.
		api.GET("/satker/keterhubungan", middleware.AuthRequired(), handlers.GetSatkerKeterhubungan)
		// Referensi Unit Eselon I (kode 5 digit -> uraian dan singkatan): dibaca semua pengguna login, dikelola admin/superadmin.
		referensi := api.Group("/referensi", middleware.AuthRequired())
		{
			referensi.GET("/ue1", handlers.GetRefUE1)
			referensi.PUT("/ue1/:kode", middleware.RequireRole("admin", "superadmin"), handlers.PutRefUE1)
			referensi.DELETE("/ue1/:kode", middleware.RequireRole("admin", "superadmin"), handlers.DeleteRefUE1)
		}
		// Data Pengadaan lengkap belum bisa dibatasi per satker, jadi hanya untuk peran yang melihat seluruh data.
		inaproc := api.Group("/inaproc", middleware.AuthRequired(), middleware.RequireCakupanSemua())
		{
			// Penarikan Data terpadu (Pengadaan, Tender, E-Katalog V5 dan V6): status, penarikan manual dan otomatis, data lokal, ekspor, dasbor.
			// Membaca terbuka bagi semua pengguna login; memulai/membatalkan/mengatur khusus admin.
			inaproc.GET("/penarikan", handlers.GetInaprocPenarikan)
			inaproc.GET("/penarikan/aktif", handlers.GetInaprocPenarikanAktif)
			inaproc.GET("/penarikan/riwayat", handlers.GetInaprocPenarikanRiwayat)
			inaproc.POST("/penarikan", middleware.RequireRole("admin", "superadmin"), handlers.StartInaprocPenarikan)
			inaproc.POST("/penarikan/batal", middleware.RequireRole("admin", "superadmin"), handlers.CancelInaprocPenarikan)
			inaproc.PUT("/penarikan/pengaturan", middleware.RequireRole("admin", "superadmin"), handlers.PutInaprocPenarikanPengaturan)
			inaproc.GET("/data/:awalan/:nama", handlers.GetInaprocData)
			inaproc.GET("/ekspor/:awalan/:nama", handlers.EksporInaprocData)
			inaproc.GET("/analitik", handlers.GetInaprocAnalitik)
		}
	}

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "service": "PASTI V3 Backend"})
	})
}
