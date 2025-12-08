# SAML/OAuth2 Enterprise SSO Integration Guide

## Overview

This guide covers integrating OpenRisk with enterprise authentication providers using:

- **OAuth2**: For modern SaaS applications (Google, GitHub, Microsoft, etc.)
- **SAML2**: For enterprise directory services (Active Directory, Okta, etc.)

## Architecture

```
┌──────────────┐
│  User        │
└──────┬───────┘
       │
       ▼
┌──────────────────────────┐
│  OpenRisk Frontend       │
└──────┬───────────────────┘
       │
       ├─→ /api/auth/oauth2/login
       ├─→ /api/auth/saml/login
       │
       ▼
┌──────────────────────────┐         ┌──────────────────┐
│  OpenRisk Backend        │◄────────►  Identity Provider  │
│  (Auth Handler)          │         (OAuth2/SAML Provider)
└──────┬───────────────────┘         └──────────────────┘
       │
       ├─→ Create/Update User
       ├─→ Assign Role
       ├─→ Generate JWT
       │
       ▼
┌──────────────────────────┐
│  Database                │
│  (users, roles, tokens)  │
└──────────────────────────┘
```

## Supported Providers

### OAuth2
- Google
- GitHub
- Microsoft Azure AD
- Auth0
- Okta
- Custom OAuth2 provider

### SAML2
- Okta
- Azure AD
- Active Directory (via ADFS)
- OneLogin
- Custom SAML provider

## Implementation Plan

### Phase 1: OAuth2 (Basic Support)
- [ ] Google OAuth2
- [ ] GitHub OAuth2
- [ ] OAuth2 token validation
- [ ] User provisioning

### Phase 2: SAML2 (Enterprise)
- [ ] SAML2 metadata parsing
- [ ] Assertion validation
- [ ] Attribute mapping
- [ ] Group/Role mapping

### Phase 3: Advanced (Multi-tenant)
- [ ] Per-tenant provider configuration
- [ ] Federated identity management
- [ ] Account linking
- [ ] SAML2 encryption

## Configuration

### Environment Variables

```env
# ============================================================================
# OAuth2 Configuration
# ============================================================================

# Google OAuth2
OAUTH2_GOOGLE_ENABLED=true
OAUTH2_GOOGLE_CLIENT_ID=your-client-id.apps.googleusercontent.com
OAUTH2_GOOGLE_CLIENT_SECRET=your-client-secret
OAUTH2_GOOGLE_REDIRECT_URI=https://openrisk.yourdomain.com/api/auth/oauth2/google/callback

# GitHub OAuth2
OAUTH2_GITHUB_ENABLED=true
OAUTH2_GITHUB_CLIENT_ID=your-github-client-id
OAUTH2_GITHUB_CLIENT_SECRET=your-github-secret
OAUTH2_GITHUB_REDIRECT_URI=https://openrisk.yourdomain.com/api/auth/oauth2/github/callback

# Microsoft Azure AD
OAUTH2_AZURE_ENABLED=true
OAUTH2_AZURE_TENANT_ID=your-tenant-id
OAUTH2_AZURE_CLIENT_ID=your-client-id
OAUTH2_AZURE_CLIENT_SECRET=your-client-secret
OAUTH2_AZURE_REDIRECT_URI=https://openrisk.yourdomain.com/api/auth/oauth2/azure/callback

# Custom OAuth2 Provider
OAUTH2_CUSTOM_ENABLED=false
OAUTH2_CUSTOM_AUTHORIZE_URL=https://provider.com/oauth/authorize
OAUTH2_CUSTOM_TOKEN_URL=https://provider.com/oauth/token
OAUTH2_CUSTOM_USERINFO_URL=https://provider.com/oauth/userinfo
OAUTH2_CUSTOM_CLIENT_ID=your-client-id
OAUTH2_CUSTOM_CLIENT_SECRET=your-client-secret

# ============================================================================
# SAML2 Configuration
# ============================================================================

# Okta SAML2
SAML2_ENABLED=true
SAML2_PROVIDER=okta
SAML2_IDP_URL=https://your-org.okta.com
SAML2_IDP_CERT_PATH=/etc/openrisk/saml/idp-cert.pem
SAML2_SP_ENTITY_ID=openrisk-saml-sp
SAML2_ACS_URL=https://openrisk.yourdomain.com/api/auth/saml/acs

# SAML2 Attribute Mapping
SAML2_ATTR_EMAIL=email
SAML2_ATTR_NAME=displayName
SAML2_ATTR_GROUPS=memberOf

# ============================================================================
# User Provisioning
# ============================================================================

# Default role for new users (viewer, analyst, admin)
SSO_DEFAULT_ROLE=viewer

# Auto-create users from SSO provider
SSO_AUTO_PROVISION=true

# Auto-update user info from provider
SSO_AUTO_UPDATE_PROFILE=true

# Map provider groups to OpenRisk roles
SSO_GROUP_ROLE_MAPPING={
  "admin-group": "admin",
  "analyst-group": "analyst",
  "viewer-group": "viewer"
}
```

