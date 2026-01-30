 Advanced Permission Enforcement Patterns

 Overview

This guide covers advanced permission enforcement patterns for implementing fine-grained, context-aware access control in OpenRisk.

 Architecture



   HTTP Request  

         
         

   Authentication Middleware             
   - JWT validation                      
   - Extract user claims                 

         
         

   Permission Enforcement Layer          
   - Check basic permissions             
   - Check resource ownership            
   - Check context (org, team, project)  

         
         

   Handler Business Logic                
   - Create/Read/Update/Delete resources 
   - Apply row-level security            
   - Log access for audit trail          

         
         

  HTTP Response  



 Permission Levels

 Level : Basic Role-Based Access Control (RBAC)

go
// Example: Admin can do everything
if user.Role.Name == "admin" {
    return true // Allow any action
}

// Example: Viewer can only read
if user.Role.Name == "viewer" && action == "read" {
    return true // Allow read
}


Pros: Simple, fast, built-in
Cons: Coarse-grained, not scalable

 Level : Permission-Based Access Control (PBAC)

go
// Check specific permission
if user.HasPermission("risk:read:any") {
    return true // User can read any risk
}

if user.HasPermission("risk:update:own") {
    // User can only update risks they own
    return risk.OwnerID == user.ID
}


Pros: Fine-grained, flexible
Cons: More complex, requires permission matrix

 Level : Attribute-Based Access Control (ABAC)

go
// Advanced: Multiple conditions
attributes := map[string]interface{}{
    "user_role":         user.Role.Name,
    "resource_owner":    risk.OwnerID,
    "resource_status":   risk.Status,
    "user_department":   user.Department,
    "resource_severity": risk.Impact,
    "time":              time.Now().Hour(),
}

if evaluatePolicy(attributes, "risk:update") {
    return true
}


Pros: Highly flexible, context-aware, powerful
Cons: Complex, slower evaluation

 Implementation Patterns

 Pattern : Middleware-based Enforcement

go
// backend/internal/middleware/permission_enforcer.go
package middleware

import (
	"github.com/gofiber/fiber/v"
	"github.com/opendefender/openrisk/internal/core/domain"
	"github.com/opendefender/openrisk/internal/services"
)

// PermissionEnforcer enforces fine-grained permissions
type PermissionEnforcer struct {
	permissionService services.PermissionService
}

// CheckResourcePermission validates access to a specific resource
func (p PermissionEnforcer) CheckResourcePermission(
	requiredPerm string,
	resourceID string,
) fiber.Handler {
	return func(c fiber.Ctx) error {
		user := c.Locals("user").(domain.UserClaims)
		
		// Get resource from database
		resource, err := p.getResource(resourceID)
		if err != nil {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Resource not found",
			})
		}

		// Check permission with resource context
		if !p.hasResourcePermission(user, requiredPerm, resource) {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "Insufficient permissions",
			})
		}

		// Store resource in context for handler
		c.Locals("resource", resource)

		return c.Next()
	}
}

// hasResourcePermission checks if user has permission for specific resource
func (p PermissionEnforcer) hasResourcePermission(
	user domain.UserClaims,
	requiredPerm string,
	resource interface{},
) bool {
	// Parse required permission
	perm, _ := domain.ParsePermission(requiredPerm)

	// Check basic permission first
	if !user.HasPermission(requiredPerm) {
		return false
	}

	// Check scope-based access
	switch resource.(type) {
	case domain.Risk:
		risk := resource.(domain.Risk)
		return p.checkRiskAccess(user, perm, risk)
	
	case domain.Mitigation:
		mitigation := resource.(domain.Mitigation)
		return p.checkMitigationAccess(user, perm, mitigation)
	
	case domain.User:
		targetUser := resource.(domain.User)
		return p.checkUserAccess(user, perm, targetUser)
	
	default:
		return false
	}
}

// checkRiskAccess verifies access to a specific risk
func (p PermissionEnforcer) checkRiskAccess(
	user domain.UserClaims,
	perm domain.Permission,
	risk domain.Risk,
) bool {
	// Admin can access anything
	if user.RoleName == "admin" {
		return true
	}

	// Check scope
	switch perm.Scope {
	case domain.PermissionScopeOwn:
		// User can only access risks they own
		return risk.OwnerID.String() == user.ID.String()
	
	case domain.PermissionScopeTeam:
		// User can access risks in their team
		return p.isUserInTeam(user.ID, risk.TeamID)
	
	case domain.PermissionScopeAny:
		// User can access any risk (unlikely for non-admin)
		return true
	
	default:
		return false
	}
}

