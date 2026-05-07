package auth

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/application/auth"
)

// OAuthHandler handles OAuth endpoints
type OAuthHandler struct {
	googleUseCase *auth.OAuthGoogleUseCase
	githubUseCase *auth.OAuthGitHubUseCase
}

// NewOAuthHandler creates a new OAuth handler
func NewOAuthHandler(
	googleUseCase *auth.OAuthGoogleUseCase,
	githubUseCase *auth.OAuthGitHubUseCase,
) *OAuthHandler {
	return &OAuthHandler{
		googleUseCase: googleUseCase,
		githubUseCase: githubUseCase,
	}
}

// GoogleOAuthRedirectRequest represents Google OAuth redirect request
type GoogleOAuthRedirectRequest struct {
	RedirectURI string `json:"redirect_uri" validate:"required,url"`
}

// GoogleOAuthRedirectResponse represents Google OAuth redirect response
type GoogleOAuthRedirectResponse struct {
	AuthURL string `json:"auth_url"`
}

// HandleGoogleRedirect handles GET /auth/oauth2/google/redirect
// Returns the Google OAuth authorization URL
func (h *OAuthHandler) HandleGoogleRedirect(c *fiber.Ctx) error {
	// In production, construct the Google OAuth URL
	// For now, return a placeholder
	authURL := "https://accounts.google.com/o/oauth2/v2/auth?client_id=YOUR_CLIENT_ID&redirect_uri=YOUR_REDIRECT_URI&response_type=code&scope=openid email profile"

	return c.Status(fiber.StatusOK).JSON(GoogleOAuthRedirectResponse{
		AuthURL: authURL,
	})
}

// GoogleOAuthCallbackRequest represents Google OAuth callback
type GoogleOAuthCallbackRequest struct {
	Code string `json:"code" validate:"required"`
}

// OAuthCallbackResponse represents OAuth callback response
type OAuthCallbackResponse struct {
	User         UserResponse `json:"user"`
	TokenPair    TokenPairResponse `json:"token_pair"`
	IsNewAccount bool `json:"is_new_account"`
}

// UserResponse represents user data in response
type UserResponse struct {
	ID       string `json:"id"`
	Email    string `json:"email"`
	Username string `json:"username"`
	FullName string `json:"full_name"`
	IsActive bool   `json:"is_active"`
}

// TokenPairResponse represents token pair in response
type TokenPairResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
}

// HandleGoogleCallback handles GET /auth/oauth2/google/callback
// Processes the OAuth code from Google
func (h *OAuthHandler) HandleGoogleCallback(c *fiber.Ctx) error {
	code := c.Query("code")
	state := c.Query("state")

	if code == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "authorization code required",
		})
	}

	// Exchange code for token with Google
	googleTokens, err := exchangeGoogleCode(code)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "failed to exchange code",
		})
	}

	// Get user info from Google
	googleUserInfo, err := getGoogleUserInfo(googleTokens.AccessToken)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "failed to get user info",
		})
	}

	// Execute OAuth use case
	input := auth.OAuthCallbackInput{
		Provider:       "google",
		Code:           code,
		Email:          googleUserInfo.Email,
		ProviderUserID: googleUserInfo.ID,
		AccessToken:    googleTokens.AccessToken,
		RefreshToken:   googleTokens.RefreshToken,
		ExpiresIn:      int64(googleTokens.ExpiresIn),
	}

	output, err := h.googleUseCase.Execute(c.Context(), input)
	if err != nil {
		return handleError(c, err)
	}

	return c.Status(fiber.StatusOK).JSON(OAuthCallbackResponse{
		User: UserResponse{
			ID:       output.User.ID.String(),
			Email:    output.User.Email,
			Username: output.User.Username,
			FullName: output.User.FullName,
			IsActive: output.User.IsActive,
		},
		TokenPair: TokenPairResponse{
			AccessToken:  output.TokenPair.AccessToken,
			RefreshToken: output.TokenPair.RefreshToken,
			TokenType:    output.TokenPair.TokenType,
			ExpiresIn:    output.TokenPair.ExpiresIn,
		},
		IsNewAccount: output.IsNewAccount,
	})
}

// GitHubOAuthRedirectResponse represents GitHub OAuth redirect response
type GitHubOAuthRedirectResponse struct {
	AuthURL string `json:"auth_url"`
}

