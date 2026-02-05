package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/opendefender/openrisk/database"
	"github.com/opendefender/openrisk/internal/core/domain"
	"github.com/opendefender/openrisk/internal/services"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
	"golang.org/x/oauth2/google"
	"golang.org/x/oauth2/microsoft"
)

// OAuth2Config holds OAuth2 provider configurations
type OAuth2Config struct {
	GoogleConfig *oauth2.Config
	GitHubConfig *oauth2.Config
	AzureConfig  *oauth2.Config
}

var oauth2Config *OAuth2Config

// InitializeOAuth2 initializes all OAuth2 configurations
func InitializeOAuth2() *OAuth2Config {
	redirectURI := os.Getenv("OAUTH2_REDIRECT_URI")
	if redirectURI == "" {
		redirectURI = "http://localhost:8080/api/v1/auth/oauth2/callback"
	}

	cfg := &OAuth2Config{
		GoogleConfig: &oauth2.Config{
			ClientID:     os.Getenv("OAUTH2_GOOGLE_CLIENT_ID"),
			ClientSecret: os.Getenv("OAUTH2_GOOGLE_CLIENT_SECRET"),
			RedirectURL:  redirectURI + "/google",
			Scopes: []string{
				"https://www.googleapis.com/auth/userinfo.email",
				"https://www.googleapis.com/auth/userinfo.profile",
			},
			Endpoint: google.Endpoint,
		},
		GitHubConfig: &oauth2.Config{
			ClientID:     os.Getenv("OAUTH2_GITHUB_CLIENT_ID"),
			ClientSecret: os.Getenv("OAUTH2_GITHUB_CLIENT_SECRET"),
			RedirectURL:  redirectURI + "/github",
			Scopes: []string{
				"user:email",
				"read:user",
			},
			Endpoint: github.Endpoint,
		},
		AzureConfig: &oauth2.Config{
			ClientID:     os.Getenv("OAUTH2_AZURE_CLIENT_ID"),
			ClientSecret: os.Getenv("OAUTH2_AZURE_CLIENT_SECRET"),
			RedirectURL:  redirectURI + "/azure",
			Scopes: []string{
				"https://graph.microsoft.com/.default",
			},
			Endpoint: microsoft.AzureADEndpoint(os.Getenv("OAUTH2_AZURE_TENANT_ID")),
		},
	}

	oauth2Config = cfg
	return cfg
}

// OAuth2UserInfo represents user information from OAuth2 provider
type OAuth2UserInfo struct {
	ID       string
	Email    string
	Name     string
	Picture  string
	Provider string
	Groups   []string
}

// OAuth2Login initiates OAuth2 login flow
func OAuth2Login(c *fiber.Ctx) error {
	provider := c.Params("provider") // google, github, azure

	var config *oauth2.Config
	switch provider {
	case "google":
		config = oauth2Config.GoogleConfig
	case "github":
		config = oauth2Config.GitHubConfig
	case "azure":
		config = oauth2Config.AzureConfig
	default:
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Unsupported OAuth2 provider",
		})
	}

	if config.ClientID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fmt.Sprintf("OAuth2 provider %s not configured", provider),
		})
	}

	// Generate random state for CSRF protection
	randomState := uuid.New().String()

	// Store state in session/cache (TODO: implement session storage)
	// For now, we'll verify state matches the provider's requirement

	authURL := config.AuthCodeURL(randomState, oauth2.AccessTypeOffline)

	return c.JSON(fiber.Map{
		"redirect_url": authURL,
		"state":        randomState,
	})
}

// OAuth2Callback handles OAuth2 provider callback
func OAuth2Callback(c *fiber.Ctx) error {
	provider := c.Params("provider")
	code := c.Query("code")
	// TODO: Validate state parameter for CSRF protection
	_ = c.Query("state")

	if code == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Authorization code not provided",
		})
	}

	var config *oauth2.Config
	switch provider {
	case "google":
		config = oauth2Config.GoogleConfig
	case "github":
		config = oauth2Config.GitHubConfig
	case "azure":
		config = oauth2Config.AzureConfig
	default:
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Unsupported OAuth2 provider",
		})
	}

	// Exchange authorization code for token
	ctx := context.Background()
	token, err := config.Exchange(ctx, code)
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

	userInfo.Provider = provider

	// Provision user (find or create)
	user, err := provisionOAuth2User(userInfo)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to provision user: %v", err),
		})
	}

	// Generate JWT token
	authService := services.NewAuthService(os.Getenv("JWT_SECRET"), 24*time.Hour)
	jwtToken, err := authService.GenerateToken(user)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to generate token",
		})
	}

	// Log successful authentication
	auditService := services.NewAuditService()
	auditService.LogLogin(user.ID, domain.ResultSuccess, c.IP(), c.Get("User-Agent"), "")

	// Return token to frontend
	return c.JSON(fiber.Map{
		"token":    jwtToken,
		"user":     user,
		"provider": provider,
	})
}

