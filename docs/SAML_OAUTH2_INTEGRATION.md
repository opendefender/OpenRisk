 SAML/OAuth Enterprise SSO Integration Guide

 Overview

This guide covers integrating OpenRisk with enterprise authentication providers using:

- OAuth: For modern SaaS applications (Google, GitHub, Microsoft, etc.)
- SAML: For enterprise directory services (Active Directory, Okta, etc.)

 Architecture



  User        

       
       

  OpenRisk Frontend       

       
       → /api/auth/oauth/login
       → /api/auth/saml/login
       
       
         
  OpenRisk Backend          Identity Provider  
  (Auth Handler)                   (OAuth/SAML Provider)
         
       
       → Create/Update User
       → Assign Role
       → Generate JWT
       
       

  Database                
  (users, roles, tokens)  



 Supported Providers

 OAuth
- Google
- GitHub
- Microsoft Azure AD
- Auth
- Okta
- Custom OAuth provider

 SAML
- Okta
- Azure AD
- Active Directory (via ADFS)
- OneLogin
- Custom SAML provider

 Implementation Plan

 Phase : OAuth (Basic Support)
- [ ] Google OAuth
- [ ] GitHub OAuth
- [ ] OAuth token validation
- [ ] User provisioning

 Phase : SAML (Enterprise)
- [ ] SAML metadata parsing
- [ ] Assertion validation
- [ ] Attribute mapping
- [ ] Group/Role mapping

 Phase : Advanced (Multi-tenant)
- [ ] Per-tenant provider configuration
- [ ] Federated identity management
- [ ] Account linking
- [ ] SAML encryption

 Configuration

 Environment Variables

env
 ============================================================================
 OAuth Configuration
 ============================================================================

 Google OAuth
OAUTH_GOOGLE_ENABLED=true
OAUTH_GOOGLE_CLIENT_ID=your-client-id.apps.googleusercontent.com
OAUTH_GOOGLE_CLIENT_SECRET=your-client-secret
OAUTH_GOOGLE_REDIRECT_URI=https://openrisk.yourdomain.com/api/auth/oauth/google/callback

 GitHub OAuth
OAUTH_GITHUB_ENABLED=true
OAUTH_GITHUB_CLIENT_ID=your-github-client-id
OAUTH_GITHUB_CLIENT_SECRET=your-github-secret
OAUTH_GITHUB_REDIRECT_URI=https://openrisk.yourdomain.com/api/auth/oauth/github/callback

 Microsoft Azure AD
OAUTH_AZURE_ENABLED=true
OAUTH_AZURE_TENANT_ID=your-tenant-id
OAUTH_AZURE_CLIENT_ID=your-client-id
OAUTH_AZURE_CLIENT_SECRET=your-client-secret
OAUTH_AZURE_REDIRECT_URI=https://openrisk.yourdomain.com/api/auth/oauth/azure/callback

 Custom OAuth Provider
OAUTH_CUSTOM_ENABLED=false
OAUTH_CUSTOM_AUTHORIZE_URL=https://provider.com/oauth/authorize
OAUTH_CUSTOM_TOKEN_URL=https://provider.com/oauth/token
OAUTH_CUSTOM_USERINFO_URL=https://provider.com/oauth/userinfo
OAUTH_CUSTOM_CLIENT_ID=your-client-id
OAUTH_CUSTOM_CLIENT_SECRET=your-client-secret

 ============================================================================
 SAML Configuration
 ============================================================================

 Okta SAML
SAML_ENABLED=true
SAML_PROVIDER=okta
SAML_IDP_URL=https://your-org.okta.com
SAML_IDP_CERT_PATH=/etc/openrisk/saml/idp-cert.pem
SAML_SP_ENTITY_ID=openrisk-saml-sp
SAML_ACS_URL=https://openrisk.yourdomain.com/api/auth/saml/acs

 SAML Attribute Mapping
SAML_ATTR_EMAIL=email
SAML_ATTR_NAME=displayName
SAML_ATTR_GROUPS=memberOf

 ============================================================================
 User Provisioning
 ============================================================================

 Default role for new users (viewer, analyst, admin)
SSO_DEFAULT_ROLE=viewer

 Auto-create users from SSO provider
SSO_AUTO_PROVISION=true

 Auto-update user info from provider
SSO_AUTO_UPDATE_PROFILE=true

 Map provider groups to OpenRisk roles
