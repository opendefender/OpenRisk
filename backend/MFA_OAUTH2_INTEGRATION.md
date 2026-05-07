# MFA + OAuth2 Integration Guide

## Overview
This document describes how to integrate the MFA and OAuth2 systems into the OpenRisk backend.

## Components Created

### 1. Crypto Layer (`backend/pkg/crypto/aes.go`)
- `EncryptAES256GCM()` - Encrypts secrets with AES-256-GCM
- `DecryptAES256GCM()` - Decrypts encrypted secrets
- 32-byte key requirement, 12-byte nonce, base64 encoding

### 2. OTP Layer (`backend/pkg/otp/totp.go`)
- `GenerateTOTPSecret()` - Generates TOTP secret
- `GenerateTOTPSecret2()` - Generates per-user TOTP secret
- `GetTOTPQRCode()` - Creates QR code for MFA setup
- `VerifyTOTP()` - Verifies 6-digit code with ±1 window
- `GenerateBackupCodes()` - Creates 8 single-use backup codes

### 3. Domain Models (`backend/internal/domain/mfa.go`)
- `MFASecret` - Stores encrypted TOTP secret
- `MFABackupCode` - Single-use backup codes (bcrypt hashed)
- `OAuthProvider` - OAuth provider links (Google, GitHub)
- `MFAToken` - Temporary token during MFA challenge

### 4. Repositories
- `backend/internal/infrastructure/repository/mfa_repository.go` (interface)
- `backend/internal/infrastructure/repository/gorm_mfa_repository.go` (implementation)
- Methods:
  - CreateMFASecret, GetMFASecret, UpdateMFASecret, DisableMFA
  - SaveBackupCodes, GetUnusedBackupCodes, MarkBackupCodeAsUsed
  - CreateOAuthProvider, GetOAuthProvider, ListOAuthProviders, etc.

### 5. Use Cases
- `backend/internal/application/auth/mfa_usecase.go`
  - `SetupMFAUseCase` - Generate secret, QR code, backup codes
  - `VerifyMFAUseCase` - Activate MFA with TOTP code
  - `ChallengeMFAUseCase` - Verify during login (TOTP or backup code)
  - `DisableMFAUseCase` - Disable MFA (requires password)

- `backend/internal/application/auth/oauth_usecase.go`
  - `OAuthGoogleUseCase` - Handle Google OAuth callback
  - `OAuthGitHubUseCase` - Handle GitHub OAuth callback

### 6. HTTP Handlers
- `backend/internal/handler/auth/mfa_handler.go`
  - POST /auth/mfa/setup
  - POST /auth/mfa/verify
  - POST /auth/mfa/challenge
  - POST /auth/mfa/disable

- `backend/internal/handler/auth/oauth_handler.go`
  - GET /auth/oauth2/google/redirect
  - GET /auth/oauth2/google/callback
  - GET /auth/oauth2/github/redirect
  - GET /auth/oauth2/github/callback

### 7. Database Migration
- `database/0025_create_mfa_tables.sql`
  - mfa_secrets table with encrypted secret storage
  - mfa_backup_codes table for backup codes
  - user_oauth_providers table for OAuth links
  - Indices on tenant_id, user_id for multi-tenancy

## Integration Steps in main.go

### Step 1: Load Encryption Key
```go
// In config.go or environment setup
encryptionKey := os.Getenv("ENCRYPTION_KEY") // 32 bytes
if len(encryptionKey) != 32 {
    log.Fatal("ENCRYPTION_KEY must be 32 bytes")
}
encKey := []byte(encryptionKey)
```

### Step 2: Initialize Repositories
```go
// In DI container (main.go or container.go)
mfaRepo := repository.NewGormMFARepository(db)
oauthProviderRepo := repository.NewGormOAuthProviderRepository(db)
```

### Step 3: Initialize Use Cases
```go
// MFA Use Cases
setupMFAUC := auth.NewSetupMFAUseCase(mfaRepo, encKey)
verifyMFAUC := auth.NewVerifyMFAUseCase(mfaRepo, userRepo, encKey)
challengeMFAUC := auth.NewChallengeMFAUseCase(mfaRepo, encKey)
disableMFAUC := auth.NewDisableMFAUseCase(mfaRepo, passwordHasher)

// OAuth Use Cases
googleOAuthUC := auth.NewOAuthGoogleUseCase(userRepo, oauthProviderRepo, tokenManager, mfaRepo)
githubOAuthUC := auth.NewOAuthGitHubUseCase(userRepo, oauthProviderRepo, tokenManager, mfaRepo)
```

### Step 4: Initialize Handlers
```go
// MFA Handler
mfaHandler := handler.NewMFAHandler(setupMFAUC, verifyMFAUC, challengeMFAUC, disableMFAUC)

// OAuth Handler
oauthHandler := handler.NewOAuthHandler(googleOAuthUC, githubOAuthUC)
```

