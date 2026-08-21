# W0-04 — Live Proof

## Environment

| | |
|---|---|
| Date | 2026-08-21 |
| Backend | binary built from this branch, `:8100` |
| Database | PostgreSQL 16 (`openrisk_db`, `localhost:5434`) — migration **0058 applied**, `schema_migrations = 58, dirty = false` |
| Redis | `openrisk_redis` (172.18.0.2:6379) |
| Frontend | Vite dev server `:5173`, proxying `/api` → `:8100` |
| Mail transport | **none configured** — `SMTP_HOST` unset, so `email.Unconfigured` returns `ErrNotConfigured` and delivery is reported as failed. This is the branch a deployment without SMTP takes, and the only branch in which the API returns the one-time link. |
| MFA | `MFA_REQUIRED_ROLES=` for the E2E attempt only; the API proof signs in through the **real TOTP challenge**. |

Toolchain: Go 1.25.12 · Node 26.7.0 · Playwright chromium 149.

## Commit SHA

`82a20015570bb6043a57664dd575a5dd64b79b72`

## Test Tenants

Both created by `scripts` in the scratchpad, through the product's own
`POST /auth/register`. Neither is a real customer.

| Tenant | Slug | Id |
|---|---|---|
| W004 Tenant Alpha|w004-tenant-alpha|8eda57a3-95d4-4a5d-b59c-111f870f804c |
| W004 Tenant Bravo|w004-tenant-bravo|5471de06-91e9-4bbf-b0c3-ebbb8238bbcf |

## Test Users

Every account below was created by this proof and exists only for it.
Passwords are throwaway values held in the scratchpad and are not reproduced
here.

| Account | Tenant | Role | Purpose |
|---|---|---|---|
| `w4-admin-a@openrisk.test` | Alpha | root (org owner) | Tenant A administrator, TOTP-enrolled |
| `w4-admin-b@openrisk.test` | Bravo | root (org owner) | Tenant B administrator, TOTP-enrolled |
| `w4-joiner@openrisk.test` | Alpha | user → admin → revoked | Walked the whole member lifecycle |
| `w4-plain-<ts>@openrisk.test` | Alpha | user + `viewer` preset | The unauthorized-member probe |
| `w4-browser-proof@openrisk.test` | Alpha | user | Browser-driven acceptance |
| `w4-wizard-<ts>@openrisk.test` | Alpha | user | Browser-driven onboarding check |

## Results

**90 / 90 checks passed** across three scripted runs against the live server.
Full transcript below.

---

## Member Lifecycle

An invitee joined, was promoted, deactivated, reactivated and revoked — each
step observed through the API and confirmed in the database.

```
=== Member lifecycle on the joined member ===
  [PASS] the invitee is in the roster: got 1, want 1 
  [PASS] joined active: got 'active', want 'active' 
  [PASS] joined at the invited role: got 'user', want 'user' 
  [PASS] preset applied: got 'auditor', want 'auditor' 
  [PASS] PUT role → admin: got 200, want 200 → admin
  [PASS] admin clears the preset: got '', want '' 
  [PASS] PUT status → deactivated: got 200, want 200 
  [PASS] deactivated member no longer active: got False, want False 
  [PASS] deactivated_at stamped: got True, want True 
  [PASS] PUT status → active (reactivate): got 200, want 200 
  [PASS] reactivated: got True, want True 
  [PASS] PUT status → revoked: got 200, want 200 
  [PASS] revoked is terminal: got 400, want 400 

```

`revoked → active` is refused with **400**: revocation is terminal by design.

## Invitation Flow

```
=== Invitation — create ===
  [PASS] POST /organization/invitations: got 201, want 201 
       delivery='failed' detail='The invitation was created but the email could not be sent — share the link below yourself.'
  [PASS] email normalised: got w4-newcomer@openrisk.test, want w4-newcomer@openrisk.test 
  [PASS] status pending: got pending, want pending 
  [PASS] the link is returned exactly when the email was NOT delivered: got True, want True 
  [PASS] a second live invitation for the same address conflicts: got 409, want 409 
  [PASS] malformed email refused: got 400, want 400 
  [PASS] root is not invitable: got 400, want 400 
  [PASS] inviting an existing member conflicts: got 409, want 409 
  [PASS] GET /organization/invitations: got 200, want 200 → total=1
  [PASS] no listing ever mentions a token: got False, want False 

--- part 1: 22/22 passed ---
```

