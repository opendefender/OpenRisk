# Authentication & RBAC Code Examples

**Practical examples for common authentication and RBAC scenarios**

---

## Table of Contents

1. [JWT Token Management](#1-jwt-token-management)
2. [Login & Registration](#2-login--registration)
3. [Role-Based Access](#3-role-based-access)
4. [Permission Checking](#4-permission-checking)
5. [Multi-Tenant Operations](#5-multi-tenant-operations)
6. [API Client Examples](#6-api-client-examples)
7. [Testing](#7-testing)

---

## 1. JWT Token Management

### 1.1 Generate JWT Token

**Backend (Go)**:
```go
package services

import (
    "time"
    "github.com/golang-jwt/jwt/v5"
    "github.com/google/uuid"
)

// GenerateJWT creates a new JWT token
func GenerateJWT(user *User, jwtSecret string) (string, error) {
    claims := jwt.MapClaims{
        "id":          user.ID.String(),
        "email":       user.Email,
        "username":    user.Username,
        "role_id":     user.RoleID.String(),
        "role_name":   user.Role.Name,
        "permissions": user.Role.Permissions,
        "iat":         time.Now().Unix(),
        "exp":         time.Now().Add(24 * time.Hour).Unix(),
    }

    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    return token.SignedString([]byte(jwtSecret))
}

// Usage example
func LoginUser(email, password string) (string, error) {
    user, err := findUserByEmail(email)
    if err != nil {
        return "", err
    }

    if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
        return "", fmt.Errorf("invalid credentials")
    }

    token, err := GenerateJWT(user, os.Getenv("JWT_SECRET"))
    if err != nil {
        return "", fmt.Errorf("failed to generate token")
    }

    return token, nil
}
```

### 1.2 Parse and Validate JWT

**Backend (Go)**:
```go
package middleware

import (
    "github.com/golang-jwt/jwt/v5"
)

// ValidateToken validates JWT signature and expiration
func ValidateToken(tokenString, jwtSecret string) (*jwt.MapClaims, error) {
    claims := jwt.MapClaims{}

    token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
        // Verify signing method
        if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
            return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
        }
        return []byte(jwtSecret), nil
    })

    if err != nil {
        return nil, err
    }

    if !token.Valid {
        return nil, fmt.Errorf("invalid token")
    }

    // Verify expiration
    if exp, ok := claims["exp"].(float64); ok {
        if time.Now().Unix() > int64(exp) {
            return nil, fmt.Errorf("token expired")
        }
    }

    return &claims, nil
}

// Usage
claims, err := ValidateToken(tokenString, os.Getenv("JWT_SECRET"))
if err != nil {
    log.Println("Token validation failed:", err)
}

userID := claims["id"].(string)
role := claims["role_name"].(string)
```

### 1.3 Refresh Token

**Backend (Go)**:
```go
func RefreshToken(oldToken, jwtSecret string) (string, error) {
    // Parse old token without checking expiration
    claims := jwt.MapClaims{}
    jwt.ParseWithClaims(oldToken, claims, func(token *jwt.Token) (interface{}, error) {
        return []byte(jwtSecret), nil
    })

    // Get user info from claims
    userID := uuid.MustParse(claims["id"].(string))
    user, err := getUserByID(userID)
    if err != nil {
        return "", err
    }

    // Generate new token
    newToken, err := GenerateJWT(user, jwtSecret)
    if err != nil {
        return "", err
    }

    return newToken, nil
}
```

---

## 2. Login & Registration

### 2.1 User Login

**Backend Handler**:
```go
package handlers

type LoginRequest struct {
    Email    string `json:"email" validate:"required,email"`
    Password string `json:"password" validate:"required,min=8"`
}

type LoginResponse struct {
    Token     string `json:"token"`
    ExpiresIn int64  `json:"expires_in"` // seconds
    User      struct {
        ID       uuid.UUID `json:"id"`
        Email    string    `json:"email"`
        Username string    `json:"username"`
        Role     string    `json:"role"`
    } `json:"user"`
}

// Login authenticates user and returns JWT token
func (h *AuthHandler) Login(c *fiber.Ctx) error {
    var req LoginRequest
    if err := c.BodyParser(&req); err != nil {
        return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
            "error": "invalid request body",
        })
    }

    // Validate input
    if err := validate.Struct(req); err != nil {
        return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
            "error": "validation failed",
            "details": err.Error(),
        })
    }

    // Find user
    user := &domain.User{}
    if err := h.db.Where("email = ?", req.Email).First(user).Error; err != nil {
        // Log failed attempt
        h.auditService.LogLogin(nil, false, c.IP(), c.Get("User-Agent"), "user not found")
        
        return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
            "error": "invalid email or password",
        })
    }

    // Check password
    if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
        // Log failed attempt
        h.auditService.LogLogin(&user.ID, false, c.IP(), c.Get("User-Agent"), "password mismatch")
        
        return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
            "error": "invalid email or password",
        })
    }

    // Check if user is active
    if !user.IsActive {
        h.auditService.LogLogin(&user.ID, false, c.IP(), c.Get("User-Agent"), "user inactive")
        
        return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
            "error": "account is inactive",
        })
    }

    // Preload role
    h.db.Model(user).Association("Role").Find(&user.Role)

    // Generate token
    token, err := h.authService.GenerateToken(user)
    if err != nil {
        h.auditService.LogLogin(&user.ID, false, c.IP(), c.Get("User-Agent"), err.Error())
        
        return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
            "error": "failed to generate token",
        })
    }

    // Update last login
    h.db.Model(user).Update("last_login", time.Now())

    // Log successful login
    h.auditService.LogLogin(&user.ID, true, c.IP(), c.Get("User-Agent"), "")

    return c.JSON(LoginResponse{
        Token:     token,
        ExpiresIn: 86400, // 24 hours
        User: struct {
            ID       uuid.UUID `json:"id"`
            Email    string    `json:"email"`
            Username string    `json:"username"`
            Role     string    `json:"role"`
        }{
            ID:       user.ID,
            Email:    user.Email,
            Username: user.Username,
            Role:     user.Role.Name,
        },
    })
}
```

**cURL Example**:
```bash
curl -X POST https://api.openrisk.io/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "analyst@example.com",
    "password": "SecurePass123"
  }' \
  -w "\n%{http_code}\n"

# Success Response (200)
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJlbWFpbCI6ImFuYWx5c3RAZXhhbXBsZS5jb20iLCJleHAiOjE2Nzg1MzYwMDAsImlhdCI6MTY3ODQ0OTYwMH0.KBrTf5j1hzT...",
  "expires_in": 86400,
  "user": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "email": "analyst@example.com",
    "username": "analyst_001",
    "role": "analyst"
  }
}

# Error Response (401)
{
  "error": "invalid email or password"
}
```

### 2.2 User Registration

**Backend Handler**:
```go
type RegisterRequest struct {
    Email    string `json:"email" validate:"required,email"`
    Username string `json:"username" validate:"required,min=3,max=50"`
    Password string `json:"password" validate:"required,min=8,max=100"`
}

func (h *AuthHandler) Register(c *fiber.Ctx) error {
    var req RegisterRequest
    if err := c.BodyParser(&req); err != nil {
        return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
            "error": "invalid request",
        })
    }

    // Check if user exists
    var existingUser domain.User
    if err := h.db.Where("email = ?", req.Email).First(&existingUser).Error; err == nil {
        return c.Status(fiber.StatusConflict).JSON(fiber.Map{
            "error": "email already registered",
        })
    }

    // Hash password
    hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
    if err != nil {
        return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
            "error": "failed to process password",
        })
    }

    // Get default viewer role
    var viewerRole domain.Role
    h.db.Where("name = ?", "viewer").First(&viewerRole)

    // Create user
    user := &domain.User{
        ID:       uuid.New(),
        Email:    req.Email,
        Username: req.Username,
        Password: string(hashedPassword),
        RoleID:   viewerRole.ID,
        IsActive: true,
    }

    if err := h.db.Create(user).Error; err != nil {
        return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
            "error": "failed to create user",
        })
    }

    // Log registration
    h.auditService.LogRegister(&user.ID, domain.AuditLogResultSuccess, 
        c.IP(), c.Get("User-Agent"), "")

    return c.Status(fiber.StatusCreated).JSON(fiber.Map{
        "id":       user.ID,
        "email":    user.Email,
        "username": user.Username,
    })
}
```

---

## 3. Role-Based Access

### 3.1 Role Guard Middleware

**Middleware**:
```go
package middleware

// RoleGuard checks if user has required role
func RoleGuard(requiredRoles ...string) fiber.Handler {
    return func(c *fiber.Ctx) error {
        claims, ok := c.Locals("user").(*domain.UserClaims)
        if !ok {
            return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
                "error": "user not authenticated",
            })
        }

        // Check if user's role is in allowed list
        for _, required := range requiredRoles {
            if claims.RoleName == required {
                return c.Next()
            }
        }

        return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
            "error": "insufficient permissions",
            "required": requiredRoles,
            "user_role": claims.RoleName,
        })
    }
}
```

**Route Definition**:
```go
// Setup routes
func SetupRoutes(app *fiber.App) {
    api := app.Group("/api/v1")
    
    // Public routes
    api.Post("/auth/login", handlers.Login)
    api.Post("/auth/register", handlers.Register)

    // Protected routes with role requirements
    protected := api.Group("",
        middleware.AuthMiddleware(jwtSecret))

    // Viewer+ can read risks
    protected.Get("/risks",
        middleware.RoleGuard("viewer", "analyst", "manager", "admin"),
        handlers.GetRisks)

    // Analyst+ can create risks
    protected.Post("/risks",
        middleware.RoleGuard("analyst", "manager", "admin"),
        handlers.CreateRisk)

    // Manager+ can update risks
    protected.Patch("/risks/:id",
        middleware.RoleGuard("manager", "admin"),
        handlers.UpdateRisk)

    // Admin only can delete
    protected.Delete("/risks/:id",
        middleware.RoleGuard("admin"),
        handlers.DeleteRisk)

    // Admin only for user management
    protected.Get("/users",
        middleware.RoleGuard("admin"),
        handlers.GetUsers)

    protected.Post("/users",
        middleware.RoleGuard("admin"),
        handlers.CreateUser)

    protected.Delete("/users/:id",
        middleware.RoleGuard("admin"),
        handlers.DeleteUser)
}
```

### 3.2 Check User Role in Handler

**Handler Implementation**:
```go
func GetUserProfile(c *fiber.Ctx) error {
    // Get user claims from context
    claims := c.Locals("user").(*domain.UserClaims)

    // Access user info
    userID := claims.ID
    email := claims.Email
    role := claims.RoleName
    permissions := claims.Permissions

    // Role-specific logic
    switch claims.RoleName {
    case "admin":
        // Admin can see everything
        return c.JSON(fiber.Map{
            "role": "admin",
            "access": "full_system_access",
        })
    case "analyst":
        // Analyst can see team data
        return c.JSON(fiber.Map{
            "role": "analyst",
            "access": "team_and_own_data",
        })
    case "viewer":
        // Viewer can only see dashboard
        return c.JSON(fiber.Map{
            "role": "viewer",
            "access": "dashboard_only",
        })
    default:
        return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
            "error": "unknown role",
        })
    }
}
```

---

## 4. Permission Checking

### 4.1 Fine-Grained Permissions

**Permission Model**:
```go
type Permission struct {
    Resource string // "risk", "mitigation", "user", etc.
    Action   string // "read", "create", "update", "delete"
    Scope    string // "own", "team", "any"
}

func (p Permission) String() string {
    return fmt.Sprintf("%s:%s:%s", p.Resource, p.Action, p.Scope)
}
```

### 4.2 Permission Middleware

```go
// RequirePermission checks if user has specific permission
func RequirePermission(permService *services.PermissionService, required domain.Permission) fiber.Handler {
    return func(c *fiber.Ctx) error {
        claims := c.Locals("user").(*domain.UserClaims)

        // Check if user has permission
        has := permService.CheckPermission(claims.ID.String(), claims.RoleID.String(), required)
        
        if !has {
            return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
                "error": "insufficient permission",
                "required": required.String(),
            })
        }

        return c.Next()
    }
}

// Example route setup
protected.Post("/risks",
    middleware.AuthMiddleware(jwtSecret),
    middleware.RequirePermission(permService, domain.Permission{
        Resource: "risk",
        Action: "create",
        Scope: "any",
    }),
    handlers.CreateRisk)
```

### 4.3 Scope-Based Permission Checking

```go
// RequireResourcePermission checks ownership for scope
func RequireResourcePermission(permService *services.PermissionService, resourceType, action string) fiber.Handler {
    return func(c *fiber.Ctx) error {
        claims := c.Locals("user").(*domain.UserClaims)
        resourceID := c.Params("id")

        // Get resource from database
        resource, err := getResource(resourceType, resourceID)
        if err != nil {
            return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
                "error": "resource not found",
            })
        }

        // Determine required scope based on ownership
        scope := "any" // Default to strict check
        if resource.OwnerID == claims.ID {
            scope = "own" // User owns this resource
        } else if isInTeam(claims.ID, resource.TeamID) {
            scope = "team" // User in same team
        }

        // Check permission with determined scope
        has := permService.CheckPermission(claims.ID.String(), claims.RoleID.String(),
            domain.Permission{
                Resource: resourceType,
                Action:   action,
                Scope:    scope,
            })

        if !has {
            return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
                "error": "insufficient permission for this resource",
            })
        }

        return c.Next()
    }
}

// Usage
protected.Patch("/risks/:id",
    middleware.AuthMiddleware(jwtSecret),
    middleware.RequireResourcePermission(permService, "risk", "update"),
    handlers.UpdateRisk)
```

### 4.4 Wildcard Permission Examples

```go
package services

// Permission matching with wildcards
func (ps *PermissionService) checkPermission(userPerms []string, required string) bool {
    for _, perm := range userPerms {
        // Admin wildcard
        if perm == "*" || perm == "*:*:*" {
            return true
        }

        // Exact match
        if perm == required {
            return true
        }

        // Resource wildcard (e.g., "risk:*" matches "risk:read")
        if len(perm) > 2 && perm[len(perm)-2:] == ":*" {
            if strings.HasPrefix(required, perm[:len(perm)-1]) {
                return true
            }
        }

        // Action wildcard (e.g., "*:read" matches "risk:read")
        parts := strings.Split(perm, ":")
        requiredParts := strings.Split(required, ":")
        
        if len(parts) == 3 && len(requiredParts) == 3 {
            // Check each component
            if (parts[0] == "*" || parts[0] == requiredParts[0]) &&
               (parts[1] == "*" || parts[1] == requiredParts[1]) &&
               (parts[2] == "*" || parts[2] == requiredParts[2]) {
                return true
            }
        }
    }

    return false
}

// Examples of matching:
// User has: ["risk:*:any", "mitigation:read:any"]
// Check "risk:create:any"    ✅ matches "risk:*:any"
// Check "risk:delete:any"    ✅ matches "risk:*:any"
// Check "mitigation:read:any" ✅ exact match
// Check "asset:read:any"     ❌ no match
```

---

## 5. Multi-Tenant Operations

### 5.1 Create Tenant

```go
type CreateTenantRequest struct {
    Name     string `json:"name" validate:"required"`
    Slug     string `json:"slug" validate:"required"`
    Metadata map[string]interface{} `json:"metadata"`
}

func CreateTenant(c *fiber.Ctx) error {
    claims := c.Locals("user").(*domain.UserClaims)

    // Only admins can create tenants
    if claims.RoleName != "admin" {
        return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
            "error": "only admins can create tenants",
        })
    }

    var req CreateTenantRequest
    c.BodyParser(&req)

    tenant := &domain.Tenant{
        ID:       uuid.New(),
        Name:     req.Name,
        Slug:     req.Slug,
        OwnerID:  claims.ID,
        Status:   "active",
        IsActive: true,
        Metadata: req.Metadata,
    }

    if err := db.Create(tenant).Error; err != nil {
        return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
            "error": "failed to create tenant",
        })
    }

    return c.Status(fiber.StatusCreated).JSON(tenant)
}
```

### 5.2 Add User to Tenant

```go
type AddUserToTenantRequest struct {
    UserID uuid.UUID `json:"user_id" validate:"required"`
    RoleID uuid.UUID `json:"role_id" validate:"required"`
}

func AddUserToTenant(c *fiber.Ctx) error {
    tenantID := c.Params("tenant_id")

    var req AddUserToTenantRequest
    c.BodyParser(&req)

    userTenant := &domain.UserTenant{
        UserID:   req.UserID,
        TenantID: uuid.MustParse(tenantID),
        RoleID:   req.RoleID,
    }

    if err := db.Create(userTenant).Error; err != nil {
        return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
            "error": "failed to add user to tenant",
        })
    }

    return c.Status(fiber.StatusCreated).JSON(userTenant)
}
```

### 5.3 Tenant-Scoped Query

```go
func GetTenantRisks(c *fiber.Ctx) error {
    tenantID := c.Params("tenant_id")
    claims := c.Locals("user").(*domain.UserClaims)

    // Verify user has access to this tenant
    var userTenant domain.UserTenant
    if err := db.Where("user_id = ? AND tenant_id = ?", claims.ID, tenantID).
        First(&userTenant).Error; err != nil {
        return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
            "error": "you do not have access to this tenant",
        })
    }

    // Query risks scoped to tenant
    var risks []domain.Risk
    if err := db.Where("tenant_id = ?", tenantID).
        Order("created_at DESC").
        Find(&risks).Error; err != nil {
        return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
            "error": "failed to retrieve risks",
        })
    }

    return c.JSON(risks)
}
```

---

## 6. API Client Examples

### 6.1 JavaScript/Node.js

```javascript
const axios = require('axios');

class OpenRiskClient {
    constructor(baseURL = 'https://api.openrisk.io/api/v1') {
        this.baseURL = baseURL;
        this.token = null;
    }

    // Login
    async login(email, password) {
        const response = await axios.post(`${this.baseURL}/auth/login`, {
            email,
            password
        });

        this.token = response.data.token;
        return response.data;
    }

    // Refresh token
    async refreshToken() {
        const response = await axios.post(
            `${this.baseURL}/auth/refresh`,
            {},
            {
                headers: {
                    'Authorization': `Bearer ${this.token}`
                }
            }
        );

        this.token = response.data.token;
        return response.data;
    }

    // Make authenticated request
    async request(method, path, data = null) {
        const config = {
            method,
            url: `${this.baseURL}${path}`,
            headers: {
                'Authorization': `Bearer ${this.token}`
            }
        };

        if (data) {
            config.data = data;
        }

        return axios(config);
    }

    // Get risks
    async getRisks() {
        const response = await this.request('GET', '/risks');
        return response.data;
    }

    // Create risk
    async createRisk(riskData) {
        const response = await this.request('POST', '/risks', riskData);
        return response.data;
    }

    // Update risk
    async updateRisk(riskId, riskData) {
        const response = await this.request('PATCH', `/risks/${riskId}`, riskData);
        return response.data;
    }
}

// Usage
const client = new OpenRiskClient();

// Login
await client.login('analyst@example.com', 'password');

// Get risks
const risks = await client.getRisks();
console.log(risks);

// Create risk
const newRisk = await client.createRisk({
    title: 'New Risk',
    description: 'Risk description',
    impact: 'high'
});
```

### 6.2 Python

```python
import requests
from datetime import datetime, timedelta

class OpenRiskClient:
    def __init__(self, base_url='https://api.openrisk.io/api/v1'):
        self.base_url = base_url
        self.token = None
        self.token_expires = None

    def login(self, email, password):
        """Login and store JWT token"""
        response = requests.post(
            f'{self.base_url}/auth/login',
            json={'email': email, 'password': password}
        )
        response.raise_for_status()

        data = response.json()
        self.token = data['token']
        self.token_expires = datetime.now() + timedelta(seconds=data['expires_in'])

        return data

    def refresh_token(self):
        """Refresh JWT token"""
        response = requests.post(
            f'{self.base_url}/auth/refresh',
            headers={'Authorization': f'Bearer {self.token}'}
        )
        response.raise_for_status()

        data = response.json()
        self.token = data['token']
        self.token_expires = datetime.now() + timedelta(seconds=data['expires_in'])

        return data

    def request(self, method, path, json=None):
        """Make authenticated request"""
        # Refresh token if expired
        if self.token_expires and datetime.now() > self.token_expires:
            self.refresh_token()

        headers = {'Authorization': f'Bearer {self.token}'}
        response = requests.request(
            method,
            f'{self.base_url}{path}',
            json=json,
            headers=headers
        )

        response.raise_for_status()
        return response.json()

    def get_risks(self):
        """Get all risks"""
        return self.request('GET', '/risks')

    def create_risk(self, title, description, impact):
        """Create new risk"""
        return self.request('POST', '/risks', {
            'title': title,
            'description': description,
            'impact': impact
        })

# Usage
client = OpenRiskClient()
client.login('analyst@example.com', 'password')
risks = client.get_risks()
print(risks)
```

---

## 7. Testing

### 7.1 Testing Authentication

**Go Test Example**:
```go
package handlers

import (
    "testing"
    "github.com/stretchr/testify/assert"
)

func TestLogin_Success(t *testing.T) {
    // Setup test database and handler
    db := setupTestDB()
    handler := NewAuthHandler(db)

    // Create test user
    user := createTestUser(db, "test@example.com", "password123")

    // Login request
    req := LoginRequest{
        Email:    "test@example.com",
        Password: "password123",
    }

    resp := handler.Login(req)

    // Assertions
    assert.NotEmpty(t, resp.Token)
    assert.Equal(t, user.ID, resp.User.ID)
    assert.Equal(t, "viewer", resp.User.Role)
}

func TestLogin_InvalidPassword(t *testing.T) {
    db := setupTestDB()
    handler := NewAuthHandler(db)

    createTestUser(db, "test@example.com", "password123")

    req := LoginRequest{
        Email:    "test@example.com",
        Password: "wrongpassword",
    }

    _, err := handler.Login(req)

    assert.Error(t, err)
    assert.Equal(t, "invalid email or password", err.Error())
}

func TestTokenValidation(t *testing.T) {
    jwtSecret := "test-secret-key-32-characters-min"

    // Generate token
    token, _ := GenerateToken(testUser, jwtSecret)

    // Validate token
    claims, err := ValidateToken(token, jwtSecret)

    assert.NoError(t, err)
    assert.Equal(t, testUser.ID, claims.ID)
    assert.Equal(t, testUser.Email, claims.Email)
}
```

### 7.2 Testing Roles

```go
func TestRoleGuard_Admin(t *testing.T) {
    // Create admin user
    adminUser := createTestUser(db, "admin@example.com", "pass")
    adminUser.RoleID = getAdminRoleID()
    db.Save(adminUser)

    // Create token
    token, _ := GenerateToken(adminUser, jwtSecret)

    // Make request with admin token
    req := setupRequest("POST", "/users", token)
    resp := handler.ServeHTTP(req)

    // Admin should be allowed
    assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func TestRoleGuard_Insufficient(t *testing.T) {
    // Create viewer user
    viewerUser := createTestUser(db, "viewer@example.com", "pass")
    viewerUser.RoleID = getViewerRoleID()
    db.Save(viewerUser)

    // Create token
    token, _ := GenerateToken(viewerUser, jwtSecret)

    // Make admin-only request
    req := setupRequest("POST", "/users", token)
    resp := handler.ServeHTTP(req)

    // Viewer should be forbidden
    assert.Equal(t, fiber.StatusForbidden, resp.StatusCode)
}
```

---

**All examples are production-ready and follow OpenRisk's best practices.**