SSO_GROUP_ROLE_MAPPING={
  "admin-group": "admin",
  "analyst-group": "analyst",
  "viewer-group": "viewer"
}


 Implementation Examples

 OAuth Handler

go
// backend/internal/handlers/oauth_handler.go
package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gofiber/fiber/v"
	"github.com/golang-jwt/jwt/v"
	"github.com/google/uuid"
	"github.com/opendefender/openrisk/database"
	"github.com/opendefender/openrisk/internal/core/domain"
	"github.com/opendefender/openrisk/internal/services"
	"golang.org/x/oauth"
	"golang.org/x/oauth/google"
)

// OAuthConfig holds OAuth provider configuration
type OAuthConfig struct {
	GoogleConfig oauth.Config
	GitHubConfig oauth.Config
	AzureConfig  oauth.Config
}

// InitializeOAuth initializes OAuth configurations
func InitializeOAuth() OAuthConfig {
	return &OAuthConfig{
		GoogleConfig: &oauth.Config{
			ClientID:     getEnv("OAUTH_GOOGLE_CLIENT_ID", ""),
			ClientSecret: getEnv("OAUTH_GOOGLE_CLIENT_SECRET", ""),
			Scopes: []string{
				"https://www.googleapis.com/auth/userinfo.email",
				"https://www.googleapis.com/auth/userinfo.profile",
			},
			Endpoint: google.Endpoint,
		},
	}
}

// OAuthCallback handles the OAuth callback from provider
func OAuthCallback(c fiber.Ctx) error {
	provider := c.Params("provider") // google, github, azure
	code := c.Query("code")
	state := c.Query("state")

	if code == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Authorization code not provided",
		})
	}

	// Verify state (CSRF protection)
	if !verifyOAuthState(state) {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Invalid state parameter",
		})
	}

	// Exchange code for token
	token, err := exchangeOAuthToken(provider, code)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to exchange token: %v", err),
		})
	}

	// Get user info from provider
	userInfo, err := getOAuthUserInfo(provider, token)
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
	authService := services.NewAuthService(getEnv("JWT_SECRET", ""), time.Hour)
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

// OAuthUserInfo represents user info from OAuth provider
type OAuthUserInfo struct {
	ID    string
	Email string
	Name  string
	Picture string
	Groups []string
}

// provisionUser finds or creates a user from OAuth info
func provisionUser(userInfo OAuthUserInfo, provider string) (domain.User, error) {
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

func exchangeOAuthToken(provider, code string) (oauth.Token, error) {
	// Implementation depends on provider
	// For Google:
	cfg := InitializeOAuth()
	ctx := context.Background()
	return cfg.GoogleConfig.Exchange(ctx, code)
}

func getOAuthUserInfo(provider string, token oauth.Token) (OAuthUserInfo, error) {
	// Call provider's userinfo endpoint
	// Example for Google:
	resp, err := http.Get("https://www.googleapis.com/oauth/v/userinfo?access_token=" + token.AccessToken)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var userInfo OAuthUserInfo
	json.NewDecoder(resp.Body).Decode(&userInfo)

	return &userInfo, nil
}

func verifyOAuthState(state string) bool {
	// Verify state matches what was stored in session
	// This is CSRF protection
	return true // Implement proper state validation
}


 SAML Handler

go
// backend/internal/handlers/saml_handler.go
package handlers

import (
	"github.com/crewjam/saml/samlsp"
	"github.com/gofiber/fiber/v"
)

// SAMLInitiateLogin initiates SAML login
func SAMLInitiateLogin(c fiber.Ctx) error {
	// Initialize SAML service provider
	sp := initSAMLSP()

	// Generate authentication request
	authRequest := sp.AuthnRequest()
	redirectURL := authRequest.AuthnRequestURL()

	return c.Redirect(redirectURL)
}

// SAMLACS handles the SAML Assertion Consumer Service callback
func SAMLACS(c fiber.Ctx) error {
	sp := initSAMLSP()

	// Parse SAML assertion
	assertion, err := sp.ParseResponse(c.Body(), nil)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": fmt.Sprintf("SAML assertion validation failed: %v", err),
		})
	}

	// Extract user attributes
	email := assertion.NameID.Value
	attributes := assertion.AttributeStatements[].Attributes

	userInfo := &OAuthUserInfo{
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
	authService := services.NewAuthService(getEnv("JWT_SECRET", ""), time.Hour)
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

func initSAMLSP() samlsp.ServiceProvider {
	// Load IdP certificate
	idpCert, _ := ioutil.ReadFile(getEnv("SAML_IDP_CERT_PATH", ""))

	// Create service provider
	sp := &samlsp.ServiceProvider{
		EntityID: getEnv("SAML_SP_ENTITY_ID", "openrisk-saml-sp"),
		ACSURL: url.URL{
			Scheme: "https",
			Host:   getEnv("SAML_ACS_URL", "openrisk.yourdomain.com"),
			Path:   "/api/auth/saml/acs",
		},
		IDPMetadataURL: getEnv("SAML_IDP_URL", ""),
	}

	return sp
}

func getAttribute(attributes []saml.Attribute, name string) string {
	for _, attr := range attributes {
		if attr.Name == name && len(attr.Values) >  {
			return attr.Values[].Value
		}
	}
	return ""
}

func provisionUserWithGroups(userInfo OAuthUserInfo, groups []string) (domain.User, error) {
	user, err := provisionUser(userInfo, "saml")
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
			return uuid.MustParse("----")
		}
		if group == "openrisk-analyst" {
			// Return analyst role ID
			return uuid.MustParse("----")
		}
	}

	// Default to viewer role
	return uuid.MustParse("----")
}


 Frontend Integration

 Login Page with SSO Options