func (p PermissionEnforcer) checkMitigationAccess(
	user domain.UserClaims,
	perm domain.Permission,
	mitigation domain.Mitigation,
) bool {
	// Similar logic for mitigations
	return true
}

func (p PermissionEnforcer) checkUserAccess(
	user domain.UserClaims,
	perm domain.Permission,
	targetUser domain.User,
) bool {
	// Only admins can modify other users
	if perm.Action == domain.PermissionDelete || perm.Action == domain.PermissionUpdate {
		return user.RoleName == "admin"
	}

	// Users can read their own profile
	if perm.Action == domain.PermissionRead {
		return user.ID.String() == targetUser.ID.String() || user.RoleName == "admin"
	}

	return false
}


 Pattern : Policy-Based Enforcement

go
// backend/internal/services/policy_service.go
package services

import (
	"github.com/open-policy-agent/opa/rego"
	"github.com/opendefender/openrisk/internal/core/domain"
)

// PolicyEngine evaluates fine-grained access policies
type PolicyEngine struct {
	policies map[string]rego.PreparedEvalQuery
}

// NewPolicyEngine creates a new policy engine
func NewPolicyEngine() PolicyEngine {
	return &PolicyEngine{
		policies: make(map[string]rego.PreparedEvalQuery),
	}
}

// RegisterPolicy registers a Rego policy
func (p PolicyEngine) RegisterPolicy(name string, policy string) error {
	query, err := rego.New(
		rego.Query("data.openrisk.allow"),
		rego.Module("openrisk.rego", policy),
	).PrepareForEval(context.Background())

	if err != nil {
		return err
	}

	p.policies[name] = query
	return nil
}

// EvaluatePolicy evaluates a policy with context
func (p PolicyEngine) EvaluatePolicy(
	policyName string,
	input map[string]interface{},
) (bool, error) {
	policy, ok := p.policies[policyName]
	if !ok {
		return false, fmt.Errorf("policy not found: %s", policyName)
	}

	results, err := policy.Eval(context.Background(), rego.EvalInput(input))
	if err != nil {
		return false, err
	}

	if len(results) ==  || len(results[].Expressions) ==  {
		return false, nil
	}

	return results[].Expressions[].Value.(bool), nil
}


 Example Rego Policy

rego
 policies/risk_access.rego
package openrisk

default allow = false

 Admin can do anything
allow {
    input.user.role == "admin"
}

 Owner can update their own risk
allow {
    input.action == "update"
    input.resource.type == "risk"
    input.resource.owner_id == input.user.id
}

 Analyst can read any risk
allow {
    input.action == "read"
    input.resource.type == "risk"
    input.user.role == "analyst"
}

 Viewer can read critical risks only
allow {
    input.action == "read"
    input.resource.type == "risk"
    input.user.role == "viewer"
    input.resource.impact >= 
}

 Enforce audit trail for sensitive operations
audit_required {
    input.action == "delete"
}

audit_required {
    input.action == "export"
}


 Pattern : Declarative Permission Routing

go
// backend/internal/handlers/permission_routes.go
package handlers

import "github.com/gofiber/fiber/v"

// PermissionRoute defines routing with permission requirements
type PermissionRoute struct {
	Method     string
	Path       string
	Handler    fiber.Handler
	Permission string
}

// RegisterPermissionRoutes registers routes with permission enforcement
func RegisterPermissionRoutes(app fiber.App, enforcer PermissionEnforcer) {
	routes := []PermissionRoute{
		// Risk endpoints
		{
			Method:     "POST",
			Path:       "/api/v/risks",
			Handler:    CreateRisk,
			Permission: "risk:create:any",
		},
		{
			Method:     "GET",
			Path:       "/api/v/risks/:id",
			Handler:    GetRisk,
			Permission: "risk:read:any",
		},
		{
			Method:     "PATCH",
			Path:       "/api/v/risks/:id",
			Handler:    UpdateRisk,
			Permission: "risk:update:any",
		},
		{
			Method:     "DELETE",
			Path:       "/api/v/risks/:id",
			Handler:    DeleteRisk,
			Permission: "risk:delete:any",
		},

		// Mitigation endpoints
		{
			Method:     "POST",
			Path:       "/api/v/risks/:riskId/mitigations",
			Handler:    CreateMitigation,
			Permission: "mitigation:create:any",
		},
		{
			Method:     "GET",
			Path:       "/api/v/risks/:riskId/mitigations/:mitigationId",
			Handler:    GetMitigation,
			Permission: "mitigation:read:any",
		},

		// User management endpoints (admin only)
		{
			Method:     "GET",
			Path:       "/api/v/users",
			Handler:    ListUsers,
			Permission: "user:read:any",
		},
		{
			Method:     "PATCH",
			Path:       "/api/v/users/:id/role",
			Handler:    UpdateUserRole,
			Permission: "user:update:any",
		},
		{
			Method:     "DELETE",
			Path:       "/api/v/users/:id",
			Handler:    DeleteUser,
			Permission: "user:delete:any",
		},
	}

	// Register routes with middleware
	for _, route := range routes {
		group := app.Group("")
		group.Use(enforcer.CheckPermission(route.Permission))
		group.Add(route.Method, route.Path, route.Handler)
	}
}


 Pattern : Dynamic Permission Checking

