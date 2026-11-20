// @title           Delcom Blog API
// @version         1.0
// @description     REST API platform blog multi-tier dengan AI Blog Generator terintegrasi Search Engine.
// @BasePath        /
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
package main

import (
	"log"

	"app/internal/config"
	"app/internal/database"
	"app/internal/server"
)

func main() {
	cfg := config.Load()

	db, err := database.Connect(cfg)
	if err != nil {
		log.Fatalf("gagal koneksi database: %v", err)
	}
	if err := database.Migrate(db); err != nil {
		log.Fatalf("gagal migrasi: %v", err)
	}
	if err := database.Seed(db, cfg); err != nil {
		log.Fatalf("gagal seeding admin: %v", err)
	}

	router := server.New(cfg, db)
	log.Printf("Delcom Blog API berjalan di http://localhost:%s (Swagger: /swagger/index.html)", cfg.AppPort)
	if err := router.Run(":" + cfg.AppPort); err != nil {
		log.Fatalf("server berhenti: %v", err)
	}
}
