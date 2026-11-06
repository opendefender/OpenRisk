# OpenRisk threat model

| | |
|---|---|
| Issue | #1313 (part of #229) |
| Date | 2026-11-06 |
| Base commit | `05b64c8f` (master) |
| Method | STRIDE per component, built by reading the code at the base commit |
| Companion | [`ASVS_GAP_REGISTER.md`](ASVS_GAP_REGISTER.md), where each OWASP ASVS 4.0.3 L2 chapter is mapped to proof or a tracking issue |

## How to read this document

Each threat row names the **mitigation that exists today** as a repository path.
A mitigation is listed only when the code was read at the base commit. When no
mitigation exists, the row says **none** and gives the issue that tracks the
gap.

Further findings were reported privately and are tracked in security advisories under coordinated disclosure. They are not described or located in this document until their fixes are released.

This document describes how things are, not how they are meant to be. Two
labels are used:

- **Proven**: a named automated test covers the mitigation.
- **Code-read**: the mitigation is in the code, but no test pins it.

Not covered by this document: the marketing website, the GitHub organisation,
the hosting provider's own controls, and physical security.

---

## 1. System overview

```
           Internet                          Customer network (on-prem)
  ┌───────────────────────┐                ┌──────────────────────────────┐
  │ Browser (SPA, React)  │                │ openrisk-agent (Go binary)   │
  │ API clients (PAT)     │                │  runs nmap / osqueryi        │
  │ Scanner webhooks      │                │  state file: token + HMAC key│
  │ OAuth2 IdPs           │                └──────────────┬───────────────┘
  └──────────┬────────────┘                               │ TB4 (HTTPS expected,
             │ TB1: TLS at ingress                        │  not enforced)
  ┌──────────▼────────────────────────────────────────────▼───────────────┐
  │ Ingress / nginx  ──►  OpenRisk API (Fiber, Go)                        │
  │                        middleware: recover, logger, helmet, CORS,     │
  │                        IP quota, PAT, Protected (JWT RS256 + CSRF),   │
  │                        tenant quota, MFA policy, audit                │
  │                        background workers, in-process cloud scanners  │
  └──────┬───────────────────────┬───────────────────────────┬────────────┘
         │ TB2                   │ TB2                       │ TB3: outbound
   ┌─────▼──────┐          ┌─────▼─────┐        ┌────────────▼─────────────┐
   │ PostgreSQL │          │  Redis    │        │ Cloud APIs, Jira/SNOW,   │
   │ (all       │          │ (JTI deny │        │ chat tools, NVD/OSV,     │
   │  tenants)  │          │  list,    │        │ HIBP, GitHub/GitLab, AD, │
   └────────────┘          │  limits)  │        │ VMware, k8s, Docker      │
                           └───────────┘        └──────────────────────────┘
  TB5: tenant ↔ tenant (logical, inside the API and the shared database)
  TB6: source repository / CI ↔ shipped artefacts (supply chain)
```

### Trust boundaries

| ID | Boundary | What crosses it | Primary control |
|---|---|---|---|
| TB1 | Internet ↔ API | Browser requests, PAT and API calls, webhooks, OAuth2 callbacks | TLS at the ingress (`helm/openrisk/values.yaml`, `force-ssl-redirect`), `backend/internal/middleware/auth.go` |
| TB2 | API ↔ PostgreSQL / Redis | SQL with tenant data, JTI deny-list, rate-limit counters | Network placement. Database TLS is not enforced (`backend/internal/infrastructure/database/database.go` builds `sslmode=disable` when `DATABASE_URL` is unset) |
| TB3 | API ↔ external services | Tenant-supplied URLs and credentials, outbound fetches | `backend/pkg/netguard/netguard.go` on some paths only (see §7) |
| TB4 | On-prem agent ↔ API | Registration, job stream (SSE), signed result push | RS256 agent tokens, hash-checked against the agent row, and an HMAC push signature (`backend/internal/scanner/agentauth.go`) |
| TB5 | Tenant ↔ tenant | Every query against the shared schema | `tenant_id` filters in the repositories; a CI gate demands an isolation decision for every route (`backend/internal/security/isolation/registry.go`) |
| TB6 | Repository / CI ↔ artefacts | Dependencies, container images, agent binary | `.github/workflows/dependency-gate.yml` (a required check on `master`), `scripts/security/dependency-gate.sh` |
| TB7 | Identity provider ↔ API | OAuth2 authorisation codes, user-info, Azure ID tokens | PKCE and a state cookie (`backend/internal/handler/oauth2_handler.go`, `backend/pkg/oauthpkce/pkce.go`) |

