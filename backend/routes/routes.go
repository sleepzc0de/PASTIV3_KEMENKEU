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
			sldk.GET("/assets/columns", handlers.GetAssetColumns)
			sldk.GET("/assets/search", handlers.SearchAssets)
		}

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
			inaproc.POST("/rup/history-kaji-ulang/sync", middleware.RequireRole("admin", "superadmin"), handlers.SyncHistoryKajiUlang)

			inaproc.GET("/rup/paket-anggaran-penyedia", handlers.GetPaketAnggaranPenyedia)
			inaproc.GET("/rup/paket-anggaran-penyedia/local", handlers.ListLocalPaketAnggaran)
			inaproc.POST("/rup/paket-anggaran-penyedia/sync", middleware.RequireRole("admin", "superadmin"), handlers.SyncPaketAnggaranPenyedia)

			inaproc.GET("/sync-log", middleware.RequireRole("admin", "superadmin"), handlers.GetSyncHistory)
		}
	}

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "service": "PASTI V3 Backend"})
	})
}
