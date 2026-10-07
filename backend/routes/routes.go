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
			// Tamu (belum punya peran) dan pengguna yang belum menyetujui pernyataan hanya boleh membaca profilnya dan menyetujui pernyataan; semua endpoint lain
			// memakai AuthRequired yang menolak mereka (403).
			auth.GET("/me", middleware.AuthTamuBoleh(), handlers.Me)
			auth.GET("/persetujuan", middleware.AuthTamuBoleh(), handlers.GetPersetujuan)
			auth.POST("/persetujuan", middleware.AuthTamuBoleh(), handlers.PostPersetujuan)
			// Peran data: daftar peran sendiri dan berpindah peran aktif.
			auth.GET("/peran", middleware.AuthTamuBoleh(), handlers.GetPeranSaya)
			auth.POST("/peran/aktif", middleware.AuthRequired(), handlers.PostPeranAktif)
		}

		// Pengguna: superadmin melihat dan mengelola semua; Pengguna Barang semua kecuali superadmin; UE1, Kanwil, dan Satker hanya MELIHAT pengguna dalam cakupan kode
		// satker SSO-nya (lihat handlers/akses_pengguna.go).
		users := api.Group("/users", middleware.AuthRequired())
		{
			lihat := users.Group("", middleware.RequireLihatPengguna())
			{
				lihat.GET("", handlers.ListUsers)
				lihat.GET("/:id", handlers.GetUserDetail)
				// Peran data pengguna (Pengguna Barang, UE1, Kanwil, Satker) dan cakupan datanya.
				lihat.GET("/:id/peran", handlers.GetPeranPengguna)
			}
			kelola := users.Group("", middleware.RequireKelolaPengguna())
			{
				kelola.POST("", handlers.CreateUser)
				kelola.PUT("/:id", handlers.UpdateUser)
				kelola.PUT("/:id/deactivate", handlers.DeactivateUser)
				kelola.DELETE("/:id", handlers.DeleteUser)
				kelola.POST("/:id/peran", handlers.PostPeranPengguna)
				kelola.DELETE("/:id/peran/:peranId", handlers.DeletePeranPengguna)
			}
		}

		// Digitalisasi Aset: data hasil sinkronisasi dari SLDK (dibaca semua pengguna yang punya peran, dibatasi menurut peran). Tab Sinkronisasi (status dan riwayat)
		// hanya bagi peran yang melihat seluruh data (superadmin dan Pengguna Barang); UE1, Kanwil, dan Satker tidak melihatnya. Menjalankan sinkronisasi khusus superadmin.
		digitalisasi := api.Group("/digitalisasi", middleware.AuthRequired())
		{
			digitalisasi.GET("/ringkasan", handlers.GetDigitalisasiRingkasan)
			digitalisasi.GET("/peta", handlers.GetDigitalisasiPeta)
			digitalisasi.GET("/ekspor/:dataset", handlers.EksporDigitalisasiData)
			digitalisasi.GET("/data/:dataset", handlers.ListDigitalisasiData)
			digitalisasi.GET("/data/:dataset/:id", handlers.GetDigitalisasiDetail)
			digitalisasi.GET("/sinkronisasi", middleware.RequireCakupanSemuaPesan("Status dan riwayat sinkronisasi hanya tersedia bagi Pengguna Barang dan superadmin. Data aset satker Anda ada di tab Ringkasan, Peta, dan Data."), handlers.GetDigitalisasiSinkronisasi)
			digitalisasi.POST("/sinkronisasi", middleware.RequireSuperadmin(), handlers.StartDigitalisasiSync)
			digitalisasi.POST("/sinkronisasi/batal", middleware.RequireSuperadmin(), handlers.CancelDigitalisasiSync)
		}

		// SAPA (Sistem Administrasi Pengelolaan Aset), modul Penjualan. Hak akses per usulan dan per tahap diperiksa di
		// paket sapa menurut peran data aplikasi pengguna (SAPA tidak punya peran sendiri); pengaturan (template, jenis/satuan BMN, referensi UE1) khusus superadmin.
		RegisterSapa(api.Group("/sapa", middleware.AuthRequired()))

		// Log audit aktivitas pengguna dan pemantauan resource: khusus superadmin, hanya membaca. Setiap rute memakai AuthRequired lalu RequireSuperadmin; superadmin yang sedang
		// bertindak sebagai peran data tidak punya hak ini selama peran itu aktif. Mengekspor log audit sendiri tercatat sebagai aksi audit.ekspor.
		auditGrup := api.Group("/audit", middleware.AuthRequired(), middleware.RequireSuperadmin())
		{
			auditGrup.GET("", handlers.GetAuditLog)
			auditGrup.GET("/ringkasan", handlers.GetAuditRingkasan)
			auditGrup.GET("/pengguna", handlers.GetAuditPengguna)
			auditGrup.GET("/opsi", handlers.GetAuditOpsi)
			auditGrup.GET("/ekspor", handlers.EksporAuditLog)
		}
		monitorGrup := api.Group("/monitor", middleware.AuthRequired(), middleware.RequireSuperadmin())
		{
			monitorGrup.GET("/ringkasan", handlers.GetMonitorRingkasan)
			monitorGrup.GET("/riwayat", handlers.GetMonitorRiwayat)
			monitorGrup.GET("/database", handlers.GetMonitorDatabase)
		}

		// Seluruh fitur HRIS2 (pencarian & detail pegawai) khusus superadmin.
		hris2 := api.Group("/hris2", middleware.AuthRequired(), middleware.RequireSuperadmin())
		{
			hris2.GET("/pegawai/search", handlers.SearchPegawai)
			hris2.GET("/pegawai/by-nip/:nip", handlers.SearchPegawaiByNIP)
		}

		// Keterhubungan satker: aset (Digitalisasi Aset) dan pengadaan (Inaproc) dihubungkan lewat kode satker 6 digit.
		api.GET("/satker/keterhubungan", middleware.AuthRequired(), handlers.GetSatkerKeterhubungan)
		// Referensi Unit Eselon I (kode 5 digit -> uraian dan singkatan): dibaca semua pengguna yang punya peran, dikelola superadmin.
		referensi := api.Group("/referensi", middleware.AuthRequired())
		{
			referensi.GET("/ue1", handlers.GetRefUE1)
			referensi.PUT("/ue1/:kode", middleware.RequireSuperadmin(), handlers.PutRefUE1)
			referensi.DELETE("/ue1/:kode", middleware.RequireSuperadmin(), handlers.DeleteRefUE1)
			// Referensi Kanwil (kode 9 digit -> uraian): dibaca semua pengguna yang punya peran, dikelola superadmin. Isinya dari nama satker pada data aset ("dari-satker")
			// atau ditarik dari SLDK ("tarik-sldk"); baris yang diisi superadmin tidak pernah ditimpa penarikan.
			referensi.GET("/kanwil", handlers.GetRefKanwil)
			referensi.PUT("/kanwil/:kode", middleware.RequireSuperadmin(), handlers.PutRefKanwil)
			referensi.DELETE("/kanwil/:kode", middleware.RequireSuperadmin(), handlers.DeleteRefKanwil)
			referensi.POST("/kanwil/dari-satker", middleware.RequireSuperadmin(), handlers.PostRefKanwilDariSatker)
			referensi.POST("/kanwil/tarik-sldk", middleware.RequireSuperadmin(), handlers.PostRefKanwilTarikSLDK)
		}
		// Pengadaan Terpadu (Pengadaan, Tender, E-Katalog V5 dan V6). Data lokal, ekspor, dan dasbor terbuka bagi semua pengguna login dan dibatasi ke satker
		// menurut peran aktif lewat kd_satker_str (UE1/Kanwil/Satker hanya melihat satkernya dan hanya dataset yang dapat dibatasi; lihat handlers/inaproc_cakupan.go).
		// Keadaan penarikan memuat seluruh data dan operasi superadmin, jadi hanya bagi peran yang melihat seluruh data; memulai/membatalkan/mengatur khusus superadmin.
		inaproc := api.Group("/inaproc", middleware.AuthRequired())
		{
			dibatasi := inaproc.Group("", middleware.RequireCakupanAda())
			{
				dibatasi.GET("/dataset", handlers.GetInaprocDataset)
				dibatasi.GET("/data/:awalan/:nama", handlers.GetInaprocData)
				dibatasi.GET("/ekspor/:awalan/:nama", handlers.EksporInaprocData)
				dibatasi.GET("/analitik", handlers.GetInaprocAnalitik)
			}
			penarikan := inaproc.Group("", middleware.RequireCakupanSemua())
			{
				penarikan.GET("/penarikan", handlers.GetInaprocPenarikan)
				penarikan.GET("/penarikan/aktif", handlers.GetInaprocPenarikanAktif)
				penarikan.GET("/penarikan/riwayat", handlers.GetInaprocPenarikanRiwayat)
				penarikan.POST("/penarikan", middleware.RequireSuperadmin(), handlers.StartInaprocPenarikan)
				penarikan.POST("/penarikan/batal", middleware.RequireSuperadmin(), handlers.CancelInaprocPenarikan)
				penarikan.PUT("/penarikan/pengaturan", middleware.RequireSuperadmin(), handlers.PutInaprocPenarikanPengaturan)
			}
		}
	}

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "service": "PASTI V3 Backend"})
	})
}
