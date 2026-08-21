# W0-04 — Organization Member Management

## Executive Summary

Before this wave, an OpenRisk administrator could not add a colleague in any
way that resembles inviting one. `POST /rbac/members` created the user account
immediately and returned a temporary password for the administrator to relay by
hand — no token, no expiry, no revocation, no resend, and a credential passing
through a third party. There was a second, older invitation implementation in
`MultitenantOrgService` that stored a UUID token in clear, took its target
organization from the URL without comparing it to the caller's session, and
never checked the invitee's email on acceptance; its table was not in
`AutoMigrate`, so it had never run.

Membership itself carried a single boolean, `is_active`, which cannot tell "this
person left" from "this person is suspended pending an investigation" — and an
audit trail that cannot tell those apart is not an audit trail. Settings ›
General showed the organization's name lifted from a login payload and the
*viewer's* time zone presented as the organization's, in inputs that saved
nothing. The sidebar rendered the literal strings `12` and `3` as risk and
mitigation counts to every tenant regardless of what they held.

W0-04 replaces all of that with one system: a bearer-token invitation flow with
expiry, rotation-on-resend and revocation; an explicit membership lifecycle
whose state and access flag cannot disagree; role and status changes guarded by
policy the API and the UI both read; a membership audit history; the tenant's
real profile and live counts; and a sidebar badge that shows a number only when
a number was measured.

The whole surface is proven against a running server and a real PostgreSQL
across two isolated tenants — **90/90 live checks** — and by **63 automated
tests** across the domain, use-case, repository and HTTP layers.

---

## Current Architecture

```
Browser
  └─ features/organization/          MembersView · AcceptInvitationPage
       └─ organizationService.ts     typed client, no `any`, no token field
            │
            ▼  HTTP  (tenant NEVER in the request)
  internal/handler/organization_member_handler.go
       │  tenant + actor from middleware.GetContext (the session)
       ▼
  internal/application/membership/   Service — list · get · role · status
       │                             invite · resend · revoke · preview · accept
       │                             organization profile · counts · audit
       ▼
  internal/domain/                   membership_lifecycle.go (states + policy)
       │                             invitation.go (token minting + policy)
       ▼
  internal/infrastructure/repository/gorm_membership_repository.go
       │  organization_id in EVERY WHERE clause, reads and writes alike
       ▼
  PostgreSQL   organization_members · invitations   (migration 0058)
```

Collaborators are attached with `With*` and are nil-safe. A missing mailer
degrades invitation *delivery*, never invitation creation. A missing audit sink
degrades the journal, never the action. This mirrors the contract the rest of
the codebase already uses (Smart Risk, SOAR, the executive dashboard).

---

## Membership Domain Model

```
Organization
User
OrganizationMember
  ├── role           root | admin | user
  ├── business_role  least-privilege preset (see domain/business_roles.go)
  ├── status         invited | active | deactivated | revoked
  ├── is_active      DERIVED from status — never written by hand
  ├── joined_at · created_at · updated_at
  ├── deactivated_at · revoked_at
  └── invited_by_id
Invitation
  ├── organization_id · email · role · business_role
  ├── token_hash     SHA-256 of the bearer token; the plaintext is never stored
  ├── status         pending | accepted | expired | revoked
  ├── expires_at     7 days
  ├── invited_by_id · accepted_at · accepted_by_id · revoked_at · revoked_by_id
  ├── last_sent_at · send_count
  └── created_at · updated_at
```

### The lifecycle

```
INVITED ──accept──▶ ACTIVE ──deactivate──▶ DEACTIVATED ──reactivate──▶ ACTIVE
                      │                          │
                      └────────revoke────────────┴──▶ REVOKED   (terminal)
```

`SetStatus` is the only writer. It refuses an illegal transition, stamps the
matching timestamp, and re-derives `is_active` from the new state. Two fields
free to disagree is exactly how "deactivated in the UI, still able to sign in"
happens, so one place decides.

