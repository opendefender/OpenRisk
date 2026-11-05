---
name: sign-out-others-posture
description: #727 reviews 2026-10-21 — PAT mint + single-pass DELETE fixed 95e8b85c; sid claim PASS; follow-ups #1059/#1060/#1061 PASS (re-issue sink removed)
metadata:
  type: project
---

Review r1 (660e87e2): PAT minted a full session via re-issue (HIGH); RevokeAllExcept single DELETE (MEDIUM).
Review r2 (95e8b85c, PASS): handler refuses is_pat 403; GormSessionRepository.sweep loops until empty (live counted, spent uncounted);
access tokens carry `sid` = refresh family_id, used only to mark Current and as the keep-family in RevokeAllExceptFamily (user_id scoped, so a foreign sid only over-revokes the caller's own sessions).

Residual (non-blocking):
- Re-issue path still mints a 30-day session from any sid-less access token (legacy, 15-min deploy window). Scanner agent tokens are sid-less but sub=agent/config ID, so orgResolver refuses; harmless. Should be deleted after one AccessTokenTTL.
- Revocation never blacklists access-token JTIs: a revoked device keeps a valid token (and its stale sid) for up to 15 min and can call sign-out-others itself. Pre-existing.
- Keep-hash path deletes the caller's own spent predecessor: a grace-window replay becomes "invalid", not served (fails closed).

Follow-ups r3 (2026-10-21, all PASS, read-only review of stacked branches):
- #1059 HasLiveSession gate: stale sid -> 401 (no `code`, so frontend neither refreshes nor redirects), nothing revoked. Race with caller's own rotation (claim -> resolver -> successor insert) = spurious 401, fails safe.
- #1060 re-issue removed; IssueSessionForOrg now only in change_password + switch_org (both pre-existing, authenticated).
- #1061 logout RevokeSession(sub, sid) via ValidateToken (any type, nil blacklist checker). Typed MFA tokens are minted with uuid.Nil sid, so they revoke no family. Blacklisted token can re-trigger revoke of an already-dead family: no-op.
- Still open: access-token JTIs not blacklisted on revoke-others/revoke-one (pre-existing).

**Why:** any new "mint a session" path is a credential-upgrade sink; check every authenticator that reaches it (PAT, scanner agents, MFA tokens), not just JWT.
**How to apply:** on any endpoint that issues tokens, grep for is_pat refusal, sweep-based revocation, and every caller of GenerateAccessToken (sid-less minters).

r4 2026-10-22 (#1116): merge fe9422c5 had silently dropped HasLiveSession/RevokeAllExceptFamily/sweep (build broke). Restoration byte-identical to 7168dd51 plus the D-064 stamp. PASS. Lesson: after any merge touching gorm_session_repository.go, diff against the #727/#1059 commits; a conflict resolution "keep one side whole" erases security fixes.