## Implementation Examples

### OAuth2 Handler

```go
// backend/internal/handlers/oauth2_handler.go
package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/opendefender/openrisk/database"
	"github.com/opendefender/openrisk/internal/core/domain"
	"github.com/opendefender/openrisk/internal/services"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// OAuth2Config holds OAuth2 provider configuration
type OAuth2Config struct {
	GoogleConfig *oauth2.Config
	GitHubConfig *oauth2.Config
	AzureConfig  *oauth2.Config
}

// InitializeOAuth2 initializes OAuth2 configurations
func InitializeOAuth2() *OAuth2Config {
	return &OAuth2Config{
		GoogleConfig: &oauth2.Config{
			ClientID:     getEnv("OAUTH2_GOOGLE_CLIENT_ID", ""),
			ClientSecret: getEnv("OAUTH2_GOOGLE_CLIENT_SECRET", ""),
			Scopes: []string{
				"https://www.googleapis.com/auth/userinfo.email",
				"https://www.googleapis.com/auth/userinfo.profile",
			},
			Endpoint: google.Endpoint,
		},
	}
}

// OAuth2Callback handles the OAuth2 callback from provider
func OAuth2Callback(c *fiber.Ctx) error {
	provider := c.Params("provider") // google, github, azure
	code := c.Query("code")
	state := c.Query("state")

	if code == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Authorization code not provided",
		})
	}

	// Verify state (CSRF protection)
	if !verifyOAuth2State(state) {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Invalid state parameter",
		})
	}

	// Exchange code for token
	token, err := exchangeOAuth2Token(provider, code)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to exchange token: %v", err),
		})
	}

	// Get user info from provider
	userInfo, err := getOAuth2UserInfo(provider, token)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to get user info: %v", err),
		})
	}

	// Find or create user
	user, err := provisionUser(userInfo, provider)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to provision user: %v", err),
		})
	}

	// Generate JWT
	authService := services.NewAuthService(getEnv("JWT_SECRET", ""), 24*time.Hour)
	jwtToken, err := authService.GenerateToken(user)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to generate token",
		})
	}

	// Log authentication
	auditService := services.NewAuditService(database.DB)
	auditService.LogLogin(user.ID, domain.ResultSuccess, c.IP(), c.Get("User-Agent"), "")

	// Redirect to frontend with token
	redirectURL := fmt.Sprintf(
		"https://openrisk.yourdomain.com/?token=%s&provider=%s",
		jwtToken,
		provider,
	)

	return c.Redirect(redirectURL)
}

// OAuth2UserInfo represents user info from OAuth2 provider
type OAuth2UserInfo struct {
	ID    string
	Email string
	Name  string
	Picture string
	Groups []string
}

// provisionUser finds or creates a user from OAuth2 info
func provisionUser(userInfo *OAuth2UserInfo, provider string) (*domain.User, error) {
	user := &domain.User{}

	// Find existing user by email
	result := database.DB.Preload("Role").Where("email = ?", userInfo.Email).First(user)

	if result.Error == gorm.ErrRecordNotFound {
		// Create new user
		if !getEnv("SSO_AUTO_PROVISION", "true") == "true" {
			return nil, fmt.Errorf("user auto-provisioning disabled")
		}

		roleID := uuid.MustParse(getDefaultRoleID())
		user = &domain.User{
			ID:       uuid.New(),
			Email:    userInfo.Email,
			Username: userInfo.Email,
			FullName: userInfo.Name,
			RoleID:   roleID,
			Active:   true,
			Provider: provider,
		}

		if err := database.DB.Create(user).Error; err != nil {
			return nil, fmt.Errorf("failed to create user: %w", err)
		}

		// Load role
		database.DB.Model(user).Association("Role").Find(&user.Role)

		return user, nil
	}

	// Update existing user if enabled
	if getEnv("SSO_AUTO_UPDATE_PROFILE", "true") == "true" {
		user.FullName = userInfo.Name
		user.Provider = provider
		database.DB.Save(user)
	}

	return user, nil
}

func exchangeOAuth2Token(provider, code string) (*oauth2.Token, error) {
	// Implementation depends on provider
	// For Google:
	cfg := InitializeOAuth2()
	ctx := context.Background()
	return cfg.GoogleConfig.Exchange(ctx, code)
}

func getOAuth2UserInfo(provider string, token *oauth2.Token) (*OAuth2UserInfo, error) {
	// Call provider's userinfo endpoint
	// Example for Google:
	resp, err := http.Get("https://www.googleapis.com/oauth2/v2/userinfo?access_token=" + token.AccessToken)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var userInfo OAuth2UserInfo
	json.NewDecoder(resp.Body).Decode(&userInfo)

	return &userInfo, nil
}

func verifyOAuth2State(state string) bool {
	// Verify state matches what was stored in session
	// This is CSRF protection
	return true // Implement proper state validation
}
```