---

## 2. Assets

| Asset | Where it lives | Why it matters |
|---|---|---|
| Tenant GRC data: risks, controls, evidence, incidents, vulnerabilities, assets | PostgreSQL, evidence files under the storage base path (`backend/pkg/storage/local.go`) | A customer's register of unmitigated weaknesses is the most sensitive thing the product holds |
| User credentials | `users.password_hash` (Argon2id, `backend/internal/auth/password_hasher.go`) | Account takeover |
| Session material | Access JWT (15 min), refresh token (30 days, stored as a SHA-256 hash, `backend/internal/auth/token.go`), CSRF cookie | Session hijacking |
| MFA secrets and backup codes | AES-256-GCM TOTP secret, bcrypt backup codes (`backend/internal/application/auth/mfa_usecase.go`) | Bypassing the second factor |
| JWT signing key (RSA private) | File or environment variable, `backend/pkg/auth/rsa.go` | Lets an attacker forge any session for any tenant |
| Encryption keys: `MFA_ENCRYPTION_KEY`, `SCANNER_CREDENTIAL_KEY`, audit hash key | Environment | Decrypting stored secrets, forging audit seals |
| Tenant integration credentials: cloud keys, ticketing tokens, directory bind passwords, notification endpoints | PostgreSQL. Scanner credentials are encrypted (`backend/internal/application/scanner/credentials.go`) | Pivoting into the customer's own infrastructure |
| Agent credentials | Agent state file (`agent/agent.go`, mode 0600): 7-day JWT and HMAC push secret | Injecting forged scan results |
| Audit trail | `audit_events`, `admin_audit_events` (append-only trigger in `migrations/0055_audit_append_only.up.sql`), `auth_audit_logs` | Non-repudiation, which customers rely on for compliance evidence |
| Availability of the service | All of the above | Customers depend on it during audits and incidents |

---

## 3. Entry points