`BeforeCreate` stamps a status on any membership written without one. This is
not belt-and-braces: GORM sends the Go zero value for a non-pointer string, so
every INSERT carried `status = ''` explicitly and the column's `DEFAULT 'active'`
never applied. Registration, onboarding, seeding and invitation acceptance all
create memberships, and requiring each to remember a column added later is the
arrangement that produced a roster reporting one member and an active count
reporting zero. Reads treat `''` and `NULL` identically, and
`PrepareForAutoMigrate` normalises rows already written.

`EffectiveStatus()` falls back to `is_active` for rows that predate the column,
so no legacy member disappears from the roster.

---

## Invitation Lifecycle

The invitation is a bearer credential — whoever holds the token can join at the
role it names — so it is treated like a password, not like an identifier:

* 32 bytes from `crypto/rand`, base64url. Not a UUID: UUIDs are structured,
  sometimes time-derived, and printed in logs by habit.
* Only `SHA-256(token)` is persisted, with a unique index. A database dump
  cannot be replayed into memberships.
* Comparison is constant-time (`crypto/subtle`), so a timing signal cannot be
  walked into a valid token.
* `json:"-"` on the hash. A test marshals the row and fails if anything
  token-shaped appears — the audit plugin snapshots rows by JSON-marshalling
  them, so that test is what stands between the credential and the trail.
* Expiry is **projected on read**, not swept: a pending invitation past
  `expires_at` reads as expired without any background job having run, so
  correctness does not depend on a worker being alive.

### Sending, re-sending, revoking

```
Admin ─▶ POST /organization/invitations
         validate email · validate role · reject existing member
         reject a live duplicate invitation
         mint token → persist HASH → audit → THEN attempt delivery
```

The invitation is persisted **before** the mail is attempted, so a mail failure
leaves a revocable, resendable invitation rather than a message referring to a
row that was never written.

**Resend rotates the token.** Exactly one token can open the door at a time.
That bounds the number of live credentials behind a button anyone can click,
and it makes "re-send" genuinely invalidate a link that was forwarded to the
wrong person. A 60-second cooldown (`429`) and a cap of 10 sends (`400`) bound
the spam; an accepted invitation is `409` and a revoked one `410`. An **expired**
invitation is deliberately resendable — extending the window is exactly what the
administrator means, and forcing a revoke-then-reinvite round trip only teaches
people to click twice.

**Revoke rotates the token too**, so the mailed link is dead on lookup rather
than merely failing a status check.

### Acceptance

`POST /invitations/accept` takes **no tenant**. The organization and the role
come from the invitation the token resolves to — never from the request — which
is what stops a crafted call joining an arbitrary tenant as an administrator.

| Situation | Outcome |
|---|---|
| No account for the invited address | Account created, membership created, signed in |
| Signed in as the invited address | Membership created |
| Signed in as **someone else** | `403`, naming the invited address |
| Account exists, nobody signed in | `401` — sign in first |
| Prior membership, deactivated | Reinstated at the invited role |
| Prior membership, **revoked** | `403` — re-admission is an administrator's decision, not an old link's |
| Token unknown / malformed / empty | `404`, identical answers |
| Token expired / revoked / already used | `410`, each with its own words |

Acceptance and membership creation happen in **one transaction**, with the
consuming UPDATE conditional on the invitation still being pending — so two
requests racing the same token produce exactly one membership.

An accepted invitation also marks the new member's onboarding complete. The
signup wizard opens by asking the user to set up an organization: the right
question for a founder, the wrong one for someone joining a tenant that already
exists and that they are not permitted to edit. The route guard holds the whole
application shut until onboarding completes, so without this the invitee's first
and only screen would be that form. This is the reasoning `BackfillExistingMembers`
already applies to members who predate the wizard, extended to those who arrive
after it.

---

## Member Lifecycle

