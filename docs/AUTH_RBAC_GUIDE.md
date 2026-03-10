# Authentication & RBAC Implementation Guide

**Complete Guide for JWT, Roles, Permissions, and Multi-Tenancy**

---

## Table of Contents

1. [JWT Authentication](#1-jwt-authentication)
2. [Role-Based Access Control](#2-role-based-access-control)
3. [Permission System](#3-permission-system)
4. [Multi-Tenancy](#4-multi-tenancy)
5. [Audit Logging](#5-audit-logging)
6. [Configuration](#6-configuration)
7. [Troubleshooting](#7-troubleshooting)

---

## 1. JWT Authentication

### 1.1 Token Generation

#### In Backend Service
```go
package services

import (
    "time"
    "github.com/golang-jwt/jwt/v5"
    "github.com/google/uuid"
)

type AuthService struct {
    jwtSecret string
    tokenTTL  time.Duration
}

// GenerateToken creates a new JWT token
func (as *AuthService) GenerateToken(user *domain.User) (string, error) {
    now := time.Now()
    expiresAt := now.Add(as.tokenTTL)

    claims := &domain.UserClaims{
        ID:          user.ID,
        Email:       user.Email,
        Username:    user.Username,
        RoleID:      user.RoleID,
        RoleName:    user.Role.Name,
        Permissions: user.Role.Permissions,
        IssuedAt:    now.Unix(),
        ExpiresAt:   expiresAt.Unix(),
    }

    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    return token.SignedString([]byte(as.jwtSecret))
}

// RefreshToken generates a new token using existing credentials
func (as *AuthService) RefreshToken(claims *domain.UserClaims) (string, error) {
    // User is already validated, just generate new token
    return as.GenerateToken(&domain.User{
        ID:       claims.ID,
        Email:    claims.Email,
        Username: claims.Username,
        RoleID:   claims.RoleID,
        Role: &domain.Role{
            Name:        claims.RoleName,
            Permissions: claims.Permissions,
        },
    })
}
```

#### Token Payload Example
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "email": "analyst@example.com",
  "username": "analyst_001",
  "role_id": "660e8400-e29b-41d4-a716-446655440000",
  "role_name": "analyst",
  "permissions": [
    "risk:create:any",
    "risk:read:any",
    "risk:update:own",
    "mitigation:create:team",
    "mitigation:update:any",
    "asset:read:any",
    "auditlog:read:team",
    "dashboard:read:any"
  ],
  "iat": 1678449600,
  "exp": 1678536000
}
```

### 1.2 Token Validation

#### Middleware Implementation
```go
package middleware

import (
    "fmt"
    "strings"
    "github.com/gofiber/fiber/v2"
    "github.com/golang-jwt/jwt/v5"
)

// AuthMiddleware validates JWT tokens
func AuthMiddleware(jwtSecret string) fiber.Handler {
    return func(c *fiber.Ctx) error {
        // Check if route is public
        if isPublicRoute(c.Path()) {
            return c.Next()
        }

        // Extract token from Authorization header
        authHeader := c.Get("Authorization")
        if authHeader == "" {
            return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
                "error": "missing authorization header",
            })
        }

        // Parse Bearer token
        parts := strings.Split(authHeader, " ")
        if len(parts) != 2 || parts[0] != "Bearer" {
            return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
                "error": "invalid authorization header format",
            })
        }

        tokenString := parts[1]

        // Parse and validate JWT
        claims := &domain.UserClaims{}
        token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
            // Verify signing method
            if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
                return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
            }
            return []byte(jwtSecret), nil
        })

        if err != nil || !token.Valid {
            return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
                "error": "invalid or expired token",
            })
        }

        // Validate claims
        if err := claims.Valid(); err != nil {
            return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
                "error": "token claims invalid",
            })
        }

        // Store user info in context
        c.Locals("user_id", claims.ID.String())
        c.Locals("user", claims)
        c.Locals("role", claims.RoleName)
        c.Locals("permissions", claims.Permissions)

        return c.Next()
    }
}

// isPublicRoute checks if route doesn't require authentication
func isPublicRoute(path string) bool {
    public := []string{
        "/health",
        "/api/v1/auth/login",
        "/api/v1/auth/register",
        "/api/v1/auth/refresh",
        "/openapi.json",
        "/swagger-ui",
    }

    for _, route := range public {
        if path == route || strings.HasPrefix(path, route) {
            return true
        }
    }
    return false
}
```

### 1.3 Accessing User Info in Handlers

```go
package handlers

