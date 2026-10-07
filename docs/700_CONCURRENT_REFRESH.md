# Spec — #700 two tabs refreshing at once must not sign the user out

Status: approved by owner 2026-10-07 (proof and tests only, no client lock) · done, see Results · Branch: `fix/700-concurrent-refresh-two-tabs` · Milestone: trust-v1

## Objective

A user with OpenRisk open in two tabs of the same browser was signed out of every
tab when both renewed their session at the same instant. The losing
`POST /auth/refresh` answered `401 REFRESH_REUSE_DETECTED` and cleared the
session cookies, and because tabs share one cookie jar, that wiped the cookies
the winning request had just set.

Success means that two tabs (or a retry, or an app waking up) refreshing together
both stay signed in. A refresh token replayed after the chain has moved on is
still treated as theft.

## What changed since the issue was filed

The backend fix exists already. The owner decided **D-048** on 2026-09-23 (grace
window plus derived successor), and #777 implemented it. It is merged on master
(`e6cff71c`, `635dd9e4`).

| Piece | File | Fact (read 2026-10-07) |
|---|---|---|
| Grace window | `backend/internal/auth/token.go:62` | `RotationGracePeriod = 10s` |
| Rotation | `backend/internal/auth/token.go` `RefreshTokenPair` | Inside the window, a replay gets the **same** successor, `HMAC(key, family_id ‖ presented)`. Outside the window, on a fingerprint mismatch, or once the successor has been spent, the family is revoked and the call returns `ErrRefreshTokenReuse` |
| Unit proof | `backend/internal/auth/token_test.go:187` `TestRefresh_ConcurrentRotation_WithinGraceAllSucceed` | 8 goroutines, one token: all succeed with one successor and the lineage does not fork. Past the window the original token still revokes the family |
| Reuse proof | `token_test.go:131` `TestRefresh_Reuse_RevokesFamily`, `:239` `TestRefresh_ReplayAfterChainMovedOn` | unchanged security semantics |
| Handler | `backend/internal/handler/auth/handler.go:427-438` | still clears cookies on `ErrRefreshTokenReuse`. That is correct now, because the error only fires on genuine reuse. **The comment still says "or a lost concurrent-rotation race", which is stale** |
| Frontend | `frontend/src/lib/api.ts:82-108` | one in-flight refresh per tab; a failed refresh goes to `/login`. CSRF is read from the `or_csrf` cookie, so the last writer wins consistently. The access token is per tab and in memory, and each tab's token is a valid JWT |

No issue comment and no PR on #700 records any of this. Nobody has shown the
issue's criteria 1, 3 and 5 against the running product.

## Assumptions (correct me or I proceed with these)

1. **No new design.** D-048 is the decision the issue's design note asked for. There is nothing new to escalate, and `docs/DECISIONS.md` is not touched.
2. **No client-side cross-tab lock** (Web Locks / `BroadcastChannel`). The server already makes concurrent refresh idempotent. A lock would add auth-client surface for no change a user can see. It stays available as defence in depth if the live pass shows a gap.
3. **Real concurrency is proved on Postgres through the live stack, not by a new pg test harness.** The handler tests run on sqlite, which serialises writes, so the `RowsAffected != 1 → re-read` branch never really races there. The issue's own evidence was taken at the API level against the local stack, and the same method serves as proof here.
4. **Criterion 4's backend test is added at the HTTP level** (`app.Test`, sqlite). The existing tests cover the token manager. What they do not cover is that the *handler* answers 200 to every request in a burst, re-issues the same `or_refresh`, and never sends a clearing `Set-Cookie`.

## Scope

In:
- **T1:** a handler-level test of a concurrent refresh burst plus a reuse-after-window regression test. Fix the stale comment at `handler.go:427`.
- **T2:** a Playwright test with one browser context, two tabs, the `or_access` cookie removed, and both tabs reloading at once. Both must stay signed in.
- **T3:** a live pass on Postgres: the issue's API burst and two-tab scenario, 3 runs each, with output pasted into the issue.

Out:
- client-side cross-tab lock (assumption 2);
- turning a network error or 5xx on `/auth/refresh` into something other than `/login` (`api.ts` `.catch(() => false)`). This is a separate defect; I open an issue if the live pass reproduces it;
- residual CSRF race: tab A reads `or_csrf`=X just as tab B's refresh lands Y, and a *mutation* sent in that microsecond window gets a 403. It is noted here but not fixed unless the live pass hits it.

## Commands

```
Backend unit:   cd backend && go test ./internal/auth/ ./internal/handler/auth/ -run 'Refresh' -count=1 -race
Backend full:   cd backend && go test ./... -count=1           # route/authz ratchets
Frontend types: cd frontend && npx tsc -b --noEmit
Playwright:     cd frontend && OPENRISK_BASE_URL=<vite> E2E_API_URL=<api>/api/v1 npx playwright test e2e/session-tabs.spec.ts --repeat-each 3 --workers 1
Live stack:     throwaway pg/redis on spare ports, server from repo root (live-boot recipe)
```

## Project structure touched