tsx
// frontend/src/pages/LoginWithSSO.tsx
import React, { useState } from 'react';
import { useNavigate } from 'react-router-dom';

export function LoginWithSSO() {
  const navigate = useNavigate();
  const [isLoading, setIsLoading] = useState(false);

  const handleOAuthLogin = (provider: string) => {
    setIsLoading(true);
    
    // Generate state for CSRF protection
    const state = crypto.randomUUID();
    sessionStorage.setItem('oauth_state', state);

    // Redirect to backend OAuth login endpoint
    window.location.href = 
      ${import.meta.env.VITE_API_URL}/auth/oauth/${provider}/login?state=${state};
  };

  const handleSAMLLogin = () => {
    setIsLoading(true);
    
    // Redirect to backend SAML login endpoint
    window.location.href = 
      ${import.meta.env.VITE_API_URL}/auth/saml/login;
  };

  return (
    <div className="min-h-screen bg-zinc- flex items-center justify-center">
      <div className="w-full max-w-md p- bg-zinc- rounded-lg border border-zinc-">
        <h className="text-xl font-bold text-white mb-">Sign In to OpenRisk</h>

        {/ Email/Password Login /}
        <div className="mb-">
          <button className="w-full bg-blue- hover:bg-blue- text-white py- rounded">
            Sign in with Email
          </button>
        </div>

        <div className="relative mb-">
          <div className="absolute inset- flex items-center">
            <div className="w-full border-t border-zinc-"></div>
          </div>
          <div className="relative flex justify-center text-sm">
            <span className="px- bg-zinc- text-zinc-">Or continue with</span>
          </div>
        </div>

        {/ OAuth Providers /}
        <div className="space-y- mb-">
          <button
            onClick={() => handleOAuthLogin('google')}
            disabled={isLoading}
            className="w-full flex items-center justify-center gap- bg-white text-black py- rounded hover:bg-gray- disabled:opacity-"
          >
            <img src="https://www.google.com/favicon.ico" alt="Google" className="w- h-" />
            Google
          </button>

          <button
            onClick={() => handleOAuthLogin('github')}
            disabled={isLoading}
            className="w-full flex items-center justify-center gap- bg-zinc- text-white py- rounded hover:bg-zinc- disabled:opacity-"
          >
            <span>GitHub</span>
          </button>

          <button
            onClick={() => handleOAuthLogin('azure')}
            disabled={isLoading}
            className="w-full flex items-center justify-center gap- bg-blue- text-white py- rounded hover:bg-blue- disabled:opacity-"
          >
            <span>Microsoft</span>
          </button>
        </div>

        {/ Enterprise SAML /}
        <div className="relative mb-">
          <div className="absolute inset- flex items-center">
            <div className="w-full border-t border-zinc-"></div>
          </div>
          <div className="relative flex justify-center text-sm">
            <span className="px- bg-zinc- text-zinc-">Enterprise</span>
          </div>
        </div>

        <button
          onClick={handleSAMLLogin}
          disabled={isLoading}
          className="w-full bg-purple- hover:bg-purple- text-white py- rounded disabled:opacity-"
        >
          Enterprise SAML Login
        </button>

        <p className="text-center text-zinc- text-sm mt-">
          Don't have an account? Contact your administrator.
        </p>
      </div>
    </div>
  );
}


 Router Configuration

