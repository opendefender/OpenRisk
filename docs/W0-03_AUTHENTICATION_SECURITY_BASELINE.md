# W0-03 — Authentication Security Baseline

> Scope: login, registration, MFA, OAuth2, SAML2, access tokens, refresh-token
> rotation, revocation, logout, sessions, recovery, organization/tenant
> switching, and the authorization tied to them.
>
> This document records the **actual, verified** state of the OpenRisk identity
> stack after the W0-03 hardening, distinguishing what is implemented and proven
> from what remains. Every "PASS" is backed by a test or a runtime observation in
> the [Live Proof Record](#live-proof-record); nothing here is asserted without
> evidence.

## Executive Summary

OpenRisk already had a substantial, modern identity stack (branch history
"auth-hardening-7layers"): RS256 access tokens minted and validated by a single
`pkg/auth` implementation, DB-backed refresh tokens with single-use rotation, an
MFA challenge/enrollment flow, HttpOnly session cookies with CSRF double-submit,
personal access tokens, an SSO session minter shared by OAuth2/SAML, and an
`auth_audit_logs` trail. Reconnaissance confirmed those work.

W0-03 closes the security gaps that remained in the **token / session / tenant
boundary** — the highest-value cluster — and removes one façade:

1. **Refresh-token reuse detection + family revocation.** Rotation was single-use
   by hard delete, but a replayed rotated token was indistinguishable from an
   unknown one, and a concurrent double-rotation could mint two valid pairs. Now
   every rotation shares a `family_id`; a spent token is marked `rotated_at`
   instead of deleted; replaying it (or losing the atomic rotation race) revokes
   the **entire family** and is audited as `refresh_reuse` (RFC 9700 §4.14.2).
2. **Organization switching, server-authorized.** There was **no reachable
   switch endpoint** — the only implementation minted HS256 tokens the RS256
   middleware rejects and was never wired. Added `GET /auth/organizations` and
   `POST /auth/switch-org` on the RS256 stack, which re-check active membership
   of the target org on the server before minting an org-scoped session.
3. **Org context preserved across refresh.** Refresh re-resolved claims for the
   user's **default** org, silently resetting a switched session every 15 min.
   Refresh now resolves for the session's **own** org.
4. **Active-membership enforcement.** A user removed from an org (membership
   `is_active=false`) kept a working password/session for it. Login and the token
   resolver now reject deactivated memberships.
5. **`/auth/me` façade removed.** It returned a hardcoded `user@example.com`; it
   now returns the real authenticated user (secrets are `json:"-"`).

**Critical findings:** 1 × P1 (reuse detection/family revocation — fixed), 3 × P2
(org-context reset, TOCTOU on rotation, active-membership gate — all fixed), 1 ×
P3 (`/auth/me` façade — fixed). No cross-tenant escape, MFA bypass, or credential
leak was found in the reachable surface.

## Current Authentication Architecture

Two layers coexist; only the **modern** one is wired into `cmd/server/main.go`:

- **Modern (authoritative):** `pkg/auth` (RS256 keys, JWT mint/validate) +
  `internal/auth.TokenManager` (DB refresh tokens, rotation, resolver) +
  `internal/application/auth/*` (use cases) + `internal/handler/auth/*`
  (handlers) + `internal/middleware` (Protected, PAT, MFA-token, session-cookie).
- **Legacy (dead):** `internal/service/multitenancy_auth_service.go`
  (HS256, stateless 7-day refresh, `SelectOrganization`) — **not wired**; its
  HS256 tokens would be rejected by the RS256 middleware anyway. Left in place
  (out of scope to delete) but documented here as non-functional so no one
  mistakes it for the live path.

Single signing authority: the same `rsaKeys` is handed to both
`middleware.Protected` and the `TokenManager`, so one implementation signs and
validates every token.

## Registration

`POST /auth/register` → `RegisterUseCase`. Server-side validation, email
normalization, uniqueness check (duplicate → `409`), Argon2id hashing, user +
organization + membership creation. **Response carries no secrets** — no
password, hash, or token; no session is issued (the caller then logs in). Verified
live: registration succeeded and a duplicate returned "user already exists"; the
JSON contained no `password` field.

## Login

`POST /auth/login` → `LoginUseCase`. Checks `user.IsActive`, verifies Argon2id,
resolves the default org and membership, and **now rejects a deactivated
membership**. Generic error messages ("invalid credentials", "account is
disabled"). Rate-limited by `authRateLimit` middleware. On success issues the
RS256 pair + HttpOnly cookies + CSRF token; audits `login`.

Two non-session outcomes:
- `mfa_required` + `mfa_token` (MFA_REQUIRED, 5-min TTL) when a verified secret
  exists — no session until the challenge succeeds.
- `mfa_enrollment_required` + `mfa_token` (MFA_ENROLLMENT, 15-min TTL) when the
  role mandates MFA (admin/root) and none is enrolled.

## Session Model

A **row in `refresh_tokens` IS a session** (device fingerprint, IP, UA recorded).
Access is a 15-min RS256 JWT; the refresh token is a 30-day opaque secret stored
as a SHA-256 hash. Browser clients hold the pair in **HttpOnly cookies**; the
access token also lives in memory for split-origin fallback. `GET /auth/sessions`
lists devices; `DELETE /auth/sessions/:id` and `/others` revoke. Each session now
also carries a **`family_id`** (rotation lineage) and **`rotated_at`** (spent
marker).

## Access Tokens

RS256, `iss=openrisk`, `aud=openrisk-api`, `sub`, `iat`, `exp`, `jti`, plus
`tenant_id`, `org_roles`, `permissions`, `feature_flags`, and a `type` claim
(`ACCESS` / `MFA_REQUIRED` / `MFA_ENROLLMENT`). The middleware validates the
signature with the public key and pins the algorithm. **The tenant is taken from
the verified token, never from a client-supplied field** — proven live: after a
switch, the new token's `tenant_id` was the target org.

## Refresh Token Rotation

Single-use with reuse detection (`internal/auth.RefreshTokenPair`):

```
R1 --refresh--> A2 + R2   (R1 marked rotated_at, same family)
R1 --replay-->  ErrRefreshTokenReuse  → whole family revoked, 401
R2 --after reuse--> invalid (family gone), 401
```

- Unknown token → `ErrRefreshTokenInvalid` (401).
- **Reuse** (spent token replayed, or the loser of a concurrent race) →
  `ErrRefreshTokenReuse`: the family is deleted, the HTTP layer clears cookies and
  audits `refresh_reuse`.
- **Concurrency:** the token is claimed atomically
  (`UPDATE ... SET rotated_at=now WHERE id=? AND rotated_at IS NULL`); exactly one
  of N racers wins, the rest are treated as reuse.
- **Org context preserved:** claims are re-resolved for the token's **own** org
  via the org resolver; if the user is no longer an active member, the family is
  revoked.
- Expired → `ErrRefreshTokenExpired`; the dead row is cleaned so a later replay
  reads as invalid, not reuse.

Device-fingerprint binding: if both stored and presented fingerprints exist they
must match (`ErrDeviceMismatch`).

## Token Revocation

- **Rotation** spends the presented token (single-use).
- **Reuse detection** revokes the whole family.
- **Logout** revokes the presented refresh token.
- **Session management** revokes one or all-other sessions.
- `RevokeAllUserTokens` exists for password-change / account-disable flows.
- `PruneExpiredTokens` removes tokens past TTL (spent tokens survive their own
  validity window so reuse stays detectable); intended for a periodic worker.

## Logout

`POST /auth/logout` clears the session cookies **unconditionally** (a logout that
leaves usable cookies is worse than one that errors), then revokes the presented
refresh token and audits `logout`. Cookie-only clients need no body.

## MFA

TOTP (SHA1 / 6 digits / 30 s), secret encrypted at rest (AES-256-GCM,
`MFA_ENCRYPTION_KEY`), 8 CSPRNG backup codes.
- **Enrollment:** `POST /auth/mfa/setup` (returns secret + QR + backup codes,
  requires a full session or an MFA_ENROLLMENT token) → `POST /auth/mfa/verify`
  with a TOTP code. Mandated enrollment issues the session on verify.
- **Login challenge:** `POST /auth/mfa/challenge` (MFA_REQUIRED token) validates a
  TOTP **or** a single-use backup code, then mints the real pair.
- **Disable:** `POST /auth/mfa/disable` (password-gated).
- Secrets never appear in responses (setup deliberately returns the secret **once**
  for provisioning), logs, or audit rows — verified live (0 occurrences of the
  secret in server logs).

Proven live end-to-end: login→enrollment→setup→TOTP→verify→session, and
login→challenge→session, with an invalid code rejected (`400`).

## Recovery

- **Password reset:** `POST /auth/password/forgot` + `/reset` + `/check`
  (`PasswordReset` use case, rate-limited, audited `password_reset_request` /
  `_confirm`). Reset tokens are not returned in responses.
- **MFA recovery:** single-use backup codes issued at setup; consuming one at the
  challenge signs the user in. Reusing a spent code fails (each code is hashed and
  deleted on use).

## OAuth2 / OIDC

`GET /auth/oauth2/login/:provider` + `/callback/:provider`. The callback resolves
the identity (known link → verified-email link → refuse) and mints a session via
the **shared** `TokenManager.IssueSession` — byte-identical to password login,
RS256. **No provisioner is wired**, so an identity with no existing OpenRisk
account is refused rather than silently admitted (SSO signs existing members in;
it does not create tenants). OAuth `state` is generated per attempt. Client
secrets live server-side only.

**Not exercised against a live IdP** (no test IdP credentials in this
environment) — the session-minting half shares the exact path proven live via the
MFA challenge, but the provider round-trip itself is unverified here. Marked
`BLOCKED` (environment), not claimed as passing.

## SAML2

`GET /auth/saml2/login`, `POST /auth/saml2/acs`, `GET /auth/saml2/metadata`.
Present and wired; the ACS mints a session through the same `IssueSession` minter.
**Not exercised against a live IdP** in this environment (no test IdP / signed
assertions) → assertion signature/audience/recipient/replay validation is
**NOT verified here**; marked `BLOCKED`. This is called out honestly rather than
claimed.

## Organization Switching

New, server-authorized, on the RS256 stack:

- `GET /auth/organizations` — lists the caller's **active** memberships (id, name,
  slug, role, business role, default flag). A deactivated membership and another
  user's org never appear.
- `POST /auth/switch-org` — re-validates active membership of the target org on
  the server, then mints an **org-scoped** session (`IssueSessionForOrg` → org
  resolver → active-membership check) and re-issues the HttpOnly cookies. Audits
  `switch_org` (success and `switch_denied`).

A non-member or deactivated-member target returns an **identical `403`** (no org
enumeration). Because the resolver re-checks membership at mint time, there is no
gap between "the switch was authorized" and "the token is valid" — a forged org id
in a body, cookie, or token cannot move a user into a tenant they do not belong
to.

## Tenant Isolation

- The access token's `tenant_id` is the sole tenant source; handlers read it from
  the verified context (`middleware.GetContext`), never from client input.
- Refresh keeps a session on its **own** org (the refresh-token row's `TenantID`).
- Switch mints for the target org only after an active-membership check.
- A prior sweep (`audit/tenant-isolation-sweep`) hardened per-repository
  `tenant_id` predicates; this work adds the identity-layer boundary on top.

## Browser Storage

- **No access or refresh token is written to `localStorage`/`sessionStorage`.**
  The durable credential is an HttpOnly cookie; the access token is held in memory
  for the tab lifetime only (`frontend/src/lib/session.ts`).
- `localStorage` holds `auth_user` (non-secret profile) and, during OAuth,
  `oauth_state_<provider>` (a public anti-CSRF nonce). Legacy `auth_token` /
  `auth_refresh_token` keys are only ever `removeItem`'d (cleanup of pre-cookie
  values).
- **Observation (P3, not fixed):** `oauth_state` in `localStorage` would be
  slightly stronger as a short-lived HttpOnly cookie; it is a nonce, not a
  credential, so the exposure is low.

## Secret Handling

- Passwords: Argon2id, `json:"-"`.
- Refresh tokens: stored as SHA-256 hashes, never in cleartext.
- MFA secret: AES-256-GCM at rest; returned once at setup for provisioning only.
- Backup codes: hashed at rest, single-use.
- JWT signing: RS256 private key from file/inline env; never logged.

## Logging and Redaction

Verified live: the TOTP secret and the test password had **0 occurrences** in the
server log across a full enrollment + rotation + switch run. Auth audit rows carry
IP, UA, geo, device fingerprint, action, success, and a failure **reason** — never
a secret.

## CSRF

Cookie-authenticated state changes require a double-submit CSRF token
(`or_csrf` readable cookie mirrored in the response body and sent back in a
header). Safe methods are exempt. Enforced by the session-cookie middleware; the
frontend axios client attaches it on non-GET requests.

## Abuse Protection

`authRateLimit` guards `/auth/login`, `/auth/register`, and the password
endpoints. MFA challenge throttling and lockout tuning are handled in the MFA use
case / middleware. Exact limits are configured in middleware (not re-tuned here).

## Audit Events

Recorded in `auth_audit_logs`: `login` (success/failure incl. `mfa_required` /
`mfa_enrollment_required`), `logout`, `refresh`, **`refresh_reuse`** (new),
`switch_org` (new, success + `switch_denied`), `mfa_setup`, `mfa_verify`
(success/failure), `password_reset_request/confirm`, `session_revoke(_all)`,
`pat_create/revoke`, `oauth_link/conflict`. All observed live for the test user.

## Frontend UX State Matrix

The SPA already renders loading / invalid-credentials / account-disabled /
rate-limited / MFA-required / MFA-enrollment / success / server-error for login,
and equivalent states for registration, MFA, and OAuth. `GET /auth/organizations`
+ `POST /auth/switch-org` give the frontend a real org switcher to bind to (UI
wiring of the switcher is a follow-up; the endpoints are live and covered).

## API Contract Security

| Endpoint | Auth | Tenant source | Secrets in response | Audit |
|---|---|---|---|---|
| `POST /auth/register` | none | new org | none | — |
| `POST /auth/login` | none (rate-limited) | default org from DB | token pair only | `login` |
| `POST /auth/mfa/challenge` | MFA_REQUIRED token | token | token pair only | `mfa_verify` |
| `POST /auth/refresh` | refresh token | token row's org | token pair only | `refresh` / `refresh_reuse` |
| `POST /auth/logout` | refresh token | — | none | `logout` |
| `GET /auth/organizations` | session | — | none | — |
| `POST /auth/switch-org` | session | validated target org | token pair only | `switch_org` |
| `GET /auth/me` | session | token | user (no secrets) | — |

## Test Coverage

**Unit / manager (`internal/auth/token_test.go`):** rotation success, reuse →
family revocation, expiry, invalid, concurrent-rotation one-winner,
org-context preservation, lost-membership revocation, device mismatch,
`IssueSessionForOrg` membership gate.

**Use case (`internal/application/auth`):** switch success / non-member 403 /
inactive-member 403 / validation / default-org marking; login deactivated
membership rejected; plus the pre-existing login-MFA / MFA / OAuth-link /
password-reset suites (kept green).

**Integration / HTTP (`internal/handler/auth_switch_e2e_test.go`):** handler →
use case → GormUserRepository → sqlite → TokenManager org resolver — lists only
the acting user's active orgs; switch to a member org mints a token whose decoded
`tenant_id` is the target; switch to a non-member / deactivated-member org → 403.

Full backend suite: `go build ./...`, `go vet`, `go test ./...` — **0 failures**.

## Positive / Negative / Expiry / Cross-Tenant Matrix

| Flow | Positive | Negative | Expiry | Cross-Tenant | Audit | Secret Leakage |
|---|---|---|---|---|---|---|
| Registration | ✓ (live) | ✓ dup 409 (live) | N/A | N/A (new org) | N/A | ✓ none (live) |
| Login | ✓ (live) | ✓ bad pw 401, disabled-membership (test) | ✓ token TTL | ✓ default-org only | ✓ (live) | ✓ (live) |
| MFA | ✓ enroll+challenge (live) | ✓ bad code 400 (live) | ✓ challenge TTL | ✓ tenant on token | ✓ (live) | ✓ 0 in logs (live) |
| Refresh | ✓ rotate (live) | ✓ reuse→family revoke (live+test) | ✓ expired (test) | ✓ org preserved (live+test) | ✓ refresh_reuse (live) | ✓ hashed store |
| Logout | ✓ | ✓ revokes refresh | N/A | ✓ per-user | ✓ | ✓ |
| Org switch | ✓ member (live) | ✓ non-member 403 (live+test) | ✓ new session | ✓ 403, no enum (live+test) | ✓ switch_org (live) | ✓ token only |
| OAuth/OIDC | BLOCKED (no IdP) | BLOCKED | — | shares switch resolver | ✓ oauth_link | ✓ secret server-side |
| SAML2 | BLOCKED (no IdP) | BLOCKED | — | — | — | ✓ |

## Security Findings

| ID | Finding | Severity | Evidence | Impact | Mitigation | Test | Status |
|----|---------|----------|----------|--------|------------|------|--------|
| P1-01 | Refresh rotation had no reuse detection / family revocation; concurrent double-rotation could mint two valid pairs | P1 | `token.go` read-then-delete, no `family_id` | A leaked refresh token could be replayed undetected; race could fork a session | family_id + rotated_at, atomic claim, family revoke on reuse | `TestRefresh_Reuse_RevokesFamily`, `TestRefresh_ConcurrentRotation_OneWinner` + live | Fixed |
| P2-01 | Refresh reset session to the user's default org, discarding switched context | P2 | resolver used default org | A switched user silently snapped back to default org every 15 min | refresh resolves for the token's own org | `TestRefresh_PreservesOrgContext` + live | Fixed |
| P2-02 | Deactivated membership still yielded a session | P2 | login/resolver ignored `is_active` | Removing a user from an org did not revoke their access to it | is_active gate at login + resolver | `TestLogin_DeactivatedMembershipIsRejected`, `TestRefresh_LostMembership_RevokesFamily` + live 403 | Fixed |
| P2-03 | No reachable org-switch endpoint; only impl minted RS256-incompatible HS256, unwired | P2 | `MultitenantAuthService` not in main.go | Multi-org users could not switch; if wired, weaker HS256 refresh with no rotation | RS256 switch with server membership check + audit | switch use-case + E2E tests + live | Fixed |
| P3-01 | `GET /auth/me` returned a hardcoded `user@example.com` | P3 | handler placeholder | Façade; clients could not read the real profile | real user via repo, secrets `json:"-"` | live `/auth/me` returns real email | Fixed |
| P3-02 | `oauth_state` nonce stored in `localStorage` | P3 | `Login.tsx` | Low — a public anti-CSRF nonce, not a credential | (not fixed) prefer short-lived HttpOnly cookie | — | Open (low) |

## Live Proof Record

### Environment
- Backend binary built from this branch, run on `:8099`.
- Postgres 15 on `localhost:5434` (container `openrisk_db`), Redis on `:6379`.
- Boot applied **migration version 57** cleanly (`migrations: applied
  successfully (version=57 dirty=false)`) on a **populated** database (104 refresh
  tokens, 53 users) — proving the pre-AutoMigrate `family_id` backfill.

### Commit SHA
`c6d5d24` (branch `feat/w0-03-authentication-security-hardening`).

### Test Tenants / Accounts
Dedicated test-only data (no real credentials):
- User `w003-tester@example.test` (created via `/auth/register`).
- Orgs: "W003 Org A" (owner, role root), "W003 Org B" (active member, admin),
  "W003 Org C (foreign)" (non-member). The password and TOTP secret are held only
  in the session scratch space, never committed.

### Commands Executed (abridged)
```
POST /auth/register                       → 201, no secrets; duplicate → "user already exists"
POST /auth/login                          → mfa_enrollment_required (admin/root MFA mandate)
POST /auth/mfa/setup  (enrollment token)  → secret(256-bit b32) + QR + 8 backup codes
POST /auth/mfa/verify (live TOTP)         → full session (access+refresh)
GET  /auth/me                             → real user w003-tester@example.test, no password
POST /auth/refresh  (R1)                  → R2 (R1 != R2)
POST /auth/refresh  (replay R1)           → 401 {"code":"REFRESH_REUSE_DETECTED"}
POST /auth/refresh  (R2 after reuse)      → 401 (family revoked)
POST /auth/mfa/challenge (live TOTP)      → full session
GET  /auth/organizations                  → Org A (root, default) + Org B (admin)
POST /auth/switch-org  → Org B (member)   → 200, new token tenant_id == Org B
POST /auth/switch-org  → Org C (non-mem)  → 403 "access denied"
POST /auth/refresh (Org-B session)        → refreshed token tenant_id STILL Org B
POST /auth/mfa/challenge (code 000000)    → 400
POST /auth/login  (wrong password)        → 401
GET  /auth/me     (no token)              → 401
```

### Authentication Flows Tested (live)
register, login (MFA enrollment + challenge paths), MFA setup/verify/challenge,
refresh rotation, refresh reuse, org list, org switch, org-context-preserving
refresh, `/auth/me`.

### Security Cases Tested (live)
reuse detection + family revocation; concurrent rotation (unit/`-race`);
non-member switch 403; deactivated membership; invalid MFA code 400; wrong
password 401; unauthenticated 401.

### Cross-Tenant Cases
Switch to a non-member org → 403 (identical to deactivated, no enumeration);
another user's org never listed; refreshed token stays on its own org; token
`tenant_id` comes only from the server, proven by decoding the post-switch JWT.

### Secret Leakage Checks
Server log grep for the TOTP secret → **0**; for the test password → **0**;
password-like assignments → **0**. Register/login/switch/me responses contain no
password, hash, MFA secret, or reset token.

### Browser Storage Checks
No access/refresh token in `localStorage`/`sessionStorage`
(`frontend/src/lib/session.ts` holds the access token in memory; the durable
credential is an HttpOnly cookie). `auth_user` (non-secret) and `oauth_state`
(public nonce) are the only auth-related keys.

### Logs Checked
`scratchpad/server.log` (boot + request logs) — migration success, no secret
leakage.

### Known Limitations
- **OAuth2/SAML2 IdP round-trips are not exercised live** (no test IdP in this
  environment). The session-minting half shares the path proven live via the MFA
  challenge; the provider handshake, SAML signature/audience/recipient/replay
  validation, and OIDC state/nonce/code-exchange are **not** verified here →
  marked `BLOCKED`, not passing.
- The legacy HS256 `MultitenantAuthService` remains in the tree (unwired,
  non-functional); deleting it is out of W0-03 scope.
- `oauth_state` nonce sits in `localStorage` (P3-02, low).
- `PruneExpiredTokens` is implemented but not yet wired to a scheduler; spent/
  expired tokens are bounded by the 30-day TTL until then.
- The frontend org-switcher UI is a follow-up; the endpoints are live and tested.

## Recommended Follow-up Work
1. Wire OAuth2/OIDC + SAML2 against a test IdP and add signature/audience/
   recipient/replay + state/nonce/code-exchange tests (lift the `BLOCKED` rows).
2. Delete the dead HS256 `MultitenantAuthService` and its handler.
3. Schedule `PruneExpiredTokens` from a worker.
4. Move `oauth_state` to a short-lived HttpOnly cookie.
5. Build the frontend org-switcher on `GET /auth/organizations` /
   `POST /auth/switch-org`.