| Capability | Endpoint | Guard |
|---|---|---|
| List | `GET /organization/members` | `organization:members:read` |
| Detail | `GET /organization/members/:memberId` | `organization:members:read` |
| Change role | `PUT /organization/members/:memberId/role` | `organization:members:update` |
| Change status | `PUT /organization/members/:memberId/status` | `organization:members:deactivate` |

Listing supports `q` (email or full name, case-insensitive, **in SQL** so that
sorting and paging are correct across pages), `status`, `role`, `limit`,
`offset`, `sort_by` and `sort_dir`. The page size is clamped to 200: an
organization with ten thousand members must not be able to answer one request
with ten thousand rows, whatever the caller asked for. The reported `total` is
the count matching the *filter*, not the size of the page, so a client does not
paginate over a number that shrinks as it pages.

---

## Role Management

`domain.CheckRoleChange` is a pure function, so the API and the UI ask the same
question and get the same answer:

1. the role must be one an administrator may assign (`admin` or `user`; `root`
   moves through an ownership transfer, not a dropdown);
2. **nobody edits their own role** — the privilege check would be judging its
   own judge, and self-promotion is the escalation this exists to stop;
3. the organization owner is not administered through this endpoint;
4. the tenant may never be left with **no active administrator**.

Changing *only* the business-role preset is a real change, not a no-op: for a
scoped member that preset is where their access actually lives.

A role change ends the member's refresh-token lineage, so a demotion takes
effect at the next renewal rather than waiting out whatever session they hold.

---

## Deactivation and Revocation

`domain.CheckStatusChange` carries the same two structural invariants — no
acting on yourself, never strand the tenant without an administrator — plus the
transition legality of the lifecycle.

Withdrawing access is not a UI state:

* the membership stops granting access (`is_active` false, derived from status);
* `login` refuses (W0-03 already rejects a deactivated membership);
* the refresh-token lineage is destroyed, so no new session can be minted;
* the access token already in the member's hands stays valid **until it expires,
  at most 15 minutes**. That is the session model, and this document says so
  rather than letting the UI imply an instant cut-off it does not deliver.

Deactivation is reversible and reactivation clears the marks of the state it
came out of. Revocation is terminal.

---

## Membership Audit

`GET /organization/members/audit` (`organization:audit:read`) serves the
membership slice of the tenant's existing hash-chained audit trail.

Every membership action is journalled with actor, action, entity type and id,
summary, before → after, IP, user agent, request id and timestamp. Two writers
feed it — the GORM plugin (which knows *which rows* changed) and the application
Recorder (which knows *what the action meant*) — and inside an HTTP request both
deposit into a per-request collector so the middleware writes exactly **one**
chained entry per user action.

The entity-type allowlist (`organization_member`, `invitation`) is applied
server-side and a caller-supplied `entity_type` is intersected with it rather
than trusted; otherwise `?entity_type=risk` would turn this into a
general-purpose trail reader for anyone holding `organization:audit:read`.

Never journalled: invitation tokens, password hashes, MFA secrets, recovery
codes, access or refresh tokens. `Invitation.TokenHash` carries `json:"-"`, and
the plugin snapshots by JSON-marshalling the row.

**Honest limitation.** A membership created by *self-registration* carries no
actor — it is written before any session exists, so there is no third party to
attribute it to. Those entries render as "System". Attributing them to somebody
would be worse than saying nothing.

---

## Organization Metadata

`GET /organization` returns the tenant's own row — name, slug, plan, status,
creation date, owner, industry, time zone — with live membership counts and a
`can_edit` flag that is the *server's* answer, so the UI renders read-only for
the same reason the API would refuse a write.

Time zone is shown only when the organization actually set one, so the UI can
say "not set" instead of presenting the viewer's own zone as the organization's.

The profile is read-only and labelled as such: no endpoint writes it yet, and an
input that looks editable and saves nothing is worse than a value that plainly
is not.

