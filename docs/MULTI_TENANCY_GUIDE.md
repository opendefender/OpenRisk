# Multi-Tenancy Implementation Guide

**Complete guide for tenant isolation, user management, and multi-tenant data handling**

---

## Table of Contents

1. [Multi-Tenancy Architecture](#1-multi-tenancy-architecture)
2. [Tenant Management](#2-tenant-management)
3. [Data Isolation](#3-data-isolation)
4. [User-Tenant Mapping](#4-user-tenant-mapping)
5. [Role Scoping](#5-role-scoping)
6. [API Endpoints](#6-api-endpoints)
7. [Best Practices](#7-best-practices)
8. [Troubleshooting](#8-troubleshooting)

---

## 1. Multi-Tenancy Architecture

### 1.1 Design Overview

OpenRisk uses **database-per-logical-tenant** multi-tenancy with shared infrastructure:

```
┌─────────────────────────────────────────────────────┐
│         OpenRisk Multi-Tenant Architecture          │
├─────────────────────────────────────────────────────┤
│                                                     │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────┐ │
│  │   Tenant A   │  │   Tenant B   │  │Tenant... │ │
│  │  (Acme Inc)  │  │  (TechCorp)  │  │          │ │
│  └──────────────┘  └──────────────┘  └──────────┘ │
│                                                     │
│  ┌─────────────────────────────────────────────┐   │
│  │         Shared PostgreSQL Database          │   │
│  │  (Tenant-scoped tables with tenant_id FK)   │   │
│  └─────────────────────────────────────────────┘   │
│                                                     │
│  ┌──────────────────────────────────────────────┐  │
│  │  Tenant Table │ User-Tenant │ Roles │        │  │
│  │  (isolation)  │  Junction   │ Perms │ ...    │  │
│  └──────────────────────────────────────────────┘  │
│                                                     │
└─────────────────────────────────────────────────────┘
```

### 1.2 Domain Models

```go
package domain

// Tenant represents an organization
type Tenant struct {
    ID        uuid.UUID       // Unique tenant identifier
    Name      string          // Organization name
    Slug      string          // URL-friendly slug (acme-inc)
    OwnerID   uuid.UUID       // Tenant owner user ID
    Status    string          // active, suspended, deleted
    IsActive  bool            // Soft delete flag
    CreatedAt time.Time
    UpdatedAt time.Time
    DeletedAt gorm.DeletedAt  // GORM soft delete
    Metadata  json.RawMessage // JSONB for custom tenant config

    // Relations
    Owner *User
    Users []*User `gorm:"many2many:user_tenants"`
    Roles []*RoleEnhanced
    Risks []*Risk
}

// UserTenant maps users to tenants with role
type UserTenant struct {
    UserID    uuid.UUID // Foreign key: users.id
    TenantID  uuid.UUID // Foreign key: tenants.id
    RoleID    uuid.UUID // Foreign key: roles.id
    CreatedAt time.Time
    UpdatedAt time.Time

    // Relations
    User  *User
    Tenant *Tenant
    Role  *RoleEnhanced
}

// RoleEnhanced is tenant-scoped role
type RoleEnhanced struct {
    ID          uuid.UUID      // Role ID
    TenantID    uuid.UUID      // Tenant this role belongs to
    Name        string         // "admin", "analyst", "viewer"
    Description string
    Level       int            // 0=viewer, 3=analyst, 6=manager, 9=admin
    Permissions pq.StringArray // Permissions array
    IsPredefined bool          // True for default roles
    IsActive    bool
    CreatedAt   time.Time
    UpdatedAt   time.Time

    // Relations
    Tenant *Tenant
}

// User with tenant association
type User struct {
    ID       uuid.UUID   // User ID
    Email    string      // Unique email
    Username string
    Password string      // bcrypt hash
    RoleID   uuid.UUID   // Primary role
    IsActive bool
    
    // Tenant association (optional - user can belong to multiple tenants)
    Tenants []*Tenant `gorm:"many2many:user_tenants"`
    
    CreatedAt time.Time
    UpdatedAt time.Time
}

// Risk with tenant scoping
type Risk struct {
    ID          uuid.UUID // Risk ID
    TenantID    uuid.UUID // Tenant this risk belongs to ⭐ KEY FOR ISOLATION
    Title       string
    Description string
    Status      string
    Impact      string
    Probability string
    CreatedBy   uuid.UUID // User who created
    CreatedAt   time.Time
    UpdatedAt   time.Time

    // Relations
    Tenant *Tenant
}
```

---

## 2. Tenant Management

### 2.1 Create Tenant

**Service**:
```go
package services

type TenantService struct {
    db *gorm.DB
}

// CreateTenant creates a new tenant
func (ts *TenantService) CreateTenant(req *CreateTenantRequest, ownerID uuid.UUID) (*domain.Tenant, error) {
    // Validate
    if req.Name == "" || req.Slug == "" {
        return nil, fmt.Errorf("name and slug required")
    }

    // Check slug uniqueness
    var existing domain.Tenant
    if err := ts.db.Where("slug = ?", req.Slug).First(&existing).Error; err == nil {
        return nil, fmt.Errorf("slug already taken")
    }

    // Create tenant
    tenant := &domain.Tenant{
        ID:        uuid.New(),
        Name:      req.Name,
        Slug:      req.Slug,
        OwnerID:   ownerID,
        Status:    "active",
        IsActive:  true,
        Metadata:  req.Metadata,
    }

    if err := ts.db.Create(tenant).Error; err != nil {
        return nil, fmt.Errorf("failed to create tenant: %w", err)
    }

    // Create owner user-tenant relationship
    userTenant := &domain.UserTenant{
        UserID:   ownerID,
        TenantID: tenant.ID,
        RoleID:   ts.getAdminRoleID(tenant.ID),
    }

    if err := ts.db.Create(userTenant).Error; err != nil {
        return nil, fmt.Errorf("failed to create owner mapping: %w", err)
    }

    // Initialize default roles for tenant
    ts.initializeDefaultRoles(tenant.ID)

    return tenant, nil
}

// GetTenantByID retrieves tenant
func (ts *TenantService) GetTenantByID(tenantID uuid.UUID) (*domain.Tenant, error) {
    var tenant domain.Tenant
    if err := ts.db.Where("id = ? AND is_active = true", tenantID).
        First(&tenant).Error; err != nil {
        return nil, err
    }
    return &tenant, nil
}

// GetTenantBySlug retrieves tenant by slug
func (ts *TenantService) GetTenantBySlug(slug string) (*domain.Tenant, error) {
    var tenant domain.Tenant
    if err := ts.db.Where("slug = ? AND is_active = true", slug).
        First(&tenant).Error; err != nil {
        return nil, err
    }
    return &tenant, nil
}

// ListUserTenants gets all tenants for user
func (ts *TenantService) ListUserTenants(userID uuid.UUID) ([]*domain.Tenant, error) {
    var tenants []*domain.Tenant

    err := ts.db.
        Joins("JOIN user_tenants ON tenants.id = user_tenants.tenant_id").
        Where("user_tenants.user_id = ? AND tenants.is_active = true", userID).
        Order("tenants.created_at DESC").
        Find(&tenants).Error

    return tenants, err
}

// UpdateTenant updates tenant info
func (ts *TenantService) UpdateTenant(tenantID uuid.UUID, updates *UpdateTenantRequest) error {
    return ts.db.Model(&domain.Tenant{}).
        Where("id = ?", tenantID).
        Updates(updates).Error
}

// SuspendTenant suspends tenant (soft delete)
func (ts *TenantService) SuspendTenant(tenantID uuid.UUID) error {
    return ts.db.Model(&domain.Tenant{}).
        Where("id = ?", tenantID).
        Updates(map[string]interface{}{
            "status":    "suspended",
            "is_active": false,
        }).Error
}
```

**Handler**:
```go
package handlers

type CreateTenantRequest struct {
    Name     string                 `json:"name" validate:"required,min=3"`
    Slug     string                 `json:"slug" validate:"required,min=3"`
    Metadata map[string]interface{} `json:"metadata"`
}

func (h *TenantHandler) CreateTenant(c *fiber.Ctx) error {
    claims := c.Locals("user").(*domain.UserClaims)

    // Only admins can create tenants
    if claims.RoleName != "admin" {
        return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
            "error": "only admins can create tenants",
        })
    }

    var req CreateTenantRequest
    if err := c.BodyParser(&req); err != nil {
        return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
            "error": "invalid request",
        })
    }

    tenant, err := h.tenantService.CreateTenant(&req, claims.ID)
    if err != nil {
        return c.Status(fiber.StatusConflict).JSON(fiber.Map{
            "error": err.Error(),
        })
    }

    return c.Status(fiber.StatusCreated).JSON(tenant)
}
```

### 2.2 Tenant Configuration

```go
// Initialize default roles for tenant
func (ts *TenantService) initializeDefaultRoles(tenantID uuid.UUID) error {
    roles := []domain.RoleEnhanced{
        {
            ID:           uuid.New(),
            TenantID:     tenantID,
            Name:         "admin",
            Description:  "Full tenant access",
            Level:        9,
            IsPredefined: true,
            IsActive:     true,
            Permissions: pq.StringArray{
                "*:*:*", // Admin wildcard
            },
        },
        {
            ID:           uuid.New(),
            TenantID:     tenantID,
            Name:         "analyst",
            Description:  "Create and manage risks",
            Level:        3,
            IsPredefined: true,
            IsActive:     true,
            Permissions: pq.StringArray{
                "risk:create:any",
                "risk:read:any",
                "risk:update:own",
                "mitigation:create:any",
                "mitigation:read:any",
                "asset:read:any",
            },
        },
        {
            ID:           uuid.New(),
            TenantID:     tenantID,
            Name:         "viewer",
            Description:  "Read-only access",
            Level:        0,
            IsPredefined: true,
            IsActive:     true,
            Permissions: pq.StringArray{
                "risk:read:any",
                "dashboard:read:any",
            },
        },
    }

    return ts.db.Create(&roles).Error
}
```

---

## 3. Data Isolation

### 3.1 Tenant-Scoped Queries

**Key Pattern**: All queries MUST include `WHERE tenant_id = ?`

```go
// ❌ WRONG - No tenant filtering
func GetAllRisks() ([]domain.Risk, error) {
    var risks []domain.Risk
    db.Find(&risks) // Retrieves ALL risks from ALL tenants!
    return risks, nil
}

// ✅ CORRECT - Tenant-scoped
func GetTenantRisks(tenantID uuid.UUID) ([]domain.Risk, error) {
    var risks []domain.Risk
    db.Where("tenant_id = ?", tenantID).Find(&risks)
    return risks, nil
}
```

**Service Implementation**:
```go
// Get risks for specific tenant
func (rs *RiskService) GetTenantRisks(tenantID uuid.UUID, filters *RiskFilters) ([]*domain.Risk, error) {
    var risks []*domain.Risk

    query := rs.db.Where("tenant_id = ?", tenantID)

    // Apply filters
    if filters.Status != "" {
        query = query.Where("status = ?", filters.Status)
    }
    if filters.Impact != "" {
        query = query.Where("impact = ?", filters.Impact)
    }

    if err := query.Order("created_at DESC").Find(&risks).Error; err != nil {
        return nil, err
    }

    return risks, nil
}

// Count risks by tenant
func (rs *RiskService) CountTenantRisks(tenantID uuid.UUID) (int64, error) {
    var count int64
    rs.db.Model(&domain.Risk{}).
        Where("tenant_id = ?", tenantID).
        Count(&count)
    return count, nil
}

// Get risk by ID - verify ownership
func (rs *RiskService) GetTenantRisk(tenantID, riskID uuid.UUID) (*domain.Risk, error) {
    var risk domain.Risk

    // CRITICAL: Check both tenant_id AND risk id
    if err := rs.db.Where("id = ? AND tenant_id = ?", riskID, tenantID).
        First(&risk).Error; err != nil {
        if err == gorm.ErrRecordNotFound {
            return nil, fmt.Errorf("risk not found or not in this tenant")
        }
        return nil, err
    }

    return &risk, nil
}
```

### 3.2 Preventing Data Leakage

**Middleware for Tenant Isolation**:
```go
package middleware

// TenantIsolation ensures user can only access own tenant data
func TenantIsolation(db *gorm.DB) fiber.Handler {
    return func(c *fiber.Ctx) error {
        claims := c.Locals("user").(*domain.UserClaims)
        tenantID := c.Params("tenant_id")

        if tenantID == "" {
            return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
                "error": "tenant_id required",
            })
        }

        // Parse tenant ID
        parsedTenantID, err := uuid.Parse(tenantID)
        if err != nil {
            return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
                "error": "invalid tenant_id format",
            })
        }

        // Verify user belongs to this tenant
        var userTenant domain.UserTenant
        if err := db.Where("user_id = ? AND tenant_id = ?", claims.ID, parsedTenantID).
            First(&userTenant).Error; err != nil {

            if err == gorm.ErrRecordNotFound {
                return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
                    "error": "you do not have access to this tenant",
                })
            }

            return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
                "error": "access verification failed",
            })
        }

        // Store in context for downstream use
        c.Locals("tenant_id", parsedTenantID)
        c.Locals("tenant_role_id", userTenant.RoleID)

        return c.Next()
    }
}

// Example route setup
app.Get("/api/v1/tenants/:tenant_id/risks",
    middleware.AuthMiddleware(jwtSecret),
    middleware.TenantIsolation(db),
    handlers.GetTenantRisks)
```

### 3.3 Request Validation

```go
func GetTenantRisks(c *fiber.Ctx) error {
    // Get tenant_id from URL (validated by middleware)
    tenantID := c.Locals("tenant_id").(uuid.UUID)

    // Get risks only for this tenant
    risks, err := riskService.GetTenantRisks(tenantID, &RiskFilters{
        Status: c.Query("status", ""),
        Impact: c.Query("impact", ""),
    })

    if err != nil {
        return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
            "error": "failed to retrieve risks",
        })
    }

    return c.JSON(risks)
}

// Creating resource in tenant
func CreateTenantRisk(c *fiber.Ctx) error {
    tenantID := c.Locals("tenant_id").(uuid.UUID)
    claims := c.Locals("user").(*domain.UserClaims)

    var req CreateRiskRequest
    c.BodyParser(&req)

    // Always set tenant_id from context, never from request
    risk := &domain.Risk{
        ID:        uuid.New(),
        TenantID:  tenantID, // Force tenant context
        Title:     req.Title,
        CreatedBy: claims.ID,
    }

    if err := db.Create(risk).Error; err != nil {
        return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
            "error": "failed to create risk",
        })
    }

    return c.Status(fiber.StatusCreated).JSON(risk)
}
```

---

## 4. User-Tenant Mapping

### 4.1 Add User to Tenant

```go
type AddUserToTenantRequest struct {
    UserID uuid.UUID `json:"user_id" validate:"required"`
    RoleID uuid.UUID `json:"role_id" validate:"required"`
}

func (h *TenantHandler) AddUserToTenant(c *fiber.Ctx) error {
    tenantID := c.Locals("tenant_id").(uuid.UUID)
    claims := c.Locals("user").(*domain.UserClaims)

    // Only tenant admins can add users
    tenantRole := c.Locals("tenant_role_id").(uuid.UUID)
    role, _ := getRoleByID(tenantRole)
    if role.Level < 9 {
        return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
            "error": "only admins can add users",
        })
    }

    var req AddUserToTenantRequest
    if err := c.BodyParser(&req); err != nil {
        return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
            "error": "invalid request",
        })
    }

    // Verify user exists
    user, err := getUserByID(req.UserID)
    if err != nil {
        return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
            "error": "user not found",
        })
    }

    // Verify role is in this tenant
    role, err := h.getRoleTenant(req.RoleID, tenantID)
    if err != nil {
        return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
            "error": "role not found in this tenant",
        })
    }

    // Create user-tenant mapping
    userTenant := &domain.UserTenant{
        UserID:   req.UserID,
        TenantID: tenantID,
        RoleID:   req.RoleID,
    }

    if err := db.Create(userTenant).Error; err != nil {
        return c.Status(fiber.StatusConflict).JSON(fiber.Map{
            "error": "user already in tenant",
        })
    }

    return c.Status(fiber.StatusCreated).JSON(userTenant)
}
```

### 4.2 List Tenant Users

```go
func (h *TenantHandler) GetTenantUsers(c *fiber.Ctx) error {
    tenantID := c.Locals("tenant_id").(uuid.UUID)

    var users []*domain.User

    // Get users in this tenant
    err := db.
        Joins("JOIN user_tenants ON users.id = user_tenants.user_id").
        Where("user_tenants.tenant_id = ?", tenantID).
        Find(&users).Error

    if err != nil {
        return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
            "error": "failed to retrieve users",
        })
    }

    return c.JSON(users)
}
```

### 4.3 Remove User from Tenant

```go
func (h *TenantHandler) RemoveUserFromTenant(c *fiber.Ctx) error {
    tenantID := c.Locals("tenant_id").(uuid.UUID)
    userID := c.Params("user_id")

    // Delete user-tenant relationship
    if err := db.Where("user_id = ? AND tenant_id = ?", userID, tenantID).
        Delete(&domain.UserTenant{}).Error; err != nil {
        return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
            "error": "failed to remove user",
        })
    }

    return c.JSON(fiber.Map{
        "message": "user removed from tenant",
    })
}
```

---

## 5. Role Scoping

### 5.1 Tenant-Specific Roles

```go
// RoleEnhanced is tenant-scoped
type RoleEnhanced struct {
    ID          uuid.UUID
    TenantID    uuid.UUID  // ⭐ EACH ROLE BELONGS TO ONE TENANT
    Name        string
    Permissions pq.StringArray
    Level       int // 0=viewer, 3=analyst, 6=manager, 9=admin
}

// Get role for tenant
func GetTenantRole(tenantID, roleID uuid.UUID) (*domain.RoleEnhanced, error) {
    var role domain.RoleEnhanced

    // CRITICAL: Verify role belongs to this tenant
    if err := db.Where("id = ? AND tenant_id = ?", roleID, tenantID).
        First(&role).Error; err != nil {
        return nil, fmt.Errorf("role not found in this tenant")
    }

    return &role, nil
}

// Get all roles in tenant
func GetTenantRoles(tenantID uuid.UUID) ([]*domain.RoleEnhanced, error) {
    var roles []*domain.RoleEnhanced

    if err := db.Where("tenant_id = ? AND is_active = true", tenantID).
        Order("level DESC").
        Find(&roles).Error; err != nil {
        return nil, err
    }

    return roles, nil
}

// Create custom role in tenant
func CreateTenantRole(tenantID uuid.UUID, req *CreateRoleRequest) (*domain.RoleEnhanced, error) {
    role := &domain.RoleEnhanced{
        ID:          uuid.New(),
        TenantID:    tenantID,  // Always scope to tenant
        Name:        req.Name,
        Description: req.Description,
        Permissions: req.Permissions,
        IsActive:    true,
    }

    if err := db.Create(role).Error; err != nil {
        return nil, err
    }

    return role, nil
}
```

### 5.2 User's Role in Tenant

```go
// Get user's role in specific tenant
func GetUserRoleInTenant(userID, tenantID uuid.UUID) (*domain.RoleEnhanced, error) {
    var userTenant domain.UserTenant

    if err := db.Where("user_id = ? AND tenant_id = ?", userID, tenantID).
        First(&userTenant).Error; err != nil {
        return nil, fmt.Errorf("user not in tenant")
    }

    // Get the actual role
    var role domain.RoleEnhanced
    if err := db.Where("id = ?", userTenant.RoleID).First(&role).Error; err != nil {
        return nil, err
    }

    return &role, nil
}

// Change user's role in tenant
func SetUserRoleInTenant(userID, tenantID, newRoleID uuid.UUID) error {
    // Verify new role is in this tenant
    var role domain.RoleEnhanced
    if err := db.Where("id = ? AND tenant_id = ?", newRoleID, tenantID).
        First(&role).Error; err != nil {
        return fmt.Errorf("role not found in this tenant")
    }

    // Update mapping
    return db.Model(&domain.UserTenant{}).
        Where("user_id = ? AND tenant_id = ?", userID, tenantID).
        Update("role_id", newRoleID).Error
}
```

---

## 6. API Endpoints

### 6.1 Tenant Management

```
GET    /api/v1/tenants              - List user's tenants
POST   /api/v1/tenants              - Create new tenant
GET    /api/v1/tenants/:tenant_id   - Get tenant details
PATCH  /api/v1/tenants/:tenant_id   - Update tenant
DELETE /api/v1/tenants/:tenant_id   - Delete tenant
```

### 6.2 Tenant Resources

```
GET    /api/v1/tenants/:tenant_id/risks           - List risks
POST   /api/v1/tenants/:tenant_id/risks           - Create risk
GET    /api/v1/tenants/:tenant_id/risks/:id       - Get risk
PATCH  /api/v1/tenants/:tenant_id/risks/:id       - Update risk
DELETE /api/v1/tenants/:tenant_id/risks/:id       - Delete risk
```

### 6.3 Tenant Users

```
GET    /api/v1/tenants/:tenant_id/users           - List users
POST   /api/v1/tenants/:tenant_id/users           - Add user
PATCH  /api/v1/tenants/:tenant_id/users/:user_id  - Update role
DELETE /api/v1/tenants/:tenant_id/users/:user_id  - Remove user
```

### 6.4 Tenant Roles

```
GET    /api/v1/tenants/:tenant_id/roles           - List roles
POST   /api/v1/tenants/:tenant_id/roles           - Create role
PATCH  /api/v1/tenants/:tenant_id/roles/:role_id  - Update role
DELETE /api/v1/tenants/:tenant_id/roles/:role_id  - Delete role
```

---

## 7. Best Practices

### 7.1 Query Safety

**Always include tenant filtering**:
```go
// ✅ SAFE - Tenant scoped
db.Where("tenant_id = ?", tenantID).Find(&records)

// ✅ SAFE - Multi-field check
db.Where("id = ? AND tenant_id = ?", recordID, tenantID).First(&record)

// ❌ DANGEROUS - No tenant check
db.Where("id = ?", recordID).First(&record)

// ❌ DANGEROUS - Assumes trusted input
db.Where("tenant_id IN (?)", userTenantsIDArray).Find(&records)  // What if array is wrong?
```

### 7.2 Tenant Isolation Checklist

- [ ] All queries include `WHERE tenant_id = ?`
- [ ] All create operations set `tenant_id` from context
- [ ] All update/delete operations verify `tenant_id` match
- [ ] Middleware validates tenant access before handler
- [ ] Roles are scoped to tenants
- [ ] API doesn't expose cross-tenant data
- [ ] Audit logs track tenant access
- [ ] Tests verify isolation

### 7.3 Migration Pattern

```sql
-- Create tenants table
CREATE TABLE tenants (
    id UUID PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(100) UNIQUE NOT NULL,
    owner_id UUID NOT NULL REFERENCES users(id),
    status VARCHAR(50),
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    deleted_at TIMESTAMP
);

-- Create user_tenants junction table
CREATE TABLE user_tenants (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    role_id UUID NOT NULL REFERENCES roles(id),
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    PRIMARY KEY (user_id, tenant_id)
);

-- Add tenant_id to risks table
ALTER TABLE risks ADD COLUMN tenant_id UUID NOT NULL REFERENCES tenants(id);
CREATE INDEX idx_risks_tenant_id ON risks(tenant_id);

-- Ensure tenant_id in all relevant tables
-- mitigations, assets, integrations, etc.
```

---

## 8. Troubleshooting

### Problem: User sees data from another tenant

**Solution**:
1. Check all queries include `WHERE tenant_id = ?`
2. Verify middleware applies `TenantIsolation`
3. Check route doesn't skip tenant validation
4. Review recent code changes for missing tenant filters

**Debug Query**:
```sql
-- Check what this user can actually see
SELECT r.* FROM risks r
JOIN user_tenants ut ON ut.tenant_id = r.tenant_id
WHERE ut.user_id = 'user-uuid'
ORDER BY r.created_at;
```

### Problem: User can't access own tenant

**Solution**:
1. Verify `user_tenants` record exists
2. Check role exists in tenant
3. Verify tenant `is_active = true`
4. Check for soft-delete conflicts

**Debug Query**:
```sql
-- Check user's tenant access
SELECT ut.* FROM user_tenants ut
WHERE ut.user_id = 'user-uuid'
AND ut.tenant_id = 'tenant-uuid';

-- Check role exists
SELECT * FROM roles_enhanced
WHERE id = (SELECT role_id FROM user_tenants WHERE user_id = 'user-uuid' AND tenant_id = 'tenant-uuid')
AND tenant_id = 'tenant-uuid';
```

### Problem: Permission check fails for correct role

**Solution**:
1. Verify role permissions are set
2. Check role scope matches query
3. Verify permission format is correct
4. Check for role inheritance issues

---

## Summary

Multi-tenancy in OpenRisk:
- ✅ Shared database with logical isolation
- ✅ Tenant-scoped roles and permissions
- ✅ Automatic data filtering by tenant
- ✅ User-tenant many-to-many relationships
- ✅ Per-tenant role customization
- ✅ Complete audit trail

**Golden Rule**: Every query that returns data must include `WHERE tenant_id = ?` verification.