import (
    "github.com/gofiber/fiber/v2"
    "github.com/google/uuid"
)

func GetUserProfile(c *fiber.Ctx) error {
    // Extract user from context (set by AuthMiddleware)
    claims, ok := c.Locals("user").(*domain.UserClaims)
    if !ok {
        return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
            "error": "user not in context",
        })
    }

    userID := claims.ID
    email := claims.Email
    role := claims.RoleName
    permissions := claims.Permissions

    return c.JSON(fiber.Map{
        "id":          userID,
        "email":       email,
        "role":        role,
        "permissions": permissions,
    })
}

func CreateRisk(c *fiber.Ctx) error {
    // All authenticated users have access
    userID := c.Locals("user_id").(uuid.UUID)
    
    // Process request...
    
    return c.JSON(fiber.Map{
        "created_by": userID,
    })
}
```

---

## 2. Role-Based Access Control

### 2.1 Role Hierarchy

```
Level 9: Admin (top)
         ├── Full system access
         ├── Can create users
         └── Can manage roles

Level 6: Manager (middle-high)
         ├── Team management
         └── Risk/mitigation oversight

Level 3: Security Analyst (middle)
         ├── Create/update risks
         ├── Create mitigations
         └── Read assets

Level 1: Auditor (middle-low)
         ├── Read-only access
         ├── View audit logs
         └── Generate reports

Level 0: Viewer (bottom)
         └── Dashboard view only
```

### 2.2 Role Guard Middleware

```go
package middleware

import (
    "github.com/gofiber/fiber/v2"
)

// RoleGuard checks if user has required role(s)
func RoleGuard(requiredRoles ...string) fiber.Handler {
    return func(c *fiber.Ctx) error {
        claims, ok := c.Locals("user").(*domain.UserClaims)
        if !ok {
            return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
                "error": "user not authenticated",
            })
        }

        // Check if user has any of the required roles
        for _, required := range requiredRoles {
            if claims.RoleName == required {
                return c.Next()
            }
        }

        return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
            "error": "insufficient role permissions",
            "required_roles": requiredRoles,
            "user_role": claims.RoleName,
        })
    }
}

// Example usage in routes
app.Post("/users",
    middleware.AuthMiddleware(jwtSecret),
    middleware.RoleGuard("admin"),
    handlers.CreateUser)

app.Post("/risks",
    middleware.AuthMiddleware(jwtSecret),
    middleware.RoleGuard("admin", "analyst"),
    handlers.CreateRisk)

app.Get("/risks",
    middleware.AuthMiddleware(jwtSecret),
    middleware.RoleGuard("viewer", "analyst", "manager", "admin"),
    handlers.GetRisks)
```

### 2.3 Role Level Checking

```go
package domain

type RoleLevel int

const (
    RoleLevelViewer   RoleLevel = 0
    RoleLevelAuditor  RoleLevel = 1
    RoleLevelAnalyst  RoleLevel = 3
    RoleLevelManager  RoleLevel = 6
    RoleLevelAdmin    RoleLevel = 9
)

func GetRoleLevel(roleName string) RoleLevel {
    switch roleName {
    case "viewer":
        return RoleLevelViewer
    case "auditor":
        return RoleLevelAuditor
    case "analyst":
        return RoleLevelAnalyst
    case "manager":
        return RoleLevelManager
    case "admin":
        return RoleLevelAdmin
    default:
        return 0
    }
}

// MinRoleGuard checks if user has minimum role level
func MinRoleGuard(minLevel RoleLevel) fiber.Handler {
    return func(c *fiber.Ctx) error {
        claims := c.Locals("user").(*domain.UserClaims)
        userLevel := GetRoleLevel(claims.RoleName)

        if userLevel < minLevel {
            return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
                "error": "insufficient role level",
                "required_level": minLevel,
                "user_level": userLevel,
            })
        }

        return c.Next()
    }
}
```

---

## 3. Permission System

### 3.1 Permission Format

Permissions follow the format: `resource:action:scope`

```
resource:action:scope
├── resource: risk, mitigation, asset, user, auditlog, dashboard, integration
├── action: read, create, update, delete, export, assign
└── scope: own (self), team (group), any (all)
```

**Examples**:
- `risk:read:any` → Read any risk
- `risk:update:own` → Update own risks
- `mitigation:create:team` → Create mitigations for team
- `user:delete:any` → Delete any user
- `audit:read:any` → Read audit logs

### 3.2 Fine-Grained Permission Checking

```go
package middleware

