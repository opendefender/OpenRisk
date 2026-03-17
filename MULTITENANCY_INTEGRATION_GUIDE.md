# Integration Guide: Adding Multi-Tenant Routes to main.go

This guide shows how to add the new multi-tenant authentication and organization routes to your existing `cmd/server/main.go`.

## Step 1: Import the new services

Add these imports after the existing service imports:

```go
import (
    // ... existing imports ...
    "github.com/opendefender/openrisk/internal/services"
    "github.com/opendefender/openrisk/internal/handlers"
)
```

## Step 2: Initialize services (around where other services are created)

In the main() function, after initializing the database and other services, add:

```go
// =========================================================================
// MULTI-TENANT AUTHENTICATION & ORGANIZATION SERVICES
// =========================================================================

multitenantAuthService := services.NewMultitenantAuthService(
    database.DB,
    os.Getenv("JWT_SECRET"),
    15 * time.Minute, // Access token TTL
)

multitenantOrgService := services.NewMultitenantOrgService(database.DB)

// Initialize handlers
multitenantAuthHandler := handlers.NewMultitenantAuthHandler(multitenantAuthService)
multitenantOrgHandler := handlers.NewMultitenantOrgHandler(multitenantOrgService)
```

## Step 3: Add routes (around where other routes are configured)

In the section where you configure routes (typically in the `api := app.Group("/api/v1")` section), add:

### Public Authentication Routes (no JWT required)

```go
// PUBLIC AUTH ROUTES
api.Post("/auth/login", multitenantAuthHandler.Login)
api.Post("/auth/refresh", multitenantAuthHandler.RefreshToken)
api.Get("/health", func(c *fiber.Ctx) error {
    return c.JSON(fiber.Map{
        "status":  "UP",
        "version": "1.0.0",
    })
})
```

### Protected Routes (JWT required)

```go
// PROTECTED ROUTES - Require JWT
protected := api.Use(middleware.Protected())

// User profile endpoints
protected.Get("/me", multitenantAuthHandler.GetProfile)
protected.Get("/me/organizations", multitenantAuthHandler.GetMyOrganizations)

// Organization management
protected.Post("/organizations", multitenantOrgHandler.CreateOrganization)
protected.Get("/organizations/:id", multitenantOrgHandler.GetOrganization)
protected.Patch("/organizations/:id", multitenantOrgHandler.UpdateOrganization)
protected.Delete("/organizations/:id", multitenantOrgHandler.DeleteOrganization)

// Multi-org user endpoints
protected.Post("/auth/select-org", multitenantAuthHandler.SelectOrganization)
protected.Post("/auth/logout", multitenantAuthHandler.Logout)

// Organization member management (org-scoped)
protected.Post("/organizations/:id/members/invite", multitenantOrgHandler.InviteMembers)
protected.Post("/organizations/:id/transfer-ownership", multitenantOrgHandler.TransferOwnership)

// Invitation endpoints
protected.Post("/invitations/:token/accept", multitenantOrgHandler.AcceptInvitation)
```

## Step 4: Keep existing routes intact

Your existing routes should continue to work as before. The new multi-tenant system runs alongside the existing auth system.

```go
// KEEP YOUR EXISTING ROUTES
// - Existing risk management routes
// - Existing dashboard routes
// - Existing integration endpoints
// etc.
```

## Step 5: Update existing handlers to use org context (Optional, can be done incrementally)

For any existing handler that manages resources (risks, assets, etc.), you can add org filtering:

```go
// Example: Update risk handler
func (h *RiskHandler) List(c *fiber.Ctx) error {
    // NEW: Get organization context
    ctx := middleware.GetContext(c)
    if ctx == nil {
        // Fall back to old auth if new context not available
        claims, ok := c.Locals("user").(*domain.UserClaims)
        if !ok {
            return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
        }
    }
    
    // OLD: Get all risks
    // risks, err := h.riskService.GetAll(c.Context())
    
    // NEW: Get risks scoped to organization
    if ctx != nil {
        risks, err := h.riskService.GetByOrganization(c.Context(), ctx.OrganizationID)
        // ... handle errors
        return c.JSON(risks)
    }
    
    // Fall back to old behavior if new context not set
    risks, err := h.riskService.GetAll(c.Context())
    // ... rest of handler
}
```

## Step 6: Test the endpoints

```bash
# 1. Create a user (using existing register endpoint)
curl -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "email": "user@example.com",
    "password": "password123",
    "username": "user",
    "full_name": "User Name"
  }'

# 2. Login
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "user@example.com",
    "password": "password123"
  }'

# Returns: { "access_token": "...", "expires_in": 900, "refresh_token": "..." }

# 3. Create an organization (authenticated)
curl -X POST http://localhost:8080/api/v1/organizations \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Acme Corp",
    "slug": "acme",
    "industry": "Technology",
    "size": "51-200",
    "plan": "professional"
  }'

# 4. Invite a member
curl -X POST http://localhost:8080/api/v1/organizations/ORG_ID/members/invite \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "invitees": [
      {
        "email": "member@example.com",
        "role": "user"
      }
    ]
  }'

# 5. Accept an invitation (as the invitee)
curl -X POST http://localhost:8080/api/v1/invitations/TOKEN/accept \
  -H "Authorization: Bearer INVITEE_TOKEN" \
  -H "Content-Type: application/json"
```

## Step 7: Environment variables

Make sure your `.env` file includes:

```bash
JWT_SECRET=your-256-bit-secret-key
JWT_ACCESS_TTL=15m
JWT_REFRESH_TTL=168h
APP_URL=http://localhost:3000
INVITATION_TTL_HOURS=72
```

## Troubleshooting

### Issue: "service not found" error
- Make sure you initialized the services before configuring routes
- Check that database.DB is connected before creating services

### Issue: Routes not found (404)
- Make sure routes are added BEFORE `app.Listen()`
- Check the route prefix matches your API versioning

### Issue: JWT validation errors
- Verify JWT_SECRET matches between login and protected routes
- Check that Authorization header is: `Authorization: Bearer TOKEN`

### Issue: Context is nil in handlers
- The new RequestContext is optional; handlers work with or without it
- Gradually migrate to use new context as handlers are updated

## Migration Path

You don't need to update all endpoints at once. The new system is designed to work alongside the existing one:

1. **Phase 1**: Add new routes (this guide)
2. **Phase 2**: Gradually update existing handlers to use org context
3. **Phase 3**: Fully migrate to new permission system

The old `UserClaims` JWT will continue to work for backward compatibility.