# Spec — #725 A revocation racing a refresh rotation

Status: approved by owner 2026-10-02 (Q1 re-scope · Q2 include user-level revokers), implemented · Branch: `fix/725-revocation-sweep` · Milestone: Wave 1 — Product foundation

## Objective

When a refresh-token lineage is revoked (reuse detected, membership lost, password
changed, member deactivated), no refresh token of that lineage may be usable
afterwards, whatever the interleaving with a rotation in flight. If one survives,
whoever holds it keeps a live session the system believes it ended. That holder
may be the person who stole the token.

## Current state (read 2026-10-02, `internal/auth/` identical to `origin/master`)

#725 describes the race that **#775 already fixed** (closed, commit `4a88fbe1`).
#777 (closed) then reworked rotation:

| Piece | File | Fact |
|---|---|---|
| Test named in #725 | `token_test.go` | `TestRefresh_ConcurrentRotation_OneWinner` no longer exists. #777 replaced it with `TestRefresh_ConcurrentRotation_WithinGraceAllSucceed`, because a burst inside `RotationGracePeriod` is now one client asking twice and is not treated as reuse |
| #775 guard | `token.go:420-431` | After inserting the successor, rotation checks that the claimed row (the "witness") still exists. If it is gone, it revokes the family and returns `ErrRefreshTokenReuse` |
| #775 test | `token_test.go:376` `TestRefresh_FamilyRevokedDuringRotation` | Deletes the family between claim and insert, then asserts refusal and 0 rows |
| Revokers | `token.go:531` `revokeFamily`, `:561` `RevokeAllUserTokens`, `:574` `RevokeUserTokensInTenant` | Each is one `DELETE … WHERE` statement |

Verified today:

```
$ go test ./internal/auth/ -run 'TestRefresh' -count=50 -race
ok  	github.com/opendefender/openrisk/internal/auth	51.924s
```

The issue's own Definition of Done command passes on the current code.

## The residual gap (Postgres only)

The witness check works if the revoking `DELETE` has **committed** before the
rotation reads the witness. Under Postgres READ COMMITTED, a `DELETE` reads from a
snapshot taken when the statement starts, and its deletions stay invisible until
it commits. So the following interleaving still leaves a live token:

```
R (revoker)                               W (rotation)
DELETE … family_id=F  ── snapshot t0
                                          INSERT successor S   (commits, t1 > t0)
  …deletes witness (uncommitted)…         SELECT witness → still visible
                                          return S to client
COMMIT  ── S was not in snapshot t0, survives
```

The window is wide whenever R's `DELETE` waits on a row lock held by another
writer in the same family. SQLite serialises writers, so the current test
harness cannot show this. #775 AC4 asked for the fix to hold on Postgres, and
nothing in the repo proves that it does.

## Proposed mechanism: sweep until empty

Each revoker repeats its `DELETE` until a pass removes no rows, with a small
upper bound on passes (5). The rotation path stays as it is.

Ordering argument, to be written next to the witness check:

- Let D be the last sweep, which removed 0 rows. At D's snapshot, every
  committed row of the lineage was already gone, witnesses included.
- A successor S committed **before** D's snapshot would have been visible to D,
  so it was already deleted.
- A successor S committed **after** D's snapshot: its rotation reads the witness
  after S commits, so after D's snapshot, when the witness's deletion has
  already committed. The rotation sees no witness and revokes itself.

The happy path is unchanged: no new lock, no transaction, and no extra query
on an uncontended refresh. Revocation costs one extra `DELETE` that removes
nothing. No schema change and no new dependency.

The alternatives were rejected:
- A transaction with `SELECT … FOR SHARE` on the witness does not close the gap.
  A `DELETE` that was blocked re-checks only the rows it already had; it does not
  pick up new ones.
- A revoked-family tombstone checked on every refresh would need a new table and
  a read on the happy path. That changes the auth design, which needs an
  escalation.
- `SERIALIZABLE` on rotation would cause retries across the app.

## Success criteria

1. On Postgres, a deterministic test holds a row lock so the revoking `DELETE`
   takes its snapshot before the successor commits. The test then asserts that
   no row of the family survives, and that the token handed out (if any) is
   refused on its next use. It fails on the current code and passes after the
   change.
2. The same holds for `RevokeAllUserTokens` and `RevokeUserTokensInTenant`.
   Password reset or change, and member deactivation, have the same window.
3. `go test ./internal/auth/ -run TestRefresh -count=50 -race` is green.
4. `go test ./...` from `backend/` is green.
5. The ordering argument is written in `token.go` next to the witness check and
   next to the sweep.
6. The happy path issues the same queries before and after the change. A test
   counts the statements, or I paste the query log in the issue comment.

## Commands

```
cd backend
go test ./internal/auth/ -run 'TestRefresh' -count=50 -race
DATABASE_URL=postgres://…throwaway… go test ./internal/auth/ -run 'Postgres' -count=20 -race
go test ./...
go vet ./internal/auth/
```

Postgres runs on a throwaway container on a spare port. Pg tests skip when
`DATABASE_URL` is unset, which is the existing pattern
(`gorm_mfa_repository_pg_test.go`).

## Files

- `backend/internal/auth/token.go`: the sweep loop in the three revokers, and the comments.
- `backend/internal/auth/token_pg_test.go` (new): the deterministic Postgres interleaving tests.
- `backend/internal/auth/token_test.go`: one SQLite test that the sweep repeats and stops when a pass removes nothing.

## Boundaries

- Always: keep the tenant and user filters on every `DELETE` exactly as they are.
  Keep revocation best-effort on errors (`revokeFamily`), but return errors from
  the exported revokers as they do today.
- Ask first: any new table, column, or lock on the happy path. Any change to
  `RotationGracePeriod` or successor derivation (#777).
- Never: weaken or delete `TestRefresh_FamilyRevokedDuringRotation` or the
  grace-window tests.

## Decisions (owner, 2026-10-02)

- **Q1.** #725 is re-scoped to the residual Postgres gap. The original symptom
  was fixed by #775.
- **Q2.** `RevokeAllUserTokens` and `RevokeUserTokensInTenant` are in scope.