// RequirePermissions checks if user has ANY of the required permissions
func RequirePermissions(ps *services.PermissionService, required ...domain.Permission) fiber.Handler {
    return func(c *fiber.Ctx) error {
        claims := c.Locals("user").(*domain.UserClaims)

        // Check if user has any required permission
        userPermissions := ps.GetUserPermissions(claims.ID.String(), claims.RoleID.String())
        
        for _, req := range required {
            for _, perm := range userPermissions {
                if perm.Matches(&req) {
                    return c.Next()
                }
            }
        }

        return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
            "error": "user does not have required permission",
            "required": required,
        })
    }
}

// RequireAllPermissions checks if user has ALL required permissions
func RequireAllPermissions(ps *services.PermissionService, required ...domain.Permission) fiber.Handler {
    return func(c *fiber.Ctx) error {
        claims := c.Locals("user").(*domain.UserClaims)
        userPermissions := ps.GetUserPermissions(claims.ID.String(), claims.RoleID.String())

        // Check if user has all required permissions
        for _, req := range required {
            found := false
            for _, perm := range userPermissions {
                if perm.Matches(&req) {
                    found = true
                    break
                }
            }
            if !found {
                return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
                    "error": "user missing required permission",
                    "missing": req.String(),
                })
            }
        }

        return c.Next()
    }
}

// Example usage in routes
app.Post("/risks",
    middleware.AuthMiddleware(jwtSecret),
    middleware.RequirePermissions(permService,
        domain.Permission{
            Resource: domain.PermissionResourceRisk,
            Action:   domain.PermissionCreate,
            Scope:    domain.PermissionScopeAny,
        }),
    handlers.CreateRisk)
```

### 3.3 Resource-Based Scope Checking

```go
package middleware

// RequireResourcePermission checks resource ownership for scope
func RequireResourcePermission(ps *services.PermissionService, resourceType, action string) fiber.Handler {
    return func(c *fiber.Ctx) error {
        claims := c.Locals("user").(*domain.UserClaims)
        resourceID := c.Params("id")

        // Get resource owner/team info
        resource, err := getResource(resourceType, resourceID)
        if err != nil {
            return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
                "error": "resource not found",
            })
        }

        // Determine scope
        var requiredScope domain.PermissionScope
        
        if resource.OwnerID == claims.ID {
            requiredScope = domain.PermissionScopeOwn
        } else if isInTeam(claims.ID, resource.TeamID) {
            requiredScope = domain.PermissionScopeTeam
        } else {
            requiredScope = domain.PermissionScopeAny
        }

        // Check permission with scope
        hasPermission := ps.CheckPermission(claims.ID.String(), claims.RoleID.String(),
            domain.Permission{
                Resource: domain.PermissionResource(resourceType),
                Action:   domain.PermissionAction(action),
                Scope:    requiredScope,
            })

        if !hasPermission {
            return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
                "error": "insufficient permission for this resource",
            })
        }

        return c.Next()
    }
}

// Example usage
app.Patch("/risks/:id",
    middleware.AuthMiddleware(jwtSecret),
    middleware.RequireResourcePermission(permService, "risk", "update"),
    handlers.UpdateRisk)
```

### 3.4 Wildcard Permission Matching

```go
package domain

// Permission represents resource:action:scope format
type Permission struct {
    Resource PermissionResource
    Action   PermissionAction
    Scope    PermissionScope
}

// Matches checks if this permission matches required
func (p *Permission) Matches(required *Permission) bool {
    // Admin wildcard
    if p.Resource == "*" && p.Action == "*" && p.Scope == "*" {
        return true
    }

    // Resource wildcard matching
    if p.Resource != required.Resource && p.Resource != "*" {
        return false
    }

    // Action wildcard matching
    if p.Action != required.Action && p.Action != "*" {
        return false
    }

    // Scope matching
    if p.Scope != required.Scope && p.Scope != "*" {
        return false
    }

    return true
}

// Example wildcard matching:
userPermissions := []Permission{
    {Resource: "risk", Action: "*", Scope: "any"},  // All risk actions
    {Resource: "*", Action: "read", Scope: "any"},  // Read everything
    {Resource: "*", Action: "*", Scope: "*"},       // Admin access
}

