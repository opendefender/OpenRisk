# Spec — #714 MFA setup: one transaction, typed errors

Status: approved by owner 2026-10-07 (both proposals below accepted) · done 2026-10-07 · Branch: `fix/714-mfa-setup-atomic` · Milestone: trust-v1

## Objective

Starting MFA enrolment must never leave an account half-enrolled, and it must never
show a database message to the user.

#889 (merged in PR #892, 2026-10-02) already fixed the lockout in #714's title:
an unverified secret is now replaced in place, so setup can be retried. Three of
#714's criteria are still open on `origin/master` (cefe453d):

- The secret write, `DeleteBackupCodes` and `SaveBackupCodes` are three separate
  statements. If the third fails, the user keeps a new secret and has **no
  backup codes**. ABSOLUTE RULE 7.
- Every untyped error falls through `mapAuthError` to
  `400 {"error": err.Error()}`, which sends the raw storage text to the client.
  ABSOLUTE RULE 3.
- Two concurrent *first* setups (no row yet) both take the `CreateMFASecret`
  branch. The second one still hits `idx_mfa_secrets_user_id` and answers 400
  "duplicated key". This is the read-then-write race that #889 left.

The frontend comments still say setup "answers 400". That is no longer true. The
guard itself is still needed, for a new reason: each call now **replaces** the key,
so a second call would invalidate the QR code already on screen.

## Acceptance criteria, mapped to #714

| #714 AC | State on master | Work here |
|---|---|---|
| 1 Unverified → 200 with a new secret | ✅ #889 (`TestSetupMFA_ReplacesAnUnverifiedSecret`) | keep it passing |
| 2 Verified → `ErrConflict already_enabled` | ✅ #889 (`TestSetupMFA_VerifiedSecretIsKept`) | keep it passing |
| 3 N calls → one row, one set of unused codes | ⚠️ one row yes; one code set only if nothing fails between the delete and the insert | covered by the transaction |
| 4 Code-storage failure → no new secret persisted | ❌ | **transaction** |
| 5 Storage failure → typed error, no DB text | ❌ | **error mapping** |
| 6 Every query scoped by `tenant_id` | ✅ | keep it, and test it |
| DoD: `TestSetupMFA_{Success,NotFound,Unauthorized,ReplacesUnverifiedSecret,VerifiedConflict}` | `Success` only. #889 named the other two `…AnUnverifiedSecret` / `…VerifiedSecretIsKept` | add `NotFound` and `Unauthorized`; keep #889's names, see Open questions |
| DoD: frontend guard comments | ❌ stale | rewrite both comments |

## Design

### 1. One repository call, one transaction (AC 3, 4)

New method on `MFARepository`:

```go
// StartMFAEnrolment stores a new unverified secret and replaces every backup
// code of the user, in one transaction (#714). It creates the row, or replaces
// the key of an unverified one; false means a verified secret exists, which it
// leaves untouched.
StartMFAEnrolment(ctx context.Context, userID, tenantID uuid.UUID,
    secretEncrypted string, codes []*domain.MFABackupCode) (bool, error)
```

Inside `db.Transaction`:

1. **Upsert the secret.** It is one statement, so it has no read-then-write race:
   `INSERT … ON CONFLICT (user_id) DO UPDATE SET secret_encrypted = excluded.secret_encrypted, last_totp_step = NULL, last_used_at = NULL, updated_at = … WHERE mfa_secrets.is_verified = false AND mfa_secrets.tenant_id = excluded.tenant_id`
   (GORM `clause.OnConflict{Columns, DoUpdates, Where}`; works on Postgres and SQLite).
   0 rows affected means a verified secret, or a row of another tenant. In both cases
   it returns `false` and rolls back. #889's reset of `last_totp_step` and
   `last_used_at` is kept, as #849 requires.
2. `DELETE FROM mfa_backup_codes WHERE user_id = ? AND tenant_id = ?`
3. `CreateInBatches(codes)`

The use case still bcrypt-hashes the 8 codes **before** it calls the repository.
About 8 × 60 ms of hashing must not hold a row lock.

`SetupMFAUseCase.Execute` becomes: validate → `GetMFASecret` (keeps the fast 409
for a verified secret) → generate the secret, QR code and codes → hash →
`StartMFAEnrolment` → `false` ⇒ `NewConflictError("MFA", "already_enabled")`.

`ReplaceUnverifiedMFASecret` loses its only caller. It is removed together with its
mock and its tests, and the new method's repository tests take over its cases.
`CreateMFASecret`, `DeleteBackupCodes` and `SaveBackupCodes` stay. They have other
callers or tests, and removing them is outside this issue.

### 2. No database text in a response (AC 5)

`mapAuthError` (`backend/internal/handler/auth/mfa_handler.go:397`) currently ends
with `400 {"error": err.Error()}`. It becomes
`500 {"error": genericFailure(locale)}`, the same answer `challengeFailed` already
gives for a store failure. The handler logs the underlying error at `error` level
with zerolog. No secret is in it: it is a GORM or driver message.

This also changes `Verify`. Its untyped errors are a lookup failure, a decryption
failure, a `ConsumeTOTPStep` failure and an update failure. Today they answer 400
with internal text; afterwards they answer 500 with generic text. Every error a
user can cause is already typed (`ErrValidation`, `ErrNotFound`, `ErrConflict`)
and is unchanged.

The setup use case also wraps its own internal failures without the storage text
(`fmt.Errorf("store MFA enrolment: %w", err)` stays internal; it is never
rendered).

### 3. Frontend comments (DoD)