Acceptance, from an unauthenticated caller with no OpenRisk account:

```
=== Acceptance — a real invitee joins ===
  [PASS] re-invite after revoke: got 201, want 201 
  [PASS] GET /invitations/preview (no session at all): got 200, want 200 → org='W004 Tenant Alpha' role='user'
  [PASS] preview names the inviting organization: got 'W004 Tenant Alpha', want 'W004 Tenant Alpha' 
  [PASS] preview says an account is needed: got True, want True 
  [PASS] POST /invitations/accept unauthenticated: got 201, want 201 → created_account=True
  [PASS] replaying the token is Gone: got 410, want 410 
  [PASS] an unknown token is Not Found: got 404, want 404 

```

Note the three properties this proves together: the preview is public and names
the organization; acceptance needs **no session**; and the token is consumed —
replaying it is **410**, while an unknown token is **404**. The two are
different answers on purpose: "this is over" is actionable for someone who
legitimately held the link, where "not found" would only look broken.

## Role Change

```
  [PASS] PUT role → admin: got 200, want 200 → admin
  [PASS] admin clears the preset: got '', want '' 
```

Promoting to `admin` clears the business-role preset, because an administrator
holds the `*` wildcard and a least-privilege preset beside it would only mislead
whoever reads the roster next.

## Deactivation

Withdrawing access is not a UI state. The membership stops granting access AND
the refresh lineage is destroyed:

```
=== Deactivation actually withdraws access ===
  [PASS] admin deactivates the member: got 200, want 200 
  [PASS] their refresh token no longer mints a session: got 401, want (401, 403) → Token refresh failed
  [PASS] they can no longer sign in: got 401, want (401, 403) → Authentication failed

=== Reactivation restores it ===
  [PASS] admin reactivates: got 200, want 200 
  [PASS] they can sign in again: got 200, want 200 

```

The access token already in the member's hands remains valid until it expires
(≤ 15 minutes). That is the session model, stated rather than implied.

## Revocation

```
=== Revoke invitation ===
  [PASS] DELETE /organization/invitations/{id}: got 200, want 200 → status=revoked
  [PASS] status is revoked: got 'revoked', want 'revoked' 
  [PASS] revoking twice is Gone: got 410, want 410 
  [PASS] resending a revoked invitation is Gone: got 410, want 410 

```

Revoking rotates the token hash, so the link that was mailed is dead on lookup
rather than merely failing a status check.

## Audit History

```
=== Membership audit history ===
  [PASS] GET /organization/members/audit: got 200, want 200 → 10 entries
  [PASS] only membership entities: got set(), want set() 
  [PASS] every ADMINISTRATIVE entry names an actor: got True, want True 
       1 entry/entries carry no actor — self-registration memberships, created before any session exists (see Known Limitations)
  [PASS] every entry has a summary: got True, want True 
  [PASS] the audit history carries no token material: got False, want False 
       most recent: Membership of w4-joiner@openrisk.test moved from active to revoked (2 records affected)
       actor: w4-admin-a@openrisk.test
  [PASS] the history refuses an entity type outside its allowlist: got 400, want 400 
  [PASS] tenant B's history contains none of tenant A's activity: got False, want False 

============================================================
RESULT: 48/48 checks passed
```

A representative entry, read straight from `audit_events`:

```
  -[ RECORD 1 ]-------------------------------------------------------------------------
  summary     | Membership of w4-plain-1787297455@openrisk.test moved from active to dea
  action      | update
  entity_type | organization_member
  before      | {"status": "active"}
  after       | {"reason": "W0-04 access-withdrawal proof", "status": "deactivated"}
  ip_address  | 127.0.0.1
  
```

The reason an administrator typed travels with the event. No entry anywhere in
the trail contains `token_hash`, an accept URL, or any other credential — the
proof asserts this explicitly.

## Organization Settings