### SAML2 Handler

```go
// backend/internal/handlers/saml2_handler.go
package handlers

import (
	"github.com/crewjam/saml/samlsp"
	"github.com/gofiber/fiber/v2"
)

// SAML2InitiateLogin initiates SAML2 login
func SAML2InitiateLogin(c *fiber.Ctx) error {
	// Initialize SAML2 service provider
	sp := initSAML2SP()

	// Generate authentication request
	authRequest := sp.AuthnRequest()
	redirectURL := authRequest.AuthnRequestURL()

	return c.Redirect(redirectURL)
}

// SAML2ACS handles the SAML2 Assertion Consumer Service callback
func SAML2ACS(c *fiber.Ctx) error {
	sp := initSAML2SP()

	// Parse SAML assertion
	assertion, err := sp.ParseResponse(c.Body(), nil)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": fmt.Sprintf("SAML assertion validation failed: %v", err),
		})
	}

	// Extract user attributes
	email := assertion.NameID.Value
	attributes := assertion.AttributeStatements[0].Attributes

	userInfo := &OAuth2UserInfo{
		Email: email,
		Name:  getAttribute(attributes, "displayName"),
	}

	// Get groups from assertion
	for _, attr := range attributes {
		if attr.Name == "memberOf" {
			for _, value := range attr.Values {
				userInfo.Groups = append(userInfo.Groups, value.Value)
			}
		}
	}

	// Provision user with group-based role mapping
	user, err := provisionUserWithGroups(userInfo, userInfo.Groups)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to provision user",
		})
	}

	// Generate JWT
	authService := services.NewAuthService(getEnv("JWT_SECRET", ""), 24*time.Hour)
	jwtToken, err := authService.GenerateToken(user)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to generate token",
		})
	}

	return c.JSON(fiber.Map{
		"token": jwtToken,
		"user": fiber.Map{
			"id":   user.ID,
			"email": user.Email,
			"role": user.Role.Name,
		},
	})
}

func initSAML2SP() *samlsp.ServiceProvider {
	// Load IdP certificate
	idpCert, _ := ioutil.ReadFile(getEnv("SAML2_IDP_CERT_PATH", ""))

	// Create service provider
	sp := &samlsp.ServiceProvider{
		EntityID: getEnv("SAML2_SP_ENTITY_ID", "openrisk-saml-sp"),
		ACSURL: url.URL{
			Scheme: "https",
			Host:   getEnv("SAML2_ACS_URL", "openrisk.yourdomain.com"),
			Path:   "/api/auth/saml/acs",
		},
		IDPMetadataURL: getEnv("SAML2_IDP_URL", ""),
	}

	return sp
}

func getAttribute(attributes []saml.Attribute, name string) string {
	for _, attr := range attributes {
		if attr.Name == name && len(attr.Values) > 0 {
			return attr.Values[0].Value
		}
	}
	return ""
}

func provisionUserWithGroups(userInfo *OAuth2UserInfo, groups []string) (*domain.User, error) {
	user, err := provisionUser(userInfo, "saml2")
	if err != nil {
		return nil, err
	}

	// Map groups to role
	roleID := mapGroupsToRole(groups)
	user.RoleID = roleID
	database.DB.Save(user)

	return user, nil
}

func mapGroupsToRole(groups []string) uuid.UUID {
	// Example mapping: check if user is in admin group
	for _, group := range groups {
		if group == "openrisk-admin" {
			// Return admin role ID
			return uuid.MustParse("00000000-0000-0000-0000-000000000001")
		}
		if group == "openrisk-analyst" {
			// Return analyst role ID
			return uuid.MustParse("00000000-0000-0000-0000-000000000002")
		}
	}

	// Default to viewer role
	return uuid.MustParse("00000000-0000-0000-0000-000000000003")
}
```