`frontend/src/features/auth/MFAEnrollmentDialog.tsx:42` and
`frontend/src/features/auth/AuthScreen.tsx:530`: the guards stay and the comments
are rewritten. Each call issues a new key and invalidates the previous one, so a
second call caused by React's double effect would leave a QR code on screen that
can no longer verify. There is no code change, so no behaviour changes and no new
strings are needed.

## Tests

Use case (`backend/internal/application/auth/`, mock repository):

- `TestSetupMFA_Success`: already exists. Additionally asserts that exactly one `StartMFAEnrolment` call carries 8 hashed codes, each scoped to the user and tenant.
- `TestSetupMFA_NotFound`: the user and tenant have no secret, so the setup **creates** one. This is the use case's not-found path; the account itself is not looked up here.
- `TestSetupMFA_Unauthorized`: a nil user or tenant → `ErrValidation`, and the repository is never touched. This is the same convention as `TestMFAChallenge_Unauthorized`.
- `TestSetupMFA_ReplacesAnUnverifiedSecret` and `TestSetupMFA_VerifiedSecretIsKept`: they already exist and must stay green.
- `TestSetupMFA_StoreFailureIsNotAConflict`: the repository returns an error, so the use case returns an untyped error, not an `AppError`.

Repository (`backend/internal/infrastructure/repository/`, SQLite plus a `_Postgres` twin gated on `DATABASE_URL`):

- creates on first call; replaces an unverified key and resets `last_totp_step`/`last_used_at`; refuses a verified one and leaves the row and codes untouched;
- after N calls: 1 row in `mfa_secrets` and exactly the last 8 codes;
- **rollback**: when the code insert fails (forced by a `BEFORE INSERT` trigger on `mfa_backup_codes`, as `DisableMFA_RollsBackWhenTheSecondWriteFails` does), the secret is unchanged (unverified case) or absent (first-call case), and the old codes are still there;
- tenant scope: a row in tenant A is not replaced by a call for tenant B, and B's codes delete leaves A's codes alone;
- Postgres: two concurrent first setups for one user both return `true` and leave one row. No duplicate-key error.

Handler (`backend/internal/handler/auth/`):

- `Setup` with a repository failure → 500, and the body does not contain the store's error text.

Live check (per [[always-verify-live]]): local stack with real Postgres:
register → login → setup → setup → verify with the **second** QR code's code → 200;
the first QR code's code → refused. Then force a failure (trigger) → the response
is 500 with the generic text, and `mfa_secrets` is unchanged.

## Commands

```
cd backend && go build ./... && go vet ./internal/application/auth/ ./internal/infrastructure/repository/ ./internal/handler/auth/
cd backend && go test -race ./internal/application/auth/ ./internal/infrastructure/repository/ ./internal/handler/auth/
cd backend && DATABASE_URL=postgres://…spare-port… go test -race -run 'MFA' ./internal/infrastructure/repository/
cd backend && go test ./...          # full suite before the PR (route/authz ratchets)
cd frontend && npx tsc -b --noEmit   # comments only, but cheap
```

## Boundaries

- **Always:** keep `tenant_id` in every WHERE; one commit per step; format only touched files; paste test output into the issue comment.
- **Ask first:** any schema or index change (none planned); changing what the frontend does on a setup error.
- **Never:** change the TOTP, encryption or backup-code scheme; touch the MFA policy or grace period; merge the PR.

## Out of scope, noted

- `idx_mfa_secrets_user_id` is unique on `user_id` alone, but every read is per `(user_id, tenant_id)`. A user who belongs to two tenants and has a secret in tenant A can never enrol in tenant B. Before this change that failed with 400 "duplicated key"; it now fails with 409 `already_enabled`, which is clearer but still a refusal. Whether MFA belongs to the person or to each membership is tenant-isolation **design**: escalated as D-066 in `docs/DECISIONS.md`, tracked in #897.

## Decisions taken at review

1. Test names: #889's `TestSetupMFA_ReplacesAnUnverifiedSecret` and `TestSetupMFA_VerifiedSecretIsKept` are kept. They are the tests #714's DoD calls `…ReplacesUnverifiedSecret` and `…VerifiedConflict`.
2. The `mapAuthError` fallback becomes 500 with a generic message, for `Verify` too.

## Tasks

- [x] `StartMFAEnrolment`: upsert, code delete and code insert in one transaction; `ReplaceUnverifiedMFASecret` removed — `gorm_mfa_repository.go`, `mfa_repository_interface.go`
- [x] Setup use case hashes first, then makes one repository call — `mfa_usecase.go`
- [x] Repository tests on SQLite and Postgres: create, replace, N calls, verified refusal, tenant scope, rollback, concurrent first setup — `gorm_mfa_enrolment_test.go`
- [x] Use case tests: `Success` (strengthened), `NotFound`, `Unauthorized`, `StoreFailureIsNotAConflict`; mock updated — `mfa_usecase_test.go`, `mfa_setup_reenrol_test.go`
- [x] `mapAuthError`: untyped error → 500, generic FR/EN text, logged — `mfa_handler.go`, `mfa_setup_http_test.go`
- [x] Frontend guard comments rewritten — `AuthScreen.tsx`, `MFAEnrollmentDialog.tsx`
- [x] Found while testing: a secret soft-deleted before #754 (a tombstone) still holds the unique `user_id`. The upsert now treats it as no secret and brings it back as a fresh, unverified enrolment, still tenant-scoped — `gorm_mfa_repository.go`, `gorm_mfa_enrolment_test.go` (`…ReusesATombstone`, SQLite and Postgres). On master such an account got 400 forever.
- [x] Live check on a throwaway Postgres: setup ×3 → one row, 8 codes; first QR code refused, last accepted; verified → 409; forced code-insert failure → 500 generic, secret and codes unchanged; verified tombstone → setup 200, verify 200