---

## Settings Integration

Settings › Members is three tabs over one job, "who has access here": the
roster, the outstanding invitations, and the history of every change to both.
Settings › General renders the real organization profile and live counts.

The previous `MembersPanel` and the `/rbac/members` client have been removed
rather than left beside their replacements. `GET /rbac/business-roles` stays —
the permission catalogue is reference material about the product, not data about
anyone's organization.

---

## Sidebar Data Integration

`NavItem.badge` no longer carries text. It names a **live counter**
(`NavCount`), which the Sidebar resolves from `GET /organization/counts`. There
is no free-text option, so a badge cannot be invented.

A badge renders only when its count is a positive number. Loading, errored,
permission-refused and zero all render **nothing** — a placeholder would be
indistinguishable from a real figure, and the hardcoded `12` this replaced was a
claim nobody measured. Zero is absent because a badge means "something is
waiting", and nothing is.

The remaining badge counts **outstanding invitations**, not headcount: a number
beside a nav item should mean something needs you.

`/organization/counts` is open to any authenticated member. Knowing how many
colleagues you have is not privileged, and gating it would leave the badge
permanently empty for everyone who is not an administrator.

---

## Authorization Model

Seven permissions were added to `domain.PermissionCatalog`, in the same
vocabulary the route guards check:

```
organization:read                  view the organization profile
organization:update                edit it
organization:members:read          list and read members and invitations
organization:members:invite        invite, re-send, revoke invitations
organization:members:update        change roles
organization:members:deactivate    deactivate, reactivate, revoke
organization:audit:read            read the membership audit history
```

They are catalogue entries, not a second system: `admin`/`root` hold `*` and
already pass all of them. They exist so a *scoped* business role can be granted
member administration without being handed the whole tenant.

The frontend only reflects authorization the server has already established;
`can_edit` and `can_resend` come from the API rather than from a client-side
guess.

---

## Tenant Isolation

The tenant is **never in the request**. Every administrative handler reads its
organization from `middleware.GetContext` — the authenticated session. There is
no path segment, query parameter or body field naming an organization, so there
is nothing for a caller to substitute.

`GormMembershipRepository` carries `organization_id` in every WHERE clause,
writes included. A forged or cross-tenant id matches no row: reads return
not-found, and writes affect zero rows and report not-found.

`FindInvitationByToken` is the single deliberate exception and takes no tenant:
acceptance happens before the caller has any relationship with the organization,
so there is none to scope by. The lookup is by hash — the plaintext never
touches a query — and the organization comes from the row that was found.

**Enumeration resistance.** A malformed member id, an id that never existed, and
a real id belonging to another tenant produce byte-identical answers. A `403`
for the third would confirm the id is real and belongs to somebody, which is the
first half of an enumeration. The HTTP suite and the live proof both assert the
three are identical.

All five parameterised routes are registered in
`internal/security/isolation/registry.go` as `Covered`, each citing the test
that covers it. The gate caught them automatically when they were added, and
caught the stale `/rbac/members/{id}/business-role` decision when it was removed.

---

## API Contracts

Authenticated, tenant from the session:

| Method | Path | Permission | Notes |
|---|---|---|---|
| GET | `/organization` | `organization:read` or `members:read` | profile + live counts + `can_edit` |
| GET | `/organization/counts` | any member | the sidebar's source |
| GET | `/organization/members` | `organization:members:read` | `q` `status` `role` `limit` `offset` `sort_by` `sort_dir` |
| GET | `/organization/members/audit` | `organization:audit:read` | mounted **before** `/:memberId` |
| GET | `/organization/members/:memberId` | `organization:members:read` | |
| PUT | `/organization/members/:memberId/role` | `organization:members:update` | `business_role` omitted = unchanged, `""` = cleared |
| PUT | `/organization/members/:memberId/status` | `organization:members:deactivate` | `{status, reason}` |
| GET | `/organization/invitations` | `organization:members:read` | never returns a token |
| POST | `/organization/invitations` | `organization:members:invite` | `201` |
| POST | `/organization/invitations/:id/resend` | `organization:members:invite` | rotates the token |
| DELETE | `/organization/invitations/:id` | `organization:members:invite` | |