## Frontend Integration

### Login Page with SSO Options

```tsx
// frontend/src/pages/LoginWithSSO.tsx
import React, { useState } from 'react';
import { useNavigate } from 'react-router-dom';

export function LoginWithSSO() {
  const navigate = useNavigate();
  const [isLoading, setIsLoading] = useState(false);

  const handleOAuth2Login = (provider: string) => {
    setIsLoading(true);
    
    // Generate state for CSRF protection
    const state = crypto.randomUUID();
    sessionStorage.setItem('oauth2_state', state);

    // Redirect to backend OAuth2 login endpoint
    window.location.href = 
      `${import.meta.env.VITE_API_URL}/auth/oauth2/${provider}/login?state=${state}`;
  };

  const handleSAML2Login = () => {
    setIsLoading(true);
    
    // Redirect to backend SAML2 login endpoint
    window.location.href = 
      `${import.meta.env.VITE_API_URL}/auth/saml/login`;
  };

  return (
    <div className="min-h-screen bg-zinc-950 flex items-center justify-center">
      <div className="w-full max-w-md p-8 bg-zinc-900 rounded-lg border border-zinc-800">
        <h1 className="text-2xl font-bold text-white mb-8">Sign In to OpenRisk</h1>

        {/* Email/Password Login */}
        <div className="mb-6">
          <button className="w-full bg-blue-600 hover:bg-blue-700 text-white py-2 rounded">
            Sign in with Email
          </button>
        </div>

        <div className="relative mb-6">
          <div className="absolute inset-0 flex items-center">
            <div className="w-full border-t border-zinc-700"></div>
          </div>
          <div className="relative flex justify-center text-sm">
            <span className="px-2 bg-zinc-900 text-zinc-400">Or continue with</span>
          </div>
        </div>

        {/* OAuth2 Providers */}
        <div className="space-y-3 mb-6">
          <button
            onClick={() => handleOAuth2Login('google')}
            disabled={isLoading}
            className="w-full flex items-center justify-center gap-2 bg-white text-black py-2 rounded hover:bg-gray-100 disabled:opacity-50"
          >
            <img src="https://www.google.com/favicon.ico" alt="Google" className="w-4 h-4" />
            Google
          </button>

          <button
            onClick={() => handleOAuth2Login('github')}
            disabled={isLoading}
            className="w-full flex items-center justify-center gap-2 bg-zinc-800 text-white py-2 rounded hover:bg-zinc-700 disabled:opacity-50"
          >
            <span>GitHub</span>
          </button>

          <button
            onClick={() => handleOAuth2Login('azure')}
            disabled={isLoading}
            className="w-full flex items-center justify-center gap-2 bg-blue-500 text-white py-2 rounded hover:bg-blue-600 disabled:opacity-50"
          >
            <span>Microsoft</span>
          </button>
        </div>

        {/* Enterprise SAML2 */}
        <div className="relative mb-6">
          <div className="absolute inset-0 flex items-center">
            <div className="w-full border-t border-zinc-700"></div>
          </div>
          <div className="relative flex justify-center text-sm">
            <span className="px-2 bg-zinc-900 text-zinc-400">Enterprise</span>
          </div>
        </div>

        <button
          onClick={handleSAML2Login}
          disabled={isLoading}
          className="w-full bg-purple-600 hover:bg-purple-700 text-white py-2 rounded disabled:opacity-50"
        >
          Enterprise SAML Login
        </button>

        <p className="text-center text-zinc-400 text-sm mt-4">
          Don't have an account? Contact your administrator.
        </p>
      </div>
    </div>
  );
}
```

## Router Configuration

```go
// backend/cmd/server/main.go (relevant OAuth2/SAML2 routes)

// OAuth2 routes
oauth2Handler := handlers.NewOAuth2Handler(authService, permissionService)
api.Get("/auth/oauth2/:provider/login", oauth2Handler.InitiateLogin)
api.Get("/auth/oauth2/:provider/callback", oauth2Handler.Callback)

// SAML2 routes
saml2Handler := handlers.NewSAML2Handler(authService, permissionService)
api.Get("/auth/saml/login", saml2Handler.InitiateLogin)
api.Post("/auth/saml/acs", saml2Handler.ACS)
api.Get("/auth/saml/metadata", saml2Handler.Metadata)
```