go
// backend/internal/handlers/risk_handler.go
package handlers

import (
	"github.com/gofiber/fiber/v"
	"github.com/opendefender/openrisk/internal/core/domain"
	"github.com/opendefender/openrisk/internal/services"
)

// UpdateRisk with dynamic permission checking
func UpdateRisk(permService services.PermissionService) fiber.Handler {
	return func(c fiber.Ctx) error {
		user := c.Locals("user").(domain.UserClaims)
		riskID := c.Params("id")

		// Get the risk
		risk := &domain.Risk{}
		if err := database.DB.First(risk, "id = ?", riskID).Error; err != nil {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Risk not found",
			})
		}

		// Dynamic permission check based on risk status
		var requiredPerm string
		
		if risk.Status == "closed" {
			// Cannot update closed risks
			requiredPerm = "risk:update:closed" // This doesn't exist, so will be denied
		} else if risk.OwnerID.String() == user.ID.String() {
			// Owner needs "update:own"
			requiredPerm = "risk:update:own"
		} else {
			// Non-owner needs "update:any"
			requiredPerm = "risk:update:any"
		}

		// Check permission
		if !user.HasPermission(requiredPerm) {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "Insufficient permissions for this operation",
			})
		}

		// Parse request
		var req UpdateRiskRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid request",
			})
		}

		// Update risk
		risk.Title = req.Title
		risk.Description = req.Description
		risk.Impact = req.Impact
		risk.Probability = req.Probability

		if err := database.DB.Save(risk).Error; err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to update risk",
			})
		}

		// Log the action
		auditService := services.NewAuditService(database.DB)
		auditService.LogRiskUpdate(user.ID, risk.ID, "update")

		return c.JSON(risk)
	}
}


 Advanced Patterns

 Pattern : Temporal Permissions

go
// Check if permission is valid at this time
func (p PermissionService) CheckTemporalPermission(
	user domain.User,
	permission string,
) bool {
	// Business hours only
	hour := time.Now().Hour()
	if hour <  || hour >  {
		return false
	}

	// Weekend restrictions
	day := time.Now().Weekday()
	if day == time.Saturday || day == time.Sunday {
		return false
	}

	return user.HasPermission(permission)
}


 Pattern : Geolocation-based Permissions

go
// Restrict access based on IP/location
func (p PermissionService) CheckLocationPermission(
	user domain.User,
	permission string,
	ipAddress string,
) bool {
	// Check if IP is in allowed locations
	location := geoipClient.GetLocation(ipAddress)
	
	if !user.AllowedCountries[location.Country] {
		return false
	}

	return user.HasPermission(permission)
}


 Pattern : Delegation & Impersonation

go
// Allow delegation of permissions
func (p PermissionService) DelegatePermission(
	from domain.User,
	to domain.User,
	permission string,
	duration time.Duration,
) error {
	delegation := &domain.PermissionDelegation{
		FromUserID:  from.ID,
		ToUserID:    to.ID,
		Permission:  permission,
		ExpiresAt:   time.Now().Add(duration),
	}

	return database.DB.Create(delegation).Error
}

// Check delegated permissions
func (p PermissionService) HasDelegatedPermission(
	user domain.User,
	permission string,
) bool {
	delegation := &domain.PermissionDelegation{}
	result := database.DB.Where(
		"to_user_id = ? AND permission = ? AND expires_at > ?",
		user.ID,
		permission,
		time.Now(),
	).First(delegation)

	return result.Error == nil
}


 Pattern : Resource-Level Row Security (RLS)

