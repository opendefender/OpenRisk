# Spec — #689 MFA challenge limits and logout revocation

Status: approved by owner 2026-10-02 (limits kept · lock email yes · frontend copy in a follow-up issue) · Branch: `689-securityauth-mfa-challenge-codes-can-be-guessed-without-limit-no-attempt-counter-no-rate-limit` · Milestone: trust-v1

## Objective

Someone who holds a user's password can currently submit TOTP or backup codes to
`POST /api/v1/auth/mfa/challenge` without limit: no attempt counter, no rate
limit, and a challenge token reusable for 5 minutes. A second factor that can be
guessed is not a second factor. Separately, `POST /auth/logout` leaves the access
token valid for up to 15 minutes.

Success means:

- a challenge token dies after 5 wrong codes;
- an account stops accepting challenges for a bounded time after repeated failures, whatever the token or source IP;
- the route has its own per-IP bucket;
- backup codes count the same as TOTP codes;
- a logged-out access token is refused on the next request.

## Current state (read 2026-10-02)

| Piece | File | Fact |
|---|---|---|
| Route | `backend/cmd/server/main.go:898` | `MFATokenMiddleware` only, no limiter |
| Handler | `backend/internal/handler/auth/mfa_handler.go` `Challenge` | audits each failure, nothing else |
| Use case | `backend/internal/application/auth/mfa_usecase.go` `ChallengeMFAUseCase.Execute` | TOTP, then bcrypt over unused backup codes; returns `ErrValidation` on a miss |
| Challenge token | `backend/internal/auth/token.go` | `MFAChallengeTTL = 5m`, carries a JTI; middleware sets `c.Locals("jti")` and `c.Locals("user")` (claims, with `ExpiresAt`) |
| Blacklist | `backend/pkg/auth/blacklist.go` | `BlacklistJTI(ctx, jti, ttl)`; read by every token middleware; fail-open if Redis is down |
| Shared counters | `backend/internal/middleware/ratelimit.go`, `backend/internal/infrastructure/redis/client.go` | `AllowRate` is a **fixed, wall-clock-aligned** window. A 5-minute token can straddle two buckets, so it cannot give an exact "5 per token" |
| Precedent | #754, `WithDisableAttemptLimit` | per-account budget on `/auth/mfa/disable` via the shared store |
| Logout | `backend/internal/handler/auth/handler.go:480`, `application/auth/logout.go` | revokes the refresh token and clears cookies; the access token is never looked at |

## Design

### 1. Per-token limit (AC 1)

- Key `mfa:challenge:token:<jti>`. It counts **failed** codes only and expires with the token's remaining lifetime.
- Before verifying, the use case checks the count. At 5 or more it refuses without touching TOTP or bcrypt.
- The 5th failure blacklists the JTI for `claims.RemainingTTL()`. From then on the middleware itself answers `401 TOKEN_REVOKED`.
- The 5th failure answers `401 {"code":"MFA_CHALLENGE_EXHAUSTED"}`, so the screen can say "sign in again".
- The counter check is what enforces the limit. The blacklist is a second barrier. So a Redis outage, where the blacklist fails open, still leaves the per-instance counter refusing the token.

### 2. Per-account limit (AC 2)

- Key `mfa:challenge:user:<user_id>`. It counts failures across all tokens and IPs, over a 15-minute window.
- At **10 failures** the lock key `mfa:challenge:lock:<user_id>` is set for **15 minutes**.
  - While the lock is set, every challenge for that user answers `429 {"code":"MFA_LOCKED"}` with `Retry-After`. The code is not evaluated.
  - The lock is not extended by attempts made while it holds, so it is bounded.
- Setting the lock is audited once, as `mfa_verify` with success=false and reason `mfa_locked`. Reusing the existing action avoids a migration.
- Setting the lock also emails the account owner, once per lock, in their locale (FR/EN). The email says that someone passed their password and then failed the second factor repeatedly, and it recommends a password reset. It reuses the async `securityMailer` (`internal/infrastructure/authmail`) through a new `MFALockedMailer { SendMFALocked(ctx, to, fullName, locale string) error }` port, on the model of `MFADisabledMailer`. A mail failure never changes the response.
- A successful challenge clears the user's failure counter. It does not clear an active lock.
- The key is the user, not (user, tenant). The lock covers the identity across organisations, which is the stricter choice. No DB query is added, so the tenant-filter rule is not engaged.

### 3. Per-IP limit (AC 3)

The existing `middleware.RateLimit` runs over `authLimiterStore`, wrapped in `handlers.PrefixedRateLimitBackend{Prefix: "mfa-challenge"}`:

