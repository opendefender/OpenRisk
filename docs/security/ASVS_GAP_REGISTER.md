# OWASP ASVS gap register

| | |
|---|---|
| Standard | OWASP Application Security Verification Standard **4.0.3**, Level **2** |
| Scope | OpenRisk backend (`backend/`), SPA (`frontend/`), on-prem agent (`agent/`), CI and release (`.github/`, `scripts/security/`) |
| Date | 2026-11-06 |
| Base commit | `05b64c8f` |
| Issue | #1313 (part of #229) |
| Threat model | [`THREAT_MODEL.md`](THREAT_MODEL.md) |

## Rules

- **met**: every applicable L2 requirement in the chapter was checked in the
  code, and the Proof column names at least one test that pins it. A chapter
  with even one unproven or failing L2 requirement is not `met`.
- **partial**: some L2 requirements are proven, while others are unproven,
  missing or failing. The detail section below separates the two.
- **gap**: the chapter's core requirements are missing.
- Every `partial` or `gap` row links the issue that closes it.
- Further findings were reported privately and are tracked in security advisories under coordinated disclosure. They are not described or located in this document until their fixes are released.
- The Proof column lists the strongest existing evidence, including for
  `partial` rows. The check script verifies that each path exists and, for a
  `path::Name` entry, that the file contains `Name`.

Re-run the check:

```
node scripts/security/check-asvs-register.mjs
```

It runs on every pull request ("ASVS Register Proofs" job in
`.github/workflows/security.yml`). Deleting or renaming a proof file breaks the
check.

## Register