| Entry point | Authentication | Source |
|---|---|---|
| `/api/v1/auth/login`, `/register`, `/password/forgot`, `/password/reset`, `/password/check` | None. Per-purpose rate limits apply | `backend/cmd/server/main.go`, `backend/internal/middleware/auth_limits.go` |
| `/api/v1/auth/refresh` | Refresh cookie, path-scoped | `backend/internal/middleware/session_cookie.go` |
| `/api/v1/auth/mfa/challenge`, `/auth/sso/mfa` | `MFA_REQUIRED` typed token | `backend/internal/middleware/auth.go` (`MFATokenMiddleware`) |
| `/api/v1/auth/mfa/setup`, `/auth/mfa/verify` | Enrolment token or session | `backend/internal/middleware/mfa_enrollment.go` |
| `/api/v1/auth/oauth2/login/:provider`, `/callback/:provider` | None (the flow itself) | `backend/internal/handler/oauth2_handler.go` |
| `/api/v1/auth/saml2/login`, `/acs`, `/metadata` | Disabled: login and ACS always refuse (#866). Metadata is still served | `backend/internal/handler/saml2_handler.go` |
| About 460 routes behind `protected` | JWT RS256 by cookie (with CSRF) or bearer, or a PAT | `backend/internal/middleware/protect.go`, `backend/internal/middleware/pat_middleware.go` |
| `/api/v1/scanner/agents/register`, `/scanner/agent/{stream,push,inventory,heartbeat}` | RS256 agent tokens, hash-checked against the agent row, and an HMAC push signature | `backend/internal/handler/scanner_agent_handler.go` |
| `/api/v1/vulnerabilities/webhook/:source` | Opaque per-integration token | `backend/internal/handler/vuln_webhook_handler.go` |
| `/api/v1/health` | None | `backend/cmd/server/health_wiring.go` |
| `/metrics` (Prometheus, root app) | **None** | `backend/cmd/server/main.go` |
| File uploads: evidence, asset/risk CSV import, avatar, organisation logo | Session | `backend/internal/handler/evidence_handler.go`, `asset_import_handler.go`, `risk_import_handler.go`, `profile_handler.go`, `organization_member_handler.go` |
| On-prem agent | Command-line flags, server job stream | `agent/main.go`, `agent/agent.go` |

---

## 4. API (core request path and tenant isolation)

| STRIDE | Threat | Mitigation today | Status / gap |
|---|---|---|---|
| S | Request forged with a stolen or forged JWT | RS256 only; the signing method is pinned in code (`backend/pkg/auth/jwt.go`). The login MFA token is refused on protected routes (`backend/internal/middleware/auth.go`) | Partial: a foreign signature is refused (`pkg/auth/jwt_test.go`) and the MFA token ratchet is proven (`internal/handler/typed_token_route_ratchet_test.go`); algorithm pinning is code-read. `iss` and `aud` are set but not validated: #1344 |
| S | Client sets `X-Forwarded-For` to dodge the per-IP quota | `EnableTrustedProxyCheck` with `TRUSTED_PROXIES` (`backend/internal/middleware/trustedproxy.go`) | Proven: `trustedproxy_test.go::TestRateLimit_SpoofedForwardedForCannotResetCounter` |
| T | Cross-tenant write (IDOR) on an ID-parameterised route | `tenant_id` filters in the repositories. A CI gate demands an isolation decision for every route (`backend/internal/security/isolation/registry.go`) | Partial. Of 200 route decisions, **45 are still `Pending`**: #1319 |
| T | Mass assignment, e.g. `tenant_id` or role sent in a body | Use cases build entities from typed inputs. The tenant comes from the token (`middleware.GetContext`) | Code-read. No test sweeps every handler: #1316 |
| T | SQL injection through sort or filter parameters | GORM bound parameters. Sort columns are allow-listed (`backend/internal/domain/risk_repository.go`, `vulnerability_repository.go`) | Code-read. No test pins the allow-lists: #1316 |
| R | Mutation without an attributable actor | `auditMutations` and `stampAuditActor` middleware, hash-chained `audit_events`, database trigger blocks UPDATE and DELETE | Proven: `internal/domain/audit_chain_test.go`, `internal/handler/audit_trail_e2e_test.go`, `internal/infrastructure/repository/audit_chain_pg_test.go` (runs when `DATABASE_URL` is set) |
| I | Cross-tenant read (IDOR) and enumeration through counts and lists | As for T above. Collection routes are also in the registry | Partial (45 pending): #1319 |
| I | Internal error text returned to the client | Global handler hides details in production (`backend/internal/middleware/errorhandler.go`). `serverError` helper. A guard test scans for `Status(5xx)` followed by `err.Error()` on the same line | Partial. The guard only matches a literal `Status(5\d\d)`: six handlers that write `fiber.StatusInternalServerError` still return `err.Error()` (`org_deletion_handler.go:99`, `billing_handler.go:119`, `cti_handler.go:128`, `dashboard_handler.go:165`, `multitenancy_org_handler.go:44,179`). About 85 more lines return `err.Error()` at 4xx: #1346 |
| I | Unauthenticated `/metrics` shows runtime and business metrics | none | #1346 |
| I | Signed-in responses cached by a shared cache | `Cache-Control: no-store` by default (`backend/internal/middleware/securityheaders.go`) | Proven: `securityheaders_test.go::TestSecurityHeaders_DefaultsToNoStore`, `securityheaders_cache_test.go` |
| D | Request flood from one IP or one noisy tenant | `IPRateLimit` and `TenantRateLimit` (`backend/internal/middleware/apiquota.go`). Fiber's default 4 MB body limit. 10 s read and write timeouts | Proven: `apiquota_test.go::TestTenantRateLimit_IsolatesNoisyNeighbour`. Route-by-route coverage is not proven: #1318 |
| E | Authenticated user reaches a route without the needed permission | `RequirePermission` / `RequireRole` guards. A ratchet stops the unguarded set from growing | Partial: `internal/handler/authz_matrix_test.go`, `authz_route_coverage_test.go`. 83 routes are allow-listed as unguarded and rely on tenant scoping alone. The matrix does not cover every role and route combination |
| E | Guard leakage through Fiber's `Use` semantics (a guard attached to a group applies to later sibling routes) | Route extraction by AST, plus the ratchet above | Proven: `internal/security/routes/extract_test.go` |

---

## 5. Authentication and session

| STRIDE | Threat | Mitigation today | Status / gap |
|---|---|---|---|
| S | Online password guessing or credential stuffing | Per-purpose IP limits (`backend/internal/middleware/auth_limits.go`). Per-account backoff and lock (`backend/internal/application/auth/login_limits.go`) | Proven: `login_throttle_test.go::TestLogin_PerAccountBackoff`, `auth_limits_test.go` |
| S | Account enumeration through login, register or reset | Uniform answers. An unknown address still pays the hash cost | Proven: `login_throttle_test.go::TestLogin_UnknownAddressCostsAHashComparison`, `password_reset_test.go::TestRequestReset_KnownAndUnknownAddressesAreIndistinguishable`, `register_test.go::TestRegister_TakenUsernameDoesNotRevealTheAddress` |
| S | Weak or breached password | Minimum of 12 runes, zxcvbn score ≥ 3, HIBP k-anonymity check (`backend/pkg/pwpolicy/pwpolicy.go`, `backend/pkg/hibp/hibp.go`) | Proven: `pwpolicy_test.go`. The HIBP check fails open when the corpus is unreachable (by design, tested). A 3-of-4 character-class rule contradicts ASVS 2.1.9: #1345 |
| S | Offline cracking after a database leak | Argon2id m=64 MiB, t=3, p=4, rehash on login | Proven: `password_hasher_test.go`, `login_rehash_test.go` |
| S | Second-factor bypass with the login MFA token | `Protected` accepts only `TokenTypeAccess` (#944). `MFATokenMiddleware` accepts only `MFA_REQUIRED` | Proven: `typed_token_route_ratchet_test.go::TestTypedTokenRoutes_ProtectedRefusesTypedTokensOnEveryRoute`, `protect_test.go` |
| S | TOTP replay or brute force | Last-step replay mark. 5 failures kill the challenge token. Per-user backoff. Backup codes are single-use | Proven: `mfa_replay_test.go`, `mfa_challenge_limits_test.go`, `mfa_backup_replay_test.go` |
| S | Reset-link theft or reuse | 30-minute single-use token. Other links are invalidated. Every session ends on reset | Proven: `password_reset_test.go` (`TestConfirmReset_TokenIsSingleUse`, `TestConfirmReset_ConcurrentUseElectsExactlyOneWinner`, `TestConfirmReset_SetsPasswordAndEndsEverySession`) |
| T | CSRF against cookie-authenticated mutations | Double-submit token (`or_csrf` cookie and `X-CSRF-Token` header) checked in `backend/internal/middleware/auth.go`. `SameSite=Lax` | Proven: `session_auth_test.go::TestVerifyCSRF_BlocksForgedStateChange`. A route-by-route proof is #1314 |
| I | Session token stolen by XSS | Access and refresh cookies are `HttpOnly`, `Secure` outside development, and the refresh cookie is path-scoped to `/api/v1/auth/refresh` | Proven: `session_auth_test.go::TestIssueSessionCookies_Attributes`. The SPA has no CSP of its own: #1314 |
| I | Refresh token theft and replay | Rotation with family revocation on reuse, a 10 s grace window, device binding | Proven: `internal/auth/token_test.go`, `token_pg_test.go::TestRevocation_RacingRotation_Postgres` |
| E | Session lives forever | 15-minute access token. **The refresh family slides by 30 days on every rotation, with no absolute lifetime and no idle limit** | Gap (ASVS 3.3.2 L2 asks for 12 h or 30 min idle): #1344 |
| E | Revoked access token still accepted | JTI deny-list in Redis | Partial. The deny-list check **fails open** when Redis errors (`backend/pkg/auth/jwt.go`), so a logged-out token stays valid for up to 15 minutes during a Redis outage: #1344 |
| R | Authentication events not recorded | `auth_audit_logs`: login, failure reason, refresh reuse, MFA, PAT, reset, org switch. The audited IP cannot be chosen by an untrusted peer | Proven: `internal/auth/audit_ip_test.go` |

---

## 6. SSO (OAuth2 / OIDC, SAML2)

| STRIDE | Threat | Mitigation today | Status / gap |
|---|---|---|---|
| S | Login CSRF or a swapped authorisation code | Per-attempt `state`, bound to a short-lived HttpOnly cookie and single-use. PKCE S256 for every provider | Proven: `oauth2_state_cookie_test.go`, `oauth2_flow_test.go::TestOAuthLogin_SendsPKCEChallengeAndState_AllProviders`, `TestOAuthCallback_StateIsSingleUse` |
| S | Takeover through an unverified IdP e-mail | Accounts link only on a verified e-mail. Password accounts are not auto-linked. No provisioning | Proven: `internal/application/auth/oauth_link_test.go`, `oauth2_password_account_test.go` |
| S | Forged Azure ID token | `tid`, `iss`, `aud` and expiry are checked. The signature is **not** verified (`ParseUnverified`); the token's integrity rests on the direct TLS back-channel from the code exchange (OIDC Core §3.1.3.7) | Proven for the claim checks: `oauth2_azure_identity_test.go`. Acceptable while the token only comes from the token endpoint |
| S | Forged SAML assertion | SAML login and ACS refuse every request (#866) | Proven: `saml2_handler_test.go::TestSAML2ACS_RefusesAWellFormedSuccessResponse`. `/auth/saml2/metadata` still advertises `WantAssertionsSigned="false"`: #1344 |
| T | Open redirect after login | `sanitiseReturnTo` on the server, `safeNextPath` on the client | Proven: `oauth2_flow_test.go::TestSanitiseReturnTo_RejectsOffsiteTargets`, `sso_session_test.go::TestIssueSSOSession_OffSiteReturnToIsIgnored` |
| I | Client secret disclosure | Secrets are held server-side only and never logged | Code-read. A log-scan test is #1317 |
| D | Provider response exhausts memory | User-info reads are capped at 1 MiB with a 10 s timeout (`oauth2_handler.go`) | Code-read |
| E | SSO login skips the MFA requirement | SSO goes through the same `IssueSession` path and the `/auth/sso/mfa` step | Proven: `internal/handler/sso_mfa_test.go` |

---

## 7. Uploads and integrations (outbound connectors, webhooks, CTI)

### 7.1 Uploads

| STRIDE | Threat | Mitigation today | Status / gap |
|---|---|---|---|
| T | Path traversal through the upload filename | Server-generated key `<tenant>/<uuid>-<sanitised>`. Keys that escape the base path are refused (`backend/pkg/storage/local.go`) | Proven: `pkg/storage/local_test.go::TestOpen_RejectsKeyEscapingBasePath`, `TestSave_SanitizesFilename` |
| T | Stored XSS through an uploaded file served inline | Downloads are sent as `attachment` with `application/octet-stream`. `nosniff` comes from helmet | Code-read. The evidence `Content-Disposition` filename is not escaped (a `"` breaks the parameter): #1316 |
| T | Malicious file type in evidence | Avatars and logos are type-sniffed (PNG/JPEG only, `backend/pkg/imageupload`). **Evidence and CSV imports have no type allow-list** | Avatar: proven (`imageupload_test.go`). Others: #1316 |
| I | Cross-tenant evidence download | Get, Delete and Download are tenant-scoped | Proven: `internal/application/evidence/usecases_test.go::TestGetAndDelete_AreTenantScoped` |
| D | Oversized upload | Fiber's default 4 MB body limit applies to every route. There is no per-endpoint limit except on avatars | #1316 |

### 7.2 Outbound connectors (SSRF and stored credentials)

| STRIDE | Threat | Mitigation today | Status / gap |
|---|---|---|---|
| S / I | SSRF to loopback, link-local or cloud metadata through a tenant-supplied URL | `backend/pkg/netguard` resolves, checks at dial time, re-checks redirects and ignores proxy variables. Used by the vulnerability live-pull, ticketing (Jira/ServiceNow), the integration test probe, and the GitHub/GitLab/Docker/Kubernetes/AD/VMware collectors (`backend/internal/scanner/collectors/egress.go`) | Proven: `netguard_test.go`, `collectors/egress_test.go::TestEgress_DefaultIsGuarded` |
| S / I | Outbound clients not yet inventoried | TheHive is operator-configured; M365 Graph uses a fixed host | #1315 inventories every outbound client that takes a tenant-supplied host |
| T | Tenant disables TLS verification for its Kubernetes collector | none: `creds["insecure"] == "true"` sets `rest.TLSClientConfig{Insecure: true}` (`collectors/kubernetes.go:64`) | #1348 |
| I | Tenant cloud credentials disclosed at rest | AES-256-GCM with a random nonce (`backend/pkg/crypto/aes.go`). **The key comes from `DeriveKey`, which pads or truncates instead of running a KDF**, and `SCANNER_CREDENTIAL_KEY` falls back to a public development key (#1200) | #1317, #1200, #1347 |

### 7.3 Inbound webhooks and CTI

| STRIDE | Threat | Mitigation today | Status / gap |
|---|---|---|---|
| T | Malformed findings payload | Typed parse with refusal | Proven: `vuln_webhook_handler_test.go::TestParseWebhookFindings_Invalid` |
| S / T | Poisoned CTI feed | Feeds come from fixed public endpoints (NVD in `backend/pkg/cti/client.go`, OSV in `internal/infrastructure/osv/client.go`) over HTTPS | Code-read. No tenant-supplied feed URL exists at the base commit |
| D | Webhook flood | Global IP quota | #1318 |

---

## 8. On-prem agent (`agent/`)

The agent is a Go binary installed on a customer host. It registers with a 24 h
token embedded in its download. It then holds a 7-day JWT and an HMAC
secret, receives jobs over SSE, runs `nmap` and `osqueryi`, and pushes signed
results. It reports available updates but never replaces itself
(`agent/update.go`).

| STRIDE | Threat | Mitigation today | Status / gap |
|---|---|---|---|
| S | Forged result push into a tenant | Agent token hash-checked against the agent row, scope check, HMAC-SHA256 over the body with a constant-time compare (`backend/internal/scanner/agentauth.go`, `backend/internal/handler/scanner_agent_handler.go`). A job can be claimed only by its own agent and provider | Proven: `internal/scanner/scanner_test.go::TestHMACPushSignature`, `internal/application/scanner/push_results_test.go` |
| I | Agent credentials read from disk | The state file is written with mode 0600 in a 0700 directory (`agent/agent.go`) | Code-read |
| I | Credentials sent in clear | none. `--server` defaults to `http://localhost:8080` and any `http://` URL is accepted | #1348 |
| T | Tampered agent binary | Updates are only reported, never installed. **A prebuilt `agent/openrisk-agent` binary is committed to git**, and the `agent/` module is not covered by the dependency gate | #1318 |
| R | Agent actions not attributable | Jobs, pushes and heartbeats are recorded per agent ID | Code-read |
| D | Server sends a scan job that floods the customer network | /24 (IPv4) and /120 (IPv6) scope limits, scan timeout | Proven: `internal/scanner/scanner_test.go::TestValidateTarget` |

---

## 9. Supply chain and configuration

| STRIDE | Threat | Mitigation today | Status / gap |
|---|---|---|---|
| T | Vulnerable dependency ships | Trivy (backend modules, frontend production dependencies, images) and govulncheck, gated on HIGH and CRITICAL. Exceptions need an owner and expire within 90 days. "Dependency gate" is a required check on `master` | Proven: `backend/pkg/vulngate/vulngate_test.go`. The `agent/` module is not scanned: #1318 |
| T | Unsigned release artefacts | none. No image signing or provenance in `.github/workflows/release.yml` | #1350 |
| I | Secrets committed to the repository | TruffleHog "Secret Detection" (required check) | Code-read (CI) |
| I | Production boots with a development key | Refused for `MFA_ENCRYPTION_KEY` and the audit hash key | Proven: `cmd/server/mfa_key_wiring_test.go`, `audit_hash_key_wiring_test.go`. `SCANNER_CREDENTIAL_KEY`: #1200 / #1317 |
| I | Secrets in logs | Production GORM logger at Warn level (no statement trace) | Partial. Errors and slow queries (> 200 ms) are still logged **with bound values** (`internal/infrastructure/database/database.go`): #1317 |

---

## 10. Findings raised by this review

These came up while building the model. Each one goes into an issue, and none
are fixed here.

Further findings were reported privately and are tracked in security advisories under coordinated disclosure. They are not described or located in this document until their fixes are released.

| Ref | Severity | Finding | Issue |
|---|---|---|---|
| TM-1 | MEDIUM | Refresh family slides with no absolute or idle lifetime. The JTI deny-list fails open. `iss` and `aud` are not validated | #1344 |
| TM-2 | MEDIUM | `DeriveKey` pads or truncates the key, and its comment claims HKDF | #1347 |
| TM-3 | MEDIUM | Six 5xx responses echo `err.Error()` past the guard test. `/metrics` is unauthenticated | #1346 |
| TM-4 | LOW | Database TLS not enforced. Tenant can disable Kubernetes TLS checks. Agent accepts `http://` | #1348 |
| TM-5 | LOW | The committed agent binary, and the `agent/` module outside the dependency gate | #1318 |

## 11. Limits of this model

- Everything here comes from reading the code at `05b64c8f`. No live
  penetration test was run for this document. #1319 owns DAST and the live
  cross-tenant suite.
- The frontend was reviewed only for token storage, URL sinks and the CSP
  question. A full client-side review is part of #1316.
- Kubernetes, Helm and ingress hardening (network policies, pod security) is
  out of scope except where the API depends on it.