Unauthenticated (mounted on `app` **before** the JWT gate, behind
`middleware.OptionalAuth`):

| Method | Path | Notes |
|---|---|---|
| GET | `/invitations/preview?token=` | organization name, invited address, role, expiry |
| POST | `/invitations/accept` | `201`; binds to the invited address when a session is present |

> **Fiber note.** `api.Use(Protected)` is registered on the `/api/v1` *prefix*
> and therefore wraps every route under it, whenever it was declared. The
> acceptance pair is mounted on `app` alongside the scanner-agent routes for
> exactly that reason: an invitee following their link may have no account at
> all, so a JWT gate would refuse the very people the endpoints exist for.

### Errors

`400` validation · `401` unauthenticated, or an existing account that must sign
in first · `403` self-modification, owner, wrong address · `404` unknown or
cross-tenant, indistinguishable · `409` already a member, live duplicate
invitation, already accepted, lost acceptance race · `410` revoked, expired,
already used · `429` resend cooldown.

Error bodies never reveal another tenant's existence, another user's membership,
or any token.

---

## Frontend Flows

**Settings › Members** — roster with search and status facets; inline role
select; Deactivate / Reactivate / Revoke; invitations with expiry, send count
and re-send; access history.

**Invite** — email + access, with a warning that the default option grants no
permissions (least privilege is the right default, but an administrator who does
not know that invites a colleague who sees an empty product and assumes it is
broken).

**Accept** (`/invitations/accept`) — previews the invitation before asking for
anything; warns when the current session is a different address; four distinct
messages for the four ways a link can be dead.

---

## UX State Matrix

| Surface | Loading | Empty | Error | Permission denied | Audit |
|---|---|---|---|---|---|
| Members | ✓ skeleton | ✓ + CTA | ✓ + retry | ✓ explains, offers no controls | ✓ |
| Invite | ✓ button state | N/A | ✓ server's message | ✓ action hidden | ✓ |
| Role change | ✓ per-row | N/A | ✓ server's message | ✓ select disabled | ✓ |
| Deactivation | ✓ per-row | N/A | ✓ toast | ✓ action hidden | ✓ |
| Invitations | ✓ skeleton | ✓ | ✓ + retry | ✓ actions hidden | ✓ |
| Audit history | ✓ skeleton | ✓ | ✓ + retry | ✓ tab hidden | ✓ |
| Organization settings | ✓ skeleton | N/A | ✓ + retry | ✓ read-only via `can_edit` | ✓ |
| Sidebar counts | ✓ no badge | ✓ no badge | ✓ no badge | ✓ no badge | N/A |

No control renders as functional that is not. Re-send is disabled by the
invitation's own `can_resend` flag — the same answer the API would give.

---

## Security Controls

* Tenant from the session, never the request.
* Uniform answers for unknown / malformed / cross-tenant ids.
* Bearer token: 32 random bytes, hashed at rest, constant-time comparison,
  expiring, rotating on resend and on revoke, never serialised, never audited.
* Acceptance bound to the invited address.
* Self-modification refused; owner protected; last administrator protected.
* Withdrawal destroys the refresh lineage.
* Resend rate-limited (60 s) and capped (10).
* Two database uniqueness constraints — one membership per (organization, user),
  one *pending* invitation per (organization, email) — carrying the weight the
  application's read-then-write checks cannot under concurrency.
* Transactional acceptance, conditional on the invitation still being pending.
* Allowlisted response views: a column added to a model tomorrow cannot leak by
  default.
* Mail transport reports failure honestly instead of returning nil.