| Chapter | Title | Status | Proof | Issue | Notes |
|---|---|---|---|---|---|
| V1 | Architecture, Design and Threat Modeling | partial | `docs/security/THREAT_MODEL.md`<br>`backend/internal/security/isolation/coverage_test.go::TestEveryParameterisedRouteHasAnIsolationDecision`<br>`backend/internal/handler/authz_route_coverage_test.go::TestProtectedRoutes_UnguardedSetHasNotGrown` | #1349, #1319 | Threat model and CI-gated isolation and authz ratchets exist. No data classification or protection levels (1.8). |
| V2 | Authentication | partial | `backend/internal/auth/password_hasher_test.go::TestArgon2id_UsesConfiguredParameters`<br>`backend/pkg/pwpolicy/pwpolicy_test.go::TestAssess_RejectsBreachedPassword`<br>`backend/internal/application/auth/login_throttle_test.go::TestLogin_PerAccountBackoff`<br>`backend/internal/application/auth/mfa_replay_test.go::TestChallengeMFA_ReplayedCodeIsRefused`<br>`backend/internal/application/auth/password_reset_test.go::TestConfirmReset_TokenIsSingleUse` | #1345, #1344 | Argon2id, breach check, throttling, TOTP and reset are proven. A 3-of-4 composition rule conflicts with 2.1.9. |
| V3 | Session Management | partial | `backend/internal/middleware/session_auth_test.go::TestIssueSessionCookies_Attributes`<br>`backend/internal/auth/token_test.go::TestRefresh_Reuse_RevokesFamily`<br>`backend/internal/application/auth/logout_test.go::TestLogout_EndsTheSessionTheTokenNames`<br>`backend/internal/handler/typed_token_route_ratchet_test.go::TestTypedTokenRoutes_ProtectedRefusesTypedTokensOnEveryRoute` | #1344, #1314 | Cookies, rotation, reuse detection and logout are proven. The 30-day refresh slides with no absolute or idle limit (3.3.2). The revocation check fails open on a Redis error. |
| V4 | Access Control | partial | `backend/internal/handler/authz_matrix_test.go::TestAuthzMatrix_ForeignTenantAdminPassesTheGuardButIsScopedOut`<br>`backend/internal/security/isolation/coverage_test.go::TestPendingGapsAreVisible`<br>`backend/internal/application/evidence/usecases_test.go::TestGetAndDelete_AreTenantScoped` | #1319 | 45 of 200 route isolation decisions are still Pending. 83 routes have no permission guard. |
| V5 | Validation, Sanitization and Encoding | partial | `backend/pkg/validation/validator_test.go::TestHumanizeErrors_NoRawValidatorText`<br>`frontend/src/shared/safeUrl.test.ts` | #1316, #1315 | No sweep of input validation or output encoding. Sort allow-lists are not tested. SSRF is in V12. |
| V6 | Stored Cryptography | partial | `backend/internal/auth/password_hasher_test.go::TestArgon2id_SaltIsRandomPerHash`<br>`backend/cmd/server/mfa_key_wiring_test.go::TestMFAEncryptionKey_ProductionRefusesMissingOrPublicKey`<br>`backend/internal/infrastructure/authmfa/rekey_test.go::TestRekeySecrets_MovesDevKeySecretsToTheRealKey`<br>`backend/pkg/crypto/aes.go` | #1317, #1347 | AES-256-GCM with a random nonce. DeriveKey pads or truncates instead of running a KDF. Scanner key falls back to a public development key (1200). pkg crypto has no unit test of its own. |
| V7 | Error Handling and Logging | partial | `backend/internal/middleware/errorhandler_test.go::TestErrorHandler_ProductionHidesInternals`<br>`backend/internal/domain/audit_chain_test.go::TestVerifyAuditChain_EditedContentIsDetected`<br>`backend/internal/infrastructure/repository/audit_chain_pg_test.go::TestAuditChain_Postgres`<br>`backend/internal/infrastructure/database/logger_test.go::TestGormLogger_ProductionDoesNotTraceStatements` | #1317, #1346 | Hash-chained, append-only audit trail is proven. Six 5xx handlers still echo raw errors past the guard regex. Production GORM logs keep bound values on errors and slow queries. |
| V8 | Data Protection | partial | `backend/internal/middleware/securityheaders_test.go::TestSecurityHeaders_DefaultsToNoStore`<br>`backend/internal/middleware/securityheaders_cache_test.go::TestSecurityHeaders_NoStoreOnCacheHit` | #1349, #1346 | No-store caching is proven. No data inventory or classification. Metrics endpoint unauthenticated. |
| V9 | Communication | partial | `backend/internal/middleware/securityheaders_test.go::TestSecurityHeaders_EmitsHSTSOverTLS`<br>`backend/pkg/netguard/netguard_test.go::TestClient_IgnoresEnvironmentProxy`<br>`helm/openrisk/values.yaml` | #1348, #1314 | TLS terminates at the ingress with a forced redirect. Database TLS not enforced. A tenant can set insecure Kubernetes TLS. The agent accepts a plain-HTTP server URL. |
| V10 | Malicious Code | partial | `backend/pkg/vulngate/vulngate_test.go::TestValidateExceptions_Expired`<br>`scripts/security/dependency-gate.sh`<br>`.github/workflows/dependency-gate.yml` | #1318, #1350 | Dependency gate is a required check. The agent module is not scanned and a prebuilt agent binary is committed. Release images are not signed. |
| V11 | Business Logic | partial | `backend/internal/middleware/apiquota_test.go::TestTenantRateLimit_IsolatesNoisyNeighbour`<br>`backend/internal/middleware/ratelimit_test.go::TestRateLimit_RefusalCarriesRetryAfterAndCode`<br>`backend/internal/application/auth/password_reset_test.go::TestConfirmReset_ConcurrentUseElectsExactlyOneWinner`<br>`backend/internal/application/scanner/push_results_test.go::TestPushResults_ClaimLostToAnotherAgent` | #1318 | Anti-automation and race safety are proven on auth and the agent push. Rate-limit coverage per sensitive route is not proven. |
| V12 | Files and Resources | partial | `backend/pkg/imageupload/imageupload_test.go::TestRead_RefusesSVGGIFAndScript`<br>`backend/pkg/storage/local_test.go::TestOpen_RejectsKeyEscapingBasePath`<br>`backend/pkg/netguard/netguard_test.go::TestClient_RedirectIsRevalidated`<br>`backend/internal/scanner/collectors/egress_test.go::TestEgress_DefaultIsGuarded` | #1316, #1315 | Avatar sniffing, path containment and netguard are proven. Evidence and CSV imports have no type allow-list. Not every outbound client is proven to use netguard. |
| V13 | API and Web Service | partial | `backend/internal/security/routes/openapi_coverage_test.go::TestOpenAPICoverage_EveryRouteIsDocumented`<br>`backend/internal/middleware/session_auth_test.go::TestVerifyCSRF_BlocksForgedStateChange`<br>`backend/internal/scanner/scanner_test.go::TestHMACPushSignature` | #1316, #1314 | Every route documented in OpenAPI. CSRF proven on the middleware but not route by route. Content-type and schema enforcement not swept. |
| V14 | Configuration | partial | `backend/internal/middleware/securityheaders_test.go::TestSecurityHeaders_EmitsCSP`<br>`backend/internal/middleware/errorhandler_test.go::TestIsProductionEnv_FailsClosed`<br>`backend/internal/middleware/trustedproxy_test.go::TestRateLimit_SpoofedForwardedForCannotResetCounter`<br>`docs/security/REPRODUCIBLE_BUILDS.md` | #1314, #1318, #1317 | Strict API CSP and production fail-closed are proven. The SPA is served without a CSP or frame protection by the compose nginx. CORS has no test. Not every production secret is refused when missing. |