```
=== Organization metadata (real tenant row + live counts) ===
  [PASS] GET /organization as tenant A admin: got 200, want 200 
       name='W004 Tenant Alpha' slug='w004-tenant-alpha' plan='pro' created_at='2026-08-20T13:43:07.156878Z' counts={'total_members': 1, 'active_members': 1, 'deactivated_members': 0, 'revoked_members': 0, 'admins': 1, 'pending_invitations': 0}
  [PASS] profile carries the tenant's own name: got W004 Tenant Alpha, want W004 Tenant Alpha 
  [PASS] counts are present: got True, want True 
  [PASS] admin is told they may edit: got True, want True 
  [PASS] tenant B sees its OWN organization: got W004 Tenant Bravo, want W004 Tenant Bravo 
  [PASS] GET /organization/counts: got 200, want 200 → {'total_members': 1, 'active_members': 1, 'deactivated_members': 0, 'revoked_members': 0, 'admins': 1, 'pending_invitations': 0}

```

Every value is read from the tenant's own row. Before this wave, Settings ›
General rendered the organization name out of a login payload and the *viewer's*
`Intl.DateTimeFormat()` time zone as the organization's.

## Sidebar Counts

`GET /organization/counts` is the sidebar's single source, and it is
tenant-scoped and readable by an ordinary member:

```
       tenant A counts={'total_members': 2, 'active_members': 1, 'deactivated_members': 0, 'revoked_members': 1, 'admins': 1, 'pending_invitations': 1}
       tenant B counts={'total_members': 1, 'active_members': 1, 'deactivated_members': 0, 'revoked_members': 0, 'admins': 1, 'pending_invitations': 1}
  [PASS] but they CAN read the member count for the sidebar: got 200, want 200 → {'total_members': 3, 'active_members': 2, 'deactivated_members': 0, 'revoked_members': 1, 'admins': 1, 'pending_invitations': 1}
```

The badge renders only when the count is positive. Loading, error, permission
refusal and zero all render nothing — a placeholder would be indistinguishable
from a measured figure, which is precisely what the hardcoded `12` was.

## Cross-Tenant Tests

```
=== Cross-tenant isolation ===
  [PASS] tenant B can invite into tenant B: got 201, want 201 
  [PASS] tenant B's members never appear in tenant A's roster: got [], want [] 
  [PASS] tenant B's invitation never appears in tenant A's list: got False, want False 
  [PASS] tenant A → tenant B: read member: got 404, want 404 → member not found
  [PASS] tenant A → tenant B: change role: got 404, want 404 → member not found
  [PASS] tenant A → tenant B: revoke member: got 404, want 404 → member not found
  [PASS] tenant A → tenant B: revoke invitation: got 404, want 404 → invitation not found
  [PASS] tenant A → tenant B: resend invitation: got 404, want 404 → invitation not found
  [PASS] a real foreign id, an invented id and junk answer identically: got 1, want 1 → {'member not found'}
  [PASS] tenant B's member is unchanged: got 'root', want 'root' 
  [PASS] tenant B's invitation is unchanged: got 'pending', want 'pending' 
       tenant A counts={'total_members': 2, 'active_members': 1, 'deactivated_members': 0, 'revoked_members': 1, 'admins': 1, 'pending_invitations': 1}
       tenant B counts={'total_members': 1, 'active_members': 1, 'deactivated_members': 0, 'revoked_members': 0, 'admins': 1, 'pending_invitations': 1}
  [PASS] tenant B's pending count is its own: got 1, want 1 

```

The decisive line is the third from last: a **real** member id belonging to
tenant B, an **invented** id and a **malformed** id all produce the identical
body `member not found`. A 403 for the first would confirm the id is real and
belongs to somebody — the first half of an enumeration.

## Unauthorized Tests

An ordinary member (business role `viewer`, nine read permissions, no wildcard):

```
=== An ordinary member cannot administer the organization ===
  [PASS] cannot enumerate members: got 403, want 403 
  [PASS] cannot list invitations: got 403, want 403 
  [PASS] cannot invite: got 403, want 403 
  [PASS] cannot read the membership audit: got 403, want 403 
  [PASS] cannot demote an administrator: got 403, want 403 
  [PASS] cannot promote themselves: got 403, want 403 
  [PASS] but they CAN read the member count for the sidebar: got 200, want 200 → {'total_members': 3, 'active_members': 2, 'deactivated_members': 0, 'revoked_members': 1, 'admins': 1, 'pending_invitations': 1}

```