### Step 5: Register Routes
```go
app := fiber.New()

// MFA Routes (Protected - require authentication)
authGroup := app.Group("/auth", AuthMiddleware)
authGroup.Post("/mfa/setup", mfaHandler.HandleSetupMFA)
authGroup.Post("/mfa/verify", mfaHandler.HandleVerifyMFA)
authGroup.Post("/mfa/disable", mfaHandler.HandleDisableMFA)

// MFA Challenge Route (Public - used after login with MFA_REQUIRED)
app.Post("/auth/mfa/challenge", mfaHandler.HandleChallengeMFA)

// OAuth Routes (Public)
app.Get("/auth/oauth2/google/redirect", oauthHandler.HandleGoogleRedirect)
app.Get("/auth/oauth2/google/callback", oauthHandler.HandleGoogleCallback)
app.Get("/auth/oauth2/github/redirect", oauthHandler.HandleGitHubRedirect)
app.Get("/auth/oauth2/github/callback", oauthHandler.HandleGitHubCallback)
```

## Security Considerations

### Rule 1: Tenant Isolation
- All MFA repository queries include `WHERE tenant_id = ?`
- Foreign tenant access returns 404 (not found)
- OAuth providers are scoped to user's tenant

### Rule 2: Secret Storage
- TOTP secrets encrypted AES-256-GCM before DB storage
- Encryption key loaded from environment (never hardcoded)
- Backup codes hashed with bcrypt (not plaintext)
- OAuth tokens encrypted before storage

### Rule 3: No Secrets in Logs
- Error messages don't expose tokens/secrets
- Use domain.AppError for consistent error handling
- Secrets redacted in audit logs

### Rule 4: MFA_REQUIRED Token
- Generated during login if MFA enabled
- Short-lived token (5 minutes)
- Allows access only to /auth/mfa/challenge endpoint
- Replaced with full token pair after MFA challenge

## Configuration

### Environment Variables
```
ENCRYPTION_KEY=32-byte-hex-encoded-key  # Required for AES-256
GOOGLE_CLIENT_ID=your-client-id
GOOGLE_CLIENT_SECRET=your-secret
GOOGLE_REDIRECT_URI=https://your-domain/auth/oauth2/google/callback
GITHUB_CLIENT_ID=your-client-id
GITHUB_CLIENT_SECRET=your-secret
GITHUB_REDIRECT_URI=https://your-domain/auth/oauth2/github/callback
```

## Database Migration

Run the migration:
```bash
migrate -path database -database "postgres://..." up
```

This creates:
- mfa_secrets table
- mfa_backup_codes table
- user_oauth_providers table
- Adds mfa_enabled, mfa_verified_at fields to users table

## Testing

Unit tests provided:
- `backend/internal/application/auth/mfa_usecase_test.go` - MFA logic tests
- `backend/internal/handler/auth/mfa_handler_test.go` - HTTP handler tests (to be created)
- `backend/internal/handler/auth/oauth_handler_test.go` - OAuth handler tests (to be created)

Run tests:
```bash
cd backend
go test ./... -v -cover
```

## API Examples

### Setup MFA
```bash
POST /auth/mfa/setup
Authorization: Bearer <access-token>
Content-Type: application/json

{
  "email": "user@example.com"
}

Response:
{
  "secret": "JBSWY3DPEBLW64TMMQQQ====",
  "qr_code": "base64-encoded-jpeg",
  "backup_codes": [
    "ABC123DEF456",
    "GHI789JKL012",
    ...
  ]
}
```

### Verify MFA
```bash
POST /auth/mfa/verify
Authorization: Bearer <access-token>
Content-Type: application/json

{
  "code": "123456"
}

Response:
{
  "verified": true,
  "message": "MFA activated successfully"
}
```

### Login with MFA Challenge
```bash
POST /auth/mfa/challenge
Authorization: Bearer <mfa-token>
Content-Type: application/json

{
  "code": "123456"  // or backup code
}

Response:
{
  "verified": true,
  "message": "MFA verified successfully"
}
```

### Google OAuth Redirect
```bash
GET /auth/oauth2/google/redirect

Response:
{
  "auth_url": "https://accounts.google.com/o/oauth2/v2/auth?..."
}
```

## Next Steps

1. Implement OAuth2 flow with actual Google/GitHub libraries
2. Add rate limiting to MFA endpoints (5 req/min)
3. Add rate limiting to OAuth callbacks (10 req/min per IP)
4. Update login use case to handle MFA requirement
5. Add WebAuthn/FIDO2 support (future)
6. Add SMS/Email OTP (future)

## References

- TOTP RFC 6238: https://tools.ietf.org/html/rfc6238
- OAuth2 RFC 6749: https://tools.ietf.org/html/rfc6749
- AES-256-GCM: https://csrc.nist.gov/publications/detail/sp/800-38d/final