```
backend/internal/handler/auth/refresh_concurrency_e2e_test.go   new — T1
backend/internal/handler/auth/handler.go                        comment only — T1
frontend/e2e/session-tabs.spec.ts                               new — T2
docs/700_CONCURRENT_REFRESH.md                                  this spec
```

## Code style

Follow the existing handler e2e tests (`logout_e2e_test.go`, `mfa_challenge_e2e_test.go`): a
fixture constructor, `app.Test(req, -1)`, `require`. Fire the burst from goroutines released
by a closed channel, as in `TestRefresh_ConcurrentRotation_WithinGraceAllSucceed`.

```go
for i, resp := range responses {
	require.Equal(t, fiber.StatusOK, resp.StatusCode, "request %d: a burst is not a theft", i)
	require.Equal(t, refreshCookie(responses[0]), refreshCookie(resp), "request %d forked the lineage", i)
	require.False(t, clearsSession(resp), "request %d cleared the session cookies", i)
}
```

The Playwright test lives in its own file, `e2e/session-tabs.spec.ts`, and not in `e2e/auth.spec.ts`.
That suite probes `/health`, while the route is `/api/v1/health`, so it skips itself on every
stack. That is tracked separately. The new file follows the same pattern: it creates an account over the API, skips when
the API is unreachable, and uses `context.clearCookies({ name: 'or_access' })`. It then runs
`Promise.all([a.reload(), b.reload()])` and asserts that neither URL is `/login`, that
`/auth/me` returns 200 from both tabs, and that the jar holds `or_access` and `or_refresh`.

## Testing strategy

| Criterion | Proof |
|---|---|
| 1. Both tabs keep a valid session | T2 (Playwright) and T3 (live, 3 runs) |
| 2. Old-token replay still revokes the family and answers `REFRESH_REUSE_DETECTED` | existing `TestRefresh_Reuse_RevokesFamily` and `TestRefresh_ReplayAfterChainMovedOn` stay green and unmodified. T1 adds the HTTP-level check (age the rotation, then 401 with the code and clearing cookies) |
| 3. A losing request never clears the winner's cookies | T1 asserts no clearing `Set-Cookie` in a burst. T3 checks the cookie jar after the API burst |
| 4. Backend concurrent and reuse tests | T1 with `-race` |
| 5. Playwright two tabs | T2, with output pasted |

The tests per CLAUDE.md rule 4 (`_Success` / `_NotFound` / `_Unauthorized`) map onto
the refresh handler as: burst succeeds, unknown token gives 401, reused token gives
401 `REFRESH_REUSE_DETECTED`.

## Boundaries

- **Always:** keep reuse-detection tests unchanged and green; run the full `go test ./...` before the PR; do the live pass with figures; post the resume-anchor comment on #700.
- **Ask first:** any change to `RotationGracePeriod`, to the successor derivation, or to when cookies are cleared. These are D-048 territory.
- **Never:** weaken reuse detection to make a test pass, or merge the PR.

## Success criteria

- [x] T1 is green under `-race`, and the existing reuse tests pass unchanged.
- [x] T2 is green against the local stack, with output pasted.
- [x] T3: the API burst gives every response 200 with the same `or_refresh` and the jar is intact afterwards, 3 out of 3 runs. The two-tab reload passes 3 out of 3.
- [x] Old-token replay outside the window, checked live: 401 `REFRESH_REUSE_DETECTED`, and the family is gone.
- [ ] PR open with `Closes #700`, and the issue carries `status:in-review`.

## Results (2026-10-07, throwaway Postgres 16 + Redis 7, master `cefe453d`)

| Check | master (with #777) | before #777 (`9dd1c3ee`) |
|---|---|---|
| API, 2 concurrent refreshes, 3 runs | 3/3: both 200, one `or_refresh`, no clearing, jar intact, `/auth/me` 200, next refresh 200 | one 200 and one `401 REFRESH_REUSE_DETECTED`; the jar ends **empty**, `/auth/me` 401 |
| API, 8 concurrent refreshes | 8 × 200, one `or_refresh` | — |
| Replay of the original token 11 s later | 401 `REFRESH_REUSE_DETECTED`, clears all 3 cookies; the legitimate successor then gets 401 (family revoked) | — |
| Family integrity in DB | each family holds exactly 1 live token (no fork) | — |
| Playwright `session-tabs.spec.ts`, `--repeat-each 3` | 6/6 passed | 6/6 failed (`REFRESH_REUSE_DETECTED`) |
| `TestRefreshHandler_*` (`-race`) | 3/3 pass | the burst test fails; the reuse test passes |
| `go test ./...` | 80 packages ok | — |

Found on the way, outside this issue:

- the login screen crashes on master (`ReferenceError: notice is not defined`, `AuthScreen.tsx`). A merge of master into #872 (`990b93f3`) dropped the prop. The Playwright pass ran with the one-line fix applied locally; it ships in its own P0 issue;
- `e2e/auth.spec.ts` probes `/health` instead of `/api/v1/health`, so the whole auth suite skips silently.

## Open questions

None. The owner approved proof and tests only. The client-side Web Lock was not built.