No chapter is `met` at the base commit. Many individual requirements are met
and proven, and they are listed below. Each chapter still has at least one L2
requirement that is unproven or failing, so the chapter is rated `partial`.

---

## Requirement detail

The marks below are per requirement:

- ✅ **proven**: a named test pins the claim. For 1.1.2, the named document is
  the deliverable.
- ⚠️ **code-read or incomplete**: the control is present in the code but no
  test pins it, or it is only partly in place.
- ❌ **missing or failing**.

Each claim is no wider than its named test. These lines are not checked by the
script, but every path was read at the base commit.

### V1 Architecture, Design and Threat Modeling

- ✅ 1.1.2 Threat modelling: `docs/security/THREAT_MODEL.md` (this issue).
- ✅ 1.4.1 Every parameterised route needs a tenant-isolation decision in CI
  (`isolation/coverage_test.go::TestEveryParameterisedRouteHasAnIsolationDecision`),
  and the set of protected routes without a permission guard cannot grow
  (`authz_route_coverage_test.go::TestProtectedRoutes_UnguardedSetHasNotGrown`).
- ⚠️ 1.2.x Authentication architecture: not verified end to end as a whole;
  see V2–V4.
- ❌ 1.8.1 / 1.8.2 Data classified into protection levels: no inventory exists.
  #1349.
- ⚠️ 1.14.x Segregation: in-process cloud collectors run inside the API
  process with tenant credentials. This is shown in the threat model §1.

### V2 Authentication

- ✅ 2.1.1 At least 12 characters (`pwpolicy_test.go::TestMinimums_MatchTheDecidedPolicy`).
- ✅ 2.1.7 Breached-password check through HIBP k-anonymity
  (`pwpolicy_test.go::TestAssess_RejectsBreachedPassword`). It fails open by
  design when the corpus is unreachable
  (`TestAssess_FailsOpenWhenBreachCorpusUnreachable`).
- ✅ 2.1.8 Strength assessment with actionable advice, served by
  `/auth/password/check`
  (`pwpolicy_test.go::TestAssess_RejectsWeakScoreWithActionableAdvice`).
- ❌ 2.1.9 "No composition rules": `backend/pkg/pwpolicy/pwpolicy.go` requires
  3 of 4 character classes (`requiredClasses = 3`). Either remove the rule or
  record a deliberate deviation in `docs/DECISIONS.md`. #1345.
- ✅ 2.2.1 Anti-automation (`login_throttle_test.go::TestLogin_PerAccountBackoff`,
  `auth_limits_test.go`).
- ✅ 2.4.1 / 2.4.4 Argon2id m=64 MiB, t=3, p=4
  (`password_hasher_test.go::TestArgon2id_UsesConfiguredParameters`).
- ✅ 2.5.x Recovery: single-use 30-minute reset, other links invalidated,
  sessions ended (`password_reset_test.go`).
- ✅ 2.8.x TOTP replay and brute force (`mfa_replay_test.go`,
  `mfa_challenge_limits_test.go`). Single-use backup codes
  (`mfa_backup_replay_test.go`).
- ✅ 2.2.x The login MFA token cannot be used on protected routes
  (`typed_token_route_ratchet_test.go`).
