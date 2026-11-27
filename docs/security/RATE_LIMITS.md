# Rate limits on the public and credential-bearing routes

Issue #1318. This table is checked by
`backend/internal/handler/rate_limit_table_test.go`, which reads it next to
`backend/cmd/server/main.go`. The test fails when:

- a route registered before the auth gate (`api.Use(middleware.Protected(…))`),
  or any `/auth/` route, is missing from the table;
- a row names a route that main.go no longer registers;
- a row names a limiter that does not reach that route, or a budget that is not
  the one the code applies;
- a `secret` route has no limiter of its own beyond the baseline quotas.

Every route below also answers 429 with a `Retry-After` header when a limiter
refuses it; each limiter family has a test for that (see the end of this file).

## Kinds

| Kind | What the request carries | What the limit has to stop |
|---|---|---|
| `public` | nothing | floods |
| `token` | a credential no one can guess: 256 random bits, a signature, or a secret this caller was just shown | floods; guessing is not a threat |
| `secret` | something a person chose or can guess: a password, a 6-digit code, a recovery code | guessing. A `secret` route must carry a limiter of its own |
| `session` | a signed-in session, nothing else | one tenant starving the others |

## Limiters

| Limiter in main.go | Built by | Keyed by |
|---|---|---|
| `ipQuota` | `middleware.IPRateLimit` | client IP (`c.IP()`, trusted proxies only) |
| `tenantRateLimit` | `middleware.TenantRateLimit` | tenant of the session |
| `authLimits.*` | `middleware.NewAuthLimiters` | client IP, one counter per purpose |
| `vendorPublicRateLimit` | `middleware.RateLimit` | client IP |

`ipQuota` and `tenantRateLimit` budgets are the defaults;
`RATE_LIMIT_IP_PER_MINUTE` and `RATE_LIMIT_TENANT_PER_MINUTE` change them per
install. The `authLimits` budgets are constants in
`backend/internal/middleware/auth_limits.go`.

## Routes

Budgets read `requests/window`.