- **20 requests per 5 minutes per IP**, its own bucket, separate from login's;
- mounted **before** `MFATokenMiddleware`, so a flood never reaches token validation.

This is the pattern #688 asks for. #688 itself (login, sign-up and reset buckets) is not touched.

### 4. Backup codes (AC 4)

Already satisfied structurally: one `Execute` call is one attempt, whether the input matched as TOTP, as a backup code, or as neither. The tests prove it explicitly.

### 5. Logout revokes the access token (AC 5)

- `Logout` reads the access token from `Authorization: Bearer` or the `or_access` cookie, the same order as `extractAccessToken`.
- It validates the token's signature and expiry with the server's RSA keys. An invalid or expired token is ignored: the response is still 200 and the cookies are still cleared, as today.
- It then blacklists the JTI for the token's remaining TTL.
- This happens whether or not a refresh token was supplied.
- The revocation lives in `LogoutUseCase`, through a `TokenRevoker` port. The handler only extracts the JTI and expiry.

### Layering

- **`application/auth` port:**
  `MFAAttemptStore { Increment(ctx, key, ttl) (int64, error); Exists(ctx, key) (bool, error); Set(ctx, key, ttl) error; Delete(ctx, key) error }`
  It sits next to `TokenRevoker { BlacklistJTI(ctx, jti, ttl) error }`, which `*authpkg.TokenBlacklistManager` already satisfies.
- **`infrastructure/authmfa/attempts.go`:** a Redis implementation (`INCR`, plus `EXPIRE` on the first increment) with an in-memory fallback, mirroring `RedisRateLimitStore`. A Redis outage degrades to per-instance limits and never fails open.
- **`ChallengeMFAUseCase.WithAttemptLimits(store, revoker)`:**
  - `ChallengeMFAInput` gains `ChallengeJTI` and `ChallengeExpiresAt`;
  - new sentinel errors `ErrMFAChallengeExhausted` and `ErrMFAChallengeLocked` (the latter carries the retry duration), following the `ErrMFADisable*` precedent;
  - unwired means today's behaviour, so the existing tests are unaffected.
- **Handler:** `Challenge` maps the two errors to 401 and 429. `Logout` gains `WithAccessTokenRevocation(rsaKeys)`.
- **`main.go`:** wiring only.

## Commands

```
cd backend && go build ./... && go vet ./... && go test ./... -race
cd backend && go test ./internal/application/auth/ ./internal/handler/auth/ ./internal/infrastructure/authmfa/ -run 'MFAChallenge|Logout' -race -v
```
Live check: throwaway pg + redis on spare ports, run the server from the repo root (live-boot recipe), then curl through login → challenge.

## Testing strategy

**Use case** (`application/auth/mfa_challenge_limits_test.go`), with a fake store and a fake revoker:

- `TestMFAChallenge_TokenDiesAfterFiveFailures`
- `TestMFAChallenge_UserBackoffAcrossTokens`
- `TestMFAChallenge_BackupCodesCounted`
- `TestMFAChallenge_Success`
- `TestMFAChallenge_NotFound`
- `TestMFAChallenge_Unauthorized` (a locked account is refused even with the right code)
- `TestMFAChallenge_SuccessClearsFailureCount`
- `TestMFAChallenge_LockSendsOneEmail` (one email per lock, not per refused attempt)

**Store** (`infrastructure/authmfa/attempts_test.go`): counting, expiry, and fallback when Redis errors.

**Handler e2e** (`handler/auth/mfa_challenge_e2e_test.go`), through real middleware with a real RSA key and an in-memory blacklist:

- the 6th request with the same token gets 401 `TOKEN_REVOKED`;
- a lock gets 429 with `Retry-After`;
- the IP limit gets 429 before token validation.

**Logout:**

- `TestLogout_AccessTokenRejectedAfterLogout`: log in, call logout with the bearer token, then `/auth/me` with the same token gets 401 `TOKEN_REVOKED`;
- a cookie-only logout gets the same result.

**Live:** criteria 1, 2 and 5 against a local instance with an enrolled account. The output goes in the issue comment.

## Boundaries

- **Always:**
  - typed errors;
  - no code and no token in logs or audit, only reason codes;
  - the full `go test ./...` before the PR (route changes trip the authz registry tests).