// getOAuth2UserInfo fetches user information from OAuth2 provider
func getOAuth2UserInfo(provider string, token *oauth2.Token) (*OAuth2UserInfo, error) {
	switch provider {
	case "google":
		return getGoogleUserInfo(token)
	case "github":
		return getGitHubUserInfo(token)
	case "azure":
		return getAzureUserInfo(token)
	default:
		return nil, fmt.Errorf("unsupported provider: %s", provider)
	}
}

// getGoogleUserInfo fetches user info from Google
func getGoogleUserInfo(token *oauth2.Token) (*OAuth2UserInfo, error) {
	req, err := http.NewRequest("GET", "https://www.googleapis.com/oauth2/v2/userinfo", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, err
	}

	return &OAuth2UserInfo{
		ID:      data["id"].(string),
		Email:   data["email"].(string),
		Name:    data["name"].(string),
		Picture: data["picture"].(string),
	}, nil
}

// getGitHubUserInfo fetches user info from GitHub
func getGitHubUserInfo(token *oauth2.Token) (*OAuth2UserInfo, error) {
	req, err := http.NewRequest("GET", "https://api.github.com/user", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Authorization", "token "+token.AccessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, err
	}

	// Get email from separate endpoint if not in user data
	email := ""
	if e, ok := data["email"].(string); ok && e != "" {
		email = e
	} else {
		email, _ = getGitHubEmail(token)
	}

	return &OAuth2UserInfo{
		ID:      fmt.Sprintf("%v", data["id"]),
		Email:   email,
		Name:    data["login"].(string),
		Picture: data["avatar_url"].(string),
	}, nil
}

// getGitHubEmail fetches email from GitHub /user/emails endpoint
func getGitHubEmail(token *oauth2.Token) (string, error) {
	req, err := http.NewRequest("GET", "https://api.github.com/user/emails", nil)
	if err != nil {
		return "", err
	}
	req.Header.Add("Authorization", "token "+token.AccessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var emails []map[string]interface{}
	if err := json.Unmarshal(body, &emails); err != nil {
		return "", err
	}

	// Return primary email
	for _, e := range emails {
		if primary, ok := e["primary"].(bool); ok && primary {
			if email, ok := e["email"].(string); ok {
				return email, nil
			}
		}
	}

	// Fallback to first email
	if len(emails) > 0 {
		if email, ok := emails[0]["email"].(string); ok {
			return email, nil
		}
	}

	return "", fmt.Errorf("no email found")
}

// getAzureUserInfo fetches user info from Azure AD
func getAzureUserInfo(token *oauth2.Token) (*OAuth2UserInfo, error) {
	req, err := http.NewRequest("GET", "https://graph.microsoft.com/v1.0/me", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Authorization", "Bearer "+token.AccessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, err
	}

	return &OAuth2UserInfo{
		ID:    data["id"].(string),
		Email: data["userPrincipalName"].(string),
		Name:  data["displayName"].(string),
	}, nil
}

// provisionOAuth2User finds or creates a user from OAuth2 info
func provisionOAuth2User(userInfo *OAuth2UserInfo) (*domain.User, error) {
	user := &domain.User{}

	// Find existing user by email
	result := database.DB.Preload("Role").Where("email = ?", userInfo.Email).First(user)

	if result.Error == gorm.ErrRecordNotFound {
		// Check if auto-provisioning is enabled
		autoProvision := os.Getenv("SSO_AUTO_PROVISION")
		if autoProvision == "" {
			autoProvision = "true"
		}

		if autoProvision != "true" {
			return nil, fmt.Errorf("user auto-provisioning disabled")
		}

		// Get default role
		defaultRole := &domain.Role{}
		if err := database.DB.Where("name = ?", "viewer").First(defaultRole).Error; err != nil {
			return nil, fmt.Errorf("default role not found: %w", err)
		}

		// Create new user
		user = &domain.User{
			ID:       uuid.New(),
			Email:    userInfo.Email,
			Username: userInfo.Email,
			FullName: userInfo.Name,
			RoleID:   defaultRole.ID,
			IsActive: true,
		}

		if err := database.DB.Create(user).Error; err != nil {
			return nil, fmt.Errorf("failed to create user: %w", err)
		}

		// Reload with role
		database.DB.Preload("Role").First(user)

		return user, nil
	}

	if result.Error != nil {
		return nil, result.Error
	}

	// Update existing user if auto-update is enabled
	autoUpdate := os.Getenv("SSO_AUTO_UPDATE_PROFILE")
	if autoUpdate == "" {
		autoUpdate = "true"
	}

	if autoUpdate == "true" {
		user.FullName = userInfo.Name
		database.DB.Save(user)
	}

	return user, nil
}
