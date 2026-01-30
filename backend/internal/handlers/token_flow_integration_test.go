//go:build integration
// +build integration

package handlers

import (
	"testing"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/database"
	"github.com/opendefender/openrisk/internal/core/domain"
	"github.com/opendefender/openrisk/internal/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTokenServiceFlow tests the complete token service operations
func TestTokenServiceFlow(t testing.T) {
	InitTestDB(t)
	defer CleanupTestDB(t, database.DB)

	tokenService := services.NewTokenService()
	userID := uuid.New()

	t.Run("create-verify-token-flow", func(t testing.T) {
		// Create a token
		tokenWithValue, err := tokenService.CreateToken(userID, &domain.TokenCreateRequest{
			Name:        "test-token",
			Description: "A test token",
		}, userID)

		require.NoError(t, err, "should create token without error")
		require.NotNil(t, tokenWithValue, "token with value should not be nil")
		assert.Equal(t, "test-token", tokenWithValue.Name, "token name should match")
		assert.NotEmpty(t, tokenWithValue.Token, "token value should not be empty")
		assert.NotEmpty(t, tokenWithValue.ID, "token ID should not be empty")

		// Verify the token works
		verified, err := tokenService.VerifyToken(tokenWithValue.Token)
		require.NoError(t, err, "should verify token without error")
		require.NotNil(t, verified, "verified token should not be nil")
		assert.Equal(t, userID, verified.UserID, "verified token user ID should match")
		assert.Equal(t, "test-token", verified.Name, "verified token name should match")
		assert.Equal(t, "active", verified.Status, "new token should be active")
	})

	t.Run("list-tokens", func(t testing.T) {
		// Create two tokens
		_, err := tokenService.CreateToken(userID, &domain.TokenCreateRequest{
			Name: "token",
		}, userID)
		require.NoError(t, err)

		_, err = tokenService.CreateToken(userID, &domain.TokenCreateRequest{
			Name: "token",
		}, userID)
		require.NoError(t, err)

		// List tokens
		tokens, err := tokenService.ListTokens(userID)
		require.NoError(t, err, "should list tokens without error")
		assert.GreaterOrEqual(t, len(tokens), , "should have at least  tokens")
	})

	t.Run("get-token", func(t testing.T) {
		// Create a token
		tokenWithValue, err := tokenService.CreateToken(userID, &domain.TokenCreateRequest{
			Name: "get-test-token",
		}, userID)
		require.NoError(t, err)

		// Get the token
		token, err := tokenService.GetToken(tokenWithValue.ID)
		require.NoError(t, err, "should get token without error")
		require.NotNil(t, token, "token should not be nil")
		assert.Equal(t, "get-test-token", token.Name, "token name should match")
	})

	t.Run("update-token", func(t testing.T) {
		// Create a token
		tokenWithValue, err := tokenService.CreateToken(userID, &domain.TokenCreateRequest{
			Name:        "update-test",
			Description: "Original description",
		}, userID)
		require.NoError(t, err)

		// Update the token
		updated, err := tokenService.UpdateToken(tokenWithValue.ID, &domain.TokenUpdateRequest{
			Name:        "updated-name",
			Description: "Updated description",
		})
		require.NoError(t, err, "should update token without error")
		require.NotNil(t, updated, "updated token should not be nil")
		assert.Equal(t, "updated-name", updated.Name, "token name should be updated")
		assert.Equal(t, "Updated description", updated.Description, "token description should be updated")
	})

	t.Run("rotate-token", func(t testing.T) {
		// Create a token
		tokenWithValue, err := tokenService.CreateToken(userID, &domain.TokenCreateRequest{
			Name: "rotate-test",
		}, userID)
		require.NoError(t, err)

		oldToken := tokenWithValue.Token

		// Rotate the token
		rotateResp, err := tokenService.RotateToken(tokenWithValue.ID, userID, "rotation test")
		require.NoError(t, err, "should rotate token without error")
		require.NotNil(t, rotateResp, "rotate response should not be nil")
		require.NotNil(t, rotateResp.OldToken, "old token should be in response")
		require.NotNil(t, rotateResp.NewToken, "new token should be in response")
		assert.NotEqual(t, oldToken, rotateResp.NewToken.Token, "new token should be different from old token")

		// Verify old token no longer works
		verified, err := tokenService.VerifyToken(oldToken)
		require.Error(t, err, "old token should fail verification after rotation")
		assert.Nil(t, verified, "verified old token should be nil")

		// Verify new token works
		verified, err = tokenService.VerifyToken(rotateResp.NewToken.Token)
		require.NoError(t, err, "new token should verify successfully")
		require.NotNil(t, verified, "verified new token should not be nil")
	})

	t.Run("revoke-token", func(t testing.T) {
		// Create a token
		tokenWithValue, err := tokenService.CreateToken(userID, &domain.TokenCreateRequest{
			Name: "revoke-test",
		}, userID)
		require.NoError(t, err)

		tokenID := tokenWithValue.ID

		// Revoke the token
		revoked, err := tokenService.RevokeToken(tokenID, "testing revocation")
		require.NoError(t, err, "should revoke token without error")
		require.NotNil(t, revoked, "revoked token should not be nil")
		assert.Equal(t, "revoked", revoked.Status, "revoked token status should be 'revoked'")

		// Verify revoked token cannot be used
		verified, err := tokenService.VerifyToken(tokenWithValue.Token)
		require.Error(t, err, "revoked token should fail verification")
		assert.Nil(t, verified, "verified revoked token should be nil")
	})

	t.Run("delete-token", func(t testing.T) {
		// Create a token
		tokenWithValue, err := tokenService.CreateToken(userID, &domain.TokenCreateRequest{
			Name: "delete-test",
		}, userID)
		require.NoError(t, err)

		tokenID := tokenWithValue.ID

		// Delete the token
		err = tokenService.DeleteToken(tokenID)
		require.NoError(t, err, "should delete token without error")

		// Verify token is deleted (should return error)
		token, err := tokenService.GetToken(tokenID)
		require.Error(t, err, "deleted token should return error on retrieval")
		assert.Nil(t, token, "deleted token should be nil")
	})

	t.Run("invalid-token-verification", func(t testing.T) {
		// Try to verify an invalid token
		verified, err := tokenService.VerifyToken("invalid-token-value")
		require.Error(t, err, "should fail to verify invalid token")
		assert.Nil(t, verified, "verified invalid token should be nil")
	})

	t.Run("token-ownership-enforcement", func(t testing.T) {
		// Create a token for user 
		userID := uuid.New()

		token, err := tokenService.CreateToken(userID, &domain.TokenCreateRequest{
			Name: "user-token",
		}, userID)
		require.NoError(t, err)

		// Verify token belongs to user 
		token, err := tokenService.GetToken(token.ID)
		require.NoError(t, err, "should be able to get any token by ID")
		require.NotNil(t, token, "token should exist")
		assert.Equal(t, userID, token.UserID, "token should belong to user ")
	})

	t.Run("token-with-permissions", func(t testing.T) {
		// Create a token with specific permissions
		tokenWithValue, err := tokenService.CreateToken(userID, &domain.TokenCreateRequest{
			Name:        "restricted-token",
			Permissions: []string{"risk:read:any", "mitigation:create:any"},
			Scopes:      []string{"risk", "mitigation"},
		}, userID)
		require.NoError(t, err)

		// Verify token has correct permissions
		verified, err := tokenService.VerifyToken(tokenWithValue.Token)
		require.NoError(t, err)
		assert.NotNil(t, verified)
		assert.Len(t, verified.Permissions, , "token should have  permissions")
		assert.Len(t, verified.Scopes, , "token should have  scopes")
	})
}
