package main

import (
	"log"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/opendefender/openrisk/config"
	"github.com/opendefender/openrisk/database"
	"github.com/opendefender/openrisk/internal/adapters/thehive"
	"github.com/opendefender/openrisk/internal/core/domain"
	"github.com/opendefender/openrisk/internal/handlers"
	"github.com/opendefender/openrisk/internal/workers"
)

func main() {
	// =========================================================================
	// 1. INITIALISATION & CONFIGURATION
	// =========================================================================
	log.Println("🚀 Initializing OpenRisk (OpenDefender Suite)...")

	// Charge la configuration (Variables d'env ou .env file)
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Println("⚠️  Warning: Could not load config file, relying on environment variables.")
	}

	// =========================================================================
	// 2. BASE DE DONNÉES (PERSISTANCE)
	// =========================================================================
	// Connexion robuste à PostgreSQL
	database.Connect()

	// AUTO-MIGRATION : Garantit que l'app est "Standalone" et s'installe toute seule.
	// Crée ou met à jour les tables sans perte de données.
	log.Println("🔄 Running Database Auto-Migrations...")
	err = database.DB.AutoMigrate(
		&domain.Risk{},
		&domain.Mitigation{},
		// Ajouter d'autres modèles ici (ex: &domain.Asset{})
	)
	if err != nil {
		log.Fatalf("❌ Migration failed: %v", err)
	}
	log.Println("✅ Database Schema is up to date.")

	// =========================================================================
	// 3. INTEGRATIONS & HEXAGONAL ADAPTERS
	// =========================================================================
	// Initialisation des connecteurs externes.
	// Ils respectent les interfaces définies dans 'internal/core/ports'.
	
	// Adapter TheHive (Incident Response)
	theHiveAdapter := thehive.NewTheHiveAdapter(cfg.Integrations.TheHive)

	// Note: On ajoutera ici OpenCTIAdapter et OpenRMFAdapter plus tard
	// openCTIAdapter := opencti.NewAdapter(...)

	// =========================================================================
	// 4. WORKERS (BACKGROUND JOBS)
	// =========================================================================
	// Le SyncEngine orchestre la récupération des données externes pour nourrir OpenRisk.
	// Il tourne indépendamment de l'API HTTP.
	
	syncEngine := workers.NewSyncEngine(theHiveAdapter)
	syncEngine.Start() // Lance la Goroutine de synchronisation
	log.Println("⚙️  SyncEngine started (Background Worker active)")

	// =========================================================================
	// 5. SERVEUR HTTP (API REST)
	// =========================================================================
	app := fiber.New(fiber.Config{
		AppName:               "OpenRisk API v2.0",
		DisableStartupMessage: false,
		ReadTimeout:           10 * time.Second,
		WriteTimeout:          10 * time.Second,
	})

	// Middlewares de Sécurité et Monitoring
	app.Use(logger.New())  // Logs des requêtes HTTP
	app.Use(recover.New()) // Empêche le crash du serveur en cas de Panic inattendue
	
	// CORS : Permet au Frontend React (localhost ou prod) de communiquer
	app.Use(cors.New(cors.Config{
		AllowOrigins: "http://localhost:5173,https://app.opendefender.io",
		AllowHeaders: "Origin, Content-Type, Accept, Authorization",
		AllowMethods: "GET, POST, PATCH, DELETE, OPTIONS",
	}))

	// =========================================================================
	// 6. ROUTING (API V1)
	// =========================================================================
	api := app.Group("/api/v1")

	// --- System & Health ---
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.Status(200).JSON(fiber.Map{
			"status":  "UP",
			"version": "2.0.0",
			"module":  "OpenRisk",
			"db":      "CONNECTED",
		})
	})

	// --- Dashboard Analytics ---
	api.Get("/stats", handlers.GetDashboardStats)

	// --- Risks Management ---
	api.Get("/risks", handlers.GetRisks)
	api.Post("/risks", handlers.CreateRisk)

	// --- Mitigations (Plans d'action) ---
	api.Post("/risks/:id/mitigations", handlers.AddMitigation)
	api.Patch("/mitigations/:mitigationId/toggle", handlers.ToggleMitigationStatus)

	// =========================================================================
	// 7. LANCEMENT
	// =========================================================================
	log.Println("✅ OpenRisk is ready on port :8080")
	if err := app.Listen(":8080"); err != nil {
		log.Fatalf("❌ Failed to start server: %v", err)
	}
}