## Testing OAuth2/SAML2

### Mock OAuth2 Provider

```go
// backend/internal/handlers/oauth2_mock.go
package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// MockOAuth2Server simulates an OAuth2 provider for testing
type MockOAuth2Server struct {
	server *http.Server
}

func NewMockOAuth2Server() *MockOAuth2Server {
	mux := http.NewServeMux()

	mux.HandleFunc("/oauth/authorize", func(w http.ResponseWriter, r *http.Request) {
		clientID := r.URL.Query().Get("client_id")
		redirectURI := r.URL.Query().Get("redirect_uri")
		state := r.URL.Query().Get("state")

		// Redirect back with authorization code
		http.Redirect(w, r, fmt.Sprintf(
			"%s?code=mock_code_123&state=%s",
			redirectURI,
			state,
		), http.StatusFound)
	})

	mux.HandleFunc("/oauth/token", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token": "mock_access_token",
			"token_type":   "Bearer",
			"expires_in":   3600,
		})
	})

	mux.HandleFunc("/oauth/userinfo", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":       "mock_user_123",
			"email":    "test@example.com",
			"name":     "Test User",
			"picture":  "https://example.com/avatar.jpg",
		})
	})

	server := &http.Server{
		Addr:    ":8888",
		Handler: mux,
	}

	return &MockOAuth2Server{server: server}
}

func (m *MockOAuth2Server) Start() error {
	return m.server.ListenAndServe()
}

func (m *MockOAuth2Server) Stop() error {
	return m.server.Close()
}
```

### Test Cases

```go
// backend/internal/handlers/oauth2_handler_test.go
package handlers

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOAuth2GoogleCallback_Success(t *testing.T) {
	// Setup mock OAuth2 server
	mockServer := NewMockOAuth2Server()
	go mockServer.Start()
	defer mockServer.Stop()

	// Test callback with valid code
	// ... test implementation
}

func TestOAuth2Callback_InvalidState(t *testing.T) {
	// Test CSRF protection
	// ... test implementation
}

func TestSAML2ACS_ValidAssertion(t *testing.T) {
	// Test SAML2 assertion processing
	// ... test implementation
}

func TestGroupToRoleMapping(t *testing.T) {
	groups := []string{"openrisk-admin", "developers"}
	roleID := mapGroupsToRole(groups)
	assert.NotNil(t, roleID)
}
```

## Deployment Checklist

- [ ] OAuth2 credentials obtained from providers
- [ ] SAML2 IdP metadata downloaded
- [ ] Certificates configured and validated
- [ ] Callback URLs configured in providers
- [ ] Environment variables set in deployment
- [ ] Frontend updated with SSO login UI
- [ ] User provisioning tested
- [ ] Group/role mapping configured
- [ ] SSL certificates valid
- [ ] Session timeout configured
- [ ] Audit logging enabled
- [ ] Documentation updated

## Security Considerations

1. **State Parameter**: Always verify state to prevent CSRF attacks
2. **Token Storage**: Store JWTs securely (httpOnly cookies preferred)
3. **Encrypted Transport**: Always use HTTPS/TLS
4. **Certificate Validation**: Verify SAML2 IdP certificates
5. **Assertion Signature**: Always validate SAML2 assertion signatures
6. **Audience Restriction**: Verify assertion intended for your service
7. **Not On/Not Before**: Check SAML2 time constraints
8. **Session Management**: Implement proper session timeout and revocation

## Troubleshooting

| Error | Cause | Solution |
|-------|-------|----------|
| Invalid state | CSRF token mismatch | Verify state before processing |
| Invalid assertion | IdP certificate wrong | Update IdP cert in configuration |
| User not found | Auto-provisioning disabled | Enable SSO_AUTO_PROVISION |
| Invalid signature | Assertion tampered | Verify certificate chain |
| Token expired | JWT TTL too short | Increase tokenTTL |

---

**Next Steps**:
- Integrate with Okta: `docs/OKTA_INTEGRATION.md`
- Integrate with Azure AD: `docs/AZURE_AD_INTEGRATION.md`
- Multi-tenant SAML: `docs/MULTI_TENANT_SAML.md`
