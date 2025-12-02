package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/helmet"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"

	"github.com/opendefender/openrisk/config"
	"github.com/opendefender/openrisk/database"
	"github.com/opendefender/openrisk/internal/adapters/thehive"
	"github.com/opendefender/openrisk/internal/core/domain"
	"github.com/opendefender/openrisk/internal/handlers"
	"github.com/opendefender/openrisk/internal/middleware"
	"github.com/opendefender/openrisk/internal/migrations"
	"github.com/opendefender/openrisk/internal/workers"
)

func main() {
	// =========================================================================
	// 1. CONFIGURATION & INFRASTRUCTURE
	// =========================================================================

	// Chargement de la configuration (.env)
	cfg := config.LoadConfig()
	// if err != nil {
	// 	log.Printf("⚠️ Warning: No config file found, using environment variables. Error: %v", err)
	// }

	// Initialisation de la Timezone (Important pour les logs/dates)
	time.Local = time.UTC

	// Connexion Base de Données
	database.Connect()

	// Run SQL migrations (if DATABASE_URL is set). This uses the `migrations` folder.
	migrations.RunMigrations()

	// =========================================================================
	// 2. MIGRATIONS & SEEDING (DevOps Friendly)
	// =========================================================================

	log.Println("Database: Running Auto-Migrations...")
	if err := database.DB.AutoMigrate(
		&domain.User{},
		&domain.Risk{},
		&domain.Mitigation{},
		&domain.Asset{},
		&domain.RiskHistory{},
	); err != nil {
		log.Fatalf("❌ Database Migration Failed: %v", err)
	}

	// Création du compte Admin par défaut si la DB est vide
	// Cela garantit que l'app est utilisable immédiatement après déploiement.
	handlers.SeedAdminUser()

	// =========================================================================
	// 3. HEXAGONAL ARCHITECTURE WIRING (Integrations)
	// =========================================================================

	// Initialisation des Adapters (TheHive, OpenRMF, OpenCTI)
	// Ils respectent les interfaces définies dans core/ports
	theHiveAdapter := thehive.NewTheHiveAdapter(cfg.Integrations.TheHive)

	// Initialisation du Moteur de Synchro (Background Worker)
	// Il tourne indépendamment de l'API HTTP
	syncEngine := workers.NewSyncEngine(theHiveAdapter)
	syncEngine.Start()

	log.Println("OpenDefender SyncEngine started in background")

	// =========================================================================
	// 4. WEB SERVER SETUP (Fiber)
	// =========================================================================

	app := fiber.New(fiber.Config{
		AppName:               "OpenRisk API (OpenDefender Suite)",
		DisableStartupMessage: true, // Plus propre dans les logs de prod
		ReadTimeout:           10 * time.Second,
		WriteTimeout:          10 * time.Second,
		// Custom Error Handler pour toujours renvoyer du JSON
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError
			if e, ok := err.(*fiber.Error); ok {
				code = e.Code
			}
			return c.Status(code).JSON(fiber.Map{
				"error": true,
				"msg":   err.Error(),
			})
		},
	})

	// --- Middlewares Globaux ---
	app.Use(recover.New()) // Empêche le crash complet en cas de panic
	app.Use(logger.New(logger.Config{
		Format: "[${time}] ${status} - ${method} ${path} (${latency})\n",
	}))
	app.Use(helmet.New()) // Sécurité headers (XSS, Content-Type, etc.)

	// Configuration CORS Stricte pour la Prod, Permissive pour Dev
	allowOrigins := "http://localhost:5173,http://localhost:3000"
	if os.Getenv("APP_ENV") == "production" {
		allowOrigins = "https://app.opendefender.io" // À changer selon ton domaine
	}
	app.Use(cors.New(cors.Config{
		AllowOrigins: allowOrigins,
		AllowHeaders: "Origin, Content-Type, Accept, Authorization",
		AllowMethods: "GET, POST, PATCH, DELETE, OPTIONS",
	}))

	// =========================================================================
	// 5. API ROUTES
	// =========================================================================

	api := app.Group("/api/v1")

	// --- Routes Publiques ---
	api.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":  "UP",
			"version": "1.0.0",
			"db":      "CONNECTED",
		})
	})
	api.Post("/auth/login", handlers.Login)

	// --- Routes Protégées (Nécessitent JWT) ---
	// Le middleware injecte user_id et role dans le contexte
	protected := api.Use(middleware.Protected())

	// Dashboard & Analytics (Read-Only accessible à tous les connectés)
	protected.Get("/stats", handlers.GetDashboardStats)
	protected.Get("/risks", handlers.GetRisks)

	// Gestion des Risques (Écriture = Analyst & Admin uniquement)
	// Respect du principe "Simplicité & Sécurité"
	writerRole := middleware.RequireRole(domain.RoleAdmin, domain.RoleAnalyst)

	protected.Post("/risks", writerRole, handlers.CreateRisk)
	protected.Post("/risks/:id/mitigations", writerRole, handlers.AddMitigation)
	protected.Patch("/mitigations/:mitigationId/toggle", writerRole, handlers.ToggleMitigationStatus)
	protected.Patch("/mitigations/:mitigationId", writerRole, handlers.UpdateMitigation)
	// Sub-actions (checklist) for mitigations
	protected.Post("/mitigations/:id/subactions", writerRole, handlers.CreateMitigationSubAction)
	protected.Patch("/mitigations/:id/subactions/:subactionId/toggle", writerRole, handlers.ToggleMitigationSubAction)
	protected.Delete("/mitigations/:id/subactions/:subactionId", writerRole, handlers.DeleteMitigationSubAction)

	api.Get("/users/me", handlers.GetMe)

	api.Get("/assets", middleware.Protected(), handlers.GetAssets)
	api.Post("/assets", middleware.Protected(), handlers.CreateAsset)

	api.Get("/stats/risk-matrix", handlers.GetRiskMatrixData)

	api.Get("/export/pdf", handlers.ExportRisksPDF)

	api.Get("/stats/trends", middleware.Protected(), handlers.GetGlobalRiskTrend)

	api.Get("/mitigations/recommended", handlers.GetRecommendedMitigations)

	api.Get("/gamification/me", middleware.Protected(), handlers.GetMyGamificationProfile)

	// =========================================================================
	// 6. GRACEFUL SHUTDOWN (Kubernetes Ready)
	// =========================================================================

	// Channel pour écouter les signaux OS (Ctrl+C, Docker Stop, K8s Terminate)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		port := os.Getenv("PORT")
		if port == "" {
			port = "8080"
		}
		log.Printf("⚡ OpenRisk API listening on port %s", port)
		if err := app.Listen(":" + port); err != nil {
			log.Panic(err)
		}
	}()

	<-quit // Bloque jusqu'à réception du signal
	log.Println("🛑 Shutting down server...")

	// Timeout de 5 secondes pour finir les requêtes en cours
	if err := app.Shutdown(); err != nil {
		log.Fatal("Forced shutdown:", err)
	}

	log.Println("✅ Server exited properly")
}