- ⚠️ SAML2 is disabled (#866). The ASVS does not require it, but the metadata
  endpoint still advertises unsigned assertions. #1344.

### V3 Session Management

- ✅ 3.2.x New session token at login, random 32-byte CSRF token
  (`session_auth_test.go::TestNewCSRFToken_IsRandom`).
- ✅ 3.3.1 Logout invalidates the session (`logout_test.go`).
- ✅ 3.3.3 Option to end other sessions
  (`sessions_test.go::TestRevokeOtherSessions_Success`).
- ✅ 3.3.4 Sessions listed (`sessions_test.go::TestListSessions_Success`).
- ✅ 3.4.1–3.4.3 / 3.4.5 Cookies: Secure outside development, HttpOnly,
  SameSite=Lax, refresh cookie path-scoped
  (`session_auth_test.go::TestIssueSessionCookies_Attributes`). The `__Host-`
  prefix (3.4.4) is not used.
- ✅ 3.5.x Token-based sessions: a foreign signature is refused, refresh
  rotation with reuse detection (`pkg/auth/jwt_test.go::TestValidateAccessToken_WrongKey`,
  `internal/auth/token_test.go::TestRefresh_Reuse_RevokesFamily`).
- ✅ 3.7.1 MFA disable and password change refuse a wrong or missing current
  password (`mfa_disable_test.go::TestDisableMFA_Unauthorized`,
  `change_password_test.go::TestChangePassword_Unauthorized`).
- ⚠️ 3.5.x Pinning the signing method to RS256 is code-read
  (`backend/pkg/auth/jwt.go`); no test sends an HS256 or `none` token.
- ⚠️ 3.7.1 Both routes carry the `Reauth` limiter: code-read
  (`backend/cmd/server/main.go`). The limiter budgets are pinned by
  `auth_limits_test.go::TestAuthRateLimit_ProductionBudgetsUnchanged`.
- ❌ 3.3.2 (L2: re-authenticate within 12 h, or after 30 min idle): the refresh
  token family slides by 30 days on every rotation
  (`backend/internal/auth/token.go`, `RefreshTokenTTL`), with no absolute cap.
  #1344.
- ⚠️ 3.3.1 Revocation: the JTI deny-list check fails open on a Redis error
  (`backend/pkg/auth/jwt.go`). `iss` and `aud` are set but not validated.
  #1344.
- ⚠️ 3.5.3 CSRF route by route, and the SPA's CSP: #1314.

### V4 Access Control

- ✅ 4.1.1 Server-side enforcement (`authz_matrix_test.go`).
- ⚠️ 4.2.1 IDOR: 119 routes are covered, 45 are still `Pending` in
  `backend/internal/security/isolation/registry.go`. #1319.
- ⚠️ 4.1.3 83 protected routes have no permission guard and rely on tenant
  scoping alone (allow-list in `authz_route_coverage_test.go`).
- ✅ 4.3.1 MFA policy for administrative roles (`mfa_policy_guard_test.go`,
  `internal/application/auth/mfa_policy_usecases_test.go`).

### V5 Validation, Sanitization and Encoding

- ✅ 5.1.x Validator messages are humanised and never echo raw validator text
  (`pkg/validation/validator_test.go`).
- ✅ 5.3.x URL sinks in the SPA are allow-listed
  (`frontend/src/shared/safeUrl.test.ts`).
- ⚠️ 5.3.3 React escapes text by default, and no `dangerouslySetInnerHTML` is
  used in `frontend/src` (grep at the base commit, no test). #1316.
- ⚠️ 5.3.4 SQL: GORM bound parameters. Sort columns are allow-listed in
  `domain/risk_repository.go` and `domain/vulnerability_repository.go`, but
  no test pins them. #1316.
- ⚠️ 5.2.6 SSRF: see V12.6.1 and #1315.

### V6 Stored Cryptography

- ⚠️ 6.2.1 / 6.2.5 AES-256-GCM with a random 96-bit nonce
  (`backend/pkg/crypto/aes.go`), used for MFA and scanner secrets. Code-read:
  `pkg/crypto` has no unit test of its own. #1347.
- ✅ 6.4.1 / 6.4.2 MFA key: production refuses a missing or public key, and
  secrets are re-keyed from the development key at boot
  (`cmd/server/mfa_key_wiring_test.go`, `authmfa/rekey_test.go`).
- ❌ 6.2.x `crypto.DeriveKey` pads short keys with predictable bytes and
  truncates long ones. Its comment says HKDF, it ignores its salt, and a short
  operator key is accepted in production
  (`TestMFAEncryptionKey_ProductionKeepsAShortExistingKey`). #1347.
- ❌ 6.4.x `SCANNER_CREDENTIAL_KEY` falls back to a public development key in
  production (#1200). #1317.
- ⚠️ 6.3.x Random values come from `crypto/rand` (grep, no test). `math/rand` appears only in
  the Monte-Carlo simulator (`pkg/crq/montecarlo.go`). `md5` and `sha1` appear
  only in a cache key and the HIBP prefix, both non-security uses.

### V7 Error Handling and Logging

- ✅ 7.4.1 Generic message with a correlation ID in production
  (`errorhandler_test.go`, `server_error_test.go`).
- ❌ 7.4.1 Six handlers return `err.Error()` with
  `fiber.StatusInternalServerError`, which the guard regex
  `Status\(5\d\d\)` does not match. About 85 handler lines echo `err.Error()` at
  4xx. #1346.
- ✅ 7.2.1 (partial scope) The IP recorded on an authentication event cannot
  be chosen by an untrusted peer
  (`audit_ip_test.go::TestLogFiber_UntrustedPeerCannotChooseTheAuditedIP`).
- ⚠️ 7.2.1 / 7.2.2 Authentication events are written to `auth_audit_logs`.
  Code-read: no test asserts which events are recorded. #1317.
- ✅ 7.3.3 Audit-log integrity: a hash chain detects edits and deletions
  (`audit_chain_test.go::TestVerifyAuditChain_EditedContentIsDetected`), and
  the database trigger refuses UPDATE
  (`audit_chain_pg_test.go::TestAuditChain_Postgres`, which runs only when
  `DATABASE_URL` is set).
- ⚠️ 7.1.1 / 7.1.2 No credentials in logs: the production GORM logger still
  prints full SQL with values on errors and slow queries
  (`internal/infrastructure/database/database.go`). Nothing proves that the
  auth and integration packages never log a secret. #1317.

### V8 Data Protection

- ✅ 8.2.1 Anti-caching headers on signed-in responses
  (`securityheaders_test.go`, `securityheaders_cache_test.go`), also checked by
  the DAST job (`.github/workflows/dast.yml`).
- ✅ 8.2.2 Session tokens are issued as HttpOnly cookies
  (`session_auth_test.go::TestIssueSessionCookies_Attributes`). That the SPA
  writes no token to browser storage is code-read (audit F-14).
- ❌ 8.1.x / 8.3.4 Sensitive-data inventory and protection levels. #1349.
- ⚠️ 8.1.x `/metrics` serves Prometheus metrics without authentication.
  #1346.

### V9 Communication

- ✅ 9.1.1 HSTS is sent over TLS only
  (`securityheaders_test.go::TestSecurityHeaders_EmitsHSTSOverTLS`), one year,
  `includeSubDomains`, no `preload`.
- ⚠️ 9.1.1 TLS for clients: the ingress `force-ssl-redirect` is set in
  `helm/openrisk/values.yaml` (configuration, no test).
- ⚠️ 9.2.1 Outbound HTTP through netguard keeps the default Go certificate
  verification, and `InsecureSkipVerify` does not appear in `backend/` (grep,
  no test).
- ❌ 9.2.1 The Kubernetes collector accepts a tenant flag `insecure=true`
  (`backend/internal/scanner/collectors/kubernetes.go`). #1348.
- ❌ 9.2.2 / 9.2.3 Database and Redis TLS are not enforced. The keyword DSN is
  hard-coded to `sslmode=disable`. The agent accepts an `http://` server
  URL. #1348.

### V10 Malicious Code

- ✅ 14.2.1 Dependency-gate exceptions need an owner and expire, and an
  expired one fails (`vulngate_test.go::TestValidateExceptions_Expired`). The
  gate (Trivy and govulncheck, HIGH and CRITICAL block) is a required status
  check on `master`.
- ⚠️ 10.3.1 The agent's update check only notifies (`agent/update.go`), and it
  never installs code. Code-read: `agent/` has no update test. #1318.
- ❌ 10.3.2 Release images and the agent binary are unsigned, with no
  provenance. #1350.
- ❌ 10.3.x A prebuilt `agent/openrisk-agent` is committed to git, and the
  `agent/` module is outside the dependency gate. #1318.

### V11 Business Logic

- ✅ 11.1.2 / 11.1.4 Anti-automation on auth flows and per-tenant quotas
  (`apiquota_test.go`, `ratelimit_test.go`).
- ✅ 11.1.6 Race conditions on single-use secrets and job claims
  (`password_reset_test.go::TestConfirmReset_ConcurrentUseElectsExactlyOneWinner`,
  `push_results_test.go::TestPushResults_ClaimLostToAnotherAgent`,
  `token_test.go::TestRefresh_ConcurrentRotation_WithinGraceAllSucceed`).
- ⚠️ 11.1.4 No table proves that every sensitive route has a limit. #1318.
- ⚠️ 11.1.5 Business-limit validation on bulk updates (impact and
  probability ranges) is not enforced in the bulk path. Recorded in earlier
  reviews; #1316.

### V12 Files and Resources

- ✅ 12.1.1 (partial scope) Avatars and logos refuse oversize files
  (`imageupload_test.go::TestRead_RefusesOversizeAndEmpty`).
- ⚠️ 12.1.1 Fiber's default 4 MB body limit applies to every other route.
  Code-read, no test. #1316.
- ✅ 12.3.1 / 12.3.2 Path containment with a server-generated stored name
  (`local_test.go::TestOpen_RejectsKeyEscapingBasePath`,
  `local_test.go::TestSave_SanitizesFilename`).
- ⚠️ 12.4.1 Files are stored outside any web root, under the storage base
  path. Code-read, no test.
- ⚠️ 12.2.1 Content checked by sniffing: avatar and logo only. Evidence and
  CSV imports are not checked. #1316.
- ⚠️ 12.5.2 Downloads are `attachment` and `nosniff`, but the evidence
  filename is not escaped in `Content-Disposition`. #1316.
- ⚠️ 12.6.1 SSRF: netguard is proven, and the collectors routed through
  `egress.go` are guarded (`egress_test.go::TestEgress_DefaultIsGuarded`). #1315 inventories every remaining outbound
  client.

### V13 API and Web Service

- ✅ 13.1.x Every route documented in OpenAPI, with a shrinking baseline
  (`openapi_coverage_test.go`).
- ✅ 13.2.3 CSRF defence for cookie-authenticated REST calls
  (`session_auth_test.go::TestVerifyCSRF_BlocksForgedStateChange`).
- ✅ 13.2.x Machine-to-machine integrity: HMAC-signed agent pushes
  (`scanner_test.go::TestHMACPushSignature`).
- ⚠️ 13.2.5 / 13.2.6 Content-Type and schema validation not swept route by
  route. #1316.

### V14 Configuration

- ✅ 14.3.2 No debug output in production. `APP_ENV` fails closed
  (`errorhandler_test.go::TestIsProductionEnv_FailsClosed`).
- ✅ 14.4.3 / 14.4.5 / 14.4.7 API responses: strict CSP
  (`default-src 'none'; frame-ancestors 'none'`), `nosniff`, framing denied,
  HSTS (`securityheaders_test.go`).
- ✅ 14.5.x Trusted-proxy handling of `X-Forwarded-For`
  (`trustedproxy_test.go`).
- ❌ 14.4.x The SPA document has no CSP, no `X-Frame-Options` and no
  `Referrer-Policy` when served by the compose nginx (`frontend/nginx.conf`), and
  `index.html` has no meta CSP. #1314.
- ⚠️ 14.5.3 CORS: an explicit allow-list with credentials
  (`backend/cmd/server/main.go`), with no test. #1314.
- ⚠️ 14.1.x Build: `docs/security/REPRODUCIBLE_BUILDS.md` describes the
  process. Production refusal of every missing secret is #1317.
