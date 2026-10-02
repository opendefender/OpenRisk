# Spec — #688 Auth throttle: per-purpose buckets, per-account backoff, constant-work login

Status: approved by owner 2026-10-02 (thresholds as written · `GetByEmail` itself becomes case-insensitive) · Branch: `688-auth-throttle`, stacked on #689 · Milestone: trust-v1

## Objective

The auth throttle protects the wrong thing in the wrong way:

- one per-IP bucket (15 / 5 min) serves login, sign-up, forgot, reset **and** the
  password-strength meter, so a person typing on the sign-up screen locks
  themselves out of signing up and signing in, and an office behind one NAT
  shares 15 requests;
- nothing limits guesses against **one account** from many IPs;
- login for an unknown address returns in ~6 ms and for a real one in ~45 ms, so
  the clock tells an attacker which addresses have accounts;
- login matches the address case-sensitively, while sign-up and reset normalise.

Success means: the meter never costs a login; a throttled person is told how long
to wait, in FR and EN; ten wrong passwords on one address from any number of IPs
pause that address for a bounded time; unknown and real addresses cost the same;
`Awa.Diallo@X.test` and `awa.diallo@x.test` are the same login.

## Current state (read 2026-10-02)

| Piece | File | Fact |
|---|---|---|
| Limiter | `backend/internal/middleware/ratelimit.go` `RateLimit` | key `c.IP()` only; 429 body `{"error":true,"msg":"Rate limit exceeded"}`, no `Retry-After`, no `code` |
| Shared bucket | `backend/cmd/server/main.go:849-882` | one `authRateLimit` (15/5 min) on login, register, forgot, reset, check; also on protected `/auth/password/change` (1143) and `/auth/mfa/disable` (1125) |
| Redis window | `backend/internal/infrastructure/redis/client.go` `AllowRate` | fixed wall-clock bucket `now / window`, so the time left in a bucket is computable without a Redis round trip |
| Prefix pattern | `backend/internal/handler/public_vendor_assessment_handler.go:37` | `PrefixedRateLimitBackend{Prefix, Inner}` |
| Login | `backend/internal/application/auth/login.go` `Execute` | unknown address → return before hashing; disabled → return before hashing; wrong password → Argon2id `Verify` |
| Lookup | `backend/internal/infrastructure/repository/gorm_user_repository.go:73` | `WHERE email = ?`, input not normalised by login |
| Legacy rows | #687 closing comment | register normalises since #694, but no backfill and no `lower(email)` index; mixed-case rows may still exist |
| Reset confirm | `backend/internal/application/auth/password_reset.go:259` | loads the user, writes the new hash — the place to clear a login lock |
| Audit | `backend/internal/auth/audit.go` `LogFiber` | `failure_reason` text column; no metadata field |
| Screen | `frontend/src/features/auth/AuthScreen.tsx` | login: any error → `signInFailed`; register: non-409/400 → `registerFailed` |
| Sibling | `docs/689_MFA_CHALLENGE_LIMITS.md` (approved, not implemented) | plans an `MFAAttemptStore` (Incr / Exists / Set / Delete) in `infrastructure/authmfa` — the same primitive this issue needs |

## Assumptions (correct me now or I proceed)