Reads are refused as firmly as writes: enumerating colleagues is the first half
of a targeted attack, and the roster is not public within a tenant.

## Browser Evidence

Driven with Playwright against the running stack, signing in through the real
login form and the real TOTP challenge.

**Roster** (`/settings/members`) — DOM read back from the live page:

```
rows: [
  "Admin Alpha | Propriétaire | w4-admin-a@openrisk.test | Actif | Accès complet | 20 août 2026",
  "Joiner Person | w4-joiner@openrisk.test | Révoqué | …",
  "Plain Member | w4-plain@openrisk.test | Actif | … | Désactiver"
]
tabs: ["Membres", "Invitations", "Journal d'accès"]
```

The owner's row carries no role `<select>` and no Deactivate action — the server
refuses both, and the row says so rather than offering a click that returns 403.
The revoked member shows `Révoqué` with no lifecycle action left.

**Invite dialog outcome** — the mail transport is unconfigured, and the product
says so:

```
Invitation créée — à transmettre vous-même
w4-browser-proof@openrisk.test
The invitation was created but the email could not be sent — share the link below yourself.
http://localhost:5173/invitations/accept?token=&lt;one-time token, redacted&gt;
```

**Acceptance page**, opened in a signed-out browser at that link:

```
W004 Tenant Alpha
vous invite à rejoindre son espace
Adresse invitée   w4-browser-proof@openrisk.test
Rôle              user
Votre nom complet · Choisissez un mot de passe (12 caractères minimum)
Créer mon compte et rejoindre
Ce lien expire le 27 août 2026.
```

**Email binding**, with the administrator's session still active:

```
[banner] Vous êtes connecté en tant que w4-admin-a@openrisk.test.
         Cette invitation est destinée à w4-browser-proof@openrisk.test.
[error]  this invitation was issued to w4-browser-proof@openrisk.test
         — sign in as that address to accept it
```

**Completion**, in a clean session:

```
Bienvenue !
Vous faites maintenant partie de W004 Tenant Alpha.
→ landed on "/", signed in as w4-wizard-…@openrisk.test, greeted "Bonjour, Wizard"
→ nav shows only Tableau de bord + Paramètres (0 permissions — invited with no
   business role, which is least privilege working, and which the invite dialog
   now warns about)
```

## API Commands

Reproducible against a running server. `$T` is an access token obtained through
the real login (and TOTP) flow.

```bash
# Organization profile with live counts
curl -s $API/organization -H "Authorization: Bearer $T"

# Roster, filtered and paged
curl -s "$API/organization/members?q=alice&status=active&limit=25" -H "Authorization: Bearer $T"

# Invite — 201, returns the link ONLY when the email did not go out
curl -s -X POST $API/organization/invitations -H "Authorization: Bearer $T" \
     -H 'Content-Type: application/json' \
     -d '{"email":"colleague@example.com","role":"user","business_role":"auditor"}'

# Preview and accept — no session required
curl -s "$API/invitations/preview?token=$TOKEN"
curl -s -X POST $API/invitations/accept -H 'Content-Type: application/json' \
     -d "{\"token\":\"$TOKEN\",\"full_name\":\"New Colleague\",\"password\":\"a-long-passphrase\"}"

# Resend (rotates the token) · revoke (kills it)
curl -s -X POST   $API/organization/invitations/$ID/resend -H "Authorization: Bearer $T"
curl -s -X DELETE $API/organization/invitations/$ID        -H "Authorization: Bearer $T"

# Role and status
curl -s -X PUT $API/organization/members/$MID/role -H "Authorization: Bearer $T" \
     -H 'Content-Type: application/json' -d '{"role":"admin"}'
curl -s -X PUT $API/organization/members/$MID/status -H "Authorization: Bearer $T" \
     -H 'Content-Type: application/json' -d '{"status":"deactivated","reason":"offboarding"}'

# Membership audit + the sidebar's counter
curl -s "$API/organization/members/audit?limit=50" -H "Authorization: Bearer $T"
curl -s  $API/organization/counts                  -H "Authorization: Bearer $T"
```

