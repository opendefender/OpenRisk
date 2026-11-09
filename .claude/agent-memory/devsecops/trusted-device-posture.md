---
name: trusted-device-posture
description: #879 PASS 2026-10-21; #1116 PASS; #1118 PASS 2026-10-22 (re-issue now TrustsFor on pre-stamp read); remaining LOWs: reset race, re-plant on self sign-out
metadata:
  type: project
---

Reviewed #879 criterion 1 (branch security/879-trusted-device, commit 8ec929fd) on 2026-10-21. Verdict PASS, no blocking finding.

- Signer sound: HMAC-SHA256 checked (hmac.Equal) before parsing, version is inside the MAC, TTL 90d + future-dated refused. Without a valid cookie the locked path is byte-for-byte the old one.
- Residuals (MEDIUM/LOW): GormUserRepository.Update is a whole-row Save(), and login writes LastLogin through it, so a sign-in that loaded the user before a revocation writes devices_revoked_at back (lost update). Every new column on users inherits this. Holders of a valid cookie get a locked-address existence oracle (GetByEmail+Preload Role runs only when locked+cookie, no Argon2 noise). DEVICE_COOKIE_KEY has no prod length floor, and a forged/leaked key mints unlimited device IDs, so no per-account cap. RevokeOtherSessions mints a cookie from a session alone (no password). DEVICE_COOKIE_KEY isn't wired in Helm or compose.
- Tenant-scoped revocation (RevokeUserTokensInTenant) correctly does NOT stamp: the cookie is account-level and doesn't skip MFA or membership checks.

**Why:** the whole-row Save on users already caused the lost-update debt in [[risk-acceptance-posture]]; it now undermines a security revocation.
**How to apply:** any new security column on users: check every userRepo.Update/Save caller for a stale-row write-back. See [[audit-hash-key-posture]] and [[recurring-defect-areas]].

Update 2026-10-22 (#1116, merge-loss fix fe9422c5): GormUserRepository.Update now `Omit("devices_revoked_at")`, so the Save() lost-update residual is closed (verify on next review). RevokeAllExcept/RevokeAllExceptFamily stamp then sweep, no tx, mirroring TokenManager.RevokeAllUserTokens; sweep failure returns 500 before re-issue, so even the caller's cookie stays untrusted (fails closed). Stale-sid callers hit HasLiveSession 401 before any stamp.
- LOW open: session_handler RevokeOtherSessions re-issues on `Verify()+UserID` only, never `TrustsFor(user)` against the PRE-revocation stamp, so a cookie already revoked by an earlier reset/sign-out-others is revived by any live session of the account. Needs a fresh full login, which plants a cookie anyway, so no real gain today. No test covers the re-issue (holder vs non-holder) on either path.

Update 2026-10-22 (#1118/#1119, b94a976e): PASS. callerStillTrusted verifies cookie, then reads user (GetByID returns nil,nil on not-found -> TrustsFor(nil) false) BEFORE the stamp; nil reader / read error -> no re-issue. Mutation (drop TrustsFor) fails both "ended cookie" subtests. Other plant sites (login, MFA challenge, change-password) mint only after a credential proof, so they don't share the flaw; reset and OAuth/SAML plant nothing.
- LOW residuals: concurrent password reset landing between the pre-read and this stamp is overwritten by the later stamp and the caller's cookie is revived (microsecond window, holder was trusted a moment before). Legacy no-refresh caller (Reauthenticate=true) still gets the cookie re-planted while its session is cleared. Test uses a table-reader fake, not GormUserRepository.GetByID; error branch untested.

Update 2026-10-23 (#1129 r3, 47e79224): PASS. X-Refresh-Token path now checks the hash against ListByUser (user-scoped, live = unexpired + rotated_at IS NULL); stale hash -> keep "" -> revoke all + Reauthenticate, and handler plants the cookie only on `keepTrust && !out.Reauthenticate`. No session newly ended vs before (a stale hash's row was dead anyway; old code silently swept a rotated token's live successor while reporting Reauthenticate=false). Mutations of both changes fail tests. Residual LOW: check-then-sweep TOCTOU against a concurrent refresh (pre-existing); reset race filed as #1130.
