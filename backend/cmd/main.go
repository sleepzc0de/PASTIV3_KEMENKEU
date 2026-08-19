package main

import (
	"log"

	"github.com/gin-gonic/gin"

	"pasti-v3-backend/config"
	"pasti-v3-backend/database"
	"pasti-v3-backend/middleware"
	"pasti-v3-backend/routes"
)

func main() {
	config.LoadConfig()
	database.Connect()
	database.ConnectSLDK()

	if config.Cfg.AppEnv == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.Default()
	r.Use(middleware.CORSMiddleware())

	routes.SetupRoutes(r)

	log.Println("[INFO] PASTI V3 Backend berjalan di port:", config.Cfg.AppPort)
	if err := r.Run(":" + config.Cfg.AppPort); err != nil {
		log.Fatal("[FATAL] Server gagal dijalankan:", err)
	}
}