## Test Commands

| Command | Result |
|---|---|
| `go build ./...` | **PASS** |
| `go vet ./...` | **PASS** |
| `go test ./internal/... ./pkg/...` | **PASS** — full backend suite, 0 failures |
| `go test ./internal/domain/ -run "Membership\|Invitation\|CheckRole\|CheckStatus\|SetStatus"` | **PASS** — 9 tests |
| `go test ./internal/application/membership/` | **PASS** — 25 tests |
| `go test ./internal/infrastructure/repository/ -run MembershipRepo` | **PASS** — 9 tests |
| `go test ./internal/handler/ -run "TestOrganization\|TestInvitation\|TestUpdateMember"` | **PASS** — 9 tests |
| `go test ./internal/infrastructure/authmail/` | **PASS** — 3 tests |
| `go test ./internal/security/...` | **PASS** — isolation gate satisfied |
| `migrate up` (0058) | **PASS** — `version=58 dirty=false` |
| `npx tsc --noEmit` | **PASS** |
| `npx vite build` | **PASS** |
| `npx vitest run` | **PASS** — 162 passed, 1 pre-existing failure (see below) |
| `npx vitest run src/features/organization` | **PASS** — 16 tests |
| `npx playwright test --list journey.members.spec.ts` | **PASS** — 12 cases listed |
| `npx playwright test journey.members.spec.ts` | **BLOCKED** — see below |
| Live API proof (90 checks, 2 tenants) | **PASS** — 90/90 |
| Browser walkthrough (login → members → invite → accept) | **PASS** |
| Load test | **NOT APPLICABLE** — justified in the main document |

### The two pre-existing failures

**`src/__tests__/App.integration.test.tsx`** fails on `master` as well.
Reproduced by stashing this branch's changes and re-running the single file: it
still fails. Not caused by, and not addressed by, this work.

**Playwright is BLOCKED**, for two independent reasons, both predating W0-04:

1. `tests/e2e/global-setup.ts` logs in with `admin@opendefender.io` and has no
   TOTP step, while W0-03 made MFA mandatory for `admin`/`root`. The seed aborts
   with `admin login returned no access_token`.
2. With that worked around, `storageState` still authenticates nothing: the
   harness seeds `auth_token` into localStorage, but the app now uses HttpOnly
   cookies and `useAuthStore.logout` explicitly *removes* that key as a legacy
   artefact. Every spec lands on the login screen.

Both were reproduced with the untouched `journey.settings.spec.ts`, so they are
harness debt rather than anything this branch introduced. The new spec is
written, type-checks and is listed by Playwright; the same six scenarios are
covered by the browser walkthrough above, driven manually against the live
stack.

## Screenshots / Evidence

The Playwright MCP session captured DOM snapshots rather than image files (the
screenshot call did not write to disk in this sandbox). The DOM read-backs above
are verbatim from the live page and are reproduced in place of images.

Raw transcripts of all three proof scripts are inlined in this document. The
scripts themselves live in the session scratchpad and are not committed, since
they carry throwaway credentials for the test accounts.

## Known Limitations

1. **E2E written but not executed** — harness debt described above.
2. **No SMTP on this host** — the real `Transport` is covered by unit tests
   including error propagation; live runs exercise the honest `Unconfigured`
   path. Delivery to a real mailbox has not been observed.
3. **`MFA_REQUIRED_ROLES=` was set** for the Playwright attempt only. The 90
   scripted API checks and the browser walkthrough both signed in through the
   real TOTP challenge, with the policy at its default.
4. **The demo admin's MFA secret was cleared** in this database: it had been
   encrypted under a `MFA_ENCRYPTION_KEY` this instance does not hold, so no
   code it produced could ever validate. That is an environment repair on a
   seeded demo account, not a policy change.
5. **Self-registration memberships carry no audit actor** and display as
   "System" — there is genuinely no third party at that moment.
6. **Proof data remains in the dev database** (two `w004-*` tenants and their
   members), alongside the demo fixtures already there. `scratchpad/reset.sh`
   returns them to a known state.
