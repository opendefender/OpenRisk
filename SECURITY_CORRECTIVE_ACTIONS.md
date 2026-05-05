# CORRECTIVE ACTIONS — IMMEDIATE SECURITY FIXES

**Priorité:** URGENT (24-48 heures)  
**Responsable:** Backend Security Team  
**Suivi:** Daily standup + Security PR review

---

## FIX #1: Remove Hardcoded Admin Password

### Fichier: `backend/internal/handler/user_handler.go`

**AVANT (Risque CWE-798):**
```go
func SeedAdminUser() {
  adminPassword := os.Getenv("INITIAL_ADMIN_PASSWORD")
  if adminPassword == "" {
    adminPassword = "admin123"  // ← HARDCODED DEFAULT!
    log.Println("WARNING: INITIAL_ADMIN_PASSWORD not set, using default password 'admin123'")
  }
  // ...
}
```

**APRÈS (Fixed):**
```go
func SeedAdminUser() {
  adminPassword := os.Getenv("INITIAL_ADMIN_PASSWORD")
  if adminPassword == "" {
    log.Fatalf("FATAL: INITIAL_ADMIN_PASSWORD env var is required. Set it before running seeding.")
    return
  }
  if len(adminPassword) < 16 {
    log.Fatalf("FATAL: INITIAL_ADMIN_PASSWORD must be at least 16 characters (use 32+ strong password)")
    return
  }
  // ...
}
```

**Testing:**
```bash
# Should FAIL
INITIAL_ADMIN_PASSWORD="" go run cmd/server/main.go

# Should SUCCEED
INITIAL_ADMIN_PASSWORD="MyS3cur3P@ssw0rdLonger32Chars!" go run cmd/server/main.go
```

---

## FIX #2: Remove fmt.Sprintf from Error Messages (incident_handler.go)

### Fichier: `backend/internal/handler/incident_handler.go`

**Problème:** Secrets can be included in error details

**AVANT (Risque CWE-532):**
```go
func (h *IncidentHandler) CreateIncident(c *fiber.Ctx) error {
  incident, err := h.incidentService.CreateIncident(tenantID, req)
  if err != nil {
    return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
      "error": fmt.Sprintf("Failed to create incident: %v", err),  // ← err may contain secrets
    })
  }
  // ...
}
```

**APRÈS (Fixed):**
```go
func (h *IncidentHandler) CreateIncident(c *fiber.Ctx) error {
  incident, err := h.incidentService.CreateIncident(tenantID, req)
  if err != nil {
    // Log error internally (with potential secrets) but DON'T expose to client
    log.WithError(err).WithField("tenant_id", tenantID).Error("Failed to create incident")
    
    // Return generic error to client
    return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
      "error": "failed_to_create_incident",
      "error_code": "INCIDENT_CREATION_FAILED",
    })
  }
  // ...
}
```

**Apply to ALL handlers:** CreateIncident, GetIncident, ListIncidents, UpdateIncident, DeleteIncident, GetIncidentTimeline, LinkRisk

---

## FIX #3: Add @RequireAuth Middleware to Unprotected Handlers

### Fichier: `backend/internal/middleware/auth.go`

**STEP 1: Verify middleware exists**
```bash
grep -n "RequireAuth" backend/internal/middleware/auth.go
```

**STEP 2: Create if missing**
```go
// RequireAuth ensures user is authenticated
func RequireAuth(c *fiber.Ctx) error {
  claims := GetUserClaims(c)
  if claims == nil {
    return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
      "error": "unauthorized",
      "error_code": "AUTH_REQUIRED",
    })
  }
  return c.Next()
}
```

**STEP 3: Apply to handlers (in router setup)**

### Fichier: `backend/cmd/server/main.go` (or wherever routes are registered)

**BEFORE:**
```go
app.Post("/organizations", handler.CreateOrganization)
app.Get("/score-engine/configs", scoreHandler.GetScoringConfigs)
app.Post("/marketplace/apps", marketplaceHandler.InstallApp)
```

**AFTER:**
```go
app.Post("/organizations", handler.CreateOrganization).Use(middleware.RequireAuth)
app.Get("/score-engine/configs", scoreHandler.GetScoringConfigs).Use(middleware.RequireAuth)
app.Post("/marketplace/apps", marketplaceHandler.InstallApp).Use(middleware.RequireAuth)
```

**Handlers requiring @RequireAuth (Priority order):**
1. score_engine_handler: GetScoringConfigs, GetScoringConfig, CreateScoringConfig, UpdateScoringConfig, ComputeRiskScore
2. organization_handler: CreateOrganization, GetOrganization, UpdateOrganization, UpgradeSubscription, AddMember, RemoveMember
3. marketplace_handler: AddConnectorReview, InstallApp, GetApp, UpdateApp, EnableApp, DisableApp, UninstallApp
4. custom_field_handler: CreateCustomField, GetCustomField, ListCustomFields
5. trend_handler: AnalyzeTrend, GenerateForecast, GetRecommendations
6. risk_timeline_handler: GetRiskTimeline, GetStatusChanges, GetScoreChanges
7. rbac_tenant_handler: GetTenant, CreateTenant