---

## Test Strategy

**Unit — domain (`internal/domain`)**
Exhaustive transition matrix over every ordered pair of states; `SetStatus`
derives `is_active` and stamps timestamps; legacy `NULL`/`""` fallback;
`CheckRoleChange` and `CheckStatusChange` including last-administrator and
self-modification; token unguessability (200 mints, length, no collisions);
**a JSON round-trip that fails if anything token-shaped is serialised**; expiry
projection; the full resend policy including the cap.

**Unit — use cases (`internal/application/membership`)**
Tenant-scoped listing; cross-tenant read indistinguishable from missing; role
change applied, audited and session-revoked; escalation, owner, unassignable
role and cross-tenant refusals; last-administrator guard; the status cycle;
invite validation and conflicts; honest delivery reporting on all three
transports; resend rotation, cooldown and cap; revoke killing the link;
cross-tenant invitation isolation; acceptance for new and existing accounts;
email mismatch; unauthenticated existing account; revoked membership not revived
by a link; deactivated-after-invite reinstatement; replay yielding one
membership; onboarding completion and its best-effort failure.

**Repository (`internal/infrastructure/repository`)**
Real GORM against sqlite, schema reconciled from the models (never hand-written
DDL) plus the production unique indexes. Tenant isolation on every read and
write; the page-size cap; filter-vs-page totals; legacy `NULL` status; the
empty-status regression; one membership per user per org; one pending invitation
per email; token lookup; transactional single-consumption acceptance; counts.

**HTTP integration (`internal/handler`)**
Nine tests through the real handler, real routes and real permission guards:
full lifecycle from profile to audit; counts open to ordinary members and
tenant-scoped; every administrative route refused for a member without the
permission, with nothing mutated as a side effect; cross-tenant refusal with
byte-identical answers for real, invented and malformed ids; token misuse;
resend/revoke/expiry; last-administrator protection; the omitted-vs-empty
business-role distinction.

**Frontend (`vitest`)**
16 tests: the complete UI state matrix including permission-denied; status in
words rather than colour alone; owner and self rows offering no controls; role
change reporting the server's reason; confirmation required before withdrawal
with the reversible alternative offered; re-send disabled for the server's
reason; and three tests asserting the product never claims a delivery it did not
make.

**E2E (`tests/e2e/journey.members.spec.ts`)**
Six scenarios (12 cases across two projects): roster cross-checked against the
API, invite → appears → link opens a usable acceptance page, revoke asks first
then kills the link, withdrawal asks first and is reversible, access history,
and the sidebar badge matching real counts. **Written and listed, not executed
— see Known Limitations.**

**Security**
Cross-tenant enumeration, mutation, invitation and audit isolation; IDOR/BOLA
via forged ids; privilege escalation via self-modification; token reuse, revoked
tokens and email mismatch; enumeration resistance; rate limits; and a test that
fails if token material is serialised anywhere.

---

## Lifecycle Validation Matrix

Filled from the live run (90/90) and the automated suite.