- **Ask first:** thresholds other than those below, a new audit action (that means a migration).
- **Never:**
  - touch the login, sign-up or reset buckets (#688);
  - change the TOTP tolerance window;
  - make the blacklist fail-closed globally. That is a design change to auth and goes to escalation.

## Success criteria

1. A 6th call with a token that has had 5 wrong codes answers 401, and the JTI is in the blacklist.
2. 10 failures over two or more tokens lock the account. During the lock a correct code answers 429 with `Retry-After`, the lock audit row exists, and after the lock expires a correct code succeeds.
3. 21 challenge requests from one IP within 5 minutes: the 21st answers 429, and `/auth/login` from that IP is still allowed.
4. Wrong backup codes increment the same counters as wrong TOTP codes.
5. After logout, the old access token answers 401 on `/auth/me`.
6. The four named tests, plus Success/NotFound/Unauthorized, pass under `-race`. The full build, vet and test run is green.

## Known limits (stated, not hidden)

- **Deliberate lockout:** an attacker who already holds the password can re-trigger the 15-minute lock every 15 minutes. That denies the owner, but grants the attacker nothing. The remedy is a password reset, which kills the attacker's first factor. The owner is emailed on each lock, so they learn the password is known.
- **Redis down:**
  - the limits become per-instance;
  - the JTI blacklist fails open, which is existing behaviour, so a logged-out access token stays valid until it expires;
  - the per-token counter still kills challenge tokens on the instance that counted them.

## Decisions (owner, 2026-10-02)

1. Limits kept: 5 per token · 10 failures / 15 min → 15-min lock per account · 20 per 5 min per IP.
2. Email the owner when a lock is set: yes.
3. Frontend copy for `MFA_CHALLENGE_EXHAUSTED` and `MFA_LOCKED`: a separate follow-up issue. This PR stays backend.

## Plan — tasks in dependency order

Each task ends green on its own `go test` and is one commit (`fix(auth): … (#689)`).

- [x] **T1 — Attempt store.** Adds the `MFAAttemptStore` port in `application/auth`, and `infrastructure/authmfa/attempts.go`: Redis (`INCR` + `EXPIRE` on the first increment, `SET` with TTL, `EXISTS`, `DEL`) with an in-memory fallback.
  - Accept: counts, expires, falls back when Redis errors.
  - Verify: `go test ./internal/infrastructure/authmfa/ -race`
  - Files: `application/auth/mfa_limits.go` (new), `infrastructure/authmfa/attempts.go`, `attempts_test.go`

- [x] **T2 — Per-token and per-account limits in the use case.**
  - `WithAttemptLimits(store, revoker)`; input `ChallengeJTI` and `ChallengeExpiresAt`; `ErrMFAChallengeExhausted` and `ErrMFAChallengeLocked{RetryAfter}`.
  - Checks the lock and the token count before verifying. Counts failures, blacklists the token at 5, locks at 10, clears the user counter on success.
  - Accept: success criteria 1, 2 and 4 at the use-case level.
  - Verify: `go test ./internal/application/auth/ -run MFAChallenge -race -v`
  - Files: `mfa_usecase.go`, `mfa_limits.go`, `mfa_challenge_limits_test.go`

- [x] **T3 — Lock email.** `MFALockedMailer` port, sent once when the lock is set; FR/EN template in `authmail`.
  - Accept: one email per lock, not per refused attempt; a mail failure does not change the result.
  - Verify: `go test ./internal/application/auth/ ./internal/infrastructure/authmail/ -race`
  - Files: `mfa_limits.go`, `authmail/` mailer and its test

- [x] **T4 — Handler and route.** `Challenge` passes the JTI and expiry and maps the errors (401 `MFA_CHALLENGE_EXHAUSTED`, 429 `MFA_LOCKED` with `Retry-After`). In `main.go`: wiring plus the per-IP `RateLimit` with the `mfa-challenge` prefix, mounted before `MFATokenMiddleware`.
  - Accept: success criteria 1, 2 and 3 over HTTP.
  - Verify: `go test ./internal/handler/auth/ -run MFAChallenge -race -v`
  - Files: `mfa_handler.go`, `main.go`, `mfa_challenge_e2e_test.go`

- [x] **T5 — Logout revokes the access token.** A `TokenRevoker` in `LogoutUseCase` (input `AccessJTI` and `AccessExpiresAt`); the handler extracts and validates the bearer or cookie token; wiring in `main.go`.
  - Accept: success criterion 5, for both bearer and cookie.
  - Verify: `go test ./internal/application/auth/ ./internal/handler/auth/ -run Logout -race -v`
  - Files: `logout.go`, `handler.go`, `main.go`, tests

- [x] **T6 — Full gate and live proof.**
  - Run `go build ./... && go vet ./... && go test ./... -race`.
  - Live boot with an enrolled account: criteria 1, 2 and 5 by curl, plus the lock email in the mail log.
  - Open the frontend follow-up issue, post the progress comment, open the PR with `Closes #689`, set `status:in-review`.

Risks: route registry and authz ratchet tests may trip on the new middleware order (T4). That is why the full `go test ./...` runs before the PR.
