# API Security Guide

**Version**: 1.0  
**Last Updated**: March 10, 2026  
**Audience**: API Developers, Security Teams, DevOps  

---

## Table of Contents

1. [Authentication](#authentication)
2. [Authorization](#authorization)
3. [Request Security](#request-security)
4. [Response Security](#response-security)
5. [Rate Limiting](#rate-limiting)
6. [Best Practices](#best-practices)
7. [Troubleshooting](#troubleshooting)

---

## Authentication

### Overview

OpenRisk API supports three authentication mechanisms:

1. **JWT (JSON Web Tokens)** - Session-based, for web applications
2. **Bearer Tokens** - API tokens, for service-to-service communication
3. **OAuth2/SAML2** - Enterprise SSO

### JWT Authentication

#### Token Generation

Tokens are issued via `POST /auth/login`:

```bash
curl -X POST https://api.openrisk.io/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "user@example.com",
    "password": "secure_password"
  }'
```

**Response**:
```json
{
  "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "token_type": "Bearer",
  "expires_in": 259200,
  "refresh_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
}
```

#### Token Structure

Tokens contain the following claims:

```go
type UserClaims struct {
    ID          string        // User UUID
    Email       string        // User email
    RoleName    string        // "admin", "analyst", "viewer"
    Permissions []Permission  // Fine-grained permissions
    ExpiresAt   int64         // Unix timestamp
    IssuedAt    int64         // Unix timestamp
}
```

#### Token Usage

Include the token in the `Authorization` header with "Bearer" scheme:

```bash
curl -H "Authorization: Bearer <token>" \
  https://api.openrisk.io/api/v1/risks
```

#### Token Expiration

- **Validity**: 72 hours
- **Refresh**: Use `POST /auth/refresh` with refresh_token before expiration
- **Expired**: Returns `401 Unauthorized`

```bash
curl -X POST https://api.openrisk.io/api/v1/auth/refresh \
  -H "Content-Type: application/json" \
  -d '{"refresh_token": "<refresh_token>"}'
```

#### Token Security

- ✅ Tokens signed with HMAC-SHA256
- ✅ Secret stored in `JWT_SECRET` environment variable
- ✅ Never expose secrets in code or logs
- ✅ Always use HTTPS in production
- ✅ Store tokens securely on client (e.g., HTTPOnly cookies)

### Bearer Tokens

#### API Token Generation

Users can generate API tokens for programmatic access:

```bash
curl -X POST https://api.openrisk.io/api/v1/tokens \
  -H "Authorization: Bearer <jwt_token>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "CI/CD Pipeline",
    "description": "Token for automated deployments",
    "expires_at": "2026-12-31T23:59:59Z"
  }'
```

**Response**:
```json
{
  "id": "token_uuid",
  "token": "opnrsk_abcd1234efgh5678ijkl9012",
  "name": "CI/CD Pipeline",
  "created_at": "2026-03-10T10:00:00Z",
  "expires_at": "2026-12-31T23:59:59Z"
}
```

#### API Token Usage

Use the token as a Bearer token:

```bash
curl -H "Authorization: Bearer opnrsk_abcd1234efgh5678ijkl9012" \
  https://api.openrisk.io/api/v1/risks
```

#### API Token Management

- Create: `POST /tokens`
- List: `GET /tokens`
- Get: `GET /tokens/:id`
- Update: `PUT /tokens/:id`
- Revoke: `POST /tokens/:id/revoke`
- Rotate: `POST /tokens/:id/rotate`
- Delete: `DELETE /tokens/:id`

#### API Token Security

- ✅ Scoped to user account
- ✅ Can be revoked immediately
- ✅ Should be rotated regularly
- ✅ Never commit to version control
- ✅ Use `.gitignore` for token files
- ✅ Rotate if compromised: `POST /tokens/:id/rotate`

### OAuth2 & SAML2

#### OAuth2 Flow

1. Redirect to: `GET /api/v1/auth/oauth2/login/:provider`
   - Supported providers: google, github, microsoft, okta
2. User authenticates with provider
3. Callback to: `GET /api/v1/auth/oauth2/callback/:provider`
4. OpenRisk creates/links user account
5. JWT token issued

#### SAML2 Flow

1. Initiate: `GET /api/v1/auth/saml2/login`
2. Redirects to SAML IdP
3. User authenticates
4. SAML assertion posted to: `POST /api/v1/auth/saml2/acs`
5. OpenRisk creates/links user account
6. JWT token issued

#### Enterprise SSO Setup

See: [SAML2 Configuration Guide](./SAML2_SETUP.md)

---

## Authorization

### Role-Based Access Control (RBAC)

#### Default Roles

| Role | Permissions | Use Case |
|------|-------------|----------|
| **admin** | Full access | System administrators |
| **analyst** | Create/update risks, manage mitigations | Security analysts |
| **viewer** | Read-only access | Stakeholders, executives |

#### Permission Matrix

| Resource | Create | Read | Update | Delete |
|----------|--------|------|--------|--------|
| Risk | analyst+ | viewer+ | analyst+ | admin |
| Mitigation | analyst+ | viewer+ | analyst+ | admin |
| Asset | analyst+ | viewer+ | analyst+ | admin |
| User | admin | admin | admin | admin |
| Report | analyst+ | analyst+ | analyst+ | admin |

#### Endpoint Protection

Endpoints are protected using middleware:

**Role-based**:
```go
protected.Post("/risks", 
    middleware.RequireRole("admin", "analyst"),
    handlers.CreateRisk)
```

**Permission-based**:
```go
protected.Post("/risks", 
    middleware.RequirePermissions(permissionService, 
        domain.Permission{
            Resource: domain.PermissionResourceRisk,
            Action:   domain.PermissionCreate,
        }),
    handlers.CreateRisk)
```

#### Custom Permissions

Organizations can define custom permissions:

```bash
curl -X POST https://api.openrisk.io/api/v1/rbac/roles \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Risk Owner",
    "description": "Can manage own risks",
    "permissions": [
      {"resource": "risk", "action": "read"},
      {"resource": "risk", "action": "update"},
      {"resource": "mitigation", "action": "read"}
    ]
  }'
```

#### Multi-Tenancy

Each user belongs to one or more tenants. Authorization checks include tenant context:

```bash
# User can only see risks in their tenant
GET /api/v1/risks
# Returns risks for user's default tenant

# User can switch tenants
GET /api/v1/risks?tenant_id=<other_tenant_id>
# Returns 403 if user not in that tenant
```

---

## Request Security

### Input Validation

All API inputs are validated before processing:

#### Required Fields

```bash
# Missing required field
POST /api/v1/risks \
  -d '{"description": "No title"}'

# Response
{
  "error": "validation_failed",
  "details": "Title is required"
}
```

#### Range Validation

```bash
# Impact out of range (must be 1-5)
POST /api/v1/risks \
  -d '{"title": "Test", "impact": 10, "probability": 3}'

# Response
{
  "error": "validation_failed",
  "details": "Impact must be between 1 and 5"
}
```

#### UUID Validation

```bash
# Invalid asset ID
POST /api/v1/risks \
  -d '{
    "title": "Test",
    "asset_ids": ["not-a-uuid"]
  }'

# Response
{
  "error": "validation_failed",
  "details": "Invalid UUID format"
}
```

#### String Array Validation

```bash
# Valid
POST /api/v1/risks \
  -d '{
    "title": "Test",
    "tags": ["production", "database"]
  }'

# Invalid (null values)
POST /api/v1/risks \
  -d '{
    "title": "Test",
    "tags": [null, "database"]
  }'

# Response
{
  "error": "validation_failed",
  "details": "Tags must be non-empty strings"
}
```

### SQL Injection Prevention

OpenRisk uses **parameterized queries** via GORM ORM:

```go
// ✅ SAFE - Parameter binding
var risks []Risk
db.Where("status = ?", "open").Find(&risks)

// ❌ UNSAFE - String concatenation (never use)
// db.Where("status = '" + userInput + "'").Find(&risks)
```

### Cross-Site Scripting (XSS) Prevention

- ✅ `Content-Security-Policy` header restricts script sources
- ✅ API responses are JSON (not HTML)
- ✅ User input is never rendered as HTML
- ✅ Frontend sanitization recommended

### Cross-Site Request Forgery (CSRF) Prevention

- ✅ CORS policy prevents cross-origin requests
- ✅ `Sec-Fetch-*` headers checked by browser
- ✅ State-changing operations use POST/PATCH/DELETE
- ✅ No cookies with SameSite policy in use

### Content Type Validation

Only `application/json` accepted:

```bash
# Invalid
curl -X POST https://api.openrisk.io/api/v1/risks \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d 'title=Test&impact=3'

# Response
400 Bad Request - Invalid input format
```

---

## Response Security

### Security Headers

All API responses include security headers:

| Header | Value | Purpose |
|--------|-------|---------|
| `X-Frame-Options` | DENY | Prevent clickjacking |
| `X-Content-Type-Options` | nosniff | Prevent MIME sniffing |
| `X-XSS-Protection` | 1; mode=block | Enable XSS filter |
| `Strict-Transport-Security` | max-age=31536000 | Force HTTPS |
| `Content-Security-Policy` | [various] | Restrict content sources |
| `Referrer-Policy` | strict-origin-when-cross-origin | Control referrers |

### Sensitive Data Masking

User passwords and tokens are never returned in API responses:

```json
{
  "id": "user_uuid",
  "email": "user@example.com",
  "role": "analyst",
  "created_at": "2026-03-10T10:00:00Z"
  // password is never included
  // api_token is never included (except on creation)
}
```

### Error Message Disclosure

Error messages reveal minimal information:

```json
// ❌ Bad - Exposes implementation details
{
  "error": "SQL error: column 'user_name' does not exist"
}

// ✅ Good - Generic error message
{
  "error": "Invalid request"
}
```

---

## Rate Limiting

### Default Limits

| Endpoint Type | Limit | Window |
|---------------|-------|--------|
| Auth endpoints | 5 | per minute |
| Read endpoints | 100 | per minute |
| Write endpoints | 20 | per minute |
| Export endpoints | 5 | per hour |

### Rate Limit Headers

```http
HTTP/1.1 200 OK
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 95
X-RateLimit-Reset: 1678512345
```

### Exceeding Rate Limits

```http
HTTP/1.1 429 Too Many Requests
{
  "error": "Rate limit exceeded",
  "retry_after": 60
}
```

### Per-User Limits

Rate limits are per-user:

```bash
# User A has 100 requests
GET /api/v1/risks -H "Authorization: Bearer token_a"
# 95 remaining

# User B has separate 100 requests
GET /api/v1/risks -H "Authorization: Bearer token_b"
# 100 remaining
```

### Custom Rate Limits

Admins can set custom limits per user or API token:

```bash
curl -X PUT https://api.openrisk.io/api/v1/tokens/token_id \
  -H "Authorization: Bearer <admin_token>" \
  -H "Content-Type: application/json" \
  -d '{
    "rate_limit": "1000/hour"
  }'
```

---

## Best Practices

### 1. Authentication

- ✅ Always use HTTPS (never HTTP)
- ✅ Store tokens securely (HTTPOnly cookies or secure storage)
- ✅ Rotate API tokens regularly (monthly recommended)
- ✅ Revoke compromised tokens immediately
- ✅ Use short-lived tokens (72 hours for JWT)
- ✅ Never hardcode secrets in code
- ✅ Use environment variables for secrets
- ✅ Enable multi-factor authentication when available

### 2. API Usage

- ✅ Use specific endpoints (don't over-fetch)
- ✅ Implement pagination for large datasets (page, limit)
- ✅ Use filtering to reduce data returned
- ✅ Cache responses client-side when appropriate
- ✅ Implement exponential backoff for retries
- ✅ Log API calls for audit trails
- ✅ Monitor API usage for anomalies

### 3. Error Handling

- ✅ Don't expose internal details in error messages
- ✅ Log errors server-side for debugging
- ✅ Return generic error messages to clients
- ✅ Use appropriate HTTP status codes
- ✅ Provide error codes for machine parsing
- ✅ Include `correlation_id` for support tickets

### 4. Data Protection

- ✅ Never log sensitive data (passwords, tokens, credit cards)
- ✅ Encrypt data in transit (HTTPS/TLS)
- ✅ Encrypt data at rest (database encryption)
- ✅ Sanitize user input
- ✅ Use parameterized queries
- ✅ Implement data retention policies
- ✅ Support data deletion requests

### 5. Monitoring

- ✅ Monitor failed authentication attempts
- ✅ Alert on unusual API patterns
- ✅ Track rate limit violations
- ✅ Monitor error rates
- ✅ Log access to sensitive endpoints
- ✅ Review audit logs regularly

### 6. Documentation

- ✅ Document API version in requests
- ✅ Keep API documentation current
- ✅ Document authentication requirements
- ✅ Document all error codes
- ✅ Provide example implementations
- ✅ Include security considerations

---

## Troubleshooting

### 401 Unauthorized

**Causes**:
- Missing Authorization header
- Invalid token format
- Expired token
- Wrong JWT secret

**Solution**:
```bash
# Check header format
curl -v https://api.openrisk.io/api/v1/risks \
  -H "Authorization: Bearer <token>"

# Should show: Authorization: Bearer eyJhb...

# Refresh token if expired
curl -X POST https://api.openrisk.io/api/v1/auth/refresh \
  -d '{"refresh_token": "<refresh_token>"}'
```

### 403 Forbidden

**Causes**:
- Insufficient permissions
- Token valid but doesn't have required role
- Accessing other user's data

**Solution**:
```bash
# Check token role
curl https://api.openrisk.io/api/v1/users/me \
  -H "Authorization: Bearer <token>"

# Check required permissions for endpoint
# See API_REFERENCE.md for endpoint details

# Request role upgrade from admin
```

### 429 Too Many Requests

**Causes**:
- Exceeded rate limit
- Too many API calls

**Solution**:
```bash
# Check rate limit headers
curl -i https://api.openrisk.io/api/v1/risks \
  -H "Authorization: Bearer <token>"

# Shows: X-RateLimit-Remaining: 0

# Wait before retrying
# Use Retry-After header: X-RateLimit-Reset

# For high-volume use, request increased limits
```

### 400 Bad Request

**Causes**:
- Missing required fields
- Invalid field format
- Invalid JSON

**Solution**:
```bash
# Validate JSON
echo '{"title": "Test"}' | jq .

# Check required fields
POST /api/v1/risks requires: title, impact, probability

# Check field types and ranges
impact: integer 1-5
probability: integer 1-5
```

### 500 Internal Server Error

**Causes**:
- Server error
- Database connectivity issue
- Service misconfiguration

**Solution**:
```bash
# Check service health
curl https://api.openrisk.io/api/v1/health

# Response indicates service status

# Contact support with correlation ID
# Find in response headers or logs

# Check server logs
docker logs openrisk-api
```

---

## Security Checklist

- [ ] All API calls use HTTPS
- [ ] Tokens stored securely (HTTPOnly, encrypted)
- [ ] Tokens rotated regularly (monthly)
- [ ] Secrets not in code repository
- [ ] Environment variables used for secrets
- [ ] Request input validated
- [ ] Error messages don't expose internals
- [ ] Rate limiting monitored
- [ ] Failed auth attempts logged
- [ ] API usage anomalies detected
- [ ] Data encryption in transit and at rest
- [ ] Audit logs reviewed regularly
- [ ] Multi-factor authentication enabled
- [ ] Permissions tested and verified

---

## Additional Resources

- [OpenAPI Specification](./openapi.yaml)
- [API Reference](./API_REFERENCE.md)
- [RBAC Guide](./ADVANCED_PERMISSIONS.md)
- [Error Codes Reference](./ERROR_CODES.md)

---

**Questions?** Contact: security@openrisk.io  
**Security Issues?** Report to: security@openrisk.io (GPG key available)
