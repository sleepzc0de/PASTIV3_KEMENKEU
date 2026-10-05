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
		}

		sldk := api.Group("/sldk", middleware.AuthRequired())
		{
			sldk.GET("/referensi", handlers.GetAssetReferences)
			sldk.GET("/satker", handlers.SearchSatker)
			sldk.GET("/assets/search", handlers.SearchAssets)
			sldk.GET("/assets/:id/detail", handlers.GetAssetDetail)
			sldk.GET("/ringkasan", handlers.GetAssetOverview)
			sldk.PUT("/ringkasan/pengaturan", middleware.RequireRole("admin", "superadmin"), handlers.UpdateOverviewSettings)
		}

		// Digitalisasi Aset: data hasil sinkronisasi dari SLDK (dibaca semua pengguna login; sinkronisasi khusus admin).
		digitalisasi := api.Group("/digitalisasi", middleware.AuthRequired())
		{
			digitalisasi.GET("/ringkasan", handlers.GetDigitalisasiRingkasan)
			digitalisasi.GET("/peta", handlers.GetDigitalisasiPeta)
			digitalisasi.GET("/data/:dataset", handlers.ListDigitalisasiData)
			digitalisasi.GET("/data/:dataset/:id", handlers.GetDigitalisasiDetail)
			digitalisasi.GET("/sinkronisasi", handlers.GetDigitalisasiSinkronisasi)
			digitalisasi.POST("/sinkronisasi", middleware.RequireRole("admin", "superadmin"), handlers.StartDigitalisasiSync)
			digitalisasi.POST("/sinkronisasi/batal", middleware.RequireRole("admin", "superadmin"), handlers.CancelDigitalisasiSync)
		}

		// SAPA (Sistem Administrasi Pengelolaan Aset), modul Penjualan. Hak akses per usulan dan per tahap diperiksa di
		// paket sapa menurut peran SAPA pengguna; pengaturan (template, peran, referensi UE1) khusus admin/superadmin.
		RegisterSapa(api.Group("/sapa", middleware.AuthRequired()))

		// Seluruh fitur HRIS2 (pencarian & detail pegawai) sekarang khusus admin/superadmin.
		hris2 := api.Group("/hris2", middleware.AuthRequired(), middleware.RequireRole("admin", "superadmin"))
		{
			hris2.GET("/pegawai/search", handlers.SearchPegawai)
			hris2.GET("/pegawai/by-nip/:nip", handlers.SearchPegawaiByNIP)
		}

		inaproc := api.Group("/inaproc", middleware.AuthRequired())
		{
			inaproc.GET("/rup/history-kaji-ulang", handlers.GetHistoryKajiUlang)
			inaproc.GET("/rup/history-kaji-ulang/local", handlers.ListLocalKajiUlang)
			inaproc.POST("/rup/history-kaji-ulang/sync", middleware.RequireRole("admin", "superadmin"), handlers.SinkronEksklusif(), handlers.SyncHistoryKajiUlang)

			inaproc.GET("/rup/paket-anggaran-penyedia", handlers.GetPaketAnggaranPenyedia)
			inaproc.GET("/rup/paket-anggaran-penyedia/local", handlers.ListLocalPaketAnggaran)
			inaproc.POST("/rup/paket-anggaran-penyedia/sync", middleware.RequireRole("admin", "superadmin"), handlers.SinkronEksklusif(), handlers.SyncPaketAnggaranPenyedia)

			inaproc.GET("/rup/paket-penyedia", handlers.GetPaketPenyedia)
			inaproc.GET("/rup/paket-penyedia/local", handlers.ListLocalPaketPenyedia)
			inaproc.POST("/rup/paket-penyedia/sync", middleware.RequireRole("admin", "superadmin"), handlers.SinkronEksklusif(), handlers.SyncPaketPenyedia)

			inaproc.GET("/rup/paket-swakelola", handlers.GetPaketSwakelola)
			inaproc.GET("/rup/paket-swakelola/local", handlers.ListLocalPaketSwakelola)
			inaproc.POST("/rup/paket-swakelola/sync", middleware.RequireRole("admin", "superadmin"), handlers.SinkronEksklusif(), handlers.SyncPaketSwakelola)

			inaproc.GET("/rup/program-master", handlers.GetProgramMaster)
			inaproc.GET("/rup/program-master/local", handlers.ListLocalProgramMaster)
			inaproc.POST("/rup/program-master/sync", middleware.RequireRole("admin", "superadmin"), handlers.SinkronEksklusif(), handlers.SyncProgramMaster)

			inaproc.GET("/rup/paket-swakelola-terumumkan", handlers.GetPaketSwakelolaTerumumkan)
			inaproc.GET("/rup/paket-swakelola-terumumkan/local", handlers.ListLocalPaketSwakelolaTerumumkan)
			inaproc.POST("/rup/paket-swakelola-terumumkan/sync", middleware.RequireRole("admin", "superadmin"), handlers.SinkronEksklusif(), handlers.SyncPaketSwakelolaTerumumkan)

			inaproc.GET("/rup/paket-penyedia-terumumkan", handlers.GetPaketPenyediaTerumumkan)
			inaproc.GET("/rup/paket-penyedia-terumumkan/local", handlers.ListLocalPaketPenyediaTerumumkan)
			inaproc.POST("/rup/paket-penyedia-terumumkan/sync", middleware.RequireRole("admin", "superadmin"), handlers.SinkronEksklusif(), handlers.SyncPaketPenyediaTerumumkan)

			inaproc.GET("/rup/paket-anggaran-swakelola", handlers.GetPaketAnggaranSwakelola)
			inaproc.GET("/rup/paket-anggaran-swakelola/local", handlers.ListLocalPaketAnggaranSwakelola)
			inaproc.POST("/rup/paket-anggaran-swakelola/sync", middleware.RequireRole("admin", "superadmin"), handlers.SinkronEksklusif(), handlers.SyncPaketAnggaranSwakelola)

			inaproc.GET("/tender/jadwal-tahapan-non-tender", handlers.GetJadwalTahapanNonTender)
			inaproc.GET("/tender/jadwal-tahapan-non-tender/local", handlers.ListLocalJadwalTahapanNonTender)
			inaproc.POST("/tender/jadwal-tahapan-non-tender/sync", middleware.RequireRole("admin", "superadmin"), handlers.SinkronEksklusif(), handlers.SyncJadwalTahapanNonTender)

			inaproc.GET("/tender/jadwal-tahapan-tender", handlers.GetJadwalTahapanTender)
			inaproc.GET("/tender/jadwal-tahapan-tender/local", handlers.ListLocalJadwalTahapanTender)
			inaproc.POST("/tender/jadwal-tahapan-tender/sync", middleware.RequireRole("admin", "superadmin"), handlers.SinkronEksklusif(), handlers.SyncJadwalTahapanTender)

			inaproc.GET("/tender/non-tender-ekontrak", handlers.GetNonTenderEkontrak)
			inaproc.GET("/tender/non-tender-ekontrak/local", handlers.ListLocalNonTenderEkontrak)
			inaproc.POST("/tender/non-tender-ekontrak/sync", middleware.RequireRole("admin", "superadmin"), handlers.SinkronEksklusif(), handlers.SyncNonTenderEkontrak)

			inaproc.GET("/tender/non-tender-ekontrak-kontrak", handlers.GetNonTenderEkontrakKontrak)
			inaproc.GET("/tender/non-tender-ekontrak-kontrak/local", handlers.ListLocalNonTenderEkontrakKontrak)
			inaproc.POST("/tender/non-tender-ekontrak-kontrak/sync", middleware.RequireRole("admin", "superadmin"), handlers.SinkronEksklusif(), handlers.SyncNonTenderEkontrakKontrak)

			inaproc.GET("/tender/non-tender-pengumuman", handlers.GetNonTenderPengumuman)
			inaproc.GET("/tender/non-tender-pengumuman/local", handlers.ListLocalNonTenderPengumuman)
			inaproc.POST("/tender/non-tender-pengumuman/sync", middleware.RequireRole("admin", "superadmin"), handlers.SinkronEksklusif(), handlers.SyncNonTenderPengumuman)

			inaproc.GET("/tender/non-tender-selesai", handlers.GetNonTenderSelesai)
			inaproc.GET("/tender/non-tender-selesai/local", handlers.ListLocalNonTenderSelesai)
			inaproc.POST("/tender/non-tender-selesai/sync", middleware.RequireRole("admin", "superadmin"), handlers.SinkronEksklusif(), handlers.SyncNonTenderSelesai)

			inaproc.GET("/tender/pencatatan-non-tender", handlers.GetPencatatanNonTender)
			inaproc.GET("/tender/pencatatan-non-tender/local", handlers.ListLocalPencatatanNonTender)
			inaproc.POST("/tender/pencatatan-non-tender/sync", middleware.RequireRole("admin", "superadmin"), handlers.SinkronEksklusif(), handlers.SyncPencatatanNonTender)

			inaproc.GET("/tender/pencatatan-non-tender-realisasi", handlers.GetPencatatanNonTenderRealisasi)
			inaproc.GET("/tender/pencatatan-non-tender-realisasi/local", handlers.ListLocalPencatatanNonTenderRealisasi)
			inaproc.POST("/tender/pencatatan-non-tender-realisasi/sync", middleware.RequireRole("admin", "superadmin"), handlers.SinkronEksklusif(), handlers.SyncPencatatanNonTenderRealisasi)

			inaproc.GET("/tender/pencatatan-swakelola", handlers.GetPencatatanSwakelola)
			inaproc.GET("/tender/pencatatan-swakelola/local", handlers.ListLocalPencatatanSwakelola)
			inaproc.POST("/tender/pencatatan-swakelola/sync", middleware.RequireRole("admin", "superadmin"), handlers.SinkronEksklusif(), handlers.SyncPencatatanSwakelola)

			inaproc.GET("/tender/pencatatan-swakelola-realisasi", handlers.GetPencatatanSwakelolaRealisasi)
			inaproc.GET("/tender/pencatatan-swakelola-realisasi/local", handlers.ListLocalPencatatanSwakelolaRealisasi)
			inaproc.POST("/tender/pencatatan-swakelola-realisasi/sync", middleware.RequireRole("admin", "superadmin"), handlers.SinkronEksklusif(), handlers.SyncPencatatanSwakelolaRealisasi)

			inaproc.GET("/tender/pengumuman", handlers.GetTenderPengumuman)
			inaproc.GET("/tender/pengumuman/local", handlers.ListLocalTenderPengumuman)
			inaproc.POST("/tender/pengumuman/sync", middleware.RequireRole("admin", "superadmin"), handlers.SinkronEksklusif(), handlers.SyncTenderPengumuman)

			inaproc.GET("/tender/peserta-tender", handlers.GetTenderPeserta)
			inaproc.GET("/tender/peserta-tender/local", handlers.ListLocalTenderPeserta)
			inaproc.POST("/tender/peserta-tender/sync", middleware.RequireRole("admin", "superadmin"), handlers.SinkronEksklusif(), handlers.SyncTenderPeserta)

			inaproc.GET("/tender/tender-ekontrak", handlers.GetTenderEkontrak)
			inaproc.GET("/tender/tender-ekontrak/local", handlers.ListLocalTenderEkontrak)
			inaproc.POST("/tender/tender-ekontrak/sync", middleware.RequireRole("admin", "superadmin"), handlers.SinkronEksklusif(), handlers.SyncTenderEkontrak)

			inaproc.GET("/tender/tender-ekontrak-kontrak", handlers.GetTenderEkontrakKontrak)
			inaproc.GET("/tender/tender-ekontrak-kontrak/local", handlers.ListLocalTenderEkontrakKontrak)
			inaproc.POST("/tender/tender-ekontrak-kontrak/sync", middleware.RequireRole("admin", "superadmin"), handlers.SinkronEksklusif(), handlers.SyncTenderEkontrakKontrak)

			inaproc.GET("/tender/tender-selesai", handlers.GetTenderSelesai)
			inaproc.GET("/tender/tender-selesai/local", handlers.ListLocalTenderSelesai)
			inaproc.POST("/tender/tender-selesai/sync", middleware.RequireRole("admin", "superadmin"), handlers.SinkronEksklusif(), handlers.SyncTenderSelesai)

			inaproc.GET("/tender/tender-selesai-nilai", handlers.GetTenderSelesaiNilai)
			inaproc.GET("/tender/tender-selesai-nilai/local", handlers.ListLocalTenderSelesaiNilai)
			inaproc.POST("/tender/tender-selesai-nilai/sync", middleware.RequireRole("admin", "superadmin"), handlers.SinkronEksklusif(), handlers.SyncTenderSelesaiNilai)

			inaproc.GET("/ekatalog-archive/instansi-satker", handlers.GetEkatalogInstansiSatker)
			inaproc.GET("/ekatalog-archive/instansi-satker/local", handlers.ListLocalEkatalogInstansiSatker)
			inaproc.POST("/ekatalog-archive/instansi-satker/sync", middleware.RequireRole("admin", "superadmin"), handlers.SinkronEksklusif(), handlers.SyncEkatalogInstansiSatker)

			inaproc.GET("/ekatalog-archive/komoditas-detail", handlers.GetEkatalogKomoditas)
			inaproc.GET("/ekatalog-archive/komoditas-detail/local", handlers.ListLocalEkatalogKomoditas)
			inaproc.POST("/ekatalog-archive/komoditas-detail/sync", middleware.RequireRole("admin", "superadmin"), handlers.SinkronEksklusif(), handlers.SyncEkatalogKomoditas)

			inaproc.GET("/ekatalog-archive/paket-e-purchasing", handlers.GetEkatalogPaket)
			inaproc.GET("/ekatalog-archive/paket-e-purchasing/local", handlers.ListLocalEkatalogPaket)
			inaproc.POST("/ekatalog-archive/paket-e-purchasing/sync", middleware.RequireRole("admin", "superadmin"), handlers.SinkronEksklusif(), handlers.SyncEkatalogPaket)

			inaproc.GET("/ekatalog-archive/penyedia-detail", handlers.GetEkatalogPenyedia)
			inaproc.GET("/ekatalog-archive/penyedia-detail/local", handlers.ListLocalEkatalogPenyedia)
			inaproc.POST("/ekatalog-archive/penyedia-detail/sync", middleware.RequireRole("admin", "superadmin"), handlers.SinkronEksklusif(), handlers.SyncEkatalogPenyedia)

			inaproc.GET("/ekatalog-archive/penyedia-distributor-detail", handlers.GetEkatalogDistributor)
			inaproc.GET("/ekatalog-archive/penyedia-distributor-detail/local", handlers.ListLocalEkatalogDistributor)
			inaproc.POST("/ekatalog-archive/penyedia-distributor-detail/sync", middleware.RequireRole("admin", "superadmin"), handlers.SinkronEksklusif(), handlers.SyncEkatalogDistributor)

			inaproc.GET("/ekatalog/penyedia-detail", handlers.GetEkatalog6Penyedia)
			inaproc.GET("/ekatalog/penyedia-detail/local", handlers.ListLocalEkatalog6Penyedia)
			inaproc.POST("/ekatalog/penyedia-detail/sync", middleware.RequireRole("admin", "superadmin"), handlers.SinkronEksklusif(), handlers.SyncEkatalog6Penyedia)

			inaproc.GET("/ekatalog/list-kategori-produk", handlers.GetEkatalog6Kategori)
			inaproc.GET("/ekatalog/list-kategori-produk/local", handlers.ListLocalEkatalog6Kategori)
			inaproc.POST("/ekatalog/list-kategori-produk/sync", middleware.RequireRole("admin", "superadmin"), handlers.SinkronEksklusif(), handlers.SyncEkatalog6Kategori)

			inaproc.GET("/ekatalog/paket-e-purchasing", handlers.GetEkatalog6Paket)
			inaproc.GET("/ekatalog/paket-e-purchasing/local", handlers.ListLocalEkatalog6Paket)
			inaproc.POST("/ekatalog/paket-e-purchasing/sync", middleware.RequireRole("admin", "superadmin"), handlers.SinkronEksklusif(), handlers.SyncEkatalog6Paket)

			inaproc.GET("/ekatalog/list-produk-penyedia", handlers.GetEkatalog6ProdukPenyedia)
			inaproc.GET("/ekatalog/list-produk-penyedia/local", handlers.ListLocalEkatalog6ProdukPenyedia)
			inaproc.POST("/ekatalog/list-produk-penyedia/sync", middleware.RequireRole("admin", "superadmin"), handlers.SinkronEksklusif(), handlers.SyncEkatalog6ProdukPenyedia)

			inaproc.GET("/ekatalog/e-purchasing-by-produk", handlers.GetEkatalog6Transaksi)
			inaproc.GET("/ekatalog/e-purchasing-by-produk/local", handlers.ListLocalEkatalog6Transaksi)
			inaproc.POST("/ekatalog/e-purchasing-by-produk/sync", middleware.RequireRole("admin", "superadmin"), handlers.SinkronEksklusif(), handlers.SyncEkatalog6Transaksi)

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

			inaproc.GET("/sync-log", middleware.RequireRole("admin", "superadmin"), handlers.GetSyncHistory)
		}
	}

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "service": "PASTI V3 Backend"})
	})
}