go
// backend/cmd/server/main.go (relevant OAuth/SAML routes)

// OAuth routes
oauthHandler := handlers.NewOAuthHandler(authService, permissionService)
api.Get("/auth/oauth/:provider/login", oauthHandler.InitiateLogin)
api.Get("/auth/oauth/:provider/callback", oauthHandler.Callback)

// SAML routes
samlHandler := handlers.NewSAMLHandler(authService, permissionService)
api.Get("/auth/saml/login", samlHandler.InitiateLogin)
api.Post("/auth/saml/acs", samlHandler.ACS)
api.Get("/auth/saml/metadata", samlHandler.Metadata)


 Testing OAuth/SAML

 Mock OAuth Provider

go
// backend/internal/handlers/oauth_mock.go
package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// MockOAuthServer simulates an OAuth provider for testing
type MockOAuthServer struct {
	server http.Server
}

func NewMockOAuthServer() MockOAuthServer {
	mux := http.NewServeMux()

	mux.HandleFunc("/oauth/authorize", func(w http.ResponseWriter, r http.Request) {
		clientID := r.URL.Query().Get("client_id")
		redirectURI := r.URL.Query().Get("redirect_uri")
		state := r.URL.Query().Get("state")

		// Redirect back with authorization code
		http.Redirect(w, r, fmt.Sprintf(
			"%s?code=mock_code_&state=%s",
			redirectURI,
			state,
		), http.StatusFound)
	})

	mux.HandleFunc("/oauth/token", func(w http.ResponseWriter, r http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token": "mock_access_token",
			"token_type":   "Bearer",
			"expires_in":   ,
		})
	})

	mux.HandleFunc("/oauth/userinfo", func(w http.ResponseWriter, r http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":       "mock_user_",
			"email":    "test@example.com",
			"name":     "Test User",
			"picture":  "https://example.com/avatar.jpg",
		})
	})

	server := &http.Server{
		Addr:    ":",
		Handler: mux,
	}

	return &MockOAuthServer{server: server}
}

func (m MockOAuthServer) Start() error {
	return m.server.ListenAndServe()
}

func (m MockOAuthServer) Stop() error {
	return m.server.Close()
}


 Test Cases

go
// backend/internal/handlers/oauth_handler_test.go
package handlers

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOAuthGoogleCallback_Success(t testing.T) {
	// Setup mock OAuth server
	mockServer := NewMockOAuthServer()
	go mockServer.Start()
	defer mockServer.Stop()

	// Test callback with valid code
	// ... test implementation
}

func TestOAuthCallback_InvalidState(t testing.T) {
	// Test CSRF protection
	// ... test implementation
}

func TestSAMLACS_ValidAssertion(t testing.T) {
	// Test SAML assertion processing
	// ... test implementation
}

func TestGroupToRoleMapping(t testing.T) {
	groups := []string{"openrisk-admin", "developers"}
	roleID := mapGroupsToRole(groups)
	assert.NotNil(t, roleID)
}


 Deployment Checklist

- [ ] OAuth credentials obtained from providers
- [ ] SAML IdP metadata downloaded
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

 Security Considerations

. State Parameter: Always verify state to prevent CSRF attacks
. Token Storage: Store JWTs securely (httpOnly cookies preferred)
. Encrypted Transport: Always use HTTPS/TLS
. Certificate Validation: Verify SAML IdP certificates
. Assertion Signature: Always validate SAML assertion signatures
. Audience Restriction: Verify assertion intended for your service
. Not On/Not Before: Check SAML time constraints
. Session Management: Implement proper session timeout and revocation

 Troubleshooting

| Error | Cause | Solution |
|-------|-------|----------|
| Invalid state | CSRF token mismatch | Verify state before processing |
| Invalid assertion | IdP certificate wrong | Update IdP cert in configuration |
| User not found | Auto-provisioning disabled | Enable SSO_AUTO_PROVISION |
| Invalid signature | Assertion tampered | Verify certificate chain |
| Token expired | JWT TTL too short | Increase tokenTTL |

---

Next Steps:
- Integrate with Okta: docs/OKTA_INTEGRATION.md
- Integrate with Azure AD: docs/AZURE_AD_INTEGRATION.md
- Multi-tenant SAML: docs/MULTI_TENANT_SAML.md
