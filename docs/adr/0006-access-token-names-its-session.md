# 0006 — The access token names its session

Status: **accepted**
Proposed: 2026-10-21, on #727
Accepted: 2026-10-21 by the owner, who asked for #727's criterion 2 in the same PR (D-073, option B)
Related: D-047 (#720), D-048 (#700), #725, #777

## Context

A session is a refresh-token family: one row in `refresh_tokens` per rotation, all sharing
`family_id`, with exactly one live row (`rotated_at IS NULL`). The refresh token travels in the
`or_refresh` cookie, scoped to `Path=/api/v1/auth/refresh` so that a credential able to mint new
sessions is not attached to every API call.

That scope has a cost: no other route can tell which session is the caller's. Two features need
to:

- `GET /auth/sessions` marks "this device", so the SPA can warn before someone revokes the device
  they are holding. With no refresh token in the request, no row was ever marked.
- `DELETE /auth/sessions/others` keeps the caller's session. With no refresh token, the keep-hash
  was empty and the caller was signed out too (#727). The first fix re-issued a session for the
  caller, as the password change does (D-047). That keeps the caller signed in, but it counts the
  caller's replaced session as revoked and cannot help the listing.

The options weighed in D-073 were: leave the list unmarked; widen the refresh cookie to
`/api/v1/auth`; or put the session in the access token.

## Decision

1. **The access token carries `sid`**, the session's `family_id`, as a string claim. It is set
   by `pkg/auth.GenerateSessionAccessToken`, called from the two places that mint an access token
   with a refresh token: `TokenManager.generatePair` (sign-in, SSO, org switch, MFA completion)
   and `TokenManager.issueSuccessor` (rotation). The family id does not change on rotation, so
   every access token of one session carries the same `sid`.
2. **Tokens without a session carry no `sid`**: scanner agent tokens and the MFA step tokens go
   through `GenerateAccessToken` / `GenerateTypedToken`, unchanged. A personal access token is
   not a JWT and has none either.
3. **The auth middleware exposes it** as `RequestContext.SessionID` (`uuid.Nil` when absent or
   malformed). It is read from a token whose signature has already been checked, so it cannot be
   forged into naming someone else's session. Every use is also scoped by `user_id`.
4. **Consumers prefer `sid`, then fall back**:
   - the session list marks the live row whose `family_id` equals `sid`, else the row whose hash
     matches a presented `X-Refresh-Token`;
   - "sign out other devices" keeps the `sid` family, else the presented token's row, else
     re-issues a session for the caller (the #727 fix, kept for access tokens minted before this
     ADR, which live at most `AccessTokenTTL`, 15 minutes).
5. **The listing shows live rows only** (`rotated_at IS NULL`). Spent rows stay in the table for
   reuse detection, but they are not devices, and listing them showed a device once per refresh.
6. **Revocation counts live sessions only**, and sweeps until a pass removes nothing (#725).

## What `sid` is not

- Not an authorization input. No route grants anything because of it; it only answers "which of
  *my* sessions is this?".
- Not a revocation handle for access tokens. Revoking a family still leaves its outstanding access
  tokens valid until they expire, as before. A later change could blacklist by `sid`; that would
  need a lookup on every request and is not decided here.
- Not secret. It is a random UUID with no power on its own: it cannot refresh, and knowing it does
  not reveal a token hash.

## Consequences

- The access token grows by one short claim (~45 bytes).
- During the first 15 minutes after a deploy, browsers still hold tokens without `sid`: their
  session is unmarked in the list until their next refresh, and "sign out other devices" uses the
  re-issue path. Nothing breaks.
- `POST /auth/logout` revokes the caller's family from `sid`, scoped to the token's subject,
  although it never sees the refresh cookie (#1061).
- Any future code that mints an access token for a session must use `GenerateSessionAccessToken`.
  The tests in `backend/internal/handler/auth/session_handler_e2e_test.go` go through the real
  `TokenManager` and fail if a session token comes back without `sid`.