---

## FIX #4: Fix API Key Exposure in integration_handler.go

### Fichier: `backend/internal/handler/integration_handler.go`

**Problem:** API keys sent in request body + logged in errors + retried with secrets exposed

**BEFORE:**
```go
type TestIntegrationInput struct {
  APIUrl string `json:"api_url" validate:"required,url"`
  APIKey string `json:"api_key" validate:"required"`  // ← In body!
}

func (h *...) TestIntegration(c *fiber.Ctx) error {
  input := new(TestIntegrationInput)
  if err := c.BodyParser(input); err != nil {
    return c.Status(http.StatusBadRequest).JSON(fiber.Map{
      "error": "Invalid input",  // ← But error may have logged APIKey
    })
  }
  // ...
  req.Header.Set("Authorization", "Bearer "+input.APIKey)  // ← Sent in error logs
}
```

**AFTER:**
```go
type TestIntegrationInput struct {
  APIUrl string `json:"api_url" validate:"required,url"`
  // Removed APIKey from body
}

func (h *...) TestIntegration(c *fiber.Ctx) error {
  input := new(TestIntegrationInput)
  if err := c.BodyParser(input); err != nil {
    return c.Status(http.StatusBadRequest).JSON(fiber.Map{
      "error": "invalid_request_body",
    })
  }
  
  // Get API key from Authorization header instead
  authHeader := c.Get("Authorization")
  if authHeader == "" {
    return c.Status(http.StatusUnauthorized).JSON(fiber.Map{
      "error": "missing_authorization_header",
    })
  }
  
  // Extract bearer token (never log this)
  apiKey := strings.TrimPrefix(authHeader, "Bearer ")
  if apiKey == authHeader {
    return c.Status(http.StatusUnauthorized).JSON(fiber.Map{
      "error": "invalid_authorization_format",
    })
  }
  
  // Test with NO retry logic (or max 1 attempt)
  client := &http.Client{Timeout: 10 * time.Second}
  req, _ := http.NewRequest("GET", input.APIUrl, nil)
  req.Header.Set("Authorization", "Bearer "+apiKey)
  
  resp, err := client.Do(req)
  if err != nil {
    // Log error WITHOUT including apiKey
    log.WithField("url", input.APIUrl).Error("Integration test failed")
    return c.Status(http.StatusBadRequest).JSON(fiber.Map{
      "error": "integration_test_failed",
    })
  }
  // ...
}
```

---

## FIX #5: Add Tenant Isolation Middleware

### Fichier: `backend/internal/middleware/tenant.go`

**BEFORE:** Some handlers check GetContext(), others don't

**AFTER:** Create explicit middleware
```go
// RequireTenant ensures tenant_id is present in context
func RequireTenant(c *fiber.Ctx) error {
  ctx := GetContext(c)
  if ctx == nil || ctx.OrganizationID == uuid.Nil {
    return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
      "error": "tenant_context_missing",
      "error_code": "TENANT_REQUIRED",
    })
  }
  return c.Next()
}
```

**Apply to ALL data endpoints:**
```go
// In router setup
dataRoutes := app.Group("/api/v1/data").Use(middleware.RequireAuth, middleware.RequireTenant)
dataRoutes.Get("/risks", riskHandler.GetRisks)
dataRoutes.Get("/assets", assetHandler.GetAssets)
dataRoutes.Get("/incidents", incidentHandler.ListIncidents)
dataRoutes.Get("/analytics/risks/metrics", analyticsHandler.GetRiskMetrics)
```

---

## FIX #6: Add @RequireAdmin Role Check

### Fichier: `backend/internal/middleware/permissions.go`

**If not exists, create:**
```go
// RequireAdmin ensures user has admin role
func RequireAdmin(c *fiber.Ctx) error {
  claims := GetUserClaims(c)
  if claims == nil {
    return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
      "error": "unauthorized",
    })
  }
  
  if claims.RoleName != "admin" && !claims.HasPermission("*") {
    return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
      "error": "admin_role_required",
    })
  }
  
  return c.Next()
}
```

**Apply to admin routes:**
```go
adminRoutes := app.Group("/api/v1/admin").Use(middleware.RequireAuth, middleware.RequireAdmin)
adminRoutes.Get("/users", userHandler.GetUsers)
adminRoutes.Post("/users", userHandler.CreateUser)
adminRoutes.Get("/audit-logs", auditLogHandler.GetAuditLogs)
```

---

## FIX #7: Validate SAML2 Signatures

### Fichier: `backend/internal/handler/saml2_handler.go`

**BEFORE:**
```go
func SAML2ACS(c *fiber.Ctx) error {
  samlResponse := c.FormValue("SAMLResponse")
  decoded, err := base64.StdEncoding.DecodeString(samlResponse)
  
  var response SAMLResponse
  if err := xml.Unmarshal(decoded, &response); err != nil {  // ← XXE vulnerable!
    return c.Status(400).JSON(...)
  }
  
  // NO signature validation!
  // NO timestamp validation!
}
```

