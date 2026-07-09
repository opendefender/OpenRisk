// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package main

import (
	"context"
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
	"github.com/rs/zerolog"

	assetapp "github.com/opendefender/openrisk/internal/application/asset"
	"github.com/opendefender/openrisk/internal/application/auth"
	"github.com/opendefender/openrisk/internal/application/compliance"
	notificationapp "github.com/opendefender/openrisk/internal/application/notification"
	"github.com/opendefender/openrisk/internal/application/risk"
	coreauth "github.com/opendefender/openrisk/internal/auth"
	"github.com/opendefender/openrisk/internal/config"
	"github.com/opendefender/openrisk/internal/domain"
	handlers "github.com/opendefender/openrisk/internal/handler"
	authhandler "github.com/opendefender/openrisk/internal/handler/auth"
	"github.com/opendefender/openrisk/internal/infrastructure/database"
	"github.com/opendefender/openrisk/internal/infrastructure/email"
	"github.com/opendefender/openrisk/internal/infrastructure/integrations/thehive"
	redisclient "github.com/opendefender/openrisk/internal/infrastructure/redis"
	"github.com/opendefender/openrisk/internal/infrastructure/repository"
	"github.com/opendefender/openrisk/internal/infrastructure/workers"
	"github.com/opendefender/openrisk/internal/middleware"
	"github.com/opendefender/openrisk/internal/migrations"
	"github.com/opendefender/openrisk/internal/service"
	authpkg "github.com/opendefender/openrisk/pkg/auth"
	"github.com/opendefender/openrisk/pkg/cache"
	"github.com/opendefender/openrisk/pkg/notify"
	"github.com/opendefender/openrisk/pkg/scoring"
	"github.com/opendefender/openrisk/pkg/storage"
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

	var cacheInstance interface{}
	var cacheErr error

	// Create Redis cache instance
	redisCache, cacheErr := cache.NewRedisCache(
		redisHost,
		redisPort,
		redisPassword,
	)
	if cacheErr != nil {
		log.Printf("Warning: Redis cache initialization failed: %v. Using in-memory cache.", cacheErr)
		cacheInstance = cache.NewMemoryCache()
	} else {
		log.Println("Cache: Redis initialized successfully")
		cacheInstance = redisCache
	}
	defer func() {
		if c, ok := cacheInstance.(interface{ Close() error }); ok {
			c.Close()
		}
	}()

	// Initialize caching handler utilities - use Redis if available
	var cacheableHandlers *handlers.CacheableHandlers
	if redisCache != nil && cacheErr == nil {
		cacheableHandlers = handlers.NewCacheableHandlers(redisCache)
		log.Println("Cache: Handler utilities initialized with Redis")
	} else {
		// Create a dummy handler that doesn't cache
		log.Println("Cache: Handler utilities initialized without caching")
		// We'll need to create a wrapper that handles nil gracefully
		// For now, we'll initialize with a placeholder
		_ = cacheableHandlers
	}

	// =========================================================================
	// 2. MIGRATIONS & SEEDING (DevOps Friendly)
	// =========================================================================

	log.Println("Database: Running Auto-Migrations...")
	if err := database.DB.AutoMigrate(
		&domain.User{},
		&domain.Organization{},
		&domain.OrganizationMember{},
		&coreauth.RefreshToken{},
		&domain.Risk{},
		&domain.Mitigation{},
		&domain.Asset{},
		&domain.AssetSnapshot{},
		&domain.RiskHistory{},
		&domain.CustomField{},
		&domain.CustomFieldTemplate{},
		&domain.BulkOperation{},
		&domain.BulkOperationLog{},
		&domain.Team{},
		&domain.TeamMember{},
		// domain.Connector / MarketplaceApp / ConnectorUpdate / MarketplaceLog are intentionally
		// excluded: they carry only `json:` tags (no `gorm:` tags, no primary key), so AutoMigrate
		// fatally errors on them ("unsupported data type"). Marketplace is a pre-existing partial
		// module (see ROADMAP.md) — needs real GORM tagging before it can be added back here.
		&domain.AdminAuditEvent{},
	); err != nil {
		log.Fatalf("Database Migration Failed: %v", err)
	}

	// Run SQL migrations (if DATABASE_URL is set). This uses the `migrations` folder.
	// Must run after AutoMigrate: these SQL migrations add indices/tables/FKs on top of
	// the base tables (users, organizations, risks, ...) that AutoMigrate creates first.
	migrations.RunMigrations()

	// Création du compte Admin par défaut si la DB est vide
	// Cela garantit que l'app est utilisable immédiatement après déploiement.
	handlers.SeedAdminUser()

	// =========================================================================
	// 3. SECURITY SERVICES INITIALIZATION
	// =========================================================================

	// Initialize Permission Service for advanced access control
	permissionService := service.NewPermissionService()
	if err := permissionService.InitializeDefaultRoles(); err != nil {
		log.Fatalf("Failed to initialize default permission roles: %v", err)
	}

	// Initialize Token Service for API token management
	tokenService := service.NewTokenService()

	// Initialize Score Engine Service for automatic risk score calculation
	scoreEngineService := service.NewScoreEngineService(database.DB)
	log.Println("Score Engine: Service initialized with default configuration")

	// =========================================================================
	// 3.5 JWT RS256 & SCORE WORKER INITIALIZATION (Critical Security & Events)
	// =========================================================================

	// Load RSA keys for JWT RS256 (fail-fast if missing)
	rsaKeys := authpkg.MustLoadRSAKeys(
		cfg.Server.RSAPrivateKeyPath,
		cfg.Server.RSAPublicKeyPath,
	)
	log.Println("Auth: RSA keys loaded successfully for JWT RS256")

	// Initialize Redis client for caching, events, and JWT blacklist
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		redisURL = "redis://localhost:6379"
	}
	redisClientInstance := redisclient.NewClient(redisURL)
	log.Println("Redis: Client connected for events, caching, and JWT blacklist")

	// Initialize JWT token blacklist manager (via Redis)
	tokenBlacklistManager := authpkg.NewTokenBlacklistManager(redisClientInstance)
	log.Println("Auth: Token blacklist manager initialized (Redis-backed)")

	// Closure used by middleware.Protected(rsaKeys, jtiBlacklistChecker) to check the JTI blacklist on every request
	jtiBlacklistChecker := tokenBlacklistManager.CheckJTIBlacklist(context.Background())

	// Initialize Score Engine (pure, stateless)
	scoreEngine := scoring.NewEngine()
	log.Println("Scoring: Engine initialized (pure, zero dependencies)")

	// Initialize file storage (compliance evidence, etc.)
	// STORAGE_DRIVER selects the backend; only "local" exists today. An
	// S3-backed driver can be added later behind the same storage.Storage
	// interface without touching any use case or handler.
	storageLocalPath := os.Getenv("STORAGE_LOCAL_PATH")
	if storageLocalPath == "" {
		storageLocalPath = "./uploads"
	}
	fileStorage, err := storage.NewLocalStorage(storageLocalPath)
	if err != nil {
		log.Fatal("Storage: failed to initialize local storage: ", err)
	}
	log.Println("Storage: local driver initialized at", storageLocalPath)

	// Initialize Score Worker (listens to Redis events)
	zeroLogger := zerolog.New(os.Stderr).With().Timestamp().Logger()
	riskRepoForWorker := repository.NewGormRiskRepository(database.DB)
	scoreWorker := workers.NewScoreWorker(redisClientInstance, scoreEngine, riskRepoForWorker, zeroLogger)

	// Start Score Worker in background goroutine
	go scoreWorker.Start(context.Background())
	log.Println("Workers: Score Engine worker started (listening for risk.updated events)")

	// =========================================================================
	// 4. HEXAGONAL ARCHITECTURE WIRING (Integrations)
	// =========================================================================

	// Initialisation des Adapters (TheHive, OpenRMF, OpenCTI)
	// Ils respectent les interfaces définies dans core/ports
	theHiveAdapter := thehive.NewTheHiveAdapter(cfg.Integrations.TheHive)

	// Get organization ID for SyncEngine (multi-tenant scoping - Rule 1)
	// In a multi-tenant setup, there would be one SyncEngine per organization
	// For now, we use the default organization from environment or placeholder
	organizationID := os.Getenv("SYNC_ORGANIZATION_ID")
	if organizationID == "" {
		// Fall back to first organization in DB or placeholder
		// TODO: In production, each organization should have its own SyncEngine instance
		organizationID = "550e8400-e29b-41d4-a716-446655440000" // Default placeholder
		log.Println("Warning: SYNC_ORGANIZATION_ID not set, using default placeholder. Set this env var for proper multi-tenant operation.")
	}

	// Initialisation du Moteur de Synchro (Background Worker)
	// Il tourne indépendamment de l'API HTTP
	syncEngine := workers.NewSyncEngine(theHiveAdapter, organizationID)
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

	// =========================================================================
	// 5.1 CLEAN ARCHITECTURE AUTH MODULE INITIALIZATION
	// =========================================================================

	// Initialize repositories
	userRepo := repository.NewGormUserRepository(database.DB)
	orgRepo := repository.NewGormOrganizationRepository(database.DB)

	// Initialize notification service (mock email transport - swap for SMTP in production)
	emailFromAddr := os.Getenv("EMAIL_FROM")
	if emailFromAddr == "" {
		emailFromAddr = "noreply@openrisk.local"
	}
	appBaseURL := os.Getenv("APP_BASE_URL")
	if appBaseURL == "" {
		appBaseURL = "http://localhost:5173"
	}
	notificationService := notify.NewEmailService(email.NewMockService(), emailFromAddr, appBaseURL)

	// Initialize password hasher (Argon2id, OWASP recommended — matches handlers.SeedAdminUser)
	passwordHasher := coreauth.NewArgon2idPasswordHasher()

	// Initialize token manager (access/refresh JWT pairs, backed by the DB).
	// internal/auth has its own RSAKeys type (distinct from pkg/auth's, used for
	// middleware.Protected above) — load the same PEM files into that type here.
	coreRSAKeys := coreauth.MustLoadRSAKeys(cfg.Server.RSAPrivateKeyPath, cfg.Server.RSAPublicKeyPath)
	tokenManager := coreauth.NewTokenManager(database.DB, coreRSAKeys)

	// Initialize use cases
	loginUseCase := auth.NewLoginUseCase(userRepo, tokenManager, passwordHasher)
	registerUseCase := auth.NewRegisterUseCase(userRepo, orgRepo, notificationService, passwordHasher)
	refreshUseCase := auth.NewRefreshTokenUseCase(tokenManager)
	logoutUseCase := auth.NewLogoutUseCase(tokenManager)

	// Initialize Clean Architecture auth handler
	cleanAuthHandler := authhandler.NewHandler(
		loginUseCase,
		registerUseCase,
		refreshUseCase,
		logoutUseCase,
		passwordHasher,
	)

	// Initialize legacy auth handler (for backward compatibility)
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

	// Clean Architecture Auth Routes
	api.Post("/auth/login", cleanAuthHandler.Login)
	api.Post("/auth/register", cleanAuthHandler.Register)
	api.Post("/auth/refresh", cleanAuthHandler.RefreshToken)
	api.Post("/auth/logout", cleanAuthHandler.Logout)

	// Legacy Auth Routes (for backward compatibility)
	api.Post("/auth/legacy/login", authHandler.Login)
	api.Post("/auth/legacy/refresh", authHandler.RefreshToken)

	// --- OAuth2 Routes ---
	api.Get("/auth/oauth2/login/:provider", handlers.OAuth2Login)
	api.Get("/auth/oauth2/callback/:provider", handlers.OAuth2Callback)

	// --- SAML2 Routes ---
	api.Get("/auth/saml2/login", handlers.SAML2InitiateLogin)
	api.Post("/auth/saml2/acs", handlers.SAML2ACS)
	api.Get("/auth/saml2/metadata", handlers.SAMLMetadata)

	// --- Routes Protégées (Nécessitent JWT) ---
	// Le middleware injecte user_id et role dans le contexte
	protected := api.Use(middleware.Protected(rsaKeys, jtiBlacklistChecker))

	// Current user profile endpoint
	api.Get("/auth/me", middleware.Protected(rsaKeys, jtiBlacklistChecker), cleanAuthHandler.Me)
	api.Get("/users/me", authHandler.GetProfile)

	// Dashboard & Analytics (Read-Only accessible à tous les connectés)
	protected.Get("/stats", cacheableHandlers.CacheDashboardStatsGET(handlers.GetDashboardStats))

	// Initialize clean architecture risk module
	riskRepo := repository.NewGormRiskRepository(database.DB)
	createRiskUseCase := risk.NewCreateRiskUseCase(riskRepo)
	getRiskUseCase := risk.NewGetRiskUseCase(riskRepo)
	listRisksUseCase := risk.NewListRisksUseCase(riskRepo)
	updateRiskUseCase := risk.NewUpdateRiskUseCase(riskRepo)
	deleteRiskUseCase := risk.NewDeleteRiskUseCase(riskRepo)
	riskHandler := handlers.NewRiskHandler(createRiskUseCase, getRiskUseCase, listRisksUseCase, updateRiskUseCase, deleteRiskUseCase, redisClientInstance)

	// NOTE: same bug class as compliance (see comment above complianceFrameworkRead) —
	// middleware.RequirePermissions reads the legacy *domain.UserClaims, which the RS256
	// middleware on `protected` never populates. Using middleware.RequirePermission instead.
	protected.Get("/risks",
		middleware.RequirePermission("risks:read"),
		cacheableHandlers.CacheRiskListGET(riskHandler.GetRisks))
	protected.Get("/risks/:id",
		middleware.RequirePermission("risks:read"),
		cacheableHandlers.CacheRiskGetByIDGET(riskHandler.GetRisk))

	// Gestion des Risques (Écriture = Analyst & Admin uniquement)
	// Respect du principe "Simplicité & Sécurité" + Fine-grained Permission Checks
	riskCreate := middleware.RequirePermission("risks:create")
	riskUpdate := middleware.RequirePermission("risks:update")
	riskDelete := middleware.RequirePermission("risks:delete")

	protected.Post("/risks", riskCreate, riskHandler.CreateRisk)
	protected.Patch("/risks/:id", riskUpdate, riskHandler.UpdateRisk)
	protected.Delete("/risks/:id", riskDelete, riskHandler.DeleteRisk)

	// Mitigation Plans (CRUD). NOTE: this whole module previously used
	// middleware.RequireRole ("writerRole"), which reads c.Locals("role") — a flat
	// string AuthMiddlewareRS256 never sets (it sets "org_roles", a map, instead).
	// Every mitigation route has therefore been returning 401 "No role in token" for
	// every request regardless of caller — the same legacy-middleware bug class
	// already fixed for /risks and /compliance/*, just never applied here. Switched
	// to middleware.RequirePermission to match those two modules.
	mitigationRead := middleware.RequirePermission("mitigations:read")
	mitigationCreate := middleware.RequirePermission("mitigations:create")
	mitigationUpdate := middleware.RequirePermission("mitigations:update")
	mitigationDelete := middleware.RequirePermission("mitigations:delete")
	// Still used by Incidents/Risk-Management routes below — now fixed to read
	// org_roles correctly (see middleware.RequireRole's doc comment).
	writerRole := middleware.RequireRole("admin", "analyst")

	protected.Get("/mitigations", mitigationRead, handlers.ListMitigations)
	protected.Post("/risks/:id/mitigations", mitigationCreate, handlers.CreateMitigation)
	protected.Get("/mitigations/:id", mitigationRead, handlers.GetMitigation)
	protected.Get("/risks/:id/mitigations", mitigationRead, handlers.ListMitigationsByRisk)
	protected.Patch("/mitigations/:id", mitigationUpdate, handlers.UpdateMitigation)
	protected.Delete("/mitigations/:id", mitigationDelete, handlers.DeleteMitigation)
	protected.Patch("/mitigations/:id/validate", mitigationUpdate, handlers.ValidateMitigation)

	// Sub-actions (checklist) for mitigations
	protected.Post("/mitigations/:id/sub-actions", mitigationCreate, handlers.CreateSubAction)
	protected.Patch("/mitigations/:id/sub-actions/:aid", mitigationUpdate, handlers.UpdateSubAction)
	protected.Post("/mitigations/:id/sub-actions/:aid/complete", mitigationUpdate, handlers.CompleteSubAction)
	protected.Post("/mitigations/:id/sub-actions/:aid/revert", mitigationUpdate, handlers.RevertSubAction)
	protected.Delete("/mitigations/:id/sub-actions/:aid", mitigationDelete, handlers.DeleteSubAction)
	protected.Patch("/mitigations/:id/reorder-subactions", mitigationUpdate, handlers.ReorderSubActions)

	// Scanner webhook for auto-completion (internal API key auth)
	protected.Post("/scanner/mitigations/auto-complete", handlers.AutoCompleteMitigationSubAction)

	// Compliance Frameworks (M1 — see ROADMAP.md §3)
	complianceRepo := repository.NewGormComplianceRepository(database.DB)
	createFrameworkUC := compliance.NewCreateFrameworkUseCase(complianceRepo)
	getFrameworkUC := compliance.NewGetFrameworkUseCase(complianceRepo)
	listFrameworksUC := compliance.NewListFrameworksUseCase(complianceRepo)
	deleteFrameworkUC := compliance.NewDeleteFrameworkUseCase(complianceRepo)
	createControlUC := compliance.NewCreateControlUseCase(complianceRepo)
	getControlUC := compliance.NewGetControlUseCase(complianceRepo)
	listControlsUC := compliance.NewListControlsUseCase(complianceRepo)
	updateControlUC := compliance.NewUpdateControlUseCase(complianceRepo)
	deleteControlUC := compliance.NewDeleteControlUseCase(complianceRepo)
	createEvidenceUC := compliance.NewCreateEvidenceUseCase(complianceRepo, fileStorage)
	listEvidencesUC := compliance.NewListEvidencesUseCase(complianceRepo)
	deleteEvidenceUC := compliance.NewDeleteEvidenceUseCase(complianceRepo, fileStorage)
	downloadEvidenceUC := compliance.NewDownloadEvidenceUseCase(complianceRepo, fileStorage)
	getProgressUC := compliance.NewGetComplianceProgressUseCase(complianceRepo)
	listCatalogsUC := compliance.NewListCatalogsUseCase()
	importCatalogUC := compliance.NewImportCatalogUseCase(complianceRepo)
	// M4 — official compliance report (PDF). Reuses userRepo/orgRepo (declared
	// above) to resolve the "generated by" and organization identity lines.
	generateReportUC := compliance.NewGenerateComplianceReportUseCase(complianceRepo, orgRepo, userRepo)
	complianceHandler := handlers.NewComplianceHandler(
		createFrameworkUC, getFrameworkUC, listFrameworksUC, deleteFrameworkUC,
		createControlUC, getControlUC, listControlsUC, updateControlUC, deleteControlUC,
		createEvidenceUC, listEvidencesUC, deleteEvidenceUC, downloadEvidenceUC,
		getProgressUC, listCatalogsUC, importCatalogUC, generateReportUC,
	)

	// NOTE: these routes sit under `protected`, whose base middleware (middleware.Protected,
	// RS256) stores the *new* multi-tenant claims in c.Locals("user")/("permissions"). The
	// legacy middleware.RequirePermissions expects the old HMAC-era *domain.UserClaims instead,
	// so it always failed the type assertion here ("user context not found", 401 on every
	// request) — the granular admin/analyst/viewer split below was never actually reachable.
	// middleware.RequirePermission (singular) is the one that matches what's really in context;
	// domain.PermissionSet only grants "*" to root/admin org members today (no per-resource
	// Profile rules for compliance yet), so this is admin/root-only until that's extended.
	complianceFrameworkRead := middleware.RequirePermission("compliance:frameworks:read")
	complianceFrameworkCreate := middleware.RequirePermission("compliance:frameworks:create")
	complianceFrameworkDelete := middleware.RequirePermission("compliance:frameworks:delete")
	complianceControlRead := middleware.RequirePermission("compliance:controls:read")
	complianceControlCreate := middleware.RequirePermission("compliance:controls:create")
	complianceControlUpdate := middleware.RequirePermission("compliance:controls:update")
	complianceControlDelete := middleware.RequirePermission("compliance:controls:delete")
	complianceEvidenceRead := middleware.RequirePermission("compliance:evidences:read")
	complianceEvidenceCreate := middleware.RequirePermission("compliance:evidences:create")
	complianceEvidenceDelete := middleware.RequirePermission("compliance:evidences:delete")

	// Catalogs are static regulatory reference data (pkg/compliance) — global, read-only,
	// same permission tier as listing frameworks. Importing one instantiates controls under
	// a tenant's own framework, so it's gated the same as creating a framework by hand.
	protected.Get("/compliance/catalogs", complianceFrameworkRead, complianceHandler.ListCatalogs)
	protected.Post("/compliance/frameworks/:frameworkId/import-catalog", complianceFrameworkCreate, complianceHandler.ImportCatalog)

	protected.Get("/compliance/frameworks", complianceFrameworkRead, complianceHandler.ListFrameworks)
	protected.Post("/compliance/frameworks", complianceFrameworkCreate, complianceHandler.CreateFramework)
	protected.Get("/compliance/frameworks/:frameworkId", complianceFrameworkRead, complianceHandler.GetFramework)
	protected.Delete("/compliance/frameworks/:frameworkId", complianceFrameworkDelete, complianceHandler.DeleteFramework)
	protected.Get("/compliance/frameworks/:frameworkId/progress", complianceControlRead, complianceHandler.GetProgress)
	// Official compliance report (PDF, 1-click) — reads a tenant's controls/evidence, same tier as reading them.
	protected.Get("/compliance/frameworks/:frameworkId/report", complianceControlRead, complianceHandler.GenerateReport)
	protected.Get("/compliance/frameworks/:frameworkId/controls", complianceControlRead, complianceHandler.ListControls)
	protected.Post("/compliance/frameworks/:frameworkId/controls", complianceControlCreate, complianceHandler.CreateControl)
	protected.Get("/compliance/controls/:controlId", complianceControlRead, complianceHandler.GetControl)
	protected.Patch("/compliance/controls/:controlId", complianceControlUpdate, complianceHandler.UpdateControl)
	protected.Delete("/compliance/controls/:controlId", complianceControlDelete, complianceHandler.DeleteControl)
	protected.Get("/compliance/controls/:controlId/evidences", complianceEvidenceRead, complianceHandler.ListEvidences)
	protected.Post("/compliance/controls/:controlId/evidences", complianceEvidenceCreate, complianceHandler.CreateEvidence)
	protected.Get("/compliance/evidences/:evidenceId/download", complianceEvidenceRead, complianceHandler.DownloadEvidence)
	protected.Delete("/compliance/evidences/:evidenceId", complianceEvidenceDelete, complianceHandler.DeleteEvidence)

	// Assets (M3 — see ROADMAP.md §3). Previously these two routes bypassed
	// RBAC entirely (any authenticated user, any role, could write inventory
	// data) — now gated the same way as risks/compliance.
	assetRepo := repository.NewGormAssetRepository(database.DB)
	createAssetUC := assetapp.NewCreateAssetUseCase(assetRepo)
	getAssetUC := assetapp.NewGetAssetUseCase(assetRepo)
	listAssetsUC := assetapp.NewListAssetsUseCase(assetRepo)
	updateAssetUC := assetapp.NewUpdateAssetUseCase(assetRepo)
	deleteAssetUC := assetapp.NewDeleteAssetUseCase(assetRepo)
	listAssetSnapshotsUC := assetapp.NewListAssetSnapshotsUseCase(assetRepo)
	assetHandler := handlers.NewAssetHandler(
		createAssetUC, getAssetUC, listAssetsUC, updateAssetUC, deleteAssetUC, listAssetSnapshotsUC,
		redisClientInstance,
	)

	assetRead := middleware.RequirePermission("assets:read")
	assetCreate := middleware.RequirePermission("assets:create")
	assetUpdate := middleware.RequirePermission("assets:update")
	assetDelete := middleware.RequirePermission("assets:delete")

	protected.Get("/assets", assetRead, assetHandler.ListAssets)
	protected.Post("/assets", assetCreate, assetHandler.CreateAsset)
	protected.Get("/assets/:id", assetRead, assetHandler.GetAsset)
	protected.Patch("/assets/:id", assetUpdate, assetHandler.UpdateAsset)
	protected.Delete("/assets/:id", assetDelete, assetHandler.DeleteAsset)
	protected.Get("/assets/:id/history", assetRead, assetHandler.GetAssetHistory)

	api.Get("/users/me", authHandler.GetProfile)
	api.Get("/stats/risk-matrix", cacheableHandlers.CacheDashboardMatrixGET(handlers.GetRiskMatrixData))
	api.Get("/stats/risk-distribution", cacheableHandlers.CacheDashboardStatsGET(handlers.GetRiskDistribution))
	api.Get("/stats/mitigation-metrics", cacheableHandlers.CacheDashboardStatsGET(handlers.GetMitigationMetrics))
	api.Get("/stats/top-vulnerabilities", cacheableHandlers.CacheDashboardStatsGET(handlers.GetTopVulnerabilities))
	api.Get("/export/pdf", handlers.ExportRisksPDF)
	api.Get("/stats/trends", middleware.Protected(rsaKeys, jtiBlacklistChecker), cacheableHandlers.CacheDashboardTimelineGET(handlers.GetGlobalRiskTrend))
	// TODO(Phase 3): Reconnect /mitigations/recommended once the handler is
	// properly implemented with tests and pagination.
	// api.Get("/mitigations/recommended", handlers.GetRecommendedMitigations)
	api.Get("/gamification/me", middleware.Protected(rsaKeys, jtiBlacklistChecker), handlers.GetMyGamificationProfile)

	// --- Score Engine Management (Protected routes) ---
	scoreEngineHandler := handlers.NewScoreEngineHandler(database.DB, scoreEngineService)
	scoreEngineRoutes := protected.Group("/score-engine")
	scoreEngineRoutes.Get("/configs", scoreEngineHandler.GetScoringConfigs)
	scoreEngineRoutes.Get("/configs/:id", scoreEngineHandler.GetScoringConfig)
	scoreEngineRoutes.Post("/configs", middleware.RequireRole("admin"), scoreEngineHandler.CreateScoringConfig)
	scoreEngineRoutes.Put("/configs/:id", middleware.RequireRole("admin"), scoreEngineHandler.UpdateScoringConfig)
	scoreEngineRoutes.Post("/compute", scoreEngineHandler.ComputeRiskScore)
	scoreEngineRoutes.Get("/matrix", scoreEngineHandler.GetRiskMatrix)
	scoreEngineRoutes.Post("/classify", scoreEngineHandler.ClassifyRisk)
	scoreEngineRoutes.Get("/metrics", scoreEngineHandler.GetScoringMetrics)

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

	// --- Incidents (Protected routes) ---
	incidentService := service.NewIncidentService(database.DB)
	incidentHandler := handlers.NewIncidentHandler(incidentService)
	incidentsGroup := protected.Group("/incidents")
	incidentsGroup.Post("", writerRole, incidentHandler.CreateIncident)
	incidentsGroup.Get("/stats", incidentHandler.GetIncidentStats)
	incidentsGroup.Get("", incidentHandler.ListIncidents)
	incidentsGroup.Get("/:id", incidentHandler.GetIncident)
	incidentsGroup.Put("/:id", writerRole, incidentHandler.UpdateIncident)
	incidentsGroup.Delete("/:id", writerRole, incidentHandler.DeleteIncident)
	incidentsGroup.Get("/:id/timeline", incidentHandler.GetIncidentTimeline)
	incidentsGroup.Post("/:id/risks/:riskId", writerRole, incidentHandler.LinkRisk)
	incidentsGroup.Post("/:id/actions", writerRole, incidentHandler.CreateIncidentAction)
	incidentsGroup.Get("/:id/actions", incidentHandler.GetIncidentActions)
	incidentsGroup.Put("/:id/actions/:actionId", writerRole, incidentHandler.UpdateIncidentAction)
	protected.Get("/risks/:id/incidents", incidentHandler.GetIncidentsForRisk)

	// --- Risk Management Lifecycle (Protected routes) ---
	riskMgmtService := service.NewRiskManagementService(database.DB)
	riskMgmtHandler := handlers.NewRiskManagementHandler(riskMgmtService)
	riskMgmtGroup := protected.Group("/risk-management")
	riskMgmtGroup.Post("/identify", writerRole, riskMgmtHandler.IdentifyRisk)
	riskMgmtGroup.Post("/analyze", writerRole, riskMgmtHandler.AnalyzeRisk)
	riskMgmtGroup.Post("/evaluate", writerRole, riskMgmtHandler.EvaluateRisk)
	riskMgmtGroup.Post("/treatment-plans", writerRole, riskMgmtHandler.CreateTreatmentPlan)
	riskMgmtGroup.Post("/treatment-plans/:id/actions", writerRole, riskMgmtHandler.AddTreatmentAction)
	riskMgmtGroup.Post("/monitoring-reviews", writerRole, riskMgmtHandler.CreateMonitoringReview)
	riskMgmtGroup.Post("/decisions", writerRole, riskMgmtHandler.RecordDecision)
	riskMgmtGroup.Post("/decisions/:id/approve", writerRole, riskMgmtHandler.ApproveDecision)
	riskMgmtGroup.Post("/audit-reports", riskMgmtHandler.GenerateAuditReport)
	riskMgmtGroup.Get("/risks/:id/lifecycle-status", riskMgmtHandler.GetRiskLifecycleStatus)

	// --- Notifications (Protected routes) ---
	notificationRepo := repository.NewNotificationRepository(database.DB)
	notificationUseCase := notificationapp.NewUseCase(notificationRepo)
	notificationHandler := handlers.NewNotificationHandler(notificationUseCase)
	notificationsGroup := protected.Group("/notifications")
	notificationsGroup.Get("", notificationHandler.GetNotifications)
	notificationsGroup.Get("/unread-count", notificationHandler.GetUnreadCount)
	notificationsGroup.Patch("/read-all", notificationHandler.MarkAllAsRead)
	notificationsGroup.Patch("/:notificationId/read", notificationHandler.MarkAsRead)
	notificationsGroup.Delete("/:notificationId", notificationHandler.DeleteNotification)
	notificationsGroup.Get("/preferences", notificationHandler.GetNotificationPreferences)
	notificationsGroup.Patch("/preferences", notificationHandler.UpdateNotificationPreferences)
	notificationsGroup.Post("/test", notificationHandler.TestNotification)

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
	analyticsService := service.NewAnalyticsService(database.DB)
	analyticsHandler := handlers.NewAnalyticsHandler(analyticsService)
	protected.Get("/analytics/risks/metrics", analyticsHandler.GetRiskMetrics)
	protected.Get("/analytics/risks/trends", analyticsHandler.GetRiskTrends)
	protected.Get("/analytics/mitigations/metrics", analyticsHandler.GetMitigationMetrics)
	protected.Get("/analytics/frameworks", analyticsHandler.GetFrameworkAnalytics)
	protected.Get("/analytics/dashboard", analyticsHandler.GetDashboardSnapshot)
	protected.Get("/analytics/export", analyticsHandler.GetExportData)

	// --- Enhanced Dashboard Analytics (Protected routes) ---
	dashboardDataService := service.NewDashboardDataService(database.DB, nil)
	enhancedDashboardHandler := handlers.NewEnhancedDashboardHandler(dashboardDataService)
	protected.Get("/dashboard/metrics", enhancedDashboardHandler.GetDashboardMetrics)
	protected.Get("/dashboard/risk-trends", enhancedDashboardHandler.GetRiskTrends)
	protected.Get("/dashboard/severity-distribution", enhancedDashboardHandler.GetSeverityDistribution)
	protected.Get("/dashboard/mitigation-status", enhancedDashboardHandler.GetMitigationStatus)
	protected.Get("/dashboard/top-risks", enhancedDashboardHandler.GetTopRisks)
	protected.Get("/dashboard/mitigation-progress", enhancedDashboardHandler.GetMitigationProgress)
	protected.Get("/dashboard/complete", enhancedDashboardHandler.GetCompleteDashboard)

	// --- Marketplace Management (Protected routes) ---
	// Marketplace can be browsed by all authenticated users
	// Installation requires analyst or admin role
	marketplaceService := service.NewMarketplaceService(database.DB, log.New(os.Stderr, "[Marketplace] ", log.LstdFlags))
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
	// 5.5 RBAC MANAGEMENT ENDPOINTS
	// =========================================================================

	// Initialize RBAC services
	rbacUserService := service.NewUserService(database.DB)
	rbacRoleService := service.NewRoleService(database.DB)
	rbacTenantService := service.NewTenantService(database.DB)

	// Initialize RBAC handlers
	rbacUserHandler := handlers.NewRBACUserHandler(rbacUserService, rbacRoleService, rbacTenantService)
	rbacRoleHandler := handlers.NewRBACRoleHandler(rbacRoleService, permissionService, rbacUserService)
	rbacTenantHandler := handlers.NewRBACTenantHandler(rbacTenantService, rbacUserService)

	// User Management Endpoints (admin-only)
	rbacUsers := protected.Group("/rbac/users", adminRole)
	rbacUsers.Get("", rbacUserHandler.ListUsers)
	rbacUsers.Post("", rbacUserHandler.AddUserToTenant)
	rbacUsers.Get("/:user_id", rbacUserHandler.GetUser)
	rbacUsers.Patch("/:user_id/role", rbacUserHandler.ChangeUserRole)
	rbacUsers.Delete("/:user_id", rbacUserHandler.RemoveUserFromTenant)
	rbacUsers.Get("/:user_id/permissions", rbacUserHandler.GetUserPermissions)
	rbacUsers.Get("/stats", rbacUserHandler.GetTenantUserStats)

	// Role Management Endpoints (admin-only)
	rbacRoles := protected.Group("/rbac/roles", adminRole)
	rbacRoles.Get("", rbacRoleHandler.ListRoles)
	rbacRoles.Post("", rbacRoleHandler.CreateRole)
	rbacRoles.Get("/:role_id", rbacRoleHandler.GetRole)
	rbacRoles.Patch("/:role_id", rbacRoleHandler.UpdateRole)
	rbacRoles.Delete("/:role_id", rbacRoleHandler.DeleteRole)
	rbacRoles.Get("/:role_id/permissions", rbacRoleHandler.GetRolePermissions)
	rbacRoles.Post("/:role_id/permissions", rbacRoleHandler.AssignPermissionToRole)
	rbacRoles.Delete("/:role_id/permissions", rbacRoleHandler.RemovePermissionFromRole)

	// Tenant Management Endpoints
	rbacTenants := protected.Group("/rbac/tenants")
	rbacTenants.Get("", rbacTenantHandler.ListTenants)                                // Users see their own tenants
	rbacTenants.Post("", rbacTenantHandler.CreateTenant)                              // Create new tenant
	rbacTenants.Get("/:tenant_id", rbacTenantHandler.GetTenant)                       // Get tenant details
	rbacTenants.Patch("/:tenant_id", adminRole, rbacTenantHandler.UpdateTenant)       // Update (admin only)
	rbacTenants.Delete("/:tenant_id", rbacTenantHandler.DeleteTenant)                 // Delete (owner only)
	rbacTenants.Get("/:tenant_id/users", adminRole, rbacTenantHandler.GetTenantUsers) // List users
	rbacTenants.Get("/:tenant_id/stats", rbacTenantHandler.GetTenantStats)            // Get stats

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