// Matches
// risk:create:any  ✅ matches "risk:*:any"
// risk:read:own    ✅ matches "risk:*:any"
// mitigation:read  ✅ matches "*:read:any"
// user:delete:any  ✅ matches "*:*:*"
```

---

## 4. Multi-Tenancy

### 4.1 Tenant Management

```go
package handlers

// CreateTenant creates a new tenant
func CreateTenant(c *fiber.Ctx) error {
    type CreateTenantInput struct {
        Name     string `json:"name" validate:"required,min=3"`
        Slug     string `json:"slug" validate:"required,min=3"`
        Metadata map[string]interface{} `json:"metadata"`
    }

    var input CreateTenantInput
    if err := c.BodyParser(&input); err != nil {
        return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
            "error": "invalid input",
        })
    }

    claims := c.Locals("user").(*domain.UserClaims)

    // Only admins can create tenants
    if claims.RoleName != "admin" {
        return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
            "error": "only admins can create tenants",
        })
    }

    tenant := &domain.Tenant{
        ID:       uuid.New(),
        Name:     input.Name,
        Slug:     input.Slug,
        OwnerID:  claims.ID,
        Status:   "active",
        IsActive: true,
        Metadata: input.Metadata,
    }

    if err := db.Create(tenant).Error; err != nil {
        return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
            "error": "failed to create tenant",
        })
    }

    // Create owner user-tenant relationship
    userTenant := &domain.UserTenant{
        UserID:   claims.ID,
        TenantID: tenant.ID,
        RoleID:   getAdminRoleID(), // Admin role in this tenant
    }
    db.Create(userTenant)

    return c.Status(fiber.StatusCreated).JSON(tenant)
}
```

### 4.2 Tenant Isolation Middleware

```go
package middleware

// TenantIsolation ensures user can only access own tenant data
func TenantIsolation(db *gorm.DB) fiber.Handler {
    return func(c *fiber.Ctx) error {
        claims := c.Locals("user").(*domain.UserClaims)
        tenantID := c.Params("tenant_id")

        // Verify user is member of this tenant
        var userTenant domain.UserTenant
        if err := db.Where("user_id = ? AND tenant_id = ?", claims.ID, tenantID).
            First(&userTenant).Error; err != nil {
            return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
                "error": "you do not have access to this tenant",
            })
        }

        // Store tenant context
        c.Locals("tenant_id", tenantID)
        c.Locals("tenant_role_id", userTenant.RoleID)

        return c.Next()
    }
}

// Usage in routes
v1 := api.Group("/rbac/tenants/:tenant_id",
    middleware.AuthMiddleware(jwtSecret),
    middleware.TenantIsolation(db))

v1.Get("/users", handlers.GetTenantUsers)
v1.Post("/users", handlers.AddUserToTenant)
```

### 4.3 Multi-Tenant Data Queries

```go
package services

// GetTenantRisks retrieves risks for a specific tenant
func (rs *RiskService) GetTenantRisks(tenantID uuid.UUID) ([]domain.Risk, error) {
    var risks []domain.Risk

    // Query scoped to tenant
    err := rs.db.Where("tenant_id = ?", tenantID).
        Order("created_at DESC").
        Find(&risks).Error

    return risks, err
}

// GetUserTenants retrieves all tenants for a user
func (ts *TenantService) GetUserTenants(userID uuid.UUID) ([]domain.Tenant, error) {
    var tenants []domain.Tenant

    // Join through UserTenant table
    err := ts.db.
        Joins("JOIN user_tenants ON tenants.id = user_tenants.tenant_id").
        Where("user_tenants.user_id = ?", userID).
        Where("tenants.is_active = true").
        Find(&tenants).Error

    return tenants, err
}

// CreateTenantUser adds user to tenant with role
func (ts *TenantService) CreateTenantUser(userID, tenantID, roleID uuid.UUID) error {
    userTenant := &domain.UserTenant{
        UserID:   userID,
        TenantID: tenantID,
        RoleID:   roleID,
    }
    return ts.db.Create(userTenant).Error
}
```

---

## 5. Audit Logging

### 5.1 Logging Authentication Events

```go
package services

type AuditService struct {
    db *gorm.DB
}

