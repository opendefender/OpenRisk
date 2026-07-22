// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package main

import (
	"context"
	"crypto/sha256"
	"fmt"
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
	"github.com/google/uuid"
	"github.com/rs/zerolog"

	appai "github.com/opendefender/openrisk/internal/application/ai"
	assetapp "github.com/opendefender/openrisk/internal/application/asset"
	"github.com/opendefender/openrisk/internal/application/auth"
	appauto "github.com/opendefender/openrisk/internal/application/automation"
	"github.com/opendefender/openrisk/internal/application/board"
	"github.com/opendefender/openrisk/internal/application/compliance"
	"github.com/opendefender/openrisk/internal/application/complianceaudit"
	"github.com/opendefender/openrisk/internal/application/governance"
	appmitigation "github.com/opendefender/openrisk/internal/application/mitigation"
	notificationapp "github.com/opendefender/openrisk/internal/application/notification"
	"github.com/opendefender/openrisk/internal/application/risk"
	scanapp "github.com/opendefender/openrisk/internal/application/scanner"
	vulnapp "github.com/opendefender/openrisk/internal/application/vulnerability"
	coreauth "github.com/opendefender/openrisk/internal/auth"
	"github.com/opendefender/openrisk/internal/config"
	"github.com/opendefender/openrisk/internal/domain"
	handlers "github.com/opendefender/openrisk/internal/handler"
	authhandler "github.com/opendefender/openrisk/internal/handler/auth"
	audittrailinfra "github.com/opendefender/openrisk/internal/infrastructure/audittrail"
	autoinfra "github.com/opendefender/openrisk/internal/infrastructure/automation"
	"github.com/opendefender/openrisk/internal/infrastructure/ctimatch"
	"github.com/opendefender/openrisk/internal/infrastructure/database"
	"github.com/opendefender/openrisk/internal/infrastructure/email"
	"github.com/opendefender/openrisk/internal/infrastructure/integrations/thehive"
	redisclient "github.com/opendefender/openrisk/internal/infrastructure/redis"
	"github.com/opendefender/openrisk/internal/infrastructure/repository"
	"github.com/opendefender/openrisk/internal/infrastructure/scanmitigation"
	"github.com/opendefender/openrisk/internal/infrastructure/vulnrisk"
	"github.com/opendefender/openrisk/internal/infrastructure/workers"
	"github.com/opendefender/openrisk/internal/middleware"
	"github.com/opendefender/openrisk/internal/migrations"
	scanpkg "github.com/opendefender/openrisk/internal/scanner"
	"github.com/opendefender/openrisk/internal/scanner/collectors"
	"github.com/opendefender/openrisk/internal/service"
	"github.com/opendefender/openrisk/pkg/ai"
	authpkg "github.com/opendefender/openrisk/pkg/auth"
	"github.com/opendefender/openrisk/pkg/cache"
	"github.com/opendefender/openrisk/pkg/crq"
	"github.com/opendefender/openrisk/pkg/cti"
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
		// L2/L4/L5/L7 auth tables. Previously absent from AutoMigrate, so the whole
		// feature set (MFA setup/challenge, PAT auth, SSO account linking, and the
		// full-fidelity auth audit trail) errored on non-existent tables.
		&domain.MFASecret{},
		&domain.MFABackupCode{},
		&domain.PersonalAccessToken{},
		&domain.OAuthProvider{},
		&domain.AuthAuditLog{},
		&domain.Risk{},
		// Smart Risk Calculation (spec §8) — per-tenant configurable weights for the
		// eight-factor multifactor score. The smart-score columns themselves live on
		// domain.Risk above. Additive; the classic Score Engine is untouched.
		&domain.RiskScoringWeights{},
		&domain.Mitigation{},
		&domain.Asset{},
		&domain.AssetSnapshot{},
		// Directed edges of the asset dependency graph ("cartographie des
		// dépendances"). Tenant-scoped; both endpoints reference assets.
		&domain.AssetDependency{},
		// Cross-framework control crosswalks ("cross-mapping entre référentiels").
		// Tenant-scoped undirected links between two compliance controls.
		&domain.ControlMapping{},
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
		// M4 (second half) — monthly board-of-directors report (draft → approved),
		// with a per-tenant posture snapshot and an editable AI/template narrative.
		&domain.BoardReport{},
		// RBAC + audit + multi-tenant tables. These back the Settings admin tabs
		// (Roles / Organizations / Audit log). RoleEnhanced maps onto the existing
		// "roles" table and only ADDS columns (tenant_id/level/is_predefined/...);
		// it never drops the legacy Role columns. Seeded by SeedRBAC() below.
		&domain.PermissionDB{},
		&domain.RoleEnhanced{},
		&domain.RolePermission{},
		&domain.Tenant{},
		&domain.UserTenant{},
		&domain.AuditLog{},
		// M5 (Incident Management) — the incident register + its timeline and
		// mitigation actions. Previously missing here, so every /incidents route
		// 500'd on a non-existent table; War Room stayed a fixture-only preview.
		&domain.Incident{},
		&domain.IncidentTimeline{},
		&domain.IncidentAction{},
		// Scanner engine — tenant-scoped scan configs, on-prem Agents, and scan
		// jobs. The pipeline never writes Assets/Risks itself: results land in a
		// Redis preview (48h TTL) and the user imports/ignores from there.
		&domain.ScanConfig{},
		&domain.ScannerAgent{},
		&domain.ScanJob{},
		// CTI / Intel Threat — vulnerabilities pulled from NVD + CISA KEV, enriched
		// with MITRE ATT&CK. Matched against asset CPEs to auto-create risks.
		&cti.CTIVulnerability{},
		// Vulnerability Management (Module 3) — the tenant-scoped vulnerability
		// register: findings normalised from Nessus/OpenVAS/Qualys/Defender/
		// Inspector/Azure Defender/CrowdStrike and risk-based prioritised.
		&domain.Vulnerability{},
		// Vulnerability integrations — per-source connector config (encrypted API
		// credentials, live-pull schedule, inbound webhook token, automation
		// toggles) + tenant ITSM/ticketing config for auto-ticketing.
		&domain.VulnIntegration{},
		&domain.VulnTicketingConfig{},
		// Notifications — the in-app centre + delivery preferences. Previously
		// missing from AutoMigrate, so every /notifications route errored on a
		// non-existent table (and the scan-completion in-app notification had
		// nowhere to land). Metadata is jsonb; it stays NULL unless a producer
		// sets it (a bare map[string]interface{} has no driver.Valuer).
		&domain.Notification{},
		&domain.NotificationPreference{},
		// Compliance audits ("Audits" — plan/execute/history) and remediation
		// plans ("Plans de remédiation" — close a gap, assign, track). Tenant-scoped.
		&domain.ComplianceAudit{},
		&domain.RemediationPlan{},
		// Security Automation / SOAR (spec §10 « Automatisation »): tenant-scoped
		// playbooks (trigger + conditions + action chain + SLA policy), their
		// execution audit trail, and the live SLA countdowns the monitor escalates.
		&domain.AutomationRule{},
		&domain.AutomationExecution{},
		&domain.SLATracker{},
		&domain.AutomationChannelConfig{},
		// Governance (spec §15 « Gouvernance »): the immutable audit trail
		// (append-only who/what/when/before→after), time-boxed delegations, and
		// the configurable Maker-Checker approval engine (workflows + requests).
		&domain.AuditEvent{},
		&domain.Delegation{},
		&domain.ApprovalWorkflow{},
		&domain.ApprovalRequest{},
	); err != nil {
		log.Fatalf("Database Migration Failed: %v", err)
	}

	// Run SQL migrations (if DATABASE_URL is set). This uses the `migrations` folder.
	// Must run after AutoMigrate: these SQL migrations add indices/tables/FKs on top of
	// the base tables (users, organizations, risks, ...) that AutoMigrate creates first.
	migrations.RunMigrations()

	// Governance audit trail (spec §15): install the GORM plugin AFTER the tables
	// exist. From here on, every struct-form mutation of an Auditable model
	// (Asset, ComplianceControl, …) is journaled automatically into audit_events —
	// developers can't forget to log. Best-effort: a failure never blocks a write.
	if err := database.DB.Use(audittrailinfra.New(database.DB)); err != nil {
		log.Printf("audit trail: failed to install GORM plugin: %v", err)
	} else {
		log.Println("Governance: immutable audit-trail plugin installed")
	}

	// Création du compte Admin par défaut si la DB est vide
	// Cela garantit que l'app est utilisable immédiatement après déploiement.
	handlers.SeedAdminUser()

	// Provision RBAC / tenant / audit tables (permissions, predefined roles, a Tenant
	// per Organization, UserTenant per membership) so the Settings admin tabs are live.
	handlers.SeedRBAC()

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
	// Email transport (mock in dev; swap for SMTP in prod). Kept as a var so the
	// scan-notification sink below can also send through it.
	emailTransport := email.NewMockService()
	notificationService := notify.NewEmailService(emailTransport, emailFromAddr, appBaseURL)

	// Initialize password hasher (Argon2id, OWASP recommended — matches handlers.SeedAdminUser)
	passwordHasher := coreauth.NewArgon2idPasswordHasher()

	// Initialize token manager (access/refresh JWT pairs, backed by the DB).
	// There is now a SINGLE RSA key set + JWT implementation (pkg/auth): the exact
	// same `rsaKeys` used by middleware.Protected is handed to the TokenManager, so
	// every token is signed and validated by one implementation.
	tokenManager := coreauth.NewTokenManager(database.DB, rsaKeys)

	// One session resolver, shared by refresh + OAuth2/SAML, re-derives the user's
	// tenant + org roles + permissions from the DB at mint time. This guarantees
	// (a) OAuth/SAML sessions are identical to password login, and (b) refresh
	// preserves — and freshens — permissions instead of dropping them.
	resolveSession := func(ctx context.Context, uid uuid.UUID) (*coreauth.SessionClaims, error) {
		org, err := userRepo.GetUserDefaultOrganization(ctx, uid)
		if err != nil {
			return nil, err
		}
		if org == nil {
			return nil, fmt.Errorf("user has no organization")
		}
		sc := &coreauth.SessionClaims{TenantID: org.ID, OrgRoles: map[uuid.UUID]string{}}
		member, err := userRepo.GetOrganizationMember(ctx, uid, org.ID)
		if err != nil {
			return nil, err
		}
		if member != nil {
			sc.OrgRoles[org.ID] = string(member.Role)
			sc.Permissions = member.GetPermissionSet().GetAllPermissions()
		}
		return sc, nil
	}
	tokenManager.SetSessionResolver(resolveSession)

	// L5 — Personal Access Tokens. DB-backed service (survives restarts, scoped),
	// its auth middleware, and a management handler. The same resolveSession gives a
	// PAT the owner's tenant + permissions (narrowed to the token's scopes).
	patService := coreauth.NewPersonalAccessTokenService(repository.NewGormPersonalAccessTokenRepository(database.DB))

	// L7 — full-fidelity auth audit trail (auth_audit_logs: IP, UA, geo, device
	// fingerprint, timestamp). Shared by the clean auth handler, MFA, and SSO.
	authAudit := coreauth.NewAuditService(repository.NewGormAuthAuditLogRepository(database.DB))

	// MFA (L4). AES-256 key for the encrypted TOTP secret at rest.
	mfaRepo := repository.NewGormMFARepository(database.DB)
	mfaKeyRaw := os.Getenv("MFA_ENCRYPTION_KEY")
	if mfaKeyRaw == "" {
		mfaKeyRaw = "openrisk-dev-mfa-encryption-key-change-me"
		log.Println("Warning: MFA_ENCRYPTION_KEY not set — using an insecure dev key. Set a strong 32-byte key in production.")
	}
	mfaKey := sha256.Sum256([]byte(mfaKeyRaw)) // 32 bytes for AES-256-GCM

	// Initialize use cases. Login enforces MFA when the user has a verified secret.
	loginUseCase := auth.NewLoginUseCase(userRepo, tokenManager, passwordHasher).WithMFA(mfaRepo)
	registerUseCase := auth.NewRegisterUseCase(userRepo, orgRepo, notificationService, passwordHasher)
	refreshUseCase := auth.NewRefreshTokenUseCase(tokenManager)
	logoutUseCase := auth.NewLogoutUseCase(tokenManager)

	// MFA use cases + handler.
	setupMFAUseCase := auth.NewSetupMFAUseCase(mfaRepo, mfaKey[:])
	verifyMFAUseCase := auth.NewVerifyMFAUseCase(mfaRepo, *userRepo, mfaKey[:])
	disableMFAUseCase := auth.NewDisableMFAUseCase(mfaRepo, passwordHasher)
	challengeMFAUseCase := auth.NewChallengeMFAUseCase(mfaRepo, mfaKey[:])
	mfaHandler := authhandler.NewMFAHandler(setupMFAUseCase, verifyMFAUseCase, disableMFAUseCase, challengeMFAUseCase, tokenManager, userRepo, authAudit)
	patHandler := authhandler.NewPATHandler(patService, authAudit)

	// Initialize Clean Architecture auth handler
	cleanAuthHandler := authhandler.NewHandler(
		loginUseCase,
		registerUseCase,
		refreshUseCase,
		logoutUseCase,
		passwordHasher,
		authAudit,
	)

	// Initialize legacy auth handler (for backward compatibility)
	authHandler := handlers.NewAuthHandler()

	// Initialize OAuth2 and SAML2 configurations. Hand SSO the SAME token manager +
	// audit service so OAuth/SAML issue RS256 access+refresh pairs identical to
	// password login (previously they minted HS256 tokens the RS256 middleware
	// rejected) and are audited with the full field set.
	handlers.InitializeOAuth2()
	handlers.SetSSOTokenManager(tokenManager, authAudit, userRepo, orgRepo)

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

	// --- MFA challenge (L4, second login leg) ---
	// Reached with the short-lived MFA_REQUIRED token from /auth/login. Registered
	// on `api` BEFORE the Protected group: MFATokenMiddleware validates the special
	// token itself and rejects full/absent tokens. On a valid code the handler
	// mints the real access+refresh pair.
	api.Post("/auth/mfa/challenge", middleware.MFATokenMiddleware(rsaKeys, jtiBlacklistChecker), mfaHandler.Challenge)

	// Scanner AGENT endpoints (register/stream/push) are mounted on `app` HERE —
	// deliberately BEFORE the /api/v1 user-token middleware below — so they are
	// NOT wrapped by middleware.Protected or the marketplace's RequireRole (both
	// mounted at the /api/v1 prefix, which /api/v1/scanner/... would otherwise
	// inherit). Agents authenticate with their own scoped tokens (+ HMAC on push)
	// inside the handlers. scannerHandler is assigned in §5.6; these closures
	// capture it and run only at request time, by which point it is set.
	var scannerHandler *handlers.ScannerHandler
	app.Post("/api/v1/scanner/agents/register", func(c *fiber.Ctx) error { return scannerHandler.RegisterAgent(c) })
	app.Get("/api/v1/scanner/agent/stream", func(c *fiber.Ctx) error { return scannerHandler.AgentStream(c) })
	app.Post("/api/v1/scanner/agent/push", func(c *fiber.Ctx) error { return scannerHandler.AgentPush(c) })
	app.Post("/api/v1/scanner/agent/heartbeat", func(c *fiber.Ctx) error { return scannerHandler.AgentHeartbeat(c) })

	// Mitigation SSE stream — mounted here (before the /api/v1 user middleware) so it
	// escapes the JWT gate: native EventSource can't send a Bearer header, so the
	// token is validated from ?token= inside the handler. Assigned in §5.7.
	var mitigationEventsHandler *handlers.MitigationEventsHandler
	app.Get("/api/v1/mitigations/events", func(c *fiber.Ctx) error { return mitigationEventsHandler.Stream(c) })

	// Vulnerability scanner webhook — external tools (Nessus/Qualys/Defender/…) POST
	// findings here authenticated by the integration's opaque webhook token (NOT a
	// user JWT). Mounted on `app` BEFORE the /api/v1 JWT gate for the same reason as
	// the scanner agent endpoints. Assigned in the vulnerability section below.
	var vulnWebhookHandler *handlers.VulnWebhookHandler
	app.Post("/api/v1/vulnerabilities/webhook/:source", func(c *fiber.Ctx) error { return vulnWebhookHandler.Ingest(c) })

	// --- Routes Protégées (Nécessitent JWT) ---
	// Le middleware injecte user_id et role dans le contexte
	// L5 — PAT authentication runs BEFORE the JWT gate: it authenticates PAT-shaped
	// bearers and is a no-op for JWTs (which the RS256 middleware then handles). The
	// JWT middleware skips when a PAT already authenticated the request.
	api.Use(middleware.PATMiddleware(patService, resolveSession))
	protected := api.Use(middleware.Protected(rsaKeys, jtiBlacklistChecker))

	// Governance audit trail (spec §15): stamp the acting identity + request
	// metadata onto the request context for every authenticated route, so any
	// repository that threads c.UserContext() into GORM lets the audittrail
	// plugin attribute the mutation to the real user (the "Qui"). Additive and
	// value-only — it never alters request handling.
	protected.Use(func(c *fiber.Ctx) error {
		if mw := middleware.GetContext(c); mw != nil {
			var actorID *uuid.UUID
			if mw.UserID != uuid.Nil {
				id := mw.UserID
				actorID = &id
			}
			c.SetUserContext(audittrailinfra.WithActor(c.UserContext(), audittrailinfra.Actor{
				ID:        actorID,
				TenantID:  mw.OrganizationID,
				IPAddress: c.IP(),
				UserAgent: c.Get("User-Agent"),
				RequestID: c.Get("X-Request-ID"),
			}))
		}
		return c.Next()
	})

	// --- MFA enrollment (L4) — full session required ---
	protected.Post("/auth/mfa/setup", mfaHandler.Setup)
	protected.Post("/auth/mfa/verify", mfaHandler.Verify)
	protected.Post("/auth/mfa/disable", mfaHandler.Disable)

	// --- Personal Access Tokens (L5) management — full session required ---
	protected.Post("/auth/pat", patHandler.Create)
	protected.Get("/auth/pat", patHandler.List)
	protected.Delete("/auth/pat/:id", patHandler.Revoke)

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
	// Cyber Risk Quantification: XAF→USD rate configurable via XAF_USD_RATE
	// (default ≈ 600 FCFA/USD). Reference ALE bands match the board ExposureModel.
	xafPerUSD := crq.DefaultXAFPerUSD
	if v := os.Getenv("XAF_USD_RATE"); v != "" {
		if parsed, perr := strconv.ParseFloat(v, 64); perr == nil && parsed > 0 {
			xafPerUSD = parsed
		}
	}
	riskQuantifier := crq.NewQuantifier(xafPerUSD, crq.DefaultReference())
	markReviewedUseCase := risk.NewMarkRiskReviewedUseCase(riskRepo)
	transitionPhaseUseCase := risk.NewTransitionPhaseUseCase(riskRepo)
	riskHandler := handlers.NewRiskHandler(createRiskUseCase, getRiskUseCase, listRisksUseCase, updateRiskUseCase, deleteRiskUseCase, markReviewedUseCase, transitionPhaseUseCase, redisClientInstance, riskQuantifier)

	// Financial Risk Quantification (spec §9): tenant-wide CFO/CISO dashboard
	// (portfolio ALE, worst-case, residual, remediation budget, ROSI). Reuses the
	// same quantifier as per-risk CRQ so figures agree.
	financialSummaryUseCase := risk.NewFinancialSummaryUseCase(riskRepo, riskQuantifier)
	financialAnalyticsHandler := handlers.NewFinancialAnalyticsHandler(financialSummaryUseCase)

	// NOTE: same bug class as compliance (see comment above complianceFrameworkRead) —
	// middleware.RequirePermissions reads the legacy *domain.UserClaims, which the RS256
	// middleware on `protected` never populates. Using middleware.RequirePermission instead.
	protected.Get("/risks",
		middleware.RequirePermission("risks:read"),
		cacheableHandlers.CacheRiskListGET(riskHandler.GetRisks))
	protected.Get("/risks/:id",
		middleware.RequirePermission("risks:read"),
		cacheableHandlers.CacheRiskGetByIDGET(riskHandler.GetRisk))
	// Financial Risk Quantification (spec §9). Read-only: full per-risk assessment
	// and a non-persisting investment-scenario simulator. Static "financial"/
	// "simulate" segments are risk-scoped so they never collide with :id parsing.
	protected.Get("/risks/:id/financial",
		middleware.RequirePermission("risks:read"), riskHandler.GetRiskFinancial)
	protected.Post("/risks/:id/simulate",
		middleware.RequirePermission("risks:read"), riskHandler.SimulateRiskFinancial)

	// Gestion des Risques (Écriture = Analyst & Admin uniquement)
	// Respect du principe "Simplicité & Sécurité" + Fine-grained Permission Checks
	riskCreate := middleware.RequirePermission("risks:create")
	riskUpdate := middleware.RequirePermission("risks:update")
	riskDelete := middleware.RequirePermission("risks:delete")

	protected.Post("/risks", riskCreate, riskHandler.CreateRisk)
	protected.Patch("/risks/:id", riskUpdate, riskHandler.UpdateRisk)
	protected.Post("/risks/:id/review", riskUpdate, riskHandler.MarkReviewed)
	// ISO 31000 lifecycle transition (Identifier → … → Clôturer). Tenant-scoped, audited.
	protected.Post("/risks/:id/transition", riskUpdate, riskHandler.TransitionPhase)
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
	getGapAnalysisUC := compliance.NewGetGapAnalysisUseCase(complianceRepo)
	controlMappingRepo := repository.NewGormControlMappingRepository(database.DB)
	createMappingUC := compliance.NewCreateControlMappingUseCase(controlMappingRepo, complianceRepo)
	listMappingsUC := compliance.NewListControlMappingsUseCase(controlMappingRepo, complianceRepo)
	deleteMappingUC := compliance.NewDeleteControlMappingUseCase(controlMappingRepo)
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
		getGapAnalysisUC,
		createMappingUC, listMappingsUC, deleteMappingUC,
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
	// Gap analysis ("analyse d'écarts") — every unsatisfied control across the
	// tenant's frameworks (optional ?framework_id= scopes to one).
	protected.Get("/compliance/gap-analysis", complianceControlRead, complianceHandler.GetGapAnalysis)
	// Cross-framework control mappings ("cross-mapping entre référentiels"). Static
	// path — no :param collision with /compliance/controls/:controlId.
	protected.Get("/compliance/control-mappings", complianceControlRead, complianceHandler.ListControlMappings)
	protected.Post("/compliance/control-mappings", complianceControlUpdate, complianceHandler.CreateControlMapping)
	protected.Delete("/compliance/control-mappings/:mappingId", complianceControlUpdate, complianceHandler.DeleteControlMapping)
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

	// -------------------------------------------------------------------------
	// Compliance audits ("Audits") + remediation plans ("Plans de remédiation").
	// One Gorm repo backs both aggregates. New permission strings — admin/root
	// hold "*" so they're granted; a future Profile rule can open them per-role.
	// -------------------------------------------------------------------------
	complianceAuditRepo := repository.NewGormComplianceAuditRepository(database.DB)
	complianceAuditHandler := handlers.NewComplianceAuditHandler(
		complianceaudit.NewCreateAuditUseCase(complianceAuditRepo),
		complianceaudit.NewListAuditsUseCase(complianceAuditRepo),
		complianceaudit.NewGetAuditUseCase(complianceAuditRepo),
		complianceaudit.NewUpdateAuditUseCase(complianceAuditRepo),
		complianceaudit.NewDeleteAuditUseCase(complianceAuditRepo),
		complianceaudit.NewCreateRemediationUseCase(complianceAuditRepo, complianceRepo),
		complianceaudit.NewListRemediationsUseCase(complianceAuditRepo, complianceRepo),
		complianceaudit.NewUpdateRemediationUseCase(complianceAuditRepo),
		complianceaudit.NewDeleteRemediationUseCase(complianceAuditRepo),
		complianceaudit.NewGenerateRemediationsFromAuditUseCase(complianceAuditRepo, complianceAuditRepo, complianceRepo),
	)
	complianceAuditRead := middleware.RequirePermission("compliance:audits:read")
	complianceAuditWrite := middleware.RequirePermission("compliance:audits:write")
	complianceRemediationRead := middleware.RequirePermission("compliance:remediations:read")
	complianceRemediationWrite := middleware.RequirePermission("compliance:remediations:write")

	// Static paths — registered as siblings of /compliance/frameworks etc.; no
	// dynamic :segment under /compliance would greedily catch "audits"/"remediations".
	protected.Get("/compliance/audits", complianceAuditRead, complianceAuditHandler.ListAudits)
	protected.Post("/compliance/audits", complianceAuditWrite, complianceAuditHandler.CreateAudit)
	protected.Get("/compliance/audits/:id", complianceAuditRead, complianceAuditHandler.GetAudit)
	protected.Patch("/compliance/audits/:id", complianceAuditWrite, complianceAuditHandler.UpdateAudit)
	protected.Delete("/compliance/audits/:id", complianceAuditWrite, complianceAuditHandler.DeleteAudit)
	// One-click: open a remediation plan for every open gap under the audit's framework.
	protected.Post("/compliance/audits/:id/generate-remediations", complianceRemediationWrite, complianceAuditHandler.GenerateRemediations)

	protected.Get("/compliance/remediations", complianceRemediationRead, complianceAuditHandler.ListRemediations)
	protected.Post("/compliance/remediations", complianceRemediationWrite, complianceAuditHandler.CreateRemediation)
	protected.Patch("/compliance/remediations/:id", complianceRemediationWrite, complianceAuditHandler.UpdateRemediation)
	protected.Delete("/compliance/remediations/:id", complianceRemediationWrite, complianceAuditHandler.DeleteRemediation)

	// =========================================================================
	// Board Report (M4, second half — see ROADMAP.md §3 M4).
	// Monthly, non-technical board-of-directors report: aggregates the tenant's
	// risk/compliance posture, estimates financial exposure in FCFA, and asks an
	// AI advisor to write the narrative. The LLM is best-effort: when
	// ANTHROPIC_API_KEY is set a ClaudeAdvisor (claude-opus-4-8) writes the prose;
	// otherwise (or on any API error) a deterministic TemplateAdvisor does, so the
	// feature works out of the box with no key. Human-in-the-loop: reports are
	// generated as drafts, editable, and must be approved before diffusion.
	// =========================================================================
	boardRepo := repository.NewGormBoardReportRepository(database.DB)
	boardAdvisor := ai.NewAdvisor(os.Getenv("ANTHROPIC_API_KEY"), os.Getenv("ANTHROPIC_MODEL"))
	if _, isTemplate := boardAdvisor.(*ai.TemplateAdvisor); isTemplate {
		log.Println("Board Report: no ANTHROPIC_API_KEY set — using deterministic template advisor")
	} else {
		log.Printf("Board Report: Claude advisor enabled (%s)", boardAdvisor.Name())
	}
	generateBoardUC := board.NewGenerateBoardReportUseCase(
		boardRepo, riskRepo, complianceRepo, orgRepo, boardAdvisor, board.DefaultExposureModel(),
	)
	getBoardUC := board.NewGetBoardReportUseCase(boardRepo)
	listBoardUC := board.NewListBoardReportsUseCase(boardRepo)
	updateBoardUC := board.NewUpdateBoardReportUseCase(boardRepo)
	approveBoardUC := board.NewApproveBoardReportUseCase(boardRepo)
	deleteBoardUC := board.NewDeleteBoardReportUseCase(boardRepo)
	boardHandler := handlers.NewBoardReportHandler(
		generateBoardUC, getBoardUC, listBoardUC, updateBoardUC, approveBoardUC, deleteBoardUC, userRepo,
	)

	// admin/root-only today (same permission model as compliance — see the note above).
	boardRead := middleware.RequirePermission("reports:board:read")
	boardCreate := middleware.RequirePermission("reports:board:create")
	boardUpdate := middleware.RequirePermission("reports:board:update")
	boardApprove := middleware.RequirePermission("reports:board:approve")
	boardDelete := middleware.RequirePermission("reports:board:delete")

	protected.Get("/reports/board", boardRead, boardHandler.List)
	protected.Post("/reports/board", boardCreate, boardHandler.Generate)
	protected.Get("/reports/board/:reportId", boardRead, boardHandler.Get)
	protected.Patch("/reports/board/:reportId", boardUpdate, boardHandler.Update)
	protected.Post("/reports/board/:reportId/approve", boardApprove, boardHandler.Approve)
	protected.Delete("/reports/board/:reportId", boardDelete, boardHandler.Delete)
	protected.Get("/reports/board/:reportId/pdf", boardRead, boardHandler.DownloadPDF)

	// Assets (M3 — see ROADMAP.md §3). Previously these two routes bypassed
	// RBAC entirely (any authenticated user, any role, could write inventory
	// data) — now gated the same way as risks/compliance.
	assetRepo := repository.NewGormAssetRepository(database.DB)
	assetDepRepo := repository.NewGormAssetDependencyRepository(database.DB)
	createAssetUC := assetapp.NewCreateAssetUseCase(assetRepo)
	getAssetUC := assetapp.NewGetAssetUseCase(assetRepo)
	listAssetsUC := assetapp.NewListAssetsUseCase(assetRepo)
	updateAssetUC := assetapp.NewUpdateAssetUseCase(assetRepo)
	// Deleting an asset also prunes its dependency edges (no dangling links).
	deleteAssetUC := assetapp.NewDeleteAssetUseCase(assetRepo).WithDependencyRepository(assetDepRepo)
	// Resolve each history snapshot's changed_by UUID to an email so the
	// inventory's history view shows "qui a modifié" as a human name, not a UUID.
	listAssetSnapshotsUC := assetapp.NewListAssetSnapshotsUseCase(assetRepo).WithUserLookup(userRepo)
	assetHandler := handlers.NewAssetHandler(
		createAssetUC, getAssetUC, listAssetsUC, updateAssetUC, deleteAssetUC, listAssetSnapshotsUC,
		redisClientInstance,
	)

	// Asset dependency graph (cartography). Both endpoints must belong to the
	// tenant; edges cascade-delete when either asset is removed.
	assetDepHandler := handlers.NewAssetDependencyHandler(
		assetapp.NewListAssetDependenciesUseCase(assetDepRepo),
		assetapp.NewCreateAssetDependencyUseCase(assetDepRepo, assetRepo),
		assetapp.NewDeleteAssetDependencyUseCase(assetDepRepo),
	)

	assetRead := middleware.RequirePermission("assets:read")
	assetCreate := middleware.RequirePermission("assets:create")
	assetUpdate := middleware.RequirePermission("assets:update")
	assetDelete := middleware.RequirePermission("assets:delete")

	protected.Get("/assets", assetRead, assetHandler.ListAssets)
	protected.Post("/assets", assetCreate, assetHandler.CreateAsset)
	// NB: register the static /asset-dependencies resource as a sibling of
	// /assets (not /assets/:id/...) so "dependencies" is never parsed as an
	// asset UUID.
	protected.Get("/asset-dependencies", assetRead, assetDepHandler.ListAssetDependencies)
	protected.Post("/asset-dependencies", assetUpdate, assetDepHandler.CreateAssetDependency)
	protected.Delete("/asset-dependencies/:id", assetUpdate, assetDepHandler.DeleteAssetDependency)
	protected.Get("/assets/:id", assetRead, assetHandler.GetAsset)
	protected.Patch("/assets/:id", assetUpdate, assetHandler.UpdateAsset)
	protected.Delete("/assets/:id", assetDelete, assetHandler.DeleteAsset)
	protected.Get("/assets/:id/history", assetRead, assetHandler.GetAssetHistory)

	// --- Vulnerability Management (Module 3) — integrations + risk-based
	// prioritisation. Findings from Nessus/OpenVAS/Qualys/Defender/Inspector/
	// Azure Defender/CrowdStrike are normalised (internal/vulnscan), scored by
	// pkg/vulnprio (CVSS + exploitability + business criticality + affected
	// assets) and upserted into a tenant-scoped register.
	vulnRepo := repository.NewGormVulnerabilityRepository(database.DB)
	// Enrich ingested findings against the CTI feed (CISA-KEV / CVSS / severity)
	// before prioritisation. A stateless repo instance on the shared DB — the CTI
	// handler wires its own later.
	vulnCTIEnricher := vulnapp.NewCTIRepoEnricher(repository.NewGormCTIRepository(database.DB))
	vulnIngestUC := vulnapp.NewIngestUseCase(vulnRepo, assetRepo).
		WithCTIEnricher(vulnCTIEnricher).
		WithRiskProposer(vulnrisk.NewRiskCreator(database.DB))
	vulnHandler := handlers.NewVulnerabilityHandler(
		vulnIngestUC,
		vulnapp.NewListUseCase(vulnRepo),
		vulnapp.NewGetUseCase(vulnRepo),
		vulnapp.NewUpdateStatusUseCase(vulnRepo),
		vulnapp.NewDeleteUseCase(vulnRepo),
		vulnapp.NewStatsUseCase(vulnRepo),
	)
	vulnRead := middleware.RequirePermission("vulnerabilities:read")
	vulnWrite := middleware.RequirePermission("vulnerabilities:update")
	vulnDelete := middleware.RequirePermission("vulnerabilities:delete")

	// --- Smart Risk Calculation (spec §8 "Calcul de risque intelligent") ---
	// The multifactor risk score: blends business criticality, internet exposure,
	// vulnerabilities/CVSS, control maturity, incident history, exploitability,
	// financial value and live threat intel (CTI) via pkg/scoring.ComputeSmart with
	// per-tenant CONFIGURABLE weights. The classic Score Engine (P × I × AC) is
	// untouched — this is an additional, richer view. Every signal source is wired
	// here from the already-built repositories; each is nil-safe in the use case.
	smartWeightsRepo := repository.NewGormRiskScoringWeightsRepository(database.DB)
	computeSmartUC := risk.NewComputeSmartScoreUseCase(riskRepo, smartWeightsRepo).
		WithAssetRepo(assetRepo).
		WithVulnLister(vulnRepo).
		WithCompliance(complianceRepo).
		WithIncidents(repository.NewGormIncidentCounter(database.DB)).
		WithQuantifier(riskQuantifier).
		WithPersister(riskRepo)
	smartScoreHandler := handlers.NewSmartScoreHandler(
		computeSmartUC,
		risk.NewGetRiskWeightsUseCase(smartWeightsRepo),
		risk.NewUpdateRiskWeightsUseCase(smartWeightsRepo),
	)
	// Per-risk multifactor score + breakdown (read), and a non-persisting simulator
	// for live weight tuning. Static "risk-scoring" path is a sibling of /risks.
	protected.Get("/risks/:id/smart-score",
		middleware.RequirePermission("risks:read"), smartScoreHandler.GetRiskSmartScore)
	protected.Post("/risks/:id/smart-score/simulate",
		middleware.RequirePermission("risks:read"), smartScoreHandler.SimulateRiskSmartScore)
	// Per-tenant factor weights: read for anyone with risks:read, write admin-only.
	protected.Get("/risk-scoring/weights",
		middleware.RequirePermission("risks:read"), smartScoreHandler.GetRiskWeights)
	protected.Put("/risk-scoring/weights",
		middleware.RequireRole("admin"), smartScoreHandler.UpdateRiskWeights)

	// Connector + ticketing configuration. Credentials are AES-256-GCM encrypted
	// with the same key family as the scanner (SCANNER_CREDENTIAL_KEY) and are
	// never returned to the API. A tenant can wire the 7 external tools, an inbound
	// webhook token, and its ITSM (Jira/ServiceNow) here.
	vulnIntegKeyRaw := os.Getenv("SCANNER_CREDENTIAL_KEY")
	if vulnIntegKeyRaw == "" {
		vulnIntegKeyRaw = "openrisk-dev-scanner-credential-key-change-me"
	}
	vulnIntegCipher, vulnIntegCipherErr := scanapp.NewCredentialCipher([]byte(vulnIntegKeyRaw))
	if vulnIntegCipherErr != nil {
		log.Fatalf("failed to init vulnerability integration cipher: %v", vulnIntegCipherErr)
	}
	vulnIntegRepo := repository.NewGormVulnIntegrationRepository(database.DB)
	// Auto-ticketing: the opener composes the tenant ITSM config + Jira/ServiceNow
	// providers (pkg/ticketing). Wired into ingest (auto-open for P1/KEV) and into
	// the manual "Open ticket" use case. Mutating vulnIngestUC here still affects the
	// already-built handlers — they hold the same *IngestUseCase pointer.
	vulnTicketOpener := vulnapp.NewConfigTicketOpener(vulnIntegRepo, vulnIntegCipher)
	vulnIngestUC.WithTicketOpener(vulnTicketOpener)
	vulnLivePullUC := vulnapp.NewTriggerLivePullUseCase(vulnIntegRepo, vulnIntegCipher, vulnapp.LivePullAdapter{}, vulnIngestUC)
	vulnIntegHandler := handlers.NewVulnIntegrationHandler(
		vulnapp.NewSaveIntegrationUseCase(vulnIntegRepo, vulnIntegCipher),
		vulnapp.NewListIntegrationsUseCase(vulnIntegRepo),
		vulnapp.NewGetIntegrationUseCase(vulnIntegRepo),
		vulnapp.NewDeleteIntegrationUseCase(vulnIntegRepo),
		vulnLivePullUC,
		vulnapp.NewSaveTicketingUseCase(vulnIntegRepo, vulnIntegCipher),
		vulnapp.NewGetTicketingUseCase(vulnIntegRepo),
		vulnapp.NewDeleteTicketingUseCase(vulnIntegRepo),
		vulnapp.NewCreateTicketUseCase(vulnRepo, vulnTicketOpener),
	)
	// Assign the forward-declared webhook handler (route mounted before the JWT gate).
	vulnWebhookHandler = handlers.NewVulnWebhookHandler(vulnIntegRepo, vulnIngestUC)

	// Static resources first so they are never parsed as /:id.
	protected.Get("/vulnerability-connectors", vulnRead, vulnHandler.ListConnectors)
	protected.Get("/vulnerabilities/stats", vulnRead, vulnHandler.Stats)
	protected.Post("/vulnerabilities/ingest", vulnWrite, vulnHandler.Ingest)
	// Integration + ticketing config (static prefixes before /vulnerabilities/:id).
	protected.Get("/vulnerabilities/integrations", vulnRead, vulnIntegHandler.ListIntegrations)
	protected.Post("/vulnerabilities/integrations", vulnWrite, vulnIntegHandler.SaveIntegration)
	protected.Get("/vulnerabilities/integrations/:id", vulnRead, vulnIntegHandler.GetIntegration)
	protected.Post("/vulnerabilities/integrations/:id/pull", vulnWrite, vulnIntegHandler.TriggerPull)
	protected.Delete("/vulnerabilities/integrations/:id", vulnDelete, vulnIntegHandler.DeleteIntegration)
	protected.Get("/vulnerabilities/ticketing", vulnRead, vulnIntegHandler.GetTicketing)
	protected.Put("/vulnerabilities/ticketing", vulnWrite, vulnIntegHandler.SaveTicketing)
	protected.Delete("/vulnerabilities/ticketing", vulnDelete, vulnIntegHandler.DeleteTicketing)
	protected.Get("/vulnerabilities", vulnRead, vulnHandler.List)
	protected.Get("/vulnerabilities/:id", vulnRead, vulnHandler.Get)
	protected.Patch("/vulnerabilities/:id/status", vulnWrite, vulnHandler.UpdateStatus)
	protected.Post("/vulnerabilities/:id/ticket", vulnWrite, vulnIntegHandler.CreateTicket)
	protected.Delete("/vulnerabilities/:id", vulnDelete, vulnHandler.Delete)

	// =========================================================================
	// AI GRC Assistant (spec §12 — see ROADMAP.md Module 12).
	// Unified AI service over the tenant's own GRC data: treatment-plan
	// suggestions, emerging-risk detection, a natural-language Q&A assistant
	// (hybrid RAG over risks/controls/vulnerabilities), audit-report generation,
	// and evidence document analysis. The LLM is best-effort: when
	// ANTHROPIC_API_KEY is set a ClaudeAssistant (claude-opus-4-8) is used;
	// otherwise (or on any API error) a deterministic TemplateAssistant does, so
	// every endpoint works out of the box with no key. Reuses the same key/model
	// env vars as the board report.
	// =========================================================================
	aiAssistant := ai.NewAssistant(os.Getenv("ANTHROPIC_API_KEY"), os.Getenv("ANTHROPIC_MODEL"))
	if ai.IsLLMBacked(aiAssistant) {
		log.Printf("AI Assistant: Claude enabled (%s)", aiAssistant.Name())
	} else {
		log.Println("AI Assistant: no ANTHROPIC_API_KEY set — using deterministic template assistant")
	}
	aiTreatmentUC := appai.NewSuggestTreatmentPlanUseCase(aiAssistant, riskRepo).WithAssetReader(assetRepo)
	aiEmergingUC := appai.NewDetectEmergingRisksUseCase(aiAssistant).WithRiskLister(riskRepo)
	aiQueryUC := appai.NewAssistantQueryUseCase(aiAssistant).
		WithRisks(riskRepo).
		WithCompliance(complianceRepo).
		WithVulns(vulnRepo).
		WithOrgs(orgRepo)
	aiAuditReportUC := appai.NewGenerateAuditReportUseCase(aiAssistant, complianceAuditRepo).WithGapAnalyzer(getGapAnalysisUC)
	aiEvidenceUC := appai.NewAnalyzeEvidenceUseCase(aiAssistant, complianceRepo)
	aiHandler := handlers.NewAIHandler(aiAssistant, aiTreatmentUC, aiEmergingUC, aiQueryUC, aiAuditReportUC, aiEvidenceUC)

	// AI features are advisory (non-mutating): guarded by the read permission of
	// the relevant module. The assistant/emerging endpoints use risks:read.
	aiRiskRead := middleware.RequirePermission("risks:read")
	aiComplianceRead := middleware.RequirePermission("compliance:read")
	protected.Get("/ai/status", aiHandler.Status)
	protected.Post("/ai/assistant/query", aiRiskRead, aiHandler.AssistantQuery)
	protected.Post("/ai/emerging-risks", aiRiskRead, aiHandler.DetectEmergingRisks)
	protected.Post("/ai/risks/:id/treatment-plan", aiRiskRead, aiHandler.SuggestTreatmentPlan)
	protected.Post("/ai/audits/:id/report", aiComplianceRead, aiHandler.GenerateAuditReport)
	protected.Post("/ai/evidence/:id/analyze", aiComplianceRead, aiHandler.AnalyzeEvidence)

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

	// NOTE: the legacy /risk-management/* lifecycle subsystem (service +
	// handler + duplicate RiskRegister/TreatmentPlan/… models) was removed. Its
	// tables were never in AutoMigrate (every route 500'd) and its queries were
	// not tenant-scoped (cross-tenant leak). The ISO 31000 lifecycle now lives on
	// the real Risk entity: POST /risks/:id/transition (see riskHandler above).

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
	// Financial Risk Quantification dashboard (spec §9) — tenant-wide portfolio
	// ALE / worst-case / residual / remediation budget / ROSI for the CFO/CISO screen.
	protected.Get("/analytics/financial",
		middleware.RequirePermission("risks:read"), financialAnalyticsHandler.GetFinancialSummary)

	// Executive dashboard (spec §11) — ONE consolidated, tenant-scoped aggregation
	// (cyber score, financial exposure, KRIs, top-10 risks, risk & incident trends,
	// compliance coverage) so the frontend makes a single request. Composes the
	// existing tenant-scoped sources; every source is nil-safe in the use case.
	executiveDashboardHandler := newExecutiveDashboardHandler(
		financialSummaryUseCase, riskRepo, getGapAnalysisUC, vulnRepo, incidentService, riskQuantifier,
	)
	protected.Get("/analytics/executive",
		middleware.RequirePermission("risks:read"), executiveDashboardHandler.GetExecutiveDashboard)

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
	// 5.6 SCANNER ENGINE (cloud SDK scans + on-prem Agent, Redis preview)
	//
	// The pipeline NEVER writes Assets/Risks — every scan lands in a Redis preview
	// (48h TTL) and the user imports/ignores from the Scan Preview page. Cloud
	// scans run in-process (credentials AES-256-GCM, decrypted only at scan time);
	// nmap/osquery only ever run on the on-prem Agent, which pushes results back
	// over an RS256 (scoped "scanner") + HMAC-signed channel.
	// =========================================================================

	// Credential cipher (AES-256-GCM) for cloud creds + per-agent push secrets.
	scannerKeyRaw := os.Getenv("SCANNER_CREDENTIAL_KEY")
	if scannerKeyRaw == "" {
		scannerKeyRaw = "openrisk-dev-scanner-credential-key-change-me"
		log.Println("Warning: SCANNER_CREDENTIAL_KEY not set — using an insecure dev key. Set a strong key in production.")
	}
	scannerCipher, cipherErr := scanapp.NewCredentialCipher([]byte(scannerKeyRaw))
	if cipherErr != nil {
		log.Fatalf("Scanner: credential cipher init failed: %v", cipherErr)
	}

	// Provider registry. Cloud scanners run real SDK collectors (aws-sdk-go-v2,
	// Azure Resource Graph, GCP Compute) — credentials are decrypted only at scan
	// time. Agent/nmap providers validate here but execute on the on-prem Agent.
	scanRegistry := scanpkg.NewRegistry()
	scanRegistry.Register(scanpkg.NewAWSScanner(collectors.NewAWS()))
	scanRegistry.Register(scanpkg.NewAzureScanner(collectors.NewAzure()))
	scanRegistry.Register(scanpkg.NewGCPScanner(collectors.NewGCP()))
	scanRegistry.Register(scanpkg.NewNmapScanner())
	scanRegistry.Register(scanpkg.NewAgentScanner())
	// Auto-discovery API providers (spec "6. Découverte automatique des actifs").
	// Each runs in-process in the SaaS worker via its official SDK/REST collector.
	scanRegistry.Register(scanpkg.NewGitHubScanner(collectors.NewGitHub()))
	scanRegistry.Register(scanpkg.NewGitLabScanner(collectors.NewGitLab()))
	scanRegistry.Register(scanpkg.NewActiveDirectoryScanner(collectors.NewActiveDirectory()))
	scanRegistry.Register(scanpkg.NewM365Scanner(collectors.NewM365()))
	scanRegistry.Register(scanpkg.NewDockerScanner(collectors.NewDocker()))
	scanRegistry.Register(scanpkg.NewVMwareScanner(collectors.NewVMware()))
	scanRegistry.Register(scanpkg.NewKubernetesScanner(collectors.NewKubernetes()))

	scanPreview := scanpkg.NewPreviewStore(redisClientInstance)
	// In-app + e-mail sink: a completed scan raises a durable in-app notification
	// for the user who triggered it and (best-effort) e-mails them. Failures never
	// block the scan. A Nil user (e.g. a failed cloud scan) is skipped.
	scanInApp := func(ctx context.Context, tenantID, userID uuid.UUID, title, message string) {
		if userID == uuid.Nil {
			return
		}
		if err := notificationUseCase.NotifyInApp(userID, tenantID, domain.NotificationTypeScanComplete, title, message, nil, "scan"); err != nil {
			zeroLogger.Warn().Err(err).Msg("scanner: could not create in-app notification")
		}
		if user, err := userRepo.GetByID(ctx, userID); err == nil && user != nil && user.Email != "" {
			_ = emailTransport.SendEmail(ctx, user.Email, title, message)
		}
	}
	scanNotifier := scanpkg.NewRedisNotifier(redisClientInstance, scanInApp)

	// Risk review cadence: a background worker nudges each risk's owner (in-app +
	// e-mail) when a review is due, keeping the register "updated regularly".
	riskReviewRepo := repository.NewGormRiskReviewRepository(database.DB)
	riskReviewWorker := workers.NewRiskReviewWorker(riskReviewRepo, func(ctx context.Context, tenantID, ownerID, riskID uuid.UUID, riskTitle string) {
		subject := "Revue de risque requise"
		message := "Le risque « " + riskTitle + " » est dû pour revue."
		if err := notificationUseCase.NotifyInApp(ownerID, tenantID, domain.NotificationTypeRiskReview, subject, message, &riskID, "risk"); err != nil {
			zeroLogger.Warn().Err(err).Msg("risk review: in-app notification failed")
		}
		if user, uerr := userRepo.GetByID(ctx, ownerID); uerr == nil && user != nil && user.Email != "" {
			_ = emailTransport.SendEmail(ctx, user.Email, subject, message)
		}
	}, zeroLogger)
	go riskReviewWorker.Start(context.Background())
	scanPipeline := scanpkg.NewPipeline(scanRegistry, scanPreview, scanNotifier, zeroLogger)

	// Remediation auto-detection: after a scan, a finding (CVE) that is no longer
	// detected auto-completes the matching mitigation sub-action on the linked risk
	// (CompletedSource=scanner, CompletedBy=nil, AutoDetectedAt=now) and publishes
	// mitigation.auto_completed for the SSE stream. Manual complete/revert stay available.
	ctiSubActionRepo := repository.NewGormMitigationSubActionRepository(database.DB)
	ctiMitigationRepo := repository.NewGormMitigationRepository(database.DB)
	autoCompleteUC := appmitigation.NewAutoCompleteSubActionUseCase(ctiSubActionRepo, ctiMitigationRepo)
	mitigationDetector := scanmitigation.NewDetector(database.DB, autoCompleteUC, ctiSubActionRepo, redisClientInstance, zeroLogger)
	scanPipeline = scanPipeline.WithMitigationDetector(mitigationDetector)

	// Assign the forward-declared SSE handler (route registered earlier on `app`,
	// before the /api/v1 JWT middleware).
	mitigationEventsHandler = handlers.NewMitigationEventsHandler(redisClientInstance, rsaKeys, jtiBlacklistChecker)
	scanLock := scanpkg.NewScanLock(redisClientInstance)

	scanConfigRepo := repository.NewGormScanConfigRepository(database.DB)
	scanAgentRepo := repository.NewGormScannerAgentRepository(database.DB)
	scanJobRepo := repository.NewGormScanJobRepository(database.DB)

	createScanConfigUC := scanapp.NewCreateScanConfigUseCase(scanConfigRepo, scanRegistry, scannerCipher)
	listScanConfigsUC := scanapp.NewListScanConfigsUseCase(scanConfigRepo)
	getScanConfigUC := scanapp.NewGetScanConfigUseCase(scanConfigRepo)
	deleteScanConfigUC := scanapp.NewDeleteScanConfigUseCase(scanConfigRepo)
	triggerScanUC := scanapp.NewTriggerScanUseCase(
		scanConfigRepo, scanJobRepo, scanLock, scanRegistry, scanPipeline, scannerCipher, redisClientInstance, zeroLogger,
	)
	// Recurring scans: a background scheduler triggers due configs every minute.
	scanScheduler := workers.NewScanScheduler(scanConfigRepo, triggerScanUC, zeroLogger)
	go scanScheduler.Start(context.Background())
	listAgentsUC := scanapp.NewListAgentsUseCase(scanAgentRepo)
	revokeAgentUC := scanapp.NewRevokeAgentUseCase(scanAgentRepo, redisClientInstance)
	registerAgentUC := scanapp.NewRegisterAgentUseCase(scanAgentRepo, rsaKeys, scannerCipher)
	pushResultsUC := scanapp.NewPushResultsUseCase(scanAgentRepo, scanJobRepo, scanLock, scanPipeline)
	heartbeatAgentUC := scanapp.NewHeartbeatAgentUseCase(scanAgentRepo)
	listScanJobsUC := scanapp.NewListScanJobsUseCase(scanJobRepo)
	getScanPreviewUC := scanapp.NewGetScanPreviewUseCase(scanPreview)
	importPreviewUC := scanapp.NewImportPreviewUseCase(scanPreview, assetRepo)
	ignorePreviewUC := scanapp.NewIgnorePreviewUseCase(scanPreview)

	scannerHandler = handlers.NewScannerHandler(
		createScanConfigUC, listScanConfigsUC, getScanConfigUC, deleteScanConfigUC, triggerScanUC,
		listAgentsUC, revokeAgentUC, registerAgentUC, pushResultsUC, heartbeatAgentUC,
		listScanJobsUC, getScanPreviewUC, importPreviewUC, ignorePreviewUC,
		scanAgentRepo, scanJobRepo, scannerCipher, rsaKeys, jtiBlacklistChecker, redisClientInstance,
	)

	// User-facing routes (RS256 user token). Admin/root pass via the "*" wildcard.
	scannerRead := middleware.RequirePermission("scanner:read")
	scannerCreate := middleware.RequirePermission("scanner:create")
	scannerDelete := middleware.RequirePermission("scanner:delete")
	scannerScan := middleware.RequirePermission("scanner:scan")
	scannerImport := middleware.RequirePermission("scanner:import")

	protected.Get("/scanner/configs", scannerRead, scannerHandler.ListScanConfigs)
	protected.Post("/scanner/configs", scannerCreate, scannerHandler.CreateScanConfig)
	protected.Delete("/scanner/configs/:id", scannerDelete, scannerHandler.DeleteScanConfig)
	protected.Post("/scanner/configs/:id/scan", scannerScan, scannerHandler.TriggerScan)
	protected.Post("/scanner/configs/:id/registration-token", scannerCreate, scannerHandler.IssueRegistrationToken)
	protected.Get("/scanner/agents", scannerRead, scannerHandler.ListAgents)
	protected.Delete("/scanner/agents/:id", scannerDelete, scannerHandler.RevokeAgent)
	protected.Get("/scanner/jobs", scannerRead, scannerHandler.ListScanJobs)
	protected.Get("/scanner/jobs/:id/preview", scannerRead, scannerHandler.GetScanPreview)
	protected.Post("/scanner/jobs/:id/import", scannerImport, scannerHandler.ImportPreview)
	protected.Post("/scanner/jobs/:id/ignore", scannerImport, scannerHandler.IgnorePreview)
	protected.Get("/scanner/events", scannerRead, scannerHandler.StreamScanEvents)

	// The agent-facing routes (register/stream/push) were mounted earlier on `app`
	// (before the /api/v1 user middleware) — see the note above `var scannerHandler`.
	log.Println("Scanner: engine wired (cloud validate + agent register/stream/push, Redis preview)")

	// =========================================================================
	// 5.7 CTI / INTEL THREAT ENGINE (NVD + CISA KEV + MITRE ATT&CK)
	// =========================================================================
	// Feeds are ingested into the global cti_vulnerabilities table, enriched with
	// embedded MITRE ATT&CK data, then matched against each tenant's asset CPEs to
	// auto-create risks (Source=cti_auto). NVD hourly, CISA KEV every 6h.
	ctiRepo := repository.NewGormCTIRepository(database.DB)
	ctiClient := cti.NewExternalClient(nil, os.Getenv("NVD_API_KEY"))
	ctiService := cti.NewService(ctiRepo, ctiClient)
	ctiRiskCreator := ctimatch.NewAutoRiskCreator(database.DB)
	ctiMatcher := ctimatch.NewTenantSweepMatcher(database.DB, ctiRepo, ctiRiskCreator)
	ctiSyncWorker := cti.NewSyncWorker(ctiRepo, ctiClient, ctiMatcher, zeroLogger)
	ctiHandler := handlers.NewCTIHandler(ctiService, ctiSyncWorker, ctiMatcher, database.DB)

	ctiRead := middleware.RequirePermission("risks:read")
	ctiAdmin := middleware.RequireRole("admin", "root")
	protected.Get("/cti/vulnerabilities", ctiRead, ctiHandler.List)
	protected.Get("/cti/vulnerabilities/:cve", ctiRead, ctiHandler.Get)
	protected.Get("/cti/stats", ctiRead, ctiHandler.Stats)
	protected.Post("/cti/sync", ctiAdmin, ctiHandler.Sync)
	protected.Post("/cti/match", ctiAdmin, ctiHandler.Match)

	// The periodic sync worker (NVD 1h / CISA 6h + post-sync matching) runs in
	// production. In dev it stays off by default to avoid hitting the feeds on every
	// restart — the manual POST /cti/sync + /cti/match endpoints are always live.
	if os.Getenv("CTI_SYNC_ENABLED") == "true" {
		go ctiSyncWorker.Start(context.Background())
		log.Println("CTI: sync worker started (NVD hourly, CISA KEV 6h, post-sync matching)")
	} else {
		log.Println("CTI: engine wired (periodic sync disabled — set CTI_SYNC_ENABLED=true; manual /cti/sync + /cti/match live)")
	}

	// Vulnerability live-pull scheduler — polls due integrations (schedule_minutes)
	// on the same pipeline as the manual "Pull now" button. Off by default in dev;
	// enable with VULN_LIVEPULL_ENABLED=true. Manual POST /vulnerabilities/
	// integrations/:id/pull is always live.
	if os.Getenv("VULN_LIVEPULL_ENABLED") == "true" {
		vulnPullScheduler := vulnapp.NewLivePullScheduler(vulnIntegRepo, vulnLivePullUC, time.Minute)
		go vulnPullScheduler.Run(context.Background())
		log.Println("Vuln: live-pull scheduler started (due integrations polled every minute)")
	} else {
		log.Println("Vuln: live-pull scheduler wired (disabled — set VULN_LIVEPULL_ENABLED=true; manual pull live)")
	}

	// =========================================================================
	// 5.9 SECURITY AUTOMATION / SOAR ENGINE (spec §10 « Automatisation »)
	// =========================================================================
	// Rules bind platform triggers (a newly detected vulnerability, a risk score
	// change) to ordered action chains (scan → create risk → assign → ticket →
	// notify → start SLA). The AutomationWorker consumes Redis events into the
	// engine; the SLAMonitor escalates overdue remediations and auto-closes
	// resolved ones. Action ports reuse the existing capabilities of the platform.
	automationRuleRepo := repository.NewGormAutomationRuleRepository(database.DB)
	automationExecRepo := repository.NewGormAutomationExecutionRepository(database.DB)
	slaTrackerRepo := repository.NewGormSLATrackerRepository(database.DB)
	automationChannelRepo := repository.NewGormAutomationChannelRepository(database.DB)

	// Multi-channel dispatcher (in-app + email + Slack + Teams) with owner/role
	// recipient resolution, and the concrete risk/ticket/scan action adapters.
	automationNotifier := autoinfra.NewNotifier(automationChannelRepo, notificationUseCase, emailTransport, userRepo, database.DB, zeroLogger)
	automationRiskActions := autoinfra.NewRiskActions(riskRepo, userRepo, database.DB, zeroLogger)
	automationTicketer := autoinfra.NewTicketer(vulnIntegRepo, vulnIntegCipher)
	automationScanAction := autoinfra.NewScanAction(scanConfigRepo, triggerScanUC, zeroLogger)

	automationEngine := appauto.NewEngine(automationRuleRepo, automationExecRepo, slaTrackerRepo, zeroLogger).
		WithNotifier(automationNotifier).
		WithTicketer(automationTicketer).
		WithRiskCreator(automationRiskActions).
		WithRiskAssigner(automationRiskActions).
		WithRiskResolver(automationRiskActions).
		WithAssetScanner(automationScanAction)

	automationSLAService := appauto.NewSLAService(slaTrackerRepo, zeroLogger).
		WithNotifier(automationNotifier).
		WithRiskLookup(automationRiskActions)

	automationHandler := handlers.NewAutomationHandler(
		appauto.NewRuleService(automationRuleRepo),
		appauto.NewExecutionService(automationExecRepo),
		automationSLAService,
		appauto.NewChannelService(automationChannelRepo),
		automationEngine,
	)

	// A newly detected vulnerability fires the engine's vulnerability_detected
	// trigger. Mutating vulnIngestUC here still affects the vuln handlers/webhook
	// (they hold the same pointer — same pattern as WithTicketOpener above).
	vulnIngestUC.WithEventPublisher(autoinfra.NewVulnEventPublisher(redisClientInstance))

	automationRead := middleware.RequirePermission("automation:read")
	automationWrite := middleware.RequirePermission("automation:write")
	automationAdmin := middleware.RequireRole("admin", "root")
	protected.Get("/automation/rules", automationRead, automationHandler.ListRules)
	protected.Post("/automation/rules", automationWrite, automationHandler.CreateRule)
	// Static sub-paths before /:id so "executions"/"sla"/"channels" never parse as UUID.
	protected.Get("/automation/executions", automationRead, automationHandler.ListExecutions)
	protected.Get("/automation/sla", automationRead, automationHandler.ListSLA)
	protected.Get("/automation/sla/stats", automationRead, automationHandler.SLAStats)
	protected.Get("/automation/channels", automationRead, automationHandler.GetChannels)
	protected.Put("/automation/channels", automationAdmin, automationHandler.SaveChannels)
	protected.Get("/automation/rules/:id", automationRead, automationHandler.GetRule)
	protected.Put("/automation/rules/:id", automationWrite, automationHandler.UpdateRule)
	protected.Delete("/automation/rules/:id", automationWrite, automationHandler.DeleteRule)
	protected.Post("/automation/rules/:id/test", automationWrite, automationHandler.TestRule)
	protected.Get("/automation/rules/:id/executions", automationRead, automationHandler.ListRuleExecutions)

	// Background workers: the SOAR engine (event-driven) and the SLA monitor (cadence).
	automationWorker := workers.NewAutomationWorker(redisClientInstance, automationEngine, zeroLogger)
	go automationWorker.Start(context.Background())
	slaMonitor := workers.NewSLAMonitor(automationSLAService, zeroLogger)
	go slaMonitor.Start(context.Background())
	log.Println("Automation: SOAR engine + SLA monitor started (triggers: vulnerability.detected, risk.score_updated)")

	// =========================================================================
	// 5.10 GOVERNANCE (spec §15 « Gouvernance »)
	// =========================================================================
	// One GORM store backs the four governance aggregates. Audit reads/writes and
	// email resolution reuse the existing tenant-scoped user repo. Writes are
	// guarded by role (audit trail + workflow config = admin; delegations +
	// approval decisions = any authenticated member — the use cases enforce
	// four-eyes and role-eligibility internally).
	auditEventRepo := repository.NewGormAuditEventRepository(database.DB)
	delegationRepo := repository.NewGormDelegationRepository(database.DB)
	approvalRepo := repository.NewGormApprovalRepository(database.DB)
	governanceRecorder := governance.NewAuditRecorder(auditEventRepo)

	governanceHandler := handlers.NewGovernanceHandler(handlers.GovernanceDeps{
		ListAudit: governance.NewListAuditEventsUseCase(auditEventRepo).WithUserLookup(userRepo),
		Recorder:  governanceRecorder,

		CreateDelegation: governance.NewCreateDelegationUseCase(delegationRepo).WithRecorder(governanceRecorder).WithUserLookup(userRepo),
		ListDelegations:  governance.NewListDelegationsUseCase(delegationRepo).WithUserLookup(userRepo),
		RevokeDelegation: governance.NewRevokeDelegationUseCase(delegationRepo).WithRecorder(governanceRecorder),
		EffectivePerms:   governance.NewResolveEffectivePermissionsUseCase(delegationRepo),

		CreateWorkflow: governance.NewCreateWorkflowUseCase(approvalRepo),
		ListWorkflows:  governance.NewListWorkflowsUseCase(approvalRepo),
		UpdateWorkflow: governance.NewUpdateWorkflowUseCase(approvalRepo),
		DeleteWorkflow: governance.NewDeleteWorkflowUseCase(approvalRepo),

		SubmitApproval: governance.NewSubmitApprovalRequestUseCase(approvalRepo, approvalRepo).WithRecorder(governanceRecorder),
		DecideApproval: governance.NewDecideApprovalStepUseCase(approvalRepo).WithRecorder(governanceRecorder),
		CancelApproval: governance.NewCancelApprovalRequestUseCase(approvalRepo),
		ListApprovals:  governance.NewListApprovalRequestsUseCase(approvalRepo).WithUserLookup(userRepo),
		GetRequest:     approvalRepo,
	})

	governanceAdmin := middleware.RequireRole("admin", "root")

	// Audit trail — admin only (matches /audit-logs). Static export path first.
	protected.Get("/governance/audit-events/export", governanceAdmin, governanceHandler.ExportAuditEvents)
	protected.Get("/governance/audit-events", governanceAdmin, governanceHandler.ListAuditEvents)

	// Delegations — any authenticated member manages their own; static paths first.
	protected.Get("/governance/delegations/effective", governanceHandler.EffectiveDelegatedPermissions)
	protected.Get("/governance/delegations", governanceHandler.ListDelegations)
	protected.Post("/governance/delegations", governanceHandler.CreateDelegation)
	protected.Post("/governance/delegations/:id/revoke", governanceHandler.RevokeDelegation)

	// Approval workflows (config) — admin only.
	protected.Get("/governance/workflows", governanceHandler.ListWorkflows)
	protected.Post("/governance/workflows", governanceAdmin, governanceHandler.CreateWorkflow)
	protected.Put("/governance/workflows/:id", governanceAdmin, governanceHandler.UpdateWorkflow)
	protected.Delete("/governance/workflows/:id", governanceAdmin, governanceHandler.DeleteWorkflow)

	// Approval requests (the Maker-Checker inbox) — any authenticated member.
	protected.Get("/governance/approvals", governanceHandler.ListApprovals)
	protected.Post("/governance/approvals", governanceHandler.SubmitApproval)
	protected.Get("/governance/approvals/:id", governanceHandler.GetApproval)
	protected.Post("/governance/approvals/:id/decide", governanceHandler.DecideApproval)
	protected.Post("/governance/approvals/:id/cancel", governanceHandler.CancelApproval)
	log.Println("Governance: audit trail + delegations + approval workflows mounted (/governance/*)")

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