| Route | Kind | Limiters | Also limited in the handler or use case |
|---|---|---|---|
| `GET /metrics` | public | `ipQuota` 300/1m | |
| `GET /api/v1/status` | public | `ipQuota` 300/1m | |
| `GET /api/v1/health` | public | `ipQuota` 300/1m | |
| `POST /api/v1/auth/login` | secret | `ipQuota` 300/1m; `authLimits.Login` 15/5m | per-account and per-address lockout (`application/auth/login_limits.go`) |
| `POST /api/v1/auth/register` | secret | `ipQuota` 300/1m; `authLimits.Register` 10/15m | duplicate-address attempts (`application/auth/register_duplicates.go`) |
| `POST /api/v1/auth/refresh` | token | `ipQuota` 300/1m | |
| `POST /api/v1/auth/logout` | token | `ipQuota` 300/1m | |
| `POST /api/v1/auth/password/forgot` | secret | `ipQuota` 300/1m; `authLimits.Reset` 15/5m | 3 requests per address per hour (`domain.PasswordResetMaxPerHour`) |
| `POST /api/v1/auth/password/reset` | secret | `ipQuota` 300/1m; `authLimits.Reset` 15/5m | |
| `POST /api/v1/auth/password/check` | secret | `ipQuota` 300/1m; `authLimits.Check` 120/5m | |
| `GET /api/v1/auth/oauth2/login/:provider` | public | `ipQuota` 300/1m | |
| `GET /api/v1/auth/oauth2/callback/:provider` | token | `ipQuota` 300/1m | |
| `GET /api/v1/auth/saml2/login` | public | `ipQuota` 300/1m | |
| `POST /api/v1/auth/saml2/acs` | token | `ipQuota` 300/1m | |
| `POST /api/v1/auth/sso/mfa` | secret | `ipQuota` 300/1m; `authLimits.MFAChallenge` 20/5m | |
| `GET /api/v1/auth/saml2/metadata` | public | `ipQuota` 300/1m | |
| `POST /api/v1/auth/mfa/challenge` | secret | `ipQuota` 300/1m; `authLimits.MFAChallenge` 20/5m | per-challenge and per-account lockout (`application/auth/mfa_limits.go`) |
| `POST /api/v1/auth/mfa/setup` | token | `ipQuota` 300/1m; `tenantRateLimit` 1200/1m | |
| `POST /api/v1/auth/mfa/verify` | token | `ipQuota` 300/1m; `tenantRateLimit` 1200/1m | the code checks a secret `/auth/mfa/setup` just showed this caller; a factor already verified answers 409 and judges no code |
| `POST /api/v1/scanner/agents/register` | token | `ipQuota` 300/1m | |
| `GET /api/v1/scanner/agent/stream` | token | `ipQuota` 300/1m | |
| `POST /api/v1/scanner/agent/push` | token | `ipQuota` 300/1m | |
| `POST /api/v1/scanner/agent/inventory` | token | `ipQuota` 300/1m | |
| `POST /api/v1/scanner/agent/heartbeat` | token | `ipQuota` 300/1m | 12 per agent per minute (`handler/scanner_agent_handler.go`) |
| `GET /api/v1/mitigations/events` | token | `ipQuota` 300/1m | |
| `GET /api/v1/invitations/preview` | token | `ipQuota` 300/1m | |
| `POST /api/v1/invitations/accept` | token | `ipQuota` 300/1m | |
| `GET /api/v1/public/vendor-assessment` | token | `ipQuota` 300/1m; `vendorPublicRateLimit` 60/1m | 300 per token per hour (`handler/public_vendor_assessment_handler.go`) |
| `PUT /api/v1/public/vendor-assessment/answers` | token | `ipQuota` 300/1m; `vendorPublicRateLimit` 60/1m | 300 per token per hour |
| `POST /api/v1/public/vendor-assessment/submit` | token | `ipQuota` 300/1m; `vendorPublicRateLimit` 60/1m | 300 per token per hour, 10 submits per token per hour |
| `POST /api/v1/vulnerabilities/webhook/:source` | token | `ipQuota` 300/1m | |
| `POST /api/v1/auth/mfa/disable` | secret | `ipQuota` 300/1m; `tenantRateLimit` 1200/1m; `authLimits.Reauth` 15/5m | 5 per account per 15 minutes, and the in-session lockout |
| `POST /api/v1/auth/mfa/backup-codes/regenerate` | secret | `ipQuota` 300/1m; `tenantRateLimit` 1200/1m; `authLimits.Reauth` 15/5m | shares the disable budget |
| `GET /api/v1/auth/sessions` | session | `ipQuota` 300/1m; `tenantRateLimit` 1200/1m | |
| `DELETE /api/v1/auth/sessions/others` | session | `ipQuota` 300/1m; `tenantRateLimit` 1200/1m | |
| `POST /api/v1/auth/password/change` | secret | `ipQuota` 300/1m; `tenantRateLimit` 1200/1m; `authLimits.Reauth` 15/5m | |
| `DELETE /api/v1/auth/sessions/:id` | session | `ipQuota` 300/1m; `tenantRateLimit` 1200/1m | |
| `GET /api/v1/auth/organizations` | session | `ipQuota` 300/1m; `tenantRateLimit` 1200/1m | |
| `POST /api/v1/auth/switch-org` | session | `ipQuota` 300/1m; `tenantRateLimit` 1200/1m | |
| `POST /api/v1/auth/pat` | session | `ipQuota` 300/1m; `tenantRateLimit` 1200/1m | |
| `GET /api/v1/auth/pat` | session | `ipQuota` 300/1m; `tenantRateLimit` 1200/1m | |
| `DELETE /api/v1/auth/pat/:id` | session | `ipQuota` 300/1m; `tenantRateLimit` 1200/1m | |
| `GET /api/v1/auth/me` | session | `ipQuota` 300/1m; `tenantRateLimit` 1200/1m | |

## 429 and Retry-After, per limiter family

| Family | Test |
|---|---|
| `ipQuota`, `tenantRateLimit` | `internal/middleware/apiquota_test.go` |
| `authLimits.*`, `vendorPublicRateLimit` (`middleware.RateLimit`) | `internal/middleware/ratelimit_test.go` |
| login lockout | `internal/handler/auth/login_throttle_e2e_test.go` |
| duplicate sign-up attempts | `internal/handler/auth/register_duplicate_e2e_test.go` |
| MFA challenge lockout | `internal/handler/auth/mfa_challenge_e2e_test.go` |
| MFA in-session lockout | `internal/handler/auth/mfa_in_session_lock_e2e_test.go` |
| MFA disable / backup-code budget | `internal/handler/auth/mfa_disable_e2e_test.go`, `mfa_regenerate_backup_codes_e2e_test.go` |
| password reset per address | `internal/handler/auth/forgot_password_limit_test.go` |
| questionnaire per token | `internal/handler/public_vendor_assessment_handler_test.go` |
| scanner heartbeat per agent | `internal/handler/scanner_agent_heartbeat_test.go` |
| every `domain` rate-limit error (`writeAppError`) | `internal/handler/write_app_error_retry_after_test.go` |

## What the table does not see

The test reads `main.go` only, and only `<recv>.<Method>("literal", …)`
registrations. A route registered before the auth gate on a receiver other than
`app` or `api` fails the test, because its prefix cannot be resolved; a route
registered in another file is not seen at all.