1. **Thresholds** (per IP, each its own Redis key prefix):
   | Bucket | Routes | Limit |
   |---|---|---|
   | `auth-login:` | `/auth/login` | 15 / 5 min (unchanged) |
   | `auth-register:` | `/auth/register` | 10 / 15 min |
   | `auth-reset:` | `/auth/password/forgot`, `/auth/password/reset` | 15 / 5 min |
   | `auth-check:` | `/auth/password/check` | 120 / 5 min |
   | `auth-reauth:` | `/auth/password/change`, `/auth/mfa/disable` | 15 / 5 min (today's limit, moved off the login key) |
2. **Per-account backoff**, keyed by `sha256(normalised address)`, so unknown
   addresses lock exactly like real ones (no new oracle):
   10 failed logins in 15 min → the address is refused for 15 min with
   `429 {"code":"LOGIN_LOCKED","retry_after":N}` + `Retry-After`. Refused attempts
   neither count nor extend the lock, so it is bounded. A successful login clears
   the failure count; a successful password reset clears the count **and** the
   lock. Same shape and numbers as #689's per-account MFA lock.
3. **What counts as a failure:** unknown address, disabled account, wrong
   password. Not: revoked membership or missing organisation (the password was right).
4. **Audit:** every failed login row carries `failure_reason =
   "invalid_credentials email_sha256=<hex>"`; a refused-by-lock attempt carries
   `"login_locked email_sha256=<hex>"`. The hash is `domain.HashEmailForReset`
   (unsalted SHA-256, the function reset already uses). No migration.
5. **Constant work:** the login use case hashes a fixed dummy password once at
   construction with the live hasher (current Argon2id params) and verifies the
   submitted password against it for unknown addresses. A disabled account is
   verified against its real hash, then refused.
6. **Case-insensitive lookup:** login normalises with `domain.NormaliseEmail`;
   `GormUserRepository.GetByEmail` becomes `LOWER(email) = ?` with
   `ORDER BY (email = ?) DESC, created_at ASC`, so a legacy case-variant duplicate
   resolves deterministically to the exact-spelling row, then the oldest. This
   makes every `GetByEmail` caller (register, reset, invitations, OAuth link,
   automation) case-insensitive — which is what #687 intended. No index here
   (migration numbering waits on the TPRM stack, per #687); a seq scan on `users`
   is fine at current scale, and the index is a follow-up issue.
7. **Throttle response, every `RateLimit` route:** 429 gains `Retry-After`
   (seconds to the end of the fixed Redis window) and the body gains
   `"code":"RATE_LIMITED","retry_after":N`. `error` and `msg` stay for existing
   clients. The JSON field exists because a cross-origin SPA cannot read
   `Retry-After` unless CORS exposes it.
8. **Frontend:** sign-in and sign-up show "Trop de tentatives. Réessayez dans
   N minute(s)." / "Too many attempts. Try again in N minute(s)." for any 429
   (`RATE_LIMITED` or `LOGIN_LOCKED`), N = ceil(retry_after / 60), min 1.
9. **Shared store with #689:** #689 (PR #871) already ships the store this
   needs: `infrastructure/authmfa.AttemptStore`, Redis with an in-memory
   fallback. This branch is stacked on #689 and reuses it through a narrower
   login-side port; no second store is written.

## Design

- **Port** `application/auth/login_limits.go`: `LoginAttemptStore { Increment; Lock; LockedFor; Delete }` — a subset of #689's `MFAAttemptStore`, satisfied by `authmfa.AttemptStore` without changes. Keys `login:fail:<sha256>` and `login:lock:<sha256>`.
- **Use case** `LoginUseCase.WithAttemptLimits(store)` → sentinel `ErrLoginLocked` carrying `RetryAfter` (`errors.As`). Unwired = today's behaviour; existing tests untouched.
- **Use case** `ConfirmPasswordResetUseCase.WithLoginLockClearer(store)` clears both keys after the new hash is written.
- **Middleware** `RateLimit` sets `Retry-After` and the new body fields. The prefix wrapper moves to `middleware.PrefixedBackend`; `handler.PrefixedRateLimitBackend` becomes an alias, no churn elsewhere.
- **Handler** `Login` maps `ErrLoginLocked` → 429; writes the hashed address in every failure audit row.
- **main.go**: wiring only.

## Commands

```
cd backend && go build ./... && go vet ./... && go test ./... -race
cd backend && go test ./internal/application/auth/ ./internal/handler/auth/ ./internal/middleware/ -race -v -run 'Login|AuthRateLimit|RateLimit'
cd frontend && npx tsc -b --noEmit && npx eslint src/features/auth && npx vitest run && npm run build
```
Live: throwaway pg + redis on spare ports, server from the repo root (live-boot recipe); re-run reproductions 1, 2, 4 by curl; 20-attempt timing medians; real-browser pass on sign-in and sign-up at 429.

## Testing strategy
- **Use case** (`login_throttle_test.go`, fake store + counting hasher): `TestLogin_UnknownAddressCostsAHashComparison` (Verify called once for unknown and for disabled), `TestLogin_EmailCaseInsensitive`, `TestLogin_PerAccountBackoff` (10 failures → `ErrLoginLocked` even with the right password; unknown address locks too; lock bounded; success clears count), plus Success / NotFound / Unauthorized.
- **Reset**: `TestConfirmPasswordReset_ClearsLoginLock`.
- **Repository**: `GetByEmail` case-insensitive and deterministic on duplicates.
- **HTTP**, real middleware chain: `TestAuthRateLimit_PasswordCheckDoesNotConsumeLoginBudget`; 429 carries `Retry-After` + `code`; `LOGIN_LOCKED` over HTTP.
- **Frontend**: `authThrottle.test.tsx` — login 429 and register 429 render the wait message in FR and EN.
- **Timing**: live medians over 20 attempts, unknown vs existing, < 10 ms apart.

## Boundaries
- **Always:** typed errors; no address, password or token in logs (hash only); full `go test ./...` before the PR.
- **Ask first:** thresholds other than the ones above; any migration (index, audit metadata column); CORS changes.
- **Never:** touch the MFA challenge route (#689); change the password hash algorithm or params; make Redis fail-closed.

## Success criteria
1. 121 × `/auth/password/check` from one IP, then `/auth/login` with valid credentials → 200; `/auth/register` → 201.
2. Any throttled auth call → 429 with `Retry-After` and `code`; both screens show the wait in FR and EN.
3. 10 wrong passwords on one address over ≥ 2 IPs → the 11th, even correct, answers 429 `LOGIN_LOCKED`; after the lock, correct → 200; a password reset clears it immediately; every failure has an audit row with `email_sha256=`.
4. Median login time over 20 attempts, unknown vs existing address, differs by < 10 ms.
5. `Awa.Diallo@X.test` logs into `awa.diallo@x.test`.
6. The five named tests pass under `-race`; full backend and frontend gates green.

## Known limits
- **Deliberate lockout:** anyone can pause any address for 15 min, repeatedly. Bounded per lock; the owner's way out is a password reset. Same trade-off accepted for #689.
- **Redis down:** buckets and locks become per-instance.
- **`Retry-After` in degraded mode:** computed from the Redis fixed window; the in-memory fallback slides, so the hint may be early by up to one window.
- **Audit hash:** unsalted SHA-256 of an address is dictionary-reversible for a known address list. Kept for consistency with reset; HMAC is a follow-up if wanted.
- **Legacy hashes:** accounts on a pre-upgrade hash verify at a different cost than the dummy until their next login migrates them.
- **`LogFiber` reads `X-Forwarded-For` raw** for the audit IP (spoofable). Pre-existing, out of scope; follow-up issue.

## Plan — one commit per task, `fix(auth): … (#688)`
- [ ] **T1** Throttle response (`Retry-After`, `code`, `retry_after`) + `middleware.PrefixedBackend`. Verify `go test ./internal/middleware/ -race`.
- [ ] **T2** Five per-purpose buckets in `main.go`; `TestAuthRateLimit_PasswordCheckDoesNotConsumeLoginBudget`.
- [ ] **T3** Case-insensitive lookup (login normalises; repo lower + deterministic order); `TestLogin_EmailCaseInsensitive` + repo test.
- [ ] **T4** Constant work (dummy hash; unknown + disabled verified); `TestLogin_UnknownAddressCostsAHashComparison`.
- [ ] **T5** ~~Attempt store~~ — reused from #689.
- [ ] **T6** Per-account backoff: login lock, reset clears, handler 429 + hashed audit, wiring; `TestLogin_PerAccountBackoff`, `TestConfirmPasswordReset_ClearsLoginLock`.
- [ ] **T7** Frontend FR/EN strings + 429 on sign-in and sign-up + vitest.
- [ ] **T8** Full gates; reproductions 1, 2, 4 re-run live; browser pass; issue comment; PR `Closes #688`; follow-up issues (email index, audit IP header).

Risk: the route registry / authz ratchet tests may trip on the new middleware (T2) — full `go test ./...` before the PR.