go
// Apply row-level security automatically
func (p PermissionService) ApplyRLS(
	user domain.User,
	query gorm.DB,
	resource string,
) gorm.DB {
	switch resource {
	case "risks":
		// User can only see their own risks unless admin
		if user.Role.Name != "admin" {
			query = query.Where("owner_id = ?", user.ID)
		}
		return query

	case "users":
		// Users can only see themselves unless admin
		if user.Role.Name != "admin" {
			query = query.Where("id = ?", user.ID)
		}
		return query

	case "audit_logs":
		// Users can only see logs related to them unless admin
		if user.Role.Name != "admin" {
			query = query.Where("user_id = ? OR resource_id IN (SELECT id FROM risks WHERE owner_id = ?)", user.ID, user.ID)
		}
		return query

	default:
		return query
	}
}


 Testing Permission Enforcement

go
// backend/internal/services/permission_enforcer_test.go
package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/opendefender/openrisk/internal/core/domain"
)

func TestPermissionEnforcement_AdminCanAccessAnything(t testing.T) {
	admin := &domain.UserClaims{
		RoleName: "admin",
	}

	enforcer := NewPermissionEnforcer()

	risk := &domain.Risk{}
	allowed := enforcer.CheckResourcePermission(admin, "risk:delete:any", risk)

	assert.True(t, allowed)
}

func TestPermissionEnforcement_ViewerCannotDelete(t testing.T) {
	viewer := &domain.UserClaims{
		RoleName: "viewer",
		Permissions: []string{"risk:read:any"},
	}

	enforcer := NewPermissionEnforcer()

	risk := &domain.Risk{}
	allowed := enforcer.CheckResourcePermission(viewer, "risk:delete:any", risk)

	assert.False(t, allowed)
}

func TestPermissionEnforcement_OwnerCanUpdateOwn(t testing.T) {
	owner := &domain.UserClaims{
		ID:       uuid.MustParse("e-eb-d-a-"),
		RoleName: "analyst",
		Permissions: []string{"risk:update:own"},
	}

	enforcer := NewPermissionEnforcer()

	risk := &domain.Risk{
		OwnerID: owner.ID,
	}
	
	allowed := enforcer.CheckResourcePermission(owner, "risk:update:own", risk)

	assert.True(t, allowed)
}

func TestPermissionEnforcement_NonOwnerCannotUpdateOwn(t testing.T) {
	user := &domain.UserClaims{
		ID:       uuid.MustParse("e-eb-d-a-"),
		RoleName: "analyst",
		Permissions: []string{"risk:update:own"},
	}

	enforcer := NewPermissionEnforcer()

	risk := &domain.Risk{
		OwnerID: uuid.MustParse("e-eb-d-a-"),
	}
	
	allowed := enforcer.CheckResourcePermission(user, "risk:update:own", risk)

	assert.False(t, allowed)
}


 Performance Optimization

 Caching Permissions

go
// Cache permission checks to avoid database queries
func (p PermissionService) CheckPermissionCached(
	userID uuid.UUID,
	permission string,
	ttl time.Duration,
) (bool, error) {
	cacheKey := fmt.Sprintf("perm:%s:%s", userID, permission)

	// Try cache first
	cached, _ := p.cache.Get(cacheKey)
	if cached != nil {
		return cached.(bool), nil
	}

	// Fall back to database
	result := p.CheckPermission(userID, permission)

	// Cache result
	p.cache.Set(cacheKey, result, ttl)

	return result, nil
}


 Batch Permission Checking

go
// Check multiple permissions efficiently
func (p PermissionService) CheckPermissionsAny(
	user domain.User,
	permissions ...string,
) bool {
	for _, perm := range permissions {
		if user.HasPermission(perm) {
			return true
		}
	}
	return false
}

func (p PermissionService) CheckPermissionsAll(
	user domain.User,
	permissions ...string,
) bool {
	for _, perm := range permissions {
		if !user.HasPermission(perm) {
			return false
		}
	}
	return true
}


 Deployment Checklist

- [ ] Permission domain model defined
- [ ] Permission service implemented
- [ ] Permission middleware configured
- [ ] Resource-level checks added to handlers
- [ ] Audit logging for permission denials
- [ ] Test coverage >%
- [ ] Performance baseline established
- [ ] Documentation updated
- [ ] Team trained on permission system

---

Next Steps:
- Implement advanced policy engine with OPA
- Add temporal and geolocation-based permissions
- Create permission delegation framework
- Integrate with SAML group mapping