**AFTER:**
```go
import "github.com/russellhaering/goxmldsig"

func SAML2ACS(c *fiber.Ctx) error {
  samlResponse := c.FormValue("SAMLResponse")
  if samlResponse == "" {
    return c.Status(400).JSON(fiber.Map{"error": "missing_saml_response"})
  }
  
  decoded, err := base64.StdEncoding.DecodeString(samlResponse)
  if err != nil {
    return c.Status(400).JSON(fiber.Map{"error": "invalid_base64"})
  }
  
  // Validate XML signature
  idpCertPEM := os.Getenv("SAML2_IDP_CERT")  // Load IDP public cert
  if idpCertPEM == "" {
    log.Fatal("SAML2_IDP_CERT must be set")
  }
  
  cert, err := tls.LoadX509KeyPair("", "")  // Load cert properly
  if err != nil {
    return c.Status(500).JSON(fiber.Map{"error": "saml_config_error"})
  }
  
  // Verify signature
  ctx := dsig.NewDefaultValidationContext(cert)
  root, err := etree.ParseFromReader(bytes.NewReader(decoded))
  if err != nil {
    return c.Status(400).JSON(fiber.Map{"error": "invalid_xml"})
  }
  
  _, err = ctx.Validate(root)
  if err != nil {
    log.Error("SAML signature validation failed:", err)
    return c.Status(400).JSON(fiber.Map{"error": "invalid_saml_signature"})
  }
  
  // Validate timestamp
  var response SAMLResponse
  if err := xml.Unmarshal(decoded, &response); err != nil {
    return c.Status(400).JSON(fiber.Map{"error": "invalid_xml_format"})
  }
  
  // Check NotBefore/NotOnOrAfter
  now := time.Now().UTC()
  notBefore, _ := time.Parse(time.RFC3339, response.Assertion.Conditions.NotBefore)
  notAfter, _ := time.Parse(time.RFC3339, response.Assertion.Conditions.NotOnOrAfter)
  
  if now.Before(notBefore) || now.After(notAfter) {
    return c.Status(400).JSON(fiber.Map{"error": "saml_response_expired"})
  }
  
  // ... Rest of flow
}
```

---

## FIX #8: Implement Audit Logging for Mutations

### Fichier: `backend/internal/middleware/audit.go`

**BEFORE:** Only some handlers log

**AFTER:** Middleware logs ALL mutations
```go
func AuditLog(c *fiber.Ctx) error {
  method := c.Method()
  path := c.Path()
  
  // Log write operations
  if method != "GET" && method != "HEAD" && method != "OPTIONS" {
    claims := GetUserClaims(c)
    ctx := GetContext(c)
    
    log.WithFields(log.Fields{
      "method":         method,
      "path":           path,
      "user_id":        claims.ID,
      "tenant_id":      ctx.OrganizationID,
      "ip_address":     c.IP(),
      "user_agent":     c.Get("User-Agent"),
      "timestamp":      time.Now().UTC(),
    }).Info("API mutation")
    
    // TODO: Store in DB audit_logs table
  }
  
  return c.Next()
}
```

---

## VALIDATION CHECKLIST

Before pushing to production, verify:

- [ ] `SeedAdminUser()` fails if INITIAL_ADMIN_PASSWORD not set
- [ ] No `fmt.Sprintf()` in error messages for ANY handler
- [ ] All 15 unprotected handlers have `@RequireAuth`
- [ ] All data handlers have `@RequireTenant`
- [ ] API keys NOT in request body (use Authorization header)
- [ ] SAML2ACS validates XML signatures
- [ ] Audit logs recorded for all write operations
- [ ] Rate limiting middleware active on all endpoints
- [ ] CSRF tokens checked on POST/PUT/DELETE
- [ ] Security headers set on all responses

---

## ROLLOUT PLAN

### Day 1 (Emergency)
1. Apply Fix #1: Remove hardcoded password
2. Apply Fix #2: Remove fmt.Sprintf errors
3. Deploy to staging + test

### Day 2 (Critical)
1. Apply Fix #3: Add @RequireAuth
2. Apply Fix #4: Move API keys to headers
3. Apply Fix #5: Add @RequireTenant
4. Deploy to staging + integration tests

### Day 3 (High)
1. Apply Fix #6: Add @RequireAdmin
2. Apply Fix #7: SAML2 signatures
3. Apply Fix #8: Audit logging
4. Full security regression testing

### Day 4 (Deploy to Production)
1. Monitor logs for new errors
2. Alert on failed auth/tenant checks
3. 24/7 on-call team ready

---

## MONITORING & ALERTS

```
Alert if:
- 401 Unauthorized rate > 10/min per user → account compromise?
- 403 Forbidden rate > 5/min per tenant → permission escalation?
- Failed SAML signature validation > 1/hour → attack?
- Missing audit logs for write operations → query count changes?
```

---

## SIGN-OFF

- [ ] Security Lead: ___________________ Date: ___
- [ ] Backend Lead: ___________________ Date: ___
- [ ] DevOps Lead: ___________________ Date: ___
