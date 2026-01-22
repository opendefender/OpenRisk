package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"strconv"
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
	"github.com/opendefender/openrisk/internal/cache"
	"github.com/opendefender/openrisk/internal/core/domain"
	"github.com/opendefender/openrisk/internal/handlers"
	"github.com/opendefender/openrisk/internal/middleware"
	"github.com/opendefender/openrisk/internal/migrations"
	"github.com/opendefender/openrisk/internal/services"
	"github.com/opendefender/openrisk/internal/workers"
)

func main() {
	// =========================================================================
	// 1. CONFIGURATION & INFRASTRUCTURE
	// =========================================================================

	// Chargement de la configuration (.env)
	cfg := config.LoadConfig()
	// if err != nil {
	// 	log.Printf("Warning: No config file found, using environment variables. Error: %v", err)
	// }

	// Initialisation de la Timezone (Important pour les logs/dates)
	time.Local = time.UTC

	// Connexion Base de Données
	database.Connect()

	// Run SQL migrations (if DATABASE_URL is set). This uses the `migrations` folder.
	migrations.RunMigrations()

	// =========================================================================
	// 1.5 CACHE INITIALIZATION
	// =========================================================================

	// Initialize Redis cache for performance optimization
	redisHost := os.Getenv("REDIS_HOST")
	if redisHost == "" {
		redisHost = "localhost"
	}
	redisPort := os.Getenv("REDIS_PORT")
	if redisPort == "" {
		redisPort = "6379"
	}
	redisPassword := os.Getenv("REDIS_PASSWORD")
	if redisPassword == "" {
		redisPassword = "redis123" // Development default
	}

	var cacheInstance cache.Cache
	var cacheErr error

	// Create Redis cache instance
	cacheInstance, cacheErr = cache.NewRedisCache(
		redisHost,
		redisPort,
		redisPassword,
	)
	if cacheErr != nil {
		log.Printf("Warning: Redis cache initialization failed: %v. Using in-memory cache.", cacheErr)
		cacheInstance = cache.NewMemoryCache()
	} else {
		log.Println("Cache: Redis initialized successfully")
	}
	defer cacheInstance.Close()

	// Initialize caching handler utilities
	cacheableHandlers := handlers.NewCacheableHandlers(cacheInstance)
	log.Println("Cache: Handler utilities initialized")

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
		&domain.CustomField{},
		&domain.CustomFieldTemplate{},
		&domain.BulkOperation{},
		&domain.BulkOperationLog{},
		&domain.Team{},
		&domain.TeamMember{},
		&domain.Connector{},
		&domain.MarketplaceApp{},
		&domain.ConnectorUpdate{},
		&domain.MarketplaceLog{},
	); err != nil {
		log.Fatalf("Database Migration Failed: %v", err)
	}

	// Création du compte Admin par défaut si la DB est vide
	// Cela garantit que l'app est utilisable immédiatement après déploiement.
	handlers.SeedAdminUser()

	// =========================================================================
	// 3. SECURITY SERVICES INITIALIZATION
	// =========================================================================

	// Initialize Permission Service for advanced access control
	permissionService := services.NewPermissionService()
	permissionService.InitializeDefaultRoles()

	// Initialize Token Service for API token management
	tokenService := services.NewTokenService()

	// =========================================================================
	// 4. HEXAGONAL ARCHITECTURE WIRING (Integrations)
	// =========================================================================

	// Initialisation des Adapters (TheHive, OpenRMF, OpenCTI)
	// Ils respectent les interfaces définies dans core/ports
	theHiveAdapter := thehive.NewTheHiveAdapter(cfg.Integrations.TheHive)

	// Initialisation du Moteur de Synchro (Background Worker)
	// Il tourne indépendamment de l'API HTTP
	syncEngine := workers.NewSyncEngine(theHiveAdapter)
	syncEngine.Start(context.Background())

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

	// Initialize auth handler
	authHandler := handlers.NewAuthHandler()

	// Initialize OAuth2 and SAML2 configurations
	handlers.InitializeOAuth2()

	// --- Routes Publiques ---
	api.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":  "UP",
			"version": "1.0.0",
			"db":      "CONNECTED",
		})
	})
	api.Post("/auth/login", authHandler.Login)
	api.Post("/auth/register", authHandler.Register)
	api.Post("/auth/refresh", authHandler.RefreshToken)

	// --- OAuth2 Routes ---
	api.Get("/auth/oauth2/login/:provider", handlers.OAuth2Login)
	api.Get("/auth/oauth2/callback/:provider", handlers.OAuth2Callback)

	// --- SAML2 Routes ---
	api.Get("/auth/saml2/login", handlers.SAML2InitiateLogin)
	api.Post("/auth/saml2/acs", handlers.SAML2ACS)
	api.Get("/auth/saml2/metadata", handlers.SAMLMetadata)

	// --- Routes Protégées (Nécessitent JWT) ---
	// Le middleware injecte user_id et role dans le contexte
	protected := api.Use(middleware.Protected())

	// Dashboard & Analytics (Read-Only accessible à tous les connectés)
	protected.Get("/stats", cacheableHandlers.CacheDashboardStatsGET(handlers.GetDashboardStats))
	protected.Get("/risks",
		middleware.RequirePermissions(permissionService, domain.Permission{
			Resource: domain.PermissionResourceRisk,
			Action:   domain.PermissionRead,
		}),
		cacheableHandlers.CacheRiskListGET(handlers.GetRisks))
	protected.Get("/risks/:id",
		middleware.RequirePermissions(permissionService, domain.Permission{
			Resource: domain.PermissionResourceRisk,
			Action:   domain.PermissionRead,
		}),
		cacheableHandlers.CacheRiskGetByIDGET(handlers.GetRisk))

	// Gestion des Risques (Écriture = Analyst & Admin uniquement)
	// Respect du principe "Simplicité & Sécurité" + Fine-grained Permission Checks
	riskCreate := middleware.RequirePermissions(permissionService, domain.Permission{
		Resource: domain.PermissionResourceRisk,
		Action:   domain.PermissionCreate,
	})
	riskUpdate := middleware.RequirePermissions(permissionService, domain.Permission{
		Resource: domain.PermissionResourceRisk,
		Action:   domain.PermissionUpdate,
	})
	riskDelete := middleware.RequirePermissions(permissionService, domain.Permission{
		Resource: domain.PermissionResourceRisk,
		Action:   domain.PermissionDelete,
	})
	// Backward compatibility: writerRole for other RBAC-based endpoints
	writerRole := middleware.RequireRole("admin", "analyst")

	protected.Post("/risks", riskCreate, handlers.CreateRisk)
	protected.Patch("/risks/:id", riskUpdate, handlers.UpdateRisk)
	protected.Delete("/risks/:id", riskDelete, handlers.DeleteRisk)
	protected.Post("/risks/:id/mitigations", writerRole, handlers.AddMitigation)
	protected.Patch("/mitigations/:mitigationId/toggle", writerRole, handlers.ToggleMitigationStatus)
	protected.Patch("/mitigations/:mitigationId", writerRole, handlers.UpdateMitigation)
	// Sub-actions (checklist) for mitigations
	protected.Post("/mitigations/:id/subactions", writerRole, handlers.CreateMitigationSubAction)
	protected.Patch("/mitigations/:id/subactions/:subactionId/toggle", writerRole, handlers.ToggleMitigationSubAction)
	protected.Delete("/mitigations/:id/subactions/:subactionId", writerRole, handlers.DeleteMitigationSubAction)

	api.Get("/users/me", authHandler.GetProfile)
	api.Get("/assets", middleware.Protected(), handlers.GetAssets)
	api.Post("/assets", middleware.Protected(), handlers.CreateAsset)
	api.Get("/stats/risk-matrix", cacheableHandlers.CacheDashboardMatrixGET(handlers.GetRiskMatrixData))
	api.Get("/stats/risk-distribution", cacheableHandlers.CacheDashboardStatsGET(handlers.GetRiskDistribution))
	api.Get("/stats/mitigation-metrics", cacheableHandlers.CacheDashboardStatsGET(handlers.GetMitigationMetrics))
	api.Get("/stats/top-vulnerabilities", cacheableHandlers.CacheDashboardStatsGET(handlers.GetTopVulnerabilities))
	api.Get("/export/pdf", handlers.ExportRisksPDF)
	api.Get("/stats/trends", middleware.Protected(), cacheableHandlers.CacheDashboardTimelineGET(handlers.GetGlobalRiskTrend))
	api.Get("/mitigations/recommended", handlers.GetRecommendedMitigations)
	api.Get("/gamification/me", middleware.Protected(), handlers.GetMyGamificationProfile)

	// --- User Management (Admin only) ---
	adminRole := middleware.RequireRole("admin")
	protected.Get("/users", adminRole, handlers.GetUsers)
	protected.Post("/users", adminRole, handlers.CreateUser)
	protected.Patch("/users/:id/status", adminRole, handlers.UpdateUserStatus)
	protected.Patch("/users/:id/role", adminRole, handlers.UpdateUserRole)
	protected.Delete("/users/:id", adminRole, handlers.DeleteUser)
	protected.Patch("/users/:id", handlers.UpdateUserProfile)

	// --- Team Management (Admin only) ---
	protected.Post("/teams", adminRole, handlers.CreateTeam)
	protected.Get("/teams", adminRole, handlers.GetTeams)
	protected.Get("/teams/:id", adminRole, handlers.GetTeam)
	protected.Patch("/teams/:id", adminRole, handlers.UpdateTeam)
	protected.Delete("/teams/:id", adminRole, handlers.DeleteTeam)
	protected.Post("/teams/:id/members/:userId", adminRole, handlers.AddTeamMember)
	protected.Delete("/teams/:id/members/:userId", adminRole, handlers.RemoveTeamMember)

	// --- Integration Testing (Protected routes) ---
	protected.Post("/integrations/:id/test", handlers.TestIntegration)

	// --- Audit Logs (Admin only) ---
	auditHandler := handlers.NewAuditLogHandler()
	protected.Get("/audit-logs", adminRole, auditHandler.GetAuditLogs)
	protected.Get("/audit-logs/user/:user_id", adminRole, auditHandler.GetUserAuditLogs)
	protected.Get("/audit-logs/action/:action", adminRole, auditHandler.GetAuditLogsByAction)

	// --- API Token Management (Protected routes) ---
	// Tokens can be managed by any authenticated user for their own tokens
	tokenHandler := handlers.NewTokenHandler(tokenService)

	protected.Post("/tokens", tokenHandler.CreateToken)
	protected.Get("/tokens", tokenHandler.ListTokens)
	protected.Get("/tokens/:id", tokenHandler.GetToken)
	protected.Put("/tokens/:id", tokenHandler.UpdateToken)
	protected.Post("/tokens/:id/revoke", tokenHandler.RevokeToken)
	protected.Post("/tokens/:id/rotate", tokenHandler.RotateToken)
	protected.Delete("/tokens/:id", tokenHandler.DeleteToken)

	// --- Custom Fields Management (Protected routes) ---
	customFieldHandler := handlers.NewCustomFieldHandler()
	protected.Post("/custom-fields", customFieldHandler.CreateCustomField)
	protected.Get("/custom-fields", customFieldHandler.ListCustomFields)
	protected.Get("/custom-fields/:id", customFieldHandler.GetCustomField)
	protected.Patch("/custom-fields/:id", customFieldHandler.UpdateCustomField)
	protected.Delete("/custom-fields/:id", customFieldHandler.DeleteCustomField)
	protected.Get("/custom-fields/scope/:scope", customFieldHandler.ListCustomFieldsByScope)
	protected.Post("/custom-fields/templates/:id/apply", customFieldHandler.ApplyTemplate)

	// --- Bulk Operations (Protected routes) ---
	bulkOpHandler := handlers.NewBulkOperationHandler()
	protected.Post("/bulk-operations", bulkOpHandler.CreateBulkOperation)
	protected.Get("/bulk-operations", bulkOpHandler.ListBulkOperations)
	protected.Get("/bulk-operations/:id", bulkOpHandler.GetBulkOperation)

	// --- Risk Timeline (Protected routes) ---
	timelineHandler := handlers.NewRiskTimelineHandler()
	protected.Get("/risks/:id/timeline", timelineHandler.GetRiskTimeline)
	protected.Get("/risks/:id/timeline/status-changes", timelineHandler.GetStatusChanges)
	protected.Get("/risks/:id/timeline/score-changes", timelineHandler.GetScoreChanges)
	protected.Get("/risks/:id/timeline/trend", timelineHandler.GetRiskTrend)
	protected.Get("/risks/:id/timeline/changes/:type", timelineHandler.GetChangesByType)
	protected.Get("/risks/:id/timeline/since/:timestamp", timelineHandler.GetChangesSince)
	protected.Get("/timeline/recent", timelineHandler.GetRecentActivity)

	// --- Analytics & Advanced Reporting (Protected routes) ---
	analyticsService := services.NewAnalyticsService(database.DB)
	analyticsHandler := handlers.NewAnalyticsHandler(analyticsService)
	protected.Get("/analytics/risks/metrics", analyticsHandler.GetRiskMetrics)
	protected.Get("/analytics/risks/trends", analyticsHandler.GetRiskTrends)
	protected.Get("/analytics/mitigations/metrics", analyticsHandler.GetMitigationMetrics)
	protected.Get("/analytics/frameworks", analyticsHandler.GetFrameworkAnalytics)
	protected.Get("/analytics/dashboard", analyticsHandler.GetDashboardSnapshot)
	protected.Get("/analytics/export", analyticsHandler.GetExportData)

	// --- Incidents Management (Protected routes) ---
	incidentHandler := handlers.NewIncidentHandler(database.DB)
	protected.Get("/incidents", incidentHandler.GetIncidents)
	protected.Get("/incidents/:id", incidentHandler.GetIncident)

	// --- Threat Intelligence (Protected routes) ---
	threatHandler := handlers.NewThreatHandler(database.DB)
	protected.Get("/threats", threatHandler.GetThreats)
	protected.Get("/threats/stats", threatHandler.GetThreatStats)

	// --- Reports Management (Protected routes) ---
	reportHandler := handlers.NewReportHandler(database.DB)
	protected.Get("/reports", reportHandler.GetReports)
	protected.Get("/reports/:id", reportHandler.GetReport)
	protected.Get("/reports/stats", reportHandler.GetReportStats)

	// --- Marketplace Management (Protected routes) ---
	// Marketplace can be browsed by all authenticated users
	// Installation requires analyst or admin role
	marketplaceService := services.NewMarketplaceService(database.DB, log.New(os.Stderr, "[Marketplace] ", log.LstdFlags))
	marketplaceHandler := handlers.NewMarketplaceHandler(marketplaceService)

	// Public marketplace endpoints (all authenticated users can browse)
	protected.Get("/marketplace/connectors", marketplaceHandler.ListConnectors)
	protected.Get("/marketplace/connectors/:id", marketplaceHandler.GetConnector)
	protected.Get("/marketplace/connectors/search", marketplaceHandler.SearchConnectors)

	// Protected marketplace endpoints (analysts and admins only)
	protectedMarketplace := protected.Use(middleware.RequireRole("admin", "analyst"))
	protectedMarketplace.Post("/marketplace/apps", marketplaceHandler.InstallApp)
	protectedMarketplace.Get("/marketplace/apps", marketplaceHandler.ListApps)
	protectedMarketplace.Get("/marketplace/apps/:id", marketplaceHandler.GetApp)
	protectedMarketplace.Put("/marketplace/apps/:id", marketplaceHandler.UpdateApp)
	protectedMarketplace.Post("/marketplace/apps/:id/enable", marketplaceHandler.EnableApp)
	protectedMarketplace.Post("/marketplace/apps/:id/disable", marketplaceHandler.DisableApp)
	protectedMarketplace.Delete("/marketplace/apps/:id", marketplaceHandler.UninstallApp)
	protectedMarketplace.Put("/marketplace/apps/:id/sync", marketplaceHandler.UpdateAppSync)
	protectedMarketplace.Post("/marketplace/apps/:id/sync", marketplaceHandler.TriggerSync)
	protectedMarketplace.Get("/marketplace/apps/:id/logs", marketplaceHandler.GetAppLogs)

	// Connector reviews (all authenticated users can review)
	protected.Post("/marketplace/connectors/:id/reviews", marketplaceHandler.AddConnectorReview)

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
	log.Println("Shutting down server...")

	// Timeout de 5 secondes pour finir les requêtes en cours
	if err := app.Shutdown(); err != nil {
		log.Fatal("Forced shutdown:", err)
	}

	log.Println("Server exited properly")
}

// parseEnvInt safely parses environment variables to integers
func parseEnvInt(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil {
			return parsed
		}
	}
	return defaultVal
}
