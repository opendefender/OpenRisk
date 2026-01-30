package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/opendefender/openrisk/internal/core/domain"
	"github.com/opendefender/openrisk/internal/services"
)

func TestTokenAuth_ExtractTokenFromRequest_Success(t testing.T) {
	tokenAuth := &TokenAuth{}

	app := fiber.New()
	app.Get("/", func(c fiber.Ctx) error {
		token, err := tokenAuth.ExtractTokenFromRequest(c)
		if err != nil {
			return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"token": token})
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer test-token-value")

	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestTokenAuth_ExtractTokenFromRequest_MissingHeader(t testing.T) {
	tokenAuth := &TokenAuth{}

	app := fiber.New()
	app.Get("/", func(c fiber.Ctx) error {
		_, err := tokenAuth.ExtractTokenFromRequest(c)
		if err != nil {
			return c.Status(http.StatusUnauthorized).JSON(fiber.Map{"error": err.Error()})
		}
		return c.SendStatus(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestTokenAuth_ExtractTokenFromRequest_InvalidFormat(t testing.T) {
	tokenAuth := &TokenAuth{}

	app := fiber.New()
	app.Get("/", func(c fiber.Ctx) error {
		_, err := tokenAuth.ExtractTokenFromRequest(c)
		if err != nil {
			return c.Status(http.StatusUnauthorized).JSON(fiber.Map{"error": err.Error()})
		}
		return c.SendStatus(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "InvalidFormat")
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestTokenAuth_ExtractTokenFromRequest_WrongScheme(t testing.T) {
	tokenAuth := &TokenAuth{}

	app := fiber.New()
	app.Get("/", func(c fiber.Ctx) error {
		_, err := tokenAuth.ExtractTokenFromRequest(c)
		if err != nil {
			return c.Status(http.StatusUnauthorized).JSON(fiber.Map{"error": err.Error()})
		}
		return c.SendStatus(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestTokenAuth_Verify_Success(t testing.T) {
	tokenService := services.NewTokenService()
	tokenAuth := NewTokenAuth(tokenService)

	app := fiber.New()
	app.Use(tokenAuth.Verify)
	app.Get("/protected", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"message": "protected resource"})
	})

	userID := uuid.New()
	tokenWithValue, err := tokenService.CreateToken(userID, &domain.TokenCreateRequest{
		Name: "Test Token",
	}, userID)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+tokenWithValue.Token)

	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestTokenAuth_Verify_NoHeader(t testing.T) {
	tokenService := services.NewTokenService()
	tokenAuth := NewTokenAuth(tokenService)

	app := fiber.New()
	app.Use(tokenAuth.Verify)
	app.Get("/protected", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"message": "protected"})
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestTokenAuth_Verify_InvalidToken(t testing.T) {
	tokenService := services.NewTokenService()
	tokenAuth := NewTokenAuth(tokenService)

	app := fiber.New()
	app.Use(tokenAuth.Verify)
	app.Get("/protected", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"message": "protected"})
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")

	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestTokenAuth_Verify_RevokedToken(t testing.T) {
	tokenService := services.NewTokenService()
	tokenAuth := NewTokenAuth(tokenService)

	app := fiber.New()
	app.Use(tokenAuth.Verify)
	app.Get("/protected", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"message": "protected"})
	})

	userID := uuid.New()
	tokenWithValue, err := tokenService.CreateToken(userID, &domain.TokenCreateRequest{
		Name: "Revokable Token",
	}, userID)
	require.NoError(t, err)

	// Revoke token
	tokenService.RevokeToken(tokenWithValue.ID, "testing")

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+tokenWithValue.Token)

	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestTokenAuth_RequireTokenPermission_Success(t testing.T) {
	tokenService := services.NewTokenService()
	tokenAuth := NewTokenAuth(tokenService)

	app := fiber.New()
	// Register both Verify and RequireTokenPermission
	app.Use("/risks", tokenAuth.Verify)
	app.Get("/risks", tokenAuth.RequireTokenPermission("risk:read:any"), func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"risks": []string{}})
	})

	userID := uuid.New()
	tokenWithValue, err := tokenService.CreateToken(userID, &domain.TokenCreateRequest{
		Name:        "Read Risks Token",
		Permissions: []string{"risk:read:any"},
	}, userID)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/risks", nil)
	req.Header.Set("Authorization", "Bearer "+tokenWithValue.Token)

	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestTokenAuth_RequireTokenPermission_Denied(t testing.T) {
	tokenService := services.NewTokenService()
	tokenAuth := NewTokenAuth(tokenService)

	app := fiber.New()
	app.Use("/risks", tokenAuth.Verify)
	app.Get("/risks", tokenAuth.RequireTokenPermission("risk:delete:any"), func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"message": "deleted"})
	})

	userID := uuid.New()
	tokenWithValue, err := tokenService.CreateToken(userID, &domain.TokenCreateRequest{
		Name:        "Read Only Token",
		Permissions: []string{"risk:read:any"},
	}, userID)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/risks", nil)
	req.Header.Set("Authorization", "Bearer "+tokenWithValue.Token)

	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestTokenAuth_RequireTokenScope_Success(t testing.T) {
	tokenService := services.NewTokenService()
	tokenAuth := NewTokenAuth(tokenService)

	app := fiber.New()
	app.Use("/risks", tokenAuth.Verify)
	app.Get("/risks", tokenAuth.RequireTokenScope("risk"), func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"risks": []string{}})
	})

	userID := uuid.New()
	tokenWithValue, err := tokenService.CreateToken(userID, &domain.TokenCreateRequest{
		Name:   "Risk Scoped Token",
		Scopes: []string{"risk", "mitigation"},
	}, userID)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/risks", nil)
	req.Header.Set("Authorization", "Bearer "+tokenWithValue.Token)

	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestTokenAuth_RequireTokenScope_Denied(t testing.T) {
	tokenService := services.NewTokenService()
	tokenAuth := NewTokenAuth(tokenService)

	app := fiber.New()
	app.Use("/assets", tokenAuth.Verify)
	app.Get("/assets", tokenAuth.RequireTokenScope("asset"), func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"assets": []string{}})
	})

	userID := uuid.New()
	tokenWithValue, err := tokenService.CreateToken(userID, &domain.TokenCreateRequest{
		Name:   "Risk Only Token",
		Scopes: []string{"risk"},
	}, userID)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/assets", nil)
	req.Header.Set("Authorization", "Bearer "+tokenWithValue.Token)

	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestTokenAuth_ContextPopulation(t testing.T) {
	tokenService := services.NewTokenService()
	tokenAuth := NewTokenAuth(tokenService)

	userID := uuid.New()
	tokenWithValue, err := tokenService.CreateToken(userID, &domain.TokenCreateRequest{
		Name: "Context Test Token",
	}, userID)
	require.NoError(t, err)

	app := fiber.New()
	app.Use(tokenAuth.Verify)
	app.Get("/test", func(c fiber.Ctx) error {
		retrievedUserID := c.Locals("userID")
		retrievedTokenID := c.Locals("tokenID")

		if retrievedUserID == nil || retrievedTokenID == nil {
			return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
				"error": "context not populated",
			})
		}

		return c.JSON(fiber.Map{
			"userID":  retrievedUserID,
			"tokenID": retrievedTokenID,
		})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+tokenWithValue.Token)

	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
}