// HandleGitHubRedirect handles GET /auth/oauth2/github/redirect
// Returns the GitHub OAuth authorization URL
func (h *OAuthHandler) HandleGitHubRedirect(c *fiber.Ctx) error {
	// In production, construct the GitHub OAuth URL
	authURL := "https://github.com/login/oauth/authorize?client_id=YOUR_CLIENT_ID&redirect_uri=YOUR_REDIRECT_URI&scope=user:email"

	return c.Status(fiber.StatusOK).JSON(GitHubOAuthRedirectResponse{
		AuthURL: authURL,
	})
}

// HandleGitHubCallback handles GET /auth/oauth2/github/callback
// Processes the OAuth code from GitHub
func (h *OAuthHandler) HandleGitHubCallback(c *fiber.Ctx) error {
	code := c.Query("code")
	state := c.Query("state")

	if code == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "authorization code required",
		})
	}

	// Exchange code for token with GitHub
	githubTokens, err := exchangeGitHubCode(code)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "failed to exchange code",
		})
	}

	// Get user info from GitHub
	githubUserInfo, err := getGitHubUserInfo(githubTokens.AccessToken)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "failed to get user info",
		})
	}

	// Execute OAuth use case
	input := auth.OAuthCallbackInput{
		Provider:       "github",
		Code:           code,
		Email:          githubUserInfo.Email,
		ProviderUserID: fmt.Sprintf("%d", githubUserInfo.ID),
		AccessToken:    githubTokens.AccessToken,
		RefreshToken:   githubTokens.RefreshToken,
	}

	output, err := h.githubUseCase.Execute(c.Context(), input)
	if err != nil {
		return handleError(c, err)
	}

	return c.Status(fiber.StatusOK).JSON(OAuthCallbackResponse{
		User: UserResponse{
			ID:       output.User.ID.String(),
			Email:    output.User.Email,
			Username: output.User.Username,
			FullName: output.User.FullName,
			IsActive: output.User.IsActive,
		},
		TokenPair: TokenPairResponse{
			AccessToken:  output.TokenPair.AccessToken,
			RefreshToken: output.TokenPair.RefreshToken,
			TokenType:    output.TokenPair.TokenType,
			ExpiresIn:    output.TokenPair.ExpiresIn,
		},
		IsNewAccount: output.IsNewAccount,
	})
}

// Google OAuth helper types
type GoogleTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
}

type GoogleUserInfo struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

// exchangeGoogleCode exchanges Google authorization code for tokens
func exchangeGoogleCode(code string) (*GoogleTokenResponse, error) {
	// TODO: Implement with actual Google OAuth2 library
	// This is a placeholder
	return &GoogleTokenResponse{
		AccessToken: "mock_access_token",
		TokenType:   "Bearer",
		ExpiresIn:   3600,
	}, nil
}

// getGoogleUserInfo retrieves user info from Google
func getGoogleUserInfo(accessToken string) (*GoogleUserInfo, error) {
	// TODO: Implement with actual Google API call
	// This is a placeholder
	return &GoogleUserInfo{
		ID:    "google_user_123",
		Email: "user@example.com",
		Name:  "Test User",
	}, nil
}

// GitHub OAuth helper types
type GitHubTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
}

type GitHubUserInfo struct {
	ID    int    `json:"id"`
	Login string `json:"login"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

// exchangeGitHubCode exchanges GitHub authorization code for tokens
func exchangeGitHubCode(code string) (*GitHubTokenResponse, error) {
	// TODO: Implement with actual GitHub OAuth2 library
	// This is a placeholder
	return &GitHubTokenResponse{
		AccessToken: "mock_access_token",
		TokenType:   "Bearer",
	}, nil
}

// getGitHubUserInfo retrieves user info from GitHub
func getGitHubUserInfo(accessToken string) (*GitHubUserInfo, error) {
	// Make request to GitHub API
	client := &http.Client{}
	req, err := http.NewRequest("GET", "https://api.github.com/user", nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "token "+accessToken)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var userInfo GitHubUserInfo
	if err := json.Unmarshal(body, &userInfo); err != nil {
		return nil, err
	}

	return &userInfo, nil
}