| Capability | Positive | Negative | Expiry | Cross-Tenant | Permission | Audit | UI |
|---|---|---|---|---|---|---|---|
| List members | ✓ | ✓ bad filter 400 | N/A | ✓ 404 / absent | ✓ 403 | N/A | ✓ |
| Member detail | ✓ | ✓ 404 | N/A | ✓ identical answer | ✓ 403 | N/A | ✓ |
| Invite | ✓ 201 | ✓ 400 / 409 | ✓ 7 days | ✓ scoped | ✓ 403 | ✓ create | ✓ |
| Accept invite | ✓ 201 | ✓ 403 / 401 / 404 | ✓ 410 | ✓ tenant from token | ✓ n/a public | ✓ create | ✓ |
| Resend invite | ✓ 200 rotates | ✓ 429 / cap / 409 / 410 | ✓ extends | ✓ 404 | ✓ 403 | ✓ update | ✓ |
| Revoke invite | ✓ 200 | ✓ 410 twice | ✓ | ✓ 404 | ✓ 403 | ✓ revoke | ✓ |
| Role assignment | ✓ 200 | ✓ 403 self / 400 last-admin | N/A | ✓ 404 | ✓ 403 | ✓ update | ✓ |
| Deactivate | ✓ 200 | ✓ 403 self / 400 terminal | N/A | ✓ 404 | ✓ 403 | ✓ update | ✓ |
| Revoke member | ✓ 200 | ✓ 400 reactivate | N/A | ✓ 404 | ✓ 403 | ✓ revoke | ✓ |
| Audit history | ✓ 200 | ✓ 400 bad entity | N/A | ✓ isolated | ✓ 403 | ✓ | ✓ |
| Org metadata | ✓ 200 | ✓ | N/A | ✓ own tenant | ✓ 403 | N/A | ✓ |

---

## Performance Considerations

* **Indexes** — `(organization_id, status)` on memberships;
  `(organization_id, status)`, `(organization_id, email)`, `expires_at` and a
  unique `token_hash` on invitations; unique `(organization_id, user_id)`.
* **No N+1** — the roster joins `users` (so search and sort happen in SQL);
  invitation inviters resolve in one `EmailsByIDs`; audit actors likewise.
* **One pass for counts** — `COUNT(*) FILTER` returns every membership number in
  a single row, plus one indexed count for pending invitations.
* **Bounded** — every listing is clamped to 200 rows regardless of the caller's
  request. The sidebar reads counts, never a member list it would count itself.
* **Cheap refresh** — counts carry a 30-second stale window.

A dedicated load test is not warranted at this stage: the queries are single
indexed lookups or one aggregate per tenant, bounded by the page cap, and the
largest realistic tenant is thousands of members rather than millions of rows.
This is a stated judgement, not a measurement.

---

## Live Proof

See `docs/W0-04_ORGANIZATION_MEMBER_MANAGEMENT_LIVE_PROOF.md`.

---

## Known Limitations

1. **E2E written but not executed.** The shared Playwright harness authenticates
   by seeding `auth_token` into localStorage, but the application migrated to
   HttpOnly cookies and now actively *removes* that key — so `storageState`
   authenticates nothing and every spec lands on the login screen. Reproduced
   with `journey.settings.spec.ts` on an unmodified checkout, so it predates this
   work. The harness also has no TOTP step, so W0-03's mandatory admin MFA blocks
   `global-setup` before that. Both are recorded in the live-proof record; fixing
   the harness is follow-up work, not member management.
2. **No SMTP on this host.** The mail path is exercised through the real
   `Transport` code in unit tests and through the honest `Unconfigured`
   transport live. Delivery to a real mailbox has not been observed.
3. **Self-registration memberships carry no actor** in the audit trail, and
   render as "System". There genuinely is no third party at that moment.
   Attributing them to the registrant would need the registration path to stamp
   an audit actor — a change in W0-03's territory.
4. **The organization profile is read-only.** No endpoint writes it. The UI says
   so rather than offering inputs that save nothing.
5. **Only the invitations badge is live.** The fabricated risk and mitigation
   badges were removed rather than replaced; wiring real counts for them needs
   endpoints those screens do not yet expose.
6. **Notification-preference storage is unchanged** (still localStorage) — out
   of scope here.

---

## Follow-up Work

* Repair the E2E auth harness for cookie sessions and add a TOTP step, then run
  `journey.members.spec.ts` in CI.
* Stamp an audit actor on self-registration so founding memberships are
  attributed.
* Writable organization profile (`PUT /organization`) behind
  `organization:update`, which the catalogue and `can_edit` already anticipate.
* Live counters for the risk and vulnerability nav items.
* Bulk invite from a pasted list or CSV.
* Ownership transfer as a first-class, audited action.