// LogLogin logs authentication attempts
func (as *AuditService) LogLogin(userID uuid.UUID, success bool, ip, userAgent string, errorMsg string) error {
    result := domain.AuditLogResultSuccess
    if !success {
        result = domain.AuditLogResultFailure
    }

    action := domain.AuditLogActionLogin
    if !success {
        action = domain.AuditLogActionLoginFailed
    }

    log := &domain.AuditLog{
        ID:           uuid.New(),
        UserID:       &userID,
        Action:       action,
        Resource:     domain.AuditLogResourceAuth,
        Result:       result,
        ErrorMessage: errorMsg,
        IPAddress:    parseIP(ip),
        UserAgent:    userAgent,
        Timestamp:    time.Now(),
    }

    return as.db.Create(log).Error
}

// LogRoleChange logs when a user's role is changed
func (as *AuditService) LogRoleChange(changedUserID, changedByID uuid.UUID, oldRole, newRole string, ip, userAgent string) error {
    log := &domain.AuditLog{
        ID:       uuid.New(),
        UserID:   &changedByID,
        Action:   domain.AuditLogActionRoleChange,
        Resource: domain.AuditLogResourceRole,
        ResourceID: &changedUserID,
        ErrorMessage: fmt.Sprintf("changed role from %s to %s", oldRole, newRole),
        Result:   domain.AuditLogResultSuccess,
        IPAddress: parseIP(ip),
        UserAgent: userAgent,
        Timestamp: time.Now(),
    }

    return as.db.Create(log).Error
}

// LogTokenRefresh logs token refresh events
func (as *AuditService) LogTokenRefresh(userID uuid.UUID, success bool, ip, userAgent string) error {
    result := domain.AuditLogResultSuccess
    if !success {
        result = domain.AuditLogResultFailure
    }

    log := &domain.AuditLog{
        ID:        uuid.New(),
        UserID:    &userID,
        Action:    domain.AuditLogActionTokenRefresh,
        Resource:  domain.AuditLogResourceAuth,
        Result:    result,
        IPAddress: parseIP(ip),
        UserAgent: userAgent,
        Timestamp: time.Now(),
    }

    return as.db.Create(log).Error
}
```

### 5.2 Querying Audit Logs

```go
package handlers

// GetAuditLogs retrieves audit logs (admin only)
func GetAuditLogs(c *fiber.Ctx) error {
    // Query parameters
    action := c.Query("action", "")
    userID := c.Query("user_id", "")
    limit := c.QueryInt("limit", 50)
    offset := c.QueryInt("offset", 0)

    var logs []domain.AuditLog
    query := db

    if action != "" {
        query = query.Where("action = ?", action)
    }
    if userID != "" {
        query = query.Where("user_id = ?", userID)
    }

    if err := query.
        Order("timestamp DESC").
        Limit(limit).
        Offset(offset).
        Find(&logs).Error; err != nil {
        return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
            "error": "failed to retrieve audit logs",
        })
    }

    return c.JSON(logs)
}

// GetUserAuditLogs gets audit logs for specific user
func GetUserAuditLogs(c *fiber.Ctx) error {
    userID := c.Params("user_id")

    var logs []domain.AuditLog
    if err := db.Where("user_id = ?", userID).
        Order("timestamp DESC").
        Find(&logs).Error; err != nil {
        return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
            "error": "failed to retrieve logs",
        })
    }

    return c.JSON(logs)
}
```

---

## 6. Configuration

### 6.1 Environment Variables

```bash
# JWT Configuration
JWT_SECRET=your-secret-key-here (min 32 characters)
JWT_EXPIRATION_HOURS=24

# Server
SERVER_PORT=8080
SERVER_ENV=production

# Database
DB_HOST=localhost
DB_PORT=5432
DB_USER=openrisk
DB_PASSWORD=secure-password
DB_NAME=openrisk

# CORS
CORS_ALLOWED_ORIGINS=https://example.com,https://api.example.com

# Rate Limiting
RATE_LIMIT_AUTH_REQUESTS=5
RATE_LIMIT_AUTH_WINDOW=60

# OAuth2
OAUTH2_GOOGLE_CLIENT_ID=xxx.apps.googleusercontent.com
OAUTH2_GOOGLE_CLIENT_SECRET=xxx
OAUTH2_GITHUB_CLIENT_ID=xxx
OAUTH2_GITHUB_CLIENT_SECRET=xxx

# SAML2
SAML2_IDP_METADATA_URL=https://idp.example.com/metadata.xml
SAML2_SP_ENTITY_ID=https://openrisk.example.com
SAML2_SP_ACS_URL=https://openrisk.example.com/api/v1/auth/saml/acs
```

### 6.2 Service Initialization

```go
package main

