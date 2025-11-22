package main

import (
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/opendefender/openrisk/database"
	"github.com/opendefender/openrisk/internal/core/domain" // Import du domain
	"github.com/opendefender/openrisk/internal/handlers"    // Import des handlers
)

func main() {
	// 1. Database
	database.Connect()
	
	// 🔄 AUTO-MIGRATION : Crée la table 'risks' dans Postgres automatiquement
	// C'est vital pour le déploiement facile "One Command"
	log.Println("🔃 Running Auto-Migration...")
	database.DB.AutoMigrate(&domain.Risk{})

	// 2. App Setup
	app := fiber.New(fiber.Config{
		AppName: "OpenRisk API v2",
	})

	app.Use(logger.New())
	app.Use(cors.New())

	// 3. Routes API
	api := app.Group("/api/v1") // Versioning API
	
	// Routes Risques
	api.Post("/risks", handlers.CreateRisk)
	api.Get("/risks", handlers.GetRisks)

	// Health
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "OK", "service": "OpenRisk"})
	})

	log.Fatal(app.Listen(":8080"))
}