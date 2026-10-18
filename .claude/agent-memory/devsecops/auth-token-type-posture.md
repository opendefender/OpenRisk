---
name: auth-token-type-posture
description: JWT token-type enforcement and MFA flow posture in OpenRisk; MFA_REQUIRED tokens accepted by Protected (live-proven 2026-10-11)
metadata:
  type: project
---

Found 2026-10-11 during the #714 review (PR #898). Proven live on the PR head and still present on origin/master 6999958d:
- `AuthMiddlewareRS256` (internal/middleware/auth.go) and `pkg/auth.ValidateAccessToken` never check `claims.Type`. A password-only MFA_REQUIRED token (5 min) or an MFA_ENROLLMENT token (15 min) therefore passes `Protected` on every route. Only routes with RequirePermission/RequireRole refuse it, because those tokens carry no permissions.
- Proven exploits: `POST /auth/mfa/disable` with password succeeds for non-privileged roles. `POST /auth/pat` with scopes ["*"] works for root too, and gives a PAT with the owner's full permissions. Net effect: full MFA bypass with the password alone.
- At review time nothing tracked it, so it went to the lead as a P0.

Other MFA debts seen in the same review, all pre-existing:
- VerifyMFA ends with `Omit("last_totp_step").Save(stale struct)`. A Setup landing inside its window leaves key K1 verified, codes from the second setup, and the replay mark NULL (reproduced on Postgres). If the row is gone, Save falls back to INSERT.
- `MarkBackupCodeAsUsed` is `WHERE id = ?` with no `used_at IS NULL` and no tenant. Backup codes can be double-spent.
- The `MFA_ENCRYPTION_KEY` fallback is a literal dev key, with no production refusal.
- The GORM logger writes full SQL with values: every statement outside production, errors and slow queries over 200 ms in production. That includes secret_encrypted and bcrypt code hashes.
- `/auth/mfa/setup` and `/auth/mfa/verify` have no per-route limiter, only the global IP quota. `MFARateLimit` exists but is never wired.

**Why:** the per-feature auth reviews kept checking only their own middleware (MFAEnrollmentMiddleware). The generic gate, which every typed token also passes, was never re-checked.
**How to apply:** on any auth or token change, mint each typed token and send it to a protected route plus /auth/pat and /auth/mfa/disable. See [[recurring-defect-areas]].