import (
    "os"
    "time"
)

func initializeAuthService() *services.AuthService {
    jwtSecret := os.Getenv("JWT_SECRET")
    if jwtSecret == "" {
        log.Fatal("JWT_SECRET environment variable not set")
    }

    if len(jwtSecret) < 32 {
        log.Fatal("JWT_SECRET must be at least 32 characters")
    }

    ttlHours := 24 // default
    if ttlStr := os.Getenv("JWT_EXPIRATION_HOURS"); ttlStr != "" {
        if hours, err := strconv.Atoi(ttlStr); err == nil {
            ttlHours = hours
        }
    }

    return &services.AuthService{
        jwtSecret: jwtSecret,
        tokenTTL:  time.Duration(ttlHours) * time.Hour,
        db:        dbConn,
    }
}

func initializePermissionService() *services.PermissionService {
    ps := services.NewPermissionService()
    
    // Initialize default role permissions
    ps.InitializeDefaultRoles()

    return ps
}
```

---

## 7. Troubleshooting

### Token Issues

**Problem**: "Invalid or expired token"
```
Solution:
1. Check JWT_SECRET is set correctly
2. Verify token hasn't expired (24 hour default)
3. Use /auth/refresh to get new token
4. Ensure Authorization header format is "Bearer <token>"
```

**Problem**: "Token claims invalid"
```
Solution:
1. Check token signature matches JWT_SECRET
2. Verify all required claims present
3. Check token not tampered with
4. Re-login to get fresh token
```

### Permission Issues

**Problem**: User sees "insufficient permissions"
```
Solution:
1. Check user's role with GET /profile
2. Verify role has required permissions
3. Check resource scope (own/team/any)
4. Contact admin to update permissions
```

**Problem**: Role guard failing for valid user
```
Solution:
1. Verify role name matches exactly (case-sensitive)
2. Check middleware order (Auth before Role)
3. Verify user role in UserClaims
4. Check role not changed since token issued
```

### Multi-Tenant Issues

**Problem**: User sees "you do not have access to this tenant"
```
Solution:
1. Verify user-tenant relationship exists
2. Check UserTenant record in database
3. Verify tenant is_active = true
4. Contact tenant admin for access
```

### Audit Log Issues

**Problem**: Audit logs not appearing
```
Solution:
1. Verify AuditService initialized
2. Check database connection working
3. Verify audit_logs table exists
4. Check for database constraint errors
```

---

## Complete Example: Secured Risk API

```go
package main

import (
    "github.com/gofiber/fiber/v2"
    "middleware"
    "handlers"
    "services"
)

func setupRiskRoutes(app *fiber.App, authService *services.AuthService, permService *services.PermissionService) {
    // Public routes
    api := app.Group("/api/v1")
    
    api.Post("/auth/login", handlers.AuthHandler.Login)
    api.Post("/auth/register", handlers.AuthHandler.Register)
    api.Post("/auth/refresh", handlers.AuthHandler.RefreshToken)

    // Protected routes
    protected := api.Group("", 
        middleware.AuthMiddleware(os.Getenv("JWT_SECRET")))

    // Anyone can view risks
    protected.Get("/risks",
        handlers.GetRisks)

    // Only analysts can create
    protected.Post("/risks",
        middleware.RoleGuard("admin", "analyst"),
        middleware.RequirePermissions(permService, domain.Permission{
            Resource: domain.PermissionResourceRisk,
            Action:   domain.PermissionCreate,
            Scope:    domain.PermissionScopeAny,
        }),
        handlers.CreateRisk)

    // Update with scope checking
    protected.Patch("/risks/:id",
        middleware.RequireResourcePermission(permService, "risk", "update"),
        handlers.UpdateRisk)

    // Only admins can delete
    protected.Delete("/risks/:id",
        middleware.RoleGuard("admin"),
        handlers.DeleteRisk)

    // Audit logs - admin only
    protected.Get("/audit-logs",
        middleware.RoleGuard("admin"),
        handlers.GetAuditLogs)

    // Multi-tenant endpoints
    protected.Get("/rbac/tenants",
        handlers.GetUserTenants)

    protected.Post("/rbac/tenants/:tenant_id/users",
        middleware.TenantIsolation(db),
        middleware.RoleGuard("admin"),
        handlers.AddUserToTenant)
}
```

---

**This guide provides everything needed to implement and use OpenRisk's authentication and RBAC system.**
